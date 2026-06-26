# SOW-0118 Caller-Chain Reachability Classification

Generated: 2026-06-26

Purpose: companion classification for the 285 `unsafe-convert` rows in
`SOW-0118-direct-read-inventory.md`. This is a maintainer/SOW artifact, not
public documentation.

## Scope

This file groups every current `unsafe-convert` direct-read hit by reachable
caller chain. It does not replace the line-level inventory; it tells the
implementation where to capture a coherent snapshot and which paths must be
proved startup-only, request-local, or operation-local.

## Summary

| Reachability class | Unsafe hits | Files | Snapshot plan |
|---|---:|---|---|
| Downloader/artifact acquisition FIFO | 65 | `download_stage.go`, `artifact_stage.go`, `artifact_paths.go`, `download_recovery.go`, `asn_url_resolver.go` | Capture one operation snapshot when the queued download or artifact item starts. Pass it through source fetch, artifact fetch, child materialization, path helpers, and ASN URL resolution. |
| Engine run pipeline and feed finalization | 69 | `run.go`, `run_pipeline.go`, `process.go`, `feed_body_stage.go`, `finalize.go`, `retention_update.go`, `merge_inputs.go`, `helpers.go`, `web_ipsets.go`, `public_series.go`, `history_stats.go`, `legacy_failure_bootstrap.go` | Capture one run snapshot after lane admission and before phase execution. Pass it through processor setup, merge input expansion, history/retention writers, finalize, web-ipsets sync, and helper paths. |
| Provider/heavy phases | 21 | `geoloc.go`, `asn.go`, `bogons.go`, `critical.go`, `critical_feed_writer.go` | Use the run snapshot for provider selection, provider caches, worker counts, critical context, and generated critical writer metadata. Heavy workers inherit the same snapshot for the whole phase/wave. |
| Metadata/comparison/insights/public artifact builders | 49 | `metadata_write.go`, `metadata.go`, `output_comparison.go`, `output_comparison_pair_ledger.go`, `unique_share.go`, `insights.go`, `home_aggregates.go`, `home_detail.go`, `home_summary.go`, `home_globe.go`, `public_url.go`, `public_compose.go`, `markdown.go`, `admin_manifest_builder.go` | Use a run snapshot for metadata/insights/comparison generation and a request/build snapshot for public/admin builders. Builder structs must receive the captured snapshot instead of reading engine fields later. |
| Integrity and repair | 36 | `integrity.go`, `integrity_check.go`, `integrity_payloads.go`, `integrity_recovery.go`, `home_aggregate_integrity.go` | Capture one integrity operation snapshot when the integrity refresh/recovery work is admitted. Payload helpers must receive snapshot config/runtime explicitly, including non-`e` aliases such as `eng.cfg`. |
| Entity/background/detail paths | 45 | `home_entity_builders.go`, `entity_integrity_refs.go`, `entity_feed_sidecar_build.go`, `bootstrap_entries.go`, `entry_timestamp_sanitize.go`, `entity_feed_sidecar_single.go`, `home_entity_detail_live.go`, `entity_artifacts_write.go`, `effective_entry.go`, `entity_integrity.go`, `entity_detail_selection.go`, `entity_artifacts.go` | Entity artifact/health/background work captures one operation snapshot per queue wave. Public detail builders capture one request/build snapshot. Startup/bootstrap helpers are either converted to explicit startup snapshots or proved startup-only with file/line evidence before closure. |

Total unsafe hits covered: 285.

## Implementation Notes

- The default treatment for every row in the line-level inventory remains:
  convert to an operation/request/build snapshot unless the row is proven
  startup-only, reload-only, lane-admission-only, intentionally latest-at-use,
  or already safe through a local snapshot object.
- Fixes should be batched by caller chain, not by individual line. A batch is
  complete only when the line-level inventory rows for that class either
  disappear from the same-failure scan or are reclassified with evidence.
