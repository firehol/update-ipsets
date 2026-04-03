# update-ipsets (Go rewrite)

This workspace contains a working Go replacement for the FireHOL `update-ipsets` pipeline.

## Subcommands

- **`update-ipsets iprange`** — standalone iprange-compatible mode
  - IPv4-compatible `iprange` rewrite
  - compare / compare-first / compare-next
  - diff / exclude / intersect / combine
  - CIDR / range / single-IP / binary I/O
  - hostname resolution
  - prefix reduction (--ipset-reduce / --reduce-factor)
  - count-unique / count-unique-all
  - `@filelist` and `@directory` input expansion
- **`update-ipsets query`** — query which lists contain an IP
  - `update-ipsets query <ip>` — reports which generated lists contain the IPv4 address
  - `update-ipsets query --set "set1 + set2 - set3"` — compose sets and dump the result
  - `update-ipsets query --set "..." <ip>` — test whether an IP is in the composed set
  - flags: `--config`, `--set`, `--ip`, `--format` (cidr|range|single), `--silent`, `--verbose`
- **`update-ipsets enable`** — enable or disable sources
  - creates/removes enable markers for one or more sets, or all known sets
  - flags: `--config`, `--all`, `--disable`, `--silent`, `--verbose`
- **`update-ipsets daemon`** — scheduler + web server + API + admin
  - internal scheduler with configurable `--interval` (default 1m)
  - systemd notify/watchdog support
  - native netlink ipset apply when running as root on Linux
  - HTTPS listener via `--tls-cert` / `--tls-key`
  - optional separate admin listener via `--admin-listen`
  - explicit admin auth mode via `--admin-auth-mode=required|disabled`
  - unsafe unauthenticated admin requires `--allow-unauthenticated-admin`
  - admin credentials in `required` mode come from `UPDATE_IPSETS_ADMIN_USER` / `UPDATE_IPSETS_ADMIN_PASSWORD`
  - flags: `--config`, `--listen` (default `:8080`), `--admin-listen`, `--admin-auth-mode`, `--allow-unauthenticated-admin`, `--interval`, `--enable-all`, `--push-git`, `--tls-cert`, `--tls-key`, `--web-dir`, `--web-files-dir`, `--silent`, `--verbose`
  - SIGHUP reloads configuration without restart
- **`update-ipsets version`** — print version string

### Geolocation and comparison pipeline

- GeoLite2, IPDeny, IP2Location, IPIP, and DB-IP parsers
- per-country comparison JSON generation
- pairwise comparison output for regular sets

## REST API (daemon mode)

The daemon always exposes the public endpoints below on the `--listen` address.
Admin endpoints are exposed:

- on `--listen` when `--admin-listen` is not set
- on `--admin-listen` only when split mode is enabled

### Public endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/healthz` | Health check, returns `ok` |
| GET | `/robots.txt` | Public crawler policy with the sitemap pointer |
| GET | `/sitemap.xml` | Public XML sitemap for indexable pages |
| GET | `/llms.txt` | Public Markdown map for AI agents |
| GET | `/api/v1/status` | Engine status, scheduler state, and system info |
| GET | `/api/v1/categories` | Public category metadata |
| GET | `/api/v1/home/summary` | Homepage aggregate payload |
| GET | `/api/v1/home/globe` | Homepage globe payload |
| GET | `/api/v1/sets` | List all sets with metadata |
| GET | `/api/v1/sets/{name}` | Detail for a single set |
| GET | `/api/v1/sets/{name}/data` | Raw IP list (text/plain) |
| GET | `/api/v1/sets/{name}/history` | History CSV (DateTime,Entries,UniqueIPs) |
| GET | `/api/v1/sets/{name}/compare` | Pairwise overlap comparison JSON |
| GET | `/api/v1/sets/{name}/retention` | Retention analysis JSON |
| GET | `/api/v1/sets/{name}/insights` | Deterministic insight payload |
| GET | `/api/v1/sets/{name}/countries/{provider}` | Geolocation breakdown JSON |
| GET | `/api/v1/sets/{name}/asn` | Available ASN providers for one feed |
| GET | `/api/v1/sets/{name}/asn/{provider}` | ASN attribution detail for one feed/provider |
| GET | `/api/v1/sets/{name}/bogons` | Available bogon providers for one feed |
| GET | `/api/v1/sets/{name}/bogons/{provider}` | Bogon detail for one feed/provider |
| GET | `/api/v1/sets/{name}/infrastructure` | Critical-infrastructure overlap summary for one feed |
| GET | `/api/v1/sets/{name}/infrastructure/providers` | Configured critical-infrastructure reference providers |
| GET | `/api/v1/sets/{name}/infrastructure/{provider}` | Critical-infrastructure overlap for one feed/provider |
| GET | `/api/v1/search?ip=...` | Which sets contain the IP (add `&details=true` for full match info) |
| GET | `/api/v1/countries/{code}` | Country detail surface payload |
| GET | `/api/v1/asns/{asn}` | ASN detail surface payload |
| GET | `/api/v1/maintainers` | Maintainer index payload |
| GET | `/api/v1/maintainers/{slug}` | Maintainer detail payload |
| GET | `/api/v1/methodology` | Methodology index payload |
| GET | `/api/v1/methodology/{slug}` | Methodology page payload |
| GET | `/api/v1/compose?include=...&exclude=...&format=...` | Compose sets on the fly (text/plain) |

