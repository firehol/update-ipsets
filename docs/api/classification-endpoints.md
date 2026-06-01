# Classification Endpoints

You will learn how to browse feeds by country, ASN, and maintainer through the public API.

## How these endpoints work

Country and ASN endpoints serve precomputed published artifacts. They do not aggregate data at request time. When an artifact is missing, the endpoint returns a service-unavailable or not-found response.

Maintainer endpoints are derived from current public feed state. They are still read-only and cheap, but they are not backed by country/ASN entity artifact files.

The maintainer index and detail endpoints include feeds that are eligible for homepage-style aggregation:

- public category
- not hidden
- not ASN or GeoIP provider-only data
- provenance `primary` or `secondary_upstream`
- current health `healthy` or `delayed`

## Countries

### Country index

```
GET /api/v1/countries
```

Returns the configured country provider plus a list of all countries that
appear in any public feed, with summary counts.

Key response fields:

- top level: `provider`, `countries`
- per country: `code`, `feed_count`, `attributed_ips`

### Country detail

```
GET /api/v1/countries/{code}
```

Returns full detail for one country. Includes matching feeds grouped by category, ASN composition specific to that country, and summary statistics.

The `{code}` parameter is a two-letter ISO country code. Lowercase input is accepted and normalized.

Example:

```
GET /api/v1/countries/US
```

Key response fields: `code`, `provider`, `totals`, `feeds`,
`feeds_by_category`, `top_categories`, `top_maintainers`,
`top_asns_in_country`, and `asn_provider`.

## Autonomous systems

### ASN index

```
GET /api/v1/asns
```

Returns the configured ASN provider plus a list of all ASNs that appear in any
public feed, with summary counts.

Key response fields:

- top level: `provider`, `asns`
- per ASN: `asn`, `name`, `feed_count`, `attributed_ips`

### ASN detail

```
GET /api/v1/asns/{asn}
```

Returns full detail for one ASN. Includes matching feeds grouped by category, country distribution, and summary statistics.

The `{asn}` parameter is an ASN number. The optional `AS` prefix is accepted and normalized.

Example:

```
GET /api/v1/asns/13335
GET /api/v1/asns/AS13335
```

Key response fields: `asn`, `name`, `provider`, `geo_provider`, `totals`,
`feeds`, `feeds_by_category`, `top_categories`, `top_maintainers`,
`top_countries`, and `country_distribution`.

## Maintainers

### Maintainer index

```
GET /api/v1/maintainers
```

Returns maintainers that have at least one currently eligible public feed.

Optional query parameters:

| Parameter | Meaning |
|---|---|
| `categories` | Comma-separated public category names. Only maintainers with at least one currently eligible feed in those categories are returned. |

Key response fields per maintainer: `slug`, `name`, `url`, `feed_count`,
`unique_ips`, `categories`.

### Maintainer detail

```
GET /api/v1/maintainers/{slug}
```

Returns full detail for one maintainer. Includes their currently eligible public feeds with metadata.

Example:

```
GET /api/v1/maintainers/firehol
```

Key response fields: `slug`, `name`, `url`, `totals`, and
`feeds_by_category`.
