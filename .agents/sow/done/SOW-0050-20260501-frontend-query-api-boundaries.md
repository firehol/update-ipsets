# SOW-0050 | 2026-05-01 | frontend-query-api-boundaries

## Status

completed

## Requirements

### Purpose

Make the frontend data layer easier to maintain by splitting oversized API and
query helpers by concern, while preserving route-code splitting and typed
request cancellation.

### User request quoted verbatim

> the next 4 sows 31-34 are yours, about the code quality and testing of
> this application. I don't want to be involved. Consider them a gift from
> me. I have also researched 4 related skills which you can use while
> coding. I need you to review them, decide what is valid and what is not,
> research the application properly, and implement the ones you believe are
> justified. No questions for me.

### Assistant understanding

- SOW-0040 found `ui/src/lib/api.ts` had become a broad API object covering
  unrelated catalog, feed-detail, entity, methodology, home, search, and admin
  concerns.
- Many components repeated TanStack Query keys and query functions inline,
  which made prefetch and invalidation contracts harder to reuse safely.
- The valid improvement is a narrow frontend data boundary: small typed API
  client modules, stable shared query keys, query-option factories split by
  concern, and measured bundle output to prove route splitting still works.

### Acceptance criteria

- API helpers are split into per-concern modules with a shared HTTP helper.
- Query keys and `queryOptions()` factories exist for repeated query families.
- Components consume query factories without behavior changes.
- Feed navigation prefetch exists without pulling feed-detail section endpoints
  into the public shell.
- Suspense migration is evaluated with evidence.
- Validation covers frontend gates, relevant Go gates, install, and runtime
  smoke.

## Analysis

Facts:

- Existing `ui/src/App.tsx` already had route-level `Suspense` and
  `RouteErrorBoundary`.
- `ui/src/lib/api.ts` was about 584 lines before this work and mixed every API
  concern into one object.
- A first pass with one central query-options module built successfully but
  made the public entry chunk larger: `index` was about 475.46 kB / 150.49 kB
  gzip.
- A subsequent build scan proved why this was still fragile: shared public chrome
  can import one prefetch factory and Rollup may hoist unrelated endpoint
  families into the main chunk when query/API modules are too broad.

Decision:

- Use concern-specific API modules under `ui/src/lib/api-client/`.
- Keep `ui/src/lib/api.ts` only as a compatibility shim.
- Use shared query keys in `ui/src/lib/query-keys.ts`.
- Use concern-specific query factories under `ui/src/lib/queries/`.
- Split `feed-core` from feed-detail section queries so hover prefetch imports
  only feed metadata.
- Lazy-load `AdminLayout`, not just `AdminPage`, because it fetches admin
  status and otherwise leaks admin query code into the public entry chunk.
- Reject a Suspense data-query migration for this work. Evidence: route-level
  `Suspense` already exists, feed-detail sections intentionally render
  independent loading/error states behind section error boundaries, and
  switching query state handling to Suspense would be a visible UX/error-flow
  change rather than a narrow data-boundary cleanup.

## Plan

1. Split API helpers by concern.
2. Add query key and query-option factories by concern.
3. Migrate components to narrow imports.
4. Add bounded feed-detail prefetch from list/sidebar links.
5. Validate route chunk output and fix any leaked broad modules.
6. Update specs and project skills with the durable frontend boundary lesson.

## Execution log

- Replaced broad `ui/src/lib/api.ts` implementation with a compatibility shim.
- Added typed API client modules:
  - `ui/src/lib/api-client/http.ts`
  - `ui/src/lib/api-client/catalog.ts`
  - `ui/src/lib/api-client/feed-core.ts`
  - `ui/src/lib/api-client/feed.ts`
  - `ui/src/lib/api-client/search.ts`
  - `ui/src/lib/api-client/home.ts`
  - `ui/src/lib/api-client/entities.ts`
  - `ui/src/lib/api-client/methodology.ts`
  - `ui/src/lib/api-client/admin.ts`
- Added shared query keys in `ui/src/lib/query-keys.ts`.
- Added query factories under `ui/src/lib/queries/`, split by concern.
- Migrated pages/components from inline query functions to query factories.
- Changed admin mutation components to import `api-client/admin` directly
  instead of the broad compatibility `api` object.
- Added `ui/src/lib/feed-prefetch.ts` and wired hover/focus prefetch in the
  homepage feed cards and feed sidebar rows.
- Lazy-loaded `AdminLayout` in `ui/src/App.tsx`.
- Updated `.agents/sow/specs/website.md` with the public-shell import contract.
- Updated project skills:
  - `.agents/skills/project-coding/SKILL.md`
  - `.agents/skills/frontend-best-practices/SKILL.md`
  - `.agents/skills/project-reviewing/SKILL.md`

## Validation

Acceptance evidence:

- API split: `ui/src/lib/api-client/` contains concern-specific client modules;
  `ui/src/lib/api.ts` is now a 19-line compatibility shim.
- Query factories: `ui/src/lib/query-keys.ts` and `ui/src/lib/queries/*.ts`.
- Component migration: `rg "@/lib/query-options" ui/src` returns no matches.
- Broad API object removal from components: `rg "import \\{ api \\}|api\\." ui/src`
  returns no matches.
- Query cancellation: every `queryFn: ({ signal })` match lives in
  `ui/src/lib/queries/` and passes `signal` to the typed client.
- Bundle evidence after final split:
  - public entry chunk: `465.74 kB / 148.10 kB gzip`
  - public `home` route chunk: `22.03 kB / 6.08 kB gzip`
  - admin endpoint strings appear in the lazy admin chunk only
  - feed-detail section endpoint strings appear in the lazy feed-detail chunk
    only
  - public entry keeps only catalog, feed-metadata prefetch, and IP-search
    endpoints

Commands passed:

- `pnpm --dir ui lint`
- `pnpm --dir ui build`
- `pnpm --dir ui test`
- `make build`
- `make test`
- `make lint`
- `make test-strict`
- `make fuzz-replay`
- `make race`
- `make staticcheck`
- `make golangci-lint`
- `make vulncheck`
- `git diff --check`

Reviewer evidence:

- Project-reviewing checklist applied locally.
- Independent assistant review was not run because the active tool policy
  allows subagents or external assistants only when explicitly requested. The
  gap is recorded; validation was strengthened with emitted-bundle endpoint
  scans and full project gates.

## Outcome

The frontend data layer now has narrow API clients, reusable query factories,
explicit shared query keys, and measured route-boundary behavior. The public
shell no longer imports admin endpoint helpers or feed-detail section endpoint
families through broad barrels.

## Lessons extracted

- Source-level modularity is insufficient for frontend boundary work. The built
  chunks must be inspected because shared route chrome can pull a broad query
  or API module into the public shell.
- Query factories should be split by route/concern. A central query-options
  registry is convenient but can defeat route splitting.
- Route-specific layouts that fetch data need the same lazy-loading treatment
  as route pages.
