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

Those table values are the shipped YAML templates. When the daemon runs as a
non-root user and those path settings are unset or still equal to the built-in
defaults, runtime resolution moves the main state paths to user-owned locations:

| Runtime path | Effective non-root default |
|---|---|
| `base_dir` | `$HOME/.update-ipsets/ipsets` |
| `run_parent_dir` | `$HOME/.update-ipsets/run` |
| `cache_dir` | `$HOME/.cache/update-ipsets` |
| `lib_dir` | `$HOME/.local/share/update-ipsets` |

Explicit YAML values or environment-variable overrides take priority over this
non-root relocation.

## Supplementary config directories

These variables point to directories containing additional feed YAML files. They are merged with the built-in catalog at startup.

| Variable | Shipped catalog fallback when unset | Description |
|---|---|---|
| `ADMIN_SUPPLIED_IPSETS` | `${FIREHOL_CONFIG_DIR}/ipsets.d` | Admin-managed feed config overlays. |
| `DISTRIBUTION_SUPPLIED_IPSETS` | `${FIREHOL_SHARE_DIR}/ipsets.d` | Distribution-packaged feed configs. |
| `USER_SUPPLIED_IPSETS` | `${HOME}/.update-ipsets/ipsets.d` | User-managed feed configs. |

The `FIREHOL_CONFIG_DIR` and `FIREHOL_SHARE_DIR` names are legacy placeholders
used by the shipped templates. If your environment does not set them, set
`ADMIN_SUPPLIED_IPSETS` or `DISTRIBUTION_SUPPLIED_IPSETS` directly to the
directory you want the daemon to load.

## Web publishing variables

These are runtime YAML fields, not process environment overrides in the shipped
catalog. The shipped `runtime.yaml` sets the URL values directly. To change
them, edit the YAML value, or first change the YAML to reference an environment
template such as `${PUBLIC_BASE_URL-...}` and then set the matching environment
variable.

| Runtime setting | Shipped value | Description |
|---|---|---|
| `web_owner` | (none) | Filesystem owner for published web files. |
| `web_url` | `https://iplists.firehol.org/ipsets/` | Public website feed-detail URL prefix. |
| `public_base_url` | (none) | Externally visible base URL. |
| `local_copy_url` | `https://iplists.firehol.org/files/` | Base URL for raw file downloads. |

## API key variables

These are not path overrides. They hold API keys used in URL templates for feeds that require authentication.

| Variable | Used by | Description |
|---|---|---|
| `MAXMIND_LICENSE_KEY` | MaxMind GeoLite2 ASN and Country feeds | MaxMind license key for GeoLite2 downloads. |
| `IP2LOCATION_API_KEY` | IP2Proxy PX1LITE feed | API key for IP2Location downloads. |
| `BLUELIV_API_KEY` | Blueliv Crimeserver feed | API key for Blueliv downloads. |

Set these in `$HOME/.update-ipsets.env` to avoid exposing them in the systemd unit. The daemon reads this file at startup and sets any unset environment variables from it. In the installed unit, `HOME=/opt/update-ipsets`, so the installed service reads `/opt/update-ipsets/.update-ipsets.env`.

## Artifact credentials

Some artifact parents need credentials that are not part of the YAML catalog.

| Variable | Used by | Description |
|---|---|---|
| `DRONEBL_RSYNC_PASSWORD` | DroneBL `dronebl_buildzone` artifact parent | Preferred rsync password variable for the DroneBL buildzone fetch. |
| `RSYNC_PASSWORD` | DroneBL `dronebl_buildzone` artifact parent | Fallback rsync password variable accepted when `DRONEBL_RSYNC_PASSWORD` is not set. |

Store these in `$HOME/.update-ipsets.env` or in a protected systemd drop-in.
Do not put real secrets in catalog YAML.

## Outbound proxy variables

HTTP and HTTPS feed downloads use Go's standard proxy environment handling. Set these in the service environment when the host must reach upstream feeds through a forward proxy:

| Variable | Description |
|---|---|
| `HTTP_PROXY` / `http_proxy` | Proxy URL for HTTP feed downloads. |
| `HTTPS_PROXY` / `https_proxy` | Proxy URL for HTTPS feed downloads. |
| `NO_PROXY` / `no_proxy` | Comma-separated hosts, domains, or IP ranges that should bypass the proxy. |

