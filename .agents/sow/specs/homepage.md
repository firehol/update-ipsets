# Homepage Contract

## Status

This document is normative unless a section explicitly says **Non-normative**.

The words **MUST**, **MUST NOT**, **SHOULD**, and **MAY** describe the required
behavior of the homepage.

This document refines the homepage surface described in `.agents/sow/specs/website.md`.

## Purpose

The homepage is the primary product surface of the observatory.

It MUST let a first-time visitor:

1. understand what the site is and why it exists
2. look up one IP
3. explore every tracked feed with enough power to evaluate feeds against
   each other

The homepage MUST be treated as a tool first and a presentation second.

The homepage MUST absorb the full feed inventory. The previously separate
catalog surface is retired.

## Audience

The homepage MUST serve, on the same page, visitors with different goals and
different expertise levels. Named audiences include:

- end users who consume feeds
- feed maintainers comparing their work to peers
- security program staff (SOC, detection engineering, network security,
  incident response, threat intelligence, vulnerability management,
  compliance, cloud security, fraud prevention)
- researchers
- organizations evaluating whether a feed fits their context

The homepage MUST serve both novice visitors who want guided entry points
and experienced visitors who want direct analytical control.

## Page structure

The homepage MUST be composed of three zones, in this order:

1. Hero zone
2. IP lookup zone
3. Feed explorer zone

Each zone MUST render independently and degrade gracefully if its own data
is unavailable.

---

## Zone 1: Hero

### Purpose

The hero MUST explain what the site is.

The hero MUST NOT present itself as the IP lookup tool or as the feed
explorer. Those are separate zones.

### Required content

The hero MUST include:

- a short mission statement
- a live platform stats strip covering at least: feeds tracked,
  maintainers represented, categories covered
- a primary call-to-action that moves the visitor into the feed explorer

For the current launch contract, the hero headline MUST present the homepage
as:

- `All Cybercrime IP Feeds`

The hero's `feeds tracked` stat MUST count the full public feed inventory
available for homepage browsing. It MUST NOT reuse the narrower homepage
aggregation subset defined later in this document.

### Visual

The hero MUST follow the editorial design language defined in
`.agents/sow/specs/website.md` and `.agents/sow/specs/design.md`. The hero MUST be
typography-driven. The hero MUST NOT contain the interactive globe.

---

## Zone 2: IP Lookup

### Purpose

The IP lookup zone MUST allow a visitor to answer "what do you know about
this IP?".

This zone is the primary home of the global IP search contract defined in
`.agents/sow/specs/website.md`.

For the current launch contract, the zone headline MUST read:

- `Search any IPv4 address.`

### Required result content

For a looked-up IP, the zone MUST show:

- country of origin with a recognizable country indicator
- position on a geographic map
- ASN number and ASN name
- the list of feeds that match the IP, grouped by category
- per-match context: when available, the first-seen and last-seen time of
  the IP in each matching feed

For the current product contract, this timing context is sourced from the
existing local evidence model:

- `last-seen` is the timestamp of the current matching feed body
- `first-seen` is exposed only when downloader-owned retained history
  snapshots exist for that feed family and contain the IP
- synthetic feed families without retained per-IP history MAY omit
  `first-seen`

For an IP that matches no feeds, the zone MUST still present the geography
and ASN facts and MUST explicitly state that the IP is not currently
tracked in any feed.

If the configured geo provider resolves the IP to a special non-country marker
(for example a provider-specific `COUNTRYLESS` / unknown / special-use bucket),
the zone MUST render that fact safely as text-only geography context. It MUST
NOT assume such a marker is an ISO country code, MUST NOT crash localizing it,
and MUST NOT pretend there is a mappable country when there is not one.

When the IP lookup renders a country map for a real ISO country code, the
rendered countries MUST be clickable so the user can navigate directly to the
country-detail surface from the map.

The homepage IP lookup MUST use the same shared global IP-search surface and
result renderer the rest of the public site uses. Homepage-specific framing MAY
wrap that shared surface, but the search interaction and result semantics MUST
not drift into a separate bespoke implementation.

When the homepage loads with no explicit `ip` query parameter, it MUST be able
to seed the shared search field with the daemon's current view of the client
IPv4 address when such an address is available. This bootstrap rule is
normative:

- it MUST NOT overwrite an explicit `ip` query parameter
- it MUST NOT overwrite a non-empty field the visitor has already edited
- it MUST NOT auto-submit the lookup by itself
- it MAY expose a local hint such as "detected from your connection"

### Optional decorative background

This zone MAY use the interactive globe as a scoped background element. If
used:

- the globe MUST respond to the looked-up IP (pin drop on the relevant
  country)
- the globe MUST NOT couple its state to feed hovers or other interactions
  in the explorer zone
- the globe MUST NOT degrade text readability in the zone
- the globe MUST NOT continue to run persistent WebGL work outside this
  zone

The globe background is non-normative. The zone MUST fully function
without it.

