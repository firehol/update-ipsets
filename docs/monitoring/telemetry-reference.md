# Telemetry Reference

You will learn the runtime fields and local/exported metrics that update-ipsets exposes.

## Where telemetry appears

The admin status API and exported metrics are related but not identical.

| Surface | Location | Meaning |
|---------|----------|---------|
| Admin status | `GET /api/v1/admin/status` | Point-in-time JSON snapshot for operators |
| Admin scheduler counters | `metrics` | Monotonic scheduler counters and latest batch timings |
| Admin engine timings | `engine.current_metrics`, `engine.last_metrics`, `engine.lifetime_metrics.operations` | Run and operation timings captured by the processing engine |
| Admin engine counters | `engine.lifetime_metrics.counters` | Engine, downloader-status, public HTTP, admin HTTP, and entity counters |
| Admin queue state | `queues` | Waiting, active, deferred, and recently transitioned work |
| Admin system state | `system` | Go runtime, process, disk, CPU, I/O, and file-descriptor snapshots |
| Prometheus scrape | `GET /metrics` on the admin surface | Current local metrics in Prometheus text format |
| OpenTelemetry | OTLP metrics | Designed counters, gauges, and duration aggregates exported from local metrics |

Counters are cumulative. Duration metrics use the `<operation>.duration_ms`
naming pattern and export count, sum, and max aggregates. Byte counters are
exported only for operations where byte volume is part of the designed metric
surface.

## Metric Labels

Metric labels are reserved for bounded identity that helps operators group
series. update-ipsets keeps only compile-time finite labels such as status,
route, operation type, component, and engine phase where they have direct
diagnostic value.

Runtime quantities are values, not labels. Queue depth, batch size,
selected-feed count, processor-step count, input bytes, fan-in counts, process
ID, feed/provider identity, automatic host/OS identity, and service-version
churn are not attached to metrics by default. Queue, host, feed/provider, and
process details remain available through the admin status API, normal
host/process monitoring, local logs, local traces, or explicit
operator-provided resource attributes.

HTTP API metrics use normalized route templates. The default HTTP duration
metric keeps only `http.route`, `http.request.method`, and
`http.response.status_code`. Raw feed names, provider names, query strings,
client addresses, request paths, server addresses, and protocol details are
not default HTTP metric labels.

API-triggered recalculation and dynamic work uses only `api.surface`,
`api.action`, and `api.result` labels. Target counts are recorded as metric
values, not labels.

The default metric surface is a compile-time allow-list. Ad hoc internal
operation timings remain available in admin snapshots, local traces, or logs,
but they do not become default Prometheus/OTLP metric families.

`GET /metrics` is intentionally not protected by admin basic authentication.
When the daemon uses a separate admin listener, this route is available on that
admin listener and not on the public listener. When the daemon uses one shared
listener, `/metrics` is exposed on that listener. Scrapes are bounded; if a
scrape is already active or telemetry collection cannot finish quickly enough,
the endpoint returns `503 Service Unavailable` instead of blocking web serving.
If the timed-out scrape worker is still unwinding, later scrapes also fail fast
instead of starting more scrape workers.

Metric updates are exact local atomic state. OTLP exporter failure or
backpressure cannot change local metric values.

Runtime and process gauges are sampled by the daemon's local runtime sampler.
Prometheus scrapes and OTLP export reads use the sampled local values; they do
not read `/proc` or runtime counters directly.

Local logs use a bounded daemon-owned queue. Local traces are disabled by
default and use a bounded daemon-owned queue only when explicitly enabled. If
an enabled buffer is full, the daemon drops records before delaying application
work and increments `telemetry.logs.dropped` or `telemetry.traces.dropped`.

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
| `num_gc`, `last_gc_unix`, `gc_pause_total_ns` | Garbage-collection statistics; `last_gc_unix` may be unset when the safe runtime sampler cannot collect it without stop-the-world APIs |
| `disk_free` | Free space string for the configured runtime disk |
| `cpu_user_seconds`, `cpu_system_seconds`, `cpu_total_seconds` | Process CPU usage |

OS process memory, file descriptor, and process I/O counters are intentionally
not collected by the application runtime sampler. Use host monitoring for
those signals so admin/web serving does not depend on procfs reads.

## Default Metrics

The default metric surface is deliberately small. It currently contains
75 designed metric names before Prometheus expands counters and duration
aggregates into text-format sample names.

Detailed engine, scheduler, metadata, entity, file, and processor timings still
appear in admin status snapshots where they are useful for local diagnosis.
They are not default metric families.