`/api/v1/ipsets` and `/api/v1/ipsets/...` are aliases for `/api/v1/sets` and `/api/v1/sets/...`.
`/api/v1/query` is a legacy alias for `/api/v1/search`.

### Admin endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/admin` | Admin dashboard (single-page app) |
| GET | `/api/v1/admin/status` | Detailed engine + scheduler status |
| GET | `/api/v1/admin/feeds` | All feeds with per-feed status |
| GET | `/api/v1/admin/feeds/{name}` | Single feed detail |
| GET | `/api/v1/admin/feeds/{name}/manifest` | File-manifest detail for one feed |
| POST | `/api/v1/admin/feeds/{name}/recheck` | Recheck a feed (ignore schedule) |
| POST | `/api/v1/admin/feeds/{name}/reprocess` | Reprocess a feed (rebuild even if unchanged) |
| POST | `/api/v1/admin/feeds/{name}/enable` | Enable a feed |
| POST | `/api/v1/admin/feeds/{name}/disable` | Disable a feed |
| GET | `/api/v1/admin/artifacts` | Artifact-parent inventory |
| POST | `/api/v1/admin/artifacts/{name}/recheck` | Recheck an artifact parent |
| POST | `/api/v1/admin/artifacts/{name}/enable` | Enable an artifact parent |
| POST | `/api/v1/admin/artifacts/{name}/disable` | Disable an artifact parent |
| GET | `/api/v1/admin/schedule` | Scheduler state |
| GET | `/api/v1/admin/integrity` | Settled integrity findings |
| POST | `/api/v1/admin/integrity/reprocess` | Queue integrity recovery |
| POST | `/api/v1/admin/run` | Trigger due work now or broad reprocess, depending on query flags |

Auth behavior:

- `--admin-auth-mode=required` is the safe/default mode
- in `required` mode, missing admin credentials fail closed
- `--admin-auth-mode=disabled` is unsafe and only works together with `--allow-unauthenticated-admin`
- when `--admin-listen` is set, `/admin` and `/api/v1/admin/*` are removed from the public listener and return `404` there

## Config

- The repository includes the source catalog as a directory at [configs/firehol](configs/firehol).
- Runtime also supports loading the legacy FireHOL script directly through `pkg/config`.
- Default config resolution prefers the local directory catalog when present.
- `./install.sh` deploys `configs/firehol/` to `/opt/update-ipsets/etc/config/` as the active installed catalog. Reinstalls refresh that active directory from the repo and create a timestamped backup when the previous installed directory differs.
- `runtime.public_base_url` is the externally visible base URL of the public website, used by admin-to-public links.
- `runtime.web_url` remains the published feed-detail prefix used in generated metadata/output files.

Example runtime settings for split admin:

```yaml
runtime:
  public_base_url: "https://iplists.firehol.org"
  web_url: "https://iplists.firehol.org/ipsets/"
```

Example local development command:

```bash
update-ipsets daemon \
  --config configs/firehol \
  --listen :18888 \
  --admin-auth-mode=disabled \
  --allow-unauthenticated-admin
```

Example production command with a separate admin port:

```bash
UPDATE_IPSETS_ADMIN_USER=admin \
UPDATE_IPSETS_ADMIN_PASSWORD=secret \
update-ipsets daemon \
  --config /opt/update-ipsets/etc/config \
  --listen :18888 \
  --admin-listen 127.0.0.1:18889 \
  --admin-auth-mode=required
```

### OpenTelemetry

The daemon can export traces, metrics, and logs through OTLP/HTTP or OTLP/gRPC.

Enable export by setting either:

- `UPDATE_IPSETS_OTEL=1`
- `OTEL_EXPORTER_OTLP_ENDPOINT` or a signal-specific endpoint such as
  `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`

Set `UPDATE_IPSETS_OTEL=0` to force-disable export even when OTLP endpoint
variables are present. Local logs are still written normally; when enabled, the
same structured log records are also sent through OpenTelemetry.

Use `UPDATE_IPSETS_OTEL_PROTOCOL=grpc` or `OTEL_EXPORTER_OTLP_PROTOCOL=grpc`
for OTLP/gRPC. The default protocol remains `http/protobuf`. The metric export
interval can be set with `UPDATE_IPSETS_OTEL_METRIC_INTERVAL` or
`OTEL_METRIC_EXPORT_INTERVAL`; integer values are milliseconds, so `1000`
means one second. For OTLP/gRPC, endpoint environment values must include an
`http` or `https` scheme.

