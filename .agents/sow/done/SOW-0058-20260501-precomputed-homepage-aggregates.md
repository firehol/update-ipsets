# SOW-0058 - Precomputed Homepage Aggregates

## Status

Status: completed

Sub-state: validated and closed after iterative quality audit cycle 5 repair

## Requirements

### Purpose

Keep public homepage API serving cheap, cache-first, and fit for repeated public traffic by moving broad homepage aggregation off the request path.

### User Request

Review project quality against the Go/frontend best-practice and behavioral-testing skills, identify gaps, create SOWs for actionable improvements, then proceed with the first recommended SOW.

### Assistant Understanding

Facts:

- Public homepage endpoints are registered at `pkg/web/routes.go:28` and `pkg/web/routes.go:29`.
- `handleHomeGlobe` computes `eng.HomeGlobeInDir(...)` per request at `pkg/web/home_api.go:19`.
- `handleHomeSummary` computes `eng.HomeSummaryInDir(...)` per request at `pkg/web/home_api.go:50`.
- `HomeGlobeInDir` walks `EntriesSnapshot()` and reads per-feed country artifacts at `pkg/engine/home_globe.go:70` and `pkg/engine/home_globe.go:82`.
- `HomeSummaryInDir` walks `EntriesSnapshot()` and reads per-feed country/ASN artifacts at `pkg/engine/home_summary.go:146`, `pkg/engine/home_summary.go:184`, and `pkg/engine/home_summary.go:210`.
- `.agents/sow/specs/operating-principles.md` requires public browsing to use precomputed artifacts, not live broad recomputation.

Inferences:

- The request path is local-only I/O, not upstream I/O, but still scales with feed count and artifact count.
- Category-filter behavior needs an explicit product contract before artifacts are designed.

Unknowns:

- Current production latency and I/O profile for these endpoints on the published corpus.

### Acceptance Criteria

- Homepage summary/globe data needed by public routes is produced or refreshed during processing/entity refresh/repair, not on first public request.
- Public homepage handlers read precomputed artifacts or bounded static cache entries.
- Category-filter behavior is specified, including supported filter combinations and missing/stale artifact behavior.
- A repair/reprocess path restores missing or stale homepage aggregate artifacts.
- Tests prove public requests do not trigger broad per-feed artifact reads.
- Reopened scope: malformed `web/home/aggregates.json` artifacts are treated as
  homepage aggregate not-ready errors, preserve the decode error chain, and
  return HTTP `503` from homepage summary/globe routes instead of `400`.

## Analysis

Sources checked:

- `pkg/web/home_api.go`
- `pkg/web/routes.go`
- `pkg/engine/home_globe.go`
- `pkg/engine/home_summary.go`
- `.agents/sow/specs/operating-principles.md`
- Official TanStack Query cancellation docs were checked during the broader review for frontend request-boundary context: https://tanstack.com/query/v5/docs/react/guides/query-cancellation

Current state:

- The homepage endpoints recompute aggregate payloads on each request.
- The code records counters such as `http.home_summary.country_json_requests`, which confirms per-request artifact reads are expected today.

Risks:

- Homepage traffic can turn into repeated O(number of feeds + artifact reads) work.
- Adding generated artifacts without clear refresh ownership could create stale homepage data or request-time fallbacks.

## Pre-Implementation Gate

Status: ready

Problem / root-cause model:

- Public homepage summary/globe endpoints recompute broad aggregate payloads per request.
- The request path walks feed snapshots and reads per-feed country/ASN artifacts, which violates the cache-first public serving contract for repeated traffic.
- The fix must create producer-owned aggregate artifacts and make public handlers read those artifacts instead of triggering broad request-time work.

Evidence reviewed:

- `pkg/web/home_api.go`
- `pkg/web/routes.go`
- `pkg/engine/home_globe.go`
- `pkg/engine/home_summary.go`
- `.agents/sow/specs/operating-principles.md`

Affected contracts and surfaces:

- Public homepage API handlers.
- Homepage summary/globe generated artifact ownership.
- Processing/entity refresh/repair paths.
- Missing/stale artifact behavior.
- Specs and operator docs if the homepage artifact contract becomes user/operator visible.

Existing patterns to reuse:

- Public serving must remain cache-first and cheap.
- Generated artifacts need explicit producer paths and logical mtimes.
- Repair/reprocess paths restore missing or stale artifacts.

Risk and blast radius:

- Incorrect artifact refresh ownership could create stale homepage data.
- Request-time fallback would preserve the failure mode.
- Category-filter semantics can diverge from feed explorer behavior if not specified first.

Implementation plan:

1. Specify homepage aggregate artifact ownership and category-filter behavior.
2. Add producer logic to refresh/repair paths with explicit logical mtimes.
3. Update homepage handlers to read only precomputed artifacts or bounded static cache entries.
4. Add missing/stale artifact behavior and tests.
5. Search public handlers for similar broad request-time aggregation.

Validation plan:

- Go tests for producer, serving, stale/missing behavior, and no request-time broad recomputation.
- Same-failure scan across public API handlers.
- Specs/docs validation for any changed operator-visible behavior.

Artifact impact plan:

- AGENTS.md: no update expected unless new public-serving workflow rules are learned.
- Runtime project skills: update if the implementation adds reusable artifact-generation/review rules.
- Specs: update homepage/operating-principles specs for artifact ownership and missing/stale behavior.
- End-user/operator docs: update if behavior, API semantics, or repair operation changes are operator-visible.
- End-user/operator skills: no update expected unless operator workflow changes.
- SOW lifecycle: keep this SOW current until implementation and validation close gates are complete.

Open decisions:

- Artifact strategy decision has been recorded below: base aggregate plus per-category aggregate slices.

## Implications And Decisions

1. Artifact strategy

   Selection: B. Precompute a base aggregate plus per-category aggregate slices.

   Evidence:

   - `pkg/web/home_api.go:19` and `pkg/web/home_api.go:50` currently recompute homepage payloads per request.
   - `pkg/engine/home_globe.go:70` and `pkg/engine/home_summary.go:146` walk the feed snapshot per request.
   - `pkg/engine/home_summary.go:184` and `pkg/engine/home_summary.go:210` read per-feed country/ASN artifacts per request.

   Reasoning:

   - Precomputing every category combination can grow combinatorially.
   - Keeping request-time aggregation behind an in-memory cache still leaves a first-public-request rebuild path.
   - Base plus per-category generated artifacts keeps artifact count bounded while allowing public handlers to satisfy category filters from already-published local artifacts.

   Implications:

   - The producer path must run during normal processing or repair, not public request handling.
   - The serving path must return a clear not-ready/error response if required aggregate artifacts are missing or stale.
   - Specs must define refresh ownership and missing-artifact behavior.

   Risks:

   - If generated artifacts are not stamped with the correct logical mtime, integrity may report false drift.
   - If category filters are composed incorrectly, homepage counts can disagree with feed explorer filters.

Recommended starting decision:

1. Artifact strategy
   - A. Precompute all supported category combinations.
     - Pros: fastest public reads.
     - Cons: combinatorial artifact growth if categories expand.
   - B. Precompute a base aggregate plus per-category aggregate slices. Recommended.
     - Pros: bounded artifact count and cheap request-time merge/filter work.
     - Cons: still needs careful stale/missing semantics.
   - C. Keep request-time aggregation and add an in-memory cache.
     - Pros: smallest change.
     - Cons: preserves first-request broad work and weakens cache-first serving.

## Plan

1. Specify homepage aggregate artifact ownership in `.agents/sow/specs/website.md` and/or operating principles.
2. Add producer logic to processing/entity refresh paths with explicit logical mtimes.
3. Update homepage handlers to read only precomputed artifacts.
4. Add repair/reprocess coverage for missing/stale homepage aggregate artifacts.
5. Add Go tests for producer, serving, stale/missing behavior, and no request-time broad recomputation.

