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

**OpenTelemetry export** — continuous push of metrics, traces, and logs to a collector.

- Configure an OTLP endpoint and the daemon pushes data automatically.
- Works with Netdata, Grafana, Jaeger, Honeycomb, or any OTLP-compatible backend.
- Exports application counters, operation duration histograms, optional traces, and logs.

Use the admin API for quick checks and ad-hoc debugging. Use OpenTelemetry for continuous dashboards, alerting, and historical trends.

## What to watch

These signals give the most operational insight.

- **Download failure rate** — in OpenTelemetry, compare `download.failed`, `download.error`, and `download.status.download_failed` against successful statuses such as `download.ok` and `download.status.downloaded`. In the admin API, inspect `engine.lifetime_metrics.counters` entries beginning with `download.status.`.
- **Scheduler throughput** — sample `metrics.download_enqueued`, `metrics.download_started`, `metrics.download_finished`, `metrics.processing_enqueued`, and `metrics.processing_batches_completed`.
- **Processing duration** — watch `engine.<phase>.duration_ms`, `engine.last_metrics.phase_times`, and operation timings in `engine.lifetime_metrics.operations`.
- **Memory** — track `system.rss_kb`, `system.heap_alloc`, `system.heap_sys`, `system.num_gc`, and host process charts. Sustained growth above `GOMEMLIMIT` suggests a leak or an unbounded workload.
- **Public/API activity** — watch HTTP counters in `engine.lifetime_metrics.counters`, such as `http.home_summary.requests`, `http.compare_set.requests`, `http.admin_status`, and `http.admin_feeds`.

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

See [OpenTelemetry Setup](opentelemetry-setup.md) for configuration. Once enabled, point your collector at the daemon's OTLP endpoint and build dashboards from the metric names in the [Telemetry Reference](telemetry-reference.md).
