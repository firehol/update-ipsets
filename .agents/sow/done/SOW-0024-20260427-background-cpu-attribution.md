# SOW-0024 | 2026-04-27 | background-cpu-attribution

## Status

completed — hot-path fix and hardening implemented, installed on the local development service, runtime-validated with admin counters/process CPU samples, and captured in specs/project skills.

## Requirements

### Purpose

Understand where CPU is spent by background work so update-ipsets can keep background maintenance resource-bounded, explainable, and fit for release/production use.

### User request quoted verbatim

> I need us to investigate the cpu utilization of background work - there is an sow I think

> No. not this skill. We need to review the cpu consumption of background work. It is not a matter of admin visibility. It is understanding where the cpu goes.

### Assistant understanding

- Stated: Costa wants CPU consumption of background work investigated.
- Stated: This is not primarily an admin-visibility/UI task.
- Inferred from evidence: `SOW-0007` is the related release umbrella because it records entity artifact performance as a release-gate investigation.
- Inferred from evidence: `SOW-0015` is not the right owner because it focuses on admin operation visibility.
- Inferred: This SOW should first produce a CPU attribution inventory and only later recommend code/config changes if evidence supports them.

### Acceptance criteria

1. Identify all code paths that run background work or background-adjacent heavy work.
   - Verification: source file/function evidence listed in Analysis.
2. Separate actual background work from foreground heavy-phase work.
   - Verification: map each path to its concurrency domain.
3. Determine what CPU attribution evidence exists today.
   - Verification: admin/system counters, run metrics, OpenTelemetry hooks, tests, and runtime snapshots listed.
4. Determine what is missing to understand "where CPU goes".
   - Verification: concrete gaps with file/function evidence.
5. Produce a next-step plan with options before implementation.
   - Verification: numbered options and recommendation in `## Implications and decisions`.

## Analysis

### Sources checked

- `SOW-0007-20260426-release-pending-work.md`
- `SOW-0003-20260426-release-readiness.md`
- `.agents/sow/specs/operating-principles.md`
- `.agents/sow/specs/config.md`
- `.agents/sow/specs/pipeline.md`
- `.agents/sow/specs/admin-ui.md`
- `.agents/skills/project-operations/SKILL.md`
- `.agents/skills/project-reviewing/SKILL.md`
- `.agents/skills/project-testing/SKILL.md`
- `pkg/engine/background_tasks.go`
- `pkg/engine/runtime.go`
- `pkg/engine/run_metrics.go`
- `pkg/engine/run_metrics_state.go`
- `pkg/engine/entity_refresh_queue.go`
- `pkg/engine/entity_surgical.go`
- `pkg/engine/entity_artifacts.go`
- `pkg/engine/entity_feed_sidecar.go`
- `pkg/engine/home_entity_builders.go`
- `pkg/engine/run.go`
- `pkg/scheduler/scheduler.go`
- `configs/firehol/runtime.yaml`
- `pkg/web/sysinfo.go`
- `pkg/web/admin.go`
- `ui/src/components/admin/current-run.tsx`

### Initial facts

- `SOW-0007` records "Entity artifact performance was made a release-gate investigation" and notes it must be repeated before production release.
- `SOW-0003` says country/ASN precompute and entity artifact operational profile still need release-grade validation.
- The specs require material CPU, memory, network, and I/O activity to be measurable and require separate concurrency controls for downloader, feed-processing, heavy-phase, and background-maintenance work.
- Background-worker defaulting must be `1` unless explicitly configured.
- `SOW-0015` is related only as UI/admin evidence; it is not the owner for CPU attribution.

### Runtime samples from local daemon

Read-only observation target:

- systemd unit: `update-ipsets`
- PID: `2815527`
- command: `/opt/update-ipsets/bin/update-ipsets daemon --config /opt/update-ipsets/etc/config --listen :18888 --admin-auth-mode=disabled --allow-unauthenticated-admin --enable-all --verbose --web-dir /opt/update-ipsets/web --web-files-dir /opt/update-ipsets/web/files`

Samples:

