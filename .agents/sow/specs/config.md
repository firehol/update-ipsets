# Configuration Contract

## Status

This document is normative.

The words **MUST**, **MUST NOT**, **SHOULD**, and **MAY** describe the required
behavior of the product.

## Purpose

Configuration defines:

- what the product tracks
- how often it refreshes tracked items
- how feeds are grouped and described
- which items are downloadable, derived, merged, or artifact-backed
- how redistribution and attribution must be handled

Configuration is the authoritative source of feed identity and feed family.

## Authored catalog layout

The primary authored catalog MUST be a directory, not a monolithic YAML file.

The repository catalog lives under `configs/firehol/`. The installed active
catalog lives under `etc/config/` relative to the installation root.

Rules:

- each authored source feed MUST live in its own YAML file under
  `sources/<category>/<feed>.yaml`
- each authored merge feed MUST live in its own YAML file under `merges/`
- each artifact parent SHOULD live in its own YAML file under `artifacts/`
- shared registries such as `runtime`, `categories`, `renames`, and `deleted`
  MAY live in shared YAML files
- the loader MUST read catalog directories recursively
- the loader MUST merge all YAML fragments first and only then normalize,
  expand derivatives, inject synthetic sources, canonicalize outputs, and
  validate the resulting config
- individual feed fragments MUST NOT be required to be self-contained; they may
  reference categories, artifacts, and other feeds defined in other fragments

## Top-level configuration model

The product configuration MUST support these top-level concerns:

### Runtime settings

Controls operational behavior such as:

- directories and storage locations
- download concurrency and limits
- ingest concurrency ceiling across daemon acquisition, parsing, processing,
  heavy-phase, and background ingest pools
- source-processing concurrency
- heavy-phase concurrency
- engine-lane concurrency for serialized engine, integrity, and artifact
  maintenance work
- background-work concurrency
- public JSON/static artifact cache entry, byte, and per-file limits
- processing cadence
- publication and integration options
- public/admin web exposure settings
- trusted proxy header policy for client IP detection, including:
  - independent enable/disable of proxy headers (`X-Forwarded-For`, `X-Real-IP`)
  - independent enable/disable of Cloudflare headers (`CF-Connecting-IP`)
  - default: secure by default (no headers trusted, use `RemoteAddr`)
- health thresholds, including:
  - single-observation grace
  - default healthy/risky cadence floors
  - category-specific healthy/risky cadence floors
  - archival threshold after prolonged continuous `unavailable`

The runtime model MUST support `max_ingest_workers` as an optional ceiling for
ingest-side worker pools. When this value is greater than zero, the effective
download, DNS parsing, source-processing, heavy-phase, engine-lane, and
background worker counts MUST NOT exceed it. Public/admin request serving and
watchdog work MUST NOT acquire this ingest ceiling. A value of zero disables
the ceiling and leaves the per-domain runtime controls as the effective limits.

The runtime model MUST support `max_engine_lane_workers`. This controls the
bounded FIFO lane used by processing-engine runs, startup/operator integrity
refreshes, integrity-triggered reprocess admission, entity artifact repair,
entity refresh, entity rebuild, and generated-artifact cleanup. It defaults to
`1`, MUST reject negative authored values, and MUST be clamped by
`max_ingest_workers` when the ingest ceiling is enabled. It is intentionally
separate from `max_background_workers`: the engine lane limits admission and
serialization of broad engine-owned operations, while background worker counts
limit bounded fan-out inside an admitted operation.

The runtime model MUST support `push_to_git_timeout`, expressed in seconds.
This bounds each Git subprocess used by generated artifact publication,
including add, diff, commit, push, and auto-maintenance. The default is `600`;
zero or omission means the default; negative authored values are invalid.

Runtime resource-control integers that default when set to zero MUST reject
negative authored values during validation. This includes download/DNS worker
counts, processing/heavy/engine-lane/background worker counts, ingest ceiling,
scheduling interval controls, git publication timeout, download-error
suppression count, and public artifact cache limits. Zero keeps its existing
default or disabled meaning.

### Categories

Defines the public taxonomy used to group and describe feeds.

Category definitions MUST support at least:

- `label`
- `description`
- `color`
- `sort_order`

Category definitions MAY mark a category as non-public (for example
`public: false`).

Rules:

