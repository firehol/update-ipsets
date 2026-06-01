# Telemetry Reference

You will learn the runtime fields and OpenTelemetry metrics that update-ipsets exposes.

## Where telemetry appears

The admin status API and OpenTelemetry are related but not identical.

| Surface | Location | Meaning |
|---------|----------|---------|
| Admin status | `GET /api/v1/admin/status` | Point-in-time JSON snapshot for operators |
| Admin scheduler counters | `metrics` | Monotonic scheduler counters and latest batch timings |
| Admin engine timings | `engine.current_metrics`, `engine.last_metrics`, `engine.lifetime_metrics.operations` | Run and operation timings captured by the processing engine |
| Admin engine counters | `engine.lifetime_metrics.counters` | Engine, downloader-status, public HTTP, admin HTTP, and entity counters |
| Admin queue state | `queues` | Waiting, active, deferred, and recently transitioned work |
| Admin system state | `system` | Go runtime, process, disk, CPU, I/O, and file-descriptor snapshots |
| OpenTelemetry | OTLP metrics, traces, logs | Continuous cumulative counters, byte counters, duration histograms, spans, and logs |

OpenTelemetry counters are cumulative. Any operation recorded with byte or duration data can also emit:

- `<metric>.bytes` as a cumulative byte counter.
- `<metric>.duration_ms` as a duration histogram.

## Admin scheduler counters

These fields appear under `metrics` in the admin status response.

| Field | Meaning |
|-------|---------|
| `download_enqueued` | Items admitted to the download queue |
| `download_deferred` | Download items deferred because inputs are not settled |
| `download_started` | Download items started by workers |
| `download_finished` | Download items completed by workers |
| `processing_enqueued` | Items admitted to the processing queue |
| `processing_requeued` | Processing items requeued for another pass |
| `processing_batches_started` | Processing batches started |
| `processing_batches_completed` | Processing batches completed |
| `processing_items_started` | Total items included in started processing batches |
| `max_download_waiting` | Highest observed download queue depth |
| `max_processing_waiting` | Highest observed processing queue depth |
| `last_batch_size` | Number of items in the latest processing batch |
| `last_batch_duration_ms` | Duration of the latest completed processing batch |
| `snapshot_persist_errors` | Scheduler snapshot persistence failures |
| `operations` | Scheduler operation timing rows with `name`, `count`, `total_ms`, `avg_ms`, and `max_ms` |

## Scheduler operation timings

These operation names can appear in the admin status `metrics.operations` rows.
They are admin snapshot timings, not separate OpenTelemetry metric names.

| Operation name | Meaning |
|----------------|---------|
| `scheduler.fetch_and_stage` | Time spent fetching and staging one downloader item |
| `scheduler.promote_committed_downloads` | Time spent promoting staged provider/artifact inputs before publishing |
| `scheduler.run_once` | Time spent in one processing-engine run for a scheduler batch |
| `scheduler.processing_batch_total` | Total wall time for a processing batch, including success or failure handling |

## Admin system fields

These fields appear under `system`. They are snapshots, not monotonic counters.

| Field | Meaning |
|-------|---------|
| `uptime` | Daemon uptime |
| `go_version`, `goos`, `goarch` | Go runtime and platform |
| `goroutines` | Current goroutine count |
| `heap_alloc`, `heap_sys`, `heap_inuse`, `stack_inuse`, `sys` | Go runtime memory statistics in bytes |
| `num_gc`, `last_gc_unix`, `gc_pause_total_ns` | Garbage-collection statistics |
| `disk_free` | Free space string for the configured runtime disk |
| `rss_kb`, `vms_kb`, `data_kb` | Process memory from the operating system, in KiB |
| `cpu_user_seconds`, `cpu_system_seconds`, `cpu_total_seconds` | Process CPU usage |
| `proc_read_bytes`, `proc_write_bytes`, `proc_cancelled_write_bytes` | Process I/O byte counters |
| `proc_read_syscalls`, `proc_write_syscalls` | Process I/O syscall counters |
| `open_fds` | Current open file descriptors |

## Download metrics

