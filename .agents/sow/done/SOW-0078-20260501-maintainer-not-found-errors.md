# SOW-0078 - Maintainer Not-Found Errors

## Status

Status: completed

Sub-state: completed and validated

## Requirements

### Purpose

Make maintainer-detail not-found behavior typed and robust, avoiding brittle
HTTP error mapping based on string matching.

### User Request

Review project quality against the four named project skills, identify gaps,
create SOWs for actionable improvements, and iterate the audit until valid
findings are fixed or tracked.

### Assistant Understanding

Facts:

- Audit cycle 2 found `pkg/engine/home_detail.go` returns a string error for a
  missing maintainer.
- Audit cycle 2 found `pkg/web/home_detail_api.go` maps maintainer not-found
  behavior with `strings.Contains(err.Error(), "not found")`.
- The Go best-practices skill prefers wrapped sentinel or typed errors with
  `errors.Is`/`errors.As`.

Inferences:

- This is a narrow backend/API correctness cleanup with low product-design
  risk.

Unknowns:

- Resolved: no compatible maintainer-detail sentinel existed. A focused scan
  found only the maintainer string-match path in scope.

### Acceptance Criteria

- Introduce or reuse a typed/sentinel not-found error for maintainer detail.
- Map the public API status with `errors.Is`/`errors.As`, not substring checks.
- Remove dead or nonsensical error branches in the touched handler.
- Add/adjust behavioral HTTP tests for maintainer not-found and real backend
  errors.
- Run `go test ./pkg/engine ./pkg/web`.

## Analysis

Sources checked:

- `project-go-best-practices`
- `project-go-behavioral-testing`
- `project-testing`
- `project-coding`
- `pkg/engine/home_detail.go`
- `pkg/web/home_detail_api.go`
- `pkg/web/maintainer_api_test.go`
- Cycle-2 Go best-practices findings

Current state:

- Not-found mapping depends on error text, which is fragile under normal error
  wrapping.
- `pkg/engine/home_detail.go` returns `fmt.Errorf("maintainer not found")`
  when a slug has no matching eligible maintainer.
- `pkg/web/home_detail_api.go` maps any error whose rendered text contains
  `not found` to `404`, and otherwise returns `400`.

Risks:

- Incorrect mapping can turn real backend failures into `404`, or not-found
  cases into `500`.
- Returning `400` for real backend errors is also misleading because the client
  did not necessarily send an invalid slug.

## Pre-Implementation Gate

Status: ready

Problem / root-cause model:

- Maintainer detail has a real not-found condition, but the engine reports it
  as an untyped string error. The web handler then classifies errors by
  substring, so unrelated wrapped errors containing `not found` can become
  `404`, and future wording changes can break legitimate not-found mapping.

Evidence reviewed:

- `pkg/engine/home_detail.go` uses `fmt.Errorf("maintainer not found")`.
- `pkg/web/home_detail_api.go` checks
  `strings.Contains(err.Error(), "not found")`.
- `pkg/web/maintainer_api_test.go` covers successful maintainer index/detail
  responses but not missing maintainer or backend error classification.

Affected contracts and surfaces:

- Public API route `/api/v1/maintainers/{slug}`.
- Engine maintainer-detail error contract.
- Go behavioral tests for the maintainer public API.

Existing patterns to reuse:

- Package-level sentinel errors with `%w` wrapping and `errors.Is`.
- Existing `newHandler` + scheduler test fixture in `pkg/web/maintainer_api_test.go`.

Risk and blast radius:

- Low runtime blast radius: one API handler and one engine error branch.
- The main risk is changing non-not-found backend errors to a more accurate
  `500` instead of the current `400`; this is intentional because backend
  failures are not client input errors.

Implementation plan:

1. Add an exported engine sentinel for missing maintainer detail.
2. Wrap the sentinel at the engine return site while preserving the current
   human-readable error text.
3. Update the web handler to use `errors.Is` and return `500` for other
   backend failures.
4. Add public HTTP tests for missing maintainer and backend error mapping.

Validation plan:

- `go test ./pkg/engine ./pkg/web`
- Focused same-failure scan for `strings.Contains(err.Error(), "not found")`
  in touched maintainer paths.

Artifact impact plan:

- AGENTS.md: no update expected; existing rules already forbid brittle error
  handling.
- Runtime project skills: no update expected; existing Go best-practices rule
  already covers typed/wrapped errors.
- Specs: no product contract change expected; not-found remains `404`.
- End-user/operator docs: no update expected; public API behavior remains the
  same for not-found.
- End-user/operator skills: no update expected.
- SOW lifecycle: move this SOW from `current/` to `done/` after validation.

Open decisions:

- None. This is maintainer-owned correctness cleanup; the existing public API
  contract already implies `404` for a missing maintainer and non-404 for
  backend failures.

## Plan

1. Inspect existing detail not-found error patterns.
2. Add a sentinel or typed error at the engine boundary.
3. Update the web handler to use `errors.Is`/`errors.As`.
4. Add behavioral tests for the public API response.

## Execution Log

### 2026-05-01

- Created from iterative audit cycle 2.
- Moved from `pending/` to `current/` as autonomous maintainer-owned quality
  work.
- Added `engine.ErrMaintainerNotFound`.
- Updated `handleMaintainerDetail` to classify not-found with `errors.Is`.
- Changed non-not-found maintainer-detail backend failures from `400` to `500`.
- Added engine and public HTTP tests for missing maintainer and backend error
  classification.

## Validation

Acceptance criteria evidence:

- `pkg/engine/home_detail.go` exposes `ErrMaintainerNotFound` and returns it
  when no eligible feed matches the maintainer slug.
- `pkg/web/home_detail_api.go` uses
  `errors.Is(err, engine.ErrMaintainerNotFound)`.
- `pkg/web/home_detail_api.go` no longer has the dead `errors.Is(err, nil)`
  branch or the substring-based not-found mapping.
- `pkg/engine/home_detail_test.go` verifies `MaintainerDetail` returns the
  sentinel for missing maintainers.
- `pkg/web/maintainer_api_test.go` verifies missing maintainers return `404`
  and real backend errors return `500`.

Tests or equivalent validation:

- `go test ./pkg/engine ./pkg/web` passed.

Real-use evidence:

- Public API route exercised through `newHandler` in
  `pkg/web/maintainer_api_test.go`.

Reviewer findings:

- Go best-practices review found string-matched maintainer not-found behavior.

Same-failure scan:

- `rg 'strings.Contains\\(err\\.Error\\(\\), "not found"\\)|ErrMaintainerNotFound|maintainer not found' pkg/engine pkg/web`
  found only the new sentinel, typed handler check, and tests.

Artifact maintenance gate:

- AGENTS.md: no update needed; existing rules already require wrapped/typed
  errors.
- Runtime project skills: no update needed; `project-go-best-practices`
  already covers `errors.Is`/`errors.As`.
- Specs: no update needed; missing maintainer remains a public `404`.
- End-user/operator docs: no update needed; documented user behavior did not
  change.
- End-user/operator skills: no update needed.
- SOW lifecycle: completed; moved to `done/`.

Specs update:

- None needed; this was an implementation correctness cleanup.

Project skills update:

- None needed; no new durable workflow lesson.

End-user/operator docs update:

- None needed.

End-user/operator skills update:

- None needed.

Lessons:

- Existing Go best-practices guidance was sufficient.

Follow-up mapping:

- None.

## Outcome

Completed.

## Lessons Extracted

None.

## Followup

None.
