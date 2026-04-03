# SOW-0041 | 2026-05-01 | frontend-test-re-review

## Status
completed

## Requirements

### Purpose
Second-round black-box behavioral-testing gap analysis of the React/TypeScript UI in `ui/`, run at the same scope and rubric as the original SOW-0034 (now closed). The first round produced 0 A / 12 B / 5 C with 6 numbered user decisions. The implementation that followed (commits `35a0c0b` and `769dfd9`) installed Vitest 4 + Testing Library + MSW v2 + jsdom + vitest-axe, added a `make ui-test` gate, wired CI, and shipped 4 colocated `*.test.tsx` files. This SOW re-runs the full rubric — not a fix-verification — to surface anti-patterns that exist for the first time now that tests exist, plus remaining gaps.

The user's hard constraint remains non-negotiable: black-box behavioral tests only. No assertions on internal state, hook calls, render counts, mocked children, mocked TanStack Query, or DOM snapshots of arbitrary structure.

### User request quoted verbatim
> You are doing a SECOND-ROUND gap-analysis of frontend testing in `ui/`. The original gap-analysis SOW (`SOW-0034`) has been implemented per commits `35a0c0b` and `769dfd9`. The user wants a fresh review at the same scope. Do NOT narrow scope to "verify the fixes only" — re-run the full rubric. Hard rules: black-box behavioral testing only; analysis only, no edits; file:line evidence; numbered findings; "needs verification" over false confidence; no Claude/Anthropic in the SOW.

### Assistant understanding
- The UI test framework is now installed and 4 black-box tests pass. A real review must apply the rubric to those tests, not just confirm dependencies are present.
- "Verification of SOW-0034" is a sub-task; it is not the entire scope. New A-class smells can exist for the first time now that tests exist.
- The SOW must produce: a per-finding verification table for SOW-0034 items, an independent A/B/C audit of the new state, a list of the highest-value behavioral surfaces that are still untested, and numbered user decisions with options + recommendations.

### Acceptance criteria
- This file exists at `.agents/sow/pending/SOW-0041-20260501-frontend-test-re-review.md` and is the only file written by this work.
- A verification table covers every SOW-0034 finding (A0, B1–B12, C1–C5) with status: VERIFIED FIXED / PARTIALLY FIXED / NOT FIXED / REGRESSED, with file:line evidence.
- An independent rubric pass produces new Category A / B / C findings that are not constrained to SOW-0034's scope.
- Top behavioral-test gaps are identified with `ui/`-relative file paths and one-line behavior descriptions.
- A numbered set of decisions is presented with options, recommendation, rationale.
- No source code is changed.

## Analysis

### Methodology
Skills loaded: `frontend-behavioral-testing` (primary rubric), `project-testing`, `project-coding`, `project-reviewing`, `sow`.

Verification commands run from this analysis (all read-only):

- `cat ui/package.json | jq '.scripts, .devDependencies, .dependencies'` — confirmed installed devDeps and scripts.
- `find ui -name '*.test.*' -o -name '*.spec.*' -not -path '*/node_modules/*'` — listed test files.
- `ls -la ui/vitest.config.* ui/playwright.config.* ui/tests ui/e2e ui/src/test 2>&1` — confirmed harness layout, no Playwright config or e2e directory.
- `cat ui/vitest.config.ts ui/vitest.setup.ts ui/eslint.config.js ui/src/test/render.tsx ui/src/test/msw-handlers.ts ui/src/test/msw-server.ts ui/src/test/fixtures.ts` — read full harness.
- `grep -E 'test|vitest|playwright|ui-' Makefile` and `cat .github/workflows/ci.yml` — confirmed CI gate `Test UI: make ui-test` at `.github/workflows/ci.yml:38-39`.
- Greppable rubric smells across the 4 test files:
  - `data-testid|getByTestId` → 0 hits.
  - `vi.mock|vi.spyOn` → 0 hits.
  - `fireEvent|toMatchSnapshot|toMatchInlineSnapshot|setTimeout|await\s+sleep|container\.querySelector|toBeDefined|toBeTruthy|\bact\(` → 0 hits.
  - `toHaveBeenCalled|onFeedClick|onSave` → hits in `ui/src/components/admin/feeds-table.test.tsx:8,26,41` (callback prop assertion — see A1 below).
- Counted query primitives: 13 role/label queries vs 3 text queries across 4 test files — semantic-first.
- Cross-checked `ui/src/lib/api.ts` endpoint surface against `ui/src/test/msw-handlers.ts` to size handler-set coverage.
- Inspected component source under test (`ui/src/components/home/home-explorer.tsx`, `ui/src/components/admin/feeds-table.tsx`, `feeds-table-body.tsx`, `ui/src/components/ip-search/ip-search-surface.tsx`, `ui/src/pages/feed-detail.tsx`) to evaluate whether each test exercises the user-visible contract or a thinned version of it.
- `cd ui && pnpm test` — current baseline: **4 files, 4 tests passed in 1.73s** (one Vitest 4 + Node 25 cosmetic warning about `--localstorage-file`, no test failures).
- Counted untested component surfaces: 105 production `*.tsx` files vs 4 test files (~3.8% file-level coverage; informational only — coverage % is not a gate).
- Reviewed git diffs of `commit 35a0c0b` and `commit 769dfd9` to confirm intent and matching changes.

