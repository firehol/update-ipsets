import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import type {
  ComparisonRow,
  FeedHealthClass,
  FeedHealthSnapshot,
  FeedSummary,
} from "@/lib/api-types";
import { comparisonOptions } from "@/lib/queries/feed";
import { feedsOptions } from "@/lib/queries/catalog";
import { CategoryBadge } from "@/components/category-badge";
import { StatRow, StatTile } from "@/components/editorial/stat-row";
import { DataTable, type DataTableColumn } from "@/components/editorial/data-table";
import { HoverTip } from "@/components/editorial/hover-tip";
import { FeedHealthTip } from "@/components/feed-health-tip";
import {
  feedHealthColor,
  feedHealthLabel,
} from "@/lib/feed-health";
import { Layers } from "lucide-react";
import { useCategoryAccent } from "@/lib/categories";
import { DetailNotice, DetailSection } from "./section";
import { FeedRef } from "./feed-ref";
import { ViewTabBar, ViewTab } from "./provider-tabs";
import { OverlapSankey } from "./overlap-sankey";
import { OverlapNetwork } from "./overlap-network";
import { formatIPs, formatNum } from "@/lib/utils";

// Single source of truth for the overlap section's view height. Every
// view (List / Sankey / Network) uses this exact number so switching
// tabs cannot resize the section.
const OVERLAP_VIEW_HEIGHT = 780;
const EMPTY_COMPARISON_ROWS: ComparisonRow[] = [];

/**
 * Comparison / overlap section. Three layers:
 *
 *   1. Headline tiles:
 *      - INCLUDED IN — how many other feeds fully contain this one
 *        (every IP here is also there). Strong "subset" signal.
 *      - INCLUDES — how many other feeds are fully contained in this
 *        one (every IP there is also here). Strong "superset" signal.
 *      - OVERLAP ≥50% — feeds that share at least half of this one.
 *      - UNIQUE — bounded proxy derived from the strongest independent
 *        overlap, not a full N-way set subtraction.
 *
 *   2. A scrollable DataTable with every non-zero pairwise overlap —
 *      sortable, searchable, CSV-exportable. Not paged; the user scrolls the
 *      full overlap set in place.
 *
 *   3. (Reserved) Sankey and force-graph visualizations. They need a
 *      focused implementation pass rather than a half-built chart.
 */
