# Environment Variables

You will learn which environment variables the daemon reads, what each one controls, and how to set them in a systemd drop-in.

## Admin credentials

These variables set the admin username and password when `--admin-auth-mode=required` is active.

| Variable | Default | Description |
|---|---|---|
| `UPDATE_IPSETS_ADMIN_USER` | (none) | Admin username for HTTP Basic auth. |
| `UPDATE_IPSETS_ADMIN_PASSWORD` | (none) | Admin password for HTTP Basic auth. |

If either is missing when auth is required, admin access fails closed. The daemon does not fall back to open access.

## Path overrides

These environment variables override filesystem paths. They are expanded from `configs/firehol/runtime.yaml` at startup.

The installed systemd unit sets all of these. You normally do not need to change them unless you want a non-standard layout.

| Variable | Default (root) | Default (non-root) | Description |
|---|---|---|---|
| `BASE_DIR` | `/etc/firehol/ipsets` | `$HOME/.update-ipsets/ipsets` | Root directory for committed ipset/netset output files. |
| `CONFIG_FILE` | `/etc/firehol/update-ipsets` | `$HOME/.update-ipsets/config` | Path to the legacy bash config file. |
| `RUN_PARENT_DIR` | `/var/run` | `$HOME/.update-ipsets/run` | Parent directory for lock and socket files. |
| `CACHE_DIR` | `/var/cache/update-ipsets` | `$HOME/.cache/update-ipsets` | Download cache directory. |
| `LIB_DIR` | `/var/lib/update-ipsets` | `$HOME/.local/share/update-ipsets` | Persistent library and state directory. |
| `HISTORY_DIR` | `$BASE_DIR/history` | `$BASE_DIR/history` | Feed history storage. |
| `ERRORS_DIR` | `$BASE_DIR/errors` | `$BASE_DIR/errors` | Feed error log storage. |
| `TMP_DIR` | `/tmp` | `/tmp` | Temporary files directory. |
| `WEB_DIR` | (empty, disabled) | (empty, disabled) | Published web artifacts directory. Set to enable the public website. |
| `WEB_DIR_FOR_IPSETS` | (empty, disabled) | (empty, disabled) | Directory served for raw ipset/netset file downloads. |

## Supplementary config directories

These variables point to directories containing additional feed YAML files. They are merged with the built-in catalog at startup.

| Variable | Default (root) | Description |
|---|---|---|
| `ADMIN_SUPPLIED_IPSETS` | `/etc/firehol/ipsets.d` | Admin-managed feed config overlays. |
| `DISTRIBUTION_SUPPLIED_IPSETS` | `/usr/share/firehol/ipsets.d` | Distribution-packaged feed configs. |
| `USER_SUPPLIED_IPSETS` | `$HOME/.update-ipsets/ipsets.d` | User-managed feed configs. |

## Web publishing variables

These are not path overrides but configure how published files are served.

| Variable | Default | Description |
|---|---|---|
| `WEB_OWNER` | (none) | Filesystem owner for published web files. |
| `WEB_URL` | (none) | Public website URL prefix. |
| `PUBLIC_BASE_URL` | (none) | Externally visible base URL. |
| `LOCAL_COPY_URL` | (none) | Base URL for raw file downloads. |

## API key variables

These are not path overrides. They hold API keys used in URL templates for feeds that require authentication.

| Variable | Used by | Description |
|---|---|---|
| `MAXMIND_LICENSE_KEY` | MaxMind GeoLite2 ASN and Country feeds | MaxMind license key for GeoLite2 downloads. |
| `IP2LOCATION_API_KEY` | IP2Proxy PX1LITE feed | API key for IP2Location downloads. |
| `BLUELIV_API_KEY` | Blueliv Crimeserver feed | API key for Blueliv downloads. |

Set these in `~/.update-ipsets.env` to avoid exposing them in the systemd unit. The daemon reads this file at startup and sets any unset environment variables from it.

## Legacy config file

| Variable | Default | Description |
|---|---|---|
| `USER_AGENT` | `update-ipsets/...` | HTTP User-Agent header for upstream downloads. |
| `UPDATE_IPSETS_LOCK_FILE` | `$RUN_PARENT_DIR/update-ipsets.lock` | Lock file path. `LOCK_FILE` is a legacy alias. |
| `GITHUB_CHANGES_URL` | (none) | GitHub changes URL template. |
| `GITHUB_SETINFO` | (none) | GitHub set info URL template. |

## systemd drop-in variables

