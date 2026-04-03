# TODO-HOMEPAGE — Implementation tracker

## Status

**This tracker supersedes the 2026-04-14 plan.** That plan centered the globe in
the hero and organized the homepage around three ranked lists (top countries,
top ASNs, top maintainers) plus compact category lanes. None of it was
implemented.

The normative contract for the homepage lives in `specs/homepage.md`. This file
is an implementation tracker only.

## TL;DR

Rebuild the homepage from scratch as three zones:

1. **Hero** — typography-driven mission + stats strip. No globe, no IP search.
2. **IP Lookup** — full section. Input + rich result card (geo + map + ASN +
   role + matching feeds grouped by category). Optional scoped globe background
   with pin-drop only.
3. **Feed Explorer** — the product. Preset lenses on top, faceted filter rail,
   multiple view modes over the filtered result set. **Absorbs `/catalog`**.

Visual language: the editorial system already established on feed-detail pages
(Apple / Samsung product-page, hairline chrome, single red accent, restrained
typography). No SaaS-dashboard density, no rainbow palettes, no per-section
card frames.

## Locked decisions

From the design conversation (2026-04-20):

| # | Decision | Choice |
|---|----------|--------|
| 1 | Scope | Complete redesign. Homepage **is** the explorer. `/catalog` retires. |
| 2 | Exploration dimensions | All: category, threat type, maintainer, credibility tier, size, cadence, freshness, health, provenance, uniqueness, license, free-text. |
| 3 | Viewing modes | All: dense table, rich cards, treemap, overlap matrix, freshness timeline, maintainer groups, world map, globe. Table + cards required; the rest SHOULD. |
| 4 | Ad-hoc combines | Deferred (backend not ready). |
| 5 | IP Lookup result card | Country, flag, map pin, ASN, ASN name, infrastructure role, matching feeds grouped by category, first/last-seen when available. |
| 6 | Hero visual | Pure editorial typography + stats strip. No globe in hero. |
| 7 | Explorer IA | Lens strip (entry) → faceted filter rail + result surface (drill-down). |
| 8 | Globe scope | Only two places: (a) IP Lookup section background with pin-drop (no feed-hover coupling), (b) one of the explorer view modes (full-bleed, feed-hover lights countries). |
| 9 | Lens set | At launch: Freshest, Most unique, Largest coverage, Starter pack, By threat type, By maintainer. |
| 10 | Default view mode | Rich card grid. User preference persists in localStorage. |
| 11 | Filter rail | Left rail on desktop, off-canvas drawer on mobile. |
| 12 | Result count | All feeds visible via virtual scroll; no paging. |
| 13 | Mobile | Simplified: card list + category chips + search; analytical view modes may be omitted. Must not hide feeds from the filtered result set. |

2026-04-24 default health filter decision:

- Costa wants the homepage feed explorer to start with these health classes
  selected by default:
  - `healthy`
  - `delayed`
  - `risky`
  - `unavailable`
- Costa explicitly does **not** want these selected by default:
  - `archived`
  - `unmaintained`
  - `empty`

Implication:

- the homepage explorer default state must be opinionated, not "show every
  health class"
- URL state must still remain authoritative when a specific `health=` filter
  is present
- shareable URLs still need a way to express "no health restriction", so the
  implementation may use an explicit sentinel instead of treating omitted
  `health=` as "show everything"

Verification note:

- implemented in:
  - `ui/src/lib/explorer-state.ts`
  - `ui/src/components/home/home-explorer-filter-rail.tsx`
  - `ui/src/components/home/home-explorer.tsx`
  - `specs/homepage.md`
- behavior:
  - default selection = `healthy`, `delayed`, `risky`, `unavailable`
  - `archived`, `unmaintained`, `empty` remain opt-in
  - `Clear all` restores this baseline instead of clearing health entirely
  - the mobile active-filter badge treats this default baseline as zero

## Resolved items (Costa authorized the implementation agent to decide)

- **O1. "Starter pack" lens composition**: members of `firehol_level1` (already
  curator-rule-based; no new editorial judgment).
