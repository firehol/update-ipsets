# TODO — Archived Feed State

## Purpose

Protect the application from abandoned or repurposed upstream feeds by adding a
terminal archived state for feeds/artifacts that are no longer trusted as live
inputs. Archived items must preserve historical local state for visibility, but
they must stop all automatic retries and must not accept new upstream content
until explicitly unarchived.

## TL;DR

- user wants a terminal state for feeds that are no longer live.
- Archived feeds must never be retried automatically.
- user is now considering `archived` as a health state that is entered
  automatically after a feed remains `unavailable` for more than 2 months.
- user also wants operators to be able to trigger a manual health check, and a
  successful check should allow the feed to leave `archived` naturally.
- Archived feeds must remain visible on the public site; archival is not a
  visibility flag and the public site must not create holes in feed URLs.
- user is now reconsidering raw/public archived URLs and is leaning toward
  disabling archived feed URLs that could be used operationally, while keeping
  archived feeds visible for historical reference and completeness.
- user clarified that "all URLs" means:
  - the upstream source URL
  - the local download URL that gives the user the raw IP feed
  - not the website's analytical artifacts such as comparisons, geo, ASN, and
    similar historical/reference pages
- The reason is security: an abandoned domain or URL can later be reclaimed and
  used to inject malicious data into the system.

## Analysis

### Verified current behavior

### Follow-up verification after install

- Live verification on 2026-04-22 shows a likely remaining runtime gap:
  - the admin UI reports only `archived = 2`
  - user expects materially more feeds to have crossed the archival threshold
- This must be checked against:
  - live `/api/v1/admin/status`
  - live `/api/v1/admin/feeds`
  - installed cache state under `/opt/update-ipsets`
  - the runtime classification path for long-unavailable feeds
- Working theory to verify or reject:
  - either the archived classification is still too narrow in live conditions
  - or the admin summary/counting path is excluding feeds that the operator
    expects to count
- Verified cause:
  - the live archived count was low because archival classification was using
    the active downloader failure streak too narrowly
  - many currently unavailable feeds had a recent `failure_started_date`, but
    their last usable local refresh was already older than
    `unavailable threshold + archival threshold`
  - this made obviously dead feeds stay in `unavailable` instead of advancing
    to `archived`
- UI follow-up:
  - the admin heartbeat/cards layout currently leaves `unmaintained` alone on a
    separate row
  - user wants the health cards to stay on the same row with the others
    instead of producing an ugly single-card wrap

- The product currently has operational enable/disable, but no terminal archived
  lifecycle state:
  - `pkg/config/config.go`
  - `pkg/engine/enabled_state.go`
  - `specs/feeds.md`

- Scheduler retry/backoff currently continues indefinitely for enabled feeds:
  - `pkg/scheduler/scheduler.go:1140-1226`
  - Backoff rules:
    - start at cadence / 16
    - double on failures
    - cap at ordinary cadence until unmaintained
    - then continue doubling up to 43200 minutes (30 days)
  - There is no terminal "stop retrying forever" state.

- Feed health classification currently marks bad feeds as `unavailable`,
  `empty`, `delayed`, `risky`, or `unmaintained`, but these are observational
  classes, not lifecycle controls:
  - `pkg/feedhealth/feedhealth.go:10-19`
  - `pkg/feedhealth/feedhealth.go:74-159`
  - `pkg/feedhealth/feedhealth.go:266-292`

- Single-source retention derivatives already reuse the parent's effective
  last-change clock for health classification in the current engine path:
  - `pkg/engine/effective_entry.go:52-63`
  - `pkg/engine/effective_entry_test.go:12-65`
  - this means user's "single source derived health follows the source"
    expectation is already partially implemented for retention derivatives

- Merge feeds currently use the newest effective parent change timestamp for
  operator-facing last-change views, but merge composition excludes only
  disabled inputs today:
  - newest parent timestamp for effective entry:
    - `pkg/engine/effective_entry.go:52-63`
    - `pkg/engine/effective_entry_test.go:68-109`
  - merge composition skips only inputs for which
    `EffectiveSourceEnabledForRun(...)` is false:
    - `pkg/engine/feed_body_stage.go:337-360`
  - current feed spec also only says merges are evaluated against currently
    enabled inputs:
    - `specs/feeds.md:254-259`
  - therefore excluding `archived` or `unmaintained` inputs from merges would be
    a new product rule, not existing behavior

