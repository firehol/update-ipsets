---
name: project-frontend-behavioral-testing
description: "Black-box behavioral testing for React 19 + TypeScript UIs: test what users see and do, never internals. Vitest + Testing Library + MSW + Playwright patterns, anti-mock guidance, and AI-generated-test pitfalls. Use when writing or reviewing UI tests."
---

## TL;DR

- A UI's contract is what users perceive, what users can do, where they end up, and what requests it makes to the backend; everything else is an implementation detail.
- Render real components against a real network seam (MSW), drive them with `userEvent`, and assert on accessible roles/text/URL/visible state. Never assert on hooks, state, render counts, or component-tree shape.
- Installed stack for this repo: Vitest + `@testing-library/react` +
  `@testing-library/user-event` + `@testing-library/jest-dom` + MSW v2 for
  component/integration tests, plus `vitest-axe` for a11y checks. Playwright
  remains a browser-validation tool for canvas/WebGL/visual flows, not the
  default component test layer.
- Charts, maps, and Three.js render to canvas/WebGL that jsdom cannot inspect. Test the data layer separately and use a tiny set of Playwright visual regressions only on flows where pixels matter.
- LLM-generated UI tests have predictable failure modes: `data-testid` everywhere, mocked children, mocked `useQuery`, asserting on prop calls, snapshots locking arbitrary DOM. Reject them on review.

## Status of this skill

The component-level frontend test framework is installed in `ui/package.json`.
Canonical setup files are `ui/vitest.config.ts`, `ui/vitest.setup.ts`, and
`ui/src/test/`. Use `make ui-test` or `pnpm --dir ui test`. Test files are
also covered by `eslint-plugin-testing-library` through `ui/eslint.config.js`.
The Vitest package scripts deliberately pass
`--no-experimental-webstorage` through `NODE_OPTIONS` on Node 25 to avoid
Node's global localStorage persistence warning in jsdom tests.
The Playwright package scripts clear `NO_COLOR`; otherwise local shells that
set `NO_COLOR` can conflict with Playwright's forced color output and produce
Node process warnings even when the browser tests pass.

## What's the contract of a UI?

The contract is what an end user can observe and do:

- **DOM the user perceives**: visible text, accessible roles, labels, status messages, focus, error text, disabled/enabled states, URL bar.
- **Interactions the user can perform**: clicks, typing, keyboard navigation, form submit, drag where applicable.
- **Navigation outcomes**: which route is rendered, which content is shown after a transition.
- **Network requests visible at the boundary with the server**: which endpoints are called, with which parameters, in response to which user actions. The user cannot see this directly, but it is part of the published contract with the backend.

The following are NOT part of the contract and must not be tested directly:

- Internal state values (`useState`, refs, derived memos).
- Whether a particular hook ran or how often.
- Component tree shape (which child component rendered where).
- Exact prop values passed between components.
- Render counts, reconciliation order, or effect ordering.
- CSS class names, except where they are the only available signal for an observable behavior (rare; prefer `aria-*` or `role`).
- Implementation files imported by the component.

If a refactor that preserves user-visible behavior breaks a test, the test was testing implementation, not contract. Delete or rewrite it.

## Test toolchain

The component/integration layer is installed. Justifications follow.

| Layer | Tool | Why |
|---|---|---|
| Test runner | **Vitest 4.x** | Vite-native, shares the project's existing Vite 8 config, near-Jest-compat API, parallel by default, watch mode. Same runner for unit and component tests. |
| DOM environment | **jsdom** (default) | Battle-tested, complete enough for Testing Library queries. `happy-dom` is faster but trades edge-case completeness; start with jsdom and switch only with measured benefit. |
| Component queries | **@testing-library/react** | Encourages role/label/text queries; explicitly designed to make implementation-detail testing hard. |
| Interactions | **@testing-library/user-event v14+** | Realistic event sequences (focus, pointer, keyboard); async API surfaces real timing bugs; supersedes `fireEvent`. |
| DOM assertions | **@testing-library/jest-dom** | Adds `toBeVisible`, `toHaveAccessibleName`, `toHaveAttribute`, etc. Vitest-compatible. |
| Network seam | **MSW v2** (`setupServer` for node) | Mocks at the network layer, not the HTTP-client layer. Tests stay agnostic of `fetch`/`axios`/TanStack Query implementation. |
| Accessibility checks | **vitest-axe** | Vitest-native fork of `jest-axe`; runs `axe-core` against the rendered tree. |
| End-to-end | **Playwright** | Real browsers (Chromium, Firefox, WebKit), built-in tracing, parallelism, network interception, optional visual regression. Use for a small set of critical flows. |
| Visual regression | **Playwright `toHaveScreenshot()`** | Only on flows where pixels are part of the contract (e.g. brand logo, hero layout). Run baselines in CI, mask dynamic content, disable animations. |

