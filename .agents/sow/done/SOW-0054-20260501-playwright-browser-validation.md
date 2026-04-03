# SOW-0054 | 2026-05-01 | playwright-browser-validation

## Status

Status: completed

Sub-state: completed and validated after reopening

## Requirements

### Purpose

Add a small browser-level validation layer for UI behavior that jsdom cannot
honestly prove, especially canvas/WebGL/visual layout and focus behavior.

### Scope

- Evaluate and add Playwright only if it fits the repo and CI.
- Design deterministic backend/test-fixture mode for browser tests.
- First critical flows: homepage render, IP lookup, feed-detail success with a
  real chart surface, admin modal focus behavior, and one entity/detail page.

### Acceptance criteria

- Browser tests are few, stable, and high signal.
- Dynamic content is controlled or masked.
- CI/runtime cost is measured.
- Component tests remain the default for ordinary behavioral coverage.

## Analysis

Created from SOW-0041 Decision 3 = (a). This SOW should start only after the
component-level page behavior tests are stronger, so Playwright does not become
a broad substitute for missing component tests.

Official Playwright documentation supports the intended shape:

- use `webServer` to start the local app server before browser tests;
- set `baseURL` so tests navigate with relative paths;
- install browser binaries explicitly for the selected browser project.

Maintainer decision for this first browser layer:

- Add Playwright, but keep it narrow and Chromium-only in the default command.
  This gives real browser coverage for the highest-risk UI behaviors without
  turning browser tests into the primary test layer.
- Use Playwright route interception as the deterministic API fixture for v1.
  This avoids coupling the browser smoke tests to a live daemon while still
  exercising the production Vite bundle and real browser event/focus/layout
  behavior.
- Keep an explicit `test:e2e:all` hook available as a manual browser-matrix
  expansion point, but do not require WebKit/Firefox in the default gate.

Reopened finding from audit cycle 2:

- `ui/e2e/api-fixtures.ts` returns a fixture `500` for unhandled `/api` or
  `/world` routes, but the test harness does not fail loudly when a browser
  test accidentally hits an unhandled API route and ignores the response.
- `ui/package.json` exposes `test:e2e:all`, but `ui/playwright.config.ts`
  currently defines only the Chromium project, so the script name suggests a
  browser matrix that does not exist.

Reopened acceptance criteria:

- Unhandled API/world fixture routes MUST fail the Playwright test run
  clearly, not only return a response that a test may ignore.
- `test:e2e:all` MUST either run an actual browser matrix or be renamed and
  documented so it does not imply broader browser coverage.
- Validation MUST include `make ui-e2e` or the equivalent package script after
  the fixture/script change.

## Plan

1. Add Playwright dependency, scripts, config, and ignored report artifacts.
2. Add deterministic API route fixtures for the browser tests.
3. Add high-signal browser tests for homepage/IP lookup, feed detail chart
   rendering, admin modal focus behavior, and an entity detail page.
4. Wire the default browser smoke into `make` and CI with explicit Chromium
   browser installation.
5. Measure and record runtime cost, then run UI/Go validation gates.

## Validation

- `pnpm --dir ui exec playwright install chromium` passed locally. Playwright
  reported fallback Ubuntu 24.04 browser builds for this workstation OS.
- Initial `make ui-e2e` exposed that `vite preview` is the wrong server for
  this app's production bundle: it treats `/static/` as the app base, while
  the embedded Go serving model serves public routes from `/` and assets from
  `/static/*`. Fixed by adding `ui/e2e/static-server.mjs`.
- Initial `pnpm --dir ui test` exposed that adding `exclude: ["e2e/**"]` to
  Vitest replaced the default dependency excludes. Fixed by using an explicit
  `include: ["src/**/*.test.{ts,tsx}"]`.
- `make ui-e2e` passed: 4 Chromium tests, about 8.1 seconds including
  production build and about 2.8 seconds for the browser test phase.
- `pnpm --dir ui test` passed: 8 files, 18 tests.
- `pnpm --dir ui lint` passed.
- `pnpm --dir ui build` passed. Existing static-font Vite warnings remain
  unrelated to this SOW and are tracked separately.
- `make ui-test` passed.
- `make build` passed.

Reopened validation status:

- Completed for the cycle-2 fixture strictness and browser-script naming
  findings.
- `ui/e2e/api-fixtures.ts` now throws after fulfilling an unhandled `/api` or
  `/world` fixture route with `500`, so ignored unhandled browser fixture
  requests fail the Playwright run clearly.
- Renamed `test:e2e:all` to `test:e2e:configured` so the script name matches
  the configured Playwright project set instead of implying a broader browser
  matrix.
- `pnpm --dir ui lint` passed.
- `pnpm --dir ui test:e2e` passed: 4 Chromium tests.
- `pnpm --dir ui test:e2e:configured` passed: 4 configured-project tests
  currently running Chromium.
- `git diff --check` passed.
- `make test` passed.
- `git diff --check` passed.

## Outcome

- Added Playwright as a UI dev dependency with `test:e2e` and `test:e2e:all`
  scripts.
- Added `make ui-e2e` and wired the Chromium browser smoke gate into CI after
  explicit Playwright browser installation.
- Added `ui/playwright.config.ts` with a production-bundle web server, base URL,
  traces on retry, screenshots on failure, and a Chromium default project.
- Added deterministic browser API fixtures for homepage, search, feed detail,
  admin, and country entity routes.
- Added four browser smoke tests covering homepage/IP lookup, feed-detail chart
  SVG layout, admin drawer focus behavior, and country entity context.
- Ignored Playwright report/result artifacts.
- Closed the reopened fixture/script hardening gap.

## Lessons extracted

- Browser tests for this repo must mimic the embedded Go serving model, not
  assume `vite preview` behaves like production. Public routes are served from
  `/`; bundled assets are served from `/static/*`.
- Vitest and Playwright test discovery must stay separate. Vitest is now
  explicitly scoped to `src/**/*.test.{ts,tsx}` so Playwright specs and
  dependency tests cannot be collected accidentally.
