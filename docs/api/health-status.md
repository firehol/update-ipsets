# Health and Status

You will learn how to monitor service health and retrieve runtime status information.

## Health check

```
GET /healthz
```

Returns the string `ok` with HTTP 200 when the service is running.

Use this endpoint for load balancer health checks, Kubernetes liveness probes, and monitoring systems. It is lightweight and does not depend on feed processing state.

Example:

```
$ curl http://localhost:18888/healthz
ok
```

This endpoint is excluded from rate limiting.

## Service status

```
GET /api/v1/status
```

Returns high-level public runtime facts as JSON.

The response includes:

- engine run state
- engine start/end timestamps
- source and merge counts known to the engine
- process uptime

Example response (simplified):

```json
{
  "engine": {
    "running": true,
    "last_started": "2026-05-31T10:20:30Z",
    "last_ended": "2026-05-31T10:21:45Z",
    "source_count": 423,
    "merge_count": 12
  },
  "system": {
    "uptime_seconds": 86400,
    "uptime": "24h0m0s"
  }
}
```

## What status does not expose

This endpoint is designed for public consumers. It does not expose:

- queue backlog or active execution details
- internal filesystem paths
- admin runtime internals
- operator-only state

For detailed operational status, use the admin endpoints at `/api/v1/admin/status`.

## Client IP utility

```
GET /api/v1/client-ip
```

Returns the client IPv4 address after the configured trusted-proxy policy is applied.

Example response:

```json
{
  "ip": "198.51.100.10"
}
```

If the client address is unavailable or is not IPv4, the endpoint returns an empty
string in the `ip` field. This endpoint is a public UI utility, not an authentication
or identity signal.

## Categories

```
GET /api/v1/categories
```

Returns the list of feed categories with metadata. Each category has a slug, display name, and description.

This endpoint serves the category index used by the public UI feed browser.

## Home page data

```
GET /api/v1/home/globe?categories=intrusion,malware_infrastructure
GET /api/v1/home/summary?categories=intrusion,malware_infrastructure&limit=10
```

These endpoints serve precomputed data for the public homepage:

- `globe` — geographic visualization data for the homepage globe widget. The `categories` query parameter is required.
- `summary` — aggregated summary statistics. `categories` is optional; when omitted, all public categories participate. `limit` optionally bounds top-N lists.
