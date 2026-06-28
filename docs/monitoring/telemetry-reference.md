# Telemetry Reference

You will learn the runtime fields and OpenTelemetry metrics that update-ipsets exposes.

## Where telemetry appears

The admin status API and OpenTelemetry are related but not identical.

| Surface | Location | Meaning |
|---------|----------|---------|
| Admin status | `GET /api/v1/admin/status` | Point-in-time JSON snapshot for operators |
| Admin scheduler counters | `metrics` | Monotonic scheduler counters and latest batch timings |
| Admin engine timings | `engine.current_metrics`, `engine.last_metrics`, `engine.lifetime_metrics.operations` | Run and operation timings captured by the processing engine |
| Admin engine counters | `engine.lifetime_metrics.counters` | Engine, downloader-status, public HTTP, admin HTTP, and entity counters |
| Admin queue state | `queues` | Waiting, active, deferred, and recently transitioned work |
| Admin system state | `system` | Go runtime, process, disk, CPU, I/O, and file-descriptor snapshots |
| Prometheus scrape | `GET /metrics` on the admin surface | Current OpenTelemetry metrics in Prometheus text format |
| OpenTelemetry | OTLP metrics, traces, logs | Designed counters, gauges, duration histograms, spans, and logs |

OpenTelemetry counters are cumulative. Duration metrics use the
`<operation>.duration_ms` histogram naming pattern. Byte counters are exported
only for operations where byte volume is part of the designed metric surface.

## OpenTelemetry metric labels

Metric labels are reserved for bounded identity that helps operators group
series. update-ipsets keeps labels such as feed name, status, route, operation
type, component, and engine phase where they have direct diagnostic value.

Runtime quantities are values, not labels. Queue depth, batch size,
selected-feed count, processor-step count, input bytes, fan-in counts, process
ID, automatic host/OS identity, and service-version churn are not attached to
OpenTelemetry metrics by default. Queue, host, and process details remain
available through the admin status API, normal host/process monitoring, traces,
logs, or explicit operator-provided resource attributes.

HTTP API metrics use normalized route templates. The default HTTP duration
metric keeps only `http.route`, `http.request.method`, and
`http.response.status_code`. Raw feed names, provider names, query strings,
client addresses, request paths, server addresses, and protocol details are
not default HTTP metric labels.

API-triggered recalculation and dynamic work uses only `api.surface`,
`api.action`, and `api.result` labels. Target counts are recorded as metric
values, not labels.

The default OpenTelemetry metric surface is an allow-list. Ad hoc internal
operation timings remain available in admin snapshots, traces, or logs, but
they do not become default Prometheus/OTLP metric families.

`GET /metrics` is intentionally not protected by admin basic authentication.
When the daemon uses a separate admin listener, this route is available on that
admin listener and not on the public listener. When the daemon uses one shared
listener, `/metrics` is exposed on that listener. Scrapes are bounded; if a
scrape is already active or telemetry collection cannot finish quickly enough,
the endpoint returns `503 Service Unavailable` instead of blocking web serving.
If the timed-out scrape worker is still unwinding, later scrapes also fail fast
instead of starting more scrape workers.

Metric export is best-effort under backpressure. The daemon records production
metrics through non-blocking local queues; if telemetry export cannot keep up,
some metric samples may be dropped before ingestion, admin, public serving,
health, or watchdog work is delayed.

OpenTelemetry log export is also best-effort. The local application log still
uses the configured local handler, while the OTel log branch uses a bounded
async queue. If that queue is full, OTel log records may be dropped before
daemon work is delayed.

## Admin scheduler counters

These fields appear under `metrics` in the admin status response.

| Field | Meaning |
|-------|---------|
| `download_enqueued` | Items admitted to the download queue |
| `download_deferred` | Download items deferred because inputs are not settled |
| `download_started` | Download items started by workers |
| `download_finished` | Download items completed by workers |
| `processing_enqueued` | Items admitted to the processing queue |
| `processing_requeued` | Processing items requeued for another pass |
| `processing_batches_started` | Processing batches started |
| `processing_batches_completed` | Processing batches completed |
| `processing_items_started` | Total items included in started processing batches |
| `max_download_waiting` | Highest observed download queue depth |
| `max_processing_waiting` | Highest observed processing queue depth |
| `last_batch_size` | Number of items in the latest processing batch |
| `last_batch_duration_ms` | Duration of the latest completed processing batch |
| `snapshot_persist_errors` | Scheduler snapshot persistence failures |
| `operations` | Scheduler operation timing rows with `name`, `count`, `total_ms`, `avg_ms`, and `max_ms` |

## Scheduler operation timings

