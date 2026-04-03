---
name: project-frontend-best-practices
description: "Modern React 19 + TS 6 + Tailwind v4 + Radix/shadcn + TanStack DOs and DON'Ts for high-performance UIs, clean code, separation of concerns, and avoiding AI-generated frontend pitfalls. Use when writing or reviewing UI code."
---

## TL;DR

- This stack is React 19.2, TS 6, Tailwind v4, TanStack Query 5, TanStack Table 8, Radix primitives, react-router-dom 7, Vite 8. Treat React 18, Tailwind v3, and useEffect-fetching as legacy.
- React Compiler is stable (1.0, October 2025). Stop hand-rolling `useMemo` / `useCallback` / `React.memo` for render perf; keep them only as escape hatches for referential-identity contracts and effect dependencies.
- Server data lives in TanStack Query. URL holds operator/explorer state. `useState` is for transient local UI. Never copy server data into `useState`.
- `pnpm --dir ui build` and `pnpm --dir ui lint` are the project gates. Strict TS is on (`noUnusedLocals`, `noUnusedParameters`, `noFallthroughCasesInSwitch`). Treat warnings as errors.
- Generated frontend assets (`pkg/web/static/assets/*`, generated `pkg/web/static/index.html`) are NOT source. Edit `ui/`, run the install flow.

This skill complements `project-coding` (conventions and contracts), `project-reviewing` (review priorities), and `project-content-surfaces` (UI copy audience). Read those before relying on this one alone.

---

## 1. Stack-aware foundations

Verify the stack before applying any pattern from training data. Most "best-practice" snippets on the public internet are React 18 + Tailwind v3 + useEffect-fetching. They are wrong here.

| Concern | What this project uses | What is OUT |
|---|---|---|
| React | 19.2 (`use`, Actions, ref-as-prop, `useActionState`, `useOptimistic`, `useFormStatus`, document metadata as JSX) | `forwardRef`, `propTypes`, `defaultProps` on function components, string refs, `<Context.Provider>` |
| Compiler | React Compiler 1.0 (stable since Oct 2025) handles render-time memoization automatically when enabled | Hand-rolling `useMemo` / `useCallback` / `React.memo` for ordinary render perf |
| TypeScript | 6.0.3, strict, `noUnusedLocals`, `noUnusedParameters`, `noFallthroughCasesInSwitch` | `any`, `as any`, `@ts-ignore`, non-null `!` assertions, runtime `propTypes` |
| Bundler | Vite 8 + `@vitejs/plugin-react` | Webpack idioms, CRA, `process.env.*` outside the Vite env shape |
| CSS | Tailwind v4 via `@import "tailwindcss"` and `@theme`. Project keeps a `tailwind.config.ts` mounted via `@config` for legacy theme tokens — see Section 7 | `@tailwind base/components/utilities`, JS-only theme config without `@config`, `bg-opacity-*`, `flex-shrink-*`, `flex-grow-*` |
| Components | Radix primitives + shadcn-style copies under `ui/src/components/ui/`, composed via `Slot` and `cva` | Headless rolled-from-scratch dropdowns/dialogs/tooltips that re-implement a11y |
| Data | TanStack Query 5 with explicit keys and invalidations through `ui/src/lib/api.ts` | `useEffect(() => fetch(...), [])`, manual loading state, ad-hoc fetch caches |
| Tables | TanStack Table 8 (headless, column defs colocated) | `<table>` with hand-rolled sort/filter/paging logic |
| Router | `react-router-dom` 7 in library mode (no `routes.ts` typegen) | Framework-mode loaders/actions; route-module typegen files |
| Theming | `next-themes` + `.dark` class, design tokens in `ui/src/index.css` and `ui/tailwind.config.ts` | Inline hex colors, ad-hoc `style={{ ... }}` for theme values |
| Toasts | `sonner` | `react-hot-toast`, hand-rolled portals |
| Sanitization | `dompurify` for any HTML pulled from feeds/methodology, via `lib/safe-html.ts` | Raw HTML injection from untrusted strings |

Rule: when an idiom from a search result conflicts with this table, the table wins.

React Compiler is installed in opt-in annotation mode in `ui/vite.config.ts`
through `@vitejs/plugin-react`'s `reactCompilerPreset()` and
`@rolldown/plugin-babel`. Existing manual memoization may remain until a
focused test/profile proves it is safe to remove; new `useMemo`,
`useCallback`, or `memo()` usage needs an identity, effect-dependency, or
measured-expensive-work reason. New `"use memo"` annotations require a route
profile and passing `pnpm --dir ui build:budget` evidence (from SOW-0080).

Package-lint rule: before adding an ESLint plugin, verify its current npm peer
range against this project's ESLint major. Do not peer-override lint plugins by
default during release hardening; record the incompatibility instead (from
SOW-0040).