| Metric | Surface | Meaning |
|--------|---------|---------|
| `download.queued` | Admin `metrics.download_enqueued`, OpenTelemetry | Items admitted to the scheduler download queue |
| `download.deferred` | Admin `metrics.download_deferred`, OpenTelemetry | Items deferred because inputs are not ready |
| `download.started` | Admin `metrics.download_started`, OpenTelemetry | Download worker starts |
| `download.finished` | Admin `metrics.download_finished`, OpenTelemetry | Download worker completions |
| `download.fetch` | OpenTelemetry | Downloader fetch attempts |
| `download.fetch.bytes` | OpenTelemetry | Response bytes from downloader fetches |
| `download.fetch.duration_ms` | OpenTelemetry | Downloader fetch duration histogram |
| `download.ok` | OpenTelemetry | Fetches that returned new content |
| `download.not_modified` | OpenTelemetry | Fetches where upstream returned not-modified |
| `download.same` | OpenTelemetry | Fetches where content matched the current body |
| `download.skipped` | OpenTelemetry | Fetches skipped by the downloader |
| `download.failed` | OpenTelemetry | Fetches that produced a downloader failure result |
| `download.error` | OpenTelemetry | Fetches that ended before a downloader result was available |
| `download.http_status.<code>` | Admin engine counters, OpenTelemetry | HTTP response status counts |
| `download.status.<status>` | Admin engine counters, OpenTelemetry | Scheduler decision status counts |
| `download.processing_names` | Admin engine counters, OpenTelemetry | Number of processing names produced by download decisions |

`download.status.<status>` can include `skipped`, `disabled`, `failed`, `download_failed`, `missing_env`, `url_resolve_failed`, `not_modified`, `same`, `downloaded`, `empty`, `prepare_failed`, `history_snapshot_failed`, and `materializing`.

## Engine and scheduler metrics

| Metric | Surface | Meaning |
|--------|---------|---------|
| `engine.run` | OpenTelemetry | Processing runs |
| `engine.run.duration_ms` | OpenTelemetry | End-to-end processing run duration |
| `engine.queued` | Admin `metrics.processing_enqueued`, OpenTelemetry | Items admitted to the processing queue |
| `engine.requeued` | Admin `metrics.processing_requeued`, OpenTelemetry | Items returned to the processing queue |
| `engine.batch.started` | Admin `metrics.processing_batches_started`, OpenTelemetry | Processing batch starts |
| `engine.batch.completed` | Admin `metrics.processing_batches_completed`, OpenTelemetry | Processing batch completions |
| `engine.batch.completed.duration_ms` | OpenTelemetry | Processing batch duration histogram |
| `snapshot_persist_errors` | Admin `metrics.snapshot_persist_errors` | Snapshot persistence failures |
| `engine.latest_set.binary_open` | Admin engine counters/operations, OpenTelemetry | Latest binary set opens |
| `engine.latest_set.text_parse` | Admin engine counters/operations, OpenTelemetry | Latest text set parses |

Phase metrics use `engine.<phase>` and `engine.<phase>.duration_ms`. Current phases are:

- `engine.preflight`
- `engine.sources`
- `engine.geoip`
- `engine.bogons`
- `engine.critical_infrastructure`
- `engine.asn`
- `engine.entities`
- `engine.metadata`
- `engine.insights`
- `engine.publish`

## Processing operation timings

These operation names appear in admin engine timing snapshots. Most also appear as OpenTelemetry duration histograms with `.duration_ms`.

Aggregate comparison timings also emit the metric name as a counter and `<metric>.aggregate.duration_ms` as the OpenTelemetry duration histogram.

| Metric | Meaning |
|--------|---------|
| `sources.parse_feed_body` | Parse and normalize a downloaded feed body |
| `sources.finalize` | Commit a processed source |
| `sources.finalize.kernel_apply` | Apply kernel optimization during finalization |
| `sources.finalize.write_latest` | Write latest committed source body |
| `sources.finalize.write_text` | Write text output |
| `sources.finalize.append_history` | Append source history |
| `sources.finalize.observe_history` | Observe history statistics |
| `sources.update_retention` | Update retention artifacts |
| `sources.refresh_rotation` | Refresh rotation statistics |
| `metadata.write_comparison_files` | Write comparison metadata files |
| `metadata.comparison_prepare_sets` | Prepare sets for comparison work |
| `metadata.comparison_pair_overlap` | Aggregate pair-overlap timing |
| `metadata.comparison_pair_skipped` | Aggregate skipped-pair timing |
| `metadata.comparison_merge_rows` | Aggregate comparison row merge timing |
| `metadata.update_unique_shares` | Update unique-share metadata |
| `metadata.write_public_metadata_files` | Write public metadata |
| `metadata.write_per_feed_outputs` | Write per-feed public outputs |
| `metadata.write_indexes` | Write public indexes |
| `metadata.write_git_artifacts` | Write Git-oriented artifacts when enabled |
| `metadata.write_home_aggregates` | Write homepage aggregate artifacts |

