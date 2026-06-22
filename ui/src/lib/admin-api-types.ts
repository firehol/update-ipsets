import type { FeedHealthSnapshot, MergeInputState } from "./api-types";

/** Admin feed row from /api/v1/admin/feeds. Field names mirror
 *  pkg/web/admin.go:adminFeed. 100% coverage of cache.Entry +
 *  config.Source so the feed modal can render without a second fetch. */
export interface AdminFeed {
  name: string;
  kind: string;
  uses?: string[];
  category: string;
  hidden?: boolean;
  enabled: boolean;
  status: string;
  health: FeedHealthSnapshot;
  last_status: string;
  last_status_label?: string;
  last_run_reason?: string;
  last_processing_ms?: number;
  last_error?: string;
  last_problem_class?: AdminProblemClass;
  last_check: number;
  last_update: number;
  processed_date: number;
  started_date: number;
  next_check: number;
  clock_skew_seconds?: number;
  entries: number;
  entries_min?: number;
  entries_max?: number;
  unique_ips: number;
  ips_min?: number;
  ips_max?: number;
  version?: number;
  avg_update_mins?: number;
  min_update_mins?: number;
  max_update_mins?: number;
  download_failures: number;
  frequency_minutes: number;
  /** Human-readable scheduler state. Examples: "never checked",
   *  "12/30 mins passed, next check in 18 mins (base 30 mins)",
   *  "next check in 150 mins (base 30 mins)" (backoff active),
   *  "due now". Exposes the current retry / backoff multiplier. */
  scheduler_detail?: string;
  url?: string;
  public_url?: string;
  ipv?: string;
  hash?: string;
  output?: string;
  processor_raw?: string;
  downloader?: string;
  downloader_options?: string;
  history_minutes?: number[];
  accept_empty?: boolean;
  maintainer?: string;
  maintainer_url?: string;
  info?: string;
  license?: string;
  attribution?: string;
  redistributable?: boolean;
  file?: string;
  source?: string;
  derived_from?: string[];
  merge_included?: MergeInputState[];
  merge_subtracted?: MergeInputState[];
  merge_excluded?: MergeInputState[];
}

export type AdminProblemClass = "downloader" | "processing";

export interface AdminQueueItem {
  name: string;
  kind?: "normal" | "recovered_artifact" | string;
  reason?: string;
  queued_at?: string;
  status?: string;
  status_label?: string;
  problem_class?: AdminProblemClass;
  detail?: string;
  blocked?: boolean;
  blocked_parents?: string[];
}

export interface AdminActiveQueueItem {
  name: string;
  kind?: "normal" | "recovered_artifact" | string;
  reason?: string;
  started_at?: string;
  status?: string;
  status_label?: string;
  problem_class?: AdminProblemClass;
  detail?: string;
}

export interface AdminActiveOperation {
  operation: string;
  phase?: string;
  feed?: string;
  stage?: string;
  unit: string;
  started_at?: string;
  elapsed_ms: number;
  current: number;
  total: number;
  completion_pct: number;
  rate_per_second: number;
  counters?: Record<string, number>;
}

export interface AdminRunBatch {
  total: number;
  completed: number;
  active: number;
  pending: number;
  names?: string[];
  completed_names?: string[];
  active_names?: string[];
  pending_names?: string[];
  source_total?: number;
  source_completed?: number;
  history_total?: number;
  history_completed?: number;
  merge_total?: number;
  merge_completed?: number;
  started_at?: string;
}

export interface AdminRunPhasePlan {
  phases?: string[];
  current?: string;
  current_position?: number;
  total?: number;
  final: boolean;
}

export interface AdminArtifact {
  name: string;
  type: string;
  enabled: boolean;
  status: string;
  last_status: string;
  last_status_label?: string;
  last_error?: string;
  last_problem_class?: AdminProblemClass;
  last_check: number;
  last_update: number;
  next_check: number;
  frequency_minutes: number;
  download_failures: number;
  scheduler_detail?: string;
  info?: string;
  maintainer?: string;
  maintainer_url?: string;
  child_feeds?: string[];
}

export interface HealthTransition {
  feed: string;
  from_class: string;
  to_class: string;
  at: string;
}

export type IntegrityRecoveryAction = "recheck" | "reprocess";
export type IntegrityCacheState =
  | "cold"
  | "fresh"
  | "stale"
  | "refresh_queued"
  | "refresh_running";

export interface AdminLaneTicket {
  id: string;
  kind?: string;
  component?: string;
  queued: boolean;
  coalesced: boolean;
  state: "queued" | "active" | "completed" | "failed" | "canceled" | "skipped";
}

/** A single row from /api/v1/admin/integrity. One finding per feed whose
 *  last successful local publication no longer matches the current on-disk
 *  public outputs. Mirrors pkg/engine/integrity.go:IntegrityFinding. */
