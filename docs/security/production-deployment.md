# Production Deployment

You will learn the recommended production setup with split listeners, TLS, and firewall rules.

## Recommended setup

The installer defaults to a private-network operating model:

- public listener on `127.0.0.1:18888`
- admin listener on the Tailscale IPv4 address when Tailscale is available, otherwise `127.0.0.1:18889`
- admin authentication disabled with `--allow-unauthenticated-admin`

Use that default only when tailnet or localhost reachability is the admin
access-control layer. For internet-facing, shared, or untrusted networks, use
split listener mode with required authentication:

First set the external public URL in the active catalog:

```yaml
runtime:
  public_base_url: "https://iplists.example.org"
  web_url: "https://iplists.example.org/ipsets/"
```

```bash
UPDATE_IPSETS_ADMIN_USER=admin \
UPDATE_IPSETS_ADMIN_PASSWORD=your-secret-password \
update-ipsets daemon \
  --config /opt/update-ipsets/etc/config \
  --listen :18888 \
  --admin-listen 127.0.0.1:18889 \
  --admin-auth-mode=required \
  --enable-all
```

- Public traffic on `:18888` (all interfaces)
- Admin traffic on `127.0.0.1:18889` (localhost only)
- Admin requires Basic Auth
- When split mode is active, admin routes return 404 on the public listener
- The daemon refuses split mode unless `runtime.public_base_url` is set

## TLS options

### Built-in TLS

The daemon supports TLS directly:

```bash
update-ipsets daemon \
  --listen :443 \
  --tls-cert /etc/ssl/certs/update-ipsets.pem \
  --tls-key /etc/ssl/private/update-ipsets.key
```

### Reverse proxy (recommended)

Use nginx, Caddy, or HAProxy in front of the daemon. This handles TLS termination, HTTP/2, and connection buffering.

nginx example:

```nginx
server {
    listen 443 ssl;
    server_name iplists.example.org;

    ssl_certificate /etc/ssl/certs/example.pem;
    ssl_certificate_key /etc/ssl/private/example.key;

    location / {
        proxy_pass http://127.0.0.1:18888;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Do not buffer large feed downloads
        proxy_buffering off;
        proxy_request_buffering off;
    }
}
```

Caddy example:

```
iplists.example.org {
    reverse_proxy localhost:18888
}
```

Caddy handles TLS automatically with Let's Encrypt.

### Reverse proxy tips

- Set `runtime.public_base_url` and `runtime.web_url` to the externally visible
  site URLs. The daemon does not derive generated public URLs from `Host` or
  `X-Forwarded-Proto` request headers.
- Passing `Host` and `X-Forwarded-Proto` is still normal reverse-proxy hygiene,
  but those headers do not replace the runtime URL settings.
- Disable response buffering for large feed downloads
- Keep request body limits modest. Admin actions are bodyless POST requests; the public `/mcp` endpoint sends small JSON-RPC messages.

## Trusted proxy configuration

By default the daemon uses the TCP connection source address (`RemoteAddr`) for rate limiting, logging, and the `/api/v1/client-ip` endpoint. Forwarded headers (`X-Forwarded-For`, `X-Real-IP`, `CF-Connecting-IP`) are ignored unless you explicitly enable them.

This is a **security decision**: enabling these headers without a trusted proxy in front allows any client to spoof their IP and bypass per-client rate limits.

### Deployment checklist

Decide which topology matches your infrastructure before enabling header trust:

| Topology | Flags to set | Risk |
|---|---|---|
| Direct exposure (no proxy) | none | None — default is secure |
| Reverse proxy (nginx/Caddy/Haproxy) | `--trust-proxy-headers` | Low — only your proxy sets headers |
| Cloudflare | `--trust-cloudflare-headers` | Low — Cloudflare strips and sets `CF-Connecting-IP` |
| Cloudflare + own proxy | both flags | Low — both header types trusted |

### CLI flags

```bash
# Trust X-Forwarded-For and X-Real-IP from your reverse proxy
update-ipsets daemon --trust-proxy-headers

# Trust CF-Connecting-IP from Cloudflare
update-ipsets daemon --trust-cloudflare-headers

# Both (Cloudflare -> your proxy -> daemon)
update-ipsets daemon --trust-proxy-headers --trust-cloudflare-headers
```

### YAML configuration

You can also set these in your runtime config:

```yaml
runtime:
  trust_proxy_headers: true
  trust_cloudflare_headers: true
```

### Header priority

When enabled, headers are checked in this order:

1. `CF-Connecting-IP` (only if `--trust-cloudflare-headers`)
2. `X-Forwarded-For` first entry (only if `--trust-proxy-headers`)
3. `X-Real-IP` (only if `--trust-proxy-headers`)
4. TCP `RemoteAddr` (always, as fallback)

### Security implications

- **Without trust flags** (default): forwarded headers are ignored. Rate limiting, logging, and `/api/v1/client-ip` all use the real TCP source. This is the safest configuration.
- **With `--trust-proxy-headers`**: any client that can reach the daemon directly can set `X-Forwarded-For` or `X-Real-IP` and bypass rate limits. Only use this when a trusted proxy is the only path to the daemon.
- **With `--trust-cloudflare-headers`**: similar risk, but scoped to the `CF-Connecting-IP` header. Only use when traffic arrives exclusively through Cloudflare.

## Firewall rules

Block the admin port from external access:

```bash
# Allow admin only from localhost
sudo iptables -A INPUT -p tcp --dport 18889 -s 127.0.0.1 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 18889 -j DROP
```

Or with `ufw`:

```bash
sudo ufw deny 18889
```

If you use a reverse proxy on the same host, the admin port only needs to be reachable from the proxy process. Binding to `127.0.0.1` achieves this without firewall rules.

## systemd configuration

The installed systemd unit supports environment-based configuration. Use a drop-in:

```ini
# /etc/systemd/system/update-ipsets.service.d/production.conf
[Service]
Environment="UPDATE_IPSETS_LISTEN=:18888"
Environment="UPDATE_IPSETS_ADMIN_LISTEN_ARG=--admin-listen=127.0.0.1:18889"
Environment="UPDATE_IPSETS_ADMIN_AUTH_ARG=--admin-auth-mode=required"
Environment="UPDATE_IPSETS_ALLOW_UNAUTHENTICATED_ADMIN_ARG="
Environment="UPDATE_IPSETS_ADMIN_USER=admin"
Environment="UPDATE_IPSETS_ADMIN_PASSWORD=your-secret-password"
```

Apply:

```bash
sudo systemctl daemon-reload
sudo systemctl restart update-ipsets
```

## Memory limits

Configure memory limits for production:

```ini
[Service]
MemoryHigh=640M
MemoryMax=768M
Environment="GOMEMLIMIT=512MiB"
```

This gives the daemon a soft memory budget. It degrades performance instead of crashing when memory pressure increases.

## Health checks

Use `/healthz` for load balancer health checks:

```nginx
# nginx upstream health check
location /healthz {
    proxy_pass http://127.0.0.1:18888/healthz;
}
```

The endpoint returns `ok` with HTTP 200 when the daemon is running.
