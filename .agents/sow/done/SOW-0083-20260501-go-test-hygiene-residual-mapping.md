# SOW-0083 - Go Test Hygiene Residual Mapping

## Status

Status: completed

Sub-state: closed

## Requirements

### Purpose

Resolve the remaining C-class and low-priority Go behavioral-testing leftovers
from SOW-0039 by either fixing cohesive items or recording evidence-backed
non-goals.

### User Request

Review project quality against the four named project skills, identify gaps,
create SOWs for actionable improvements, and iterate the audit until valid
findings are fixed or tracked.

### Assistant Understanding

Facts:

- Iterative audit cycle 6 found SOW-0039 still listed several `NOT FIXED`
  C-class or neutral test-hygiene items without concrete follow-up ownership.
- The items include sparse `t.Parallel`, sparse `t.Cleanup`, missing `t.Helper`
  in repeated helpers, inconsistent test naming, table-driven opportunities,
  opt-in count-only catalog/script tests, file-cache freshness coverage,
  fuzz-input bounds, private `IPSet` representation comparisons, and duplicated
  polling helpers.

Inferences:

- These items are maintainability/testing-quality cleanup, not urgent product
  behavior defects.
- A broad mechanical sweep could create churn; item-by-item classification is
  safer.

Unknowns:

- Which tests cannot safely use `t.Parallel` because they share filesystem,
  process, scheduler, or global state.
- Whether a shared polling helper is worth introducing before SOW-0066 decides
  scheduler behavioral harness direction.

### Acceptance Criteria

- Review SOW-0039 B11, C3-C8, and C-new-1 through C-new-5 against current
  tests.
- For each item, choose one outcome: implement focused cleanup, split to a
  narrower pending SOW, or reject as not worth changing with evidence.
- If adding `t.Parallel`, prove no hidden global state or shared filesystem
  assumption is violated.
- If introducing shared helpers, keep them small and avoid turning behavior
  tests into internal utility tests.
- Update SOW-0039 follow-up mapping so no residual item remains as prose-only
  future work.
- Run affected package tests plus `make test`.

## Analysis

Sources checked:

- `project-go-behavioral-testing`
- `project-testing`
- Iterative audit cycle 6 Go behavioral-testing findings
- `.agents/sow/done/SOW-0039-20260501-go-test-re-review.md`

Current state:

- Larger test architecture items are already mapped to SOW-0064, SOW-0065,
  SOW-0066, SOW-0067, SOW-0075, and SOW-0081.
- Smaller residual hygiene items still need explicit ownership or rejection.

Residual outcome map:

| Item | Outcome | Evidence |
| --- | --- | --- |
| B11 sparse `t.Parallel` | Focused cleanup implemented; broad sweep rejected. | Added `t.Parallel()` only to isolated tempdir/cache/property tests in `pkg/web/cache_test.go:13`, `pkg/web/cache_test.go:32`, `pkg/web/cache_test.go:44`, `pkg/web/cache_test.go:56`, `pkg/web/cache_test.go:74`, and `pkg/iprange/set_properties_test.go:8`. Current total is 12 calls. Tests that share runner, engine, server, process, or catalog state should opt in only when proven isolated. |
| C3 sparse `t.Cleanup` | Rejected as broad mechanical churn. | Helper-owned resources already use cleanup where it matters: `pkg/web/http_test_helpers_test.go:20` and `pkg/web/feature_test.go:934`. Leaf-test `defer server.Close()` remains acceptable because it is scoped to one test body and has no hidden ownership boundary. |
| C4 missing `t.Helper` | Fixed. | Added `t.Helper()` to the two forwarding web helpers at `pkg/web/feature_test.go:918` and `pkg/web/feature_test.go:923`. A static scan for lower-case helpers taking `*testing.T` without `t.Helper()` now returns no matches. |
| C5 naming inconsistency | Rejected as broad rename churn. | Current names are mostly behavior/state descriptions; renaming passing tests does not improve runtime safety. New/edited tests in this SOW use behavior names such as `TestFileCacheInvalidatesCachedBodyWhenMTimeChanges` and table row names in `pkg/engine/run_pipeline_test.go:18`, `pkg/engine/run_pipeline_test.go:35`, `pkg/engine/run_pipeline_test.go:57`, and `pkg/engine/run_pipeline_test.go:79`. |
| C6 and C-new-1 table-driven opportunities | Focused cleanup implemented; broader function-per-contract style accepted. | Collapsed four `buildPipelineRunPlan` scenario functions into table-driven `TestBuildPipelineRunPlan` at `pkg/engine/run_pipeline_test.go:9`. Left provider-default tests as separate functions because they exercise distinct exported methods and summary surfaces, not one input matrix. |
| C7 opt-in `TestExtractLegacyScriptCounts` | Rejected as non-goal. | The count-only opt-in smoke remains useful when an operator provides `UPDATE_IPSETS_LEGACY_BASH` (`pkg/config/config_test.go:41`). Stronger name-diff coverage already exists in `TestCatalogMatchesLegacyExtraction` (`pkg/config/catalog_verify_test.go:1452`) and is also opt-in because the external bash checkout is not a repo dependency. |
| C8 catalog count test describes metrics | Accepted as catalog-shape guard. | The current test is named `TestCatalogExpectedCounts` and documents why counts matter (`pkg/config/catalog_verify_test.go:30`). `project-testing` explicitly requires updating duplicated source-count assertions when catalog inventory changes, so this is deliberate drift detection rather than padding. |
| C-new-2 file-cache freshness-only coverage | Fixed. | Added `TestFileCacheInvalidatesCachedBodyWhenMTimeChanges` at `pkg/web/cache_test.go:32`, proving same-size body changes are served after only the mtime changes. |
| C-new-3 fuzz bounds | Already fixed by SOW-0039. | Current fuzz targets use a 1 MiB pathological-input cap and assert success-path invariants: `pkg/processor/fuzz_test.go:17` and `pkg/config/fuzz_test.go:26`. |
| C-new-4 private `IPSet` representation comparison | Fixed. | `sameSet` now compares `UniqueCount()` plus public membership checks through `Contains()` over the generated domain (`pkg/iprange/set_properties_test.go:114`). |
| C-new-5 duplicated polling helpers | Mapped to existing SOW-0066. | The scheduler polling helper belongs to the scheduler behavioral harness decision because `synctest`, a shared `WaitFor`, and public `Runner.Run` harnessing are coupled choices. SOW-0066 now owns this classification. |

