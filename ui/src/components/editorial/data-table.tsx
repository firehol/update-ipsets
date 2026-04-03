import { useMemo, useState, type ReactNode } from "react";
import { Download, Search } from "lucide-react";
import { cn } from "@/lib/utils";

/**
 * Editorial data table primitive. Replaces the old "Top 25 rows then
 * stop" pattern with a scrollable, sortable, searchable, exportable
 * table. Meant to be dropped into any section that has tabular data
 * without repeating the same boilerplate.
 *
 * Hairline-only visual treatment — matches the MinimalTable family.
 * No shadcn Card chrome, no row stripes. Generous row height, tabular
 * numerals on the numeric columns, uppercase eyebrow headers.
 *
 * The search box filters rows by the column values the consumer
 * flags as `searchable` (defaults to all string columns). The export
 * button downloads the CURRENTLY VISIBLE rows as CSV so users get the
 * filter + sort they see on screen.
 */

export interface DataTableColumn<Row> {
  /** Unique key (used for React keys and export CSV header). */
  key: string;
  /** Column label rendered in the <th>. */
  label: string;
  /** Cell renderer — receives the row, returns a ReactNode. */
  render: (row: Row) => ReactNode;
  /** Accessor used by normal sort comparisons and CSV export. */
  sortValue: (row: Row) => string | number;
  /** Optional ascending comparator for compound/domain-specific ordering. */
  compare?: (left: Row, right: Row) => number;
  /** Accessor used by the search filter. Default: sortValue stringified. */
  searchValue?: (row: Row) => string;
  /** Alignment for the cell and header. Defaults to "left". */
  align?: "left" | "right" | "center";
  /** Whether this column is sortable. Defaults to true. */
  sortable?: boolean;
  /** Whether this column is searchable. Defaults to true. */
  searchable?: boolean;
  /** Extra className on the <td>. */
  className?: string;
}

interface DataTableProps<Row> {
  /** The full data set. Don't pre-filter it outside — the table handles that. */
  rows: Row[];
  /** Column definitions in the order they should render. */
  columns: DataTableColumn<Row>[];
  /** Stable React key for each row. */
  rowKey: (row: Row) => string | number;
  /** Default sort column key. */
  initialSortKey?: string;
  /** Default sort direction. Defaults to "desc". */
  initialSortDir?: "asc" | "desc";
  /** CSV export filename (without extension). */
  exportFilename?: string;
  /**
   * Max height of the SCROLL REGION only (does NOT include the toolbar).
   * Default 540px. The table itself has no fixed height; it grows
   * inside a position:sticky thead container so long lists are
   * browseable inline.
   */
  maxHeight?: number;
  /**
   * Total desired height including the toolbar. When set, the scroll
   * region is sized so that toolbar + scroll = viewportHeight, which
   * lets a parent section that swaps tabs (e.g. Map | List, Treemap |
   * Bubble | List) keep all view heights identical and avoid the
   * page jumping when the user switches tabs. Takes precedence over
   * `maxHeight` when both are set.
   */
  viewportHeight?: number;
  /** Hide the toolbar (search + export). Defaults to false. */
  hideToolbar?: boolean;
  /** Placeholder for the search input. */
  searchPlaceholder?: string;
  /** Optional extra toolbar content rendered on the right. */
  toolbarExtra?: ReactNode;
}

// Approximate rendered height of the toolbar row (h-9 input, mb-4
// spacing, no top margin). Used by the viewportHeight calculation
// below so a parent that wants tab-stable view heights can specify
// one number that covers toolbar + scroll. Measured rather than
// guessed: a single-row toolbar with the input field + counter +
// export button consistently lands in this range.
const TOOLBAR_HEIGHT = 52;