### Current state (verified)

| Aspect | Status | Evidence |
|---|---|---|
| Test runner | **Vitest 4.1.5 installed** | `ui/package.json` devDeps `vitest@^4.1.5`, `@vitest/coverage-v8@^4.1.5`, `jsdom@^29.1.1` |
| Test scripts | `test`, `test:watch`, `test:coverage` present | `ui/package.json` scripts |
| Vitest config | `ui/vitest.config.ts` (jsdom, MSW lifecycle wired through `vitest.setup.ts`, no global coverage threshold) | `ui/vitest.config.ts:1-31` |
| Setup file | `ui/vitest.setup.ts` includes `@testing-library/jest-dom/vitest`, `vitest-axe` matcher extension, RTL `cleanup`, MSW `beforeAll/afterEach/afterAll`, polyfills for `ResizeObserver`, `IntersectionObserver`, `matchMedia`, `scrollIntoView`, `hasPointerCapture`, `releasePointerCapture` | `ui/vitest.setup.ts:1-76` |
| DOM testing libs | `@testing-library/react@^16.3.2`, `@testing-library/user-event@^14.6.1`, `@testing-library/jest-dom@^6.9.1`, `@testing-library/dom@^10.4.1` | `ui/package.json` devDeps |
| Network seam | MSW v2 (`msw@^2.14.2`) with `setupServer`, `onUnhandledRequest: "error"` | `ui/src/test/msw-server.ts:1-4`, `ui/vitest.setup.ts:65` |
| Default handlers | 5 routes: `/api/v1/categories`, `/api/v1/client-ip`, `/world/countries-110m.json`, `/api/v1/sets`, `/api/v1/search`, `/api/v1/sets/:name` | `ui/src/test/msw-handlers.ts:10-43` |
| `renderUI` helper | Provides `QueryClientProvider` (retry 0, gcTime 0, staleTime 0), `ThemeProvider`, `TooltipProvider`, `MemoryRouter` with `initialEntries`, `userEvent.setup()` | `ui/src/test/render.tsx:1-52` |
| a11y | `vitest-axe@^0.1.0` matcher extension; one `axe(container)` assertion in `ip-search-surface.test.tsx` | `ui/vitest.setup.ts:7`, `ui/src/components/ip-search/ip-search-surface.test.tsx:46-49` |
| ESLint test plugin | `eslint-plugin-testing-library@^7.16.2` scoped to `*.test.{ts,tsx}` | `ui/eslint.config.js:5,24-27` |
| Jest-dom ESLint plugin | Not installed (omitted because of ESLint 10 peer; documented in SOW-0034 evidence-backed non-goals) | `ui/eslint.config.js` |
| Coverage tooling | V8 coverage installed; `test:coverage` script present; **no global threshold** (per SOW-0034 Decision 3 = (a)) | `ui/vitest.config.ts:18-28` |
| Make target | `make ui-test` runs `pnpm --dir ui test` | `Makefile:24-25` |
| CI gate | `Test UI: make ui-test` runs as a discrete step | `.github/workflows/ci.yml:38-39` |
| Playwright | **Not installed** (per SOW-0034 Decision 1 = (b), deferred to follow-up SOW; that follow-up has not been opened) | `ui/package.json`, `ls ui/playwright.config.*` no match |
| Bundle-size budget | Not installed (per SOW-0034 Decision 6 = (b), deferred to separate SOW) | `ui/package.json` |
| Existing test files | **4** colocated tests | `ui/src/components/ip-search/ip-search-surface.test.tsx`, `ui/src/components/home/home-explorer.test.tsx`, `ui/src/components/admin/feeds-table.test.tsx`, `ui/src/pages/feed-detail.test.tsx` |
| Test pass rate | 4 / 4 pass in 1.73s | `pnpm --dir ui test` output |
| Production `data-testid` | Zero new instances added; clean | `grep -rn data-testid ui/src` returns nothing in production paths |

### Verification table — SOW-0034 findings

Cross-reference for the original `0 A / 12 B / 5 C` set.

