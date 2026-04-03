# Classification Endpoints

You will learn how to browse feeds by country, ASN, and maintainer through the public API.

## How these endpoints work

Classification endpoints serve precomputed published artifacts. They do not aggregate data at request time. When an artifact is missing, the endpoint returns a service-unavailable or not-found response.

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

The `{code}` parameter is a two-letter ISO country code.

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

The `{asn}` parameter is an ASN number (digits only, without the `AS` prefix).

Example:

```
GET /api/v1/asns/13335
```

Key response fields: ASN identity, ASN name, summary totals, feed composition by category, country distribution block.

## Maintainers

### Maintainer index

```
GET /api/v1/maintainers
```

Returns a list of all maintainers that have public feeds in the catalog.

Unlike countries and ASNs, maintainer data is served from live engine state, not precomputed artifacts.

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