- Around 11:37 Europe/Athens, the process had consumed `12771.858062` CPU seconds after `1h37m23s` uptime, with RSS around `370272 KiB`.
- A 15-second `/proc` sample showed `53.420` process CPU seconds over the 15-second wall-clock window, or `356.133%` of one core.
- A 10-second `pidstat -u -t -p 2815527 1 10` sample during visible entity refresh averaged `334.80%` process CPU, with many Go runtime threads active.
- During that sample, admin status showed one background task:
  - `Entity artifacts refresh`
  - trigger `scheduled_due`
  - stage `patching entity details`
  - detail examples:
    - `patching affected entity artifacts: 178 countries and 5973 ASNs`
    - then `189 countries and 7316 ASNs`
    - then `140 countries and 1666 ASNs`
- The same admin snapshots also showed a foreground scheduled run in `publish` phase. Therefore the CPU load is not pure background work; it is background entity refresh overlapping with scheduled processing/heavy-phase work.

### CPU sink evidence

Profiler evidence:

- A 10-second `perf record -F 99 -g -p 2815527` sample captured `3440` samples with `0` lost samples.
- Non-root perf was blocked; non-interactive sudo perf succeeded and wrote `/tmp/update-ipsets-cpu-2815527-1777279463.perf.data`.
- `perf report` did not resolve Go symbols directly, so sampled addresses were mapped with `go tool addr2line /opt/update-ipsets/bin/update-ipsets`.
- The largest sampled class was Go GC work:
  - `runtime.gcBgMarkWorker`
  - `runtime.gcDrain`
  - `runtime.scanObject`
  - allocator/sweeper paths including `runtime.mallocgc`, `runtime.mapassign`, and `runtime.bgsweep`
- The application stack causing those allocations maps to:
  - `pkg/engine/entity_refresh_queue.go:60`
  - `pkg/engine/entity_refresh_queue.go:159`
  - `pkg/engine/background_tasks.go:179`
  - `pkg/engine/entity_refresh_queue.go:156`
  - `pkg/engine/entity_refresh_queue.go:226-227`
  - `pkg/engine/background_tasks.go:203`
  - `pkg/engine/entity_surgical.go:201` (`payload := e.materializeASNDetail(sidecar)`)
  - `pkg/engine/home_entity_builders.go:890` (`HealthClass: e.feedHealthClass(base.Name)`)
  - `pkg/engine/home_entity_builders.go:926` (`view := e.entryView(name, entry)`)
  - `pkg/engine/effective_entry.go:124` (`newEffectiveEntryResolver(e.cfg, e.state.SnapshotEntries())`)
  - `pkg/cache/cache.go:218-222` (allocates and fills a full `map[string]Entry` snapshot)
- The country materialization path has the same pattern:
  - `pkg/engine/entity_surgical.go:163`
  - `pkg/engine/home_entity_builders.go:848`
  - `pkg/engine/home_entity_builders.go:926`
  - `pkg/engine/effective_entry.go:124`
  - `pkg/cache/cache.go:218-222`

Interpretation:

- The hottest proven mechanism is allocation-driven GC from repeatedly snapshotting the full cache state while materializing entity JSON.
- `materializeASNDetail` and `materializeCountryDetail` call `feedHealthClass` once per feed row.
- `feedHealthClass` calls `entryView`.
- `entryView` creates a new `effectiveEntryResolver` using `e.state.SnapshotEntries()` every call.
- `SnapshotEntries()` allocates a full map and copies all cache entries.
- During a task that patches thousands of ASNs, this repeats thousands to tens of thousands of times, which explains both `entity.refresh.asn_materialize` wall time and the GC-heavy perf profile.

Facts from lifetime operation counters:

- `entity.writer_lock_hold`: `111` observations, `4,043,484 ms` total, `106,024 ms` max.
- `entity.refresh.asn_materialize`: `399,917` observations, `3,112,638 ms` total.
- `metadata.comparison_pair_overlap`: `52,149` observations, `721,171 ms` total.
- `entity.refresh.country_materialize`: `15,274` observations, `465,677 ms` total.
- `metadata.write_comparison_files`: `111` observations, `203,683 ms` total.
- `entity.refresh.asn_public_write`: `399,917` observations, `105,348 ms` total.
- `entity.refresh.asn_patch`: `399,919` observations, `100,861 ms` total.
- `entity.refresh.asn_sidecar_read`: `399,919` observations, `84,431 ms` total.

