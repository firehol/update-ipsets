# SOW-0068 - Frontend Admin Action Failure Tests

## Status

Status: completed

Sub-state: validated

## Requirements

### Purpose

Add black-box UI coverage for admin write-action failure states so operators see accurate feedback when actions fail.

### User Request

Review project quality against the four named project skills, identify gaps, and create SOWs for actionable improvements.

### Assistant Understanding

Facts:

- `ui/src/pages/admin-actions.test.tsx` covers successful recheck, reprocess, enable, disable, integrity recovery, and entity rebuild flows.
- Shared admin write handlers in `ui/src/test/page-scenarios.ts` return success responses.
- The frontend behavioral-testing skill expects action tests to cover visible failure states, not only happy paths.

Inferences:

- Current tests can miss broken error toasts, stuck pending state, or misleading success messages on failed admin actions.

Unknowns:

- Resolved: existing UI copy uses toast messages from mutation `onError`
  handlers; tests assert those visible failure semantics and absence of
  success toasts.

### Acceptance Criteria

- Tests use MSW at the HTTP boundary to return representative 4xx/5xx failures.
- Feed recheck/reprocess/enable/disable failure states are covered through visible UI outcomes.
- Integrity recovery and entity rebuild failure states are covered where practical.
- Tests assert no misleading success message is shown after failure.
- Add page-level `vitest-axe` coverage to existing
  `ui/src/pages/admin-actions.test.tsx` where jsdom can evaluate reliably, or
  record a specific jsdom/tooling limitation with evidence.
- Tests remain black-box: no mocked hooks, mocked children, snapshots, or internal state assertions.

## Analysis

Sources checked:

- `ui/src/pages/admin-actions.test.tsx`
- `ui/src/test/page-scenarios.ts`
- `project-frontend-behavioral-testing` skill.

Current state:

- Admin write-path tests now cover both success and representative failure
  outcomes.

Risks:

- Error-handling regressions can silently ship in the operator control plane.
- Over-specific toast wording assertions can make tests brittle; assertions should focus on visible failure semantics.

## Implications And Decisions

No implementation decision is taken in this pending SOW.

Recommended starting decision:

1. Failure scenario breadth
   - A. One generic failed admin mutation.
     - Pros: small.
     - Cons: misses per-panel failure handling.
   - B. One failure per admin action family. Recommended.
     - Pros: covers feed modal, integrity panel, and entity panel behavior.
     - Cons: more MSW fixtures.
   - C. Exhaustive failure matrix for every endpoint/status.
     - Pros: strongest.
     - Cons: high maintenance for little extra value.

## Plan

1. Inspect admin mutation components and current toast/error behavior.
2. Add failure MSW handlers for feed actions and integrity actions.
3. Add black-box tests asserting visible failure outcomes.
4. Run UI tests/lint/build and relevant project gates.

## Execution Log

### 2026-05-01

- Created from the four-skill quality review.
- Added MSW failure handlers directly in `ui/src/pages/admin-actions.test.tsx`
  for feed recheck, reprocess, enable, disable, integrity recovery, and entity
  rebuild endpoints.
- Added black-box assertions for visible error toasts and absence of misleading
  success toasts after failed actions.
- Added page-level `vitest-axe` coverage to the existing admin action page
  test.

## Validation

Acceptance criteria evidence:

- Tests use MSW at the HTTP boundary with failed POST handlers.
- Feed recheck, reprocess, disable, and enable failure states are covered.
- Integrity recovery and entity rebuild failure states are covered.
- Failure tests assert the relevant success toast is absent after the failed
  action.
- Page-level `vitest-axe` coverage was added to
  `ui/src/pages/admin-actions.test.tsx`.
- Tests remain black-box: no mocked hooks, children, snapshots, or component
  internals.

Tests or equivalent validation:

- `pnpm --dir ui test src/pages/admin-actions.test.tsx` passed: 8 tests.
- `pnpm --dir ui lint` passed.
- `pnpm --dir ui test` passed: 10 files, 27 tests.
- `pnpm --dir ui build` passed with existing unresolved static font warnings.
- `pnpm --dir ui build:budget` passed.

Real-use evidence:

- Operators now have covered feedback for failed admin write actions: the UI
  reports failure and does not show a success message for the same action.

Reviewer findings:

- Frontend behavioral-testing review found admin write workflows are success-only.
- Iterative audit cycle 5 found `ui/src/pages/admin-actions.test.tsx` is an
  existing page-level admin test without explicit axe coverage.

Same-failure scan:

- Admin action mutation coverage in `ui/src/pages/admin-actions.test.tsx` now
  includes success and failure paths for the existing action families.

Artifact maintenance gate:

- AGENTS.md: no update needed; workflow did not change.
- Runtime project skills: no update needed; existing frontend behavioral
  testing skill already requires failure-state tests.
- Specs: no product/application contract changed.
- End-user/operator docs: no documentation change needed.
- End-user/operator skills: no update needed.
- SOW lifecycle: moved from pending to current; will move to done after
  validation.

Specs update:

- Not needed.

Project skills update:

- Not needed.

End-user/operator docs update:

- Not needed.

End-user/operator skills update:

- Not needed.

Lessons:

- Admin write-action tests should pair success coverage with representative
  failed responses so toast regressions cannot silently ship.

Follow-up mapping:

- None.

## Outcome

Completed.

## Lessons Extracted

Black-box admin action tests should assert what the operator sees on failure
and also prove the matching success message is not shown.

## Followup

None.
