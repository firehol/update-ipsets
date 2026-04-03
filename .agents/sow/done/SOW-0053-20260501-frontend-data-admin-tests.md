# SOW-0053 | 2026-05-01 | frontend-data-admin-tests

## Status

Status: completed

Sub-state: completed and validated after reopening

## Requirements

### Purpose

Add black-box frontend tests for pure explorer data behavior and high-value
admin write paths after the page-level batch lands.

### Scope

- `ui/src/lib/explorer-state.test.ts` for pure filter/sort/state helpers.
- Admin write-path tests for recheck, reprocess, enable, disable, integrity
  recovery, and entity rebuild where practical.
- Page tests for country, ASN, maintainer, and `HomeIPLookup` wrapper behavior
  if not covered by SOW-0052.

### Acceptance criteria

- Tests assert observable behavior or structured pure-function output.
- Admin action tests use MSW at the HTTP boundary and assert visible UI state,
  not internal mutation calls.
- No source behavior changes unless a real bug is uncovered and recorded.
- Reopened acceptance criterion: the no-URL `HomeIPLookup` client-IP
  auto-detect branch must be covered through the visible homepage wrapper,
  including the detected-IP text and the seeded search input value.

## Analysis

Created from SOW-0041 Decision 2 = (b). This is separate from SOW-0052 because
admin write paths and pure data helpers have different fixtures and review
risks.

Reopened from iterative audit cycle 3:

- SOW-0041 explicitly identified `HomeIPLookup` URL-prefill and client-IP
  auto-detect wrapper behavior as a remaining frontend behavioral-testing gap.
- This SOW scoped `HomeIPLookup` wrapper behavior, but its completed outcome
  did not include the client-IP auto-detect branch.
- Existing `ui/src/pages/home.test.tsx` covers URL hydration and clear behavior,
  but not the no-URL `/api/v1/client-ip` branch or visible
  "Detected from your connection" state.

## Plan

1. Inspect the explorer-state helper contracts and entity page/admin action
   components before writing tests.
2. Add pure tests for explorer filtering, sorting, URL state, and critical
   filter fields.
3. Add admin write-path tests through MSW for high-value visible action flows.
4. Add page tests for country, ASN, and maintainer pages where practical.
5. Run UI lint/test/build and relevant project gates.
6. Update specs/skills only if the tests reveal durable product or testing
   lessons, then close this SOW.

## Validation

- `pnpm --dir ui test src/lib/explorer-state.test.ts src/pages/entities.test.tsx src/pages/admin-actions.test.tsx` passed:
  3 files, 11 tests.
- `pnpm --dir ui test` passed: 8 files, 18 tests.
- `pnpm --dir ui lint` passed.
- `pnpm --dir ui build` passed. Existing static-font Vite warnings remain
  unrelated to this SOW and are tracked separately.
- `make ui-test` passed.
- `make build` passed.
- `make test` passed.
- `git diff --check` passed.

Reopened validation status:

- Completed for `HomeIPLookup` no-URL client-IP auto-detect coverage.
- Added `ui/src/pages/home.test.tsx` coverage through the visible homepage
  wrapper and MSW `/api/v1/client-ip` handler. The test asserts both
  "Detected from your connection" text and the seeded search input value.
- `pnpm --dir ui test src/pages/home.test.tsx` passed: 1 file, 3 tests.
- `pnpm --dir ui lint` passed.
- `pnpm --dir ui test` passed: 9 files, 22 tests.
- `pnpm --dir ui build` passed. Existing Inter display font runtime
  resolution warnings remain unchanged.
- `git diff --check` passed.

## Outcome

- Completed after cycle 3 reopening for missing `HomeIPLookup` client-IP
  auto-detect coverage.
- Previously added pure tests for explorer URL state, critical-infrastructure filters,
  health visibility, and sorting.
- Added page-level tests for country, ASN, and maintainer detail routes backed
  by provider-shaped MSW fixtures.
- Added admin write-path tests for feed recheck, reprocess, enable, disable,
  integrity reprocess, and entity-artifact rebuild actions through the real
  HTTP boundary and visible UI outcomes.
- Extended shared frontend page fixtures so subsequent page tests can reuse the
  same API-shaped scenarios without mocking component internals.

## Lessons extracted

No new durable spec or skill lessons beyond SOW-0052. The existing frontend
behavioral-testing rule remains correct: page/admin tests should exercise
observable UI and MSW-backed HTTP boundaries, not callbacks or component
internals.