export function SectionComparison({
  feedName,
  feedIPs,
  feedHealthClass,
  category,
}: {
  feedName: string;
  feedIPs: number;
  feedHealthClass: FeedHealthClass;
  category?: string | null;
}) {
  const accent = useCategoryAccent(category);
  const compareQuery = useQuery(comparisonOptions(feedName));
  const catalogQuery = useQuery({
    ...feedsOptions(),
    staleTime: 5 * 60 * 1000,
  });

  const rows = compareQuery.data ?? EMPTY_COMPARISON_ROWS;
  const [view, setView] = useState<"table" | "sankey" | "network">("table");
  const summaryByFeed = useMemo(() => {
    const out = new Map<string, FeedSummary>();
    for (const feed of catalogQuery.data ?? []) {
      out.set(feed.name, feed);
    }
    return out;
  }, [catalogQuery.data]);
  const displayRows = useMemo<ComparisonDisplayRow[]>(
    () =>
      rows.map((row) => {
        const summary = summaryByFeed.get(row.name);
        return {
          ...row,
          health: summary?.health,
          summary,
        };
      }),
    [summaryByFeed, rows],
  );

  // Derived facts used by the tiles. We compute these once and feed
  // the DataTable the raw rows — no filtering.
  const facts = useMemo(
    () => computeOverlapFacts(displayRows, feedIPs),
    [displayRows, feedIPs],
  );
  const staleStructuralRows = useMemo(
    () => collectStaleStructuralRows(displayRows),
    [displayRows],
  );
  const showStaleStructuralWarning =
    !isStaleHealth(feedHealthClass) && staleStructuralRows.length > 0;

  const columns: DataTableColumn<ComparisonDisplayRow>[] = useMemo(
    () => [
      {
        key: "name",
        label: "Feed",
        sortValue: (row) => row.name,
        render: (row) => (
          <FeedRef
            name={row.name}
            feed={row.summary}
            className="font-mono text-[14px] text-foreground hover:text-primary"
          />
        ),
      },
      {
        key: "category",
        label: "Category",
        sortValue: (row) => row.category || "",
        searchValue: (row) => row.category || "",
        render: (row) => <CategoryBadge category={row.category} />,
      },
      {
        key: "health",
        label: "Health",
        sortValue: (row) => overlapHealthLabel(row.health),
        searchValue: (row) => overlapHealthLabel(row.health),
        render: (row) => <OverlapHealth health={row.health} />,
      },
      {
        key: "ips",
        label: "Other size",
        align: "right",
        sortValue: (row) => row.ips,
        render: (row) => formatIPs(row.ips),
      },
      {
        key: "common",
        label: "Overlap",
        align: "right",
        sortValue: (row) => row.common,
        render: (row) => formatIPs(row.common),
      },
      {
        key: "pct_self",
        label: "% of this",
        align: "right",
        sortValue: (row) => (feedIPs > 0 ? row.common / feedIPs : 0),
        render: (row) => (
          <span className="text-muted-foreground">
            {feedIPs > 0
              ? ((row.common / feedIPs) * 100).toFixed(2) + "%"
              : "—"}
          </span>
        ),
      },
      {
        key: "pct_other",
        label: "% of other",
        align: "right",
        sortValue: (row) => (row.ips > 0 ? row.common / row.ips : 0),
        render: (row) => (
          <span className="text-muted-foreground">
            {row.ips > 0 ? ((row.common / row.ips) * 100).toFixed(2) + "%" : "—"}
          </span>
        ),
      },
    ],
    [feedIPs],
  );

  return (
    <DetailSection
      eyebrow="Overlap"
      title="Where else these IPs appear"
      icon={Layers}
      accentColor={accent}
      lede="Which other tracked feeds share addresses with this one. The tiles surface the clean structural relationships (full subsets, supersets, uniqueness); the table below is the complete non-zero overlap record."
    >
      {compareQuery.isLoading ? (
        <div className="h-64 animate-pulse bg-muted/40" />
      ) : compareQuery.isError ? (
        <DetailNotice title="Comparison data could not be loaded" tone="danger">
          {compareQuery.error instanceof Error
            ? compareQuery.error.message
            : "The pairwise comparison artifact for this feed was unavailable or malformed."}
        </DetailNotice>
      ) : rows.length === 0 ? (
        <p className="py-16 text-center text-sm text-muted-foreground">
          No comparison data computed for this feed yet.
        </p>
      ) : (
        <>
          {/* Four matched tiles on a single row, matching the 4-up
              shape used by the ASN and Geo sections. Only the first
              tile carries the accent rule, in line with the other
              sections on the page. */}
          <div className="mb-10">
            <StatRow>
              <StatTile
                label="Included in"
                value={formatNum(facts.includedIn.length)}
                caption={
                  facts.includedIn.length > 0
                    ? `other feeds contain 100% of this list`
                    : `no feed is a strict superset`
                }
                accent
              />
              <StatTile
                label="Includes"
                value={formatNum(facts.includes.length)}
                caption={
                  facts.includes.length > 0
                    ? `other feeds are fully contained in this one`
                    : `this feed is no feed's strict superset`
                }
              />
              <StatTile
                label="≥50% overlap"
                value={formatNum(facts.halfOrMore.length)}
                caption="other feeds share at least half of this list"
              />
              <StatTile
                label="Unique"
                value={
                  feedIPs > 0
                    ? `${((facts.uniqueIPs / feedIPs) * 100).toFixed(1)}%`
                    : "—"
                }
                caption={
                  feedIPs > 0
                    ? `${formatIPs(facts.uniqueIPs)} not covered by the strongest overlap`
                    : "no IPs to compare"
                }
              />
            </StatRow>
          </div>

          {showStaleStructuralWarning && (
            <DetailNotice
              title="Archived / unmaintained peers affect this overlap view"
              tone="warning"
              className="mb-10"
            >
              <p>
                This feed is not itself archived or unmaintained, but it has
                structural overlap with stale peers. High containment or very
                strong overlap can partly reflect inherited upstream composition,
                not only fresh independent agreement.
              </p>
              <ul className="mt-3 flex flex-wrap gap-x-6 gap-y-2">
                {staleStructuralRows.map((row) => (
                  <li key={row.name}>
                    <FeedRef
                      name={row.name}
                      feed={row.summary}
                      className="font-mono text-[13px] text-foreground underline-offset-4 hover:text-primary hover:underline"
                    />
                    <span className="ml-2 text-[12px] text-muted-foreground">
                      {overlapHealthLabel(row.health)} · {formatIPs(row.common)} shared
                    </span>
                  </li>
                ))}
              </ul>
            </DetailNotice>
          )}

          {facts.includedIn.length > 0 && (
            <InclusionList
              label="INCLUDED IN"
              description="Every IP in this list also appears in the feeds below."
              items={facts.includedIn}
            />
          )}
          {facts.includes.length > 0 && (
            <InclusionList
              label="INCLUDES"
              description="Every IP in the feeds below also appears in this list."
              items={facts.includes}
            />
          )}

          <div className="mt-10">
            <div className="mb-6 flex flex-wrap items-center justify-between gap-3">
              <div className="eyebrow">Pairwise overlap</div>
              <ViewTabBar>
                <ViewTab
                  label="List"
                  active={view === "table"}
                  onClick={() => setView("table")}
                />
                <ViewTab
                  label="Sankey"
                  active={view === "sankey"}
                  onClick={() => setView("sankey")}
                />
                <ViewTab
                  label="Network"
                  active={view === "network"}
                  onClick={() => setView("network")}
                />
              </ViewTabBar>
            </div>
            {/*
              All three views share the same total height. DataTable's
              new viewportHeight prop sizes the toolbar+scroll combo
              to exactly OVERLAP_VIEW_HEIGHT, matching the SVG height
              passed to sankey/network. Switching tabs no longer makes
              the section grow or shrink.
            */}
            {view === "table" && (
              <DataTable
                rows={displayRows}
                columns={columns}
                rowKey={(row) => row.name}
                initialSortKey="common"
                initialSortDir="desc"
                exportFilename={`overlap-${feedName}`}
                searchPlaceholder="Filter by feed name or category…"
                viewportHeight={OVERLAP_VIEW_HEIGHT}
              />
            )}
            {view === "sankey" && (
              <>
                {rows.length > 14 && (
                  <DetailNotice title="Top 14 overlaps only" className="mb-6">
                    The sankey view keeps only the 14 strongest overlaps so the
                    flows remain readable. Use the List view for the full
                    pairwise record.
                  </DetailNotice>
                )}
                <OverlapSankey
                  feedName={feedName}
                  feedIPs={feedIPs}
                  rows={rows}
                  topN={14}
                  height={OVERLAP_VIEW_HEIGHT}
                />
              </>
            )}
            {view === "network" && (
              <>
                {rows.length > 24 && (
                  <DetailNotice title="Top 24 overlaps only" className="mb-6">
                    The network view keeps only the 24 strongest overlaps so
                    the graph stays interpretable. Use the List view for the
                    full pairwise record.
                  </DetailNotice>
                )}
                <OverlapNetwork
                  feedName={feedName}
                  feedIPs={feedIPs}
                  rows={rows}
                  topN={24}
                  height={OVERLAP_VIEW_HEIGHT}
                />
              </>
            )}
          </div>
        </>
      )}
    </DetailSection>
  );
}

