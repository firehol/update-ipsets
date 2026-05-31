# Feed Model Contract

## Status

This document is normative.

The words **MUST**, **MUST NOT**, **SHOULD**, and **MAY** describe the required
behavior of the product.

## Purpose

This document defines what the product knows about feeds, what it preserves
about them over time, how feed families differ, and which files and published
artifacts belong to each feed.

## Feed families

The product MUST recognize these feed families:

### 1. Plain feeds

- directly acquired from upstream
- produce a canonical feed body from upstream material
- may retain a raw `.source` for debugging
- later produce downstream published artifacts after processing

### 2. History derivatives

- depend on one parent feed
- are prepared by the downloader immediately after the parent update
- combine the fresh parent feed body with downloader-owned retained history
  snapshots for that parent
- represent the additive union of IPs observed in the parent during the last `X`
  days
- do not progress independently with wall-clock time; they move only when the
  parent updates
- require a parent that itself produces a canonical public feed body; supporting
  provider databases are not valid parents
- a newly introduced history derivative is valid from its first successful
  parent-derived snapshot onward
- until enough retained history exists to cover the full configured window, it
  represents the currently observed
  partial window rather than an integrity failure
- history derivatives use whatever successful parent-derived snapshots are
  available; they do not assume one rollup exists for every calendar day

### 3. Merges

- depend on multiple input feeds
- may declare additive inputs and subtractive inputs
- are prepared by the downloader from the latest durable local canonical feed
  bodies of their currently enabled sources
- contain `union(additive inputs) - union(subtractive inputs)`
- do not fetch their own upstream body directly
- are the only synthetic public feed family that progresses purely because time
  passed

### 4. Artifact-backed child feeds

- derive their input from one artifact parent
- are first-class feeds operationally, but not first-class upstream downloads

### 5. Artifact parents

- are not public feeds
- are upstream artifact families that materialize one or more child feeds

### 6. Provider databases

- support enrichment such as ASN or geolocation
- are supporting datasets, not normal public feeds

## Feed identity

Every feed MUST have a stable identity defined by configuration.

For authored catalog maintainability, each direct source feed and each merge
feed MUST have its own catalog fragment under `configs/firehol/`.

That identity MUST be sufficient to determine:

- its public name
- its category
- its source family
 - its provenance
- its update policy
- its legal and attribution policy
- its visibility

The product MUST NOT rely on hardcoded feed names in order to know what kind of
feed something is.

## Feed provenance

Feed provenance is distinct from feed family.

It answers how a public feed should be presented to users when they browse the
catalog and explorer.

The canonical provenance values are:

- `primary`
- `secondary_upstream`
- `secondary_merge`
- `secondary_retention`

Meaning:

- `primary`
  - a first-order public source feed
- `secondary_upstream`
  - a public feed that still behaves like a source feed, but is curator-marked
    as mirrored, upstream-derived, or otherwise secondary to another public
    source relationship
- `secondary_merge`
  - a merge feed
- `secondary_retention`
  - a history derivative

Public surfaces MAY render the secondary values with shorter labels such as
`upstream`, `merge`, and `retention`, but the product-wide canonical values
remain the four values above.

## What the product knows about a feed

For each feed, the product MUST preserve enough information to answer these
questions:

### Catalog meaning

- what the feed is called
- what category it belongs to
- what provenance it has
- who maintains it
- what the operator and public should know about it

### Acquisition state

- when it was last checked
- when it last changed
- whether it is currently failing
- how long the current failure streak has lasted
- whether a staged or processing feed body is waiting for processing

### Processing state

- when it was last successfully published locally
- why it last ran
- how long the last processing run took
- whether it currently has actionable local input for rebuild

### Observed behavior

- how large it is
- how often it changes
- how much it rotates or churns
- how long entries remain listed

### Derived analysis

- geographic distribution
- ASN distribution
- bogon overlap
- feed-to-feed overlap
- deterministic insights derived from the above

### Legal/publication policy

- what license or terms are known
- whether redistribution is allowed
- what attribution must accompany publication

### Public researched context

- official feed/project name and URL
- short and long public descriptions
- feed roles and maintainers
- derivation/source-feed relationships
- listing and unlisting policy
- upstream-stated update cadence, distinct from scheduler polling cadence
- detection method and scope/intended use
- current status and successor relationships
- community/reputation signals
- sources consulted and last research timestamp