Tools deliberately NOT recommended:

- `enzyme` — encourages shallow rendering and instance-property access; effectively unmaintained.
- Vitest browser mode for first wave — newer surface area, project must evaluate before adopting; jsdom + MSW covers the bulk of behavioral tests today. Verify before committing.
- Storybook — useful for development and visual review, but not a substitute for behavioral tests. Out of scope here.
- Snapshot tests as the primary tool — see "What NOT to test".

Verify versions before relying: Vitest 4.x, React 19.2, `@testing-library/react` (matching React 19 peer), `@testing-library/user-event` v14+, MSW v2.x, Playwright 1.5x+. React 19 + Vitest is reported to work without special configuration in current docs; check the React Testing Library release notes pinned to your React 19 minor before committing.

## Current setup

This is the installed component-level test setup. Keep new tests aligned with
these files instead of adding a second harness.

### Installed dependencies

```bash
pnpm --dir ui add -D \
  vitest \
  @vitest/coverage-v8 \
  jsdom \
  @testing-library/react \
  @testing-library/user-event \
  @testing-library/jest-dom \
  vitest-axe \
  msw
```

### `ui/vitest.config.ts`

```ts
import { defineConfig, mergeConfig } from "vitest/config";
import viteConfig from "./vite.config";

export default mergeConfig(
  viteConfig,
  defineConfig({
    test: {
      environment: "jsdom",
      globals: true,
      setupFiles: ["./vitest.setup.ts"],
      css: false,
      restoreMocks: true,
      clearMocks: true,
      coverage: {
        provider: "v8",
        reporter: ["text", "lcov"],
        include: ["src/**/*.{ts,tsx}"],
        exclude: ["src/**/*.d.ts", "src/main.tsx"],
      },
    },
  }),
);
```

### `ui/vitest.setup.ts`

Polyfills Radix and chart libraries assume; without them tests crash inside jsdom.

```ts
import "@testing-library/jest-dom/vitest";
import { afterAll, afterEach, beforeAll } from "vitest";
import { server } from "./src/test/msw-server";

// jsdom does not implement these; Radix popovers/menus, Recharts, and
// scroll containers expect them to exist.
class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}
class IntersectionObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
  takeRecords() {
    return [];
  }
  root = null;
  rootMargin = "";
  thresholds = [];
}

if (!("ResizeObserver" in globalThis)) {
  // @ts-expect-error stub
  globalThis.ResizeObserver = ResizeObserverStub;
}
if (!("IntersectionObserver" in globalThis)) {
  // @ts-expect-error stub
  globalThis.IntersectionObserver = IntersectionObserverStub;
}

if (!window.matchMedia) {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: (query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener: () => {},
      removeEventListener: () => {},
      addListener: () => {},
      removeListener: () => {},
      dispatchEvent: () => false,
    }),
  });
}

// Radix uses these on pointer/keyboard interactions; jsdom does not.
if (!Element.prototype.scrollIntoView) {
  Element.prototype.scrollIntoView = function () {};
}
if (!Element.prototype.hasPointerCapture) {
  // @ts-expect-error stub
  Element.prototype.hasPointerCapture = () => false;
}
if (!Element.prototype.releasePointerCapture) {
  // @ts-expect-error stub
  Element.prototype.releasePointerCapture = () => {};
}

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());
```

`onUnhandledRequest: "error"` makes unmocked network calls fail loudly. The point is to catch tests that accidentally rely on real backends and to surface routes the test author forgot to mock.

### `ui/src/test/msw-server.ts`

```ts
import { setupServer } from "msw/node";
import { handlers } from "./msw-handlers";

export const server = setupServer(...handlers);
```

### `ui/src/test/msw-handlers.ts`

Default handlers describe the success-path shape of every API the UI talks to. Per-test handlers override these via `server.use(...)` for failure/edge cases.

```ts
import { http, HttpResponse } from "msw";

export const handlers = [
  http.get("/api/v1/sets", () =>
    HttpResponse.json({ sets: [], generated_at: "2026-01-01T00:00:00Z" }),
  ),
  http.get("/api/v1/status", () =>
    HttpResponse.json({ running: false, queue_depth: 0 }),
  ),
];
```

### `ui/src/test/render.tsx`

A single render helper that wires the providers a real page would use. Components that need extra providers extend, never replace, this helper.

```tsx
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, type RenderOptions } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import type { ReactElement, ReactNode } from "react";

export function makeQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0, staleTime: 0 },
      mutations: { retry: false },
    },
  });
}

interface UIRenderOptions extends Omit<RenderOptions, "wrapper"> {
  route?: string;
  client?: QueryClient;
}

export function renderUI(ui: ReactElement, options: UIRenderOptions = {}) {
  const client = options.client ?? makeQueryClient();
  const route = options.route ?? "/";

  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={client}>
        <MemoryRouter initialEntries={[route]}>{children}</MemoryRouter>
      </QueryClientProvider>
    );
  }

  return {
    user: userEvent.setup(),
    ...render(ui, { wrapper: Wrapper, ...options }),
  };
}
```