Facts from lifetime byte/counter totals:

- `entity.sidecar_build.asn_lookups`: `16,005,927`
- `entity.sidecar_build.geo_segments`: `15,992,690`
- `entity.sidecar_build.source_ranges`: `15,962,711`
- `entity.sidecar_build.country_asn_hits`: `14,804,299`
- `entity.refresh.affected_asns`: `405,528`
- `entity.refresh.affected_countries`: `15,274`
- `entity.refresh.asn_public_write`: about `9.66 GB`
- `entity.refresh.asn_sidecar_write`: about `5.13 GB`
- `entity.refresh.asn_sidecar_read`: about `5.13 GB`
- `engine.latest_set.binary_open`: `42,407` opens, about `16.40 GB`

Facts from current-run operation counters during the visible background refresh:

- `entity.refresh.asn_materialize`: `6,732` observations, `52,834 ms` total.
- `entity.refresh.country_materialize`: `189` observations, `5,652 ms` total.
- `metadata.comparison_pair_overlap`: `555` observations, `3,483 ms` total.
- `entity.refresh.asn_public_write`: `6,732` observations, `1,893 ms` total.
- `metadata.write_comparison_files`: `1` observation, `1,536 ms` total.

Counter interpretation:

- The largest observed operation-counter cost is ASN detail materialization and related ASN-sidecar patch/write work.
- Metadata pair overlap is the second major CPU class.
- Entity feed sidecar building also does substantial range-attribution work, but today it is represented mainly by counters, not operation durations.
- The samples prove multi-core process load while a background entity refresh and foreground scheduled processing overlap.
- The operation counters alone did not prove exact stack-level CPU attribution, but the later perf sample identified the dominant stack as allocation/GC triggered by per-row cache snapshots during entity materialization.

### Code path map

Background entity refresh:

- `pkg/scheduler/scheduler.go:812-818` queues entity refresh targets after each processing batch.
- `pkg/engine/entity_refresh_queue.go:41-61` coalesces feed-update refresh requests and starts `runQueuedEntityArtifactRefresh` in a goroutine.
- `pkg/engine/entity_refresh_queue.go:148-158` wraps the queued refresh in `withBackgroundTask`.
- `pkg/engine/entity_refresh_queue.go:201-230` drains queued names and calls `refreshEntityArtifactsForFeedUpdates`.
- `pkg/engine/background_tasks.go:157-185` creates/removes the visible background task and enforces only the background task-count limiter.
- `pkg/engine/background_tasks.go:188-204` holds the entity artifact mutation lock and records `entity.writer_lock_hold`.

Entity refresh CPU path:

- `pkg/engine/entity_surgical.go:58-129` loads feed deltas and computes affected country/ASN actors.
- `pkg/engine/entity_surgical.go:137-173` patches and materializes country detail payloads.
- `pkg/engine/entity_surgical.go:175-212` patches and materializes ASN detail payloads.
- `pkg/engine/entity_surgical.go:301-392` rebuilds country/ASN sidecars from changed feed deltas.
- `pkg/engine/entity_surgical.go:477-489` JSON-marshals and writes observed entity payloads.

Foreground heavy-phase CPU path:

- `pkg/scheduler/scheduler.go:737-763` always calls `RunOnce` with `Reprocess=true` for queued processing work.
- `pkg/engine/run.go:227-288` runs GeoIP, bogon, ASN, and entity-feed-sidecar staging during the normal processing heavy block.
- `pkg/engine/entity_feed_sidecar.go:614-697` builds pending feed entity sidecars during the foreground heavy phase.
- `pkg/engine/entity_feed_sidecar.go:633-653` uses `HeavyPhaseWorkers()` for this foreground sidecar fan-out.
- `pkg/engine/asn.go:247-299`, `pkg/engine/geoloc.go:136-175`, and `pkg/engine/output.go:376-434` also use `HeavyPhaseWorkers()` for ASN, GeoIP, and pairwise-comparison work.

