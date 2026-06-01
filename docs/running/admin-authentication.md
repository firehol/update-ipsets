# Admin Authentication

You will learn the two admin authentication modes, how to set credentials, why disabling auth requires two flags, and how the fail-closed design protects you from misconfiguration.

## Authentication modes

The daemon supports two modes, controlled by `--admin-auth-mode`:

| Mode | Flag | Behavior |
|---|---|---|
| `required` (default) | `--admin-auth-mode=required` | Admin endpoints require HTTP Basic authentication. Missing or wrong credentials deny access. |
| `disabled` | `--admin-auth-mode=disabled` | Admin endpoints skip authentication. Requires a second safety flag. |

## Required mode

Set credentials through environment variables:

```bash
export UPDATE_IPSETS_ADMIN_USER=admin
export UPDATE_IPSETS_ADMIN_PASSWORD=your-strong-password
update-ipsets daemon --admin-auth-mode=required
```

With systemd, set them in the drop-in:

```ini
[Service]
Environment="UPDATE_IPSETS_ADMIN_USER=admin"
Environment="UPDATE_IPSETS_ADMIN_PASSWORD=your-strong-password"
```

Access the admin UI or API with HTTP Basic auth:

```bash
curl -u "$UPDATE_IPSETS_ADMIN_USER:$UPDATE_IPSETS_ADMIN_PASSWORD" http://127.0.0.1:18889/api/v1/admin/status
```

Browser access prompts for username and password when you visit `/admin`.

### Fail-closed behavior

If `UPDATE_IPSETS_ADMIN_USER` or `UPDATE_IPSETS_ADMIN_PASSWORD` is missing when auth is required:

- admin endpoints return 503 (Service Unavailable)
- admin access is denied — it does not fall back to open access

The daemon does not guess, default, or silently disable authentication. A missing configuration variable is a hard deny, not a soft skip.

## Disabled mode

Disabled mode skips authentication entirely. It is intended for local testing or trusted lab networks where you control all access.

To enable it, set **both** flags:

```bash
update-ipsets daemon \
  --admin-auth-mode=disabled \
  --allow-unauthenticated-admin
```

### Why two flags

The `--allow-unauthenticated-admin` flag is a deliberate safety acknowledgment. Without it, `--admin-auth-mode=disabled` alone is not accepted.

This prevents these common mistakes:

- Copy-pasting a command from documentation without noticing auth is off
- Automation scripts that set the mode from a variable and accidentally request disabled auth

The two-flag design means a disabled-auth deployment needs both the disabled mode and a separate risk acknowledgment.

## Bind address is not a safety signal

The daemon does not treat loopback or localhost bind addresses as "secure enough" to skip authentication.

Binding `--admin-listen 127.0.0.1:18889` restricts network reachability, but:

- other local processes and users can still reach the port
- reverse proxies may forward external traffic to localhost
- SSH tunneling can expose localhost ports remotely

Network-level isolation and application-level authentication are separate layers. Use both.

## Production recommendation

For production deployments:

1. Use `--admin-auth-mode=required` (the default).
2. Use split mode with `--admin-listen 127.0.0.1:18889`.
3. Set strong credentials in the systemd drop-in.
4. Restrict the admin port further with a firewall if possible.
5. Configure `runtime.public_base_url` for the public site; split mode will not start without it.

```bash
UPDATE_IPSETS_ADMIN_USER=admin \
UPDATE_IPSETS_ADMIN_PASSWORD=your-strong-password \
update-ipsets daemon \
  --config /opt/update-ipsets/etc/config \
  --listen :18888 \
  --admin-listen 127.0.0.1:18889 \
  --admin-auth-mode=required
```

This gives you:

- public site on `:18888` with no admin content
- admin UI on `localhost:18889` with authentication
- no admin routes accessible from the public listener (they return 404)

## Quick reference

| Goal | Flags |
|---|---|
| Local testing, no auth | `--admin-auth-mode=disabled --allow-unauthenticated-admin` |
| Production, shared listener | `--admin-auth-mode=required` + environment credentials |
| Production, split listener | `--admin-listen 127.0.0.1:18889 --admin-auth-mode=required` + environment credentials + `runtime.public_base_url` |

## See also

- [Daemon Reference](daemon-reference.md) — all flags
- [Listener Topologies](listener-topologies.md) — shared versus split mode
- [Environment Variables](environment-variables.md) — credential and systemd variables
