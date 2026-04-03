# TODO: Fix zero-delta changeset rows

## TL;DR

Costa found a bug in the Go rewrite: the "IPs added vs removed per update" data must represent the last real content changes, not rebuilds or checks where the effective set did not change.

Required behavior:
- Do not write `changesets.csv` rows where `AddedIPs == 0` and `RemovedIPs == 0`.
- Keep writing rows where the net delta is zero but churn is real, for example `AddedIPs == RemovedIPs > 0`.
- Filter any historical polluted zero-delta rows when serving the API and deriving insights.
- Match the legacy bash behavior.

## Purpose

The feed detail page needs a trustworthy activity signal for "is this feed maintained?". The changeset series is the source for update cadence and churn, so it must only contain actual set changes.

## Analysis

### Bash behavior

Evidence from `/home/costa/src/firehol/firehol/sbin/update-ipsets`:

- `finalize` returns before marking a feed updated when the newly processed file is equal to the existing output (`2700-2706`).
- The retention/web changeset generation loop iterates only over `UPDATED_SETS` (`2453-2464`).
- `retention_detect` writes `DateTime,IPsAdded,IPsRemoved` rows only after the update path has accepted the new snapshot (`1762-1804`).

Conclusion: in the bash implementation, equal-content runs do not become changeset points.

### Go rewrite behavior

Evidence in this repo:

- [process.go](/home/costa/src/firehol/update-ipsets/pkg/engine/process.go):310 skips equal processed sets only when `opts.Rebuild` is false.
- [process.go](/home/costa/src/firehol/update-ipsets/pkg/engine/process.go):324 then calls `updateRetention()` even for rebuilds of identical content.
- [retention.go](/home/costa/src/firehol/update-ipsets/pkg/engine/retention.go):61 appends `changesets.csv` unconditionally.
- [query.go](/home/costa/src/firehol/update-ipsets/pkg/engine/query.go):117 returns every parsed row from `changesets.csv`.
- [server.go](/home/costa/src/firehol/update-ipsets/pkg/web/server.go):253 documents the wrong behavior: "the total may be zero while the list was refreshed".
- Existing local generated data confirms pollution exists: `/home/costa/.update-ipsets/lib/dshield/changesets.csv:3` and `/home/costa/.update-ipsets/lib/bogons/changesets.csv:3` contain `0,0` rows.

Conclusion: the Go rewrite diverged from bash by recording rebuild/check events as changesets.

## Decisions

No pending Costa decision.

Decision already made by Costa:
- Treat zero-delta rows as an implementation bug.
- The changeset chart is a chart of the last 500 changes, not the last 500 checks.
- The bash implementation is the behavior reference.
- 2026-04-10: Verify every external-review parity claim in detail against bash and Go source, fix every confirmed discrepancy, and do not implement fixes for claims rejected by evidence.

## Plan

1. Change `updateRetention()` so it appends a changeset row only when `added > 0 || removed > 0`.
2. Keep all other retention work intact so `retention.json` and removal-age data still refresh after successful processing.
3. Filter existing `0,0` rows in `ChangesetSeries()` so already polluted data does not reach the API/UI.
4. Filter existing `0,0` rows in the insights churn reader so insight rules do not treat old rebuild rows as activity.
5. Move the insights size/churn CSV readers out of the already-large `insights.go` file into a small focused file while making the churn fix.
6. Update comments/API type docs that currently describe zero-delta refresh rows as valid.
7. Add tests proving:
   - rebuild after HTTP 304 still reprocesses when requested;
   - identical rebuild does not append a `0,0` changeset row;
   - API changeset series ignores historical `0,0` rows;
   - insights churn series ignores historical `0,0` rows.

## Implied Decisions

- Do not rewrite or clean existing on-disk generated `changesets.csv` files in this task. Runtime readers filter old bad rows safely.
- Do not change the chart component unless backend/API semantics require it; the UI already renders whatever `/changesets` returns.
- Keep `AddedIPs == RemovedIPs > 0` as a valid changeset because the feed changed even when net size did not.

## Testing Requirements

- Run `go test ./pkg/engine ./pkg/web`.
- Run `go test ./...`.
- Run `go vet ./...`.
- Run `go test -race ./pkg/engine ./pkg/web`.
- Run `go build ./...`.
- Run `git diff --check`.
- If package tests fail because of unrelated dirty worktree changes, report the exact failure and do not claim full verification.

