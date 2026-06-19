/** TypeScript mirrors of the JSON shapes served under /api/v1/*. Keep field
 * names aligned with Go json tags so backend contract drift is visible. */

import type {
  EnrichmentCurrentStatusState,
  FeedEnrichment,
  FeedEnrichmentCurrentStatus,
} from "./enrichment-types";

/* ============================================================================
   Per-feed metadata — what /<name>.json and /api/v1/sets/<name> return.
   ========================================================================== */

export type FeedHealthClass =
  | "unavailable"
  | "archived"
  | "empty"
  | "delayed"
  | "risky"
  | "unmaintained"
  | "healthy";

export type FeedProvenance =
  | "primary"
  | "secondary_upstream"
  | "secondary_merge"
  | "secondary_retention";

export interface CategoryMeta {
  name: string;
  label: string;
  description: string;
  color?: string;
  sort_order?: number;
}

export type FeedHealthThresholdBasis =
  | "category_cadence"
  | "single_observation_grace";

export interface FeedHealthSnapshot {
  class: FeedHealthClass;
  threshold_basis?: FeedHealthThresholdBasis;
  threshold_mins?: number;
  avg_update_mins?: number;
  min_update_mins?: number;
  max_update_mins?: number;
  observed_updates?: number;
  /** Unix seconds. */
  first_observed_at?: number;
  /** Unix seconds. */
  last_change_at?: number;
  time_since_last_change_mins?: number;
  /** Unix seconds. */
  failure_started_at?: number;
  time_since_failure_mins?: number;
  download_failures?: number;
  exclude_from_unmaintained?: boolean;
  healthy_cadence_mins?: number;
  risky_cadence_mins?: number;
  effective_healthy_gap_mins?: number;
  unmaintained_threshold_mins?: number;
  archival_threshold_mins?: number;
  single_observation_grace_mins?: number;
}

export interface MergeInputState {
  name: string;
  role?: string;
  reason?: string;
  health_class?: FeedHealthClass;
  enabled: boolean;
  has_feed_body: boolean;
}

export interface FeedMetadata {
  name: string;
  entries: number;
  entries_min: number;
  entries_max: number;
  ips: number;
  ips_min: number;
  ips_max: number;
  ipv: string;
  hash: string;
  frequency: number;
  aggregation: number;
  /** Started/Updated/Processed/Checked are Unix milliseconds, NOT seconds. */
  started: number;
  updated: number;
  processed: number;
  checked: number;
  /** Unix-millisecond duration, matching the legacy static JSON contract. */
  clock_skew: number;
  category: string;
  provenance?: FeedProvenance;
  maintainer: string;
  maintainer_url: string;
  /** License under which the maintainer publishes the data. May be empty. */
  license?: string;
  /** Required attribution string from the maintainer. May be empty. */
  attribution?: string;
  official_name?: string;
  short_description?: string;
  current_status?: FeedEnrichmentCurrentStatus;
  enrichment?: FeedEnrichment;
  info: string;
  source: string;
  file: string;
  history: string;
  /** Geo file fan-out keyed by source name (e.g. "geolite2_country"). */
  geo?: Record<string, string>;
  comparison: string;
  file_local: string;
  commit_history: string;
  errors: number;
  version: number;
  average_update: number;
  min_update: number;
  max_update: number;
  rotation_median_pct?: number;
  rotation_p75_pct?: number;
  rotation_samples?: number;
  change_ratio_median_pct?: number;
  change_ratio_p75_pct?: number;
  change_ratio_samples?: number;
  health: FeedHealthSnapshot;
  downloader: string;
  used_for?: string[];
  hidden?: boolean;
  merge_included?: MergeInputState[];
  merge_subtracted?: MergeInputState[];
  merge_excluded?: MergeInputState[];
  processor?: string;
  pre_processor?: string;
  dont_redistribute?: boolean;
  format?: string;
}

/** Catalog row returned by /api/v1/sets — the daemon serialises its
 *  cache.Entry struct directly here, so the field names differ from the
 *  per-feed setMetadata shape returned by /api/v1/sets/{name}.
 *
 *  Notably:
 *    - The IP count field is `unique_ips`, not `ips`
 *    - The frequency field is `frequency_minutes`, not `frequency`
 *    - Timestamps (`source_date`, `processed_date`, `checked_date`,
 *      `started_date`) are Unix seconds, not milliseconds.
 */