## Execution Log

### 2026-05-01

- Created from the four-skill quality review.
- Moved to current after user said "Proceed".
- Recorded artifact strategy decision before implementation.
- Added `web/home/aggregates.json` as the bounded homepage rollup artifact.
- Wired aggregate generation into normal publication, entity repair/rebuild, and health-transition refresh paths.
- Changed public homepage summary/globe handlers to read the aggregate and return not-ready service responses when it is missing or unsupported.
- Updated homepage, website, and files-layout specs with the artifact and serving contract.
- Reopened after audit cycle 2 found the repair/integrity acceptance criterion and HTTP not-ready test coverage were incomplete.
- Reopened scope:
  - Add integrity detection and automatic repair for missing, stale, and malformed `web/home/aggregates.json`.
  - Preserve request cancellation in the selected entity-artifact repair path that refreshes the homepage aggregate.
  - Stop silently swallowing malformed country/ASN sidecar inputs when producing the homepage aggregate.
  - Add behavioral coverage for HTTP not-ready responses, generated-file timestamp records, malformed sidecars, and integrity repair.
- Added entity-integrity detection and automatic repair for missing, stale, and malformed `web/home/aggregates.json`.
- Propagated the caller/test context through selected entity-artifact repair and the homepage aggregate refresh it performs.
- Changed homepage aggregate production to return errors for malformed country/ASN input artifacts instead of silently dropping those rows.
- Added behavioral tests for aggregate generated-file timestamp records, malformed aggregate inputs, entity-integrity repair, and HTTP `503` not-ready responses.
- Split homepage integrity and selected entity repair helpers into focused files after the architecture posture gate rejected file/function growth.
- Revalidated after iterative audit cycle 4 found that the first closure missed
  `make staticcheck` and `make golangci-lint`.
- Fixed the Staticcheck `S1016` finding in the homepage aggregate globe country
  conversion and reran both Go lint gates.
- Reopened after iterative audit cycle 5 found malformed aggregate JSON was not
  classified as `ErrHomeAggregatesNotReady` and therefore returned `400` from
  homepage API handlers.
- Changed aggregate decoding errors to wrap `ErrHomeAggregatesNotReady` while
  preserving the JSON decode error.
- Added HTTP coverage proving malformed aggregate JSON returns `503` for both
  homepage summary and globe endpoints.

## Validation

Acceptance criteria evidence:

- Producer path:
  - `pkg/engine/home_aggregates.go:77` stages the aggregate under `home/aggregates.json` and returns a generated-file record with an explicit timestamp.
  - `pkg/engine/home_aggregates.go:112` builds per-category aggregate slices from the feed snapshot and staged/live published enrichment artifacts.
  - `pkg/engine/run_pipeline.go:304` refreshes the aggregate during normal publication when the heavy path ran.
- Repair/reprocess path:
  - `pkg/engine/entity_artifacts.go:200` refreshes the aggregate for health transitions even when no entity sidecars exist.
  - `pkg/engine/entity_artifacts.go:288`, `pkg/engine/entity_artifacts.go:562`, `pkg/engine/entity_artifacts.go:722`, and `pkg/engine/entity_artifacts.go:841` refresh the aggregate during entity refresh/repair/rebuild paths.
- Serving path:
  - `pkg/engine/home_summary.go:108` loads the precomputed aggregate and only does bounded in-memory composition for the requested category filter and limit.
  - `pkg/engine/home_globe.go:53` loads the precomputed aggregate for globe responses.
  - `pkg/web/home_api.go:22` and `pkg/web/home_api.go:57` map missing/unsupported aggregate artifacts to service-unavailable not-ready responses.
- Category-filter behavior:
  - `pkg/engine/home_aggregates.go:29` stores one slice per category.
  - `pkg/engine/home_aggregates.go:321` selects category slices and `pkg/engine/home_aggregates.go:333` composes summary payloads from selected slices.