- Disabled is not strong enough for user's requirement:
  - Disabled removes the feed from ordinary scheduling:
    - `pkg/engine/enabled_state.go:44-47`
    - `pkg/scheduler/scheduler.go:1100-1107`
  - But explicit operator actions can override the root disable bit:
    - `pkg/engine/helpers.go:41-48`
    - `pkg/engine/enabled_state.go:23-27`
    - `pkg/engine/download_stage.go:255-259`
  - So a disabled feed today is not a true terminal trust boundary.

- Manual admin actions currently schedule recheck/reprocess directly:
  - `pkg/web/admin.go:311-352`
  - If we add archival, these endpoints will need explicit policy.

- Current lifecycle spec only defines:
  - known
  - enabled/disabled
  - downloader waiting/active
  - processing waiting/active
  - committed/published
  - settled exceptional condition
  - `specs/feeds.md:290-316`
  - There is no archived / retired / terminal-trust stage.

- Current config semantics already distinguish:
  - public vs hidden
  - historical
  - deleted historical names
  - `specs/config.md:42-82`
  - But there is no config-level notion of "known, preserved, but no longer
    live".

### Security relevance

- user's threat model is valid:
  - a feed may fail for months or years
  - the scheduler still retries it forever
  - if the upstream domain or path is later taken over, the downloader will
    trust that location again and attempt to ingest it
  - if parsing succeeds, the new content can become canonical local feed state

- The current system treats long-term failure as a health state, not as a trust
  decision. That is the gap.

### Verified nearby existing concepts

- `historical: true` keeps a feed queryable but only changes public-default
  visibility, not scheduler trust:
  - `pkg/config/config.go:197-201`
  - `specs/config.md:404-409`
  - `specs/feeds.md:467-471`

- Current admin exposure is incomplete relative to user's new requirement:
  - the feed modal shows `historical` today:
    - `ui/src/components/admin/feed-modal.tsx:154-165`
  - the admin table exposes filter axes for:
    - `health`
    - `kind`
    - `category`
    - `hidden`
    - `disabled`
    - `ui/src/components/admin/feeds-table.tsx:157-167`
  - there is currently no `historical` filter axis in the admin table
  - the current admin spec only says that if historical is exposed as a filter,
    it must be a separate axis:
    - `specs/admin-ui.md:257-258`

- `deleted:` is stronger than needed here:
  - deleted names are cleanup/migration entries and remove local state during
    startup cleanup:
    - `pkg/engine/helpers.go:160-171`
    - `pkg/engine/helpers.go:205-220`
  - That does not fit user's requirement to preserve the current state and stop
    changing it.

- Legacy bash had `disabled` and deleted-name cleanup, but no explicit archived
  terminal source state:
  - `/home/user/src/firehol/firehol/sbin/update-ipsets`
  - `ipset_disabled()` exists
  - extracted config supports `dont_enable_with_all`
  - no distinct archived/retired-live-state contract was found

- Current public and admin route shapes are global and do not yet support an
  archived-specific integrity scope toggle:
  - public feed detail pages are stable SPA routes under `/ipsets/{name}`:
    - `pkg/web/server.go:797-805`
  - raw public downloads serve only real local files and return `404` when
    missing:
    - `pkg/web/server.go:755-787`
  - public feed-scoped API/asset routes include metadata/history/compare/
    retention/insights/raw downloads:
    - `specs/website.md:106-110`
    - `pkg/web/server.go:398-464`
    - `pkg/web/server.go:732-787`
  - the public feed page currently exposes both operational feed URLs:
    - a prominent local raw download CTA in the hero:
      - `ui/src/components/feed-detail/hero.tsx:66-80`
    - the upstream source URL in the hero and the specs section:
      - `ui/src/components/feed-detail/hero.tsx:82-91`
      - `ui/src/components/feed-detail/section-specs.tsx:99-113`
  - the public feed-detail UI is API-first and depends on these machine-readable
    per-feed routes:
    - metadata: `ui/src/lib/api.ts:97-100`
    - compare: `ui/src/lib/api.ts:163-168`
    - history: `ui/src/lib/api.ts:171-180`
    - retention: `ui/src/lib/api.ts:183-186`
    - insights: `ui/src/lib/api.ts:189-206`
    - search: `ui/src/lib/api.ts:217-220`
  - implication:
    - disabling all archived per-feed public APIs would also break the current
      human-facing archived detail page unless a new human-only archived-detail
      data path is added
  - admin integrity is a single global report/recovery surface today:
    - `GET /api/v1/admin/integrity`
    - `POST /api/v1/admin/integrity/reprocess`
    - `pkg/web/server.go:569-570`
    - `pkg/web/integrity.go:40-68`
    - `pkg/web/integrity.go:123-180`
  - the heartbeat and integrity panels currently show one global integrity
    count/result with no include/exclude toggle:
    - `ui/src/components/admin/heartbeat.tsx:166-179`
    - `ui/src/components/admin/integrity-panel.tsx:78-147`