export interface FeedSummary {
  name: string;
  category: string;
  provenance?: FeedProvenance;
  maintainer: string;
  maintainer_url?: string;
  license?: string;
  redistributable: boolean;
  official_name?: string;
  short_description?: string;
  current_status_state?: EnrichmentCurrentStatusState;
  info?: string;
  ipv?: string;
  hash?: string;
  url?: string;
  public_url?: string;
  file?: string;
  source?: string;
  /** Unix seconds. */
  started_date: number;
  /** Unix seconds. */
  source_date: number;
  /** Unix seconds. */
  processed_date: number;
  /** Unix seconds. */
  checked_date: number;
  unique_ips: number;
  entries: number;
  entries_min?: number;
  entries_max?: number;
  ips_min?: number;
  ips_max?: number;
  frequency_minutes: number;
  average_update_mins?: number;
  min_update_mins?: number;
  max_update_mins?: number;
  rotation_median_pct?: number;
  rotation_p75_pct?: number;
  rotation_samples?: number;
  change_ratio_median_pct?: number;
  change_ratio_p75_pct?: number;
  change_ratio_samples?: number;
  version?: number;
  last_status?: string;
  last_error?: string;
  download_failures?: number;
  unique_share_pct?: number;
  unique_share_samples?: number;
  critical?: {
    tier?: CriticalTier;
    role?: string;
  };
  critical_overlap_tiers?: CriticalTier[];
  health: FeedHealthSnapshot;
}

/* ============================================================================
   Provider listings — /api/v1/sets/{name}/{asn,countries,bogons}
   ========================================================================== */

export interface ASNProvider {
  name: string;
  label?: string;
  type: string;
  info?: string;
  license?: string;
  attribution?: string;
  redistributable: boolean;
  maintainer?: string;
  maintainer_url?: string;
}

export interface GeoProvider {
  name: string;
  label?: string;
  type: string;
  info?: string;
  license?: string;
  attribution?: string;
  redistributable: boolean;
  maintainer?: string;
  maintainer_url?: string;
}

export interface BogonProvider {
  name: string;
  label?: string;
  type: string;
  feed?: string;
  info?: string;
  maintainer?: string;
  maintainer_url?: string;
  /** True for the source carrying the RFC reserved baseline. The
   *  frontend uses this to render the "Authoritative bogons"
   *  subsection without needing to know which feed name (or which
   *  format identifier) the backend assigned to that source. */
  authoritative?: boolean;
}

export interface CriticalInfrastructureProvider {
  name: string;
  label?: string;
  type?: string;
  tier: "hard" | "soft" | "contextual" | string;
  role: string;
  source_type: string;
  source_quality: string;
  rationale: string;
  info?: string;
  license?: string;
  attribution?: string;
  redistributable: boolean;
  maintainer?: string;
  maintainer_url?: string;
}

/* ============================================================================
   Per-feed-per-provider payloads.
   ========================================================================== */

export interface ASNEntry {
  asn: number;
  name: string;
  count: number;
  percent: number;
}

export interface ASNFeedPayload {
  provider: string;
  feed_ips: number;
  attributed_ips: number;
  bogon_ips: number;
  unknown_ips: number;
  by_asn: ASNEntry[];
}

export interface CountryValue {
  code: string;
  name?: string;
  value: number;
}

export interface CountryComparisonPayload {
  total_mapped: number;
  countries: CountryValue[];
}

export type CriticalTier = "hard" | "soft" | "contextual" | string;

/** Per-feed bogon overlap payload returned by /api/v1/sets/{name}/bogons/{provider}. */
export interface BogonFeedPayload {
  provider: string;
  feed_ips: number;
  bogon_ips: number;
  percent: number;
  by_range?: BogonRange[];
}

export interface BogonRange {
  cidr: string;
  name: string;
  rfc?: string;
  count: number;
}

export interface CriticalInfrastructureOverlap {
  provider: CriticalInfrastructureProvider;
  provider_set_id: string;
  feed_ips: number;
  critical_ips: number;
  percent: number;
}

export interface CriticalInfrastructureTierSummary {
  tier: string;
  critical_ips: number;
  percent: number;
  providers: number;
}

export interface CriticalInfrastructureMissingProvider {
  name: string;
  reason?: string;
}

export interface CriticalASNContextMatch {
  asn: number;
  name: string;
  tier: "soft" | "contextual" | string;
  role: string;
  source_quality: string;
  rationale: string;
  ips: number;
  percent: number;
}

export interface CriticalASNContextPayload {
  provider?: string;
  feed_ips: number;
  ips: number;
  percent: number;
  matches?: CriticalASNContextMatch[];
}

export interface CriticalInfrastructurePayload {
  feed: string;
  family: string;
  feed_ips: number;
  critical_ips: number;
  percent: number;
  complete: boolean;
  provider_set_id: string;
  configured_providers?: string[];
  missing_providers?: CriticalInfrastructureMissingProvider[];
  tiers?: CriticalInfrastructureTierSummary[];
  providers?: CriticalInfrastructureOverlap[];
  asn_context?: CriticalASNContextPayload;
}

