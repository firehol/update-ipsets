# TODO: Spec Gap Alignment

## Purpose

Bring the implementation into full alignment with the authoritative contracts in `specs/*.md`, across backend, pipeline, filesystem layout, admin/public APIs, and website behavior. If the implementation reveals behavior that should exist but is missing from the specs, define that behavior explicitly in the appropriate spec first and then align code with it.

## TL;DR

- Perform a full gap analysis between implementation and specs.
- Fix every implementation/spec mismatch that can be verified from the codebase.
- Repeat the analysis after each batch of fixes until no further gaps are found.
- Update `specs/*.md` immediately for any newly formalized behavior required by the current implementation and design goals.

## Analysis

- Initial state:
  - User requested a new TODO file instead of reusing an existing tracker.
  - Full code/spec audit has not started yet.
  - This file will be expanded with evidence-backed findings as the audit progresses.
- Verified gaps found in the first audit pass:
  - Integrity recovery planning drops retention derivatives after blocked-parent
    handling, so a retention derivative with intact local feed-body state but
    missing/stale public outputs never gets scheduled for `reprocess`.
    Evidence:
    - `pkg/engine/integrity_recovery.go:24-44`
    - `pkg/engine/integrity.go:176-213`
  - The integrity panel exposes a per-row `reprocess` action even though the
    integrity contract requires recovery to split into `recheck` vs
    `reprocess`, and some findings require parent/artifact recovery rather than
    feed reprocess.
    Evidence:
    - `ui/src/components/admin/integrity-panel.tsx:62-85`
    - `ui/src/components/admin/integrity-panel.tsx:255-266`
    - `pkg/web/admin.go:307-319`
  - Admin queue wording still describes the processing consumer as a
    `processing scheduler`, which contradicts the explicit two-loop pipeline
    contract.
    Evidence:
    - `ui/src/components/admin/current-run.tsx:198-204`
    - `specs/pipeline.md`
  - The heartbeat caption for `unavailable` says `failing beyond threshold`,
    but the feed contract also uses `unavailable` for enabled feeds before
    first successful local publication.
    Evidence:
    - `ui/src/components/admin/heartbeat.tsx:112-119`
    - `specs/feeds.md`
  - Feed-modal action copy describes `recheck` as a forced download and
    `reprocess` as secondary-only regeneration, which is incorrect for
    derivatives and too narrow for the engine replay contract.
    Evidence:
    - `ui/src/components/admin/feed-modal.tsx:214-227`
    - `specs/admin-ui.md`
  - The integrity recovery response does not expose the actual split recovery
    plan. It returns finding feed names even when the recovery work is a parent
    `recheck`, so the machine-readable contract is weaker than the integrity
    recovery model defined in specs.
    Evidence:
    - `pkg/web/integrity.go:92-151`
    - `ui/src/lib/api-types.ts:692-699`
    - `ui/src/lib/api.ts:395-404`
    - `specs/integrity.md`
    - `specs/admin-ui.md`
  - The integrity API already distinguishes malformed secondary outputs, but
    the TypeScript contract and admin UI ignore `malformed_files`, so operators
    lose the promised distinction between malformed and stale output classes.
    Evidence:
    - `pkg/engine/integrity.go:60-63`
    - `ui/src/lib/api-types.ts:664-676`
    - `ui/src/components/admin/integrity-panel.tsx`
    - `ui/src/components/admin/feed-modal.tsx`
    - `specs/integrity.md`
  - The homepage/public-category path still hardcodes `asn` and `geolocation`
    as non-public categories instead of deriving category visibility from
    configuration, even though the homepage and website specs require category
    semantics to come from configuration.
    Evidence:
    - `ui/src/lib/explorer-state.ts`
    - `ui/src/components/home/home-explorer-filter-rail.tsx`
    - `pkg/engine/home_summary.go:322-334`
    - `pkg/engine/public_categories.go`
    - `specs/homepage.md:276-299`
    - `specs/website.md:195-200`
    - `specs/config.md`
  - The admin feed and artifact surfaces still expose raw stored status codes
    such as `parse_failed`, `history_snapshot_failed`, and `not_modified`
    instead of operator-facing meanings, and they do not explicitly classify
    the latest local failure as downloader-stage vs processing-stage. This
    violates the admin contract requirement to present operational meaning and
    to distinguish severe runtime faults without reading logs.
    Evidence:
    - `ui/src/components/admin/feeds-table.tsx:1036-1047`
    - `ui/src/components/admin/feed-modal.tsx:989-1008`
    - `ui/src/components/admin/artifacts-panel.tsx:90-114`
    - `pkg/web/admin.go:624-628`
    - `pkg/web/admin.go:751-760`
    - `specs/admin-ui.md:210-211`
    - `specs/admin-ui.md:345-358`
  - The live queue snapshots still carry raw cache status codes through the
    scheduler lookup path, so queue subtitles can still show internal values
    such as `parse_failed` or `history_snapshot_failed` even after the feed and
    artifact rows were fixed.
    Evidence:
    - `pkg/scheduler/scheduler.go:249-267`
    - `pkg/scheduler/queue_state.go:10-23`
    - `ui/src/components/admin/current-run.tsx:555-566`
    - `specs/admin-ui.md:210-211`
  - The admin SPA shell is served without authentication even when admin
    credentials are configured. That exposes operator controls to
    unauthenticated users and contradicts the admin access model, which says
    the admin surface itself must be access-controlled.
    Evidence:
    - `pkg/web/server.go:402-417`
    - `pkg/web/feature_test.go:36-69`
    - `specs/admin-ui.md:31-39`
  - The public `/api/v1/status` endpoint exposes operator runtime detail,
    including scheduler snapshots and the raw engine status object with local
    paths, active feeds, phases, and run reports. The website contract keeps
    operator queue state on the admin side, so the public status payload is
    too broad.
    Evidence:
    - `pkg/web/server.go:178-183`
    - `pkg/engine/engine.go:114-129`
    - `specs/website.md:101-134`
    - `specs/website.md:251-259`
  - Admin auth is still fail-open when the auth env vars are unset. The
    middleware returns the protected handler directly in that case, making the
    admin surface public instead of unavailable. That violates the
    access-controlled admin contract.
    Evidence:
    - `pkg/web/middleware.go:87-103`
    - `specs/admin-ui.md:31-49`
  - Public feed catalog construction still includes provider-role datasets
    because it filters only `hidden` and `historical`. ASN and geolocation
    sources therefore leak into `/api/v1/sets` and the public feed navigation,
    even though the feed contract says provider databases are supporting
    datasets, not normal public feeds.
    Evidence:
    - `pkg/engine/public_catalog.go:57-74`
    - `ui/src/components/feed-sidebar.tsx:294-304`
    - `ui/src/components/site-header.tsx:37-44`
    - `specs/feeds.md:65-68`
  - Feed-scoped public publication still does not enforce the same
    public-feed boundary consistently. Hidden sources and supporting
    provider datasets can still remain reachable through feed-scoped
    compatibility artifacts and mirror paths even though the public
    feed API now rejects them.
    Evidence:
    - `pkg/engine/metadata.go:40-137`
    - `pkg/engine/web_ipsets.go:14-43`
    - `pkg/web/server.go:214-377`
    - `pkg/web/server.go:481-571`
    - `pkg/config/config.go:189-197`
    - `specs/website.md:93-101`
    - `specs/files-layout.md:302-330`
  - Public pairwise comparison and insight publication still derive from the
    full output-name set instead of the public publishable feed set, so hidden
    sources and supporting provider datasets can leak into peer-facing public
    comparison artifacts and derived public insights.
    Evidence:
    - `pkg/engine/output.go:294-548`
    - `pkg/engine/query.go:352-407`
    - `pkg/engine/insights.go:64-83`
    - `pkg/engine/unique_share.go:38-43`
    - `specs/processing-engine.md:163-181`
  - `sitemap.xml` enumeration still walks the unrestricted output-name set, so
    hidden or otherwise non-public feed identities can leak into published
    crawlable URLs even when the catalog/detail API boundary is fixed.
    Evidence:
    - `pkg/engine/output.go:710-738`
    - `specs/website.md`
    - `specs/files-layout.md`
