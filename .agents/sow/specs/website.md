# Website Contract

## Status

This document is normative unless a section explicitly says **Non-normative**.

The words **MUST**, **MUST NOT**, **SHOULD**, and **MAY** describe the required
behavior of the product.

## Purpose

The public website is the human-facing surface of the observatory.

It exists to let users:

- discover feeds
- compare them
- inspect one feed in depth
- search for IP membership
- understand the methodology behind published metrics and insights

The website is where end users receive the value described in
[design.md](design.md): comparative understanding of many feeds through stable
published artifacts.

## Public surface

The public site MUST provide distinct surfaces for:

### 1. Homepage

The primary product surface. The homepage MUST introduce the product, support
global IP lookup, and expose the full feed inventory in an interactive
explorer.

Detailed requirements for the homepage are normative in `.agents/sow/specs/homepage.md`.
The homepage absorbs the responsibilities of the previous separate catalog
surface; no standalone catalog surface is required.

### 2. Feed detail

A dedicated page per feed that exposes feed-local facts and comparative context.

### 3. Classification surfaces

Dedicated index and detail pages for classification entities referenced from
the homepage and from feed-detail pages:

- country index and country detail by ISO country code
- ASN index and ASN detail by ASN number
- maintainer index and maintainer detail by maintainer slug

### 4. Methodology

A browseable and linkable explanation surface for how metrics and insights are
defined.

## Route contract

The exact route names are part of the public contract.

The product MUST provide stable routes equivalent to:

- homepage
- feed detail by feed name
- country index
- country detail by country code
- ASN index
- ASN detail by ASN number
- maintainer index
- maintainer detail by maintainer slug
- methodology index
- methodology detail by slug
- MCP endpoint at `/mcp`

Public route ownership rule:

- public website routes belong to the public listener/surface
- admin HTML routes and `/api/v1/admin/*` do not belong to the public website
- when the product is configured with a separate admin listener, the public
  listener MUST NOT serve admin routes
- public route construction MUST derive published artifact roots and runtime
  limits from one coherent config/runtime generation instead of mixing
  separately fetched runtime values

## Frontend runtime contract

The public website is an artifact-backed SPA, so it must keep the homepage
cheap and make navigation failures local to the current route.

- Top-level pages MUST be loaded through route-level lazy imports with a shared
  loading boundary and route-level error boundary.
- Public homepage rendering MUST NOT eagerly load admin-only, feed-detail-only,
  methodology-only, or entity-detail-only page modules.
- Public-shell imports MUST NOT pull admin-only endpoints or feed-detail
  section endpoint families into the main entry chunk through broad API/query
  barrels or eager route-layout imports. Query helpers and API clients SHOULD
  be split by route/concern so prefetch and shared chrome import only the
  endpoint families they actually use.
- Route navigation and query-key changes SHOULD cancel obsolete in-flight API
  requests through the browser `AbortSignal` exposed by TanStack Query.
- Theme state MUST have one owner. The React app uses `next-themes` to write
  the `light`/`dark` class on the document root, and theme-aware UI such as
  toasts MUST read the same provider state.
- Interactive table rows and equivalent non-button surfaces MUST support
  keyboard activation and an accessible name, unless the interaction is
  represented as a native link or button.
- WebGL/Three scenes MUST be safe to mount, unmount, and remount during normal
  navigation. Data-derived HTML labels rendered by visualization libraries MUST
  escape or sanitize user/upstream-controlled text.

## Public data contract

The public website MUST be backed by public machine-readable data that exposes
at least:

- feed catalog entries
- feed detail metadata
- bounded history for charts
- feed comparison data
- retention summaries
- ASN summaries
- ASN detail payloads
- geography summaries
- country detail payloads
- bogon summaries
- insight summaries
- category metadata
- country index summaries
- ASN index summaries
- global IP lookup
- maintainer index summaries (dynamic, served from live engine state; not precomputed artifacts)
- maintainer detail payloads (dynamic, served from live engine state; not precomputed artifacts)

Maintainer index and detail payloads use the homepage aggregation eligibility
boundary: feeds must be public-category, not hidden, not ASN or geolocation
provider-only data, have `primary` or `secondary_upstream` provenance, and
currently classify as `healthy` or `delayed`. Maintainer endpoints MUST NOT
surface unhealthy, archived, hidden, provider-only, or non-public-category
sources merely because those sources have maintainer metadata in the catalog.

Normal public feed catalog/detail payloads apply only to public feeds.
Supporting provider datasets such as ASN and geolocation databases are not
normal public feeds; they MAY power public derived views and provider-scoped
analysis, but they MUST NOT appear as ordinary public catalog rows, public feed
sidebar entries, or ordinary `/api/v1/sets/{name}` feed-detail targets.

ASN and geolocation provider defaults MUST be chosen by explicit configuration,
not by accidental source order. Public ASN detail pages, country detail pages,
IP lookup context, homepage summaries, and feed-detail ASN/GEO default tabs
MUST use the configured defaults. Provider-list APIs and UI tab lists MUST put
the configured default provider first and then retain normal catalog order for
the remaining providers.