Rules:

- This researched context MAY be embedded in source or merge YAML under
  `enrichment:`.
- Embedded enrichment is public metadata. It MUST NOT contain internal agent
  audit fields, raw private research notes, or sensitive data.
- The scheduler `frequency` remains the operational polling cadence. The
  enrichment `update_frequency` is maintainer-stated or researched upstream
  cadence and MUST NOT be treated as an automatic scheduler override.
- Public surfaces (feed-detail UI and feed markdown) MUST NOT render
  `scope_and_intent.not_intended_for`. The Web UI feed-detail page renders
  an editorial opening from `long_description` with an "Intended for"
  sidebar from `scope_and_intent.intended_for`, a fact-card "Method" section
  from `derivation` and `detection_classification`, a "How IPs get on and
  off this list" section from listing/unlisting/removal policies (positioned
  above Overlap), a quiet "Reputation and community signals" block below the
  technical specs, and a folded "Sources consulted" block at the very end
  with the researcher evidence and last-researched timestamp. The markdown
  mirrors the same order, with Reputation and Sources consulted appended
  after the technical specifications.

## Feed time model

Each feed MUST preserve at least these distinct time concepts:

### Last check

When the system most recently attempted to refresh the feed's input freshness.

### Last upstream change

When the system believes the input content last changed upstream, when that can
be observed.

### Last successful local publication

When the system last successfully produced and committed the feed's outputs.

This is the authoritative local success timestamp for integrity.

### Failure streak start

The lower bound for the beginning of the current continuous failure period.

## Feed health contract

The product MUST classify feed health in the backend, not in the UI.

The health model MUST distinguish at least:

- healthy
- delayed
- risky
- unmaintained
- empty
- unavailable
- archived

The health model MUST consider:

- observed update cadence
- configured healthy/risky cadence floors, including category-specific
  thresholds
- single-observation grace
- current failure streak
- successful empty publication as distinct from upstream unavailability
- configured archival threshold for prolonged continuous `unavailable`

Before the first successful local publication, an enabled feed is classified as
`unavailable`.

This pre-publication `unavailable` state means the product does not yet have a
successful local version of the feed. It is expected to be brief for newly
added healthy feeds because they are due immediately.

After a feed has already had a successful local publication, it MUST also be
classified as `unavailable` when it is currently in a local unavailability
state and remains beyond the configured recovery threshold without a usable
local refresh. This covers settled local unavailability such as a continuing
download/provider failure or a stale last successful change that has aged past
the threshold while the feed is still in an unavailable state.

If a feed remains continuously `unavailable` beyond the configured archival
threshold, its health class MUST become `archived`.

For an unavailable feed, the archival decision MUST consider how long the
product has gone without a usable local refresh, not only the currently tracked
failure streak. If the last successful local refresh is already older than the
ordinary unavailable threshold plus the archival threshold, the feed MUST move
to `archived` immediately while it is in an unavailable state, even if the most
recent downloader failure streak started later.

`archived` replaces `unavailable` on the health axis; the two MUST NOT coexist
for the same feed state.

`archived` is a derived health state, not a curated per-feed configuration flag.

An archived feed MUST be excluded from ordinary automatic retry scheduling, but
an explicit operator `recheck` MAY still refresh it and allow it to leave
`archived` naturally if the refresh succeeds.

If a feed is marked as excluded from age-based unmaintained classification:

- `empty` still applies
- `unavailable` still applies
- `archived` still applies
- age-based states (`delayed`, `risky`, `unmaintained`) are suppressed

The same age-based suppression applies automatically to configured
reference/provider roles where content stability is not a threat-feed freshness
signal: `critical_infrastructure`, `provider_context`, `asn`, and `geoip`.
These roles can still be `empty`, `unavailable`, or `archived`; only the
timestamp-only freshness ladder is suppressed.

The UI and APIs MUST consume the backend health classification directly and MUST
NOT independently invent health states from raw timestamps.

For one-parent derivatives, the operator-facing health/freshness view MUST
follow the parent feed rather than the local derivative rebuild timestamp.

## Enable and disable semantics

### Normal feeds

A normal feed is operationally enabled when:

- the operator has enabled it
- or a global "enable all" mode is in effect

Normal feed enablement MUST be represented by a dedicated source enable marker,
not by the presence of a raw `.source` file.

### History derivatives

