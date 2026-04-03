# SOW-0065 - Web HTTP Behavior Test Migration Phase 2

## Status

Status: completed

Sub-state: validated

## Requirements

### Purpose

Continue moving broad web/API tests through the real HTTP server harness so route registration, middleware, auth, CORS, gzip, and headers are tested together.

### User Request

Review project quality against the four named project skills, identify gaps, and create SOWs for actionable improvements.

### Assistant Understanding

Facts:

- SOW-0047 added `webHTTPTestServer` and migrated a first broad set of route tests.
- SOW-0047 intentionally left direct recorder tests where they were unit-level or outside the selected first wave.
- The current review found broad API tests still using `httptest.NewRecorder`/direct handler calls and some JSON substring assertions.

Inferences:

- A second phase should not chase the raw direct-recorder count mechanically.
- The highest-value targets are broad public/admin API tests and JSON contract tests that still bypass the full HTTP stack.

Unknowns:

- None for the selected second wave. Remaining direct tests are classified
  below as focused unit/path-safety/middleware tests or broad legacy route
  suites outside this SOW's selected migration target.

### Acceptance Criteria

- Re-inventory remaining direct recorder/handler tests in `pkg/web`.
- Classify direct tests as intentional unit tests or full-server migration targets.
- Migrate a focused second wave, including categories, maintainer/client-IP, methodology, or other public API tests where full HTTP behavior matters.
- Replace raw JSON substring checks with decoded structured assertions in
  selected migration targets and explicitly classify any remaining raw JSON
  substring checks.
- Include the full-HTTP lifecycle admin-status assertion in
  `pkg/web/run_lifecycle_test.go`; it currently checks
  `public_base_url` by substring and must be parsed or explicitly justified.
- Preserve direct tests for middleware, gzip, file-cache units, and focused path-safety checks where justified.

## Analysis

Sources checked:

- SOW-0047.
- `pkg/web/*_test.go` representative direct route tests.

Current state:

- Broad route test migration has started but is incomplete.

Risks:

- Direct handler tests can pass while real route wiring, middleware, auth, CORS, or gzip behavior breaks.
- A broad mechanical rewrite can obscure test intent and create fragile fixtures.

## Implications And Decisions

User delegated implementation-quality, cleanup, testing, and audit SOWs that do
not require product direction. This SOW is classified as assistant-owned because
it improves test coverage style without changing runtime behavior.

Decision:

1. Migration scope
   - A. Convert every remaining direct recorder test.
     - Pros: uniform style.
     - Cons: wastes effort and harms focused unit tests.
   - B. Convert only broad route/API contract tests. Selected.
     - Pros: improves behavioral coverage without losing unit precision.
     - Cons: direct recorder count remains nonzero by design.
   - C. Stop after SOW-0047.
     - Pros: no churn.
     - Cons: leaves known route-contract gaps.

## Plan

1. Inventory and classify remaining direct web tests.
2. Select the highest-value broad tests for migration.
3. Use `webHTTPTestServer` for real HTTP requests.
4. Decode JSON into minimal structs/maps in touched tests.
5. Run `go test ./pkg/web`, strict tests, race, and lint gates.

## Execution Log

### 2026-05-01

- Created from the four-skill quality review.
- Moved to current as assistant-owned test-quality work.
- Re-inventoried `pkg/web` direct recorder/handler tests with `rg`.
- Migrated a focused second wave of broad route/API contract tests to
  `webHTTPTestServer`:
  - `pkg/web/categories_api_test.go`
  - `pkg/web/maintainer_api_test.go`
  - `pkg/web/methodology_test.go`
  - `pkg/web/client_ip_api_test.go`
- Replaced raw JSON substring checks in the migrated tests with decoded
  structured assertions.
- Replaced the `pkg/web/run_lifecycle_test.go` admin status
  `public_base_url` substring assertion with structured JSON decoding.
- Remaining direct tests are classified:
  - Intentional unit/focused tests: cache, middleware, gzip, direct artifact
    safety, integrity handler units, and path-safety checks.
  - Larger broad suites left unchanged in this phase: `feature_test.go`,
    `admin_unification_test.go`, home search/entity tests, and country route
    tests. They already have high coverage and need separate focused migration
    if changed again.

## Validation

Acceptance criteria evidence:

- Direct test inventory repeated.
- Categories, maintainer, methodology, and client-IP route tests now use the
  full HTTP server harness.
- Selected JSON substring checks were replaced by decoded structured
  assertions.
- `pkg/web/run_lifecycle_test.go` now decodes admin status JSON for
  `public_base_url`.
- Remaining direct tests are classified as focused unit tests or broad legacy
  suites outside this selected second wave.

Tests or equivalent validation:

- `go test ./pkg/web -run 'TestCategoriesAndProvenanceAPI|TestMaintainerAPIEndpoints|TestMaintainerDetailBackendErrorIsNotMappedToNotFound|TestMethodology|TestClientIPAPIEndpointReturnsForwardedIPv4|TestRunServesSplitAdminOnSeparateListeners'`
- `go test ./pkg/web`
- `make test`
- `make lint`
- `git diff --check`

Real-use evidence:

- Migrated tests now exercise route registration, middleware stack, and real
  HTTP client/server behavior instead of direct handler calls.

Reviewer findings:

- Go behavioral-testing review found broad web/API tests still bypass real HTTP server behavior.
- Iterative audit cycle 5 found `pkg/web/run_lifecycle_test.go` still has a
  raw JSON substring assertion outside the concrete migration inventory.

Same-failure scan:

- Repeated the direct recorder/handler inventory with `rg -n
  "httptest\\.NewRecorder|httptest\\.NewRequest|ServeHTTP\\(|handle[A-Za-z0-9_]+\\("
  pkg/web -g '*_test.go'`.
- Verified the touched files no longer use direct recorders or direct handler
  calls.

Artifact maintenance gate:

- AGENTS.md: no update needed; existing web test guidance covers this.
- Runtime project skills: no update needed; existing project-testing web HTTP
  fixture guidance already covers this pattern.
- Specs: no update needed; no product behavior changed.
- End-user/operator docs: no update needed; no user/operator behavior changed.
- End-user/operator skills: no update needed.
- SOW lifecycle: current SOW completed and ready to move to done.

Specs update:

- Not needed.

Project skills update:

- Not needed.

End-user/operator docs update:

- Not needed.

End-user/operator skills update:

- Not needed.

Lessons:

- Use `webHTTPTestServer` for broad route/API contract tests and decode JSON
  into minimal structs or existing payload types.
- Keep direct recorder tests for focused middleware, gzip, cache, path-safety,
  and small handler units where a full server adds no contract value.

Follow-up mapping:

- No unmapped item remains in this SOW. Large legacy broad suites are
  classified as outside this selected second wave, not remaining work from this
  SOW.

## Outcome

Completed. The selected web/API tests now use the full HTTP harness and
structured JSON assertions, and validation passed.

## Lessons Extracted

Broad web/API tests should use the real HTTP harness when route registration,
middleware, auth, CORS, gzip, or headers matter. Direct recorder tests remain
valid for focused units where server behavior is not part of the contract.

## Followup

None.