### `package.json` scripts

```json
{
  "scripts": {
    "test": "vitest run",
    "test:watch": "vitest",
    "test:coverage": "vitest run --coverage"
  }
}
```

### CI integration

Wire `pnpm --dir ui test -- --run` into the same workflow that already runs `pnpm --dir ui build` and `pnpm --dir ui lint`. Coverage thresholds should follow `project-testing`'s pattern of being meaningful, not vanity numbers; do not enforce a global percentage that incentivizes trivial tests.

## Query priority

Use queries in this order. Each step down is a step further from the user.

1. **`getByRole`** with `name` — what assistive tech sees. `screen.getByRole("button", { name: /save changes/i })`.
2. **`getByLabelText`** — form fields with associated labels. `screen.getByLabelText(/email/i)`.
3. **`getByPlaceholderText`** — only when no label is present (label is preferred).
4. **`getByText`** — for non-interactive text content the user reads.
5. **`getByDisplayValue`** — for verifying an input's current value.
6. **`getByAltText` / `getByTitle`** — semantic but weaker.
7. **`getByTestId`** — last resort.

`data-testid` is acceptable only when (a) the element has no role/label, (b) adding a role would mislead assistive tech, and (c) the test would otherwise depend on brittle text or class lookups. Treat every `data-testid` as a small a11y debt.

`screen` is preferred over destructured queries from `render`. It eliminates accidental scope drift across helpers and matches how Testing Library docs read.

`*ByRole` is more expensive than `*ByText` because it parses every element's accessible name. In practice the cost is negligible for component tests; do not pre-optimize by reaching for `getByText`.

## Interactions

`userEvent` v14+ is mandatory. `fireEvent` is for cases where the real event sequence is provably the wrong granularity (extremely rare).

```tsx
const { user } = renderUI(<FeedModal />);
await user.click(screen.getByRole("button", { name: /save/i }));
await user.type(screen.getByLabelText(/url/i), "https://example.test/feed.txt");
```

Pitfalls:

- Always `await` every `user.*` call. v14 returns promises and synchronous calls silently drop work.
- Call `userEvent.setup()` once per test (the `renderUI` helper does this) and reuse the returned `user`. Do not call `userEvent.setup()` per interaction; pointer state is per-instance and resetting it loses hover/enter/leave history.
- Do not call `userEvent.setup()` inside `beforeEach` of a shared describe; fresh test isolation already happens at the test level. Co-locate setup with `render`.
- Fake timers and `userEvent` together require `userEvent.setup({ advanceTimers: vi.advanceTimersByTime })`. Default to real timers; reach for fake timers only when the assertion depends on `setTimeout`/`setInterval` and waiting for real time would be slow.

## TanStack Query in tests

The UI uses TanStack Query 5. Tests must let `useQuery` run for real against MSW handlers. Mocking `useQuery` is forbidden — it removes the contract being tested (request shape, loading/error states, refetch behavior).

Required `QueryClient` defaults for tests:

- `retry: false` — otherwise transient mocked failures retry and tests get slow or flaky.
- `gcTime: 0` — releases caches between tests; prevents leakage across files when clients are shared.
- `staleTime: 0` — predictable refetch behavior on remount.

The `renderUI` helper above already does this. Override per-test only when the test is specifically about retry or stale behavior:

```tsx
const client = new QueryClient({
  defaultOptions: { queries: { retry: 1, gcTime: 0 } },
});
renderUI(<Page />, { client });
```

## Routing

Render real router code with `MemoryRouter` and an explicit `initialEntries`; assert on user-visible content and on URL via `useLocation` only when the test directly cares about the URL.

```tsx
const { user } = renderUI(<App />, { route: "/admin/feeds" });

await user.click(await screen.findByRole("link", { name: /home/i }));

expect(
  await screen.findByRole("heading", { name: /feed explorer/i }),
).toBeInTheDocument();
```

For React Router 7 data routes (loaders/actions), prefer `createMemoryRouter` with explicit route definitions in the test; do not stub `useLoaderData` to return canned values. The loader's data-shape is the contract under test.

## Forms

Test the form by filling it out, submitting it, and asserting on:

- the success state the user sees (toast, redirect, updated row),
- the failure state for each error path (validation error text, server error text),
- and the network request that was issued (handled via MSW).