A history derivative MUST follow its parent.

If the parent is disabled, the derivative is operationally disabled.

For operator-facing health/freshness semantics, a history derivative MUST follow
the parent's health.

### Merges

A merge MUST be evaluated only against inputs that are both:

- currently enabled
- not currently excluded from merge composition by health policy
  (`archived` and `unmaintained`)

Configured subtractive inputs are hard dependencies for any merge that has at
least one eligible additive input. A subtractive input that is disabled,
archived, unmaintained, or missing MUST fail composition rather than silently
broadening the published set.

If any currently eligible input lacks a durable local canonical feed body, the
merge composition attempt MUST fail.

If no additive inputs remain currently eligible for composition, the merge is
operationally disabled for composition.

Public and operator-facing merge detail surfaces MUST expose the merge's current
included inputs, current subtracted inputs, current health-excluded inputs, and
exclusion reasons.

Merge-derived feeds MAY carry ipset-compatible `use:` roles. When they do, the
expanded feed participates in the same provider lists and generated artifacts as
plain sources with that role.

### Artifact-backed child feeds

An artifact-backed child feed is operationally enabled only when:

- the child itself is enabled
- and its artifact parent is enabled

### Artifact parents

An artifact parent is the master switch for its child family.

Disabling the parent MUST disable its child feeds operationally, even if the
children remain individually marked as enabled.

Children MUST NOT control whether the parent itself is enabled.

### Provider databases

Provider databases follow normal source enable/disable semantics.

If a provider database is disabled:

- the downloader stops refreshing that provider
- the processing engine stops using newly refreshed data from that provider
- previously published artifacts derived from older successful provider data MAY
  remain authoritative until later publication replaces them

## Feed lifecycle

These lifecycle stages are conceptual product stages, not the live downloader/
processing queue state exposed by the admin runtime.

The feed lifecycle MUST support these stages:

1. known in configuration
2. enabled or disabled
3. waiting for downloader-stage work or not applicable
4. active in downloader-stage work or not applicable
5. waiting to process
6. processing
7. committed and published
8. settled exceptional condition such as failed, unavailable, or empty as
   applicable

The system MUST preserve committed good state across failures.

For stages 3 and 4, `not applicable` means the feed has no downloader-stage
work of its own pending at that moment, for example because:

- the feed is disabled
- its downloader work is parent-driven and no parent-triggered refresh is
  currently pending
- it is a non-autonomous item that only runs on explicit operator action

## File ownership model

This section defines which file families exist and what they mean.

The detailed behavior of the subsystems that create, stage, promote, or consume
those files is owned by:

- [downloader.md](downloader.md)
- [processing-engine.md](processing-engine.md)

The product maintains two kinds of files for feeds:

1. **local operational files**
2. **public published artifacts**

## Local operational files

Each processable feed MUST be able to own the following file concepts:

### 1. Source enable marker

A durable operator-controlled source enable marker separate from raw input and
canonical feed-body files.

The concrete path contract is defined in
[files-layout.md](files-layout.md).

### 2. Committed feed body

The authoritative committed feed body from which the processing engine can
rebuild the feed locally.

This concept applies to every processable feed family, including:

- plain feeds
- history derivatives
- merges
- artifact-backed child feeds

### 3. Staged feed body

A complete staged feed body waiting to be processed and promoted.

### 4. Processing feed body

A claimed in-flight feed body in `.{ip,net}set.processing`.

### 5. Temporary local input scratch state

Incomplete scratch state that MUST NOT be treated as valid.

### 6. Downloader-owned history snapshots

For feed families that need downloader-side retained history, the product MUST
be able to keep downloader-owned history snapshots separate from engine outputs.

For one parent feed:

- a snapshot represents one successful parent update timestamp in
  `{unix_timestamp}.set` form
- multiple successful parent updates MAY therefore produce multiple retained
  snapshots in the same UTC day
- a period with no successful parent update produces no new snapshot
- these snapshots are sparse by observed successful parent updates, not dense by
  calendar day
- these snapshots belong to the downloader side and exist to support history
  derivatives

### 7. Historical evidence

Persistent history used to describe change over time and to support derivatives.

This includes:

- point-in-time summaries
- change summaries
- engine-owned historical evidence and retention facts

Downloader-owned history snapshots and engine-owned historical/retention evidence
MUST remain separate concepts.

Clarification:

