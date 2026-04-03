# Background Work

You will learn what background work the daemon does outside the four live queues, how to see its progress, and how to tell idle from broken.

## What is background work

Some daemon work does not belong to the downloader or processing queues. It runs in a separate background-maintenance pool. The admin UI shows this work in its own panel.

Examples:

- **Startup repairs** — fixing missing or stale artifacts discovered during startup integrity checks.
- **Entity refreshes** — updating country and ASN detail pages after feed updates or health transitions.
- **Health transitions** — artifact refreshes triggered when a feed changes health class.
- **Config-reload rebuilds** — rebuilding artifacts after a successful configuration reload.
- **Full entity rebuilds** — operator-requested rebuild of all country and ASN artifacts from scratch.

## What you see

For each active background task:

| Field | Meaning |
|---|---|
| **Task name** | What is being done. |
| **Trigger** | What started this task (startup, feed update, health transition, operator action, config reload). |
| **Current stage** | Where in the task lifecycle it is right now. |
| **Started at** | When the task began. |
| **Progress** | How far along it is, when meaningful. |

## Shown even when idle

The background-work panel is visible even when no background tasks are running. This lets you distinguish three states:

- **Active** — tasks are running, you see progress.
- **Idle** — the panel is visible but empty. Nothing is pending.
- **Missing** — if the panel were hidden, you could not tell idle from broken.

## Coalescing

Repeated background requests for the same target are coalesced. If the same feed needs an entity refresh while one is already queued or running, the daemon does not create a second serial task. It deduplicates by feed name.

## Serialization

Entity artifact writes serialize even when the background worker limit is greater than one. Multiple background tasks may be admitted, but country/ASN artifact publication happens one at a time. This prevents two writers from publishing overlapping entity artifacts.

## See also

- [Live Queues](live-queues.md) — the four main queue panels
- [Runtime Status](runtime-status.md) — overall daemon health and telemetry