/* ============================================================================
   Comparison / overlap payload from /api/v1/sets/{name}/compare
   ========================================================================== */

/** A row from /api/v1/sets/{name}/compare. The backend returns only non-zero
 *  overlap rows; pct_self and pct_other are computed client-side from the
 *  containing feed's IP count.
 *
 *  `related` marks rows that belong to the same derivative family as
 *  the containing feed — a retention variant of the same parent, a
 *  merge whose inputs include the current feed, the parent source of
 *  a retention variant, or variants of merge inputs. The public UI
 *  excludes related rows from the overlap-facts tiles (UNIQUE /
 *  INCLUDED IN / INCLUDES / ≥50%) because a retention variant
 *  trivially overlaps its parent at 100% and a merge trivially
 *  contains every one of its inputs; counting those would make the
 *  "unique IPs" tile always zero for feeds with any derivative.
 *  Related rows are still shown in the overlap table when they have a non-zero
 *  overlap. */
export interface ComparisonRow {
  name: string;
  category: string;
  ips: number;
  common: number;
  related?: boolean;
}

/* ============================================================================
   History (CSV) and retention/age (JSON).
   ========================================================================== */

export interface RetentionData {
  ipset?: string;
  /** Unix milliseconds. */
  started?: number;
  /** Unix milliseconds. */
  updated?: number;
  /** 0 or 1 in the backend payload. */
  incomplete?: number;
  past?: RetentionWindow;
  current?: RetentionWindow;
}

export interface RetentionWindow {
  hours: number[];
  ips: number[];
  total?: number;
}

/**
 * A single record from /api/v1/sets/{name}/changesets — one per
 * successful update where the feed's set actually changed. Added and
 * Removed are absolute counts and their sum is non-zero. Frontend chart
 * convention: added is plotted above the x-axis, removed below (negated).
 * Net delta is `added - removed`; churn is `added + removed` as a share of
 * the previous size.
 */
export interface ChangesetPoint {
  timestamp: number;
  added: number;
  removed: number;
}

/* ============================================================================
   Insights (deterministic facts engine).
   ========================================================================== */

/**
 * Mirror of `pkg/insights.Section` in the Go source. These are the stable
 * lowercase strings that `Section.MarshalJSON` emits. If the Go side grows
 * a new section, add it here too — the values are part of the API shape,
 * not an implementation detail.
 */
export type InsightSection =
  | "overview"
  | "composition"
  | "retention"
  | "trends"
  | "relationships"
  | "freshness";

export interface Insight {
  code: string;
  section: InsightSection;
  headline: string;
  evidence?: Record<string, unknown>;
  methodology?: string;
}

/**
 * Full envelope shape returned by /api/v1/sets/{name}/insights. The
 * backend wraps the insights array in a small metadata object; callers
 * that just want the items should destructure `.items`.
 */
export interface InsightsPayload {
  name: string;
  /** Unix seconds when the engine computed this snapshot. */
  computed: number;
  items: Insight[];
}

/* ============================================================================
   IP search — /api/v1/search?ip=...
   ========================================================================== */

export interface IPSearchMatch {
  name: string;
  category?: string;
  provenance?: FeedProvenance;
  info?: string;
  maintainer?: string;
  /** Unix seconds. */
  first_seen?: number;
  /** Unix seconds. */
  last_seen?: number;
  health?: FeedHealthSnapshot;
  error?: string;
}

export interface IPSearchContext {
  ip: string;
  country_code?: string;
  geo_provider?: string;
  geo_provider_label?: string;
  asn?: number;
  asn_name?: string;
  asn_provider?: string;
  asn_provider_label?: string;
}

export interface IPSearchResult {
  ip: string;
  scope?: "global" | "feed";
  searched_feed?: string;
  matches: IPSearchMatch[] | string[];
  context?: IPSearchContext;
}

export interface HomeSummaryProvider {
  name?: string;
  label?: string;
}

export interface ClientIPPayload {
  ip: string;
}

export interface CountryIndexEntry {
  code: string;
  feed_count: number;
  attributed_ips: number;
}

export interface CountryIndexPayload {
  provider: HomeSummaryProvider;
  countries: CountryIndexEntry[];
}

export interface ASNIndexEntry {
  asn: number;
  name?: string;
  feed_count: number;
  attributed_ips: number;
}

export interface ASNIndexPayload {
  provider: HomeSummaryProvider;
  asns: ASNIndexEntry[];
}

export interface CountryDetailFeed {
  name: string;
  category: string;
  provenance?: FeedProvenance;
  maintainer?: string;
  attributed_ips: number;
  unique_ips: number;
  health_class: FeedHealthClass;
  last_change_ts?: number;
}

