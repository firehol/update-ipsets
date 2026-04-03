# SOW-0084 - Fix GLM Audit 6 findings (1 HIGH, 3 MEDIUM, 5 LOW)

## Status

Status: completed

Sub-state: implemented, validated, and ready for review.

## Requirements

### Purpose

Fix the remaining GLM audit 6 code and specification discrepancies so feed
health, admin HTTP semantics, processing status reporting, and durable project
specs match the implementation contracts operators rely on.

### User Request

> "Create an SOW for all these bugs found and I will spawn another agent to fix them now"

Follow-up request:

> "ok, fix it."

### Assistant Understanding

Facts:

- GLM audit 6 was a fresh independent audit of the project specs against
  current code.
- The audit reported 1 HIGH severity code bug, 3 MEDIUM findings, and 5 LOW
  documentation/spec consistency findings.
- Current code already clears raw-feed failures for `StatusNotModified`; the
  remaining D4 success-path gap is in same/downloaded/empty canonical feed-body
  success paths.
- `go doc net/http.ServeMux` for the active Go toolchain says a `GET` method
  pattern matches both GET and HEAD requests.
- The active worktree already contains unrelated SOW/spec/code edits. The
  implementation must preserve them and avoid broad reverts.

Inferences:

- D4 remains the highest-risk issue because stale `DownloadFailures` can keep
  backoff/operator-health state wrong after a successful raw-feed download.
- D1 needs a broader fix than the original SOW text listed: every admin read
  handler reached by a `GET` ServeMux pattern must allow HEAD unless it is an
  action endpoint that is intentionally POST-only.
- D2 and the LOW findings are durable spec corrections, but they still need
  precise owner files so they are not lost during implementation.

Unknowns:

- None for planning. Any unexpected test failures are implementation findings
  and must be recorded in the execution log before changing scope.

### Acceptance Criteria

- D4: direct raw-feed successful outcomes clear `DownloadFailures` and
  `FailureStartedDate` for `not_modified`, `same`, `downloaded`, and `empty`
  success results; tests prove the behavior.
- D1: admin read endpoints that are registered as GET read surfaces accept HEAD
  with the same status semantics as GET, while POST-only action endpoints still
  reject GET/HEAD with `405` and `Allow: POST`; HTTP tests prove both sides.
- D2/D3: `missing_input` and `cancelled` processing exception classes are
  consistently represented in code, operator status, tests, and
  `.agents/sow/specs/processing-engine.md`.
- LOW findings: each of the 5 LOW findings is either fixed in the relevant spec
  file or explicitly rejected in this SOW with evidence. No LOW item may remain
  prose-only or unmapped.
- Validation commands listed in this SOW pass, or any failure is recorded with
  concrete cause, scope, and follow-up decision before closing the SOW.

## Analysis

Sources checked:

- `.agents/sow/specs/admin-ui.md`
- `.agents/sow/specs/architecture-posture.md`
- `.agents/sow/specs/downloader.md`
- `.agents/sow/specs/operating-principles.md`
- `.agents/sow/specs/pipeline.md`
- `.agents/sow/specs/processing-engine.md`
- `pkg/cache/download_status.go`
- `pkg/cache/entry_config.go`
- `pkg/cache/entry_lifecycle.go`
- `pkg/cache/cache_test.go`
- `pkg/downloader/downloader.go`
- `pkg/engine/download_stage.go`
- `pkg/engine/process.go`
- `pkg/engine/processing_result.go`
- `pkg/web/admin.go`
- `pkg/web/admin_manifest.go`
- `pkg/web/http.go`
- `pkg/web/integrity.go`
- `pkg/web/routes.go`
- `pkg/web/routes_test.go`
- `tools/archposture/testdata/posture_baseline.json`
- `go doc net/http.ServeMux`

Current state:

- `pkg/engine/download_stage.go:370-381` already calls
  `clearFailure(entry)` for raw-feed `StatusNotModified`.
