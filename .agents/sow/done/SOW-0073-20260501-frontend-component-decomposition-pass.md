# SOW-0073 - Frontend Component Decomposition Pass

## Status

Status: closed

Sub-state: completed

## Requirements

### Purpose

Reduce the largest frontend components into smaller, reviewable units without changing product behavior or creating new abstractions without need.

### User Request

Review project quality against the four named project skills, identify gaps, and create SOWs for actionable improvements.

### Assistant Understanding

Facts:

- The frontend best-practices skill treats TSX files over roughly 400 lines as a smell and recommends extraction when components exceed about 250 lines with branching.
- Current large files include `ui/src/components/feed-sidebar.tsx`, `ui/src/components/admin/current-run.tsx`, `ui/src/components/feed-detail/section-retention.tsx`, `ui/src/components/ip-search/ip-search-results.tsx`, and `ui/src/components/admin/feeds-table-body.tsx`.
- Some previous SOWs split admin feed modal/table files successfully.

Inferences:

- Decomposition should start with files likely to be touched by accessibility/test work to avoid unrelated churn.
- Behavior-preserving splits still need UI tests/lint/build to catch import and route-boundary regressions.

Unknowns:

- None remaining for this first-wave cleanup.

### Acceptance Criteria

- Inventory large frontend files and classify by risk/churn.
- Split a focused first wave of top offenders into cohesive subcomponents/helpers.
- Preserve route chunk boundaries and bundle budget.
- No UI behavior or copy changes unless separately justified.
- Tests/lint/build/bundle-budget pass.

## Analysis

Sources checked:

- `ui/src/components/*`
- `project-frontend-best-practices` skill.
- SOW-0040 and SOW-0050.

Current state:

- Several high-traffic components remain large and mixed.

Risks:

- Large files make review and future accessibility/testing changes harder.
- Refactors can accidentally change rendering, imports, or chunk boundaries.

## Implications And Decisions

No user decision was required. This was a behavior-preserving implementation
cleanup with no product behavior, copy, route, or operator-policy change.

Assistant implementation decision:

1. First wave
   - Chosen: split files touched by active accessibility/test work plus the
     worst nearby admin offender.
   - Reason: this keeps the cleanup reviewable and avoids broad standalone
     churn across unrelated public pages.
   - Explicit non-goal: split every remaining file over 400 lines in this SOW.
     Remaining large files are maintainability smells, not runtime bugs, and
     should be split when active work touches them or a later focused cleanup is
     opened.

## Plan

1. Inventory large files and coupling. Done.
2. Pick a focused first wave. Done.
3. Extract subcomponents/helpers using existing local patterns. Done.
4. Verify route chunks and bundle budget. Done.
5. Run UI tests/lint/build and update skills if needed. Done.

## Execution Log

### 2026-05-01

- Created from the four-skill quality review.
- Ran a TS/TSX line-count inventory.
- Split `ui/src/components/admin/feeds-table-body.tsx` into:
  - `ui/src/components/admin/feeds-table-body.tsx`
  - `ui/src/components/admin/feeds-table-header.tsx`
  - `ui/src/components/admin/feeds-table-row.tsx`
- Split admin live-queue rendering out of
  `ui/src/components/admin/current-run.tsx` into:
  - `ui/src/components/admin/current-run-queue-columns.tsx`
  - `ui/src/components/admin/current-run-shared.ts`
- Updated `project-frontend-best-practices` with the Fast Refresh export rule
  found during validation.

## Validation

Acceptance criteria evidence:

- Inventory before the split showed these top TS/TSX files:
  - `ui/src/components/feed-sidebar.tsx`: 713 lines
  - `ui/src/components/admin/current-run.tsx`: 670 lines
  - `ui/src/components/feed-detail/section-retention.tsx`: 550 lines
  - `ui/src/components/ip-search/ip-search-results.tsx`: 538 lines
  - `ui/src/components/admin/feeds-table-body.tsx`: 503 lines
- First-wave split results:
  - `ui/src/components/admin/feeds-table-body.tsx`: 63 lines
  - `ui/src/components/admin/feeds-table-header.tsx`: 196 lines
  - `ui/src/components/admin/feeds-table-row.tsx`: 263 lines
  - `ui/src/components/admin/current-run.tsx`: 307 lines
  - `ui/src/components/admin/current-run-queue-columns.tsx`: 362 lines
  - `ui/src/components/admin/current-run-shared.ts`: 9 lines
- Route boundaries were preserved; no route imports moved out of the admin
  surface.
- Bundle budget remained green. Admin route after the split:
  - raw 134.7 KiB / 170.0 KiB
  - gzip 37.2 KiB / 50.0 KiB

Tests or equivalent validation:

- PASS: `pnpm --dir ui lint`
- PASS: `pnpm --dir ui test -- ui/src/components/admin/feeds-table.test.tsx ui/src/pages/admin-actions.test.tsx`
- PASS: `pnpm --dir ui build:budget`
- PASS: `pnpm --dir ui test:e2e`

Real-use evidence:

- Browser smoke tests still open the admin feed drawer and verify dialog focus
  behavior through the production bundle.
- Homepage, feed-detail visualization, and entity-route browser smoke tests
  still pass after the route chunk rebuild.

Reviewer findings:

- Frontend best-practices review found several frontend files exceed project split heuristics.

Same-failure scan:

- Completed with `rg --files ui/src -g '*.tsx' -g '*.ts' | xargs wc -l | sort -nr | head -25`.
- Remaining files over the heuristic line count are intentionally not split in
  this first wave because they were not part of the active admin/accessibility
  work path. This is a scoped cleanup decision, not a hidden deferral.

Artifact maintenance gate:

- AGENTS.md: no update needed; no workflow rule changed.
- Runtime project skills: updated
  `.agents/skills/project-frontend-best-practices/SKILL.md` with the
  component-file export rule for Fast Refresh.
- Specs: no update needed; behavior and route contracts unchanged.
- End-user/operator docs: no update needed; no user-visible workflow changed.
- End-user/operator skills: no update needed; no exported skill changed.
- SOW lifecycle: moved from pending to current for implementation, then to done
  after validation.

Specs update:

- Not needed.

Project skills update:

- Updated `.agents/skills/project-frontend-best-practices/SKILL.md`.

End-user/operator docs update:

- Not needed.

End-user/operator skills update:

- Not needed.

Lessons:

- React component files should export only components; shared constants and
  helpers belong in plain `.ts` modules to satisfy the Fast Refresh lint rule.

Follow-up mapping:

- None.

## Outcome

Completed.

## Lessons Extracted

- For decomposition work, keep the split at natural responsibility boundaries:
  table shell, table header, table row, queue columns, and shared helpers.
- Validate route chunks with the bundle-budget gate after moving component
  imports, even when behavior is intended to be unchanged.

## Followup

None.
