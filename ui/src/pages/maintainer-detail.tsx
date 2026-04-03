import { useMemo } from "react";
import { Link, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { AccentBar } from "@/components/editorial/accent-bar";
import { CategoryBadge } from "@/components/category-badge";
import { FeedRef } from "@/components/feed-detail/feed-ref";
import { useFeedRefDescriptorMap } from "@/components/feed-detail/feed-ref-descriptor";
import { feedHealthDotColor, feedHealthLabel } from "@/lib/feed-health";
import { orderCategories, useCategoriesQuery } from "@/lib/categories";
import type { MaintainerDetailFeed } from "@/lib/api-types";
import { formatNum } from "@/lib/utils";
import { maintainerOptions } from "@/lib/queries/entities";
import { safeExternalUrl } from "@/lib/safe-url";

function formatRelative(ts: number | undefined): string {
  if (!ts) return "—";
  const now = Math.floor(Date.now() / 1000);
  const diff = now - ts;
  if (diff < 60) return "just now";
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  if (diff < 86400 * 30) return `${Math.floor(diff / 86400)}d ago`;
  return `${Math.floor(diff / (86400 * 30))}mo ago`;
}

export function MaintainerDetailPage() {
  const { slug } = useParams<{ slug: string }>();
  const normalized = (slug ?? "").trim();

  const query = useQuery({
    ...maintainerOptions(normalized),
    enabled: normalized.length > 0,
  });
  const categoriesQuery = useCategoriesQuery();

  const payload = query.data;
  const maintainerUrl = safeExternalUrl(payload?.url);
  const refMap = useFeedRefDescriptorMap();
  const feedsByCategory = useMemo(
    () =>
      orderCategories(
        categoriesQuery.data ?? [],
        Object.keys(payload?.feeds_by_category ?? {}),
      ).map((category) => [category, payload?.feeds_by_category[category] ?? []] as const),
    [categoriesQuery.data, payload],
  );

  if (query.isLoading) {
    return (
      <div className="page-container py-20 md:py-24 text-[13px] text-muted-foreground">
        Loading maintainer…
      </div>
    );
  }

  if (query.isError || !payload) {
    return (
      <div className="page-container py-20 md:py-24">
        <AccentBar />
        <div className="eyebrow mt-6 text-muted-foreground">Maintainer</div>
        <h1 className="display-title mt-4 text-foreground">Not found</h1>
        <p className="lede mt-5 max-w-[62ch] text-muted-foreground">
          No tracked maintainer matches this slug.
        </p>
        <div className="mt-8 text-[13px]">
          <Link to="/maintainers" className="text-primary hover:underline">
            ← All maintainers
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="page-container py-20 md:py-24">
      <AccentBar />
      <div className="eyebrow mt-6 text-muted-foreground">Maintainer</div>
      <h1 className="display-title mt-4 text-foreground">{payload.name}</h1>
      {payload.url && maintainerUrl && (
        <p className="mt-3 text-[14px] text-muted-foreground">
          <a
            href={maintainerUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="hover:text-primary"
          >
            {payload.url} ↗
          </a>
        </p>
      )}
      {payload.url && !maintainerUrl && (
        <p className="mt-3 text-[14px] text-muted-foreground">
          {payload.url}
        </p>
      )}

      <div className="mt-10 grid grid-cols-2 gap-px overflow-hidden border-t border-b border-border md:grid-cols-3">
        <Stat label="Feeds" value={formatNum(payload.totals.feeds)} accent />
        <Stat label="IPs across feeds" value={formatNum(payload.totals.unique_ips)} />
        <Stat label="Categories" value={formatNum(payload.totals.categories)} />
      </div>

      <p className="lede mt-8 max-w-[62ch] text-muted-foreground">
        Feeds grouped by category for this maintainer.
      </p>

      <div className="mt-12 space-y-12">
        {feedsByCategory.map(([category, feeds]) => (
          <section key={category}>
            <header className="flex items-baseline justify-between gap-4 border-b border-border pb-3">
              <div className="flex items-baseline gap-3">
                <CategoryBadge category={category} />
                <span className="text-[12px] text-muted-foreground">
                  {feeds.length} feed{feeds.length === 1 ? "" : "s"}
                </span>
              </div>
            </header>
            <ul className="mt-4 divide-y divide-border">
              {feeds.map((feed: MaintainerDetailFeed) => (
                <li key={feed.name} className="flex items-center gap-4 py-3">
                  <span
                    role="img"
                    className="inline-block h-2 w-2 rounded-full"
                    style={{
                      backgroundColor: feedHealthDotColor(feed.health_class),
                    }}
                    aria-label={feedHealthLabel(feed.health_class)}
                  />
                  <FeedRef
                    name={feed.name}
                    feed={refMap.get(feed.name)}
                    className="flex-1 truncate font-mono text-[13px] font-semibold text-foreground hover:text-primary"
                  >
                    {feed.name}
                  </FeedRef>
                  <span className="num w-28 text-right text-[12px] text-muted-foreground">
                    {formatNum(feed.unique_ips)} IPs
                  </span>
                  <span className="num w-24 text-right text-[12px] text-muted-foreground">
                    {formatRelative(feed.last_change_ts)}
                  </span>
                </li>
              ))}
            </ul>
          </section>
        ))}
      </div>
    </div>
  );
}

function Stat({
  label,
  value,
  accent = false,
}: {
  label: string;
  value: string;
  accent?: boolean;
}) {
  return (
    <div className="border-r border-border px-2 py-8 last:border-r-0">
      <div className="eyebrow text-muted-foreground">{label}</div>
      <div
        className={
          "num display-stat display-stat-medium mt-3 " +
          (accent ? "text-primary" : "text-foreground")
        }
      >
        {value}
      </div>
    </div>
  );
}
