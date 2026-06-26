# SOW-0118 Accessor Generation Inventory

Generated: 2026-06-26

Purpose: inventory for `Config()` / `Runtime()` accessor calls that are race-safe individually but can mix reload generations when a caller needs both config and runtime or uses several runtime snapshots for one builder. This is a maintainer/SOW artifact, not public documentation.

Command:

```bash
rg -n '\b(eng|e|r\.eng)\.(Runtime|Config)\(\)' pkg/engine pkg/scheduler pkg/web --glob '*.go' --glob '!**/*_test.go'
```

Classification legend:

- `mixed-generation-review`: caller must be reviewed for one combined config/runtime generation or proved intentionally latest-at-use.
- `single-accessor-review`: single accessor call appears locally, but still must be reviewed for caller-chain generation coherence.

## Single-Accessor Caller-Chain Plan

The 50 `single-accessor-review` rows are not terminally safe just because each
local line calls only one accessor. They are grouped here so implementation can
finish them by caller-chain boundary instead of ignoring them during the
mixed-generation conversion.

| Caller-chain class | Rows | Locations | Terminal handling plan |
|---|---:|---|---|
| Public/request builders | 17 | `pkg/web/server.go:102`; `pkg/engine/public_catalog.go:74`; `pkg/engine/query.go:149,254,334,362,367`; `pkg/engine/effective_entry.go:31`; `pkg/engine/feed_health.go:12`; `pkg/engine/public.go:40,105,206,233,261,472,516`; `pkg/engine/public_categories.go:17` | Use one request/build snapshot for each public/admin response builder, or prove the call is startup-only. Do not combine a single accessor here with separate live config/runtime reads deeper in the response path. |
| Scheduler iteration/admission | 4 | `pkg/scheduler/processing_loop.go:11`; `pkg/scheduler/automatic_due.go:20`; `pkg/scheduler/actions.go:45,71` | Use the SOW-0118 combined config/runtime snapshot at scheduler iteration/admission boundaries, or document intentionally latest-at-use behavior with file/line evidence. |
| Engine operation/run helpers | 15 | `pkg/engine/output_metadata.go:199,223,233,308`; `pkg/engine/public_series.go:22`; `pkg/engine/bootstrap_entries.go:167,189,238`; `pkg/engine/fileset_helpers.go:127`; `pkg/engine/query_history.go:101,131`; `pkg/engine/metadata.go:68`; `pkg/engine/runtime_ledger_loaders.go:235`; `pkg/engine/retention.go:37,66` | Use the operation/run snapshot captured at the engine work boundary, or prove startup-only/bootstrap-only behavior. Runtime-ledger loaders must preserve durable-file-as-source-of-truth semantics. |
| Provider and critical-infrastructure helpers | 7 | `pkg/engine/provider_selection.go:12,35`; `pkg/engine/critical.go:131,167,250,659,836` | Use the run/provider snapshot for provider selection, marker paths, and critical artifact generation, or prove the helper is already called from a coherent captured plan. |
| Path/publication helper projections | 5 | `pkg/engine/integrity_cache.go:382`; `pkg/engine/helpers.go:85,96,318`; `pkg/engine/web_batch.go:54` | Convert call sites to pass captured runtime/config projections into path helpers, or prove the helper is called only during startup/reload with lock-protected state. |
| Engine lifecycle helpers | 2 | `pkg/engine/engine.go:254,370` | Classify as reload/lifecycle/lockfile handling. Keep lock-protected or prove intentionally latest-at-use; do not route through long-running operation snapshots unless the caller becomes part of active work. |

Total single-accessor rows covered: 50.

## `cfg.Runtime` Downstream Hazard

Pre-implementation accessor calls that return `*config.Config` could still lead
to a reload race when the caller later read `cfg.Runtime`. The original hazard
was runtime overrides mutating `e.cfg.Runtime.WebDir` and
`e.cfg.Runtime.WebDirForIPSets` in place under the engine mutex. The final
implementation closes that mutation hazard by applying overrides to `e.runtime`
only, while operation/request/build code uses captured `Runtime` values or
captured feed-health policy for effective runtime state.