Risks:

- Adding `t.Parallel` blindly can expose or create flakes around shared
  tempdirs, global env, local HTTP ports, or global caches.
- Excess helper abstraction can make tests less readable if it hides the
  observable behavior under test.

## Plan

1. Re-check each residual SOW-0039 item against current tests.
2. Implement only cohesive, low-risk cleanup in the selected first pass.
3. Split or reject broader items with evidence.
4. Update SOW-0039 mapping and validation evidence.

## Execution Log

### 2026-05-01

- Created from iterative audit cycle 6.
- Reviewed each SOW-0039 residual B11, C3-C8, and C-new-1 through C-new-5
  item against current tests.
- Added focused cache freshness coverage, public-membership IP set comparison,
  isolated `t.Parallel()` opt-ins, and missing `t.Helper()` wrappers.
- Converted the cohesive `buildPipelineRunPlan` scenario group to a
  table-driven test.
- Rejected broad mechanical sweeps for `t.Parallel`, `t.Cleanup`, and naming
  churn with evidence.
- Mapped duplicated scheduler polling-helper policy to SOW-0066.

## Validation

Acceptance criteria evidence:

- Reviewed all requested residual items; see the residual outcome map above.
- Focused package validation:
  - `go test ./pkg/web -run 'TestFileCache|TestRawFeedRoutesDoNotEnterArtifactCache'` — passed.
  - `go test ./pkg/web -run 'TestFileCache|TestRawFeedRoutesDoNotEnterArtifactCache|TestPublic|TestAdmin'` — passed.
  - `go test ./pkg/iprange -run TestSetAlgebraProperties` — passed.
  - `go test ./pkg/engine -run TestBuildPipelineRunPlan` — passed.
- Full validation:
  - `make test` — passed.
  - `make lint` — passed.
  - `git diff --check` — passed.
  - `.agents/sow/audit.sh` — passed after moving this completed SOW to
    `.agents/sow/done/`.

Reviewer findings:

- Go behavioral-testing review found SOW-0039 residual C-class and neutral
  items were not represented by concrete pending SOWs.

Artifact maintenance gate:

- AGENTS.md: not updated. Reason: no workflow or project-wide guardrail changed.
- Runtime project skills: not updated. Reason: existing Go behavioral-testing
  guidance already covers the outcomes.
- Specs: not updated. Reason: tests and SOW ledger only; no product behavior
  contract changed.
- End-user/operator docs: not updated. Reason: no operator-facing behavior
  changed.
- End-user/operator skills: not updated. Reason: no portable operator workflow
  changed.
- SOW lifecycle: SOW-0039 follow-up mapping updated; SOW-0066 owns the
  scheduler polling-helper classification.

## Outcome

Completed.

Shipped changes:

- Added same-size, mtime-only file-cache freshness coverage.
- Replaced the IP set property-test equality helper's internal slice
  comparison with public membership checks.
- Added `t.Parallel()` to isolated file-cache/property tests only.
- Added missing `t.Helper()` calls to web handler fixture wrappers.
- Converted `buildPipelineRunPlan` tests to a table-driven scenario matrix.

Lessons extracted:

- Parallelization belongs at proven isolated test boundaries, not as a
  repository-wide mechanical sweep.
- Local `defer` cleanup in a leaf test is acceptable; `t.Cleanup` matters most
  when a helper owns the resource lifecycle.
- Representation-neutral property tests should compare observable membership,
  not storage slices.
