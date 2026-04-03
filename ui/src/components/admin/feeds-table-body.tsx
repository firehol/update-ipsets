import type { AdminFeed, IntegrityFinding } from "@/lib/api-types";
import { FeedsTableHeader } from "@/components/admin/feeds-table-header";
import { FeedRow } from "@/components/admin/feeds-table-row";
import type {
  SortDir,
  SortKey,
} from "@/components/admin/feeds-table-model";

export function FeedsTableBody({
  feeds,
  integrityByFeed,
  sortKey,
  sortDir,
  onSort,
  publicBaseURL,
  onFeedClick,
  nowMs,
}: {
  feeds: AdminFeed[];
  integrityByFeed: Map<string, IntegrityFinding>;
  sortKey: SortKey | null;
  sortDir: SortDir;
  onSort: (key: SortKey) => void;
  publicBaseURL?: string | null;
  onFeedClick: (feed: AdminFeed) => void;
  nowMs: number;
}) {
  if (feeds.length === 0) {
    return (
      <div className="flex flex-col items-center gap-2 rounded-md border border-dashed border-border py-16 text-center">
        <div className="text-sm text-muted-foreground">
          No feeds match the current filters.
        </div>
        <div className="text-xs text-muted-foreground">
          Clear the search or adjust the filters to see more.
        </div>
      </div>
    );
  }
  return (
    <div className="overflow-x-auto rounded-md border border-border">
      <table className="w-full border-collapse text-[12px]">
        <FeedsTableHeader
          sortKey={sortKey}
          sortDir={sortDir}
          onSort={onSort}
        />
        <tbody>
          {feeds.map((feed) => (
            <FeedRow
              key={feed.name}
              feed={feed}
              finding={integrityByFeed.get(feed.name)}
              publicBaseURL={publicBaseURL}
              onFeedClick={onFeedClick}
              nowMs={nowMs}
            />
          ))}
        </tbody>
      </table>
    </div>
  );
}