- **O2. Maintainer credibility tier filter**: omitted from v1. Adds later with
  `specs/maintainers.md`.
- **O3. IP-level first-seen / last-seen per feed**: skipped in v1 (spec already
  allows "when available"). Revisit as a backend workstream post-v1.
- **O4. Globe view mode inside the explorer**: shipped in Wave 5.

---

## Implementation waves

Each wave is a self-contained chunk that lands behind a commit, preferably a
working daemon. Wave ordering follows dependencies, not feature priority.

### Wave 0 — Housekeeping ✅ DONE

- ✅ Deleted `ui/src/components/home/home-dimensions.tsx`.
- ✅ Deleted `ui/src/components/home/home-workflows.tsx`.
- ✅ Removed their imports from `ui/src/pages/home.tsx`.

### Wave 1 — Hero zone ✅ DONE

- ✅ Rewrote `home-hero.tsx` as typography-driven. No globe, no search.
- ✅ Stats strip derived client-side from `/api/v1/sets`: tracked feeds, IPs
  across feeds (labelled honestly as a sum, not a dedup), maintainers,
  categories.
- ✅ Primary CTA "Explore all feeds" scrolls to `#explorer`; secondary
  "Look up an IP" scrolls to `#ip-lookup`.
- ℹ️ `home-globe-panel.tsx` and `home-globe-scene.tsx` kept for Wave 2b reuse
  (scoped background behind the IP Lookup zone).

### Wave 2a — IP Lookup zone (UI scaffolding) ✅ DONE

- ✅ New component `home-ip-lookup.tsx`. Editorial section wrapper below the
  hero, above the explorer.
- ✅ Uses `IPSearchSurface` with `variant="section"`, `syncToUrl`,
  `scope=global` — URL-shareable `/ ?ip=...` state.
- ✅ Current result card renders matching feeds with category + maintainer +
  health (existing capability in `IPSearchResults`).
- ✅ Deleted `ip-search-panel.tsx` (dead code).

### Wave 2b — IP Lookup backend enrichment ✅ DONE

- ✅ Backend: new `engine.LookupIPContext(ip)` resolves single-IP country +
  ASN + infrastructure role via lazy-loaded provider caches
  (`ip_context.go`). Geo uses the existing `geoProviderCache`; ASN uses a new
  `asnDatabaseCache` mirrored on the engine.
- ✅ Search API: `GET /api/v1/search?ip=…&details=true` now returns a
  `context` block alongside `matches`.
- ✅ UI: `home-ip-lookup.tsx` rebuilt with a rich result card — country + ASN
  + ASN name + infrastructure role tiles plus matches grouped by category,
  with direct links into `/countries/{code}` and `/asns/{asn}`.
- ℹ️ Scoped globe background remains a future enhancement; not required by
  the normative spec (optional MAY clause).

### Wave 3 — Explorer shell + default card view + facets ✅ DONE

- ✅ `ui/src/lib/explorer-state.ts` — URL encoding, filter/sort logic, lens
  definitions.
- ✅ `home-explorer.tsx` orchestrator: reads URL state, runs filter+sort
  pipeline, hosts the lens strip + filter rail + view switcher + cards.
- ✅ `home-explorer-lens-strip.tsx` — 4 v1 lenses: Freshest, Largest coverage,
  By threat type, By maintainer. (Most unique + Starter pack land in Wave 4.)
- ✅ `home-explorer-filter-rail.tsx` — category chips, health chips, provenance
  chips, freshness bucket chips, size min/max, maintainer dropdown, free-text
  search. License filter deferred (field not yet on `FeedSummary`).
- ✅ `home-explorer-view-switcher.tsx` — sort chips + view mode indicator (card
  only in v1).
- ✅ `home-explorer-view-cards.tsx` — card grid; each card shows name,
  maintainer, category badge, health dot with tooltip, relative freshness, and
  the sort-relevant primary metric.