/* -------------------------------------------------------------------------- */

interface OverlapFacts {
  includedIn: ComparisonDisplayRow[];
  includes: ComparisonDisplayRow[];
  halfOrMore: ComparisonDisplayRow[];
  /** IPs in this feed that appear in NO other feed we track (approx — the
   *  backend emits pairwise data, so we take the smallest "uncovered"
   *  count across all overlaps and cap it at the feed size). */
  uniqueIPs: number;
}

function computeOverlapFacts(
  rows: ComparisonDisplayRow[],
  feedIPs: number,
): OverlapFacts {
  const includedIn: ComparisonDisplayRow[] = [];
  const includes: ComparisonDisplayRow[] = [];
  const halfOrMore: ComparisonDisplayRow[] = [];

  // The tiles compare against INDEPENDENT feeds only. Related
  // rows — retention variants of the same parent, merges that
  // contain this feed as an input, the parent source of a
  // retention variant, variants of merge inputs — are excluded
  // because their overlap with the current feed is trivially
  // explained by the shared ancestry rather than by genuine
  // cross-feed agreement. Including them makes the "unique IPs"
  // tile always zero for any feed that has a derivative, and
  // fills "INCLUDED IN" / "INCLUDES" with tautological
  // self-matches. Related rows still appear in the overlap table; the filter
  // only affects the four headline tiles.
  const independent = rows.filter((r) => !r.related);

  for (const r of independent) {
    const pctSelf = feedIPs > 0 ? r.common / feedIPs : 0;
    const pctOther = r.ips > 0 ? r.common / r.ips : 0;
    // "Included in": every IP of this feed is in the other.
    if (pctSelf >= 0.999 && feedIPs > 0) {
      includedIn.push(r);
    }
    // "Includes": every IP of the other feed is in this one.
    if (pctOther >= 0.999 && r.ips > 0) {
      includes.push(r);
    }
    if (pctSelf >= 0.5) {
      halfOrMore.push(r);
    }
  }

  // Cheap lower-bound estimate of truly unique IPs: the largest single
  // overlap defines the smallest known union; IPs beyond it are not
  // claimed by any single feed, but they may still appear in another.
  // For an exact value we'd need a set-subtraction pass on the backend.
  // This is clearly labelled as an estimate on the tile caption.
  let maxCommon = 0;
  for (const r of independent) if (r.common > maxCommon) maxCommon = r.common;
  const uniqueIPs = Math.max(0, feedIPs - maxCommon);

  return { includedIn, includes, halfOrMore, uniqueIPs };
}