These operation names can appear in the admin status `metrics.operations` rows.
They are admin snapshot timings, not separate OpenTelemetry metric names.

| Operation name | Meaning |
|----------------|---------|
| `scheduler.fetch_and_stage` | Time spent fetching and staging one downloader item |
| `scheduler.promote_committed_downloads` | Time spent promoting staged provider/artifact inputs before publishing |
| `scheduler.run_once` | Time spent in one processing-engine run for a scheduler batch |
| `scheduler.processing_batch_total` | Total wall time for a processing batch, including success or failure handling |

## Admin system fields

These fields appear under `system`. They are snapshots, not monotonic counters.

| Field | Meaning |
|-------|---------|
| `uptime` | Daemon uptime |
| `go_version`, `goos`, `goarch` | Go runtime and platform |
| `goroutines` | Current goroutine count |
| `heap_alloc`, `heap_sys`, `heap_inuse`, `stack_inuse`, `sys` | Go runtime memory statistics in bytes |
| `num_gc`, `last_gc_unix`, `gc_pause_total_ns` | Garbage-collection statistics |
| `disk_free` | Free space string for the configured runtime disk |
| `rss_kb`, `vms_kb`, `data_kb` | Process memory from the operating system, in KiB |
| `cpu_user_seconds`, `cpu_system_seconds`, `cpu_total_seconds` | Process CPU usage |
| `proc_read_bytes`, `proc_write_bytes`, `proc_cancelled_write_bytes` | Process I/O byte counters |
| `proc_read_syscalls`, `proc_write_syscalls` | Process I/O syscall counters |
| `open_fds` | Current open file descriptors |

## Default OpenTelemetry Metrics

The default OpenTelemetry surface is deliberately small. It currently contains
48 designed instrument names before Prometheus expands counters and histograms
into text-format sample names.

Detailed engine, scheduler, metadata, entity, file, and processor timings still
appear in admin status snapshots where they are useful for local diagnosis.
They are not default OpenTelemetry metric families.

## HTTP and API

Default OpenTelemetry API metrics are intentionally small.

| Metric | Surface | Meaning |
|--------|---------|---------|
| `http.server.request.duration` | OpenTelemetry | RED metric for public and admin API requests. Use histogram count/sum/buckets for rate and latency; use `http.response.status_code` for errors. Labels are limited to route, method, and status. |
| `api.recalculation.requests` | OpenTelemetry | Public or admin API calls that performed dynamic compute or requested recalculation/recovery work. |
| `api.recalculation.targets` | OpenTelemetry | Number of feeds/artifacts queued by an API-triggered recalculation/recovery action. |

`api.recalculation.requests` and `api.recalculation.targets` use these bounded
labels:

| Label | Meaning |
|-------|---------|
| `api.surface` | `public` or `admin` |
| `api.action` | Bounded action such as `compose`, `search`, `feed_search`, `run_due`, `feed_recheck`, `feed_reprocess`, `artifact_recheck`, `integrity_reprocess`, or `entity_rebuild` |
| `api.result` | Bounded result such as `ok`, `error`, `scheduled`, `conflict`, `rejected`, `in_progress`, or `clean` |

Default OpenTelemetry export drops `http.server.request.body.size`,
`http.server.response.body.size`, and ad hoc handler metrics under
`http.admin_*`, `http.home_*`, `http.compare_set.*`, and
`http.entity_artifact.*`.

Some detailed HTTP work counters still appear in admin engine snapshots for
local operator inspection. They are not part of the default OpenTelemetry API
metric surface unless a later area-specific metric design reintroduces them.

## Feed State

| Metric | Meaning |
|--------|---------|
| `feed.state` | Numeric current-state gauge per public feed |
| `feed.health.state` | Numeric health-class gauge per public feed |
| `feed.entries` | Current entry count per public feed |
| `feed.unique_ips` | Current unique-IP count per public feed |
| `feed.errors` | Current downloader failure count per public feed |
| `feed.freshness.seconds` | Seconds since the feed was last processed |
| `feed.last_success.timestamp` | Unix timestamp of the last successful processed output |

Feed metrics use only the `feed.name` label.

`feed.state` values:

| Value | Meaning |
|-------|---------|
| `0` | Unknown or no explicit status |
| `1` | Disabled |
| `2` | Pending first observation |
| `3` | Running |
| `4` | Completed or otherwise known |
| `5` | Degraded health |
| `6` | Error or unavailable |

`feed.health.state` values:

| Value | Meaning |
|-------|---------|
| `0` | Unknown |
| `1` | Healthy |
| `2` | Delayed |
| `3` | Risky |
| `4` | Unavailable |
| `5` | Archived |
| `6` | Empty |
| `7` | Unmaintained |

