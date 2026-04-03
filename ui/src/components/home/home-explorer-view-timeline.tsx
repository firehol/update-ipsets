import { useMemo } from "react";
import type { FeedSummary } from "@/lib/api-types";
import { explorerTimestamp } from "@/lib/explorer-state";
import { CategoryBadge } from "@/components/category-badge";
import { FeedRef } from "@/components/feed-detail/feed-ref";
import { feedHealthDotColor, feedHealthLabel } from "@/lib/feed-health";
import { formatNum } from "@/lib/utils";

type Bucket = "hour" | "day" | "week" | "month" | "quarter" | "older" | "never";

const BUCKETS: Array<{ id: Bucket; label: string; description: string }> = [
  { id: "hour", label: "Past hour", description: "Fresh, recently published." },
  { id: "day", label: "Past day", description: "Updated within the last 24 hours." },
  { id: "week", label: "Past week", description: "Updated in the last 7 days." },
  { id: "month", label: "Past month", description: "Updated in the last 30 days." },
  { id: "quarter", label: "Past 90 days", description: "Older than a month but still active this quarter." },
  { id: "older", label: "Older than 90 days", description: "No update in the last 90 days." },
  { id: "never", label: "No timestamp", description: "No published update timestamp on record." },
];

function bucketFor(feed: FeedSummary): Bucket {
  const ts = explorerTimestamp(feed);
  if (!ts) return "never";
  const now = Math.floor(Date.now() / 1000);
  const age = now - ts;
  if (age < 3600) return "hour";
  if (age < 86400) return "day";
  if (age < 86400 * 7) return "week";
  if (age < 86400 * 30) return "month";
  if (age < 86400 * 90) return "quarter";
  return "older";
}

function formatRelative(ts: number | undefined): string {
  if (!ts) return "—";
  const now = Math.floor(Date.now() / 1000);
  const diff = now - ts;
  if (diff < 60) return "just now";
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  if (diff < 86400 * 30) return `${Math.floor(diff / 86400)}d ago`;
  if (diff < 86400 * 365) return `${Math.floor(diff / (86400 * 30))}mo ago`;
  return `${Math.floor(diff / (86400 * 365))}y ago`;
}

export function HomeExplorerViewTimeline({
  feeds,
}: {
  feeds: FeedSummary[];
}) {
  const grouped = useMemo(() => {
    const map = new Map<Bucket, FeedSummary[]>();
    for (const feed of feeds) {
      const b = bucketFor(feed);
      let bucketFeeds = map.get(b);
      if (!bucketFeeds) {
        bucketFeeds = [];
        map.set(b, bucketFeeds);
      }
      bucketFeeds.push(feed);
    }
    for (const arr of map.values()) {
      arr.sort((a, b) => explorerTimestamp(b) - explorerTimestamp(a));
    }
    return map;
  }, [feeds]);

  const maxCount = useMemo(() => {
    let max = 0;
    for (const arr of grouped.values()) {
      if (arr.length > max) max = arr.length;
    }
    return max || 1;
  }, [grouped]);

  if (feeds.length === 0) {
    return (
      <div className="border border-dashed border-border py-24 text-center text-[13px] text-muted-foreground">
        No feeds match the current filter.
      </div>
    );
  }

  return (
    <div className="space-y-12">
      <div className="text-[12px] text-muted-foreground">
        Buckets use the feed source timestamp when available, otherwise the last
        publication time, otherwise the last checked time. Relative ages are
        calculated from your browser clock.
      </div>
      {BUCKETS.map((bucket) => {
        const list = grouped.get(bucket.id) ?? [];
        if (list.length === 0) return null;
        const ratio = list.length / maxCount;
        return (
          <div key={bucket.id}>
            <div className="flex items-baseline justify-between gap-4 border-b border-border pb-3">
              <div>
                <div className="eyebrow text-muted-foreground">
                  {bucket.label}
                </div>
                <div className="mt-1 flex items-baseline gap-3">
                  <span className="num display-stat display-stat-compact text-foreground">
                    {list.length}
                  </span>
                  <span className="text-[12px] text-muted-foreground">
                    {bucket.description}
                  </span>
                </div>
              </div>
              <div className="hidden min-w-[8rem] flex-1 md:block">
                <div className="h-1 w-full bg-border">
                  <div
                    className="h-full bg-primary"
                    style={{ width: `${Math.max(5, ratio * 100)}%` }}
                  />
                </div>
              </div>
            </div>
            <ul className="mt-4 divide-y divide-border">
              {list.slice(0, 50).map((feed) => {
                const healthClass = feed.health?.class ?? "healthy";
                return (
                  <li key={feed.name} className="flex items-center gap-4 py-3">
                    <span
                      role="img"
                      className="inline-block h-2 w-2 rounded-full"
                      style={{
                        backgroundColor: feedHealthDotColor(healthClass),
                      }}
                      aria-label={feedHealthLabel(healthClass)}
                    />
                    <FeedRef
                      name={feed.name}
                      feed={feed}
                      className="flex-1 truncate font-mono text-[13px] font-semibold text-foreground hover:text-primary"
                    >
                      {feed.name}
                    </FeedRef>
                    <CategoryBadge category={feed.category} />
                    <span className="num w-28 text-right text-[12px] text-muted-foreground">
                      {formatNum(feed.unique_ips ?? 0)} IPs
                    </span>
                    <span className="num w-24 text-right text-[12px] text-muted-foreground">
                      {formatRelative(explorerTimestamp(feed))}
                    </span>
                  </li>
                );
              })}
              {list.length > 50 && (
                <li className="py-3 text-[12px] text-muted-foreground">
                  + {list.length - 50} more in this window
                </li>
              )}
            </ul>
          </div>
        );
      })}
    </div>
  );
}
