# Running Integrity Checks

You will learn when integrity checks run automatically, how to trigger a manual check, and how to read the results.

## Automatic checks

The daemon runs integrity checks after each processing cycle settles. This catches local drift that happens between runs — for example, if an external process modified or deleted published artifacts.

During an active processing run, integrity reports `in progress` instead of evaluating files that are still being written.

## Manual checks

Trigger a manual check from the admin UI integrity panel. Click **Re-check** to
start a fresh evaluation of all settled local state.

The page itself is cache-first. Loading or refreshing the admin page returns
the last settled integrity snapshot when one exists. If the cache is cold or
stale, the daemon queues an integrity refresh in the engine lane and reports
`in_progress` instead of scanning the artifact tree inside the HTTP request.

### Include archived feeds

By default, integrity checks skip archived feeds. Toggle the "include archived" option to evaluate them as well. This is useful when you are verifying the complete published state before a migration or backup.

## Operator API

The integrity panel is backed by authenticated admin API endpoints:

| Endpoint | Purpose |
|---|---|
| `GET /api/v1/admin/integrity` | Return cached feed-output integrity findings, or queue a refresh and report `in_progress` when the cache is cold or stale. |
| `GET /api/v1/admin/integrity?include_archived=true` | Use the archived-inclusive cache scope. |
| `POST /api/v1/admin/integrity/refresh` | Queue a fresh feed-output integrity refresh in the engine lane. |
| `POST /api/v1/admin/integrity/refresh?include_archived=true` | Queue a fresh archived-inclusive integrity refresh. |
| `POST /api/v1/admin/integrity/reprocess` | Queue the recovery plan from fresh cached findings. If the cache is not fresh, queue refresh first and report `in_progress`. |
| `POST /api/v1/admin/integrity/reprocess?include_archived=true` | Queue recovery from fresh archived-inclusive findings. |

Recovery may queue both rechecks and reprocesses. The response separates them
as `recheck_names` and `reprocess_names` so operators can see what was queued.
Refresh and recovery endpoints require `POST`; `GET` returns method not
allowed.

## Reading the results

The integrity panel shows one row per finding. Each row includes:

- **Class** — the finding type: missing primary, missing secondary, stale secondary, malformed secondary, or blocked by merge input.
- **Affected feed** — the feed name or entity artifact involved.
- **Recovery action** — whether the daemon will `recheck` (needs fresh input) or `reprocess` (local-only rebuild).
- **Queued target** — the specific feed or artifact the daemon will actually process to fix the finding.

## Recovery workflow

1. Refresh evaluates all settled outputs and stores the result in the integrity cache.
2. Findings appear in the panel with their recovery plans.
3. Recovery actions are disabled until the cache is fresh.
4. Once the check finishes, click **Recover all** to queue the recovery plan for all current findings.
5. The scheduler runs recovery during the next cycle.
6. After recovery settles, run another check to confirm everything is clean.

## What to expect

On a healthy daemon with no local corruption, the integrity check reports `clean` — no findings.

After a disk issue, partial crash, or manual filesystem intervention, expect findings of the appropriate class. Queue recovery and re-check.
