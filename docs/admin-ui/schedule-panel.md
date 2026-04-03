# Schedule Panel

You will learn how to read the schedule view, what trigger reasons mean, and how different feed families schedule their next run.

## What the schedule shows

The schedule panel lists items with their expected next run time and the reason they will run.

| Field | Meaning |
|---|---|
| **Item** | Feed or artifact parent name. |
| **Next run** | When the next automatic action is expected. |
| **Trigger reason** | Why the item will run. |

## Trigger reasons

| Reason | When you see it | What it means |
|---|---|---|
| **Cadence-driven** | Merges, plain feeds, artifact parents, provider databases | The item runs on its own configured time interval. |
| **Input-triggered** | History derivatives, merges after input changes | The item runs because its parent or input feed updated. |
| **Manual** | After operator recheck or reprocess | An operator explicitly requested this run. |
| **Retry** | After a failed download | The item is retrying after a hard failure, using exponential backoff. |
| **Recovery** | After integrity check | The item was queued to repair missing or stale outputs. |

## How feed families schedule

### Plain feeds

Run on their configured cadence. After a successful check, the next run is scheduled at `now + cadence`. After a failure, the next retry uses exponential backoff.

### Merges

Cadence-driven. Merges are the only synthetic feed family that progresses purely because time passed. They recompose from their inputs on each cadence tick.

### History derivatives

Input-triggered. They run when their parent feed updates. They do not have their own cadence. If the parent does not update, the derivative does not run.

### Artifact parents

Run on their own cadence, independently of their child feeds.

### Provider databases

Run on their own cadence. When a provider updates successfully, a reprocess wave is triggered for all feeds that depend on that provider.

## See also

- [Triggers and Reprocessing](../pipeline/triggers-reprocessing.md) — what causes work to happen
- [Pipeline Overview](../pipeline/pipeline-overview.md) — the two-loop model
