# TODO — implementation/spec gap closure

## Purpose

Bring the current Go implementation into full behavioral alignment with the
authoritative contracts in `specs/*.md`, focusing on:

- downloader ownership and queue behavior
- processing-engine ownership and queue behavior
- feed lifecycle correctness across staged/committed/public states
- scheduler timing and manual-action semantics

The fit-for-purpose target is operational correctness first:

- the downloader must be the exclusive automatic producer of processing work
- the processing engine must consume only admitted feed bodies and must not
  silently lose required work
- operator-visible queues must reflect the real lifecycle without hidden
  shortcuts or dropped events

## TL;DR

Verified gaps to fix:

1. public outputs are published before committed staged feed bodies are
   promoted
2. per-feed processing failures are dropped instead of returning to
   `waiting to be processed`
3. artifact-parent due times are ignored by the fetch-loop sleep calculation
4. merge composition reads staged parent feed bodies instead of committed ones
5. merges do not have their own configurable cadence and silently inherit the
   processing interval
6. manual recheck of a history derivative does not redirect to the parent when
   rollups are missing/corrupt
7. provider-triggered full reprocess waves incorrectly skip hidden feeds

## Analysis

### Verified implementation/spec discrepancies

1. Publication ordering mismatch
   - Spec:
     - `specs/pipeline.md` requires processing to finish, staged feed bodies to
       be promoted, and only then public publication to occur.
   - Implementation facts:
     - `pkg/engine/run.go` starts public publication during `RunOnce()`.
     - `pkg/scheduler/scheduler.go` promotes staged feed bodies only after
       `RunOnce()` returns.
     - `pkg/engine/finalize.go` writes committed per-feed primary outputs during
       processing.
   - Risk:
     - public publication can observe state before downloader-stage commitment
       is complete.

2. Per-feed processing failures are dropped
   - Spec:
     - `specs/pipeline.md` requires failed processing items to return to the
       processing queue with their age preserved.
   - Implementation facts:
     - `pkg/engine/run.go` records per-feed failures in the batch report.
     - `pkg/scheduler/scheduler.go` only requeues on batch-level failure, not on
       per-feed failures inside a successful batch.
   - Risk:
     - processing work can be silently lost after a partial batch failure.

3. Artifact-parent cadence can be missed
   - Spec:
     - downloader scheduling applies to artifact parents as independent
       downloader-owned items.
   - Implementation facts:
     - `pkg/scheduler/scheduler.go` computes automatic artifact due work, but
       the loop sleep calculation only considers normal source snapshot items.
   - Risk:
     - the fetch loop can sleep past an artifact parent due time.

4. Merge composition uses staged parent bodies
   - Spec:
     - `specs/feeds.md` / `specs/pipeline.md` require merge composition to read
       committed source feed bodies, not staged ones.
   - Implementation facts:
     - `pkg/engine/feed_body_stage.go` prefers `.new` over committed parent
       feed bodies during merge composition.
   - Risk:
     - merge downloader output can depend on uncommitted parent state.

5. Merge cadence is not user-configurable
   - Spec:
     - merges are downloader-owned cadence items with their own schedule.
   - Implementation facts:
     - `pkg/config/config.go` merge config lacks `frequency`
     - `pkg/config/expand.go` expands merges with zero cadence
     - `pkg/scheduler/scheduler.go` falls back to
       `runtime.processing_interval_minutes`
   - Risk:
     - config cannot express merge cadence as required by the spec.

6. History-derivative recheck target is wrong when rollups are missing
   - Spec:
     - manual recheck of a derivative with missing/corrupt rollups must target
       the parent.
   - Implementation facts:
     - `pkg/engine/download_stage.go` only redirects artifact-backed children
       during recheck target resolution.
     - derivative composition fails locally when rollups are missing.
   - Risk:
     - operators hit a downloader-stage failure instead of automatic parent
       recovery.

7. Hidden feeds are skipped from provider-triggered full reprocess targets
   - Spec:
     - hidden affects public visibility, not participation in processing or
       provider-dependent derived outputs.
   - Implementation facts:
     - `pkg/engine/download_stage.go` excludes hidden feeds from full reprocess
       targets after provider updates.
   - Risk:
     - hidden feeds keep stale ASN/GEO/provider-derived artifacts.

### Scope boundaries

- This TODO covers only the verified gaps above.
- It does not reopen the full pipeline redesign or the full specs rewrite.
- If code changes require a spec clarification, the relevant `specs/*.md` file
  must be updated in the same change set.

