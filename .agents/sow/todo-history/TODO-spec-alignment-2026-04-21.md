# TODO: Spec Alignment 2026-04-21

## Purpose

Bring the implementation into complete, evidence-based alignment with the
authoritative specs under `specs/`, closing drift in code, docs, UI, API,
runtime behavior, and filesystem semantics so the application behaves exactly as
its contracts describe.

## TL;DR

- Perform a full gap analysis of implementation vs. specs.
- Fix every confirmed mismatch.
- If the implementation contains behavior that fits the product but is missing
  from the specs, specify it first and then align the implementation.
- Repeat the audit/fix/verify cycle until no further gaps can be found.

## Analysis

- Baseline verification:
  - `go test ./...` passes before changes, so this task is about contract drift,
    not an already-broken tree.
- Confirmed gaps from the first audit pass:
  1. Homepage aggregation policy is inconsistently applied.
     - Spec:
       - `specs/homepage.md` requires homepage aggregations to exclude system
         categories, include only `healthy` / `delayed`, and include only
         `primary` / `secondary_upstream` feeds.
     - Implementation evidence:
       - `ui/src/pages/home.tsx` hero totals count all non-system feeds.
       - `ui/src/lib/feed-ranking.ts` `homepageEligible()` filters by health
         only, not provenance.
       - `pkg/engine/home_globe.go` filters by category and health but does not
         exclude merge/retention provenance.
     - Impact:
       - Homepage totals and homepage globe eligibility can over-count feeds the
         spec says must be excluded from public aggregations.
  2. Explorer filtering is missing required axes and the public summary payload
     does not expose enough fields to implement them.
     - Spec:
       - `specs/homepage.md` requires filtering by update cadence,
         uniqueness, license, and redistributability.
     - Implementation evidence:
       - `ui/src/components/home/home-explorer-filter-rail.tsx` exposes only
         search, category, health, provenance, freshness, size, and maintainer.
       - `ui/src/lib/explorer-state.ts` has no cadence, uniqueness, license, or
         redistributability filter state.
       - `pkg/engine/public_catalog.go` `PublicFeedSummary` omits license and
         redistributability, so `/api/v1/sets` cannot drive those filters.
     - Impact:
       - The homepage explorer cannot satisfy the required faceted contract.
  3. Explorer view selection is not persisted across sessions.
     - Spec:
       - `specs/homepage.md` says the selected view mode SHOULD persist at a
         per-visitor level.
     - Implementation evidence:
       - `ui/src/lib/explorer-state.ts` reads only URL params.
       - `ui/src/components/home/home-explorer.tsx` writes only URL state.
     - Impact:
       - A returning visitor loses the last-selected explorer view.
  4. The IP lookup zone does not show a geographic map or a recognizable
     country indicator.
     - Spec:
       - `specs/homepage.md` requires the lookup result to show country of
         origin with a recognizable country indicator and position on a
         geographic map.
     - Implementation evidence:
       - `ui/src/components/home/home-ip-lookup.tsx` renders only a country code
         link and no map surface.
     - Impact:
       - Zone 2 is missing required result content.
  5. The admin UI does not expose the full required batch action set and uses
     ambiguous wording for the existing global run.
     - Spec:
       - `specs/admin-ui.md` requires batch-level actions for run due work now,
         force broad reprocessing where supported, and integrity recovery.
     - Implementation evidence:
       - `pkg/web/server.go` already supports `POST /api/v1/admin/run` with
         `reprocess=true`.
       - `ui/src/components/admin/current-run.tsx` exposes only `Trigger global
         run`.
       - `ui/src/components/admin/integrity-panel.tsx` covers integrity
         recovery, but no UI control exposes broad reprocess.
     - Impact:
       - The operator UI does not surface a supported batch capability and the
         run button does not clearly express "run due work now".
  6. A second-pass homepage audit found missing maintainer-detail links on
     explorer and lookup result surfaces.
     - Spec:
       - `specs/homepage.md` requires ranked entries and explorer rows to link,
         where applicable, to the public maintainer detail surface.
     - Implementation evidence:
       - `ui/src/components/home/home-explorer-view-table.tsx` rendered
         maintainer names as plain text.
       - `ui/src/components/home/home-explorer-view-maintainers.tsx` rendered
         the maintainer heading as plain text.
       - `ui/src/components/home/home-ip-lookup.tsx` rendered matched-feed
         maintainer names as plain text.
     - Impact:
       - The homepage exposes maintainer labels without the required navigation
         path to maintainer detail pages.
  7. A later Zone 2 audit found the IP lookup payload and UI lack per-match
     first-seen / last-seen context entirely.
     - Spec:
       - `specs/homepage.md` requires per-match context showing the first-seen
         and last-seen time of the IP in each matching feed when available.
     - Implementation evidence:
       - `pkg/engine/engine.go` `QueryMatch` has no time fields.
       - `pkg/engine/query.go` populates only name/category/provenance/info/
         maintainer/health/error.
       - `ui/src/lib/api-types.ts` `IPSearchMatch` has no time fields.
       - `ui/src/components/home/home-ip-lookup.tsx` has no timing row/column.
     - Impact:
       - Zone 2 cannot satisfy the required per-match timing context even for
         feeds where local retained history snapshots exist.
  8. The methodology API surface is HTML, not machine-readable, and the SPA
     scrapes it instead of consuming a stable public data contract.
     - Spec:
       - `specs/website.md` requires `/api/v1/methodology` and
         `/api/v1/methodology/{slug}` as part of the machine-readable public
         surface, and the UI must treat these as product data contracts rather
         than screen scraping.
     - Implementation evidence:
       - `pkg/web/methodology.go` serves wrapped HTML from both endpoints.
       - `ui/src/pages/methodology.tsx` fetches text and extracts `<main>` /
         `<article>` / `<body>` from the returned HTML.
     - Impact:
       - The public methodology surface does not satisfy the documented API
         contract and the frontend depends on brittle HTML scraping.
  9. Public map surfaces fetch world topology from a third-party CDN at
     runtime.
     - Spec:
       - `specs/operating-principles.md` forbids repeated-view upstream
         dependency for ordinary public browsing and requires local committed
         assets where practical.
     - Implementation evidence:
       - `ui/src/components/feed-detail/geo-map.tsx`,
         `ui/src/components/home/ip-lookup-country-map.tsx`, and
         `ui/src/components/home/home-globe-scene.tsx` fetched
         `https://cdn.jsdelivr.net/npm/world-atlas@2.0.2/countries-110m.json`
         at runtime.
     - Impact:
       - Public map rendering depended on third-party network availability and
         violated the local-asset serving rule for ordinary browsing.
  10. Public maintainer pages reconstruct maintainer data from the feed catalog
      instead of consuming the dedicated maintainer public API, and the website
      spec does not list those maintainer endpoints explicitly in its API
      route-family contract.
      - Spec:
        - `specs/website.md` says the public website MUST be backed by stable
          machine-readable product data and the UI MUST treat these as product
          data contracts rather than opportunistic reconstruction.
        - `specs/homepage.md` says the public data layer MUST expose the
          per-maintainer detail payloads required by linked detail surfaces.
      - Implementation evidence:
        - `pkg/web/home_detail_api.go` already serves
          `/api/v1/maintainers` and `/api/v1/maintainers/{slug}`.
        - `ui/src/lib/api.ts` already exposes `listMaintainers()` and
          `getMaintainerDetail()`.
        - `ui/src/pages/maintainers-index.tsx` ignores those endpoints and
          rebuilds maintainers client-side from `api.listFeeds()`.
        - `ui/src/pages/maintainer-detail.tsx` ignores the dedicated detail
          endpoint and reconstructs the page from the full feed list using
          `maintainerSlug()`.
        - `specs/website.md` public API route-family list omits the maintainer
          endpoints entirely even though they are part of the implemented public
          surface.
      - Impact:
        - The public UI depends on duplicated client-side derivation instead of
          the canonical public maintainer contract, and the spec understates the
          stable public API surface for maintainer data.
  11. The website spec still omits other implemented public route families that
      the UI already depends on for the public contract.
      - Spec:
        - `specs/website.md` route contract includes country detail, ASN
          detail, and global IP lookup as part of the public website surface.
      - Implementation evidence:
        - `pkg/web/server.go` serves `/api/v1/search`,
          `/api/v1/countries/{code}`, and `/api/v1/asns/{asn}`.
        - `ui/src/lib/api.ts` already consumes `/api/v1/search`,
          `/api/v1/countries/{code}`, and `/api/v1/asns/{asn}`.
        - `specs/website.md` public API route-family list omitted all three
          endpoint families.
      - Impact:
        - The normative website API contract understates already-implemented
          machine-readable public routes that are required by the public UI.
  12. The maintainer detail page orders category sections by local UI
      heuristics instead of the configured category ordering required by the
      public category-presentation contract.
      - Spec:
        - `specs/homepage.md` says category labels, descriptions, colors, and
          ordering MUST come from configuration.
        - `specs/website.md` says category presentation MUST be data-driven,
          including ordering.
      - Implementation evidence:
        - `ui/src/lib/categories.ts` already exposes `orderCategories()` using
          category metadata and `sort_order`.
        - `ui/src/pages/maintainer-detail.tsx` sorts
          `feeds_by_category` sections by feed-count and then alphabetically,
          bypassing the configured category order entirely.
      - Impact:
        - Public maintainer pages can present category sections in a different
          order from the configured product taxonomy used elsewhere on the site.
  13. The admin surface exposes an extra per-feed `run` operation that
      contradicts the admin action contract and duplicates explicit
      `recheck` / `reprocess` semantics.
      - Spec:
        - `specs/admin-ui.md` says feed-level actions are `enable`, `disable`,
          `recheck`, and `reprocess`.
        - The same spec says there is no third feed-level action between
          `recheck` and `reprocess`, and any UI label such as "run now" for a
          feed row MUST map unambiguously to one of those two actions.
      - Implementation evidence:
        - `ui/src/components/admin/feed-modal.tsx` exposes a separate `Run`
          button.
        - `ui/src/lib/api.ts` exposes `adminRunFeed()` against
          `/api/v1/admin/feeds/{name}/run`.
        - `pkg/web/admin.go` handles `/api/v1/admin/feeds/{name}/run`.
        - `pkg/web/server.go` also exposes a second named endpoint,
          `/api/v1/admin/run/{name}`, with its own default manual-run behavior.
      - Impact:
        - Operators are given an undocumented third feed-level action, and the
          API surface duplicates that ambiguity through two overlapping routes.
  14. Unknown API paths fall through to the SPA shell and return `200` instead
      of failing as unmapped API routes.
      - Spec:
        - `specs/website.md` and `specs/admin-ui.md` define stable machine-
          readable API route families; paths outside those families are not part
          of the website/admin API contract.
      - Implementation evidence:
        - The catch-all `/` handler in `pkg/web/server.go` served the embedded
          SPA shell for any extension-less path.
        - After the per-feed admin `run` route was removed, the test request to
          `/api/v1/admin/run/sample` still returned `200` because the request
          fell through to that SPA handler instead of returning `404`.
      - Impact:
        - Clients can receive HTML success responses for unknown API paths,
          which violates the machine-readable API contract and masks routing
          mistakes as successful requests.
  15. Legacy top-level admin feed enable/disable routes remain implemented even
      though the operator contract defines the feed-level actions under
      `/api/v1/admin/feeds/{name}/...`.
      - Spec:
        - `specs/admin-ui.md` defines feed-level enable/disable as
          `POST /api/v1/admin/feeds/{name}/enable` and
          `POST /api/v1/admin/feeds/{name}/disable`.
      - Implementation evidence:
        - `pkg/web/server.go` still serves
          `/api/v1/admin/enable/{name}` and `/api/v1/admin/disable/{name}`.
        - `pkg/web/feature_test.go` still exercised those legacy routes.
      - Impact:
        - The authenticated admin API exposes duplicate route families for the
          same operation, increasing ambiguity and contract surface without
          adding operator value.
  16. Supporting documentation still describes the transitional `latest.set`
      filename as canonical even though the specs and implementation use
      `latest`.
      - Spec:
        - `specs/files-layout.md` says the canonical binary latest snapshot is
          `lib/{feed}/latest` and `latest.set` is legacy read compatibility
          only.
        - `specs/compatibility.md` says the product MAY accept `latest.set` as
          an earlier Go transitional name.
      - Implementation evidence:
        - `pkg/engine/finalize.go` writes `lib/{feed}/latest`.
        - `README.md` still said the canonical storage format is
          `lib/{name}/latest.set`.
      - Impact:
        - The repo-level operator/developer guide describes a deprecated file
          contract as canonical, which can mislead future changes and
          verification work.
  17. The `README.md` API table still documents retired admin routes and omits
      newer public route families that are now part of the stable website
      contract.
      - Spec:
        - `specs/website.md` and `specs/admin-ui.md` define the stable public
          and admin API route families.
      - Implementation evidence:
        - `README.md` still listed `/api/v1/admin/feeds/{name}/run`,
          `/api/v1/admin/run/{name}`, `/api/v1/admin/enable/{name}`, and
          `/api/v1/admin/disable/{name}`.
        - `README.md` did not list the canonical public route families for
          `/api/v1/search`, `/api/v1/countries/{code}`, `/api/v1/asns/{asn}`,
          `/api/v1/maintainers`, or `/api/v1/methodology`.
      - Impact:
        - The repo-level API guide can direct operators and developers toward
          retired routes while hiding part of the current public contract.
  18. The daemon still exposes an unauthenticated public schedule endpoint even
      though queue/schedule visibility belongs to the authenticated admin
      surface.
      - Spec:
        - `specs/website.md` says public users MUST NOT be exposed to
          operator-only queue state.
        - `specs/admin-ui.md` defines `GET /api/v1/admin/schedule` as the admin
          schedule endpoint.
      - Implementation evidence:
        - `pkg/web/server.go` serves unauthenticated `GET /api/v1/schedule`.
        - `README.md` documented `/api/v1/schedule` as a public endpoint.
        - `pkg/web/feature_test.go` exercised that unauthenticated route.
      - Impact:
        - The public API exposes operator-focused schedule state outside the
          authenticated admin contract.
  19. Public maintainer pages still reference future/unavailable maintainer
      features instead of sticking to current factual content.
      - Spec:
        - `specs/website.md` says the public site MUST NOT pad pages with
          placeholder content when the product has no fact to show.
        - `specs/homepage.md` says public presentation MUST NOT mention
          features that are not yet available.
      - Implementation evidence:
        - `ui/src/pages/maintainers-index.tsx` says richer maintainer records
          "land with a future maintainer registry."
        - `ui/src/pages/maintainer-detail.tsx` repeats the same future-feature
          framing.
      - Impact:
        - Public maintainer pages spend visible copy budget on unavailable
          future features instead of current product facts.

