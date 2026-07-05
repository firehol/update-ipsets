# Monitoring Overview

You will learn how to observe update-ipsets at runtime and what signals matter most.

## Two monitoring surfaces

update-ipsets exposes two independent monitoring surfaces.

**Admin status API** — a snapshot of runtime state, scheduler counters, queues, feed health, and system resources you query on demand.

- Poll `GET /api/v1/admin/status` at regular intervals.
- Scheduler counters live under `metrics`. Engine and HTTP counters live under `engine.lifetime_metrics.counters`.
- Queue snapshots live under `queues`; process and Go runtime resource snapshots live under `system`.
- Sample twice, compute deltas, divide by elapsed time to get rates.
- No collector or agent required. Works with `curl`, cron, or any HTTP client.

**OpenTelemetry metric export** — continuous push of designed metrics to a collector.

- Configure an OTLP endpoint and the daemon pushes data automatically.
- Works with Netdata, Grafana, or any OTLP-compatible metrics backend.
- Exports application counters, gauges, and duration aggregates from the local metric registry.

Use the admin API for quick checks and ad-hoc debugging. Use OpenTelemetry
metric export for continuous dashboards, alerting, and historical trends.

## What to watch

These signals give the most operational insight.

- **Download failure rate** — in exported metrics, compare `download.errors` and `download.fetches{download.status="error"}` against successful `download.fetches` statuses. In the admin API, inspect `engine.lifetime_metrics.counters` entries beginning with `download.status.`.
- **Scheduler throughput** — sample `metrics.download_enqueued`, `metrics.download_started`, `metrics.download_finished`, `metrics.processing_enqueued`, and `metrics.processing_batches_completed`.
- **Processing duration** — watch `engine.phase.duration_ms`, `engine.run.duration_ms`, `engine.last_metrics.phase_times`, and operation timings in `engine.lifetime_metrics.operations`.
- **Memory** — track `system.heap_alloc`, `system.heap_sys`, `system.num_gc`, and host process charts for RSS/I/O pressure. Sustained growth above `GOMEMLIMIT` suggests a leak or an unbounded workload.
- **Public/API activity** — in exported metrics, watch `http.server.request.duration` and `api.recalculation.*`. In the admin API, detailed counters such as `http.home_summary.requests`, `http.compare_set.requests`, `http.admin_status`, and `http.admin_feeds` remain under `engine.lifetime_metrics.counters`.

## Quick check with the admin API

```bash
# First sample
curl -s -u "$UPDATE_IPSETS_ADMIN_USER:$UPDATE_IPSETS_ADMIN_PASSWORD" http://localhost:18889/api/v1/admin/status > /tmp/s1.json
sleep 60
# Second sample
curl -s -u "$UPDATE_IPSETS_ADMIN_USER:$UPDATE_IPSETS_ADMIN_PASSWORD" http://localhost:18889/api/v1/admin/status > /tmp/s2.json

# Compare scheduler counters
jq '.metrics | {
  download_enqueued,
  download_started,
  download_finished,
  processing_enqueued,
  processing_batches_completed,
  last_batch_duration_ms
}' /tmp/s1.json /tmp/s2.json

# Inspect downloader status counters recorded by the engine
jq '.engine.lifetime_metrics.counters[]? | select(.name | startswith("download.status."))' /tmp/s2.json
```

## Quick check with OpenTelemetry

See [OpenTelemetry Setup](opentelemetry-setup.md) for configuration. Once
enabled, point your collector at the daemon's OTLP endpoint and build dashboards
from the metric names in the [Telemetry Reference](telemetry-reference.md).