Pre-implementation accessor-derived `cfg.Runtime` consumer sites:

| Caller-chain class | Location | Required handling |
|---|---|---|
| Admin/request builder | `pkg/web/admin.go:688` | Use request snapshot `Runtime`, not `cfg.Runtime`. |
| Admin/request builder | `pkg/web/admin.go:949` | Use request snapshot `Runtime`, not `cfg.Runtime`. |
| Scheduler snapshot builder | `pkg/scheduler/snapshot_build.go:58` | Use scheduler iteration snapshot `Runtime`, not `cfg.Runtime`. |
| Scheduler snapshot builder | `pkg/scheduler/snapshot_build.go:110` | Use scheduler iteration snapshot `Runtime`, not `cfg.Runtime`. |
| Public/request builder | `pkg/engine/public_catalog.go:87` | Use request/build snapshot `Runtime`, not `cfg.Runtime`. |
| Public/request builder | `pkg/engine/feed_health.go:16` | Use request/build snapshot `Runtime`, not `cfg.Runtime`. |

The matching pre-implementation direct `e.cfg.Runtime` consumer sites are tracked in
`SOW-0118-caller-chain-classification.md`.

| Class | Location | Code |
|---|---|---|
| `single-accessor-review` | `pkg/web/server.go:102` | `		if strings.TrimSpace(eng.Runtime().PublicBaseURL) == "" {` |
| `mixed-generation-review` | `pkg/web/server_run.go:92` | `	integrityWebDir := outputDirFromOptions(eng.Runtime().BaseDir, choose(opts.WebDir, eng.Runtime().WebDir))` |
| `mixed-generation-review` | `pkg/web/admin.go:419` | `		if eng.Config().ArtifactByName(name) == nil {` |
| `mixed-generation-review` | `pkg/web/admin.go:535` | `	cfg := eng.Config()` |
| `mixed-generation-review` | `pkg/web/admin.go:546` | `		PublicBaseURL: strings.TrimSpace(eng.Runtime().PublicBaseURL),` |
| `mixed-generation-review` | `pkg/web/admin.go:623` | `	cfg := eng.Config()` |
| `mixed-generation-review` | `pkg/web/admin.go:632` | `	for _, item := range scheduler.BuildArtifactItems(cfg, eng.Runtime(), entries, runner.EnableAll(), time.Now().UTC()) {` |
| `mixed-generation-review` | `pkg/web/admin.go:687` | `	cfg := eng.Config()` |
| `mixed-generation-review` | `pkg/web/admin.go:948` | `	cfg := eng.Config()` |
| `single-accessor-review` | `pkg/engine/output_metadata.go:199` | `	cfg := e.Config()` |
| `single-accessor-review` | `pkg/engine/output_metadata.go:223` | `	rt := e.Runtime()` |
| `single-accessor-review` | `pkg/engine/output_metadata.go:233` | `	cfg := e.Config()` |
| `single-accessor-review` | `pkg/engine/output_metadata.go:308` | `	return lookupSourceForConfig(e.Config(), name)` |
| `single-accessor-review` | `pkg/engine/public_series.go:22` | `	return webChartsEntriesFromRuntime(e.Runtime())` |
| `mixed-generation-review` | `pkg/web/admin_status_light.go:24` | `	cfg := eng.Config()` |
| `mixed-generation-review` | `pkg/web/admin_status_light.go:28` | `		PublicBaseURL: strings.TrimSpace(eng.Runtime().PublicBaseURL),` |
| `mixed-generation-review` | `pkg/web/surface_routes.go:30` | `	outputDir := outputDirFromOptions(eng.Runtime().BaseDir, choose(opts.WebDir, eng.Runtime().WebDir))` |
| `mixed-generation-review` | `pkg/web/surface_routes.go:31` | `	ipsetsDir := filesDir(eng.Runtime().BaseDir, choose(opts.FilesDir, eng.Runtime().WebDirForIPSets))` |
| `mixed-generation-review` | `pkg/web/surface_routes.go:32` | `	runtime := eng.Runtime()` |
| `mixed-generation-review` | `pkg/web/surface_routes.go:44` | `		baseDir:   eng.Runtime().BaseDir,` |
| `mixed-generation-review` | `pkg/web/admin_manifest.go:112` | `		cfg := eng.Config()` |
| `mixed-generation-review` | `pkg/web/admin_manifest.go:122` | `		rt := eng.Runtime()` |
| `single-accessor-review` | `pkg/engine/public_catalog.go:74` | `	cfg := e.Config()` |
| `single-accessor-review` | `pkg/engine/query.go:149` | `	rt := e.Runtime()` |
| `single-accessor-review` | `pkg/engine/query.go:254` | `	cfg := e.Config()` |
| `single-accessor-review` | `pkg/engine/query.go:334` | `	rt := e.Runtime()` |
| `single-accessor-review` | `pkg/engine/query.go:362` | `	cfg := e.Config()` |
| `single-accessor-review` | `pkg/engine/query.go:367` | `	cfg := e.Config()` |
| `single-accessor-review` | `pkg/scheduler/processing_loop.go:11` | `	interval := time.Duration(r.eng.Runtime().ProcessingIntervalMinutes) * time.Minute` |
| `single-accessor-review` | `pkg/engine/provider_selection.go:12` | `	return preferredGeoProviderForConfig(e.Config())` |
| `single-accessor-review` | `pkg/engine/provider_selection.go:35` | `	return preferredASNProviderForConfig(e.Config())` |
| `mixed-generation-review` | `pkg/scheduler/download_loop.go:14` | `	workers := r.eng.Runtime().ParallelDownloads` |
| `mixed-generation-review` | `pkg/scheduler/download_loop.go:21` | `		snapshot := BuildSnapshot(r.eng.Config(), r.eng.Runtime(), r.eng.EntriesSnapshot(), r.enableAll, now)` |
| `mixed-generation-review` | `pkg/scheduler/download_loop.go:22` | `		artifactItems := BuildArtifactItems(r.eng.Config(), r.eng.Runtime(), r.eng.EntriesSnapshotWithArtifacts(), r.enableAll, now)` |
| `single-accessor-review` | `pkg/scheduler/automatic_due.go:20` | `		src := r.eng.Config().Sources[item.Name]` |
| `single-accessor-review` | `pkg/scheduler/actions.go:45` | `			names = config.SortedSourceNames(r.eng.Config())` |
| `single-accessor-review` | `pkg/scheduler/actions.go:71` | `			names = config.SortedSourceNames(r.eng.Config())` |
| `mixed-generation-review` | `pkg/scheduler/scheduler.go:77` | `		statePath:          filepath.Join(eng.Runtime().CacheDir, "scheduler-state.json"),` |
| `mixed-generation-review` | `pkg/scheduler/scheduler.go:203` | `		r.eng.Config(),` |
| `mixed-generation-review` | `pkg/scheduler/scheduler.go:204` | `		r.eng.Runtime(),` |
| `single-accessor-review` | `pkg/engine/engine.go:254` | `	currentRuntime := e.Runtime()` |
| `single-accessor-review` | `pkg/engine/engine.go:370` | `	return acquireLock(e.Runtime().LockFile)` |
| `single-accessor-review` | `pkg/engine/bootstrap_entries.go:167` | `	seedEntryFromArtifactConfigForRuntime(e.Runtime(), entry, name, artifact)` |
| `single-accessor-review` | `pkg/engine/bootstrap_entries.go:189` | `	seedEntryFromSourceConfigForRuntime(e.Runtime(), entry, name, src)` |
| `single-accessor-review` | `pkg/engine/bootstrap_entries.go:238` | `	return e.currentSetStatsForRuntime(e.Runtime(), name, src)` |
| `single-accessor-review` | `pkg/engine/effective_entry.go:31` | `	cfg := e.Config()` |
| `single-accessor-review` | `pkg/engine/feed_health.go:12` | `	cfg := e.Config()` |
| `single-accessor-review` | `pkg/engine/critical.go:131` | `	return criticalInfrastructureProvidersForConfig(e.Config())` |
| `single-accessor-review` | `pkg/engine/critical.go:167` | `	id := CriticalInfrastructureProviderSetIDForSnapshot(e.Config())` |
| `single-accessor-review` | `pkg/engine/critical.go:250` | `	path := CriticalInfrastructureProviderSetMarkerPath(e.Runtime())` |
| `single-accessor-review` | `pkg/engine/critical.go:659` | `	cfg := e.Config()` |
| `single-accessor-review` | `pkg/engine/critical.go:836` | `	cfg := e.Config()` |
| `single-accessor-review` | `pkg/engine/integrity_cache.go:382` | `		opts.WebDir = e.Runtime().WebDir` |
| `single-accessor-review` | `pkg/engine/fileset_helpers.go:127` | `	return hasBinaryLatestSetForRuntime(e.Runtime(), name)` |
| `single-accessor-review` | `pkg/engine/query_history.go:101` | `	rt := e.Runtime()` |
| `single-accessor-review` | `pkg/engine/query_history.go:131` | `	rt := e.Runtime()` |
| `single-accessor-review` | `pkg/engine/metadata.go:68` | `	rt := e.Runtime()` |
| `single-accessor-review` | `pkg/engine/public.go:40` | `	return isPublicFeedNameForConfig(e.Config(), name)` |
| `single-accessor-review` | `pkg/engine/public.go:105` | `	rt := e.Runtime()` |
| `single-accessor-review` | `pkg/engine/public.go:206` | `	cfg := e.Config()` |
| `single-accessor-review` | `pkg/engine/public.go:233` | `	cfg := e.Config()` |
| `single-accessor-review` | `pkg/engine/public.go:261` | `	cfg := e.Config()` |
| `single-accessor-review` | `pkg/engine/public.go:472` | `	return outputDirForRuntime(e.Runtime())` |
| `single-accessor-review` | `pkg/engine/public.go:516` | `	return configuredNamesForConfig(e.Config())` |
| `single-accessor-review` | `pkg/engine/runtime_ledger_loaders.go:235` | `	rt := e.Runtime()` |
| `single-accessor-review` | `pkg/engine/helpers.go:85` | `	return providerArchivePathForRuntime(e.Runtime(), name, src)` |
| `single-accessor-review` | `pkg/engine/helpers.go:96` | `	return finalPathForRuntime(e.Runtime(), name, output)` |
| `single-accessor-review` | `pkg/engine/helpers.go:318` | `	return isRedistributableForConfig(e.Config(), name)` |
| `single-accessor-review` | `pkg/engine/public_categories.go:17` | `	cfg := e.Config()` |
| `single-accessor-review` | `pkg/engine/retention.go:37` | `	rt := e.Runtime()` |
| `single-accessor-review` | `pkg/engine/retention.go:66` | `	rt := e.Runtime()` |
| `single-accessor-review` | `pkg/engine/web_batch.go:54` | `	return newWebPublishBatchForRuntime(e.Runtime())` |