The same eligibility rule applies to feed-scoped compatibility artifacts and
download routes that expose the published web tree. Legacy/static paths such as
`/{feed}.json`, `/{feed}_history.csv`, `/{feed}_comparison.json`,
`/{feed}_insights.json`, and raw feed downloads (`/{feed}.ipset`,
`/{feed}.netset`, `/files/{feed}.ipset`, `/files/{feed}.netset`) MUST follow
the same public-feed boundary. Hidden feeds and supporting provider datasets
MUST NOT become public feed targets merely because an old file exists on disk.

Archived and non-redistributable feeds remain public catalog/detail and
analytical/reference targets.

However, they MUST NOT expose operational feed URLs or raw feed bodies for public
use. At minimum:

- raw feed download routes MUST NOT serve archived or non-redistributable feeds,
  even if a local canonical body still exists
- any equivalent raw feed-body endpoint such as `/api/v1/sets/{name}/data` MUST
  NOT serve archived or non-redistributable feeds
- dynamic raw-body composition endpoints such as `/api/v1/compose` MUST apply
  the same public-feed, archived, and redistributable checks to every included
  and excluded input
- raw feed body routes MUST stream canonical `.ipset`/`.netset` files from
  disk or another bounded reader; they MUST NOT load and retain whole raw bodies
  in the long-lived JSON/static artifact cache
- raw feed body routes MUST return an error status when the resolved body cannot
  be served; they MUST NOT fall through to an empty successful response
- public feed metadata/detail payloads for archived or non-redistributable feeds
  MUST NOT expose an actionable upstream source URL or actionable local
  raw-download URL

The long-lived public JSON/static artifact cache MUST be bounded by configured
runtime controls for maximum entries, total cached bytes, and maximum cached
file size. Cache-eligible files above the per-file limit are still public
artifacts when their route allows them, but they are streamed from disk instead
of retained in memory. Direct raw `.ipset`/`.netset` routes remain outside this
cache.

`GET /api/v1/categories` MUST expose only categories that configuration marks
as public. Non-public/system-role categories remain configuration data, but
they are not part of the public category registry consumed by homepage browse
surfaces.

The UI MUST treat these as product data contracts, not as opportunistic
best-effort screen scraping.

`GET /api/v1/sets` feed summaries MUST expose typed critical-infrastructure
filter facts needed by the homepage explorer:

- `critical` metadata for feeds that are themselves configured critical
  reference feeds
- `critical_overlap_tiers` for normal feeds with positive published overlap
  against critical reference tiers

These facts MUST be derived from configuration and generated critical-overlap
artifacts. Public website code MUST NOT classify them through feed-name or
artifact-filename pattern matching.

For merge feeds, public feed detail payloads/pages MUST expose:

- the current included inputs
- the current subtracted inputs
- the current health-excluded inputs
- exclusion reasons

This visibility is publication data. Public routes MUST serve the latest
published metadata artifact and MUST NOT recompute merge composition during the
request. Health-based merge exclusions change over time, so processing,
reprocess, and repair paths own refreshing the published metadata artifact.

Critical-infrastructure overlap routes are also publication-data readers. The
routes `/api/v1/sets/{feed}/infrastructure`,
`/api/v1/sets/{feed}/infrastructure/providers`, and
`/api/v1/sets/{feed}/infrastructure/{provider}` MUST serve already-published
critical-overlap artifacts or configured provider metadata only. They MUST NOT
download, intersect, or regenerate overlaps during the request. They MUST
serve any artifact that exists on disk and passes structural validation,
regardless of its `provider_set_id` value: the public surface is cache-first
and MUST NOT enforce internal identity equality at request time. Drift between
the engine's current provider-set identity and the identity stamped into
published artifacts is the admin integrity path's concern, not a public
concept. The same cache-first rule applies to direct static JSON routes such
as `/{feed}_critical_infrastructure.json` and
`/{feed}_critical_{provider}.json`.
These routes MUST reject requests for feeds that are not critical-infrastructure
comparison targets (critical reference feeds, provider-context feeds, or
non-IPv4 feeds in the v1 writer) and for providers that are not in the
configured catalog. Those are catalog-shape rejections, not identity-drift
rejections, and they protect against serving requests the catalog does not
describe.
The provider-list route may expose critical reference metadata for configured
providers even when those providers are not raw-redistributable. Raw body and
compose routes for those provider feeds remain governed by the normal
public-feed, archived, and redistributable checks.

Per-feed provider-list routes for GeoIP, ASN, and bogon data are configuration
metadata routes:

- `/api/v1/sets/{feed}/countries`
- `/api/v1/sets/{feed}/asn`
- `/api/v1/sets/{feed}/bogons`

They MUST list configured providers for that provider family and MUST NOT use
the current feed's artifact availability as a filter. Provider-specific routes
such as `/api/v1/sets/{feed}/countries/{provider}` are the artifact readers;
those routes MAY return a missing-artifact response when the provider is
configured but the feed-specific artifact is absent.

Generated artifact route parsing MUST use exact configured identities and typed
artifact descriptors. A configured public feed whose name contains generated
artifact tokens such as `_asn_`, `_bogons_`, `_critical_`, or
`_critical_infrastructure` MUST still be treated as that exact feed first.
Public routes MUST NOT infer artifact family by substring matching alone.

