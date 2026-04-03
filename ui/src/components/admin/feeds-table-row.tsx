import { memo } from "react";
import { ExternalLink, EyeOff, Lock, ShieldAlert } from "lucide-react";
import { CategoryBadge } from "@/components/category-badge";
import type { AdminFeed, IntegrityFinding } from "@/lib/api-types";
import {
  feedHealth,
  formatDuration,
  formatFrequency,
  formatNumber,
  healthColor,
  kindLabel,
  lastStatusLabel,
  problemClassLabel,
  problemClassTone,
  relativeTime,
  scheduleDetail,
  scheduleNextCheckLabel,
  usesInputTriggeredSchedule,
} from "@/lib/admin-format";
import { formatRunReason } from "@/lib/admin-run-reason";
import { publicFeedURL } from "@/lib/public-url";
import { cn } from "@/lib/utils";
import { cadenceRatio } from "@/components/admin/feeds-table-model";

export const FeedRow = memo(function FeedRow({
  feed,
  finding,
  publicBaseURL,
  onFeedClick,
  nowMs,
}: {
  feed: AdminFeed;
  finding: IntegrityFinding | undefined;
  publicBaseURL?: string | null;
  onFeedClick: (feed: AdminFeed) => void;
  nowMs: number;
}) {
  const health = feedHealth(feed);
  const overdueThresholdMs = Math.max(
    60_000,
    (feed.frequency_minutes || 0) * 6_000,
  );
  const inputTriggered = usesInputTriggeredSchedule(feed);
  const overdue =
    !inputTriggered &&
    nowMs > 0 &&
    feed.next_check > 0 &&
    feed.next_check * 1000 < nowMs - overdueThresholdMs;
  const missingCount = finding?.missing_files?.length ?? 0;
  const staleCount = finding?.stale_files?.length ?? 0;
  const cadence = cadenceRatio(feed.frequency_minutes, feed.avg_update_mins);
  const scheduleState = scheduleDetail(feed);
  const statusLabel = lastStatusLabel(feed);
  const problemLabel = problemClassLabel(feed.last_problem_class);
  const publicHref = publicFeedURL(publicBaseURL, feed.name);
  const openFeed = () => onFeedClick(feed);
  return (
    <tr className="border-b border-border/60 transition-colors hover:bg-muted/40">
      <td className="py-3 pl-3 pr-3 align-top">
        <button
          type="button"
          className="flex min-w-0 items-center gap-2 text-left"
          aria-label={`Open ${feed.name} feed details`}
          onClick={openFeed}
        >
          <span className={cn("text-base leading-none", healthColor(health))}>
            ●
          </span>
          <span className="font-mono text-[12px] font-medium text-foreground break-all">
            {feed.name}
          </span>
        </button>
        <div className="mt-0.5 text-[10px] text-muted-foreground">
          {kindLabel(feed.kind)}
          {feed.version ? ` · v${feed.version}` : ""}
          {feed.health.class === "archived" ? " · archived" : ""}
          {feed.maintainer ? ` · ${feed.maintainer}` : ""}
        </div>
      </td>

      <td className="py-3 px-2 align-top">
        {feed.category && <CategoryBadge category={feed.category} />}
      </td>

      <td className="py-3 px-2 text-center align-top">
        <VisibilityBadge feed={feed} />
      </td>

      <td className="py-3 px-2 text-right align-top">
        <div className="tabular-nums">
          {feed.frequency_minutes > 0
            ? formatFrequency(feed.frequency_minutes).replace("every ", "")
            : "—"}
        </div>
      </td>

      <td className="py-3 px-2 text-right align-top">
        {feed.avg_update_mins && feed.avg_update_mins > 0 ? (
          <div>
            <div className="tabular-nums">
              {formatFrequency(feed.avg_update_mins).replace("every ", "")}
            </div>
            {cadence && (
              <div className={cn("text-[10px] tabular-nums", cadence.color)}>
                {cadence.label}
              </div>
            )}
          </div>
        ) : (
          <span className="text-muted-foreground">—</span>
        )}
      </td>

      <td className="py-3 px-2 text-right align-top">
        <span
          className={cn(
            "tabular-nums",
            overdue ? "text-destructive" : "text-foreground",
          )}
        >
          {scheduleNextCheckLabel(feed)}
        </span>
      </td>

      <td className="py-3 px-2 text-right align-top">
        <span className="tabular-nums">
          {relativeTime(feed.processed_date)}
        </span>
      </td>

      <td className="py-3 px-2 align-top">
        <span className="text-foreground">
          {formatRunReason(feed.last_run_reason)}
        </span>
      </td>

      <td className="py-3 px-2 text-right align-top">
        <span className="tabular-nums">
          {feed.last_processing_ms && feed.last_processing_ms > 0
            ? formatDuration(feed.last_processing_ms)
            : "—"}
        </span>
      </td>

      <td className="py-3 px-2 text-right align-top">
        <span className="tabular-nums text-muted-foreground">
          {relativeTime(feed.last_update)}
        </span>
      </td>

      <td className="py-3 px-2 text-right align-top">
        <span className="tabular-nums">{formatNumber(feed.unique_ips)}</span>
      </td>

      <td className="py-3 px-2 text-right align-top">
        <span className="tabular-nums text-muted-foreground">
          {formatNumber(feed.entries)}
        </span>
      </td>

      <td className="py-3 px-3 align-top">
        <div className="space-y-0.5">
          <div className="text-foreground">{statusLabel}</div>
          {problemLabel && (
            <div
              className={cn(
                "text-[10px] font-medium uppercase tracking-[0.08em]",
                problemClassTone(feed.last_problem_class),
              )}
            >
              {problemLabel}
            </div>
          )}
          {feed.last_error && (
            <div className="break-words whitespace-normal text-destructive">
              {feed.last_error}
            </div>
          )}
          {scheduleState && (
            <div className="break-words whitespace-normal text-[10px] text-muted-foreground">
              {scheduleState}
            </div>
          )}
        </div>
      </td>

      <td className="py-3 px-2 text-right align-top">
        {feed.download_failures > 0 ? (
          <span className="tabular-nums text-destructive font-medium">
            {feed.download_failures}
          </span>
        ) : (
          <span className="text-muted-foreground">—</span>
        )}
      </td>

      <td className="py-3 px-2 align-top">
        {finding ? (
          <span className="inline-flex items-center gap-1 text-destructive">
            <ShieldAlert className="h-3 w-3" />
            <span className="tabular-nums text-[10px]">
              {missingCount > 0 && `${missingCount}m`}
              {missingCount > 0 && staleCount > 0 && "/"}
              {staleCount > 0 && `${staleCount}s`}
            </span>
          </span>
        ) : (
          <span className="text-[10px] text-muted-foreground">clean</span>
        )}
      </td>

      <td className="py-3 pr-3 text-right align-top">
        {publicHref && (
          <a
            href={publicHref}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex h-6 w-6 items-center justify-center rounded-sm border border-border text-muted-foreground transition-colors hover:border-foreground hover:text-foreground"
            aria-label="Open public feed page"
          >
            <ExternalLink className="h-3 w-3" />
          </a>
        )}
      </td>
    </tr>
  );
}, areFeedRowPropsEqual);

function VisibilityBadge({ feed }: { feed: AdminFeed }) {
  if (feed.hidden) {
    return <EyeOff className="mx-auto h-3.5 w-3.5 text-muted-foreground" />;
  }
  if (!feed.redistributable) {
    return <Lock className="mx-auto h-3.5 w-3.5 text-status-warning" />;
  }
  return (
    <span className="inline-block h-1.5 w-1.5 rounded-full bg-status-healthy" />
  );
}

function areFeedRowPropsEqual(
  prev: Readonly<{
    feed: AdminFeed;
    finding: IntegrityFinding | undefined;
    publicBaseURL?: string | null;
    onFeedClick: (feed: AdminFeed) => void;
    nowMs: number;
  }>,
  next: Readonly<{
    feed: AdminFeed;
    finding: IntegrityFinding | undefined;
    publicBaseURL?: string | null;
    onFeedClick: (feed: AdminFeed) => void;
    nowMs: number;
  }>,
): boolean {
  return (
    prev.feed === next.feed &&
    prev.finding === next.finding &&
    prev.publicBaseURL === next.publicBaseURL &&
    prev.nowMs === next.nowMs
  );
}
