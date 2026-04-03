# Processing Lifecycle

You will learn what happens after the downloader stages a feed body, how the processing engine produces published artifacts, and how heavy phases and background work fit in.

## Steps

When the processing loop wakes, it follows this sequence:

1. **Claim staged body** — rename a `.new` file to `.processing` for each feed in the batch.
2. **Feed-local processing** — analyze the canonical feed body for per-feed state.
3. **Heavy phases** — run global enrichment and comparison across all relevant feeds.
4. **Commit** — atomically promote `.processing` files to committed feed bodies and publish all artifacts.

## Feed-local processing

For each admitted feed, the engine produces:

- **Metadata** — size, unique IP count, IP family, change rate.
- **History** — bounded point-in-time snapshots of feed size over time.
- **Retention** — how long IPs have been listed, how long removed IPs had stayed.
- **Change rate** — rotation percentage, update frequency measurements.
- **Provider enrichment** — ASN distribution, geographic distribution, bogon overlap.

The engine reads only local canonical feed bodies and local provider databases. It never fetches upstream.

## Heavy phases

After feed-local work completes for the batch, the engine runs global phases:

| Phase | What it does |
|---|---|
| **Pairwise comparison** | Compares each updated feed against every other enabled public feed. Updates overlap counts on both sides. |
| **GeoIP fan-out** | Updates geographic enrichment for affected feeds using the current GeoIP provider. |
| **ASN fan-out** | Updates ASN enrichment for affected feeds using the current ASN provider. |
| **Bogon analysis** | Checks affected feeds against bogon reference data. |
| **Critical infrastructure** | Generates overlap artifacts for critical-infrastructure reference feeds. |
| **Insights** | Produces deterministic insights from all computed facts. |

Heavy-phase concurrency is independently configurable. The engine stops admitting new heavy work during shutdown and waits for in-flight workers to settle.

## Commit

The commit step:

1. Writes all public artifacts (metadata, history, comparisons, enrichment, insights).
2. Promotes each `.processing` body to the committed feed body.
3. Sets correct mtimes on published files for pipeline integrity.

If any step fails before promotion, the previous committed outputs remain authoritative. The staged `.processing` body stays on disk for retry.

## Background work

After the main batch commits, some work runs in the background:

- **Entity artifact patching** — country and ASN detail pages update incrementally.
- **Entity sidecar generation** — per-feed entity sidecars are precomputed during processing, then consumed by the background patcher.

Background work is visible in the admin UI. It does not block the next processing cycle.

## Processing order within a batch

Within one batch, feeds are processed in this order:

1. Normal feeds (plain sources, artifact children)
2. History derivatives
3. Merges (ordered by increasing dependency count)

This ensures deterministic publication order. The engine does not compose history derivatives or merges — that is the downloader's job.

## See also

- [Pipeline Overview](pipeline-overview.md) — how processing fits into the full pipeline
- [Download Lifecycle](download-lifecycle.md) — what happens before processing
- [Triggers and Reprocessing](triggers-reprocessing.md) — what causes processing to run
