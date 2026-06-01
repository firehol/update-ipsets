# Listener Topologies

You will learn the two listener modes the daemon supports, when to use each, and how admin routes behave in split mode.

## Shared mode (default)

Public and admin endpoints are served on the same listener. This is the default when `--admin-listen` is not set.

```
                     shared mode
                 ┌────────────────────┐
                 │   :18888 (one listener)   │
 clients ──────► │                        │
                 │  /api/v1/sets/...      │  public
                 │  /api/v1/search?ip=... │  public
                 │  /admin                │  admin
                 │  /api/v1/admin/...     │  admin
                 └────────────────────────┘
```

Admin endpoints on the shared listener require authentication when `--admin-auth-mode=required` is set.

Example:

```bash
UPDATE_IPSETS_ADMIN_USER=admin \
UPDATE_IPSETS_ADMIN_PASSWORD=change-this-secret \
update-ipsets daemon \
  --config /opt/update-ipsets/etc/config \
  --listen :18888 \
  --admin-auth-mode=required
```

Both public and admin traffic reach `:18888`.

## Split mode

Public and admin endpoints are served on separate listeners. Enable this with `--admin-listen`.

```
                     split mode
                 ┌────────────────────┐
  public  ──────►│   :18888 (public)       │
  clients        │  /api/v1/sets/...      │
                 │  /api/v1/search?ip=... │
                 │                        │
                 │  /admin ──────► 404    │
                 │  /api/v1/admin/ ► 404  │
                 └────────────────────────┘

                 ┌────────────────────┐
  admin   ──────►│   127.0.0.1:18889 (admin) │
  operators      │  /admin                │
                 │  /api/v1/admin/...     │
                 └────────────────────────┘
```

In split mode:

- The public listener serves only public endpoints.
- Requests to `/admin`, `/admin/*`, and `/api/v1/admin/*` on the public listener return 404. They do not redirect, authenticate, or serve admin content.
- The admin listener serves admin endpoints, the admin SPA, and embedded assets required by that SPA. It does not serve public API routes or public pages.
- `runtime.public_base_url` must be configured. The daemon refuses to start in split mode without it because admin-to-public links need an external public base URL.

Set the public base URL in the catalog before enabling `--admin-listen`:

```yaml
runtime:
  public_base_url: "https://iplists.firehol.org"
  web_url: "https://iplists.firehol.org/ipsets/"
```

Example:

```bash
UPDATE_IPSETS_ADMIN_USER=admin \
UPDATE_IPSETS_ADMIN_PASSWORD=change-this-secret \
update-ipsets daemon \
  --config /opt/update-ipsets/etc/config \
  --listen :18888 \
  --admin-listen 127.0.0.1:18889 \
  --admin-auth-mode=required
```

Public site: `http://your-server:18888`
Admin UI: `http://127.0.0.1:18889/admin`

## When to use each

| Scenario | Mode | Reason |
|---|---|---|
| Local testing | Shared | Simplest setup. Use `--admin-auth-mode=disabled` with `--allow-unauthenticated-admin`. |
| Production, single host | Split | Admin on localhost only. Public on external interface. No admin content exposed publicly. |
| Production, reverse proxy | Either | If the proxy handles access control, shared mode works. For defense in depth, use split mode and restrict the admin port at the network level. |

## Admin-to-public links

In split mode, the admin UI may link to public pages (feed details, for example). These links use the configured `public_base_url` from the catalog runtime settings, not the admin listener address.

Set this in your catalog:

```yaml
runtime:
  public_base_url: "https://iplists.firehol.org"
  web_url: "https://iplists.firehol.org/ipsets/"
```

Without `public_base_url`, the daemon rejects the split-listener configuration at startup.

## See also

- [Daemon Reference](daemon-reference.md) — all flags
- [Admin Authentication](admin-authentication.md) — auth modes and credential setup
- [Environment Variables](environment-variables.md) — systemd drop-in configuration
