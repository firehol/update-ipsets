# TODO: Live integrity endpoint run-aware behavior

## TL;DR

user asked to fix the live integrity endpoint after it reported issues while the daemon was actively processing feeds.

Purpose: operators should be able to trust `/api/v1/admin/integrity`. It must not show broken pipeline findings for feeds that are still being completed by the current scheduler run.

## Analysis

Evidence collected from the live daemon on 2026-04-10:

1. Startup integrity was clean.
   - Journal log at 12:42:46: `integrity check passed - all feeds have up-to-date secondary files`.
2. During the scheduler run, `/api/v1/admin/status` showed:
   - `engine.running: true`
   - `last_started: 2026-04-10T09:42:46.146453205Z`
   - `last_ended: 0001-01-01T00:00:00Z`
3. During that same run, `/api/v1/admin/integrity` reported 3 stale feeds:
   - `blocklist_net_ua`
   - `dronebl_anonymizers`
   - `dronebl_auto_botnets`
4. The same run later completed successfully.
   - Journal log at 12:49:46: `run finished updated=3 skipped=16 failed=0 elapsed=7m0.048s`.
   - `last_report.updated` contained exactly those 3 feeds.
5. After completion, `/api/v1/admin/integrity` returned:
   - `count: 0`
   - `findings: null`

Code evidence:

1. `pkg/engine/run.go`
   - Source workers finish first and the wait happens at `wg.Wait()`.
   - Geo, bogon, ASN, metadata, comparison, and insights fan-out starts after source workers.
2. `pkg/engine/integrity.go`
   - `CheckIntegrity()` compares secondary file mtimes against `cache.Entry.ProcessedDate`.
   - It has a fixed 60 second in-flight tolerance.
3. `pkg/engine/process.go`
   - `observedAt := e.now().UTC()` is captured before processing/finalize completes.
4. `pkg/web/server.go`
   - `/api/v1/admin/integrity` directly returns `eng.CheckIntegrity()` with no run-state context.

Conclusion:

The live issue is a false positive caused by checking integrity while a run is still in progress. The 60 second tolerance is not enough for longer runs, especially when large DroneBL feeds or provider fan-out are involved.

Also found: the JSON field `source_mtime` is currently misleading. `IntegrityFinding.SourceMTime` is populated with `ProcessedDate`, not the real source file mtime.

## Decisions

Decision made by user:

1. Implement the recommended fix.
   - The integrity endpoint should be run-aware.
   - It should not show in-flight feeds as broken.
   - Timestamp naming should become accurate.

No further design decision is pending.

## Plan

1. Add a run-state snapshot method on `Engine`.
   - Purpose: allow web handlers and integrity logic to know whether the engine is actively processing.
   - Evidence: run state is already guarded by `e.mu` in `tryMarkRunStart()` and `markRunEnd()`.
2. Change `/api/v1/admin/integrity` response to include:
   - `running`
   - `last_started`
   - `last_ended`
   - `status`, expected values: `clean`, `in_progress`, `issues`
   - `findings`, always an array, never `null`
3. While a run is active, suppress findings in the live endpoint and return `status: in_progress`.
   - Reason: active runs are allowed to have source outputs newer than secondary files until fan-out finishes.
4. Preserve startup behavior.
   - Startup check happens before the scheduler loop starts, so it can keep using `CheckIntegrity()` to catch genuinely stale feeds and queue reprocess.
5. Make timestamp fields accurate in integrity findings.
   - Add `processed_at` for the cache reference timestamp.
   - Add `source_file_mtime` for the actual `.ipset` / `.netset` mtime.
   - Keep `source_mtime` only if needed by existing UI/types, but do not use it for new UI copy.
6. Update TypeScript API types and admin UI if needed.
7. Update `AGENTS.md` with the new endpoint contract.
8. Add or update tests for:
   - clean findings return an empty array in the endpoint envelope
   - in-progress endpoint suppresses findings
   - integrity finding timestamps are accurate

## Implied Decisions

1. The integrity checker itself remains usable for startup and reprocess discovery.
2. The live API endpoint is responsible for suppressing in-progress false positives.
3. The `/integrity/reprocess` endpoint should not schedule feeds while a run is active; it should return `in_progress` so it does not race the current run.

## Testing Requirements

Completed:

1. `go test ./pkg/engine ./pkg/web` passed.
2. `go test ./...` passed.
3. `pnpm --dir ui build` passed.
4. `pnpm --dir ui exec eslint src/components/admin/integrity-panel.tsx src/lib/api.ts src/lib/api-types.ts` passed.
5. `git diff --check` passed.
6. Installed with `./install.sh`.
7. Live verification during active run:
   - `/api/v1/admin/status` returned `engine.running: true`.
   - `/api/v1/admin/integrity` returned `status: "in_progress"`, `running: true`, `count: 0`, `findings: []`.
8. Live verification after the run settled:
   - `/api/v1/admin/status` returned `engine.running: false`.
   - `/api/v1/admin/integrity` returned `status: "clean"`, `running: false`, `count: 0`, `findings: []`.

## Documentation Updates Required

1. Update `AGENTS.md` to document that `/api/v1/admin/integrity` is run-aware and suppresses findings while a run is active.
