# Daemon Reference

You will learn how to start the update-ipsets daemon, what each flag does, and how to choose the right options for local development versus production.

## Starting the daemon

The daemon is the main operating mode. It runs a scheduler that downloads feeds, processes them, and serves both a public website and an admin UI.

```bash
update-ipsets daemon [flags]
```

## Flags

### `--config` (path)

Path to the YAML catalog directory.

The directory holds source definitions, merges, artifact parents, runtime settings, and supporting registries. The installed catalog lives at `/opt/update-ipsets/etc/config/` by default.

```bash
--config /opt/update-ipsets/etc/config
```

### `--listen` (address:port)

Address and port for the public listener. Default: `:8080`.

All public endpoints and, in shared mode, admin endpoints are served here.

```bash
--listen :18888
--listen 0.0.0.0:80
```

### `--admin-listen` (address:port)

Optional separate address and port for admin endpoints only. When set, admin routes are removed from the public listener and return 404 there.

See [Listener Topologies](listener-topologies.md) for details.

```bash
--admin-listen 127.0.0.1:18889
```

### `--admin-auth-mode` (required|disabled)

Controls admin authentication. Default: `required`.

- `required` — admin endpoints require HTTP Basic authentication using credentials from environment variables. If credentials are missing, admin access fails closed (no access, not open access).
- `disabled` — admin endpoints skip authentication. Requires `--allow-unauthenticated-admin` as a safety acknowledgment.

See [Admin Authentication](admin-authentication.md) for details.

```bash
--admin-auth-mode=required
--admin-auth-mode=disabled --allow-unauthenticated-admin
```

### `--allow-unauthenticated-admin`

Explicit acknowledgment that you accept the risk of running without admin authentication. This flag has no effect unless `--admin-auth-mode=disabled` is also set.

The two-flag design prevents accidental exposure. A single typo in a flag name does not open the admin surface.

### `--interval` (duration)

Scheduler check interval. Default: `1m`.

The daemon checks for due work (downloads, processing, maintenance) at this cadence. Shorter intervals give faster turnaround. Longer intervals reduce idle CPU.

```bash
--interval 30s
--interval 5m
```

### `--enable-all`

Enable all known feeds at startup. Without this flag, only feeds that have been explicitly enabled (via the admin UI or the `enable` subcommand) are active.

Use this for initial deployment when you want everything running from the start.

### `--push-git`

Commit output changes to git after each processing cycle.

Use this when the output directory is a git repository and you want automatic versioned commits of published feed data.

### `--tls-cert` and `--tls-key` (paths)

Enable HTTPS on all listeners. Provide the certificate and key file paths.

```bash
--tls-cert /etc/ssl/certs/update-ipsets.pem \
--tls-key /etc/ssl/private/update-ipsets.key
```

When TLS is enabled, HTTP requests to the same port are rejected.

### `--web-dir` (path)

Directory containing published artifacts (feed data, JSON metadata, history CSVs, comparison files). The daemon reads precomputed files from here for public serving.

If not set, the daemon uses the default output directory derived from the installation layout.

### `--web-files-dir` (path)

Directory containing downloadable raw `.ipset` and `.netset` files served from `/files/` and `/api/v1/sets/{name}/data`.

This is not a static asset directory. The public and admin SPA assets are embedded in the binary.

### `--trust-proxy-headers`

Trust `X-Forwarded-For` and `X-Real-IP` when determining the client IP address for logging, search context, and rate limiting.

Use this only when every request reaches the daemon through a trusted reverse proxy. If clients can reach the daemon directly, they can forge these headers.

### `--trust-cloudflare-headers`

Trust `CF-Connecting-IP` when determining the client IP address.

Use this only when traffic reaches the daemon exclusively through Cloudflare or a trusted proxy that strips untrusted `CF-Connecting-IP` headers.

When both proxy modes are enabled, Cloudflare's header has priority, then `X-Forwarded-For`, then `X-Real-IP`, then the TCP peer address.

### `--silent` and `--verbose`

Log level control.

- `--silent` — suppress most output, log only errors.
- `--verbose` — increase log detail, useful for troubleshooting.

Default: normal log level (info-level structured output).

## Subcommands

The `daemon` subcommand is the main operating mode. Other subcommands handle specific tasks:

| Subcommand | Purpose |
|---|---|
| `iprange` | Standalone iprange-compatible mode. Compare, diff, intersect, combine IP sets. Supports CIDR, range, single-IP, and binary I/O. |
| `query` | Look up which lists contain an IP, or compose sets and test membership. |
| `enable` | Enable or disable source feeds. Use `--all` to enable everything, `--disable` to remove enable markers. |
| `cache-merge` | Migration helper that merges legacy bash cache state with local Go cache state. |
| `version` | Print the version string and exit. |

```bash
update-ipsets query 1.2.3.4
update-ipsets query --set "firehol_level1 + firehol_level2 - firehol_webserver" 1.2.3.4
update-ipsets enable --all
update-ipsets enable --disable firehol_level1
update-ipsets iprange --compare file1.ipset file2.ipset
```

## Example: local development

Start the daemon locally with no authentication and a short interval:

```bash
update-ipsets daemon \
  --config configs/firehol \
  --listen :18888 \
  --admin-auth-mode=disabled \
  --allow-unauthenticated-admin \
  --enable-all \
  --interval 30s \
  --verbose
```

Public site: `http://localhost:18888`
Admin UI: `http://localhost:18888/admin`

## Example: production with split listener

Run the public site on a public port and the admin UI on localhost only, with authentication:

```bash
UPDATE_IPSETS_ADMIN_USER=admin \
UPDATE_IPSETS_ADMIN_PASSWORD=change-this-secret \
update-ipsets daemon \
  --config /opt/update-ipsets/etc/config \
  --listen :18888 \
  --admin-listen 127.0.0.1:18889 \
  --admin-auth-mode=required \
  --interval 1m
```

Public site: `http://your-server:18888`
Admin UI: `http://127.0.0.1:18889/admin`

## See also

- [Environment Variables](environment-variables.md)
- [Configuration Reload](configuration-reload.md)
- [Listener Topologies](listener-topologies.md)
- [Admin Authentication](admin-authentication.md)
