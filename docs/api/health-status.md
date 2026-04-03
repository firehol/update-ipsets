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

- service availability state
- uptime
- catalog counts (number of public feeds, countries, ASNs)
- basic service health indicators

Example response (simplified):

```json
{
  "status": "running",
  "uptime_seconds": 86400,
  "feeds": 523,
  "countries": 249,
  "asns": 12480
}
```

## What status does not expose

This endpoint is designed for public consumers. It does not expose:

- queue backlog or active execution details
- internal filesystem paths
- admin runtime internals
- operator-only state

For detailed operational status, use the admin endpoints at `/api/v1/admin/status`.

## Categories

```
GET /api/v1/categories
```

Returns the list of feed categories with metadata. Each category has a slug, display name, and description.

This endpoint serves the category index used by the public UI feed browser.

## Home page data

```
GET /api/v1/home/globe
GET /api/v1/home/summary
```

These endpoints serve precomputed data for the public homepage:

- `globe` — geographic visualization data for the homepage globe widget
- `summary` — aggregated summary statistics (total feeds, total IPs, recent activity)
