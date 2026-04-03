# D1 Sync Safety TODO

## TL;DR

Purpose: safely re-seed the Go update-ipsets installation from the long-running d1 production data without losing feeds that exist only in the local Go configuration, without preserving stale files produced by earlier Go bugs, and without using slow file-by-file promotion loops.

Costa selected option 2 originally, then refined the live-copy contract: use a staging import from d1, copy the full d1 payload in large batches, keep local-only Go feeds by avoiding live delete passes, and run cleanup/transformations only after all copy phases complete.

## Analysis

Facts already verified:

- The previous `sync-from-d1.sh` rewrite still used per-feed promotion loops for base/web/lib/history data after staging.
- Per-feed rsync promotion is too slow for the d1 dataset and was interrupted mid-run.
- The live install should currently be treated as partially synced because Costa stopped that run after it had already entered live promotion.
- Direct live `--delete` still risks removing local-only Go feeds that are not present on d1 yet.
- Stale local examples found under `/opt/update-ipsets/lib` include:
  - `lib/<feed>/latest.set`
  - `lib/<feed>/new/<epoch>.set`
- Bash production writes retention files as:
  - `lib/<feed>/latest`
  - `lib/<feed>/new/<epoch>`
  - `history/<feed>/<epoch>.set`
- Current Go reads `lib/<feed>/new/*.set` as valid pending retention entries for compatibility, so keeping stale `new/*.set` files can cause old duplicate retention batches to be processed.
- History `.set` files are correct and must not be treated as stale.
- Both local and remote rsync installations support `--compress-choice=zstd`:
  - local: `rsync 3.4.1`
  - d1: `rsync 3.2.7`
- d1 holds more than just active feed output files. Verified examples include:
  - hidden `.lastchecked` files
  - `.cache`, `.cache.*`, `.cache.old`
  - helper scripts like `check.sh` and `find-blacklisted.sh`
  - provider directories like `dbip_country/`
  - the full `web/files/` tree
- The local daemon serves public outputs from `/opt/update-ipsets/web` and `/opt/update-ipsets/web/files` via systemd flags, not from `/opt/update-ipsets/data`.
- The implementation now keeps the sync script under the repo's approximate 500-line file-size rule (`sync-from-d1.sh` is 426 lines).

## Decisions

1. Import strategy: staging import plus selective promotion.
   - Decision: selected by Costa.
   - Implication: production data becomes authoritative only for feeds present on d1.
   - Implication: local-only feeds are preserved.
   - Risk: script must correctly classify production-owned vs local-only feed names.

2. Copy strategy: batch rsync, not per-file or per-feed promotion loops.
   - Decision: selected by Costa.
   - Implication: sync must move whole directory trees or large filtered batches, not thousands of tiny rsync calls.
   - Risk: delete/protect rules must be correct before using bulk `--delete` on live trees.

3. Transport compression: remote copy must use compression, preferably zstd, otherwise gzip.
   - Decision: selected by Costa.
   - Implication: d1 staging import must explicitly negotiate compressed transport.
   - Risk: rsync feature support must be verified on both ends before preferring zstd.

4. Historical scope: copy all production data, including old/obsolete/disabled/not-alive feeds.
   - Decision: selected by Costa.
   - Implication: d1 files are historical product data and must all be preserved locally for future UI work.
   - Risk: the sync must not filter by current config enabled state or current feed liveness.

5. Ordering: cleanup or filename transformations must happen only after all d1 data has been copied.
   - Decision: selected by Costa.
   - Implication: sync pipeline is copy first, then post-copy cleanup/transform.
   - Risk: cleanup logic must operate only after staging and live promotion complete.

## Plan

1. [done] Review `sync-from-d1.sh` and the installed path contract again before editing.
2. [done] Add a pre-sync backup before any live changes.
3. [done] Import d1 files into a staging directory using `rsync --delete` against staging only.
4. [done] Discover production-owned feed names from the staged d1 data.
5. [done] Discover local-only feed names from existing local files/directories.
6. [done] Replace per-feed promotion with batched rsync of the full staged `data/`, `lib/`, and `web/` trees into live directories.
7. [done] Keep remote compression explicit and prefer zstd when supported by both ends.
8. [done] Ensure production-owned scope comes from staged d1 data, not current enabled/live feed logic.
9. [done] Move stale-file cleanup to a dedicated post-copy phase after all bulk copies complete.
10. [done] Remove stale old-Go duplicate files only when the correct unsuffixed twin exists:
   - `lib/<feed>/latest.set` when `lib/<feed>/latest` exists
   - `lib/<feed>/new/<epoch>.set` when `lib/<feed>/new/<epoch>` exists
