# SOW-0035 | 2026-04-30 | go-concurrency-cancellation-hardening

## Status

completed

Promoted before SOW-0032 because this is a direct SOW-0031 codebase-design
follow-up. The testing SOWs remain pending until the SOW-0031 design leftovers
are complete.

## Requirements

### Purpose

Follow up the valid SOW-0031 deep-concurrency findings with a focused Go
hardening pass that improves context propagation, fan-out cancellation, error
aggregation, and goroutine-leak protection without mixing this work into
unrelated code-quality changes.

### Source

Created from the SOW-0031 follow-up/rejection ledger correction. SOW-0031
implemented the bounded low/medium-risk findings and explicitly tracked these
remaining valid findings instead of leaving them as prose-only deferrals.

### Acceptance criteria

- Current fan-out and context-boundary paths are inventoried before code
  changes.
- Valid conversions to `errgroup` or equivalent structured cancellation are
  implemented only where the public/engine behavior and validation path are
  clear.
- Selected work cancelled before completion remains operator-visible where the
  pipeline contract requires it.
- Tests cover cancellation behavior and do not assert private implementation
  details.
- Race validation and the relevant package tests pass.
- Any remaining non-implemented item is rejected with evidence or moved to a
  concrete pending SOW path before this SOW closes.

## Analysis

Seed findings from SOW-0031:

- A2/A3/A6 broad context propagation and errgroup conversion are valid but
  cross-package and high-blast-radius.
- The work touches long-running downloader, engine, scheduler, and background
  paths, so it needs a dedicated cancellation test plan instead of a quick
  modernization sweep.

Inventory findings:

- Engine `RunOnce` already had a top-level context and source processing used
  it, but global heavy fan-out writers did not accept a context. A cancelled
  run could continue admitting geo/ASN/bogon/critical/comparison/entity jobs
  until those phase-local queues drained.
- `processGeoIPDatabases` and `processASNDatabases` accepted context but did
  not check it between provider sources; ASN provider open failures after a
  previous successful open also needed cleanup before returning.
- Scheduler staged recovery was launched as an untracked goroutine and the
  staged artifact recovery path dropped context before materializing children.
- `Runner.Run` returned as soon as its parent context was cancelled, without
  waiting for the fetch loop, processing loop, staged recovery, or download
  workers it had started. Strict shuffled tests exposed this as a race between
  scheduler shutdown and temporary cache directory cleanup.
- Downloader canonical parsing accepted context at the outer
  `PrepareCanonicalFeedBody` boundary, then parsed processed files through
  `context.Background()`, which could ignore cancellation during hostname/DNS
  parsing.
- `pkg/iprange.ResolveHostnames6` differed from the IPv4 implementation: it
  started worker goroutines without a wait/close lifecycle for the results
  channel. This was not an observed persistent leak, but it was a real lifecycle
  inconsistency in a standalone package.
- Remaining `context.Background()` uses are not all bugs:
  - CLI entrypoints create root command contexts.
  - observability helpers use background only as a nil-context fallback.
  - shutdown timeouts intentionally use bounded background contexts after the
    parent has already been cancelled.
  - local stats/snapshot readers without a caller context are non-goals for
    this SOW unless they appear in a long-running cancellable path.
- Entity artifact repair/rebuild public methods still do not expose caller
  context. The sidecar builder now accepts context for run-bound work, but the
  standalone repair/rebuild API cancellation contract is a separate API-design
  question, not an untracked implementation TODO in this SOW.

## Plan

- Inventory context and goroutine fan-out paths in `pkg/engine`,
  `pkg/scheduler`, `pkg/downloader`, and `cmd/update-ipsets`.
- Separate correctness-impacting cancellation holes from cosmetic idiom
  changes.
- Implement the smallest behavior-preserving structured-concurrency changes
  that improve cancellation, error propagation, and operator visibility.
- Add targeted tests and run race validation.

## Validation

- Acceptance criteria evidence:
  - Current fan-out/context boundaries were inventoried in the Analysis section.
  - Heavy engine fan-out now accepts caller context for geo, ASN, bogon,
    critical-infrastructure, comparison, metadata, and foreground entity
    sidecar generation.
  - Cancelled runs now return cancellation before publishing partial staged
    heavy-phase batches.