- omitted `public` means the category is public
- non-public categories remain valid configuration for system/provider roles
- category visibility is not a feed privacy control; source visibility is
  controlled by source roles such as `asn`/`geoip` and by source-level flags
  such as `hidden`
- public website surfaces MUST derive category visibility from configuration,
  not from hardcoded category names

### Artifacts

Defines downloadable artifact parents that are not themselves public feeds.

### Sources

Defines processable feeds.

The source model accepts `enabled_by_all` as catalog metadata retained from the
legacy catalog conversion. In the current application, runtime enablement is
controlled by enable marker files, explicit operator actions, and the daemon
`--enable-all` override. The `--enable-all` override treats every configured
source as enabled and does not filter sources by `enabled_by_all`.

The source model accepts both `processor` and `processor_raw`:

- `processor` is the normalized processing pipeline and takes precedence when
  present. A processor step MAY be either a scalar step name or a single-key
  mapping whose value becomes the step `args` map for argument-bearing
  processors such as `grep`, `csv_column`, `json_path`, and `regex`.
- `processor_raw` is a legacy single processor-name field retained from the
  bash-era catalog conversion. If `processor` is absent, the engine MAY use
  `processor_raw` as the one processing step. If `processor` is present,
  `processor_raw` is compatibility metadata and MUST NOT be treated as a
  second raw-archive pipeline.

### Merges

Defines synthetic feeds composed from other feeds.

### Embedded enrichment

Source and merge entries MAY carry an `enrichment:` block containing the public
AI-researched feed knowledge projection.

Rules:

- `enrichment:` is authored catalog metadata, not runtime state.
- It MUST contain only fields allowed by the public embedded-enrichment schema.
- It MUST NOT contain internal agent audit fields such as raw evidence text,
  maintainer quotes, assistant reasoning, confidence labels, or evidence IDs.
- Markdown-capable string fields MUST remain markdown strings. Conversion and
  cleanup tools MAY split paragraphs for readability, but MUST NOT change the
  factual claim without a new researched source update.
- Engine readers MUST tolerate entries with no `enrichment:` block.
- Engine readers MUST decode enrichment into typed public-schema fields,
  validate the public metadata that affects rendering, and strip unexpected or
  internal fields defensively before exposing enrichment through API, UI,
  markdown, or MCP surfaces.

### Defaults

Defines explicit catalog-level defaults for ambiguous provider families.

The product MUST support at least:

- `defaults.asn_provider`
- `defaults.geo_provider`

These values are configured source names, not public labels. Validation MUST
reject a default provider that does not exist or that does not carry the
matching `use:` role.

Rules:

- ASN and geolocation defaults MUST be explicit configuration when the catalog
  wants a canonical provider.
- Source-directory order MUST NOT decide the canonical ASN or geolocation
  provider when a default is configured.
- Provider-list APIs MUST return the configured default provider first and then
  preserve normal catalog order for the remaining providers.
- Canonical ASN/geolocation-derived artifacts, IP lookup context, homepage
  summaries, entity detail pages, and feed-detail default provider tabs MUST
  use the configured defaults.
- Changing a configured default provider is a pipeline-significant config
  change. The runtime MUST detect the drift and rebuild affected public
  feed/entity artifacts instead of waiting for the selected provider body to
  change upstream.

### Supporting registries

The product MAY define other top-level registries for auxiliary data, such as:

- typed critical-infrastructure reference-feed metadata
- rename cleanup mappings for historical feed-name changes
- explicitly deleted historical names

`renames` and `deleted` are local-state cleanup registries. They MUST migrate or
remove existing generated state during cleanup-enabled runs; they MUST NOT be
interpreted as public API aliases for old feed names.

The legacy top-level `infrastructure_asns` registry is no longer supported.
Critical-infrastructure warning truth MUST be modeled as normal sources or
merges with `use: [critical_infrastructure]` and typed `critical:` metadata, so
overlap checks operate on configured IP reference feeds instead of broad ASNs.
This release supports IPv4 critical-infrastructure reference feeds only; configs
with `use: [critical_infrastructure]` and `ipv: ipv6` MUST fail validation until
the IPv6 overlap writer is implemented.

