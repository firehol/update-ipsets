# Artifact Inventory

You will learn how artifact parents appear in the admin UI, what information is available for each, and how to manage them separately from feeds.

## Why a separate inventory

Artifact parents are upstream artifacts that produce one or more child feeds. They are not public feeds themselves. They appear in the downloader queues but never in the processing queue.

The artifact inventory gives you a dedicated view of these parents without mixing them into the feed table.

## What you see

For each artifact parent:

| Field | Meaning |
|---|---|
| **Name** | Artifact parent identifier from configuration. |
| **Enable state** | Whether the parent is enabled or disabled. |
| **Last check** | When the daemon last attempted to refresh this artifact. |
| **Last change** | When the artifact content last changed. |
| **Next scheduled check** | When the next automatic refresh is expected. |
| **Family / type** | What kind of artifact this is (e.g. DNSBL-style downloader). |
| **Child feeds** | The feeds that are materialized from this parent. |

## Actions

| Action | What it does |
|---|---|
| **Enable** | Start refreshing the parent on its cadence. Children become eligible for processing. |
| **Disable** | Stop refreshing the parent. Children stop receiving new input. Existing children remain individually enabled but are operationally disabled. |
| **Recheck** | Force a fresh download of the parent artifact now. |

## Relationship to child feeds

- Disabling a parent operationally disables all its children, even if the children are individually marked as enabled.
- A child recheck that has no local materialized input resolves to a parent recheck automatically.
- Children do not control whether the parent is enabled.

## See also

- [Feed Inventory](feed-inventory.md) — the main feed table
- [Enable and Disable](enable-disable.md) — how enable/disable works across the system