Concurrency controls:

- `configs/firehol/runtime.yaml:67-77` configures `max_processing_workers: 2`, default heavy-phase auto behavior up to 8, and `max_background_workers: 1`.
- `pkg/engine/runtime.go:188-196` defaults processing workers to 2, heavy-phase workers via `autoHeavyPhaseWorkers`, and background workers to 1.
- `pkg/engine/runtime.go:206-225` caps automatic heavy-phase workers at 8, never below processing workers.
- `pkg/engine/runtime.go:227-232` defaults background workers to 1.
- `pkg/engine/entity_feed_sidecar.go:565-587` uses `BackgroundWorkers()` only for full background rebuild feed-sidecar generation.
- `pkg/engine/entity_surgical.go:137-212` patches actor details sequentially inside one background task, but it can overlap with foreground heavy-phase work from a later scheduler batch.

Telemetry attribution gaps:

- `pkg/engine/background_tasks.go:11-21` background task snapshots have stage/progress only; no CPU, wall-time operation breakdown, byte counters, or per-task metrics.
- `pkg/engine/run_metrics_state.go:9-20` records operation durations into global lifetime metrics and also into `e.currentMetrics` whenever a foreground run is active.
- `pkg/engine/run_metrics_state.go:54-60` counters go only to observability/lifetime counters, not current run metrics and not background task metrics.
- Because background refresh can overlap a foreground `RunOnce`, background operation durations can appear inside the foreground run's `current_metrics`. This was observed: current run phase was `publish`, but `current_metrics.operations` was dominated by `entity.refresh.asn_materialize`.
- Therefore current metrics are useful as a live mixed workload view, but they are not reliable per-run CPU attribution when background tasks overlap foreground runs.
- No in-process pprof endpoint was found in the inspected code. External `perf` worked with sudo for this local investigation, but a controlled in-process profiling path would be more reproducible and safer for future debug sessions.

### Working theory

CPU cost comes from a mix of:

1. True background entity artifact refresh, dominated by ASN detail materialization. The profiler shows this is largely allocation/GC from `feedHealthClass` -> `entryView` -> full `SnapshotEntries()` inside per-feed materialization loops.
2. Foreground scheduled processing/heavy-phase work, including pairwise comparison and pending feed entity sidecar range attribution.
3. Overlap between those two domains because the scheduler can start another foreground processing batch while a background entity refresh from the prior batch is still running.

The strongest evidence is the runtime sample showing one background entity refresh active while `engine.running=true` for a scheduled foreground run, combined with lifetime/current metrics dominated by `entity.refresh.asn_materialize`.

The exact Go function stack is now sampled by `perf`, but the current product telemetry still cannot provide this attribution without external profiling.

### Fragility review

Costa asked why this class was fragile and why it appeared again after related performance work.

Evidence-backed answer:

- The exact background ASN materialization issue was not a direct revert. The class of bug reappeared because expensive cache snapshot work was hidden behind small helpers.
- `pkg/engine/effective_entry.go:120-124` introduced `entryView()` in commit `2e070cc` to fix derivative feed health timestamps. That helper creates a new effective-entry resolver with `e.state.SnapshotEntries()` on every call.
- `pkg/cache/cache.go:214-224` shows `SnapshotEntries()` allocates a full map and copies all cache entries. This is correct for a batch snapshot, but expensive when repeated inside row loops.
- `pkg/engine/home_entity_builders.go` was added in commit `ee8b609`; its original country/ASN materializers called `e.feedHealthClass(base.Name)` for every feed row. `feedHealthClass()` called `entryView()`, so the new entity-artifact path inherited the hidden full-cache snapshot cost.
- Commit `6950522` added caching for `entityOutputView.countryComparison()` and `topASNs()` and recorded the "avoid full-cache snapshots inside HTTP row loops" rule, but it did not cover background entity materialization and did not remove the dangerous helper abstraction.
- Commit `e4b383b` simplified entity refresh around sidecars and commit `0a86298` added health-transition/integrity repair paths. Those changes increased the number of paths that materialized country/ASN detail payloads, while the materializer still hid the full-cache snapshot inside feed-health rendering.
- Review/tests missed it because correctness stayed true; the failure mode was asymptotic cost. No test asserted `SnapshotEntries()` count, allocation budget, or `entity.refresh.asn_materialize` budget for thousands of ASN payloads.
- Follow-up hardening found lower-amplification examples of the same fragility pattern: `PublicFeedSummaries()` called `EntriesSnapshot()` and then `entryView()` per row; several entity index/detail sidecar builders did the same. These were not the proven dominant background sink after SOW-0024, but they showed the abstraction was still unsafe. The hardening pass converted these paths to reuse already-effective entries or explicit batch resolvers.

