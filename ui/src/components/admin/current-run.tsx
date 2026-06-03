import { useState } from "react";
import { Clock, Play, RefreshCw } from "lucide-react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import type { AdminFeed, AdminStatus, HealthTransition } from "@/lib/api-types";
import { adminRunAll } from "@/lib/api-client/admin";
import { queryKeys } from "@/lib/query-keys";
import { HoverTip } from "@/components/editorial/hover-tip";
import { absoluteTime, relativeTime } from "@/lib/admin-format";
import { useNow } from "@/lib/use-now";
import {
  ActiveDownloadColumn,
  ProcessingNowColumn,
  QueueColumn,
} from "@/components/admin/current-run-queue-columns";
import {
  LIVE_QUEUE_EMPTY_CLASS,
  LIVE_QUEUE_VIEWPORT_CLASS,
  parseGoTime,
} from "@/components/admin/current-run-shared";

/**
 * Queue / processing panel — answers "what is the daemon doing
 * right now?" with only the four operator queues Costa asked
 * for:
 *
 *   1. waiting to be downloaded
 *   2. being downloaded now
 *   3. waiting to be processed
 *   4. being processed now
 *
 * The main feeds table remains the deep-dive surface; this panel
 * is only the live queue view.
 */
export function CurrentRunPanel({
  status,
  feeds,
  onFeedClick,
}: {
  status: AdminStatus | undefined;
  feeds: AdminFeed[];
  onFeedClick: (feed: AdminFeed) => void;
}) {
  const queryClient = useQueryClient();
  const [confirmAction, setConfirmAction] = useState<"run_due" | "reprocess_all" | null>(null);
  const nowMs = useNow();

  const invalidateAdmin = () => {
    queryClient.invalidateQueries({ queryKey: queryKeys.adminStatus() });
    queryClient.invalidateQueries({ queryKey: queryKeys.adminFeeds() });
    queryClient.invalidateQueries({ queryKey: queryKeys.adminIntegrityRoot() });
    queryClient.invalidateQueries({ queryKey: queryKeys.adminEntityIntegrity() });
  };

  const runAll = useMutation({
    mutationFn: () => adminRunAll(),
    onSuccess: () => {
      toast.success("Run due work scheduled");
      invalidateAdmin();
      setConfirmAction(null);
    },
    onError: (e: Error) => {
      toast.error(`Run failed: ${e.message}`);
      setConfirmAction(null);
    },
  });

  const reprocessAll = useMutation({
    mutationFn: () => adminRunAll({ reprocess: true }),
    onSuccess: () => {
      toast.success("Broad reprocess scheduled");
      invalidateAdmin();
      setConfirmAction(null);
    },
    onError: (e: Error) => {
      toast.error(`Reprocess failed: ${e.message}`);
      setConfirmAction(null);
    },
  });

  if (!status) return null;

  const running = status.engine.running;
  const lastReport = status.engine.last_report;
  const feedIndex = new Map(feeds.map((feed) => [feed.name, feed]));
  const downloadWaiting = [...(status.queues?.download_waiting ?? [])]
    .sort(
      (left, right) =>
        Number(left.blocked ?? false) - Number(right.blocked ?? false) ||
        parseGoTime(left.queued_at) - parseGoTime(right.queued_at) ||
        left.name.localeCompare(right.name),
    );
  const downloadActive = [...(status.queues?.download_active ?? [])].sort(
    (left, right) =>
      parseGoTime(left.started_at) - parseGoTime(right.started_at) ||
      left.name.localeCompare(right.name),
  );
  const processingWaiting = [...(status.queues?.processing_waiting ?? [])].sort(
    (left, right) =>
      parseGoTime(left.queued_at) - parseGoTime(right.queued_at) ||
      left.name.localeCompare(right.name),
  );
  const processingBatch = [...(status.queues?.processing_active ?? [])].sort(
    (left, right) =>
      parseGoTime(left.started_at) - parseGoTime(right.started_at) ||
      left.name.localeCompare(right.name),
  );
  const downloadRefetchPending = status.queues?.download_refetch_pending ?? [];
  const processingDeferred = status.queues?.processing_deferred ?? [];
  const recentHealthTransitions = status.queues?.recent_health_transitions ?? [];
  const backgroundTasks = [...(status.engine.background_tasks ?? [])].sort(
    (left, right) =>
      parseGoTime(left.started_at) - parseGoTime(right.started_at) ||
      left.name.localeCompare(right.name),
  );
  const backgroundRunning =
    status.engine.background_running ?? backgroundTasks.length;
  const backgroundLimit = status.engine.background_limit ?? 1;

  return (
    <section id="admin-current-run-panel" className="mb-10">
      <div className="mb-4 flex items-center gap-2">
        <Clock className="h-4 w-4 text-muted-foreground" />
        <span className="eyebrow">
          {running ? "Run in progress" : "Schedule"}
        </span>
        <div className="ml-auto flex items-center gap-3">
          {lastReport && (
            <HoverTip
              text={`Last run started ${absoluteTime(parseGoTime(lastReport.started_at))}`}
            >
              <span className="text-xs text-muted-foreground">
                last run:{" "}
                <span className="text-foreground">
                  {lastReport.updated?.length ?? 0} updated ·{" "}
                  {lastReport.skipped?.length ?? 0} skipped ·{" "}
                  <span
                    className={
                      (lastReport.failed?.length ?? 0) > 0
                        ? "text-destructive"
                        : ""
                    }
                  >
                    {lastReport.failed?.length ?? 0} failed
                  </span>
                </span>
              </span>
            </HoverTip>
          )}
          {confirmAction ? (
            <div className="inline-flex items-center gap-2">
              <button
                type="button"
                onClick={() =>
                  confirmAction === "run_due"
                    ? runAll.mutate()
                    : reprocessAll.mutate()
                }
                disabled={runAll.isPending || reprocessAll.isPending}
                className="inline-flex items-center gap-1.5 rounded-sm bg-primary px-3 py-1.5 text-[12px] font-semibold uppercase tracking-wider text-primary-foreground transition-opacity hover:opacity-90 disabled:opacity-50"
              >
                {runAll.isPending || reprocessAll.isPending ? (
                  <RefreshCw className="h-3 w-3 animate-spin" />
                ) : (
                  <Play className="h-3 w-3" />
                )}
                {confirmAction === "run_due"
                  ? "Confirm run due work"
                  : "Confirm broad reprocess"}
              </button>
              <button
                type="button"
                onClick={() => setConfirmAction(null)}
                className="text-xs text-muted-foreground hover:text-foreground"
              >
                cancel
              </button>
            </div>
          ) : (
            <>
              <button
                type="button"
                onClick={() => setConfirmAction("run_due")}
                disabled={running}
                className="inline-flex items-center gap-1.5 rounded-sm border border-border bg-card px-3 py-1.5 text-[12px] font-medium text-foreground transition-colors hover:border-foreground disabled:opacity-40"
              >
                <Play className="h-3 w-3" />
                Run due work now
              </button>
              <button
                type="button"
                onClick={() => setConfirmAction("reprocess_all")}
                className="inline-flex items-center gap-1.5 rounded-sm border border-border bg-card px-3 py-1.5 text-[12px] font-medium text-foreground transition-colors hover:border-foreground"
              >
                <RefreshCw className="h-3 w-3" />
                Force broad reprocess
              </button>
            </>
          )}
        </div>
      </div>

      {status.engine.last_config_reload_error && (
        <div className="mt-3 flex items-center gap-2 rounded-sm border border-destructive/40 bg-destructive/[0.03] px-4 py-2 text-xs text-destructive">
          Last config reload failed: {status.engine.last_config_reload_error}
        </div>
      )}

      <div className="grid gap-px overflow-hidden rounded-sm border border-border bg-border md:grid-cols-2 xl:grid-cols-4">
        <QueueColumn
          title="Waiting To Be Downloaded"
          items={downloadWaiting}
          feedIndex={feedIndex}
          onFeedClick={onFeedClick}
          itemLabel="item"
          emptyText="No item is waiting for a download worker."
          pendingCount={downloadRefetchPending.length}
          pendingItems={downloadRefetchPending}
        />
        <ActiveDownloadColumn
          items={downloadActive}
          feedIndex={feedIndex}
          nowMs={nowMs}
          onFeedClick={onFeedClick}
        />
        <QueueColumn
          title="Waiting To Be Processed"
          items={processingWaiting}
          feedIndex={feedIndex}
          onFeedClick={onFeedClick}
          itemLabel="feed"
          emptyText="No feed is waiting for the processing loop."
          pendingCount={processingDeferred.length}
          pendingItems={processingDeferred}
        />
        <ProcessingNowColumn
          running={running}
          currentPhase={status.engine.current_phase}
          processingBatch={processingBatch}
          feedIndex={feedIndex}
          nowMs={nowMs}
          onFeedClick={onFeedClick}
        />
      </div>
      <div className="mt-4 overflow-hidden rounded-sm border border-border bg-card">
        <div className="flex flex-wrap items-start justify-between gap-3 border-b border-border px-6 py-3">
          <div>
            <div className="eyebrow">Background Work</div>
            <p className="mt-1 text-sm text-muted-foreground">
              Daemon work running outside the four live feed queues.
            </p>
          </div>
          <div className="text-right text-xs text-muted-foreground tabular-nums">
            <div>
              workers:{" "}
              <span className="text-foreground">
                {backgroundRunning}/{backgroundLimit}
              </span>
            </div>
            <div>
              visible tasks:{" "}
              <span className="text-foreground">{backgroundTasks.length}</span>
            </div>
          </div>
        </div>
        <div className={LIVE_QUEUE_VIEWPORT_CLASS}>
          {backgroundTasks.length === 0 &&
          status.engine.entity_refresh_pending == null &&
          !(status.engine.entity_rebuild_pending && backgroundTasks.every((t) => t.name !== "Entity artifacts rebuild")) &&
          recentHealthTransitions.length === 0 ? (
            <div className={LIVE_QUEUE_EMPTY_CLASS}>
              No background maintenance task is currently running.
            </div>
          ) : (
            <>
              {status.engine.entity_rebuild_pending &&
                backgroundTasks.every((t) => t.name !== "Entity artifacts rebuild") && (
                  <div className="border-b border-border/60 px-6 py-3 text-sm text-muted-foreground">
                    Entity artifacts rebuild: waiting for worker slot
                  </div>
                )}
              {status.engine.entity_refresh_pending != null &&
                status.engine.entity_refresh_pending > 0 &&
                !backgroundTasks.some((t) => t.trigger === "feed_update" && t.name === "Entity artifacts refresh") && (
                  <div className="border-b border-border/60 px-6 py-3 text-sm text-muted-foreground">
                    Entity refresh: {status.engine.entity_refresh_pending} feeds coalescing
                  </div>
                )}
              {status.engine.entity_health_pending != null &&
                status.engine.entity_health_pending > 0 &&
                !backgroundTasks.some((t) => t.trigger === "health_transition" && t.name === "Entity artifacts refresh") && (
                  <div className="border-b border-border/60 px-6 py-3 text-sm text-muted-foreground">
                    Entity health refresh: {status.engine.entity_health_pending} feeds coalescing
                  </div>
                )}
              {backgroundTasks.map((task) => (
                <div
                  key={task.id}
                  className="border-b border-border/60 px-6 py-3 last:border-b-0"
                >
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div className="text-sm font-semibold text-foreground">
                      {task.name}
                    </div>
                    <div className="text-xs text-muted-foreground">
                      started {relativeTime(parseGoTime(task.started_at))}
                    </div>
                  </div>
                  <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
                    {task.trigger && (
                      <span>trigger: {formatBackgroundTrigger(task.trigger)}</span>
                    )}
                    {task.stage && <span>stage: {task.stage}</span>}
                    {typeof task.total === "number" && task.total > 0 && (
                      <span>
                        progress: {Math.min(task.current ?? 0, task.total)}/
                        {task.total}
                      </span>
                    )}
                    {task.trigger === "feed_update" &&
                      status.engine.entity_refresh_pending != null &&
                      status.engine.entity_refresh_pending > 0 && (
                        <span className="text-status-warning">
                          +{status.engine.entity_refresh_pending} more queued
                        </span>
                      )}
                    {task.trigger === "health_transition" &&
                      status.engine.entity_health_pending != null &&
                      status.engine.entity_health_pending > 0 && (
                        <span className="text-status-warning">
                          +{status.engine.entity_health_pending} more queued
                        </span>
                      )}
                  </div>
                  {task.detail && (
                    <div className="mt-2 text-sm text-foreground/90">
                      {task.detail}
                    </div>
                  )}
                </div>
              ))}
              {recentHealthTransitions.length > 0 && (
                <HealthTransitionsList transitions={recentHealthTransitions} />
              )}
            </>
          )}
        </div>
      </div>
    </section>
  );
}

