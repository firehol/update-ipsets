# SOW-0052 | 2026-05-01 | frontend-page-behavior-tests

## Status

Status: completed

Sub-state: completed and validated after reopening

## Requirements

### Purpose

Raise the frontend test suite from leaf-component smoke tests to black-box
page-level behavioral coverage for the highest-risk user-visible UI flows.

### User request quoted verbatim

> the next 4 sows 31-34 are yours, about the code quality and testing of this application. I don't want to be involved. Consider them a gift from me. I have also researched 4 related skills which you can use while coding. I need you to review them, decide what is valid and what is not, research the application properly, and implement the ones you believe are justified. No questions for me.

### Assistant understanding

- SOW-0041 found that the first UI tests pass but mostly render leaf
  components instead of the pages users exercise.
- The only Category A test smell is a callback-prop assertion in
  `feeds-table.test.tsx`; it should be rewritten against the page-level modal
  outcome.
- This SOW covers the first behavioral-test implementation batch only. It must
  not batch data-layer/admin write-path tests, Playwright setup, bundle budgets,
  or test-tooling hygiene.
- Iterative audit cycle 4 found a closure gap: this SOW accepted page-level
  axe checks where jsdom can evaluate them, but the feed-detail success-path
  test still has no `vitest-axe` assertion or evidence-backed jsdom exception.

### Acceptance criteria

- Rewrite the admin feeds-table callback-prop assertion into a page-level test
  that opens the feed modal and asserts visible modal content.
- Add a page-level `HomePage` behavioral test that exercises query-backed data
  flow, feed filtering, and explorer view switching through visible UI.
- Add a page-level `HomePage` or wrapper behavioral test for IP lookup
  URL/result behavior without mocking hooks.
- Add a `FeedDetailPage` success-path behavioral test that loads via MSW and
  asserts visible feed content.
- Add `axe` checks to the new page-level tests where jsdom can evaluate the
  rendered tree reliably.
- Reopened scope: add feed-detail success-path page-level `vitest-axe`
  coverage, or record a specific jsdom/tooling limitation with evidence if the
  rendered tree cannot be evaluated reliably.
- Shared MSW scenario helpers are added only where they reduce real
  duplication.
- Tests stay black-box: no mocked children, mocked hooks, callback-prop
  assertions, render-count assertions, snapshots, or `data-testid` selectors.
- Validation includes `pnpm --dir ui lint`, `pnpm --dir ui test`,
  `pnpm --dir ui build`, `make ui-test`, `make build`, and relevant Go gates.

## Analysis

Starting evidence comes from SOW-0041:

- `ui/src/components/admin/feeds-table.test.tsx` asserts
  `onFeedClick` invocation instead of the visible modal outcome.
- `ui/src/components/home/home-explorer.test.tsx` tests `<HomeExplorer />`
  directly, not `<HomePage />` query composition.
- `ui/src/components/ip-search/ip-search-surface.test.tsx` tests
  `<IPSearchSurface />` directly, not the homepage wrapper behavior.
- `ui/src/pages/feed-detail.test.tsx` covers only the missing-feed path.

This SOW is medium risk because it expands test harness coverage without
changing product behavior. The main risk is writing brittle tests that encode
component structure instead of user-visible outcomes.

## Plan

1. Inspect current test fixtures, handlers, and the target page components.
2. Add reusable MSW scenario helpers if needed.
3. Rewrite the admin table anti-pattern as page-level behavior.
4. Add the `HomePage` and `FeedDetailPage` page-level tests.
5. Run UI gates and targeted full-project gates.
6. Update test/review skills with any durable lessons, then close the SOW.

## Execution log

- Replaced the admin feeds-table callback-prop assertion with a page-level
  admin test that loads real admin queries through MSW, filters the table, opens
  the feed drawer by keyboard, and asserts visible drawer content.
- Added shared page MSW scenarios under `ui/src/test/page-scenarios.ts` for the
  homepage, admin page, and feed detail page.
- Added page-level homepage tests for query-backed explorer data, filtering,
  view switching, critical-infrastructure filter visibility, and URL-seeded IP
  lookup results.
- Added a feed-detail success-path test that exercises the published API
  surfaces used by the feed page and asserts visible feed, about,
  critical-infrastructure, ASN, and geo content.
- The new axe checks exposed source issues rather than test-only failures:
  homepage native selects lacked accessible names; admin table had an empty
  action-column header; admin feed rows were exposed as focusable buttons while
  containing a nested public-page link. Fixed the source semantics.
- Updated the homepage/admin specs and UI testing skills so new page-level
  axe findings are handled as product bugs unless the rule is genuinely
  jsdom-impossible.
- Reopened after iterative audit cycle 4 found that feed-detail page-level axe
  coverage was still missing from `ui/src/pages/feed-detail.test.tsx`.

## Validation

- `pnpm --dir ui test` passed: 5 files, 7 tests.
- `pnpm --dir ui lint` passed.
- `pnpm --dir ui build` passed. Vite kept the existing unresolved static-font
  build warnings.
- `make ui-test` passed: 5 files, 7 tests.
- `make build` passed.
- `make test` passed.
- `git diff --check` passed.
- Vitest still prints the known `--localstorage-file` warning; SOW-0055 tracks
  that tooling hygiene item.

## Outcome

Completed after reopening. The frontend test suite includes page-level
behavioral coverage for the admin feed drawer flow, homepage explorer/IP
lookup, and feed-detail success path, including feed-detail page-level axe
coverage.

Reopened validation:

- Added `vitest-axe` coverage to
  `ui/src/pages/feed-detail.test.tsx` for the feed-detail success path.
- `pnpm --dir ui test src/pages/feed-detail.test.tsx` passed: 1 file, 2 tests.
- `pnpm --dir ui lint` passed.
- `pnpm --dir ui test` passed: 9 files, 22 tests.
- `pnpm --dir ui build` passed. Existing Inter display font runtime
  resolution warnings remain unchanged.
- `git diff --check` passed.

## Lessons extracted

- Page-level axe checks are useful because they reveal source accessibility
  defects that leaf tests miss. Do not suppress actionable axe failures; fix
  the source semantics first.
- For dense admin tables, row-level pointer convenience must be separate from
  keyboard accessibility. Use native buttons/links for keyboard actions instead
  of making a row a focusable control around nested links.
