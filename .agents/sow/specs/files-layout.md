# Files Layout Contract

## Status

This document is normative.

The words **MUST**, **MUST NOT**, **SHOULD**, and **MAY** describe the required
filesystem contract of the product.

## Purpose

This document defines the stable on-disk layout the product maintains.

It answers these questions:

- which directories and files exist under an installation
- which subsystem owns each file family
- which files are durable committed truth, staged replayable state, or
  disposable scratch state
- which names remain compatible with the historical bash implementation
- which migration/import surfaces exist for moving bash-era state into the Go
  product

The product behavior around those files is owned by other specs:

- downloader behavior: [downloader.md](downloader.md)
- processing behavior: [processing-engine.md](processing-engine.md)
- queue choreography and restart rules: [pipeline.md](pipeline.md)
- compatibility guarantees: [compatibility.md](compatibility.md)

This document owns the concrete filesystem layout itself.

## Scope

This document covers files and directories maintained by the installed product.

It does not attempt to define the repository source tree except where source
files are themselves operator-facing tools, such as migration helpers.

## Ownership model

The product maintains five broad filesystem classes:

1. installation/runtime support
2. downloader-owned raw source and provider input state
3. canonical public-feed state
4. published website and mirror artifacts
5. migration/import workspaces

Every durable file family MUST have one owning subsystem.

Every durable file family that participates in integrity MUST also have an
mtime owner. The owner is the subsystem that writes the file and deliberately
sets its mtime to the timestamp required by [integrity.md](integrity.md).
Writers MUST NOT leave committed feed-publication files with accidental local
write mtimes.

## Installation root

An installation root typically contains at least these top-level directories:

- `bin/`
  - installed binaries
- `etc/`
  - operator-managed configuration files
- `data/`
  - source enable markers, raw source debug files, and canonical public-feed
    files
- `cache/`
  - runtime caches and scheduler state that are not feed bodies
- `lib/`
  - per-feed durable engine state, provider archives, and artifact-local state
- `web/`
  - published machine-readable website artifacts
- `run/`
  - lock files and runtime coordination files
- `tmp/`
  - scratch space for incomplete active writes
- `import-bash-version/`
  - migration workspace created by the bash-import helper

The exact absolute path of the installation root is deployment-specific.

### Installed ownership contract

For managed systemd installs:

- the installation root, `bin/`, and `etc/` MUST be owned by `root:iplists`,
  readable/searchable by the `iplists` group, and not world-readable
- the daemon binary under `bin/` MUST be executable by the service user but not
  writable by it
- the active catalog under `etc/config/` and installed templates under
  `etc/config/templates/` MUST be readable by the service user but not writable
  by it
- mutable runtime directories `data/`, `cache/`, `lib/`, `web/`, `run/`, and
  `tmp/` MUST be owned and writable by the service user and SHOULD NOT be
  world-readable
- daemon-created mutable runtime and publication directories MUST be
  owner-readable/searchable/writable only; managed installs use `0700`
- daemon-created non-executable runtime and publication files MUST be
  owner-readable/writable only; managed installs use `0600`
- managed systemd installs MUST set a compatible process umask, currently
  `UMask=0077`, so direct file and directory creation preserves the generated
  artifact permission contract
- install and packaging flows MUST repair existing mutable runtime directories
  and files to the same directory/file modes during reinstall
- public HTTP availability is provided by the daemon or configured serving
  process, not by making generated runtime/publication files world-readable
- systemd write access SHOULD be scoped to mutable runtime directories instead
  of the full installation root

Installer or packaging flows MUST NOT recursively make the whole installation
root service-owned as a shortcut for runtime write access.

### Non-root default runtime layout

The shipped catalog may express path defaults with shell-style templates such
as `${HOME}/ipsets`, but the effective runtime layout has a separate non-root
fallback rule. When the daemon is not running as root and the path settings are
unset or still equal to the built-in defaults, the runtime resolver MUST use
user-owned defaults for the main mutable state:

- `base_dir`: `$HOME/.update-ipsets/ipsets`
- `run_parent_dir`: `$HOME/.update-ipsets/run`
- `cache_dir`: `$HOME/.cache/update-ipsets`
- `lib_dir`: `$HOME/.local/share/update-ipsets`