export function DataTable<Row>({
  rows,
  columns,
  rowKey,
  initialSortKey,
  initialSortDir = "desc",
  exportFilename = "data",
  maxHeight = 540,
  viewportHeight,
  hideToolbar = false,
  searchPlaceholder = "Filter rows…",
  toolbarExtra,
}: DataTableProps<Row>) {
  // Resolve the effective scroll-region height. When the caller passes
  // viewportHeight (the total desired height including toolbar), we
  // subtract the toolbar so the scroll region fills exactly the
  // remaining space and toolbar+scroll = viewportHeight. When toolbar
  // is hidden the full viewport goes to the scroll region.
  const effectiveScrollHeight =
    viewportHeight !== undefined
      ? Math.max(120, viewportHeight - (hideToolbar ? 0 : TOOLBAR_HEIGHT))
      : maxHeight;
  const [sortKey, setSortKey] = useState<string | undefined>(
    initialSortKey ?? columns.find((c) => c.sortable !== false)?.key,
  );
  const [sortDir, setSortDir] = useState<"asc" | "desc">(initialSortDir);
  const [query, setQuery] = useState("");

  const columnByKey = useMemo(() => {
    const m = new Map<string, DataTableColumn<Row>>();
    for (const col of columns) m.set(col.key, col);
    return m;
  }, [columns]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return rows;
    return rows.filter((row) => {
      for (const col of columns) {
        if (col.searchable === false) continue;
        const raw =
          col.searchValue?.(row) ?? String(col.sortValue(row));
        if (raw.toLowerCase().includes(q)) return true;
      }
      return false;
    });
  }, [rows, columns, query]);

  const sorted = useMemo(() => {
    if (!sortKey) return filtered;
    const col = columnByKey.get(sortKey);
    if (!col) return filtered;
    const dir = sortDir === "asc" ? 1 : -1;
    return [...filtered].sort((a, b) => {
      if (col.compare) {
        return col.compare(a, b) * dir;
      }
      const va = col.sortValue(a);
      const vb = col.sortValue(b);
      if (typeof va === "number" && typeof vb === "number") {
        return (va - vb) * dir;
      }
      const sa = String(va);
      const sb = String(vb);
      return sa.localeCompare(sb) * dir;
    });
  }, [filtered, columnByKey, sortKey, sortDir]);

  const toggleSort = (key: string) => {
    const col = columnByKey.get(key);
    if (!col || col.sortable === false) return;
    if (sortKey === key) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
      return;
    }
    setSortKey(key);
    setSortDir(col.align === "right" ? "desc" : "asc");
  };

  const exportCSV = () => {
    const header = columns.map((c) => escapeCSV(c.label)).join(",");
    const lines = sorted.map((row) =>
      columns.map((c) => escapeCSV(String(c.sortValue(row)))).join(","),
    );
    const csv = [header, ...lines].join("\n");
    const blob = new Blob([csv], { type: "text/csv;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `${exportFilename}.csv`;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  };

  return (
    <div>
      {!hideToolbar && (
        <div className="mb-4 flex flex-wrap items-center gap-3">
          <label className="relative flex-1 min-w-[220px]">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
            <input
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={searchPlaceholder}
              className="h-9 w-full rounded-md border border-border bg-transparent pl-9 pr-3 text-[13px] text-foreground placeholder:text-muted-foreground focus:border-primary/60 focus:outline-none"
            />
          </label>
          {toolbarExtra}
          <div className="text-xs text-muted-foreground tabular-nums">
            {sorted.length === rows.length ? (
              <>{rows.length.toLocaleString()} rows</>
            ) : (
              <>
                {sorted.length.toLocaleString()} of{" "}
                {rows.length.toLocaleString()} rows
              </>
            )}
          </div>
          <button
            type="button"
            onClick={exportCSV}
            className="inline-flex h-9 items-center gap-2 rounded-md border border-border bg-transparent px-3 text-[13px] font-medium text-foreground transition-colors hover:border-primary/60 hover:text-primary"
          >
            <Download className="h-3.5 w-3.5" />
            Export CSV
          </button>
        </div>
      )}

      <div
        className="overflow-auto rounded-md border border-border"
        style={{ maxHeight: effectiveScrollHeight }}
      >
        <table className="w-full border-collapse text-[14px]">
          <thead className="sticky top-0 z-10 bg-card">
            <tr className="border-b border-border">
              {columns.map((col) => {
                const isActive = sortKey === col.key;
                const sortable = col.sortable !== false;
                const nextSortDir =
                  isActive
                    ? sortDir === "asc"
                      ? "descending"
                      : "ascending"
                    : col.align === "right"
                      ? "descending"
                      : "ascending";
                return (
                  <th
                    key={col.key}
                    scope="col"
                    aria-sort={
                      isActive
                        ? sortDir === "asc"
                          ? "ascending"
                          : "descending"
                        : sortable
                          ? "none"
                          : undefined
                    }
                    className={cn(
                      "eyebrow whitespace-nowrap bg-card px-3 py-3 first:pl-5 last:pr-5",
                      col.align === "right" && "text-right",
                      col.align === "center" && "text-center",
                      col.align !== "right" && col.align !== "center" && "text-left",
                      isActive && "text-foreground",
                    )}
                  >
                    {sortable ? (
                      <button
                        type="button"
                        onClick={() => toggleSort(col.key)}
                        aria-label={`Sort by ${col.label} ${nextSortDir}`}
                        className={cn(
                          "inline-flex items-center gap-1 transition-colors hover:text-foreground focus:outline-none focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary",
                          col.align === "right" && "justify-end",
                          col.align === "center" && "justify-center",
                        )}
                      >
                        <span>{col.label}</span>
                        <span
                          aria-hidden="true"
                          className="text-[10px] text-muted-foreground"
                        >
                          {isActive ? (sortDir === "asc" ? "▲" : "▼") : "↕"}
                        </span>
                      </button>
                    ) : (
                      <span>{col.label}</span>
                    )}
                  </th>
                );
              })}
            </tr>
          </thead>
          <tbody>
            {sorted.length === 0 ? (
              <tr>
                <td
                  colSpan={columns.length}
                  className="px-5 py-8 text-center text-sm text-muted-foreground"
                >
                  No rows match.
                </td>
              </tr>
            ) : (
              sorted.map((row) => (
                <tr
                  key={rowKey(row)}
                  className="border-b border-border/60 transition-colors hover:bg-muted/40 last:border-b-0"
                >
                  {columns.map((col) => (
                    <td
                      key={col.key}
                      className={cn(
                        "px-3 py-3 first:pl-5 last:pr-5",
                        col.align === "right" && "text-right tabular-nums",
                        col.align === "center" && "text-center",
                        col.align !== "right" && col.align !== "center" && "text-left",
                        col.className,
                      )}
                    >
                      {col.render(row)}
                    </td>
                  ))}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function escapeCSV(value: string): string {
  if (/[",\n]/.test(value)) {
    return `"${value.replace(/"/g, '""')}"`;
  }
  return value;
}