Root cause:

- The invariant was encoded in local call sites and prose, not in the API boundary.
- `entryView()` and `feedHealthClass()` look cheap but are not cheap.
- The previous durable rule was too narrow: it protected "frequently polled HTTP handlers" but not background batch materialization.
- The validation gate tested correctness, not cost shape.

Hardening direction:

- Make batch-scoped effective-entry/feed-health resolution the normal API.
- Rename or remove single-use helpers that hide full snapshots, or make them visibly slow/single-use.
- Add a static or unit guard that fails when `entryView()`, `healthSnapshot()`, or `feedHealthClass()` are called inside loops without an explicit resolver/classifier.
- Add benchmark or allocation-budget coverage for large entity materialization batches.

## Implications and decisions

### Decision 1: Next investigation/fix direction

Evidence:

- Background refresh repeatedly patches thousands of ASN actors per task.
- `entity.refresh.asn_materialize` is the largest observed operation total.
- Background and foreground work overlap, creating multi-core CPU load despite `max_background_workers: 1`.
- Current metrics are polluted across foreground/background overlap because operation timing is attached to the global active run.

Options:

A. Add profiler-grade attribution first.
- Pros: proves exact CPU stacks before changing behavior; avoids optimizing the wrong thing.
- Cons: still leaves current CPU behavior unchanged until the next step.
- Risk: may require adding a local/admin profiling endpoint or using external `perf`, both of which need careful access control.

B. Fix metrics attribution first.
- Pros: separates foreground run metrics from background task metrics; makes future CPU investigations cheaper and safer.
- Cons: does not immediately reduce CPU.
- Risk: requires schema/API/UI decisions for how background task metrics are represented.

C. Reduce background/foreground overlap first.
- Pros: likely reduces observed workstation CPU pressure quickly.
- Cons: may delay entity consistency or processing throughput.
- Risk: changing scheduler ordering/backpressure without profiling could hide real waste instead of removing it.

D. Optimize ASN materialization first.
- Pros: targets the largest observed cost.
- Cons: needs deeper algorithm review and tests; more risk than telemetry/backpressure work.
- Risk: country/ASN public payload correctness is user-facing and release-critical.

Recommendation: choose D for the immediate CPU fix, with B included in the same or next SOW.

Rationale:

- A has now been partially completed via `perf`.
- The hot mechanism is specific and low-level: repeated full cache snapshots during materialization.
- A likely fix is to build/reuse an effective-entry resolver or health-class lookup once per materialization batch instead of once per feed row.
- B is still needed because current admin metrics mix background and foreground attribution during overlap.

User decision:

- 2026-04-27: Costa approved trying the immediate hot-path fix.
- Scope: precompute/reuse health classification state during entity detail materialization.
- Non-goals for this step: admin/UI metrics attribution, scheduler backpressure, profiling endpoint.
- 2026-04-27: Costa clarified this is a development server and approved restarting the local service without further prompts to try the fix.
- 2026-04-27: Costa approved hardening the code, specs, and project skills so expensive helper use becomes a conscious decision instead of an accidental regression path.
- Scope: expose fresh full-cache snapshots in helper names, move loop/batch paths to explicit batch-scoped effective-entry resolution, add tests/static guards for the expensive-helper pattern, and strengthen specs/skills.

## Plan

Investigation chunk completed. Implementation chunk now approved.