Explicit runtime YAML values or environment-variable overrides MUST take
priority over this non-root fallback rule.

## Stable top-level files

### `etc/config/`

- canonical runtime catalog/configuration directory
- deployment-managed active config directory
- install flow MAY replace it from the deployed repo catalog directory when
  content changed; identical reinstalls SHOULD avoid rewriting it so freshness
  checks do not treat a no-op install as a catalog change
- if local divergence matters, it SHOULD be preserved via backup/overlay
  mechanisms rather than by silently leaving the active directory stale
- not generated by downloader or processing
- feed and merge fragments MAY contain authored `enrichment:` metadata blocks;
  these blocks are catalog source metadata and MUST be preserved by install and
  catalog round-trip tooling even when a runtime component does not yet consume
  them

### `run/update-ipsets.lock`

- runtime lock/co-ordination file
- owned by process lifecycle, not by downloader or engine logic

### `data/.cache.json`

- canonical durable JSON cache of feed state
- owned by the engine/runtime state layer
- this remains under `data/` for bash-migration continuity

### `data/.cache`

- legacy bash cache file
- read/import compatibility input only
- MUST NOT be treated as the canonical writable cache by the Go product

### `cache/scheduler-state.json`

- scheduler/runtime ledger
- owned by the scheduler, not by the feed-state cache

### `cache/comparison-pairs-v2.bin`

- internal comparison-pair optimization ledger
- owned by the engine heavy-phase comparison writer
- records feed-pair names, normalized content hashes, comparison algorithm
  version, and exact common-IP count
- compact binary cache; it is not a public, portable, or operator-authored file
- not a public artifact and not an integrity source of truth
- missing, malformed, oversized, incompatible, or unwritable state MUST be
  ignored or replaced without blocking public artifact publication
- missing, malformed, oversized, or incompatible readable state MUST force a
  full comparison-ledger rebuild so incremental runs cannot publish or persist a
  partial replacement ledger
- regenerated atomically from retained current-key hits and fresh current-run
  comparison results when a valid ledger exists, or from a full current pair set
  when the ledger is absent or untrusted
- if `comparison-pairs-v2.bin` is absent, legacy
  `cache/comparison-pairs-v1.json` MAY be read once as an upgrade input; v1 JSON
  is not canonical, MUST NOT be written by current code, and MUST be removed
  after a successful v2 write so stale JSON cannot be reused on later runs

## Source enable markers

Normal source feeds have a dedicated durable enable marker:

- `data/{feed}.enabled`

Meaning:

- operator-controlled source enablement state
- separate from both raw `.source` retention and canonical feed-body files
- applicable to plain feeds, merges, history derivatives, artifact-backed child
  feeds, and provider datasets

Rules:

- `data/{feed}.source` MUST NOT be used as an enable marker
- missing `data/{feed}.enabled` means the source is explicitly disabled unless
  global enable-all mode overrides it
- source enablement state is a control-plane contract, not downloader content

## Downloader-owned raw source files

The downloader owns retained raw-source files for feed families that begin from
raw upstream bytes.

### Committed raw source

For direct downloadable public feeds, the retained raw source is:

- `data/{feed}.source`

Meaning:

- latest raw upstream body retained for debugging
- not part of the canonical feed-body contract
- not consumed by the processing engine
- not an enable/disable marker

Synthetic public feeds such as history derivatives and merges MUST NOT require
their own `data/{feed}.source` file, because their canonical feed bodies are
composed locally.

### Canonical staged feed body

For ordinary public feeds, the downloader stages the canonical feed body as
exactly one of:

- `data/{feed}.ipset.new`
- `data/{feed}.netset.new`

Meaning:

- complete durable canonical downloader result
- admitted or about to be admitted to processing
- MUST survive restart recovery until the engine claims it

### Canonical processing feed body

When the engine starts processing staged canonical work, it claims that exact
file by renaming it to:

- `data/{feed}.ipset.processing`
- `data/{feed}.netset.processing`

Meaning:

- engine-owned in-flight canonical feed body
- still restart-recoverable
- content-identical to the downloader-produced staged body

### Committed canonical feed body

Exactly one of the following MUST exist for a successfully processed public
feed:

- `data/{feed}.ipset`
- `data/{feed}.netset`

