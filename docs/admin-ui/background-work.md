# Background Work

You will learn what background work the daemon does outside the four live feed queues, how to see its progress, and how to tell idle from broken.

## What is background work

Some daemon work does not belong to the downloader or processing queues. It is
admitted through the engine lane and may then use bounded background workers
inside the admitted job. The admin UI shows this work in the **Background
Work** section of the runtime and queues panel.

Examples:

- **Entity refreshes after feed updates** — updating country and ASN detail pages after public feed outputs change.
- **Entity refreshes after health transitions** — updating entity pages when feed health affects published country or ASN facts.
- **Integrity refreshes and recovery admission** — checking cached integrity
  state or queueing recovery from the last settled findings.
- **Startup entity checks and repairs** — rebuilding or repairing country and ASN artifacts when startup detects missing or stale entity artifacts.
- **Config-reload entity checks** — checking country and ASN artifacts again after a successful configuration reload.
- **Operator-requested entity work** — full or selected country and ASN artifact rebuilds from the admin integrity surfaces.

Feed-output integrity recovery is different from entity artifact publishing:
the engine lane admits the recovery request, then the scheduler queues the
resulting recheck or reprocess work in the normal feed queues. Entity artifact
recovery itself remains engine-lane/background work.

DroneBL staged-artifact recovery is not engine-lane background work. It appears
as recovered artifact work in the downloader queue, and a downloader worker
materializes DroneBL child inputs before normal processing.

## What you see

For each active background task:

| Field | Meaning |
|---|---|
| **Task name** | What is being done. |
| **Trigger** | What started this task (startup, feed update, health transition, operator action, config reload). |
| **Current stage** | Where in the task lifecycle it is right now. |
| **Started at** | When the task began. |
| **Progress** | How far along it is, when meaningful. |
| **Lane count** | How many top-level engine-lane jobs are active compared with the configured engine-lane limit. |
| **Worker count** | For admitted background work, how many internal background workers are active compared with the configured background-worker limit. |

The same section can also show pending coalesced entity work, such as feed updates or health transitions waiting behind an already-running background task.

## Shown even when idle

The background-work panel is visible even when no background tasks are running. This lets you distinguish three states:

- **Active** — tasks are running, you see progress.
- **Idle** — the panel is visible but empty. Nothing is pending.
- **Missing** — if the panel were hidden, you could not tell idle from broken.

## Coalescing

Repeated requests for the same maintenance target are coalesced. If the same
feed needs an entity refresh while one is already queued or running, the daemon
does not create a second serial task for the same feed. Full entity rebuild
requests and integrity refresh/reprocess requests are also coalesced while
equivalent work is already pending or running.

## Serialization

Entity artifact writes serialize even when the engine-lane or background worker
limit is greater than one. Multiple jobs may be admitted, but country/ASN
artifact publication happens one at a time. This prevents two writers from
publishing overlapping entity artifacts.

## See also

- [Live Queues](live-queues.md) — the four main queue panels
- [Runtime Status](runtime-status.md) — overall daemon health and telemetry
