# SOW-0080 - React Compiler Memoization Decision

## Status

Status: completed

Sub-state: validated

## Requirements

### Purpose

Decide and implement the project position on React Compiler adoption and manual
memoization cleanup so frontend performance guidance is concrete, testable, and
not left as a prose-only deferral.

### User Request

Review project quality against the four named project skills, identify gaps,
create SOWs for actionable improvements, and iterate the audit until valid
findings are fixed or tracked.

### Assistant Understanding

Facts:

- `project-frontend-best-practices` prefers React Compiler and warns against
  cargo-culted `useMemo`, `useCallback`, and `React.memo`.
- Iterative audit cycle 4 found that SOW-0040 still records React Compiler as a
  separate decision without a concrete pending SOW path.
- React Compiler was not wired in `ui/vite.config.ts` before this SOW.
- Manual memoization remains in active components including
  `ui/src/components/admin/feeds-table-body.tsx` and
  `ui/src/components/home/home-explorer.tsx`.
- Imported `memo()` wrappers remain in active components including
  `ui/src/components/feed-sidebar.tsx`,
  `ui/src/components/admin/feeds-table-body.tsx`, and
  `ui/src/components/admin/feeds-table-filters.tsx`.

Inferences:

- This is a frontend architecture/performance decision, not a safe mechanical
  search-and-replace.
- Global compiler adoption is not automatically better for this project because
  the UI has explicit route bundle budgets.

Unknowns:

- Whether future profiled route work can safely annotate selected components
  without exceeding `pnpm --dir ui build:budget`.

### Acceptance Criteria

- Verify current official React Compiler/Vite/ESLint compatibility before
  changing dependencies or build configuration.
- Choose one explicit outcome:
  - enable React Compiler and remove unnecessary manual memoization where
    covered by tests, or
  - keep React Compiler disabled with evidence and add a concrete local rule for
    where manual memoization is allowed.
- Audit active `useMemo`, `useCallback`, `React.memo`, and imported `memo()`
  sites and classify each as required, removable, or dependent on compiler
  adoption.
- Update SOW-0040 closure mapping so React Compiler/manual memoization is no
  longer a prose-only deferral.
- Run `pnpm --dir ui lint`, `pnpm --dir ui test`, and `pnpm --dir ui build`.

## Analysis

Sources checked:

- `project-frontend-best-practices`
- Official React Compiler installation docs:
  https://react.dev/learn/react-compiler/installation
- Official React hooks ESLint docs:
  https://react.dev/reference/eslint-plugin-react-hooks
- Installed `@vitejs/plugin-react@6.0.1` README and types under
  `ui/node_modules/@vitejs/plugin-react/`
- Iterative audit cycle 4 frontend best-practices findings
- `ui/vite.config.ts`
- `ui/package.json`
- `ui/src/components/admin/feeds-table-body.tsx`
- `ui/src/components/home/home-explorer.tsx`
- `ui/src/components/feed-sidebar.tsx`
- `ui/src/components/admin/feeds-table-filters.tsx`

Current state:

- React 19.2.5, Vite 8.0.10, `@vitejs/plugin-react` 6.0.1,
  `eslint-plugin-react-hooks` 7.1.1, and React Compiler 1.0.0 are compatible
  enough to install and build.
- The installed `@vitejs/plugin-react@6.0.1` API does not accept the older
  inline `react({ babel: ... })` shape. Its local README documents
  `reactCompilerPreset()` through `@rolldown/plugin-babel`.
- Global compiler mode builds, but fails the project's bundle budget gate:
  home route gzip 17.1 KiB / 14.0 KiB, admin route gzip 56.9 KiB / 50.0 KiB,
  and entity routes gzip 13.9 KiB / 12.0 KiB.
- Annotation mode has no current `"use memo"` targets, keeps the adoption path
  available, and passes `pnpm --dir ui build:budget`.

Risks:

- Blindly enabling global compilation breaks a project-owned performance gate.
- Blindly deleting memoization remains risky because many sites protect table
  state, expensive list transforms, visualization layout inputs, hover handler
  identity, or child row identity.

Decision:

- Use React Compiler only in opt-in annotation mode for now.
- Keep existing manual memoization unless a focused test/profile proves removal
  is behaviorally safe and bundle-budget neutral.
- New `useMemo`, `useCallback`, `memo()`, or `"use memo"` annotations require a
  concrete identity/effect/measured-performance reason.

Manual memoization inventory:

| Classification | Files / sites | Evidence |
|---|---|---|
| Required now: derived route data and category/grouping transforms | `ui/src/pages/home.tsx`, `ui/src/pages/asn-detail.tsx`, `ui/src/pages/country-detail.tsx`, `ui/src/pages/maintainer-detail.tsx`, `ui/src/lib/categories.ts`, `ui/src/components/ip-search/ip-search-results.tsx` | These derive arrays/groups from async query data that flow into child components and tables. |
| Required now: table/filter state and counts | `ui/src/components/editorial/data-table.tsx`, `ui/src/components/admin/feeds-table.tsx`, `ui/src/components/admin/admin-command-palette.tsx`, `ui/src/components/admin/feed-modal-manifest.tsx` | These compute filter results, sort order, column maps, counts, or modal file lists. |
| Required now: visualization inputs/layouts | `ui/src/components/feed-detail/asn-bubble-chart.tsx`, `asn-treemap.tsx`, `geo-map.tsx`, `overlap-network.tsx`, `overlap-sankey.tsx`, `section-behavior.tsx`, `section-bogons.tsx`, `section-comparison.tsx`, `section-retention.tsx`, `hero.tsx`, and home explorer view components | These protect layout calculations, parsed CSV points, force/network/sankey/treemap inputs, color scales, and grouped timelines. |
| Required now: callback identity for child/event contracts | `ui/src/lib/feed-prefetch.ts`, `ui/src/components/home/home-explorer.tsx`, `ui/src/components/feed-sidebar.tsx`, visualization hover callbacks | These callbacks are passed to child components, event handlers, or effect-like interaction paths. |
| Required now: memo-wrapped rows/chips | `ui/src/components/feed-sidebar.tsx`, `ui/src/components/admin/feeds-table-body.tsx`, `ui/src/components/admin/feeds-table-filters.tsx` | These wrap repeated rows/chips that receive stable props from filtered/sorted collections. |
| Dependent on future compiler proof, not a current defect | trivial query fallback arrays such as `feedsQuery.data ?? []` and small grouping transforms in page components | These can be revisited only with route-level behavioral tests/profile evidence and passing bundle budget. |
| Removable now | none | Global compiler mode fails budget; annotation mode compiles nothing without `"use memo"` directives. |

## Plan

1. Verify current React Compiler integration requirements against the installed
   frontend toolchain.
2. Inventory active manual memoization sites and classify them.
3. Implement the chosen path or record a concrete evidence-backed non-goal.
4. Update frontend skills/specs only if a durable local rule changes.
5. Validate with UI lint, tests, and build.

## Execution Log

### 2026-05-01

- Created from iterative audit cycle 4 after frontend best-practices review
  found the SOW-0040 React Compiler/manual memoization mapping too vague.
- Installed React Compiler dependencies:
  `babel-plugin-react-compiler@1.0.0` and `@rolldown/plugin-babel@0.2.3`.
- Updated `ui/vite.config.ts` to use `reactCompilerPreset()` in opt-in
  annotation mode.
- Updated `ui/README.md` and
  `.agents/skills/project-frontend-best-practices/SKILL.md` with the local
  memoization rule.
- Tested global compiler mode and rejected it for now because it fails
  `pnpm --dir ui build:budget`.
- Updated SOW-0040 closure mapping so this work is no longer a prose-only
  deferral.

## Validation

Acceptance criteria evidence:

- Official compatibility checked:
  - React docs say React Compiler is designed to work best with React 19.
  - React docs say the Vite integration uses a Babel plugin path.
  - Local `@vitejs/plugin-react@6.0.1` docs require
    `reactCompilerPreset()` with `@rolldown/plugin-babel`.
  - Local `eslint-plugin-react-hooks@7.1.1` config already exposes compiler
    diagnostics such as `react-hooks/config`, `react-hooks/gating`, and
    `react-hooks/preserve-manual-memoization`.
- Explicit outcome chosen: opt-in annotation mode, with global compilation
  rejected until a future route-level profile proves budget-neutral value.
- Active memoization sites audited and classified above.
- SOW-0040 closure mapping updated.
- Validation commands:
  - `pnpm --dir ui lint` passed.
  - `pnpm --dir ui test` passed: 9 files, 22 tests.
  - `pnpm --dir ui build` passed with the existing unresolved static font
    warnings.
  - `pnpm --dir ui build:budget` passed in annotation mode.
  - Control check: global compiler mode failed `build:budget`, which is the
    evidence for not enabling global compilation now.

Reviewer findings:

- Frontend best-practices review found that React Compiler/manual memoization
  remains unfixed and lacked a concrete pending SOW.

Artifact maintenance gate:

- `AGENTS.md`: no update needed; SOW process rules did not change.
- Runtime project skills: updated
  `.agents/skills/project-frontend-best-practices/SKILL.md`.
- Specs: no product/application contract changed.
- End-user/operator docs: updated `ui/README.md`, a developer-facing UI
  surface.
- End-user/operator skills: no external skills changed.
- SOW lifecycle: SOW moved from pending to current and will move to done after
  validation; SOW-0040 mapping updated.

## Outcome

Completed.

React Compiler is available only through opt-in `"use memo"` annotation mode.
Global compilation is not enabled because it currently violates the UI bundle
budget gate. Existing manual memoization remains valid unless a focused future
change proves a specific removal is safe and keeps `pnpm --dir ui build:budget`
green.