These are the authoritative committed canonical feed bodies for the entire
product.

Rules:

- they are plain text, not binary
- they contain only canonical feed entries in the configured output family
- they MUST NOT require special header parsing
- if they include comment headers, those headers are engine-generated
  publication context only; they MUST identify `FireHOL's update-ipsets`
  and MUST NOT mention retired wrapper names such as `update-ipsets.sh`
  or obsolete external binaries such as `FireHOL's iprange`
- the downloader defines their canonical body content before processing
- the engine writes successful normal feed bodies into these files during
  feed-local finalization, before publishing the corresponding staged public
  artifacts

### Temporary downloader scratch state

The downloader MAY use:

- per-file temporary siblings during atomic replacement
- files under `tmp/` for download spill and intermediate extraction

Rules:

- incomplete scratch names are not part of the stable operator contract
- they MAY use random suffixes
- they MUST be safe to discard after crash/restart
- they MUST NOT be interpreted as committed truth

## Downloader-owned history snapshots

History-derivative support uses downloader-owned retained snapshots under:

- `data/history/{parent}/{unix_timestamp}.set`

Rules:

- one file per successful parent update timestamp
- sparse by observed successful parent updates, not dense by calendar day
- owned by the downloader, not by the engine retention subsystem
- used only for history-derivative composition and repair
- MUST NOT be treated as the ordinary source of current per-IP listing age for
  search or UI timing

## Artifact-parent local storage

Artifact parents have a dedicated private delivery area under:

- `lib/artifacts/{artifact}/`

Stable children of that directory are:

- `enabled`
  - artifact enable marker
- `fetch/`
  - private acquisition workspace for custom artifact transports
  - may contain the current transport-local parent input when an artifact family
    needs one before generic source staging
  - MUST NOT retain unconsumed upstream sibling files, persistent partial files,
    or stale per-run fetch directories
- `source`
  - committed parent artifact input
- `source.new`
  - staged parent artifact input
- `extract/`
  - child materialization workspace and outputs owned by that artifact family
  - per-run materialization directories are private scratch and MUST be safe to
    remove before a new materialization attempt

Artifact-child public feeds then use materialized local input from this private
area, but the child configuration MUST NOT require the operator to know these
paths.

## Provider dataset storage

Supporting provider datasets are downloader-owned local inputs, but they are
not normal public feeds.

### Geolocation providers

Committed provider archive/body:

- `lib/geolocation/{provider}.source`

Staged provider archive/body:

- `lib/geolocation/{provider}.source.new`

### ASN providers

Committed provider archive/body:

- `lib/asn/{provider}/source`

Staged provider archive/body:

- `lib/asn/{provider}/source.new`

### Provider enable semantics

Provider datasets follow normal source enable/disable rules, but their files
remain provider-local inputs rather than public feed bodies.

## Processing-engine-owned durable feed state

The processing engine consumes downloader-produced canonical feed bodies and
maintains durable per-feed state under `data/` and `lib/`.

### Set-info sidecar

Per-feed human-readable summary:

- `data/{feed}.setinfo`

### Binary latest snapshot

Canonical binary latest snapshot:

- `lib/{feed}/latest`

Rules:

- this is the canonical current binary snapshot name
- `lib/{feed}/latest.set` is legacy read compatibility only
- new writes MUST target `latest`, not `latest.set`

### Feed history ledger

Append-only internal history ledger:

- `lib/{feed}/history.csv`

Rules:

- the ledger records feed-level observations in timestamp order
- if the same timestamp appears more than once because a local repair or import
  corrected the last observation, runtime/state rebuilds and public readers
  MUST treat the last row for that timestamp as authoritative rather than
  counting it as an additional observation

### Feed changeset ledger

Append-only internal added/removed ledger:

- `lib/{feed}/changesets.csv`

Rules:

- bounded public/API/chart consumers that need only the recent change window
  MUST use runtime cache state, bounded published artifacts, or bounded tail
  reads; full-ledger scans are reserved for explicit full-series or repair
  work

### Retention cohorts

Per-update cohort snapshots live under:

- `lib/{feed}/new/{unix_timestamp}`

Rules:

- this is the canonical modern name
- legacy bash-compatible `.set` suffix variants MAY be accepted on read
- new writes MUST target the suffix-less canonical name
- each cohort file contains the subset of IPs that were added at that timestamp
  and are still currently listed in the feed