- `pkg/engine/download_stage.go:714-744` marks `same`, `empty`, and
  `downloaded` prepared feed-body results without clearing failure state.
- `pkg/engine/download_stage.go:753-760` marks an existing retained feed body
  as `same` without clearing failure state.
- `pkg/engine/download_stage.go:651`, `:663`, and `:678` show the staged
  provider/artifact path clearing failure state before successful terminal
  statuses.
- `pkg/web/admin.go:320` and `:414`, `pkg/web/admin_manifest.go:97`,
  `pkg/web/integrity.go:95`, and `:142` reject `HEAD` inside admin read
  handlers even though their `GET` route registrations match HEAD.
- `.agents/sow/specs/admin-ui.md:509-512` defines admin read endpoints as
  GET/HEAD and action endpoints as POST.
- `pkg/cache/entry_lifecycle.go:30` stores `failed` for missing feed-body input
  while `pkg/engine/process.go:86-94` returns the structured
  `missing_input` exception and `pkg/engine/operator_status.go:89-93` already
  knows how to display `missing_input`.
- `pkg/cache/cache_test.go:508-510` currently expects the old D3 behavior.
- `pkg/engine/processing_result.go:18` defines `cancelled`, and
  `.agents/sow/specs/pipeline.md:56-58` references it, but
  `.agents/sow/specs/processing-engine.md:258-281` does not list it in the
  exception model.
- `tools/archposture/testdata/posture_baseline.json:769`, `:771`, and `:972`
  report `test_entry_calls: 85`, `test_field_writes: 336`, and
  `mux_handle_calls: 3`, while `.agents/sow/specs/architecture-posture.md`
  still lists stale values at lines 73, 203, and 205.
- `pkg/cache/download_status.go:6-20` defines cache/operator downloader-stage
  statuses including `downloaded`, `missing_env`, `url_resolve_failed`,
  `prepare_failed`, and `materializing`.
- `pkg/downloader/downloader.go:21-29` defines lower-level downloader result
  statuses including `ok`; the spec must distinguish this from the
  cache/operator `downloaded` status rather than replacing either one.
- `pkg/web/http.go:52-64` handles all `OPTIONS` with `204`, but deliberately
  omits wildcard CORS headers for `/api/v1/admin/` and `/admin` paths.

Risks:

- Treating D4 as a cache-method problem would clear failures too broadly or not
  at the right ownership layer. The safer pattern is to clear in successful
  engine download-stage paths, matching existing staged-download behavior.
- Fixing only two D1 method checks would leave other admin read endpoints
  spec-inconsistent.
- Collapsing `ok` and `downloaded` into one term would blur two different
  layers: low-level acquisition result versus cache/operator terminal status.
- Updating architecture posture prose without running the posture test can leave
  stale numbers again.

## Pre-Implementation Gate

Status: ready

Problem / root-cause model:

1. D4 HIGH code bug:
   - Raw-feed success paths that pass through `applyPreparedFeedBodyResult` and
     `applyExistingFeedBodySameResult` update terminal success status without
     clearing previous download-failure counters.
   - `StatusNotModified` is not part of the remaining bug because
     `pkg/engine/download_stage.go:380` already clears failure state.
   - Root cause: the raw-feed helpers own terminal cache status updates but did
     not reuse the `clearFailure(entry)` success invariant already present in
     `applyStagedDownloadResult`.

2. D1 MEDIUM code bug:
   - Admin read routes are registered with `GET` ServeMux patterns, and Go
     serves HEAD to those handlers too.
   - Several admin read handlers then reject `r.Method == HEAD` manually.
   - Root cause: route registration adopted Go 1.22 method patterns, but older
     handler-local method guards were not updated to mirror GET/HEAD semantics.

3. D2 MEDIUM spec bug:
   - Code and pipeline spec define/report `cancelled`, but
     `processing-engine.md` omits it from the processing exception model.