1. Inventory background and background-adjacent CPU code paths.
2. Inspect current telemetry and status counters for attribution quality.
3. If local service is running, collect read-only status samples and process CPU deltas.
4. Identify likely CPU sinks and missing probes.
5. Present options before any code/config change.
6. Remove repeated full-cache snapshots from country/ASN materialization.
7. Add focused regression coverage proving the materializer does not snapshot per feed row.
8. Run focused Go tests, then broader package tests if feasible.
9. Harden the effective-entry/feed-health helper API so fresh full-cache snapshots are explicit and batch paths reuse resolvers.
10. Add static regression coverage for the old cheap helper names and fresh-snapshot calls inside loops.
11. Update specs and project skills so future work treats expensive helper use as a deliberate design choice.

## Execution log

- 2026-04-27: Opened focused SOW after rejecting `SOW-0015` as the primary owner.
- 2026-04-27: Collected read-only admin-status, process CPU, and `pidstat` samples from local daemon PID `2815527`.
- 2026-04-27: Collected a 10-second sudo `perf` sample from local daemon PID `2815527` and mapped Go addresses with `go tool addr2line`.
- 2026-04-27: Implemented batch-scoped feed-health classification for entity detail materialization.
- 2026-04-27: Updated `.agents/sow/specs/operating-principles.md` and `.agents/skills/project-coding/SKILL.md` with the batch-scope materialization lesson.
- 2026-04-27: Installed via `./install.sh` and restarted local development service `update-ipsets`.
- 2026-04-27: Collected post-restart admin-status and `pidstat` samples from PID `3146381`.
- 2026-04-27: Reviewed why the bug class was fragile: `entryView()`/`feedHealthClass()` hid full-cache snapshots behind cheap-looking names, and prior rules covered HTTP hot paths but not background batch materialization.
- 2026-04-27: Replaced cheap effective-entry/feed-health helper names with explicit fresh-snapshot and already-effective-entry APIs.
- 2026-04-27: Converted additional loop/batch paths to reuse `EntriesSnapshot()` or `effectiveEntryResolver` once per batch, including public catalog, IP query, metadata writing, integrity, merge composition, entity indexes/details, and feed entity sidecar building.
- 2026-04-27: Added `TestEffectiveEntryHelpersExposeSnapshotCost` to fail on reintroduced cheap helper names or fresh full-cache snapshot helpers inside loops.
- 2026-04-27: Strengthened `.agents/sow/specs/operating-principles.md` and project coding/reviewing/testing skills for this failure class.
- 2026-04-27: Reinstalled via `./install.sh` and restarted local development service after the hardening patch.

Files changed:

- `pkg/engine/effective_entry.go`
- `pkg/engine/effective_entry_test.go`
- `pkg/engine/feed_health.go`
- `pkg/engine/home_entity_builders.go`
- `pkg/engine/entity_surgical.go`
- `pkg/engine/entity_artifacts.go`
- `pkg/engine/entity_integrity.go`
- `pkg/engine/entity_feed_sidecar.go`
- `pkg/engine/integrity.go`
- `pkg/engine/merge_inputs.go`
- `pkg/engine/metadata.go`
- `pkg/engine/output.go`
- `pkg/engine/public.go`
- `pkg/engine/public_catalog.go`
- `pkg/engine/query.go`
- `pkg/engine/home_detail_test.go`
- `.agents/sow/specs/operating-principles.md`
- `.agents/skills/project-coding/SKILL.md`
- `.agents/skills/project-reviewing/SKILL.md`
- `.agents/skills/project-testing/SKILL.md`
- `.agents/sow/current/SOW-0024-20260427-background-cpu-attribution.md`

## Validation

