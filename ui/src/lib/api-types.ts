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

export type {
  AdminActiveOperation,
  AdminActiveQueueItem,
  AdminArtifact,
  AdminCounterStat,
  AdminFeed,
  AdminProblemClass,
  AdminQueueItem,
  AdminRunBatch,
  AdminRunPhasePlan,
  AdminStatus,
  AdminTimingStat,
  EntityIntegrityActionResult,
  EntityIntegrityFinding,
  EntityIntegrityReport,
  FeedManifest,
  HealthTransition,
  IntegrityFinding,
  IntegrityRecoveryAction,
  IntegrityReport,
  IntegrityReprocessResult,
  ManifestFile,
  ManifestSummary,
} from "./admin-api-types";