## Decisions

### User decisions already made

- We need a terminal state for non-live feeds.
- Archived feeds must not be retried automatically.
- This is a security requirement, not just a UX preference.
- user currently recognizes only two non-health feed-control flags as meaningful:
  - `hidden` = do not show on the public UI
  - `disabled` = do not process this again
- user challenged the current use of `historical` and wants the state model
  clarified before implementation proceeds.
- New spec requirement from user:
  - all internal feed states and flags MUST be exposed to the admin UI
  - they MUST be available both for filtering and for display in the feed modal
  - operators must never be blind to an internal feed state
- user decided:
  - archived feeds remain public on the website for historical/reference use
  - but archived feeds MUST disable the two operational feed URLs:
    - the upstream source URL
    - the local raw feed download URL
- user decided:
  - there MUST NOT be an `archived: true` per-feed configuration flag
  - `archived` is not a curated config property
  - `archived` is a derived health state entered automatically from prolonged
    `unavailable`
- user decided:
  - remove `historical` as a behavioral flag/state from the product
  - if legacy/bash-restored origin still matters, it may survive only as
    non-behavioral provenance/metadata
- user decided:
  - single-source derived feeds follow the health of their parent source
  - merges exclude both `archived` and `unmaintained` inputs from composition
- user decided:
  - the public site must dynamically show merged feed composition
  - the public site must explicitly flag dynamic merge exclusions
  - users must be able to see which merge inputs are currently included and
    which are excluded, with reasons
- user decided:
  - admin integrity gets a single operator-visible tick, default disabled, to
    include archived feeds in integrity and recovery
  - changing this tick invalidates any currently shown integrity result and
    forces a fresh integrity pass for the selected scope
  - archived recovery becomes available only after that fresh scoped integrity
    pass completes, so the operator sees exactly which archived feeds are in
    scope before running recovery
- user is now proposing:
  - keep `recheck` and `reprocess` working as they do today, even for archived
    feeds
  - do not introduce a separate `check-health` action
  - `recheck` should satisfy the archived manual health-check need

### Latest user proposal under evaluation

- user is considering `archived` as part of feed health, not only as a
  separate lifecycle/control flag.
- Proposed trigger:
  - a feed enters `archived` automatically after remaining `unavailable` for
    more than 2 months
- Proposed recovery:
  - the operator can use the existing `recheck`
  - a successful `recheck` can move the feed out of `archived` naturally
- Proposed health transition:
  - `archived` replaces `unavailable` after the archival threshold; the two do
    not coexist on the health axis

### Design decisions

1. **What the source of truth for `archived` is**
   - Fact:
     - the earlier "config-curated `archived: true`" idea is now in conflict
       with user's chosen model where `archived` is an automatic health state
       entered after 2 months of continuous `unavailable`
   - A. `archived` is purely a derived health state
     - no per-feed `archived:` config flag
     - only threshold/runtime config controls it
     - manual health check can move a feed out of `archived`
   - B. `archived` has two sources of truth:
     - automatic health transition
     - plus a manual/config archive override
   - C. `archived` is manual/config only
     - this would contradict user's chosen automatic-health model
   - Decision: `A`
   - Why:
     - it matches user's current design exactly
     - it avoids conflicting state sources
     - user already reserved `disabled` for manual operator control

2. **What archived does to manual actions**
   - A. Block both admin `recheck` and `reprocess` until explicitly unarchived
   - B. Block `recheck`, allow `reprocess` from existing local canonical state
   - C. Allow both with warnings
   - D. Keep current semantics:
     - `recheck` continues to fetch/recompose as it does today
     - `reprocess` continues to rebuild from local state as it does today
     - archived adds no separate manual-action restrictions
   - Decision: `D`
   - Why:
     - user confirmed that explicit operator intent is sufficient guardrail for
       archived manual actions.
     - This makes `recheck` double as the archived health probe.