## HTTP and API

Default API metrics are intentionally small.

| Metric | Surface | Meaning |
|--------|---------|---------|
| `http.server.request.duration` | Metrics | RED metric for public and admin API requests. Use count/sum/max for rate and latency; use `http.response.status_code` for errors. Labels are limited to route, method, and status. |
| `api.recalculation.requests` | Metrics | Public or admin API calls that performed dynamic compute or requested recalculation/recovery work. |
| `api.recalculation.targets` | Metrics | Number of feeds/artifacts queued by an API-triggered recalculation/recovery action. |

`api.recalculation.requests` and `api.recalculation.targets` use these bounded
labels:

| Label | Meaning |
|-------|---------|
| `api.surface` | `public` or `admin` |
| `api.action` | Bounded action such as `compose`, `search`, `feed_search`, `run_due`, `feed_recheck`, `feed_reprocess`, `artifact_recheck`, `integrity_reprocess`, or `entity_rebuild` |
| `api.result` | Bounded result such as `ok`, `error`, `scheduled`, `conflict`, `rejected`, `in_progress`, or `clean` |

The default metric schema omits `http.server.request.body.size`,
`http.server.response.body.size`, and ad hoc handler metrics under
`http.admin_*`, `http.home_*`, `http.compare_set.*`, and
`http.entity_artifact.*`.

Some detailed HTTP work counters still appear in admin engine snapshots for
local operator inspection. They are not part of the default API
metric surface unless a later area-specific metric design reintroduces them.

## Feed Catalog

| Metric | Meaning |
|--------|---------|
| `feed.catalog.feeds` | Number of public catalog feed summaries |
| `feed.catalog.entries` | Aggregate public catalog entry count |
| `feed.catalog.unique_ips` | Aggregate public catalog unique-IP count |
| `feed.catalog.errors` | Aggregate downloader errors across public catalog rows |

Per-feed state, health, timestamps, entries, and error detail remain in the
admin status API and public catalog artifacts. They are intentionally not metric
labels.

## Artifact Cache

| Metric | Meaning |
|--------|---------|
| `web.artifact.cache.lookups` | Artifact cache lookups by result |
| `web.artifact.cache.evictions` | Artifact cache evictions by reason |
| `web.artifact.cache.entries` | Current cached artifact entry count |
| `web.artifact.cache.bytes` | Current cached artifact bytes |

Allowed labels are `cache.result` for lookups and `cache.reason` for evictions.

## File Writes

| Metric | Meaning |
|--------|---------|
| `file.write_atomic` | Atomic file write operations |
| `file.write_atomic.bytes` | Bytes written by atomic file writes |
| `file.write_atomic.duration_ms` | Atomic file write duration aggregate |

Allowed label is `file.sync`.

## Scheduler

| Metric | Meaning |
|--------|---------|
| `scheduler.queue.admissions` | Queue admissions by queue and result |
| `scheduler.work.started` | Work starts by queue |
| `scheduler.work.completed` | Work completions by queue |
| `scheduler.queue.depth` | Current queue depth by queue |
| `scheduler.batch.items` | Current or latest processing batch size |
| `scheduler.batch.duration_ms` | Processing batch duration aggregate |
| `scheduler.action.admission_failures` | Failed scheduler action admissions |
| `scheduler.recovered_panics` | Recovered scheduler panics by component |

Allowed labels are `scheduler.queue`, `scheduler.result`, and
`scheduler.component`. Queue depth and batch size are metric values, not labels.

## Downloader

| Metric | Meaning |
|--------|---------|
| `download.fetches` | Downloader fetch attempts by result status |
| `download.fetch.bytes` | Response bytes from downloader fetches |
| `download.fetch.duration_ms` | Downloader fetch duration aggregate |
| `download.errors` | Downloader fetch failures |

Allowed label is `download.status`.

## Processor

| Metric | Meaning |
|--------|---------|
| `processor.runs` | Processor pipeline runs by mode and status |
| `processor.run.duration_ms` | Processor run duration aggregate |
| `processor.temp.writes` | Temporary processor writes by kind |
| `processor.temp.write.duration_ms` | Temporary processor write duration aggregate |

Allowed labels are `processor.mode`, `processor.status`, and
`processor.temp.kind`. Per-step processor timings remain admin snapshot or
trace detail, not default metrics.

## Engine