Sources: [React 19 release notes](https://react.dev/blog/2024/12/05/react-19), [React Compiler 1.0](https://react.dev/blog/2025/10/07/react-compiler-1), [Tailwind v4 upgrade guide](https://tailwindcss.com/docs/upgrade-guide), [React Router v7 type-safety](https://reactrouter.com/explanation/type-safety).

---

## 2. Component design and separation of concerns

Three layers, kept distinct:

1. **UI components** — JSX, classes, accessibility wiring. Almost no logic.
2. **Data hooks** — TanStack Query hooks, mutations, derived selectors. One concern per hook.
3. **Domain helpers** — pure functions in `lib/` (formatting, classification, parsing). Unit-testable without React.

The repo already follows this: `lib/api.ts` is the typed client, `lib/api-types.ts` holds the shared types, formatters live in `lib/admin-format.ts` / `lib/feed-health.ts` / etc., and components import them. Mirror this when adding features.

### When to extract

- Component file longer than ~250 lines, or with non-trivial branching → split into subcomponents in a sibling file or a folder.
- Files that export React components must not also export shared constants,
  helpers, or non-component functions; `react-refresh/only-export-components`
  is enforced. Move shared values to a plain `.ts` module and import them into
  component files (from SOW-0073).
- A `useState` + a few derived values + a TanStack Query call repeated in two places → extract to a custom hook (`use<Thing>`) in `lib/` or alongside the component.
- Render branches that look like `if (loading) … if (error) … if (empty) …` repeated across pages → extract a `<QueryStates>` boundary or use Suspense + ErrorBoundary.
- Any component reaching for >4 props that all flow to a single child → consider composition (`<Foo>{children}</Foo>` with `Slot`) instead of prop pass-through.
- A visual or heavy component is not "shipped" until a real route imports or
  mounts it. When deleting unreachable frontend feature code, remove its
  frontend API helpers, types, and direct package dependencies too (from
  SOW-0040).
- Route-splitting and dependency changes must keep the frontend bundle budget
  green. Run `make ui-budget` after changing route imports, visualization
  dependencies, chart/map libraries, or shared query/client modules (from
  SOW-0056).

### File-size heuristic (project policy)

`tools/archposture` enforces architecture posture for backend. The frontend has no automatic gate, but the same spirit applies: a single TSX file over ~400 lines is a smell. Recent SOWs split feed modal and feeds table into many small files (`feed-modal-hero.tsx`, `feed-modal-identity.tsx`, `feed-modal-status-sections.tsx`, `feeds-table-body.tsx`, `feeds-table-filters.tsx`, `feeds-table-model.ts`). Mirror that split.

### BAD vs GOOD

```tsx
// BAD: god component, JSX mixed with fetching, mixed with formatting, mixed with logic
export default function FeedPage({ name }: { name: string }) {
  const [feed, setFeed] = useState<any>(null);
  const [error, setError] = useState<any>(null);
  useEffect(() => {
    fetch(`/api/v1/sets/${name}`).then(r => r.json()).then(setFeed).catch(setError);
  }, [name]);
  const fmt = (n: number) => {
    if (n > 1_000_000) return (n/1_000_000).toFixed(1) + "M";
    return String(n);
  };
  if (error) return <div className="text-red-500">error</div>;
  if (!feed) return <div>loading</div>;
  return <div className="rounded-lg shadow p-4 bg-white dark:bg-slate-900">
    <h1 className="text-xl font-bold">{feed.name}</h1>
    <p>{fmt(feed.ip_count)} IPs</p>
    {/* 600 more lines */}
  </div>;
}
```

```tsx
// GOOD: typed query hook, formatter helper, design-token classes, small component
import { useFeedDetail } from "@/lib/feed-detail";
import { formatIPs } from "@/lib/utils";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { QueryStates } from "@/components/query-states";

export function FeedPage({ name }: { name: string }) {
  const query = useFeedDetail(name);
  return (
    <QueryStates query={query}>
      {(feed) => (
        <Card>
          <CardHeader><CardTitle>{feed.name}</CardTitle></CardHeader>
          <CardContent>{formatIPs(feed.ipCount)} IPs</CardContent>
        </Card>
      )}
    </QueryStates>
  );
}
```

---

## 3. State management hierarchy

Pick the lowest applicable level:

1. **URL state** for anything that affects operator workflow, sharability, or back/forward — the project does this in `lib/admin-url-state.ts` and `lib/explorer-state.ts`. Use `useSearchParams` or the typed wrappers, not `useState`.
2. **Server state** (anything from the daemon) → TanStack Query. Never duplicate it into `useState` to "transform it" — use `select` instead.
3. **Local component state** → `useState` for transient UI (open/closed, hover, in-flight form input). If two siblings need it, lift it; if many distant siblings need it, prefer URL or a small context.
4. **Cross-cutting client state** (theme, toast queue) → existing providers (`next-themes`, `sonner`). Do not introduce Redux/Zustand without a SOW decision; this codebase deliberately has none.

### BAD vs GOOD: derived state

```tsx
// BAD: useEffect to "compute" derived state (extra render, stale closure risk)
const [filtered, setFiltered] = useState<Feed[]>([]);
useEffect(() => {
  setFiltered(feeds.filter(f => f.category === category));
}, [feeds, category]);
```

```tsx
// GOOD: compute in render (React Compiler memoizes when needed)
const filtered = feeds.filter(f => f.category === category);
```

If the computation is genuinely expensive (>1ms in a profile), use `useMemo`. If you used `useEffect` because you wanted to re-fetch when input changes, that is a query, not state — see Section 4.

Sources: [You Might Not Need an Effect](https://react.dev/learn/you-might-not-need-an-effect).

---

## 4. Data fetching with TanStack Query 5

### Rules

- Every query goes through a typed hook in `lib/` (existing pattern in `ui/src/lib/api.ts`). Components import the hook, never `fetch` directly.
- Query keys are arrays whose first element is a stable feature root, followed by all variables that affect the response. Type matters (`["feed", 1]` ≠ `["feed", "1"]`).
- Prefer `queryOptions()` factories so the same key/fn pair is reusable for prefetch, `useQuery`, `setQueryData`, and `invalidateQueries`.
- Split query option factories by route/concern. A single central
  query-options module that imports every API helper can be hoisted by Rollup
  into the public shell when one shared layout imports one factory; keep narrow
  modules such as catalog, feed-core, feed-detail sections, admin, entities,
  methodology, and search separate (from SOW-0050).
- Use `select` to slice/transform; do not `useState` + `useEffect` to mirror server data.
- Mutations call `queryClient.invalidateQueries({ queryKey: [...] })` on success, with the most specific key that still covers everything that could have changed. Optimistic updates use `onMutate` + rollback in `onError`.
- `staleTime` is "how long data is considered fresh" (no refetch). `gcTime` (default 5 min) is "how long inactive cached data lives in memory". Tune `staleTime` per query family; default `gcTime` is usually fine.
- Never trigger writes from a `useEffect` that watches a query result. Use `useMutation` triggered by the user action.

### queryOptions factory

```ts
// ui/src/lib/feeds.ts
import { queryOptions, useQuery } from "@tanstack/react-query";
import { fetchFeed, type FeedDetail } from "@/lib/api";

export const feedKeys = {
  all: ["feeds"] as const,
  detail: (name: string) => [...feedKeys.all, "detail", name] as const,
};

export function feedDetailOptions(name: string) {
  return queryOptions({
    queryKey: feedKeys.detail(name),
    queryFn: ({ signal }) => fetchFeed(name, signal),
    staleTime: 30_000,
  });
}

export function useFeedDetail(name: string) {
  return useQuery(feedDetailOptions(name));
}
```

Why this shape:

- `feedKeys.detail(name)` is reusable for `invalidateQueries`, `setQueryData`, prefetch.
- `feedDetailOptions(name)` carries `queryKey + queryFn` types together — `setQueryData` becomes type-safe.
- The `signal` is forwarded to `fetch`/`AbortController` so cancellation works on unmount and key changes.

### BAD vs GOOD

```tsx
// BAD: fetch in useEffect, manual loading state, no cancellation, untyped data
function Feed({ name }: { name: string }) {
  const [data, setData] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    setLoading(true);
    fetch(`/api/v1/sets/${name}`).then(r => r.json()).then(d => { setData(d); setLoading(false); });
  }, [name]);
  // race conditions on rapid name change, no error handling, no cache reuse
}
```

```tsx
// GOOD: typed hook, cancellation, cache reuse, error/loading from the query
function Feed({ name }: { name: string }) {
  const { data, isLoading, error } = useFeedDetail(name);
  if (isLoading) return <Skeleton className="h-32" />;
  if (error) return <ErrorState error={error} />;
  if (!data) return null;
  return <FeedCard feed={data} />;
}
```

### Mutations and invalidation

```ts
const qc = useQueryClient();
const mutation = useMutation({
  mutationFn: (req: ToggleFeedRequest) => api.toggleFeed(req),
  onSuccess: (_, vars) => {
    qc.invalidateQueries({ queryKey: feedKeys.detail(vars.name) });
    qc.invalidateQueries({ queryKey: feedKeys.all });
  },
});
```

Anti-pattern: calling `setQueryData` to "fix" UI without invalidating, then drift between cache and server.

Sources: [Query Options](https://tanstack.com/query/v5/docs/react/guides/query-options), [Query Keys](https://tanstack.com/query/v5/docs/react/guides/query-keys).

---

## 5. Tables with TanStack Table 8

- Define column defs **outside** the component or memoize them with `useMemo`. A new array on every render destroys table state and trashes performance.
- Type the row shape; let TanStack infer accessors (`accessorKey: "name"` instead of `accessorFn: (r) => r.name as any`).
- For tables larger than ~1k rows, switch to server-side: `manualPagination: true`, `manualSorting: true`, `manualFiltering: true`, then move the state into URL params and refetch via TanStack Query.
- For very large tables that must stay client-side (rare), use TanStack Virtual on top of TanStack Table — TanStack Table itself does not virtualize.
- Co-locate the column definitions with their domain types. The repo already does this in `feeds-table-model.ts`.

### BAD vs GOOD

```tsx
// BAD: column defs recreated each render, accessorFn returns any
function FeedsTable({ rows }: { rows: any[] }) {
  const columns = [
    { accessorFn: (r: any) => r.name, header: "Name" },
    { accessorFn: (r: any) => r.ips, header: "IPs" },
  ];
  const table = useReactTable({ data: rows, columns, getCoreRowModel: getCoreRowModel() });
  // …
}
```

```tsx
// GOOD: typed columns at module scope, memoized data, a model file for column defs
import type { ColumnDef } from "@tanstack/react-table";
import type { FeedRow } from "./feeds-table-model";

const columns: ColumnDef<FeedRow>[] = [
  { accessorKey: "name", header: "Name" },
  { accessorKey: "ips", header: "IPs", cell: (c) => formatIPs(c.getValue<number>()) },
];

export function FeedsTable({ rows }: { rows: FeedRow[] }) {
  const table = useReactTable({ data: rows, columns, getCoreRowModel: getCoreRowModel() });
  // …
}
```

Sources: [TanStack Table v8 pagination + virtualization](https://tanstack.com/table/v8/docs/guide/pagination).

---

## 6. Routing with react-router-dom 7 (library mode)

This project uses **library mode** — `BrowserRouter` + `<Routes>` + `<Route>` JSX, no `routes.ts` typegen, no Framework-mode loaders/actions. Do not introduce framework-mode features.

DOs:

- Lazy-load expensive routes with `React.lazy(() => import("./pages/admin"))` and wrap in `<Suspense>`. Bundle splitting is the lever that keeps the public site fast.
- Keep search-param state typed via the `lib/admin-url-state.ts` / `lib/explorer-state.ts` helpers; do not parse `URLSearchParams` ad-hoc in components.
- Use `useParams<{ name: string }>()` — narrow it once at the route boundary, then pass typed values down.
- Use `<ScrollRestoration />` if you ever change scroll behavior; verify with the home/admin layout.

DON'Ts:

- Do not migrate to framework mode silently. It changes the build, the `index.html` shape, and how SSR/typegen works. That is a SOW-level decision.
- Do not add a second router (`createBrowserRouter` data-router) alongside the JSX router. Pick one.
- Do not derive routing decisions from `window.location` directly inside components — use the router hooks; otherwise SSR/test mounts misbehave.

Sources: [React Router v7 modes](https://reactrouter.com/explanation/type-safety).

---

## 7. Styling with Tailwind v4 + Radix + shadcn-style

### Tailwind v4 reality in this repo

Tailwind v4 changes are real and AI tools regularly produce v3 syntax. The hard rules:

- `@import "tailwindcss"` (single line) replaces the v3 trio of `@tailwind base; @tailwind components; @tailwind utilities;`.
- This project mounts a legacy `tailwind.config.ts` via `@config "../tailwind.config.ts"` for design tokens. That is a deliberate hybrid — do **not** delete the config file; do not move all theme tokens into CSS without a SOW.
- Custom utilities use `@utility` (v4), not `@layer utilities` (v3).
- Theme values are CSS variables and the `theme()` function still works inside `@apply` if needed; prefer `var(--color-foo)` in new CSS.
- Renamed utilities: `shadow-sm`→`shadow-xs`, `rounded-sm`→`rounded-xs`, `outline-none`→`outline-hidden`, `flex-shrink-0`→`shrink-0`, `flex-grow-0`→`grow-0`. Do not use `bg-opacity-*` / `text-opacity-*` / `border-opacity-*` — use the `/N` opacity modifier (`bg-black/50`).
- Important modifier: trailing, not leading: `flex!` not `!flex`.
- Default `border-color` is `currentColor` in v4; the design tokens in `index.css` set explicit border colors where needed. If a border looks wrong, the fix is a token, not `border-gray-200`.

If a generated snippet has any of `tailwind.config.js` (top-level reconfig), `@tailwind base/components/utilities`, `@layer utilities { … }`, or `bg-opacity-*` — that snippet is v3 and must be rewritten before commit.

### shadcn-style composition

This repo follows shadcn conventions exactly:

- Primitives copied into `ui/src/components/ui/` (`button.tsx`, `dialog.tsx`, `select.tsx`, …). They are not a npm package — they are owned source code. Edit them in place when behavior changes.
- `cn()` lives at `@/lib/utils` and is the single class-merging helper. Use it everywhere; it merges via `clsx` + `tailwind-merge` so conflicts (`p-4` vs `p-2`) resolve last-wins.
- Variant systems use `class-variance-authority` (`cva`) — see `button.tsx` for the canonical shape (`variants`, `defaultVariants`, exported `VariantProps`).
- `asChild` + `Slot` is the way to forward a primitive's behavior onto a different element (e.g., `<Button asChild><Link to="/x">Go</Link></Button>`). The child must be a single focusable element that spreads incoming props and forwards refs. In React 19, "forward refs" means accept `ref` as a normal prop — no `forwardRef`.

### BAD vs GOOD: variants

```tsx
// BAD: ad-hoc class concatenation, dark-mode by hand, no variants
<button className={
  (variant === "primary" ? "bg-blue-600 text-white " : "bg-gray-200 ") +
  (size === "sm" ? "px-2 py-1 " : "px-4 py-2")
}>…</button>
```

```tsx
// GOOD: use the existing Button (cva, design tokens, a11y from Radix Slot)
import { Button } from "@/components/ui/button";
<Button variant="default" size="sm">…</Button>
```

### BAD vs GOOD: theme tokens

```tsx
// BAD: hardcoded hex, will not switch with theme
<div className="bg-[#0f131b] text-[#fafaf7]">…</div>
// or
<div style={{ background: "#0f131b" }}>…</div>
```

```tsx
// GOOD: design tokens that adapt to light/dark
<div className="bg-background text-foreground">…</div>
```

### Dark mode

The project uses `next-themes` with a `.dark` class on `<html>`. Do not introduce a `data-theme` selector or roll a separate theme system. Any new color must exist in `ui/src/index.css` for both `:root` and `.dark` (or live in the Tailwind config).

Sources: [Tailwind v4 upgrade guide](https://tailwindcss.com/docs/upgrade-guide), [shadcn/ui core concepts](https://vercel.com/academy/shadcn-ui/core-concepts), [asChild + Slot](https://peerlist.io/jagss/articles/understanding-aschild-and-slot-in-react-clean-flexible-compo).

---

## 8. Visualization layers

This repo ships several visualization stacks. Each has a niche; do not mix them inside one component.

| Library | Use for | Avoid for |
|---|---|---|
| Recharts | Standard time-series, bars, areas, pies on operator/admin pages. Works with React state, lightweight, accessible defaults | Highly custom layouts, sankey, force, geo |
| VisX | Custom layouts when Recharts cannot express the design (hierarchies, tile maps, custom scales) | Trivial bars/lines (over-engineering) |
| D3 modules (`d3-force`, `d3-sankey`, `d3-geo`, `d3-scale`) | Layout/scale primitives invoked from React; never let D3 own the DOM | Direct DOM mutation (`d3.select(svgRef).…`) — keep React in charge |
| react-simple-maps + topojson-client | 2D world maps with country choropleth | 3D, panning at high zoom |

### Three.js / globe lifecycle if reintroduced

The current frontend does not carry direct Three.js/globe dependencies. If a future SOW reintroduces a Three.js scene (directly or through `react-globe.gl`), enforce these before closing that SOW:

```tsx
useEffect(() => {
  // mount scene/objects here
  return () => {
    // dispose every Geometry, Material (or array), Texture, RenderTarget
    scene.traverse((obj) => {
      if (obj instanceof Mesh) {
        obj.geometry.dispose();
        const mats = Array.isArray(obj.material) ? obj.material : [obj.material];
        mats.forEach((m) => {
          for (const k of Object.keys(m)) {
            const v = (m as any)[k];
            if (v && typeof v.dispose === "function" && v.isTexture) v.dispose();
          }
          m.dispose();
        });
      }
    });
    renderer?.dispose();
    renderer?.forceContextLoss();
  };
}, [/* keys that recreate the scene */]);
```

Common symptoms of missed cleanup: GPU memory creeps up across navigation, hot-reload eventually crashes the tab, `renderer.info.memory` rises monotonically.

Other rules:

- Lazy-load any 3D/globe feature: `const Globe = lazy(() => import("./globe"))`. Three.js + globe.gl is hundreds of KB; do not ship it in the main chunk for users who never see the feature.
- Do not put scene state in React state — use refs. Re-rendering must not recreate the WebGL context.
- Cancel `requestAnimationFrame` in cleanup; otherwise the loop keeps a closure to old props.

### SVG vs Canvas

Recharts/D3-as-React produce SVG. SVG is fine up to a few thousand DOM nodes; past that, switch to Canvas (e.g., a custom Canvas chart, or D3 + canvas). Force-directed graphs above ~500 nodes should be Canvas/WebGL.

Sources: [Three.js memory leaks in React](https://discourse.threejs.org/t/r3f-threejs-memory-leak-when-canvas-is-scrolled-out-of-view/48440), [dispose patterns](https://discourse.threejs.org/t/when-to-dispose-how-to-completely-clean-up-a-three-js-scene/1549).

---

## 9. Performance

Public site requirements (from `.agents/sow/specs/operating-principles.md`): cache-first, fast TTFB, no expensive request-time computation. The frontend's job is to load only what the visible surface needs.

### Bundle splitting

- Route-level code splitting by default: every top-level page (`pages/admin.tsx`, `pages/feed.tsx`, `pages/methodology.tsx`, etc.) is `lazy()`-imported and wrapped in `<Suspense>` at the router level.
- Heavy libs (large D3 modules, large Recharts compositions, and any future Three.js/globe libraries) are dynamic-imported inside their feature, not statically imported in the route module.
- Lazy route pages are not enough when their layouts fetch data. Lazy-load
  route-specific layouts too, and inspect the emitted chunk list or endpoint
  strings after data-boundary refactors. SOW-0050 found an eager `AdminLayout`
  import leaking admin query helpers into the main entry chunk.
- Vite manual chunks: only intervene when the bundle visualizer shows a hot path. Do not configure `manualChunks` from training-data templates blindly — broken chunking can recreate full vendor blobs on every route.
- Avoid barrel files (`index.ts` re-exporting many submodules). They defeat tree-shaking. Import directly from the file.

### Render performance

With React Compiler enabled the right reflex is "let the compiler do it":

- Do not wrap every callback in `useCallback` and every value in `useMemo`. The compiler memoizes render-time work granularly, including conditionally after early returns.
- Keep `useMemo` / `useCallback` only when:
  - The value/function crosses a referential-identity contract (third-party DnD, charts, map libs, debounce/throttle factories).
  - The value is a `useEffect` dependency and should not over-fire.
  - You measured a real perf cost (>1ms in profile) that the compiler did not eliminate.
- Use `useTransition` to mark non-urgent UI updates (filtering, large list re-render) so input stays responsive.
- Use `useDeferredValue` for derived list inputs (search filter against a long list) to skip stale renders.
- Lists: provide stable keys; never use array index as key when the list reorders or items are added/removed.
- Virtualize lists/tables above ~500–1000 rows (TanStack Virtual + TanStack Table).

### Loading and layout stability

- Reserve space for async content with `<Skeleton>` (already in `components/ui/skeleton.tsx`). No layout shift (CLS) when data arrives.
- Use `<Suspense fallback={…}>` boundaries at meaningful points (route, panel, expensive widget) — not the whole tree.
- Use `<ErrorBoundary>` around any third-party visualization. A chart or future Three.js crash should not kill the page.

### TTFB / public site

- Public pages must render with cached/published artifacts only. Do not introduce a UI flow that calls a route which would build artifacts on demand. Cross-check with `project-coding` and `.agents/sow/specs/operating-principles.md` before adding a new public surface.
- Defer non-critical JS: fonts via `<link rel="preload">` only when measured; otherwise let the browser's default policy run.

Sources: [React Compiler 1.0](https://react.dev/blog/2025/10/07/react-compiler-1), [Vite code-splitting](https://github.com/vitejs/vite/discussions/9440).

---

## 10. TypeScript discipline

Strict is on. Treat the compiler as a teammate, not an obstacle.

### Hard rules

- No `any`. Replace with the precise type, `unknown` + narrowing, or a discriminated union. If a third-party type is `any`, wrap it in a typed adapter at the boundary.
- No `as Foo` to silence a real type error. Casting hides bugs the compiler already caught. The legitimate uses are: narrowing after a runtime check (`as const`, `as typeof X[number]`), branded type construction inside a factory, and discriminated-union selection where the compiler cannot infer.
- No non-null `!` assertions on values that could be undefined at runtime. Narrow with `if (!x) return null;` or use a type guard.
- No `@ts-ignore` / `@ts-expect-error` without a comment explaining why and what would unblock removing it.
- Discriminated unions over flag bags: `type Status = { kind: "ok"; data: T } | { kind: "err"; error: Error }` beats `{ ok: boolean; data?: T; error?: Error }`.
- Exhaustiveness in switches: end the switch with `default: const _exhaustive: never = value; throw new Error(...)`. With `noFallthroughCasesInSwitch` already on, a missing `case` becomes a compile-time error.

### `satisfies`

`satisfies` lets a value conform to a wider type without losing its narrow inferred type. Useful for keyed configs and column defs.

```ts
// BAD: loses narrow keys
const routes: Record<string, RouteConfig> = {
  home: { path: "/" },
  admin: { path: "/admin", role: "admin" },
};
routes.hom; // typo — but Record<string,…> permits any key

// GOOD: satisfies preserves the literal keys
const routes = {
  home: { path: "/" },
  admin: { path: "/admin", role: "admin" },
} satisfies Record<string, RouteConfig>;
routes.hom; // compile error
```

### Branded types (when invariants matter)

Use a phantom-typed nominal wrapper for values that look like primitives but carry invariants (a validated CIDR, a feed name that exists, a sanitized HTML string).

```ts
type CIDR = string & { readonly __brand: "CIDR" };
function parseCIDR(s: string): CIDR | null { /* validate */ }
function summarize(c: CIDR) { /* … */ }
summarize("10.0.0.0/8"); // compile error — must go through parseCIDR first
```

### `unknown` over `any` at boundaries

Untyped data (third-party JSON, `localStorage`, postMessage) is `unknown`. Validate then narrow:

```ts
// BAD
const cfg: any = JSON.parse(raw);
return cfg.foo.bar;

// GOOD
const cfg: unknown = JSON.parse(raw);
if (typeof cfg === "object" && cfg && "foo" in cfg) {
  const foo = (cfg as { foo: unknown }).foo;
  // narrow further or use a schema validator
}
```

For repeated parsing, prefer a schema validator (Zod/Valibot — neither is in `package.json` today, so introducing one is a SOW-level decision).

Sources: [unknown vs never](https://betterstack.com/community/guides/scaling-nodejs/never-unknown-types/), [satisfies for exhaustiveness](https://return2.net/typescript-use-satisfies-for-exhaustive-type-checks/).

---

## 11. Accessibility

Radix gives a lot for free. Don't break it.

### What Radix delivers

- ARIA roles and `aria-*` attributes on dialog, popover, dropdown, select, tooltip, tabs, scroll-area, separator.
- Focus management (focus trap on dialog, focus return on close, roving tab index in menus).
- Keyboard navigation (arrows, escape, home/end where applicable).
- Portal and z-index handling that avoids stacking-context bugs.

### What we still own

- Semantic HTML inside the primitives. A clickable `<div>` is an accessibility bug. Use `<button>`, `<a>`, or a Radix primitive.
- Labels on every form control: visible `<Label>` or `aria-label` / `aria-labelledby`. Placeholder text is not a label.
- Color contrast of 4.5:1 for body text, 3:1 for large/UI text. Verify both light and dark themes whenever a token changes.
- Visible focus rings. Tailwind `focus-visible:` classes are already in the design system; do not blanket-remove `outline` without replacing it.
- `aria-live="polite"` on `sonner` toasts is set correctly by the library — do not duplicate it.
- Reduced motion: gate non-essential motion behind `@media (prefers-reduced-motion: reduce)` (Tailwind `motion-safe:` / `motion-reduce:` variants).
- Don't render only an icon button without an accessible name (`aria-label` or `<span className="sr-only">`).

### BAD vs GOOD

```tsx
// BAD: clickable div, no role/keyboard, no label
<div onClick={open} className="rounded-md p-2 hover:bg-accent">
  <PencilIcon />
</div>
```

```tsx
// GOOD: real button, label, focus ring inherited from Button
<Button variant="ghost" size="icon" aria-label="Edit feed" onClick={open}>
  <PencilIcon />
</Button>
```

---

## 12. Security

This UI talks to the daemon's typed API, but it also renders content from third-party feed pages, methodology markdown, and admin operator inputs. Treat everything that touches HTML as hostile until proven otherwise.

- Use `dompurify` (already a dep) for any HTML you must inject as raw markup. The repo has a helper at `lib/safe-html.ts` — use it. Never inline `DOMPurify.sanitize` calls scattered across components without going through the helper.
- Prefer plain text + JSX over HTML strings. If a description has bold/links and is short, render Markdown server-side or use a tightly-scoped renderer; do not free-form HTML.
- URLs from server data: validate scheme before rendering as `<a href>`. Reject `javascript:`, `data:`, and unknown schemes. Use `new URL(value, base)` and check `url.protocol`.
- Open external links with `rel="noopener noreferrer"` and consider `target="_blank"` only when justified.
- No inline event handlers built from data (e.g. `onclick="…"` strings inside HTML you inject). React JSX does not execute string handlers, but unsanitized raw HTML can still smuggle them — always go through `lib/safe-html.ts`.
- No JSON-in-`<script>` for runtime config without escaping; use a typed bootstrap endpoint.
- Do not log tokens, API keys, or admin auth in `console.*`. Browser dev tools = log scraping by anyone with the page open.
- CSP: the daemon ships a CSP. New external scripts/styles/fonts/images are a CSP change — it's a SOW decision, not a one-line PR.

---

## 13. Working with AI-generated UI code

This is where most reviews fail. LLMs are trained predominantly on React 18 + Tailwind v3 + useEffect-fetching tutorials. Anything they emit that matches that profile is wrong here. Read this section every time you accept a generated component.

### Concrete failure modes (community evidence)

1. **`forwardRef` everywhere.** Generated components wrap with `React.forwardRef((props, ref) => …)`. In React 19, accept `ref` as a regular prop. Strip the wrapper.
2. **`tailwind.config.js` reconfigurations / `@tailwind base; @tailwind components; @tailwind utilities;`** — v3 syntax. Replace with `@import "tailwindcss"` at most; this repo additionally mounts a legacy config via `@config`. Do not paste a fresh `tailwind.config.js` — touch `ui/tailwind.config.ts` only with a SOW. ([Tailwind v4 / Claude 3.7 Sonnet write-up](https://medium.com/@dpzhcmy/tailwind-css-v4-the-archenemy-of-claude-3-7-sonnet-209ce7470f76); [official discussion](https://github.com/tailwindlabs/tailwindcss/discussions/19594))
3. **`useEffect` for fetching.** "Fat component" anti-pattern. Replace with `useQuery` through a typed hook in `lib/`. Race conditions, no cancellation, no cache reuse. ([Vercel React best practices](https://vercel.com/blog/introducing-react-best-practices))
4. **`useState` mirroring server data ("for transformations").** Use `select` in `useQuery`.
5. **`useEffect` for derived state.** Compute in render. ([You Might Not Need an Effect](https://react.dev/learn/you-might-not-need-an-effect))
6. **Missing dependency arrays / stale closures.** Either fix the deps or rewrite to not need an effect.
7. **`any` and `as any` to silence TS errors.** Replace with the precise type. The lint will fail anyway.
8. **`!` non-null assertions on optional values.** Narrow with a guard.
9. **Cargo-culted `useMemo` / `useCallback` on every primitive value/lambda.** Remove. The Compiler memoizes render work; manual memo here just bloats the file.
10. **"Hallucinated" Radix or shadcn API surface.** Examples: `<Dialog.Trigger asChild>` written as `<DialogTrigger asChild={true}>` from a different lib version, `cva(...).default("primary")` (no such API), `cn(... ?? "")` confusion. Cross-check against the actual installed primitives in `ui/src/components/ui/`.
11. **Inline styles for theme values.** `style={{ background: "#0f131b" }}` defeats the design system. Use tokens from `index.css` / `tailwind.config.ts`.
12. **Hex colors instead of design tokens.** Same problem; also breaks dark mode.
13. **Massive single-file components (god component).** Split — see Section 2.
14. **Missing accessibility on icon buttons.** No `aria-label`, no semantic element. Section 11.
15. **Layout shift / no skeleton placeholders.** Reserve space (`<Skeleton>`).
16. **Future Three.js scenes without disposal.** Section 8.
17. **Heavy chart/globe libraries imported statically into the main bundle.** Use dynamic import / `lazy()`.
18. **Console statements left in render paths.** Block at review.
19. **Raw HTML injection without sanitization.** Section 12 — go through `lib/safe-html.ts`.
20. **`fetch("/api/...")` directly from a component.** Centralize through `lib/api.ts`.
21. **Custom dropdown/dialog/tooltip implementations.** Use the existing Radix/shadcn primitive. AI will happily reinvent them with broken focus management.
22. **`<Context.Provider>` instead of `<Context>`.** Deprecated — use the value directly as a provider in React 19.
23. **`propTypes` / `defaultProps`.** Removed in React 19. Use TS types and default parameter values.
24. **`useReducer` for trivial state.** Smell that the LLM was told "complex state needs a reducer". Use `useState` until justified.
25. **A new state-management library (Redux/Zustand/Jotai) introduced "to avoid prop drilling".** Use Context or composition. Adding a new store is a SOW decision. ([Builder.io: React compiler does not solve prop drilling](https://www.builder.io/blog/react-compiler-will-not-solve-prop-drilling))
26. **react-router-dom confused with framework mode** (`loader: ...` / `action: ...` route objects with typegen files). We use library mode. Strip framework imports.
27. **Hand-coded global types in `globals.d.ts`** to "fix" missing third-party types when the library exposes its own types correctly.
28. **Missing Suspense / ErrorBoundary** around lazy/heavy widgets — a tiny error in a Recharts chart kills the whole page.

### Reviewer checklist for AI-generated UI

Before accepting any AI-generated component or skill output:

- [ ] No `forwardRef`. Refs are props.
- [ ] No `useEffect` for fetching, no manual loading state. Goes through `lib/api.ts` + TanStack Query.
- [ ] No `useState` mirroring server data. `select` in queries instead.
- [ ] No `useEffect` for derived state. Computed in render.
- [ ] No `any`, `as any`, `@ts-ignore`, `!` assertion. Strict-TS-clean.
- [ ] No `useMemo` / `useCallback` / `React.memo` without a clear identity-contract or measured-perf justification.
- [ ] No `tailwind.config.js` add, no `@tailwind base/components/utilities`, no `@layer utilities` for new utilities. v4 syntax.
- [ ] No `bg-opacity-*` / `flex-shrink-*` / `flex-grow-*` / pre-rename utilities.
- [ ] No hex colors / inline styles for theme values. Design tokens only.
- [ ] No clickable `<div>`. Real semantic elements.
- [ ] Every icon-only button has `aria-label`.
- [ ] `cn()` from `@/lib/utils` for class merging. No string concatenation of conditionals.
- [ ] Variants via `cva` for any reusable component family.
- [ ] Lazy-load any heavy widget (charts, map, and any future globe). No global static imports of heavy visualization libraries.
- [ ] If Three.js / globe code is reintroduced, it has a working `useEffect` cleanup that disposes geometries, materials, textures, renderer.
- [ ] Component file is under ~250 lines or split into siblings. Single responsibility.
- [ ] Any raw HTML pulled from data goes through `lib/safe-html.ts` (DOMPurify).
- [ ] Public site: no new public surface that would trigger upstream/expensive work. Cross-check `project-coding` and `operating-principles.md`.
- [ ] `pnpm --dir ui lint` and `pnpm --dir ui build` pass.

If a generated patch fails any of these, send it back. Do not "fix while landing" — the same model will reproduce the pattern next time unless the prompt/skills are updated.

---

## 14. Quick reference — DO / DON'T

| Concern | DO | DON'T |
|---|---|---|
| React refs | Accept `ref` as prop | `forwardRef`, `React.forwardRef` |
| Memoization | Trust the React Compiler; manual memo as escape hatch | Wrap every callback/value in `useMemo` / `useCallback` |
| Server data | TanStack Query hook in `lib/` | `useEffect` + `fetch` + `useState` in component |
| Derived state | Compute in render | `useEffect` to copy/transform |
| Form actions | `useActionState` / `<form action={...}>` for new forms | Manual `onSubmit` boilerplate when an Action fits |
| Tailwind | `@import "tailwindcss"` (v4) + design tokens | `@tailwind base/components/utilities`, `tailwind.config.js` rewrite |
| Class merge | `cn()` from `@/lib/utils` | String concatenation of conditional classes |
| Variants | `cva()` with a `defaultVariants` block | Inline conditional class trees |
| Composition | Radix `asChild` + `Slot` | Re-implementing dropdowns/dialogs/tooltips |
| Tables | TanStack Table 8 + columns at module scope | New `columns` array on every render; ad-hoc `<table>` |
| Routing | Library mode, lazy routes | Framework mode without an explicit SOW |
| Visualization | Recharts default; VisX/D3 for custom; Three.js only inside lazy boundary | Three.js statically imported in main bundle |
| Three.js | `useEffect` cleanup disposes geom/mat/tex/renderer | Mount scene once, never dispose |
| Performance | Skeletons, Suspense, lazy routes | Layout shift, blocking imports, unmemoized 1k-row lists |
| TypeScript | Strict, `satisfies`, discriminated unions, `unknown` at boundaries | `any`, `as any`, `!`, `@ts-ignore` |
| Accessibility | Semantic elements, labels, focus rings, contrast | Clickable `<div>`, icon-only buttons without `aria-label` |
| Security | `dompurify` via `lib/safe-html.ts`; URL scheme checks | Raw HTML injection from data |
| Toasts | `sonner` | Hand-rolled portals |
| Theming | `next-themes` + `.dark` class + design tokens | Hex/RGB literals, `style={{ background: ... }}` for theme values |
| Bundle | Route-level `lazy()`, dynamic import for heavy libs | Barrel re-exports, static imports of three.js/globe.gl |
| State libs | URL + TanStack Query + local `useState` | Adding Redux/Zustand without a SOW |

---

## Sources

All accessed April 2026 unless noted.

- [React 19 release notes](https://react.dev/blog/2024/12/05/react-19) — Dec 5, 2024.
- [React Compiler 1.0](https://react.dev/blog/2025/10/07/react-compiler-1) — Oct 7, 2025; stable.
- [React 19.2 release](https://react.dev/blog/2025/10/01/react-19-2) — Oct 1, 2025.
- [You Might Not Need an Effect](https://react.dev/learn/you-might-not-need-an-effect) — React docs.
- [Tailwind CSS v4 upgrade guide](https://tailwindcss.com/docs/upgrade-guide) — current.
- [Tailwind v4 official LLM/skill discussion](https://github.com/tailwindlabs/tailwindcss/discussions/19594) — community LLM-skill thread.
- [Tailwind v4: archenemy of Claude 3.7 Sonnet](https://medium.com/@dpzhcmy/tailwind-css-v4-the-archenemy-of-claude-3-7-sonnet-209ce7470f76) — community post on AI v3-vs-v4 mismatch.
- [TanStack Query v5: Query Options](https://tanstack.com/query/v5/docs/react/guides/query-options).
- [TanStack Query v5: Query Keys](https://tanstack.com/query/v5/docs/react/guides/query-keys).
- [TanStack Query v5: Important Defaults](https://tanstack.com/query/latest/docs/framework/react/guides/important-defaults).
- [TanStack Table v8: Pagination + Virtualization](https://tanstack.com/table/v8/docs/guide/pagination).
- [React Router v7: Type Safety](https://reactrouter.com/explanation/type-safety).
- [React Router v7 modes](https://blog.logrocket.com/react-router-v7-modes/).
- [React Router v7 lazy loading](https://remix.run/blog/faster-lazy-loading).
- [shadcn/ui core concepts](https://vercel.com/academy/shadcn-ui/core-concepts).
- [Understanding asChild and Slot in React](https://peerlist.io/jagss/articles/understanding-aschild-and-slot-in-react-clean-flexible-compo).
- [Vite code-splitting: chunks larger than 500 KiB](https://github.com/vitejs/vite/discussions/9440).
- [Three.js: when to dispose, full scene cleanup](https://discourse.threejs.org/t/when-to-dispose-how-to-completely-clean-up-a-three-js-scene/1549).
- [Three.js memory leak in R3F when canvas scrolls out of view](https://discourse.threejs.org/t/r3f-threejs-memory-leak-when-canvas-is-scrolled-out-of-view/48440).
- [Tips on preventing memory leaks in Three.js scenes](https://roger-chi.vercel.app/blog/tips-on-preventing-memory-leak-in-threejs-scene).
- [TypeScript: unknown vs never](https://betterstack.com/community/guides/scaling-nodejs/never-unknown-types/).
- [TypeScript: satisfies for exhaustive type checks](https://return2.net/typescript-use-satisfies-for-exhaustive-type-checks/).
- [Vercel: Introducing React best practices](https://vercel.com/blog/introducing-react-best-practices).
- [Builder.io: React compiler will not solve prop drilling](https://www.builder.io/blog/react-compiler-will-not-solve-prop-drilling).
- [Code Rot vs Code Gen: AI-React strategy 2025–2026](https://fullstacktechies.com/code-rot-vs-code-gen-ai-react-strategy/).
- [Stop Using useEffect for Derived State](https://medium.com/@dreamerkumar/stop-using-useeffect-for-derived-state-a-react-anti-pattern-thats-killing-your-app-s-performance-8dcb83b48805).

Verify before relying on anything time-sensitive: React Compiler ESLint rule names (`set-state-in-render`, `set-state-in-effect`, `refs`) and React Router v7 minor-version lazy-loading API; both are still evolving as of the cited dates.
