# Runtime Status

You will learn what the admin heartbeat shows, what is available through the admin status API, and how to compare snapshots to find resource waste.

## Heartbeat strip

The top heartbeat strip answers "is anything on fire?" without opening a detail view.

The first row shows:

- **Daemon** — `RUNNING` while an engine run is active, otherwise `Idle`.
- **Healthy, delayed, risky, unavailable, archived, empty, unmaintained** — feed-health counts. These tiles are clickable and filter the feed table.

The second row shows compact runtime vitals:

- **Uptime** — daemon uptime and configured feed count.
- **Config** — successful reload count or the latest reload error.
- **Heap** — Go heap allocation and total Go runtime memory.
- **Goroutines** — current goroutine count and GC count.
- **Disk free** — free space on the runtime data volume.
- **Integrity** — current pipeline-integrity finding count, with startup repair status when repairs were deferred.

The four live queues are shown in the current-run panel below the heartbeat. See [Live Queues](live-queues.md).

## Admin status API

The admin UI polls `GET /api/v1/admin/status`. The response includes more fields than the heartbeat renders.

Important top-level blocks:

| Block | Meaning |
|---|---|
| `system` | Go runtime, process, disk, CPU, I/O, and file-descriptor snapshots. |
| `engine` | Engine running state, last run, current phase, background tasks, config reload state, and lifetime counters/timings. |
| `scheduler` | Scheduler item snapshot: enabled state, next due time, last check, frequency, failures, and scheduler detail per item. |
| `queues` | Waiting, active, deferred, and pending queue items. |
| `metrics` | Scheduler counters and latest batch timings. |
| `feeds` | Feed health and visibility summary used by the heartbeat. |
| `artifacts` | Artifact parent state shown by the artifact inventory. |

## Telemetry counters and timings

The daemon exposes monotonic operation counters that increment over the process lifetime:

- **Download counts** — scheduler download counters under `metrics`, plus downloader status counters in `engine.lifetime_metrics.counters`.
- **Processing counts** — scheduler processing counters under `metrics`, engine phases under `engine.current_metrics` and `engine.last_metrics`, and operation timings in `engine.lifetime_metrics.operations`.
- **Pairwise comparison** — candidate pairs checked, overlap computations, skip reasons.
- **iprange operations** — loads, saves, merges, compares, diffs, searches (text and binary).
- **Entity work** — sidecar reads/writes, country/ASN patch counts, range attribution operations.
- **Admin polling** — request counts, response bytes, build timings for admin API endpoints.

These counters are cumulative. To see what happened in the last hour, sample the status endpoint twice and compute the delta.

## Process resources

The heartbeat shows the compact resource fields operators need most often: heap, goroutines, disk free, and integrity. The admin status API also exposes process-level resource snapshots under `system`:

- **CPU** — user, system, and total seconds since start.
- **Memory** — Go heap and OS process memory such as `rss_kb`, `vms_kb`, and `data_kb`.
- **File descriptors** — open FD count.
- **I/O** — process read/write byte counters and syscall counters.

## Snapshot-diff workflow

To identify what consumed resources between two points in time:

1. Record the status endpoint output at time A.
2. Record the status endpoint output at time B.
3. Compare counter deltas against elapsed time and CPU deltas.

This tells you which operations moved the most. For example, if `iprange.compare.ops` jumped by 50,000 in five minutes, pairwise comparisons dominated that period.

The counters use stable names that work across daemon restarts and OpenTelemetry export.

## See also

- [Live Queues](live-queues.md) — the four queue panels below the status strip
- [Background Work](background-work.md) — deferred work not shown in the queues
- [Telemetry Reference](../monitoring/telemetry-reference.md) — field and metric names