```tsx
const { user } = renderUI(<FeedModal mode="create" />);

await user.type(screen.getByLabelText(/feed name/i), "example.txt");
await user.type(screen.getByLabelText(/source url/i), "https://e.test/f.txt");
await user.click(screen.getByRole("button", { name: /save feed/i }));

expect(
  await screen.findByRole("status", { name: /feed created/i }),
).toBeInTheDocument();
```

For server-side validation errors, set up an MSW override:

```tsx
server.use(
  http.post("/api/v1/admin/feeds", () =>
    HttpResponse.json({ error: "duplicate name" }, { status: 409 }),
  ),
);
```

## Accessibility

- Use `vitest-axe` to assert no violations on rendered output of every page-level component test. The cost is small and it catches missing labels, contrast issues (where applicable), and ARIA misuse.
- When `vitest-axe` reports actionable violations such as missing accessible
  names, empty table headers, invalid dialog labelling, or nested interactive
  controls, fix the component. Do not suppress those rules just to make the
  test green; reserve rule disabling for jsdom-impossible checks such as color
  contrast.
- Assert on focus management explicitly: after closing a dialog, focus must return to the trigger. `expect(screen.getByRole("button", { name: /open settings/i })).toHaveFocus()`.
- Drive keyboard flows with `user.tab()`, `user.keyboard("{Enter}")`, and `user.keyboard("{Escape}")`. Verify the same behavior is reachable without a mouse.

```tsx
import { axe } from "vitest-axe";

const { container } = renderUI(<HomeExplorer />);
expect(await axe(container)).toHaveNoViolations();
```

Limitations: `axe-core` cannot detect color-contrast issues in jsdom (no layout engine). Catch those in Playwright or design review.

## Visualizations

Recharts/D3/VisX/Three.js render to SVG, canvas, or WebGL. jsdom has no canvas implementation and no WebGL. Behavioral tests for visualizations should:

1. **Test the data layer in isolation** — unit-test the function that maps API data to chart data (`buildSeries`, `bucketByCountry`, `aggregateByCidr`). This is plain data, no DOM. Heavy logic belongs here.
2. **Test the accessible summary** — every chart should expose an alternative representation: a visually-hidden `<table>`, an `aria-label` summary, or a screen-reader-only list. Assert on those.
3. **Test interactive controls** — buttons, filters, legends, tooltips: all are real DOM. Verify the data they request (via MSW) and the visible state changes (e.g., switching from "by country" to "by ASN" triggers a new request and updates the visible heading).
4. **Use Playwright sparingly for visual regression** — only when pixel layout is part of the contract. Mask dynamic content; run baselines in CI.

What does NOT belong in component tests:

- Asserting on canvas pixel positions.
- Asserting on `<svg>` `path d=` strings (brittle to D3 internals).
- Asserting that a Three.js mesh exists.

If the test cannot observe the contract through accessible DOM, the contract is wrong (missing a11y) or the test is in the wrong layer (move to Playwright or unit-test the data function).

## Network as the contract boundary

MSW intercepts at the request level. Tests assert on the **user-visible consequence** of a request, not on whether MSW received the request.

GOOD:

```tsx
await user.click(screen.getByRole("button", { name: /apply filter/i }));
expect(await screen.findByText(/showing 12 feeds/i)).toBeInTheDocument();
```

ACCEPTABLE when the contract is "this button issues request X with payload Y" and there is no observable UI change:

```tsx
const requests: Request[] = [];
server.events.on("request:start", ({ request }) => requests.push(request));

await user.click(screen.getByRole("button", { name: /reprocess/i }));
await waitFor(() => expect(requests).toHaveLength(1));
expect(requests[0].url).toContain("/api/v1/admin/reprocess");
```

Use this pattern only when there is genuinely no DOM change for the user (e.g., fire-and-forget admin action with no immediate feedback). If the user sees a toast or a status flip, assert on that instead.

BAD:

```tsx
// Asserts on internal MSW state, not on what the user sees.
expect(server.listHandlers()).toHaveLength(1);
```

## What NOT to test

- **Internal state**: `useState`, refs, derived memos. If the state matters, it produces a visible effect; assert on the effect.
- **Hook calls**: do not spy on `useSomething`, do not assert "the hook ran". The contract is the rendered output and the network behavior, not which hook produced them.
- **Render counts**: not a user-visible property. Performance budgets belong in benchmarks or Playwright traces, not in unit tests.
- **Component tree shape**: do not test "child component X received prop Y". Render the real tree; assert on what the user sees.
- **Mocked children**: replacing `<HomeChart />` with `<div data-testid="chart-mock" />` and asserting on `data-testid` is a contract test of the mock, not the page. If the chart is too heavy, refactor the page to make the chart's data path testable in isolation, or move that test to Playwright.
- **Mocked hooks**: never `vi.mock("@tanstack/react-query")`, never spy on `useQuery`. Mock the network with MSW and let the hook run.
- **Class names as behavior**: `expect(el).toHaveClass("active")` is acceptable when the class is the only signal for an observable state and the component does not expose a role/`aria-pressed`/text. In that case, treat it as a smell and add a real semantic signal.
- **Snapshot tests of large DOM**: forbidden. They lock arbitrary structure, fail on safe refactors, and are accepted by reviewers who skim. Inline snapshots of small, meaningful output (a JSON config, a parsed query string) are the only acceptable use. If you do snapshot, it must be `toMatchInlineSnapshot()` and ≤10 lines.
- **Tests that pass when the component is broken**: if the test never fails on a regression, delete it. Common form: `expect(container).toBeTruthy()`, `expect(component).toBeDefined()`.

