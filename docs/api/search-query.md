# Search and Query

You will learn how to look up which feeds contain a given IPv4 address, how to get detailed match info, and what `first_seen` means.

## Basic IP search

```
GET /api/v1/search?ip=1.2.3.4
```

Returns a list of public feeds that currently contain the requested IP address.

Example:

```
GET /api/v1/search?ip=1.2.3.4
```

Response (simplified):

```json
{
  "ip": "1.2.3.4",
  "matches": ["firehol_level1", "spamhaus_drop"]
}
```

## Detailed match info

```
GET /api/v1/search?ip=1.2.3.4&details=true
```

Returns structured match objects with feed metadata such as name, file, category, provenance, maintainer, health, and timestamps when available. The response may also include best-effort IP context from the configured default country, ASN, and critical-infrastructure providers.

Use this when you need to present feed information alongside the match results without making separate requests for each feed.

## Legacy alias

```
GET /api/v1/query?ip=1.2.3.4
```

The `/api/v1/query` endpoint is a backward-compatible alias for `/api/v1/search`. It accepts the same parameters and returns the same response. Use `/api/v1/search` for new integrations.

## What `first_seen` means

The `first_seen` timestamp represents the start of the current contiguous listing interval for that IP in that feed.

This means:

- the IP has been continuously present in the feed since that timestamp
- it may have been in the feed earlier, left, and come back — `first_seen` only covers the current unbroken stay
- the value is a Unix timestamp in seconds
- the value comes from retention tracking, not from full historical scans

## Rate limiting

IP search has its own rate limit: **10 requests per minute per client**. This is independent of the general API rate limit (240/min).

See [Rate Limits and CORS](rate-limits-cors.md) for details.
