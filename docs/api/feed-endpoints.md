# Feed Endpoints

You will learn the feed-related API endpoints, what each returns, and how to use them.

## Feed catalog

```
GET /api/v1/sets
```

Lists all public feeds with summary metadata. Each entry includes feed name, category, IP version, current size, health state, maintainer, and critical-infrastructure overlap tiers where applicable.

Example:

```
GET /api/v1/sets
```

Key response fields per feed: `name`, `category`, `ip_version`, `entries`, `unique_ips`, `health`, `maintainer`.

## Feed detail

```
GET /api/v1/sets/{name}
```

Returns full metadata for a single feed. Includes identification, data characteristics, update statistics, health class, merge composition (for merge feeds), and provider enrichment summaries.

Example:

```
GET /api/v1/sets/firehol_level1
```

Key response fields: `name`, `category`, `maintainer`, `license`, `redistributable`, `ip_version`, `entries`, `unique_ips`, `health`, `tracked`, `updated`, `processed`, `source_timestamp`.

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

Example:

```
GET /api/v1/sets/firehol_level1/history
```

Response:

```csv
DateTime,Entries,UniqueIPs
2025-01-15T06:00:00Z,142,9834
2025-01-15T12:00:00Z,145,10012
2025-01-15T18:00:00Z,139,9756
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

Key response fields: array of `{ peer, common_ips, common_pct_self, common_pct_peer }` objects.

## Retention analysis

```
GET /api/v1/sets/{name}/retention
```

Returns a JSON summary of IP retention patterns. Shows how long IPs tend to remain in the feed based on observed additions and removals.

Example:

```
GET /api/v1/sets/firehol_level1/retention
```

Key response fields: retention distribution buckets, median retention, age of oldest current IP.

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

Returns whether the given IP address is present in this specific feed. This is a scoped version of the global search endpoint — it checks only one feed instead of all public feeds.

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

Returns the list of geo providers for this feed, or country comparison data for a specific provider.

### ASN

```
GET /api/v1/sets/{name}/asn
GET /api/v1/sets/{name}/asn/{provider}
```

Returns the list of ASN providers, or ASN data for a specific provider.

### Bogons

```
GET /api/v1/sets/{name}/bogons
GET /api/v1/sets/{name}/bogons/{provider}
```

Returns the list of bogon providers, or bogon overlap data for a specific provider.

### Infrastructure

```
GET /api/v1/sets/{name}/infrastructure
GET /api/v1/sets/{name}/infrastructure/providers
GET /api/v1/sets/{name}/infrastructure/{provider}
```

Returns the critical infrastructure aggregate, list of providers, or provider-specific overlap data.