11. [done] Preserve valid history snapshots:
   - `data/history/<feed>/*.set`
12. [done] Merge d1 bash cache with local-only Go cache entries via the internal `cache-merge` command.
13. [done] Preserve unmanaged local env entries while replacing managed API-key values from d1.
14. [done] Add visible progress reporting for the long-running backup/copy phases.
15. [done] Add audit output through production/local manifests and summary counts.
16. [done] Validate with shell syntax checks and non-destructive local checks.

## Current Live State

- The previous run was interrupted by Costa.
- It completed the pre-sync backup and created staging/manifests under `/opt/update-ipsets/import-d1`.
- It had already entered live promotion before interruption, so the current `/opt/update-ipsets` data should be treated as partially synced until the script is re-run successfully or restored from backup.
- After the corrected rerun, Costa reported a post-sync runtime issue:
  - the service was still stopped because `sync-from-d1.sh` only restarts it if it was running before the sync
  - `http://localhost:18888/ipsets/abuseipdb_1d` was missing country data in the UI
- Investigation findings for the country-data issue:
  - the synced on-disk file existed: `/opt/update-ipsets/web/abuseipdb_1d_geolite2_country.json`
  - the synced file used the legacy bash payload shape: JSON array of `{code,value}` rows
  - the Go API/UI contract expects `{total_mapped, countries:[...]}` for `/api/v1/sets/:name/countries/:provider`
  - the web handler was bypassing engine normalization and serving the raw synced file bytes directly, so the frontend received the wrong shape
  - integrity only checked existence + mtime and did not validate geo payload shape/content
- Investigation findings for the "Integrity check is waiting for the active run to finish" report:
  - live admin integrity currently returns:
    - `status: "in_progress"`
    - `running: true`
    - `last_started: 2026-04-10T21:05:42.815161247Z`
    - `last_ended: zero`
  - this is not just a stale flag:
    - the daemon process is alive and healthy (`systemctl is-active update-ipsets` -> `active`, `/healthz` -> `ok`)
    - CPU stays around 20%
    - `/proc/<pid>/fd` shows active temp files inside retention ledgers, for example:
      - `/opt/update-ipsets/lib/firehol_proxies/new/.tmp-*`
      - `/opt/update-ipsets/lib/dronebl_anonymizers/new/.tmp-*`
    - `/proc/<pid>/io` counters keep increasing over time, proving ongoing disk work
  - the current run is heavy because the imported bash history ledgers are large:
    - `firehol_proxies/new`: `69,538` files, `291M`
    - `dronebl_anonymizers/new`: `122,463` files, `1.5G`
  - the last source-stage logs line up with retention work:
    - `firehol_proxies` wrote `latest`, `new/`, `history.csv`, and `changesets.csv`
    - the later retention outputs (`histogram`, `retention.json`) were not refreshed yet for that feed
    - this places the daemon inside `updateRetention()` / `buildRetentionData()` for large ledgers
  - the bash version performs the same class of work:
    - it scans all `lib/<feed>/new/*` files
    - rewrites affected snapshot files
    - recalculates retention histograms from the full ledger
  - conclusion:
    - the admin message is misleading because it looks like a dead wait
    - but the daemon is still actively processing imported retention history and has not yet returned from `RunOnce()`
- Investigation findings for the retention `.tmp-*` warning spam:
  - `pkg/engine/retention.go` scans every file in `lib/<feed>/new/` and parses filenames as timestamps in both `updateRetention()` and `buildRetentionData()`
  - `internal/fileutil/fileutil.go` writes atomic temp files as `.tmp-*` in that same directory before rename
  - the retention scanner therefore sees its own transient scratch files and emits false malformed-filename warnings
  - this is internal noise, not feed corruption