Example systemd drop-in:

```ini
[Service]
Environment="HTTPS_PROXY=http://proxy.example:3128"
Environment="NO_PROXY=127.0.0.1,localhost,.example.internal"
```

## Legacy config-file assignment names

These names are parsed from legacy `.conf` files. They are not process
environment overrides for the shipped YAML catalog unless a YAML template
explicitly references them.

| Legacy assignment | Default | Description |
|---|---|---|
| `USER_AGENT` | `FireHOL-Update-Ipsets/3.0 (linux-gnu) https://iplists.firehol.org/` | HTTP User-Agent header for upstream downloads. |
| `UPDATE_IPSETS_LOCK_FILE` | `$RUN_PARENT_DIR/update-ipsets.lock` | Lock file path. `LOCK_FILE` is a legacy alias. |
| `GITHUB_CHANGES_URL` | `https://github.com/firehol/blocklist-ipsets/commits/master/` | GitHub changes URL template. |
| `GITHUB_SETINFO` | `https://github.com/firehol/blocklist-ipsets/tree/master/` | GitHub set info URL template. |

## systemd drop-in variables

The installed systemd unit supports runtime configuration through environment variables. This lets you change listen addresses and auth settings without editing the `ExecStart=` line.

The generated unit expands the `*_ARG` variables with systemd `${VAR}`
substitution, which preserves whitespace as one argument. Use `--flag=value`
for variables that carry both a flag and its value.

| Variable | Default | Description |
|---|---|---|
| `UPDATE_IPSETS_LISTEN` | `127.0.0.1:18888` | Public listener address:port. |
| `UPDATE_IPSETS_ADMIN_LISTEN_ARG` | `--admin-listen=127.0.0.1:18889`, or `--admin-listen=<tailscale-ip>:18889` when Tailscale is detected during install | Full `--admin-listen` flag with value. Empty means shared mode. |
| `UPDATE_IPSETS_ADMIN_AUTH_ARG` | `--admin-auth-mode=disabled` | Full `--admin-auth-mode` flag with value. Use `--admin-auth-mode=required` for authenticated admin. |
| `UPDATE_IPSETS_ALLOW_UNAUTHENTICATED_ADMIN_ARG` | `--allow-unauthenticated-admin` | Acknowledges unauthenticated admin mode. Set empty when auth is required. |

The Tailscale value is written when `install.sh` generates the unit. It is not a
dynamic lookup on every service start.

Example drop-in at `/etc/systemd/system/update-ipsets.service.d/override.conf`:

```ini
[Service]
Environment="UPDATE_IPSETS_LISTEN=:18888"
Environment="UPDATE_IPSETS_ADMIN_LISTEN_ARG=--admin-listen=127.0.0.1:18889"
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

## systemd notification variables

systemd sets these automatically when the service uses `Type=notify` and `WatchdogSec=`. Operators normally should not set them in drop-ins or shell environments.

| Variable | Set by | Description |
|---|---|---|
| `NOTIFY_SOCKET` | systemd | Socket used for readiness and watchdog notifications. |
| `WATCHDOG_USEC` | systemd | Watchdog interval in microseconds. The daemon sends watchdog heartbeats at half this interval. |

## OpenTelemetry

The daemon can export traces, metrics, and logs through OTLP. See the [Monitoring](../monitoring/monitoring-overview.md) section for the full setup guide.

The admin surface also serves `GET /metrics` for Prometheus scraping. The OTLP
environment variables below control push export; they do not remove the admin
Prometheus scrape endpoint.

| Variable | Default | Description |
|---|---|---|
| `UPDATE_IPSETS_OTEL` | (empty) | Set to `1`, `true`, or `enabled` to enable export. Set to `0`, `false`, or `disabled` to force-disable even when endpoint variables are present. |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | (none) | OTLP collector endpoint. With the default `http/protobuf` protocol, use an OTLP/HTTP endpoint such as `http://127.0.0.1:4318`. For plaintext gRPC, set `UPDATE_IPSETS_OTEL_PROTOCOL=grpc` and include the scheme, for example `http://127.0.0.1:4317`. |
| `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` | (none) | Signal-specific OTLP traces endpoint. Setting it also enables export unless `UPDATE_IPSETS_OTEL` disables export. |
| `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT` | (none) | Signal-specific OTLP metrics endpoint. Setting it also enables export unless `UPDATE_IPSETS_OTEL` disables export. |
| `OTEL_EXPORTER_OTLP_LOGS_ENDPOINT` | (none) | Signal-specific OTLP logs endpoint. Setting it also enables export unless `UPDATE_IPSETS_OTEL` disables export. |
| `UPDATE_IPSETS_OTEL_PROTOCOL` | `http/protobuf` | Export protocol: `http/protobuf` or `grpc`. Falls back to `OTEL_EXPORTER_OTLP_PROTOCOL` if not set. |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | (none) | Standard OTLP protocol variable. Used when `UPDATE_IPSETS_OTEL_PROTOCOL` is unset. |
| `OTEL_METRIC_EXPORT_INTERVAL` | (none) | Metric export interval. Accepts integer milliseconds such as `10000` or duration strings such as `10s`. |
| `UPDATE_IPSETS_OTEL_METRIC_INTERVAL` | (none) | Same as `OTEL_METRIC_EXPORT_INTERVAL`. Takes priority if both are set. |
| `UPDATE_IPSETS_OTEL_TRACES` | (unset) | Set to `0`, `false`, `disabled`, `off`, or `none` to suppress trace export. |
| `UPDATE_IPSETS_OTEL_METRICS` | (unset) | Set to `0`, `false`, `disabled`, `off`, or `none` to suppress OTLP metric export. |
| `UPDATE_IPSETS_OTEL_LOGS` | (unset) | Set to `0`, `false`, `disabled`, `off`, or `none` to suppress log export. |
| `OTEL_TRACES_EXPORTER` | (unset) | Set to `none` to disable traces. Standard OpenTelemetry variable. |
| `OTEL_METRICS_EXPORTER` | (unset) | Set to `none` to disable OTLP metric export. Standard OpenTelemetry variable. |
| `OTEL_LOGS_EXPORTER` | (unset) | Set to `none` to disable logs. Standard OpenTelemetry variable. |

The daemon also uses the standard OpenTelemetry SDK resource detector and OTLP
exporters. These variables are read by the OpenTelemetry SDK when export is
enabled. Signal-specific variants use `TRACES`, `METRICS`, or `LOGS` in place
of `<SIGNAL>` and take priority for that signal.

Metric export uses stable default resource identity. Automatic host, OS,
process resource attributes and the daemon build version are not attached to
metrics by default, so host/kernel churn, process IDs, and local dirty-build
churn do not create new metric series. Operators can still add explicit
resource attributes through the standard OpenTelemetry environment variables
when that trade-off is intentional.

| Variable | Default | Description |
|---|---|---|
| `OTEL_SERVICE_NAME` | SDK default | Service name resource attribute. |
| `OTEL_RESOURCE_ATTRIBUTES` | (none) | Comma-separated resource attributes, such as `deployment.environment=prod`. |
| `OTEL_EXPORTER_OTLP_HEADERS` / `OTEL_EXPORTER_OTLP_<SIGNAL>_HEADERS` | (none) | Key-value headers or gRPC metadata sent with OTLP exports. |
| `OTEL_EXPORTER_OTLP_TIMEOUT` / `OTEL_EXPORTER_OTLP_<SIGNAL>_TIMEOUT` | `10000` | Export timeout in milliseconds. |
| `OTEL_EXPORTER_OTLP_COMPRESSION` / `OTEL_EXPORTER_OTLP_<SIGNAL>_COMPRESSION` | (none) | OTLP payload compression. `gzip` is supported. |
| `OTEL_EXPORTER_OTLP_INSECURE` / `OTEL_EXPORTER_OTLP_<SIGNAL>_INSECURE` | `false` | Disables transport security for exporter endpoint forms that support it. Prefer explicit `http://` or `https://` endpoints. |
| `OTEL_EXPORTER_OTLP_CERTIFICATE` / `OTEL_EXPORTER_OTLP_<SIGNAL>_CERTIFICATE` | (none) | Trusted server certificate path for TLS verification. |
| `OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE` / `OTEL_EXPORTER_OTLP_<SIGNAL>_CLIENT_CERTIFICATE` | (none) | Client certificate path for mTLS. |
| `OTEL_EXPORTER_OTLP_CLIENT_KEY` / `OTEL_EXPORTER_OTLP_<SIGNAL>_CLIENT_KEY` | (none) | Client private key path for mTLS. |

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