Feed detail pages MUST show both aggregate critical-overlap facts and compact
hard/soft/contextual tier summaries. Hard-tier overlap should be visually
distinct from soft/contextual overlap; contextual provider-space overlap must
be presented as policy-dependent rather than as proof that the feed is wrong.
Provider-context feed pages MUST explain that broad provider/customer-hosting
space is context and not critical-warning truth instead of querying missing
critical-overlap artifacts.
If an aggregate includes critical ASN context, the feed page MUST show it as a
separate secondary signal and not add it to matched reference-feed counts.
When a feed-detail page lists matched critical-infrastructure reference feeds,
default ordering MUST follow the risk model: hard before soft before
contextual, then larger matched-IP counts within the same tier, then a stable
provider-name tie-breaker. Broad provider-context coverage MUST NOT
bury hard-tier DNS or root-service matches merely because it has more matched
IPs.

## Public API route families

The machine-readable public surface MUST expose stable endpoint families
equivalent to:

- `GET /healthz`
- `GET /api/v1/status`
- `GET /api/v1/categories`
- `GET /api/v1/client-ip`
- `GET /api/v1/home/summary`
- `GET /api/v1/home/globe`
- `GET /api/v1/search`
- `GET /api/v1/query` (same handler as `/search`, backward-compat alias)
- `GET /api/v1/compose`
- `GET /api/v1/sets`
- `GET /api/v1/ipsets` (backward-compat alias for `/sets`)
- `GET /api/v1/sets/{name}`
- `GET /api/v1/ipsets/{name}` (backward-compat alias for `/sets/{name}`)
- `GET /api/v1/sets/{name}/search`
- `GET /api/v1/sets/{name}/data`
- `GET /api/v1/sets/{name}/history`
- `GET /api/v1/sets/{name}/changesets` (published as CSV file, served as JSON via API)
- `GET /api/v1/sets/{name}/compare` (also accepts `comparison` as alias)
- `GET /api/v1/sets/{name}/retention`
- `GET /api/v1/sets/{name}/insights`
- `GET /api/v1/countries`
- `GET /api/v1/countries/{code}`
- `GET /api/v1/asns`
- `GET /api/v1/asns/{asn}`
- `GET /api/v1/sets/{name}/countries` and provider-scoped country detail
- `GET /api/v1/sets/{name}/asn` and provider-scoped ASN detail
- `GET /api/v1/sets/{name}/bogons` and provider-scoped bogon detail
- `GET /api/v1/sets/{name}/infrastructure`, provider listing, and
  provider-scoped critical-infrastructure detail
- `GET /api/v1/maintainers`
- `GET /api/v1/maintainers/{slug}`
- `GET /api/v1/methodology`
- `GET /api/v1/methodology/{slug}`

`GET /api/v1/compose` is a bounded dynamic composition endpoint over committed
local feed bodies. It MUST require at least one included set, MUST cap include
sets at 20, MUST cap exclude sets at 20, and MUST cap output at 32 MiB. It MUST
apply the public raw-feed policy to every included and excluded set: the set
must be public, redistributable, not archived, and backed by an available raw
`.ipset` or `.netset` body. Supported output formats are CIDR/net notation,
range notation, and single-IP notation. The route MUST NOT fetch upstream data
or broad-recompute analytics during the request.

`GET /api/v1/sets` feed entries MUST expose the short public enrichment
summary when available: `official_name`, `short_description`, and
`current_status_state`. `short_description` is treated as the feed's
identifying caption: every clickable feed reference across the website
(explorer cards/table/timeline/treemap/maintainer views, sidebar,
maintainer/ASN/country detail pages, in-feed lookup, overlap rows, source
feeds) MUST render a tooltip that surfaces `official_name` (when distinct
from the bare name), `short_description`, and `maintainer` so that hovering
any feed name is enough to identify what the feed is.

`GET /api/v1/sets/{name}` per-feed metadata MUST expose the full public
`enrichment` payload when available, plus top-level `official_name`,
`short_description`, and `current_status` convenience fields. These fields are
generated into the published metadata artifact and served as existing cache-first
metadata; public requests MUST NOT build enrichment data on demand.

The removed `GET /api/v1/sets/about/{name}` HTML description route MUST NOT be
reintroduced. Descriptive content belongs in the generated per-feed metadata and
markdown artifacts through the embedded public enrichment payload.

### MCP endpoint

The product MUST expose a Streamable HTTP MCP endpoint at `/mcp` that provides
programmatic access to the public website data for AI agents and MCP-compatible
tools.

The MCP endpoint:

- MUST use the Streamable HTTP transport defined by the MCP specification
  (2025-03-26), supporting POST (JSON-RPC), GET (SSE), and DELETE (session
  termination)
- MUST register two tools: `find_feeds` for metadata-based feed discovery and
  `fetch_analysis` for retrieving pre-generated markdown analysis pages
- MUST be read-only — no tools may modify server state or trigger computation
- MUST share the same rate limit as the general public API (240 req/min per
  client)
- MUST set CORS headers that allow POST, GET, DELETE, and OPTIONS from any
  origin, allow the Streamable HTTP request headers `Content-Type`,
  `Mcp-Session-Id`, `MCP-Protocol-Version`, and `Last-Event-ID`, and expose
  `Mcp-Session-Id` to browser clients
- MUST NOT expose admin operations, internal state, or configuration details
- MUST serve pre-generated artifacts for `fetch_analysis` — not generate
  markdown on demand during the request
