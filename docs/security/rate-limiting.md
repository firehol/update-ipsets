# Rate Limiting

You will learn how rate limiting works on the public API and how to tune it for your deployment.

## Limits

| Endpoint group | Limit |
|---------------|-------|
| General API | 240 requests/minute per client IP |
| IP search (`/api/v1/search`, `/api/v1/query`) | 10 requests/minute per client IP |

Search requests also consume tokens from the general `/api/` bucket before the stricter search bucket is checked. For search/query traffic, the effective limit is the stricter search bucket unless the client has already exhausted the general API bucket.

## Excluded endpoints

These endpoints are not rate-limited:

- `/healthz` — health checks need to work unconditionally
- `/admin` and `/admin/*` — the browser shell is protected by admin authentication

Admin API routes under `/api/v1/admin/*` are still under the general `/api/` rate limiter.

## Implementation

Rate limiting uses a per-client token bucket. Each client IP gets its own bucket. Tokens refill at the configured rate. Requests that exceed the bucket are rejected with HTTP 429.

The "client IP" is determined by the trusted proxy policy. By default, the daemon uses the TCP connection source address. When behind a reverse proxy or Cloudflare, you must enable `--trust-proxy-headers` or `--trust-cloudflare-headers` for rate limiting to key by the real client IP. See [Production Deployment](production-deployment.md#trusted-proxy-configuration) for details.

Cleanup is lazy — expired client buckets are removed when new clients arrive, not by a background goroutine. This avoids unbounded background processes.

## What rate limiting is not

Rate limiting prevents accidental abuse and load spikes. It is not a security control. A determined attacker can distribute requests across IPs or use other techniques.

For security, rely on:

- Authentication on the admin surface
- Network-level controls (firewall, reverse proxy)
- TLS for encryption in transit

## Path traversal protection

All artifact and file-serving routes validate the request path against directory traversal attacks. Attempts to access files outside the published artifact directory (e.g., `/api/v1/sets/../../../etc/passwd`) are rejected.

## When clients hit the limit

Clients exceeding the rate limit receive:

```
HTTP 429 Too Many Requests
Retry-After: <seconds>
```

The `Retry-After` header tells the client when it can retry.

## Tuning

Rate limits are not currently configurable at runtime. If you need different limits for your deployment, use a reverse proxy with its own rate limiting:

```nginx
# nginx rate limiting example
limit_req_zone $binary_remote_addr zone=api:10m rate=10r/s;

location /api/ {
    limit_req zone=api burst=20 nodelay;
    proxy_pass http://127.0.0.1:18888;
}
```

Place the reverse proxy in front of the daemon and let it enforce the deployment-specific limits you need. The daemon's built-in limits remain a backstop for `/api/` and `/mcp` requests.
