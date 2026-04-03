# TODO-release-master

## Purpose

Prepare `update-ipsets` for an upstream `firehol/update-ipsets` public release
and later production cutover on top of the legacy bash implementation.

Fit for purpose:

- keep the public site cheap to serve
- keep operator workflows explicit and well documented
- keep feed/catalog management maintainable
- keep security, auditability, and release hygiene first-class

**Important constraint from Costa**:

- no implementation work starts from this master TODO without explicit approval
- this file is the planning and execution control point

## TL;DR

Costa requested 22 items. After code/spec review and external research, they
fall into 8 workstreams:

1. **Public serving + precomputation**
   - precompute country/ASN detail payloads
   - reduce request-time CPU
   - add entity index pages
2. **Config/catalog reorganization**
   - move from one giant feed YAML to directory/file-per-feed authoring
   - preserve current data-driven catalog semantics
3. **Homepage and UX**
   - client-IP bootstrap
   - copy updates
   - browser back/forward performance fix
   - merged-feed presentation
4. **Cadence and health observability**
   - adaptive feed frequency
   - public health-transition log
5. **Data quality reviews**
   - all MISP feeds audit/import
   - critical infrastructure ASN review
   - visualization review
   - FireHOL merge review
6. **Public/operator documentation**
   - contributor-facing “how to add a feed / make a PR”
   - operator docs in `docs/`
   - GitHub wiki publication strategy
7. **Upstream release preparation**
   - new upstream repository
   - flattened history
   - security and path audit
8. **New public/API/AI surfaces**
   - public MCP server at `/mcp`
   - public paste-a-set overlap analysis
   - public feed composer page
   - AI-agent enrichment/evaluation workflows

The first approved low-hanging-fruit batch is now implemented:

- homepage copy changes (`#4`, `#5`, `#6`)
- homepage client-IP bootstrap (`#3`)
- browser back/forward chart sizing bug (`#7`)
- menu/index surfacing for countries and ASNs (`#17`, first slice)
- old TODO consolidation into `docs/todo-history/`

## Analysis

### External constraints and current official facts

#### GitHub wiki constraints

Official GitHub Docs currently say:

- every repository wiki is a separate wiki surface
- wiki content can be edited locally via a separate repository:
  - `https://github.com/YOUR-USERNAME/YOUR-REPOSITORY.wiki.git`
- public wikis are public
- wikis have a soft limit of 5,000 files
- if search-engine indexing matters, GitHub recommends GitHub Pages instead

Evidence:

- `https://docs.github.com/en/communities/documenting-your-project-with-wikis/about-wikis`
- `https://docs.github.com/en/communities/documenting-your-project-with-wikis/adding-or-editing-wiki-pages`

Implication:

- `docs/` should remain the source of truth in the main repo
- if Costa wants the docs “served as a GitHub wiki”, we likely need a
  sync/mirror workflow into the separate `.wiki.git` repository
- the wiki should not become the only source of truth

#### MCP Streamable HTTP constraints

The current official MCP transport spec says:

- the server must expose a **single MCP endpoint**
- that endpoint must support **POST** and **GET**
- a Streamable HTTP server should validate `Origin`
- localhost-only binding is recommended for local deployments
- authentication is recommended
- the endpoint can be a path like `https://example.com/mcp`

Evidence:

- `https://modelcontextprotocol.io/specification/draft/basic/transports`

Implication:

- public `/mcp` is feasible, but not “just another JSON endpoint”
- it needs an explicit security model, transport behavior, and a deliberate
  contract for tools/resources/prompts

#### MISP feed metadata facts

Official MISP documentation currently says:

- MISP ships a set of default feeds described in a simple JSON metadata file
- feeds can be in MISP/CSV/freetext formats
- the published MISP site documents the default-feed catalog
- the published contribution path for adding a feed is:
  - fork MISP
  - update the default feed JSON
  - open a pull request

Evidence:

- `https://www.misp.software/feeds/`
- `https://misp.github.io/misp-website/feeds/`
- `https://www.circl.lu/doc/misp/managing-feeds/`
- `https://www.misp-project.org/misp-training/handout/a.3-misp-feed_handout.pdf`

Implication:

- “add all MISP feeds” needs a real audit against the current MISP default-feed
  metadata, not just adding more warninglists blindly
- our contributor docs can mirror the same basic PR workflow logic

### Item-by-item current-state analysis

#### 1. Precompute all country and ASN data

**Current state**:

- country index is served live from:
  - `pkg/web/home_detail_api.go`
  - `handleCountryIndex() -> eng.CountryIndex()`
- country detail is served live from:
  - `pkg/web/home_detail_api.go`
  - `handleCountryDetail() -> eng.CountryDetail(code)`
- ASN index is served live from:
  - `pkg/web/home_detail_api.go`
  - `handleASNIndex() -> eng.ASNIndex()`
- ASN detail is served live from:
  - `pkg/web/home_detail_api.go`
  - `handleASNDetail() -> eng.ASNDetail(asn)`
- the heavy work is inside:
  - `pkg/engine/home_index.go`
  - `pkg/engine/home_detail.go`

Verified facts:

- `CountryIndex()` walks `EntriesSnapshot()` and geo comparison payloads at
  request time
- `CountryDetail()` walks the current feed inventory, loads comparison/provider
  data, and builds grouped summaries at request time
- `ASNIndex()` walks `EntriesSnapshot()` and per-feed ASN summaries at request
  time
- `ASNDetail()` walks the current feed inventory and builds grouped summaries at
  request time
- `ASNDetail()` also opens latest feed sets and computes per-ASN country
  distribution at request time
- the daemon already has a publication-time web artifact model:
  - staging via `pkg/engine/web_batch.go`
  - atomic publish into `web/`
  - cache-first serving from disk in `pkg/web/cache.go`

Implication:

- Costa is correct: country/ASN detail pages still generate CPU load on the
  daemon
- the same is now true for the new country/ASN index pages
- this should not become a new special-case subsystem; it should join the
  existing publication-time web artifact contract

Approved direction:

- move both index and detail surfaces together in the same pass
- keep `/api/v1/countries*` and `/api/v1/asns*` stable and make them thin
  artifact readers

#### 2. Config should be a directory with a file per feed

**Current state**:

- the primary authored catalog is still:
  - `configs/firehol.yaml`
- however, the code already supports loading directories of config fragments:
  - `pkg/config/config.go` -> `LoadDirectory(dir string)`
- runtime already merges 3 configured fragment directories:
  - `DistributionSuppliedIPSets`
  - `AdminSuppliedIPSets`
  - `UserSuppliedIPSets`
- merge points:
  - `pkg/engine/engine.go`

Implication:

- this is not greenfield work
- the code already understands directory-based config fragments
- the missing step is promoting fragment directories into the **primary authored
  catalog model**, instead of only as supplementary layers

#### 3. Homepage should auto-select the client IP when empty

**Current state**:

- homepage lookup is in:
  - `ui/src/components/home/home-ip-lookup.tsx`
- it renders the shared `IPSearchSurface`, but does not bootstrap a client IP
- backend already knows how to resolve client IPs from headers:
  - `pkg/web/middleware.go`
  - `CF-Connecting-IP`
  - `X-Forwarded-For`
  - `X-Real-IP`

Implication:

- the missing piece is a public bootstrap mechanism, not basic IP parsing
- likely implementation direction:
  - small public endpoint that returns the daemon’s view of client IP
  - homepage autofills only when the field is empty

#### 4. Hero title copy

**Current state**:

- current text lives in:
  - `ui/src/components/home/home-hero.tsx`
- current string:
  - `Normalize public IP feeds. Compare them on facts.`

This is a bounded copy-only change.

#### 5. “Paste any IPv4 address” copy

**Current state**:

- current text lives in:
  - `ui/src/components/home/home-ip-lookup.tsx`
- current string:
  - `Paste any IPv4 address.`

This is a bounded copy-only change.

#### 6. “What should never appear here” copy

**Current state**:

- current text lives in:
  - `ui/src/components/feed-detail/section-bogons.tsx`
- current string:
  - `Private IP Address Space`

This is a bounded copy-only change.

#### 7. Browser back/forward slowness and chart width/height warnings

**Current state**:

- user-reported warnings match current Recharts code almost exactly
- current chart containers include:
  - `ui/src/components/feed-detail/section-retention.tsx`
  - `ui/src/components/feed-detail/section-behavior.tsx`
  - `ui/src/components/feed-detail/hero.tsx`
- several current containers use:
  - `ResponsiveContainer width="99%" height="100%" ... minHeight={120}`

Working theory, grounded in the current code:

- some feed-detail charts are mounting/restoring while their container width is
  transiently zero/negative during browser history navigation
- the warnings likely correlate with the 20-second “forward” stall Costa sees

Implication:

- this is a real bug, not cosmetic noise
- it is a strong low-hanging-fruit candidate

#### 8. Add all MISP feeds

**Current state**:

- current repo already contains `24` `misp_*` feeds
- these are MISP warninglist-derived feeds from:
  - `https://github.com/MISP/misp-warninglists`
- evidence in current config:
  - `configs/firehol.yaml` around lines `3085` to `3443`

Implication:

- the codebase already has MISP parser coverage and real MISP feed handling
- but “all MISP feeds” still requires a gap audit against the official MISP
  default feed metadata, because the current set is only a subset/family

