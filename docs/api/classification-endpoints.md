# Classification Endpoints

You will learn how to browse feeds by country, ASN, and maintainer through the public API.

## How these endpoints work

Country and ASN endpoints serve precomputed published artifacts. They do not aggregate data at request time. When an artifact is missing, the endpoint returns a service-unavailable or not-found response.

Maintainer endpoints are derived from the current public catalog state. They are still read-only and cheap, but they are not backed by country/ASN entity artifact files.

## Countries

### Country index

```
GET /api/v1/countries
```

Returns a list of all countries that appear in any public feed, with summary counts.

Key response fields per country: `code`, `name`, `feeds`, `ips`, `categories`, `asns`.

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

Key response fields: country identity, summary totals, feed composition by category, country-specific ASN block.

## Autonomous systems

### ASN index

```
GET /api/v1/asns
```

Returns a list of all ASNs that appear in any public feed, with summary counts.

Key response fields per ASN: `asn`, `name`, `feeds`, `ips`, `categories`, `countries`.

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

Key response fields: ASN identity, ASN name, summary totals, feed composition by category, country distribution block.

## Maintainers

### Maintainer index

```
GET /api/v1/maintainers
```

Returns a list of all maintainers that have public feeds in the catalog.

Optional query parameters:

| Parameter | Meaning |
|---|---|
| `categories` | Comma-separated public category names. Only maintainers with at least one feed in those categories are returned. |

Key response fields per maintainer: `slug`, `name`, `feeds`, `categories`.

### Maintainer detail

```
GET /api/v1/maintainers/{slug}
```

Returns full detail for one maintainer. Includes their public feeds with metadata.

Example:

```
GET /api/v1/maintainers/firehol
```

Key response fields: maintainer identity, feed list with per-feed metadata.