## Post-Implementation Reconciliation - 2026-06-26

Post-change command:

```bash
rg -n 'ConfigRuntimePolicySnapshot\(|ConfigRuntimeSnapshot\(|eng\.Runtime\(\)|eng\.Config\(\)|r\.eng\.Runtime\(\)|r\.eng\.Config\(' pkg/scheduler pkg/web --glob '*.go' --glob '!**/*_test.go'
```

Remaining rows and classification:

| Class | Location | Status |
|---|---|---|
| Startup/server construction | `pkg/web/server.go:102` | Startup-only public base URL default. Safe for SOW-0118; SOW-0119 owns public route live rebinding. |
| Startup/server construction | `pkg/web/server_run.go:92` | Uses one `ConfigRuntimeSnapshot()` for startup integrity web-dir binding. Safe for SOW-0118; SOW-0119 owns live server route/root rebinding. |
| Admin request snapshot | `pkg/web/admin_status_light.go:24` | Uses combined `ConfigRuntimeSnapshot()` once for light response. |
| Admin request snapshot | `pkg/web/admin_manifest.go:112` | Uses combined `ConfigRuntimeSnapshot()` once for manifest builder. |
| Admin request snapshot | `pkg/web/admin.go:419,492,537,625,689,956` | Uses combined config/runtime/policy snapshots or config snapshots at request boundaries; inner builders receive captured `cfg`, `rt`, `policy`, and config-bound entries. |
| Scheduler iteration/admission | `pkg/scheduler/processing_loop.go:28` | `processingInterval()` uses `ConfigRuntimeSnapshot()` at timer creation and reset/wake boundaries, so processing cadence observes reloads at the next loop boundary without mixing generations. |
| Scheduler iteration/admission | `pkg/scheduler/download_loop.go:17` | Uses combined config/runtime/policy snapshot once per fetch-loop iteration. |
| Scheduler action/admission | `pkg/scheduler/actions.go:27,43,59` | Uses batched engine admission helpers for recheck, reprocess, and run actions; each batch captures one operation snapshot before queue decisions are produced. |
| Scheduler action/admission | `pkg/scheduler/automatic_due.go:26` | Uses the scheduler's supplied config snapshot directly for source classification instead of calling live engine accessors. |
| Scheduler startup | `pkg/scheduler/scheduler.go:77` | Startup state-file path; not active reload processing. |
| Scheduler request/status | `pkg/scheduler/scheduler.go:202,332` | Uses combined snapshot for scheduler cache rebuild and queue-status lookup. |
| Public route binding | `pkg/web/surface_routes.go:30` | Uses one `ConfigRuntimeSnapshot()` for startup route roots and cache limits. Live route-root rebinding after reload remains deferred to SOW-0119. |