| Metric | Meaning |
|--------|---------|
| `engine.runs` | Processing-engine runs by reason and status |
| `engine.run.duration_ms` | End-to-end processing-engine run duration |
| `engine.running` | Current engine running state, `1` or `0` |
| `engine.phase.duration_ms` | Engine phase duration aggregate |
| `engine.phase.current` | Current engine phase gauge, `1` for active phase and `0` otherwise |

Allowed labels are `run.reason`, `run.status`, and `engine.phase`.

Current phases are `preflight`, `sources`, `geoip`, `bogons`,
`critical_infrastructure`, `asn`, `entities`, `metadata`, `insights`, and
`publish`.

## Integrity

| Metric | Meaning |
|--------|---------|
| `integrity.checks` | Integrity checks by kind and result |
| `integrity.check.duration_ms` | Integrity check duration aggregate |
| `integrity.findings` | Current finding count by integrity kind |
| `integrity.recovery.targets` | Recovery targets scheduled by kind and action |

Allowed labels are `integrity.kind`, `integrity.result`, and
`integrity.action`.

## Background Work

| Metric | Meaning |
|--------|---------|
| `background.tasks` | Background task starts/completions/failures by component |
| `background.worker.wait.duration_ms` | Time spent waiting for a background worker slot |
| `background.worker.long_running` | Long-running background worker diagnostics by component |
| `background.workers.active` | Active background workers |
| `background.workers.attach_duplicate` | Duplicate background-lane context attachment attempts |
| `background.workers.finalization_panic` | Recovered panics while finalizing background-lane work |
| `background.workers.limit` | Configured background worker limit |

Allowed labels are `background.component` and `background.result`.

## Config, Runtime Cache, and Daemon

| Metric | Meaning |
|--------|---------|
| `config.loads` | Configuration load attempts by result |
| `config.load.duration_ms` | Configuration load duration aggregate |
| `runtime.cache.operations` | Runtime cache load/save operations |
| `runtime.cache.operation.duration_ms` | Runtime cache operation duration aggregate |
| `runtime.go.goroutines` | Current Go goroutine count |
| `runtime.go.heap.alloc.bytes` | Bytes allocated and still in use by Go heap objects |
| `runtime.go.heap.sys.bytes` | Bytes obtained from the OS for the Go heap |
| `runtime.go.heap.inuse.bytes` | Bytes in Go heap spans currently in use |
| `runtime.go.heap.released.bytes` | Bytes of idle Go heap released to the OS |
| `runtime.go.heap.objects` | Live Go heap object count |
| `runtime.go.stack.inuse.bytes` | Bytes in stack spans currently in use |
| `runtime.go.sys.bytes` | Total bytes obtained from the OS by the Go runtime |
| `runtime.go.gc.count` | Completed Go GC cycle count |
| `runtime.go.gc.pause.total.ms` | Cumulative Go GC pause estimate in milliseconds from runtime metrics |
| `runtime.go.mem.limit.bytes` | Active Go memory limit, or `-1` when unlimited |
| `runtime.process.cpu.user.ms` | Cumulative process user CPU time in milliseconds |
| `runtime.process.cpu.system.ms` | Cumulative process system CPU time in milliseconds |
| `runtime.process.cpu.total.ms` | Cumulative total process CPU time in milliseconds |
| `daemon.up` | Daemon liveness gauge, `1` while the process is scraping/exporting metrics |
| `daemon.goroutine.panics` | Recovered daemon-control goroutine panics |
| `daemon.watchdog.diagnostics` | Watchdog diagnostic events |
| `engine.lane.diagnostics.panics` | Recovered engine-lane diagnostics panics |
| `systemd.notify.failures` | systemd notify failures |
| `telemetry.metrics.unknown` | Attempts to record a metric name outside the compile-time schema |
| `telemetry.logs.dropped` | Local log records dropped because the bounded buffer was full |
| `telemetry.traces.dropped` | Local trace records dropped because the bounded buffer was full |

Allowed labels are `config.result`, `cache.operation`, `cache.result`, and
`daemon.goroutine`.

## iprange

`pkg/iprange` returns plain operation counters to callers and does not import
the application telemetry package. Those operation stats are not exported as
default metric families until a production caller records them through a
predeclared metric surface.

## Computing rates

Most counters are monotonic over the daemon lifetime. To compute rates, sample twice and divide by elapsed seconds:

```text
rate = (counter_t2 - counter_t1) / (t2 - t1)
```

Use admin status for spot checks. Use OpenTelemetry metric export for durable dashboards, alerting, and history.
