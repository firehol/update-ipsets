# Artifact Inventory

You will learn how artifact parents appear in the admin UI, what information is available for each, and how to manage them separately from feeds.

## Why a separate inventory

Artifact parents are upstream artifacts that produce one or more child feeds. They are not public feeds themselves. They appear in the downloader queues but never in the processing queue.

The artifact inventory gives you a dedicated view of these parents without mixing them into the feed table.

## What you see

For each artifact parent:

| Field | Meaning |
|---|---|
| **Artifact** | Artifact parent identifier, artifact type, and optional descriptive information. |
| **Status** | Current artifact state, last settled status, problem class, latest error, and failure count. |
| **Last check** | When the daemon last attempted to refresh this artifact. |
| **Next check** | When the next automatic refresh is expected, plus scheduler detail when available. |
| **Children** | Child feeds materialized from this parent. Selecting a child opens that feed's detail drawer. |
| **Actions** | Recheck plus enable or disable. Recheck is unavailable while the artifact is disabled. |

## Actions

| Action | What it does |
|---|---|
| **Enable** | Start refreshing the parent on its cadence. Children become eligible for processing. |
| **Disable** | Stop refreshing the parent. Children stop receiving new input. Existing children remain individually enabled but are operationally disabled. |
| **Recheck** | Force a fresh download of the parent artifact now. |

## Operator API

The artifact inventory is backed by authenticated admin API endpoints:

| Endpoint | Purpose |
|---|---|
| `GET /api/v1/admin/artifacts` | List artifact parents and their current state. |
| `GET /api/v1/admin/artifacts/{name}` | Return one artifact parent row. |
| `POST /api/v1/admin/artifacts/{name}/recheck` | Queue a fresh artifact download. |
| `POST /api/v1/admin/artifacts/{name}/enable` | Enable the artifact parent. |
| `POST /api/v1/admin/artifacts/{name}/disable` | Disable the artifact parent. |

Action endpoints require `POST`. A `GET` request to an action endpoint is rejected.

## Relationship to child feeds

- Disabling a parent operationally disables all its children, even if the children are individually marked as enabled.
- A child recheck that has no local materialized input resolves to a parent recheck automatically.
- Children do not control whether the parent is enabled.

## See also

- [Feed Inventory](feed-inventory.md) — the main feed table
- [Enable and Disable](enable-disable.md) — how enable/disable works across the system