- No broad public request reads:
  - `pkg/engine/home_summary.go:108` and `pkg/engine/home_globe.go:53` are the homepage request read points.
  - Same-failure scan found no remaining `http.home_summary.country_json_requests`, `http.home_summary.asn_json_requests`, or `http.home_globe.country_json_requests` counters.

Tests or equivalent validation:

- `go test ./pkg/engine ./pkg/web` passed.
- `go test ./...` passed.
- `make build` passed.
- `make test` passed.
- `make lint` passed.
- `git diff --check` passed.
- `./.agents/sow/audit.sh` passed.

Cycle-2 validation addendum:

- `go test ./pkg/engine ./pkg/web` passed after adding missing repair and HTTP not-ready coverage.
- `go test ./...` passed after splitting architecture-posture regressions into focused files.
- `make build` passed.
- `make test` passed.
- `make lint` passed.
- `git diff --check` passed.

Cycle-4 validation addendum:

- `make staticcheck` passed.
- `make golangci-lint` passed.
- The repaired lint finding was `pkg/engine/home_aggregates.go` Staticcheck
  `S1016`; the current implementation converts `HomeSummaryCountry` to
  `HomeGlobeCountry` directly.

Cycle-5 validation addendum:

- `go test ./pkg/engine ./pkg/web` passed after classifying malformed
  aggregate JSON as `ErrHomeAggregatesNotReady` and adding malformed-artifact
  HTTP coverage.

Real-use evidence:

- Not run against production data. Validation used behavioral unit/API tests and full project Go test/build/lint gates.

Reviewer findings:

- Go best-practices review found broad request-time homepage aggregation.
- Iterative audit cycle 2 found the initial implementation still lacked
  integrity/repair coverage for `web/home/aggregates.json`, dropped malformed
  sidecar input errors, and used a detached context in selected repair. These
  findings were fixed in this SOW.
- Iterative audit cycle 4 found that the first closure lacked explicit
  Staticcheck/golangci-lint validation. Those gates now pass.
- Iterative audit cycle 5 found malformed aggregate JSON still returned
  `400 Bad Request` instead of the not-ready `503` contract. This SOW was
  reopened for the focused repair, and the repair is complete.

Same-failure scan:

- `rg` scan confirmed homepage handlers now call only `HomeGlobeInDir` and `HomeSummaryInDir`; those functions load `web/home/aggregates.json`.
- The remaining `CountryComparisonInDir` public use is feed-scoped `/api/v1/sets/{name}/countries` in `pkg/web/routes.go:201`, which is a targeted artifact read and not the homepage broad rollup failure mode.

Artifact maintenance gate:

- AGENTS.md: no update needed; existing cache-first and generated-artifact rules already covered this pattern.
- Runtime project skills: no update needed; existing project skills already require cache-first serving, explicit producers, repair paths, and behavioral tests.
- Specs: updated `.agents/sow/specs/website.md`, `.agents/sow/specs/homepage.md`, and `.agents/sow/specs/files-layout.md`.
- End-user/operator docs: no update needed; public API shape is unchanged and the new artifact is an internal publication contract.
- End-user/operator skills: no update needed.
- SOW lifecycle: completed and ready to move to `.agents/sow/done/`.

Specs update:

- `.agents/sow/specs/website.md:397` documents homepage rollup serving rules.
- `.agents/sow/specs/homepage.md:450` documents precomputed homepage summary/globe rollups and refresh ownership.
- `.agents/sow/specs/files-layout.md:390` documents `web/home/aggregates.json`.

Project skills update:

- Not needed.

End-user/operator docs update:

- Not needed.

End-user/operator skills update:

- Not needed.

Lessons:

- Health transitions can change homepage rollup eligibility even when no country/ASN entity sidecar exists; repair paths must refresh aggregate artifacts from the state they expose, not only from entity-delta membership.

Follow-up mapping:

- None.

## Outcome

Completed.

## Lessons Extracted

No project-skill update required. The lesson is captured above for this SOW.

## Followup

None.
