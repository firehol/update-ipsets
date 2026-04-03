import type { AdminFeed } from "@/lib/api-types";
import {
  type FeedHealthFilter,
  feedHealth,
} from "@/lib/admin-format";

export const HEALTH_FILTERS: { id: FeedHealthFilter; label: string }[] = [
  { id: "healthy", label: "Healthy" },
  { id: "delayed", label: "Delayed" },
  { id: "risky", label: "Risky" },
  { id: "archived", label: "Archived" },
  { id: "unavailable", label: "Unavailable" },
  { id: "empty", label: "Empty" },
  { id: "unmaintained", label: "Unmaintained" },
];

export const KIND_FILTERS: { id: string; label: string }[] = [
  { id: "source", label: "Sources" },
  { id: "merge", label: "Merges" },
  { id: "retention", label: "Retention" },
  { id: "asn", label: "ASN" },
  { id: "geolocation", label: "Geo" },
  { id: "bogon", label: "Bogons" },
];
export const KIND_FILTER_IDS = KIND_FILTERS.map((filter) => filter.id);

export const BOOLEAN_FILTERS: { id: string; label: string }[] = [
  { id: "yes", label: "Yes" },
  { id: "no", label: "No" },
];
export const BOOLEAN_FILTER_IDS = BOOLEAN_FILTERS.map((filter) => filter.id);

export type SortKey =
  | "name"
  | "kind"
  | "category"
  | "status"
  | "frequency"
  | "actual_freq"
  | "cadence_ratio"
  | "unique_ips"
  | "entries"
  | "version"
  | "last_check"
  | "last_update"
  | "processed_date"
  | "last_run_reason"
  | "last_processing_ms"
  | "next_check"
  | "download_failures";

export type SortDir = "asc" | "desc";

const SORT_KEYS: SortKey[] = [
  "name",
  "kind",
  "category",
  "status",
  "frequency",
  "actual_freq",
  "cadence_ratio",
  "unique_ips",
  "entries",
  "version",
  "last_check",
  "last_update",
  "processed_date",
  "last_run_reason",
  "last_processing_ms",
  "next_check",
  "download_failures",
];

export function readSortKey(value: string | null): SortKey | null {
  return SORT_KEYS.includes(value as SortKey) ? (value as SortKey) : null;
}

export function readSortDir(value: string | null): SortDir {
  return value === "desc" ? "desc" : "asc";
}

export function compareDefault(a: AdminFeed, b: AdminFeed): number {
  const aErr = a.last_error || a.download_failures > 0 ? 1 : 0;
  const bErr = b.last_error || b.download_failures > 0 ? 1 : 0;
  if (aErr !== bErr) return bErr - aErr;
  const aNext = a.next_check || Number.MAX_SAFE_INTEGER;
  const bNext = b.next_check || Number.MAX_SAFE_INTEGER;
  if (aNext !== bNext) return aNext - bNext;
  return a.name.localeCompare(b.name);
}

export function compareByKey(key: SortKey, dir: SortDir) {
  const sign = dir === "asc" ? 1 : -1;
  return (a: AdminFeed, b: AdminFeed): number => {
    const va = sortValue(a, key);
    const vb = sortValue(b, key);
    if (typeof va === "string" && typeof vb === "string") {
      return va.localeCompare(vb) * sign;
    }
    if (typeof va === "number" && typeof vb === "number") {
      return (va - vb) * sign;
    }
    return 0;
  };
}

function sortValue(f: AdminFeed, key: SortKey): string | number {
  switch (key) {
    case "name":
      return f.name;
    case "kind":
      return f.kind;
    case "category":
      return f.category || "";
    case "status":
      return healthSortRank(feedHealth(f));
    case "frequency":
      return f.frequency_minutes || 0;
    case "actual_freq":
      return f.avg_update_mins || 0;
    case "cadence_ratio": {
      if (!f.frequency_minutes || !f.avg_update_mins) return 0;
      return f.avg_update_mins / f.frequency_minutes;
    }
    case "unique_ips":
      return f.unique_ips || 0;
    case "entries":
      return f.entries || 0;
    case "version":
      return f.version || 0;
    case "last_check":
      return f.last_check || 0;
    case "last_update":
      return f.last_update || 0;
    case "processed_date":
      return f.processed_date || 0;
    case "last_run_reason":
      return f.last_run_reason || "";
    case "last_processing_ms":
      return f.last_processing_ms || 0;
    case "next_check":
      return f.next_check || 0;
    case "download_failures":
      return f.download_failures || 0;
  }
}

export type FacetAxis = "health" | "kind" | "category" | "hidden" | "disabled";

