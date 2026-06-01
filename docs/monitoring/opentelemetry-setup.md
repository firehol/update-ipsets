# OpenTelemetry Setup

You will learn how to enable, configure, and disable OpenTelemetry export in update-ipsets.

The admin surface also serves `GET /metrics` in Prometheus text format. This
scrape endpoint is available without admin basic authentication and does not
require OTLP export to be enabled. In split-listener deployments, expose only
the admin listener to systems that should scrape it.

## Enabling export

Set either of these to enable export:

```bash
export UPDATE_IPSETS_OTEL=1
```

or set an OTLP endpoint directly:

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
```

The daemon also enables export automatically when any signal-specific OTLP endpoint is set:

```bash
export OTEL_EXPORTER_OTLP_METRICS_ENDPOINT=http://localhost:4318
export OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=http://localhost:4318
export OTEL_EXPORTER_OTLP_LOGS_ENDPOINT=http://localhost:4318
```

Use the collector's OTLP/HTTP endpoint when the protocol is left at the default
`http/protobuf`. Use the OTLP/gRPC endpoint, commonly port `4317`, only when
`UPDATE_IPSETS_OTEL_PROTOCOL=grpc` or `OTEL_EXPORTER_OTLP_PROTOCOL=grpc` is set.

## Disabling export

```bash
export UPDATE_IPSETS_OTEL=0
```

This disables export even when `OTEL_EXPORTER_OTLP_ENDPOINT` is set. Local structured logs continue writing to stderr regardless of this setting.

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

The OTLP exporters also honor standard OpenTelemetry collector options. Use
signal-specific variables only when traces, metrics, or logs need different
settings:

```bash
export OTEL_EXPORTER_OTLP_HEADERS=tenant=iplists
export OTEL_EXPORTER_OTLP_TIMEOUT=10000
export OTEL_EXPORTER_OTLP_COMPRESSION=gzip
```

For TLS and mTLS collectors, use the standard certificate variables:
`OTEL_EXPORTER_OTLP_CERTIFICATE`, `OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE`, and
`OTEL_EXPORTER_OTLP_CLIENT_KEY`. Each has signal-specific variants such as
`OTEL_EXPORTER_OTLP_METRICS_CERTIFICATE`.

## Suppressing individual signals

Disable traces when you only want metrics and logs:

```bash
export OTEL_TRACES_EXPORTER=none
```

The same standard form works for metrics and logs:

```bash
export OTEL_METRICS_EXPORTER=none
export OTEL_LOGS_EXPORTER=none
```

Or use the daemon-specific variables:

```bash
export UPDATE_IPSETS_OTEL_TRACES=0
export UPDATE_IPSETS_OTEL_METRICS=0
export UPDATE_IPSETS_OTEL_LOGS=0
```

Each variable disables one signal independently.

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

## Example: minimal HTTP export

```bash
UPDATE_IPSETS_OTEL=1 \
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318 \
update-ipsets daemon --config /opt/update-ipsets/etc/config
```

## Example: full gRPC export with traces suppressed

```bash
UPDATE_IPSETS_OTEL=1 \
UPDATE_IPSETS_OTEL_PROTOCOL=grpc \
OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4317 \
OTEL_METRIC_EXPORT_INTERVAL=10000 \
OTEL_TRACES_EXPORTER=none \
update-ipsets daemon --config /opt/update-ipsets/etc/config
```