#### 9. Public methodology doc: how to add feeds / how to make a PR

**Current state**:

- there is no public contributor-facing methodology page for adding feeds
- current methodology pages are metric-centric only:
  - `pkg/web/static/methodology/*.md`

Implication:

- this is documentation work plus probably a dedicated public route/page entry
- it also overlaps with item `13` operator docs and item `2` config reorg

#### 10. Automatic optimal update frequency

**Current state**:

- current specs already say:
  - automatic cadence selection belongs to the downloader loop
  - `specs/design.md`
- current runtime already records observed cadence facts:
  - average update cadence
  - minimum update cadence
  - maximum update cadence
- these are already exposed in UI/admin:
  - `avg_update_mins`
  - related scheduler detail/state

Evidence:

- `specs/design.md`
- `specs/config.md`
- admin/home views already showing observed cadence metrics

Implication:

- this is not a blank-slate idea
- the raw observations already exist
- what is missing is:
  - control semantics
  - scheduler behavior
  - operator visibility/override
  - persistence contract

Related historical planning input already exists:

- `TODO-auto-refresh-cadence.md`

#### 11. Public health-transition log per feed

**Current state**:

- feed health classification exists
- public/admin UI expose the current health state
- I did **not** find a current per-feed health-transition history/log artifact

Evidence:

- health exists in:
  - `specs/feeds.md`
  - `pkg/feedhealth`
  - admin/public payloads
- no current public transition-log artifact or route was found in:
  - `pkg/engine`
  - `pkg/web`
  - `ui/src`

Implication:

- this looks like net-new storage + publication + UI work

#### 12. New upstream repo with flattened history + security audit

**Current state**:

- there is no current documented upstream-publication workflow for this repo
- current repo still contains Costa-specific local paths in docs/comments and
  likely in historical TODOs
- no dedicated release-audit checklist exists yet

Implication:

- this needs a dedicated release-prep stream
- it must include:
  - secret scan
  - hardcoded-path scan
  - environment/config audit
  - docs/branding cleanup
  - history rewrite/export plan

#### 13. End-user/operator docs in `docs/`, to be served as GitHub wiki

**Current state**:

- `docs/` currently contains only:
  - `docs/migration-from-bash.md`
- specs are detailed, but they are product contracts, not operator guides

Implication:

- operator docs are missing
- GitHub wiki serving has real constraints:
  - separate `.wiki.git`
  - soft file limit
  - weak search indexing
- likely correct model:
  - `docs/` in repo is the source of truth
  - sync/export to wiki if desired

#### 14. Homepage should present the `firehol_*` merged feeds and what they are good for; review merges because they are stale

**Current state**:

- I found no homepage-specific surface that highlights `firehol_*` feeds
- no current homepage implementation references `firehol_` directly
- homepage explorer is generic

Implication:

- this is both UX and data-quality work:
  - present the curated merges
  - review their current composition and freshness

#### 15. Review critical infrastructure ASNs

**Current state**:

- critical infrastructure ASNs are a manually curated config list:
  - `configs/firehol.yaml` -> `infrastructure_asns:`
- this list already drives:
  - ASN attribution/infrastructure tagging
  - public methodology pages
  - insight rules

Implication:

- Costa is correct that this must be reviewed carefully
- this is not just UI; it affects analytical truth

#### 16. Review all feed visualizations for better visualization choices

**Current state**:

- the project already has many visualization surfaces
- there are prior redesign/history TODOs:
  - `TODO-website.md`
  - `TODO-website-phase3-design.md`
  - `TODO-website-phase3-impl.md`
  - `TODO-UI-REWORK.md`

Implication:

- this needs a structured audit, not ad-hoc tweaks
- likely output:
  - visualization inventory
  - per-chart purpose
  - current risk/misread cases
  - candidate replacements

#### 17. Index pages for all ASNs and all Countries, added to the menu

**Current state**:

- detail routes already exist:
  - `/countries/:code`
  - `/asns/:asn`
- but there are no corresponding public index routes/pages in `App.tsx`
- current public header/footer menu also does not expose countries or ASNs
- only maintainers have a public index page today

Evidence:

- `ui/src/App.tsx`
- `ui/src/components/site-header.tsx`
- `ui/src/components/site-footer.tsx`

Implication:

- this is a clear missing navigation surface
- it pairs naturally with item `1` precomputed entity data

#### 18. `skills/` directory with AI skills for repeated operations

**Current state**:

- there is no project-local `skills/` directory today
- `find skills -maxdepth 2 -type f` returned nothing

Implication:

- this is net-new repo structure and documentation
- likely targets:
  - health-monitoring
  - add-feed
  - search-new-feed
  - audit-merge
  - release-audit

#### 19. Public streamable HTTP MCP server at `/mcp`

**Current state**:

- no current MCP server implementation was found in:
  - `cmd`
  - `pkg`
  - `specs`
  - `ui/src`

Implication:

- this is net-new server/API work
- must be designed deliberately against the official Streamable HTTP spec

#### 20. Public page to paste an `{ip,net}set` and tell overlaps, plus APIs

**Current state**:

- current public APIs support:
  - `/api/v1/query`
  - `/api/v1/compose`
- but there is no obvious current public page or API for:
  - uploading/pasting an arbitrary set body
  - comparing it against tracked feeds

Implication:

- this is net-new public API + UI surface
- likely heavy request-time work unless carefully designed

#### 21. Public page to compose an IP feed by merging and excluding feeds

**Current state**:

- backend compose API already exists:
  - `pkg/web/server.go`
  - `/api/v1/compose`
- but there is no current public page/route that exposes a real composer UI

Implication:

- user-visible feature is missing even though part of backend groundwork exists

#### 22. Use AI agents to enrich/evaluate feeds from time to time

**Current state**:

- no current built-in AI enrichment/evaluation pipeline exists in this repo
- this overlaps with:
  - `skills/`
  - operator docs
  - MCP
  - future feed quality reviews

Implication:

- this is a strategic extension, not a low-risk polish task
- it should come late in the release track, unless Costa wants a design-only
  exploration earlier

## Low-hanging fruits identified

These are the bounded, high-signal items that can be started first **after
Costa approval**:

1. Homepage hero copy update (`#4`)
2. Homepage IP-lookup copy update (`#5`)
3. Bogon section copy update (`#6`)
4. Homepage client-IP bootstrap (`#3`)
5. Feed-detail back/forward chart-container bug (`#7`)
6. Countries/ASNs public index routes + menu links (`#17`, first slice)

These were the recommended first implementation batch and are now complete.

## Old TODO cleanup plan

### Facts

- `TODO.md` is stale and describes an old architecture/UI history
- several TODOs contain still-useful design/research history
- a few are finished and now mainly historical

### Proposed classification

#### Keep as historical/reference inputs for now

- `TODO-website.md`
- `TODO-website-phase3-design.md`
- `TODO-website-phase3-impl.md`
- `TODO-UI-REWORK.md`
- `TODO-auto-refresh-cadence.md`
- `TODO-feed-audit-2026-04.md`

#### Superseded by this master TODO

- `TODO.md`

#### Candidate archive/delete after Costa approval

- `TODO-processing-interval.md`
- `TODO-misp-empty-history.md`

### Outcome

- Costa approved moving historical/superseded TODO files out of the repo root
- the historical TODO set now lives under `docs/todo-history/`
- the active execution tracker remains `TODO-release-master.md`

## Decisions pending Costa approval

### 0. Phase 2 entity artifact storage/read model

Context:

- Costa approved nested `web/` layout for public country/ASN artifacts
- the current public server serves any `.json` path under `web/`
- internal sidecars placed under `web/` would therefore become publicly
  reachable unless we add extra routing/filtering rules
- the remaining choice is where reusable composition sidecars live and whether
  API routes read one final payload or merge fragments on request

### 0.A. Phase 2 targeted invalidation backbone

Context:

- Costa approved targeted split rebuilds instead of dummy full recomputation
- the codebase currently has no persisted feed -> country/ASN reverse index and
  no private entity-cache tree
- health-transition rebuilds therefore need an explicit design for discovering
  which countries/ASNs are affected by one feed without rescanning everything

### 1. Primary config decomposition model

Context:

- directory loading already exists
- the missing choice is the primary authored structure

Options to evaluate with Costa:

- keep one top-level repo config plus per-feed fragments
- move everything feed-related to one file per feed plus separate shared files
  for categories/infrastructure/runtime
- generate the monolithic file from fragments for compatibility only

### 2. Docs source-of-truth vs wiki publication model

Context:

- GitHub wiki is a separate `.wiki.git` repo and has soft limits
- `docs/` is the natural source-of-truth location in the main repo

Options to evaluate with Costa:

- `docs/` as source of truth, wiki as generated mirror
- wiki as source of truth, `docs/` generated from it
- `docs/` + GitHub Pages instead of wiki

### 3. Client-IP auto-lookup privacy/UX behavior

Context:

- Costa wants auto-selection when empty
- this requires a public “what IP do we think you are?” source

Open points:

- autofill only, or auto-submit too
- how to behave when the client IP is private/proxied/unknown
- whether to expose a visible “detected from your connection” hint

### 4. Adaptive cadence control model

Context:

- observations already exist
- what is missing is the policy

Open points:

- advisory-only
- runtime auto-override without editing catalog files
- persistent learned schedule written back somewhere
- per-feed opt-out semantics