4. D3 MEDIUM code bug:
   - Processing returns `ProcessingExceptionMissingInput`, and operator status
     handles `missing_input`, but cache `LastStatus` stores generic `failed`.
   - Root cause: the cache lifecycle method predates the structured exception
     class and was not updated with the rest of the status model.

5. LOW spec consistency bugs:
   - Architecture posture prose has stale numeric values.
   - Downloader spec no longer fully documents cache/operator downloader-stage
     statuses.
   - Processing-engine spec omits provider/heavy-phase operator-visible status
     strings.
   - CORS/admin OPTIONS behavior is implemented but not stated clearly enough in
     the serving policy spec.
   - Downloader spec must explain `ok` versus `downloaded` as separate layer
     statuses.

Evidence reviewed:

| Finding | Evidence |
|---------|----------|
| D4 | `pkg/engine/download_stage.go:370-381` clears `StatusNotModified`; `pkg/engine/download_stage.go:714-744` and `:753-760` do not clear same/downloaded/empty success paths; `pkg/engine/download_stage.go:651`, `:663`, `:678` show the pattern to reuse. |
| D1 | `.agents/sow/specs/admin-ui.md:509-512` requires GET/HEAD reads and POST actions; `pkg/web/routes.go:259-270` registers admin reads as GET; `pkg/web/admin.go:320`, `:414`, `pkg/web/admin_manifest.go:97`, `pkg/web/integrity.go:95`, `:142` reject HEAD. |
| D2 | `pkg/engine/processing_result.go:18` defines `cancelled`; `.agents/sow/specs/pipeline.md:56-58` references it; `.agents/sow/specs/processing-engine.md:258-281` omits it. |
| D3 | `pkg/cache/entry_lifecycle.go:30` stores `failed`; `pkg/engine/process.go:86-94` returns `missing_input`; `pkg/engine/operator_status.go:89-93` maps `missing_input`; `pkg/cache/cache_test.go:508-510` expects the old generic value. |
| LOW-1 | `tools/archposture/testdata/posture_baseline.json:769`, `:771`, `:972` conflict with `.agents/sow/specs/architecture-posture.md:73`, `:203`, `:205`. |
| LOW-2 | `pkg/cache/download_status.go:6-20` lists cache/operator downloader-stage statuses not fully documented by `.agents/sow/specs/downloader.md`. |
| LOW-3 | `pkg/cache/entry_config.go:232`, `:266`, `:277`, `:299`, `:326` and `pkg/cache/entry_lifecycle.go:300` write provider/heavy-phase statuses not fully documented by `.agents/sow/specs/processing-engine.md`. |
| LOW-4 | `pkg/web/http.go:52-64` shows admin OPTIONS returns `204` without wildcard CORS headers; `.agents/sow/specs/operating-principles.md:221-229` only says admin endpoints exclude CORS. |
| LOW-5 | `pkg/downloader/downloader.go:21-29` uses `ok`; `pkg/cache/download_status.go:16` and `pkg/engine/download_stage.go:691`, `:747` use `downloaded`. |

Affected contracts and surfaces:

- Cache entry lifecycle and operator-visible feed health.
- Scheduler retry/backoff inputs via `DownloadFailures` and
  `FailureStartedDate`.
- Admin HTTP API method semantics for GET/HEAD read surfaces and POST actions.
- Processing exception/status model in code, tests, admin UI inputs, and specs.
- Downloader and processing specs under `.agents/sow/specs/`.
- Architecture posture baseline prose and guard validation.
- CORS serving policy for public versus admin surfaces.

Existing patterns to reuse:

- `applyStagedDownloadResult` clears failure state before success markers.
- `clearFailure(entry)` centralizes reset of `DownloadFailures` and
  `FailureStartedDate`.
- Admin action endpoints already use `Allow: POST` and `405` for wrong methods.
- `OperatorStatusForLastStatus` already handles `missing_input`.
- `routes_test.go` uses `webHTTPTestServer` for black-box route and method
  behavior.
- `cache_test.go` already has table/transition tests for cache lifecycle status
  writers.

