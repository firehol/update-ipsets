# Rate Limits and CORS

You will learn the rate limiting policy, CORS headers, and compression behavior for the HTTP API surfaces.

## Rate limits

### General API

Paths under `/api/` and the MCP endpoint are rate limited at **240 requests per minute per client**.

This includes public API paths, admin API paths under `/api/v1/admin/*`, and the MCP endpoint at `/mcp`. MCP tool calls (`find_feeds`, `fetch_analysis`) share this general limit because they read precomputed metadata, not scan live feed entries.

The rate limiter uses a token-bucket algorithm. Per-client state is tracked by source IP address.

### IP search

IP search endpoints (`/api/v1/search`, `/api/v1/query`, and per-feed `/api/v1/sets/{name}/search` or `/api/v1/ipsets/{name}/search`) have an additional rate limit of **10 requests per minute per client**.

Search requests also pass through the general `/api/` limiter. The effective search limit is the stricter search bucket unless the client has already exhausted the general API bucket.

### Excluded from limits

These paths are not rate limited:

- `/healthz` — designed for high-frequency load balancer checks
- `/admin` and `/admin/*` — the admin SPA shell

Admin API routes under `/api/v1/admin/*` are still under the general `/api/` rate limiter.

## Rate limit responses

When a client exceeds the rate limit, the API returns `429 Too Many Requests`.

## CORS

### Public endpoints

All public endpoints set these CORS headers on `GET` and `OPTIONS` responses:

```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, OPTIONS
Access-Control-Allow-Headers: Content-Type
```

This allows any origin to call the public API from browser-based applications.

### MCP endpoint

The MCP endpoint at `/mcp` sets CORS headers that additionally allow POST and DELETE, required by the Streamable HTTP transport:

```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, POST, DELETE, OPTIONS
Access-Control-Allow-Headers: Content-Type, Mcp-Session-Id, MCP-Protocol-Version, Last-Event-ID
Access-Control-Expose-Headers: Mcp-Session-Id
```

This enables browser-based MCP clients to connect directly. See [MCP Endpoint](mcp-endpoint.md).

### Admin endpoints

Admin endpoints do **not** set CORS headers. This prevents cross-origin credential theft from basic-auth protected admin surfaces.

`OPTIONS` requests to admin paths may return `204 No Content`, but without wildcard CORS headers.

## Compression

The server applies gzip compression when the client sends `Accept-Encoding: gzip`.

Compression applies to:

- all paths starting with `/api/`
- all paths starting with `/static/`
- responses with `.json`, `.xml`, `.txt`, `.csv`, `.js`, `.css`, `.html` suffixes
- the root path `/`

Compressed responses include:

```
Content-Encoding: gzip
Vary: Accept-Encoding
```
