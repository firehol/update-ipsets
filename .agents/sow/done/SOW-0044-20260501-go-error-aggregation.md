# SOW-0044 | 2026-05-01 | go-error-aggregation

## Status

Status: completed

Sub-state: completed and validated after reopening

## Requirements

### Purpose

Implement SOW-0038 Decision 3(c): improve multi-feed/heavy-phase operator
triage by aggregating worker errors where the current code reports only the
first failure.

### User request quoted verbatim

> Reviewers have created SOWs 38-41 as a follow up work on 31-34.

### Assistant understanding

- SOW-0043 is complete. This SOW was previously current, was reopened by the
  iterative audit, and is now pending focused repair.
- Focus on direct operator value: when bounded heavy/background fan-out has
  multiple in-flight failures, callers/operators should see all failures, not
  only whichever one won the race to `firstErr`.
- Preserve the existing cancellation policy: the first failure still stops
  admission of new work, but already-running workers can report their own
  failures.
- Do not change pairwise-comparison success policy in this SOW. That path
  intentionally logs/skips missing/open failures today; converting skipped
  comparisons into fatal run failures is a separate behavioral decision.

### Acceptance criteria

- Multi-worker failures preserve enough detail for operator triage.
- Existing single-error behavior remains understandable.
- Tests cover multiple simultaneous worker failures.
- Public/admin error surfaces remain stable or are intentionally improved with
  documentation.
- Reopened acceptance criterion: preserve the underlying unreadable-sidecar
  error chain in `pkg/engine/entity_surgical.go` when returning
  `errEntitySurgicalNeedsFullRebuild`; use wrapping/joining that keeps
  `errors.Is`/`errors.As` useful for both the full-rebuild sentinel and the
  source error.

## Analysis

Facts:

- SOW-0038 recorded B2: `runBoundedNameJobs` returns only `firstErr`, and
  entity sidecar fan-outs do the same.
- Current code evidence:
  - `pkg/engine/concurrency.go` stores only `firstErr` for
    `runBoundedNameJobs`.
  - `pkg/engine/entity_feed_sidecar_build.go` stores only `firstErr` in both
    `buildFeedEntitySidecars` and
    `stageFeedEntitySidecarsFromLoadedProviders`.
  - `runBoundedNameJobs` adopters include geo, ASN, bogon, and critical
    infrastructure fan-out.
- Existing cancellation behavior is valuable and should stay: the first failure
  cancels the child context so no new names are admitted.

Inference:

- The narrow fix is to collect all non-nil errors that in-flight workers report
  and return `errors.Join(errs...)`.
- Returning `errors.Join` preserves `errors.Is`/`errors.As`, so callers that
  check for `context.Canceled` or another wrapped sentinel still work.
- Entity sidecar workers must be able to publish an error result even after a
  sibling worker already cancelled the context; otherwise the aggregation still
  drops later in-flight failures.
- Non-goal with evidence: pairwise comparison open/I/O problems in
  `pkg/engine/output.go` are warning-and-skip paths, not first-error return
  paths. Converting those warnings into fatal run errors would change artifact
  publication semantics and is not required to satisfy this SOW.

Reopened from iterative audit cycle 3:

- `pkg/engine/entity_surgical.go` wraps `errEntitySurgicalNeedsFullRebuild`
  with `%w`, but includes the real unreadable-sidecar error with `%v`. This
  loses the original error chain and was not covered by the first SOW-0044
  implementation.

## Implications and Decisions

Autonomous maintainer decision from SOW-0038: implement option 3(c).

## Plan

- Add a small joined-error collector in `pkg/engine` and use it in
  `runBoundedNameJobs`.
- Update entity sidecar fan-outs to collect joined errors and preserve
  per-feed context in error messages.
- Add tests proving multiple in-flight worker errors are all preserved while
  cancellation behavior remains compatible.
- Update pipeline/operating specs if the behavior is now a product contract.
- Validate focused packages plus full Go tests.

## Execution Log

- Created as concrete follow-up from SOW-0038 so error aggregation work is not
  lost.
- Historical: moved to current after SOW-0043 completed.
- Added a shared `joinedErrorCollector` in `pkg/engine/concurrency.go`.
- Updated `runBoundedNameJobs` to return joined in-flight worker errors while
  preserving first-error cancellation of new work admission.
- Updated entity sidecar fan-outs in
  `pkg/engine/entity_feed_sidecar_build.go` to return joined build/stage
  errors and preserve feed-name context.
- Added `TestRunBoundedNameJobsReturnsJoinedWorkerErrors` in
  `pkg/engine/concurrency_test.go`.
- Updated `.agents/sow/specs/pipeline.md` with the worker-error aggregation
  contract.
- Updated `.agents/skills/project-coding/SKILL.md` with the fan-out
  aggregation rule.

## Validation

Historical validation before reopening:

- `go test ./pkg/engine` passed.
- `go test ./pkg/processor` passed.
- `make test` passed.
- `make lint` passed.
- `make vulncheck` passed for root and `tools/dronebl2ipsets`.
- `git diff --check` passed.
- `./install.sh` passed and restarted `update-ipsets`.
- `curl -fsS http://localhost:18888/healthz` returned `ok`.
- `systemctl is-active update-ipsets` returned `active`.
- `/api/v1/admin/integrity` returned `clean`.
- `/api/v1/admin/integrity/entities` returned `clean` after startup entity
  repair drained.

Reopened validation status:

- Completed. `pkg/engine/entity_surgical.go` now wraps both
  `errEntitySurgicalNeedsFullRebuild` and the unreadable-sidecar source error
  with `%w`.
- `pkg/engine/entity_surgical_test.go` verifies `errors.Is` still matches the
  full-rebuild sentinel and `errors.As` still reaches the JSON syntax error.
- `go test ./pkg/engine` passed.

## Outcome

Reopened after cycle 3 for the remaining entity-surgical error-chain loss.
The reopened entity-surgical error-chain gap is fixed and validated.

Previously, bounded engine fan-out was changed to preserve all observed
in-flight worker failures:

- Geo/ASN/bogon/critical fan-out paths benefit through `runBoundedNameJobs`.
- Entity sidecar build/stage fan-outs now return joined errors with feed-name
  context.
- First failure still cancels admission of new names, so cancellation and
  shutdown behavior from SOW-0042 remains intact.

## Lessons Extracted

- Error aggregation is an operator-facing contract for heavy/background
  fan-out. Hiding sibling worker failures behind `firstErr` makes triage
  depend on scheduler timing.
- Not every warning-only path should become fatal during an error-aggregation
  SOW. If a path currently skips and logs by product policy, changing it to a
  returned error needs a separate behavior decision.

## Followup

None.
