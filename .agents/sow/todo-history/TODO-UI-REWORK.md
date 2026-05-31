# TODO-UI-REWORK — Fix the new React UI

> **Purpose**: The React rewrite dropped a lot of design detail and
> deterministic signals that the previous iteration had, and lost prior
> editorial decisions. user's words:
>
> *"Generally, the agents that baked the new UI, rushed to finish and
> did not pay attention to detail. We had a long discussion about various
> deterministic signals the backend detects, which are not shown at all.
> We had also discussed a lot about the UI presentation and the order of
> information, which all have been lost — the new UI is just a mediocre
> copy of the old."*
>
> **Process rule**: research what was already discussed before asking
> user anything. Fix what has an already-stated answer. Only present
> items that need design input.

---

## TL;DR

**25 issues on the list.** After cross-referencing prior discussions in
`TODO-website.md`, `TODO-website-phase3-design.md`,
`TODO-website-phase3-impl.md`, `TODO-insights.md`, git history, and the
conversation transcripts:

- **18 are autonomous fixes** — prior decisions exist, or the fix is a
  technical correction of an obvious regression. I can do these without
  more input.
- **6 need user's input** — new design territory or explicit
  open-ended direction ("home page from scratch").
- **1 is missing** — user skipped #13 in his numbering.

2026-04-23 extension — search-surface consistency:

- user requires the feed-scoped IP search surface to use the **same
  component** the homepage uses, not a separate variant that drifts in
  layout or data rendering.
- user also requires the homepage IP lookup result rendering to show
  **ASN and country names**, not only machine identifiers / short codes.
- This is a shared-surface correctness task:
  - inspect both existing consumers of `IPSearchSurface`
  - inspect the result-row rendering path
  - unify the feed and homepage surfaces on the same component contract
  - expose human-friendly ASN / country names anywhere the homepage
    result surface currently shows only numeric/code identifiers

2026-04-24 extension — homepage inventory truthfulness:

- user reported three concrete homepage regressions:
  - the hero said `201 tracked feeds`, which was too low for the public
    inventory
  - the `IPs across feeds` metric was useless and misleading for IPv4
    because it surfaced multi-billion counts that do not help operators
  - the six explorer lens cards overflowed because they were variable-width
    and their descriptions did not wrap cleanly
- Verified root causes in the React code:
  - homepage hero and explorer were both still using the narrower
    `homepageEligible()` subset instead of the full public feed inventory
  - the hero still rendered the `IPs across feeds` tile
  - the lens strip used a horizontal scroller/flex treatment that did not
    force stable widths or description wrapping
- Fix direction for this pass:
  - homepage counts and explorer filtering must use the full public feed
    inventory exposed by the public categories/feeds APIs
  - the `IPs across feeds` metric must be removed entirely
  - the lens cards must use a wrapping grid with stable widths and wrapped
    descriptions instead of horizontal overflow

2026-04-24 extension — map navigation, feed-header fitting, and generated-feed copy:

- user reported three additional regressions / polish gaps:
  - country maps should make countries clickable
  - the feed header wraps the `Search IP` button at some widths because the
    header action area does not fit cleanly
  - generated feed headers still mention:
    - `FireHOL's update-ipset.sh` — the `.sh` suffix should be removed
    - `FireHOL's iprange` — this line is obsolete and should be removed

Verified scope before implementation:

- country maps likely affect at least:
  - feed-detail geo maps
  - homepage IP-lookup country map, if it is using the same country-shape
    interaction model
- feed-header fitting is a feed-detail hero/header layout issue, not a global
  site header issue
- generated-feed copy is backend-owned output text and must be fixed in the
  generator template / formatter, not papered over in the UI

2026-04-24 extension — overlap rows must expose peer health and stale-merge warnings:

- user requires the overlap list to state the health of each peer feed.
- user also requires a warning when a feed that is itself not `archived` or
  `unmaintained` overlaps/includes third-party upstream-merge peers that are
  currently `archived` or `unmaintained`.

Verified scope before implementation:

- per-row peer health belongs in the overlap rows/list UI, not only in the
  feed modal or methodology
- the stale-peer warning belongs in the local overlap notifications area
  because it changes how a user should interpret a high-overlap relationship
  with a merge-derived upstream feed

Implemented on 2026-04-24:

- overlap rows now join against the live public feed catalog so the table and
  inclusion lists show each peer feed's current health
- the overlap section now warns when a feed that is itself neither archived nor
  unmaintained has structural overlap with archived/unmaintained peers
- the live `/api/v1/sets/{name}/compare` fallback now restores the same
  `related` semantics as the static comparison artifact, so the warning logic
  does not disappear when the site falls back to request-time comparison

Implemented on 2026-04-24 for the same public-UI polish pass:

- country maps are now navigable to country-detail pages from both feed-detail
  geo maps and the homepage IP-lookup map
- the header IP-search surface now uses a single-row shrinking layout on feed
  pages so the `Search IP` action stays on one line at supported widths
- generated feed headers now say `Generated by FireHOL's update-ipsets` and no
  longer mention the retired `.sh` wrapper or the obsolete external
  `FireHOL's iprange` binary

2026-04-24 extension — ASN surfaces should navigate to ASN detail pages:

- user requires ASN visualizations to behave like the country maps:
  clicking an ASN in the public UI should take the user directly to that ASN's
  public detail page

Verified scope before implementation:

- IP-search results already link ASN facts to `/asns/{asn}`
- the feed-detail ASN section still renders the main treemap, bubble chart,
  and ASN tables as non-clickable SVG/text rows
- this is a navigation/consistency gap, not a new product design decision

Implemented on 2026-04-24:

- the feed-detail ASN treemap now opens `/asns/{asn}` directly from each ASN
  node
- the feed-detail ASN bubble chart now opens `/asns/{asn}` directly from each
  ASN node
- the ASN list and the critical-infrastructure ASN table now link their ASN
  column to `/asns/{asn}`

2026-04-24 extension — country and ASN detail surfaces need a real product design:

- user reported that both public classification-detail pages are too
  primitive and asked for a significantly better UX and more information,
  explicitly suggesting ideas such as:
  - a country map per ASN
  - grouped feed categories
  - richer presentation on both country and ASN pages

Verified current implementation before proposing redesign options:

- `ui/src/pages/country-detail.tsx` is currently limited to:
  - a hero with country code + provider sentence
  - a three-tile stat strip (`Feeds attributing`, `IPs across feeds`,
    `Provider`)
  - one flat feed table
  - one flat `Top ASNs in this country` table
- `ui/src/pages/asn-detail.tsx` is currently limited to:
  - a hero with ASN identity + optional infrastructure badge
  - a three-tile stat strip (`Feeds attributing`, `IPs across feeds`,
    `Provider`)
  - one flat feed table
- `ui/src/pages/maintainer-detail.tsx` is already a stronger in-repo pattern:
  - grouped feeds by category
  - clearer navigation structure
  - less "single flat table" presentation

Verified payload / backend constraints:

- `ui/src/lib/api-types.ts` exposes only thin payloads today:
  - `CountryDetailPayload`:
    - code
    - provider / asn_provider
    - totals
    - flat `feeds[]`
    - `top_asns_in_country[]`
  - `ASNDetailPayload`:
    - ASN identity
    - provider
    - totals
    - flat `feeds[]`
- Neither payload currently exposes:
  - grouped feeds by category
  - grouped feeds by maintainer
  - health rollups
  - provenance rollups
  - freshness summaries
  - country distribution for a specific ASN
  - category composition summaries
- `pkg/engine/home_detail.go` currently builds both pages from the same
  narrower aggregation policy used by homepage summary/globe:
  - `homeSummaryEligible(...)`
  - `homeGlobeHealthEligible(...)`
- That means country/ASN detail pages currently inherit the homepage's
  restricted eligibility instead of acting like broad reference/detail pages.

Critical truthfulness constraint:

- A truthful ASN country map cannot be derived from the current ASN detail
  data shape.
- `pkg/engine/home_summary.go:readTopASNs()` reads only:
  - ASN
  - name
  - count
- That is enough for top-ASN rankings, but not for "which countries this ASN
  spans inside the observed feeds".
- If user wants an ASN country map, the backend must either:
  - publish real per-ASN country-distribution data, or
  - the UI must clearly render a different, weaker semantic instead of
    pretending it is country distribution.

Pending design decisions to present before implementation:

- whether country/ASN detail pages should stay on the homepage aggregation
  subset or widen to the full public inventory
- whether this pass is allowed to expand backend detail payloads
- whether an ASN map should wait for truthful data, use a weaker explicitly
  labeled semantic, or add a new backend artifact/payload

user decisions recorded on 2026-04-24:

- `1. B` — country and ASN detail pages become full public reference pages,
  not homepage-summary subsets
- `2. B` — this pass may expand backend payloads; frontend-only polish is not
  enough
- `3. B` — add truthful ASN-country distribution data so ASN pages can render
  a real country map
- `4. B` — redesign both pages around grouped entity views and composition
  blocks, not single large flat tables

Implementation plan for this extension:

1. expand backend country / ASN detail payloads so they can support:
   - grouped feeds by category
   - category summaries
   - maintainer summaries
   - health / provenance visibility
   - ASN top-countries distribution for the real ASN map
2. update the public website spec so country and ASN detail pages have a real
   normative contract instead of merely "route exists"
3. redesign the React country page around:
   - stronger hero
   - country map
   - richer summary strip
   - composition blocks
   - feeds grouped by category
4. redesign the React ASN page around:
   - stronger hero
   - real ASN country map
   - richer summary strip
   - composition blocks
   - feeds grouped by category
5. add/update tests for backend detail payloads and build verification for the

2026-04-24 extension — country / ASN detail pages need bounded table viewports:

- user reported that the country and ASN detail pages can become
  unimaginably long because the tabular/list sections grow without limit.
- Verified current implementation:
  - `ui/src/pages/country-detail.tsx`
    - unbounded summary panel content (`Top Categories`, `Top Maintainers`,
      `Top ASNs In This Country`)
    - unbounded grouped-feeds stack (`Feeds Grouped By Category`)
  - `ui/src/pages/asn-detail.tsx`
    - unbounded summary panel content (`Top Countries`, `Top Categories`,
      `Top Maintainers`)
    - unbounded grouped-feeds stack (`Feeds Grouped By Category`)
- Required fix:
  - cap the long list/table regions in these pages
  - use scrollbars inside those bounded regions
  - keep the rest of each page stable instead of letting entity pages become
    arbitrarily tall
- Implemented on 2026-04-24:
  - both country and ASN detail pages now cap their long summary lists with
    bounded internal scroll viewports
  - both pages now wrap the grouped-by-category feed sections in bounded
    scrollable containers so the overall page height remains stable even for
    very large entities
   frontend

2026-04-24 extension — bounded entity-detail viewports need polish fixes:

- user reported four regressions after the first bounded-viewport pass:
  - page wheel scrolling gets trapped while the pointer hovers the bounded
    regions
  - grouped feed rows lost clear health visibility and their two-line text
    alignment reads wrong
  - very large attributed/feed IP counts overflow their tiles or metric cells
  - the country-detail ASN table does not show ASN names
- Implemented on 2026-04-24:
  - removed the viewport overscroll containment that was blocking normal page
    wheel chaining
  - rebuilt grouped feed rows so the health indicator and the two-line feed
    metadata align as one text block
  - hardened entity-detail stat/metric numbers against very large counts using
    wrapping-safe number treatment
  - exposed ASN names in the country-detail ASN composition rows
  - fixed the broken health-dot rendering path by introducing a real
    color-value helper for dot indicators instead of reusing Tailwind text
    class names as inline CSS colors

Implemented on 2026-04-24:

- backend detail payloads widened in `pkg/engine/home_detail.go` so country
  and ASN detail pages now expose:
  - grouped feeds by category
  - category summaries
  - maintainer summaries
  - richer totals
  - recent change timestamps per feed row
  - truthful ASN-country distribution for ASN pages
- added `pkg/engine/home_detail_helpers.go` to keep the new entity-specific
  composition logic separate from the route builders:
  - `detailSurfaceEligible(...)` for public reference-page eligibility
  - `countryFilteredRangeSource(...)` for country-specific canonical
    intersections
  - `countCountriesForASNSource(...)` for ASN-specific geo distribution
- removed the old homepage-summary restriction from country / ASN detail
  builders:
  - these pages now include any public feed that currently contributes to the
    selected entity, including stale / derived public contributors
- corrected the old truthfulness bug on the country page:
  - `Top ASNs in this country` is now rebuilt from canonical feed ranges
    intersected with the selected country and the active ASN provider
  - it no longer reuses whole-feed top-ASN rows and pretends they belong to
    the selected country
- added a real ASN country map data path:
  - ASN detail now computes country distribution from the canonical feed
    bodies intersected with the selected ASN and the active geo provider
  - it is not inferred from homepage aggregates or feed-level country tables
- redesigned the React country page into a reference page with:
  - stronger hero
  - country outline
  - richer summary strip
  - top categories / maintainers / country-specific ASNs
  - feeds grouped by category with visible health / provenance / maintainer
- redesigned the React ASN page into a reference page with:
  - stronger hero
  - real ASN country map
  - richer summary strip
  - top countries / categories / maintainers
  - feeds grouped by category with visible health / provenance / maintainer
- added backend regression coverage in `pkg/engine/home_detail_test.go` for:
  - inclusion of older/stale public feeds on detail pages
  - truthful country-specific ASN composition
  - truthful ASN-country distribution
- updated the public contract / methodology docs to match:
  - `specs/website.md`
  - `pkg/web/static/methodology/asn-attribution.md`
  - `pkg/web/static/methodology/geographic-distribution.md`

Verification:

- `go test ./pkg/engine`
- `go test ./pkg/web`
- `pnpm --dir ui build`

2026-04-24 follow-up — homepage lens strip width / row count:

- user clarified that the six homepage lens tiles should not become merely
  "a bit narrower" or remain a wrapped multi-row desktop grid.
- Required behavior:
  - at full-width desktop layout, all six lens tiles must fit in a single row
  - text must still wrap cleanly inside each tile
  - the fix is a homepage explorer layout adjustment, not a change to the
    lens semantics or filtering behavior

Implementation direction:

- tighten the per-tile horizontal chrome enough to reduce each card width
- switch the large-screen grid from 3 columns to 6 columns so the strip
  becomes one row on full-width desktop
- keep smaller breakpoints wrapped so the layout still degrades naturally on
  narrower screens

2026-04-24 follow-up — homepage `By maintainer` ordering:

- user requires the homepage explorer's `By maintainer` view to be
  alphabetical.
- Verified current behavior in
  `ui/src/components/home/home-explorer-view-maintainers.tsx`:
  - maintainer groups are currently sorted by descending feed count first
  - maintainer name is only a secondary tie-breaker
- Required change:
  - maintainer groups must be ordered alphabetically by maintainer name
  - this is only a group-ordering change; the feed cards inside each
    maintainer group can keep their current local ordering

2026-04-24 follow-up — homepage lens tiles must unselect after manual changes:

- user requires the homepage lens tiles to unselect once the user manually
  changes the explorer state.
- Verified current behavior:
  - the explorer currently preserves `state.lens` across manual filter / sort /
    view changes
  - this leaves the tile highlighted even when the live explorer state no
    longer matches that preset
- Required change:
  - the lens highlight must remain active only while the current explorer state
    still matches the selected lens preset
  - once the user manually diverges from that preset, the lens selection must
    clear automatically

Implemented on 2026-04-24:

- explorer state normalization now clears `lens` whenever the live explorer
  state no longer matches the selected lens preset:
  - `ui/src/lib/explorer-state.ts`
- the homepage spec now makes that behavior normative:
  - `specs/homepage.md`

2026-04-23 extension — hero background readability:

- user confirmed the right-column background evolution chart placement is
  now correct.
- New issue:
  - the four small hero tiles (`Frequency`, `Health`, `Updated`,
    `IP version`) need some transparency / glass treatment so the chart
    behind them does not look visually broken
  - the large red `Unique IPs tracked` number can clash with the red
    evolution graph behind it
- This is now a small visual-design decision on the foreground treatment
  of the right hero column.
- user decision:
  - **A selected** for the four small hero tiles:
    use a frosted / glass treatment so the background chart remains
    visible without the tiles reading like broken solid blocks
  - **A selected** for the large `Unique IPs tracked` number:
    switch it to a high-contrast neutral foreground instead of the same
    red accent the background chart uses
