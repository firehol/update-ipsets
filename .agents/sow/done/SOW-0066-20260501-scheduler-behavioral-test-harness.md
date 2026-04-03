# SOW-0066 - Scheduler Behavioral Test Harness

## Status

Status: completed

Sub-state: implemented and validated

## Requirements

### Purpose

Reduce scheduler tests that assert private queue mechanics by adding a behavioral harness around `Runner.Run`, public triggers, configured engine inputs, and admin-visible activity.

### User Request

Review project quality against the four named project skills, identify gaps, and create SOWs for actionable improvements.

### Assistant Understanding

Facts:

- SOW-0048 added lifecycle/leak guards for scheduler cancellation and `ActivitySnapshot`.
- The quality review found scheduler tests still directly calling private methods and inspecting private queue fields.
- Scheduler queue-lock invariants are important and justify some same-package tests.
- SOW-0083 mapped the duplicated scheduler polling-helper question here because shared wait helpers, `testing/synctest`, and a public `Runner.Run` harness are coupled scheduler test-design choices.

Resolved:

- A focused first wave can be converted through public `Runner.Run` without making the suite slow.
- Some private queue tests remain valid because they cover queue admission, ordering, and requeue invariants that are not exposed without brittle timing.

### Acceptance Criteria

- Inventory scheduler tests that call private methods or inspect private queue fields.
- Classify each as behavior-testable or internal-invariant.
- Classify duplicated polling helpers and decide whether scheduler tests should use a shared helper, `testing/synctest`, or harness-visible state instead.
- Add a behavioral harness that drives scheduler actions through public APIs and asserts observable state/artifacts/activity.
- Convert a focused first wave of behavior-testable private tests.
- Record explicit rationale for remaining internal-invariant tests.

## Analysis

Sources checked:

- `pkg/scheduler/*_test.go`
- SOW-0048.
- `project-go-behavioral-testing` skill.

Current state:

- `pkg/scheduler/runner_harness_test.go` provides a package-local `Runner.Run` harness and bounded observable wait helpers.
- `pkg/scheduler/scheduler_test.go` drives manual recheck through `TriggerQueuedAction` and observes `ActivitySnapshot` download activity.
- `pkg/scheduler/scheduler_test.go` verifies scheduled download-to-process handoff by waiting for the published `scheduled.ipset` artifact, not by asserting a private processing wake channel.
- Full-runner race validation exposed a real cache-entry race between downloader metadata writes, processing metadata writes, and cache snapshots. This is fixed with entry-level lifecycle locking and locked cache snapshot cloning in `pkg/cache`.

Remaining intentional private-test coverage:

- `pkg/scheduler/policy_test.go` keeps active-download refetch deferral/release and provider-default/staged-work queue admission checks as internal invariants.
- `pkg/scheduler/scheduler_test.go` keeps processing requeue, processing promotion/retry, derived download ordering, artifact-child admission, and active/deferred processing checks as internal invariants.
- `pkg/scheduler/queue_state_test.go` keeps queue snapshot metadata checks as operator-facing helper contract coverage.

Risks handled:

- Queue refactors are less likely to break behavior tests for public trigger/run-loop behavior.
- Subtle queue invariants remain covered where public behavior would be too slow, too broad, or timing-sensitive.
- Cache snapshots are now safe while scheduler download and processing work mutate entry lifecycle metadata.

## Implications And Decisions

Assistant-owned implementation decision:

1. Harness scope
   - A. Convert all scheduler tests to black-box.
     - Pros: strongest public-contract purity.
     - Cons: may lose precise internal invariant coverage.
   - B. Add behavioral harness and convert behavior-testable private tests. Chosen.
     - Pros: improves quality while preserving necessary internal tests.
     - Cons: required classification work and race validation.
   - C. Keep current scheduler tests unchanged.
     - Pros: no churn.
     - Cons: private queue mechanics remain test contracts.

## Plan

1. Inventory private method/field assertions.
2. Design test harness around `Runner.Run`, triggers, activity snapshots, and fixture engine state.
3. Convert first wave of behavior-testable tests.
4. Preserve and document internal-invariant tests.
5. Run scheduler package tests, strict tests, race, and lint gates.

## Execution Log

### 2026-05-01