- MUST remain limited to the registered `find_feeds` and `fetch_analysis`
  tools until a later SOW explicitly changes the MCP tool contract

The `find_feeds` tool MUST support full-text search across feed names,
official names, maintainer names, short descriptions, and feed descriptions,
plus metadata filters equivalent to the homepage explorer: category,
maintainer, provenance, health, freshness, cadence, uniqueness, license,
redistributable, critical tier, and size range.
The maintainer filter uses a case-insensitive substring match against
maintainer names, while category, provenance, health, freshness, cadence,
uniqueness, license, redistributable, and critical-tier filters use
case-insensitive exact bucket/value matching.
The tool input schema MUST expose JSON Schema enums for known string domains.
Fixed taxonomy filters (`provenance`, `health`, `freshness`, `cadence`,
`uniqueness`, `redistributable`, and `critical`) use stable enum lists.
Catalog-derived filters (`category`, `maintainer`, and `license`) use enum
values discovered from the active public feed catalog when the MCP server is
constructed.
The tool result MUST be markdown, not JSON: a compact table for structured
fields, including inline official name and short description when available,
followed by per-feed sections containing official name, short description,
maintainer, and description text. Result table dates MUST be RFC3339 UTC with
relative age in parentheses. The `unique_share_pct` table cell MUST be
formatted with one fractional digit and a trailing `%`.

The `fetch_analysis` tool MUST read pre-generated markdown from the published
artifact tree and return it as text content. Markdown paths are entity-local:
feeds use `{web-dir}/{name}.md`, countries use `{web-dir}/countries/{code}.md`,
ASNs use `{web-dir}/asns/{asn}.md`, and maintainers use
`{web-dir}/maintainers/{slug}.md`. Current ASN markdown identifiers are numeric
artifact names without the `AS` prefix.

Feed markdown returned through `fetch_analysis` MUST be compact enough for AI
agent context windows: it renders only the configured default ASN provider and
the configured default GeoIP provider, renders critical-infrastructure provider
labels/names instead of raw provider objects, and rolls retention age tables
into non-zero days within the first 365 days plus `>365 days` for later
buckets. The markdown must state that omitted days have zero IP count. Full
provider fan-out remains available through JSON/API artifacts.

Feed markdown MUST be a self-contained analysis report for AI clients, not a
reduced clone of the visual feed-detail page. Analytical sections MUST include
compact interpretation text explaining what the section means, which
observation window or artifact family it uses, and what a client must not infer
from the data.

Feed markdown MUST keep monitoring start separate from behavior history.
Monitoring start is exposed as `Tracked since` from feed metadata and MUST NOT
be inferred from feed-level `history.csv` or `changesets.csv` rows.

Feed markdown behavior MUST align with the visual feed-detail behavior surface:
the section describes how the list moves over time using bounded published
history and changeset artifacts, normally the last recorded-run window, not a
fixed calendar period and not a full lifetime ledger. It MUST NOT introduce a
separate publication-lifecycle section or label the first retained changeset as
the initial publication.

Feed markdown behavior MUST show additions, removals, and churn for retained
content changes. If no retained content changes are available, the markdown
MUST state that explicitly instead of emitting a zero-counter activity table.

Feed markdown activity rollups MUST be semantic and explained. Raw observed
change rows SHOULD be used when the row count is small enough for AI context.
When rollup is required, the selected bucket MUST NOT be smaller than the
feed's configured or observed cadence. The markdown MUST state the chosen
resolution and explain whether values are raw rows or bucketed sums/averages.

Feed markdown cadence MUST be separate from activity rollups. Cadence MUST be
described as gaps between published history timestamps/content-changing
observations, not source checks or HTTP requests.

Feed markdown technical specifications MUST NOT emit blank property rows.
Normal ipset/netset feeds use the configured output format (`ipset` or
`netset`) for the `Format` row when no specialized source `format:` exists.
Update timing rows MUST be populated from the generated metadata cadence fields
(`average_update`, `min_update`, and `max_update`) when available, and omitted
when no value is known.

Bogon sections in feed markdown MUST use only configured `use: [bogons]`
reference providers. Stale themed lists that remain normal feeds, such as
`iblocklist_bogons`, MUST NOT appear as bogon reference-provider overlap.

Feed markdown overlap tables MUST include:

- `Common`: number of IPs shared by the current feed and the row feed.
- `This %`: common IPs divided by total IPs in the current feed.
- `Their %`: common IPs divided by total IPs in the row feed.

The markdown MUST define `This %` and `Their %` next to the table so AI clients
do not need to infer the denominator.

MCP tool descriptions MUST be written so an AI model can understand the
semantic meaning of each parameter and its valid values without external
documentation.

Exact payload schemas MAY evolve compatibly, but these endpoint families and
their semantic purpose are part of the public contract.

The public API families above are GET/HEAD read surfaces. Unsupported methods
for known public routes MUST return `405 Method Not Allowed` with an `Allow`
header rather than reaching the read handler. Public CORS preflight requests
remain the intentional exception: `OPTIONS` requests are handled by the CORS
middleware before route dispatch.

## Public metadata files

The public listener MUST serve:

- `GET /robots.txt`
- `GET /sitemap.xml`
- `GET /sitemap-*.xml`
- `GET /llms.txt`

