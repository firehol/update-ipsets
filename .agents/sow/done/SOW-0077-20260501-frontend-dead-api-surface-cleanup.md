# SOW-0077 - Frontend Dead API Surface Cleanup

## Status

Status: completed

Sub-state: completed and validated

## Requirements

### Purpose

Remove frontend API helpers, query wrappers, and generated type references that
are no longer reachable from the UI, so source code reflects the actual product
surface.

### User Request

Review project quality against the four named project skills, identify gaps,
create SOWs for actionable improvements, and iterate the audit until valid
findings are fixed or tracked.

### Assistant Understanding

Facts:

- Audit cycle 2 found unused homepage summary frontend API/query/type surfaces
  in files such as `ui/src/lib/queries/home.ts`,
  `ui/src/lib/api-client/home.ts`, and `ui/src/lib/api-types.ts`.
- SOW-0058 moved homepage summary/globe serving to a precomputed backend
  artifact, but the frontend source still needs a reachability check.

Inferences:

- Some type definitions may be generated or shared by other API helpers and
  should not be removed casually.
- Dead frontend API helpers can keep stale endpoint assumptions alive.

Unknowns:

- Resolved: the homepage summary declarations in scope were source-owned
  frontend TypeScript declarations. Shared `HomeSummaryProvider` remains
  reachable from country/ASN detail payload types.

### Acceptance Criteria

- Prove which homepage API helpers, query wrappers, and types are reachable
  from routes/components/tests.
- Remove dead source-only helpers and types when safe.
- If generated or shared types must remain, record the reason with evidence.
- Run UI lint, tests, and build.

## Analysis

Sources checked:

- Cycle-2 frontend best-practices findings
- SOW-0058 homepage aggregate work

Current state:

- At least one homepage summary frontend data surface may be stale or
  unreachable.

Risks:

- Removing generated/shared types without understanding their owner can break
  API-client consistency.

## Plan

1. Search imports and route usage for homepage API/query/type names.
2. Classify each artifact as reachable, generated/shared, or dead source.
3. Remove dead source only and keep generated/shared artifacts with rationale.
4. Run UI validation gates.

## Execution Log

### 2026-05-01

- Created from iterative audit cycle 2.
- Moved from `pending/` to `current/` as autonomous maintainer-owned frontend
  cleanup.
- Proved `getHomeSummary`, `homeSummaryOptions`, `queryKeys.homeSummary`, and
  `HomeSummaryPayload` were not imported by any route, component, or test.
- Removed the unreachable `/api/v1/home/summary` frontend helper and query
  option.
- Removed the unused homepage summary query key.
- Removed unused homepage summary-only payload/type declarations while keeping
  `HomeSummaryProvider`, which country/ASN detail types still use.

## Validation

Acceptance criteria evidence:

- Reachability scan after cleanup found no references to
  `HomeSummaryPayload`, `HomeSummaryTotals`, `HomeSummaryCountry`,
  `HomeSummaryASN`, `HomeSummaryMaintainer`, `getHomeSummary`,
  `homeSummaryOptions`, `homeSummary(`, or `/api/v1/home/summary` in
  `ui/src`.
- Shared `HomeSummaryProvider` was kept because it is still used by country
  and ASN detail payload types in `ui/src/lib/api-types.ts`.

Tests or equivalent validation:

- `pnpm --dir ui lint` passed.
- `pnpm --dir ui test` passed: 9 files, 21 tests.
- `pnpm --dir ui build` passed. Existing Inter display font runtime
  resolution warnings remain unchanged.

Reviewer findings:

- Frontend best-practices review found unused homepage summary frontend API
  surface after SOW-0058.

Artifact maintenance gate:

- AGENTS.md: no update needed; existing rules already say dead frontend
  feature/API code should be removed with unreachable features.
- Runtime project skills: no update needed; existing frontend best-practices
  skill already covers dead frontend API cleanup.
- Specs: no update needed; public backend `/api/v1/home/summary` behavior was
  not changed.
- End-user/operator docs: no update needed; UI behavior did not change.
- End-user/operator skills: no update needed.
- SOW lifecycle: completed; moved to `done/`.

Follow-up mapping:

- None.

## Outcome

Completed.

## Lessons Extracted

Keep shared API types only when they remain reachable from live payloads.
