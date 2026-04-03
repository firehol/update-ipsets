# OpenTelemetry Setup

You will learn how to enable, configure, and disable OpenTelemetry export in update-ipsets.

## Enabling export

Set either of these to enable export:

```bash
export UPDATE_IPSETS_OTEL=1
```

or set an OTLP endpoint directly:

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
```

The daemon detects the endpoint variable and enables export automatically.

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

For gRPC, the endpoint value requires an explicit scheme:

```bash
# Correct
export OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4317

# Incorrect — gRPC exporters reject bare host:port
export OTEL_EXPORTER_OTLP_ENDPOINT=127.0.0.1:4317
```

## Suppressing individual signals

Disable traces when you only want metrics and logs:

```bash
export OTEL_TRACES_EXPORTER=none
```

Or use the daemon-specific variables:

```bash
export UPDATE_IPSETS_OTEL_TRACES=none
export UPDATE_IPSETS_OTEL_METRICS=none
export UPDATE_IPSETS_OTEL_LOGS=none
```

Each variable disables one signal independently.

## Metric export interval

Control how often the daemon pushes metrics:

```bash
# Push every 10 seconds (value is milliseconds)
export UPDATE_IPSETS_OTEL_METRIC_INTERVAL=10000
```

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