`robots.txt` MUST point to the public sitemap when a public site base URL can
be determined. It MUST discourage crawler access to live/query endpoints with
`Disallow` rules for `/api/v1/search`, `/api/v1/query`, `/api/v1/compose`,
`/api/v1/client-ip`, and per-feed search endpoints. Robots rules are advisory
crawler hints only; they are not security controls or rate limits. The file
MUST NOT list authenticated admin paths or private local paths.

`sitemap.xml` MUST be a Sitemaps.org sitemap index. Sitemap shard files MUST
use absolute URLs and the Sitemaps.org XML namespace. Shards MUST enumerate
the homepage, public index pages (`/countries`, `/asns`, `/maintainers`,
`/methodology`), one feed-detail URL for each public output feed, one country
detail URL for each country in the public country index, one ASN detail URL for
each ASN in the public ASN index, and one maintainer detail URL for each public
maintainer. Route shards SHOULD stay below 45,000 URLs so they remain well
under the 50,000-URL sitemap protocol limit. Do not enumerate admin routes,
API routes, raw file downloads, authenticated routes, or private runtime
details in sitemaps.

`llms.txt` MUST be concise Markdown that points to public pages, methodology,
public API indexes, and feed catalog surfaces. It MUST NOT expose admin
routes, authenticated operations, local filesystem paths, or private runtime
details. The product treats `llms.txt` as an emerging convention for curated
AI-readable site context, not as a security or crawler-control mechanism.

`GET /api/v1/status` is a coarse public service-status endpoint. It MAY expose
high-level public runtime facts such as service availability, uptime, and
catalog counts, but it MUST NOT expose operator-only queue/backlog state,
active-feed execution detail, local filesystem paths, or other authenticated
admin/runtime internals.

When a separate admin listener is configured:

- the public listener MUST continue to serve the public website and the public
  API route families above
- the public listener MUST NOT serve `/admin`, `/admin/*`, or
  `/api/v1/admin/*`
- the admin listener MAY serve `GET /api/v1/categories` as read-only product
  metadata for the shared admin SPA shell; this does not make the admin
  listener a public website listener and MUST NOT imply raw feed-body access or
  public page serving on the admin origin

Unmapped public API paths under `/api/v1/` MUST fail as API requests. They
MUST NOT fall through to the website SPA shell or return HTML route content.

## Serving discipline

Ordinary public browsing MUST be cache-first and publication-driven.

This means:

- public pages MUST read already published local artifacts
- public pages MUST NOT trigger upstream acquisition
- public pages MUST NOT trigger broad recomputation of feed analytics
- public pages MAY call local HTTP endpoints when those endpoints serve
  precomputed local artifacts or viewer-specific lookup responses
- feed-scoped artifact endpoints such as metadata (`/api/v1/sets/{name}`),
  history, changesets, comparison, retention, insights, ASN, GeoIP, bogon, and
  critical-infrastructure details MUST return a missing response when their
  published artifact is absent instead of deriving it from internal ledgers or
  feed bodies during the request
- critical-infrastructure endpoints MUST serve any artifact that exists on
  disk and passes structural validation; the public surface MUST NOT reject
  served artifacts on internal identity equality (`provider_set_id`) and MUST
  NOT render integrity-drift verdicts as user-facing editorial content. The
  engine's single-snapshot pipeline contract is responsible for keeping
  artifacts and the runtime marker in agreement; admin integrity is the
  operator-facing channel if that contract is ever violated.
- when a `WebDir`/published-output override is configured, feed-scoped artifact
  endpoints MUST read that published artifact tree, not the engine's default
  runtime web directory
- public artifact and raw-download serving MUST open files relative to the
  configured served root and reject traversal or symlink escapes outside that
  root; a bad path, missing file, unreadable file, or escaping symlink is not a
  reason to compute or fetch replacement data on the public request path

The broader cross-cutting rule is owned by
[operating-principles.md](operating-principles.md).

Country/ASN reference serving rules:

- `/api/v1/countries`
- `/api/v1/countries/{code}`
- `/api/v1/asns`
- `/api/v1/asns/{asn}`

These routes MUST behave as cache-first readers over the published entity
artifact tree under `web/`. They MUST NOT do request-time aggregation over the
full feed catalog during ordinary public browsing.

Missing-artifact behavior:

- missing country/ASN index artifacts MUST return a not-ready service response
  rather than run live aggregation
- missing country/ASN detail artifacts MUST return a missing-entity response
  rather than run live aggregation
- country detail route parameters MUST be validated as two ASCII letters before
  constructing an artifact path
- unreadable published entity artifacts MUST return a service-unavailable
  response
- producer, startup integrity, admin repair, or explicit rebuild actions own
  regeneration; public entity routes are readers only

The engine MAY keep private reusable entity sidecars under `lib/`, but those
sidecars are an internal optimization layer. The public routes above MUST
serve only the final published JSON payloads.

Homepage rollup serving rules:

- `/api/v1/home/summary`
- `/api/v1/home/globe`

These routes MUST compose their public responses from the published homepage
aggregate artifact under `web/home/`. They MUST NOT scan the full feed catalog
or read per-feed GeoIP/ASN artifacts during ordinary public requests. Missing
or unsupported homepage aggregate artifacts MUST return a not-ready service
response rather than rebuilding the aggregate on the first request. Entity
integrity and admin repair MUST treat the homepage aggregate as a repairable
published artifact and restore missing, malformed, or stale copies from current
runtime state.