---

## Zone 3: Feed Explorer

### Purpose

The feed explorer is the main product surface of the site. It MUST let
visitors explore every tracked public feed and evaluate feeds against each
other.

The feed explorer MUST absorb the previous catalog surface. No separate
catalog surface is required.

The explorer inventory, explorer counts, and explorer filters MUST operate on
the full public feed inventory. The narrower homepage aggregation filter is for
rollups only and MUST NOT hide otherwise public feeds from explorer browsing.

### Entry surface: preset lenses

The explorer MUST provide a discoverable entry surface of curated lenses.

A lens MUST be an opinionated view that sets a meaningful default
combination of filter, sort, and view mode, in service of a recognizable
visitor intent.

A lens MUST be a navigable entry point into the faceted surface, not a
terminal destination. A visitor who enters through a lens MUST be able to
refine the view further using the full set of filters.

The explorer MUST treat the active lens highlight as truthful UI state, not
just as "the last tile the visitor clicked". If the visitor manually changes
filters, sort order, or view mode so the live explorer state no longer matches
the selected lens preset, that lens tile MUST unselect.

The set of lenses is curator-controlled and MAY evolve. At launch it
SHOULD cover at least the following intents:

- most recently active feeds
- feeds with the lowest overlap with other feeds
- broadest feeds by address-space coverage
- a small curated set of high-credibility starter feeds
- browse by threat type or category
- browse by maintainer

Until richer maintainer-credibility records exist, the starter-feed intent is
satisfied by a factual lens over feeds that are both:

- `primary`
- redistributable

Lenses MUST NOT use editorial verdicts ("best", "reliable") as their
framing. Lens framing MUST be phrased as a fact or a dimension the visitor
is interested in.

The lens entry surface MUST lay out without horizontal overflow at supported
desktop widths. If multiple lens tiles are shown together, their descriptions
MUST wrap cleanly and the tiles SHOULD align on a stable wrapping grid rather
than degrading into a single overflowing strip.

### Faceted surface

The explorer MUST support faceted filtering across the full set of tracked
feed dimensions. The visitor MUST be able to filter by at least:

- category
- threat type or intent
- maintainer
- maintainer credibility tier, when maintainer records expose one
- feed size
- update cadence
- freshness and recency
- health class
- provenance
  - canonical values are `primary`, `secondary_upstream`, `secondary_merge`,
    and `secondary_retention`
  - public labels MAY render these as Primary, Upstream, Merge, and Retention
- critical-infrastructure reference tier for feeds that are themselves
  configured critical reference feeds
  - canonical values are `hard`, `soft`, and `contextual`
- critical-infrastructure overlap tier for normal feeds whose published
  critical-overlap artifact reports positive overlap
  - canonical values are `hard`, `soft`, and `contextual`
- uniqueness
- license and redistributability
- free-text search against feed name, maintainer, and description

Filters MUST be freely combinable. The resulting filter state MUST be
URL-encoded so any filtered view is shareable.

Every explorer filter control MUST expose an accessible name. Visual group
labels are not enough when the actual input is a native `select`, text input,
number input, or button group.

Critical-infrastructure filters MUST use typed API fields derived from
configuration and published overlap artifacts. They MUST NOT infer critical
status from feed-name substrings or generated artifact filenames.

The homepage explorer MUST start with this default health selection:

- `healthy`
- `delayed`
- `risky`
- `unavailable`

The following health classes MUST remain available in the filter UI but MUST
NOT be selected by default:

- `archived`
- `unmaintained`
- `empty`

For the current product contract, the following filter semantics are
normative:

- when the URL does not specify a health filter, the explorer uses the default
  health selection above
- the current web contract MAY use an explicit URL sentinel to represent
  "all health classes / no health restriction" so shareable URLs can still
  express a fully unfiltered health view
- update cadence is bucketed by observed average update interval when available,
  otherwise by configured frequency
- freshness filters, the freshest sort, and timeline bucketing all use the same
  timestamp chain: `source_date`, otherwise `processed_date`, otherwise
  `checked_date`
- uniqueness is bucketed by `unique_share_pct` bands
- license filtering matches the exact configured license string
- redistributability distinguishes redistributable from
  non-redistributable feeds

### View modes

The explorer MUST support multiple view modes over the same filtered
result set.

At minimum the explorer MUST support:

- a dense sortable table view for analytical comparison
- a rich card grid view for browsing

In card/grid browsing, feed names MAY be visually truncated to preserve card
layout. When a name is truncated, the UI MUST expose the full name through the
application tooltip system. Browser-default `title` tooltips are not sufficient
for this interaction.

The explorer SHOULD additionally support view modes such as:

- a category treemap
- a pairwise overlap matrix
- a freshness or activity timeline
- a maintainer-grouped view
- a two-dimensional world map view
- an interactive globe view

The view mode MAY be user-selected. The selected view mode SHOULD persist
across sessions at a per-visitor level.