Current ordinary source and merge validation accepts `ipv: ipv6`, but this is
not full public-pipeline IPv6 support. The shipped catalog is IPv4, public IP
lookup is IPv4-only, and the feed-body preparation path currently uses the
IPv4 canonical set parser. Treat `ipv: ipv6` as an accepted configuration value
that requires further implementation before operators can rely on IPv6 feed
processing end to end.

## Configuration responsibilities

Configuration MUST define product behavior at the semantic level.

This means configuration MUST be able to describe:

- what kind of item something is
- what cadence it follows
- whether it is public or hidden
- what transformation/output policy it follows
- what legal/publication policy applies

Configuration MUST NOT require product logic to hardcode feed identity in order
to know how the item should behave.

## Semantic classification authority

Configuration field names, `use:` roles, and typed metadata are the source of
truth for product semantics.

The application MUST NOT derive semantic meaning from substrings, prefixes, or
suffixes in configured feed names, provider names, or generated artifact
filenames. Examples of forbidden semantic inference include treating a name as
ASN, bogon, or critical-infrastructure data because it contains tokens such as
`_asn_`, `_bogons_`, or `_critical_`.

Exact configured-name lookup is allowed only as identity lookup. For example, a
route or integrity check MAY resolve `foo_critical_bar.json` by checking whether
`foo` is an exact configured public feed and `bar` is an exact configured
critical-infrastructure provider. It MUST NOT classify the artifact by scanning
for `_critical_` without those exact configured identities.

Generated artifact filenames are storage addresses. Their semantics MUST come
from config-backed artifact descriptors or equivalent typed metadata carrying
at least the artifact family, target feed identity, provider identity where
applicable, provider role, and validator/schema.

## Web exposure and URL contract

Runtime configuration and daemon startup configuration together MUST be able to
define:

- the public listener address
- an optional separate admin listener address
- the admin authentication mode
- the externally visible base URL of the public website
- the published feed-detail prefix used in generated metadata/artifacts

These are distinct concerns. The product MUST NOT treat:

- listen addresses
- public website base URL
- feed-detail publication prefix

as interchangeable values.

### Public website base URL

The product MUST support an explicit runtime setting equivalent to
`public_base_url`.

Contract:

- it identifies the externally visible base URL of the public website
- it MAY include a path prefix when the site is served below a subpath
- it MUST NOT include a feed-specific suffix such as `/ipsets/<name>`
- admin UI links to public website pages MUST be built from this value, not
  from same-origin assumptions

When the admin surface is exposed on a different listener from the public
website, the configured public website base URL MUST be authoritative for any
admin-to-public navigation.

### Published feed-detail prefix

The existing runtime setting equivalent to `web_url` remains a different
contract.

`web_url` is the published feed-detail URL prefix used by generated metadata and
feed-artifact outputs. It is not the generic website base URL.

Rules:

- `web_url` MAY include a feed-detail path prefix such as `/ipsets/`
- generated outputs MAY append a feed name directly to `web_url`
- UI routing and admin-to-public navigation MUST NOT assume `web_url` is the
  public website base URL

### Admin exposure settings

The product MUST support explicit admin exposure settings equivalent to:

- shared listener vs separate admin listener
- `admin_auth_mode=required|disabled`
- an additional explicit unsafe acknowledgment for unauthenticated admin mode

Rules:

- a separate admin listener MUST be opt-in
- when unauthenticated admin mode is selected, it MUST require a second explicit
  unsafe acknowledgment knob
- missing admin credentials MUST NOT silently disable authentication
- bind-address heuristics such as loopback-only MUST NOT be treated as a safety
  signal for enabling unauthenticated admin
- installer-generated units MAY deliberately set disabled admin authentication
  with the unsafe acknowledgment for an operator-controlled private-network
  deployment, but this is install policy and MUST be documented with the
  generated listener defaults and override path

## Source families in configuration

The configuration model MUST be able to express at least:

### 1. Direct upstream feeds

- acquired from HTTP, HTTPS, or local files

### 1a. Static config-backed feeds

- acquired from a source's `static:` YAML list
- used for small curated reference feeds where the data must be operator
  customizable without rebuilding the binary
- MUST NOT be implemented as IP/CIDR lists compiled into Go code
- when `frequency: 0` is used, scheduler snapshots MUST still compare the
  materialized source body with current config and queue the source when the
  configured `static:` body changes

### 2. Artifact-backed child feeds

- refer to an artifact parent plus one or more named deliveries from that parent

