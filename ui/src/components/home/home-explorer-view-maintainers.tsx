import { useMemo } from "react";
import { Link } from "react-router-dom";
import type { FeedSummary } from "@/lib/api-types";
import { CategoryBadge } from "@/components/category-badge";
import { FeedRef } from "@/components/feed-detail/feed-ref";
import { feedHealthDotColor, feedHealthLabel } from "@/lib/feed-health";
import { maintainerSlug } from "@/lib/maintainers";
import { externalUrlLabel, safeExternalUrl } from "@/lib/safe-url";
import { formatNum } from "@/lib/utils";

interface MaintainerGroup {
  name: string;
  url?: string;
  feeds: FeedSummary[];
  totalIPs: number;
  categoryCount: number;
}

export function HomeExplorerViewMaintainers({
  feeds,
}: {
  feeds: FeedSummary[];
}) {
  const groups = useMemo<MaintainerGroup[]>(() => {
    const map = new Map<string, MaintainerGroup>();
    for (const feed of feeds) {
      const name = (feed.maintainer ?? "").trim() || "Unknown";
      const existing =
        map.get(name) ??
        ({
          name,
          url: feed.maintainer_url,
          feeds: [],
          totalIPs: 0,
          categoryCount: 0,
        } satisfies MaintainerGroup);
      existing.feeds.push(feed);
      existing.totalIPs += feed.unique_ips ?? 0;
      if (!existing.url && feed.maintainer_url) existing.url = feed.maintainer_url;
      map.set(name, existing);
    }
    for (const group of map.values()) {
      group.categoryCount = new Set(group.feeds.map((f) => f.category)).size;
      group.feeds.sort(
        (a, b) => (b.unique_ips ?? 0) - (a.unique_ips ?? 0),
      );
    }
    return Array.from(map.values()).sort((a, b) =>
      a.name.localeCompare(b.name, undefined, { sensitivity: "base" }),
    );
  }, [feeds]);

  if (groups.length === 0) {
    return (
      <div className="border border-dashed border-border py-24 text-center text-[13px] text-muted-foreground">
        No feeds match the current filter.
      </div>
    );
  }

  return (
    <div className="space-y-12">
      {groups.map((group) => (
        <MaintainerBlock key={group.name} group={group} />
      ))}
    </div>
  );
}

function MaintainerBlock({ group }: { group: MaintainerGroup }) {
  const safeUrl = safeExternalUrl(group.url);
  return (
    <section>
      <header className="flex flex-wrap items-baseline justify-between gap-3 border-b border-border pb-4">
        <div className="min-w-0">
          <div className="eyebrow text-muted-foreground">Maintainer</div>
          <div className="mt-1 flex items-baseline gap-3">
            <h3 className="truncate text-[20px] font-semibold tracking-tight text-foreground">
              <Link
                to={`/maintainers/${maintainerSlug(group.name)}`}
                className="hover:text-primary"
              >
                {group.name}
              </Link>
            </h3>
            {group.url && safeUrl && (
              <a
                href={safeUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="text-[12px] text-muted-foreground hover:text-primary"
              >
                {externalUrlLabel(group.url)} ↗
              </a>
            )}
            {group.url && !safeUrl && (
              <span className="text-[12px] text-muted-foreground">
                {externalUrlLabel(group.url)}
              </span>
            )}
          </div>
        </div>
        <div className="flex items-baseline gap-6 text-[12px] text-muted-foreground">
          <span>
            <span className="num font-semibold text-foreground">
              {group.feeds.length}
            </span>{" "}
            feeds
          </span>
          <span>
            <span className="num font-semibold text-foreground">
              {formatNum(group.totalIPs)}
            </span>{" "}
            IPs
          </span>
          <span>
            <span className="num font-semibold text-foreground">
              {group.categoryCount}
            </span>{" "}
            categories
          </span>
        </div>
      </header>

      <ul className="mt-4 grid gap-px bg-border sm:grid-cols-2 xl:grid-cols-3">
        {group.feeds.map((feed) => {
          const healthClass = feed.health?.class ?? "healthy";
          return (
            <li key={feed.name} className="bg-card p-4">
              <FeedRef
                name={feed.name}
                feed={feed}
                className="group flex min-w-0 flex-col gap-2"
              >
                <div className="flex min-w-0 items-center gap-2">
                  <span
                    role="img"
                    className="inline-block h-2 w-2 shrink-0 rounded-full"
                    style={{
                      backgroundColor: feedHealthDotColor(healthClass),
                    }}
                    aria-label={feedHealthLabel(healthClass)}
                  />
                  <span className="truncate font-mono text-[13px] font-semibold text-foreground group-hover:text-primary">
                    {feed.name}
                  </span>
                </div>
                <div className="flex items-center justify-between gap-2 text-[11px] text-muted-foreground">
                  <CategoryBadge category={feed.category} />
                  <span className="num">{formatNum(feed.unique_ips ?? 0)} IPs</span>
                </div>
              </FeedRef>
            </li>
          );
        })}
      </ul>
    </section>
  );
}