- Additional follow-up requirement:
  - the `Unique IPs tracked` value itself must scale to the width of the
    right-column stat area so it never clips on narrower widths or long
    formatted numbers
  - the hero evolution status headline must insert a line break before
    the `N-month range:` fragment so long range labels do not crowd the
    line

The dominant pattern is regression: the React rewrite threw away the
Phase 3 design spec that was already locked. Half the complaints map
directly to sections that were fully specced in
`TODO-website-phase3-design.md` but never made it into the React build.

### 2026-04-23 extension — chart interpretation contract

user decision:

- **A selected**: do **not** clone the old bash charts 1:1.
- Instead, extract the **truthfulness / interpretation-safety
  principles** the old site applied to charts, write them into the
  website specs, and apply them to **all** charts in the React site,
  including newer charts that never existed in bash.

Verified evidence from the legacy-vs-current chart audit:

- Legacy age/retention charts explicitly disclosed **partial /
  incomplete** data and adjusted the wording/math around that:
  - `/home/user/src/firehol/firehol/html/ipsets/index.html:1179-1183`
  - `/home/user/src/firehol/firehol/html/ipsets/index.html:1266-1273`
  - `/home/user/src/firehol/firehol/html/ipsets/index.html:1284-1286`
- Current React retention UI does not expose those semantics in its type
  contract or rendering path:
  - `ui/src/lib/api-types.ts:330-338`
  - `ui/src/components/feed-detail/section-retention.tsx:141-214`
- Legacy age chart aged the histogram forward to **now** using the
  artifact timestamp and warned when the client clock looked wrong:
  - `/home/user/src/firehol/firehol/html/ipsets/index.html:1168-1176`
  - `/home/user/src/firehol/firehol/html/ipsets/index.html:294-298`
- Current React UI does not expose `updated` for this chart and does not
  perform any equivalent "as of now" handling:
  - `ui/src/lib/api-types.ts:330-338`
- Legacy rendered zero-hour buckets as **`< 1`**, preventing false
  precision:
  - `/home/user/src/firehol/firehol/html/ipsets/index.html:1185`
  - `/home/user/src/firehol/firehol/html/ipsets/index.html:1274`
- Current React chart clamps `0` to `1` for log rendering and then
  labels that bucket as `1h`, which can mislead:
  - `ui/src/components/feed-detail/section-retention.tsx:193-196`
  - `ui/src/components/feed-detail/section-retention.tsx:261-290`
- Legacy charts used specific explanatory boxes for **empty**, **cannot
  load**, **not enough history**, and **provider has no overlap** cases.
  Current React UI often collapses these into generic empty states or
  hides the provider/chart entirely:
  - legacy:
    - `/home/user/src/firehol/firehol/html/ipsets/index.html:1159`
    - `/home/user/src/firehol/firehol/html/ipsets/index.html:1263`
    - `/home/user/src/firehol/firehol/html/ipsets/index.html:1357-1358`
    - `/home/user/src/firehol/firehol/html/ipsets/index.html:1510-1524`
  - current:
    - `ui/src/components/feed-detail/section-retention.tsx:54-55`
    - `ui/src/components/feed-detail/section-retention.tsx:107-120`
    - `ui/src/components/feed-detail/section-behavior.tsx:328-333`
    - `ui/src/components/feed-detail/section-geo.tsx:77-102`

Implication for this TODO:

- Add a **chart interpretation contract** to the specs before further UI
  work.
- Audit every current chart against that contract, not just the charts
  that existed in bash.
- Restore missing semantics even when the final React visualization is
  different from the legacy chart.

### 2026-04-23 extension — website spec gap

Current normative spec coverage in `specs/website.md` is too broad for
chart truthfulness. It requires methodology, factual presentation, and
graceful degradation, but it does **not** yet define:

- the chart-state model (`loading`, `load failed`, `empty but valid`,
  `not enough history`, `partial/incomplete`, `not applicable`,
  `fully available`)
- the obligation to disclose **coverage / denominator / exclusions**
- the obligation to disclose **time anchor** ("as of artifact time" vs
  "aged to now")
- precision rules for display buckets like **`< 1h`**
- provider-visibility rules when one configured provider has no usable
  payload
- when an explanatory info box must replace a misleading chart
- when legends / scales / units are required to avoid visual ambiguity

Verified from the current codebase:

- `specs/website.md:274-307` defines methodology and graceful
  degradation, but not the chart-state or interpretation contract.
- The current React site contains chart/visual surfaces beyond the bash
  site, so a legacy-parity spec would be too narrow:
  - feed detail:
    - `ui/src/components/feed-detail/section-asn.tsx`
    - `ui/src/components/feed-detail/section-geo.tsx`
    - `ui/src/components/feed-detail/section-behavior.tsx`
    - `ui/src/components/feed-detail/section-retention.tsx`
    - `ui/src/components/feed-detail/section-comparison.tsx`
  - feed-detail visuals:
    - `asn-treemap.tsx`, `asn-bubble-chart.tsx`, `geo-map.tsx`,
      `overlap-sankey.tsx`, `overlap-network.tsx`
  - homepage visuals:
    - `home-explorer-view-treemap.tsx`
    - `home-explorer-view-timeline.tsx`
    - `ip-lookup-country-map.tsx`

Implication:

- The missing contract belongs in the **website spec**, with chart-wide
  rules that every public visualization must follow.
- Feed-specific metric nuances stay in the methodology pages and
  feed-specific specs.

### 2026-04-23 extension — chart-by-chart gap matrix

Scope of this audit pass:

- Included live public visualizations:
  - feed detail: retention, behaviour, geo, ASN, overlap
  - homepage explorer: treemap, timeline
  - homepage IP lookup: country map
- Excluded from this pass:
  - `home-globe-panel.tsx` / `home-globe-scene.tsx` because
    `HomePage` does not mount them today (`ui/src/pages/home.tsx:44-60`)
  - country / ASN / maintainer detail pages because they are currently
    tables/text surfaces, not chart surfaces

Verified gaps:

1. **Retention / freshness — critical**
   - Distinct states are collapsed:
     - fetch error -> `RetentionEmptyState`
     - no `current` and no `past` -> same `RetentionEmptyState`
     - empty histogram inside the chart -> separate "Not enough data yet"
     - Evidence:
       - `ui/src/components/feed-detail/section-retention.tsx:54-63`
       - `ui/src/components/feed-detail/section-retention.tsx:216-220`
   - The React type contract dropped backend fields needed for truthful
     interpretation:
     - backend payload includes `started`, `updated`, `incomplete`, and
       per-series `total`
     - frontend only keeps `hours` and `ips`
     - Evidence:
       - `pkg/engine/engine.go:100-112`
       - `pkg/engine/retention.go:268-289`
       - `ui/src/lib/api-types.ts:330-338`
   - Rendering convenience changes visible meaning:
     - zero-hour bucket is clamped to `1` for the log axis and then shown
       as `1h`
     - Evidence:
       - `ui/src/components/feed-detail/section-retention.tsx:193-196`
       - `ui/src/components/feed-detail/section-retention.tsx:261-290`
   - Required fix shape:
     - restore the dropped retention metadata in TS/API types
     - separate `load failed`, `empty`, `not enough history`, and
       `partial/incomplete`
     - restore truthful `< 1h` labeling
     - disclose whether freshness is "as of artifact time" or aged to now

2. **Behaviour charts — high**
   - History / changeset failures degrade into generic empty states:
     - `parseHistoryCSV(historyQuery.data)` and `changesetsQuery.data ?? []`
       lose the error distinction because the render path only checks
       loading vs data length
     - Evidence:
       - `ui/src/components/feed-detail/section-behavior.tsx:70-84`
       - `ui/src/components/feed-detail/section-behavior.tsx:177-183`
       - `ui/src/components/feed-detail/section-behavior.tsx:286-333`
   - The cadence chart silently drops intervals longer than 7 days:
     - `if (dt > 0 && dt < 1440 * 7) intervals.push(dt);`
     - This contradicts the methodology, which explicitly says
       rarely-changing feeds should naturally report long cadence values
     - Evidence:
       - `ui/src/components/feed-detail/section-behavior.tsx:236-241`
       - `pkg/web/static/methodology/update-cadence.md:66-87`
   - Required fix shape:
     - separate `failed to load` from `not enough history`
     - either stop truncating long intervals or disclose the truncation
       locally and in methodology