- ✅ URL state shareable: `?lens=…&category=…&maintainer=…&health=…&
  provenance=…&size_min=…&size_max=…&fresh=…&sort=…&q=…`.
- ✅ Deleted `home-category-lanes.tsx`.
- ℹ️ Mobile: filter rail stacks above cards on narrow screens. An off-canvas
  drawer is a Wave 7 polish item.

### Wave 4 — Table view + uniqueness dimension ✅ DONE

- ✅ Built `home-explorer-view-table.tsx` with sortable columns (name,
  maintainer, IPs, **unique %**, cadence, freshness), hairline chrome,
  per-row category badge and health dot.
- ✅ Backend: `unique_share_pct` + `unique_share_samples` on `cache.Entry`
  and `PublicFeedSummary`. Computed in `updateUniqueShares` right after
  `writeComparisonFiles` (`pkg/engine/unique_share.go`). Definition:
  share of a feed's IPs not covered by its closest independent peer,
  with same-maintainer + family-related peers excluded. Bounded proxy;
  methodology page explains the trade-offs.
- ✅ Methodology page: `pkg/web/static/methodology/unique-share.md`.
- ✅ UI: "Most unique" lens and uniqueness sort key + table column
  (`explorer-state.ts`, `home-explorer-view-switcher.tsx`,
  `home-explorer-view-cards.tsx`, `home-explorer-view-table.tsx`).

### Wave 5 — Additional view modes (partial ✅)

Shipped using existing data:

- ✅ `home-explorer-view-treemap.tsx` — per-category treemap keyed by
  `unique_ips`, palette from the config-driven category colors, click through
  to feed detail.
- ✅ `home-explorer-view-timeline.tsx` — freshness buckets (past hour → past
  90 days → older → no timestamp), ranked feed list per bucket with health
  dots and IP counts.
- ✅ `home-explorer-view-maintainers.tsx` — feeds grouped by maintainer with
  totals (feeds / IPs / categories) and category-card rows.

Deferred to the Go backend batch (need aggregated endpoints):

- ⏳ `home-explorer-view-world-map.tsx` — aggregated 2D geo. Needs
  `/api/v1/home/summary` or equivalent aggregate.
- ⏳ `home-explorer-view-overlap-matrix.tsx` — pairwise overlap across feeds
  in the current filter. Expensive to aggregate client-side; waits for a
  backend aggregation or a focused endpoint.
- ⏳ `home-explorer-view-globe.tsx` — interactive globe with feed-hover
  country overlays. Same dependency as world-map.

### Wave 6 — Classification detail pages + catalog retirement ✅ DONE

- ✅ Backend: `home_detail.go` + `home_detail_api.go` add four endpoints:
  - `GET /api/v1/countries/{code}` — contributing feeds + top ASNs in country
  - `GET /api/v1/asns/{asn}` — contributing feeds + ASN metadata + role
  - `GET /api/v1/maintainers` — maintainer index
  - `GET /api/v1/maintainers/{slug}` — maintainer detail with feeds grouped
    by category
- ✅ `/countries/:code` — real content: totals, feed table linking to feed
  detail, top-ASNs table linking into `/asns/{asn}`.
- ✅ `/asns/:asn` — real content: totals, ASN name + description +
  infrastructure role when registered, feed table.
- ✅ `/maintainers` — catalog-driven index (kept the client-side derivation
  since the backend agrees on slugs via `maintainerSlugify`).
- ✅ `/maintainers/:slug` — real content, feeds grouped by category with
  health + freshness.
- ✅ Routes wired into `App.tsx`; header and footer link to "Explore" and
  "Maintainers".
- ✅ `/catalog` redirects to `/#explorer`; `pages/catalog.tsx` deleted.

### Wave 7 — Polish (UI ✅ / backend ✅)

- ✅ Off-canvas mobile filter drawer — the left filter rail becomes a
  slide-in drawer below the `lg` breakpoint, with a sticky toggle button
  showing the active-filter count.