Risk and blast radius:

- D4: low code blast radius, medium operational impact. Changes are localized
  but affect backoff and visible feed health after recovery.
- D1: low code blast radius. Risk is accidentally allowing HEAD for POST-only
  action endpoints; tests must prove action endpoints remain POST-only.
- D2: spec-only, zero runtime risk.
- D3: low runtime risk. Operator-visible `LastStatus` changes from generic
  `failed` to specific `missing_input`; this is intended but must update tests.
- LOW spec fixes: low runtime risk, but they protect future agents from
  repeating incorrect assumptions.

Sensitive data handling plan:

- No raw external audit logs, prompts, customer identifiers, public
  customer-identifying IPs, secrets, bearer tokens, SNMP communities, private
  endpoints, or proprietary incident details are required.
- Durable artifacts should cite only repository file paths, line numbers,
  status strings, commands, and sanitized behavior descriptions.

Implementation plan:

1. D4 code and tests:
   - In `pkg/engine/download_stage.go`, call `clearFailure(entry)` immediately
     before terminal success markers in `applyPreparedFeedBodyResult` for
     `same`, `empty`, and `downloaded`.
   - In `applyExistingFeedBodySameResult`, call `clearFailure(entry)` before
     `entry.MarkDownloadSame()`.
   - Do not move this reset into `cache.Entry.MarkDownload*`; cache methods are
     generic status writers and do not own engine download-attempt success
     semantics.
   - Add or extend engine tests to seed prior `DownloadFailures` and assert they
     clear for raw-feed `not_modified`, `same`, `downloaded`, and `empty`
     successful outcomes. Prefer exported `FetchAndStage` behavior; use
     same-package helper coverage only where reaching the branch through the
     public surface would be much broader than the contract.

2. D1 code and tests:
   - Update admin read method checks in `pkg/web/admin.go`,
     `pkg/web/admin_manifest.go`, and `pkg/web/integrity.go` so `GET` and
     `HEAD` are accepted for read handlers.
   - Ensure `Allow` headers for read-handler method failures include both
     `GET` and `HEAD`.
   - Preserve POST-only behavior for `recheck`, `reprocess`, `enable`,
     `disable`, integrity rebuild, integrity reprocess, and admin run actions.
   - Add route tests using `webHTTPTestServer` for HEAD admin read endpoints
     and wrong-method POST action endpoints.

3. D3 code and tests:
   - Change `MarkSourceProcessingMissingInput` to store
     `LastStatus = "missing_input"`.
   - Update `pkg/cache/cache_test.go` expectations.
   - Confirm operator status continues to map `missing_input` as a processing
     problem.

4. D2 and LOW spec fixes:
   - In `.agents/sow/specs/processing-engine.md`, add `cancelled` to the
     exception model and add the provider/heavy-phase visible statuses:
     `config_error`, `extract_failed`, `open_failed`, `unavailable`, `stale`,
     and `running`.
   - In `.agents/sow/specs/downloader.md`, document both layers:
     low-level downloader result status `ok`, and cache/operator
     downloader-stage terminal status `downloaded`. Also restore/document
     `missing_env`, `url_resolve_failed`, `prepare_failed`, `materializing`,
     and the rest of `pkg/cache/download_status.go`.
   - In `.agents/sow/specs/architecture-posture.md`, update route inventory to
     3 `Handle` registrations, test `Entry()` calls to 85, and test mutable
     cache-entry field writes to 336.
   - In `.agents/sow/specs/operating-principles.md`, clarify that admin
     OPTIONS may be answered by shared middleware with `204`, but MUST NOT emit
     wildcard CORS headers or advertise cross-origin admin access.

5. Same-failure scan:
   - Search for every `r.Method != http.MethodGet` admin read guard and update
     all GET/HEAD read surfaces together.
   - Search for every raw-feed success marker path that writes
     `MarkDownloadNotModified`, `MarkDownloadSame`, `MarkDownloadDownloaded`,
     or `MarkDownloadEmpty` and confirm the D4 invariant.
   - Search specs for `defer`, `later`, `follow-up`, `future`, `TODO`, and
     `pending` before closure and map any valid remaining item.