3. **Geographic distribution — high**
   - Configured providers are silently hidden when their payload is
     missing/empty because the UI filters to `aliveProviders`
     - Evidence:
       - `ui/src/components/feed-detail/section-geo.tsx:77-81`
       - `ui/src/components/feed-detail/section-geo.tsx:91-102`
   - The choropleth uses a sqrt colour scale with no visible legend or
     local scale explanation
     - Evidence:
       - `ui/src/components/feed-detail/geo-map.tsx:132-145`
       - `ui/src/components/feed-detail/geo-map.tsx:264-279`
   - Required fix shape:
     - preserve all configured providers in the tab strip
     - show provider-local state messages for empty/malformed/no-overlap
     - add a visible scale/legend or equally strong local explanation

4. **ASN composition — high**
   - Configured ASN providers are also filtered to `aliveProviders`,
     hiding provider-local failures/empties
     - Evidence:
       - `ui/src/components/feed-detail/section-asn.tsx:49-53`
       - `ui/src/components/feed-detail/section-asn.tsx:66-77`
   - Visual views are silently partial:
     - treemap shows only top 80 ASNs
     - bubble chart shows only top 60 ASNs
     - UI does not disclose that either visual is truncated
     - Evidence:
       - `ui/src/components/feed-detail/asn-treemap.tsx:90-92`
       - `ui/src/components/feed-detail/asn-bubble-chart.tsx:67-69`
   - Required fix shape:
     - keep all configured providers visible with provider-local states
     - disclose top-N truncation locally, or remove the truncation, or
       make the views explicitly "top-N" views by label

5. **Overlap — high**
   - Comparison fetch errors collapse into the same outcome as "no rows"
     because the UI uses `compareQuery.data ?? EMPTY_COMPARISON_ROWS`
     and only branches on loading vs row count
     - Evidence:
       - `ui/src/components/feed-detail/section-comparison.tsx:49-55`
       - `ui/src/components/feed-detail/section-comparison.tsx:131-137`
   - Sankey and network visualizations are silently truncated to the top
     overlaps
     - sankey: `topN={14}`
     - network: `topN={24}`
     - Evidence:
       - `ui/src/components/feed-detail/section-comparison.tsx:240-256`
       - `ui/src/components/feed-detail/overlap-sankey.tsx:99-122`
       - `ui/src/components/feed-detail/overlap-network.tsx:111-135`
   - Required fix shape:
     - separate `load failed` from `no comparison data`
     - label the visual views as top-N, or disclose the truncation in
       local copy

6. **Homepage explorer timeline — medium**
   - The timeline buckets feeds by `source_date || processed_date ||
     checked_date` against `Date.now()`, but the UI never explains this
     timestamp fallback chain
     - Evidence:
       - `ui/src/components/home/home-explorer-view-timeline.tsx:20-31`
       - `ui/src/components/home/home-explorer-view-timeline.tsx:34-43`
   - Required fix shape:
     - local caption or methodology link explaining the time anchor and
       fallback source used for this view

7. **Homepage explorer treemap — medium**
   - The treemap area encodes `unique_ips`, but the UI does not explain
     that locally when the visitor switches to the Treemap view
     - Evidence:
       - `ui/src/components/home/home-explorer-view-treemap.tsx:44-53`
       - `ui/src/components/home/home-explorer-view-switcher.tsx:12-18`
   - Required fix shape:
     - add a one-line local explanation that tile area = unique IPs and
       category colour comes from configuration

8. **Homepage IP lookup country map — mostly aligned**
   - This surface already distinguishes:
     - no geographic result
     - topology load failure
     - topology loading
     - successful highlight
   - Evidence:
     - `ui/src/components/home/ip-lookup-country-map.tsx:19-40`
   - Remaining issue is mainly theming (hardcoded colours), not
     interpretation truthfulness.

Non-gap notes from this pass:

- The old dual-series evolution chart (`Entries` + `Unique IPs`) is **not**
  automatically a required re-add. The current methodology already defines
  the public evolution chart as unique IP count only:
  - `pkg/web/static/methodology/evolution.md:71-75`
- The issue to fix is not "difference from bash". The issue is
  "difference from the truthfulness contract".

---

## Key findings from prior-discussion research

### Navy palette was already decided (not new)
Commit `c45497f` on 2026-04-05 locked a "cohesive navy palette":
- `--bg: #101520` (deep navy)
- `--bg-surface: #171d2b` (lifted navy)
- `--bg-surface-alt: #1d2435`
- `--bg-inset: #141a26`
- `--bg-elevated: #212940`

Earlier conversations (April 5) referenced even darker navy `#080b12`
inspired by HackTheBox `#0f1623`.

The signature accent is a **red-to-blue gradient** ("police line"),
used sparingly. Two variants in history:
- Newer Alpine: `linear-gradient(90deg, #dc2626, #7c3aed, #2563eb)`
- Older brief: `linear-gradient(90deg, #ec0000, #0024ff)`

The React rewrite threw all this out and replaced it with cream
(`#fafafa`) + a single solid red `#dc2626`. user's complaint #2
("background should be navy") is a **regression**, not a new ask.

**Quality reference**: HackTheBox. user quote: *"refined dark theme,
clean cards, minimal noise, one signature accent."*

### "Luxury Apple product page" editorial language (TODO-website-phase3-design.md)
Every aspect of the feed detail page was designed in concrete pixel
values in a 1175-line spec:
- 2-column hero (7/5 split): category strip → name → tagline → CTAs on
  left, cinematic all-time evolution area chart on right
