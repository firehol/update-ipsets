# SOW-0048 | 2026-05-01 | lifecycle-leak-guards

## Status

completed

## Requirements

### Purpose

Add lifecycle/goroutine leak guards for long-running scheduler, engine, and web
work without turning tests into implementation-coupled goroutine-count checks.

### User request quoted verbatim

> deferred items from an SOW does not mean "let's do it later". It means "I
> want to be focused for this, let's do it immediately after alone".

### Assistant understanding

- SOW-0035 and SOW-0042 improved cancellation ownership, but SOW-0039 found
  that tests still mostly prove caller return, not full child-work shutdown.
- SOW-0039 rejected adding a new leak dependency mechanically. This SOW should
  first evaluate project-native lifecycle signals and only add a dependency if
  evidence shows it is the simplest robust guard.
- The target is externally meaningful shutdown behavior: no queued work keeps
  running after cancellation, and long-running owners report settled state.

### Acceptance criteria

- Inventory long-running test surfaces: scheduler runner, engine heavy phases,
  daemon/admin-triggered background work, and web server lifecycle.
- Define observable shutdown contracts per surface.
- Add tests that cancel owners, wait through public/status contracts, and
  assert settled state without relying on arbitrary sleeps.
- Evaluate whether a package-level leak checker is justified; record the
  evidence and decision in the SOW before adding any dependency.
- Validation includes relevant package tests, `make test`, `make test-strict`,
  `make race`, and blocking analysis gates.

## Analysis

- Source SOW: SOW-0039.
- Finding class: SOW-0032 A5/B9 and SOW-0039 B-new-1.
- This was separated from engine/web fixture migration because lifecycle
  contracts cut across scheduler, engine, daemon, and web.
- Inventory:
  - Scheduler runner cancellation lived in `pkg/scheduler/cancel_test.go`.
    Existing tests cancelled the context and waited for `Runner.Run` to return,
    but they did not assert the public activity snapshot was settled afterward.
  - Engine bounded worker and writer cancellation already had focused
    `testing/synctest` coverage in `pkg/engine/concurrency_test.go` and
    `pkg/engine/output_cancellation_test.go`. Those tests assert
    `context.Canceled` and no partial output publication for the heavy-phase
    helpers they touch.
  - Web daemon lifecycle tests lived inside the large
    `pkg/web/feature_test.go` route test file. They asserted request behavior
    while the daemon was running and that `Run` returned after cancellation,
    but not that listeners were actually closed after `Run` returned.
  - Admin entity-integrity background-task suppression is already covered by
    `pkg/web/integrity_test.go`; no additional lifecycle state was needed for
    this SOW.
- Dependency decision:
  - A package-level goroutine leak checker was not added. The robust contract
    here is observable owner state: scheduler active queues empty after
    `Runner.Run` returns, and web listeners closed after `Run` returns.
  - Goroutine-count assertions would be implementation-coupled and noisy around
    Go HTTP internals. The existing `go.sum` contains an old `goleak` checksum,
    but the main module does not depend on it through `go.mod`, and this SOW
    does not add or revive that dependency.
- Architecture posture finding:
  - Adding web lifecycle assertions directly to `feature_test.go` initially
    made `tools/archposture` fail because the file had grown beyond its
    baseline. The fix was to move daemon lifecycle tests and their TCP/TLS
    helpers into `pkg/web/run_lifecycle_test.go`, which reduced
    `feature_test.go` to 988 lines.

## Plan

- Add project-native lifecycle assertions to scheduler cancellation tests:
  cancel, wait for `Runner.Run`, then assert `ActivitySnapshot()` has no active
  downloads or processing.
- Add web daemon lifecycle assertions:
  cancel the daemon context, wait for `Run` to return, then prove the relevant
  listener endpoints are closed by attempting real HTTP requests.
- Keep web daemon lifecycle tests in a focused file rather than expanding the
  broad feature-route test file.
- Run focused package tests plus the full Go validation gate set.

## Validation

- `go test ./pkg/scheduler ./pkg/web` — passed.
- `make test` — passed, including `tools/archposture`.
- `make test-strict` — passed.
- `make fuzz-replay` — passed.
- `make lint` — passed.
- `make staticcheck` — passed.
- `make golangci-lint` — passed.
- `make vulncheck` — passed, no vulnerabilities found.
- `make race` — passed.
- `git diff --check` — passed.
- Same-failure scan:
  - `rg -n "time\\.Sleep" pkg/scheduler/cancel_test.go pkg/web/run_lifecycle_test.go pkg/web/feature_test.go` returned no matches.
  - `wc -l` confirmed `pkg/web/feature_test.go` is 988 lines after moving
    daemon lifecycle tests to `pkg/web/run_lifecycle_test.go`.
- Specs update: N/A. This SOW changes test coverage and project testing
  discipline, not product behavior, configuration semantics, file layout,
  public/admin API behavior, or runtime contracts.
- Skill update: `.agents/skills/project-testing/SKILL.md` updated with the
  lifecycle guard pattern.

## Outcome

- Scheduler cancellation tests now verify both sides of shutdown ownership:
  `Runner.Run` returns and `ActivitySnapshot()` has no active download or
  processing work.
- Web daemon lifecycle tests now verify HTTPS and split public/admin listeners
  close after `Run` returns.
- Web daemon lifecycle coverage lives in `pkg/web/run_lifecycle_test.go`;
  `pkg/web/feature_test.go` no longer owns daemon lifecycle helpers.

## Lessons extracted

- Lifecycle tests should assert externally meaningful settled state after the
  owner returns. For this codebase, that means public snapshots, background task
  lists, artifacts, or listener closure rather than package-wide goroutine
  counts.
- Architecture posture failures are useful design feedback. When a broad test
  file grows, move the behavior into a focused test surface before considering
  baseline changes.
