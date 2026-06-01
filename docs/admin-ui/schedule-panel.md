# Schedule State

You will learn how to read schedule state in the admin UI and how different feed families become due.

## Where schedule state appears

The current admin UI does not have a separate full-page schedule panel. Schedule state appears in three places:

| Surface | What it shows |
|---|---|
| Feed table | Each feed's next check label and scheduler-state text. |
| Feed detail drawer | Scheduled frequency, observed update cadence, next check, scheduler state, and retry/backoff count. |
| Artifact inventory | Each artifact parent's next check and scheduler detail. |

The admin API also exposes `GET /api/v1/admin/schedule` for automation. It returns schedule rows with item name, kind, enable state, next due time, last check, configured frequency, failure count, and detail text.

## Scheduler state text

The scheduler detail is an operator-facing explanation, not an internal code. Common values include:

| Text pattern | Meaning |
|---|---|
| `never checked` | No successful check has been recorded yet. The item is due. |
| `due now` | The next check time has passed. |
| `next check in ...` | The item is not due yet. The text includes the base configured cadence. |
| `retry due now` | A failing item is ready for another retry. |
| `retry in ... after ... hard failures` | The scheduler is backing off after repeated failures. |
| `archived (automatic retries disabled)` | Archived feeds no longer run automatically. |
| `triggered by inputs` | A derivative runs when its parent or inputs update, not on a fixed wall-clock cadence. |
| `static source (never expires)` | A static source has no independent expiry after its first materialization. |

## How feed families schedule

### Plain feeds

Run on their configured cadence. After a check, the next due time is based on the last checked time plus the configured cadence. After a hard failure, the retry delay grows with the failure count.

### Merges

Expanded merge feeds are first-class sources. If they have no explicit frequency, they use the runtime processing interval. They recompose from currently eligible local inputs when they run.

### History derivatives

Input-triggered. They run when their parent feed updates. They do not have their own cadence. If the parent does not update, the derivative does not run.

### Artifact parents

Run on their own cadence, independently of their child feeds.

### Provider databases

Run on their own cadence. When a provider updates successfully, a reprocess wave is triggered for all feeds that depend on that provider.

## Manual actions

Manual actions appear as run reasons in queue and feed-detail views:

- **Run due work now** evaluates the current schedule and enqueues currently due items.
- **Recheck** targets one feed or artifact parent and forces downloader-stage work.
- **Reprocess** targets one feed, or all eligible feeds when using broad reprocess, and uses existing local input.
- **Integrity recovery** queues the recheck/reprocess plan produced by the integrity checker.

## See also

- [Triggers and Reprocessing](../pipeline/triggers-reprocessing.md) — what causes work to happen
- [Pipeline Overview](../pipeline/pipeline-overview.md) — the two-loop model
