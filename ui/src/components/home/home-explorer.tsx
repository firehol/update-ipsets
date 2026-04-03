import { lazy, Suspense, useCallback, useMemo, useState } from "react";
import { useLocation, useNavigate, useSearchParams } from "react-router-dom";
import { X } from "lucide-react";
import type { CategoryMeta, FeedSummary } from "@/lib/api-types";
import {
  applyFilters,
  applySort,
  defaultHealthSelection,
  distinctMaintainers,
  distinctLicenses,
  publicExplorerFeeds,
  readExplorerState,
  rememberExplorerView,
  writeExplorerState,
  type ExplorerState,
  type LensDefinition,
  type SortKey,
  type ViewMode,
} from "@/lib/explorer-state";
import { AccentBar } from "@/components/editorial/accent-bar";
import { HomeExplorerLensStrip } from "./home-explorer-lens-strip";
import { HomeExplorerFilterRail } from "./home-explorer-filter-rail";
import { HomeExplorerViewSwitcher } from "./home-explorer-view-switcher";
import { HomeExplorerViewCards } from "./home-explorer-view-cards";

const HomeExplorerViewTable = lazy(() =>
  import("./home-explorer-view-table").then((m) => ({
    default: m.HomeExplorerViewTable,
  })),
);

const HomeExplorerViewTreemap = lazy(() =>
  import("./home-explorer-view-treemap").then((m) => ({
    default: m.HomeExplorerViewTreemap,
  })),
);

const HomeExplorerViewTimeline = lazy(() =>
  import("./home-explorer-view-timeline").then((m) => ({
    default: m.HomeExplorerViewTimeline,
  })),
);

const HomeExplorerViewMaintainers = lazy(() =>
  import("./home-explorer-view-maintainers").then((m) => ({
    default: m.HomeExplorerViewMaintainers,
  })),
);

