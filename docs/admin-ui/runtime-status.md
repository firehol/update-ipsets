# Runtime Status

You will learn what the top panel of the admin UI shows, how to read telemetry counters, and how to compare snapshots to find resource waste.

## Top panel

The status panel shows:

- **Service uptime** — how long the daemon has been running.
- **Current pipeline state** — whether the downloader and processing loops are active or idle.
- **Active work summary** — counts of items in each of the four live queues (waiting to download, downloading, waiting to process, processing).

## Telemetry counters

The daemon exposes monotonic operation counters that increment over the process lifetime:

- **Download counts** — total downloads, HTTP status distribution, transferred bytes.
- **Processing counts** — feeds processed, processing phases completed, operation timings.
- **Pairwise comparison** — candidate pairs checked, overlap computations, skip reasons.
- **iprange operations** — loads, saves, merges, compares, diffs, searches (text and binary).
- **Entity work** — sidecar reads/writes, country/ASN patch counts, range attribution operations.
- **Admin polling** — request counts, response bytes, build timings for admin API endpoints.

These counters are cumulative. To see what happened in the last hour, sample the status endpoint twice and compute the delta.

## Process metrics

The panel shows process-level resource usage:

- **CPU** — user, system, and total seconds since start.
- **Memory** — current memory size and resident set.
- **File descriptors** — open FD count.

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
