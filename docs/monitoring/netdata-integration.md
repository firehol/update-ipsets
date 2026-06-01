# Netdata Integration

You will learn how to export update-ipsets metrics to a local Netdata instance via OpenTelemetry.

## Prerequisites

- Netdata installed and running with the `otel-plugin` enabled
- update-ipsets daemon running on the same host (or reachable over the network)

Netdata's `otel-plugin` listens for OTLP/gRPC on port 4317 by default.

## Default systemd drop-in

The installed systemd unit already includes these defaults. If you customized the unit, add a drop-in at `/etc/systemd/system/update-ipsets.service.d/otel.conf`:

```ini
[Service]
Environment="UPDATE_IPSETS_OTEL=1"
Environment="UPDATE_IPSETS_OTEL_PROTOCOL=grpc"
Environment="OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4317"
Environment="OTEL_METRIC_EXPORT_INTERVAL=10000"
Environment="OTEL_TRACES_EXPORTER=none"
```

Then reload and restart:

```bash
sudo systemctl daemon-reload
sudo systemctl restart update-ipsets
```

## What this configuration does

- Enables OpenTelemetry export
- Uses gRPC protocol (required by Netdata's otel-plugin)
- Pushes metrics every 10 seconds, matching Netdata's default OTel chart interval
- Suppresses traces — update-ipsets primarily needs metrics, not distributed tracing
- Exports logs through OpenTelemetry unless you set `OTEL_LOGS_EXPORTER=none`

## Verifying the integration

After restarting, open Netdata and look for new charts under the `update_ipsets` or `iprange` application group. You should see counters for:

- Download operations (`download.ok`, `download.failed`, `download.error`, `download.status.downloaded`, etc.)
- Processing phases and queues (`engine.queued`, `engine.batch.completed`, `engine.<phase>`)
- iprange primitives (`iprange.load.text`, `iprange.merge.ops`, etc.)
- Public/admin API activity (`http.admin_status`, `http.home_summary.requests`, etc.)

Process CPU, memory, and file-descriptor usage is available through the admin status API under `system` and through Netdata's normal host/process monitoring charts.

If you don't see charts within 30 seconds, check:

1. Netdata is running: `sudo netdatacli uptime`
2. otel-plugin is enabled in `netdata.conf`
3. Port 4317 is open: `ss -tlnp | grep 4317`
4. Daemon logs show OTLP connection: `journalctl -u update-ipsets -n 50`

## Tuning for Netdata

The 10-second metric interval aligns with Netdata's default OTel chart update rate. If you changed Netdata's `update every` for the otel-plugin, match that interval here:

```ini
# If Netdata's otel-plugin is configured for 5-second updates
Environment="OTEL_METRIC_EXPORT_INTERVAL=5000"
```

Shorter intervals produce finer charts but increase CPU and network overhead on both sides.

## Remote Netdata

If Netdata runs on a different host, change the endpoint:

```ini
Environment="OTEL_EXPORTER_OTLP_ENDPOINT=http://netdata-host:4317"
```

Ensure the network path allows gRPC traffic on port 4317.