- together, the cohort files partition the current feed membership by the start
  of the current contiguous listing interval
- these cohort files are the authoritative engine-owned source for current
  per-IP listing age and search `first_seen`

### Retention ledgers and summaries

The engine maintains:

- `lib/{feed}/retention.csv`
- `lib/{feed}/retention.json`
- `lib/{feed}/retention_cohorts.csv`
- `lib/{feed}/histogram`

Meaning:

- `retention.csv`
  - detailed removal-life ledger
  - authoritative engine-owned source for the age of removed IPs at the time
    they were removed
- `retention.json`
  - durable structured retention summary used by publication
- `retention_cohorts.csv`
  - compact cohort index used by runtime/state rebuilding
  - index over the current cohort files in `lib/{feed}/new/`
- `histogram`
  - bash-compatible shell-format histogram cache

## Published website artifacts

The published machine-readable website output lives under `web/`.

Published website artifacts are committed publication data. Their filenames are
addresses, not semantic classifiers. The owning writer or staged-publish path
MUST preserve the logical mtime assigned by the producer when files move from a
staging directory into `web/`.

The `web/` tree and the optional raw-download mirror are publication-owned
served roots. Operators may replace the roots with configured directories, but
public request handlers MUST treat entries inside those roots as untrusted path
components at serve time: `..` traversal, absolute paths, hidden implementation
paths, and symlinks that escape the served root MUST NOT be served. Public
serving may follow symlinks only when the resolved target remains inside the
same served root.

### Global published files

- `web/index.json`
- `web/all-ipsets.json`
- `web/home/aggregates.json`
- `web/sitemap.xml`
- `web/sitemap-pages.xml`
- `web/sitemap-feeds.xml`
- `web/sitemap-countries.xml`
- `web/sitemap-maintainers.xml`
- `web/sitemap-asns-*.xml`
- `web/robots.txt`
- `web/llms.txt`

`web/index.json` and `web/all-ipsets.json` are public catalog/index artifacts.
They MUST contain only public feed entries. Hidden feeds and supporting
provider datasets are not part of these indexes.

`web/home/aggregates.json` is the precomputed homepage rollup artifact. It
stores the base provider metadata and per-category aggregate slices used by
`/api/v1/home/summary` and `/api/v1/home/globe`. Public request handlers MUST
read this artifact instead of scanning feed-level GeoIP/ASN artifacts.
The artifact participates in entity-integrity validation. Missing, malformed,
or stale homepage aggregates MUST be reported with a repair action that
republishes the aggregate from current cache/entity inputs. Its published mtime
MUST be at least as recent as the feed state and health-transition timestamps it
summarizes.

`web/sitemap.xml`, `web/sitemap-*.xml`, `web/robots.txt`, and `web/llms.txt`
are public metadata artifacts generated from the public route/feed/entity
inventory. They MUST NOT include authenticated admin routes, local filesystem
paths, or private runtime details. The metadata generator owns root-level
`web/sitemap-*.xml` shard files and removes stale shards after a successful
sitemap write.

### Per-feed published files

For each public feed the product MUST be able to publish:

- `web/{feed}.json`
- `web/{feed}_history.csv`
- `web/{feed}_changesets.csv`
- `web/{feed}_retention.json`
- `web/{feed}_comparison.json`
- `web/{feed}_insights.json`

Hidden feeds and supporting provider datasets MUST NOT be published as ordinary
feed-scoped `web/{feed}*` public artifacts.

### Per-provider enrichment files

When relevant providers are configured, the product also publishes:

- `web/{feed}_{geo_provider}.json`
- `web/{feed}_asn_{asn_provider}.json`
- `web/{feed}_bogons_{bogon_provider}.json`
- `web/{feed}_critical_infrastructure.json`
- `web/{feed}_critical_{critical_provider}.json`