- Verified follow-up gap after commit/install and external review pass:
  - The feed-health implementation and tests classify `unavailable` in two
    cases: before the first successful local publication, and after a feed that
    was previously available remains locally unavailable past the configured
    recovery threshold. The spec text only documented the pre-publication case,
    leaving the settled unavailability case underspecified even though the
    current implementation and UI already reflect it.
    Evidence:
    - `pkg/feedhealth/feedhealth.go:86-137`
    - `pkg/feedhealth/feedhealth_test.go:188-210`
    - `ui/src/components/admin/heartbeat.tsx:112-119`
    - `specs/feeds.md:190-226`

## Decisions

- User decision already made:
  - Create a new TODO file for this work.
  - Commit the completed alignment batch, run the local install flow, and then
    run external read-only reviewers (Claude, GLM, MiniMax, Kimi, Qwen) to
    perform the same implementation-versus-spec gap analysis.
- Working policy for this task:
  - Treat `specs/*.md` as the source of truth unless the specs are missing a necessary contract.
  - When a necessary contract is missing, define it in specs before aligning implementation.
  - Continue iterative audit/fix cycles until no additional verified gaps are found in the inspected surface.
- Autonomous decisions made for this implementation batch:
  - Extend integrity findings with explicit operator-visible recovery metadata
    (`recovery_action`, `recovery_targets`) so the admin UI can reflect the
    real recovery class for each finding without guessing.
  - Extend the integrity recovery response with class-split target lists so the
    API reports what was actually scheduled, not just which findings existed.
  - Add explicit category visibility to configuration (`public: false` for
    non-public/system categories) and derive public category filtering from
    configuration everywhere the homepage/public site needs it.
  - Add operator-facing admin status metadata (`last_status_label`,
    `last_problem_class`) for feeds and artifact parents in the authenticated
    admin API, so the UI can present stable operational meaning without
    duplicating raw-status decoding logic.
  - Centralize operator-facing status decoding in shared backend code and reuse
    it for queue items too, so the admin status model does not drift between
    feed rows, artifact rows, and live queue cards.
  - Treat the HTML admin shell under `/admin` and `/admin/*` as part of the
    authenticated operator surface, not just the JSON APIs.
  - Narrow the public `/api/v1/status` contract to coarse public service
    status only, and remove scheduler/backlog, filesystem-path, and other
    admin-only runtime details from that endpoint.
  - Make admin auth fail closed: if admin credentials are not configured, the
    admin surface must reject access instead of becoming public.
  - Exclude provider-role datasets (`use: [asn]`, `use: [geoip]`) from public
    feed summaries and public feed navigation surfaces; they remain operator
    and supporting-dataset concepts, not public feeds.
  - Apply the same public-feed eligibility rule to every feed-scoped public
    publication surface: compatibility JSON/CSV artifacts under `web/`,
    `/api/v1/sets/{name}/*`, and raw mirror downloads under `/files/`.
  - Restrict public pairwise comparison, insight publication, and public
    comparison API rows to public publishable feeds only; hidden feeds and
    supporting provider datasets are not public peers.
  - Restrict sitemap publication to public publishable feeds only.
  - Formalize `unavailable` as the backend health class for both
    pre-publication feeds and feeds that remain locally unavailable beyond the
    configured recovery threshold after previously having a local publication.

## Plan

1. Inventory the authoritative specs and the implementation areas that map to them.
2. Audit code against specs and record concrete evidence with file/line references.
3. Patch the confirmed integrity/admin gaps first, keeping backend behavior,
   API semantics, and operator wording aligned.
4. Run targeted tests/build/lint verification for changed areas.
5. Re-audit the same surfaces plus adjacent affected areas to look for newly exposed gaps.
6. Repeat steps 2-5 until the audit no longer finds verified mismatches.

## Implied Decisions

- The audit is repository-wide, not limited to one subsystem.
- Spec updates are part of the task, not optional documentation follow-up.
- Verification must cover both behavior and contract/documentation alignment.

## Testing Requirements

- Run targeted unit/integration tests for touched packages.
- Run broader project verification where feasible (`make test`, and narrower commands where faster feedback is needed).
- Verify frontend build paths if UI/source changes are required.
- Re-run verification after each significant batch of fixes.

## Documentation Updates Required

- Update any affected file under `specs/` immediately when behavior, contracts, or ownership need clarification.
- Update supporting docs if the implementation changes operator-facing behavior beyond the spec set.