- `--font-size-display-lg: 7rem` (112px) for hero title
- **Big primary button CTAs** in the hero (P4 decision: "big primary
  button — 'Download list' for redistributable feeds")
- Vitals strip with **sparklines as background texture**, number
  tick-up animation
- Composition grid: Geo + ASN + Bogons side-by-side, with
  **multi-view tabs** framework (table/bubble/treemap/etc.)
- **Data freshness** as a distinct section AND as a vital card
- **Retention** with distinct `removed-age p75` histogram + Kaplan-Meier
  survival curve + multi-view tabs
- **Comparison** with horizontal scroll strip + table + sankey +
  **force graph network** as multi-tab visualizations
- **Tech specs table**: 6 groups (Identification/Data/Updates/Access/
  Processing/Maintainer) with ~30 fields
- **Insights callout** ("What we noticed") with 16 deterministic rules
  from `TODO-insights.md`, each linked to its methodology page

### Home page was also designed (TODO-website.md + earlier transcripts)
The old Alpine site had a storefront layout:
- Hero: **IP search bar** ("Is this IP a known threat?") + live-map
  background + stats strip
- Guided paths: PROTECT / REAL-TIME / RESEARCH
- Curated rows (horizontal scroll): Editor's Picks, Most Active,
  Largest Coverage, Recently Added
- Browse by threat type (category rows)
- **Research groups section** (maintainer spotlight cards) — this is
  complaint #20, already designed
- Community (most discussed)

The old Alpine site also had `globe.gl` 3D globe on the hero — user
complaint #5 is a reference to that.

### ASN provider order: MaxMind first by design
`TODO-website.md` decision 2: *"ASN database — first provider: MaxMind
GeoLite2-ASN (we already have credentials from geo work)"*. `TODO-asn-providers.md`
adds CAIDA, iptoasn, DB-IP as additional providers. Tab order is YAML
declaration order (verified in `pkg/engine/public.go:143`).

Current YAML has `caida_prefix2as:` at line 354 (before
`maxmind_geolite2_asn:` at line 1977 by alphabetical order of the
source block). CAIDA has **no ASN names**, only numbers (verified in
TODO-asn-providers.md) — user's complaint #9 is factually correct.

### Two independent bugs in insights wiring (complaint #7)
1. Backend returns `{name, computed, items: [...]}`. Frontend's
   `api.getInsights` is typed as `Promise<Insight[]>` and returns the
   raw envelope — `insightsQuery.data` is the envelope object, not the
   array. `.length === 0` is `undefined === 0` → false. `.length > 0`
   is `undefined > 0` → false. Grid never renders. Empty state.
2. Frontend type `InsightSection` is completely wrong:
   `"feed_health" | "data_quality" | "asn_attribution" | ...`. Backend
   serializes lowercase enums: `"overview" | "composition" |
   "retention" | "trends" | "relationships" | "freshness"`. The values
   simply don't match.

Both are obvious autonomous fixes.

### Retention and data freshness: not deleted, just not wired
- Backend still produces `{name}_retention.json` with `current`
  (currently-listed age histogram) and `past` (removed-IP age
  histogram).
- `ui/src/lib/api.ts:150` has `getRetention()`.
- `ui/src/lib/api-types.ts:225` has `RetentionData` type.
- **No component renders it.** No `SectionRetention`, no
  `SectionFreshness`. The data flows but stops at the API client.

### Specs regression
Original Phase 3 design had an exhaustive 6-group spec sheet, but user
later explicitly pruned the public specs surface to **3 groups only**
(`d049979`: Identification / Data / Updates; Access + Processing were
removed on purpose as public-reader noise). So the remaining work here
is **not** "bring back the 6-group table".

Current `section-specs.tsx` already exposes a richer 3-group surface,
including health thresholds and timestamps, but it still does not
surface all of the remaining public facts already present in
`FeedMetadata`.

Verified current omissions / drift:
- `FeedMetadata` exposes `provenance` and `attribution`, but
  `section-specs.tsx` never renders them:
  - `ui/src/lib/api-types.ts:98-104`
  - `ui/src/components/feed-detail/section-specs.tsx`
- `section-specs.tsx` still carries stale internal guidance saying the
  old 6-group layout should not come back, but the active TODO item
  below still describes #16 as "full 6-group / ~30-field layout". Those
  two documents contradict each other.

Remaining autonomous fix shape:
- enrich the **existing 3-group** public spec sheet with the missing
  public facts already published by the daemon
- keep Access / Processing out of the public spec table unless user
  changes that decision
- update the normative website spec so the public technical-specs
  contract matches the 3-group implementation

Progress on 2026-04-23:
- `ui/src/components/feed-detail/section-specs.tsx`
  - added the missing public provenance facts already in
    `FeedMetadata`: provenance / lineage, attribution, commit-history
    link when published, and truthful raw-URL availability wording
- `specs/website.md`
  - added a normative feed-detail technical-specifications contract so
    the spec matches the public 3-group implementation
- verification:
  - `pnpm --dir ui build`
  - `git diff --check`

### Charts use hard-coded colors, no theme wiring
Every chart in `ui/src/components/feed-detail/`:
- `asn-bubble-chart.tsx`: `const ACCENT = "#dc2626"` hardcoded
- `geo-map.tsx`: `range(["#fdebec", "#dc2626"])` hardcoded, wrapped in
  `bg-card border-border` box
- `section-behavior.tsx`: `const ACCENT = "#dc2626"; const GRID =
  "rgba(0,0,0,0.06)"; const AXIS = "#737373";` hardcoded

None of these respond to `.dark` class. Tooltip styling is Recharts
default (white background). All autonomous fixes.

### Behavior charts use `Math.abs` on the delta
`section-behavior.tsx:223`:
```ts
const churn = prev && prev.ips > 0
  ? Math.abs(ips - prev.ips) / prev.ips
  : null;
```
user's complaint #11: net delta can be 0 while the list is completely
replaced. The `Math.abs` hides the sign. Need two dimensions: signed
delta (net change) and churn (added + removed, as a share of current).

---

## Categorization

Legend:
- **[A]** Autonomous fix — prior decision exists or technical correction
- **[D]** Needs user's design input

| #  | Issue | Class | Effort | Evidence |
|----|-------|-------|--------|----------|
| 1  | Soften square edges — use generous radii (sm=4, md=8, lg=12, xl=16) like the old design | **A** | S | Old `--radius-sm: 4px; --radius-md: 8px; --radius-lg: 12px; --radius-xl: 16px` (git show of pre-React app.css). New React UI uses 4/2/0. Regression. |
| 2  | Navy background | **A** | M | Commit `c45497f` locked the navy palette. React rewrite replaced with cream. Regression. |
| 3  | Dark/light theme on maps, charts | **A** | M | Hard-coded hex in `asn-bubble-chart.tsx`, `geo-map.tsx`, `section-behavior.tsx`. No CSS-var wiring. |
| 4  | Sticky feed name while scrolling | **A** | S | Standard UX pattern. Add sticky header showing feed name once hero scrolls out of view. |
| 5  | Home page redesign from scratch | **D** | XL | user explicitly: *"put it as a separate TODO item"*. Prior storefront spec exists but is stale (globe.gl, Alpine). **Split to `TODO-HOMEPAGE.md`.** Needs direction: how much of old storefront to keep, globe or different hero, what stats live in the pulse strip. |
| 6  | Methodology page: intro + presentation | **D** | M | Prior spec left this as "may polish later". Needs direction on grouping (by section? by category? flat list?), intro copy, whether each rule gets a teaser card or just a link. |
| 7  | "What the data says" always empty | **A** | S | Two bugs: (a) `api.getInsights` doesn't extract `.items` from envelope, (b) `InsightSection` type has wrong values. Both trivial to fix. |
| 8  | List tables: scrollable, sortable, search, export | **A** | M | Use TanStack Table v8 (already in deps). Replace Top-25 truncation with full sortable table with search filter + CSV/JSON export. |
| 9  | MaxMind first, CAIDA not first | **A** | S | Reorder `sources:` block in `configs/firehol.yaml` so `maxmind_geolite2_asn` precedes `caida_prefix2as`. YAML order is the tab order. |
| 10 | Chart tooltip theme | **A** | S | Part of task #3. Recharts `<Tooltip contentStyle={...}/>` using CSS vars. |
| 11 | Per-Update Delta: signed + churn, two dimensions | **A** | S | Fix `Math.abs` in `section-behavior.tsx:223`. Show signed delta AND separate churn metric. |
| 12 | Retention completely lost | **A** | M | Phase 3 spec exists. Data + API wired. Add `SectionRetention` with `age histogram + p75/p90/p100 marks` and `survival curve` from `_retention.json`. |
| 14 | Data freshness completely lost | **A** | M | Same as 12 — data exists in `_retention.json`'s `current` histogram. Add a vital card + section. |
| 15 | Overlaps: INCLUDED IN / INCLUDED tiles + scrollable table + sankey + network | **A** | L | Phase 3 spec has "horizontal scroll strip + uniqueness callout" + multi-view tabs (table/sankey/force-graph/chord). Data in `_comparison.json`. Needs `SectionComparison` rebuild. |
| 16 | Specs richer | **A** | M | user explicitly pruned the public table to 3 groups in commit `d049979`. The remaining work is to enrich those 3 groups with the missing public facts already in `FeedMetadata` (`provenance`, `attribution`, availability/status wording, etc.), not to restore Access/Processing. |
| 17 | Global search: IPs + researchers + lists | **D** | M | Backend has `/api/v1/query?ip=...` and catalog has name list. **No researcher index exists.** Needs direction on: unified single input or 3 modes? Should hit-type icons be shown? |
| 18 | Hero: evolution chart (background or right column) | **A**/**D** | M | Prior P1 decision: right-column cinematic chart. user now says *"it can be a background"*. **Autonomous to restore right-column version** (matches spec); needs input only if user prefers the background treatment. |
| 19 | Hero CTAs too small | **A** | S | Restore P4 decision: "big primary button in the hero" for Download. Current impl uses thin ghost links. |
| 20 | Research lab listing with contact details | **A** | L | Phase 4 deferred from TODO-website.md (*"/maintainers/<name> listing all their feeds, contact details"*). Data source: `all-ipsets.json` has `maintainer` + `maintainer_url` per feed. Group by maintainer, render list + per-maintainer page. |
| 21 | Composition: treemap default with AS number + name | **A** | M | Current default is table. Spec had bubble pack default. user now overrides to treemap default. Treemap via `@visx/hierarchy` (already in deps). Labels: `AS{num} {name}`. |
| 22 | Critical infrastructure standout | **A** | S | Current `CriticalInfraBlock` is a subsection. Promote to a top-level block with accent framing, bigger numbers, always visible (not behind a toggle). |
| 23 | Geo map: transparent, themed for dark | **A** | S | `geo-map.tsx` wraps in `bg-card border-border`. Remove box. Swap hardcoded cream→red gradient for CSS-var driven (`--chart-accent-soft` → `--chart-accent`). |
| 24 | Admin: scrollable tables not paged | **A** | S | `ui/src/pages/admin.tsx` — switch from paged to virtual scroll. TanStack Table supports both. |
| 25 | Admin: per-feed detail view + error logs + actions | **D** | L | Backend has `/api/v1/admin/feeds/{name}/recheck` and `/reprocess`. No `/disable` exists (would need to be added). No error logs exposed via API today. Needs (a) new backend endpoints for disable + error logs, (b) per-feed detail panel in admin UI. |
| 26 | **NEW**: ASN provider tabs use ugly internal codes | **A** | S | user quote (April 7): *"the provider names of ASN selection are ugly internal codes, not friendly provider names, unlike the geo selection."* — `section-asn.tsx` and `section-geo.tsx` should render `provider.label` (already in YAML) instead of `provider.name`. |
| 27 | **NEW (verify)**: No placeholder/random values anywhere | **A** | S | user April 5: *"interesting. you have put random numbers everywhere. Nice best practice."* — audit React UI for hard-coded sample values that fall back to fake data. Replace with real "—" or "no data" treatment. |
| 28 | **NEW**: Cross-provider radar viz broken | **D** | M | user quote (April 7): *"Cross-provider radar... we have 4 configured and it does not work."* — Phase 3 spec listed `Cross-provider radar (when ≥2 ASN sources configured)` as Tab 3 of ASN section. Current React UI has no radar tab at all. Needs decision: (a) drop the radar entirely, (b) build it, (c) defer. |
| 29 | **NEW (backend bug)**: ASN data integrity | **D** | ? | user: *"I see an IP feed with 5k entries, and the ASN table reports hundreds of thousands of IPs."* — backend ASN attribution is over-counting somewhere. Not in the original list, but worth flagging. May have been fixed since; verify on a small feed. |

**Autonomous fixes**: 1, 2, 3, 4, 7, 8, 9, 10, 11, 12, 14, 15, 16, 19, 21, 22, 23, 24, 26, 27 → **20 items**

**Needs user input**: 5, 6, 17, 18, 20, 25, 28, 29 → **8 items**

---

## Implementation order (for the autonomous items)

Group by dependency and theme, smallest-first within each group:

### 2026-04-23 workstream — chart interpretation safety

1. Define a **chart interpretation contract** in `specs/website.md`
   covering:
   - chart-state semantics
   - coverage / exclusions / denominator disclosure
   - time-anchor disclosure
   - precision / approximation rules
   - provider visibility rules
   - legend / scale / unit rules
   - when info boxes replace charts
2. Cross-check `specs/homepage.md` and feed-detail expectations so the
   chart contract applies consistently to homepage visuals and feed
   detail analytics.
3. Audit every current chart/visual component against that contract and
   record the gap matrix with code evidence.
4. Only after the contract is explicit, implement UI/API/spec fixes.

### 2026-04-23 decisions pending — chart safety implementation

These are the remaining product choices that are not purely technical.

1. **Top-N visual truncation policy**
   - Evidence:
     - ASN treemap truncates to top 80:
       `ui/src/components/feed-detail/asn-treemap.tsx:90-92`
     - ASN bubble truncates to top 60:
       `ui/src/components/feed-detail/asn-bubble-chart.tsx:67-69`
     - overlap sankey uses top 14:
       `ui/src/components/feed-detail/section-comparison.tsx:240-246`
     - overlap network uses top 24:
       `ui/src/components/feed-detail/section-comparison.tsx:249-255`
   - Options:
     - `A`: keep the truncation, but label each visual explicitly as top-N
       and point to the full table/list for the complete record
     - `B`: remove the truncation and attempt to render the full dataset
     - `C`: aggregate the tail into an explicit `Other` bucket / group
   - Decision: `A`
     - keep the truncation, but label each visual explicitly as top-N and
       point to the full table/list for the complete record
   - Reason:
     - keeps the visuals readable and performant while making the
       partiality truthful

2. **Cadence chart long-interval policy**
   - Evidence:
     - current UI drops intervals > 7 days:
       `ui/src/components/feed-detail/section-behavior.tsx:236-241`
     - methodology says rarely-changing feeds should naturally report
       long cadence values:
       `pkg/web/static/methodology/update-cadence.md:86-87`
   - Options:
     - `A`: remove the 7-day cutoff and let the chart show the full
       observed cadence range
     - `B`: keep the cutoff but disclose it locally and in methodology
     - `C`: redesign the cadence view to a log/banded chart that includes
       the long tail cleanly
   - Decision: `A`
     - remove the 7-day cutoff and let the chart show the full observed
       cadence range
   - Reason:
     - it is the smallest truthful fix and removes a direct contract
       contradiction

3. **Freshness time-anchor policy**
   - Evidence:
     - legacy site aged the current histogram forward to now and warned
       on client-clock issues:
       `/home/user/src/firehol/firehol/html/ipsets/index.html:1168-1176`
       `/home/user/src/firehol/firehol/html/ipsets/index.html:294-298`
     - backend still exposes the timestamp needed for this:
       `pkg/engine/engine.go:105-112`
     - current React type drops it:
       `ui/src/lib/api-types.ts:330-338`
   - Options:
     - `A`: restore the legacy "aged to now" behavior and show a local
       client-clock warning when relevant
     - `B`: keep artifact-time values and label them explicitly as "as of
       last publication"
     - `C`: show both artifact-time and aged-to-now values
   - Decision: `A`
     - restore the legacy "aged to now" behavior and show a local
       client-clock warning when relevant
   - Reason:
     - for a freshness view this is the most truthful reading of
       "how old are the currently listed IPs right now"

### Wave 1 — Theme foundation (changes touch every component)
**Goal: before any section rewrites, fix the base design tokens so every subsequent rebuild inherits correct colors/radii/theme.**

1. **#1 Soften radii** (`ui/src/index.css`) — bump `--radius` to `0.625rem` (10px) base, keep the shadcn derivative pattern
2. **#2 Navy palette** — add navy-first dark tokens matching `c45497f` commit
3. **#3 + #10 Chart theme wiring** — replace all hardcoded hex in charts with CSS vars; wire Recharts `contentStyle`, XAxis/YAxis `stroke`, `CartesianGrid stroke` to theme tokens
4. **#23 Geo map transparent + dark** — remove the bordered card, switch to CSS-var-driven colors

### Wave 2 — Structural feed detail rebuild
**Goal: restore the Phase 3 spec.**

5. **#7 Insights envelope + section type bugs** — tiny edits in `api.ts` and `api-types.ts`. Unblocks "What the data says".
6. **#9 MaxMind before CAIDA** — YAML reorder, one commit
7. **#4 Sticky feed name** — subheader appearing after hero scrolls out
8. **#16 Specs richer** — extend `section-specs.tsx` within the current 3-group public contract; add the missing public rows already in `FeedMetadata` and update the website spec accordingly
9. **#19 Hero CTAs bigger** — restore big primary button
10. **#18 Hero evolution chart restored** — add uPlot or Recharts area chart in the right column
11. **#12 Retention section** — build from `_retention.json` with the spec's p75/p90/p100 age histogram
12. **#14 Data freshness** — vital card + section
13. **#11 Signed delta + churn** — fix `Math.abs`, show both dimensions

### Wave 3 — Composition and comparison
14. **#21 Treemap default for ASN** — new `asn-treemap.tsx`, wire into `section-asn.tsx` as default tab
15. **#22 Critical infrastructure standout** — promote the block, accent framing
16. **#8 List tables** — replace Top-25 with full sortable/searchable/exportable table (affects ASN/geo sections)
17. **#15 Overlap rebuild** — tiles (INCLUDED IN / INCLUDED), scrollable table, sankey, force-graph

### Wave 4 — Admin
18. **#24 Admin scrollable tables** — TanStack virtual scroll

Each wave ends in a commit.

### 2026-04-23 progress — hero evolution restored

Completed:

- `ui/src/components/feed-detail/hero.tsx`
  - added an all-time hero evolution chart using the existing
    `["history", feedName]` query and the published history CSV
  - added explicit local states for loading, load failure, and not
    enough history yet
  - added a factual headline under the curve:
    current IP count + observed time-span range + observed min/max IPs
  - restored the P4 primary CTA rule fully:
    redistributable feeds get `Download ...`, while
    non-redistributable / metadata-only feeds now get `View metadata`
    pointing to the published `/<name>.json` artifact
  - restored the hero's tracking context line using the existing
    `started` timestamp: maintainer + tracking-since year + observed
    tracking age
- `ui/src/lib/feed-history.ts`
  - extracted the shared history CSV parser so the hero and behaviour
    section use the same source-of-truth series
- `ui/src/components/feed-detail/section-behavior.tsx`
  - switched to the shared history parser
- `specs/website.md`
  - added a normative feed-detail hero contract, including the rule that
    the hero evolution visual uses the same public history series and
    distinguishes loading / failed / not-enough-history / available

Verification:

- `pnpm --dir ui build`
- `git diff --check`

Follow-up correction requested by user:

- The first hero-chart pass restored the data and states, but the
  layout still drifted from the locked hero contract.
- The chart was rendered below the right-column stats, so it read like
  a lower subsection rather than the primary hero visual.
- Required correction:
  - keep the static dark hero background
  - keep the chart in the hero's right column
  - move the chart to be the dominant right-column visual, with the
    supporting right-column stats demoted below it

Completed:
- `ui/src/components/feed-detail/hero.tsx`
  - moved the all-time evolution chart to the top of the hero's right
    column so it now reads as the primary hero visual
  - demoted the current size / fact tiles below the chart
- verification:
  - `pnpm --dir ui build`
  - `git diff --check`

---

## Open research gap (CRITICAL)

The "deterministic signals we discussed" that user says are missing
from the UI — the discussion is **not in the April 3-7 conversation
files**. The background research agent recommends checking pre-April-3
session files. I need to do this research pass before Wave 2 lands or
risk repeating the same omissions.

Action: spawn a follow-up explore agent on `~/.claude/projects/-home-user-src-firehol-update-ipsets/86c989d2-33c8-4a1d-b9c7-8e0a58cbb28d.jsonl`
and any other older conversation files. Look for: "signals", "metrics
we detect", "facts we know", "what the backend computes", any list of
deterministic outputs user wanted shown. Specifically: anything
beyond the 16 insight rules already in `pkg/insights/`.

---

## Questions for user (the 8 items that need input)

1. **#5 Home page** — split to `TODO-HOMEPAGE.md`. Key questions:
   (a) globe.gl or a different hero visual? (b) IP search vs list
   search vs both in the hero? (c) which curated rows and in what
   order? (d) keep the 3 guided paths (PROTECT/REAL-TIME/RESEARCH) or
   different framing? (e) globe must pause when tab loses focus.

2. **#6 Methodology page** — grouping strategy (by section? flat?),
   intro copy tone, whether to show per-rule preview cards or just
   links.

3. **#17 Global search scope** — unified box or 3 modes (IP / list /
   researcher)? Researcher search would need a new index on
   maintainers from `all-ipsets.json`.

4. **#18 Hero evolution** — user has expressed THREE different
   preferences in the history:
   - April 5 (earlier): *"the map IS the hero visual"*
   - Phase 3 spec: right-column cinematic evolution chart
   - Today: *"the evolution chart can be a background"*
   Need explicit choice: map / right-column-evolution / background-evolution / map+evolution overlay.
   - 2026-04-23 clarification:
     - the current implementation now places the chart at the top of the
       right column, which is still wrong for user's intent here
     - user clarified he means **behind** the hero content, not
       above it and not below it
     - pending design decision:
       - full-bleed background chart behind the whole hero
       - background chart only behind the right hero column
       - subtle decorative background layer plus a smaller foreground chart
   - user decision:
     - `B` selected: the evolution chart becomes a **background layer
       behind the right hero column only**
   - Completed:
     - `ui/src/components/feed-detail/hero.tsx`
       - refactored the right hero column into a single visual panel
       - moved the evolution chart into an absolute background layer
         behind the right-column content
       - demoted the factual summary and stat blocks into the
         foreground overlay
       - dropped interactive tooltip behavior there so the hero chart
         behaves as a true background visual
     - `specs/website.md`
       - updated the feed-detail hero contract to reflect the chosen
         right-column background treatment
     - verification:
       - `pnpm --dir ui build`
       - `git diff --check`

5. **#20 Research lab listing** — confirm scope: flat list page at
   `/maintainers`, per-maintainer detail page with feeds + contact?
   Any extra fields beyond `maintainer` and `maintainer_url` that
   should be shown (email, GitHub org, Twitter)?

6. **#25 Admin per-feed detail** — needs new backend endpoints
   (`/disable`, error log streaming). Confirm scope: what actions
   beyond download/reprocess/disable? Should error logs stream live
   or show the last N lines? What goes in the per-feed detail panel
   besides current status + actions?

7. **#28 Cross-provider ASN radar** — drop, build, or defer? Phase 3
   spec listed it as Tab 3 of ASN section.

8. **#29 ASN data integrity bug** — verify: is the over-counting
   still happening on the current build? If yes, that's a backend
   investigation, not a UI fix.

---

## Progress log

- 2026-04-07: TODO file created with raw list verbatim.
- 2026-04-07: Research complete. 18 autonomous fixes identified, 6 need input. Ready to start Wave 1.
- 2026-04-23: user chose the principle-driven chart approach (`A`): derive truthfulness / interpretation-safety rules from the legacy site, codify them in specs, and apply them to all current public visualizations.
- 2026-04-23: Added a public chart interpretation contract to `specs/website.md` covering chart-state semantics, partiality disclosure, time anchors, precision rules, provider visibility, and info-box-vs-chart behavior.
- 2026-04-23: Implemented the first chart-safety pass in the React UI: retention now separates error/empty/partial states and ages freshness to now with a browser-clock safeguard; behavior charts no longer hide load failures or discard long cadence intervals; geo/ASN/overlap views now disclose provider-local failures and top-N truncation; homepage treemap/timeline now explain their time/size semantics locally.
- 2026-04-23: Unified homepage explorer "updated" semantics across timeline, freshest sort, cards, and table to use the same timestamp chain (`source_date` -> `processed_date` -> `checked_date`) and wrote that contract into `specs/homepage.md`.
- 2026-04-23: Found and fixed a runtime website bug affecting geo maps: feed country payloads were present, but the vendored world topology at `/world/countries-110m.json` was intercepted by the generic root `.json` artifact branch in `pkg/web/server.go`, which returned an empty `200` on misses. Added an explicit embedded-static `/world/` route and corrected the missing-artifact path to return `404` instead of a blank success.
- 2026-04-23: user reported a follow-up UI regression to address in the next pass: in any two-column chart layout, local notices such as "Time anchor", "Partial observation window", and "Observed removals only" must not push one chart lower than its sibling. The layout contract for paired panels is now: row 1 titles, row 2 descriptions, row 3 notifications, row 4 charts, so both charts remain horizontally aligned.
- 2026-04-23: Implemented the paired-panel alignment fix. Added a shared two-column panel primitive in `ui/src/components/feed-detail/section.tsx`, moved the retention section onto row-aligned title/description/notice/chart slots, refactored the behaviour section into two aligned chart rows, and wrote the paired-row requirement into `specs/website.md`.
- 2026-04-23: user chose the hero foreground treatment over the right-column background chart: frosted / semi-transparent small tiles (`A`) and a neutral high-contrast `Unique IPs tracked` number (`A`). The follow-up implementation pass must also make the large value auto-scale to its container width and force the hero range headline to break before the `N-month range:` fragment when history is available.
- 2026-04-23: Implemented user's hero readability follow-up. The right-column fact tiles now use a frosted/glass treatment, the `Unique IPs tracked` value now auto-fits to the available width instead of clipping, the stat color is neutral/high-contrast instead of red-on-red, and the hero evolution headline now breaks before the `N-month range:` fragment.
- 2026-04-23: Finished the shared IP-search unification pass. The homepage IP lookup now wraps the shared `IPSearchSurface` instead of maintaining a bespoke duplicate implementation, and the shared global result renderer now shows human-friendly country names plus ASN name/number context. Added a shared clear action + optional hash-preserving URL sync so the homepage still behaves as a shareable anchored section.
- 2026-04-23: Follow-up correction from user: the feed-detail search still did not provide country / ASN / map context like the homepage. Fixed by enriching the feed-scoped search endpoint with the same best-effort `IPContext` used by the global lookup when `details=true`, switching the feed page to request that enriched payload, and reusing the same detailed section renderer for both homepage and feed-detail lookups.
- 2026-04-23: Follow-up regression from user: IP lookup for `127.0.0.1` crashed the page because the backend correctly returned the special geo marker `COUNTRYLESS`, while the shared frontend lookup renderer incorrectly passed every `country_code` to `Intl.DisplayNames.of()` as if it were always an ISO alpha-2 region. The fix must harden the shared lookup surface so special/non-ISO geo markers render as text-only context with no region localization crash and no invalid map/link assumptions.
- 2026-04-23: New performance regression reported by user: global IP lookup takes about 4-5 seconds on localhost for ordinary lookups like `1.2.3.4`.
  Verified evidence:
  - live timing on this installation:
    - `/api/v1/search?ip=1.2.3.4` -> about `5.15s`
    - `/api/v1/search?ip=1.2.3.4&details=true` -> about `5.16s`
    - feed-scoped `/api/v1/sets/bogons/search?ip=1.2.3.4&details=true` -> about `0.006s`
  - current catalog size: `326` public feeds
  - `pkg/engine/query.go`
    - `QueryIP()` loops every public feed
    - `queryNamedIPv4()` opens the latest set for each feed, checks one IP, then closes it
  - `pkg/engine/fileset_helpers.go`
    - `openLatestSet()` opens the binary fileset from disk fresh on each call
  - `pkg/engine/latest_set_cache.go`
    - the engine already has a tested latest-set cache for heavy processing blocks, but the public search path does not use it
  - live daemon tracing proved the lookup also opens retained history snapshots to compute `first_seen`
    - `pkg/engine/query.go:117-182`
    - `populateQueryMatchTiming()` calls `queryMatchFirstSeen()`
    - `queryMatchFirstSeen()` iterates all history snapshots, opens each snapshot file, checks membership, closes it, and keeps scanning because it wants the oldest matching observation
  - the history snapshot list is sorted newest-first:
    - `pkg/engine/feed_body_stage.go:131-148`
  - measured request-time file-open counts from daemon tracing:
    - `1.2.3.4` -> `326` latest-set opens + `560` history-snapshot opens = `886` set-file opens total
    - `11.22.33.44` -> `326` latest-set opens + `148` history-snapshot opens
  implication:
  - the slowdown has two causes:
    - repeated reopen/close of all current `latest` binary sets on every global lookup
    - full history-snapshot scans for each matching feed to compute `first_seen`

user decisions:
- Use the existing latest-set cache for public search.
- Scan history the other way around for `first_seen` and stop at the first match.
  Clarified implementation meaning:
  - because `first_seen` wants the oldest observed match, the search order must be oldest-first
  - once a matching snapshot is found in oldest-first order, the scan can stop immediately

Implementation notes for the next pass:
- Reuse `latestSetCache` in the public query path without changing query results.
- Preserve freshness/correctness by clearing the cache whenever feed `latest` files are republished/replaced.
- Change `queryMatchFirstSeen()` to iterate the existing snapshot slice in reverse order instead of reopening newest-first all the way to the end.
- Add/extend tests to verify:
  - `first_seen` still returns the oldest matching snapshot
  - public lookup still works with binary latest files
  - cache invalidation does not return stale results after a set update

Implemented on 2026-04-23:
- added a safe long-lived shared latest-set cache for public search with
  per-feed invalidation after `finalize()` republishes `lib/<feed>/latest`
- wired public query paths to borrow cached binary latest sets instead of
  reopening them on every request
- changed `queryMatchFirstSeen()` to scan snapshots oldest-first and stop at
  the first matching snapshot
- added regression tests for:
  - oldest matching `first_seen`
  - cache invalidation after a feed update/finalize

Verified results on the live daemon after install:
- before fix:
  - `/api/v1/search?ip=1.2.3.4` -> about `5.15s`
- after fix:
  - first request after restart -> about `0.83s`
  - second request -> about `0.81s`

- 2026-04-23: Follow-up correctness issue reported by user after the
  performance fix: search `first_seen` was still wrong for feeds such as
  `firehol_level1`.
  Corrected diagnosis after deeper code/spec review:
  - the bug is **not** that merges have a special shallow-history rule
  - the bug is that search was reading the **wrong storage layer**
  - `pkg/engine/query.go` was using downloader-owned
    `data/history/{parent}/{unix_timestamp}.set`
  - specs define `data/history/` as downloader-owned retained snapshots for
    history-derivative support, not the general engine store for current
    per-IP age:
    - `specs/files-layout.md`
    - `specs/feeds.md`
    - `specs/pipeline.md`
  - the engine's actual current-IP age / retention state lives under:
    - `lib/{feed}/new/{unix_timestamp}`
    - `lib/{feed}/retention.csv`
    - `lib/{feed}/retention_cohorts.csv`
  - `firehol_level1` already has long-lived retention cohorts in
    `lib/firehol_level1/new/` (oldest current cohort files from 2018), so
    search should not have been consulting `data/history/firehol_level1`
    at all
  - `fullbogons` likewise has long-lived retention cohorts in
    `lib/fullbogons/new/`
  implication:
  - search `first_seen` for a currently listed IP must be sourced from the
    retention cohort store (`lib/{feed}/new/`), not downloader snapshots
  - downloader-owned `data/history/` remains valid only for history-derivative
    composition semantics, not for ordinary search age

Implementation direction confirmed by user:
- clarify the specs so the distinction between:
  - downloader-owned `data/history/`
  - engine-owned `lib/{feed}/history.csv`
  - engine-owned retention cohorts `lib/{feed}/new/{unix_timestamp}`
  - engine-owned removed-life ledger `lib/{feed}/retention.csv`
  is explicit and impossible to confuse
- fix search `first_seen` to use the engine retention cohort store

Implemented on 2026-04-23:
- search `first_seen` now reads engine-owned retention cohorts from
  `lib/{feed}/new/{unix_timestamp}` instead of downloader-owned
  `data/history/...`
- specs updated to make the storage-layer distinction explicit:
  - `specs/files-layout.md`
  - `specs/feeds.md`
  - `specs/processing-engine.md`
  - `specs/website.md`
- regression tests updated so search timing assertions now validate the
  retention cohort source of truth instead of downloader snapshots

Verified after install:
- `firehol_level1` / `10.20.30.40` now reports
  `first_seen = 1526216403` instead of `2026-04-21`
- detailed global lookup remains fast:
  - `/api/v1/search?ip=1.2.3.4&details=true` -> about `0.38s`

- 2026-04-23: New homepage correctness/layout issues reported by user:
  1. the hero says `201 tracked feeds`
  2. `IPs across feeds` is useless and should be removed
  3. the six explorer lens tiles overflow because they are not fixed-width
     in a wrapping grid and the description text does not wrap cleanly
  Verified evidence:
  - the homepage hero and explorer both use `eligibleFeeds(...)`:
    - `ui/src/pages/home.tsx`
    - `ui/src/components/home/home-explorer.tsx`
  - `eligibleFeeds(...)` currently applies:
    - public-category filtering
    - `homepageEligible(feed)` filtering
    - `ui/src/lib/explorer-state.ts:499-506`
  - `homepageEligible(feed)` keeps only:
    - health `healthy` or `delayed`
    - provenance `primary` or `secondary_upstream`
    - `ui/src/lib/feed-ranking.ts:93-99`
  - this matches the **homepage aggregation** rules in
    `specs/homepage.md:287-305`, but it contradicts the explorer contract in
    `specs/homepage.md:151-178` and `specs/homepage.md:264-278`, which says
    the explorer must expose the full tracked public feed inventory
  - live evidence on this installation:
    - `GET /api/v1/sets` -> `331` public feeds
    - `GET /api/v1/home/summary` -> `eligible_feeds = 201`
    - the hero currently labels that narrower aggregation subset as
      `Tracked feeds`, which is semantically wrong
  - the `IPs across feeds` tile is currently a raw cross-feed sum of
    `feed.unique_ips` in `ui/src/pages/home.tsx`, which produced
    `6,670,773,757` on this installation; it is not a deduplicated union and
    is not a meaningful public stat
  - the six lens tiles are rendered in a single horizontal scrolling flex row
    with per-card `min-w-[12rem]`:
    - `ui/src/components/home/home-explorer-lens-strip.tsx`
    this causes overflow and uneven heights instead of a stable aligned grid
  implementation direction for this pass:
  - split **public feed inventory** from **homepage aggregation-eligible**
    feeds in the frontend helpers
  - use the full public feed inventory for:
    - the hero `Tracked feeds` number
    - the explorer inventory / counts / filters
  - keep the narrower health/provenance filter only for homepage rollups
    and other aggregate surfaces
  - remove the `IPs across feeds` hero stat
  - convert the lens strip into a wrapping fixed-column grid with clean text
    wrapping so the six tiles stay aligned