Validation plan:

- `go test ./pkg/cache`
- `go test ./pkg/engine`
- `go test ./pkg/web`
- `go build ./...`
- `go test ./tools/archposture -v`
- Targeted manual/source verification:
  - D4: every raw-feed success marker path clears failures before status write.
  - D1: all admin read method guards accept GET/HEAD and action guards remain
    POST-only.
  - LOW findings: every one of the 5 LOW items has an edited spec location or
    an evidence-backed rejection in the SOW.

Artifact impact plan:

- AGENTS.md: no update expected; current project rules already cover SOW,
  runtime artifacts, GET/HEAD surfaces, and deferred-work mapping.
- Runtime project skills: no update expected unless implementation discovers a
  reusable rule not already present in `project-reviewing`, `project-testing`,
  or `project-content-surfaces`.
- Specs:
  - `.agents/sow/specs/processing-engine.md`
  - `.agents/sow/specs/downloader.md`
  - `.agents/sow/specs/architecture-posture.md`
  - `.agents/sow/specs/operating-principles.md`
- End-user/operator docs: no update expected. These fixes are API/spec/status
  consistency work, not user-facing operational instructions.
- End-user/operator skills: no update expected.
- SOW lifecycle: keep this SOW pending until implementation starts; move to
  current when work begins; close only after all audit findings and validation
  evidence are recorded. No deferrals are allowed without a concrete pending
  SOW path or evidence-backed rejection.

Open decisions:

- None. The user approved fixing the SOW handoff artifact.

## Implications And Decisions

1. Decision: correct the SOW before implementation.
   - Evidence: the original SOW missed several D1 HEAD failure paths, overstated
     D4 by including `StatusNotModified`, omitted two LOW work items from the
     implementation plan, and lacked acceptance criteria.
   - Implication: the next agent can implement from this SOW without silently
     dropping audit findings.
   - Risk: line numbers may shift while the worktree remains dirty; implementer
     must re-check touched code before editing.

## Plan

1. Fix D4 failure reset in raw-feed success paths and add behavioral coverage.
2. Fix D1 admin GET/HEAD read semantics across every admin read guard and add
   HTTP route coverage.
3. Fix D3 missing-input cache status and update cache tests.
4. Fix D2 and all LOW spec consistency issues in the canonical spec owners.
5. Run validation, same-failure scans, sensitive-data gate, and artifact
   maintenance gate before closing.

## Execution Log

### 2026-05-01

- SOW handoff review corrected the scope and evidence before implementation.
- Moved SOW from `.agents/sow/pending/` to `.agents/sow/current/` and marked
  it in-progress before code changes.
- Fixed raw-feed success paths so `same`, `downloaded`, and `empty` terminal
  results clear previous downloader failure counters.
- Fixed admin read handlers so GET/HEAD read surfaces accept HEAD while
  POST-only action surfaces remain POST-only.
- Changed missing feed-body processing cache status from generic `failed` to
  `missing_input`.
- Added targeted cache, engine, and web behavioral tests.
- Updated `processing-engine.md`, `downloader.md`,
  `architecture-posture.md`, and `operating-principles.md` for the D2 and LOW
  findings.

## Validation

Acceptance criteria evidence:

- D4: `pkg/engine/download_stage.go` now calls `clearFailure(entry)` before raw
  feed-body `same`, `empty`, `downloaded`, and existing-body `same` success
  markers. Existing `not_modified` already cleared failures.
- D1: admin read method guards now share `isReadMethod` and
  `writeReadMethodNotAllowed`, accepting GET/HEAD and returning `Allow: GET,
  HEAD` for other methods. POST-only action routes still return `Allow: POST`.
- D2/D3: `cancelled` is documented in `processing-engine.md`; cache
  missing-input status now stores `missing_input`; tests assert the new cache
  status.