These are feed-facing published artifacts, but their content depends on the
current committed provider datasets. Critical-infrastructure aggregate files
and per-provider files MUST include the provider-set identity used to build
them so admin integrity and pipeline repair can detect drift after operator
config changes. Public routes MUST NOT reject a structurally valid published
critical-overlap artifact solely because its `provider_set_id` differs from the
engine's current provider-set identity.
When critical ASN context is present, it is embedded in
`web/{feed}_critical_infrastructure.json`; it does not create a separate
sidecar file and does not change `critical_ips`.
The current critical provider-set identity is persisted at
`lib/critical_infrastructure/provider_set_id` after a successful publication.
The current default-provider provider-set identity is persisted at
`lib/provider_defaults/provider_set_id`. Both are runtime state, not source
configuration.
The critical provider-set marker records stable provider metadata, provider
acquisition/processing shape, configured critical metadata, configured
`critical_asn_context`, and configured ASN-provider source shape when critical
ASN context is present. It MUST NOT encode materialized provider content,
cardinality, content hashes, volatile local timestamps, or version counters.

### Entity reference artifacts

Country and ASN public reference surfaces are published as nested website
artifacts.

The public machine-readable files are:

- `web/countries/index.json`
- `web/countries/{CODE}.json`
- `web/asns/index.json`
- `web/asns/{ASN}.json`

Rules:

- these files are public website artifacts and MUST be served from `web/`
- the API routes `/api/v1/countries*` and `/api/v1/asns*` are thin readers
  over these published files
- these files are final public payloads, not reusable internal fragments

The engine also maintains a private entity sidecar tree under `lib/`:

- `lib/entities/version`
- `lib/entities/feed-presence-v1.bin`
- `lib/entities/feeds/{feed}.json`
- `lib/entities/feeds-pending/{feed}.json`
- `lib/entities/countries/{CODE}.json`
- `lib/entities/asns/{ASN}.json`

Meaning:

- `lib/entities/version`
  - entity-artifact schema/version marker for bootstrap decisions
- `lib/entities/feed-presence-v1.bin`
  - private binary index listing feed names referenced by the current committed
    entity sidecar set
  - generated with full entity rebuilds and surgical entity refresh publish
    batches
  - used as the first proof source when a committed per-feed sidecar is missing
    and the engine must decide whether surgical refresh can continue or must
    fall back to full rebuild
  - internal, reproducible, and not a public or operator-authored file
  - missing, malformed, oversized, or incompatible state MUST be ignored in
    favor of the older bounded actor-sidecar scan fallback
- `lib/entities/feeds/{feed}.json`
  - private committed per-feed entity sidecar
  - contains the feed metadata needed by country/ASN actor rows plus the feed's
    current country and ASN contributions
  - built once per affected feed while provider data is already open during the
    processing run
  - used to target incremental entity refreshes and to build precomputed
    country-detail and ASN-detail artifacts without recomputing broad
    intersections once per entity page
  - older membership-only sidecars whose `countries` are string arrays and
    `asns` are number arrays MAY be read as migration inputs, but MUST NOT be
    used as old-side surgical delta inputs because they do not contain
    contribution counts
- `lib/entities/feeds-pending/{feed}.json`
  - private pending replacement for `lib/entities/feeds/{feed}.json`
  - written by the normal processing run for a changed feed after the new
    per-feed entity contribution sidecar has been computed
  - consumed by the background entity refresh worker as the new side of the
    per-feed delta; the committed `feeds/{feed}.json` remains available as the
    old side until the refresh succeeds
  - MUST be deleted after the background patch promotes it to
    `feeds/{feed}.json`
- `lib/entities/countries/{CODE}.json`
  - private country-detail sidecar containing the stable composition facts for
    that country
- `lib/entities/asns/{ASN}.json`
  - private ASN-detail sidecar containing the stable composition facts for
    that ASN

The private entity sidecars:

- MUST NOT be exposed directly as public web files
- exist so pure health transitions can rewrite only the affected final public
  detail payloads without recomputing the heavier composition layers
- exist so feed/provider updates can compute the expensive per-feed
  country/ASN contribution facts once per affected feed during processing and
  then rebuild only the affected public entity-detail pages
- ordinary feed-update background work MUST support per-feed delta targeting
  where changed feed contributions select affected country/ASN actors without
  scanning unrelated feeds
- ordinary feed-update background work MUST treat `feeds/{feed}.json` and
  `feeds-pending/{feed}.json` as the canonical private contribution state for
  selected actor rebuilds. `countries/{CODE}.json` and `asns/{ASN}.json` are
  derived private outputs for serving/repair, not canonical patch-state inputs
  for ordinary changed-feed refreshes.
