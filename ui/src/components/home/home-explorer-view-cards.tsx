import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import type { FeedSummary } from "@/lib/api-types";
import { explorerTimestamp, type SortKey } from "@/lib/explorer-state";
import { maintainerSlug } from "@/lib/maintainers";
import { CategoryBadge } from "@/components/category-badge";
import { HoverTip } from "@/components/editorial/hover-tip";
import { FeedRef } from "@/components/feed-detail/feed-ref";
import {
  feedHealthDotColor,
  feedHealthLabel,
} from "@/lib/feed-health";
import { usePrefetchFeedDetail } from "@/lib/feed-prefetch";
import { formatNum } from "@/lib/utils";

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

export function HomeExplorerViewCards({
  feeds,
  sort,
}: {
  feeds: FeedSummary[];
  sort: SortKey;
}) {
  if (feeds.length === 0) {
    return (
      <div className="border border-dashed border-border py-24 text-center text-[13px] text-muted-foreground">
        No feeds match the current filter.
      </div>
    );
  }

  return (
    <div className="grid grid-cols-1 gap-px bg-border sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
      {feeds.map((feed) => (
        <FeedCard key={feed.name} feed={feed} sort={sort} />
      ))}
    </div>
  );
}

function FeedCard({ feed, sort }: { feed: FeedSummary; sort: SortKey }) {
  const healthClass = feed.health?.class ?? "healthy";
  const primary = primaryMetric(feed, sort);
  return (
    <div className="group relative flex flex-col gap-3 bg-card p-5 transition hover:bg-muted/30">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <FeedNameLink feed={feed} />
          {feed.maintainer && (
            <Link
              to={`/maintainers/${maintainerSlug(feed.maintainer)}`}
              className="mt-1 block truncate text-[11px] text-muted-foreground hover:text-primary"
            >
              {feed.maintainer}
            </Link>
          )}
        </div>
        <HoverTip text={feedHealthLabel(healthClass)}>
          <span
            role="img"
            className="mt-1 inline-block h-2 w-2 rounded-full"
            style={{ backgroundColor: feedHealthDotColor(healthClass) }}
            aria-label={feedHealthLabel(healthClass)}
          />
        </HoverTip>
      </div>

      <div className="flex items-baseline gap-2">
        <div className="num text-xl font-semibold text-primary">
          {primary.value}
        </div>
        <div className="text-xs text-muted-foreground">{primary.label}</div>
      </div>

      <div className="flex items-center justify-between gap-3 text-[11px] text-muted-foreground">
        <CategoryBadge category={feed.category} />
        {sort !== "freshest" && (
          <span className="num text-[7px] opacity-60">{formatRelative(explorerTimestamp(feed))}</span>
        )}
      </div>
    </div>
  );
}

function FeedNameLink({ feed }: { feed: FeedSummary }) {
  const ref = useRef<HTMLAnchorElement | null>(null);
  const [truncated, setTruncated] = useState(false);
  const prefetch = usePrefetchFeedDetail(feed.name);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const update = () => setTruncated(el.scrollWidth > el.clientWidth + 1);
    update();
    if (typeof ResizeObserver === "undefined") {
      window.addEventListener("resize", update);
      return () => window.removeEventListener("resize", update);
    }
    const observer = new ResizeObserver(update);
    observer.observe(el);
    return () => observer.disconnect();
  }, [feed.name]);

  return (
    <FeedRef
      ref={ref}
      name={feed.name}
      feed={feed}
      fallbackDescription={truncated ? feed.name : null}
      onFocus={prefetch}
      onMouseEnter={prefetch}
      className="block truncate font-mono text-[13px] font-semibold text-foreground hover:text-primary"
    />
  );
}

function primaryMetric(
  feed: FeedSummary,
  sort: SortKey,
): { value: string; label: string } {
  switch (sort) {
    case "coverage":
      return {
        value: formatNum(feed.unique_ips ?? 0),
        label: "IPs",
      };
    case "freshest":
      return {
        value: formatRelative(explorerTimestamp(feed)),
        label: "Updated",
      };
    case "unique":
      return {
        value:
          feed.unique_share_pct !== undefined
            ? `${feed.unique_share_pct.toFixed(0)}%`
            : "—",
        label: "Unique",
      };
    case "name":
    case "maintainer":
      return {
        value: formatNum(feed.unique_ips ?? 0),
        label: "IPs",
      };
  }
}
