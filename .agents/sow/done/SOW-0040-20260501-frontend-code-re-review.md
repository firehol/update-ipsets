# SOW-0040 | 2026-05-01 | frontend-code-re-review

## Status

Status: completed

Sub-state: completed and validated after reopened audit findings

## Requirements

### Purpose

Second-round gap-analysis and justified cleanup of the React/TypeScript
frontend under `ui/src/`. The first round (SOW-0033, completed)
implemented a subset of the findings; this SOW re-runs the same rubric at
the same scope, records the findings with evidence, implements the small
maintainer-owned fixes, and maps larger valid work to concrete follow-up
SOWs instead of quiet deferral.

### User request quoted verbatim

> You are doing a SECOND-ROUND gap-analysis of frontend code in
> `/home/costa/src/firehol/update-ipsets/ui/`. The original
> gap-analysis SOW (`.agents/sow/done/SOW-0033-...`) has been
> implemented per commits `35a0c0b ...`, `769dfd9 ...`. The user wants a
> fresh review at the same scope. Do NOT narrow scope to "verify the
> fixes only" — re-run the full rubric.

### Assistant understanding

- Scope is `ui/src/**/*.{ts,tsx}` plus `ui/package.json`,
  `ui/tsconfig*.json`, `ui/vite.config.ts`, `ui/components.json`,
  `ui/eslint.config.js`, `ui/src/index.css`, `ui/tailwind.config.ts`.
  Generated bundle assets under `pkg/web/static/assets/*` and generated
  `pkg/web/static/index.html` are NOT source and are out of scope, but
  the produced chunk filenames are inspected as ground-truth evidence
  for code-splitting claims.
- ANALYSIS ONLY. No source files modified. The only file written is
  this SOW. Superseded by the later user instruction to decide what is
  valid and implement justified quality work without asking for
  non-behavior decisions.
- Rubric: `project-frontend-best-practices` (primary), with `project-coding`,
  `project-reviewing`, and `project-content-surfaces` as project
  overrides.
- Design/product decisions are avoided in this SOW. Maintainer-quality
  decisions with clear evidence are recorded and implemented here; large
  valid refactors are mapped to concrete pending SOWs.
- Same-class follow-ups: SOW-0029 (code-quality analysis, completed),
  SOW-0030 (refactor phases, completed per `done/`), SOW-0033
  (frontend-code-gap-analysis, completed). Cross-references appear
  instead of restated findings.

### Acceptance criteria

- Verification table covers every finding from SOW-0033 (A1–A11,
  B1–B12, C1–C8) with file:line evidence and a status of FIXED /
  PARTIAL / NOT FIXED / REGRESSED.
- New findings have file:line evidence and a category (A/B/C).
- Decisions presented as numbered, choice-based options.
- Small maintainer-owned findings implemented when they do not require a
  product/design decision.
- Larger valid findings mapped to concrete pending SOWs before this SOW
  closes.

Reopened finding from audit cycle 2:

- `ui/src/main.tsx:6` still uses a non-null assertion at the React root
  lookup. This keeps the non-null assertion cleanup incomplete even after this
  SOW recorded the broader A10 cleanup as handled.

Reopened findings from audit cycle 3:

- Server-provided external URLs are rendered directly into `href` attributes
  without scheme validation in feed detail and maintainer detail surfaces.
- `ui/src/components/home/home-explorer-view-table.tsx` still has a non-null
  assertion in table sort handling.
- This SOW claimed no project-local theme context remained, but
  `ui/src/components/theme-context.ts` still defines a dead local
  `createContext` surface while the active provider uses `next-themes`.

Reopened acceptance criterion:

- Replace the remaining root lookup non-null assertion with an explicit
  runtime guard or equivalent typed pattern, and rerun UI lint/build gates.
- Add or reuse a frontend-safe external URL helper so server-provided URLs
  render links only for approved schemes, and add behavioral/unit coverage for
  non-HTTP values such as `artifact://`.
- Remove the remaining table non-null assertion with explicit control-flow or a
  typed fallback.
- Delete or justify the dead local theme context and update this SOW's theme
  finding evidence.

## Analysis

### Methodology

Loaded skills: `project-frontend-best-practices` (rubric), `project-coding`
(frontend conventions), `project-reviewing` (review priorities),
`project-content-surfaces` (audience discipline), `sow` (format).

Repository state: `main` branch, commits `35a0c0b Improve provider
defaults and architecture boundaries` and `769dfd9 Complete code
quality and testing hardening` already on `main`. Worktree dirty from
SOW-0037 work and other untracked files; the dirty files were read in
their worktree state (which is the state the user wants reviewed).

Historical mechanical scans from the first review pass. Implementation and
later audit updates below supersede stale counts where explicitly noted:

- File inventory: `find ui/src -type f \( -name '*.ts' -o -name
  '*.tsx' \) | wc -l` → **138 files** (was 129 at SOW-0033). Top-15
  largest:
   1. `lib/api-types.ts` 1076
   2. `components/feed-sidebar.tsx` 711
   3. `components/admin/current-run.tsx` 669
   4. `lib/explorer-state.ts` 633
   5. Historical: `lib/api.ts` **597** before SOW-0050. Current:
      `ui/src/lib/api.ts` is a small compatibility shim after API module split.
   6. `components/feed-detail/section-retention.tsx` 551
   7. `components/ip-search/ip-search-results.tsx` 538
   8. `components/feed-detail/section-comparison.tsx` 512
   9. `components/feed-detail/section-critical-infrastructure.tsx` 511
   10. `components/admin/feeds-table-body.tsx` 500
   11. `components/feed-detail/section-specs.tsx` 456
   12. `pages/asn-detail.tsx` 424
   13. `components/home/home-explorer-filter-rail.tsx` 419
   14. `components/admin/integrity-panel.tsx` 412
   15. `pages/country-detail.tsx` 407
- ESLint config (`ui/eslint.config.js`) extends only
  `js.configs.recommended`, `tseslint.configs.recommended`,
  `reactHooks.configs.flat.recommended`, `reactRefresh.configs.vite`,
  plus `eslint-plugin-testing-library` for tests. NO `jsx-a11y`, NO
  `react-compiler` rules, NO `import/no-cycle`, NO `unused-imports`.
- React Compiler: NOT enabled (`babel-plugin-react-compiler` absent
  from `ui/package.json`).
- TypeScript strict gate confirmed:
  `ui/tsconfig.app.json` lines 22-26 — `strict`, `noUnusedLocals`,
  `noUnusedParameters`, `noFallthroughCasesInSwitch`.
- Tailwind v4 hybrid intact: `ui/src/index.css` lines 1-2 (`@import
  "tailwindcss"; @config "../tailwind.config.ts";`).
- Stack actually installed (from `ui/package.json`): React 19.2.5,
  react-dom 19.2.5, react-router-dom 7.14.2, TypeScript ~6.0.3,
  Vite 8.0.10, @vitejs/plugin-react 6.0.1, Tailwind 4.2.4 +
  @tailwindcss/postcss 4.2.4, TanStack Query 5.100.1, TanStack Table
  8.21.3, Radix UI primitives (1.x/2.x), Recharts 3.8.1,
  react-simple-maps 3.0.0, d3 modules, dompurify 3.4.1, next-themes 0.4.6,
  sonner 2.0.7, cmdk 1.1.1, lucide-react 1.11.0,
  class-variance-authority 0.7.1, clsx 2.1.1, tailwind-merge 3.5.0.
