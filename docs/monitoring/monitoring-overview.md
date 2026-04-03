# Monitoring Overview

You will learn how to observe update-ipsets at runtime and what signals matter most.

## Two monitoring surfaces

update-ipsets exposes two independent monitoring surfaces.

**Admin status API** — a snapshot of counters and state you query on demand.

- Poll `GET /api/v1/admin/status` at regular intervals.
- Each response contains monotonic counters (they only go up).
- Sample twice, compute deltas, divide by elapsed time to get rates.
- No collector or agent required. Works with `curl`, cron, or any HTTP client.

**OpenTelemetry export** — continuous push of metrics, traces, and logs to a collector.

- Configure an OTLP endpoint and the daemon pushes data automatically.
- Works with Netdata, Grafana, Jaeger, Honeycomb, or any OTLP-compatible backend.
- Covers the same counters as the admin API plus distributed traces for slow operations.

Use the admin API for quick checks and ad-hoc debugging. Use OpenTelemetry for continuous dashboards, alerting, and historical trends.

## What to watch

These signals give the most operational insight.

- **Download failure rate** — count `download.failed` vs `download.ok`. A rising failure rate means upstream sources or network connectivity are degrading.
- **Processing duration** — watch `engine.<phase>` timings. Spikes indicate large feeds, provider changes, or heavy comparison fan-out.
- **Memory** — track process RSS via the admin status or your collector. Sustained growth above `GOMEMLIMIT` suggests a leak or an unbounded workload.
- **Cache hit rates** — the public artifact cache reports hits and misses. Low hit rates on high-traffic routes mean repeated disk reads for the same files.

## Quick check with the admin API

```bash
# First sample
curl -s -u "$UPDATE_IPSETS_ADMIN_USER:$UPDATE_IPSETS_ADMIN_PASSWORD" http://localhost:18889/api/v1/admin/status > /tmp/s1.json
sleep 60
# Second sample
curl -s -u "$UPDATE_IPSETS_ADMIN_USER:$UPDATE_IPSETS_ADMIN_PASSWORD" http://localhost:18889/api/v1/admin/status > /tmp/s2.json

# Compare download counters
jq '.counters.download' /tmp/s1.json /tmp/s2.json
```

## Quick check with OpenTelemetry

See [OpenTelemetry Setup](opentelemetry-setup.md) for configuration. Once enabled, point your collector at the daemon's OTLP endpoint and build dashboards from the metric names in the [Telemetry Reference](telemetry-reference.md).
