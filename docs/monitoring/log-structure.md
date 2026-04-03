# Log Structure

You will learn what update-ipsets logs, how logs are structured, and which events to watch.

## Structured log format

All log entries are structured JSON with these fields:

| Field | Description |
|-------|-------------|
| `time` | Timestamp in RFC 3339 format |
| `level` | Log severity: `info`, `warn`, or `error` |
| `msg` | Human-readable message describing the event |
| `component` | Subsystem that emitted the log (e.g., `downloader`, `engine`, `scheduler`) |
| `feed` | Feed name, when the event is feed-specific |
| `error` | Error text, when the event describes a failure |

Example:

```json
{"time":"2025-05-01T12:00:00Z","level":"info","msg":"feed updated","component":"engine","feed":"firehol_level1"}
```

```json
{"time":"2025-05-01T12:00:01Z","level":"error","msg":"download failed","component":"downloader","feed":"tor_exits","error":"HTTP 403 Forbidden"}
```

## Key events to watch

### Startup

```json
{"level":"info","msg":"daemon started","component":"daemon","version":"...","config":"..."}
```

Startup logs include the configuration path, listener addresses, and the number of feeds loaded. If startup fails, the error appears before the process exits.

### Shutdown

```json
{"level":"info","msg":"daemon shutting down","component":"daemon"}
```

Followed by drain logs for in-flight downloads and processing work.

### Configuration reload

```json
{"level":"info","msg":"configuration reloaded","component":"config","feeds":123}
```

Or on failure:

```json
{"level":"error","msg":"configuration reload failed","component":"config","error":"..."}
```

Reload failures leave the previous valid configuration active.

### Download failures

```json
{"level":"warn","msg":"download failed","component":"downloader","feed":"...","error":"HTTP 403 Forbidden"}
```

Repeated download failures for the same feed indicate an upstream problem. The daemon retries with exponential backoff.

### Processing failures

```json
{"level":"error","msg":"processing failed","component":"engine","feed":"...","error":"parse_failed: ..."}
```

Processing failures are severe runtime faults. Check the exception class in the error message.

### Integrity findings

```json
{"level":"warn","msg":"integrity issue found","component":"integrity","feed":"...","reason":"missing secondary output"}
```

Integrity findings appear after processing settles. Transient findings during active processing are expected and self-correct.

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
journalctl -u update-ipsets | grep '"level":"error"'

# Show errors for a specific feed
journalctl -u update-ipsets | grep '"level":"error"' | grep '"feed":"tor_exits"'
```

Or redirect stderr to a file and parse with `jq`:

```bash
update-ipsets daemon --config /opt/update-ipsets/etc/config 2>daemon.log
cat daemon.log | jq 'select(.level=="error")'
```