The installed systemd unit defaults to pushing metrics and logs to the local
Netdata `otel-plugin` every 10 seconds, matching Netdata's default OTel chart
interval:

```ini
[Service]
Environment="UPDATE_IPSETS_OTEL=1"
Environment="UPDATE_IPSETS_OTEL_PROTOCOL=grpc"
Environment="OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4317"
Environment="OTEL_METRIC_EXPORT_INTERVAL=10000"
Environment="OTEL_TRACES_EXPORTER=none"
```

The installed systemd unit is also configurable via a drop-in without replacing
`ExecStart=`. Example:

```ini
[Service]
Environment="UPDATE_IPSETS_LISTEN=:18888"
Environment="UPDATE_IPSETS_ADMIN_LISTEN_ARG=--admin-listen 127.0.0.1:18889"
Environment="UPDATE_IPSETS_ADMIN_AUTH_ARG=--admin-auth-mode=required"
Environment="UPDATE_IPSETS_ALLOW_UNAUTHENTICATED_ADMIN_ARG="
Environment="UPDATE_IPSETS_ADMIN_USER=admin"
Environment="UPDATE_IPSETS_ADMIN_PASSWORD=secret"
```

## Build

```bash
make build        # build binary
make test         # run tests
make lint         # go vet
make race         # run tests with race detector
make bench        # run benchmarks
```

## Verify

```bash
go test ./...
go vet ./...
go test -race ./...
go test -bench=. ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/update-ipsets-linux-amd64 ./cmd/update-ipsets
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /tmp/update-ipsets-linux-arm64 ./cmd/update-ipsets
```

## Out-of-core memory behavior

The daemon is designed to handle large IP sets without running out of memory. Instead of loading all data into the Go heap, it uses file-backed storage and bounded-memory algorithms.

### How it works

- **File-backed binary sets (FileSet):** The canonical storage format is a fixed-width binary set snapshot written to `lib/{name}/latest`. Legacy `latest.set` files are accepted as compatibility input only. Read operations (query, compare, compose, geolocation) open these files with `mmap` (Linux/macOS) or `pread` (fallback) and perform binary search directly on the file data. The Go heap never holds the range array.
- **Iterator-based set operations:** Union, intersection, exclusion, diff, and overlap counting all use streaming two-pointer sweeps over `RangeSource` iterators. Memory usage is O(1) regardless of input size.
- **Streaming processor pipeline:** The `RunStream()` function chains processor steps as nested `io.Reader` pipelines. Data flows line-by-line through the chain and is written to a temp file at the end. Only non-streamable processors (json_path, xml_tag, unzip) fall back to in-memory processing.
- **Download spill-to-disk:** HTTP responses are streamed to temp files via `io.Copy` instead of `io.ReadAll`. Same-body detection uses file hashes instead of `bytes.Equal`. A configurable `max_download_size` (default 100MB) aborts oversized downloads.
- **Streaming geolocation parsing:** Geolocation archives (tar.gz, zip, gzip-CSV) are parsed by streaming through the archive entries. CSV files use record-by-record `csv.Reader.Read()` instead of `ReadAll()`. Zip archives use `zip.OpenReader(path)` backed by the OS file handle.

### Recommended deployment settings

Set `GOMEMLIMIT` to communicate the intended memory budget to the Go runtime. This drives more aggressive GC and memory return without being a hard kill limit:

```bash
# Example: 512MB soft limit
GOMEMLIMIT=512MiB update-ipsets daemon
```

For systemd-managed deployments, use cgroup memory controls:

```ini
[Service]
# Soft limit: kernel starts reclaiming and throttling at this threshold.
# The daemon slows down but keeps running.
MemoryHigh=512M

# Hard limit: kernel OOM-kills if exceeded. Set above MemoryHigh to
# give headroom for transient spikes.
MemoryMax=768M

# Tell Go about the memory budget so GC cooperates.
Environment="GOMEMLIMIT=512MiB"
```

The combination of `MemoryHigh` (kernel throttling) and `GOMEMLIMIT` (Go GC pressure) produces "degrade under pressure" behavior: the daemon gets slower instead of dying.

### Performance characteristics

FileSet (file-backed) vs in-memory IPSet benchmarks on 100K ranges:

| Operation | FileSet | In-memory |
|-----------|---------|-----------|
| Contains (binary search) | ~171ns, 0 allocs | ~79ns, 0 allocs |
| Iter (full scan) | ~366us, 3 allocs | ~11us, 0 allocs |
| OverlapCountIter (100K x 100K) | ~16.5ms, 14 allocs | ~14.7ms, 14 allocs |

FileSet is slower for random access (pread/mmap vs direct memory) but uses no heap. Iterator operations are comparable because they are I/O-sequential. For catalog-wide comparisons the bounded memory is the dominant benefit.

## Current gaps

- IPv4 is fully implemented and verified. IPv6 still has only an explicit stub/interface surface.
- `--push-git` commits the base output repository. If `web_dir` is a separate repository, enable `runtime.push_to_git_web` for that tree as well.