- ✅ View-mode code splitting — table, treemap, timeline, and maintainer-
  groups views are lazy-loaded as separate chunks (~2–3 kB each). Main
  homepage bundle dropped from ~1.23 MB to ~1.09 MB.
- ✅ `/api/v1/home/summary?categories=&limit=` — `home_summary.go` +
  `home_api.go`. Returns totals, top countries, top ASNs, top maintainers
  under the active category filter, applying the spec's aggregation filter
  policy (exclude system-role, ok/delayed health, primary/upstream
  provenance only).
- ✅ Methodology page for uniqueness (Wave 4). Additional pages for
  homepage lenses and feed-explorer remain a nice-to-have; not spec-required.

---

## Component inventory

### New components (Wave 1–6)

| File | Wave | Purpose |
|---|---|---|
| `ui/src/components/home/home-hero.tsx` (rewrite) | 1 | Typography hero + stats strip |
| `ui/src/components/home/home-stats-strip.tsx` | 1 | Four-tile stats row |
| `ui/src/components/home/home-ip-lookup.tsx` | 2 | IP lookup section (uses shared `ip-search-surface`) |
| `ui/src/components/home/home-ip-result-card.tsx` | 2 | Rich result card (geo + ASN + matching feeds) |
| `ui/src/components/home/home-explorer.tsx` | 3 | Orchestrator for the explorer zone |
| `ui/src/components/home/home-explorer-lens-strip.tsx` | 3 | Lens chips entry surface |
| `ui/src/components/home/home-explorer-filter-rail.tsx` | 3 | Left rail / off-canvas drawer |
| `ui/src/components/home/home-explorer-view-switcher.tsx` | 3 | View mode toggle |
| `ui/src/components/home/home-explorer-view-cards.tsx` | 3 | Default rich card grid |
| `ui/src/components/home/home-explorer-view-table.tsx` | 4 | Dense table (TanStack Table + virtual scroll) |
| `ui/src/components/home/home-explorer-view-treemap.tsx` | 5 | Category treemap |
| `ui/src/components/home/home-explorer-view-timeline.tsx` | 5 | Freshness / activity timeline |
| `ui/src/components/home/home-explorer-view-maintainer-groups.tsx` | 5 | Maintainer clustering |
| `ui/src/components/home/home-explorer-view-world-map.tsx` | 5 | Aggregated 2D geo |
| `ui/src/components/home/home-explorer-view-overlap-matrix.tsx` | 5 | Pairwise overlap matrix |
| `ui/src/components/home/home-explorer-view-globe.tsx` | 5 | Full-bleed interactive globe |
| `ui/src/pages/country-detail.tsx` | 6 | `/countries/:code` |
| `ui/src/pages/asn-detail.tsx` | 6 | `/asns/:asn` |
| `ui/src/pages/maintainers-index.tsx` | 6 | `/maintainers` |
| `ui/src/pages/maintainer-detail.tsx` | 6 | `/maintainers/:slug` |

### Components deleted

| File | Wave | Why |
|---|---|---|
| `ui/src/components/home/home-dimensions.tsx` | 0 | Filler (prior TODO decision, still applies) |
| `ui/src/components/home/home-workflows.tsx` | 0 | Filler (prior TODO decision, still applies) |
| `ui/src/components/home/home-category-lanes.tsx` | 3 | Explorer covers its purpose |
| `ui/src/components/home/ip-search-panel.tsx` | 2 | Replaced by `home-ip-lookup.tsx` |
| `ui/src/components/home/home-globe-panel.tsx` | 1 or 2 | Globe only used as IP-lookup background + explorer view mode |
| `ui/src/pages/catalog.tsx` | 6 | Catalog retires (after redirect lands) |

### Shared primitives reused

- `ui/src/components/editorial/` — accent bar, stat row/tile, big number, auto-fit text, minimal table.
- `ui/src/components/ip-search/` — shared IP search surface (component variants).
- `ui/src/components/category-badge.tsx` — config-driven category rendering.
- `ui/src/components/feed-detail/geo-map.tsx` — reusable for IP-lookup map + world-map view mode.

