# Feed Endpoints

You will learn the feed-related API endpoints, what each returns, and how to use them.

## Feed catalog

```
GET /api/v1/sets
```

Lists all public feeds with summary metadata. Each entry includes feed name,
category, IP family, current size, health state, maintainer, redistribution
state, and critical-infrastructure overlap tiers where applicable.

Example:

```
GET /api/v1/sets
```

Key response fields per feed:

- Identity and provenance: `name`, `official_name`, `short_description`,
  `category`, `provenance`, `maintainer`, `maintainer_url`, `license`,
  `redistributable`, and `info`.
- Source and output: `ipv`, `url`, `public_url`, `file`, `source`, and `hash`.
- Timestamps: `started_date`, `source_date`, `processed_date`, and
  `checked_date`.
- Size and cadence: `entries`, `unique_ips`, `entries_min`, `entries_max`,
  `ips_min`, `ips_max`, `frequency_minutes`, `average_update_mins`,
  `min_update_mins`, and `max_update_mins`.
- Change and uniqueness metrics: `rotation_median_pct`, `rotation_p75_pct`,
  `rotation_samples`, `change_ratio_median_pct`, `change_ratio_p75_pct`,
  `change_ratio_samples`, `unique_share_pct`, and `unique_share_samples`.
- Status and risk signals: `health`, `current_status_state`, `last_status`,
  `last_error`, `download_failures`, `version`, `critical`, and
  `critical_overlap_tiers`.

## Feed detail

```
GET /api/v1/sets/{name}
```

Returns full metadata for a single feed. Includes identification, data characteristics, update statistics, health class, merge composition (for merge feeds), and provider enrichment summaries.

Example:

```
GET /api/v1/sets/firehol_level1
```

Key response fields:

- Identity and ownership: `name`, `category`, `provenance`, `maintainer`,
  `maintainer_url`, `license`, `attribution`, `official_name`,
  `short_description`, `current_status`, `enrichment`, and `info`.
- Output metadata: `entries`, `entries_min`, `entries_max`, `ips`, `ips_min`,
  `ips_max`, `ipv`, `hash`, `frequency`, `aggregation`, `source`, `file`,
  `file_local`, `output`, `format`, and `version`.
- Timestamps and cadence: `started`, `updated`, `processed`, `checked`,
  `clock_skew`, `average_update`, `min_update`, `max_update`,
  `rotation_median_pct`, `rotation_p75_pct`, `rotation_samples`,
  `change_ratio_median_pct`, `change_ratio_p75_pct`, and
  `change_ratio_samples`.
- Published related artifacts: `history`, `comparison`, `commit_history`,
  `geo`, and provider-specific classification routes.
- Operational state: `health`, `errors`, `downloader`, `processor`,
  `pre_processor`, `used_for`, `hidden`, `dont_redistribute`,
  `merge_included`, `merge_subtracted`, and `merge_excluded`.

## Raw IP list

```
GET /api/v1/sets/{name}/data
```

Returns the raw IP list as `text/plain`. One CIDR range or IP address per line.

This endpoint streams the canonical feed body from disk. It does not load the entire file into memory. Archived and non-redistributable feeds return an error.

Example:

```
GET /api/v1/sets/firehol_level1/data
```

Response:

```
1.2.3.0/24
10.20.30.0/24
192.168.1.1
```

## Feed history

```
GET /api/v1/sets/{name}/history
```

Returns feed history as CSV with columns: `DateTime`, `Entries`, `UniqueIPs`.

Each row represents one historical snapshot of the feed's size. Snapshots are sparse — they exist only for observed successful updates.
`DateTime` is a Unix timestamp in seconds.

Example:

```
GET /api/v1/sets/firehol_level1/history
```

Response:

```csv
DateTime,Entries,UniqueIPs
1736920800,142,9834
1736942400,145,10012
1736964000,139,9756
```

## Pairwise comparison

```
GET /api/v1/sets/{name}/compare
GET /api/v1/sets/{name}/comparison
```

Returns a JSON object with pairwise overlap data between this feed and all other public feeds.

The comparison contains only non-zero overlaps. Feeds with zero shared IPs are absent from the results.

Example:

```
GET /api/v1/sets/firehol_level1/compare
```

Key response fields: array of comparison rows with `name`, `category`, `ips`, `common`, and optional `related`.

## Retention analysis

```
GET /api/v1/sets/{name}/retention
```

Returns a JSON summary of IP retention patterns. Shows how many IPs are in past and current age buckets based on observed additions and removals.

Example:

```
GET /api/v1/sets/firehol_level1/retention
```

Key response fields: `ipset`, `started`, `updated`, `incomplete`, `past`, and `current`.
The `past` and `current` objects each contain aligned `hours` and `ips` arrays plus a `total` count.

## Deterministic insights

```
GET /api/v1/sets/{name}/insights
```

Returns a deterministic insight payload generated from the feed's current state, comparison data, and retention patterns. Insights are precomputed — the endpoint reads a published artifact and does not compute anything at request time.

Example:

```
GET /api/v1/sets/firehol_level1/insights
```

## Changesets

```
GET /api/v1/sets/{name}/changesets
```

Returns changeset data as JSON. Changesets record the additions and removals between consecutive feed updates.

Example:

```
GET /api/v1/sets/firehol_level1/changesets
```

Key response fields: array of changeset entries with added/removed IP counts and timestamps.

## Per-feed scoped search

```
GET /api/v1/sets/{name}/search?ip=1.2.3.4
```

Returns whether the given IP address is present in this specific feed. This is a scoped version of the global search endpoint — it checks only one feed instead of all public feeds. The response includes `scope: "feed"`, `searched_feed`, and a structured `matches` array; a miss returns HTTP 200 with an empty `matches` array.

Example:

```
GET /api/v1/sets/firehol_level1/search?ip=1.2.3.4
```

## Per-feed classification data

Each feed has sub-routes for country, ASN, bogon, and critical-infrastructure overlap data. These return precomputed comparison artifacts.

### Countries

```
GET /api/v1/sets/{name}/countries
GET /api/v1/sets/{name}/countries/{provider}
```

The provider-list route returns configured GeoIP providers in default-first
order. It does not prove that every provider has a materialized artifact for
this feed. Request a specific `{provider}` to retrieve that provider's
precomputed country comparison data; missing provider artifacts return `404`.

### ASN

```
GET /api/v1/sets/{name}/asn
GET /api/v1/sets/{name}/asn/{provider}
```

The provider-list route returns configured ASN providers in default-first order.
It does not prove that every provider has a materialized artifact for this
feed. Request a specific `{provider}` to retrieve that provider's precomputed
ASN attribution data; missing provider artifacts return `404`.

### Bogons

```
GET /api/v1/sets/{name}/bogons
GET /api/v1/sets/{name}/bogons/{provider}
```

The provider-list route returns configured bogon providers. Hidden bogon
providers can appear here because the route describes reference data, not
navigable feed pages. Request a specific `{provider}` to retrieve that
provider's precomputed bogon overlap data; missing provider artifacts return
`404`.

### Infrastructure

```
GET /api/v1/sets/{name}/infrastructure
GET /api/v1/sets/{name}/infrastructure/providers
GET /api/v1/sets/{name}/infrastructure/{provider}
```

Returns the critical infrastructure aggregate, list of providers, or provider-specific overlap data.
