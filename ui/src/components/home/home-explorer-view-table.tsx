import { Link } from "react-router-dom";
import type { FeedSummary } from "@/lib/api-types";
import { explorerTimestamp, type SortKey } from "@/lib/explorer-state";
import { CategoryBadge } from "@/components/category-badge";
import { FeedRef } from "@/components/feed-detail/feed-ref";
import { feedHealthDotColor, feedHealthLabel } from "@/lib/feed-health";
import { maintainerSlug } from "@/lib/maintainers";
import { formatNum, cn } from "@/lib/utils";

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

function formatMinutes(mins: number | undefined): string {
  if (!mins || mins <= 0) return "—";
  if (mins < 60) return `${mins}m`;
  if (mins < 60 * 24) return `${Math.round(mins / 60)}h`;
  return `${Math.round(mins / (60 * 24))}d`;
}

const COLUMNS: Array<{ id: SortKey | "size" | "cadence" | "provenance" | "health" | "unique_pct"; label: string; sort?: SortKey }> = [
  { id: "name", label: "Feed", sort: "name" },
  { id: "maintainer", label: "Maintainer", sort: "maintainer" },
  { id: "size", label: "IPs", sort: "coverage" },
  { id: "unique_pct", label: "Unique", sort: "unique" },
  { id: "cadence", label: "Cadence" },
  { id: "freshest", label: "Updated", sort: "freshest" },
  { id: "provenance", label: "Provenance" },
  { id: "health", label: "Health" },
];

export function HomeExplorerViewTable({
  feeds,
  sort,
  onSortChange,
}: {
  feeds: FeedSummary[];
  sort: SortKey;
  onSortChange: (sort: SortKey) => void;
}) {
  if (feeds.length === 0) {
    return (
      <div className="border border-dashed border-border py-24 text-center text-[13px] text-muted-foreground">
        No feeds match the current filter.
      </div>
    );
  }

  return (
    <div className="border border-border">
      <div className="overflow-x-auto">
        <table className="w-full border-collapse text-[13px]">
          <thead>
            <tr className="border-b border-border bg-muted/30">
              {COLUMNS.map((col) => {
                const sortKey = col.sort;
                return (
                  <th
                    key={col.id}
                    className="px-4 py-3 text-left align-middle"
                  >
                    {sortKey ? (
                      <button
                        type="button"
                        onClick={() => onSortChange(sortKey)}
                        className={cn(
                          "eyebrow transition",
                          sort === sortKey
                            ? "text-foreground"
                            : "text-muted-foreground hover:text-foreground",
                        )}
                      >
                        {col.label}
                        {sort === sortKey && (
                          <span className="ml-1" aria-hidden="true">
                            ↓
                          </span>
                        )}
                      </button>
                    ) : (
                      <span className="eyebrow text-muted-foreground">
                        {col.label}
                      </span>
                    )}
                  </th>
                );
              })}
            </tr>
          </thead>
          <tbody>
            {feeds.map((feed, idx) => {
              const healthClass = feed.health?.class ?? "healthy";
              return (
                <tr
                  key={feed.name}
                  className={cn(
                    "border-b border-border transition hover:bg-muted/30",
                    idx === feeds.length - 1 && "border-b-0",
                  )}
                >
                  <td className="px-4 py-3 align-middle">
                    <FeedRef
                      name={feed.name}
                      feed={feed}
                      className="font-mono text-[13px] font-semibold text-foreground hover:text-primary"
                    />
                    <div className="mt-0.5">
                      <CategoryBadge category={feed.category} />
                    </div>
                  </td>
                  <td className="px-4 py-3 align-middle text-muted-foreground">
                    {feed.maintainer ? (
                      <Link
                        to={`/maintainers/${maintainerSlug(feed.maintainer)}`}
                        className="hover:text-primary"
                      >
                        {feed.maintainer}
                      </Link>
                    ) : (
                      "—"
                    )}
                  </td>
                  <td className="num px-4 py-3 text-right align-middle font-medium text-foreground">
                    {formatNum(feed.unique_ips ?? 0)}
                  </td>
                  <td className="num px-4 py-3 text-right align-middle text-muted-foreground">
                    {feed.unique_share_pct !== undefined
                      ? `${feed.unique_share_pct.toFixed(0)}%`
                      : "—"}
                  </td>
                  <td className="num px-4 py-3 align-middle text-muted-foreground">
                    {formatMinutes(feed.frequency_minutes)}
                  </td>
                  <td className="num px-4 py-3 align-middle text-muted-foreground">
                    {formatRelative(explorerTimestamp(feed))}
                  </td>
                  <td className="px-4 py-3 align-middle text-[11px] uppercase tracking-[0.08em] text-muted-foreground">
                    {(feed.provenance ?? "primary").replace(
                      "secondary_",
                      "",
                    )}
                  </td>
                  <td className="px-4 py-3 align-middle">
                    <span
                      className="inline-flex items-center gap-2 text-[12px] text-muted-foreground"
                      title={feedHealthLabel(healthClass)}
                    >
                      <span
                        className="inline-block h-2 w-2 rounded-full"
                        style={{
                          backgroundColor: feedHealthDotColor(healthClass),
                        }}
                      />
                      {feedHealthLabel(healthClass)}
                    </span>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