function collectStaleStructuralRows(
  rows: ComparisonDisplayRow[],
): ComparisonDisplayRow[] {
  return rows
    .filter(
      (row) =>
        row.related &&
        isStaleHealth(row.health?.class),
    )
    .sort((left, right) => {
      const healthCmp = staleHealthRank(left.health?.class) - staleHealthRank(right.health?.class);
      if (healthCmp !== 0) return healthCmp;
      if (left.common !== right.common) return right.common - left.common;
      return left.name.localeCompare(right.name);
    });
}

function isStaleHealth(value: FeedHealthClass | undefined): boolean {
  return value === "archived" || value === "unmaintained";
}

function staleHealthRank(value: FeedHealthClass | undefined): number {
  switch (value) {
    case "archived":
      return 0;
    case "unmaintained":
      return 1;
    default:
      return 2;
  }
}

function OverlapHealth({ health }: { health: FeedHealthSnapshot | undefined }) {
  if (!health) {
    return <span className="text-muted-foreground">—</span>;
  }
  return (
    <HoverTip text={<FeedHealthTip health={health} compact />}>
      <span
        className={`whitespace-nowrap text-[13px] font-medium ${feedHealthColor(health.class)}`}
      >
        {feedHealthLabel(health.class)}
      </span>
    </HoverTip>
  );
}

function overlapHealthLabel(health: FeedHealthSnapshot | undefined): string {
  if (!health) return "—";
  return feedHealthLabel(health.class);
}

function InclusionList({
  label,
  description,
  items,
}: {
  label: string;
  description: string;
  items: ComparisonDisplayRow[];
}) {
  return (
    <div className="mt-10 border-l-[3px] border-primary/60 pl-6">
      <div className="eyebrow text-primary">{label}</div>
      <p className="mt-2 text-[15px] text-muted-foreground">{description}</p>
      <ul className="mt-4 flex flex-wrap gap-x-6 gap-y-2">
        {items.map((r) => (
          <li key={r.name}>
            <FeedRef
              name={r.name}
              feed={r.summary}
              className="font-mono text-[14px] text-foreground underline-offset-4 hover:text-primary hover:underline"
            />
            <span className="ml-2 text-[12px] text-muted-foreground">
              ({r.category}) · {overlapHealthLabel(r.health)} · {formatIPs(r.ips)} IPs
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}

interface ComparisonDisplayRow extends ComparisonRow {
  health?: FeedHealthSnapshot;
  summary?: FeedSummary;
}