- Initial runtime sampling was read-only; after Costa approved using the development service, `./install.sh` restarted the local `update-ipsets` service.
- `perf record` non-root failed due event permission error; sudo `perf record` succeeded.
- Focused test: `go test ./pkg/engine -run TestMaterializeASNDetailUsesBatchHealthClassifierSnapshot -count=1`
- Focused hardening tests: `go test ./pkg/engine -run 'Test(EffectiveEntryHelpersExposeSnapshotCost|EntrySnapshotUses|MaterializeASNDetailUsesBatchHealthClassifierSnapshot)' -count=1`
- Focused entity tests: `go test ./pkg/engine -run 'Test(CountryDetail|ASNDetail|MaterializeASNDetail|RebuildEntityArtifacts|RefreshEntityArtifacts)' -count=1`
- Engine package: `go test ./pkg/engine -count=1`
- Full Go tests: `make test`
- Build: `make build`
- Vet: `make lint`
- Diff hygiene: `git diff --check`
- Source scan: `rg -n "entryView\\(|healthSnapshot\\(|feedHealthClass\\(" pkg/engine --glob '!pkg/web/static/assets/**'` returned no matches.
- Install/restart: `./install.sh`, which rebuilt the UI bundle and Go binary, installed `/opt/update-ipsets/bin/update-ipsets`, reloaded systemd, and restarted `update-ipsets`.
- Service health: `curl -fsS http://localhost:18888/healthz` returned `ok`.
- Post-restart service: PID `3146381`, systemd active, public listener `:18888`.
- Hardening reinstall/restart: `./install.sh`, then `curl -fsS http://localhost:18888/healthz` returned `ok`; service active on PID `3279216`.
- During live scheduled/background work after restart, `pidstat -u -t -p 3146381 1 15` averaged `314.12%` process CPU.
- After the scheduled burst ended, `pidstat -u -p 3146381 1 8` averaged `0.75%` process CPU.
- Post-fix admin counters after three scheduled batches:
  - `entity.refresh.asn_materialize`: `14,934` observations, `430 ms` total, `1 ms` max.
  - `entity.refresh.country_materialize`: `507` observations, `68 ms` total, `1 ms` max.
  - `metadata.comparison_pair_overlap`: `1,194` observations, `10,078 ms` total.
  - `metadata.write_comparison_files`: `3` observations, `5,612 ms` total.
  - `entity.writer_lock_hold`: `4` observations, `21,873 ms` total.

Runtime comparison against the pre-fix sample:

- Before fix: `entity.refresh.asn_materialize` showed `6,732` observations and `52,834 ms` total in the live current-run counters.
- After fix: `entity.refresh.asn_materialize` showed `14,934` observations and `430 ms` total in lifetime counters after restart.
- Interpretation: the proven row-snapshot waste is removed from materialization. Remaining multi-core CPU during scheduled bursts is now primarily foreground processing/metadata comparison and writer-lock-held work, not ASN detail materialization.

## Outcome

Background CPU was primarily explained by entity artifact refresh, especially ASN detail materialization. The profiler showed the dominant mechanism was allocation/GC caused by repeatedly snapshotting the full cache state from `feedHealthClass` during per-feed materialization loops.

Commits:

- `11eb69e fix: reuse feed health state for entity artifacts`
- `6ca8bdb fix: make feed snapshot costs explicit`

Implemented fix:

- `feedHealthClassifier` now captures one cache snapshot, effective-entry resolver, policy, and timestamp.
- Bulk entity materialization paths create one classifier per refresh/repair/rebuild batch.
- Country/ASN materializers reuse that classifier instead of rebuilding effective-entry state per feed row.
- Existing single-detail API paths still create a classifier per request, which is acceptable because they materialize one detail payload.
- `entryView()` / `healthSnapshot()` / `feedHealthClass()` cheap helper names were removed from engine source.
- Single-entry full-cache snapshot use is now named `entryViewFromFreshStateSnapshot()` / `healthSnapshotFromFreshStateSnapshot()`.
- Loop/batch paths now use already-effective snapshots or explicit batch-scoped `effectiveEntryResolver` / `feedHealthClassifier`.
- Static guard coverage now fails if the cheap helper names return or if fresh full-cache snapshot helpers are called inside loops.

## Lessons extracted

- Spec: entity artifact materialization must build expensive shared lookup state at batch scope, not row scope.
- Project skill: avoid repeated full cache snapshots inside entity materialization loops; reuse effective-entry/feed-health lookup state per batch.
- Spec: frequently polled HTTP handlers and background batch processors both count as hot paths and must not duplicate full-cache snapshots inside per-row loops.
- Project skills: names must expose expensive helper cost, reviewers must check loop/batch paths for hidden snapshots, and tests must include cost-shape regression guards when hot-path helpers change.
