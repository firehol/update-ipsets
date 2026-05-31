# YAML Field Reference

You will learn the YAML fields source feeds and merge feeds can have, organized by group, with type, default, and example for each.

## Identity fields

| Field | Type | Default | Description | Example |
|-------|------|---------|-------------|---------|
| YAML key under `sources:` | string | — | Unique feed name. Used as filename, URL slug, reference key. No path separators, commas, control characters, or non-ASCII. | `dshield` |
| `label` | string | feed name | Human-readable name shown in the UI | `Team Cymru bogons (aggregated)` |
| `info` | string | — | Markdown description shown on the public feed-detail page | `[DShield.org](https://dshield.org/) top 20 attacking class C subnets` |
| `category` | string | — | Category key from `categories.yaml`. Required. | `intrusion` |
| `maintainer` | string | — | Feed maintainer name | `DShield.org` |
| `maintainer_url` | string (URL) | — | Link to maintainer website | `https://dshield.org/` |
| `homepage` | string (URL) | — | Not a direct config field. Use `info` with a markdown link to the upstream page instead. | — |
| `provenance` | string | `primary` | Public provenance classification: `primary`, `secondary_upstream`, `secondary_merge`, `secondary_retention` | `secondary_upstream` |

## Source fields

| Field | Type | Default | Description | Example |
|-------|------|---------|-------------|---------|
| `url` | string (URL) | — | Download URL. Supports `https://`, `http://`, `file:///`, `artifact://`, and `internal://`. | `https://feeds.dshield.org/block.txt` |
| `static` | list of strings | — | IP/CIDR list provided directly in YAML. Alternative to `url`. | `["1.1.1.1", "8.8.8.8"]` |
| `frequency` | integer | — | Minutes between automatic checks. `0` means not auto-scheduled. | `1440` |
| `ipv` | string | `ipv4` | IP version marker. Use `ipv4` for current feed processing and public lookup. `ipv6` is accepted by validation for ordinary set feeds, but the shipped catalog and public query/enrichment pipeline are IPv4-only in this release. Critical-infrastructure references reject `ipv6`. | `ipv4` |
| `downloader` | string | default HTTP/file downloader | Specialized downloader name for provider-database downloads. Normal feed downloads use `attributes.downloader`. | `copyfile` |
| `downloader_options` | string | — | Curl-like options for provider-database downloads. Normal feed downloads use `attributes.downloader_options`. | `--header 'Authorization: bearer ${API_TOKEN}'` |

## Processing fields

| Field | Type | Default | Description | Example |
|-------|------|---------|-------------|---------|
| `output` | string | — | Canonical output shape: `ipset` (one IP per line) or `netset` (one CIDR per line) | `netset` |
| `processor` | list of strings | — | Pipeline of transformations for the normalized output | `["remove_comments"]` |
| `processor_raw` | string | — | Pipeline for the raw download archive | `remove_comments` |
| `format` | string | — | Input format hint for specialized parsers | `maxmind_asn_mmdb_tar_gz` |

## Output and history fields

| Field | Type | Default | Description | Example |
|-------|------|---------|-------------|---------|
| `history` | list of integers | — | Minutes for history-derivative windows. Each creates a child feed. Valid on sources and merges that produce feed bodies. | `[1440, 10080, 43200]` |

## Legal fields

| Field | Type | Default | Description | Example |
|-------|------|---------|-------------|---------|
| `license` | string | — | SPDX identifier or free-text license | `CC0 1.0` |
| `attribution` | string | — | Required attribution text displayed on public pages | `This product includes GeoLite Data created by MaxMind` |
| `redistributable` | boolean | `true` | Whether raw feed body can be redistributed. Set `false` only when terms explicitly forbid redistribution. | `false` |

## Public enrichment metadata

Source and merge entries may include an `enrichment:` block. This is authored catalog metadata shown in public feed pages and API payloads; it is not runtime state.

| Field | Type | Default | Description | Example |
|-------|------|---------|-------------|---------|
| `enrichment.enrichment_schema_version` | integer | — | Public enrichment schema version. Current value is `2`. | `2` |
| `enrichment.run_at` | RFC3339 timestamp | — | When the enrichment was produced or last verified. | `2026-05-31T00:00:00Z` |
| `enrichment.official_name` | string | — | Upstream/public feed name. | `DShield block list` |
| `enrichment.official_url` | string (URL) | — | Upstream/public feed page. | `https://www.dshield.org/block.txt` |
| `enrichment.short_description` | string | — | Short public summary. | `Top attacking networks observed by DShield.` |
| `enrichment.long_description` | string | — | Longer public explanation. | `This feed tracks...` |
| `enrichment.roles` | list | — | Organizations or people involved, with role values such as `maintainer`, `publisher`, `aggregator`, `source_contributor`, `original_author`, or `successor`. | `[{role: maintainer, name: DShield.org}]` |
| `enrichment.derivation` | object | — | Whether the feed is original, derivative, aggregate, reformat, mirror, and what source feeds it derives from. | `{type: original, description: First-party feed}` |
| `enrichment.detection_classification` | object | — | Detection method and explanation. | `{primary_method: honeypot, description: ...}` |
| `enrichment.current_status` | object | — | Current upstream status, such as `active`, `discontinued`, `merged`, `forked`, `reformatted`, `altered_scope`, or `unknown`. | `{state: active, description: ...}` |
| `enrichment.sources_consulted` | list | — | Public URLs used to verify the enrichment. | `[{url: https://example.com/docs, validation_date: 2026-05-31}]` |