### 3. History derivatives

- one-parent time-window derivatives

### 4. Merges

- many-input synthetic feeds

### 5. Supporting provider datasets

- such as ASN or geolocation sources
- configured as source entries with provider-specific roles

## Use roles

The `use` field assigns a source or merge to an engine role. Valid roles are:

- `bogons`
- `critical_infrastructure`
- `provider_context`
- `asn`
- `geoip`

Role semantics:

- no `use` value means a normal public ipset/netset feed
- `bogons` is an ipset-compatible role; the feed or merge still produces a
  committed set and can be used as a bogon comparison provider
- `bogons` MUST be assigned only to maintained bogon reference providers. A
  feed name, title, or description containing "bogon" is not sufficient
  authority for this role. Stale themed lists MUST remain normal feeds so they
  do not make unrelated feed analyses look like they overlap bogon space.
- `critical_infrastructure` is an ipset-compatible role; the feed or merge still
  produces a committed set and can be treated as infrastructure reference data
- `provider_context` is an ipset-compatible role; the feed or merge still
  produces a public set, but is broad provider/customer-hosting context and MUST
  NOT be used as critical-infrastructure warning truth
- `asn` is a provider-database role, not a public ipset/netset feed role
- `geoip` is a provider-database role, not a public ipset/netset feed role
- merges MAY declare only ipset-compatible roles (`bogons`,
  `critical_infrastructure`, `provider_context`), because merge outputs are set
  files
- sources and merges MUST NOT combine `critical_infrastructure` with `bogons`
  because the public artifact semantics and UI meaning are different
- sources and merges MUST NOT combine `critical_infrastructure` with
  `provider_context`; exact reference warnings and broad provider context are
  separate signals
- sources MUST NOT combine `critical_infrastructure` with provider-database
  roles such as `asn` or `geoip`; critical references must produce normal
  ipset/netset artifacts

## Critical infrastructure metadata

Any source or merge that declares `use: [critical_infrastructure]` MUST declare
a typed `critical:` metadata block. Sources or merges without that use role MUST
NOT declare `critical:`.

Required fields:

- `tier`: one of `hard`, `soft`, `contextual`
- `role`: validated semantic role such as `public_dns_core`,
  `public_dns_extended`, `dns_root`, `dns_sink_infrastructure`,
  `public_time`, `cdn_edge`, `cdn_edge_shared`, `cloud_provider`,
  `cloud_customer_hosting`, `cloud_service_edge`, `cloud_service_tag`,
  `cloud_control_plane`, `cloud_service_google`, `cloud_proxy`,
  `developer_platform`, `dev_platform_saas`, `container_registry`,
  `payment_or_commerce`, `certificate_validation`, `software_update`,
  `identity_saas`, `saas_productivity`, `saas_productivity_devops`,
  `saas_control_plane`, `saas_crm_platform`, `email_delivery`,
  `email_delivery_saas`, `observability_saas`, `synthetic_monitoring`,
  `local_control_plane`, `social_platform`, or another role accepted
  by config validation
- `source_type`: validated source-shape value such as
  `authoritative_provider_json`, `authoritative_provider_api`,
  `authoritative_plain_text`, `authoritative_service_tag_json`,
  `authoritative_static_docs`, `authoritative_root_hints`,
  `authoritative_rfc`, `authoritative_geofeed_csv`, `curated_static`,
  `secondary`, `generated_bgp`, `dns_derived`, or `analytical_only`
- `source_quality`: one of `A`, `B`, `C`, `D`
- `rationale`: non-empty public explanation of why the reference source is in
  the critical-infrastructure catalog

Merge expansion MUST copy the `critical:` block onto the expanded source entry
so downstream engine, API, and UI code see the same typed metadata whether a
reference feed is direct or merge-derived.

Critical-infrastructure provider names `providers` and `infrastructure` are
reserved. `providers` is the provider-list route segment, and `infrastructure`
would collide with the generated aggregate artifact suffix. Config validation
MUST reject a critical reference source or merge with either name.

Public feed names MUST NOT collide with generated critical-infrastructure
artifact names. If feed `foo` is a comparable public target, another public feed
named `foo_critical_infrastructure` or `foo_critical_{provider}` MUST fail
validation.