Refresh rules for these entity artifacts:

- feed-content changes MUST update the affected entity composition and final
  payloads through a bounded per-feed delta:
  - compute the changed feed's new per-feed entity sidecar once during the
    processing run while geo/ASN providers are already open
  - use the old and new feed sidecars to identify affected countries and ASNs
  - patch only those country/ASN artifacts
  - update every derived aggregate in those artifacts, including feeds, totals,
    top categories, top maintainers, top ASNs, top countries, and country
    distribution
- provider changes MAY require all public feeds to refresh their provider-derived
  sidecars, but that refresh MUST be admitted as bounded background/entity work
  rather than hidden inside ordinary page serving
- the expensive country<->ASN intersection work for those pages SHOULD happen
  once per affected feed during normal processing, be persisted in private
  pending feed sidecars, and then be consumed by the background entity patcher
  rather than recomputed by that background patcher or once per entity page
- pure health transitions MAY rewrite only the affected final public detail
  payloads, reusing private sidecars instead of recomputing the heavier
  composition layers
- startup and live config reload MAY schedule a broader rebuild in the
  background so the public service becomes available quickly while entity
  artifacts are refreshed safely
- the operator/admin surface SHOULD also offer an explicit action to queue a
  full rebuild of the country/ASN entity surface from scratch

Freshness rule for comparison-driven pages:

- pairwise comparison views on the site MUST reflect the latest known state of
  either side of the comparison
- a feed page MUST therefore show the current comparison with peer feeds even
  when only the peer changed recently
- pairwise comparison artifacts and UI tables MUST contain non-zero overlaps
  only; a peer with zero shared IPs is represented by absence, not by a public
  row with `common: 0`
- any public insight derived from pairwise comparison data MUST follow the same
  freshness rule
- if a required comparison artifact is missing, the public route returns a
  missing response and the repair/reprocess path owns regeneration

## Design principles

The public site MUST behave like an editorial data product, not like a dense
infrastructure console.

The design system MUST emphasize:

- readability
- factual presentation
- strong hierarchy
- low clutter
- consistent semantics

The public site MUST NOT:

- present operator-only concerns as primary public content
- use decorative confidence language for deterministic findings
- pad pages with placeholder content when the product has no fact to show

## Public navigation contract

The public chrome MUST expose first-class navigation to the public reference
surfaces, not only to feed browsing.

At minimum, the public header/footer navigation MUST make these surfaces
discoverable:

- homepage / feed explorer
- countries
- autonomous systems
- maintainers
- methodology

## Content principles

The public site MUST present:

- facts
- measurements
- methodology
- source provenance

The public site MUST NOT present:

- hidden ranking logic as objective truth
- unsupported reputation claims
- fake certainty where evidence is missing

## Feed-detail technical specifications contract

Every public feed-detail page MUST include a technical-specifications section
rendered from the feed-detail metadata payload.

This section is a **public fact sheet**, not an operator surface.

Its structure is normative:

- it MUST remain a 3-group section:
  - Identification
  - Data
  - Updates
- it MUST NOT reintroduce separate public `Access` or `Processing` groups
  merely because those facts exist in internal config or runtime state

Minimum Identification coverage:

- feed name
- category
- provenance / lineage
- maintainer identity and homepage when known
- upstream source URL when it is public for that feed
- license
- required attribution text when provided
- redistributable status
- whether operational raw-feed URLs are available to the public

Minimum Data coverage:

- IP version
- canonical data format when known
- current size
- observed min/max size bounds when available
- aggregation window when applicable
- canonical body hash when available

Minimum Updates coverage:

- configured cadence
- observed interval statistics when available
- health class and enough threshold context to interpret it
- tracked / updated / processed / checked timestamps
- clock skew
- download-failure count
- version / revision counter when present

If the feed is a merge, the page MUST also expose its current merge
composition, including included inputs, subtracted inputs, health-excluded
inputs, and exclusion reasons, as already required by the public data
contract.

## Feed-detail hero contract

The feed-detail hero is the primary orientation surface for one feed.

It MUST expose, at minimum:

- the feed identity
- the primary public CTA for that feed
- a short current-state summary

When feed history is available, the hero SHOULD also include an all-time
evolution visual sourced from the same bounded public history data the rest of
the site uses.

Hero evolution rules:

- the source of truth is the published feed history series, not a separate
  backend-only computation
- the hero visual SHOULD show the all-time shape of the feed's IP count
  evolution, not a second copy of the behaviour-section chart chrome
- when the product uses the evolution chart as a background treatment, the
  preferred placement is behind the **right hero column only**, not as a
  full-page background behind the entire hero
- in that background treatment, the chart MAY be non-interactive and serve as
  a factual visual layer behind the right-column content rather than as a
  standalone foreground chart block
- when the product uses that right-column background treatment, any overlay
  fact tiles or stat panels in front of the chart MUST preserve the chart as a
  visible background layer while keeping the text readable; solid opaque blocks
  that visually sever the chart are not acceptable
- the large primary stat value in that overlay MUST scale to the available
  width of its container rather than clip
- the primary stat text SHOULD remain visually distinct from the chart stroke
  or fill behind it; avoid assigning the same loud accent color to both when
  that reduces readability