- per-feed delta updates MUST keep every derived country/ASN aggregate
  equivalent to a clean rebuild of the same country/ASN actor
- ordinary feed-update background work MUST skip actor rewrites when the
  per-feed contribution for that actor is unchanged
- MAY be rebuilt wholesale during explicit maintenance when that is simpler or
  safer than incremental repair
- SHOULD be checked and repaired on startup/reload through entity-integrity
  validation rather than unconditional full rebuilds

### Markdown page artifacts

Markdown page artifacts are pre-generated `.md` files published alongside the
corresponding JSON artifacts for each entity. They provide LLM-friendly
structured summaries for AI agent consumption (consumed by the MCP
`fetch_analysis` tool).

The published files are entity-local, sitting next to each entity's other
artifacts rather than in a separate subtree:

- `web/{feed}.md` (sibling of `web/{feed}.json` and other per-feed files)
- `web/countries/{CODE}.md` (sibling of `web/countries/{CODE}.json`)
- `web/asns/{ASN}.md` (sibling of `web/asns/{ASN}.json`)
- `web/maintainers/{slug}.md` (no JSON sibling today; the API
  `/api/v1/maintainers/{slug}` is computed at request time)

Rules:

- markdown artifacts MUST be generated during the same staging pass that writes
  the corresponding JSON artifacts
- feed markdown is generated after the metadata/insights phase writes feed JSON
- country and ASN markdown is generated alongside each entity detail JSON write
  across all four entity write paths (health transition, full rebuild, surgical
  feed update, selected repair)
- maintainer markdown is generated during full entity rebuilds only
- markdown artifacts are published atomically with other staged artifacts
- markdown generation MUST NOT trigger upstream downloads or broad recomputation
- the markdown template directory is `{config_path}/templates/markdown/`; if the
  directory does not exist, markdown generation is silently skipped
- generated markdown files are registered as `GeneratedFile` entries with
  `Redistributable: true`
- feed markdown MUST render only the configured default ASN and GeoIP providers;
  full provider fan-out remains available through JSON/API artifacts
- feed markdown critical-infrastructure provider rows MUST render a readable
  provider label or name, never a raw provider object
- feed markdown retention tables MUST roll hourly retention buckets into days 1
  through 365, omit zero-count days with a note that missing days are zero, and
  append a `>365 days` row for all later buckets
- feed markdown overlap tables MUST include both overlap percentages:
  `This %` is common IPs divided by current-feed IPs, and `Their %` is common
  IPs divided by the row feed's IPs
- the old `web/markdown/{entity_type}/{id}.md` layout has been removed; nothing
  in the product writes to or reads from that subtree

### Mirror/download files

If a downloadable mirror directory is configured with
`runtime.web_dir_for_ipsets` or the daemon `--web-files-dir` override, the engine
MUST create that directory during runtime directory setup. Redistributable set
files are copied to:

- `web/files/{feed}.ipset`
- `web/files/{feed}.netset`

Exactly one of the two output forms applies per feed, matching the committed
canonical feed body under `data/`.

The mirror is publication-owned and MUST be updated atomically from committed
canonical feed files rather than by recomputing the feed.

When the committed canonical feed file and existing mirror file are
byte-identical, mirror publication MAY keep the mirror file in place and update
its mode, owner, and mtime to match the committed feed file instead of copying
through a new temporary file. If comparison cannot prove identity, publication
MUST fall back to the normal temporary-copy-and-rename path.

Mirror/download publication follows the same public-feed boundary as the rest
of the public website artifacts. Hidden feeds and supporting provider datasets
MUST NOT be exposed as raw mirror downloads.

Raw mirror serving follows the same served-root safety model as `web/`: direct
download routes MUST open `<feed>.ipset` and `<feed>.netset` relative to the
configured mirror root or base data root and reject symlinks that escape those
roots.

## Git-facing convenience files

When the base data directory is itself a git worktree, the engine MAY also
maintain:

- `data/README.md`
- `data/.gitignore`
- `data/set_file_timestamps.sh`

These files are compatibility/convenience outputs for repository publication
and are not required in non-git deployments.

