# Compose Endpoint

You will learn how to compose custom IP sets on the fly from existing feeds.

## Endpoint

```
GET /api/v1/compose?include=set1,set2&exclude=set3&format=cidr
```

Compose takes existing public feeds, combines them using set operations, and returns the result as plain text.

## Parameters

| Parameter | Required | Description |
|-----------|----------|-------------|
| `include` | Yes | Comma-separated list of feed names to include (union). Maximum 20 names. |
| `exclude` | No | Comma-separated list of feed names to exclude (subtraction). Maximum 20 names. |
| `format` | No | Output format. Accepted values: `cidr`, `net`, `nets` (default CIDR output); `range`, `ranges`; `single`, `ip`, `ips`. |

## How it works

The daemon opens the latest local set representation for each named feed,
preferring committed binary set files and falling back to the materialized
`.ipset` / `.netset` body when needed. It performs the union of all included
feeds, subtracts all excluded feeds, buffers the composed output with the
32 MiB cap below, and returns the result.

The compose operation runs entirely against local committed data. It does not trigger downloads or recomputation.

The endpoint caps output at 32 MiB. Very large compositions return an error
instead of producing an unbounded response.

## Output format

The response is `text/plain` with one entry per line.

**CIDR format** (default):

```
1.2.3.0/24
10.20.0.0/16
```

**Range format:**

```
1.2.3.0-1.2.3.255
10.20.0.0-10.20.255.255
```

**Single IP format:**

```
1.2.3.0
1.2.3.1
...
```

Note: `single` format expands all ranges into individual IPs. For large feeds, this can produce very large responses.

## Example

```
GET /api/v1/compose?include=firehol_level1,firehol_level2&exclude=firehol_webserver&format=cidr
```

This returns the union of firehol_level1 and firehol_level2, with anything also in firehol_webserver removed.

## Eligibility rules

Compose applies the same checks as individual feed endpoints:

- archived feeds are excluded from composition
- non-redistributable feeds are excluded from composition
- hidden feeds are excluded from composition
- ASN and GeoIP provider databases are excluded from composition because they are not public feed bodies

Bogon sources are different from ASN and GeoIP databases: a redistributable,
non-archived bogon source can participate when it is configured as a public feed.

If any named feed fails these checks, the endpoint returns an error.
