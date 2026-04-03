# Architecture Posture Contract

## Status

This document is normative for internal implementation governance. It is not a
public product contract and it does not make package names part of the external
API.

The goal is to keep code-quality work measurable. Future refactors MUST improve
or intentionally preserve these metrics; they MUST NOT silently increase
architecture debt.

## Baseline Tool

Architecture posture is measured by:

- command: `go run ./tools/archposture -root .`
- baseline: `tools/archposture/testdata/posture_baseline.json`
- guard: `go test ./tools/archposture`

The baseline records current debt. The guard fails when new or worsened debt
appears without an explicit SOW decision.

The baseline MAY be updated only when the active SOW records why the changed
metric is acceptable.

## Measured Axes

The posture baseline measures:

- source file count and line count
- large files
- large Go functions and approximate branch complexity
- direct imports and transitive Go dependencies by package
- direct production/test `cache.Entry` pointer access
- direct production/test mutable cache-entry field writes
- heuristic matches for artifact-token substring classification
- web route registration count
- standalone-package invariants such as `pkg/iprange` importing no project
  packages

Current baseline highlights after SOW-0030 Phase 3 scheduler decomposition:

- source scope: 528 Go/TS/TSX files and 110,994 lines
- largest files:
  - `pkg/config/catalog_verify_test.go`: 1,677 lines
  - `pkg/cache/cache_test.go`: 1,443 lines
  - `pkg/engine/output.go`: 1,372 lines
  - `pkg/scheduler/scheduler_test.go`: 1,337 lines
  - `pkg/web/feature_test.go`: 990 lines
- largest functions:
  - `pkg/engine/entity_integrity.go` `(*Engine).CheckEntityArtifactsIntegrity`: 449 lines
  - `pkg/iprange/cli.go` `runCLIV4`: 381 lines
  - `pkg/iprange/cli6.go` `runCLIV6`: 374 lines
  - `pkg/config/catalog_verify_test.go` `TestCatalogSourcesComplete`: 314 lines
  - `pkg/config/config_test.go` `TestValidateCriticalMetadataContract`: 299 lines
- core package dependency posture:
  - `pkg/engine`: 133 files, 36,107 lines, 50 direct imports, 465 transitive dependencies
  - `pkg/web`: 39 files, 9,378 lines, 41 direct imports, 479 transitive dependencies
  - `pkg/scheduler`: 17 files, 3,838 lines, 21 direct imports, 466 transitive dependencies
  - `pkg/cache`: 7 files, 3,250 lines, 14 direct imports, 428 transitive dependencies
  - `pkg/iprange`: 0 project imports

Phase 4b reduced `pkg/engine/run.go` `(*Engine).RunOnce` below the
large-function threshold by moving existing behavior into explicit phase helpers
in `pkg/engine/run_pipeline.go`. Future changes MUST NOT grow `RunOnce` or the
new phase helpers back above the guard thresholds without an explicit SOW
decision.

Phase 2 reduced `pkg/web/server.go` `newSurfaceHandler` below the
large-function threshold by moving existing route registration behavior into
explicit route-family helpers in `pkg/web/routes.go`. The public route
inventory is unchanged at 51 `HandleFunc` registrations and 3 `Handle`
registrations. Future route changes MUST preserve public/admin listener
separation, raw-feed safety, redistributability checks, and stale critical
artifact rejection.

Phase 5 reduced the two largest admin UI files by moving existing behavior into
local owner files:

- `ui/src/components/admin/feeds-table.tsx`: 1,298 lines to 368 lines
- `ui/src/components/admin/feed-modal.tsx`: 1,295 lines to 68 lines

The extracted admin files keep separate ownership for table state/model,
filter chips, table body/rows, modal hero/actions, identity, schedule/timeline,
manifest, diagnostics, and modal primitives. Future UI work MUST preserve URL
state, sorting/filtering semantics, admin action invalidation, and visual
structure when moving these pieces.

Phase 1b initial cache replacement migration removed production full-entry
replacement through mutable `Entry()` pointers. Complete synthesized entries
must be stored with `cache.State.ReplaceEntry()`, which copies slice fields and
normalizes the configured entry name. Future cache work MUST continue toward
semantic lifecycle APIs instead of adding generic field setters or generic
mutation buckets.

Phase 1b failure-state migration moved download-failure counter transitions
behind `cache.Entry.RecordDownloadFailure()` and
`cache.Entry.ClearDownloadFailure()`. Future lifecycle migrations should follow
this pattern: a small method that names the domain transition, preserves the
existing JSON schema, and keeps engine code from writing unrelated cache fields.