Critical-infrastructure service addresses and prefixes MUST be data, not code.
Small curated bodies use `static:` in source YAML; larger or externally
maintained bodies use `url:` or merge composition. Production code may implement
generic static-source plumbing, but MUST NOT contain hardcoded critical
infrastructure IP/CIDR lists. Static entries for critical-infrastructure
sources MUST parse as IPv4 addresses or IPv4 CIDRs at config-validation time.
Public critical-provider metadata and overlap artifacts do not imply raw-body
redistribution permission. The `critical_infrastructure` use role MUST NOT make
a source non-redistributable by itself. Critical-infrastructure reference feeds
MUST follow the same direct-upstream redistribution rule as other feeds, and
operator-maintained static reference data MUST use the operator's publication
policy for the raw body.

## Provider context

Provider-context sources use `use: [provider_context]` for broad cloud,
hosting, or provider address space that helps operators understand collateral
risk but is too tenant-mixed or broad to be critical-infrastructure warning
truth. They are ordinary public ipset/netset feeds. They MUST be excluded from
critical-overlap target generation so broad provider pages do not receive
misleading critical-warning artifacts against narrower service references.

Provider-context semantics MUST be carried in configuration fields/tags, not in
feed-name substrings. Current catalog entries may use attributes such as
`context_role`, `context_source_type`, `context_source_quality`, and
`context_rationale`. These attributes are accepted as freeform metadata but are
not enforced by config validation — they serve as operator documentation and
catalog tooling hints.

## Critical ASN context

`critical_asn_context` is a top-level list for the separate secondary ASN
signal. It MUST NOT resurrect the legacy `infrastructure_asns` warning model.
Each entry requires:

- `asn`
- `name`
- `tier`: `soft` or `contextual`; `hard` is not valid for ASN context
- `role`
- `source_quality`
- `rationale`

Validation MUST reject duplicates, empty rationale/name fields, invalid roles,
invalid qualities, and known broad hyperscaler/customer-hosting ASNs such as
AWS, broad Microsoft/Azure, broad Google/GCP, and Cloudflare customer-edge ASNs.
ASN context is coarse fallback evidence and MUST NOT replace exact
reference-feed overlap.

## URL and source reference contract

The configuration surface MUST accept only URL/reference forms that have clear
product meaning.

### Direct upstream URLs

The product MUST support:

- `http://...`
- `https://...`

### Local-file inputs

The product MUST support:

- `file:///absolute/path`

Rules:

- local file inputs MUST use absolute local paths
- host-qualified file URLs MUST NOT be treated as valid remote sources

### Artifact-backed child references

The product MUST support:

- `artifact://<artifact-name>?parts=<comma-separated-parts>`

Rules:

- the artifact name MUST reference a configured artifact parent
- `parts=` MUST support one or more named deliveries
- child feed configuration MUST NOT require operators to know the artifact's
  internal delivery directory

### Internal synthetic references

The product MAY use internal synthetic references for generated derivatives.

However:

- internal synthetic forms are an implementation mechanism
- the product contract is the semantic existence of derivatives and merges, not
  the exact internal reference string chosen by one implementation

## Frequency and cadence contract

Every autonomously scheduled downloader-stage item MUST have an explicit
cadence.

Rules:

- frequency MUST be non-negative
- frequency zero means the item is not autonomously scheduled by wall-clock
  cadence
- artifact-backed child feeds MUST NOT own an independent fetch cadence
- merges MAY own a downloader composition cadence without an upstream fetch
  cadence
- history derivatives MUST NOT own an independent wall-clock cadence
- history derivatives MUST follow parent-driven downloader behavior
- history derivatives MUST be declared only on parents that produce committed
  feed bodies; supporting provider databases are not valid
  history-derivative parents
- among synthetic public feed families, merges are the only family that
  progresses purely because time passed

## Runtime concurrency contract

Runtime configuration MUST separate at least five concurrency domains:

- downloader workers
- feed-processing workers
- heavy-phase workers
- engine-lane workers
- background workers

Downloader workers control upstream acquisition and local feed-body composition.

Feed-processing workers control the processing batch that turns staged feed
bodies into committed feed outputs and feed-local artifacts.

Heavy-phase workers control the global enrichment and comparison block,
including:

- pairwise metadata/comparison generation
- GeoIP fan-out
- ASN fan-out
- bogon fan-out

