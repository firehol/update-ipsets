# SOW-0079 - Go Best-Practice Cleanup Mapping

## Status

Status: completed

Sub-state: validated

## Requirements

### Purpose

Convert valid but previously vague Go best-practice cleanup items from SOW-0038
into concrete implementation decisions, fixes, or explicit evidence-backed
non-goals.

### User Request

Review project quality against the four named project skills, identify gaps,
create SOWs for actionable improvements, and iterate the audit until valid
findings are fixed or tracked.

### Assistant Understanding

Facts:

- SOW-0038 mapped some remaining Go best-practice items as "opportunistic" or
  folded into already-completed SOW-0030 phases.
- Iterative audit cycle 3 found that mapping too vague under the project SOW
  rules.
- Current examples include duplicated entity sidecar fan-out code, shallow-copy
  behavior in `cache.EntrySnapshot`, and older `sync.Once` lazy-init patterns.

Inferences:

- These are maintainability issues, not urgent product behavior defects.
- Each item needs a concrete decision: implement now in this SOW, split to a
  narrower pending SOW, or reject as not worth changing with file-level
  evidence.

Unknowns:

- Resolved: `EntrySnapshot` should match `ReplaceEntry`'s copy contract for
  slice fields.
- Resolved: `sync.OnceValue`/`sync.OnceValues` improve the two reviewed
  process-wide lazy singletons without changing semantics.

### Acceptance Criteria

- Review duplicated entity sidecar fan-out in
  `pkg/engine/entity_feed_sidecar_build.go` and either consolidate it or record
  a concrete follow-up/non-goal decision with evidence.
- Review `pkg/cache.EntrySnapshot` slice/map copy semantics against
  `ReplaceEntry`; either harden with tests or record why the current contract is
  safe.
- Review `sync.Once` lazy-init sites such as `pkg/web/methodology.go` and
  `pkg/engine/insights.go`; either modernize with `sync.OnceValue`/
  `sync.OnceValues` where useful or record why not.
- Update SOW-0038 follow-up mapping so the older "opportunistic" language no
  longer leaves valid work untracked.
- Run focused package tests plus `make test`.

## Analysis

Sources checked:

- `project-go-best-practices`
- `project-reviewing`
- Cycle-3 Go best-practices reviewer findings
- SOW-0038 follow-up ledger

Current state:

- The vague SOW-0038 cleanup mapping is now resolved by concrete code changes
  and a SOW-0038 closure addendum.

Risks:

- A broad cleanup can create churn across unrelated packages. This SOW limited
  implementation to cohesive low-risk fixes in cache snapshots, lazy
  singletons, and entity sidecar fan-out.

Item outcomes:

| Item | Outcome | Evidence |
|---|---|---|
| Duplicated entity sidecar fan-out | Fixed | `pkg/engine/entity_feed_sidecar_build.go` now uses `startFeedEntitySidecarBuild` for both standalone sidecar builds and staged provider fan-out. |
| `EntrySnapshot` shallow slice copy | Fixed | `pkg/cache/cache.go` now uses `cloneEntry` for both `ReplaceEntry` and `EntrySnapshot`; `pkg/cache/cache_test.go` mutates original slice fields after snapshot and verifies isolation. |
| `sync.Once` lazy init in methodology pages | Fixed | `pkg/web/methodology.go` now uses `sync.OnceValues` to cache rendered methodology pages and ordered index. |
| `sync.Once` lazy init in insights engine | Fixed | `pkg/engine/insights.go` now uses `sync.OnceValue` for the shared insights engine. |
| Broader aesthetic modernization (`sort` to `slices`, `b.Loop`, field alignment, doc-comment lint, OTel exhaustiveness) | Non-goal here | These were outside the SOW acceptance criteria and remain lower-signal advisory cleanup. No runtime or test contract required touching them in this focused mapping SOW. |

## Plan

1. Re-check SOW-0038 remaining B/C items and current code evidence.
2. Decide item-by-item whether to fix, split, or reject with evidence.
3. Implement only cohesive low-risk fixes here.
4. Update SOW-0038 follow-up mapping and validation evidence.

## Execution Log

### 2026-05-01

- Created from iterative audit cycle 3.
- Hardened `pkg/cache.EntrySnapshot` copy semantics and expanded the
  behavioral test.
- Replaced reviewed `sync.Once` lazy-init globals with `sync.OnceValue(s)`.
- Consolidated duplicated entity sidecar fan-out worker setup.
- Updated `.agents/sow/done/SOW-0038-20260501-go-code-re-review.md` with the
  completed SOW-0079 mapping.

## Validation

Acceptance criteria evidence:

- Duplicated entity sidecar fan-out reviewed and consolidated in
  `pkg/engine/entity_feed_sidecar_build.go`.
- `pkg/cache.EntrySnapshot` copy semantics reviewed against `ReplaceEntry` and
  hardened with `cloneEntry` plus slice-isolation tests.
- `sync.Once` lazy-init sites in `pkg/web/methodology.go` and
  `pkg/engine/insights.go` modernized with `sync.OnceValues` and
  `sync.OnceValue`.
- SOW-0038 follow-up mapping updated with a closure addendum.
- Same-failure scans:
  - `rg -n "sync\\.Once|\\.Do\\(|insightsEngineOnce|methodologyPagesOnce|methodologyPages|methodologyOrdered" pkg/web/methodology.go pkg/engine/insights.go ...` now only finds the intended `sync.OnceValue(s)` calls.

Reviewer findings:

- Go best-practices review found SOW-0038's remaining cleanup mapping too vague.

Tests or equivalent validation:

- `go test ./pkg/cache` passed.
- `go test ./pkg/web -run 'TestMethodology|TestAPIEndpointsAndCORS|TestSurfaceHandlerModesRegisterExpectedSurfaces'` passed.
- `go test ./pkg/engine -run 'Test.*Entity|Test.*Insight|TestPipelineIntegrity|TestFeedEntity'` passed.
- `go test ./pkg/web ./pkg/engine ./pkg/cache` passed.
- `go test ./pkg/engine -race -run 'Test.*Entity|TestPipelineIntegrity'` passed.
- `go test ./pkg/cache -race` passed.
- `make test` passed.
- `make lint` passed.

Artifact maintenance gate:

- `AGENTS.md`: no update needed; project-wide rules did not change.
- Runtime project skills: no update needed; existing Go best-practice skill
  already recommends these patterns.
- Specs: no product/application contract changed.
- End-user/operator docs: no user/operator behavior changed.
- End-user/operator skills: no update needed.
- SOW lifecycle: moved from pending to current; SOW-0038 mapping updated; will
  move to done after validation.

## Outcome

Completed.

The previously vague SOW-0038 Go cleanup mapping now has concrete outcomes.
Low-risk valid items were fixed, broader advisory modernization stayed out of
scope, and the old SOW-0038 follow-up text points to this completed SOW.