3. **How archived should appear publicly**
   - A. Keep publicly visible, clearly labeled as archived/frozen
   - B. Hide from public by default but keep queryable
   - C. Fully hidden from public, admin only
   - Decision: `A`
   - Why:
     - Archived does not mean deleted.
     - Users should understand the feed exists but is no longer live.
     - This preserves historical continuity and avoids silent disappearance.

4. **Which public URLs archived feeds should disable**
   - A. Disable only the operational feed URLs:
     - upstream source URL
     - local raw feed download URL
     - keep feed detail and analytical/reference pages public
   - B. Disable all public feed routes
   - C. Keep all public URLs but label the feed archived
   - Decision: `A`
   - Why:
     - user clarified that archived feeds are historical/reference entries, not
       operational feed sources.
     - This protects users while preserving the website's explanatory and
       analytical value.

5. **How derived feeds behave when a parent is archived**
   - A. Single-source derived feeds follow the parent health; merges exclude
     both `archived` and `unmaintained` inputs from composition
   - B. Parent and all descendants become frozen; no local reprocess path
   - C. Descendants stay live independently
   - Decision: `A`
   - Why:
     - user has now made the precise derivative and merge rules explicit.

6. **Whether auto-archival exists**
   - A. No auto-archival; only explicit human-curated archival
   - B. Optional suggestion only: system can recommend archival, never enforce it
   - C. Automatic archival after threshold
   - D. Automatic archival after threshold, but reversible by explicit operator
     health check success
   - Decision: `D`
   - Why:
     - This matches user's latest direction more closely.
     - It keeps the system conservative by default after long-term
       unavailability, while still allowing an operator to re-validate a feed
       deliberately.

7. **Can we reuse `historical` instead of adding `archived`**
   - A. Reuse `historical` for archival/trust-stop semantics
   - B. Keep `historical` as-is and add separate `archived`
   - C. Rename `historical` and repurpose it, migrating all current usages
   - D. Remove `historical` as a behavioral flag and keep legacy/baseline origin
     only as non-behavioral metadata if needed
   - Decision: `D`
   - Why:
     - user's current model already has:
       - health: including `archived`
       - visibility: `hidden`
       - operator control: `disabled`
     - keeping `historical` as behavior leaves an overlapping fourth axis whose
       current meaning is mainly public-default hiding of older bash-restored
       feeds
     - if legacy provenance still matters, metadata can preserve it without
       affecting behavior

8. **What integrity should do for archived public feeds**
   - A. Exclude archived feeds entirely from integrity and recovery
   - B. Keep archived feeds in integrity for local published-state correctness,
     but forbid integrity-driven upstream `recheck`
   - C. Keep archived feeds in integrity and allow both `recheck` and
     `reprocess`
   - D. Add an operator-visible toggle, default off, to include archived feeds
     in integrity and recovery explicitly
   - Decision: `D`, with a stricter UX rule:
     - toggling archived inclusion invalidates the previous integrity report
     - the product MUST run a new integrity pass for the newly selected scope
     - recovery actions MUST remain disabled until that scoped pass finishes
     - recovery then applies only to the findings shown by that scoped pass
   - Why:
     - This matches user's latest direction and UX rationale.
     - The default remains safe.
     - The forced rerun prevents recovery from using a stale or narrower report,
       which is necessary because today `Recover all` schedules work directly
       from the currently loaded report:
       - `pkg/web/integrity.go:132-179`
       - `ui/src/components/admin/integrity-panel.tsx:62-75`

9. **What direct raw download routes should do for archived feeds**
   - A. Disable archived raw feed distribution URLs by policy even when the
     local file still exists, while keeping archived detail and analytical
     pages available
   - B. Return raw files if they still exist, but hide the download CTA
   - C. Return placeholder or redirect content
   - Decision: `A`
   - Why:
     - Archived feeds are historical/reference entries, not operational feed
       sources.
     - The product must not encourage or permit operational consumption of an
       archived feed from the public site.