## Implementation Status

Implemented in this worktree:
- `updateRetention()` now writes `changesets.csv` only when `AddedIPs > 0 || RemovedIPs > 0`.
- `ChangesetSeries()` filters historical `0,0` rows, drops the bootstrap row, and returns the last `web_charts_entries` changes.
- Insights churn reading filters historical `0,0` rows.
- Size/churn insight readers were moved from `pkg/engine/insights.go` to `pkg/engine/insights_series.go`; `insights.go` is now below 500 lines.
- API/type comments and `AGENTS.md` now document that changesets are real content changes, not checks.
- Tests were added for the identical rebuild path and for historical zero-row filtering.
- `lib/<feed>/history.csv` is now the internal full history ledger.
- Public `<feed>_history.csv` is regenerated from the internal ledger using the configured `web_charts_entries` window; legacy Go public history is used only as a fallback when the internal ledger does not exist yet.
- Public `<feed>_changesets.csv` is generated with bash public headers (`DateTime,AddedIPs,RemovedIPs`).
- Public `<feed>_retention.json` is generated for every output feed.
- Internal changesets keep the bash internal header (`DateTime,IPsAdded,IPsRemoved`).
- Existing old Go internal changeset ledgers with header `DateTime,AddedIPs,RemovedIPs` are normalized to the bash internal header when public changesets are generated or new changes are appended.
- History, changesets, retention rows, `latest.set`, output text files, and history snapshots now use source/output mtime semantics where the bash source does.
- Public web artifacts are generated in a staging directory under the web directory and published after the batch is complete.
- Integrity/admin manifest expectations now include public changesets and retention files.
- `AGENTS.md` records the restored public/internal file contract and staging-publication rule.

Verification run:
- `go test ./pkg/engine` — passed.
- `go test ./pkg/engine ./pkg/web` — passed.
- `go test ./...` — passed.
- `go vet ./...` — passed.
- `go test -race ./pkg/engine ./pkg/web` — passed.
- `go build ./...` — passed.
- `git diff --check` — passed.

Important bash evidence rechecked during implementation:
- Bash `history_keep()` writes snapshots named from the processed file mtime and touches them to that mtime (`/home/costa/src/firehol/firehol/sbin/update-ipsets:973-987`).
- Bash `history_cleanup()` deletes snapshots older than the current time minus the configured history window by comparing file mtimes to a reference file (`/home/costa/src/firehol/firehol/sbin/update-ipsets:990-1002`).
- Therefore an old upstream `Last-Modified` timestamp can be pruned immediately from short retention windows; the Go test was corrected to match this bash behavior.
- Bash public publication stages generated files in `RUN_DIR` and then uses `mv -f RUN_DIR/* WEB_DIR/` (`/home/costa/src/firehol/firehol/sbin/update-ipsets:2473-2474`). This is not a single filesystem transaction; the Go rewrite now matches the staging-before-move pattern, not a stronger directory-swap design.

## External Reviewer Parity Audit — 2026-04-10

Costa requested independent checks from `glm`, `qwen`, `kimi`, and `minimax` for bash-vs-Go file and logic-per-file parity.

Execution status:
- `glm-5.1` completed a read-only review.
- `qwen3.6-plus` completed a read-only review.
- `minimax-m2.7-coder` completed a read-only review.
- `kimi-k2.5-alibaba` initially failed with an `opencode` SQLite WAL error. On retry it produced a partial review, then violated the read-only/no-delegation prompt by spawning nested `opencode` reviewers. Those exact descendant PIDs were terminated after verification.

Consolidated findings that survived local sanity-check:

1. `StartedDate` is not bash-compatible.
   - Bash sets a missing `IPSET_STARTED_DATE` from `IPSET_SOURCE_DATE` (`/home/costa/src/firehol/firehol/sbin/update-ipsets:2781`).
   - Go sets missing `entry.StartedDate` from `entry.ProcessedDate` in normal feed finalization (`pkg/engine/finalize.go:62-64`) and in Geo/ASN provider paths (`pkg/engine/geoloc.go:140-144`, `pkg/engine/asn.go:200-204`).
   - Impact: first-run `started` timestamps can differ from bash when the upstream source mtime is older than processing time.

