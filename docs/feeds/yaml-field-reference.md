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
| `frequency` | integer | — | Seconds between automatic checks. `0` means not auto-scheduled. | `1440` |
| `ipv` | string | `ipv4` | IP version: `ipv4` or `ipv6` | `ipv4` |

## Processing fields

| Field | Type | Default | Description | Example |
|-------|------|---------|-------------|---------|
| `output` | string | — | Canonical output shape: `ipset` (one IP per line) or `netset` (one CIDR per line) | `netset` |
| `processor` | list of strings | — | Pipeline of transformations for the normalized output | `["remove_comments"]` |
| `processor_raw` | string or list | — | Pipeline for the raw download archive | `remove_comments` |
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

Used in `merges/` YAML files instead of `url` and `frequency` on the merge definition.

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
| `frequency` | integer | — | Seconds between automatic downloads | `60` |
| `max_download_size` | integer | runtime default | Per-artifact download size limit | `268435456` |

## Miscellaneous fields

| Field | Type | Default | Description | Example |
|-------|------|---------|-------------|---------|
| `attributes` | map | — | Freeform metadata for operator documentation | `context_role: cloud_customer_hosting` |