- Installed fix status:
  - the API now normalizes both legacy bash arrays and native Go country payloads before serving them
  - integrity now validates geo country payloads semantically instead of only checking file presence/mtime
  - `./install.sh` completed successfully and restarted `update-ipsets`
  - live verification after install:
    - `systemctl is-active update-ipsets` -> `active`
    - `curl -fsS http://localhost:18888/healthz` -> `ok`
    - `curl -fsS http://localhost:18888/api/v1/sets/abuseipdb_1d/countries/geolite2_country` now returns `total_mapped=93240` and `174` country rows
    - `curl -fsS http://localhost:18888/api/v1/admin/integrity` -> `count=0`
  - retention temp-file warning fix status:
    - retention scanners now ignore dot-prefixed scratch files in `lib/<feed>/new/` before timestamp parsing
    - targeted regression test passed
    - package test suite `go test ./pkg/engine` passed
    - fresh journal after restart showed no matches for:
      - `retention: skipping malformed`
      - `retention: skipping malformed new-set filename`

## Implementation Notes

- `sync-from-d1.sh` now syncs d1 data into `${INSTALL_DIR}/import-d1` first.
- Staging imports are now full-tree and compressed:
  - `${D1_BASE}/` -> `${STAGE_DATA}/`
  - `${D1_LIB}/` -> `${STAGE_LIB}/`
  - `${D1_WEB}/` -> `${STAGE_WEB}/`
  - `${D1_CONFIG}` -> `${STAGE_CONFIG}`
- Remote staging rsync prefers `--compress-choice=zstd` and falls back to gzip only if `--compress-choice` is unavailable on one side.
- Live promotion is now batched and directory-wide:
  - `${STAGE_DATA}/` -> `${LOCAL_DATA}/`
  - `${STAGE_LIB}/` -> `${LOCAL_LIB}/`
  - `${STAGE_WEB}/` -> `${LOCAL_WEB}/`
- Live promotion no longer uses feed-by-feed or file-by-file rsync loops.
- Live promotion does not use live `--delete`, so local-only Go feeds remain intact.
- Post-copy transformations/cleanup now run only after all staged trees are copied into live directories:
  - merge d1 `.cache` with preserved local-only entries into `.cache.json`
  - extract managed env keys from `${STAGE_CONFIG}`
  - remove stale old-Go duplicate filenames when the correct bash/Go twin exists
- Local-only feed names are written to:
  - `${INSTALL_DIR}/import-d1/manifests/local-only-feeds.txt`
- Feeds seen on d1 are written to:
  - `${INSTALL_DIR}/import-d1/manifests/production-feeds.txt`
- A small internal `cache-merge` subcommand was added so the script can create an active `.cache.json` from d1's bash `.cache` while preserving local-only entries from the previous Go `.cache.json`.
- Country payload compatibility and integrity hardening were added after the d1 sync exposed a live contract mismatch:
  - `pkg/engine/country_payload.go` now accepts both payload shapes for country-comparison JSON:
    - native Go object: `{total_mapped, countries:[...]}`
    - legacy bash array: `[{code,value}, ...]`
  - `pkg/engine/public.go` now loads country payloads through the shared compatibility decoder
  - `pkg/web/server.go` now serves `/api/v1/sets/:name/countries/:provider` through engine normalization instead of raw file passthrough
  - `pkg/engine/integrity.go` now validates geo payload content/shape using the same decoder and flags invalid country payloads as stale findings
  - tests were added for:
    - legacy array acceptance
    - invalid country payload rejection in integrity
    - live country API normalization behavior
- Retention scanner hardening is needed after live logs showed false malformed-filename warnings for `.tmp-*` files created by the atomic writer in `lib/<feed>/new/`.
- Retention scanner hardening is now implemented:
  - `pkg/engine/retention.go` ignores dot-prefixed entries in both `updateRetention()` and `buildRetentionData()`
  - `pkg/engine/retention_test.go` verifies `.tmp-*` files in `lib/<feed>/new/` are ignored silently

## Implied Decisions

- The script should be safer than the current version even if interrupted.
- The daemon should be stopped and confirmed inactive before live promotion.
- The staging directory is safe to delete/recreate because it is import scratch space, not authoritative state.
- Do not delete local-only feed state.

## Testing Requirements