2. `clock_skew` JSON units are not bash-compatible.
   - Bash emits `clock_skew` multiplied by 1000 in per-feed JSON and `all-ipsets.json` (`/home/costa/src/firehol/firehol/sbin/update-ipsets:1581`, `:1632`).
   - Go emits raw seconds (`pkg/engine/output.go:131-135`, `pkg/engine/metadata.go:42-44`).
   - Impact: raw static JSON consumers see different units. This divergence may be intentional for the current UI, but it is not bash file-content parity.

3. Go rewrites public metadata/series files for all output feeds on every run; bash writes web files only when there are updated sets or a forced rebuild.
   - Bash exits web update when no `UPDATED_SETS` and no `FORCE_WEB_REBUILD` (`/home/costa/src/firehol/firehol/sbin/update-ipsets:2096-2099`), and changesets/retention loop only over `UPDATED_SETS` (`:2453-2464`).
   - Go always enters `writeMetadataFiles()` and loops over `outputNames()` (`pkg/engine/run.go:273-382`, `pkg/engine/metadata.go:25-84`).
   - Impact: content may be the same, but mtimes and I/O differ for feeds that did not update.

4. Internal binary state filenames still differ.
   - Bash primary retention latest file is `lib/<name>/latest` (`/home/costa/src/firehol/firehol/sbin/update-ipsets:1808`).
   - Go writes `lib/<name>/latest.set` (`pkg/engine/finalize.go:43`) while reading both names.
   - Bash retention new-IP files are `lib/<name>/new/<epoch>` (`/home/costa/src/firehol/firehol/sbin/update-ipsets:1780-1781`).
   - Go writes `lib/<name>/new/<epoch>.set` (`pkg/engine/retention.go:75-78`) while reading both names.
   - Impact: Go is migration-compatible, but file-name parity is not exact.

5. Bash keeps `lib/<name>/histogram`; Go does not.
   - Bash writes a shell cache at `/home/costa/src/firehol/firehol/sbin/update-ipsets:1940`.
   - Go rebuilds retention data from `retention.csv` and `new/`, and writes `lib/<name>/retention.json` instead (`pkg/engine/retention.go:131-139`).
   - Impact: internal maintained-file parity is not exact, even if derived JSON can be equivalent.

6. Go has additive public/internal files that bash did not produce.
   - Examples: `index.json`, `robots.txt`, `_insights.json`, `_asn_*.json`, `_bogons_*.json`, internal `lib/<name>/retention.json`.
   - Impact: if "100% file parity" means no extra generated files, these are discrepancies. If Go-site extensions are allowed, they should be documented as intentional contract extensions.

7. Rename/delete cleanup parity is incomplete.
   - Bash rename/delete handles selected web suffixes including `_comparison.json`, hardcoded geo country files, `_history.csv`, `.json`, `.html` (`/home/costa/src/firehol/firehol/sbin/update-ipsets:3208-3215`, `:3312-3318`).
   - Go rename/delete only handles `.source`, `.ipset`, `.netset`, `.setinfo`, `.json` for base/web paths and removes history/lib directories (`pkg/engine/helpers.go:123-170`).
   - Impact: Go can leave orphan public secondary files on rename/delete.

Reviewer findings rejected after local verification:
- Retention JSON field name mismatch was false: Go tags `RetentionData.Name` as `json:"ipset"` (`pkg/engine/engine.go:94-100`), matching bash.
- Public changeset off-by-one was false for normal and young ledgers: bash `tail -n (N+1) | grep -v header | tail -n +2` and Go `drop first data row, then trim to N` produce the same selected rows after the zero-row cleanup policy.
- Geo/ASN provider public changesets/retention generation was false for current Go paths: `processSource()` skips `use:[geoip]` and `use:[asn]` before setting `entry.File`, while `outputNames()` requires `entry.File != ""` (`pkg/engine/process.go:54-56`, `pkg/engine/public.go:317-328`).
- Non-redistributable `.setinfo` "disabled" mismatch was false: bash only omits the source link, matching Go (`/home/costa/src/firehol/firehol/sbin/update-ipsets:2865-2871`, `pkg/engine/metadata.go:176-184`).