export interface IntegrityFinding {
  feed: string;
  source_path: string;
  /** RFC3339 timestamp of the committed canonical feed body. */
  source_mtime: string;
  /** RFC3339 timestamp of the committed canonical feed body, explicit name. */
  source_file_mtime: string;
  /** RFC3339 timestamp of the successful finalize() reference time. */
  processed_at: string;
  missing_files?: string[];
  stale_files?: string[];
  malformed_files?: string[];
  blocked_feeds?: string[];
  recovery_action?: IntegrityRecoveryAction;
  recovery_targets?: string[];
  reason: string;
}

/** Envelope for GET /api/v1/admin/integrity. */
export interface IntegrityReport {
  include_archived?: boolean;
  status: "clean" | "issues" | "in_progress";
  generation?: number;
  cache_state?: IntegrityCacheState;
  running: boolean;
  startup_scan_running?: boolean;
  queued?: boolean;
  coalesced?: boolean;
  ticket?: AdminLaneTicket;
  last_started?: string;
  last_ended?: string;
  checked_at?: string;
  last_error?: string;
  count: number;
  findings: IntegrityFinding[];
}

/** Envelope for POST /api/v1/admin/integrity/reprocess. Returns "clean"
 *  with count=0 when nothing needs recovery, or "scheduled" with the
 *  class-split recovery targets that were queued. */
export interface IntegrityReprocessResult {
  include_archived?: boolean;
  status: "clean" | "scheduled" | "in_progress";
  generation?: number;
  cache_state?: IntegrityCacheState;
  running?: boolean;
  startup_scan_running?: boolean;
  queued?: boolean;
  coalesced?: boolean;
  ticket?: AdminLaneTicket;
  last_started?: string;
  last_ended?: string;
  checked_at?: string;
  last_error?: string;
  count: number;
  names?: string[];
  recheck_names?: string[];
  reprocess_names?: string[];
  findings?: IntegrityFinding[];
}

export interface EntityIntegrityFinding {
  scope: string;
  kind: string;
  subject?: string;
  feed?: string;
  country?: string;
  asn?: number;
  path?: string;
  path_mtime?: string;
  reference_path?: string;
  reference_mtime?: string;
  repair_action?: string;
  reason: string;
  affected_countries?: number;
  affected_asns?: number;
}

export interface EntityIntegrityReport {
  status: "clean" | "issues" | "in_progress";
  generation?: number;
  cache_state?: IntegrityCacheState;
  running: boolean;
  startup_scan_running?: boolean;
  queued?: boolean;
  coalesced?: boolean;
  ticket?: AdminLaneTicket;
  last_started?: string;
  last_ended?: string;
  checked_at?: string;
  last_error?: string;
  count: number;
  findings: EntityIntegrityFinding[];
}

export interface EntityIntegrityActionResult {
  status: "scheduled" | "in_progress";
  generation?: number;
  cache_state?: IntegrityCacheState;
  running?: boolean;
  startup_scan_running?: boolean;
  queued?: boolean;
  coalesced?: boolean;
  ticket?: AdminLaneTicket;
  last_started?: string;
  last_ended?: string;
  checked_at?: string;
  last_error?: string;
}

export interface AdminEngineLaneWork {
  id: string;
  kind: string;
  component: string;
  name: string;
  trigger?: string;
  stage?: string;
  detail?: string;
  state: "queued" | "active" | "completed" | "failed" | "canceled" | "skipped";
  queued_at?: string;
  started_at?: string;
  elapsed_ms?: number;
  wait_ms?: number;
}

export interface AdminEngineLane {
  limit: number;
  active_count: number;
  waiting_count: number;
  active?: AdminEngineLaneWork[];
  waiting?: AdminEngineLaneWork[];
}

export interface AdminPipelineIntegrityCache {
  generation: number;
  cache_state: IntegrityCacheState;
  running?: boolean;
  startup_scan_running?: boolean;
  queued?: boolean;
  coalesced?: boolean;
  ticket?: AdminLaneTicket;
  include_archived?: boolean;
  enable_all?: boolean;
  web_dir?: string;
  checked_at?: string;
  last_started?: string;
  last_ended?: string;
  last_error?: string;
  count: number;
}

export interface AdminEntityIntegrityCache {
  generation: number;
  cache_state: IntegrityCacheState;
  running?: boolean;
  startup_scan_running?: boolean;
  queued?: boolean;
  coalesced?: boolean;
  ticket?: AdminLaneTicket;
  checked_at?: string;
  last_started?: string;
  last_ended?: string;
  last_error?: string;
  count: number;
}