Other enrichment subfields cover listing policy, unlisting policy, scope and intent, redistribution details, update frequency, community context, and unlist-request instructions. Keep these fields public-safe; do not store private research notes, internal reasoning, raw evidence dumps, credentials, or personal data in `enrichment:`.

## Visibility and lifecycle fields

| Field | Type | Default | Description | Example |
|-------|------|---------|-------------|---------|
| `hidden` | boolean | `false` | Hide from public browsing. Feed remains active in admin and processing. | `true` |
| `exclude_from_unmaintained` | boolean | `false` | Suppress age-based health states (delayed, risky, unmaintained). | `true` |
| `enabled_by_all` | boolean | `false` | Whether `--enable-all` includes this feed | `true` |
| `accept_empty` | boolean | `false` | Do not flag empty downloads as errors | `true` |

## Use role fields

| Field | Type | Default | Description | Example |
|-------|------|---------|-------------|---------|
| `use` | list of strings | — | Engine role assignment. Valid values: `bogons`, `critical_infrastructure`, `provider_context`, `asn`, `geoip`. | `[bogons]` |

## Critical infrastructure metadata

Only allowed when `use: [critical_infrastructure]` is set.

| Field | Type | Default | Description | Example |
|-------|------|---------|-------------|---------|
| `critical.tier` | string | — | One of: `hard`, `soft`, `contextual` | `hard` |
| `critical.role` | string | — | Validated semantic role (e.g. `public_dns_core`, `cdn_edge`, `cloud_provider`) | `public_dns_core` |
| `critical.source_type` | string | — | Source shape (e.g. `authoritative_provider_json`, `curated_static`, `secondary`) | `curated_static` |
| `critical.source_quality` | string | — | One of: `A`, `B`, `C`, `D` | `C` |
| `critical.rationale` | string | — | Non-empty public explanation of why this reference is in the catalog | `Core public recursive DNS resolver addresses; blocking them breaks name resolution.` |

## Merge-specific fields

Used in `merges/` YAML files. Merge definitions use `sources` and optional `exclude` instead of `url`; they still have their own `frequency`.

| Field | Type | Default | Description | Example |
|-------|------|---------|-------------|---------|
| `sources` | list of strings | — | Additive input feed names | `["dshield", "feodo"]` |
| `exclude` | list of strings | — | Subtractive input feed names | `["bogons"]` |
| `history` | list of integers | — | Optional history windows generated from the merge output | `[1440, 10080, 43200]` |

## Artifact parent fields

Used in `artifacts/` YAML files.

| Field | Type | Default | Description | Example |
|-------|------|---------|-------------|---------|
| `type` | string | — | Artifact family/type controlling parse behavior | `dronebl_buildzone` |
| `frequency` | integer | — | Minutes between automatic downloads | `60` |
| `max_download_size` | integer | runtime default | Per-artifact download size limit in bytes. `0` uses the downloader default; `-1` disables the cap. | `268435456` |
| `info` | string | — | Admin-facing artifact description. | `DroneBL shared buildzone download` |
| `maintainer` | string | — | Artifact source attribution. | `DroneBL.org` |
| `maintainer_url` | string (URL) | — | Artifact source website. | `https://dronebl.org` |
| `rsync_url` | string (URL) | — | Artifact-specific rsync source URL, used by supported artifact types. | `rsync://example.com/path/` |

## Miscellaneous fields

| Field | Type | Default | Description | Example |
|-------|------|---------|-------------|---------|
| `attributes.public_url` | string (URL) | `url` | Public-safe URL shown in metadata when the real `url` contains credentials or tokens. | `https://example.com/feed.txt?token=TOKEN` |
| `attributes.downloader` | string | default HTTP/file downloader | Specialized downloader name for normal source-feed downloads. | `copyfile` |
| `attributes.downloader_options` | string | — | Curl-like options for normal source-feed downloads: `--data`, `--request`, `--referer`, `--user`, and `--header` are supported. | `--data 'export_type=text'` |
| `attributes.no_if_modified_since` | string | unset | Set a non-empty value to suppress `If-Modified-Since` on HTTP downloads for sources that reject conditional requests. | `true` |
| `attributes.context_role` | string | — | Provider-context role, used with `use: [provider_context]`. | `cloud_customer_hosting` |
| `attributes.context_source_type` | string | — | Provider-context source shape. | `authoritative_provider_json` |
| `attributes.context_source_quality` | string | — | Provider-context quality grade. | `A` |
| `attributes.context_rationale` | string | — | Operator-facing reason this provider-context feed exists. | `Overlap is policy-dependent.` |