- the hero MUST distinguish:
  - loading
  - failed to load
  - not enough history yet
  - available

The hero's local explanatory copy MUST remain factual. It MAY summarize the
current count and the observed historical range, but it MUST NOT present
editorial verdicts or quality grades.

## Classification-detail contract

Country-detail and ASN-detail surfaces are public reference pages, not thin
homepage summary fragments.

For these two surfaces, the narrower homepage aggregation eligibility rules
MUST NOT be reused as the page-eligibility contract. A country or ASN detail
page MUST be allowed to surface any feed that is otherwise a public feed and
currently contributes to that entity, even when that feed is:

- `risky`
- `unmaintained`
- `archived`
- provenance `merge`
- provenance `derived`

The purpose of these pages is to explain public composition truthfully, not to
silently hide less desirable contributors.

### Country detail

The country-detail page MUST expose, at minimum:

- the country identity
- the active geo provider used for the page
- summary totals for:
  - matching feeds
  - attributed IPs
  - categories represented
  - maintainers represented
  - ASNs represented
- grouped feed composition by category
- visible health and provenance on individual feed rows
- maintainer identity on individual feed rows
- a country-specific ASN composition block

When the country-detail page contains long summary/composition lists or very
large grouped feed sections, it MAY cap those regions with internal scrollable
viewports to keep the overall page height bounded. Such bounded regions MUST
NOT trap normal page-wheel scrolling when the inner region itself cannot
continue scrolling.

The country-specific ASN block MUST be truthful to the selected country.
It MUST NOT be produced by reusing each feed's whole-feed top-ASN ranking and
pretending that those ASN totals belong to the country. Instead, it MUST be
derived from the canonical feed body intersected with:

- the selected country's geographic attribution
- the selected ASN provider

When ASN names are available for that country-specific ASN block, the public
page SHOULD show them alongside the ASN number instead of rendering bare ASN
numbers only.

If the page renders a country outline/map for the single target country, that
map MAY be intentionally non-analytical and serve as entity orientation rather
than as an intra-country distribution chart.

### ASN detail

The ASN-detail page MUST expose, at minimum:

- the ASN identity
- the active ASN provider used for the page
- the active geo provider used for ASN-country geography, when available
- summary totals for:
  - matching feeds
  - attributed IPs
  - categories represented
  - maintainers represented
  - countries represented
- grouped feed composition by category
- visible health and provenance on individual feed rows
- maintainer identity on individual feed rows
- a country-distribution block for the ASN

When the ASN-detail page contains long summary/composition lists or very large
grouped feed sections, it MAY cap those regions with internal scrollable
viewports to keep the overall page height bounded. Such bounded regions MUST
NOT trap normal page-wheel scrolling when the inner region itself cannot
continue scrolling.

If the page renders an ASN country map, that map MUST be backed by real
ASN-country distribution data for the selected ASN. It MUST NOT be inferred
from:

- whole-feed top-country rankings
- whole-feed top-ASN rankings
- homepage aggregate summaries

The source of truth for ASN-country geography MUST be the canonical feed bodies
intersected with:

- the selected ASN provider
- the selected geo provider
- the target ASN

### Shared row semantics

Grouped feed rows on country-detail and ASN-detail pages MUST keep the
following facts visible in the row itself or its immediate metadata line:

- feed identity
- current health
- provenance
- maintainer when known
- entity-attributed IP count for that row
- total feed IP count
- recent change timing when available

These rows MUST remain readable for very large counts as well; attributed and
total-IP values MUST NOT clip or overflow their visible cells.

## Category presentation

Category presentation MUST be data-driven.

This includes:

- category labels
- category descriptions
- ordering
- color meaning

The public website MUST NOT hardcode category semantics that already belong to
configuration.

## Search contract

The product MUST support:

### Global IP search

Given an IP, the public site MUST be able to tell the user which feeds contain
it and provide enough context to inspect those results further.

Global IP search MAY be dynamic, but it MUST run against local committed feed
state. The implementation SHOULD reuse local cached filesets / indexes where
safe, and it SHOULD avoid extra request-time history scans once the exposed
`first_seen` contract has been satisfied.

When search exposes `first_seen` for a currently matching IP, that timestamp
MUST mean the start of the current contiguous listing interval for that exact
feed. It MUST be derived from engine-owned retention cohorts
(`lib/{feed}/new/{timestamp}`), not from downloader-owned `data/history/...`
snapshots and not from feed-level `lib/{feed}/history.csv`.

### Feed-scoped search

Given an IP and one feed, the public site MUST be able to answer membership for
that feed.

When the same best-effort IP context is available for the looked-up IP, the
feed-scoped search surface SHOULD expose the same contextual facts the global
lookup does, including geography, ASN attribution, and geographic-map context.

The search experience MUST be shared and consistent across surfaces.

When the same search contract appears in multiple public surfaces, the product
MUST reuse the same search component and result rendering path unless a real
surface-specific difference is required.

## Methodology contract

Every public metric, classifier, and deterministic insight SHOULD have a
methodology document.

Methodology pages MUST be:

- independently addressable
- human-readable
- linkable from the relevant public UI surface