export interface CountryDetailASN {
  asn: number;
  name?: string;
  feed_count: number;
  attributed_ips: number;
}

export interface DetailCategorySummary {
  category: string;
  feed_count: number;
  attributed_ips: number;
}

export interface DetailMaintainerSummary {
  slug: string;
  name: string;
  url?: string;
  feed_count: number;
  attributed_ips: number;
}

export interface CountryDetailPayload {
  code: string;
  provider: HomeSummaryProvider;
  asn_provider?: HomeSummaryProvider;
  totals: {
    feeds_matching: number;
    attributed_ips_in_feeds: number;
    categories: number;
    maintainers: number;
    asns: number;
  };
  feeds: CountryDetailFeed[];
  feeds_by_category?: Record<string, CountryDetailFeed[]>;
  top_categories?: DetailCategorySummary[];
  top_maintainers?: DetailMaintainerSummary[];
  top_asns_in_country?: CountryDetailASN[];
}

export interface ASNDetailFeed {
  name: string;
  category: string;
  provenance?: FeedProvenance;
  maintainer?: string;
  attributed_ips: number;
  unique_ips: number;
  health_class: FeedHealthClass;
  last_change_ts?: number;
}

export interface ASNDetailCountry {
  code: string;
  feed_count: number;
  attributed_ips: number;
}

export interface ASNDetailPayload {
  asn: number;
  name?: string;
  description?: string;
  provider: HomeSummaryProvider;
  geo_provider?: HomeSummaryProvider;
  totals: {
    feeds_matching: number;
    attributed_ips: number;
    categories: number;
    maintainers: number;
    countries: number;
  };
  feeds: ASNDetailFeed[];
  feeds_by_category?: Record<string, ASNDetailFeed[]>;
  top_categories?: DetailCategorySummary[];
  top_maintainers?: DetailMaintainerSummary[];
  top_countries?: ASNDetailCountry[];
  country_distribution?: CountryComparisonPayload;
}

export interface MaintainerIndexEntry {
  slug: string;
  name: string;
  url?: string;
  feed_count: number;
  unique_ips: number;
  categories: string[];
}

export interface MaintainerIndexPayload {
  maintainers: MaintainerIndexEntry[];
}

export interface MaintainerDetailFeed {
  name: string;
  category: string;
  provenance?: string;
  unique_ips: number;
  health_class: FeedHealthClass;
  last_change_ts?: number;
}

export interface MaintainerDetailPayload {
  slug: string;
  name: string;
  url?: string;
  totals: { feeds: number; unique_ips: number; categories: number };
  feeds_by_category: Record<string, MaintainerDetailFeed[]>;
}

export interface MethodologyIndexEntry {
  slug: string;
  title: string;
  summary?: string;
}

export interface MethodologyIndexPayload {
  items: MethodologyIndexEntry[];
}

export interface MethodologyPagePayload {
  slug: string;
  title: string;
  summary?: string;
  body: string;
}

/* ============================================================================
   Admin endpoints.
   ========================================================================== */

/** Admin feed row from /api/v1/admin/feeds. Field names mirror
 *  pkg/web/admin.go:adminFeed. 100% coverage of cache.Entry +
 *  config.Source — every field an operator might need is here
 *  so the feed modal can render without a second fetch. */
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
  running: boolean;
  last_started?: string;
  last_ended?: string;
  count: number;
  findings: IntegrityFinding[];
}

/** Envelope for POST /api/v1/admin/integrity/reprocess. Returns "clean"
 *  with count=0 when nothing needs recovery, or "scheduled" with the
 *  class-split recovery targets that were queued. */
export interface IntegrityReprocessResult {
  include_archived?: boolean;
  status: "clean" | "scheduled" | "in_progress";
  running?: boolean;
  last_started?: string;
  last_ended?: string;
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
  running: boolean;
  last_started?: string;
  last_ended?: string;
  count: number;
  findings: EntityIntegrityFinding[];
}

export interface EntityIntegrityActionResult {
  status: "scheduled" | "in_progress";
  running?: boolean;
  last_started?: string;
  last_ended?: string;
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
    active_feeds?: Array<{
      name: string;
      reason?: string;
      started_at?: string;
    }>;
    active_operations?: AdminActiveOperation[];
    background_tasks?: Array<{
      id: string;
      name: string;
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
    max_ingest_workers?: number;
    parallel_downloads?: number;
    parallel_dns_queries?: number;
    max_processing_workers?: number;
    max_heavy_phase_workers?: number;
    max_background_workers?: number;
    lifetime_metrics?: {
      operations?: AdminTimingStat[];
      counters?: AdminCounterStat[];
    };
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