The interactive globe MAY appear as one of the view modes. When used as a
view mode the globe MUST be full-bleed and interactive, and MAY couple its
state to feed hovers inside the explorer.

### Default state

When a visitor lands on the explorer with no URL state:

- the default view mode SHOULD be browsing-friendly rather than
  analyst-dense
- the default surface MUST show some opinionated content (for example the
  lens strip) so a first-time visitor has a reason to scroll further

### Result count and paging

The explorer MUST NOT truncate the tracked feed set for browsing. Every
feed that passes the active filter MUST be reachable from the visible
surface without page navigation. Virtual scrolling and similar techniques
are acceptable implementations; page-based "show more" navigation is
discouraged.

### Mobile behavior

On mobile devices the explorer MAY simplify to a browsing-first experience:

- category chips or search as the primary control
- a card list as the visible view mode
- analytical view modes such as overlap matrix or globe MAY be omitted

Mobile simplification MUST NOT hide feeds that would appear in the desktop
view for the same filter state.

---

## Shared rules

### Aggregation filter policy

Any aggregation rendered on the homepage (totals, rankings, lens
populations) MUST apply the following filters before aggregating:

1. exclude system-role categories (`asn`, `geolocation`) and any other
   non-public role registered in configuration
2. include only feeds whose health class is `healthy` or `delayed`
3. include only feeds whose provenance is `primary` or `secondary_upstream`;
   merge and retention derivatives MUST be excluded from aggregations to avoid
   double-counting the same IPs

This homepage aggregation filter is intentionally narrower than the admin
filter surface.

`risky`, `unavailable`, `archived`, `empty`, and `unmaintained` feeds remain
first-class catalog entries and MUST remain directly navigable, but they are
excluded from homepage rollups so public aggregate totals do not present
unstable or degraded population counts as the current summary view.

Feed-detail navigation MUST NOT apply these filters. A direct link to any
feed MUST resolve regardless of the feed's health, provenance, or category
role.

### Category semantics

Category labels, descriptions, colors, and ordering MUST come from
configuration as defined in `.agents/sow/specs/config.md`. The homepage MUST NOT
hardcode category semantics or category palettes.

Categories marked non-public in configuration MUST NOT appear as homepage
filter chips, default explorer categories, or public homepage aggregation
inputs.

### Visual language

The homepage MUST follow the editorial design language defined in
`.agents/sow/specs/design.md` and `.agents/sow/specs/website.md`. In particular:

- one accent hue only
- restrained chrome (hairline separators, no heavy card shadows)
- factual presentation without editorial verdicts
- silence for empty states

### Presentation rules

The homepage MUST NOT present:

- editorial verdicts such as "best", "reliable", or "recommended"
- confidence hedging such as "likely", "possibly", or "may be"
- placeholder content for sections that currently have nothing to show
- features that are not yet available

---

## Public detail surfaces

Ranked entries and explorer rows MUST link, where applicable, to the
following public detail surfaces:

- country detail
- ASN detail
- maintainer detail
- feed detail

These surfaces are described in `.agents/sow/specs/website.md`.

## Data contract

The homepage MUST be backed by a stable public data contract. At minimum,
the public data layer MUST expose:

- a homepage summary payload covering totals, top countries, top ASNs, and
  top maintainers under the active category filter
- an explorer payload covering the full filterable feed inventory with
  enough fields per feed to populate every required view mode and filter axis,
  including cadence, uniqueness, license, redistributability,
  critical-reference tier, and critical-overlap tiers
- a global IP lookup payload sufficient to satisfy the Zone 2 result
  requirements
- the per-country, per-ASN, per-maintainer, and per-feed detail payloads
  required by the detail surfaces the homepage links to

Exact endpoint shapes are part of the implementation contract maintained
alongside the code. The public data contract MUST remain stable across
implementation changes.

Homepage summary and globe rollups MUST be precomputed as publication
artifacts. The producer MUST refresh the aggregate during normal publication
runs and during repair/background paths that can change the visible rollup
inputs, including health-driven eligibility changes. Public route handlers
MUST read the published aggregate and MAY only do bounded in-memory composition
for the requested category filter and limit.
Entity-integrity validation MUST detect missing, malformed, or stale homepage
aggregate artifacts once public feed state exists, and the corresponding repair
path MUST republish the aggregate without relying on a public request to rebuild
it.

## Retirement of the catalog surface

The previously separate catalog surface is retired. The homepage MUST
cover every function the catalog served: browsing, sorting, filtering, and
direct access to every tracked feed.

Implementations MAY keep the previous catalog URL as a redirect into the
explorer, but MUST NOT rely on a separate catalog surface to satisfy any
part of this contract.

## Non-normative: reference implementation

The current reference implementation is a React SPA. The homepage is a
single route. The implementation uses client-side routing, a query cache,
and charting and map libraries for timeline, world-map, and globe view
modes.

These choices are replaceable as long as the product continues to satisfy
the contract above.
