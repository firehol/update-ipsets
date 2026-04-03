# SOW-0075 - Go Test Context Modernization

## Status

Status: completed

Sub-state: implemented and validated

## Requirements

### Purpose

Align Go tests with the project Go behavioral-testing skill by using test-owned
contexts where cancellation or lifecycle ownership matters.

### User Request

Review project quality against the four named project skills, identify gaps,
create SOWs for actionable improvements, and iterate the audit until valid
findings are fixed or tracked.

### Assistant Understanding

Facts:

- The Go behavioral-testing skill recommends `t.Context()` for test-scoped
  operations.
- SOW-0039 recorded low adoption of `t.Context()` but did not map the remaining
  work to a pending SOW.
- Audit cycle 2 found new or nearby uses of `context.Background()` in Go tests
  where a test-owned context is more appropriate.

Inferences:

- This is mostly test-hygiene work, but cancellation tests and background
  repair tests can hide ownership bugs when they use detached contexts.

Unknowns:

- Which `context.Background()` uses are intentional roots for constructing a
  cancellable test context and should remain.

### Acceptance Criteria

- Inventory Go test uses of `context.Background()` and classify each as
  replaceable, intentional root, or not applicable.
- Replace lifecycle/cancellation/background-work test calls with `t.Context()`
  where doing so preserves test intent.
- Keep explicit `context.WithCancel(context.Background())` only when the test
  needs an independent cancel root, and record the rationale.
- Run affected package tests, `make test-strict`, and `make race` or an
  explicit package-level `go test -race ...` gate for touched concurrent
  lifecycle/cancellation packages.

## Analysis

Sources checked:

- `project-go-behavioral-testing`
- `project-testing`
- SOW-0039 audit notes
- Cycle-2 reviewer findings

Current state:

- Some tests still use detached background contexts for engine operations.

Risks:

- Detached contexts can let background work survive past the test lifecycle.
- A mechanical replacement can break tests that intentionally own a separate
  cancel root.

## Plan

1. Search Go tests for `context.Background()`.
2. Classify and patch focused replaceable uses.
3. Add or update cancellation assertions where the replacement exposes missing
   lifecycle behavior.
4. Run focused package tests and `make test-strict`.
5. Run `make race` or an explicit package-level race gate for touched
   concurrent lifecycle/cancellation packages.

## Execution Log

### 2026-05-01

- Created from iterative audit cycle 2.
- Moved from pending to current for autonomous test-hygiene cleanup.
- Inventoried Go test uses of `context.Background()` under `cmd/`,
  `internal/`, `pkg/`, and `tools/`.
- Replaced test-owned lifecycle contexts with `t.Context()` or `b.Context()`
  where the surrounding test/benchmark provides a testing context.
- Updated scenario helper methods that store `*testing.T` to use
  `s.t.Context()` instead of detached background contexts.
- Removed now-unused `context` imports from test files.
- Re-ran the inventory and found zero remaining `context.Background()` uses in
  Go test files.

## Validation

Acceptance criteria evidence:

- Inventory covered `cmd`, `internal`, `pkg`, and `tools` Go test files with
  `rg -n "context\\.Background\\(\\)" --glob '*_test.go' cmd internal pkg tools`.
- Replaced all matching detached test contexts. Follow-up inventory result:
  zero `context.Background()` matches in Go test files.
- 47 Go test files now contain `t.Context()` or `b.Context()` calls, covering
  236 test-owned context call sites.
- No intentional independent `context.Background()` roots remain in Go test
  files after review.

Tests or equivalent validation:

- `go test ./pkg/downloader ./pkg/processor ./pkg/iprange ./pkg/engine ./pkg/scheduler ./pkg/web` passed.
- `cd tools/dronebl2ipsets && go test ./...` passed.
- `make test-strict` passed.
- `make race` passed, including the nested `tools/dronebl2ipsets` race gate.

Reviewer findings:

- Go behavioral-testing review found the `t.Context()` adoption gap was not
  mapped to pending work.

Follow-up mapping:

- Tracks the unmapped SOW-0039 `t.Context()` adoption finding.
- No remaining `context.Background()` test-context adoption item is left
  unmapped.

Artifact maintenance gate:

- AGENTS.md: not needed; no workflow or project-wide rule changed.
- Runtime project skills: not needed; `project-go-behavioral-testing` already
  requires `t.Context()` for test-owned lifecycle operations.
- Specs: not needed; no product, runtime, or API behavior changed.
- End-user/operator docs: not needed; test-only hygiene change.
- End-user/operator skills: not needed; no exported operator workflow changed.
- SOW lifecycle: moved from pending to current, completed here, then moved to
  done.

## Outcome

Completed. Go tests now use test-owned contexts consistently for the audited
surface, and the strict/race validation gates passed.
