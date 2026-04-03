# Pipeline Overview

You will learn how the two-loop pipeline works, what each loop owns, and how work flows from upstream sources to published artifacts.

## The two-loop model

The daemon runs two independent loops:

1. **Downloader loop** — fetches upstream sources and composes local feed bodies. It decides what needs processing.
2. **Processing loop** — consumes already-staged local feed bodies and produces published artifacts.

The scheduler coordinates both loops but keeps them independent. The downloader decides what to fetch and when. The processing engine decides how to analyze and publish.

## Flow diagram

```
upstream sources
       |
       v
+-------------------+
| downloader loop   |  <-- cadence, retries, manual recheck
|   fetch / compose |
+-------------------+
       |
       v
  staged .new files   (durable on disk)
       |
       v
+-------------------+
| processing loop   |  <-- batch execution
|   analyze / publish|
+-------------------+
       |
       v
  published artifacts (website, mirrors, API)
```

Work moves strictly left to right. The processing loop never fetches upstream. The downloader loop never publishes artifacts.

## Four concurrency domains

The daemon controls four independent concurrency limits:

| Domain | What it controls | Configurable |
|---|---|---|
| **Download** | concurrent upstream fetches and local compositions | yes |
| **Processing** | concurrent feed-local analysis runs | yes |
| **Heavy phase** | global enrichment after feed-local work (comparisons, GeoIP, ASN, bogon, insights) | yes, independent of processing |
| **Background** | deferred maintenance (entity patching, startup repairs, health transitions) | yes, defaults to single-threaded |

Separate limits prevent one workload from starving another. A slow download does not block processing. A heavy comparison pass does not block the next download cycle.

## What triggers each loop

- **Downloader loop** wakes on: cadence timers, manual recheck, `run due` action, retry backoff.
- **Processing loop** wakes on: new staged work admitted by the downloader, manual reprocess, provider-database updates, restart recovery.

Neither loop wakes on public page views. Public pages serve precomputed artifacts.

## The handoff

The downloader writes a complete canonical feed body to a `.new` file on disk. The processing loop claims that file, renames it to `.processing`, and produces outputs. On success, the `.processing` body is promoted to the committed feed body.

If the daemon crashes between stages, `.new` and `.processing` files survive restart. The processing loop recovers them on the next start.

## See also

- [Download Lifecycle](download-lifecycle.md) — detailed steps inside the downloader loop
- [Processing Lifecycle](processing-lifecycle.md) — detailed steps inside the processing loop
- [Triggers and Reprocessing](triggers-reprocessing.md) — what causes work to happen