## Decisions

- User decision already made:
  - Create a new TODO for this effort instead of updating an existing tracker.
- Working rule for this task:
  - Treat `specs/*.md` as the normative contract.
  - When implementation behavior is reasonable but missing from specs, update
    the relevant spec first, then align code and docs.

## Plan

1. Inventory the full spec set and map each spec to owning code paths.
2. Audit implementation behavior against each spec and record concrete gaps with
   file references.
3. Fix the confirmed homepage aggregation, homepage explorer, IP lookup, and
   admin batch-action gaps.
4. Update specs where the current contracts need implementation-level
   clarification for newly added filter semantics.
5. Re-run tests, frontend build/linting, and targeted inspections.
6. Continue spec-by-spec auditing after each fix batch instead of stopping at
   the first set of discrepancies.
7. Repeat the audit/fix/verify cycle until no additional gaps surface.

## Implied Decisions

- This pass includes backend, frontend, admin surfaces, docs, and tests because
  the specs cover all of them.
- Existing generated frontend assets will not be edited directly; source changes
  will be made under `ui/` and rebuilt as needed.
- Existing unrelated work in the tree will be preserved.
- For the Zone 2 timing contract, availability will be derived from existing
  downloader-owned retained history snapshots; feeds without such retained
  evidence will continue to omit the timing fields.