Recommended follow-up:
- Fix the seven surviving discrepancies above if Costa wants strict file parity.
- Add a parity test harness that runs bash and Go against the same small fixture and diffs generated filenames, headers, and selected JSON fields.

### Follow-up Verification — 2026-04-10

Costa requested detailed verification of the reviewer claims and fixes for confirmed discrepancies.

Verified additional facts before implementation:

1. Literal per-feed JSON content parity is broader than the reviewer list.
   - Bash per-feed JSON includes legacy fields `grade`, `protection`, `intended_use`, `false_positives`, `poisoning`, and `services` (`/home/costa/src/firehol/firehol/sbin/update-ipsets:1597-1603`).
   - Go `setMetadata` currently omits those fields and adds Go/frontend fields such as `geo`, `used_for`, `hidden`, `processor`, `pre_processor`, `dont_redistribute`, and `format` (`pkg/engine/output.go:19-76`).
   - This conflicts with the existing project rule in `AGENTS.md` that forbids phantom fields unless Costa explicitly decides to reintroduce them.

2. Literal comparison JSON content parity is broader than the reviewer list.
   - Bash `_comparison.json` rows have only `name`, `category`, `ips`, and `common` (`/home/costa/src/firehol/firehol/sbin/update-ipsets:2267-2288`).
   - Go comparison rows add `related` for derivative-family filtering (`pkg/engine/engine.go:65-86`, `pkg/engine/output.go:406-425`).
   - Removing `related` would affect current UI behavior that excludes derivative echo from uniqueness/inclusion math.

3. Literal file-set parity conflicts with current Go UI/API artifacts.
   - Go endpoints read `_insights.json`, `_asn_<provider>.json`, and `_bogons_<provider>.json` (`pkg/web/server.go:286-365`).
   - `AGENTS.md` currently documents these as Go output files and frontend/API dependencies (`AGENTS.md:423-435`, `:591-607`).
   - Bash never produced these files.

4. The reviewer list missed two additional bash contract gaps.
   - Bash supports `WEB_OWNER` chown for public web files and copied ipset files (`/home/costa/src/firehol/firehol/sbin/update-ipsets:2473-2475`, `:895-920`). Go parses `web_owner` in config but drops it when resolving runtime (`pkg/config/config.go:104-108`, `pkg/engine/runtime.go:14-50`).
   - Bash copies updated redistributable ipset/netset files into `WEB_DIR_FOR_IPSETS` via `.new` then rename (`/home/costa/src/firehol/firehol/sbin/update-ipsets:895-920`). Go only exposes `local_copy_url` in metadata; it does not copy the files.

Feasibility verdict:
- FEASIBLE AS SPECIFIED for unambiguous bash-owned file-contract fixes:
  - `StartedDate` source-time initialization.
  - public JSON `clock_skew` milliseconds while keeping internal/admin seconds.
  - bash names `lib/<name>/latest` and `lib/<name>/new/<epoch>`.
  - `lib/<name>/histogram` cache generation.
  - skip public web publication when no feeds updated and no rebuild was requested.
  - publish per-feed JSON/history/changesets/retention only for updated feeds, while rebuilding catalog/sitemap/comparison during a web update.
  - rename/delete cleanup for bash public suffixes plus current Go suffixes.
  - `WEB_OWNER` and `WEB_DIR_FOR_IPSETS` behavior.
- OBSTACLE FOUND for literal "no extra fields/files" parity:
  - Removing Go-only files/fields would break documented current frontend/API behavior.
  - Reintroducing bash phantom fields conflicts with the existing codebase rule unless Costa explicitly chooses that.

Costa decisions made on 2026-04-10 after the follow-up verification:

1. Use "legacy contract parity plus documented Go extensions".
   - Fix every bash-owned legacy file/behavior.
   - Keep Go-only files/fields that the current Go frontend/API needs.
2. Do not reintroduce bash phantom metadata fields.
   - Keep the current rule that metadata fields must have real population paths.

