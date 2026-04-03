# Accessing the Admin UI

You will learn how to reach the admin UI, how authentication works, and how to set up local development access.

## URL

The admin UI lives at `/admin` on the daemon's listener.

| Mode | URL |
|---|---|
| **Shared** (default) | `http://host:port/admin` |
| **Split** | `http://admin-host:admin-port/admin` |

In shared mode, public and admin routes share the same listener. In split mode, admin routes are served on a separate listener and return 404 on the public port.

## Authentication

The admin UI requires authentication by default. When you open `/admin` in a browser, you see an HTTP Basic auth popup.

Credentials come from environment variables:

```bash
export UPDATE_IPSETS_ADMIN_USER=admin
export UPDATE_IPSETS_ADMIN_PASSWORD=your-strong-password
```

If credentials are missing or wrong, access is denied. The daemon never falls back to open access when auth is required.

## Local development

For local development or trusted networks, disable authentication with two flags:

```bash
update-ipsets daemon \
  --admin-auth-mode=disabled \
  --allow-unauthenticated-admin
```

Two flags are required as a safety acknowledgment. The daemon does not infer "disabled" from missing credentials.

## What the admin UI is

The admin UI is a single-page application. It loads once in your browser and then queries admin API endpoints for live data.

All admin API endpoints live under `/api/v1/admin/`. The UI auto-refreshes to show current pipeline state.

## See also

- [Admin Authentication](../running/admin-authentication.md) — detailed authentication configuration
- [Listener Topologies](../running/listener-topologies.md) — shared vs. split mode setup