- Scheduler staged recovery now receives the runner context.
- `Runner.Run` now owns child loop/download goroutines with a wait group and
  waits for them to settle after cancellation before returning.
- Downloader canonical parsing now propagates context into processed/canonical
  file parsing.
  - Remaining non-implemented context roots are recorded as explicit non-goals
    with evidence above.
- Targeted tests:
  - `go test ./pkg/scheduler ./pkg/engine ./pkg/downloader ./pkg/processor ./pkg/iprange`
- Full gates:
  - `make test`
  - `make race`
  - `make lint`
  - `make build`
  - `go test ./tools/archposture`
  - `git diff --check`
- Same-failure scan:
  - Searched `context.Background()`, `go func`, `sync.WaitGroup`,
    `RecoverStagedArtifacts`, and canonical parse call paths in `cmd/`,
    `pkg/engine`, `pkg/downloader`, `pkg/scheduler`, and `pkg/iprange`.
  - Remaining `context.Background()` uses are either root contexts, test
    contexts, bounded shutdown contexts, nil-context fallbacks, local snapshot
    readers, or the standalone entity repair API boundary recorded above.
- Specs updated:
  - `.agents/sow/specs/pipeline.md`
  - `.agents/sow/specs/operating-principles.md`
- Skills updated:
  - `.agents/skills/project-coding/SKILL.md`
  - `.agents/skills/project-testing/SKILL.md`

## Outcome

Completed.

The engine heavy phases now use caller context for bounded fan-out and stop
admitting new jobs after cancellation. The scheduler recovery path preserves
runner context, downloader parsing no longer drops context at canonical parse
boundaries, and IPv6 hostname resolution now closes its worker result stream
like the IPv4 implementation. Tests and project memory now cover the
cancellation contract.

## Execution log

- Added `pkg/engine/concurrency.go` with a bounded name-job runner that stops
  admitting new jobs after context cancellation and waits for worker shutdown.
- Threaded run context through heavy phase writers:
  `writeCountryComparisonFiles`, `writeBogonComparisonFiles`,
  `writeASNComparisonFiles`, `writeCriticalInfrastructureFiles`,
  `writeComparisonFiles`, metadata comparison writing, and foreground entity
  sidecar staging.
- Added context checks between heavy phases and before metadata/insight work so
  a cancelled run returns cancellation instead of publishing partial staged
  artifacts.
- Threaded context through downloader canonical parsing and engine feed-body
  composition helpers so DNS-capable parsing does not drop caller cancellation.
- Threaded scheduler run context into staged artifact recovery and
  `RecoverStagedArtifacts`.
- Added structured scheduler runner shutdown: the runner now cancels its child
  context and waits for fetch, processing, recovery, and in-flight download
  workers before returning from `Run`.
- Hardened scheduler cancellation tests so each test cancels and waits for the
  runner goroutine before `t.TempDir` cleanup can remove cache/staging paths.
- Fixed `ResolveHostnames6` to mirror IPv4 worker lifecycle: wait for workers,
  close results, and range results.
- Added focused cancellation tests:
  - bounded worker helper stops scheduling new jobs after cancellation
  - comparison writer returns `context.Canceled` and writes no output when
    cancelled before admission
- Updated `.agents/sow/specs/pipeline.md`,
  `.agents/sow/specs/operating-principles.md`, `project-coding`, and
  `project-testing` with the heavy-phase cancellation contract.
- Split new entity sidecar and output test logic into focused files instead of
  weakening the architecture posture baseline.

## Lessons extracted

- Engine heavy-phase work should expose cancellation as behavior, not just as
  Go idiom cleanup. The durable rule belongs in `project-coding`; the regression
  tests belong in `project-testing`.
- Scheduler lifecycle tests must wait for the runner goroutine to stop, not only
  signal cancellation. Otherwise true shutdown ownership bugs appear as
  timing-sensitive TempDir cleanup failures.
- Architecture posture failures are not noise. If a change grows an already
  large file, split by behavior before considering any baseline update.
- A context-aware helper does not mean every caller has a cancellation contract.
  API boundaries without caller context must be documented as non-goals or
  opened as their own SOW, not hidden as prose-only deferrals.