- For methodology, the API contract will be normalized to structured JSON with
  stable page metadata and rendered body HTML, and the SPA will render from
  that contract instead of scraping server HTML.
- Public map topology assets will be vendored into the local static bundle and
  loaded from same-origin paths shared across all map surfaces.
- Public maintainer pages will consume the canonical maintainer endpoints
  instead of reconstructing the same views from the broader feed catalog, and
  `specs/website.md` will list those endpoints explicitly as part of the stable
  public API surface.
- `specs/website.md` will explicitly enumerate all public route families that
  back the existing country, ASN, and IP-search surfaces, not only the
  feed-scoped provider endpoints.
- The admin contract will be normalized to explicit feed-level actions only:
  `recheck`, `reprocess`, `enable`, and `disable`. Ambiguous per-feed `run`
  routes and UI affordances will be removed rather than preserved as
  undocumented aliases.
- The SPA fallback will explicitly reject unknown `/api/v1/*` paths so public
  and admin API callers never receive the HTML app shell for an unmapped API
  route.
- Legacy top-level admin feed action routes will be removed when the same
  behavior is already part of the canonical `/api/v1/admin/feeds/{name}/...`
  contract.

## Testing Requirements

- Run the relevant automated test suites after changes.
- Add or update targeted tests where a spec gap is fixed.
- Re-check API/UI/runtime behavior against the modified contracts.

## Documentation Updates Required

- Update any affected files under `specs/` immediately when behavior contracts
  change or are clarified.
- Update supporting docs such as `README.md` or migration docs if a user-facing
  or operator-facing contract changes.