Phase 1b run-attempt and critical-overlap migrations moved attempt status,
processing-duration, and overlap-tier summary writes behind named cache entry
methods. These methods deliberately copy tier slices so callers cannot mutate
cache state after storing.

Phase 1b unique-share migration moved the unique-share percent clamp and sample
count write behind `cache.Entry.SetUniqueShare()`.

Phase 1b download-preflight migration moved persisted download status vocabulary
to `pkg/cache`, kept engine compatibility aliases, and moved checked timestamp,
downloading, disabled, and missing-environment URL-template state behind named
cache entry methods.

Phase 1b download-result migration moved the remaining `download_stage.go`
direct cache field writes behind named cache entry methods for result status,
source date, and resolved provider URL state. `download_stage.go` now has zero
direct `cache.Entry` field writes.

Phase 1b bootstrap/config seeding migration moved
`pkg/engine/bootstrap_entries.go` direct cache field writes behind named cache
entry methods for source/artifact config snapshots, restored artifact evidence,
history/bootstrap timestamps, restored set stats, bootstrap finalization, and
critical-infrastructure content-hash refresh. `bootstrap_entries.go` now has
zero direct `cache.Entry` field writes; the engine still owns config
interpretation, path selection, filesystem probing, and set parsing.

Phase 1b ASN/geolocation provider migration moved `pkg/engine/asn.go` and
`pkg/engine/geoloc.go` direct cache field writes behind named cache entry
methods for provider source metadata, provider load statuses, provider
freshness/stats evidence, and updated-versus-stale load completion. Both files
now have zero direct `cache.Entry` field writes; the engine still owns provider
selection, archive extraction, parsing/opening, stats collection, fan-out, and
logging.

Phase 1b source processing/finalization migration moved `pkg/engine/process.go`,
`pkg/engine/finalize.go`, and the direct writes in `pkg/engine/helpers.go`
behind named cache entry methods for source processing metadata, processing
statuses, finalized set evidence, finalized source metadata, completion status,
and shared stats updates. These engine files now have zero direct `cache.Entry`
field writes; the engine still owns body claiming, parsing, final-set writing,
kernel apply, retention, history ledger append, rotation stats, publication,
and logging.

Phase 1b artifact-materialization migration moved
`pkg/engine/artifact_stage.go` direct cache field writes behind existing
download lifecycle methods plus a named artifact-child materialization method.
`artifact_stage.go` now has zero direct `cache.Entry` field writes; the engine
still owns artifact lookup, DroneBL fetch, local file download, child spec
selection, materialized child output, staged promotion, and scheduling
decisions.

Phase 1b runtime-ledger migration moved
`pkg/engine/runtime_ledger_cache.go` direct cache field writes behind a named
history ledger stats snapshot method. `runtime_ledger_cache.go` now has zero
direct `cache.Entry` field writes; the engine still owns ledger CSV parsing,
duplicate timestamp handling, runtime tail caching, and cadence calculation.

Phase 1b timestamp-repair migration moved
`pkg/engine/entry_timestamp_sanitize.go` direct cache field writes behind a
named invalid-timestamp repair method. `entry_timestamp_sanitize.go` now has
zero direct `cache.Entry` field writes; the engine still owns disk/history
evidence discovery and passes latest/first observed timestamps to cache.

Phase 1b rotation-stats migration moved `pkg/engine/rotation_stats.go` direct
cache field writes behind grouped rotation/change-ratio summary methods.
`rotation_stats.go` now has zero direct `cache.Entry` field writes; the engine
still owns size/churn series reading, percentile calculation, rounding, and
empty-input decisions.

Phase 1b legacy failure-start migration moved
`pkg/engine/legacy_failure_bootstrap.go` direct cache field writes behind a
narrow legacy-import method that records imported failure start timestamps
without incrementing current failure counters. All production direct mutable
cache-entry field writes outside `pkg/cache` have now been removed.

Phase 3 scheduler decomposition reduced `pkg/scheduler/scheduler.go` from
1,474 lines to 276 lines without changing package boundaries or runtime
semantics. Current line count: 289 lines. Scheduler ownership now lives in same-package concern files:

- `actions.go`: manual/admin action admission
- `automatic_due.go`: automatic source/artifact due policy
- `download_loop.go`: fetch loop and download execution
- `processing_loop.go`: processing loop, batch completion, failure retry, and
  promotion handling
- `queue_admission.go`: download/processing queue admission, dedupe, active
  deferral, refetch release, and parent-input settling
- `recovery.go`: staged-work recovery, provider-default drift enqueue, and
  provider wave fan-out