Engine-lane workers control top-level admission for broad engine-owned work that
must not run directly from HTTP handlers, watchdog paths, or unrelated
goroutines. This includes processing runs, integrity refresh/reprocess work,
entity artifact repair/rebuild/refresh work, and generated-artifact cleanup.

Background workers control bounded fan-out inside admitted background/entity
work, including:

- startup or reload entity-artifact integrity repair
- health-transition entity-artifact refreshes
- other future background maintenance tasks surfaced explicitly in the admin UI

The product MUST allow these concurrency domains to be tuned independently.

When heavy-phase concurrency is not explicitly configured, the product MAY use
an automatic default derived from machine capacity, but it MUST remain bounded
and operator-predictable.

Background-worker defaulting follows a stricter rule:

- if background-worker concurrency is not explicitly configured, the default
  MUST be `1`
- background work is intentionally low-priority and SHOULD prefer finishing
  later over competing aggressively for CPU or memory

## Output contract

The product MUST define the canonical output shape of each feed.

Current canonical output families are:

- host-oriented set output
- prefix-oriented set output

Canonical meanings:

- `ipset`
  - writes one normalized individual IP per line
  - CIDR inputs are expanded into their member IPs
- `netset`
  - writes one normalized CIDR per line
  - individual IPs render as single-address prefixes

Configuration MAY accept legacy aliases, but the canonical meaning presented by
the product MUST be stable and unambiguous.

## Retention window shorthand contract

Configuration MUST support a concise way to declare one-parent retention/history
window variants from a source or merge that produces committed feed bodies.

The product MAY expand that shorthand into multiple processable feed
definitions, but the semantic contract is:

- one parent feed, which MAY be a source or a downloader-composed merge
- one or more day windows
- each window becomes its own feed identity
- each window is additive: it contains the union of IPs observed in the parent
  during the last `X` days of downloader-owned retained history snapshots,
  anchored to the parent's current update timestamp
- the window is anchored to the parent's successful update times, not to an
  independent derivative schedule
- when declared on a merge, the base merge remains `secondary_merge`, and each
  suffixed window feed resolves to `secondary_retention` with the merge as its
  single parent

## Merge contract

Configuration MUST support defining named merges over one or more source feeds.

The semantic contract for a merge is:

- it is a first-class feed
- it has its own downloader cadence, configured with `frequency`
- it has one or more additive inputs declared with `sources`
- it MAY have subtractive inputs declared with `exclude`
- it MAY declare `history` windows, which produce retention derivatives of the
  merge output after the base merge is expanded
- it MAY declare ipset-compatible engine roles with `use`; database roles such
  as `asn` and `geoip` are invalid for merges because merges publish ipsets
- its set expression is `union(sources) - union(exclude)`; additions are
  evaluated first, then exclusions
- the same feed MUST NOT appear in both `sources` and `exclude`
- it is composed by the downloader from the latest durable local canonical feed
  bodies of the additive and subtractive inputs that are both:
  - currently enabled
  - not currently excluded from merge composition by health policy
    (`archived` and `unmaintained`)
- it does not independently fetch upstream content
- if any currently eligible input lacks a durable local canonical feed body, the
  merge composition attempt MUST fail
- if any configured subtractive input is disabled, archived, unmaintained, or
  missing while the merge has at least one eligible additive input, the merge
  composition attempt MUST fail rather than publish a broader-than-configured set
- if no additive inputs remain currently eligible for composition, the merge is
  operationally disabled for composition
- re-enabling an input does not force immediate recomposition; the merge waits
  for its cadence or an explicit operator action
- expanded runtime metadata MUST preserve both:
  - the full dependency list, so scheduler and dependency traversal see all
    additive and subtractive parents
  - the signed composition lists, so the engine and admin/public APIs can
    distinguish included, subtracted, and health-excluded inputs
  - the positive lineage list for comparison semantics is the additive side of
    signed merges; subtractive parents are dependencies but not positive
    ancestors
  - any supported `use` roles, so merge-derived feeds can participate in
    provider lists without hardcoded names
  - feed-facing metadata such as label, license, attribution, maintainer, and
    category, so public merge-derived feeds do not lose source obligations

Compatibility note:

- older imported catalogs that omit merge `frequency` MAY be assigned the
  runtime processing interval as a transitional default, but authored product
  configuration SHOULD declare merge cadence explicitly