/** /api/v1/admin/status returns a wrapped object with system,
 *  engine, scheduler, and feeds summary blocks. All numeric counts
 *  in .feeds are pre-computed by the backend so the heartbeat strip
 *  tiles and the feeds-table filter chips share the same derivation
 *  (see pkg/web/admin.go:buildAdminStatus). */
export interface AdminStatus {
  public_base_url?: string;
  system: {
    uptime: string;
    go_version: string;
    goos: string;
    goarch: string;
    goroutines: number;
    heap_alloc: number;
    heap_sys: number;
    heap_inuse: number;
    stack_inuse: number;
    sys: number;
    num_gc: number;
    last_gc_unix: number;
    gc_pause_total_ns: number;
    disk_free: string;
    rss_kb?: number;
    vms_kb?: number;
    data_kb?: number;
    cpu_user_seconds?: number;
    cpu_system_seconds?: number;
    cpu_total_seconds?: number;
    proc_read_bytes?: number;
    proc_write_bytes?: number;
    proc_cancelled_write_bytes?: number;
    proc_read_syscalls?: number;
    proc_write_syscalls?: number;
    open_fds?: number;
  };
  engine: {
    running: boolean;
    last_started?: string;
    last_ended?: string;
    current_reason?: string;
    last_reason?: string;
    current_phase?: string;
    current_batch?: AdminRunBatch;
    phase_plan?: AdminRunPhasePlan;
    active_feeds?: Array<{
      name: string;
      reason?: string;
      started_at?: string;
    }>;
    active_operations?: AdminActiveOperation[];
    background_tasks?: Array<{
      id: string;
      name: string;
      kind?: string;
      component?: string;
      trigger?: string;
      stage?: string;
      detail?: string;
      started_at?: string;
      updated_at?: string;
      current?: number;
      total?: number;
    }>;
    background_limit?: number;
    background_running?: number;
    engine_lane?: AdminEngineLane;
    pipeline_integrity_cache?: AdminPipelineIntegrityCache;
    entity_integrity_cache?: AdminEntityIntegrityCache;
    max_ingest_workers?: number;
    parallel_downloads?: number;
    parallel_dns_queries?: number;
    max_processing_workers?: number;
    max_heavy_phase_workers?: number;
    max_background_workers?: number;
    max_engine_lane_workers?: number;
    last_report?: {
      started_at: string;
      ended_at: string;
      skipped?: string[];
      updated?: string[];
      failed?: string[];
    };
    entity_refresh_pending?: number;
    entity_health_pending?: number;
    entity_rebuild_pending?: boolean;
    last_config_reload?: string;
    config_reload_count?: number;
    last_config_reload_error?: string;
    startup_repair_deferred?: boolean;
    startup_repair_deferred_targets?: number;
  };
  scheduler?: {
    generated_at?: string;
    items?: Array<{
      name: string;
      kind: string;
      enabled: boolean;
      next_due: string;
      checked_at: string;
      frequency_minutes: number;
      failures: number;
      detail: string;
    }>;
  };
  queues?: {
    download_waiting?: AdminQueueItem[];
    download_active?: AdminActiveQueueItem[];
    download_refetch_pending?: AdminQueueItem[];
    processing_waiting?: AdminQueueItem[];
    processing_active?: AdminActiveQueueItem[];
    processing_deferred?: AdminQueueItem[];
    recent_health_transitions?: HealthTransition[];
  };
  feeds: {
    total_configured: number;
    total_enabled: number;
    total_entries: number;
    total_unique_ips: number;
    healthy: number;
    delayed: number;
    risky: number;
    unavailable: number;
    archived: number;
    empty: number;
    unmaintained: number;
    stale: number;
    errors: number;
    running: number;
    never_run: number;
    disabled: number;
    hidden: number;
  };
  metrics?: {
    snapshot_persist_errors?: number;
  };
  artifacts?: AdminArtifact[];
}

export interface AdminTimingStat {
  name: string;
  count: number;
  total_ms: number;
  avg_ms: number;
  max_ms: number;
}

export interface AdminCounterStat {
  name: string;
  count?: number;
  bytes?: number;
}

/** One file in a feed's manifest. Mirrors pkg/web/admin_manifest.go:ManifestFile. */
export interface ManifestFile {
  rel: string;
  path: string;
  /** enabled / raw_source / canonical / provider_source / setinfo / metadata /
   *  history / comparison / insights / geo / asn / bogons / binary /
   *  history_snapshot */
  kind: string;
  provider?: string;
  required: boolean;
  exists: boolean;
  size?: number;
  mtime?: number;
  stale?: boolean;
}

export interface ManifestSummary {
  total: number;
  present: number;
  missing: number;
  stale: number;
  required: number;
}

export interface FeedManifest {
  feed: string;
  processed_date: number;
  files: ManifestFile[];
  summary: ManifestSummary;
}