## E2E with Playwright

Use Playwright for:

- Critical paths the user must complete (load the homepage, look up an IP, find a feed, view feed details, log in to admin).
- Cross-browser sanity (Chromium + WebKit at minimum).
- Real network behavior against the daemon's test mode where feasible.
- Visual regression on a small set of public pages.

Do not use Playwright for:

- Replacing component tests. They are slower and harder to maintain.
- Testing every UI permutation. Component tests cover that better.

For this repo, run against the production build through
`ui/e2e/static-server.mjs` plus deterministic Playwright route fixtures. Do not
use `vite preview` for the browser smoke suite: the production bundle's
`/static/` asset base matches the embedded Go server, but `vite preview` also
treats `/static/` as the app base and breaks direct public routes.

Use Playwright's `webServer` config to start the browser-test server:

```ts
// ui/playwright.config.ts (sketch)
export default defineConfig({
  testDir: "./e2e",
  use: {
    baseURL: "http://localhost:4173",
    trace: "on-first-retry",
  },
  webServer: {
    command: "node e2e/static-server.mjs --port 4173",
    url: "http://localhost:4173",
    reuseExistingServer: !process.env.CI,
  },
});
```

Keep e2e tests small (≤20 critical-path tests). Each one is expensive in CI minutes and time-to-feedback.

## Flake hunting

Flake is rejected on review. Causes and fixes:

- **Hardcoded timeouts (`setTimeout`, `await sleep(500)`)**: replace with `findBy*` or `waitFor` keyed on an actual condition.
- **Multiple assertions in one `waitFor`**: split. Each assertion gets its own deterministic check.
- **Side effects inside `waitFor`**: forbidden — `waitFor` retries, side effects multiply.
- **Shared mutable test fixtures**: every test gets its own `QueryClient`, its own MSW handlers (via `resetHandlers` after each), its own user instance.
- **Real wall-clock timers in components that schedule work**: switch to deterministic test data, not fake timers, when possible. Fake timers are a last resort and require `userEvent.setup({ advanceTimers })`.
- **Animations**: disable in tests via CSS or component prop. Visual snapshots must run with `animations: 'disabled'`.
- **Unhandled requests**: MSW with `onUnhandledRequest: "error"` catches them at source.
- **`waitFor` until it passes**: if the only way to make a test green is to extend the timeout, the test is wrong. Find the missing condition.
- **Order-dependent tests**: each test must pass in isolation. Run with `--shuffle` periodically to surface coupling.

## Working with AI-generated UI tests

LLM-generated tests look plausible and fail every behavioral standard above. Reviewers must check for these patterns explicitly. Cite each finding to the offending line.

### Common failure modes (with grep heuristics)

| Pattern | Why it fails | Grep |
|---|---|---|
| `data-testid` as primary query | Bypasses semantics, locks tests to refactor-fragile attributes | `rg 'getByTestId\|data-testid' ui/src/**/*.test.*` |
| Mocking `useQuery`/TanStack Query hooks | Removes the contract under test | `rg 'vi\.mock\(.*react-query\|mock.*useQuery' ui/src/**/*.test.*` |
| Mocked child components | Tests the mock, not the page | `rg 'vi\.mock\(.*\.\./components' ui/src/**/*.test.*` |
| Snapshot of rendered DOM | Locks arbitrary structure | `rg 'toMatchSnapshot\(\)\|toMatchInlineSnapshot' ui/src/**/*.test.*` |
| Asserting on prop calls of mocked children | Implementation detail | `rg 'toHaveBeenCalledWith.*expect\.objectContaining' ui/src/**/*.test.*` |
| `act()` wrapped manually | Cargo-cult; RTL wraps already | `rg '\bact\(' ui/src/**/*.test.*` |
| `fireEvent` instead of `user-event` | Skips real event sequence | `rg '\bfireEvent\b' ui/src/**/*.test.*` |
| `setTimeout` / `await sleep` in tests | Flake source | `rg 'setTimeout\|await\s+sleep\(' ui/src/**/*.test.*` |
| `expect(x).toBeDefined()` as the only assertion | Tests nothing | `rg 'toBeDefined\(\)\|toBeTruthy\(\)' ui/src/**/*.test.*` |
| `container.querySelector` | Bypasses RTL queries | `rg 'container\.querySelector' ui/src/**/*.test.*` |
| `getByLabelText` on labels without `for` | Fails silently or matches wrong element | `rg 'getByLabelText' ui/src/**/*.test.*` (review each) |

