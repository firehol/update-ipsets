# API Overview

You will learn the public API structure, response formats, error handling, and versioning conventions.

## Base path

All public API endpoints live under `/api/v1/`.

## Methods

All public endpoints accept `GET` and `HEAD` only. Unsupported methods on known routes return `405 Method Not Allowed` with an `Allow` header.

`OPTIONS` requests are handled by CORS middleware before route dispatch. See [Rate Limits and CORS](rate-limits-cors.md).

## Response formats

| Format | Content-Type | Used for |
|--------|-------------|----------|
| JSON | `application/json` | Most endpoints: metadata, catalogs, search results, comparisons |
| Text | `text/plain` | Raw IP lists, composed sets |
| CSV | `text/csv` | Feed history (`DateTime,Entries,UniqueIPs`) |

## Error responses

The API uses standard HTTP status codes:

- `200` — success
- `404` — feed or resource not found
- `405` — method not allowed
- `429` — rate limit exceeded
- `500` — internal server error
- `503` — service not ready (artifact missing, not computed yet)

## Alias routes

Two backward-compatible alias families exist:

- `/api/v1/ipsets` and `/api/v1/ipsets/{name}` → same as `/api/v1/sets` and `/api/v1/sets/{name}`
- `/api/v1/query` → same as `/api/v1/search`

These aliases exist for compatibility with the bash-era tooling. Use the canonical paths for new integrations.

## Versioning

The current version is `v1`. Payload schemas may evolve compatibly within v1 (new fields, not removed fields). The endpoint families and their semantic purpose are part of the public contract.

## Next steps

- [Health and Status](health-status.md) — health check and runtime status endpoints
- [Feed Endpoints](feed-endpoints.md) — catalog, detail, data, history, comparison, per-feed classification
- [Search and Query](search-query.md) — global IP lookup, per-feed scoped search
- [Compose Endpoint](compose-endpoint.md) — custom IP set composition
- [Classification Endpoints](classification-endpoints.md) — country, ASN, maintainer indexes
- [Infrastructure Endpoints](infrastructure-endpoints.md) — critical infrastructure overlap data
- [Methodology Endpoints](methodology-endpoints.md) — methodology page content
- [MCP Endpoint](mcp-endpoint.md) — Model Context Protocol endpoint for AI agents
- [Metadata Files](metadata-files.md) — robots.txt, sitemap, llms.txt
- [Raw File Downloads](raw-file-downloads.md) — direct .ipset/.netset file access
- [Rate Limits and CORS](rate-limits-cors.md) — rate limiting and cross-origin rules