Implementation status after these decisions:
- `StartedDate` now initializes from the source/output mtime (`SourceDate`) in normal, GeoIP, and ASN processing paths.
- Public per-feed JSON and `all-ipsets.json` now emit `clock_skew` in milliseconds, matching bash static JSON; internal/admin API fields keep `clock_skew_seconds`.
- The bash-compatible internal binary filenames are restored: `lib/<name>/latest` and `lib/<name>/new/<epoch>`. Read paths keep fallback support for earlier Go `latest.set` / `<epoch>.set` files.
- `lib/<name>/histogram` is generated as a bash-source-able retention cache.
- Public web files are not republished when a run has no updated feeds and no rebuild request.
- During a web update, per-feed legacy files are generated only for updated feeds (or all output feeds on full rebuild); catalog and secondary fan-out files remain Go-supported extensions.
- Rename/delete cleanup now covers bash public secondary suffixes plus Go extension suffixes derived from configured GeoIP/ASN/bogon providers.
- `web_owner` / `WEB_OWNER` is carried into runtime and used to chown staged public files and copied raw ipset files.
- `web_dir_for_ipsets` / `WEB_DIR_FOR_IPSETS` now mirrors updated redistributable `.ipset` / `.netset` files via `.new` then rename, matching bash.
- Public retention `past` excludes removals of entries first added in the bootstrap row, matching the bash histogram condition.
- `AGENTS.md` documents the restored legacy contract plus intentional Go extensions.

Verification after these fixes:
- `go test ./...` — passed.
- `go vet ./...` — passed.
- `go build ./...` — passed.
- `go test -race ./pkg/engine ./pkg/web` — passed.
- `pnpm --dir ui build` — passed.
- `git diff --check` — passed.
- `pnpm --dir ui lint` — failed on existing lint-rule violations in files outside this parity fix (`Date.now()` during render, synchronous `setState` in effects, and fast-refresh export rules). The UI build and TypeScript compilation passed.

Lint cleanup requested by Costa:
- Scope: make `pnpm --dir ui lint` pass instead of leaving the earlier verification limitation unresolved.
- Feasibility verdict: FEASIBLE AS SPECIFIED.
- Verified lint failure classes before editing:
  - Impure `Date.now()` calls during render in admin current-run, feed modal, and feeds table.
  - Synchronous `setState` inside effects in ASN provider tabs, Geo provider tabs, and feed-sidebar viewport detection.
  - Fast-refresh mixed exports in cursor tooltip, feed sidebar, theme provider, button, and badge files.
  - Warnings in comparison rows dependency handling, error-boundary console suppression, and TanStack Table's known incompatible-library rule.
- Plan:
  - Add a small `useNow()` hook so admin overdue calculations use a state value populated outside render.
  - Derive active provider tabs from user selection plus available providers instead of syncing with `setState` in effects.
  - Replace feed-sidebar media-query state sync with `useSyncExternalStore`.
  - Move non-component exports/hooks/helpers out of component files where fast-refresh requires component-only exports.
  - Fix or explicitly scope the remaining warnings, then rerun lint and UI build.
- Implementation status:
  - Added `ui/src/lib/use-now.ts` and replaced render-time `Date.now()` overdue checks in admin current-run, feed modal, and feeds table.
  - Derived active ASN/Geo provider tabs from selected provider plus live provider data; removed synchronous effect state sync.
  - Replaced feed-sidebar viewport detection with `useSyncExternalStore`.
  - Moved `useClearOnExit`, theme context/hook, and feed-sidebar events/shortcut hook into non-component modules.
  - Removed unused `buttonVariants` / `badgeVariants` exports.
  - Fixed the comparison rows dependency warning, removed the unused console-disable comment, and scoped the TanStack Table compiler warning with a specific eslint-disable comment.
- Verification after lint cleanup:
  - `pnpm --dir ui lint` — passed.
  - `pnpm --dir ui build` — passed.
  - `git diff --check` — passed.
  - `go test ./pkg/engine ./pkg/web` — passed.
  - `go vet ./pkg/engine ./pkg/web` — passed.

## Documentation Updates Required

- Inline API/type comments in Go and TypeScript were updated so they state that changesets are real content changes.
- `AGENTS.md` was updated with the restored bash-compatible public/internal file contract.
- No user-facing methodology page is required for this bug fix unless the maintained/abandoned status feature is implemented later.

## Git Output File Contract Follow-up — 2026-04-10

Costa asked to fix the remaining verified bash-vs-Go file contract issues after the external reviewer run.

### TL;DR

Fix the remaining bash-owned generated git-support files:
- `README.md`
- `.gitignore`
- `set_file_timestamps.sh`
- `.setinfo` creation lifecycle
- misleading retention snapshot comments

