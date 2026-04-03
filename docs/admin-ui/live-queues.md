# Live Queues

You will learn what the four queue panels show, how to read queue age, and why the processing queue only contains feeds.

## Four panels

The admin UI shows four live-list panels at the top:

| Panel | Contents |
|---|---|
| **Waiting to download** | Items due for downloader-stage work that have not started yet. |
| **Downloading** | Items currently being fetched or composed by downloader workers. |
| **Waiting to process** | Feeds with staged work admitted for processing, waiting for a worker slot. |
| **Processing** | Feeds currently being analyzed and published by processing workers. |

Each panel is a fixed-height scrollable list sized for about four visible rows. The list scrolls for overflow. Auto-refresh does not cause the rest of the page to jump.

## Download queues

The download queues may contain:

- Normal feeds waiting for upstream fetch.
- Artifact parents waiting for artifact download.
- Provider databases waiting for provider refresh.

## Processing queue

The processing queue is feed-only. You will not see artifact parents or provider databases here. Only public feeds appear in the processing panels.

If you see a feed in the processing queue, it was admitted by one of:

- Downloader-stage outcome (new content staged).
- Provider-update reprocess wave.
- Restart recovery of a durable staged body.
- Admin-triggered reprocess.
- Integrity-triggered local repair.

## Queue age

For items in the "waiting to process" panel, the UI shows queue age — how long the item has been waiting.

Queue age measures total time since the item first entered the waiting queue for its current staged body. If processing fails and the same body is requeued for retry, the queue age continues from the original first admission. It does not reset on retry.

## Status subtitles

Each queue item shows an operator-facing status label, not a raw status code. The labels match the same meaning contract used in the feed inventory.

For example, you see "Downloading" or "Staging new content" rather than implementation codes like `parse_failed`.

## See also

- [Pipeline Overview](../pipeline/pipeline-overview.md) — how the four queues fit into the two-loop model
- [Background Work](background-work.md) — work that happens outside these queues
