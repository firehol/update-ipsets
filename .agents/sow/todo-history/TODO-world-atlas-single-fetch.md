# World Atlas Single Fetch

## TL;DR

The feed detail map currently downloads `https://cdn.jsdelivr.net/npm/world-atlas@2.0.2/countries-110m.json` twice on first page load. It should be fetched once through the existing TanStack Query cache and never separately by `react-simple-maps`.

## Purpose

Avoid redundant CDN traffic and keep map loading deterministic. The user should see the same map behavior without duplicate network requests.

## Analysis

- `ui/src/components/feed-detail/geo-map.tsx` defines `WORLD_TOPOJSON`.
- `useWorldTopology()` fetches that URL through TanStack Query.
- The first render passes `topologyQuery.data ?? WORLD_TOPOJSON` to `<Geographies>`.
- When `topologyQuery.data` is still undefined, `<Geographies>` receives the URL string.
- `react-simple-maps` fetches string geographies internally, so first load has two fetchers:
  - TanStack Query fetch;
  - `react-simple-maps` internal fetch.

## Feasibility Verdict

FEASIBLE AS SPECIFIED.

Evidence:

- The duplicate URL is directly visible in `geo-map.tsx`.
- The local installed `react-simple-maps` package code calls `fetch(url)` when `geography` is a string.
- `<Geographies>` accepts a parsed object, and the existing code already intended to pass one.

## Decisions

No user decision required. user asked to fix duplicate loading.

## Plan

1. Keep `useWorldTopology()` as the single fetch path.
2. While `topologyQuery.data` is not available, render a fixed-height map placeholder instead of `<Geographies>`.
3. Pass only the parsed topology object to `<Geographies>`.
4. Render a quiet error state if the topology query fails.
5. Verify with UI build and focused ESLint.

## Implied Decisions

- Do not vendor the world-atlas JSON in this change.
- Do not change the geolocation provider API.
- Keep the visible map layout stable while topology loads.

## Testing Requirements

- `pnpm --dir ui build`
- `pnpm --dir ui exec eslint src/components/feed-detail/geo-map.tsx`
- `git diff --check`
- Optional browser verification: run Vite dev server and count world-atlas network requests.

## Documentation Updates Required

- No public docs update required.

## Implementation Result

- Removed the `WORLD_TOPOJSON` fallback from `<Geographies>`.
- While the topology query is still loading, the map area renders a fixed-height loading message.
- If topology loading fails after query retries, the map area renders a fixed-height unavailable message.
- `<Geographies>` now receives only `topologyQuery.data`, which is the parsed object from TanStack Query.
- Added a stable empty-country array to remove the focused ESLint warning in this file.

## Verification Result

- `pnpm --dir ui build` passed.
- `pnpm --dir ui exec eslint src/components/feed-detail/geo-map.tsx` passed with no warnings.
- Browser verification against Vite dev server passed:
  - opened `http://127.0.0.1:5175/ipsets/dronebl_bottler`;
  - counted requests containing `world-atlas@2.0.2/countries-110m.json`;
  - result: `worldAtlasRequests: 1`.
- `git diff --check` passed.
