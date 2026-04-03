# Recovery Model

You will learn the two recovery classes, how the daemon chooses between them, and why recovery targets the root cause instead of the symptom.

## Two recovery classes

Every integrity finding maps to one of two recovery actions:

### Recheck

The daemon needs fresh or staged input before it can repair the feed.

Use recheck when:

- the committed input is missing for a downloadable feed (fetch upstream again)
- the child input is missing for an artifact-backed child and the parent must be refreshed first
- the history snapshots needed to rebuild a derivative are missing (target the parent, not the derivative)
- an artifact-backed child has never been materialized locally (target the parent artifact recheck)

Recheck re-acquires upstream data or refreshes parent artifacts before local processing.

### Reprocess

The daemon already has enough local input to regenerate the missing outputs without a new upstream fetch.

Use reprocess when:

- public outputs are missing but the feed already has committed local input
- an output-only repair is needed after local artifact loss
- a secondary is stale or malformed but the primary data is intact

Reprocess is a local-only operation. The downloader has already done its job. The problem is in the engine-owned outputs.

## Root cause targeting

Integrity recovery targets the root cause, not just the visible symptom.

Example: a derivative artifact (like a history summary) is stale. The immediate cause is the stale derivative, but the root cause is that the parent feed's history snapshots are missing or corrupt. Recovery queues a parent `recheck`, not a derivative `reprocess`.

This avoids repeated repair cycles where the daemon regenerates a derivative from incomplete parent data, only to find it stale again on the next check.

Another example: a merge feed has missing outputs because one of its parents lacks a local body. Recovery queues the parent for recheck. Once the parent has fresh data, the merge can reprocess correctly.

## What happens after recovery is queued

1. The daemon adds the target to the scheduler queue.
2. The scheduler runs the recheck or reprocess during the next cycle.
3. After the repair settles, the next integrity check confirms the finding is resolved.
