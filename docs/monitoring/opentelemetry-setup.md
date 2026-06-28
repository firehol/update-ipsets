# OpenTelemetry Setup

You will learn how to enable, configure, and disable OpenTelemetry metric export in update-ipsets.

update-ipsets owns its local instrumentation. Metrics are exact local atomic
state; OpenTelemetry is only an optional OTLP metric exporter. Local logs and
local traces use bounded in-process queues and are not exported through the
OpenTelemetry SDK in this daemon version.

The admin surface also serves `GET /metrics` in Prometheus text format. This
scrape endpoint is available without admin basic authentication and does not
require OTLP export to be enabled. In split-listener deployments, expose only
the admin listener to systems that should scrape it. The endpoint is bounded:
concurrent or stuck scrapes return `503 Service Unavailable` rather than waiting
indefinitely.

## Enabling export

Set either of these to enable export:

```bash
export UPDATE_IPSETS_OTEL=1
```

or set an OTLP endpoint directly:

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
```

The daemon also enables export automatically when the metric-specific OTLP endpoint is set:

```bash
export OTEL_EXPORTER_OTLP_METRICS_ENDPOINT=http://localhost:4318
```

Use the collector's OTLP/HTTP endpoint when the protocol is left at the default
`http/protobuf`. Use the OTLP/gRPC endpoint, commonly port `4317`, only when
`UPDATE_IPSETS_OTEL_PROTOCOL=grpc` or `OTEL_EXPORTER_OTLP_PROTOCOL=grpc` is set.

OTLP export is fail-open. If the collector is unreachable, an OTLP option is
invalid, or telemetry setup times out during startup, update-ipsets logs a
warning and continues serving. Local stderr logs continue, and the admin
`GET /metrics` scrape endpoint remains available when local metrics setup
succeeds.

## Disabling export

```bash
export UPDATE_IPSETS_OTEL=0
```

This disables metric export even when `OTEL_EXPORTER_OTLP_ENDPOINT` is set.
Local structured logs continue writing to stderr regardless of this setting.

## Choosing the protocol

The default protocol is `http/protobuf` (OTLP over HTTP).

To use gRPC:

```bash
export UPDATE_IPSETS_OTEL_PROTOCOL=grpc
```

Or:

```bash
export OTEL_EXPORTER_OTLP_PROTOCOL=grpc
```

For gRPC endpoint environment variables, include an explicit scheme. Use
`http://` for plaintext gRPC and `https://` for TLS:

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4317

# Do not use this form for OTEL_EXPORTER_OTLP_ENDPOINT.
export OTEL_EXPORTER_OTLP_ENDPOINT=127.0.0.1:4317
```

## Service identity and collector options

Set the OpenTelemetry service identity when multiple services export to the
same collector:

```bash
export OTEL_SERVICE_NAME=update-ipsets
export OTEL_RESOURCE_ATTRIBUTES=deployment.environment=production,service.namespace=firehol
```

The OTLP metric exporter also honors standard OpenTelemetry collector options.
Use metric-specific variables when metrics need different settings:

```bash
export OTEL_EXPORTER_OTLP_HEADERS=tenant=iplists
export OTEL_EXPORTER_OTLP_TIMEOUT=10000
export OTEL_EXPORTER_OTLP_COMPRESSION=gzip
```

For TLS and mTLS collectors, use the standard certificate variables:
`OTEL_EXPORTER_OTLP_CERTIFICATE`, `OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE`, and
`OTEL_EXPORTER_OTLP_CLIENT_KEY`. Each has metric-specific variants such as
`OTEL_EXPORTER_OTLP_METRICS_CERTIFICATE`.

## Suppressing metric export

Disable OTLP metric export with the standard variable:

```bash
export OTEL_METRICS_EXPORTER=none
```

Or use the daemon-specific variable:

```bash
export UPDATE_IPSETS_OTEL_METRICS=0
```

Trace and log OTLP variables are ignored by this daemon version.

## Metric export interval

Control how often the daemon pushes metrics:

```bash
# Push every 10 seconds
export UPDATE_IPSETS_OTEL_METRIC_INTERVAL=10000
```

The value may be integer milliseconds, such as `10000`, or a duration string,
such as `10s`.

Or use the standard variable:

```bash
export OTEL_METRIC_EXPORT_INTERVAL=10000
```

Shorter intervals give finer resolution but consume more network and collector resources. The installed default is 10 seconds, matching Netdata's OTel chart interval.

## Local log and optional trace buffers

Local log capture is bounded and never waits for a remote collector. Local
trace capture is disabled by default and can be enabled only with an explicit
trace buffer. Full enabled buffers drop records before delaying application
work. The exact local drop counters are `telemetry.logs.dropped` and
`telemetry.traces.dropped`. Oversized log messages, log string attributes,
trace names, and trace string attributes are replaced with a fixed truncated
marker before enqueue, so bounded queues do not retain arbitrary caller-owned
strings.

```bash
# Default local log budget is 50 MiB.
export UPDATE_IPSETS_TELEMETRY_BUFFER_BYTES=50MiB

# Optional per-signal overrides.
export UPDATE_IPSETS_LOG_BUFFER_BYTES=32MB

# Local traces are opt-in; unset or 0 disables trace capture.
export UPDATE_IPSETS_TRACE_BUFFER_BYTES=18MB
```

## Example: minimal HTTP export

```bash
UPDATE_IPSETS_OTEL=1 \
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318 \
update-ipsets daemon --config /opt/update-ipsets/etc/config
```

## Example: gRPC metric export

```bash
UPDATE_IPSETS_OTEL=1 \
UPDATE_IPSETS_OTEL_PROTOCOL=grpc \
OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4317 \
OTEL_METRIC_EXPORT_INTERVAL=10000 \
update-ipsets daemon --config /opt/update-ipsets/etc/config
```
