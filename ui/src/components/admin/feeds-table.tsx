import { useMemo } from "react";
import { useSearchParams } from "react-router-dom";
import { Search, X } from "lucide-react";
import type { AdminFeed, IntegrityFinding } from "@/lib/api-types";
import { useNow } from "@/lib/use-now";
import { type FeedHealthFilter } from "@/lib/admin-format";
import {
  readListParam,
  writeListParam,
  writeTextParam,
} from "@/lib/admin-url-state";
import { FeedsTableBody } from "@/components/admin/feeds-table-body";
import { MultiSelectChipRow } from "@/components/admin/feeds-table-filters";
import {
  BOOLEAN_FILTER_IDS,
  BOOLEAN_FILTERS,
  HEALTH_FILTERS,
  KIND_FILTER_IDS,
  KIND_FILTERS,
  compareByKey,
  compareDefault,
  computeBooleanCounts,
  computeCategoryCounts,
  computeHealthCounts,
  computeKindCounts,
  type FeedFacetState,
  matchesFacetState,
  readSortDir,
  readSortKey,
  type SortKey,
} from "@/components/admin/feeds-table-model";

export function FeedsTable({
  feeds,
  loading,
  error,
  integrityFindings,
  healthFilters,
  onHealthFiltersChange,
  publicBaseURL,
  onFeedClick,
}: {
  feeds: AdminFeed[];
  loading: boolean;
  error: unknown;
  integrityFindings: IntegrityFinding[];
  healthFilters: FeedHealthFilter[];
  onHealthFiltersChange: (filters: FeedHealthFilter[]) => void;
  publicBaseURL?: string | null;
  onFeedClick: (feed: AdminFeed) => void;
}) {
  const [searchParams, setSearchParams] = useSearchParams();
  const nowMs = useNow();

  const updateSearchParams = (updater: (next: URLSearchParams) => void) => {
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        updater(next);
        return next;
      },
      { replace: true },
    );
  };

  const integrityByFeed = useMemo(() => {
    const m = new Map<string, IntegrityFinding>();
    for (const f of integrityFindings) m.set(f.feed, f);
    return m;
  }, [integrityFindings]);

  const categoryFiltersAvailable = useMemo(
    () =>
      [
        ...new Set(feeds.map((f) => f.category).filter(Boolean) as string[]),
      ].sort((a, b) => a.localeCompare(b)),
    [feeds],
  );
  const kindFilters = readListParam(searchParams, "kind", KIND_FILTER_IDS);
  const categoryFilters = readListParam<string>(
    searchParams,
    "category",
  ).filter((category) => categoryFiltersAvailable.includes(category));
  const hiddenFilters = readListParam(searchParams, "hidden", BOOLEAN_FILTER_IDS);
  const disabledFilters = readListParam(
    searchParams,
    "disabled",
    BOOLEAN_FILTER_IDS,
  );
  const search = searchParams.get("q") ?? "";
  const sortKey = readSortKey(searchParams.get("sort"));
  const sortDir = readSortDir(searchParams.get("dir"));

  const setKindFilters = (values: string[]) =>
    updateSearchParams((next) => writeListParam(next, "kind", values));
  const setCategoryFilters = (values: string[]) =>
    updateSearchParams((next) => writeListParam(next, "category", values));
  const setHiddenFilters = (values: string[]) =>
    updateSearchParams((next) => writeListParam(next, "hidden", values));
  const setDisabledFilters = (values: string[]) =>
    updateSearchParams((next) => writeListParam(next, "disabled", values));
  const setSearch = (value: string) =>
    updateSearchParams((next) => writeTextParam(next, "q", value));

  const searchTerm = search.trim().toLowerCase();

  const baseFacetState = useMemo<FeedFacetState>(
    () => ({
      health: healthFilters,
      kind: kindFilters,
      category: categoryFilters,
      hidden: hiddenFilters,
      disabled: disabledFilters,
      search: searchTerm,
    }),
    [
      healthFilters,
      kindFilters,
      categoryFilters,
      hiddenFilters,
      disabledFilters,
      searchTerm,
    ],
  );

  const feedsMatchingStructuredFilters = useMemo(
    () =>
      feeds.filter((f) =>
        matchesFacetState(f, { ...baseFacetState, search: "" }),
      ),
    [feeds, baseFacetState],
  );

  const filteredFeeds = useMemo(() => {
    const list = feeds.filter((f) => matchesFacetState(f, baseFacetState));
    if (sortKey) {
      return [...list].sort(compareByKey(sortKey, sortDir));
    }
    return [...list].sort(compareDefault);
  }, [feeds, baseFacetState, sortKey, sortDir]);

  const healthCounts = useMemo(
    () =>
      computeHealthCounts(
        feeds.filter((f) =>
          matchesFacetState(f, { ...baseFacetState, excludeAxis: "health" }),
        ),
      ),
    [feeds, baseFacetState],
  );

  const kindCounts = useMemo(
    () =>
      computeKindCounts(
        feeds.filter((f) =>
          matchesFacetState(f, { ...baseFacetState, excludeAxis: "kind" }),
        ),
      ),
    [feeds, baseFacetState],
  );

  const categoryCounts = useMemo(
    () =>
      computeCategoryCounts(
        feeds.filter((f) =>
          matchesFacetState(f, { ...baseFacetState, excludeAxis: "category" }),
        ),
      ),
    [feeds, baseFacetState],
  );

  const hiddenCounts = useMemo(
    () =>
      computeBooleanCounts(
        feeds.filter((f) =>
          matchesFacetState(f, { ...baseFacetState, excludeAxis: "hidden" }),
        ),
        (f) => !!f.hidden,
      ),
    [feeds, baseFacetState],
  );

  const disabledCounts = useMemo(
    () =>
      computeBooleanCounts(
        feeds.filter((f) =>
          matchesFacetState(f, { ...baseFacetState, excludeAxis: "disabled" }),
        ),
        (f) => !f.enabled,
      ),
    [feeds, baseFacetState],
  );

  const toggleSort = (key: SortKey) => {
    updateSearchParams((next) => {
      if (sortKey === key) {
        next.set("sort", key);
        next.set("dir", sortDir === "asc" ? "desc" : "asc");
        return;
      }
      next.set("sort", key);
      next.set("dir", "asc");
    });
  };
  const toggleMultiFilter = (
    value: string,
    selected: string[],
    setter: (next: string[]) => void,
  ) => {
    setter(
      selected.includes(value)
        ? selected.filter((item) => item !== value)
        : [...selected, value],
    );
  };
  const clearSort = () => {
    updateSearchParams((next) => {
      next.delete("sort");
      next.delete("dir");
    });
  };
  const visibleCount = feedsMatchingStructuredFilters.length;
  const hasStructuredFilters =
    healthFilters.length > 0 ||
    kindFilters.length > 0 ||
    categoryFilters.length > 0 ||
    hiddenFilters.length > 0 ||
    disabledFilters.length > 0;

  return (
    <section className="mb-12">
      <div className="mb-4 flex flex-wrap items-center gap-3">
        <span className="eyebrow">Feeds</span>
        <span className="text-xs text-muted-foreground">
          {filteredFeeds.length === visibleCount
            ? `${filteredFeeds.length.toLocaleString()} feeds`
            : `${filteredFeeds.length.toLocaleString()} of ${visibleCount.toLocaleString()} feeds`}
        </span>
        {sortKey && (
          <button
            type="button"
            onClick={clearSort}
            className="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
          >
            <X className="h-3 w-3" />
            sorted by {sortKey} ({sortDir}) — click to reset
          </button>
        )}
        <div className="ml-auto relative">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            aria-label="Search admin feeds"
            placeholder="Search name, category, URL, maintainer, error, scheduler state…"
            className="h-9 w-[min(480px,calc(100vw-2rem))] rounded-sm border border-border bg-card pl-9 pr-8 text-sm focus:border-primary focus:outline-none"
          />
          {search && (
            <button
              type="button"
              onClick={() => setSearch("")}
              className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
              aria-label="Clear search"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          )}
        </div>
      </div>

      <div className="space-y-2">
        <MultiSelectChipRow
          label="Health"
          filters={HEALTH_FILTERS}
          selected={healthFilters}
          onToggle={(id) =>
            toggleMultiFilter(id, healthFilters, (next) =>
              onHealthFiltersChange(next as FeedHealthFilter[]),
            )
          }
          onClear={() => onHealthFiltersChange([])}
          counts={healthCounts}
        />
        <MultiSelectChipRow
          label="Kind"
          filters={KIND_FILTERS}
          selected={kindFilters}
          onToggle={(id) => toggleMultiFilter(id, kindFilters, setKindFilters)}
          onClear={() => setKindFilters([])}
          counts={kindCounts}
        />
        <MultiSelectChipRow
          label="Category"
          filters={categoryFiltersAvailable.map((category) => ({
            id: category,
            label: category,
          }))}
          selected={categoryFilters}
          onToggle={(id) =>
            toggleMultiFilter(id, categoryFilters, setCategoryFilters)
          }
          onClear={() => setCategoryFilters([])}
          counts={categoryCounts}
        />
        <MultiSelectChipRow
          label="Hidden"
          filters={BOOLEAN_FILTERS}
          selected={hiddenFilters}
          onToggle={(id) =>
            toggleMultiFilter(id, hiddenFilters, setHiddenFilters)
          }
          onClear={() => setHiddenFilters([])}
          counts={hiddenCounts}
        />
        <MultiSelectChipRow
          label="Disabled"
          filters={BOOLEAN_FILTERS}
          selected={disabledFilters}
          onToggle={(id) =>
            toggleMultiFilter(id, disabledFilters, setDisabledFilters)
          }
          onClear={() => setDisabledFilters([])}
          counts={disabledCounts}
        />
      </div>
      {hasStructuredFilters && (
        <div className="mt-3 flex justify-end">
          <button
            type="button"
            onClick={() => {
              onHealthFiltersChange([]);
              updateSearchParams((next) => {
                next.delete("kind");
                next.delete("category");
                next.delete("hidden");
                next.delete("disabled");
              });
            }}
            className="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
          >
            <X className="h-3 w-3" />
            clear filters
          </button>
        </div>
      )}

      <div className="mt-5">
        {loading && <div className="h-96 animate-pulse bg-muted/40" />}
        {!loading && Boolean(error) && (
          <p className="py-10 text-center text-sm text-destructive">
            Could not load feeds: {(error as Error).message}
          </p>
        )}
        {!loading && !error && (
          <FeedsTableBody
            feeds={filteredFeeds}
            integrityByFeed={integrityByFeed}
            sortKey={sortKey}
            sortDir={sortDir}
            onSort={toggleSort}
            publicBaseURL={publicBaseURL}
            onFeedClick={onFeedClick}
            nowMs={nowMs}
          />
        )}
      </div>
    </section>
  );
}