- [done] `bash -n sync-from-d1.sh`
- [done] `go test ./cmd/update-ipsets ./pkg/cache`
- [done] `go test ./...`
- [done] `go build ./...`
- [done] `go vet ./...`
- [done] `git diff --check`
- [done] `sync-from-d1.sh --help`
- [done] `./install.sh`
- [done] live service checks:
  - `systemctl is-active update-ipsets`
  - `curl -fsS http://localhost:18888/healthz`
  - `curl -fsS http://localhost:18888/api/v1/sets/abuseipdb_1d/countries/geolite2_country`
  - `curl -fsS http://localhost:18888/api/v1/admin/integrity`
- [done] Remote zstd rsync dry-run:
  - `rsync -a -z --compress-choice=zstd --dry-run d1:/etc/firehol/update-ipsets.conf ...`
- [done] Local batch rsync dry-run:
  - `rsync -a --dry-run <stage>/ <live>/`
- [not available] `shellcheck` is not installed locally.
- [done] The d1 sync itself was executed successfully after the batch/zstd/full-copy rewrite.
- [done] Add regression coverage that `.tmp-*` files in `lib/<feed>/new/` are ignored without warnings or failures.
- [done] Verify the live journal no longer shows the retention malformed-filename warning after reinstall/restart.

## Documentation Updates Required

- [done] Update this TODO with implementation notes.
- [done] `AGENTS.md` documents the current d1 sync contract and the internal `cache-merge` helper.

---

# Admin Run Reason / Processing Time TODO

## TL;DR

Purpose: make the admin tell the operator exactly why a run is happening now, how long it has been running, and for every feed why it last ran and how long that last run took.

Costa requested three UI changes:

- add a `Currently running` section next to `Failing / degraded` and `Upcoming checks`
- show, for the current run:
  - reason
  - processing time elapsed
- show, for each feed in the table and feed modal:
  - last reason
  - last processing time

This requires a real backend enum of run reasons. The current codebase does not persist such a field yet.

Operator clarification from Costa after reviewing the first implementation attempt:

- `Currently running` must mean the actual feeds in flight, not the global daemon run.
- A feed list is required because feeds do not run strictly one by one.
- The fallback `Last run` panel was not requested and should not remain in that slot.

Operator clarification after live verification:

- the admin now shows `13 stale, 0 running` while a single batch is still active
- because only one batch can exist at a time, `Currently running` must show:
  - the feeds belonging to the current batch
  - the current phase of the batch
- feeds that belong to the active batch must not fall back to `stale`

## Feasibility Verdict

FEASIBLE AS SPECIFIED.

Verified assumptions:

- global current-run status already has a backend carrier:
  - `pkg/engine/engine.go:97` (`StatusSnapshot`)
  - `pkg/engine/query.go:351` (`StatusSnapshot()`)
- per-feed admin rows already come from persisted cache state plus scheduler snapshot:
  - `pkg/web/admin.go:449` (`buildAdminFeeds`)
  - `pkg/web/admin.go:553` (`populateFromCacheAndSchedule`)
- per-feed processing timestamps already persist in cache:
  - `pkg/cache/cache.go:15` (`Entry`)
- scheduler trigger paths are centralized and enumerable:
  - scheduled due run: `pkg/scheduler/scheduler.go:360`
  - admin-triggered feed/global actions: `pkg/scheduler/scheduler.go:377`
  - startup integrity queued rebuilds: `pkg/web/server.go:69`
  - manual integrity reprocess: `pkg/web/integrity.go:115`
  - dependency-triggered derivative runs: `pkg/engine/run.go:149`

No blocker was found. The missing pieces are only data model + plumbing + UI.

## Analysis

Facts already verified:

- There is no persisted run-reason enum today.
  - `pkg/engine/engine.go:97` has no current/last reason fields.
  - `pkg/cache/cache.go:15` has no per-feed last reason or last duration fields.
- There is no persisted processing-duration field today.
  - `pkg/cache/cache.go:15` has timestamps, but no elapsed duration.
- The admin UI already has a panel designed for “what is happening now?” but it only shows:
  - `Failing / degraded`
  - `Upcoming checks`
  - file: `ui/src/components/admin/current-run.tsx`