The installed systemd unit supports runtime configuration through environment variables. This lets you change listen addresses and auth settings without editing the `ExecStart=` line.

| Variable | Default | Description |
|---|---|---|
| `UPDATE_IPSETS_LISTEN` | `:18888` | Public listener address:port. |
| `UPDATE_IPSETS_ADMIN_LISTEN_ARG` | (empty) | Full `--admin-listen` flag with value, e.g. `--admin-listen 127.0.0.1:18889`. Empty means shared mode. |
| `UPDATE_IPSETS_ADMIN_AUTH_ARG` | `--admin-auth-mode=required` | Full `--admin-auth-mode` flag with value. |
| `UPDATE_IPSETS_ALLOW_UNAUTHENTICATED_ADMIN_ARG` | (empty) | Set to `--allow-unauthenticated-admin` to acknowledge unauthenticated admin. Empty means the flag is not passed. |

Example drop-in at `/etc/systemd/system/update-ipsets.service.d/override.conf`:

```ini
[Service]
Environment="UPDATE_IPSETS_LISTEN=:18888"
Environment="UPDATE_IPSETS_ADMIN_LISTEN_ARG=--admin-listen 127.0.0.1:18889"
Environment="UPDATE_IPSETS_ADMIN_AUTH_ARG=--admin-auth-mode=required"
Environment="UPDATE_IPSETS_ALLOW_UNAUTHENTICATED_ADMIN_ARG="
Environment="UPDATE_IPSETS_ADMIN_USER=admin"
Environment="UPDATE_IPSETS_ADMIN_PASSWORD=secret"
```

After editing, reload and restart:

```bash
systemctl daemon-reload
systemctl restart update-ipsets
```

## OpenTelemetry

The daemon can export traces, metrics, and logs through OTLP. See the [Monitoring](../monitoring/monitoring-overview.md) section for the full setup guide.

| Variable | Default | Description |
|---|---|---|
| `UPDATE_IPSETS_OTEL` | (empty) | Set to `1`, `true`, or `enabled` to enable export. Set to `0`, `false`, or `disabled` to force-disable even when endpoint variables are present. |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | (none) | OTLP collector endpoint. For gRPC, include the scheme: `http://127.0.0.1:4317`. |
| `UPDATE_IPSETS_OTEL_PROTOCOL` | `http/protobuf` | Export protocol: `http/protobuf` or `grpc`. Falls back to `OTEL_EXPORTER_OTLP_PROTOCOL` if not set. |
| `OTEL_METRIC_EXPORT_INTERVAL` | (none) | Metric export interval in milliseconds. `10000` means 10 seconds. |
| `UPDATE_IPSETS_OTEL_METRIC_INTERVAL` | (none) | Same as `OTEL_METRIC_EXPORT_INTERVAL`. Takes priority if both are set. |
| `UPDATE_IPSETS_OTEL_TRACES` | (unset) | Set to `0` or `false` to suppress trace export. |
| `UPDATE_IPSETS_OTEL_METRICS` | (unset) | Set to `0` or `false` to suppress metric export. |
| `UPDATE_IPSETS_OTEL_LOGS` | (unset) | Set to `0` or `false` to suppress log export. |
| `OTEL_TRACES_EXPORTER` | (unset) | Set to `none` to disable traces. Standard OpenTelemetry variable. |

The installed systemd unit defaults to local Netdata export:

```ini
[Service]
Environment="UPDATE_IPSETS_OTEL=1"
Environment="UPDATE_IPSETS_OTEL_PROTOCOL=grpc"
Environment="OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4317"
Environment="OTEL_METRIC_EXPORT_INTERVAL=10000"
Environment="OTEL_TRACES_EXPORTER=none"
```

## Go runtime

| Variable | Default | Description |
|---|---|---|
| `GOMEMLIMIT` | (none) | Soft memory target for the Go runtime GC. Not a hard kill limit. Drives more aggressive garbage collection and memory return. Example: `512MiB`. |

Combine `GOMEMLIMIT` with systemd `MemoryHigh` for "degrade under pressure" behavior — the daemon gets slower instead of crashing:

```ini
[Service]
MemoryHigh=512M
MemoryMax=768M
Environment="GOMEMLIMIT=512MiB"
```

## See also

- [Daemon Reference](daemon-reference.md) — all flags and subcommands
- [Admin Authentication](admin-authentication.md) — auth setup details
- [Monitoring](../monitoring/monitoring-overview.md) — OpenTelemetry integration
- [Filesystem Layout](../installation/filesystem-layout.md) — what goes where on disk