export function HomeExplorer({
  feeds,
  categories,
  loading,
}: {
  feeds: FeedSummary[];
  categories: CategoryMeta[];
  loading: boolean;
}) {
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams] = useSearchParams();
  const [drawerOpen, setDrawerOpen] = useState(false);

  const state = useMemo(
    () => readExplorerState(searchParams),
    [searchParams],
  );

  const commitState = useCallback(
    (next: ExplorerState) => {
      const nextParams = writeExplorerState(searchParams, next);
      const qs = nextParams.toString();
      navigate(
        {
          pathname: location.pathname,
          hash: "#explorer",
          search: qs ? `?${qs}` : "",
        },
        { replace: true },
      );
    },
    [navigate, location.pathname, searchParams],
  );

  const onChange = useCallback(
    (patch: Partial<ExplorerState>) => {
      commitState({ ...state, ...patch });
    },
    [commitState, state],
  );

  const onLens = useCallback(
    (lens: LensDefinition) => {
      const next = lens.apply(state);
      rememberExplorerView(next.view);
      commitState(next);
      setDrawerOpen(false);
    },
    [commitState, state],
  );

  const onSortChange = useCallback(
    (sort: SortKey) => commitState({ ...state, sort }),
    [commitState, state],
  );

  const onViewChange = useCallback(
    (view: ViewMode) => {
      rememberExplorerView(view);
      commitState({ ...state, view });
    },
    [commitState, state],
  );

  const trackedFeeds = useMemo(
    () => publicExplorerFeeds(feeds, categories),
    [feeds, categories],
  );
  const maintainers = useMemo(
    () => distinctMaintainers(trackedFeeds),
    [trackedFeeds],
  );
  const licenses = useMemo(
    () => distinctLicenses(trackedFeeds),
    [trackedFeeds],
  );

  const filtered = useMemo(
    () => applyFilters(trackedFeeds, state),
    [trackedFeeds, state],
  );
  const sorted = useMemo(
    () => applySort(filtered, state.sort),
    [filtered, state.sort],
  );

  const activeFilterCount = countActiveFilters(state);

  return (
    <section
      id="explorer"
      className="border-t border-border bg-background py-24 md:py-28"
    >
      <div className="page-container">
        <AccentBar />
        <div className="eyebrow mt-6 text-muted-foreground">
          Feed explorer
        </div>
        <h2 className="display-title mt-4 text-foreground">
          Explore every tracked feed.
        </h2>
        <p className="lede mt-5 max-w-[62ch] text-muted-foreground">
          Pick a lens to enter, then refine with filters. Every public feed this
          site tracks is here. Homepage rollups use a narrower active-feed
          subset, but explorer browsing stays on the full inventory.
        </p>

        <div className="mt-12">
          <HomeExplorerLensStrip
            activeLens={state.lens}
            onSelect={onLens}
          />
        </div>

        {/* Mobile: filter toggle button */}
        <div className="mt-6 flex items-center justify-between lg:hidden">
          <button
            type="button"
            onClick={() => setDrawerOpen(true)}
            className="inline-flex h-10 items-center gap-2 border border-border bg-background px-4 text-[13px] font-medium text-foreground transition hover:border-primary/60"
          >
            Filters
            {activeFilterCount > 0 && (
              <span className="inline-flex h-5 min-w-5 items-center justify-center rounded-full bg-primary px-1.5 text-[11px] font-semibold text-primary-foreground">
                {activeFilterCount}
              </span>
            )}
          </button>
          <div className="text-[12px] text-muted-foreground">
            <span className="font-semibold text-foreground">{sorted.length}</span>{" "}
            of <span className="font-semibold text-foreground">{trackedFeeds.length}</span>
          </div>
        </div>

        <div className="mt-6 grid gap-10 lg:mt-10 lg:grid-cols-[18rem_minmax(0,1fr)]">
          {/* Desktop filter rail */}
          <div className="hidden lg:block">
            <HomeExplorerFilterRail
              state={state}
              onChange={onChange}
              categories={categories}
              maintainers={maintainers}
              licenses={licenses}
              totalCount={trackedFeeds.length}
              visibleCount={sorted.length}
            />
          </div>

          <div className="min-w-0">
            <HomeExplorerViewSwitcher
              sort={state.sort}
              view={state.view}
              onSortChange={onSortChange}
              onViewChange={onViewChange}
            />
            <div className="mt-6">
              {loading ? (
                <div className="py-24 text-center text-[13px] text-muted-foreground">
                  Loading feeds…
                </div>
              ) : (
                <Suspense
                  fallback={
                    <div className="py-24 text-center text-[13px] text-muted-foreground">
                      Preparing view…
                    </div>
                  }
                >
                  <ExplorerView
                    view={state.view}
                    sort={state.sort}
                    feeds={sorted}
                    categories={categories}
                    onSortChange={onSortChange}
                  />
                </Suspense>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Mobile drawer */}
      {drawerOpen && (
        <div className="fixed inset-0 z-50 flex lg:hidden">
          <div
            className="absolute inset-0 bg-black/40"
            onClick={() => setDrawerOpen(false)}
            aria-hidden="true"
          />
          <aside className="relative ml-auto flex h-full w-[min(22rem,90vw)] flex-col overflow-y-auto border-l border-border bg-background p-6 shadow-xl">
            <div className="mb-6 flex items-center justify-between">
              <span className="eyebrow text-muted-foreground">Filters</span>
              <button
                type="button"
                onClick={() => setDrawerOpen(false)}
                className="inline-flex h-8 w-8 items-center justify-center border border-border text-muted-foreground hover:text-foreground"
                aria-label="Close filters"
              >
                <X className="h-4 w-4" />
              </button>
            </div>
            <HomeExplorerFilterRail
              state={state}
              onChange={onChange}
              categories={categories}
              maintainers={maintainers}
              licenses={licenses}
              totalCount={trackedFeeds.length}
              visibleCount={sorted.length}
            />
          </aside>
        </div>
      )}
    </section>
  );
}

function ExplorerView({
  view,
  sort,
  feeds,
  categories,
  onSortChange,
}: {
  view: ViewMode;
  sort: SortKey;
  feeds: FeedSummary[];
  categories: CategoryMeta[];
  onSortChange: (sort: SortKey) => void;
}) {
  switch (view) {
    case "table":
      return (
        <HomeExplorerViewTable
          feeds={feeds}
          sort={sort}
          onSortChange={onSortChange}
        />
      );
    case "treemap":
      return (
        <HomeExplorerViewTreemap feeds={feeds} categories={categories} />
      );
    case "timeline":
      return <HomeExplorerViewTimeline feeds={feeds} />;
    case "maintainers":
      return <HomeExplorerViewMaintainers feeds={feeds} />;
    case "cards":
    default:
      return <HomeExplorerViewCards feeds={feeds} sort={sort} />;
  }
}

function countActiveFilters(state: ExplorerState): number {
  let count = 0;
  const defaultHealth = defaultHealthSelection();
  if (state.q) count += 1;
  count += state.categories.size;
  count += state.maintainers.size;
  for (const item of state.health) {
    if (!defaultHealth.has(item)) count += 1;
  }
  for (const item of defaultHealth) {
    if (!state.health.has(item)) count += 1;
  }
  count += state.provenance.size;
  count += state.cadence.size;
  count += state.uniqueness.size;
  count += state.redistribution.size;
  count += state.criticalReference.size;
  count += state.criticalOverlap.size;
  if (state.license) count += 1;
  if (state.sizeMin !== null) count += 1;
  if (state.sizeMax !== null) count += 1;
  if (state.freshness) count += 1;
  return count;
}