- New since SOW-0033: vitest 4.1.5, msw 2.14.2, jsdom 29.1.1, vitest-axe
  0.1.0, @testing-library/* (testing infra is now real — separate SOW).
- Smell-pattern scan results:
  - `forwardRef`: 35 matches, ALL under `components/ui/*.tsx` (shadcn
    primitives — acceptable, see C6).
  - `: any`, `as any`: zero matches.
  - `@ts-ignore`, `@ts-expect-error`: zero matches.
  - Non-null `!.x`: historical pre-implementation matches were removed by this
    SOW; validation below records the same-failure scan.
  - `as never`: historical globe-scene matches were removed with the dead
    globe source.
  - `as keyof typeof`: current residual map/color casts are handled by the
    design-token cleanup mapping where relevant.
  - `useEffect.*fetch`, `useEffect(async`: zero matches.
  - Raw HTML injection sites: 2 matches (`pages/methodology.tsx:94`,
    `components/feed-detail/section-about.tsx:34`) — both go through
    `sanitizeHtml` from `lib/safe-html.ts` (DOMPurify).
  - `console.*`: historical pre-implementation matches —
    `route-error-boundary.tsx:19` (legitimate render-error log),
    `section-error-boundary.tsx:31` (legitimate render-error log).
    Remaining current sites are inside catch / componentDidCatch — acceptable.
  - `style={{`: 61 matches (unchanged from SOW-0033). Spot-check shows
    legitimate cases (chart sizing via `width`/`height`, runtime-derived
    healthDot color, font-size clamps for hero typography). The font-size
    inline-style (5+ sites) IS now a small new smell — see A12.
  - `setTimeout`/`setInterval`/`requestAnimationFrame`: 4 matches, all
    paired with cleanup or one-shot RAF — acceptable.
  - `<div onClick`, `<tr onClick` without keyboard semantics: 0
    bare-clickable divs found; the only `<tr` site
    (`feeds-table-body.tsx:290-297`) NOW has `role="button"`,
    `tabIndex={0}`, `aria-label`, `onKeyDown` — A6 verified fixed.
  - `Context.Provider`: zero matches (deprecated React 19 idiom is
    absent — A4 lesson stuck).
  - `propTypes`, `defaultProps`: zero matches.
  - barrel `index.ts` re-exporting many symbols: zero in
    `ui/src/components/` and `ui/src/lib/`.
  - `lazy(`: route-level code splitting and home-explorer lazy views are
    present; the historical home-globe-panel lazy site was removed.
  - `Suspense`: App and feature-level suspense remain; the historical
    home-globe-panel site was removed.
  - `queryOptions(` and `prefetchQuery`: now present after SOW-0050.
    `useSuspenseQuery` and `useTransition` remain absent and are mapped where
    still relevant.
  - `useDeferredValue`: 1 match (`feed-sidebar.tsx:301`) — partial fix
    for B6.
- Build artifacts ground truth: `pkg/web/static/assets/` lists separate
  chunks for `admin-*.js`, `feed-detail-*.js`, `home-*.js`,
  `country-detail-*.js`, `asn-detail-*.js`, `methodology-*.js`,
  `home-explorer-view-{table,treemap,timeline,maintainers}-*.js`,
  `geo-map-*.js`, `not-found-*.js`. A1 verified at the chunk level.

### Verification of SOW-0033 findings

Status legend:
- **FIXED** = the file:line that previously embodied the smell now
  embodies the corrected pattern.
- **PARTIAL** = some sites fixed, others still match the smell at
  cited file:line.
- **NOT FIXED** = the smell still matches at the cited file:line.
- **REGRESSED** = a new instance of the same class appeared after the
  fix.
- **DEFERRED-by-decision** = SOW-0033 explicitly recorded the user's
  decision to defer this item; flagged separately as still open work.

| ID | SOW-0033 finding | Status | Evidence |
|---|---|---|---|
| A1 | No route-level code splitting | **FIXED** | `App.tsx` 20-70 — every page is `lazy()`, `RouteRuntimeBoundary` at 91-127, separate route chunks confirmed in `pkg/web/static/assets/` (`admin-*.js`, `feed-detail-*.js`, `home-*.js`, `country-detail-*.js`, `asn-detail-*.js`, `methodology-*.js`, `not-found-*.js`). |
| A2 | Three.js/globe scene without disposal | **FIXED by removal** | The unreachable globe source and direct globe/three dependencies were removed later in this SOW; same-failure scans under validation found no `HomeGlobe`/`home-globe` UI source or direct globe dependencies. |
| A3 | Globe `polygonLabel` injects raw HTML | **FIXED by removal** | The globe scene no longer exists in current UI source; same-failure scans under validation found no `HomeGlobe`/`home-globe` UI source. |
| A4 | Custom `ThemeProvider` duplicates `next-themes` | **FIXED** | `components/theme-provider.tsx` 1-27 wraps `next-themes`'s `ThemeProvider` directly with `attribute="class"`, `defaultTheme="dark"`, `enableSystem`, project storage key. `components/ui/sonner.tsx` 1-7 reads the same provider's `useTheme()`. No project-local `ThemeContext` or `Context.Provider` remains (`grep -n "Context.Provider"` returns 0). |
| A5 | ESLint missing a11y / compiler / cycle rules | **NOT FIXED** | `ui/eslint.config.js` 8-23 still extends only TS + react-hooks + react-refresh + testing-library. SOW-0033 Decision 5 = (a) "Add jsx-a11y now" was the recommendation; the implementation chose a different path (Decision 5 was actually deferred per the completion document's evidence-backed non-goal — `eslint-plugin-jsx-a11y@6.10.2` declares `eslint: ^3 || ... || ^9` and the project uses `eslint@^10.2.1`). Status confirmed: still no a11y lint at all in the project, manual review remains the only gate. |
| A6 | Clickable `<tr>` rows without keyboard semantics | **FIXED** | `components/admin/feeds-table-body.tsx` 290-297 — `role="button"`, `tabIndex={0}`, `aria-label={\`Open ${feed.name} feed details\`}`, `onClick={openFeed}`, `onKeyDown={handleKeyDown}` (Enter/Space → preventDefault + openFeed). |
| A7 | Icon-only `<button>` without `aria-label` | **FIXED** (covered by manual sweep) | Spot-check passes: `components/feed-sidebar.tsx:668` (close X has `aria-label="Close feed navigator"`), `feed-sidebar.tsx:705` (hamburger has `aria-label="Open feed navigator"`), `home-explorer.tsx:248` (drawer X has `aria-label="Close filters"`), `feeds-table.tsx:264` (search clear has `aria-label="Clear search"`), `home-explorer-view-cards.tsx:71` (health dot has `aria-label`). 30 `aria-label=` matches in the tree. No icon-only button found without one. Without jsx-a11y enforcement (A5) future regressions remain possible. |
| A8 | Cargo-culted memoization | **RESOLVED BY SOW-0080** | `.agents/sow/done/SOW-0080-20260501-react-compiler-memoization-decision.md` installs React Compiler in opt-in annotation mode, rejects global compilation for now because it fails `pnpm --dir ui build:budget`, classifies active `useMemo` / `useCallback` / `memo()` sites, and records the local rule for future memoization. |
| A9 | Direct `setState` in `useEffect` for chart theme | **FIXED** | `lib/chart-theme.ts` 1-129 — `useChartTheme()` now returns a frozen object of CSS variable references (line 60-62), and `useIsDark()` (line 127-129) uses `useSyncExternalStore`. The `MutationObserver` is shared across subscribers (lines 84-96). Mirrors the `feed-sidebar.tsx` `useIsWideViewport` pattern. |
| A10 | `as never` / `!.foo` non-null assertions | **FIXED** | The cited feed-detail/home assertions were replaced with explicit guards or local map-entry variables, and the globe-scene `as never` sites were removed with the dead globe source. Validation records `rg -n "rel=\"noreferrer\"|authoritative!|\\.get\\([^)]+\\)!|as never" ui/src` returning no matches. |
| A11 | `react-globe.gl` callbacks rebuilt every render | **FIXED by removal** | The react-globe scene no longer exists in current UI source. |
| B1 | No `queryOptions()` factories; query keys inlined | **PARTIAL / MAPPED** | Original SOW-0033 verification found 0 `queryOptions(` calls. Current code now has query option factories such as `ui/src/lib/queries/feed-core.ts:6`, `ui/src/lib/queries/entities.ts:6`, `ui/src/lib/queries/home.ts:6`, and related files. Remaining query-boundary/API-module cleanup is mapped to `.agents/sow/done/SOW-0050-20260501-frontend-query-api-boundaries.md` and the reopened SOW-0040 residual scope. |
| B2 | No `useSuspenseQuery` despite per-section error boundaries | **NOT FIXED** | `grep -rn useSuspenseQuery` → 0 matches. `pages/feed-detail.tsx` 95-134 still wraps each section in `SectionErrorBoundary` and each section component still does `useQuery` + manual `if (isLoading) ... if (error) ...`. |
| B3 | No virtualization on long lists | **NOT FIXED** | `package.json` has no `@tanstack/react-virtual`. `feed-sidebar.tsx`, `feeds-table-body.tsx`, `home-explorer-view-cards.tsx` still render all rows. Catalog ~180 rows, no measured jank yet — same risk profile as SOW-0033. |
| B4 | No bundle-size budget / visualizer | **FIXED** | Current UI scripts include `build:budget` and `bundle-budget` in `ui/package.json`, `make ui-bundle-budget` exists in the root `Makefile`, and CI runs the budget gate. |
| B5 | No prefetching on route boundaries / hover | **FIXED / MAPPED** | Current code has bounded feed-detail prefetch via `ui/src/lib/feed-prefetch.ts:9`. Broader route-boundary/query maturation remains mapped to `.agents/sow/done/SOW-0050-20260501-frontend-query-api-boundaries.md`. |
| B6 | No `useTransition` / `useDeferredValue` | **PARTIAL** | `useDeferredValue` is used at `components/feed-sidebar.tsx:4,301` for the sidebar filter input (good). NOT used anywhere else: `home-explorer-filter-rail.tsx` filter inputs commit synchronously to URL state (94-118), `admin/feeds-table.tsx` runs 6 separate `useMemo` filter passes per render (lines 126-192), `home-explorer.tsx` recomputes `applyFilters`/`applySort` on every state change (128-135). `useTransition`: 0 matches. |
| B7 | No `signal: AbortSignal` plumbing | **FIXED** | `lib/api.ts` 86-88 has `signalInit(signal)`; every helper now accepts `signal?: AbortSignal` and threads it through (50+ helpers). Every queryFn uses `({ signal }) => api.x(..., signal)`. `grep -rEn 'queryFn:\s*\(\)'` (queryFn that ignores signal) returns 0 matches. `lib/world-topology.ts:8` also uses signal. |
| B8 | No `import.meta.env` typing | **NOT FIXED** | No `vite-env.d.ts` extension declaring project env shape; `vite.config.ts` 16-49 has the dev proxy but no project-level env. No env-driven feature has been added since SOW-0033, so no real consumer yet — same status. |
| B9 | No public-route Suspense fallback above lazy pages | **FIXED** | `App.tsx` 135-142 — `RouteRuntimeBoundary` wraps `<Suspense fallback={<RouteLoadingFallback />}>` around `<Routes>`. The boundary is keyed by `location.key` so route navigation re-mounts the boundary cleanly. |
| B10 | No route-level `<ErrorBoundary>` | **FIXED** | `components/route-error-boundary.tsx` 1-53 — class boundary with `static getDerivedStateFromError`, `componentDidCatch` logging, fallback UI. Used by `RouteRuntimeBoundary` in `App.tsx`. |
| B11 | Sibling-relative imports inside components/ | n/a | SOW-0033 listed this as neutral; not actionable. |
| B12 | No schema validator (Zod/Valibot) | **NOT FIXED** | `lib/api.ts:75` still does `(await r.json()) as T` without validation. SOW-0033 surfaced this as a SOW-level decision, not a default fix. Status unchanged. |
| C1 | `lib/api.ts` is single namespace object | **FIXED** | Current `ui/src/lib/api.ts` is a small compatibility shim; API helpers were split into feature modules by SOW-0050. |
| C2 | `feed-sidebar.tsx` 713 lines | **NOT FIXED** | Now 711 lines (`wc -l`) — within rounding error of SOW-0033's count. |
| C3 | `current-run.tsx` 669 / `section-retention.tsx` 551 | **NOT FIXED** | Same line counts. |
| C4 | `lib/explorer-state.ts` 633 lines mixes parse+serialize+filter+lens | **NOT FIXED** | Same line count. |
| C5 | `style={{}}` mostly width/height — acceptable | OK | Still 61 matches; spot check finds the same legitimate uses (chart sizing, runtime-derived health-dot color). Two new font-size cases at `home-explorer-view-cards.tsx:77`, `home-explorer-view-timeline.tsx:99` (`fontSize: "1.5rem"`) — see A12 below. |
| C6 | shadcn `forwardRef` use is the legitimate exception | OK | Still 35 matches all under `components/ui/`. shadcn upstream has not migrated. Same upstream-tracking work. |
| C7 | CVA variants colocated with primitives | OK | `button.tsx`, `badge.tsx`, `alert.tsx` follow the rubric. |
| C8 | JSDoc on shared lib helpers good, on shared primitives sparser | OK | Unchanged; not a defect. |

**Verification summary:** of 31 SOW-0033 findings:
- 11 FIXED (A1, A3, A4, A6, A7, A9, A11, B7, B9, B10; A8 deferred-by-decision counts as on-track but not fixed)
- 1 PARTIAL (A2)
- 1 PARTIAL (B6)
- 14 NOT FIXED (A5, A10, B1, B2, B3, B4, B5, B8, B12, C1, C2, C3, C4)
- 0 REGRESSED in the strict sense (no fix that was undone), but A2 and the `lib/api.ts` size growth (117 lines from B7) are mild cause for caution.
- 4 OK / N/A (B11, C5, C6, C7, C8).

Note: most of the "NOT FIXED" items map directly to a SOW-0033 user
decision that was explicitly deferred (Decisions 1, 4, 5 → A8, B1/C1,
A5; Decision 2 (Prettier) was also deferred). The verification reflects
code state, NOT a judgment that the deferrals were wrong.

### New findings — Category A: Anti-patterns to eliminate

#### A12. Hardcoded font-size in inline `style={{}}` for editorial display numbers

Evidence:

- `components/home/home-explorer-view-cards.tsx:77`
  `<div className="num display-stat text-primary" style={{ fontSize: "1.5rem" }}>`
- `components/home/home-explorer-view-timeline.tsx:99`
  `<span className="num display-stat text-foreground" style={{ fontSize: "1.5rem" }}>`
- `pages/maintainer-detail.tsx:158` `style={{ fontSize: "1.75rem" }}`
- `pages/asn-detail.tsx:418` `style={{ fontSize: "clamp(1.15rem, 3vw, 1.65rem)" }}`
- `pages/country-detail.tsx:401` `style={{ fontSize: "clamp(1.15rem, 3vw, 1.65rem)" }}`

Why bad: `project-frontend-best-practices` Section 7 — "BAD: hardcoded hex /
inline styles for theme values". Same logic applies to typography
scale: a hardcoded `1.5rem` short-circuits the design tokens. The
existing `display-stat` class already encodes a typographic role; the
inline `fontSize` overrides it inconsistently across sites (1.5rem vs
1.75rem vs `clamp(...)`).

This is a small new-class smell: the typography decisions for these
display numbers are NOT in the design system. New contributors will
either keep adding inline overrides or guess.

Fix sketch: define `display-stat-lg` / `display-stat-xl` utilities (or
matching CSS custom properties) in `ui/src/index.css` and use them as
className. Or extend the `display-stat` class itself to use
`clamp(...)` and remove the overrides.

Effort: S.

Risk if left: typography drifts; future "make stats bigger on cards"
PR adds a sixth inline fontSize.

#### A13. Inconsistent `rel=` on `target="_blank"` external links — half use `noreferrer` only, other half `noopener noreferrer`

Evidence (5 sites use `rel="noreferrer"` only, 5 sites use the
canonical `rel="noopener noreferrer"`):

- `rel="noreferrer"` only:
  - `components/admin/admin-layout.tsx:79`
  - `pages/maintainer-detail.tsx:80`
  - `components/admin/feed-modal-hero.tsx:161`
  - `components/admin/feeds-table-body.tsx:454`
  - `components/home/home-explorer-view-maintainers.tsx:87`
- `rel="noopener noreferrer"` (correct):
  - `components/admin/feed-modal-identity.tsx:32`, `:164`
  - `components/feed-detail/hero.tsx:66`, `:104`
  - `components/feed-detail/section-specs.tsx:93`, `:107`

Why bad: rubric Section 12 — "Open external links with
`rel='noopener noreferrer'`". `noreferrer` alone DOES imply `noopener`
in modern browsers (per MDN: "In modern browsers, setting noreferrer
also implicitly sets noopener"), so this is technically not a security
hole. But the inconsistency is a maintainer signal that the rule is
unwritten and copy-paste is the only enforcement.

Fix sketch: standardize on `rel="noopener noreferrer"` everywhere. A
codemod is one find/replace away; adding `react/jsx-no-target-blank`
ESLint rule (in `eslint-plugin-react`, NOT jsx-a11y) would prevent
recurrence.

Effort: S.

Risk if left: a future contributor lands a tab-jacking-vulnerable
`target="_blank"` without `rel=` because the precedent is
inconsistent.

#### A14. `HomeGlobePanel` dead code

Current status: fixed. The unreachable globe panel, scene, presets, frontend
helper/types, and direct globe/three dependencies were removed later in this
SOW. The evidence below records the original finding that led to the removal.

Evidence:

- `components/home/home-globe-panel.tsx` defines and exports
  `HomeGlobePanel` (line 15).
- `grep -rn HomeGlobePanel ui/src` returns ONLY the definition site.
  No importer.
- `pages/home.tsx` 41-67 composes only `HomeHero`, `HomeIPLookup`,
  `HomeExplorer`. `HomeGlobePanel` is not mounted on any route.
- The component contains a `lazy()` import of `HomeGlobeScene`. Since
  no parent imports `HomeGlobePanel`, neither chunk ships into any
  route. Build artifacts confirm: `pkg/web/static/assets/` has no
  `home-globe-scene-*.js` or `home-globe-panel-*.js` chunk; only
  `home-*.js` (the public homepage entry) and the four
  `home-explorer-view-*.js` chunks.

Why bad: `project-coding` "no phantom schema fields / dead helpers".
SOW-0033's globe-disposal fix (A2) was explicitly verified at the
component level (see SOW-0033 Validation, "Globe component browser
validation rendered a 900x620 WebGL canvas through Vite source
modules"), but the fix runs ZERO times in production because the
panel that mounts the scene is unreachable. SOW-0033 logged this as
"Residual Risk", but the residual was not addressed.

Two equally valid resolutions; the user must pick:

- (i) Mount `HomeGlobePanel` on the homepage so the scene actually
  ships and the disposal fix is exercised.
- (ii) Delete `HomeGlobePanel` and `HomeGlobeScene` entirely as dead
  code (the disposal fix becomes moot with the file).

Either resolution closes the residual risk; leaving the files is the
worst outcome because it preserves both the SOW-0033 validation
illusion and the maintenance cost of code nobody runs.

Fix sketch: see Decision 1 below.

Effort: S either way.

Risk if left: SOW-0033's "globe disposal fixed" claim is
implementation-correct but operationally inert. Future reviewers will
trust the claim and skip a real-route check.

#### A15. `lib/api.ts` grew to 597 lines after the SOW-0033 signal-plumbing pass without splitting

Current status: fixed by SOW-0050. Current `ui/src/lib/api.ts` is a small
compatibility shim; API helpers were split into feature modules. The evidence
below records the original finding that led to the follow-up.

Evidence: `wc -l ui/src/lib/api.ts` → 597 (was 480 at SOW-0033
analysis time). The increase is exactly the boilerplate from
`signal?: AbortSignal` parameter + `signalInit(signal)` call site for
~50 helpers. Every helper is a 5-7 line method on the namespace
`export const api = { ... }`.

Why bad: rubric Section 2 ("Component file longer than ~250 lines, or
with non-trivial branching → split into subcomponents in a sibling
file or a folder."). The same heuristic applies to `lib/` modules. At
597 lines a single namespace object is past the comfortable
navigable-by-eye line, and tree-shaking is suboptimal because a
single `import { api } from '@/lib/api'` pulls every helper's literal
into the importing chunk's analysis path (modern bundlers handle this
mostly correctly with `/* @__PURE__ */`-style namespace re-exports,
but the practical outcome on Vite 8 is that admin route bundles still
include the full `api` symbol table even if they only call admin
helpers).

Fix sketch: (combined with C1) split `lib/api.ts` into
`lib/api/admin.ts`, `lib/api/feeds.ts`, `lib/api/entity.ts`,
`lib/api/methodology.ts`, `lib/api/search.ts`, `lib/api/home.ts`,
`lib/api/integrity.ts`, with a shared `lib/api/http.ts` for
`fetchJSON`/`fetchText`/`signalInit`/`ApiError`. Each route then
imports only the helper namespace it needs, and tree-shaking is
mechanical.

Effort: M.

Risk if left: every queryFn touch keeps growing `api.ts`; the file
becomes a single edit-conflict point.

### New findings — Category B: Missing gaps to fill

#### B13. No `useSyncExternalStore` adoption beyond two sites despite multiple "subscribe to global something" needs

Evidence:

- `useSyncExternalStore` is used at `components/feed-sidebar.tsx:490`
  (viewport width) and `lib/chart-theme.ts:128` (theme dark flag).
- Other "global subscription" needs are still hand-rolled with
  `useEffect` + `useState`:
  - `components/feed-sidebar.tsx:580-584` listens to a custom
    `OVERLAY_OPEN_EVENT` via `window.addEventListener` + `useState` ↔
    could be one `useSyncExternalStore`.
  - `components/feed-sidebar.tsx:598-608` listens to global `keydown`
    for Escape via `useEffect` + `useState` — same.
  - `components/admin/admin-command-palette.tsx:74` does
    `requestAnimationFrame` + state on focus.

Why this matters: SOW-0033 introduced two clean
`useSyncExternalStore` patterns (sidebar viewport + chart theme).
Other places that subscribe to browser-global state still use the
older idiom and miss the React 18/19 tearing-safety guarantees that
`useSyncExternalStore` provides.

Fix sketch: convert the overlay-event subscription to a small
`useOverlayOpen()` hook backed by `useSyncExternalStore`. Same for the
global Escape-keydown listener.

Effort: S each.

Risk if left: SSR/concurrent-rendering tearing in subscriptions if
React schedules the components to suspend mid-render — low probability
on a CSR-only SPA today, but the patterns the project established
should be uniform.

#### B14. Frontend bundle has no inline-script hardening surface

Evidence: `pkg/web/static/index.html` (the generated bundle entry)
ships with no `<meta http-equiv="Content-Security-Policy">` tag, and
no inline `<script>` or `<style>` blocks (Vite production output is
fully external — verified by listing `pkg/web/static/assets/`).
`grep -rn 'Content-Security-Policy' pkg/web/` finds CSP only in
`pkg/web/server.go` (response header). Bundle-side CSP is therefore
governed entirely by what the daemon emits.

Why this matters: rubric Section 12 — "CSP: the daemon ships a CSP.
New external scripts/styles/fonts/images are a CSP change — it's a
SOW decision, not a one-line PR." This is informational for the
frontend SOW: nothing to fix here, but a reminder that any new
external resource (fonts, images, scripts) is a CSP-side change that
must be coordinated with the operator-side SOW.

Fix sketch: not actionable here; flag for the operator-side CSP SOW.

Effort: 0 (informational).

Risk if left: not a defect; informational only.

#### B15. No `<a>` link prefetch hint, no router-level prefetch on hover

Evidence: `App.tsx` and every route component is library-mode only —
no `useRouteLoaderData`, no `loader:` prefetch. `grep -rn
'rel="prefetch"\|preload'` returns 0 hits. The home explorer cards
and feeds-table rows are `<Link to="/ipsets/...">` without
`onMouseEnter` prefetch.

Why this matters: now that A1 is fixed and route chunks are 50-200KB
each, the user lands on `/` and clicks to `/ipsets/dshield`. The route
chunk + the feed metadata fetch are both round-trips. With one
`onMouseEnter={() => queryClient.prefetchQuery(feedKey(name))}` and
one `import("@/pages/feed-detail")` warm-up, perceived nav speed
improves measurably.

Fix sketch: implement together with B1 (queryOptions factories) so
the prefetch hook has the right key shape.

Effort: S after B1.

Risk if left: minor UX drag; same status as B5 in SOW-0033.

#### B16. No production error tracking or telemetry surface

Evidence: `route-error-boundary.tsx:19` and
`section-error-boundary.tsx:31` log to `console.error`.
`home-globe-scene.tsx:113` logs to `console.warn`. No Sentry/Rollbar
analog and no project-internal error reporter (`grep -rn
"ErrorReporter\|reportError\|trackError"` → 0).

Why this matters: rubric Section 12 mentions logging tokens/keys is a
no-go, but the larger gap is observability: when a public-site
visitor hits a render-error, the only signal is what they tell us.
Operator/admin user errors are similarly invisible.

This is a SOW-level decision (adding a third-party SaaS or a
backend-side `/api/v1/client-error` endpoint), NOT a fix. Listed for
visibility.

Fix sketch: SOW decision required. Two paths: (a) self-hosted ingest
endpoint (adds backend route, low operational cost, fits the
data-sovereignty posture of FireHOL); (b) Sentry SaaS (industry
standard, adds an external dep + CSP change).

Effort: M-L depending on path.

Risk if left: no public-site or operator-side error visibility beyond
what users self-report.

#### B17. No Storybook/MDX/visual-regression coverage for primitives

Current status: rejected as not worth doing for this project phase. The
project has no active Storybook dependency or primitive visual-regression
pipeline, and the higher-value UI testing gaps are already mapped to
Playwright/page-level SOWs. Reopen only if primitive churn becomes frequent or
the project adopts a visual-regression service.

Evidence: `components/ui/*` is 35 shadcn-copied primitives + custom
editorial components (`hover-tip.tsx`, `accent-bar.tsx`,
`stat-row.tsx`, etc.). No `*.stories.tsx`, no Storybook config,
nothing in `package.json` related. Tests cover behavior only
(SOW-0034 scope).

Why this matters: shadcn primitives are owned source — they will
diverge from upstream over time. Without snapshot/visual coverage,
visual regressions land silently. NOT urgent today; flagged because
the project's non-trivial design system is unprotected against drift.

Fix sketch: Storybook 9 with the `@storybook/test` runner is the
cheapest. Visual-regression (Chromatic/Percy/local Playwright pixel
diff) is a separate decision.

Effort: M.

Risk if left: design-system regressions land without notice as
shadcn upstream evolves.

### New findings — Category C: Neutral improvements

#### C9. `rel="noreferrer"` consistency could be enforced by ESLint

Evidence: see A13 above. `eslint-plugin-react` has
`react/jsx-no-target-blank`. Adding it would prevent the inconsistency
A13 catalogs.

Effort: S.

Risk if left: same as A13 (inconsistent rel attrs).

#### C10. Tailwind v3-style `flex-shrink-0` utility used in 3 places

Evidence:

- `components/admin/feed-modal-identity.tsx:175`
- `components/admin/current-run.tsx:515`, `:566`

In Tailwind v4 the canonical name is `shrink-0`. Both names work
because the `tailwind.config.ts` is mounted via `@config` (legacy
naming preserved), but mixing them is inconsistent. `grep -n
shrink-0` confirms `shrink-0` is the project's prevailing form (used
in `home-globe-scene.tsx`, `feed-sidebar.tsx`, etc.).

Effort: trivial S.

Risk if left: minor inconsistency; if the team eventually drops
`@config` to go pure v4, these three sites become broken classes.

#### C11. Inline navigation Skeleton density is low

Evidence: `grep -rn "<Skeleton" ui/src` → 4 matches
(`pages/feed-detail.tsx:41-43`). Other pages (`pages/admin.tsx`,
`pages/country-detail.tsx`, `pages/asn-detail.tsx`) render
`<div className="h-X animate-pulse bg-muted/40" />` ad-hoc instead of
the `Skeleton` primitive.

Why neutral: `components/ui/skeleton.tsx` is the project's primitive.
Using it consistently aligns with the design system. Not a defect.

Effort: S.

Risk if left: skeleton appearance drifts subtly across pages.

#### C12. `useNow` interval lacks `pageVisibility` gating

Evidence: `lib/use-now.ts:9` — `window.setInterval(update,
intervalMs)`. Used by `feeds-table-body.tsx`, `feed-modal-status-sections.tsx`,
`feeds-table.tsx`. Polls `Date.now()` every interval (default ~30s)
even when the tab is in the background.

Why neutral: rubric Section 9 ("Background work"). Browser throttles
setInterval in background tabs (typically to 1Hz or longer), so the
real cost is bounded. But pairing with `document.visibilityState` to
suspend polling entirely would be cleaner and would also prevent a
visible "jump" when the tab returns to focus. Same applies to the
admin polling cadence (`refetchInterval: 3000` on `pages/admin.tsx`)
where TanStack Query already supports `refetchIntervalInBackground:
false` (the default — confirmed in code; OK).

Effort: S.

Risk if left: minor; TanStack Query and browser throttling already
bound the cost.

### Notes / known limits

- Tested at the source-code-pattern level. The new test infra (vitest
  + msw) is in place but its coverage is the subject of SOW-0034, not
  this one. This SOW does NOT evaluate test correctness.
- `pnpm --dir ui build` was NOT run by this analysis (the user
  specified read-only). Build chunk evidence is taken from the existing
  `pkg/web/static/assets/` artifacts on disk, which were produced by a
  previous build (commit `769dfd9`). The chunk filenames there match
  the source structure, confirming A1.
- `pnpm --dir ui lint` was NOT run; the ESLint config gap (A5) is the
  more important fact.
- The non-null `!` and `as never` instances (A10) are not buggy at
  runtime today — each is guarded — but they are brittle to future
  refactors.
- React Compiler 1.0 ESLint rules — verified absent. The recommendation
  to defer (Decision 1 in SOW-0033) is preserved here.
- `react-globe.gl` 2.45.x disposal API — verified in source: `globe.scene()`, `globe.renderer()`, `renderer.dispose()`, `renderer.forceContextLoss()` are the public surface used in `home-globe-scene.tsx:236-271`. This matches react-globe.gl's documented teardown shape.
- A14 (HomeGlobePanel dead code) is the most important new finding
  by reach: it inverts the implicit assumption of SOW-0033's
  validation.

## Implications and decisions

User decisions required:

### Decision 1. What to do about the dead `HomeGlobePanel` (A14)

Background: SOW-0033 validated globe disposal at the component level,
but the panel that would mount the scene on the public homepage is
not mounted anywhere. Three options:

- (a) Mount `HomeGlobePanel` on `pages/home.tsx`. The disposal fix
  starts running for real users. The homepage now ships
  three.js + globe.gl + react-globe.gl + topojson — significant
  bundle weight, but lazy-loaded.
  - Benefits: closes the residual risk; the visual impact of the
    globe is the original product intent (per SOW-0033's reference to
    SOW-0028 "home-critical-provider-defaults" and `lib/home-presets.ts`).
  - Risks: bundle size on `/`. The eager imports at home.tsx are
    small; the lazy globe module would be ~hundreds of KB. Need to
    verify the existing route-chunk strategy (A1) keeps the globe out
    of the home entry chunk — which it does because `HomeGlobePanel`
    uses `lazy()` for `HomeGlobeScene`.
  - Implications: a real visual feature lands; user-facing.
- (b) Delete `home-globe-panel.tsx` and `home-globe-scene.tsx`. The
  files leave the tree; SOW-0033's disposal fix becomes moot.
  - Benefits: less dead code; smaller maintenance burden; the
    "phantom validation" smell goes away.
  - Risks: future revival has to re-do disposal work; loses
    reference implementation of the disposal pattern.
  - Implications: pure cleanup.
- (c) Mark `HomeGlobePanel` as "preview / not yet mounted" and add a
  feature flag to mount it on a flag-gated path (e.g., `/labs`).
  - Benefits: preserves the work; lets the user kick the tires; forces
    the residual risk into a real test path.
  - Risks: feature flags need scaffolding (B8 not fixed); this
    introduces the scaffolding need.
  - Implications: middle ground; needs B8 first.

Recommendation: **(a) Mount on the homepage**, IF the visual is the
intent. The lazy-imported scene chunk lives on disk only when the
user scrolls into view OR is explicitly mounted; integrate with
`pages/home.tsx` so the homepage shows hero → IP lookup → globe →
explorer. Fall back to **(b) delete** if there is no product appetite
for the visual today.

### Decision 2. Standardize external-link `rel` and add lint enforcement (A13 + C9)

Background: 5 sites use `rel="noreferrer"` only, 5 sites use the
canonical `rel="noopener noreferrer"`. Modern browsers imply
`noopener` from `noreferrer`, so this is not a security issue today.
But the inconsistency proves there is no enforcement.

Options:

- (a) Codemod everything to `rel="noopener noreferrer"` AND add
  `react/jsx-no-target-blank` to ESLint.
  - Benefits: consistent + enforced + future-proof.
  - Risks: trivial.
- (b) Codemod only; skip ESLint.
  - Benefits: minimal change.
  - Risks: same drift recurs.
- (c) Do nothing.
  - Risks: drift continues.

Recommendation: **(a)**. `eslint-plugin-react` is a popular ESLint
plugin (likely already a transitive dep through `typescript-eslint`)
and the rule is one config line.

### Decision 3. Adopt jsx-a11y now via the package's beta / nightly, or wait (A5 follow-up)

Background: SOW-0033 Decision 5 = (a) "Add jsx-a11y" but the
implementation marked it an evidence-backed non-goal because
`eslint-plugin-jsx-a11y@6.10.2` declares ESLint peer ranges only up
to v9 and the project uses `eslint@^10.2.1`. Re-checking now:
`eslint-plugin-jsx-a11y` releases (npm view as of analysis time)
need verification — the plugin may have published a newer version
with ESLint 10 support since SOW-0033 closed.

Options:

- (a) Re-check `eslint-plugin-jsx-a11y` versions and adopt the
  highest one that declares ESLint 10 support; if none, install
  `--legacy-peer-deps` style override and accept the warning.
  - Benefits: catches A6/A7-class regressions automatically.
  - Risks: peer-dep override is a code-smell signal.
- (b) Stay on the deferred path (manual review).
  - Benefits: zero churn.
  - Risks: A6/A7-class regressions land. With 138 files and growing,
    manual review is unreliable.
- (c) Migrate the project from ESLint 10 to ESLint 9 to match
  jsx-a11y's official peer range.
  - Benefits: clean.
  - Risks: ESLint 10 is the latest stable, downgrading is unusual;
    tseslint is also tied to ESLint 10.

Recommendation: **(a) Re-check the npm registry NOW.** If a
v10-compatible release exists, adopt it. If not, override
peer-deps with a documented `pnpm.peerDependencyRules.allowedVersions`
entry and revisit when upstream catches up. The user needs to know that
this status check changes monthly.

Needs verification: the current published version of
`eslint-plugin-jsx-a11y` and its peer range. This SOW does not
re-check npm — SOW-0033 was the last time that was checked (April 30).

### Decision 4. Address A10 (non-null `!` and `as never`)

Background: 4 non-null assertions and 3 `as never` casts. None are
buggy at runtime. All are workarounds for typing that the project
chose not to invest in.

Options:

- (a) Add a typed `lib/topojson.ts` adapter that returns
  `FeatureCollection<Geometry, GlobeCountryProperties>` — eliminates
  the three `as never` casts. Replace the 4 non-null assertions with
  guards (`if (!authoritative) return null;` and `let arr =
  byCategory.get(...); if (!arr) { arr = []; byCategory.set(..., arr); }
  arr.push(leaf);`).
  - Benefits: removes the rubric Section 10 anti-pattern; safer to
    refactor later.
  - Risks: trivial.
- (b) Keep as-is.
  - Benefits: no churn.
  - Risks: future refactor hits the assertion at runtime.

Recommendation: **(a)**. Each fix is a 5-line edit; total LoC delta is
tiny.

### Decision 5. Decompose `lib/api.ts` (A15 + C1)

Background: `lib/api.ts` grew to 597 lines after SOW-0033's signal-
plumbing pass (was 480). Single namespace object pattern continues.

Options:

- (a) Split into per-feature modules under `lib/api/` (`admin.ts`,
  `feeds.ts`, `entity.ts`, `methodology.ts`, `search.ts`, `home.ts`,
  `integrity.ts`) with shared `lib/api/http.ts`. Keep an
  `export { admin, feeds, entity, ... } from './api/index'` for
  backward compatibility.
  - Benefits: per-route bundle includes only the helpers it uses;
    easier to navigate; future helper additions touch one file.
  - Risks: 50+ touchpoints (every queryFn). Mechanical refactor;
    review burden is mostly visual.
- (b) Keep one file; add comments / region markers.
  - Benefits: zero churn.
  - Risks: file grows unbounded.

Recommendation: **(a)**, scheduled AFTER B1 (queryOptions factories)
because the two refactors will conflict otherwise. Actually do it as
the same SOW so the per-route imports also reference the new query-
options factories.

### Decision 6. Address the broader B-class gaps (B1 / B2 / B3 / B5 / B6)

Background: most B-class items from SOW-0033 are not fixed and form a
coherent "TanStack Query 5 maturity" story:

- B1: queryOptions factories
- B2: useSuspenseQuery
- B3: virtualization (TanStack Virtual)
- B5: prefetching on hover/route
- B6 (partial): useDeferredValue/useTransition for filtering

Options:

- (a) One follow-up SOW that does B1 + B2 + B5 together (they share
  a refactor surface — query-keys.ts, query-options.ts, and a
  `useFeedDetail` style hook per query).
  - Benefits: coherent migration; touches every query call site
    once.
  - Risks: large diff; coordinate with any other in-flight UI work.
- (b) Spread across multiple smaller SOWs (B1 alone first, then B5,
  then B2).
  - Benefits: smaller PRs.
  - Risks: more total work; intermediate states are inconsistent.
- (c) Defer until the catalog grows enough that the cost shows up.
  - Benefits: zero churn.
  - Risks: every new feed-detail page repeats the
    `useQuery + isLoading + error` boilerplate; adds to total
    ledger of "should refactor someday".

Recommendation: **(a)** — single SOW after Decision 5 lands.

## Plan

- Implement the small maintainer-owned findings:
  - A12: replace hardcoded inline stat font sizes with CSS utility classes.
  - A13: standardize `target="_blank"` links on
    `rel="noopener noreferrer"`.
  - A10: remove brittle non-null assertions from touched frontend code.
  - C10: replace legacy `flex-shrink-0` utility names with `shrink-0`.
  - A14: delete unreachable globe UI source and remove its frontend-only
    direct dependencies.
- Verify current npm registry peer ranges before adding any new ESLint plugin.
- Create a concrete follow-up SOW for the larger API/query-boundary work
  instead of burying it in prose.
- Run UI gates, Go gates, install, and runtime smoke.

## Execution log

- Loaded skills: `project-frontend-best-practices`, `project-coding`,
  `project-reviewing`, `project-content-surfaces`, `sow`.
- Inventoried `ui/src/`: 138 files; recorded top-15 largest.
- Read SOW-0033 from `git show d9ede3d -- .agents/sow/pending/SOW-0033*`
  (the original, before completion) and the completed `done/`
  version.
- Confirmed Tailwind v4 hybrid setup; no v3 idioms in CSS;
  `@apply`/`@layer` legitimate in v4. Found 3 `flex-shrink-0`
  legacy utility uses (C10).
- Confirmed React Compiler still NOT enabled.
- Confirmed strict TS still on.
- Smell-pattern scans across SOW-0033's full list:
  forwardRef (35, all shadcn), `: any`/`as any`/`@ts-ignore` (0
  each), non-null `!` (4 unchanged), `as never` (3 unchanged),
  `useEffect.*fetch` (0), raw HTML injection sites (2, both
  sanitized via `sanitizeHtml`), `console.*` (3, all in
  error/disposal handlers), `style={{` (61),
  `setTimeout`/`setInterval`/RAF (4 paired), clickable div/tr
  (0 unguarded — A6 fixed), `Context.Provider` (0),
  propTypes/defaultProps (0), `lazy(` (12 — A1 verified),
  `Suspense` (4), `queryOptions(` (now present after SOW-0050),
  `useSuspenseQuery` (0 — B2 unchanged), `prefetchQuery` (now present after
  SOW-0050), `useTransition` (0), `useDeferredValue` (1 — B6
  partial), `target="_blank"` rel attrs (mixed, A13), `aria-label`
  density (30 sites, no icon-only button found without one).
- Read end-to-end: `App.tsx`, `pages/home.tsx`,
  `pages/feed-detail.tsx`, `pages/admin.tsx`,
  `components/admin/feed-modal.tsx` and the new
  `feed-modal-{hero,identity,manifest,diagnostics,primitives,
  status-sections}.tsx`, `components/admin/feeds-table.tsx`,
  `components/admin/feeds-table-body.tsx`, `feeds-table-model.ts`,
  `feeds-table-filters.tsx`, `components/home/home-explorer.tsx`,
  `components/home/home-explorer-filter-rail.tsx`,
  `components/home/home-explorer-view-cards.tsx`,
  `components/home/home-globe-scene.tsx`,
  `components/home/home-globe-panel.tsx`,
  `components/feed-sidebar.tsx`, `components/theme-provider.tsx`,
  `components/ui/sonner.tsx`,
  `components/route-error-boundary.tsx`,
  `components/feed-detail/section-error-boundary.tsx`,
  `components/feed-detail/section-bogons.tsx`,
  `components/feed-detail/section-geo.tsx`, `lib/api.ts`,
  `lib/safe-html.ts`, `lib/chart-theme.ts`, `lib/query-client.ts`,
  `eslint.config.js`, `tsconfig.app.json`, `vite.config.ts`,
  `vitest.config.ts`, `package.json`, `tailwind.config.ts`,
  `index.css`.
- Cross-checked `pkg/web/static/assets/` listing against expected
  route chunks; verified no `home-globe-*.js` chunk exists, which
  confirms A14 (HomeGlobePanel never lazy-imported on a real route).
- 2026-05-01 implementation:
  - Removed unreachable `HomeGlobePanel`, `HomeGlobeScene`, and
    `home-presets` frontend source; removed the now-unused `getHomeGlobe`
    frontend helper and `HomeGlobe*` frontend types.
  - Removed direct `globe.gl`, `react-globe.gl`, `three`,
    `topojson-client`, `@types/three`, and `@types/topojson-client`
    frontend dependencies with `pnpm remove`.
  - Standardized external-link `rel` attributes.
  - Replaced non-null assertions with explicit guards or local map-entry
    variables.
  - Moved stat font-size variants to `ui/src/index.css`.
  - Created SOW-0050 for the larger query/API-boundary refactor.
- 2026-05-01 reopened cleanup:
  - Replaced the remaining React root non-null assertion with an explicit
    runtime guard in `ui/src/main.tsx`.
  - Replaced the remaining table sort non-null assertion in
    `ui/src/components/home/home-explorer-view-table.tsx` with a narrowed
    local `sortKey`.
  - Added `ui/src/lib/safe-url.ts` and unit coverage for non-web schemes such
    as `artifact://`.
  - Applied safe external URL rendering to public feed-detail, maintainer
    detail, homepage maintainer, and admin feed-modal URL surfaces.
  - Removed the dead local theme context file; active theme state remains owned
    by `next-themes`.

## Validation

- `pnpm view eslint-plugin-jsx-a11y version peerDependencies --json` —
  current version `6.10.2`; peer range excludes ESLint 10.
- `pnpm view eslint-plugin-react version peerDependencies --json` —
  current version `7.37.5`; peer range excludes ESLint 10.
- `pnpm --dir ui install --frozen-lockfile` — passed; lockfile is
  consistent after dependency removal.
- `pnpm --dir ui lint` — passed.
- `pnpm --dir ui build` — passed. Existing font-resolution warnings remain:
  Inter display font URLs are left for runtime resolution.
- `pnpm --dir ui test` — passed: 4 files, 4 tests. Existing node warnings
  about `--localstorage-file` path were emitted by the test environment.
- `make build` — passed.
- `make test` — passed, including `tools/archposture`.
- `make lint` — passed.
- `make race` — passed.
- `make staticcheck` — passed.
- `make golangci-lint` — passed.
- `make vulncheck` — passed, no vulnerabilities found.
- `git diff --check` — passed.
- Same-failure scans:
  - `rg -n "HomeGlobe|home-globe|HOME_GLOBE|HOME_PRESETS|getHomeGlobe|HomeGlobePayload|HomeGlobeCountry" ui/src ui/package.json` returned no matches.
  - `rg -n "rel=\"noreferrer\"|authoritative!|\\.get\\([^)]+\\)!|as never" ui/src` returned no matches.
  - `rg -n "fontSize:\\s*\"" ui/src` now reports only chart label
    sizing in feed-detail chart helpers, not editorial display stats.
  - Direct globe/three dependencies are absent from `ui/package.json`.
- Reopened validation after audit cycles:
  - `pnpm --dir ui test src/lib/safe-url.test.ts` passed: 1 file, 3 tests.
  - `pnpm --dir ui lint` passed.
  - `pnpm --dir ui build` passed. Existing Inter display font runtime
    resolution warnings remain unchanged.
  - `pnpm --dir ui test` passed: 9 files, 21 tests.
  - `make ui-test` passed, including UI behavioral tests and bundle-budget
    unit tests.
  - Same-failure scan for direct public maintainer/source URL hrefs returned
    no matches in the touched public pages/components.
  - Same-failure scan for `theme-context`, `ThemeContext`, root non-null
    lookup, table-sort non-null assertion, `as never`, and map-lookup non-null
    assertion returned no UI source matches.

## Maintainer decision record

- Decision 1 / A14: delete the unreachable `HomeGlobePanel`,
  `HomeGlobeScene`, and `home-presets` source. Rationale: mounting the
  globe would be a homepage product/design change and would add runtime API
  work; deleting unreachable source is the safer maintainer-quality action.
  The frontend-only globe/three/topojson direct dependencies are removed
  with it.
- Decision 2 / A13 + C9: standardize all external links on
  `rel="noopener noreferrer"`. Do not add `eslint-plugin-react` just for
  `react/jsx-no-target-blank` because the current published plugin
  `7.37.5` still declares peers only through ESLint 9.7 while the project
  uses ESLint 10.2.1.
- Decision 3 / A5: do not adopt `eslint-plugin-jsx-a11y` in this SOW.
  `pnpm view eslint-plugin-jsx-a11y version peerDependencies --json`
  returned `6.10.2` with peer range `^3 || ^4 || ^5 || ^6 || ^7 || ^8 ||
  ^9`, so the compatibility gap with ESLint 10 still exists on
  2026-05-01. A peer override would be more fragile than the value it adds
  today.
- Decision 4 / A10: remove non-null assertions in touched source by using
  explicit guards and map-entry variables.
- Decision 5 / A15 + C1 and Decision 6 / B1/B2/B5/B6: valid, but too
  broad for this cleanup pass. Created
  `.agents/sow/done/SOW-0050-20260501-frontend-query-api-boundaries.md`
  as the immediate follow-up SOW for API decomposition, query option
  factories, query prefetch, and related TanStack Query maturation.
- B3 virtualization: rejected for now. Evidence: current catalog scale is
  small enough that no measured jank exists, and virtualization would add
  dependency/complexity to tables that still need query-boundary cleanup
  first.
- B13 `useSyncExternalStore`: rejected for now. The cited overlay events are
  imperative local UI commands, not shared external state with concurrent
  rendering consistency requirements.
- B16 client error telemetry: not a frontend code-quality fix. It would add
  a backend ingest route or external service plus CSP/operator decisions, so
  it is outside this SOW.
- B17 visual-regression/storybook: belongs to the frontend testing review
  SOW-0041, not this frontend code SOW.

## Outcome

Completed after reopened iterative audit findings. Historical completed work
from the first pass:

- Completed the second-round frontend code review and implemented the small
  justified cleanup items.
- Removed unreachable globe UI source and its direct frontend dependencies,
  eliminating the SOW-0033 residual risk where disposal code was validated
  but never reachable from any route.
- Removed all `rel="noreferrer"`-only external links.
- Removed the cited non-null assertion patterns from touched code and also
  cleaned the same map-lookup pattern in `feed-sidebar.tsx`.
- Centralized display stat font-size variants in CSS classes instead of
  inline editorial styles.
- Created
  `.agents/sow/done/SOW-0050-20260501-frontend-query-api-boundaries.md`
  for the larger valid API/query-boundary refactor.
- Closed the reopened residual findings: root lookup non-null assertion,
  table sort non-null assertion, dead local theme context, and unsafe
  server-provided external URL links.

Counts:

- Verification of SOW-0033 (31 findings across A1-A11, B1-B12,
  C1-C8): **11 FIXED**, **2 PARTIAL** (A2, B6), **14 NOT FIXED**
  (most explicitly deferred per SOW-0033 user decisions),
  **0 REGRESSED**, **4 OK/N/A**.
- New Category A (anti-patterns to eliminate): **4 findings**
  (A12-A15).
- New Category B (missing): **5 findings** (B13-B17).
- New Category C (neutral): **4 findings** (C9-C12).

Top-3 most concerning new findings:

1. **A14 — `HomeGlobePanel` was dead code.** Fixed by deleting the unreachable
   globe source and removing direct globe dependencies.
2. **A13 — Inconsistent `rel=` on external links.** Half use
   `noreferrer` only, half use the canonical
   `noopener noreferrer`. Modern browsers imply `noopener`, so not a
   security incident, but the inconsistency proves no enforcement.
3. **A15 — `lib/api.ts` grew to 597 lines.** Fixed by SOW-0050's API/query
   boundary split; current `lib/api.ts` is a compatibility shim.

Closure mapping for remaining valid work:

- API/query-boundary work is mapped to SOW-0050.
- Frontend visual-regression/storybook work is rejected for now with the B17
  evidence above; do not leave it as a prose-only future item.
- Accessibility lint plugin adoption is rejected for this SOW with
  2026-05-01 npm peer-range evidence; do not add an ESLint 10 peer override
  just to satisfy this finding.
- React Compiler remains outside this SOW because SOW-0033 explicitly kept
  compiler adoption as a separate decision. SOW-0080 completed that mapping:
  `.agents/sow/done/SOW-0080-20260501-react-compiler-memoization-decision.md`.
- Iterative audit cycle 5 corrected stale B1/B5 mapping text that predated
  SOW-0050's query option factories and bounded feed prefetch.
- This SOW is now done. Remaining valid broader frontend work is represented
  by concrete pending SOWs, including accessibility cleanup (`SOW-0071`),
  component decomposition
  (`SOW-0073`), dependency hygiene (`SOW-0074`), design tokens (`SOW-0076`),
  and dead API surface cleanup (`SOW-0077`).

## Lessons extracted

- Component-level validation is not product validation when the component is
  unreachable. Reviews must prove the route imports or mounts the component
  before claiming the behavior ships.
- Deleting dead frontend feature code should also remove frontend API helpers,
  types, and direct package dependencies that existed only for that code.
- Before adding an ESLint plugin, verify the current npm peer range against
  the project's actual ESLint major version. Peer-overriding lint plugins is
  not a good default for release hardening.
- Server-provided URLs should pass through a scheme allow-list before becoming
  clickable links; non-web URL values can still be displayed as text.
