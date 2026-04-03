import type { FeedSummary } from "@/lib/api-types";

export type HomeSortKey = "freshness" | "change" | "coverage";

export interface HomeSortOption {
  key: HomeSortKey;
  label: string;
  description: string;
}

export const HOME_SORT_OPTIONS: HomeSortOption[] = [
  {
    key: "freshness",
    label: "Freshness",
    description:
      "Ranks feeds by how closely they are keeping up with their own healthy cadence.",
  },
  {
    key: "change",
    label: "Change",
    description:
      "Ranks feeds by the median 0-100% share of membership that changes between observed updates.",
  },
  {
    key: "coverage",
    label: "Coverage",
    description:
      "Ranks feeds by current unique-IP breadth. This shows scope, not quality.",
  },
];

function positiveOrFallback(
  value: number | undefined,
  fallback: number,
): number {
  if (value && value > 0) return value;
  return fallback;
}

function freshnessRatio(feed: FeedSummary): number {
  const health = feed.health;
  const age = positiveOrFallback(
    health.time_since_last_change_mins,
    Number.MAX_SAFE_INTEGER,
  );
  const healthy = positiveOrFallback(
    health.effective_healthy_gap_mins,
    positiveOrFallback(feed.average_update_mins, feed.frequency_minutes || 1),
  );
  return age / Math.max(healthy, 1);
}

export function compareFeedsForHome(
  left: FeedSummary,
  right: FeedSummary,
  mode: HomeSortKey,
): number {
  switch (mode) {
    case "change": {
      const lp = left.change_ratio_median_pct ?? -1;
      const rp = right.change_ratio_median_pct ?? -1;
      if (lp !== rp) return rp - lp;
      const ls = left.change_ratio_samples ?? -1;
      const rs = right.change_ratio_samples ?? -1;
      if (ls !== rs) return rs - ls;
      break;
    }
    case "coverage": {
      const li = left.unique_ips || 0;
      const ri = right.unique_ips || 0;
      if (li !== ri) return ri - li;
      break;
    }
    case "freshness":
    default: {
      const lDelayed = left.health.class === "delayed" ? 1 : 0;
      const rDelayed = right.health.class === "delayed" ? 1 : 0;
      if (lDelayed !== rDelayed) return lDelayed - rDelayed;
      const lr = freshnessRatio(left);
      const rr = freshnessRatio(right);
      if (lr !== rr) return lr - rr;
      break;
    }
  }
  const lAge =
    left.health.time_since_last_change_mins ?? Number.MAX_SAFE_INTEGER;
  const rAge =
    right.health.time_since_last_change_mins ?? Number.MAX_SAFE_INTEGER;
  if (lAge !== rAge) return lAge - rAge;
  return left.name.localeCompare(right.name);
}

export function homepageEligible(feed: FeedSummary): boolean {
  const provenance = feed.provenance ?? "primary";
  return (
    (feed.health.class === "healthy" || feed.health.class === "delayed") &&
    (provenance === "primary" || provenance === "secondary_upstream")
  );
}