- `snapshot_build.go`: scheduler snapshot, due calculation, source kind, and
  persisted snapshot comparison helpers

The scheduler's single shared `stateMu` lock remains the authority for
download and processing queue maps. Future scheduler changes must preserve that
invariant unless a later SOW explicitly replaces the queue ownership model.

## Cache Mutation Inventory

Current cache mutation debt:

- production `Entry()` calls: 31
- test `Entry()` calls: 85
- production mutable cache-entry field writes: 29
- test mutable cache-entry field writes: 336
- production full-entry replacements: 0

Production mutation files by count:

- `pkg/cache/legacy.go`: 29

The remaining production mutable cache-entry field writes are in the cache
package itself, not in engine owners.

Semantic mutation categories:

- bootstrap and config seeding
- downloader attempt, success, failure, and prepared-body state
- artifact fetch and artifact-child materialization state
- feed processing, finalization, and retention state
- ASN and geolocation provider state
- historical ledger, rotation, change-ratio, and unique-share statistics
- timestamp repair and legacy failure-state import
- publication metadata and min/max counters
- critical-infrastructure overlap tiers

Future cache ownership work MUST design mutation APIs around these semantic
transitions. It MUST NOT create one setter per field unless the SOW explicitly
accepts that surface.

## Engine Pipeline Lifecycle Map

`(*Engine).RunOnce` currently coordinates the central pipeline lifecycle through
explicit helpers in `pkg/engine/run_pipeline.go`. Its phase order and resource
dependencies are:

1. run lifecycle
   - normalize run reason
   - start telemetry span
   - mark run active
   - defer run-end state and cache save even on early abort
2. preflight
   - ensure directories
   - apply configured renames/deletes when requested
3. source processing
   - process selected ordinary sources first
   - process history derivatives after parents
   - process merges after inputs
   - populate `Report.Updated`, `Report.Skipped`, and `Report.Failed`
4. heavy-trigger planning
   - detect ordinary updates
   - detect selected ASN/geolocation database sources
   - detect critical provider-set drift
   - detect configured ASN/geolocation default-provider drift
   - decide `skipHeavy`
   - compute scoped/global fan-out for reprocess and provider drift
5. publish workspace setup
   - create web publish batch
   - create heavy set cache
   - create entity publish batch only when entity sidecars are staged
6. critical-only path
   - load critical infrastructure sources
   - write aggregate and per-provider critical artifacts
   - mark stale critical artifacts for deletion
7. full heavy path
   - process geolocation providers
   - write country comparison files
   - load bogon providers
   - write bogon comparison files
   - build bogon union for ASN unknown/bogon split
   - process ASN providers
   - write ASN comparison files
   - load critical infrastructure sources
   - write critical infrastructure files
   - mark stale critical artifacts
   - stage feed entity sidecars using loaded geolocation and ASN providers
8. metadata and insights
   - write feed metadata, comparison, retention, and public indexes
   - write insights after heavy artifacts when heavy work ran
9. publication
   - run optional pre-publish hook
   - apply explicit generated-file timestamps
   - publish staged public web batch
   - publish staged entity artifacts under the entity artifact mutex
   - copy updated raw public sets to the web mirror
   - sync generated-file ledger
10. markers and final cache state
   - write critical provider-set marker when needed
   - write provider-default marker when needed
   - save cache state

Resource ownership that Phase 4 work MUST preserve:

- web publish batch cleanup
- entity publish batch cleanup
- heavy set cache close
- geolocation prepared provider lifecycle
- bogon dataset lifecycle
- ASN database lifecycle
- critical dataset lifecycle
- generated-file timestamp application before publish
- marker writes only after successful publication
- cache save on early abort and final success

`pkg/engine/run_pipeline.go` owns the behavior-preserving phase boundaries:

- `processRunSources`
- `buildPipelineRunPlan`
- `runHeavyPhases`
- `runCriticalOnlyPhase`
- `runFullHeavyPhases`
- `writeRunMetadataAndInsights`
- `publishRunArtifacts`

`pkg/engine/output.go` remains part of the same pipeline ownership problem
because it contains public comparison writers and generated-file behavior used
by the pipeline.

## Review Rules

Architecture posture review MUST check:

- no new large file or large function without a SOW-recorded reason
- no increase in direct production cache mutation without a SOW-recorded reason
- no new artifact-token substring classification without a SOW-recorded reason
- `pkg/iprange` still imports no project packages
- moved public writers preserve deliberate logical mtimes
- route refactors preserve public/admin listener separation and raw-feed safety
- scheduler refactors preserve the shared queue-lock invariant unless a new
  design replaces it
