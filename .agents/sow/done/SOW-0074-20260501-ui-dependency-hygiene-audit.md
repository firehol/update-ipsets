# SOW-0074 - UI Dependency Hygiene Audit

## Status

Status: completed

Sub-state: completed and validated

## Requirements

### Purpose

Remove stale direct UI dependencies where safe, reducing dependency surface and bundle/tooling noise without breaking transitive consumers.

### User Request

Review project quality against the four named project skills, identify gaps, and create SOWs for actionable improvements.

### Assistant Understanding

Facts:

- `ui/package.json` lists `@tanstack/react-table`, but source scan found no imports of `@tanstack/react-table`, `useReactTable`, or `ColumnDef`.
- Audit cycle 2 found the unused-dependency finding conflicts with the current
  frontend best-practices skill, which says tables should use TanStack Table
  instead of hand-rolled sort/filter/paging logic.
- `ui/src/components/editorial/data-table.tsx` currently implements sorting
  and filtering locally.
- `ui/package.json` lists direct `d3-geo` and `@types/d3-geo`.
- `pnpm --dir ui why d3-geo @types/d3-geo` shows `d3-geo` and types also arrive transitively through `react-simple-maps` and `@visx/vendor`.

Inferences:

- `@tanstack/react-table` might be removable only if the project consciously
  keeps the current lightweight table pattern or updates the skill exception.
- If the editorial/admin tables need richer table behavior, the correct fix may
  be migrating those tables to TanStack Table rather than removing the direct
  dependency.
- Direct `d3-geo`/types may be stale, but removal must be proven because map libraries and type packages may depend on version-specific behavior.

Unknowns:

- Resolved: lockfile changes are acceptable for proven-unused direct
  dependencies when install, lint, tests, build, e2e, and bundle budget pass.
- Resolved: source/package scans found no direct imports of the D3 geo package
  names in `ui/src`.

### Acceptance Criteria

- Verify dependency usage with source scan and package-manager evidence.
- Decide the table strategy with evidence: migrate appropriate hand-rolled
  tables to TanStack Table, keep a documented local-table exception, or update
  the project skill if TanStack Table is no longer the desired default.
- Remove unused direct dependencies only when `pnpm install --frozen-lockfile`, lint, tests, build, e2e, and bundle budget prove safety.
- Record any dependency kept with evidence.
- Ensure lockfile changes are minimal and intentional.

## Analysis

Sources checked:

- `ui/package.json`
- `ui/src`
- `pnpm --dir ui why @tanstack/react-table d3-geo @types/d3-geo`

Current state:

- At least one direct dependency appears unused by source.

Risks:

- Removing direct dependencies can alter lockfile resolution or break type resolution.
- Dependency cleanup is low product risk but must not be done casually during unrelated work.

## Implications And Decisions

Autonomous maintainer decisions:

1. Keep `@tanstack/react-table` for now.
   - Evidence: source does not import it today, but the project frontend skill
     still defines TanStack Table as the preferred direction for non-trivial
     table behavior. Removing it while leaving hand-rolled sort/filter tables
     would weaken the stated migration path.
   - Implication: this SOW does not perform a table migration. Follow-up table
     behavior/a11y work remains represented by existing frontend SOWs.
2. Remove direct `d3-geo` and `@types/d3-geo`.
   - Evidence: source scan found no direct imports, while `pnpm why` shows D3
     geo packages still arrive transitively through `react-simple-maps` and
     `@visx/vendor`.
   - Implication: package ownership is cleaner without changing map rendering
     code.

## Plan

1. Re-run usage scans and `pnpm why`.
2. Remove one dependency group at a time.
3. Run install, lint, tests, build, e2e, and bundle budget after each removal.
4. Keep or revert specific removals based on evidence, without broad checkout/reset.
5. Record final dependency rationale.

## Execution Log

### 2026-05-01

- Created from the four-skill quality review.
- Moved from `pending/` to `current/` as autonomous maintainer-owned dependency
  hygiene work.
- Re-ran source scans for `@tanstack/react-table`, `useReactTable`,
  `ColumnDef`, `d3-geo`, and common D3 geo API names.
- Kept `@tanstack/react-table` because the project frontend skill still names
  it as the preferred path for non-trivial table behavior, and current
  hand-rolled table cleanup remains represented by frontend SOWs.
- Removed direct `d3-geo` and `@types/d3-geo` from `ui/package.json` with
  `pnpm --dir ui remove d3-geo @types/d3-geo`.
- Verified `pnpm why` still resolves D3 geo through `react-simple-maps` and
  `@visx/vendor` transitively.

## Validation

Acceptance criteria evidence:

- Source/package scan found no direct `d3-geo` imports in `ui/src` after
  removal.
- `pnpm --dir ui why d3-geo @types/d3-geo @tanstack/react-table` shows:
  `@tanstack/react-table` remains a direct dependency by decision; D3 geo and
  D3 geo types remain only transitively through map/chart libraries.
- `git diff -- ui/package.json ui/pnpm-lock.yaml` shows only direct
  `d3-geo@3.1.1` and direct `@types/d3-geo@3.1.0` importer/snapshot removal.

Tests or equivalent validation:

- `pnpm --dir ui install --frozen-lockfile` passed.
- `pnpm --dir ui lint` passed.
- `pnpm --dir ui test` passed: 9 files, 21 tests.
- `pnpm --dir ui build` passed. Existing Inter display font runtime
  resolution warnings remain unchanged.
- `pnpm --dir ui test:e2e` passed: 4 Chromium tests.
- `pnpm --dir ui build:budget` passed when run alone. A prior parallel run
  raced another command rebuilding `ui/dist`, so it was discarded and rerun.
- `git diff --check` passed.

Real-use evidence:

- The browser e2e smoke suite rendered homepage, feed detail chart surfaces,
  admin drawer focus behavior, and country detail route after dependency
  removal.

Reviewer findings:

- Frontend best-practices review found likely stale direct UI dependencies.
- Iterative audit cycle 2 found the `@tanstack/react-table` removal path is not
  safe to treat as purely mechanical because current table code is hand-rolled
  while the skill says to prefer TanStack Table.

Same-failure scan:

- `rg '@tanstack/react-table|useReactTable|ColumnDef|d3-geo|geoPath|geoMercator|geoEqualEarth|geoNaturalEarth|geoAlbers|geoGraticule' ui/src ui/package.json ui/pnpm-lock.yaml`
  shows `@tanstack/react-table` remains direct by decision, D3 geo remains in
  lockfile only as transitive dependencies, and source only contains
  react-simple-maps projection string props.

Artifact maintenance gate:

- AGENTS.md: no update needed; dependency hygiene behavior is already covered.
- Runtime project skills: no update needed; frontend table strategy remains
  unchanged.
- Specs: no update needed; no product behavior changed.
- End-user/operator docs: no update needed.
- End-user/operator skills: no update needed.
- SOW lifecycle: completed; moved to `done/`.

Specs update:

- None needed.

Project skills update:

- None needed.

End-user/operator docs update:

- None needed.

End-user/operator skills update:

- None needed.

Lessons:

- Do not remove an unused dependency when it is the declared migration path for
  existing hand-rolled source unless the skill/rule changes in the same SOW.

Follow-up mapping:

- Table cleanup remains represented by existing frontend SOWs.

## Outcome

Completed.

## Lessons Extracted

Dependency removal should be one dependency group at a time with package-manager
evidence before and after.

## Followup

None yet.
