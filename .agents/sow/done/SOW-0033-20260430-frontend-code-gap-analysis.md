# SOW-0033 | 2026-04-30 | frontend code quality hardening

## Status

completed

## Requirements

### Purpose

Improve the React/TypeScript frontend source so it is cheaper to load, safer
to navigate, easier to maintain, and less likely to leak browser or network
resources.

### Scope

- `ui/src/App.tsx` routing and route failure boundaries
- `ui/src/lib/api.ts` and TanStack Query call sites
- frontend theme ownership
- admin feed-table row accessibility
- homepage/sidebar filter responsiveness
- `react-globe.gl` scene lifecycle and label safety
- durable frontend rules in project specs and skills

Generated frontend bundle files under `pkg/web/static/` are not source and
were not edited.

## Implementation

### Route loading and failure isolation

- `ui/src/App.tsx` now lazy-loads each top-level page.
- `ui/src/components/route-error-boundary.tsx` provides the route-level error
  boundary and route loading fallback.
- Production build output confirms route chunks for admin, feed detail,
  methodology, home, country, ASN, maintainer, and not-found routes instead of
  one eager page bundle.

### Query cancellation

- `ui/src/lib/api.ts` helpers now accept an optional `AbortSignal` and pass it
  to `fetch`.
- TanStack Query call sites now use `queryFn: ({ signal }) => ...`.
- `ui/src/lib/world-topology.ts` also forwards the signal to its static fetch.
- Verification scan found no remaining direct `queryFn: api.*`,
  `queryFn: () => api.*`, or `queryFn: async ()` patterns in `ui/src`.

### Theme ownership

- `ui/src/components/theme-provider.tsx` now uses `next-themes` directly with
  `attribute="class"`, `defaultTheme="dark"`, `enableSystem`, and the existing
  project storage key.
- `ui/src/components/theme-toggle.tsx` reads `resolvedTheme` from
  `next-themes`.
- `ui/src/components/ui/sonner.tsx` now reads the same provider state as the
  rest of the application.

### Accessibility and responsiveness

- `ui/src/components/admin/feeds-table-body.tsx` feed rows now have keyboard
  activation for Enter and Space plus an accessible row action label.
- `ui/src/components/feed-sidebar.tsx` uses `useDeferredValue` for search text
  so input updates stay immediate while list filtering runs from the
  lower-priority search value.

### Globe lifecycle and label safety

- `ui/src/components/home/home-globe-scene.tsx` now disposes scene objects,
  materials, textures, renderer state, and the explicit globe material on
  unmount.
- Polygon callbacks are stable where identity matters for the globe component.
- `polygonLabel` escapes data-derived country names before returning HTML.
- StrictMode remount behavior is protected by a generation check so development
  remounts do not dispose the active scene instance.

### Evidence-backed non-goals

- `eslint-plugin-jsx-a11y` was not added because the current published package
  does not declare ESLint 10 peer compatibility:
  `eslint-plugin-jsx-a11y@6.10.2` declares `eslint: ^3 || ^4 || ^5 || ^6 ||
  ^7 || ^8 || ^9`, while this UI uses `eslint@^10.2.1`.
- React Compiler was not enabled. The project does not include the compiler
  package or Vite integration, and enabling it changes build semantics beyond
  this source-hardening work.
- Prettier was not made a gate. The package exists but no project script
  enforces it; adding it would create formatting churn unrelated to the
  behavioral fixes above.
- `queryOptions()` factories were not introduced. The cancellation issue was
  fixed without changing query-key ownership or cache APIs.
- Virtualization was not introduced. The current public catalog is roughly
  hundreds of rows and no measured scroll jank justified a table framework
  change inside this work.
- API schema validation was not introduced. The app already has generated-style
  static TypeScript contracts; adding runtime validators needs a single
  backend/frontend schema source decision, not a local component patch.

## Durable Memory Updates

- `.agents/sow/specs/website.md` now records the frontend runtime contract:
  route lazy-loading, route-level boundaries, request cancellation, single
  theme owner, keyboard-accessible non-button interactions, and safe WebGL
  lifecycle/labels.
- `.agents/skills/project-coding/SKILL.md` now records the coding rules for
  query signals, lazy routes, `next-themes`, WebGL cleanup, low-priority
  filtering, and clickable table rows.
- `.agents/skills/project-reviewing/SKILL.md` now records review checks for
  the same frontend regressions.
- `.agents/skills/project-testing/SKILL.md` now records browser validation
  expectations for route-splitting and WebGL scenes.

## Validation

- `pnpm --dir ui lint` passed.
- `pnpm --dir ui build` passed.
- Build still emits the existing unresolved `/static/fonts/InterDisplay-*.woff2`
  warnings; those font URLs are resolved by the runtime static server.
- Production build output includes separate lazy route chunks such as
  `admin-*.js`, `feed-detail-*.js`, `home-*.js`, `methodology-*.js`,
  `country-detail-*.js`, and `asn-detail-*.js`.
- Browser smoke ran against Vite with the local backend API:
  `http://127.0.0.1:5177/` loaded the real homepage with live API data.
- Globe component browser validation rendered a 900x620 WebGL canvas through
  Vite source modules, captured a nonblank 1280x720 screenshot
  (`ImageMagick mean=0.0742154`), and unmounted the component with canvas count
  changing from 1 to 0.

## Residual Risk

- `HomeGlobePanel` is present in the source tree but not mounted by the current
  homepage. The component-level browser render validates the changed scene
  behavior; it is not proof that an unreachable UI section is displayed on the
  public homepage.
- Accessibility lint coverage remains manual until an ESLint 10-compatible
  jsx-a11y rule set is available or the project changes ESLint versions.