### AI-generated test reviewer checklist

Reject the PR if any of the following is true:

1. There is more than one `data-testid` per component test, OR any new `data-testid` was added to source code purely to make a test work, without a documented a11y justification.
2. Any test mocks a TanStack Query hook, a React hook, or a custom hook from `ui/src/lib/`. Mocks belong at the network seam (MSW), not at the hook seam.
3. Any test mocks a peer/child component from the same UI package. The page must render its real children.
4. Any test asserts on prop calls of a mocked child (`expect(Mock).toHaveBeenCalledWith(...)`).
5. Any test contains `toMatchSnapshot()` or `toMatchInlineSnapshot()` against more than 10 lines of output, or against arbitrary HTML.
6. Any test passes when the assertion is removed. Remove each assertion mentally; if anything still passes, the test is decorative.
7. Any test uses `setTimeout`, `await sleep`, or hardcoded ms delays as a synchronization mechanism.
8. Any test wraps user interactions in manual `act()` blocks. RTL/userEvent wrap interactions already.
9. Any test uses `fireEvent` for an interaction that `user-event` supports (clicks, typing, hover, keyboard).
10. Test names describe the implementation ("calls onClick handler when clicked") instead of user-visible behavior ("submits the form when Save is pressed").
11. The test file mocks more than the network seam. If anything outside `setupServer` handlers is mocked, justify in the PR description.
12. Many short trivial tests added together for coverage. Coverage of meaningless tests is a regression in maintainability.
13. No accessibility assertion on a page-level component. Page-level tests should run `vitest-axe` once.
14. Test contains a comment like `// TODO: fix this test` or `// flaky on CI` and is left enabled.

A test file should read top-to-bottom as a description of how a user uses the page. If it reads as a description of how the code is structured, reject and rewrite.

### What to recommend back to the AI

When asking an LLM to write or fix UI tests:

- "Use `screen.getByRole` or `getByLabelText`. Do not introduce `data-testid` unless the element has no semantic role and adding one is documented."
- "Render real components. Mock only the network via MSW handlers."
- "Assert on visible text, accessible roles, focus, URL, and toast messages. Do not assert on prop calls or hook calls."
- "Use `userEvent.setup()` once via the `renderUI` helper. Always `await` interactions."
- "No snapshot tests. No `setTimeout`. No manual `act()`."
- "Each test name describes user-visible behavior in present tense ('shows error when …', 'navigates to …', 'creates feed with …')."

## Quick reference

### DO

| | |
|---|---|
| `screen.getByRole("button", { name: /save/i })` | Real semantics, real user view |
| `await user.click(...)` / `await user.type(...)` | Real interaction sequence |
| `await screen.findByRole(...)` | Async with built-in wait |
| MSW handler override per failure case | Network is the seam |
| One `QueryClient` per test, `retry: false`, `gcTime: 0` | Deterministic, fast |
| `MemoryRouter` with `initialEntries` | Real router, controlled URL |
| `toHaveNoViolations()` on page-level renders | Catch a11y at write-time |
| Inline snapshot ≤10 lines for parsed config | Bounded, meaningful |

### DON'T

| | |
|---|---|
| `getByTestId` as default query | Implementation tag |
| `vi.mock("@tanstack/react-query")` | Removes the contract |
| `vi.mock("../components/Chart")` | Tests the mock |
| `expect(MockChild).toHaveBeenCalledWith(...)` | Prop-spy, not behavior |
| `toMatchSnapshot()` of rendered DOM | Brittle, decorative |
| Manual `act(() => …)` around clicks | RTL handles it |
| `fireEvent.click` for normal clicks | Skips event sequence |
| `setTimeout(..., 500)` to wait for state | Flake generator |
| `expect(component).toBeDefined()` | Asserts nothing |
| `container.querySelector(".btn-primary")` | Bypasses queries |
| Shared `QueryClient` across tests | Cross-test pollution |
| Asserting on Three.js/canvas internals | Wrong test layer |

## Worked examples (BAD vs GOOD)

### 1. Asserting on a hook vs asserting on the user view

BAD:

```tsx
import * as RQ from "@tanstack/react-query";

it("calls useQuery with /api/v1/sets", () => {
  const spy = vi.spyOn(RQ, "useQuery");
  render(<FeedExplorer />);
  expect(spy).toHaveBeenCalledWith(expect.objectContaining({
    queryKey: ["sets"],
  }));
});
```