- LOW findings: all 5 LOW items were fixed in specs. No LOW item was deferred
  or rejected.

Tests or equivalent validation:

- `go test ./pkg/cache` - passed.
- `go test ./pkg/engine` - passed.
- `go test ./pkg/web` - passed.
- `go build ./...` - passed.
- `go test ./tools/archposture -v` - passed.
- `make test` - passed.

Real-use evidence:

- HTTP behavior was exercised through `webHTTPTestServer`, which uses a real
  `httptest.Server` and the project handler stack.
- No installed-service smoke was run because the change does not alter daemon
  startup, installation, service configuration, or public artifact publication.

Reviewer findings:

- Handoff review findings incorporated:
  - D1 scope expanded beyond `pkg/web/admin.go`.
  - D4 corrected to exclude already-fixed `StatusNotModified`.
  - All 5 LOW findings mapped to specific work.
  - Acceptance criteria, validation, and artifact impact plans added.

Same-failure scan:

- `rg -n 'r\.Method != http\.MethodGet|Allow", http\.MethodGet|use GET' pkg/web -g '*.go'`
  now finds only the shared `writeReadMethodNotAllowed` helper.
- `rg -n -C 2 'MarkDownload(NotModified|Same|Downloaded|Empty)\(' pkg/engine/download_stage.go pkg/engine/artifact_stage.go`
  confirmed all terminal downloader success markers in the touched raw/staged
  paths have an adjacent `clearFailure(entry)` success invariant. Artifact
  paths already had the invariant.
- SOW keyword scan found no valid deferred work. The remaining matches are
  validation instructions, lifecycle wording, or the explicit no-deferral
  statement.

Sensitive data gate:

- Passed. Durable changes contain only repository paths, status strings,
  command names, tests, and behavior descriptions. No raw secrets, credentials,
  bearer tokens, SNMP communities, customer names, personal data,
  customer-identifying public IPs, private endpoints, or proprietary incident
  details were added.

Artifact maintenance gate:

- AGENTS.md: no update needed; existing project rules already cover SOW gates,
  GET/HEAD read surfaces, POST admin actions, specs, testing, and runtime
  artifact discipline.
- Runtime project skills: no update needed; `project-coding`,
  `project-reviewing`, `project-testing`, and `project-content-surfaces`
  already cover the lessons used here.
- Specs: updated `.agents/sow/specs/processing-engine.md`,
  `.agents/sow/specs/downloader.md`,
  `.agents/sow/specs/architecture-posture.md`, and
  `.agents/sow/specs/operating-principles.md`.
- End-user/operator docs: no update needed; this is internal API/status/spec
  consistency work, not an operator workflow change.
- End-user/operator skills: no update needed; no output/reference skill surface
  changed.
- SOW lifecycle: moved from pending to current before implementation, then to
  `.agents/sow/done/` after validation and closure.

Specs update:

- Updated:
  - `.agents/sow/specs/processing-engine.md`
  - `.agents/sow/specs/downloader.md`
  - `.agents/sow/specs/architecture-posture.md`
  - `.agents/sow/specs/operating-principles.md`

Project skills update:

- No update needed. Existing skills already require the relevant HTTP,
  status-model, SOW, testing, and surface-discipline behavior.

End-user/operator docs update:

- No update needed. No user-facing operation, install, or public methodology
  contract changed.

End-user/operator skills update:

- No update needed.

Lessons:

- Handoff SOWs for external audit results must map every finding to an
  implementation item, rejection, or concrete follow-up path before coding.

Follow-up mapping:

- None. No deferrals remain.

## Outcome

Completed. SOW-0084 fixes were implemented and validated.

## Lessons Extracted

- Re-check audit line evidence against the current dirty worktree before
  implementation; external audit handoff text can become stale even when the
  finding class is valid.
- Keep low-severity audit findings explicitly mapped. "Spec cleanup" still
  needs a concrete owner file or an evidence-backed rejection.

## Followup

None.