Do not remove Go-only web/API extension files or reintroduce bash phantom metadata fields; Costa already chose "legacy contract parity plus documented Go extensions" and no phantom fields.

### Feasibility Verdict

FEASIBLE AS SPECIFIED.

Evidence:
- Bash creates `.gitignore` only in git repos and initializes it with `*.setinfo` and `*.source` (`/home/costa/src/firehol/firehol/sbin/update-ipsets:728`).
- Bash appends ignored private files instead of replacing `.gitignore` (`/home/costa/src/firehol/firehol/sbin/update-ipsets:797-804`).
- Bash `README.md` preserves `README-EDIT.md`, includes legacy generated prose and table headers, then appends `*.setinfo` (`/home/costa/src/firehol/firehol/sbin/update-ipsets:831-842`).
- Bash timestamp script uses `#!/bin/bash`, includes a safety argument guard, and writes `touch --date=@...` commands (`/home/costa/src/firehol/firehol/sbin/update-ipsets:858-867`).
- Bash writes `.setinfo` inside the git-only block (`/home/costa/src/firehol/firehol/sbin/update-ipsets:2849-2873`).
- Go centralizes the corresponding behavior in `pkg/output/sync.go` and calls it from `pkg/engine/metadata.go`.

### Plan

1. Make README generation match the bash text contract and preserve `README-EDIT.md`.
2. Make `.gitignore` initialization/append behavior match bash instead of replacing the file with a generated block.
3. Make `set_file_timestamps.sh` match bash's shell, safety guard, and command shape.
4. Gate `.setinfo`, README, `.gitignore`, and timestamp script generation on `BaseDir` being a git repo, matching bash's git-only lifecycle.
5. Correct misleading Go comments that describe bash retention snapshots as text.
6. Update focused tests for the corrected contracts.
7. Verify with Go tests, vet/build where relevant, and `git diff --check`.

### Implementation Status

- `pkg/output/sync.go`
  - `WriteREADME()` now no-ops unless `BASE_DIR/.git` is a directory.
  - `WriteREADME()` preserves or creates `README-EDIT.md` and writes the bash-compatible generated prose/table shape.
  - `WriteGitIgnore()` now no-ops unless `BASE_DIR/.git` is a directory.
  - `WriteGitIgnore()` initializes missing `.gitignore` with `*.setinfo` and `*.source`, preserves existing content, and appends only private raw `.ipset` / `.netset` paths.
  - `WriteTimestampScript()` now no-ops unless `BASE_DIR/.git` is a directory.
  - `WriteTimestampScript()` now writes the bash shebang, safety guard, and `touch --date=@...` lines only for raw `.ipset` / `.netset` outputs.
- `pkg/engine/metadata.go`
  - `.setinfo`, README, `.gitignore`, and timestamp script generation are gated by `output.HasGitDir(e.runtime.BaseDir)`.
  - Timestamp script input is now the full output-name raw-set list with source timestamps, not the broad generated web-artifact list.
- `pkg/engine/retention.go`
  - Corrected the misleading comment: bash and Go snapshots are binary by contract; text parsing remains a defensive fallback for manual/experimental leftovers.
- `pkg/output/sync_test.go`
  - Added/updated tests for README text contract, `.gitignore` initialization and preservation, timestamp-script shape, and no-op behavior outside a git checkout.
- `pkg/engine/engine_test.go`
  - `TestOutputArtifactsRespectDontRedistribute` now creates a fake base `.git` directory so it explicitly tests git-support artifacts.
  - `TestRunOnceAndQuery` now asserts no `.setinfo`, `README.md`, `.gitignore`, or `set_file_timestamps.sh` are created when `BASE_DIR/.git` is absent.
- `AGENTS.md`
  - Documented `.setinfo` as a base git-support file, not a public web file.
  - Added the bash-compatible git-support file contract.

### Verification After Follow-up Fix

- `go test ./pkg/output` — passed.
- `go test ./pkg/engine` — passed.
- `go test ./pkg/engine ./pkg/output` — passed after the extra non-git engine assertion.
- `go test ./...` — passed.
- `go build ./...` — passed.
- `go vet ./...` — passed.
- `go test -race ./pkg/output ./pkg/engine` — passed.
- `git diff --check` — passed.

