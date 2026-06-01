# Log Structure

You will learn what update-ipsets logs, how logs are structured, and which events to watch.

## Structured log format

Local stderr and journald output uses Go `slog` text format: a message plus structured `key=value` attributes. OpenTelemetry can also export logs to a collector when OTLP logging is enabled, but the local service log is not JSON.

Common fields and attributes:

| Field | Description |
|-------|-------------|
| `time` | Timestamp in RFC 3339 format |
| `level` | Log severity, such as `INFO`, `WARN`, or `ERROR` |
| `msg` | Human-readable message describing the event |
| `source`, `name`, `set`, `feed`, `artifact` | Feed, set, or artifact name, depending on the operation |
| `path`, `dir`, `config_path` | Filesystem or config path when relevant |
| `error` | Error text, when the event describes a failure |

Example:

```text
time=2026-05-01T12:00:00.000Z level=INFO msg="source updated" source=firehol_level1 entries=142 unique_ips=9834
```

```text
time=2026-05-01T12:00:01.000Z level=ERROR msg="download loop failed" name=tor_exits error="HTTP 403 Forbidden"
```

## Key events to watch

### Startup

```text
time=2026-05-01T12:00:00.000Z level=INFO msg="configuration loaded" sources=423 merges=0 geolocation_providers=5 asn_providers=4 bogon_providers=6 critical_infrastructure_providers=21 base_dir=/example/base lib_dir=/example/lib web_dir=/example/www
```

Startup logs include configuration loading, cache loading, OpenTelemetry state when enabled, and listener startup. The `sources` value is the expanded in-memory catalog, including history derivatives, merge-derived sources, and synthetic helper feeds. The `merges` value is normally `0` because configured merges are expanded into source entries during configuration loading. If startup fails, the error appears before the process exits.

### Shutdown

```text
time=2026-05-01T12:00:00.000Z level=INFO msg="run finished" updated=2 skipped=340 failed=0
```

During shutdown, in-flight work is drained through the scheduler and server shutdown paths. Errors during cleanup are logged as warnings or errors.

### Configuration reload

```text
time=2026-05-01T12:00:00.000Z level=INFO msg="config reloaded" config_path=/opt/update-ipsets/etc/config
```

Or on failure:

```text
time=2026-05-01T12:00:00.000Z level=ERROR msg="config reload failed" error="..."
```

Reload failures leave the previous valid configuration active.

### Download failures

```text
time=2026-05-01T12:00:00.000Z level=ERROR msg="download loop failed" name=tor_exits error="HTTP 403 Forbidden"
```

Repeated download failures for the same feed indicate an upstream problem. The daemon retries with exponential backoff.

### Processing failures

```text
time=2026-05-01T12:00:00.000Z level=ERROR msg="processing item failed; staged input retained and retry scheduled" name=example_feed error="parse_failed: ..."
```

Processing failures are severe runtime faults. Check the exception class in the error message.

### Integrity findings

```text
time=2026-05-01T12:00:00.000Z level=WARN msg="deferred broad startup entity artifact repair" countries=3 asns=1
```

Integrity details are normally inspected through the admin UI or admin API. Logs are useful for seeing when startup repair or background entity repair was queued or failed.

## Log levels

| Level | When to use | Action |
|-------|-------------|--------|
| `info` | Normal operations — startup, shutdown, feed updates, scheduled work | Read when investigating timeline |
| `warn` | Degraded state — download failures, integrity findings, retry backoff | Monitor trends; act if persistent |
| `error` | Failures needing attention — processing exceptions, write failures, config errors | Investigate promptly |

## Quick filtering

Find actionable issues by filtering for errors:

```bash
# Show all errors
journalctl -u update-ipsets | grep 'level=ERROR'

# Show errors for a specific feed
journalctl -u update-ipsets | grep 'level=ERROR' | grep 'tor_exits'
```

Or redirect stderr to a file and filter text records:

```bash
update-ipsets daemon --config /opt/update-ipsets/etc/config 2>daemon.log
grep 'level=ERROR' daemon.log
```
