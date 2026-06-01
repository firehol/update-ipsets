# Accessing the Admin UI

You will learn how to reach the admin UI, how authentication works, and how to set up local testing access.

## URL

The admin UI lives at `/admin` on the daemon's listener.

| Mode | URL |
|---|---|
| **Shared** (default) | `http://host:port/admin` |
| **Split** | `http://admin-host:admin-port/admin` |

In shared mode, public and admin routes share the same listener. In split mode, admin routes are served on a separate listener and return 404 on the public port.

Split mode also requires `runtime.public_base_url` in the catalog. The daemon refuses to start with `--admin-listen` unless it knows the public base URL used for admin-to-public links.

## Authentication

The admin UI requires authentication by default. When you open `/admin` in a browser, you see an HTTP Basic auth popup.

Credentials come from environment variables:

```bash
export UPDATE_IPSETS_ADMIN_USER=admin
export UPDATE_IPSETS_ADMIN_PASSWORD=your-strong-password
```

If the daemon is missing either required credential variable, admin routes fail closed with `503 Service Unavailable`. If credentials are configured but a browser or API client does not provide valid Basic auth, admin routes return `401 Unauthorized`. The daemon never falls back to open access when auth is required.

## Local testing

For local testing or trusted lab networks, disable authentication with two flags:

```bash
update-ipsets daemon \
  --admin-auth-mode=disabled \
  --allow-unauthenticated-admin
```

Two flags are required as a safety acknowledgment. The daemon does not infer "disabled" from missing credentials.

## What the admin UI is

The admin UI is a single-page application. It loads once in your browser and then queries admin API endpoints for live data.

All admin API endpoints live under `/api/v1/admin/`. The UI auto-refreshes to show current pipeline state.

In split mode, the admin listener serves the admin UI, admin API, and embedded assets required by the UI. It does not serve public API routes such as `/api/v1/status` or `/healthz`.

## See also

- [Admin Authentication](../running/admin-authentication.md) — detailed authentication configuration
- [Listener Topologies](../running/listener-topologies.md) — shared vs. split mode setup
