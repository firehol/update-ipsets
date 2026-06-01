# Operator Actions

You will learn the manual actions available in the admin UI, when to use each one, and what happens when you trigger them.

## Feed-level actions

### Recheck

Fetch the feed now and process it, even if the content has not changed.

Use recheck when:

- A feed shows as failed and you want to retry immediately.
- You know upstream changed but the daemon has not picked it up yet.
- A feed was archived and you want to test whether it is alive again.

For different feed families, recheck means:

| Family | What recheck does |
|---|---|
| Plain feed | Fetches upstream, reparses, restages. |
| History derivative | Recomputes from current parent body + retained snapshots. If local derivative inputs are not available, the action falls back to the parent. |
| Merge | Recomposes from current local input bodies. |
| Artifact child | Uses existing local materialized input when available. If no child input exists, the action targets the parent artifact. |
| Provider database | Fetches the provider dataset. May trigger a broad reprocess wave. |

If a child feed lacks local input, the recheck automatically targets the parent artifact instead of failing the child.

### Reprocess

Re-run processing from existing local data. No upstream fetch happens.

Use reprocess when:

- Published outputs are missing or stale, but the feed body is current.
- A provider update changed enrichment parameters and you want to regenerate artifacts.
- You edited configuration that affects processing but not download.

Body priority for reprocess:

1. `.processing` file if one exists (from a previous failed run).
2. `.new` file if one is staged.
3. Committed feed body.

If no local body exists at all, reprocess fails with a clear error.

### There is no third action

The admin UI offers exactly two feed-level actions: recheck and reprocess. There is no separate "run now" or "refresh" that does something different. If a button says "run now", it maps to one of these two.

## Batch-level actions

### Run due

Trigger all currently-due work immediately. This is equivalent to telling the scheduler "evaluate cadence now and enqueue everything that is overdue."

Use this when:

- The daemon was paused or network was down and you want to catch up.
- You just enabled several feeds and want them checked now instead of waiting for the next cadence cycle.

### Broad reprocess

Force reprocess every eligible feed that has local staged or committed input. This is a heavy operation.

Use this when:

- A major provider update requires regenerating all enrichment artifacts.
- You suspect widespread output corruption.

The UI requires confirmation before running a broad reprocess because it affects every feed in the catalog.

## Operator API

The admin UI uses authenticated admin API endpoints for the same actions.
These are useful for operational automation and incident runbooks:

| Endpoint | Purpose |
|---|---|
| `GET /api/v1/admin/feeds` | List feeds with current admin state. |
| `GET /api/v1/admin/feeds/{name}` | Return one feed detail record. |
| `GET /api/v1/admin/feeds/{name}/manifest` | Return expected and actual local files for one feed. |
| `POST /api/v1/admin/feeds/{name}/recheck` | Queue a feed recheck. Artifact children may resolve to their parent artifact. |
| `POST /api/v1/admin/feeds/{name}/reprocess` | Queue local-only reprocessing for one feed. Returns conflict if no local input exists. |
| `POST /api/v1/admin/feeds/{name}/enable` | Enable one feed. |
| `POST /api/v1/admin/feeds/{name}/disable` | Disable one feed. |
| `POST /api/v1/admin/run` | Run currently due work now. |
| `POST /api/v1/admin/run?reprocess=true` | Queue broad reprocess for eligible feeds. |

There is no global recheck endpoint. Use feed-level recheck or run currently
due work instead. Action endpoints require `POST`; `GET` returns method not
allowed.

## See also

- [Enable and Disable](enable-disable.md) — enabling and disabling feeds
- [Triggers and Reprocessing](../pipeline/triggers-reprocessing.md) — all automatic and manual triggers