- Pre-implementation `cfg.Runtime` policy reads require special handling even
  when the local code receives `cfg` from `Config()` instead of reading `e.cfg`
  directly. The original hazard was runtime overrides mutating `e.cfg.Runtime`
  in place under reload. The implementation closes that hazard by keeping
  overrides on `e.runtime` only, and callers must use captured `Runtime` or
  captured feed-health policy for effective runtime state.
- Pre-implementation `cfg.Runtime` consumer sites that must be converted or proved
  startup/reload-only:
  - direct engine reads: `pkg/engine/home_aggregates.go:126`,
    `pkg/engine/home_entity_builders.go:222`,
    `pkg/engine/integrity.go:266`, `pkg/engine/home_detail.go:160`, and
    `pkg/engine/home_detail.go:257`;
  - accessor-derived reads: `pkg/web/admin.go:688`,
    `pkg/web/admin.go:949`, `pkg/scheduler/snapshot_build.go:58`,
    `pkg/scheduler/snapshot_build.go:110`,
    `pkg/engine/public_catalog.go:87`, and
    `pkg/engine/feed_health.go:16`;
  - reload/runtime construction writes and reads:
    `pkg/engine/runtime.go:98`, `pkg/engine/runtime.go:99`,
    `pkg/engine/runtime.go:306`, and `pkg/engine/runtime.go:312`, which stay
    classified as startup/reload/runtime-construction code.
- `e.state` is intentionally not a snapshot field. It is a stable pointer set
  during engine construction, but functions that combine `e.state` with
  `cfg`/`runtime` still need the config/runtime side converted through the
  caller-chain snapshot.
- Old snapshot memory retention is expected after reload. It is bounded by the
  lifetime of already-admitted work that holds old config/runtime/downloader/
  provider/ledger/ASN-cache references.

## Post-Implementation Status - 2026-06-26

| Reachability class | Status | Evidence |
|---|---|---|
| Downloader/artifact acquisition FIFO | Converted to admitted-operation snapshots. | `pkg/scheduler/download_loop.go:17`; `pkg/engine/runtime_snapshot.go:12`. |
| Engine run pipeline and feed finalization | Converted to run/operation snapshots and config-bound entry snapshots. | `pkg/engine/run_pipeline.go`; `pkg/engine/query.go:371`; `pkg/engine/runtime_snapshot.go:12`. |
| Provider/heavy phases | Converted to run snapshots, including provider caches and worker-count runtime. | `pkg/engine/runtime_snapshot.go:12`; `pkg/engine/asn.go`; `pkg/engine/geoloc.go`; `pkg/engine/bogons.go`. |
| Metadata/comparison/insights/public artifact builders | Converted for run-time artifact generation and admin/request builders. | `pkg/engine/metadata_write.go`; `pkg/engine/home_aggregates.go`; `pkg/web/admin.go:535`. |
| Integrity and repair | Converted to admitted integrity-operation snapshots or short locked status snapshots. | `pkg/engine/integrity_check.go`; `pkg/engine/integrity_payloads.go`; `pkg/engine/status_snapshot.go:32`. |
| Entity/background/detail paths | Converted to per-wave operation snapshots for entity refresh/rebuild and request/build snapshots for detail builders. | `pkg/engine/entity_artifacts.go`; `pkg/engine/entity_surgical_refresh.go`; `pkg/engine/home_entity_builders.go`. |
| Public route-root binding | Startup construction now uses one config/runtime snapshot. Live route-root rebinding after reload remains tracked by derivative SOW-0119. | `pkg/web/surface_routes.go:30`; `.agents/sow/pending/SOW-0119-20260626-public-serving-runtime-rebind.md`. |

The old `cfg.Runtime` mutation model is no longer accurate after the
implementation. Runtime overrides are applied to `e.runtime` only; the config
object is no longer mutated to carry runtime directory overrides. Remaining
`cfg.Runtime` reads are recorded in
`SOW-0118-accessor-generation-inventory.md` and are limited to runtime
construction or feed-health policy derivation from caller-provided config
snapshots.