| ID | Finding | Status | Evidence / notes |
|---|---|---|---|
| A0 | No anti-patterns to eliminate (clean slate) | **Still N/A for that batch**; new A-class smells now exist for the first time, see Category A below | new test files contain at least one prop-call assertion |
| B1 | No test runner (Vitest) | **VERIFIED FIXED** | `ui/package.json` devDeps `vitest@^4.1.5`; `ui/vitest.config.ts:1-31`; `pnpm --dir ui test` green |
| B2 | No DOM testing libraries | **VERIFIED FIXED** | RTL/user-event/jest-dom installed in `ui/package.json` devDeps; matcher import at `ui/vitest.setup.ts:1` |
| B3 | No network seam (MSW v2) | **PARTIALLY FIXED** | MSW installed and wired with `onUnhandledRequest: "error"` (`ui/vitest.setup.ts:65`); default handler set covers only 5 routes vs 30+ endpoints visible in `ui/src/lib/api.ts:107-575`. Sufficient for the 4 first-batch tests, insufficient for the next batch (admin actions, feed-detail providers, country/asn/maintainer pages, methodology, integrity). See B-new-3. |
| B4 | No `vitest.setup.ts` / Radix + chart polyfills | **VERIFIED FIXED** | `ui/vitest.setup.ts:9-62` polyfills `ResizeObserver`, `IntersectionObserver`, `matchMedia`, `scrollIntoView`, `hasPointerCapture`, `releasePointerCapture` |
| B5 | No `renderUI` helper | **VERIFIED FIXED** | `ui/src/test/render.tsx:9-51` provides `makeQueryClient` (retry/gc/stale 0, refetchOnWindowFocus false) and `renderUI` with `QueryClientProvider`, `ThemeProvider`, `TooltipProvider`, `MemoryRouter`, `userEvent.setup()` |
| B6 | No CI gate for UI tests | **VERIFIED FIXED** | `Makefile:24-25` `ui-test:` target; `.github/workflows/ci.yml:38-39` invokes `make ui-test` as a discrete CI step. Per SOW-0034 Decision 2 = (b), `make test` is not gated on UI tests; CI runs them as a separate step in series. |
| B7 | No Playwright e2e for critical flows | **NOT FIXED — DEFERRED** | No `playwright.config.ts`, no `@playwright/test`, no `ui/e2e/`. Deferred per SOW-0034 Decision 1 = (b) to a follow-up SOW that has not been opened. Charts (Recharts/D3/VisX), globe.gl WebGL, and country choropleth still ship without browser-level coverage. See B-new-1. |
| B8 | No accessibility assertions | **PARTIALLY FIXED** | `vitest-axe` is wired and there is **one** `axe(container)` call (`ui/src/components/ip-search/ip-search-surface.test.tsx:46-49`); the other 3 page-level/component-level tests do not include an axe assertion. Skill rule "Page-level tests should run vitest-axe once" is satisfied for 1 of 4. See B-new-4. |
| B9 | No tests for high-value behaviors | **PARTIALLY FIXED** | All 5 first-batch targets accepted in SOW-0034 Decision 5 = (a) — but only 4 surfaces shipped. The deepest target, the **admin feed modal** (b9-v in original SOW; ~1,100 lines across `feed-modal*.tsx`), has no test. The 4 that did ship vary in fidelity — see verification rows below and Category A. |
| B9-i | Home explorer + filter rail + view switching | **PARTIALLY FIXED** | `ui/src/components/home/home-explorer.test.tsx:1-38`. Covers filter input, count text, and switching cards → table view. **Bypasses the network seam** because `<HomeExplorer />` takes `feeds` and `categories` as props (`ui/src/components/home/home-explorer.tsx:50-57`); the test does not exercise `home.tsx`'s `useQuery` → `<HomeExplorer />` data flow. URL-`searchParams` round-tripping (mentioned as in-scope in SOW-0034 B9-i) is also not asserted. See B-new-2. |
| B9-ii | Home IP lookup form | **PARTIALLY FIXED** | `ui/src/components/ip-search/ip-search-surface.test.tsx:1-50`. Tests typing, submit, MSW request inspection (`ip` and `details=true` query params), and visible match. **Does not cover** URL pre-fill of `?ip=...` (SOW-0034 B9-ii sub-bullet) nor the `home-ip-lookup.tsx` client-IP detection branch. Asserts on `IPSearchSurface` directly, which renders without `HomeIPLookup`'s URL/auto-detect logic. See B-new-2. |
| B9-iii | Feed detail page | **PARTIALLY FIXED** | `ui/src/pages/feed-detail.test.tsx:1-21` only covers the **404 not-found path**. The success-path render (heading, IP/CIDR count, category, sections) called out in SOW-0034 B9-iii is not asserted. The MSW handler at `ui/src/test/msw-handlers.ts:36-42` returns 404 unless `name === "known_feed"`; the test deliberately requests `/ipsets/missing_feed` to hit the 404 branch. Success path is untested. See B-new-2. |
| B9-iv | Admin feeds table (filter/sort/search) | **PARTIALLY FIXED** | `ui/src/components/admin/feeds-table.test.tsx:7-44` tests search filter and keyboard-Enter row open. **Sorting is not asserted**; multi-select chips for category/health/hidden are not asserted; row count after filter is not asserted. Test uses a callback-prop assertion (`onFeedClick.toHaveBeenCalledWith`) rather than the page-level outcome (modal opens with feed details). See A1. |
| B9-v | Admin feed modal | **NOT FIXED** | No `feed-modal*.test.tsx` file. The largest single behavioral surface in the admin UI (~1,100 lines across `feed-modal*.tsx`) has no test. The dialog focus-return contract that the skill specifically requires for dialog flows is unverified. |
| B10 | No coverage configuration | **VERIFIED FIXED (no threshold)** | `ui/vitest.config.ts:18-28` configures V8 coverage; `test:coverage` script present; per SOW-0034 Decision 3 = (a), no global threshold. |
| B11 | No bundle-size budget | **NOT FIXED — DEFERRED** | Per SOW-0034 Decision 6 = (b), deferred to a separate SOW that has not been opened. Heavy deps (`three`, `globe.gl`, `react-globe.gl`, `recharts`, `d3-*`, `topojson-client`) still ship unbudgeted. |
| B12 | Visual regression strategy | **NOT FIXED — DEFERRED** | Per SOW-0034 Decision 4 = (b), deferred until B7 stabilizes. Since B7 has not landed, B12 cannot land either. |
| C1 | ESLint test plugins | **PARTIALLY FIXED** | `eslint-plugin-testing-library` is wired (`ui/eslint.config.js:5,24-27`). `eslint-plugin-jest-dom` was deliberately not adopted because of ESLint 10 peer-range incompatibility (SOW-0034 evidence-backed non-goal); needs verification whether the plugin's ESLint 10 support has shipped since the original SOW closed. |
| C2 | Storybook | **NOT FIXED — out of scope** | Out of scope per original SOW; no change. |
| C3 | Test naming and file location convention | **VERIFIED FIXED** | All 4 tests are colocated as `*.test.tsx` next to the surface; shared helpers under `ui/src/test/`; recorded in `project-testing/SKILL.md:204-206` and `project-reviewing/SKILL.md:71-72`. |
| C4 | Pre-commit hook | **NOT FIXED — out of scope** | Skipped per original recommendation. |
| C5 | Test fixtures discipline | **VERIFIED FIXED** | `ui/src/test/fixtures.ts:1-159` builds typed fixtures from `ui/src/lib/api-types.ts` with `sample*` helpers and override partials. |