## Artifact contract

Artifact definitions MUST support:

- stable artifact identity
- artifact family/type
- refresh cadence
- optional artifact-specific acquisition limits such as max download size,
  overriding the global runtime default only for that artifact family
- descriptive metadata for operators
- family-specific acquisition parameters

Artifacts are not public feeds.

Artifacts exist to produce and control child feed families.

For the current `dronebl_buildzone` artifact family:

- `rsync_url` identifies the authenticated rsync source
- the rsync password MUST come from environment, not YAML
- `DRONEBL_RSYNC_PASSWORD` is the artifact-specific preferred variable
- `RSYNC_PASSWORD` is accepted as a fallback
- real secret values MUST NOT be written into catalog YAML, docs, specs, SOWs,
  or skills

## Provenance contract

Configuration MUST support a stable public provenance classification for public
feeds.

Canonical values are:

- `primary`
  - a first-order public source feed
- `secondary_upstream`
  - a public feed that still behaves like a source feed, but is curator-labeled
    as an upstream-derived or mirrored source rather than a first-order source
- `secondary_merge`
  - a downloader-composed merge
- `secondary_retention`
  - a one-parent history derivative

Rules:

- plain or artifact-backed public feeds MAY be `primary` or
  `secondary_upstream`
- merges MUST resolve to `secondary_merge`
- history derivatives MUST resolve to `secondary_retention`
- public UIs MAY label the secondary values more simply as:
  - Upstream
  - Merge
  - Retention
- the canonical stored/product values remain the four values above

## Visibility and lifecycle flags

Configuration MUST be able to express at least:

- public versus hidden
- whether a feed is excluded from age-based unmaintained classification

These flags MUST affect product behavior consistently across:

- public browsing
- admin views
- integrity
- scheduling where relevant

Flag semantics:

- `hidden` affects public browsing visibility, not queue ownership
- hidden feeds remain visible in admin and remain processable unless separately
  disabled
- `exclude_from_unmaintained` suppresses only age-based health states
  (`delayed`, `risky`, `unmaintained`)
- `exclude_from_unmaintained` MUST NOT suppress `empty`, `unavailable`, or
  `archived`
- `use:` roles that describe reference/provider data rather than threat-feed
  freshness also suppress age-based health states: `critical_infrastructure`,
  `provider_context`, `asn`, and `geoip`
- role-based suppression MUST NOT be implemented by matching source names such
  as `critical_*`; use the configured `use:` tags

## Legal and redistribution policy

The configuration contract MUST include:

- license record
- attribution record
- redistribution policy

### Redistribution rule

Redistribution MUST default to allowed unless the source terms explicitly forbid
redistribution.

Exception: critical-infrastructure reference feeds default to raw
non-redistribution in the shipped catalog until a source-specific decision is
recorded. They may still be public metadata/reference providers and may still
produce overlap artifacts for other public feeds.

The following are not, by themselves, sufficient reason to mark a source as
non-redistributable:

- attribution requirements
- non-commercial restrictions
- warranty disclaimers
- "use at your own risk" style language
- unknown license with no explicit anti-redistribution language

### Non-redistributable rule

A source MUST be marked non-redistributable only when the terms explicitly
forbid copying, redistribution, or republication.

Merge-derived sources inherit non-redistributable status conservatively from
all transitive parents, including subtractive parents, because those parents
influence the derived artifact even when their ranges are removed from the
final set. Changing this legal model requires a separate explicit decision.

## Validation rules

The product MUST reject invalid configuration early.

Validation MUST cover at least:

- unsafe or colliding names; feed, merge, and artifact names MUST reject path
  separators, commas, control characters, and non-ASCII characters because names
  are filesystem components and comma is reserved by internal list encodings
- invalid feed family declarations
- invalid artifact references
- invalid artifact-specific acquisition limits
- invalid frequencies
- invalid output declarations
- invalid or unsupported URL forms
- cycles in derivative relationships
- invalid `use` roles
- merge `sources` and `exclude` references that are duplicated, overlap, or point
  at unknown configured/generated feed names
- merge declarations using database roles (`asn`, `geoip`)

## Technology-neutral promise

The configuration contract defines semantics, not parser internals.

Any implementation is acceptable if it preserves the same:

- accepted configuration language
- validation guarantees
- resulting feed/artifact semantics