- Created from the four-skill quality review.
- Moved SOW to current.
- Inventoried scheduler private method/field assertions with `rg`.
- Added `pkg/scheduler/runner_harness_test.go` with `startSchedulerRunner`, observable activity/snapshot waits, and file-content waits.
- Converted manual recheck coverage from direct `handleAction` to public `TriggerQueuedAction` plus `Runner.Run` and `ActivitySnapshot`.
- Converted scheduled download handoff coverage from direct `runDownload` and private wake-channel assertion to a full runner flow that publishes `scheduled.ipset`.
- Replaced the scheduler-specific snapshot polling helper with the shared harness helper.
- Ran `make race`; it exposed a real cache-entry data race under the new full-runner scheduler coverage.
- Fixed cache-entry lifecycle synchronization by adding per-entry lifecycle locking, locked snapshot cloning, safe JSON marshaling, and detached entry snapshots.
- Split cache entry transition methods into `pkg/cache/entry_config.go` and `pkg/cache/entry_lifecycle.go` so the race fix did not grow `pkg/cache/cache.go` past the architecture posture baseline.
- Updated `project-testing` with the durable scheduler harness rule.

## Validation

Acceptance criteria evidence:

- Inventory completed with `rg -n "handleAction|runDownload|runQueuedProcessing|\\.download\\.|\\.processing\\." pkg/scheduler/*_test.go`.
- Behavioral harness added in `pkg/scheduler/runner_harness_test.go`.
- First-wave conversions: `TestManualRecheckQueuesDownloadWork` and `TestScheduledDownloadWithProcessingWorkWakesProcessLoop`.
- Remaining internal-invariant rationale recorded in this SOW.
- Duplicated polling helper decision: use package-local bounded ticker helpers over `testing/synctest` for these tests because they drive real HTTP servers, filesystem artifacts, and the actual `Runner.Run` goroutine lifecycle.

Tests or equivalent validation:

- `go test ./pkg/scheduler`
- `go test ./pkg/cache ./pkg/engine ./pkg/scheduler`
- `go test -race ./pkg/scheduler`
- `make test`
- `make test-strict`
- `make race`
- `make lint`
- `git diff --check`

Real-use evidence:

- The full scheduler runner test downloads from a real `httptest` upstream and waits for the published `scheduled.ipset` artifact.
- The manual recheck test queues through the public scheduler action surface and observes active download state via `ActivitySnapshot`.
- `make race` proves the scheduler full-runner path no longer races cache snapshots against download/processing cache-entry mutations.

Reviewer findings:

- Go behavioral-testing review found scheduler tests assert private queue mechanics.
- This SOW addressed the behavior-testable part and preserved internal-invariant tests with rationale.

Same-failure scan:

- Remaining private scheduler assertions are intentional internal-invariant tests, not forgotten behavior-testable public flows.
- `rg` no longer finds `context.Background`, `time.Sleep`, or the removed `waitForSchedulerSnapshotItems` helper in `pkg/scheduler/*_test.go`.

Artifact maintenance gate:

- AGENTS.md: no update needed; no project-wide workflow rule changed.
- Runtime project skills: updated `.agents/skills/project-testing/SKILL.md` with scheduler harness guidance.
- Specs: no update needed; the cache race fix enforces the existing pipeline contract that downloader and processing work may overlap.
- End-user/operator docs: no update needed; no user/operator behavior changed.
- End-user/operator skills: no update needed.
- SOW lifecycle: move current SOW to done after audit.

Specs update:

- Not needed.

Project skills update:

- `.agents/skills/project-testing/SKILL.md` records the scheduler harness rule.

End-user/operator docs update:

- Not needed.

End-user/operator skills update:

- Not needed.

Lessons:

- Full-runner behavioral tests can reveal runtime races that private queue tests hide. Keep `make race` in the validation gate when scheduler lifecycle behavior changes.
- Cache entries are intentionally copied for public/admin snapshots, so live entry lifecycle mutations need explicit synchronization and snapshots must detach slice fields.

Follow-up mapping:

- None.

## Outcome

Completed.

## Lessons Extracted

- Use `startSchedulerRunner` for future scheduler tests that claim public trigger/run-loop behavior.
- Keep same-package private scheduler tests only when the SOW names the queue admission, ordering, lock, or requeue invariant being protected.

## Followup

None.