### 5. Public MCP scope

Context:

- official MCP transport/security model is non-trivial
- `/mcp` could expose many different things

Open points:

- read-only public tools/resources only
- whether composition/query tools belong there
- authentication/rate-limiting model
- operator/admin exclusion rules

### 6. Public paste-overlap / composer limits

Context:

- arbitrary pasted set bodies can become expensive

Open points:

- max input size
- accepted formats
- synchronous vs queued execution
- rate limits
- whether results are ephemeral only or bookmarkable

### 7. Old TODO cleanup policy

Context:

- user asked to clean old TODOs up
- several are still valuable history

Open points:

- delete superseded files
- move historical ones under `docs/todo-history/`
- keep everything in root until release

## Decisions made

### Approved by Costa on 2026-04-24

- **1.A approved**:
  - historical/superseded TODO material should move out of the repo root
  - preferred direction: keep active/master TODOs at the root, move historical
    TODOs under `docs/todo-history/`
- **Public-serving work focus approved (`1.A`)**:
  - the next implementation stream should prioritize precomputing country and
    ASN public data
  - indexes and detail pages should move together in the same pass
  - the existing `/api/v1/countries*` and `/api/v1/asns*` routes should remain
    stable and become thin artifact readers
  - entity artifacts should use a nested publication layout rather than adding
    more flat root files under `web/`
  - rebuild policy should use a targeted split model:
    - expensive composition work must not rerun blindly on pure health wakes
    - pure health transitions should refresh only the health-sensitive payloads
      for affected entities
    - expensive ASN/country intersection work should rerun only when feed or
      provider content changes invalidate the underlying composition
  - public final entity payloads should live under `web/`, while reusable
    internal sidecars should live under a private `lib/` entity cache area
  - targeted refresh should use a private per-feed membership index so the
    engine can diff old/new country and ASN participation without rescanning
    the whole entity surface on each change
- **First implementation batch approved**:
  - start with the low-hanging-fruit batch identified above
  - implementation should avoid mechanical churn and should prefer thoughtful
    reuse of existing patterns over inventing new surfaces
  - for the items in this batch that still had smaller UX questions, use the
    already recommended least-surprising behavior unless Costa overrides it
    later:
    - client-IP bootstrap: autofill only when empty; do not auto-submit
    - public entity indexes: mirror the existing maintainer-index pattern first
- **Executed on 2026-04-24**:
  - moved historical TODO files into `docs/todo-history/`
  - updated handbook/code references to the new historical TODO paths
  - implemented the first approved low-hanging-fruit batch
  - refined the bogons-section public copy to
    `Private and Unassigned IP Address Space`

## Plan

### Phase 0. Approval and control-point setup

Status: complete.

1. Get Costa approval on:
   - old TODO cleanup policy
   - first low-hanging-fruit batch
   - config/docs/MCP high-level direction
2. Keep this file as the master execution tracker.

### Phase 1. Low-hanging-fruit batch

Status: complete.

1. Fix homepage/feed copy updates (`#4`, `#5`, `#6`)
2. Implement client-IP bootstrap (`#3`)
3. Fix history-navigation chart sizing/performance regression (`#7`)
4. Add country/ASN index navigation entry points (`#17`, first slice)

### Phase 2. Public-serving cost reduction

Status: in progress.

Completed on 2026-04-24:

- added recursive staged publish support for nested `web/` trees
- introduced private `lib/entities/...` sidecars for:
  - per-feed entity membership
  - country composition
  - ASN composition
- wired entity artifact generation into `RunOnce()` so content/provider-driven
  runs publish:
  - `web/countries/index.json`
  - `web/countries/{code}.json`
  - `web/asns/index.json`
  - `web/asns/{asn}.json`
- converted `/api/v1/countries*` and `/api/v1/asns*` into file-backed public
  artifact readers
- added scheduler health-transition detection and targeted entity-detail
  refreshes
- added startup/reload background rebuild hooks so entity artifacts are
  refreshed without blocking basic service availability

Remaining:

1. Extend the public web publish batch to support nested output trees under
   `web/` while preserving atomic publish semantics.
2. Introduce a private entity-cache area under `lib/` for reusable sidecars:
   - per-feed membership index:
     - `lib/entities/feeds/{feed}.json`
   - country composition sidecars
   - ASN composition sidecars
3. Publish nested public entity payloads under `web/`:
   - `web/countries/index.json`
   - `web/countries/{code}.json`
   - `web/asns/index.json`
   - `web/asns/{asn}.json`
4. Split entity rebuilds into:
   - content/provider-driven composition refresh
   - health-transition-only final payload refresh for affected entities
5. Keep `/api/v1/countries*` and `/api/v1/asns*` stable, but convert them from
   request-time aggregators into cache-first artifact readers.
6. Reuse existing per-feed country/ASN public artifacts as source inputs where
   possible, and reserve latest-set/database intersection work only for the
   sections that genuinely need it:
   - country detail `top_asns_in_country`
   - ASN detail `top_countries`
   - ASN detail `country_distribution`
7. Define full-rebuild triggers explicitly:
   - feed content changes
   - geo provider changes
   - ASN provider changes
   - config changes affecting entity eligibility/visibility
8. Define targeted health-only refresh triggers explicitly:
   - feeds whose computed health class changed at a scheduler wake
   - rebuild only the affected entity final payloads, not expensive composition
     sidecars or indexes

Open follow-up checks in this phase:

- confirm the background startup/reload rebuild behavior is acceptable on the
  live workstation and does not cause unacceptable warm-up lag for entity pages
- add broader end-to-end route tests for precomputed country/ASN APIs if the
  current engine- and scheduler-level coverage proves insufficient

Discovered on 2026-04-24 during live validation:

- the current full-bootstrap implementation is artifact-first in code, but the
  installed daemon is still serving country/ASN routes through the temporary
  live fallback path during warm-up
- concrete live evidence:
  - `GET /api/v1/asns/16276` took about `1.26s`
  - the response carried `Cache-Control: no-store`, proving it came from the
    fallback builder path instead of the static file cache
  - the final published files under `web/asns/` and `web/countries/` were not
    yet present
  - several staged nested trees existed under:
    - `web/.update-ipsets-web-*`
    - `lib/entities/.update-ipsets-entities-*`
- code-level evidence suggests the full bootstrap algorithm is too expensive:
  - `buildCountryDetailSidecar()` currently scans the full public feed set for
    each country and recomputes country-filtered ASN counting from latest sets
  - `buildASNDetailSidecar()` currently scans the full public feed set for each
    ASN and recomputes ASN-specific country counting from latest sets
- the staged live batches already show hundreds of country/ASN detail files per
  attempt, which is evidence of large bootstrap fan-out rather than a cheap
  incremental refresh

Implication:

- there is now a pending implementation-design decision for Phase 2:
  - keep the current per-entity bootstrap shape and only harden/serialize it
  - or redesign the expensive part around per-feed joint geo+ASN sidecars so
    country/ASN entity pages aggregate from one expensive per-feed computation
    instead of recomputing the same intersections entity-by-entity

Approved by Costa on 2026-04-24:

- choose the per-feed joint geo+ASN sidecar redesign
- replace the current expensive per-entity bootstrap path rather than trying
  to harden the wrong algorithm
- the new Phase 2 target is:
  - expensive geo+ASN intersection work happens once per feed
  - country/ASN entity pages aggregate from those per-feed joint sidecars
  - final public routes stay artifact-first, with the existing live fallback
    kept only as warm-up/recovery protection

Implementation direction locked before code changes:

- keep the current country/ASN index builders and the current live fallback
  builders; they are not the dominant bootstrap cost
- add one new private per-feed joint geo+ASN sidecar family under
  `lib/entities/`
- use those per-feed joint sidecars only for precomputed country/ASN detail
  artifact generation
- keep health-transition refreshes cheap by rematerializing public detail
  payloads from the existing private entity sidecars, without recomputing the
  per-feed joint intersections
- for feed/provider updates, recompute joint sidecars only for affected feeds
  and then rebuild only the affected country/ASN detail artifacts from those
  sidecars

Verified live on 2026-04-24 after the redesign:

- the private per-feed joint sidecars now materialize under
  `lib/entities/feed-joint/`
- the final published country/ASN trees now materialize under:
  - `web/countries/`
  - `web/asns/`
- `GET /api/v1/asns/16276` switched from the live fallback path
  (`Cache-Control: no-store`, about `1.3-1.5s`) to the file-cache serving path
  (`Cache-Control: public, max-age=300`, `ETag`, `Last-Modified`, about
  `0.0009s`)

Approved by Costa on 2026-04-24:

- any background work the daemon performs SHOULD be visible in the admin UI
- the system SHOULD avoid doing invisible background work
- implement this immediately for the current background entity-artifact rebuild
  path
- write the broader rule into the specs

Implemented on 2026-04-24:

- the engine now tracks operator-visible background tasks in its status snapshot
- the startup/reload/full entity-artifact rebuild path and the health-transition
  entity refresh path now register background tasks with:
  - name
  - trigger
  - current stage
  - detail text
  - started/updated times
  - progress counters when meaningful
- the admin status API now exposes these under
  `engine.background_tasks`
- the admin UI now renders a dedicated `Background work` block instead of
  leaving this daemon work invisible

Verified live on 2026-04-24:

- `GET /api/v1/admin/status` returned:
  - `engine.background_tasks[0].name = "Entity artifacts rebuild"`
  - `trigger = "startup"`
  - `stage = "aggregating entity details"`
  - progress counters for the running rebuild

### Phase 3. Config/catalog and contributor workflow

1. Design fragment-based primary config layout
2. Implement file-per-feed catalog loading
3. Add contributor docs:
   - how to add a feed
   - how to review a feed
   - how to make a PR
4. Audit all current MISP-default-feed gaps and add approved missing entries

### Phase 4. Cadence and health observability

1. Design adaptive cadence algorithm and controls
2. Implement/publicize health transition history
3. Expose both in admin and public methodology/docs

### Phase 5. Data-quality review stream

1. Review `firehol_*` merges and present them on homepage
2. Review `infrastructure_asns`
3. Audit current visualizations and propose better encodings where needed

### Phase 6. Release documentation and upstreamization

1. Build operator docs in `docs/`
2. Add wiki publication/export workflow
3. Run upstream-publication security audit
4. Prepare flattened-history upstream repository creation plan

### Phase 7. New public/API/AI surfaces

1. Public compose page
2. Public paste-overlap page
3. Public MCP server
4. AI-agent enrichment/evaluation design and pilot

## Implied decisions

- precomputed public entity pages should become publication artifacts, not
  request-time live aggregation
- public entity payloads should be fully materialized JSON files, not
  request-time merges of internal fragments
- nested `web/` entity artifacts imply recursive staged publish support in the
  existing web batch rather than a second publication subsystem
- internal entity sidecars must not live under `web/`, because `web/` is a
  public file surface by contract
- targeted entity refresh requires persisted feed membership sidecars so the
  engine can diff old/new country and ASN participation safely
- `docs/` should be treated as authored documentation, even if a wiki mirror is
  later added
- arbitrary heavy public surfaces must be rate-limited and bounded
- operator-facing docs and contributor-facing docs are separate deliverables
- release-prep/security-audit work is mandatory before upstream publication

## Testing requirements

- every public-surface move from live computation to precomputed artifacts needs:
  - artifact-generation tests
  - nested publish-tree tests
  - invalidation-diff tests for feed membership sidecars
  - health-transition targeted-refresh tests
  - route-serving tests
  - cache/header tests where relevant
- config fragmentation work needs:
  - merge-order tests
  - duplicate/conflict validation tests
  - migration compatibility tests
- adaptive cadence work needs:
  - synthetic-history scheduler tests
  - operator override tests
  - stability/no-oscillation tests
- public new compute-heavy surfaces need:
  - request-size bound tests
  - rate-limit tests
  - latency and cancellation behavior checks
- upstream release prep needs:
  - secret scan
  - hardcoded-path scan
  - repo-history audit checklist

## Documentation updates required

Likely affected files/specs once implementation starts:

- `specs/config.md`
- `specs/design.md`
- `specs/downloader.md`
- `specs/pipeline.md`
- `specs/feeds.md`
- `specs/files-layout.md`
- `specs/operating-principles.md`
- `specs/website.md`
- `specs/homepage.md`
- `specs/admin-ui.md`
- `README.md`
- `docs/` operator documentation set
- `pkg/web/static/methodology/*.md` for new public metrics/behaviors

## Active approved work

### Newly requested, approved for implementation

- Background entity/artifact work must use an explicit worker limit dedicated to background work.
- That background-worker limit must control CPU and memory pressure; default must be `1`.
- Rationale from Costa: background work is non-critical because runtime routes already have fallbacks, so correctness/availability should be preserved while allowing slower completion.
- Investigate current startup/background entity rebuild path to determine where concurrency is currently uncontrolled or implicit.
- Investigate admin UI wheel/scroll traps; multiple admin tiles/lists are currently blocking normal page scroll while hovered, even when they should not.
- Verified facts:
  - startup currently launches a full entity rebuild unconditionally via `pkg/web/server.go`
  - that full rebuild bypasses the existing `EnsureEntityArtifactsCurrent()` guard and calls `writeEntityArtifacts(..., full=true, ...)`
  - scheduler already detects health transitions and calls targeted entity refreshes
  - scheduler cold-start currently treats an empty previous snapshot as "all feed health changed", so the first fetch-loop tick would trigger a broad entity refresh even if the explicit startup rebuild were removed
  - runtime config already exposes `max_processing_workers` and `max_heavy_phase_workers`, but there is no dedicated background-work worker limit today
  - admin queue panels still use `overscroll-contain`, and the main feeds table still uses `overflow-auto`, both of which are likely scroll traps
- Costa-approved direction:
  - add a dedicated runtime background-worker limit with default `1`
  - startup should stop doing unconditional broad entity rebuilds and should use a guarded bootstrap path instead
  - admin UI needs a shared scrolling rule; remove wheel-trap scroll containers
  - entity artifact integrity should also verify freshness against local inputs so stale/broken country/ASN artifacts are repaired automatically
- Costa suggestion to integrate:
  - use integrity to verify country/ASN artifacts against provider freshness, specifically newer-than provider data dates, so startup background rebuilds happen only when needed or when the last update left stale outputs behind
- Important analysis note:
  - provider freshness alone is not sufficient for entity artifacts; feed-content changes and health-transition rewrites can require entity refresh even when provider DB dates did not change
  - the correct integrity model is hybrid local-input integrity, not provider-only freshness:
    - feed membership sidecars depend on the per-feed geo/asn public inputs they read
    - feed-joint sidecars depend on the feed's latest set plus the local geo/asn provider datasets
    - public country/ASN payloads depend on their private sidecars and also embed health classes at materialization time
- Costa-approved implementation decisions:
  - add `runtime.max_background_workers` with default `1`
  - route background entity work through a dedicated limiter instead of leaving it uncapped
  - use guarded startup entity bootstrap instead of unconditional full startup rebuild
  - prevent the scheduler's first cold-start snapshot from causing a fake all-feeds health-transition refresh
  - add entity-artifact integrity as a separate operator-facing integrity family in the admin surface, not as synthetic feed findings
  - use hybrid local-input entity-artifact integrity so startup repair only rebuilds stale or broken pieces
- Implemented on 2026-04-24:
  - added `runtime.max_background_workers` with default `1`
  - routed entity-artifact background jobs through a dedicated background limiter
  - changed startup/reload from unconditional full entity rebuilds to guarded entity-integrity repair
  - prevented scheduler cold-start from treating an empty previous health snapshot as "all feeds changed"
  - added separate admin entity-integrity API/UI surface for country/ASN artifacts
  - changed entity health-drift integrity from file-mtime guesses to semantic comparison of the rendered public payload health
  - removed the known admin wheel-trap containers in the queue panels and feeds table
- New release-gate analysis requested by Costa on 2026-04-24:
  - the current country/ASN artifact maintenance appears to change the operational profile of the daemon too much
  - Costa observed:
    - only a few feeds updated
    - the daemon still spent minutes in post-processing/background entity work
    - multi-core CPU use remained high
  - this must be investigated before release and then repeated again before production release
- Costa-approved operating principles to encode in specs and use for the redesign:
  - DO NOT WASTE RESOURCES FOR DOING THINGS THAT ARE NOT ABSOLUTELY NEEDED.
  - EVENTUAL CONSISTENCY IS GOOD ENOUGH ACROSS ACTORS.
  - consistency must remain synchronized within the same actor:
    - per-feed outputs must remain self-consistent with their own derived artifacts
    - per-country outputs must remain self-consistent with their own derived artifacts
    - per-ASN outputs must remain self-consistent with their own derived artifacts
  - consistency may be eventual across actors:
    - a changed feed does not require every affected country/ASN artifact to be refreshed inline before the run can be considered complete
- Mandatory analysis scope before redesign:
  - find exactly where resource waste happens today
  - identify where concurrency is not respecting the intended caps
  - identify where work is repeated unnecessarily
  - evaluate whether country/ASN artifact maintenance should become:
    - a separate queue with its own operator-visible schedule/state
    - or capped background work with eventual consistency
  - ensure the admin UI/API continues to surface all such backend work accurately
- Costa clarification on 2026-04-24:
  - country/ASN refresh targets for ordinary feed updates should come from the
    union of old and new membership sidecars:
    - old countries + new countries
    - old ASNs + new ASNs
  - contribution-count diffs are not required for deciding which country/ASN
    actors need refresh
  - contribution counts still remain part of the rebuilt country/ASN payload
    contents, but they are not the targeting mechanism
  - old feed bodies must not be re-compared just to discover old country/ASN
    memberships; the persisted old sidecars are the source of truth for that
  - surgical country/ASN artifact updates must update the full artifact, not
    only splice the feed row:
    - country artifacts must refresh feeds, totals, top categories, top
      maintainers, top ASNs, and the public materialized payload
    - ASN artifacts must refresh feeds, totals, top categories, top maintainers,
      top countries, country distribution, and the public materialized payload
    - any helper that removes an old feed contribution and adds the new one must
      leave every derived aggregate equivalent to a clean rebuild of that
      country/ASN actor