GOOD:

```tsx
it("renders the feed list when /api/v1/sets responds", async () => {
  server.use(
    http.get("/api/v1/sets", () =>
      HttpResponse.json({ sets: [{ name: "firehol_level1", count: 1234 }] }),
    ),
  );
  renderUI(<FeedExplorer />);
  expect(
    await screen.findByRole("link", { name: /firehol_level1/i }),
  ).toBeInTheDocument();
});
```

### 2. Mocking a child component vs rendering it

BAD:

```tsx
vi.mock("@/components/home/home-explorer-view-cards", () => ({
  HomeExplorerViewCards: () => <div data-testid="cards" />,
}));

it("renders the cards view", () => {
  render(<HomeExplorer />);
  expect(screen.getByTestId("cards")).toBeInTheDocument();
});
```

GOOD:

```tsx
it("shows the feed cards in cards view", async () => {
  server.use(
    http.get("/api/v1/sets", () =>
      HttpResponse.json({
        sets: [
          { name: "firehol_level1", count: 1234 },
          { name: "firehol_level2", count: 5678 },
        ],
      }),
    ),
  );
  const { user } = renderUI(<HomeExplorer />);
  await user.click(await screen.findByRole("button", { name: /cards/i }));
  expect(
    await screen.findByRole("article", { name: /firehol_level1/i }),
  ).toBeInTheDocument();
});
```

### 3. Snapshot vs targeted assertions

BAD:

```tsx
it("renders the feeds table", () => {
  const { container } = render(<FeedsTable feeds={fixtures.feeds} />);
  expect(container).toMatchSnapshot();
});
```

GOOD:

```tsx
it("shows one row per feed with name, count, and category", () => {
  renderUI(<FeedsTable feeds={fixtures.feeds} />);
  const rows = screen.getAllByRole("row");
  expect(rows).toHaveLength(fixtures.feeds.length + 1); // header + data
  for (const feed of fixtures.feeds) {
    const row = screen.getByRole("row", { name: new RegExp(feed.name) });
    expect(row).toHaveTextContent(feed.count.toLocaleString());
    expect(row).toHaveTextContent(feed.category);
  }
});
```

### 4. `fireEvent` vs `user-event`

BAD:

```tsx
fireEvent.change(input, { target: { value: "abc" } });
fireEvent.click(button);
expect(submitted).toBe(true);
```

GOOD:

```tsx
const { user } = renderUI(<SearchForm onSubmit={onSubmit} />);
await user.type(screen.getByRole("searchbox", { name: /search/i }), "abc");
await user.click(screen.getByRole("button", { name: /go/i }));
expect(
  await screen.findByRole("status", { name: /searching/i }),
).toBeInTheDocument();
```

### 5. Asserting on prop calls vs asserting on outcomes

BAD:

```tsx
const onSave = vi.fn();
render(<FeedModal onSave={onSave} feed={fixtures.feed} />);
fireEvent.click(screen.getByText("Save"));
expect(onSave).toHaveBeenCalledWith(expect.objectContaining({ id: 1 }));
```

GOOD:

```tsx
let saved: unknown;
server.use(
  http.put("/api/v1/admin/feeds/:id", async ({ request }) => {
    saved = await request.json();
    return HttpResponse.json({ ok: true });
  }),
);

const { user } = renderUI(<FeedModal feed={fixtures.feed} />);
await user.click(screen.getByRole("button", { name: /save/i }));
expect(
  await screen.findByRole("status", { name: /feed updated/i }),
).toBeInTheDocument();
expect(saved).toMatchObject({ name: fixtures.feed.name });
```

### 6. Routing with stubbed loader vs real router

BAD:

```tsx
vi.mock("react-router-dom", async (importActual) => {
  const actual = await importActual<typeof import("react-router-dom")>();
  return { ...actual, useLoaderData: () => fixtures.feedDetail };
});
```

GOOD:

```tsx
const router = createMemoryRouter(
  [
    {
      path: "/feeds/:name",
      element: <FeedDetail />,
      loader: async ({ params }) =>
        fetch(`/api/v1/sets/${params.name}`).then((r) => r.json()),
    },
  ],
  { initialEntries: ["/feeds/firehol_level1"] },
);

server.use(
  http.get("/api/v1/sets/firehol_level1", () =>
    HttpResponse.json(fixtures.feedDetail),
  ),
);

renderUI(<RouterProvider router={router} />);
expect(
  await screen.findByRole("heading", { name: /firehol_level1/i }),
).toBeInTheDocument();
```

### 7. Chart contract: data layer vs DOM internals

BAD:

```tsx
it("renders the bar chart", () => {
  const { container } = render(<TrafficChart data={fixtures.points} />);
  expect(container.querySelectorAll("svg path")).toHaveLength(7);
});
```

