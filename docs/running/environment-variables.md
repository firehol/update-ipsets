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

The installed systemd unit sets the deployment paths under `/opt/update-ipsets`. You normally do not need to change them unless you want a non-standard layout.

| Variable | Shipped catalog fallback when unset | Installed unit value | Description |
|---|---|---|---|
| `BASE_DIR` | `${HOME}/ipsets` | `/opt/update-ipsets/data` | Root directory for committed ipset/netset output files. |
| `CONFIG_FILE` | `${HOME}/.update-ipsets/config` | not set; daemon uses `--config /opt/update-ipsets/etc/config` | Path to the legacy bash config file. |
| `RUN_PARENT_DIR` | `${HOME}/.update-ipsets` | `/opt/update-ipsets/run` | Parent directory for the process lock. |
| `CACHE_DIR` | `${HOME}/.update-ipsets/cache` | `/opt/update-ipsets/cache` | Scheduler/runtime cache directory. |
| `LIB_DIR` | `${HOME}/.update-ipsets/lib` | `/opt/update-ipsets/lib` | Persistent library and state directory. |
| `HISTORY_DIR` | `${BASE_DIR}/history` | `/opt/update-ipsets/data/history` | Feed history storage. |
| `ERRORS_DIR` | `${BASE_DIR}/errors` | `/opt/update-ipsets/data/errors` | Feed error log storage. |
| `TMP_DIR` | `/tmp` | `/opt/update-ipsets/tmp` | Temporary files directory. |
| `WEB_DIR` | empty, disabled | `/opt/update-ipsets/web` | Published web artifacts directory. |
| `WEB_DIR_FOR_IPSETS` | empty, disabled | `/opt/update-ipsets/web/files` | Directory served for raw ipset/netset file downloads. |

## Supplementary config directories

These variables point to directories containing additional feed YAML files. They are merged with the built-in catalog at startup.

| Variable | Shipped catalog fallback when unset | Description |
|---|---|---|
| `ADMIN_SUPPLIED_IPSETS` | `${FIREHOL_CONFIG_DIR}/ipsets.d` | Admin-managed feed config overlays. |
| `DISTRIBUTION_SUPPLIED_IPSETS` | `${FIREHOL_SHARE_DIR}/ipsets.d` | Distribution-packaged feed configs. |
| `USER_SUPPLIED_IPSETS` | `${HOME}/.update-ipsets/ipsets.d` | User-managed feed configs. |

## Web publishing variables

These are not path overrides but configure how published files are served.

| Variable | Default | Description |
|---|---|---|
| `WEB_OWNER` | (none) | Filesystem owner for published web files. |
| `WEB_URL` | `https://iplists.firehol.org/ipsets/` | Public website feed-detail URL prefix. |
| `PUBLIC_BASE_URL` | (none) | Externally visible base URL. |
| `LOCAL_COPY_URL` | `https://iplists.firehol.org/files/` | Base URL for raw file downloads. |

## API key variables

These are not path overrides. They hold API keys used in URL templates for feeds that require authentication.

| Variable | Used by | Description |
|---|---|---|
| `MAXMIND_LICENSE_KEY` | MaxMind GeoLite2 ASN and Country feeds | MaxMind license key for GeoLite2 downloads. |
| `IP2LOCATION_API_KEY` | IP2Proxy PX1LITE feed | API key for IP2Location downloads. |
| `BLUELIV_API_KEY` | Blueliv Crimeserver feed | API key for Blueliv downloads. |

Set these in `$HOME/.update-ipsets.env` to avoid exposing them in the systemd unit. The daemon reads this file at startup and sets any unset environment variables from it. In the installed unit, `HOME=/opt/update-ipsets`, so the installed service reads `/opt/update-ipsets/.update-ipsets.env`.

## Legacy config file

| Variable | Default | Description |
|---|---|---|
| `USER_AGENT` | `FireHOL-Update-Ipsets/3.0 (linux-gnu) https://iplists.firehol.org/` | HTTP User-Agent header for upstream downloads. |
| `UPDATE_IPSETS_LOCK_FILE` | `$RUN_PARENT_DIR/update-ipsets.lock` | Lock file path. `LOCK_FILE` is a legacy alias. |
| `GITHUB_CHANGES_URL` | `https://github.com/firehol/blocklist-ipsets/commits/master/` | GitHub changes URL template. |
| `GITHUB_SETINFO` | `https://github.com/firehol/blocklist-ipsets/tree/master/` | GitHub set info URL template. |

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
Environment="UPDATE_IPSETS_ADMIN_PASSWORD=change-this-secret"
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