10. **How derived feeds behave when a parent becomes archived**
   - A. Single-source derived feeds inherit parent health; merges exclude only
     `archived` inputs
   - B. Single-source derived feeds inherit parent health; merges exclude both
     `archived` and `unmaintained` inputs
   - C. Single-source derived feeds inherit parent health; merges exclude every
     input whose health is not `healthy`, `delayed`, `risky`, or `empty`
   - D. Single-source derived feeds keep their own independent health; merges
     stay unchanged
   - Decision: `B`
   - Why:
     - user explicitly selected this boundary.
     - It keeps merges protected from both no-longer-trusted inputs and
       effectively abandoned inputs without overreacting to merely delayed or
       risky sources.

11. **How the operator-triggered archived health probe should surface in the UI/API**
   - A. Reuse the existing `recheck` action, but relabel it in the UI as
     "Check health" for archived feeds
   - B. Introduce a new explicit archived-only action, e.g. `check-health`
   - C. Keep the existing `recheck` label everywhere
   - D. No separate action:
     - archived feeds use the existing `recheck` and `reprocess` actions
     - no relabeling, no new endpoint
   - Decision: `D`
   - Why:
     - user confirmed that the existing actions are sufficient.
     - Extra action surface would add UX complexity without adding real value.

## Plan

1. Extend the specs to define archived feed semantics naturally across:
   - `specs/config.md`
   - `specs/feeds.md`
   - `specs/downloader.md`
   - `specs/admin-ui.md`
   - `specs/website.md` if public labeling is retained
2. Add runtime/config support only for archival thresholds and UI/API behavior:
   - do not add a per-feed `archived:` config flag
   - remove `historical` as a behavioral flag
3. Update enable/effective-run logic so archived cannot fetch new upstream
   content, even through manual recheck.
4. Allow or block local reprocess per the chosen policy.
5. Update scheduler snapshots and admin feed rows to expose archived state
   explicitly.
6. Exclude archived items from automatic retry scheduling and integrity
   recovery that would require upstream refresh.
7. Update public/admin labeling so archived is visible and unambiguous.
8. Update public merge detail surfaces so they show dynamic included inputs,
   excluded inputs, and exclusion reasons.
9. Add tests for:
   - no automatic retry
   - no manual recheck if archived
   - allowed/blocked manual reprocess per chosen policy
   - archived parent / derivative behavior
   - dynamic merge inclusion/exclusion visibility
   - API/UI status exposure
10. Compare the live installed archived count against the full feed inventory and
    close any remaining mismatch between runtime behavior and the spec.

## Implied decisions

- Archived must be different from disabled:
  - disabled is an operational scheduling toggle
  - archived, under user's current proposal, is an automatically-derived trust
    health state with operational consequences

- Archived must preserve existing local canonical feed bodies and website
  artifacts unless separately deleted.

- Archived must not be treated as an integrity defect just because no new
  upstream refresh occurs while it is archived.

- Archived must not silently accept future domain resurrection unless a human
  explicitly re-validates the feed.

- Archived public visibility means feed-detail and catalog surfaces should
  continue to represent archived feeds even if some local downloadable artifacts
  are missing.

- user's `1A` decision must apply to every public path that gives the user the
  raw feed body, not only the obvious hero download button:
  - disable raw file routes such as `/{feed}.ipset`, `/{feed}.netset`,
    `/files/{feed}.ipset`, `/files/{feed}.netset`
  - disable any plain-text feed-body API route such as
    `/api/v1/sets/{name}/data`
  - the public metadata/detail payload for archived feeds must not expose an
    actionable upstream source URL or actionable local raw-download URL

- Because merge membership is now health-aware, the public merge detail surface
  cannot rely on a static source list alone:
  - it must expose the current included inputs
  - it must expose the current excluded inputs
  - it must expose exclusion reasons
  - these facts must be dynamic because health-based exclusions change over time

## Testing requirements

- Unit tests for runtime archival threshold/config parsing and validation.
- Unit tests for scheduler next-due exclusion for archived items.
- Unit tests for downloader fetch refusal when archived.
- Unit tests for admin/manual recheck and reprocess policies on archived items.
- Unit tests for parent/child behavior with archived parents.
- Unit/UI tests for dynamic merge input inclusion/exclusion rendering.
- API tests for admin/public exposure of archived state.

## Documentation updates required

- Add archived state to lifecycle documentation.
- Explain the difference between:
  - disabled
  - hidden
  - archived
  - deleted
- Document merge composition visibility and dynamic exclusion reasons.
- Document the security rationale: abandoned upstreams can later become hostile.
