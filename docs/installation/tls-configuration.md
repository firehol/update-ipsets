# TLS Configuration

You will learn how to serve the public site and admin dashboard over HTTPS, either with the built-in TLS support or through a reverse proxy.

## Built-in TLS

The daemon accepts TLS flags directly:

```bash
UPDATE_IPSETS_ADMIN_USER=admin \
UPDATE_IPSETS_ADMIN_PASSWORD=change-this-secret \
update-ipsets daemon \
    --config /opt/update-ipsets/etc/config \
    --listen :443 \
    --tls-cert /etc/ssl/certs/iplists.example.com.pem \
    --tls-key /etc/ssl/private/iplists.example.com.key
```

With a separate admin listener:

Split listener mode requires `runtime.public_base_url` in the active catalog.

```bash
UPDATE_IPSETS_ADMIN_USER=admin \
UPDATE_IPSETS_ADMIN_PASSWORD=change-this-secret \
update-ipsets daemon \
    --config /opt/update-ipsets/etc/config \
    --listen :443 \
    --tls-cert /etc/ssl/certs/iplists.example.com.pem \
    --tls-key /etc/ssl/private/iplists.example.com.key \
    --admin-listen 127.0.0.1:18889 \
    --admin-auth-mode=required
```

### Systemd drop-in for built-in TLS

Create `/etc/systemd/system/update-ipsets.service.d/tls.conf`:

```ini
[Service]
Environment="UPDATE_IPSETS_LISTEN=:443"
ExecStart=
ExecStart=/opt/update-ipsets/bin/update-ipsets daemon \
    --config /opt/update-ipsets/etc/config \
    --listen ${UPDATE_IPSETS_LISTEN} \
    --tls-cert /etc/ssl/certs/iplists.example.com.pem \
    --tls-key /etc/ssl/private/iplists.example.com.key \
    ${UPDATE_IPSETS_ADMIN_LISTEN_ARG} \
    ${UPDATE_IPSETS_ADMIN_AUTH_ARG} \
    ${UPDATE_IPSETS_ALLOW_UNAUTHENTICATED_ADMIN_ARG} \
    --enable-all \
    --verbose \
    --web-dir /opt/update-ipsets/web \
    --web-files-dir /opt/update-ipsets/web/files
```

The blank `ExecStart=` clears the inherited value before setting the new one. This is required when overriding `ExecStart` in a drop-in.

### Certificate renewal

After renewing certificates, restart the service to load the new files:

```bash
sudo systemctl restart update-ipsets
```

The daemon does not watch certificate files for changes.

## Reverse proxy alternative

Run the daemon on localhost and put nginx or Caddy in front. The proxy handles TLS termination.

### Daemon side

```ini
[Service]
Environment="UPDATE_IPSETS_LISTEN=127.0.0.1:18888"
```

### Nginx example

```nginx
server {
    listen 443 ssl;
    server_name iplists.example.com;

    ssl_certificate /etc/ssl/certs/iplists.example.com.pem;
    ssl_certificate_key /etc/ssl/private/iplists.example.com.key;

    location / {
        proxy_pass http://127.0.0.1:18888;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Generated public URLs come from `runtime.public_base_url` and
`runtime.web_url`, not from the request `Host` or `X-Forwarded-Proto` headers.
Keep those runtime settings aligned with the external HTTPS URL.

### Caddy example

```
iplists.example.com {
    reverse_proxy 127.0.0.1:18888
}
```

Caddy provisions certificates automatically via ACME (Let's Encrypt).

## Choosing an approach

| | Built-in TLS | Reverse proxy |
|---|---|---|
| Simplicity | Fewer moving parts, one process | Extra service to manage |
| Certificate automation | Handle externally (certbot, etc.) | Caddy does it automatically; nginx needs certbot |
| Admin isolation | Admin listener also gets TLS | Admin can stay on localhost, no TLS needed for it |
| Overhead | None | One extra hop, negligible for this workload |
| Use case | Simple deployments, single-purpose servers | Shared servers, existing proxy infrastructure |

For a dedicated server running only update-ipsets, built-in TLS is the simplest option. For servers that already run a reverse proxy, keep using it.
