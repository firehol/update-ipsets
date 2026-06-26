import { useState } from "react";
import { Clock, Play, RefreshCw } from "lucide-react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import type {
  AdminEngineLaneWork,
  AdminFeed,
  AdminQueueItem,
  AdminStatus,
  HealthTransition,
} from "@/lib/api-types";
import { adminRunAll } from "@/lib/api-client/admin";
import { queryKeys } from "@/lib/query-keys";
import { HoverTip } from "@/components/editorial/hover-tip";
import {
  absoluteTime,
  formatDuration,
  relativeTime,
} from "@/lib/admin-format";
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
 * Queue / processing panel answers "what is the daemon doing
 * right now?" with only the four requested operator queues:
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
  const [confirmAction, setConfirmAction] = useState<
    "run_due" | "reprocess_all" | null
  >(null);
  const nowMs = useNow();

  const invalidateAdmin = () => {
    queryClient.invalidateQueries({ queryKey: queryKeys.adminStatus() });
    queryClient.invalidateQueries({ queryKey: queryKeys.adminFeeds() });
    queryClient.invalidateQueries({ queryKey: queryKeys.adminIntegrityRoot() });
    queryClient.invalidateQueries({
      queryKey: queryKeys.adminEntityIntegrity(),
    });
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
  const runState = status.engine.run_state ?? (running ? "running" : "idle");
  const cachePersistence = status.engine.cache_persistence;
  const cachePersistenceActive =
    cachePersistence?.state === "pending" ||
    cachePersistence?.state === "saving" ||
    cachePersistence?.state === "failed";
  const lastReport = status.engine.last_report;
  const feedIndex = new Map(feeds.map((feed) => [feed.name, feed]));
  const downloadRefetchPending = status.queues?.download_refetch_pending ?? [];
  const processingDeferred = status.queues?.processing_deferred ?? [];
  const downloadWaiting = [
    ...(status.queues?.download_waiting ?? []),
    ...blockedQueueItems(
      downloadRefetchPending,
      "blocked by active download; refetch after it finishes",
    ),
  ].sort(
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
  const processingWaiting = [
    ...(status.queues?.processing_waiting ?? []),
    ...blockedQueueItems(
      processingDeferred,
      "blocked by active processing batch; rerun after it finishes",
    ),
  ].sort(
    (left, right) =>
      Number(left.blocked ?? false) - Number(right.blocked ?? false) ||
      parseGoTime(left.queued_at) - parseGoTime(right.queued_at) ||
      left.name.localeCompare(right.name),
  );
  const processingBatch = [...(status.queues?.processing_active ?? [])].sort(
    (left, right) =>
      parseGoTime(left.started_at) - parseGoTime(right.started_at) ||
      left.name.localeCompare(right.name),
  );
  const recentHealthTransitions =
    status.queues?.recent_health_transitions ?? [];
  const backgroundTasks = [...(status.engine.background_tasks ?? [])].sort(
    (left, right) =>
      parseGoTime(left.started_at) - parseGoTime(right.started_at) ||
      left.name.localeCompare(right.name),
  );
  const isEntityRebuildTask = (task: (typeof backgroundTasks)[number]) =>
    task.kind === "entity_rebuild" || task.name === "Entity artifacts rebuild";
  const engineLane = status.engine.engine_lane;
  const gitLane = status.engine.git_lane;
  const engineLaneActive = [...(engineLane?.active ?? [])].sort(
    (left, right) =>
      parseGoTime(left.started_at) - parseGoTime(right.started_at) ||
      left.name.localeCompare(right.name),
  );
  const engineLaneWaiting = [...(engineLane?.waiting ?? [])].sort(
    (left, right) =>
      parseGoTime(left.queued_at) - parseGoTime(right.queued_at) ||
      left.name.localeCompare(right.name),
  );
  const gitLaneActive = [...(gitLane?.active ?? [])].sort(
    (left, right) =>
      parseGoTime(left.started_at) - parseGoTime(right.started_at) ||
      left.name.localeCompare(right.name),
  );
  const gitLaneWaiting = [...(gitLane?.waiting ?? [])].sort(
    (left, right) =>
      parseGoTime(left.queued_at) - parseGoTime(right.queued_at) ||
      left.name.localeCompare(right.name),
  );
  const engineLaneWorkCount =
    engineLaneActive.length +
    engineLaneWaiting.length +
    gitLaneActive.length +
    gitLaneWaiting.length;
  const backgroundRunning =
    (status.engine.background_running ?? backgroundTasks.length) +
    (gitLane?.active_count ?? gitLaneActive.length);
  const backgroundLimit =
    (status.engine.background_limit ?? 1) + (gitLane?.limit ?? 0);
  const backgroundWaiting =
    (engineLane?.waiting_count ?? engineLaneWaiting.length) +
    (gitLane?.waiting_count ?? gitLaneWaiting.length);
  const hasBackgroundWork =
    backgroundTasks.length > 0 ||
    engineLaneWorkCount > 0 ||
    (status.engine.entity_refresh_pending ?? 0) > 0 ||
    (status.engine.entity_health_pending ?? 0) > 0 ||
    Boolean(status.engine.entity_rebuild_pending) ||
    cachePersistenceActive ||
    recentHealthTransitions.length > 0;

  return (
    <section id="admin-current-run-panel" className="mb-10">
      <div className="mb-4 flex items-center gap-2">
        <Clock className="h-4 w-4 text-muted-foreground" />
        <span className="eyebrow">
          {runState === "finalizing"
            ? "Run finalizing"
            : running
              ? "Run in progress"
              : "Schedule"}
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

      <div className="grid items-stretch gap-px overflow-hidden rounded-sm border border-border bg-border md:grid-cols-2 xl:grid-cols-4">
        <QueueColumn
          title="Waiting To Be Downloaded"
          items={downloadWaiting}
          feedIndex={feedIndex}
          onFeedClick={onFeedClick}
          itemLabel="item"
          emptyText="No item is waiting for a download worker."
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
        />
        <ProcessingNowColumn
          running={running}
          currentPhase={status.engine.current_phase}
          currentBatch={status.engine.current_batch}
          phasePlan={status.engine.phase_plan}
          activeOperations={status.engine.active_operations ?? []}
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
          </div>
          <div className="text-right text-xs text-muted-foreground tabular-nums">
            <div>
              lane:{" "}
              <span className="text-foreground">
                {backgroundRunning}/{backgroundLimit}
              </span>
            </div>
            <div>
              waiting:{" "}
              <span className="text-foreground">{backgroundWaiting}</span>
            </div>
          </div>
        </div>
        <div className={LIVE_QUEUE_VIEWPORT_CLASS}>
          {!hasBackgroundWork ? (
            <div className={LIVE_QUEUE_EMPTY_CLASS}>
              No background maintenance task is currently running.
            </div>
          ) : (
            <>
              {engineLaneActive.map((work) => (
                <EngineLaneWorkItem
                  key={work.id}
                  work={work}
                  label="active"
                  nowMs={nowMs}
                />
              ))}
              {engineLaneWaiting.map((work) => (
                <EngineLaneWorkItem
                  key={work.id}
                  work={work}
                  label="waiting"
                  nowMs={nowMs}
                />
              ))}
              {gitLaneActive.map((work) => (
                <EngineLaneWorkItem
                  key={work.id}
                  work={work}
                  label="active"
                  nowMs={nowMs}
                />
              ))}
              {gitLaneWaiting.map((work) => (
                <EngineLaneWorkItem
                  key={work.id}
                  work={work}
                  label="waiting"
                  nowMs={nowMs}
                />
              ))}
              {status.engine.entity_rebuild_pending &&
                backgroundTasks.every((task) => !isEntityRebuildTask(task)) && (
                  <div className="border-b border-border/60 px-6 py-3 text-sm text-muted-foreground">
                    Entity artifacts rebuild: waiting for worker slot
                  </div>
                )}
              {status.engine.entity_refresh_pending != null &&
                status.engine.entity_refresh_pending > 0 &&
                !backgroundTasks.some(
                  (t) =>
                    t.trigger === "feed_update" &&
                    t.name === "Entity artifacts refresh",
                ) && (
                  <div className="border-b border-border/60 px-6 py-3 text-sm text-muted-foreground">
                    Entity refresh: {status.engine.entity_refresh_pending} feeds
                    coalescing
                  </div>
                )}
              {status.engine.entity_health_pending != null &&
                status.engine.entity_health_pending > 0 &&
                !backgroundTasks.some(
                  (t) =>
                    t.trigger === "health_transition" &&
                    t.name === "Entity artifacts refresh",
                ) && (
                  <div className="border-b border-border/60 px-6 py-3 text-sm text-muted-foreground">
                    Entity health refresh: {status.engine.entity_health_pending}{" "}
                    feeds coalescing
                  </div>
                )}
              {cachePersistenceActive && cachePersistence && (
                <div className="border-b border-border/60 px-6 py-3 text-sm text-muted-foreground">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <span className="font-semibold text-foreground">
                      Cache persistence
                    </span>
                    <span>{cachePersistence.state}</span>
                  </div>
                  <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs">
                    {cachePersistence.last_started && (
                      <span>
                        started{" "}
                        {relativeTime(parseGoTime(cachePersistence.last_started))}
                      </span>
                    )}
                    {cachePersistence.last_saved && (
                      <span>
                        saved{" "}
                        {relativeTime(parseGoTime(cachePersistence.last_saved))}
                      </span>
                    )}
                    {cachePersistence.last_error && (
                      <span className="text-destructive">
                        {cachePersistence.last_error}
                      </span>
                    )}
                  </div>
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
                      <span>
                        trigger: {formatBackgroundTrigger(task.trigger)}
                      </span>
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

function EngineLaneWorkItem({
  work,
  label,
  nowMs,
}: {
  work: AdminEngineLaneWork;
  label: "active" | "waiting";
  nowMs: number;
}) {
  const startedAt = parseGoTime(work.started_at);
  const queuedAt = parseGoTime(work.queued_at);
  const elapsedMs =
    label === "active" && startedAt > 0 && nowMs > 0
      ? Math.max(0, nowMs - startedAt * 1000)
      : Math.max(0, work.elapsed_ms ?? work.wait_ms ?? 0);
  return (
    <div className="border-b border-border/60 px-6 py-3 last:border-b-0">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="min-w-0 text-sm font-semibold text-foreground">
          <span className="font-mono">{work.name || work.kind}</span>
        </div>
        <div className="text-xs text-muted-foreground tabular-nums">
          {label === "active"
            ? elapsedMs > 0
              ? formatDuration(elapsedMs)
              : "running"
            : queuedAt > 0
              ? relativeTime(queuedAt)
              : "waiting"}
        </div>
      </div>
      <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
        <span>{label}</span>
        <span>{formatEngineWorkKind(work.kind)}</span>
        {work.component && <span>{formatEngineWorkKind(work.component)}</span>}
        {work.trigger && <span>{formatBackgroundTrigger(work.trigger)}</span>}
        {work.stage && <span>{work.stage}</span>}
      </div>
      {work.detail && (
        <div className="mt-2 text-sm text-foreground/90">{work.detail}</div>
      )}
    </div>
  );
}

function formatEngineWorkKind(value: string): string {
  return value.replace(/_/g, " ");
}

function blockedQueueItems(
  items: AdminQueueItem[],
  detail: string,
): AdminQueueItem[] {
  return items.map((item) => ({
    ...item,
    blocked: true,
    detail: item.detail ? `${detail}; ${item.detail}` : detail,
  }));
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
            <span className="font-mono text-foreground">{t.feed}</span>:{" "}
            {t.from_class || "new"} → {t.to_class},{" "}
            {relativeTime(parseGoTime(t.at))}
          </li>
        ))}
      </ul>
    </div>
  );
}