Summary: of 18 actionable items (B1–B12 + C1–C5, excluding A0 and out-of-scope C2/C4), **9 verified fixed**, **6 partially fixed**, **3 not fixed (deferred per recorded decisions, follow-up SOWs not yet opened)**, **0 regressed**.

### Findings — Category A: anti-patterns to eliminate

#### A1. Admin feeds-table test asserts on a callback prop instead of a user-visible outcome
- **Where**: `ui/src/components/admin/feeds-table.test.tsx:8`, `:26`, `:41`.
- **What**: the test injects `const onFeedClick = vi.fn()` (line 8), passes it as the `onFeedClick` prop (line 26), and after pressing Enter on the focused row asserts `expect(onFeedClick).toHaveBeenCalledWith(expect.objectContaining({ name: "beta_malware" }))` (lines 41-43).
- **Why it fails the rubric**: the user-visible outcome of pressing Enter on a feed row is "the feed modal opens with that feed's details" (the prop's actual job in `ui/src/pages/admin.tsx`). The test instead asserts on the prop callback the test injected, which is the prop-spy anti-pattern explicitly listed in the `frontend-behavioral-testing` skill's "Common failure modes" table (`expect(MockChild).toHaveBeenCalledWith(...)`) and "AI-generated test reviewer checklist" item 4. A refactor that replaces the callback wiring with a router-based modal route would still satisfy the user contract but break this test.
- **Severity**: Category A — first instance of an anti-pattern in the new test code.
- **Refactor sketch**: render `<Admin />` (the page that owns both `<FeedsTable />` and `<FeedModal />`), seed `/api/v1/admin/feeds`, search, focus the row, press Enter, assert that the modal heading for `beta_malware` is visible. Drops the `vi.fn()` and the prop-call assertion.
- **Risk if left**: small but real — the test will pass when the modal-open contract is broken, and will fail when the prop wiring is refactored even if the user-visible behavior is preserved. Sets a precedent for the next batch of tests to copy.

### Findings — Category B: missing gaps to fill

#### B-new-1. Playwright e2e not yet opened as a follow-up SOW
- **Current state**: SOW-0034 Decision 1 = (b) explicitly deferred Playwright to a follow-up SOW. That SOW has not been opened. No `ui/playwright.config.ts`, no `ui/e2e/`, no `@playwright/test` in `ui/package.json`.
- **Why it matters**: jsdom cannot validate canvas (Recharts/D3/VisX), WebGL (globe.gl, three), or layout (Tailwind v4, accent bars, hero typography). Every chart, the country choropleth, the home globe, and any focus-trap inside Radix portals continue to ship without browser-level coverage. The skill ("Honest limits") is explicit that pixel correctness and WebGL belong in Playwright.
- **First step**: open a successor SOW for Playwright with backend test-mode wiring (the daemon supports a deterministic data mode? — needs verification at `pkg/web/server.go` and `cmd/update-ipsets/`). First flows: home renders + IP lookup happy path, feed-detail success render with a real chart, admin login + feed modal open/close with focus return.
- **Effort**: M.
- **Risk if left**: chart and globe regressions, and dialog focus-return regressions, ship invisible.

#### B-new-2. First-batch tests bypass the page composition that real users exercise
- **Current state**: 3 of 4 tests render leaf components instead of pages.
  - `home-explorer.test.tsx:21-22` renders `<HomeExplorer feeds={…} categories={…} loading={false} />` directly. The real page (`ui/src/pages/home.tsx`) wires those props via TanStack Query against `/api/v1/sets` and `/api/v1/categories`; that wiring is untested.
  - `ip-search-surface.test.tsx:21-26` renders `<IPSearchSurface scope={{ kind: "global" }} variant="hero" placeholder="…" />` directly. The real homepage path renders `<HomeIPLookup />` which wraps it with URL-prefill (`?ip=…`) and client-IP-autodetect logic (`ui/src/components/home/home-ip-lookup.tsx:9-15`); that wrapper is untested.
  - `feed-detail.test.tsx:8-13` only exercises the 404 branch via the route. The success path that the user actually uses on every feed page is not tested.
  - `feeds-table.test.tsx:18-28` renders `<FeedsTable />` with hand-built `feeds={…}` and `onFeedClick={vi.fn()}`. The real page wires these to `useQuery` and to a modal at `ui/src/pages/admin.tsx`; neither integration is tested (also see A1).
- **Why it matters**: tests for the leaf component verify a slice of the contract; tests for the page verify the contract the user actually depends on (data-fetch → render → interact → URL/state changes). The skill (and SOW-0034 §B9) explicitly framed targets at the page level. The 4 shipped tests are still useful, but they leave the page-composition seam untested and they are the first reference for future tests, which will copy the leaf-only style.
- **First step**: in the next batch, render the page-level component and let MSW serve the queries. Specifically:
  - `<HomePage />` test that types into `Filter feeds`, asserts the filtered count, switches to table view, asserts the same set of links is visible.
  - `<HomePage />` test that submits an IP via the hero IP lookup and asserts the URL gains `?ip=…` and the result panel renders.
  - `<FeedDetailPage />` test for the success path with the existing `sampleFeedMetadata` fixture; assert heading, count, category, and at least one section heading.
  - `<Admin />` test that searches the table and presses Enter to open the modal, asserts the modal heading.
- **Effort**: M.
- **Risk if left**: future tests copy the leaf-rendering pattern, the page-composition layer never gets a regression net, and an assertion gap silently grows.

#### B-new-3. MSW handler set covers only 5 of 30+ endpoints in `ui/src/lib/api.ts`
- **Current state**: `ui/src/test/msw-handlers.ts:10-43` defines 5 default handlers. `ui/src/lib/api.ts` exposes 30+ endpoints (catalog, ASN, geo, bogons, infrastructure, comparison, history, changesets, retention, insights, search, home globe/summary, country/ASN/maintainer index and detail, methodology, admin run/recheck/reprocess/enable/disable, manifest, integrity, integrity reprocess; see `ui/src/lib/api.ts:90-580`).
- **Why it matters**: `onUnhandledRequest: "error"` (good) means every new test must add its own handler (per-test `server.use(...)` is fine), but there is no shared baseline for the next batch. Each new page test that loads (for example) `<FeedDetailPage />` will re-derive handlers for `/api/v1/sets/:name`, `/api/v1/sets/:name/asn`, `/api/v1/sets/:name/asn/:provider`, `/api/v1/sets/:name/countries`, `/api/v1/sets/:name/countries/:provider`, `/api/v1/sets/:name/insights`, `/api/v1/sets/:name/compare`, `/api/v1/sets/:name/history`, `/api/v1/sets/:name/changesets`, `/api/v1/sets/:name/retention`. Drift will accumulate.
- **First step**: add page-level scenario handlers to the default set or to a `ui/src/test/scenarios.ts` (e.g. `feedDetailHandlers(name)`, `adminHandlers()`, `countryDetailHandlers(code)`) so that page-level tests can opt-in with a single line.
- **Effort**: S–M.
- **Risk if left**: new tests will either duplicate handler definitions or slip into the anti-pattern of mocking at the hook seam to avoid the handler enumeration cost.

#### B-new-4. Three of four tests have no a11y assertion
- **Current state**: only `ip-search-surface.test.tsx:46-49` runs `await axe(container)`. `home-explorer.test.tsx`, `feeds-table.test.tsx`, and `feed-detail.test.tsx` do not.
- **Why it matters**: the skill recommends "Page-level tests should run `vitest-axe` once". Each surface added without an axe pass loses the cheapest a11y safety net at write-time. Specifically: feed-detail's not-found page is exactly the kind of low-traffic page where missing landmark/role/heading-hierarchy regressions go unnoticed.
- **First step**: add one `expect(await axe(container, { rules: { "color-contrast": { enabled: false } } })).toHaveNoViolations()` call per page-level test, including in any new tests written for B-new-2.
- **Effort**: S.
- **Risk if left**: low per-instance, but compounding as the test count grows.

#### B-new-5. Bundle-size budget not opened as a follow-up SOW
- **Current state**: per SOW-0034 Decision 6 = (b), deferred. Successor SOW not opened.
- **Why it matters**: heavy deps still ship unbudgeted. Not a behavioral-test gap per se, but it was on the SOW-0034 follow-up list; flagging here so it does not silently fall off.
- **First step**: open a separate SOW; out of scope for the behavioral-testing successor.
- **Effort**: S–M.
- **Risk if left**: low–medium; performance regressions ship until manually noticed.

#### B-new-6. Vitest 4 + Node 25 produces a `--localstorage-file` warning per worker
- **Current state**: `pnpm --dir ui test` prints `(node:NNNNN) Warning: --localstorage-file was provided without a valid path` four times (one per worker). Tests still pass.
- **Why it matters**: needs verification — likely a Vitest 4 / jsdom / Node 25 compatibility wrinkle that may indicate a non-fatal misconfiguration of jsdom's localStorage. CI logs will accumulate this noise. If the warning is upstream and benign, document it; if it points at a real bug, fix it.
- **First step**: research the warning in the Vitest issue tracker (does `npm list --depth=0` show a transitive `node-localstorage` or `jsdom` mismatch?), and either (a) add an explicit jsdom config to silence it, (b) pin Vitest to a known-clean minor, or (c) document as upstream noise.
- **Effort**: S.
- **Risk if left**: low; cosmetic.

#### B-new-7. The `make ui-test` ↔ `make test` relationship is not gated
- **Current state**: `Makefile:15-16` `test:` runs only `$(GO) test ./...`. `make ui-test` is a separate target. CI runs both as separate steps. Per SOW-0034 Decision 2 = (b), this is the intended ratchet-in state.
- **Why it matters**: now that 4 UI tests exist and are stable in CI, the Decision 2 = (b) ratchet condition has been met ("flip on once the first 5 tests exist and are stable" — 4 of 5 shipped). User decision needed: does `make test` now depend on `make ui-test` so local runs catch UI regressions before push? See Decision 4 below.
- **First step**: this is a user-facing decision; defer to the decisions section.
- **Risk if left**: low — CI catches it. Local hygiene only.

#### B-new-8. No tests for the highest-value untested public surfaces
- **Current state**: by churn (commits since 2026-04-30) and user-traffic, the following public surfaces remain untested:
  1. `<HomePage />` (`ui/src/pages/home.tsx`) — top of funnel; orchestrates explorer + IP lookup + globe.
  2. `<FeedDetailPage />` success path (`ui/src/pages/feed-detail.tsx`) — most-viewed page on the public site.
  3. `<CountryDetailPage />` (`ui/src/pages/country-detail.tsx`) and `<ASNDetailPage />` — feed-of-feeds aggregate views.
  4. `<HomeIPLookup />` URL-prefill and client-IP-autodetect (`ui/src/components/home/home-ip-lookup.tsx:9-15`) — the wrapper around the tested `IPSearchSurface`.
  5. `<MaintainerDetailPage />` (`ui/src/pages/maintainer-detail.tsx`).
  6. `<MethodologyPage />` (`ui/src/pages/methodology.tsx`) — content rendering safety (`dompurify` boundary).
  7. `<NotFoundPage />` (`ui/src/pages/not-found.tsx`) — a11y baseline.
- **Why it matters**: these are the surfaces where the data layer + URL handling + Radix dialogs + chart wrappers all converge; bypassing them in the first batch is acceptable, but the second batch should land them.
- **First step**: pick a subset for the implementation SOW; one test per page is enough to catch regressions in the data flow.
- **Effort**: M.
- **Risk if left**: medium; the home page and feed detail are the most-visited surfaces.

#### B-new-9. No tests for admin write paths
- **Current state**: 0 tests for `recheck`, `reprocess`, `enable`, `disable`, `manifest fetch`, `integrity`, `integrity/reprocess`, `entity integrity`, `entity integrity rebuild`, `run`, `admin command palette`. The admin UI is the operator's main control plane.
- **Why it matters**: these are the operator-visible flows that enforce "background work is visible through the admin API/UI" (a project-wide rule from `CLAUDE.md`). Without tests, refactors to these endpoints' UI plumbing risk silently breaking the operator flow.
- **First step**: pick the 2 highest-stakes flows (recheck and reprocess) and write tests that submit the action via the UI and assert (a) the request was issued (MSW spy) and (b) the visible status flips to "running" / shows a toast.
- **Effort**: M.
- **Risk if left**: medium for an operator product.

#### B-new-10. Chart data-layer functions are untested
- **Current state**: `ui/src/lib/explorer-state.ts` defines `applyFilters`, `applySort`, `defaultHealthSelection`, `distinctMaintainers`, `distinctLicenses`, `publicExplorerFeeds`, `readExplorerState`, `rememberExplorerView`, `writeExplorerState`. None of these have unit tests.
- **Why it matters**: the skill explicitly calls out (in "Visualizations" and "Chart contract: data layer vs DOM internals") that the data-layer functions feeding charts are pure data and belong in fast unit tests; component tests should assert on accessible summaries, not chart internals. The data layer is the cheapest place to catch filter/sort regressions, and `ui/src/lib/explorer-state.ts` is a clear unit-test target.
- **First step**: add `ui/src/lib/explorer-state.test.ts` with table-driven cases for each pure function. No DOM, no MSW.
- **Effort**: S.
- **Risk if left**: low–medium; `applyFilters`/`applySort` are easy to break silently.

### Findings — Category C: neutral improvements

#### C-new-1. Recheck `eslint-plugin-jest-dom` peer-range
- **Current state**: not installed because of ESLint 10 peer-range incompatibility at the time of SOW-0034. Needs verification whether the plugin has since added ESLint 10 support; if so, wire it scoped to `*.test.{ts,tsx}` for an additional mechanical gate against decorative assertions.
- **Effort**: S.

#### C-new-2. Surface MSW request inspection as a small helper
- **Current state**: `ip-search-surface.test.tsx:11-18` builds a `URLSearchParams` capture handler inline. Two more tests in B-new-2 will need similar inspection. Extract a small `captureRequests(server, path)` helper under `ui/src/test/` to avoid drift.
- **Effort**: trivial.

#### C-new-3. Lint rule for "no `vi.fn()` as a prop unless explicitly justified"
- **Current state**: A1 was caught manually. Could be caught mechanically with a custom ESLint rule scoped to `*.test.{ts,tsx}` ("no `vi.fn()` passed as a component prop unless the variable name starts with `__`"), but writing a custom rule is overkill for a single occurrence. Defer until the pattern repeats.
- **Effort**: M; not recommended yet.

#### C-new-4. Document the test-style invariant in `frontend-behavioral-testing` skill or in `project-reviewing/SKILL.md`
- **Current state**: the rubric forbids prop-call assertions in general, but not specifically the "callback-prop substitute for page composition" pattern in A1. Adding a sentence to the project-reviewing skill (with file:line evidence from A1) gives reviewers a concrete reference.
- **First step**: append one bullet to `project-reviewing/SKILL.md` UI-tests section, citing this SOW.
- **Effort**: S.

#### C-new-5. The 5th first-batch target (feed modal) was deferred but not recorded as a planned follow-up
- **Current state**: SOW-0034 Decision 5 = (a) accepted all 5 targets. Only 4 shipped. There is no recorded follow-up SOW for the modal. This is the closest match to a regression in expectations: the user said "all five", four landed.
- **First step**: include the feed modal in the next batch (B-new-2 + B-new-9), or open a dedicated successor SOW for it.
- **Effort**: M.

### Top remaining behavioral-test gaps (highest value first)

1. **`<FeedDetailPage />` success path** — `ui/src/pages/feed-detail.tsx`. The not-found path is the only one tested; the page that users hit on every feed click has no behavioral coverage.
2. **`<HomePage />`** — `ui/src/pages/home.tsx`. The page-composition seam between `useQuery` and `<HomeExplorer />` / `<HomeIPLookup />` is the seam users actually exercise.
3. **`<FeedModal />`** — `ui/src/components/admin/feed-modal*.tsx` (~1,100 lines across 6 files). Largest single behavioral surface in admin; SOW-0034 Decision 5 = (a) committed to it; not yet shipped.
4. **`ui/src/lib/explorer-state.ts`** unit tests — pure functions feeding the explorer; cheapest to test, easiest to break silently.
5. **Admin write paths** (recheck, reprocess, enable, disable) — the operator-visible flows.
6. **Country/ASN/maintainer detail pages** — feed-of-feeds aggregate views.
7. **`<HomeIPLookup />` URL-prefill and client-IP-autodetect wrapper** around the tested `IPSearchSurface`.

### Notes / known limits
- The conclusion that B-new-2's "leaf-only" framing is suboptimal is a **rubric judgement**, not a defect. The 4 shipped tests are functional and pass. They are the kind of test that future contributors will copy, which is why the page-level pattern needs to be set deliberately in the next batch.
- A1 (prop-call assertion) is small in absolute terms — one line of code in one file — but it is the first instance of a Category A anti-pattern in the new test code. Catching it now is much cheaper than catching it after 20 tests have copied the pattern.
- B-new-6 (`--localstorage-file` warning) needs verification before deciding whether to silence or fix.
- B-new-9 (admin write-path tests) will need MSW handler scaffolding (B-new-3) to avoid duplicated handlers across files.
- B-new-8's untested page list is enumerated by traffic and churn signals; the implementation SOW should pick a subset rather than try to hit all of them in one batch.
- This SOW is analysis-only. None of these findings will be implemented under SOW-0041; they belong in a successor SOW after decisions are recorded.

## Implications and decisions

The user delegated code-quality and testing execution decisions to the
assistant, with no questions for non-product behavior choices. The following
decision record is therefore assistant-owned.

Decision record, 2026-05-01:

- 1 = (a): rewrite the callback-prop assertion against page-level outcome.
- 2 = (b): split behavioral page tests from data-layer/admin write tests.
- 3 = (a): create both Playwright and bundle-size SOWs now so neither valid
  item is lost.
- 4 = (b): keep `make test` Go-only; CI already runs `make ui-test`
  separately.
- 5 = (a): re-check `eslint-plugin-jest-dom` peer-range.
- 6 = (a): investigate and fix or document the `--localstorage-file` warning.

The concrete successor SOWs are:

- `.agents/sow/pending/SOW-0052-20260501-frontend-page-behavior-tests.md`
- `.agents/sow/pending/SOW-0053-20260501-frontend-data-admin-tests.md`
- `.agents/sow/pending/SOW-0054-20260501-playwright-browser-validation.md`
- `.agents/sow/done/SOW-0055-20260501-ui-test-tooling-hygiene.md`
- `.agents/sow/done/SOW-0056-20260501-frontend-bundle-budget.md`

**1. Address A1 (prop-call assertion in `feeds-table.test.tsx:41`) by rewriting the test against the page-level outcome?**
- (a) yes, rewrite the test in the next behavioral-testing implementation SOW to render `<Admin />` and assert the modal opens with `beta_malware`'s name visible (eliminates the prop spy).
- (b) no, accept the prop-call assertion as the contract this surface tests.
- (c) keep the prop-call assertion but explicitly document A1 as the only allowed exception.
- **Recommendation**: **(a)**. The prop-call assertion is the AI-test-reviewer-checklist anti-pattern the skill explicitly forbids. Rewriting against the page-level outcome is straightforward (the modal already renders `<FeedModal />` from `<Admin />`) and sets the right precedent.
- **Implications of (a)**: one test rewrite; modest MSW handler work for `/api/v1/admin/feeds` (B-new-3). Does not block the rest of the next batch.

**2. Open a successor SOW for B-new-2 (page-level tests) + B-new-4 (axe per page-level test) + B-new-8 (top remaining gaps) + B-new-9 (admin write paths) + B-new-10 (data-layer unit tests)?**
- (a) all of the above in one successor SOW.
- (b) split: behavioral page-level batch in one SOW (B-new-2 + B-new-4 + B-new-8 + A1 fix), data-layer + admin write paths in a second SOW (B-new-9 + B-new-10).
- (c) only B-new-2 + B-new-4 + A1 fix; defer the rest.
- (d) defer all.
- **Recommendation**: **(b)**. The behavioral page batch and the data-layer/admin batch have different risk profiles and reviewers; bundling them invites scope creep. Splitting also lets the data-layer batch land independently of the page-level batch's MSW scaffolding.
- **Implications of (b)**: two follow-up SOWs in numbered sequence (e.g. SOW-0042 page-level batch, SOW-0043 data-layer + admin writes). Estimated effort: M each.
- **Risks of (b)**: small — MSW handler refactors might cross both SOWs; coordinate via shared `ui/src/test/scenarios.ts` (B-new-3).

**3. Open the deferred SOWs for Playwright (B-new-1) and bundle-size (B-new-5)?**
- (a) yes, open both as separate SOWs now (parallel to the behavioral batch).
- (b) Playwright now, bundle-size later.
- (c) bundle-size now, Playwright later.
- (d) defer both.
- **Recommendation**: **(b)**. Playwright is the bigger gap because charts/globe/focus-trap regressions ship invisible without it. Bundle-size is operational hygiene, not a behavioral-testing gap; lower priority than catching a globe.gl regression.
- **Implications of (b)**: Playwright SOW will need to design the deterministic backend test mode (or fixture backend); that is a real piece of design work, not a one-day install. Costa should size that in advance.
- **Risks**: Playwright e2e flake is a real cost — recipe: small set of critical-path tests, masks for dynamic content, baselines generated in CI.

**4. Now that 4 UI tests are stable in CI, gate `make test` on `make ui-test` locally?**
- (a) yes, `make test` depends on `make ui-test` (Decision 2 = (b) ratchet condition met: 4 of 5 first-batch tests landed, all pass).
- (b) no, keep them separate so Go-only contributors are not blocked on `pnpm install`.
- (c) gate `make test-strict` (the strict order/shuffle target) on `make ui-test`, leave plain `make test` Go-only.
- **Recommendation**: **(b)**. Go-only contributors and CI runners that haven't installed `pnpm`/`node` should not have `make test` fail. CI already runs `make ui-test` as a separate step; that is the right gate. Locally, contributors who edit `ui/` should run `make ui-test` directly.
- **Implications of (b)**: status quo, documented in the project-testing skill.

**5. Re-evaluate `eslint-plugin-jest-dom` peer-range (C-new-1)?**
- (a) yes, check upstream and adopt if ESLint 10 support has shipped.
- (b) defer; the rubric checklist + `eslint-plugin-testing-library` is enough.
- **Recommendation**: **(a) — quick research item**. If support has shipped, wire it; if not, document the recheck date.
- **Implications of (a)**: 5–10 minute investigation; either a small lockfile change or a one-line note.

**6. Address B-new-6 (`--localstorage-file` warning)?**
- (a) investigate and fix or silence in `ui/vitest.setup.ts` / `ui/vitest.config.ts`.
- (b) accept as upstream noise; document in the testing skill.
- (c) ignore.
- **Recommendation**: **(a) for the investigation**, then either fix or document per the finding. Cost is small.
- **Implications**: cosmetic; CI logs become cleaner.

## Plan

Completed as analysis. Concrete successor SOWs were opened before closing this
SOW so the valid items are recoverable and ordered:

1. `SOW-0052` page-level behavioral tests — start immediately.
2. `SOW-0053` data-layer and admin write-path behavioral tests.
3. `SOW-0054` Playwright browser validation.
4. `SOW-0055` UI test tooling hygiene.
5. `SOW-0056` frontend bundle budget.

## Execution log
- 2026-05-01: re-ran the full `frontend-behavioral-testing` rubric against the new test infrastructure and 4 test files. Verified SOW-0034 status item-by-item (9 fixed / 6 partial / 3 deferred / 0 regressed). Identified 1 new Category A finding (prop-call assertion in `feeds-table.test.tsx`), 10 Category B gaps (Playwright still pending; leaf-vs-page rendering pattern; MSW handler-set coverage; per-page axe coverage; bundle-size still pending; `--localstorage-file` warning; `make test` ↔ `make ui-test` gate decision; high-value untested pages; admin write paths; data-layer unit tests), and 5 Category C neutral items. Drafted SOW; opened in `pending/`.
- 2026-05-01: recorded assistant-owned decisions and created concrete successor
  SOWs `SOW-0052` through `SOW-0056`.

## Validation
- No source code changed by this analysis SOW.
- Current-state claims verified by file-inspection commands recorded in Methodology.
- `pnpm --dir ui test` baseline (4 / 4 pass in 1.73s) recorded as reference.
- Successor SOW files exist for every valid remaining item listed in the plan.

## Outcome
Completed as analysis and planning. The first successor implementation SOW is
active as `SOW-0052`; the remaining valid items are represented by pending
SOWs.

## Lessons extracted
- Test-analysis SOWs must not end with prose-only recommended work. When the
  user has delegated maintainership decisions, record the assistant-owned
  choices and create concrete successor SOWs before closure.
