# SOW-0034 | 2026-04-30 | frontend behavioral test foundation

## Status

completed

## Requirements

### Purpose

Add a real frontend behavioral test foundation for the React/TypeScript UI so
frontend changes can be validated by user-visible behavior, accessible roles,
and backend-boundary requests instead of internal implementation details.

### Scope

- `ui/package.json` and `ui/pnpm-lock.yaml`
- Vitest/jsdom/MSW/test setup files
- initial black-box tests for high-value UI surfaces
- Makefile and CI wiring for the new UI test gate
- durable project testing/reviewing skills

Hard rule: tests must not assert component internals, hooks, render counts,
mocked children, mocked TanStack Query, or arbitrary DOM snapshots.

## Implementation

### Test stack

Installed the component/integration layer:

- `vitest@4.1.5`
- `@vitest/coverage-v8@4.1.5`
- `jsdom@29.1.1`
- `@testing-library/react@16.3.2`
- `@testing-library/user-event@14.6.1`
- `@testing-library/jest-dom@6.9.1`
- `@testing-library/dom@10.4.1`
- `msw@2.14.2`
- `vitest-axe@0.1.0`
- `eslint-plugin-testing-library@7.16.2`

Package registry checks showed:

- Vitest 4.1.5 supports Vite `^6 || ^7 || ^8`.
- Testing Library React 16.3.2 supports React `^18 || ^19`.
- MSW 2.14.2 supports the current TypeScript range.
- `eslint-plugin-testing-library` supports ESLint `^8.57 || ^9 || ^10`.

### Harness

Added:

- `ui/vitest.config.ts`
- `ui/vitest.setup.ts`
- `ui/src/test/render.tsx`
- `ui/src/test/msw-server.ts`
- `ui/src/test/msw-handlers.ts`
- `ui/src/test/fixtures.ts`

The harness:

- uses jsdom
- starts MSW with `onUnhandledRequest: "error"`
- provides QueryClient, MemoryRouter, ThemeProvider, and TooltipProvider
- disables query retries and cache retention for deterministic tests
- includes Radix/jsdom polyfills for ResizeObserver, IntersectionObserver,
  matchMedia, scrollIntoView, and pointer capture APIs
- supports `vitest-axe` with local rule overrides for jsdom-impossible checks

### Initial tests

Added four black-box tests:

- `ui/src/components/ip-search/ip-search-surface.test.tsx`
  - types an IP, submits the form, proves `/api/v1/search` received the right
    query parameters, and verifies the visible matching feed result.
- `ui/src/components/home/home-explorer.test.tsx`
  - filters visible feeds through the public explorer search and proves the
    same filtered set remains visible after switching to table view.
- `ui/src/components/admin/feeds-table.test.tsx`
  - filters admin rows and proves the selected row opens via keyboard Enter.
- `ui/src/pages/feed-detail.test.tsx`
  - renders the feed-detail route on a 404 response and verifies the
    user-facing not-found state.

Small production accessibility fixes were required so tests could query the UI
through accessible names:

- `ui/src/components/ip-search/ip-search-surface.tsx`
  - IP search inputs now expose scope-aware `aria-label` text.
- `ui/src/components/home/home-explorer-filter-rail.tsx`
  - explorer search input now has `aria-label="Filter feeds"`.
- `ui/src/components/admin/feeds-table.tsx`
  - admin feed-table search input now has `aria-label="Search admin feeds"`.

### Gates

- Added `test`, `test:watch`, and `test:coverage` scripts to
  `ui/package.json`.
- Added `make ui-test`.
- CI now installs UI dependencies and runs:
  - `make ui-test`
  - `pnpm --dir ui lint`
  - `pnpm --dir ui build`
- Added `/ui/coverage/` to `.gitignore`.
- `ui/eslint.config.js` now applies `eslint-plugin-testing-library` to
  `*.test.{ts,tsx}` files.

## Evidence-Backed Non-Goals

- Project Playwright tests were not added. There is no deterministic backend
  fixture mode for the full public/admin SPA. Browser tests that depend on a
  live development daemon would be flaky, data-dependent, and less trustworthy
  than the MSW-backed component layer added here. Browser validation remains
  appropriate for specific WebGL/canvas checks, as done in SOW-0033.
- Visual snapshot baselines were not added. Without a deterministic browser
  test backend and stable dynamic-data masking, snapshots would mostly encode
  local workstation state.
- Bundle-size budget tooling was not added. It is performance governance, not
  frontend behavioral testing.
- Storybook was not added. It helps component review, but it is not a test
  harness and would not validate user flows by itself.
- A coverage threshold was not added. Coverage reports are useful for finding
  untested files, but a global percentage threshold would incentivize shallow
  tests instead of behavioral assertions.
- `eslint-plugin-jest-dom` was not added because its current peer dependency
  range excludes ESLint 10. The compatible `eslint-plugin-testing-library`
  gate was added.

No valid item was left as untracked prose-only intent.

## Durable Memory Updates

- `.agents/skills/project-testing/SKILL.md` now records the UI test stack,
  `make ui-test`, MSW/Testing Library rules, colocated test convention, and
  axe usage.
- `.agents/skills/project-reviewing/SKILL.md` now records the UI test review
  checks.
- `.agents/skills/frontend-behavioral-testing/SKILL.md` now reflects the
  installed stack and canonical setup files.

## Validation

- `pnpm --dir ui install --frozen-lockfile` passed.
- `pnpm --dir ui test` passed: 4 files, 4 tests.
- `make ui-test` passed: 4 files, 4 tests.
- `pnpm --dir ui lint` passed.
- `pnpm --dir ui build` passed.
- `pnpm --dir ui test:coverage` passed and produced an informational baseline:
  statements 15.99%, branches 10.15%, functions 15.31%, lines 16.77%.

Observed warnings:

- Vitest/Node prints `--localstorage-file was provided without a valid path`.
  This warning does not fail tests and appears before test execution.
- `pnpm install` reports ignored MSW build scripts. The node test server uses
  MSW directly and does not need a generated service-worker file.
- Existing React 19 peer warnings remain for `@visx/*` and
  `react-simple-maps`; they predate this test harness.
