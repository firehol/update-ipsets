# SOW-0029 | 2026-04-30 | code-quality-analysis

## Status

completed

## Requirements

### Purpose

Assess the repository's code quality, separation of concerns, and clean-code
health using concrete local evidence. The output should help decide whether the
codebase is structurally healthy, where it is fragile, and what refactoring
work would improve maintainability without breaking the working pipeline.

### User request quoted verbatim

> I need you to do an analysis on code quality in this repo, separation of
> concerns, clean code principles. How does this codebase stand?

### Assistant understanding

- This is an analysis/review task, not an implementation request.
- The answer must separate evidence from interpretation.
- The answer must identify strengths, risks, and recommended refactoring
  priorities with file/package evidence.

### Acceptance criteria

- Inspect repository structure, package boundaries, large files/modules, tests,
  and representative code paths.
- Identify concrete examples of good separation and weak separation.
- Explain whether the codebase is currently maintainable and where it is likely
  to get worse.
- Recommend prioritized improvements without inventing unsupported claims.

## Analysis

Local evidence gathered on 2026-04-30:

- Source size, excluding generated/frontend dependency artifacts:
  - 439 Go/TS/TSX source files.
  - 102,960 total source lines.
  - Largest source areas by line count:
    - `pkg/engine`: 34,634 lines in 119 files.
    - `pkg/iprange`: 10,719 lines in 53 files.
    - `pkg/web`: 8,330 lines in 29 files.
    - `pkg/config`: 6,914 lines in 16 files.
    - `pkg/processor`: 4,581 lines in 14 files.
    - `pkg/scheduler`: 3,449 lines in 8 files.
- Largest files include:
  - `pkg/scheduler/scheduler.go` at 1,474 lines.
  - `pkg/engine/output.go` at 1,366 lines.
  - `pkg/web/server.go` at 1,157 lines.
  - `pkg/web/admin.go` at 1,043 lines.
  - `pkg/engine/home_entity_builders.go` at 1,021 lines.
  - `pkg/engine/entity_integrity.go` at 1,020 lines.
  - `pkg/engine/critical.go` at 987 lines.
  - `ui/src/components/admin/feeds-table.tsx` at 1,298 lines.
  - `ui/src/components/admin/feed-modal.tsx` at 1,295 lines.
- Import boundary evidence:
  - `pkg/engine` has 50 direct imports and 465 transitive dependencies.
  - `pkg/web` has 38 direct imports and 478 transitive dependencies.
  - `pkg/scheduler` has 21 direct imports and 466 transitive dependencies.
  - `pkg/iprange` has no project-package imports; it remains a clean standalone
    package boundary.
- Representative code path evidence:
  - `pkg/engine/run.go` `RunOnce()` coordinates source processing, heavy
    comparison phases, entity artifacts, metadata, publish markers, cache save,
    and cleanup in one procedural pipeline.
  - `pkg/scheduler/scheduler.go` owns fetch admission, processing admission,
    admin actions, status snapshots, and queue dispatch in one dense state
    machine.
  - `pkg/web/server.go` centralizes public route registration and many route
    decisions in one large handler setup/switch.
  - `pkg/cache/cache.go` exposes mutable `*Entry` values via `Entry(name)` with
    a caller-serialization contract; safer snapshot APIs exist, but mutation
    safety still depends on callers.

Interpretation:

- The codebase is functionally mature and heavily validated, but not clean-small.
- The strongest boundaries are data/config-driven semantics, `pkg/iprange`,
  smaller utility/domain packages, and the growing specs/tests around pipeline
  integrity.
- The weakest boundaries are the central `pkg/engine`, `pkg/web`, and
  `pkg/scheduler` packages. File splitting improves navigation, but many
  concerns still share one package namespace and one mutation surface.
- The recent cache race and integrity regressions are consistent with the
  structural evidence: the system has good runtime detectors and tests, but some
  core ownership boundaries are still too implicit.

## Implications and decisions

No user design decision is required for this analysis-only SOW.

## Plan

1. Collect repository metrics: package/file sizes, import boundaries, tests, and
   build/lint status already known from recent validation.
2. Read representative high-risk modules (`pkg/engine`, `pkg/web`,
   `pkg/scheduler`, config, UI explorer/admin components).
3. Compare findings against project specs/skills and clean-code principles:
   cohesion, coupling, ownership boundaries, naming, testability, resource
   boundaries, and public/runtime separation.
4. Deliver concise findings with evidence and prioritized recommendations.

## Execution log

- Read applicable project skills for reviewing, coding, and content-surface
  discipline.
- Created this SOW as the analysis ledger.
- Collected source metrics with `rg --files`, `wc -l`, and `go list`.
- Inspected representative files in `pkg/engine`, `pkg/web`, `pkg/scheduler`,
  `pkg/cache`, and UI source directories.
- Checked common smell markers such as TODO/FIXME/HACK, production
  `context.Background`, broad mutable state access, and package import
  boundaries.

## Validation

- No product code was changed.
- No tests were required for this analysis-only request.
- Recent validation from the previous implementation remains relevant context:
  `make test`, `make lint`, `make race`, `pnpm --dir ui lint`,
  `pnpm --dir ui build`, install smoke, and integrity checks were run for the
  currently dirty worktree before this analysis started.

## Outcome

Completed. The user-facing answer should report that the repository stands at
roughly B-/B: operationally mature and tested, with good data/config discipline,
but with maintainability risk concentrated in `pkg/engine`, `pkg/web`,
`pkg/scheduler`, mutable cache state, and large UI admin components.

## Lessons extracted

- Future feature SOWs that touch `pkg/engine`, `pkg/web`, `pkg/scheduler`, or
  cache state should explicitly record a separation-of-concerns impact before
  implementation.
- Refactoring should be incremental and boundary-focused. A broad package split
  would be risky; safer first steps are typed mutation APIs, route-family
  extraction, scheduler submodule extraction, and large UI component
  decomposition.