External reviewer rerun status:
- Minimax completed a full read-only review and reported no remaining filename/content/lifecycle/logic mismatch requiring a fix before commit.
- Kimi violated the read-only instruction by writing `/tmp/review_prompt.txt` and launching nested `opencode` reviewers. I terminated the exact Kimi-created process tree and did not use Kimi as evidence.
- GLM and Qwen were both attempted with the required 30-minute `timeout 1800` wrapper. They failed before a usable final report with `Timeout on reading data from socket`.

## Broader File Contract Audit

Costa raised a broader concern after the zero-delta changeset fix: other maintained files may have similar Go-vs-bash contract drift, especially files with append/truncate behavior.

Initial audit findings:

1. Bash maintained an internal full history ledger at `LIB_DIR/<feed>/history.csv`, then generated the public `<feed>_history.csv` from the last `WEB_CHARTS_ENTRIES` rows. Go writes directly to public `<feed>_history.csv` and truncates that same file to a hardcoded 500 rows.
2. Bash maintained an internal full changeset ledger at `LIB_DIR/<feed>/changesets.csv`, then generated public `<feed>_changesets.csv` from the last `WEB_CHARTS_ENTRIES` rows while skipping the first data row. Go maintains only internal `lib/<feed>/changesets.csv` and serves JSON from the API; it does not write public `<feed>_changesets.csv`.
3. Bash timestamps history, changesets, and retention rows from the finalized source/output file mtime. Go currently timestamps history and retention/changesets with `observedAt`.
4. Bash wrote public `<feed>_retention.json`. Go writes `lib/<feed>/retention.json` and serves it through the API; it does not write public `<feed>_retention.json`.
5. Bash wrote `LIB_DIR/<feed>/histogram` as a shell-cache file. Go does not maintain that file; it rebuilds `retention.json` from `retention.csv` and `new/`.
6. Go still exposes `web_charts_entries` in configuration, but current truncation/read paths use literal `500`.

Decision options presented before broader compatibility implementation:

1. Should Go preserve the bash public file contract?
   - A: Yes, restore/generate public `<feed>_changesets.csv` and `<feed>_retention.json`, and keep `<feed>_history.csv` as a generated public window.
   - B: No, APIs are the public contract now; only fix internal correctness.
   - Recommendation: A, because `iplists.firehol.org` historically exposed static files and downstream consumers may rely on them.
2. Should Go preserve the bash internal full-ledger contract for history?
   - A: Yes, add `lib/<feed>/history.csv` as the append-only full ledger and generate public `<feed>_history.csv` from it.
   - B: No, keep only the public last-N history file.
   - Recommendation: A, because full history is needed for reliable cadence/abandonment calculations beyond the last 500 public points.
3. Should changeset chart/API skip the first data row like bash public `<feed>_changesets.csv` did?
   - A: Yes, treat the first row as baseline bootstrap, not activity.
   - B: No, show the initial population as a valid first change.
   - Recommendation: A for bash parity and for better maintenance-cadence math.
4. Which timestamp should ledgers use?
   - A: Bash parity: source/output mtime (`sourceMTime`) for history, changesets, and retention rows.
   - B: Go current behavior: processing observation time (`observedAt`).
   - Recommendation: A, because it matches bash and reflects the upstream feed version when Last-Modified is available.
5. Should `web_charts_entries` be honored everywhere?
   - A: Yes, replace hardcoded 500s with runtime config.
   - B: No, make 500 a fixed product rule and remove/deprecate config.
   - Recommendation: A, because the field already exists in YAML and bash used the equivalent setting.

Costa decisions made on 2026-04-10:

1. Preserve the bash public static file contract. The Go rewrite must not change the file contract between the two versions.
2. Preserve bash internal ledger behavior when confirmed by re-reading the bash source.
3. Preserve bash changeset window semantics, including treating the bootstrap row the same way bash treated it.
4. Preserve bash timestamp semantics where the bash source shows source/output mtimes are used.
5. Honor `web_charts_entries` instead of hardcoding `500`.
6. Restore the bash batch-publication pattern: generate a batch of public files into a temporary/staging area and move them into place together so a stopped run cannot leave half of the public files updated and the rest stale/missing.
7. The bash implementation is the reliability reference; replicate its file-maintenance patterns without changing contracts.