Public methodology pages MUST explain the user-facing meaning of a signal and
its interpretation limits. For critical-infrastructure methodology this means
the page MUST explain what critical infrastructure means, hard/soft/contextual
levels, why each level matters, current included coverage, strengths,
weaknesses, missing/deferred coverage, and false-positive/false-negative risks.
It MUST NOT be written as an implementation guide. Config schemas, code paths,
artifact filenames, migration history, internal validation mechanics, and SOW
decision records belong in operator docs, specs, or SOWs unless a brief link is
necessary for user interpretation.

The methodology API endpoints under `/api/v1/methodology` MUST expose
structured machine-readable page data such as slug, title, summary, and
rendered body content. The public UI MUST NOT depend on scraping wrapped
HTML documents from those API routes.

## Public visualization contract

Any public chart, map, treemap, network graph, timeline, or other quantitative
visualization MUST be truthful about what it can and cannot say.

This contract applies to feed-detail analytics, homepage visuals, lookup
visuals, and any future public visualization surface.

### Interpretation rules

Public visualizations MUST disclose enough local context to prevent a careful
reader from drawing a materially wrong conclusion from the visual alone.

At minimum, a visualization MUST make clear, either in the local UI or via an
immediately linked methodology page:

- the unit of measure
- the denominator or population when percentages/shares are shown
- any exclusions, unmapped populations, or hidden cohorts that materially
  affect interpretation
- any transformation that changes how the data is visually encoded
  (log scaling, cumulative rendering, clipping, normalization, bucketing, and
  similar)

When a public visualization compares one feed against peer feeds, the local UI
MUST expose the current health state of those peers whenever stale peers could
materially change interpretation of a strong relationship.

At minimum:

- overlap lists/tables MUST show the current health of each compared peer feed
- when a feed that is itself neither `archived` nor `unmaintained` has
  structural overlap with peers that are currently `archived` or
  `unmaintained`, the overlap surface MUST show a local warning that strong
  containment or overlap can reflect stale upstream composition as well as
  fresh independent agreement

If color, area, position, thickness, or other visual encoding carries meaning
that is not obvious from surrounding labels, the visualization MUST provide a
legend, axis, caption, or equivalent local explanation.

When a public map renders country shapes and the product exposes dedicated
country-detail routes, those rendered countries MUST be navigable to their
country-detail page directly from the map.

When a public visualization or table renders ASN entities and the product
exposes dedicated ASN-detail routes, those rendered ASNs MUST be navigable to
their ASN-detail page directly from that surface.

Shared public visualization assets such as vendored map or topology files MUST
be served from the embedded website static tree. Public visualizations MUST NOT
depend on third-party CDNs or on engine-generated output paths for those shared
runtime assets.

### Visualization state model

The public UI MUST distinguish the following states when they are semantically
different for a given visualization:

- loading
- failed to load
- empty but valid
- not enough history yet
- not applicable
- partial or incomplete
- fully available

The UI MUST NOT collapse distinct states into one generic message when that
would hide meaning from the viewer.

When two related visualizations are presented side-by-side, the public UI MUST
keep corresponding structural rows aligned across the pair. Titles,
descriptions, local notices, and chart bodies MUST line up horizontally so one
panel's explanatory notice cannot push its chart lower than the sibling panel.

### Partiality and coverage rules

If a visualization is built from a partial observation window, excludes known
cohorts, lacks attribution for part of the population, or otherwise represents
less than the full conceptual dataset, the UI MUST disclose that locally.

Examples include:

- age/freshness windows that cannot see the true first appearance of older IPs
- retention distributions built only from observed removals
- provider-attributed views with unmapped or unattributed remainder
- merge/comparison views with dynamic exclusions

If showing the visualization would be materially misleading without a local
explanation, the UI SHOULD render an explanatory info box instead of the chart.

### Time-anchor rules

Time-based visualizations MUST disclose whether their values are:

- as of the last published artifact time
- dynamically aged forward to the viewer's current time

If client-clock assumptions materially affect the displayed interpretation, the
UI SHOULD surface that fact locally.

### Precision and rendering rules

Rendering convenience MUST NOT silently change user-visible meaning.

If a rendering transform is required for visual reasons but would otherwise
change the label users read, the UI MUST preserve truthful visible wording.

Examples:

- a zero-hour bucket rendered at `1` for a log axis MUST still be labeled
  `< 1 hour` or an equivalent truthful approximation
- clipped tails, grouped buckets, or rounded percentages MUST not imply false
  exactness

### Provider visibility rules

When a visualization supports multiple configured providers, the UI MUST NOT
silently hide a configured provider solely because its current payload is
missing, empty, malformed, or non-overlapping for the current feed.

Instead, the public UI SHOULD preserve provider visibility and explain the
provider-local state, unless the provider is not configured for public use at
all.

## Resilience contract

The public site MUST degrade gracefully when one page section or one provider
payload is unavailable or malformed.

This means:

- one broken section SHOULD NOT take down the whole page
- partial data SHOULD degrade locally where possible
- public pages SHOULD continue to explain what is still available

## Separation from admin

The public website MUST remain separate from the admin/operator interface.

Public users MUST NOT be exposed to:

- operator-only queue state
- operator actions
- authenticated-only controls

## Non-normative: current reference stack

The current implementation uses a React-based SPA with TypeScript, client-side
routing, data fetching/caching libraries, charting libraries, map rendering
libraries, and a utility-first CSS stack.

These choices are replaceable as long as the product continues to satisfy the
contract above.