Post-change `cfg.Runtime` command:

```bash
rg -n '\bcfg\.Runtime\b|\.cfg\.Runtime\b' pkg/engine pkg/scheduler pkg/web --glob '*.go' --glob '!**/*_test.go'
```

Remaining rows and classification:

| Location | Status |
|---|---|
| `pkg/engine/runtime.go:98` | Runtime construction from config; startup/reload construction code. |
| `pkg/engine/runtime.go:99` | Runtime construction from config; startup/reload construction code. |
| `pkg/engine/runtime_snapshot.go:37` | Feed-health policy derived from the captured config pointer inside the snapshot boundary. |
| `pkg/scheduler/snapshot_build.go:40` and `pkg/scheduler/snapshot_build.go:100` | Feed-health policy derived from caller-provided snapshot config inside scheduler snapshot and artifact-item construction. |

The original mutation hazard is closed: runtime overrides no longer mutate
`e.cfg.Runtime`; they update `e.runtime` only. Evidence:
`pkg/engine/runtime.go:299`.

Engine package accessor reconciliation:

- Remaining `e.Runtime()` / `e.Config()` calls inside `pkg/engine` are not part
  of scheduler/web mixed-generation builders. They are either thin public or
  compatibility wrappers that immediately delegate to `*ForRuntime`,
  `*ForConfig`, or `*WithSnapshot` variants, or standalone latest-at-use query
  helpers whose caller contract is "current engine view".
- Admitted run, download/artifact, integrity, metadata, comparison, entity, and
  retention paths use the operation snapshot or receive the captured snapshot
  from their caller. Evidence: `pkg/engine/run.go:227`,
  `pkg/engine/run_pipeline.go:44`, `pkg/engine/artifact_stage.go:74`,
  `pkg/engine/integrity_check.go:79`, `pkg/engine/metadata_write.go:70`,
  and `pkg/engine/entity_refresh_queue.go:610`.
- Locked engine snapshot helpers remain accepted direct readers because their
  public contract is to acquire one short engine view under `e.mu.RLock()`.
  Evidence: `pkg/engine/runtime_snapshot.go:27`,
  `pkg/engine/status_snapshot.go:21`,
  `pkg/engine/runtime_ledger_cache.go:36`,
  `pkg/engine/ip_context.go:334`, and
  `pkg/engine/scheduler_config_snapshot.go:33`.
