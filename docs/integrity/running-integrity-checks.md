# Running Integrity Checks

You will learn when integrity checks run automatically, how to trigger a manual check, and how to read the results.

## Automatic checks

The daemon runs integrity checks after each processing cycle settles. This catches local drift that happens between runs — for example, if an external process modified or deleted published artifacts.

During an active processing run, integrity reports `in progress` instead of evaluating files that are still being written.

## Manual checks

Trigger a manual check from the admin UI integrity panel. Click **Re-check** to
start a fresh evaluation of all settled local state.

### Include archived feeds

By default, integrity checks skip archived feeds. Toggle the "include archived" option to evaluate them as well. This is useful when you are verifying the complete published state before a migration or backup.

## Operator API

The integrity panel is backed by authenticated admin API endpoints:

| Endpoint | Purpose |
|---|---|
| `GET /api/v1/admin/integrity` | Run a feed-output integrity check and return the current findings. |
| `GET /api/v1/admin/integrity?include_archived=true` | Include archived feeds in the check. |
| `POST /api/v1/admin/integrity/reprocess` | Queue the recovery plan for the current findings. |
| `POST /api/v1/admin/integrity/reprocess?include_archived=true` | Queue recovery after checking active and archived feeds. |

Recovery may queue both rechecks and reprocesses. The response separates them
as `recheck_names` and `reprocess_names` so operators can see what was queued.
The recovery endpoint requires `POST`; `GET` returns method not allowed.

## Reading the results

The integrity panel shows one row per finding. Each row includes:

- **Class** — the finding type: missing primary, missing secondary, stale secondary, malformed secondary, or blocked by merge input.
- **Affected feed** — the feed name or entity artifact involved.
- **Recovery action** — whether the daemon will `recheck` (needs fresh input) or `reprocess` (local-only rebuild).
- **Queued target** — the specific feed or artifact the daemon will actually process to fix the finding.

## Recovery workflow

1. The check evaluates all settled outputs.
2. Findings appear in the panel with their recovery plans.
3. Recovery actions are disabled until the check completes.
4. Once the check finishes, click **Recover all** to queue the recovery plan for all current findings.
5. The scheduler runs recovery during the next cycle.
6. After recovery settles, run another check to confirm everything is clean.

## What to expect

On a healthy daemon with no local corruption, the integrity check reports `clean` — no findings.

After a disk issue, partial crash, or manual filesystem intervention, expect findings of the appropriate class. Queue recovery and re-check.