- The feeds table and feed modal also do not expose last reason / last processing time.
  - files:
    - `ui/src/components/admin/feeds-table.tsx`
    - `ui/src/components/admin/feed-modal.tsx`
- The currently-running state is also incomplete today:
  - `deriveFeedStatus()` understands `running/downloading/processing`
  - file: `pkg/web/admin.go:603`
  - but the engine does not actually persist those in-progress states per feed
  - `pkg/engine/process.go:176` only sets `pending`, then final terminal statuses
- Feed processing does not happen from one path only:
  - plain sources / derivatives go through `processConcreteSource()` and `processAndCommit()`
    - `pkg/engine/process.go:157`
    - `pkg/engine/process.go:278`
  - ASN providers use `processASNDatabases()`
    - `pkg/engine/asn.go:40`
  - GeoIP providers use `processGeoIPDatabases()`
    - `pkg/engine/geoloc.go:19`
- Therefore, if we add last reason / last duration only to `processConcreteSource()`, ASN and GeoIP feeds will silently miss these fields.
- The first implementation attempt exposed only global engine fields in the new panel:
  - `ui/src/components/admin/current-run.tsx` reads `status.engine.current_reason`
  - `ui/src/components/admin/current-run.tsx` reads `status.engine.last_reason`
  - it does not receive or render an in-flight feed list
- The engine already runs feeds concurrently through a worker pool, so a single-feed interpretation is wrong:
  - `pkg/engine/run.go:92` limits worker concurrency with `MaxProcessingWorkers`
  - default is `2` at `pkg/config/config.go:435`
- The backend currently has no active-feed snapshot structure.
 - The backend currently has no batch-level feed snapshot or batch phase.
  - `pkg/engine/engine.go:105` exposes only global run fields in `StatusSnapshot`
  - `pkg/engine/query.go:351` returns no list of active feeds
 - The first fix added `active_feeds`, but this only tracks currently executing feed attempts.
   - once the batch moves into global post-feed work (`geoip`, `asn`, publish, etc.), `active_feeds` becomes empty while `engine.running` remains true
 - `stale` is derived only from `last_check` age when the feed is not currently marked running.
   - this lets feeds from the active batch regress to `stale` while the batch is still in its global phases

## Enumerated Run Reasons

Verified trigger families in the codebase:

1. `scheduled_due`
   - source/feed selected by the normal scheduler because it is due now
   - evidence:
     - `pkg/scheduler/scheduler.go:151`
     - `pkg/scheduler/scheduler.go:360`

2. `startup_integrity_rebuild`
   - queued during daemon startup because integrity found stale/missing outputs
   - evidence:
     - `pkg/web/server.go:69`
     - `pkg/web/server.go:84`

3. `admin_run`
   - explicit operator run of a feed from admin action endpoints
   - evidence:
     - `pkg/web/admin.go:308`
     - `pkg/web/server.go:454`

4. `admin_recheck`
   - explicit operator recheck, forcing upstream probe regardless of schedule
   - evidence:
     - `pkg/web/admin.go:272`
     - `pkg/web/server.go:437`

5. `admin_rebuild`
   - explicit operator reprocess/rebuild of a feed or global set
   - evidence:
     - `pkg/web/admin.go:281`
     - `pkg/web/server.go:437`

6. `integrity_rebuild`
   - explicit operator click on integrity reprocess endpoint
   - evidence:
     - `pkg/web/integrity.go:115`

7. `dependency_update`
   - derivative feed injected because a parent feed updated in this same run
   - evidence:
     - `pkg/engine/run.go:149`

Important nuance:

- The current global `/api/v1/admin/run` without query flags does not create a structured `PendingAction`.
  - It only wakes the scheduler via `runner.Trigger()`.
  - evidence:
    - `pkg/web/server.go:448`
- That means the backend cannot currently distinguish:
  - “scheduler woke up naturally”
  - vs
  - “scheduler woke up because operator clicked global run”
- To satisfy Costa’s request truthfully, the wake-up path must carry explicit reason state too.

## Decisions

Already decided by Costa:

1. Current run must show:
   - reason
   - elapsed processing time

2. Each feed must show:
   - last reason
   - last processing time

3. The table and the modal must both expose those two per-feed fields.