- Implementation started on 2026-04-24:
  - specs updated to require bounded surgical entity deltas, actor-scoped
    consistency, and operator-visible background entity refresh work
  - implementation direction:
    - normal scheduler processing batches enqueue background entity refresh after
      feed publication instead of running entity aggregation inside `RunOnce`
    - background entity refresh computes changed feed sidecars once and
      surgically patches affected country/ASN artifacts plus indexes
    - `RunOnce` no longer charges country/ASN artifact time to the `insights`
      phase
  - implemented and locally verified:
    - added surgical per-feed country/ASN artifact refresh path
    - added corrupted-sidecar fallback to full rebuild when old contributions
      cannot be safely subtracted
    - added regression coverage for feeds, totals, top categories,
      maintainers, top ASNs, top countries, country distribution, and indexes
    - verified that the live `insights` phase no longer includes entity
      artifact work after install
  - Costa observations after install:
    - background work effectively behaves like a queue
    - queued entity refresh jobs start one after another without deduplication
    - each item says it is surgical, but practical runtime still looks similar
      to full comparison work
  - Costa follow-up observation:
    - the engine `insights` phase still appears to run all ASN/country work even
      when entity refreshes are queued, so similar work seems to happen multiple
      times
  - Verified facts from code review:
    - `pkg/engine/insights.go` currently ignores the `updatedNames` argument in
      `writeInsightsForFeeds()` and sweeps every public feed on every run
    - that insights sweep reads existing geo/ASN/bogon/comparison JSON files;
      it does not itself run the heavy geo/ASN comparison algorithms
    - `pkg/engine/run.go` still loads every configured geo/ASN provider during
      the normal heavy block whenever any feed updated
    - `writeCountryComparisonFiles()` and `writeASNComparisonFiles()` scope the
      actual comparisons via `targetFeedsForFanOut()`:
      - normal feed update: changed public feeds only
      - provider update: every public feed, because provider-derived facts
        changed globally
    - the new background entity refresh performs a second per-changed-feed joint
      country/ASN pass for the preferred providers, which duplicates part of
      the already generated per-feed facts and is the expensive part currently
      hidden behind "surgical" wording
  - Follow-up requirement:
    - entity background refresh must coalesce/deduplicate feed targets before
      running the next refresh
    - admin wording must distinguish "computing changed feed sidecars" from
      "patching affected entity artifacts" so the expensive part is visible
    - repeated pending refreshes for the same feed must not repeat work
    - insights must stop sweeping all feeds by default; it should regenerate
      changed/provider-affected feeds plus missing insight files only
  - Follow-up implementation started on 2026-04-24:
    - added an in-memory coalescing layer for ordinary feed-update entity
      refresh requests before background tasks are created
    - changed the scheduler to enqueue entity refresh work through that
      coalescer instead of spawning one background task per processing batch
    - live admin status after install exposed the same queue problem for
      health-transition refreshes, so health-transition entity refreshes were
      routed through a matching feed-name coalescer too
    - live admin status also exposed that `install.sh` refreshed
      `/opt/update-ipsets/etc/config.yaml` mtime even when the config content was
      identical; entity integrity interpreted this as `config_newer` and queued
      a full startup entity rebuild
    - changed `install.sh` to copy the active config only when content differs,
      preserving mtime on no-op reinstalls
    - changed admin/background task wording so the expensive changed-feed
      sidecar computation is separate from affected-entity patching
    - changed insights regeneration to target the same affected/provider fan-out
      as comparison files, plus feeds still missing an insights artifact
  - Costa escalation on 2026-04-24:
    - live `patching entity details` remains extremely slow and consumes 2-3
      CPU cores for minutes
    - this indicates the phase still performs real computational work, not just
      bounded JSON/CSV patching
    - requirement: perform a full code-path audit of geo/ASN entity artifact
      updates caused by feed updates, down to files opened/read/saved/closed,
      range comparisons, sidecar loading, sidecar patching, and public JSON
      publication
    - requirement: prove the path is not wasting work; where it is wasting
      work, fix it
    - requirement: after the internal audit, run external read-only reviews with
      Claude, Codex, GLM, Kimi, Qwen, and MiniMax using this TODO file as
      context, asking specifically for smelly code, wasted CPU/memory/I/O,
      unwanted side effects, and security issues
    - requirement: validate external findings before patching; do not cargo-cult
      reviewer suggestions
  - Internal audit findings so far:
    - confirmed: `patchASNSidecarForFeedDeltas()` calls
      `asnCountryRows(delta.oldJoint/newJoint, asn)` for each affected ASN
    - confirmed: `asnCountryRows()` scans all countries and ASN rows in the
      feed-joint sidecar every time; this is repeated work during the phase
      named `patching entity details`
    - measured local sidecar scale:
      - `firehol_level4` feed-joint sidecar: 185 countries, 9,289 country/ASN
        rows
      - `firehol_anonymous` feed-joint sidecar: 208 countries, 19,159
        country/ASN rows
    - implication: refreshing thousands of ASNs for a broad feed can perform
      tens or hundreds of millions of in-memory row checks before writing JSON
    - confirmed related issue: `countryJointRows()` also scans countries for
      each affected country, but the country count is small; fixing both via a
      pre-indexed sidecar is straightforward and safer
  - Internal fix applied:
    - added one-time `feedEntityJointSidecar` indexing per changed feed
    - replaced repeated `asnCountryRows()` / `countryJointRows()` scans with
      map lookups during country/ASN sidecar patching
    - removed the repeated-scan helpers
    - added regression coverage for the sidecar index lookup behavior
  - External review findings validated so far:
    - Qwen confirmed the repeated ASN/country joint-sidecar scan and noted a
      smaller repeated `buildInfrastructureMap()` cost in the patch loop
    - GLM identified the larger remaining problem: ordinary background
      feed-update entity refresh still built the new country/ASN joint sidecar
      by reopening the changed feed and re-running geo+ASN range attribution
    - Codex confirmed the repeated scan and flagged a correctness risk if
      entity writers are allowed to publish concurrently when background worker
      count is greater than one
    - Claude identified an I/O amplification issue: entity JSON writes went
      through `writeFileAtomic()`, which fsyncs every temporary file; this is
      unnecessary for regenerated staged entity/web batch files and can dominate
      wall time when thousands of entity files are touched
  - Second internal fix applied:
    - the normal processing run now has a dedicated `entities` phase that
      precomputes the changed feed's new country/ASN joint sidecar while geo
      and ASN providers are already open
    - the precomputed sidecar is written as
      `lib/entities/feed-joint-pending/{feed}.json` so the committed old
      `feed-joint/{feed}.json` remains available for safe subtraction
    - provider-database-only runs now report explicit entity refresh targets,
      because provider sources are not normal feed updates and `report.Updated`
      is intentionally empty for them
    - the background feed-update entity refresh now loads the pending new
      sidecar, loads the committed old sidecar, patches affected country/ASN
      artifacts, promotes the pending sidecar to committed, and deletes the
      pending file
    - ordinary background patching no longer opens latest feed sets and no
      longer runs country<->ASN range attribution
    - background entity artifact writers now serialize publication with an
      entity-artifact mutation lock, independent of the configured background
      worker count
    - entity JSON staging now uses a no-fsync atomic write helper; durable
      committed feed bodies and other non-staged outputs still use the original
      fsyncing atomic writer
    - aggregate underflow during surgical subtraction now maps to the existing
      full-rebuild fallback instead of leaving the refresh queue stuck on a
      repeatable error
    - the infrastructure ASN lookup map is built once for a surgical refresh
      and reused by affected country/ASN patching
    - specs updated to document pending sidecars, the `entities` run phase, and
      the rule that background entity patching must not recompute range
      intersections
  - External review validation completed:
    - Kimi identified a valid provider-only edge case: when only a geo/ASN
      provider source is reprocessed, `report.Updated` remains empty by design,
      so the scheduler previously had no feed names to queue for entity
      refresh even though pending sidecars had been staged; fixed by adding
      `Report.EntityRefreshTargets` and making the scheduler prefer it over
      `report.Updated`
    - MiniMax re-raised the staged-JSON fsync amplification issue; this had
      already been fixed with `WriteAtomicNoSync()` for regenerated staged
      entity/web JSON files
    - MiniMax suggested using background-worker concurrency for
      `stageFeedEntityJointSidecarsFromLoadedProviders()`; rejected for the
      normal run path because this function is part of the foreground heavy
      processing block and therefore correctly follows `HeavyPhaseWorkers()`;
      the true background rebuild path already uses `BackgroundWorkers()`
    - MiniMax suggested `countCountryASNJointSource()` may be IP-by-IP; rejected
      after code review because the cursor advances by the returned ASN network
      boundary (`end := max(cur, min(network.Hi, hi))`), not by one IP
    - remaining intentional cost: if a broad feed changes, every country/ASN
      actor that had or now has that feed must be loaded, patched, sorted,
      marshaled, and republished; this is required for actor-local consistency
      and does not compare IP ranges or scan unrelated feeds
    - remaining exceptional cost: if the committed per-feed membership sidecar
      is missing but old country/ASN artifacts still reference that feed, the
      code scans entity sidecars to detect corruption and falls back to a full
      rebuild; this is a repair path, not the ordinary update path
  - Costa follow-up on 2026-04-24:
    - performance improved, but CPU and memory consumption remain higher than
      expected
    - repeat the external static analysis with Claude, Codex, GLM, Kimi, Qwen,
      and MiniMax against the current working tree
    - if static analysis does not fully explain the remaining resource use,
      expand the existing metrics/telemetry model professionally
    - telemetry must measure operations that materially affect CPU, memory,
      network, and I/O, so the daemon can run normally, snapshots can be
      compared over elapsed time and CPU consumption, and the fastest-moving
      counters can identify waste
    - expected outcome: either validated code fixes from static review, or
      enough operation-level counters to prove where CPU, memory, network, and
      I/O are being spent
  - Static review findings validated during the repeated review:
    - current metrics are timing-first, not operation/byte-first:
      - `RunMetricsSnapshot` exposes phase timing, operation timing, and slow
        feeds, but not file-open counts, read/write bytes, JSON encode/decode
        bytes, range-iteration counts, download bytes, mmap bytes, RSS deltas,
        or queue delta counters
      - scheduler metrics are daemon-lifetime, but mostly cover queue event
        counts and timing, not CPU/memory/network/I/O causes
    - live admin metrics from the installed daemon showed the last completed
      run dominated by metadata comparison work, not entity patching:
      - `metadata.comparison_pair_overlap`: 558 overlap operations,
        6407 ms aggregate worker time
      - `metadata.write_comparison_files`: 1562 ms wall time
      - `metadata.comparison_merge_rows`: 331 files/rows groups merged,
        580 ms aggregate time
      - `entities` phase: 33 ms in that run
      - live process sample over 10 seconds showed about 1.53 CPU seconds
        (~15% of one core), while `%CPU` from `ps` remained a lifetime average
    - validated foreground cost:
      - `stageFeedEntityJointSidecarsFromLoadedProviders()` still runs in the
        foreground `entities` phase and uses `HeavyPhaseWorkers()`, not
        `BackgroundWorkers()`
      - this is the precomputation of new pending per-feed country/ASN joint
        sidecars; it is not the later surgical entity patch
      - broad feeds can still spend real CPU here because the feed's latest set
        is walked against the preferred geo and ASN providers
    - validated repeated JSON read amplification:
      - `buildFeedEntityMembership()` reads/unmarshals the per-feed country and
        ASN public JSON files to derive the new membership set
      - `buildFeedEntityDelta()` reads/unmarshals the same per-feed country and
        ASN public JSON files again to build the feed rows used by the delta
      - this affects ordinary background entity refreshes and is wasteful
    - validated provider cache waste:
      - `geoProviderCache.LoadOrParse()` computes a SHA-256 hash of the full
        local provider source before checking the in-memory cache
      - cache hits therefore still read/hash the whole provider file
      - ASN lookup cache already uses the cheaper size+mtime key
    - validated repair-path cost:
      - `entityArtifactsContainFeed()` scans and unmarshals all country and ASN
        private sidecars if the committed per-feed membership sidecar is
        missing
      - this is a corruption/recovery path, not the ordinary update path, but
        it needs telemetry and preferably a cheaper reverse-index fallback
    - validated visibility/correctness gap:
      - queued feed-update and health-transition entity refreshes call the
        lower-level refresh functions directly; they should be audited to
        ensure every entity artifact mutation path consistently holds the
        entity-artifact mutation lock and is visible in admin background work
    - rejected or downgraded reviewer claims:
      - repeated loading of the same country/ASN sidecar inside one surgical
        patch batch was not proven; affected country/ASN sets are unique and
        each affected actor is loaded once per batch
      - `countCountryASNJointSource()` is not IP-by-IP; it advances by geo
        segment and ASN network boundaries
      - repeated `buildInfrastructureMap()` is real in fallback builders, but
        small compared with range walking, JSON I/O, and pairwise comparisons
    - additional Qwen review findings validated and kept as follow-up evidence:
      - `HomeSummary()` and `HomeGlobe()` still build request-time aggregate
        payloads by walking the current feed inventory and loading per-feed
        country/ASN JSON outputs
      - `CompareSet()` still opens each candidate latest set during the public
        comparison request; this is request-time I/O and mmap/file-open work
      - maintainer index/detail endpoints still compute live feed groupings per
        request; this is cheaper than entity detail pages but should still be
        precomputed before release
      - `entityArtifactsContainFeed()` remains an O(country+ASN sidecar scan)
        corruption-detection fallback when the per-feed membership sidecar is
        missing; it should get counters now and a reverse-index repair path
        later
    - additional Qwen review claims downgraded after code review:
      - `countCountryASNJointSource()` is expensive for broad feeds, but it is
        not a literal per-IP loop; it advances by provider/network boundaries
      - `fileCache` `os.Stat()` per cached public request is measurable but
        currently acceptable compared with live aggregation and range walks
  - Immediate implementation direction from this review:
    - add daemon-lifetime operation/byte counters suitable for snapshot deltas,
      starting with engine, scheduler/background, downloader, file/JSON, and
      heavy comparison/entity paths
    - split comparison skip counters by reason instead of one collapsed
      `metadata.comparison_pair_skipped` bucket
    - add counters for entity membership/delta JSON reads, sidecar reads/writes,
      pending sidecar reads/writes, affected countries/ASNs, and foreground
      entity joint range-walk work
    - add counters for download requests/bytes/statuses and atomic write
      bytes/renames/fsync/no-fsync writes
    - make admin status expose cumulative counters and process snapshots so
      operators can compare deltas against elapsed time and CPU consumption
    - add request-time counters for home summary/globe aggregate JSON reads and
      public `CompareSet()` latest-set opens, so remaining live public-surface
      cost is visible before the larger precomputed-home rewrite
    - fix the low-risk static waste found above:
      - avoid double-reading per-feed country/ASN JSON during entity delta
        construction
      - use cheap file size+mtime freshness checks before provider full-hash
        verification
  - Telemetry validation after local install on 2026-04-24:
    - first 10-second admin-status delta after the initial telemetry patch:
      - process CPU increased by ~36.8 CPU-seconds
      - process write bytes increased by ~58.7 MB
      - only `entity.refresh.asn_sidecar_read` moved materially
      - conclusion: the first counters were not deep enough around surgical
        sidecar patch/read/write work
    - second 10-second delta after adding sidecar read/write bytes:
      - process CPU increased by ~32.0 CPU-seconds
      - process write bytes increased by ~49.1 MB
      - top moving counters:
        - `entity.refresh.asn_public_write`: 1,135 writes, ~29.1 MB
        - `entity.refresh.asn_sidecar_read`: 1,135 reads, ~15.3 MB
        - `entity.refresh.asn_sidecar_write`: 1,135 writes, ~15.3 MB
      - active background task was patching 208 countries and 8,910 ASNs
      - conclusion: I/O is now visible, but actor-level patch/materialization
        CPU needs explicit timings because JSON read/write timers alone do not
        explain CPU consumption
    - third validation after adding actor-level timings:
      - active task changed to startup `Entity artifacts repair`
      - it was building 243 country pages and 44,196 ASN pages
      - top lifetime operations became:
        - `entity.refresh.asn_materialize`
        - `entity.refresh.country_materialize`
        - `metadata.comparison_pair_overlap`
      - top lifetime counters included:
        - `engine.latest_set.binary_open`
        - `entity.output_view.asn_json_read/decode`
        - `entity.refresh.asn_public_write`
      - conclusion: the biggest remaining confirmed waste is a startup
        repair/full-rebuild path that materializes tens of thousands of ASN
        pages and reopens/re-walks many feed sets; this must be fixed next by
        preventing unnecessary full startup repairs and by making full repair
        bounded/scheduled, not automatic broad rebuild work
  - Costa follow-up after telemetry validation:
    - fix the broad startup repair
    - also fix the ongoing post-repair problem
  - Current evidence:
    - after startup repair moved on, admin status showed a normal
      `scheduled_due` entity refresh patching 206 countries and 9,060 ASNs
    - this proves the remaining waste is not only startup repair; broad-feed
      ordinary refreshes can still rewrite thousands of entity actors
    - the refresh path currently rewrites each affected actor after computing a
      patched sidecar, even when the effective sidecar may be unchanged
  - Immediate fix direction:
    - ordinary surgical entity refresh should skip unchanged country/ASN actors
      after patching, avoiding private sidecar writes, public JSON writes,
      materialization, and index updates for actors whose final sidecar is
      semantically unchanged
    - telemetry must count skipped unchanged country/ASN actors
    - startup entity repair must not enqueue a full broad rebuild just because
      a non-critical freshness marker differs; the integrity plan should be
      narrowed or suppressed where committed artifacts are otherwise usable
  - Implemented follow-up fix:
    - queued startup entity-integrity repair plans are revalidated after the
      entity-writer lock is acquired; stale broad plans are skipped
    - automatic startup entity repair is deferred when the repair is not a full
      bootstrap and would touch more than 1,024 targets
    - ordinary surgical feed-update refresh compares patched country/ASN actor
      sidecars against committed sidecars and skips JSON rewrites when the
      actor is unchanged
    - unchanged actors receive metadata-only mtime touches on private and
      public files so later integrity checks do not see false stale artifacts
    - repair/rewrite paths use the same unchanged-actor suppression where they
      can compare rebuilt sidecars with committed sidecars
	  - Validation after install:
	    - after restart, admin status showed no broad startup entity background
	      task
	    - logs showed `country and ASN entity artifacts checked at startup`, not a
	      startup repair/rebuild
	    - 15-second idle snapshot delta after startup:
	      - process CPU increased by ~2.56 CPU-seconds
	      - process read bytes and write bytes were unchanged
	      - no lifetime operation/counter deltas moved materially
	      - no background tasks were active
	  - Passive metrics investigation on 2026-04-24 after OpenTelemetry/dependency
	    install:
	    - six admin-status snapshots were collected over five ~15-second
	      intervals from the installed daemon while no engine run and no entity
	      background task were active
	    - first four intervals were quiet:
	      - ~0.12-0.15 CPU-seconds per ~15 seconds
	      - no material disk bytes
	      - moving counters were admin polling and small downloader checks only
	    - fifth interval reproduced CPU use:
	      - elapsed: ~15.01 seconds
	      - process CPU increased by ~5.77 CPU-seconds
	      - process read bytes increased by ~2.15 MB
	      - process write bytes increased by ~4.12 MB
	      - scheduler counter delta:
	        - `scheduler.fetch_and_stage`: 30 operations, 5,920 ms total
	      - downloader status counter delta:
	        - `download.status.same`: 18
	        - `download.status.not_modified`: 6
	        - `download.status.disabled`: 5
	        - `download.status.downloaded`: 1
	      - no engine phase and no entity background task were active
	    - scheduler snapshot evidence at the same time showed many merge feeds
	      scheduled on the same 5-minute cadence, including `firehol_level1`,
	      `firehol_level2`, `firehol_level3`, `firehol_level4`,
	      `firehol_webclient`, `firehol_webserver`, `firehol_proxies`,
	      `firehol_anonymous`, `firehol_abusers_1d`,
	      `firehol_abusers_30d`, and `cleantalk_*`
	    - code evidence:
	      - `pkg/scheduler/scheduler.go` automatic due scheduling skips history
	        derivatives and artifact children but not merges
	      - `pkg/engine/download_stage.go` implements merge composition in
	        `fetchAndStageMerge()` by reading every latest parent feed body,
	        parsing a combined body into an IP set, rendering canonical output,
	        and comparing/staging the result
	      - `specs/config.md`, `specs/downloader.md`, and `specs/pipeline.md`
	        currently describe merges as owning an independent downloader cadence
	    - conclusion:
	      - the measured post-repair CPU is not idle background entity patching
	      - it is autonomous merge recomposition in the downloader queue
	      - this is real CPU work hidden behind `download.status.same`, because a
	        same merge result can still require full recomposition before the
	        daemon knows it is unchanged
	    - Pending design decision:
	      - option A: change merges from autonomous 5-minute wall-clock
	        recomposition to dependency-driven recomposition after input feed
	        bodies change, plus explicit operator `recheck`
	        - benefit: avoids repeated full merge parse/render/compare when no
	          parent changed
	        - implication: merge freshness follows changed inputs and operator
	          intent, not time passing
	        - risk: config-driven health eligibility changes (`archived` /
	          `unmaintained` exclusions) still need a trigger, most likely health
	          transitions queue affected merges once
	      - option B: keep current merge cadence and only add more detailed
	        counters
	        - benefit: smallest behavioral change
	        - implication: CPU spikes every merge cadence remain expected behavior
	        - risk: violates the bounded-work principle for unchanged inputs
	      - recommendation: A; time-only merge recomposition is mechanically simple
	        but wasteful, and the measured CPU spike proves it changes the daemon's
	        operational profile without producing new public output
	  - Costa clarification on 2026-04-26:
	    - Do not build a dependency-tracking scheduler or merge dependency graph.
	    - Any strategy that requires ordering resolution, circular dependency
	      handling, or broad dependency orchestration is over-engineering for this
	      project.
	    - Acceptable direction:
	      - when a dependency finishes processing successfully, mark dependent
	        merges dirty
	      - do this after the dependency's engine turn completed, not before
	      - dirty merges should run at the next engine/scheduler invocation
	      - no recursive same-turn recomposition, no graph solver, no circular
	        dependency resolution machinery
	    - Rationale:
	      - this preserves the simple existing pipeline shape
	      - it avoids blind time-only recomposition when no input changed
	      - it avoids introducing a complex dependency scheduler
	  - Costa final decision on 2026-04-26:
	    - Keep merges time-based.
	    - Do not implement dependency-dirty merge recomposition.
	    - Rationale:
	      - even simple dirty flags can create new edge cases around merge-on-merge
	        dependencies
	      - circular or malformed merge relationships must not risk perpetual
	        dirty/run loops that waste resources
	      - the existing time-based model is simpler and more predictable
	    - Follow-up allowed:
	      - keep improving merge telemetry and operator visibility
	      - tune merge frequencies in config when a merge is too expensive or too
	        frequent
	      - do not replace the merge scheduling model without explicit approval
	  - Costa final decision on 2026-04-26 for feed-detail provider fetching:
	    - Leave eager Geo and ASN provider payload fetching as-is.
	    - It is acceptable for the current provider counts and does not require
	      an optimization pass now.
	  - Follow-up idle CPU investigation on 2026-04-24:
	    - Costa reported the daemon still consumed about 20-30% CPU while the
	      engine and background queues appeared idle
    - a 20-second admin-status counter delta showed:
      - process CPU increased by ~4.36 CPU-seconds
      - process write bytes increased by ~1.19 MB
      - GC ran 104 times
      - no engine/downloader/entity lifetime operation or counter moved
    - perf PC resolution showed the hot request path was:
      - `/api/v1/admin/status`
      - `buildAdminStatus()`
      - `buildAdminFeedsWithStatus()`
      - `populateFromCacheAndSchedule()`
      - `eng.EntrySnapshot(name)`
      - `cache.State.SnapshotEntries()`
    - root cause:
      - `buildAdminFeedsWithStatus()` already builds one effective
        `entryIndex` from `eng.EntriesSnapshot()`
      - `populateFromCacheAndSchedule()` ignored that index and called
        `eng.EntrySnapshot(name)` for every feed
      - `eng.EntrySnapshot()` copies the full cache map for effective-entry
        resolution
      - therefore every admin status poll performed roughly
        `configured feeds * cache entries` map-copy work, producing avoidable
        allocation, GC, and CPU while the system was otherwise idle
    - Immediate fix direction:
      - make admin status use the already-computed effective entry index
      - add request-level timing/counters for admin status/admin feeds so the
        same issue is visible through telemetry without requiring perf
  - Costa telemetry correction on 2026-04-24:
    - The telemetry contract must be primitive-operation complete, not a set of
      sampled high-level breadcrumbs.
    - Required iprange counters:
      - `iprange.load.text`
      - `iprange.load.binary`
      - `iprange.save.text`
      - `iprange.save.binary`
      - `iprange.merge.ops`
      - `iprange.compare.ops`
      - `iprange.diff.ops`
      - `iprange.<operation>.ops` for every other supported iprange operation
      - `iprange.binary.searches`
    - Required subsystem counters:
      - `download.queued`
      - `download.<status>` for every downloader return status
      - `engine.queued`
      - `engine.<phase>` for every engine phase
      - full detail for all steps, phases, actions, file accesses, and saves
        across the application
    - Interpretation:
      - every material CPU, memory, network, and I/O operation must be
        countable from telemetry
      - low-level primitive counters are mandatory because high-level timers can
        hide the true cause of CPU burn, as happened with ipset comparisons
	  - Costa dependency hygiene requirement on 2026-04-24:
	    - Update all package dependencies used by the project to their latest
	      available versions.
    - Scope confirmed from repository manifests:
      - Go modules in `go.mod` and `go.sum`.
      - Frontend pnpm packages in `ui/package.json` and `ui/pnpm-lock.yaml`.
    - Verification:
      - `go list -m -u all` must not report pending direct/runtime updates that
        can be safely applied.
      - `pnpm outdated` must not report pending frontend package updates that
        can be safely applied.
      - If a latest version cannot be adopted due to breakage, document the
        concrete failure and keep the package pinned intentionally.
    - Implementation status:
      - Go direct/runtime dependencies were updated with `go get -u` and
        OpenTelemetry dependencies were added at current latest versions.
      - Frontend pnpm dependencies were updated with `pnpm update --latest`.
      - Tailwind CSS was migrated to the v4 PostCSS package and explicit
        `@config` loading because Tailwind v4 no longer auto-detects
        JavaScript/TypeScript config files.
	      - Remaining `go list -m -u all` entries after `go mod tidy` are upstream
	        module-graph declarations that `go mod why -m` reports as not needed by
	        the main module; forcing them would intentionally make `go.mod`
	        non-tidy.
	  - Costa follow-up on 2026-04-24:
	    - Make the installed `update-ipsets` service push OpenTelemetry metrics to
	      the local Netdata Agent every second.
	  - Costa correction on 2026-04-24:
	    - Send OpenTelemetry metrics every 10 seconds, matching Netdata's default
	      OTel chart interval and avoiding wasted localhost traffic.
	  - Costa design clarification on 2026-04-25 for geo/ASN refresh:
	    - The model must stay simple.
	    - During the engine geo/ASN phase, each updated feed is already compared
	      against all ASNs and countries.
	    - That phase must emit:
	      - whatever the feed itself needs
	      - the feed's new ASN list and country list
	      - the per-updated-feed values needed by the affected ASN and country
	        artifacts
	    - Before replacing the per-feed ASN/country lists, the engine must load
	      the previous per-feed lists so it knows the old ASN list and old
	      country list.
	    - After the engine finishes and publishes feed updates, it must push to
	      background work only:
	      - affected ASNs = old ASNs + new ASNs
	      - affected countries = old countries + new countries
	      - the new per-feed values needed to patch those actors
	    - The background worker must only:
	      1. load the artifacts of these old + new ASNs and countries
	      2. update them by removing the old value and adding the new value, as
	         applicable
	      3. save them
	    - The background worker must not redo expensive comparisons or rebuild
	      broad composition state that the engine already computed.
	  - Costa additional requirement on 2026-04-25:
	    - If the old and new per-feed values for a given country or ASN actor do
	      not change, that actor must not be loaded and saved.
	    - This deduplication may happen either:
	      - in the engine before enqueueing actor work
	      - or in the background worker before opening actor artifacts
	    - The implementation goal is the same in both cases:
	      - no pointless read/write churn for unchanged actors
	      - no cosmetic "patching" work when the effective actor contribution is
	        identical
	  - Costa optimization principle on 2026-04-25:
	    - Feed updates do not affect all countries and ASNs in the old+new
	      membership union.
	    - Ordinary feed-update background work must narrow the target set to the
	      actors whose per-feed contribution actually changed.
	    - The expected changed set should be very small relative to the full
	      country/ASN membership of broad feeds.
	  - Costa safeguard requirement on 2026-04-25:
	    - Keep the explicit full rebuild path for all country and ASN artifacts.
	    - Add an admin UI action to queue a complete country/ASN artifact rebuild
	      from scratch as an operator-visible background task.
	    - This exists as a protection mechanism in case incremental maintenance
	      ever leaves ASN/country artifacts out of sync.
	    - Ordinary feed-update refresh must stay incremental; the full rebuild
	      path is a fallback and operator tool, not part of the normal refresh
	      flow.
	  - Costa UI follow-up on 2026-04-25:
	    - On country and ASN pages, the feeds table should size columns more
	      naturally.
	    - Cells that need more horizontal space should get it instead of wrapping
	      prematurely because of rigid table widths.
	  - Costa admin follow-up on 2026-04-25:
	    - The admin background-worker queue/status section should always be
	      visible, even when empty.
	    - The `Rebuild All` action in entity integrity was pressed once and did
	      not visibly queue or run the rebuild; inspect and fix the full action
	      path.
	  - Costa rate-limit follow-up on 2026-04-25:
	    - The web server appears to stop responding to `/api/` requests under the
	      current rate limiter.
	    - Re-evaluate the rate-limit scope so cheap/static delivery is not
	      throttled unnecessarily.
	    - Prefer rate limiting only request classes that create meaningful load,
	      or raise the broad default significantly if a general `/api/*` limiter
	      remains.
	  - Costa integrity follow-up on 2026-04-25:
	    - Investigate why downloaded feeds appear with integrity issues in the
	      admin surface.
	    - Determine whether the findings reflect real missing/stale files or an
	      incorrect integrity rule/reporting bug.
	    - Decision made:
	      - fix the entity-sidecar compatibility crash so processing batches can
	        complete and publish secondary artifacts again
	    - Follow-up after recovery:
	      - `/api/v1/admin/integrity/entities` reports `21298` stale or broken
	        country/ASN artifact targets.
	      - Dominant reason is `ASN detail sidecar is older than its private
	        feed sidecars`.
	      - This conflicts with the approved surgical-refresh behavior: if an
	        actor's feed contribution did not change, the background worker
	        intentionally skips rewriting that actor.
	      - Fix the entity-integrity rule so actor detail artifacts are not
	        reported stale solely because related feed sidecars have newer
	        mtimes; private actor sidecars should still be checked for
	        existence, readability, public-payload freshness, and health drift.
	    - Implementation direction:
	      - remove mtime-only country/ASN actor staleness findings from entity
	        integrity because unchanged surgical actor contributions are not
	        rewritten by design
	      - keep missing/malformed actor checks and public-payload-newer-than-
	        private-sidecar checks
	      - keep feed-sidecar freshness checks, but make repair recompute/touch
	        feed sidecars from live inputs instead of treating a missing pending
	        sidecar as a feed deletion
	      - do not report missing feed sidecars for feeds whose preferred
	        country/ASN inputs have zero entity contributions
	      - touch unchanged committed feed sidecars after a successful unchanged
	        recomputation so future integrity checks have explicit freshness
	        metadata without rewriting identical JSON
	      - treat a feed sidecar as current when its recorded source timestamp
	        covers a future-dated `latest` local set mtime; upstream/source
	        mtimes can be ahead of daemon wall clock, so file mtime alone is
	        not proof of stale derivation in that case
	  - Verified facts for the integrity investigation:
	    - `/api/v1/admin/integrity` currently reports `305` findings:
	      - `304` are `stale secondary files (older than last processing)`
	      - `1` is a history-derivative recovery finding
	    - Sample feed `abuseipdb_1d` shows the exact failure pattern:
	      - cache entry `processed_date=1777086461` (`2026-04-25T03:07:41Z`)
	      - committed feed body mtime is `2026-04-25T00:41:47Z`
	      - web secondaries such as `abuseipdb_1d.json`,
	        `abuseipdb_1d_history.csv`, and `abuseipdb_1d_retention.json` are
	        older than that `processed_date`
	    - The integrity checker is behaving as written:
	      - it compares secondary mtimes against `cache.Entry.ProcessedDate`
	        in `pkg/engine/integrity.go`
	    - Recent journaled runs repeatedly end with the same post-processing
	      failure after the source-update phase:
	      - `run finished ...`
	      - `processing batch failed error="json: cannot unmarshal string into Go struct field feedEntitySidecar.countries of type engine.feedEntityCountryContribution"`
	    - The failing code path is in
	      `pkg/engine/entity_feed_sidecar.go:stageFeedEntitySidecarsFromLoadedProviders()`:
	      - it loads the committed feed entity sidecar with
	        `loadCommittedFeedEntitySidecar()`
	      - that decoder now expects structured `countries[]` rows
	    - The installed committed sidecars are still in the legacy format:
	      - example `/opt/update-ipsets/lib/entities/feeds/abuseipdb_1d.json`
	        has `"countries": ["AD", "AE", ...]`
	    - `pkg/engine/run.go` calls
	      `stageFeedEntitySidecarsFromLoadedProviders()` before
	      `writeMetadataFiles()` and before publish.
	    - Implication:
	      - updated feed bodies and cache `ProcessedDate` are committed first
	      - then the entity sidecar decode crash aborts the run
	      - metadata/history/retention/comparison/insight publication for the
	        updated feeds does not complete
	      - integrity later reports those feeds as stale relative to the new
	        `ProcessedDate`
	  - Verified facts for the freeze/pending-request investigation:
	    - Static assets under `/static/*` are not rate-limited by the current
	      broad limiter; only `/api/*` is covered.
	    - A local burst test confirmed the current broad limiter trips on
	      `/api/v1/status` after 240 requests/minute and does not affect
	      `/static/assets/*`.
	    - The feed-detail page mounts many sections that each issue their own
	      queries after the main feed metadata request resolves.
	    - Some logically identical data is fetched under different React Query
	      keys, so those requests are not deduplicated by the client cache:
	      - `/api/v1/sets` is fetched separately by the header, sidebar, and
	        comparison-health logic using different query keys.
	    - The feed-detail page also eagerly fans out provider payload requests
	      via `useQueries()` for every configured geo/ASN provider, not only the
	      selected tab.
	  - Verified facts:
	    - local Netdata `otel-plugin` is listening on `127.0.0.1:4317`
	    - official Netdata documentation says the plugin receives OTLP/gRPC
	      metrics and logs on `127.0.0.1:4317`
	    - `update-ipsets` currently creates OTLP/HTTP exporters, whose normal
	      endpoint is `4318`
	    - `/metrics` is not a Prometheus endpoint in `update-ipsets`; it falls
	      through to the website SPA
	  - Implementation plan:
	    - add OTLP/gRPC exporter support alongside the existing OTLP/HTTP support
	    - make OTLP protocol selectable with `OTEL_EXPORTER_OTLP_PROTOCOL` or
	      `UPDATE_IPSETS_OTEL_PROTOCOL`
	    - support `OTEL_METRIC_EXPORT_INTERVAL` / local equivalent and configure
	      the installed service to export every `10000ms`
	    - configure the installed systemd unit for local Netdata:
	      - `UPDATE_IPSETS_OTEL=1`
	      - `UPDATE_IPSETS_OTEL_PROTOCOL=grpc`
	      - `OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4317`
	      - `OTEL_METRIC_EXPORT_INTERVAL=10000`
	    - disable trace export in the installed Netdata path unless Netdata exposes
	      an OTLP trace receiver, while keeping trace support available for generic
	      OTLP backends
	    - document the behavior in `README.md` and `specs/operating-principles.md`
	  - Verification notes:
	    - Go OpenTelemetry v1.43 requires OTLP/gRPC endpoint environment values to
	      include an `http` or `https` scheme, so the systemd unit uses
	      `http://127.0.0.1:4317`.
	    - The daemon originally pushed metrics every second through
	      `OTEL_METRIC_EXPORT_INTERVAL=1000`, but Netdata's generated OTel charts
	      use `update_every: 10` by default. The installed service now uses
	      `OTEL_METRIC_EXPORT_INTERVAL=10000` to match Netdata's default chart
	      cadence.
