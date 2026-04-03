# SOW-0047 | 2026-05-01 | web-http-test-fixture-migration

## Status

completed

## Requirements

### Purpose

Make web/API tests more behavioral by using shared HTTP server fixtures and
structured response assertions instead of direct handler-recorder patterns and
raw JSON substring checks.

### User request quoted verbatim

> deferred items from an SOW does not mean "let's do it later". It means "I
> want to be focused for this, let's do it immediately after alone".

### Assistant understanding

- SOW-0039 fixed selected high-risk JSON assertions and added one top-level
  `WebDir` serving test, but did not migrate the broader web suite.
- The remaining work is large enough to deserve a dedicated pass: many web
  tests still call handlers directly and several response assertions still
  inspect rendered strings where a structured contract exists.
- The goal is to make web tests fail on broken public behavior, not on route
  implementation rearrangements.

### Acceptance criteria

- Inventory direct `httptest.NewRecorder`/`handler.ServeHTTP` web tests and
  classify which should remain direct handler tests versus full HTTP server
  tests.
- Add shared web test fixtures that build installed-like public/admin servers
  with explicit `WebDir`, auth mode, cache limits, and runtime paths.
- Migrate high-value route tests to full HTTP behavior, especially raw-body,
  public artifact, admin auth, cache, and rate-limit behavior.
- Replace remaining JSON substring assertions in touched web tests with
  structured decoding.
- Validation includes `go test ./pkg/web`, `make test`, `make test-strict`,
  `make race`, and blocking analysis gates.

## Analysis

- Source SOW: SOW-0039.
- Finding class: SOW-0032 A3/B6/A-new-3/A-new-5 residuals.
- SOW-0039 implemented a small safe slice but intentionally avoided a broad
  migration inside the same change.
- Fresh direct-handler inventory at SOW start:
  - 249 direct recorder/handler patterns across `pkg/web/*_test.go`.
  - The broad count includes intentionally direct middleware, gzip, file-cache,
    and single-handler path-safety tests.
- Classification:
  - Migrate to full HTTP server fixtures: broad route-surface tests, public
    artifact serving, cache/raw-route interaction, and handler-mode/admin auth
    surfaces.
  - Keep direct handler tests: low-level middleware behavior, gzip wrapping,
    file-cache `ServeFile` unit behavior, and focused handler functions where
    a network server adds no useful contract.
- Scope selected for this SOW: migrate the high-value broad route tests in
  `server_test.go`, `routes_test.go`, and the raw-route/cache interaction in
  `cache_test.go`; do not mechanically rewrite the large feature matrix.

## Plan

1. Add a shared `httptest.Server` helper for web tests.
2. Migrate broad full-stack route tests to use real HTTP requests.
3. Replace JSON substring checks in touched route tests with structured JSON
   decoding.
4. Preserve direct handler tests where they are intentionally unit-level.
5. Run full Go validation and update project testing guidance.

## Execution log

- Added `webHTTPTestServer` with helper methods for real HTTP requests and
  response decoding.
- Moved `TestAPIEndpointsAndCORS` to the server fixture and replaced feed,
  search, insights, and topology JSON checks with structured decoding.
- Moved `TestTopLevelArtifactsAreServedFromConfiguredWebDir` to the server
  fixture.
- Moved `TestSurfaceHandlerModesRegisterExpectedSurfaces` to server-backed
  shared/public/admin handlers, preserving the auth checks.
- Moved `TestRawFeedRoutesDoNotEnterArtifactCache` to the server fixture.
- Direct recorder/handler patterns in the three touched files dropped from 41
  to 1. The remaining one is the `fileCache.ServeFile` unit helper, which is
  intentionally not an HTTP server test.
- Total direct recorder/handler patterns across `pkg/web` dropped from 249 to
  209.

## Validation

- `go test ./pkg/web` - passed.
- `make test` - passed.
- `make test-tools` - passed.
- `make test-strict` - passed.
- `make fuzz-replay` - passed.
- `make lint` - passed.
- `make staticcheck` - initially failed on an unused response body assignment;
  passed after cleanup.
- `make golangci-lint` - initially failed on unchecked `resp.Body.Close`;
  passed after cleanup.
- `make vulncheck` - passed.
- `make race` - passed.
- `git diff --check` - passed.
- Product specs: not updated. Reason: this SOW changes test fixture shape and
  test assertions only; no runtime behavior, API, UI, file layout, or operator
  contract changed.
- Project skills: `project-testing` updated with the web HTTP server fixture
  rule and direct-handler exception boundary.

## Outcome

Completed.

Shipped changes:

- Added `webHTTPTestServer` for real HTTP behavior tests.
- Migrated broad route, handler-mode, top-level artifact, and raw-route cache
  tests to use server-backed requests.
- Replaced raw JSON substring assertions in touched route tests with structured
  decoding where JSON was the contract.
- Kept direct recorder tests for intentionally unit-level file-cache behavior.

## Lessons extracted

- A direct recorder count is not a quality metric by itself. Middleware,
  gzip, file-cache, and single-handler path-safety tests are valid direct unit
  tests.
- Broad route tests should go through the real HTTP stack so auth, middleware,
  CORS, gzip, route registration, and serving paths are tested together.
- When migrating raw string assertions, preserve the original behavioral
  intent. The public catalog and search APIs legitimately include retention
  derivatives, so structured assertions should check for required entries, not
  accidental singleton counts.