The `.git/` directories under generated publication trees are private Git
object stores, not product data. When generated Git publication is enabled, the
runtime MAY run Git auto-maintenance after sync attempts. During managed
installs where mutable runtime repair is allowed and the service is stopped, the
installer MAY compact/prune these generated Git object stores without changing
working-tree content.

## Staging and temporary naming rules

Stable rules:

- staged durable replacements use the `.new` sibling naming convention
- incomplete scratch writes use temporary names that are not authoritative
- public and entity artifact publish may touch an existing live artifact in
  place instead of replacing it when the staged artifact has identical bytes;
  the live artifact still inherits the staged artifact's logical mtime and
  committed permission contract
- public and entity artifact publish batches may carry metadata-only touch
  intents for proven-current live artifacts; those intents are applied only by
  the publish step, never directly by the staging producer
- raw mirror/download publication may touch an existing live mirror file in
  place instead of replacing it when the committed canonical feed file has
  identical bytes; the live mirror file still inherits the canonical feed file's
  mtime and committed permission contract
- public and entity artifact publish stages use hidden directories under their
  owning publication roots, such as `.update-ipsets-web-*` and
  `.update-ipsets-entities-*`; these directories are scratch state and MUST NOT
  be served as public content
- daemon startup MUST clean old publish-stage leftovers; because process death
  can leave pre-start stage directories that are still inside the immediate
  cleanup grace period, the daemon SHOULD run a delayed cleanup that removes
  only publish-stage directories whose mtimes predate the current process start
  time

Examples:

- `data/{feed}.ipset.new`
- `data/{feed}.netset.new`
- `data/{feed}.ipset.processing`
- `data/{feed}.netset.processing`
- `lib/geolocation/{provider}.source.new`
- `lib/asn/{provider}/source.new`

Non-goals:

- the exact temporary scratch filename used during atomic replacement is not a
  stable public contract
- operators SHOULD reason in terms of committed vs staged vs temporary state,
  not in terms of one specific temporary filename pattern

## Migration/import workspace

The canonical bash-import workspace is:

- `import-bash-version/`

It is owned by the migration helper, not by ordinary steady-state operation.

Stable contents may include:

- imported legacy/public trees staged for promotion
- manifests of source and local-only feeds
- imported legacy config snapshot
- merged cache JSON used to bootstrap continuity

The product's legacy failure bootstrap MAY read:

- `import-bash-version/merged-cache.json`

For transition compatibility, implementations MAY also accept the older
`import-d1/merged-cache.json` location while that older helper name still
exists.

## Historical bash compatibility surface

The following legacy names remain important compatibility inputs or outputs:

- `data/.cache`
- `/etc/firehol/ipsets/history/{feed}/{unix_timestamp}.set`
- `lib/{feed}/latest`
- `lib/{feed}/latest.set` (read compatibility only)
- `lib/{feed}/new/{unix_timestamp}`
- `lib/{feed}/new/{unix_timestamp}.set` (read compatibility only)
- `lib/{feed}/history.csv`
- `lib/{feed}/changesets.csv`
- `lib/{feed}/retention.csv`
- `lib/{feed}/histogram`
- `data/{feed}.setinfo`
- `web/all-ipsets.json`
- `web/{feed}_history.csv`
- `web/{feed}_changesets.csv`
- `web/{feed}_comparison.json`

Important distinction:

- the canonical retained history layout is the bash-compatible timestamped form
  `data/history/{parent}/{unix_timestamp}.set`
- older Go installs MAY still contain transitional day-bucket files named
  `data/history/{parent}/{YYYY-MM-DD}.set`
- migration/import tooling SHOULD preserve bash-era timestamped snapshots
  directly, not translate them into a different native layout

Compatibility behavior is owned by [compatibility.md](compatibility.md).
This document defines where those files live and what they mean.

## Review checklist

A reviewer checking filesystem-layout compliance SHOULD be able to answer:

- does every successfully processed public feed have exactly one committed
  canonical feed body?
- are staged replayable inputs clearly separated from committed truth?
- are downloader-owned history snapshots separate from retention ledgers?
- are provider datasets stored separately from normal public feed bodies?
- do published website artifacts live under `web/` rather than being computed
  on request?
- do bash-era compatibility files map to explicit modern locations and rules?
