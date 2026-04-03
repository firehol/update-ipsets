import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import type { AdminFeed } from "@/lib/api-types";
import {
  adminFeedsOptions,
  adminIntegrityOptions,
  adminStatusOptions,
} from "@/lib/queries/admin";
import { HeartbeatPanel } from "@/components/admin/heartbeat";
import { CurrentRunPanel } from "@/components/admin/current-run";
import { ArtifactsPanel } from "@/components/admin/artifacts-panel";
import { IntegrityPanel } from "@/components/admin/integrity-panel";
import { EntityIntegrityPanel } from "@/components/admin/entity-integrity-panel";
import { FeedsTable } from "@/components/admin/feeds-table";
import { FeedModal } from "@/components/admin/feed-modal";
import { AdminCommandPalette } from "@/components/admin/admin-command-palette";
import type { FeedHealthFilter } from "@/lib/admin-format";
import {
  readListParam,
  writeListParam,
  writeTextParam,
} from "@/lib/admin-url-state";

const ADMIN_HEALTH_FILTERS: FeedHealthFilter[] = [
  "healthy",
  "delayed",
  "risky",
  "archived",
  "unavailable",
  "empty",
  "unmaintained",
];

/**
 * Operator admin console. Full-width, no public chrome (the
 * AdminLayout owns the header/footer). The page composes four
 * panels plus a feed-detail modal:
 *
 *   1. HeartbeatPanel — "is anything on fire?"
 *   2. CurrentRunPanel — "what is happening right now?"
 *   3. IntegrityPanel — "is the pipeline structurally broken?"
 *   4. EntityIntegrityPanel — "are precomputed country/ASN artifacts current?"
 *   5. FeedsTable — the main real estate, 14 sortable columns
 *
 * Every clickable feed name anywhere on the page (feeds table,
 * failing-list, upcoming-list) opens the same FeedModal with
 * 100% coverage of that feed's state + file manifest + actions.
 *
 * Polling cadence:
 *   - /admin/status     → 3s   (heartbeat feels live)
 *   - /admin/feeds      → 10s  (rich rows re-render is not free)
 *   - /admin/integrity  → manual only (stat()s every secondary)
 */
export function AdminPage() {
  const [searchParams, setSearchParams] = useSearchParams();

  const statusQuery = useQuery({
    ...adminStatusOptions(),
    retry: false,
  });

  const feedsQuery = useQuery({
    ...adminFeedsOptions(),
    refetchInterval: 10000,
    retry: false,
  });

  const integrityQuery = useQuery({
    ...adminIntegrityOptions(false),
    retry: false,
    refetchInterval: false,
  });

  const feeds = feedsQuery.data ?? [];
  const integrityFindings = integrityQuery.data?.findings ?? [];
  const integrityCount = integrityQuery.data?.count ?? 0;
  const healthFilters = readListParam<FeedHealthFilter>(
    searchParams,
    "health",
    ADMIN_HEALTH_FILTERS,
  );
  const selectedFeedName = searchParams.get("feed")?.trim() ?? "";

  const updateSearchParams = (
    updater: (next: URLSearchParams) => void,
    replace = true,
  ) => {
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        updater(next);
        return next;
      },
      { replace },
    );
  };

  // Keep the drawer's feed state in sync with polling updates
  // so the operator sees fresh data without closing it.
  const currentModalFeed =
    selectedFeedName !== ""
      ? (feeds.find((f) => f.name === selectedFeedName) ?? null)
      : null;

  const setHealthFilters = (filters: FeedHealthFilter[]) => {
    updateSearchParams((next) => writeListParam(next, "health", filters));
  };

  const openFeed = (feed: AdminFeed) => {
    updateSearchParams((next) => writeTextParam(next, "feed", feed.name), false);
  };
  const closeFeed = () => {
    updateSearchParams((next) => next.delete("feed"), false);
  };

  const summary = statusQuery.data?.feeds;
  const publicBaseURL = statusQuery.data
    ? (statusQuery.data.public_base_url ?? null)
    : undefined;

  return (
    <div className="admin-container py-6">
      {/* Page header: daemon identity + totals breakdown. The
          totals row explicitly calls out the configured /
          enabled / hidden split so the "1 feed delta" that
          previously confused operators is now self-explanatory. */}
      <header className="mb-8 flex flex-wrap items-end justify-between gap-4 border-b border-border pb-5">
        <div>
          <div className="eyebrow">Dashboard</div>
          <h1 className="mt-1 text-2xl font-bold tracking-tight">
            Pipeline overview
          </h1>
        </div>
        <div className="flex flex-wrap items-center justify-end gap-3">
          {summary && (
            <div className="text-xs text-muted-foreground tabular-nums">
              <span className="text-foreground font-medium">
                {summary.total_configured}
              </span>{" "}
              configured
              {" · "}
              <span className="text-foreground">
                {summary.total_enabled}
              </span>{" "}
              enabled
              {summary.disabled > 0 && (
                <>
                  {" · "}
                  <span>{summary.disabled}</span> disabled
                </>
              )}
              {summary.hidden > 0 && (
                <>
                  {" · "}
                  <span>{summary.hidden}</span> hidden
                </>
              )}
            </div>
          )}
          <AdminCommandPalette feeds={feeds} onFeedClick={openFeed} />
        </div>
      </header>

      <HeartbeatPanel
        data={statusQuery.data}
        loading={statusQuery.isLoading}
        error={statusQuery.error}
        integrityCount={integrityCount}
        onFilterByHealth={(h) => {
          setHealthFilters(h ? [h] : []);
          document
            .getElementById("admin-feeds-table")
            ?.scrollIntoView({ behavior: "smooth", block: "start" });
        }}
      />

      <CurrentRunPanel
        status={statusQuery.data}
        feeds={feeds}
        onFeedClick={openFeed}
      />

      <ArtifactsPanel
        status={statusQuery.data}
        feeds={feeds}
        onFeedClick={openFeed}
      />

      <IntegrityPanel />

      <EntityIntegrityPanel />

      <div id="admin-feeds-table">
        <FeedsTable
          feeds={feeds}
          loading={feedsQuery.isLoading}
          error={feedsQuery.error}
          integrityFindings={integrityFindings}
          healthFilters={healthFilters}
          onHealthFiltersChange={setHealthFilters}
          publicBaseURL={publicBaseURL}
          onFeedClick={openFeed}
        />
      </div>

      <FeedModal
        feed={currentModalFeed}
        integrityFinding={
          currentModalFeed
            ? integrityFindings.find((f) => f.feed === currentModalFeed.name)
            : undefined
        }
        open={selectedFeedName !== ""}
        publicBaseURL={publicBaseURL}
        onClose={closeFeed}
      />
    </div>
  );
}