## Comparison counters

These counters appear in admin engine counters and OpenTelemetry.

| Metric | Meaning |
|--------|---------|
| `metadata.comparison_pair_candidates` | Candidate pairs considered for comparison |
| `metadata.comparison_pair_overlap` | Pairs that produced overlap rows |
| `metadata.comparison_pair_skipped` | Pairs skipped by comparison logic |
| `metadata.comparison_pair_skipped_empty` | Pairs skipped because one side was empty |
| `metadata.comparison_pair_skipped_range` | Pairs skipped by range filtering |
| `metadata.comparison_pair_skipped_prefix` | Pairs skipped by prefix filtering |
| `metadata.comparison_zero_rows_removed` | Zero-value comparison rows removed during cleanup |

## Entity artifact metrics

These metrics cover country and ASN artifact refresh, repair, sidecar, and public detail work.

| Metric or prefix | Meaning |
|------------------|---------|
| `entity.refresh.target_feeds` | Feeds selected for entity refresh |
| `entity.refresh.full_rebuild_fallback` | Entity refreshes that fell back to a full rebuild |
| `entity.refresh.affected_countries` | Countries touched by a refresh |
| `entity.refresh.affected_asns` | ASNs touched by a refresh |
| `entity.refresh.country_unchanged` | Country artifacts left unchanged |
| `entity.refresh.asn_unchanged` | ASN artifacts left unchanged |
| `entity.refresh.country_materialize` | Country artifact materialization duration |
| `entity.refresh.asn_materialize` | ASN artifact materialization duration |
| `entity.refresh.country_patch` | Country artifact patch duration |
| `entity.refresh.asn_patch` | ASN artifact patch duration |
| `entity.refresh.country_sidecar_read` | Country sidecar reads |
| `entity.refresh.asn_sidecar_read` | ASN sidecar reads |
| `entity.refresh.country_index_read` | Country index reads |
| `entity.refresh.asn_index_read` | ASN index reads |
| `entity.refresh.*_write` | Country, ASN, feed-sidecar, and index writes during refresh |
| `entity.refresh.*_touch` | Mtime-only refresh touches for unchanged entity artifacts |
| `entity.sidecar_build.*` | Per-feed sidecar build counters |
| `entity.sidecar_stage.unchanged_feed` | Sidecar stage skipped unchanged feed |
| `entity.sidecar_stage.unchanged_feed_touch` | Mtime-only touch for an unchanged sidecar-stage feed |
| `entity.output_view.*` | Public country/ASN JSON cache, read, and decode counters |
| `entity.repair.*` | Selected entity repair counters, writes, and touches |
| `entity.repair_feed_scan.*` | Repair scan counters over entity sidecars |
| `entity.integrity_startup_repair_deferred` | Startup integrity repair deferred to background work |
| `entity.integrity_startup_repair_deferred_after_revalidation` | Startup repair deferred after the plan was revalidated |
| `entity.integrity_repair.stale_plan_skipped` | Stale repair plans skipped |
| `entity.writer_lock_wait` | Wait time for the entity writer lock |
| `entity.writer_lock_hold` | Hold time for the entity writer lock |

## Background task metrics

These metrics appear in admin engine counters/operations and OpenTelemetry.

| Metric | Meaning |
|--------|---------|
| `background.tasks.started` | Background tasks started |
| `background.tasks.completed` | Background tasks completed |
| `background.tasks.failed` | Background tasks failed |
| `background.worker_wait` | Time spent waiting for a background worker slot |

## HTTP and admin metrics

These counters and timings appear in admin engine metrics and OpenTelemetry.

