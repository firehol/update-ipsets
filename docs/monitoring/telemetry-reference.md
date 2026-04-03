# Telemetry Reference

You will learn every metric and counter that update-ipsets exposes through the admin API and OpenTelemetry.

## iprange namespace

These counters track IP set primitive operations. They appear in the admin status response under `counters.iprange` and as OpenTelemetry metrics with the `iprange.` prefix.

| Metric | Description |
|--------|-------------|
| `iprange.load.text` | Number of times a text-format set was loaded from disk |
| `iprange.load.binary` | Number of times a binary FileSet was loaded (mmap or pread) |
| `iprange.save.text` | Number of text-format set writes |
| `iprange.save.binary` | Number of binary FileSet writes |
| `iprange.merge.ops` | Union/merge operations between sets |
| `iprange.compare.ops` | Pairwise comparison operations |
| `iprange.diff.ops` | Diff operations (added/removed IPs) |
| `iprange.binary.searches` | Binary-search lookups into FileSet files |

Additional operation counters follow the pattern `iprange.<operation>.ops` for intersect, exclude, union, count-unique, optimize, and contains.

## Download namespace

These counters track the downloader loop. They appear under `counters.download`.

| Metric | Description |
|--------|-------------|
| `download.queued` | Items admitted to the downloader queue |
| `download.ok` | Downloads that produced new or changed content |
| `download.failed` | Downloads that failed (HTTP error, timeout, DNS, etc.) |
| `download.not_modified` | Upstream confirmed no change (304 or equivalent) |
| `download.same` | Content downloaded but semantically identical to current |
| `download.empty` | Download succeeded but produced an empty set |
| `download.disabled` | Items skipped because they are disabled |
| `download.materializing` | Artifact parents actively materializing child data |

## Engine namespace

These counters track the processing engine. They appear under `counters.engine`.

| Metric | Description |
|--------|-------------|
| `engine.queued` | Feeds admitted to the processing queue |
| `engine.updated` | Feeds successfully processed with new output |
| `engine.skipped` | Feeds skipped (already current, disabled, etc.) |
| `engine.failed` | Processing failures |
| `engine.cancelled` | Runs cancelled by shutdown or context timeout |

Phase-specific counters follow the pattern `engine.<phase>` for each processing stage:

| Phase | Description |
|-------|-------------|
| `engine.claim` | Claiming staged feed bodies for processing |
| `engine.analyze` | Analyzing the canonical feed body |
| `engine.history` | Updating history and retention artifacts |
| `engine.enrich` | Provider enrichment (ASN, GEO, bogon) |
| `engine.compare` | Pairwise comparison updates |
| `engine.insights` | Deterministic insight generation |
| `engine.publish` | Writing public artifacts to disk |
| `engine.finalize` | Promoting processing to committed state |

## Pairwise comparison counters

These track the cost of catalog-wide feed comparison.

| Metric | Description |
|--------|-------------|
| `compare.candidates` | Candidate feed pairs evaluated |
| `compare.overlap_checks` | Actual overlap computations performed |
| `compare.skipped` | Pairs skipped (no overlap expected, same feed, etc.) |

## Entity artifact counters

These track country/ASN entity artifact work.

| Metric | Description |
|--------|-------------|
| `entity.sidecar.read` | Feed sidecar reads |
| `entity.sidecar.write` | Feed sidecar writes |
| `entity.pending.read` | Pending sidecar reads |
| `entity.pending.write` | Pending sidecar writes |
| `entity.countries.affected` | Countries touched in a refresh batch |
| `entity.asns.affected` | ASNs touched in a refresh batch |

## Process resource counters

These appear under `process` in the admin status response. Values are snapshots, not monotonic counters.

| Field | Description |
|-------|-------------|
| `process.cpu_user_ms` | User-mode CPU time in milliseconds |
| `process.cpu_system_ms` | Kernel-mode CPU time in milliseconds |
| `process.memory_rss_bytes` | Resident set size in bytes |
| `process.memory_vms_bytes` | Virtual memory size in bytes |
| `process.fd_count` | Open file descriptors |
| `process.io_read_bytes` | Bytes read from disk |
| `process.io_write_bytes` | Bytes written to disk |

## Using these counters

All counters are monotonic — they increase over the daemon's lifetime. To compute rates, sample twice and divide the delta by the elapsed seconds:

```
rate = (counter_t2 - counter_t1) / (t2 - t1)
```

The admin status endpoint returns these values in JSON. OpenTelemetry exports them as cumulative counters.