export type FeedFacetState = {
  health: FeedHealthFilter[];
  kind: string[];
  category: string[];
  hidden: string[];
  disabled: string[];
  search: string;
  excludeAxis?: FacetAxis;
};

export function matchesFacetState(
  feed: AdminFeed,
  state: FeedFacetState,
): boolean {
  if (
    state.excludeAxis !== "health" &&
    state.health.length > 0 &&
    !state.health.includes(feedHealth(feed))
  ) {
    return false;
  }
  if (
    state.excludeAxis !== "kind" &&
    state.kind.length > 0 &&
    !state.kind.includes(feed.kind)
  ) {
    return false;
  }
  if (state.excludeAxis !== "category" && state.category.length > 0) {
    if (!feed.category || !state.category.includes(feed.category)) {
      return false;
    }
  }
  if (state.excludeAxis !== "hidden" && state.hidden.length > 0) {
    const hiddenState = feed.hidden ? "yes" : "no";
    if (!state.hidden.includes(hiddenState)) {
      return false;
    }
  }
  if (state.excludeAxis !== "disabled" && state.disabled.length > 0) {
    const disabledState = feed.enabled ? "no" : "yes";
    if (!state.disabled.includes(disabledState)) {
      return false;
    }
  }
  return matchesFeedSearch(feed, state.search);
}

function matchesFeedSearch(feed: AdminFeed, query: string): boolean {
  if (!query) return true;
  const haystack = [
    feed.name,
    feed.category,
    feed.maintainer,
    feed.url,
    feed.license,
    feed.last_error,
    feed.last_status,
    feed.last_run_reason,
    feed.scheduler_detail,
    feed.kind,
  ]
    .filter(Boolean)
    .join(" ")
    .toLowerCase();
  return haystack.includes(query);
}

export function computeHealthCounts(
  feeds: AdminFeed[],
): Record<string, number> {
  const counts: Record<string, number> = {
    healthy: 0,
    delayed: 0,
    risky: 0,
    archived: 0,
    unavailable: 0,
    empty: 0,
    unmaintained: 0,
  };
  for (const f of feeds) {
    counts[feedHealth(f)]++;
  }
  return counts;
}

export function computeKindCounts(feeds: AdminFeed[]): Record<string, number> {
  const counts: Record<string, number> = {};
  for (const f of feeds) {
    counts[f.kind] = (counts[f.kind] ?? 0) + 1;
  }
  return counts;
}

export function computeCategoryCounts(
  feeds: AdminFeed[],
): Record<string, number> {
  const counts: Record<string, number> = {};
  for (const f of feeds) {
    if (!f.category) continue;
    counts[f.category] = (counts[f.category] ?? 0) + 1;
  }
  return counts;
}

export function computeBooleanCounts(
  feeds: AdminFeed[],
  predicate: (feed: AdminFeed) => boolean,
): { yes: number; no: number } {
  let yes = 0;
  let no = 0;
  for (const f of feeds) {
    if (predicate(f)) {
      yes++;
    } else {
      no++;
    }
  }
  return { yes, no };
}

function healthSortRank(value: ReturnType<typeof feedHealth>): number {
  switch (value) {
    case "archived":
      return 0;
    case "unavailable":
      return 1;
    case "unmaintained":
      return 2;
    case "risky":
      return 3;
    case "delayed":
      return 4;
    case "empty":
      return 5;
    case "healthy":
    default:
      return 6;
  }
}

export function cadenceRatio(
  scheduledMins: number,
  actualMins: number | undefined,
): { label: string; color: string; hint: string } | null {
  if (!scheduledMins || !actualMins) return null;
  const ratio = actualMins / scheduledMins;
  if (ratio < 0.5) {
    return {
      label: `${ratio.toFixed(1)}× schedule`,
      color: "text-status-warning",
      hint: `Feed changes ${(1 / ratio).toFixed(1)}× faster than we check — increase frequency to catch updates`,
    };
  }
  if (ratio > 3) {
    return {
      label: `${ratio.toFixed(1)}× schedule`,
      color: "text-status-warning",
      hint: `Checking ${ratio.toFixed(1)}× too often — decrease frequency to save resources`,
    };
  }
  if (ratio >= 0.8 && ratio <= 1.5) {
    return {
      label: "in sync",
      color: "text-status-healthy",
      hint: "Configured frequency matches observed update cadence",
    };
  }
  return {
    label: `${ratio.toFixed(1)}× schedule`,
    color: "text-muted-foreground",
    hint: "Observed cadence is close to configured frequency",
  };
}