function formatBackgroundTrigger(trigger: string): string {
  switch (trigger) {
    case "startup":
      return "startup";
    case "reload":
      return "config reload";
    case "bootstrap":
      return "artifact bootstrap";
    case "health_transition":
      return "health transition";
    case "feed_update":
      return "feed update";
    default:
      return trigger.replace(/_/g, " ");
  }
}

function HealthTransitionsList({
  transitions,
}: {
  transitions: HealthTransition[];
}) {
  const [expanded, setExpanded] = useState(false);
  const visible = expanded ? transitions : transitions.slice(0, 5);
  return (
    <div className="border-t border-border/60 px-6 py-3">
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        className="text-[11px] uppercase tracking-wider text-muted-foreground hover:text-foreground"
      >
        Recent health transitions ({transitions.length})
        {transitions.length > 5 && (expanded ? " · collapse" : " · show all")}
      </button>
      <ul className="mt-2 space-y-1">
        {visible.map((t) => (
          <li
            key={`${t.feed}-${t.at}`}
            className="text-xs text-muted-foreground"
          >
            <span className="font-mono text-foreground">{t.feed}</span>
            : {t.from_class || "new"} → {t.to_class},{" "}
            {relativeTime(parseGoTime(t.at))}
          </li>
        ))}
      </ul>
    </div>
  );
}