| Metric | Meaning |
|--------|---------|
| `http.admin_status` | Admin status responses |
| `http.admin_status.build` | Admin status build duration |
| `http.admin_status.write_json` | Admin status JSON write duration |
| `http.admin_status.total` | Total admin status request duration |
| `http.admin_feeds` | Admin feeds-list responses |
| `http.admin_feeds.build` | Admin feeds-list build duration |
| `http.admin_feeds.write_json` | Admin feeds-list JSON write duration |
| `http.admin_feeds.total` | Total admin feeds-list request duration |
| `admin.entity_integrity_check` | Entity integrity check requests |
| `http.home_summary.requests` | Home summary requests |
| `http.home_summary.request` | Home summary request duration |
| `http.home_summary.eligible_feeds` | Eligible feeds counted for home summary |
| `http.home_summary.contributing_feeds` | Contributing feeds counted for home summary |
| `http.home_globe.requests` | Home globe requests |
| `http.home_globe.request` | Home globe request duration |
| `http.home_globe.eligible_feeds` | Eligible feeds counted for home globe |
| `http.home_globe.contributing_feeds` | Contributing feeds counted for home globe |
| `http.home_aggregates.read` | Home aggregate file reads |
| `engine.country_comparison_json_read` | Country comparison JSON reads |
| `engine.country_comparison_json_load` | Country comparison JSON load duration |
| `http.compare_set.requests` | Compare-set API requests |
| `http.compare_set.request` | Compare-set request duration |
| `http.compare_set.target_open` | Compare-set target opens |
| `http.compare_set.candidates` | Compare-set candidates considered |
| `http.compare_set.candidate_open` | Compare-set candidate opens |
| `http.entity_artifact.country_index_hit` | Country index artifact cache hits |
| `http.entity_artifact.country_index_miss` | Country index artifact cache misses |
| `http.entity_artifact.country_detail_hit` | Country detail artifact cache hits |
| `http.entity_artifact.country_detail_miss` | Country detail artifact cache misses |
| `http.entity_artifact.asn_index_hit` | ASN index artifact cache hits |
| `http.entity_artifact.asn_index_miss` | ASN index artifact cache misses |
| `http.entity_artifact.asn_detail_hit` | ASN detail artifact cache hits |
| `http.entity_artifact.asn_detail_miss` | ASN detail artifact cache misses |

## Cache, config, processor, and file metrics

| Metric | Meaning |
|--------|---------|
| `cache.load` | Cache load attempts |
| `cache.load.error` | Cache load failures |
| `cache.save` | Cache saves |
| `cache.save.error` | Cache save failures |
| `config.load` | Single config file loads |
| `config.load.error` | Config load failures |
| `config.load_directory` | Config directory loads |
| `processor.run` | In-memory processor pipeline runs |
| `processor.step` | In-memory processor steps |
| `processor.stream` | Streaming processor pipeline runs |
| `processor.stream.segment` | Streaming processor segment work |
| `processor.stream.step` | Streaming processor steps |
| `processor.temp.write` | Temporary processor writes |
| `file.copy` | File copy operations |
| `file.write_atomic` | Atomic file writes |

## iprange metrics

These OpenTelemetry metrics track IP set primitive operations.

| Metric | Meaning |
|--------|---------|
| `iprange.load.text` | Text-format set loads |
| `iprange.load.binary` | Binary FileSet loads |
| `iprange.save.text` | Text-format set writes |
| `iprange.save.binary` | Binary FileSet writes |
| `iprange.add.ops` | Address/range additions |
| `iprange.optimize.ops` | Set optimization operations |
| `iprange.merge.ops` | Union/merge operations |
| `iprange.union.ops` | Union iterator operations |
| `iprange.exclude.ops` | Exclude operations |
| `iprange.intersect.ops` | Intersect operations |
| `iprange.diff.ops` | Diff operations |
| `iprange.compare.ops` | Pairwise comparison operations |
| `iprange.overlap.ops` | Overlap-count operations |
| `iprange.count_unique.ops` | Unique-IP counting operations |
| `iprange.contains.ops` | Membership checks |
| `iprange.binary.searches` | Binary-search lookups |

## Computing rates

Most counters are monotonic over the daemon lifetime. To compute rates, sample twice and divide by elapsed seconds:

```text
rate = (counter_t2 - counter_t1) / (t2 - t1)
```

Use admin status for spot checks. Use OpenTelemetry for durable dashboards, alerting, and history.