## Decisions

Already decided by Costa:

1. Close the verified gaps against the specs.
2. The downloader and processing engine must remain strongly separated.
3. Automatic processing input must come only from downloader outcomes and
   integrity-triggered engine recovery; direct processing is otherwise an admin
   action.

Implied execution decisions for this task:

1. Use the verified local code evidence as the implementation target, not the
   lower-confidence reviewer claims that were not confirmed.
2. Fix correctness gaps before optimization or cleanup.
3. Add or update tests for each closed gap where the behavior is observable in
   unit/integration tests.

## Plan

1. Fix publication ordering so downloader-stage commitment completes before
   public publication.
2. Fix processing-queue requeue behavior for per-feed failures.
3. Fix artifact-parent scheduler wake calculations.
4. Fix merge composition to read committed parent feed bodies only.
5. Add merge cadence to config, expansion, validation, scheduling, and tests.
6. Fix derivative recheck target resolution for missing/corrupt rollups.
7. Include hidden feeds in provider-triggered full reprocess target sets.
8. Run focused Go tests first, then broader package test coverage.
9. Update specs only if any implementation detail reveals an actual contract
   ambiguity.

## Implied decisions

- Preserve current operator-visible queue names and admin API shapes unless a
  code fix strictly requires an already-specified field update.
- Prefer local, minimal structural changes over broad refactors while closing
  these gaps.
- Treat staged feed bodies as authoritative for reprocess only where the specs
  already allow it; otherwise keep committed/staged boundaries strict.

## Testing requirements

- Add or update tests for:
  - publication after committed feed-body promotion
  - processing failure requeue behavior
  - artifact-parent next-wake scheduling
  - merge cadence config propagation
  - derivative recheck redirection
  - hidden-feed inclusion in provider reprocess target sets
- Run targeted package tests for:
  - `pkg/scheduler`
  - `pkg/engine`
  - `pkg/config`
- Run a wider `go test ./...` pass if the targeted tests are green and the
  touched areas are stable.

## Documentation updates required

- Recheck whether any touched behavior requires edits to:
  - `specs/pipeline.md`
  - `specs/config.md`
  - `specs/feeds.md`
- If no contract changes are needed, no spec edits should be made just to mirror
  code-level refactors.

## Status — 2026-04-21

Implemented:

1. Publication ordering
   - processing now invokes a scheduler-supplied pre-publication hook so staged
     downloader outputs are promoted before public publication begins
   - files:
     - `pkg/engine/engine.go`
     - `pkg/engine/run.go`
     - `pkg/scheduler/scheduler.go`
     - test helpers under `pkg/{engine,scheduler,web}/`

2. Per-feed processing failure requeue
   - successful items are completed normally
   - failed items are returned to `waiting to be processed` with queue age
     preserved
   - retry logging is emitted for the failed items
   - files:
     - `pkg/scheduler/scheduler.go`
     - `pkg/scheduler/scheduler_test.go`

3. Artifact-parent wake timing
   - fetch-loop sleep calculation now considers artifact due times alongside
     normal source due times
   - files:
     - `pkg/scheduler/scheduler.go`
     - `pkg/scheduler/scheduler_test.go`

4. Merge committed-input boundary
   - merge composition now reads committed parent feed bodies only
   - file:
     - `pkg/engine/feed_body_stage.go`

5. Merge cadence configuration
   - `merges:` now support explicit `frequency`
   - expanded merge sources carry that cadence
   - scheduler no longer silently treats merges as `processing_interval`
   - current catalog merges were updated to declare `frequency: 5`
   - files:
     - `pkg/config/config.go`
     - `pkg/config/expand.go`
     - `pkg/config/validate.go`
     - `pkg/config/config_test.go`
     - `configs/firehol.yaml`
     - `specs/config.md`

6. Derivative recheck target recovery
   - manual recheck of a history derivative now falls back through the parent
     chain when local derivative recomposition prerequisites are broken
   - file:
     - `pkg/engine/download_stage.go`

7. Hidden-feed provider reprocess coverage
   - provider-triggered full reprocess target collection now includes hidden
     feeds
   - file:
     - `pkg/engine/download_stage.go`

Verification:

- `go test ./pkg/config`
- `go test ./pkg/engine`
- `go test ./pkg/scheduler`
- `go test ./pkg/web`
- `go test ./...`