GOOD (split into two tests):

```tsx
// Pure data test, no DOM:
it("buckets points by day", () => {
  expect(bucketByDay(fixtures.points)).toEqual([
    { day: "2026-01-01", value: 12 },
    { day: "2026-01-02", value: 18 },
  ]);
});

// Component test asserts on accessible summary:
it("describes weekly traffic in the chart's accessible label", () => {
  renderUI(<TrafficChart data={fixtures.points} />);
  const fig = screen.getByRole("figure", { name: /weekly traffic/i });
  expect(fig).toHaveAccessibleDescription(/peak 18 on jan 2/i);
});
```

## Honest limits

- **jsdom does not implement canvas, WebGL, layout, or color contrast.** Do not pretend a chart renders correctly there. Test the data layer separately and the controls behaviorally; defer pixel correctness to Playwright.
- **Radix portals render outside the test container.** `screen.getByRole(...)` finds them because it queries `document.body`. `within(container)` does not. Use `screen` for portalled UI; use `within` only for explicitly scoped DOM regions.
- **Radix `pointer-events: none` quirk** in jsdom can cause `userEvent.click` to fail with "element has pointer-events: none" even though the real browser handles the cascade correctly. Add the `hasPointerCapture`/`releasePointerCapture` polyfills above; for stubborn cases, fall back to Playwright component testing for that specific component.
- **MSW v2 + Vitest** sometimes surfaces request-resolution races on slow machines. If you see flakes, increase `findBy` timeout for that test only (`{ timeout: 5000 }`); don't increase the global default.
- **React 19 + Vitest** is reported working in current docs and projects, but minor issues (StrictMode double-effect counts, `act` import paths) appear in some setups. Verify against the React 19 minor pinned in `ui/package.json` before committing test infrastructure.
- **Playwright visual regression** is high-value but high-cost: baselines must be generated in the same CI environment that runs them, masks must be aggressive, and flake budget must be near zero. Adopt only when there is a concrete contract worth the cost.

## Sources

- [Testing Library — Queries Priority](https://testing-library.com/docs/queries/about) (verified 2026-04)
- [Testing Library — user-event Intro](https://testing-library.com/docs/user-event/intro/) (verified 2026-04)
- [Kent C. Dodds — Testing Implementation Details](https://kentcdodds.com/blog/testing-implementation-details)
- [Kent C. Dodds — Common Mistakes with React Testing Library](https://kentcdodds.com/blog/common-mistakes-with-react-testing-library)
- [Kent C. Dodds — Fix the "not wrapped in act(...)" warning](https://kentcdodds.com/blog/fix-the-not-wrapped-in-act-warning)
- [Kent C. Dodds — Write tests. Not too many. Mostly integration.](https://kentcdodds.com/blog/write-tests)
- [TkDodo — Test IDs are an a11y smell](https://tkdodo.eu/blog/test-ids-are-an-a11y-smell)
- [Vitest — Getting Started](https://vitest.dev/guide/) (Vitest 4.x verified 2026-04)
- [Vitest — Browser Mode](https://vitest.dev/guide/browser/)
- [TanStack Query — Testing](https://tanstack.com/query/latest/docs/framework/react/guides/testing)
- [MSW — Node Integrations](https://mswjs.io/docs/integrations/node)
- [MSW — Quick Start](https://mswjs.io/docs/quick-start/)
- [Playwright — Test Snapshots / Visual Comparisons](https://playwright.dev/docs/test-snapshots)
- [Playwright — Configuration](https://playwright.dev/docs/test-configuration)
- [vitest-axe](https://github.com/chaance/vitest-axe)
- [jest-axe](https://github.com/NickColley/jest-axe)
- [React Router v7 — Testing](https://reactrouter.com/start/framework/testing)
- [Radix UI — Pointer-events / pointer-capture issues in jsdom](https://github.com/testing-library/user-event/discussions/1087)
- [Radix UI — Portals and testing libraries](https://github.com/radix-ui/primitives/discussions/1130)
- [Lessons learned from upgrading user-event to v14 (Kibana)](https://walterra.dev/blog/2025-05-06-user-event-v14)
- [LogRocket — I replaced my test suite with AI agents: what broke](https://blog.logrocket.com/replaced-test-suite-ai-agents/)
- [Callstack — Using AI to write tests for React components](https://www.callstack.com/blog/using-ai-to-write-tests-for-react-components)
- [Epic Web Dev — Vitest Browser Mode vs Playwright](https://www.epicweb.dev/vitest-browser-mode-vs-playwright)
- [Medium — Flake-resistant Playwright visual testing](https://medium.com/@david-auerbach/how-to-conduct-visual-testing-with-playwright-a-complete-flake-resistant-guide-58714ebfbf05)