## Artifact Cache

| Metric | Meaning |
|--------|---------|
| `web.artifact.cache.lookups` | Artifact cache lookups by result |
| `web.artifact.cache.evictions` | Artifact cache evictions by reason |
| `web.artifact.cache.entries` | Current cached artifact entry count |
| `web.artifact.cache.bytes` | Current cached artifact bytes |

Allowed labels are `cache.result` for lookups and `cache.reason` for evictions.

## Scheduler

| Metric | Meaning |
|--------|---------|
| `scheduler.queue.admissions` | Queue admissions by queue and result |
| `scheduler.work.started` | Work starts by queue |
| `scheduler.work.completed` | Work completions by queue |
| `scheduler.queue.depth` | Current queue depth by queue |
| `scheduler.batch.items` | Current or latest processing batch size |
| `scheduler.batch.duration_ms` | Processing batch duration histogram |

Allowed labels are `scheduler.queue` and `scheduler.result`. Queue depth and
batch size are metric values, not labels.

## Downloader

| Metric | Meaning |
|--------|---------|
| `download.fetches` | Downloader fetch attempts by downloader and result status |
| `download.fetch.bytes` | Response bytes from downloader fetches |
| `download.fetch.duration_ms` | Downloader fetch duration histogram |
| `download.errors` | Downloader fetch failures |

Allowed labels are `download.downloader` and `download.status`.

## Processor

| Metric | Meaning |
|--------|---------|
| `processor.runs` | Processor pipeline runs by mode and status |
| `processor.run.duration_ms` | Processor run duration histogram |
| `processor.temp.writes` | Temporary processor writes by kind |
| `processor.temp.write.duration_ms` | Temporary processor write duration histogram |

Allowed labels are `processor.mode`, `processor.status`, and
`processor.temp.kind`. Per-step processor timings remain admin snapshot or
trace detail, not default metrics.

## Engine

| Metric | Meaning |
|--------|---------|
| `engine.runs` | Processing-engine runs by reason and status |
| `engine.run.duration_ms` | End-to-end processing-engine run duration |
| `engine.running` | Current engine running state, `1` or `0` |
| `engine.phase.duration_ms` | Engine phase duration histogram |
| `engine.phase.current` | Current engine phase gauge, `1` for active phase and `0` otherwise |

Allowed labels are `run.reason`, `run.status`, and `engine.phase`.

Current phases are `preflight`, `sources`, `geoip`, `bogons`,
`critical_infrastructure`, `asn`, `entities`, `metadata`, `insights`, and
`publish`.

## Integrity

| Metric | Meaning |
|--------|---------|
| `integrity.checks` | Integrity checks by kind and result |
| `integrity.check.duration_ms` | Integrity check duration histogram |
| `integrity.findings` | Current finding count by integrity kind |
| `integrity.recovery.targets` | Recovery targets scheduled by kind and action |

Allowed labels are `integrity.kind`, `integrity.result`, and
`integrity.action`.

## Background Work

| Metric | Meaning |
|--------|---------|
| `background.tasks` | Background task starts/completions/failures by component |
| `background.worker.wait.duration_ms` | Time spent waiting for a background worker slot |
| `background.workers.active` | Active background workers by component |
| `background.workers.limit` | Configured background worker limit by component |

Allowed labels are `background.component` and `background.result`.

## Config, Runtime Cache, and Daemon

| Metric | Meaning |
|--------|---------|
| `config.loads` | Configuration load attempts by result |
| `config.load.duration_ms` | Configuration load duration histogram |
| `runtime.cache.operations` | Runtime cache load/save operations |
| `runtime.cache.operation.duration_ms` | Runtime cache operation duration histogram |
| `daemon.up` | Daemon liveness gauge, `1` while the process is scraping/exporting metrics |

Allowed labels are `config.result`, `cache.operation`, and `cache.result`.

## iprange

These OpenTelemetry metrics track IP set primitive operations.

| Metric | Meaning |
|--------|---------|
| `iprange.operations` | IP range primitive operation counts |
| `iprange.operation.duration_ms` | IP range primitive operation duration histogram |

Allowed labels are `ip.version` and `iprange.operation`. Source type, compare
mode, count mode, and bytes are not default metric labels.

## Computing rates

Most counters are monotonic over the daemon lifetime. To compute rates, sample twice and divide by elapsed seconds:

```text
rate = (counter_t2 - counter_t1) / (t2 - t1)
```

Use admin status for spot checks. Use OpenTelemetry for durable dashboards, alerting, and history.