4. Reasons must be modeled as an enum, not free text.

5. `Currently running` means the actual in-flight feed list, not a global run summary.

6. The unrequested `Last run` fallback panel should be removed from that slot.

7. While a batch is active, the admin must expose the feeds that belong to that batch and the current batch phase.

8. Feeds that belong to the active batch must surface as running in admin, not stale.

Implied implementation decisions based on verified code shape:

- current run reason belongs in `engine.StatusSnapshot`
- currently-running feeds need a dedicated snapshot list in `engine.StatusSnapshot`
- current batch feeds and current batch phase also need dedicated fields in `engine.StatusSnapshot`
- per-feed last reason + last duration belong in `cache.Entry`
- scheduler-trigger reasons must be carried through `PendingAction`
- dependency-triggered derivative reasons must be assigned inside `RunOnce`
- ASN and GeoIP database feeds need the same metadata write path as normal feeds

## Plan

1. [done] Add a shared run-reason enum type in the backend.
2. [done] Extend `engine.RunOptions`, `engine.StatusSnapshot`, and engine run state to persist:
   - current run reason
   - last run reason
   - current elapsed basis (`last_started` already exists)
3. [done] Add active-feed tracking in the engine and expose it in `StatusSnapshot`:
   - feed name
   - reason
   - started timestamp / elapsed basis
4. [done] Add current-batch tracking in the engine and expose it in `StatusSnapshot`:
   - batch feeds
   - per-batch feed reason/status
   - current batch phase
5. [done] Extend `cache.Entry` to persist:
   - last_run_reason
   - last_processing_ms
6. [done] Extend `scheduler.PendingAction` so startup integrity, integrity reprocess, admin run, admin recheck, admin rebuild, and admin global wake all carry explicit reasons.
7. [done] Add per-feed in-progress marking and final duration accounting in:
   - `pkg/engine/process.go`
   - `pkg/engine/asn.go`
   - `pkg/engine/geoloc.go`
8. [done] Make admin status derivation treat feeds in the active batch as running instead of stale.
9. [done] Propagate dependency-triggered derivative runs as `dependency_update`.
10. [done] Expose the new batch fields through:
   - `pkg/web/admin.go`
   - `ui/src/lib/api-types.ts`
11. [done] Update admin UI:
   - `ui/src/components/admin/current-run.tsx`
   - `ui/src/components/admin/feeds-table.tsx`
   - `ui/src/components/admin/feed-modal.tsx`
   - replace the active-feed-only panel with the full current batch + phase
12. [done] Add tests for:
   - enum propagation
   - per-feed duration persistence
   - active-feed snapshot exposure
   - current batch snapshot exposure
   - stale-vs-running classification while batch active
   - admin API shape
13. [done] Build, test, install, restart, verify live admin output.

## Testing Requirements

- [done] `go test ./pkg/engine ./pkg/scheduler ./pkg/web`
- [done] `go test ./...`
- [done] `pnpm -C ui build`
- [done] `git diff --check`
- [done] `./install.sh`
- [pending] live checks:
  - admin status JSON shows current batch feeds and current phase
  - feeds in the active batch appear as running, not stale
  - feeds JSON still shows last reason / last processing fields
  - backend payload for the admin UI carries the batch feed list and phase
- [done] live checks:
  - `systemctl is-active update-ipsets` -> `active`
  - `curl -fsS http://localhost:18888/healthz` -> `ok`
  - `curl -fsS http://localhost:18888/api/v1/admin/status` immediately after restart returned:
    - `running=true`
    - `current_phase="preflight"`
    - `batch_count=31`
    - `feeds.running=31`
    - `feeds.stale=0`
  - a later status sample during source processing returned:
    - `current_phase="sources"`
    - `batch_count=32`
    - `feeds.running=32`
    - `feeds.stale=0`
  - `curl -fsS http://localhost:18888/api/v1/admin/feeds` showed batch members like `bitwire_iplistfetch_blacklist` with:
    - `status="running"`
    - `last_run_reason="scheduled_due"`
    - `last_processing_ms=1`

## Documentation Updates Required

- [done] Update `AGENTS.md` with the run-reason enum contract once implemented.
- [done] Update `AGENTS.md` with the current batch-feed / phase admin contract.