- downloader-owned `data/history/{parent}/{timestamp}.set` snapshots exist to
  support history-derivative composition and repair
- engine-owned `lib/{feed}/history.csv` records feed-level size/evolution over
  time, not per-IP listing age
- engine-owned `lib/{feed}/new/{timestamp}` retention cohorts record the start
  of the current contiguous listing interval for currently listed IPs
- engine-owned `lib/{feed}/retention.csv` records the age of removed IP
  cohorts at the time they were removed

## Public published artifacts

For each public feed, the product MUST be able to publish:

- a public metadata summary
- bounded public history
- bounded public change/churn history
- retention summaries
- feed comparison results
- deterministic insights
- per-provider enrichment views where available
- critical-infrastructure reference-feed overlap summaries where configured
- redistributable set files when licensing permits

Feed-scoped published artifacts are generated by processing or explicit
reprocess/repair work. Public routes for history, changesets, retention,
comparison, provider enrichment, critical-infrastructure overlap, and insights
MUST serve or read the already published files and return a clear missing
response when absent; they MUST NOT generate missing artifacts from internal
ledgers, cache state, or feed bodies on first request.

Critical-infrastructure aggregate artifacts MUST identify the configured
provider set used to build them and whether the build was complete. Public
routes MUST serve structurally valid published aggregate artifacts even when
their `provider_set_id` no longer matches current config, because the public
surface is cache-first and provider-set drift is an admin integrity concern.
UI/API consumers MUST treat incomplete artifacts as incomplete rather than as
evidence of no overlap.

Critical-infrastructure aggregate artifacts MAY include an `asn_context` section
when configured critical ASN context matches the feed's ASN attribution payload.
This section is secondary context only. It MUST NOT increase `critical_ips`, and
UI/API consumers MUST present it separately from reference-feed overlap.

Provider-context feeds are public context feeds, not critical-infrastructure
reference providers and not critical-overlap targets. They help operators
inspect broad cloud/hosting exposure without turning customer-hosting ranges
into default critical warnings.

The exact filenames and directory layout are owned by
[files-layout.md](files-layout.md). The existence of these artifact classes is
part of the product contract.

## File staging and promotion

For every file class that is written as part of active work:

- incomplete writes MUST use temporary paths
- complete but uncommitted work MUST use staged paths
- only successful work may replace committed authoritative state

This applies to:

- downloaded inputs
- artifact parent inputs
- materialized child feed inputs
- mirror outputs
- public published files written as part of a batch

## Artifact parent storage

Each artifact parent MUST have its own local delivery area separate from normal
feed identity.

That storage MUST support:

- committed parent input
- staged parent input
- child materialization outputs
- restart recovery

Child feeds MUST NOT need to know the implementation path of the parent's local
delivery area as part of their configuration semantics.

## What makes a feed rebuildable

A feed is locally rebuildable when the product has enough committed or staged
local input to regenerate its outputs without fresh upstream acquisition.

Examples:

- a plain feed with committed or staged local input is rebuildable
- a history derivative is rebuildable when its downloader-owned history snapshots
  still exist for the configured window
- a merge is rebuildable when every currently eligible input has a committed
  feed body available
- an artifact-backed child is rebuildable when its local materialized input
  exists

## Hidden feeds

The product MAY keep hidden feeds for reference.

Rules:

- hidden feeds MAY be excluded from normal public browsing
- hidden status does not by itself disable downloader, processing, or integrity
  behavior
- hidden status MUST NOT silently rewrite core feed history

## Archived feeds

Archived feeds remain first-class feed identities for reference, operator
visibility, and public analytical/detail surfaces.

Archived is not a visibility flag.

Rules:

- archived feeds remain visible to operators and users
- archived feeds remain eligible for public feed detail and analytical/reference
  surfaces
- archived feeds MUST disable operational feed URLs for public consumption:
  - the upstream source URL
  - the local raw feed download URL and any equivalent raw feed-body endpoint
- archived feeds remain distinct from `disabled`
- archived feeds MUST NOT silently accept future domain resurrection except
  through an explicit operator-triggered `recheck`

## Public truth contract for feeds

The product MUST present feeds as observed data sources, not as endorsed or
discredited sources.

This means:

- feed pages and APIs MUST explain what is observed
- they MUST NOT invent reputation scores without a separately specified model
- they MUST preserve attribution and redistribution semantics