---

## Backend contract

### Endpoints to extend

- `GET /api/v1/search?ip=…&details=true` (and its alias `/api/v1/query`) MUST
  return enough to populate the IP Lookup result card:
  - matching feeds (exists)
  - country code + country name (exists partially; verify)
  - ASN + ASN name + infrastructure role (verify)
  - latitude/longitude for map pin (verify; if not present, add from the
    preferred geo provider)
  - per-match first-seen / last-seen (optional, may be omitted in v1)

- `GET /api/v1/sets` (feed summary) MUST add per-feed fields needed by the
  explorer:
  - `unique_share_pct`, `unique_share_samples`, optional
    `unique_share_per_category` (Wave 4)
  - `last_change_ts` / `average_update_mins` (exist? verify)
  - `license`, `redistributable` flags (verify; exposed)

### Endpoints to add

| Endpoint | Wave | Purpose |
|---|---|---|
| `GET /api/v1/home/summary?categories=…` | 7 | Totals, top countries, top ASNs, top maintainers, lens populations under active filter |
| `GET /api/v1/countries/{code}` | 6 | Country detail page payload |
| `GET /api/v1/asns/{asn}` | 6 | ASN detail page payload |
| `GET /api/v1/maintainers` | 6 | Maintainer index |
| `GET /api/v1/maintainers/{slug}` | 6 | Maintainer detail |

All aggregation endpoints MUST apply the filter policy defined in
`specs/homepage.md` ("Aggregation filter policy"): exclude system-role
categories, keep only `ok`/`delayed` health, keep only primary / upstream
provenance.

### Data needed that does not exist yet

- `unique_share_pct` per feed — computed from existing `_comparison.json` data.
  Exclude same-maintainer feeds + retention variants + merges from the
  "independent" set.
- Maintainer index — derive from `configs/firehol.yaml` `maintainer` and
  `maintainer_url` fields. Rich maintainer records come later with
  `specs/maintainers.md`.

---

## Dependencies on other specs

- **`specs/maintainers.md` (not yet written)** — the "credibility tier" filter
  dimension and the rich maintainer detail page both depend on maintainer
  records. v1 ships with a minimal maintainer view derived from the YAML
  fields; full functionality lands with that spec.

---

## Testing requirements

- Manual visual check at 1280 / 1536 / 1920 / 2560 viewport widths; mobile at
  375 / 768.
- Verify URL state round-trip: load with `?lens=…&category=…&view=…`, toggle
  filters, use back button; confirm explorer and lens strip stay consistent.
- Verify no filtered feed is hidden on mobile for the same filter state.
- Verify feed-detail links bypass the aggregation filter (a `risky` or
  `unmaintained` feed is still directly reachable).
- Verify the scoped globe (Wave 2) does not continue running WebGL after the
  visitor scrolls past the IP Lookup zone.
- `pnpm --dir ui typecheck` and `pnpm --dir ui build` clean.
- `go test ./...` and `go test -race ./...` clean for any new backend work.

## Documentation updates required

- `specs/homepage.md` (written): keep current with any contract changes
  discovered during implementation.
- `specs/website.md` (updated): catalog retired, detail surfaces added.
- `AGENTS.md` (updated): `specs/homepage.md` linked in documentation map.
- `pkg/web/static/methodology/unique-share.md` (Wave 4): new.
- `pkg/web/static/methodology/homepage-lenses.md` (Wave 7): new.
- `CLAUDE.md` / `AGENTS.md` "Public route contract" section: already updated
  when `/catalog` retirement lands.

## Out of scope

- Ad-hoc feed combining (requires backend that does not exist).
- User accounts, stars, notifications, submissions, ownership claims,
  enterprise IP-space monitoring. These are a separate platform workstream.
- Feed detail page redesign (already done).
- Admin UI changes.
- Any changes to `pkg/iprange`, engine pipeline, scheduler, or cache format.
- IPv6 (phase 1 only supports IPv4 above the `pkg/iprange` boundary).
