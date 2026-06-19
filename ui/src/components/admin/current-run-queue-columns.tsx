import type {
  AdminActiveOperation,
  AdminActiveQueueItem,
  AdminFeed,
  AdminQueueItem,
  AdminStatus,
} from "@/lib/api-types";
import {
  formatDuration,
  problemClassLabel,
  relativeTime,
} from "@/lib/admin-format";
import { describeRunPhase, formatRunPhase } from "@/lib/admin-run-phase";
import { describeRunReason, formatRunReason } from "@/lib/admin-run-reason";
import { HoverTip } from "@/components/editorial/hover-tip";
import {
  LIVE_QUEUE_EMPTY_CLASS,
  LIVE_QUEUE_VIEWPORT_CLASS,
  parseGoTime,
} from "@/components/admin/current-run-shared";

const LIVE_QUEUE_ITEM_BUTTON_CLASS =
  "flex min-h-14 w-full items-start gap-3 px-6 py-2 text-left transition-colors hover:bg-muted/40";
const LIVE_QUEUE_ITEM_STATIC_CLASS =
  "flex min-h-14 items-start gap-3 px-6 py-2";

export function QueueColumn({
  title,
  items,
  feedIndex,
  onFeedClick,
  emptyText,
  itemLabel,
  pendingCount,
  pendingItems,
}: {
  title: string;
  items: AdminQueueItem[];
  feedIndex: Map<string, AdminFeed>;
  onFeedClick: (feed: AdminFeed) => void;
  emptyText: string;
  itemLabel: "feed" | "item";
  pendingCount?: number;
  pendingItems?: AdminQueueItem[];
}) {
  return (
    <div className="bg-card">
      <div className="flex items-baseline justify-between border-b border-border/60 px-6 py-3">
        <div className="eyebrow">{title}</div>
        <div className="text-[11px] tabular-nums text-muted-foreground">
          {items.length} {items.length === 1 ? itemLabel : `${itemLabel}s`}
          {pendingCount != null && pendingCount > 0 && (
            <HoverTip
              text={
                pendingItems?.map((i) => i.name).join(", ") ??
                `${pendingCount} pending`
              }
            >
              <span className="ml-1 text-status-warning">
                {" "}
                · +{pendingCount} pending
              </span>
            </HoverTip>
          )}
        </div>
      </div>
      <div className={LIVE_QUEUE_VIEWPORT_CLASS}>
        {items.length === 0 ? (
          <div className={LIVE_QUEUE_EMPTY_CLASS}>{emptyText}</div>
        ) : (
          <ul className="divide-y divide-border/40">
            {items.map((item) => {
              const feed = feedIndex.get(item.name);
              return (
                <QueueFeedItem
                  key={item.name}
                  name={item.name}
                  sublabel={queueSublabel(item)}
                  right={formatQueueAge(item.queued_at)}
                  blocked={item.blocked}
                  blockedParents={item.blocked_parents}
                  onClick={feed ? () => onFeedClick(feed) : undefined}
                />
              );
            })}
          </ul>
        )}
      </div>
    </div>
  );
}

export function ActiveDownloadColumn({
  items,
  feedIndex,
  nowMs,
  onFeedClick,
}: {
  items: AdminActiveQueueItem[];
  feedIndex: Map<string, AdminFeed>;
  nowMs: number;
  onFeedClick: (feed: AdminFeed) => void;
}) {
  return (
    <div className="bg-card">
      <div className="flex items-baseline justify-between border-b border-border/60 px-6 py-3">
        <div className="eyebrow">Being Downloaded Now</div>
        <div className="text-[11px] tabular-nums text-muted-foreground">
          {items.length} {items.length === 1 ? "item" : "items"}
        </div>
      </div>
      <div className={LIVE_QUEUE_VIEWPORT_CLASS}>
        {items.length === 0 ? (
          <div className={LIVE_QUEUE_EMPTY_CLASS}>
            No download worker is busy right now.
          </div>
        ) : (
          <ul className="divide-y divide-border/40">
            {items.map((item) => {
              const feed = feedIndex.get(item.name);
              const startedAt = parseGoTime(item.started_at);
              const elapsedMs =
                startedAt > 0 && nowMs > 0
                  ? Math.max(0, nowMs - startedAt * 1000)
                  : 0;
              return (
                <QueueFeedItem
                  key={item.name}
                  name={item.name}
                  sublabel={queueSublabel(
                    item,
                    startedAt > 0
                      ? `started ${relativeTime(startedAt)}`
                      : undefined,
                  )}
                  right={
                    elapsedMs > 0 ? formatDuration(elapsedMs) : "downloading"
                  }
                  onClick={feed ? () => onFeedClick(feed) : undefined}
                />
              );
            })}
          </ul>
        )}
      </div>
    </div>
  );
}

export function ProcessingNowColumn({
  running,
  currentPhase,
  activeOperations,
  processingBatch,
  feedIndex,
  nowMs,
  onFeedClick,
}: {
  running: boolean;
  currentPhase: string | undefined;
  activeOperations: AdminActiveOperation[];
  processingBatch: NonNullable<AdminStatus["queues"]>["processing_active"];
  feedIndex: Map<string, AdminFeed>;
  nowMs: number;
  onFeedClick: (feed: AdminFeed) => void;
}) {
  return (
    <div className="bg-card">
      <div className="flex items-baseline justify-between border-b border-border/60 px-6 py-3">
        <div className="eyebrow">Being Processed Now</div>
        <div className="text-[11px] tabular-nums text-muted-foreground">
          {processingBatch?.length ?? 0}{" "}
          {(processingBatch?.length ?? 0) === 1 ? "feed" : "feeds"}
        </div>
      </div>
      <div className="border-b border-border/40 px-6 py-3">
        <HoverTip text={describeRunPhase(currentPhase)}>
          <div className="flex items-center justify-between gap-3 text-xs">
            <span className="text-muted-foreground">Phase</span>
            <span className="font-medium text-foreground">
              {running ? formatRunPhase(currentPhase) : "Idle"}
            </span>
          </div>
        </HoverTip>
      </div>
      <div className={LIVE_QUEUE_VIEWPORT_CLASS}>
        {!processingBatch || processingBatch.length === 0 ? (
          <div className={LIVE_QUEUE_EMPTY_CLASS}>
            {running
              ? emptyProcessingText(currentPhase)
              : "No batch is active right now."}
          </div>
        ) : (
          <ul className="divide-y divide-border/40">
            {processingBatch.map((batchFeed) => {
              const feed = feedIndex.get(batchFeed.name);
              const operation = bestActiveOperationForFeed(
                batchFeed.name,
                activeOperations,
              );
              const startedAt = parseGoTime(batchFeed.started_at);
              const elapsedMs =
                startedAt > 0 && nowMs > 0
                  ? Math.max(0, nowMs - startedAt * 1000)
                  : 0;
              return (
                <ProcessingFeedItem
                  key={batchFeed.name}
                  name={batchFeed.name}
                  reason={batchFeed.reason}
                  startedAt={startedAt}
                  detail={batchFeed.detail}
                  operation={operation}
                  nowMs={nowMs}
                  right={elapsedMs > 0 ? formatDuration(elapsedMs) : "running"}
                  onClick={feed ? () => onFeedClick(feed) : undefined}
                />
              );
            })}
          </ul>
        )}
      </div>
    </div>
  );
}

function ProcessingFeedItem({
  name,
  reason,
  startedAt,
  detail,
  operation,
  nowMs,
  right,
  onClick,
}: {
  name: string;
  reason?: string;
  startedAt: number;
  detail?: string;
  operation?: AdminActiveOperation;
  nowMs: number;
  right: string;
  onClick?: () => void;
}) {
  const content = (
    <>
      <div className="min-w-0 flex-1">
        <div className="font-mono text-[12px] text-foreground truncate">
          {name}
        </div>
        {startedAt > 0 && (
          <div className="mt-0.5 truncate text-[10px] tabular-nums text-muted-foreground">
            started {relativeTime(startedAt)}
          </div>
        )}
        {detail && detail !== "running" && detail !== "processing" && (
          <HoverTip text={detail}>
            <div className="mt-0.5 truncate text-[10px] text-muted-foreground">
              {detail}
            </div>
          </HoverTip>
        )}
        {operation && (
          <ActiveOperationProgress operation={operation} nowMs={nowMs} />
        )}
      </div>
      <div className="shrink-0 text-right">
        <HoverTip text={describeRunReason(reason)}>
          <div className="text-[11px] text-foreground">
            {formatRunReason(reason)}
          </div>
        </HoverTip>
        <div className="text-[10px] tabular-nums text-muted-foreground">
          {right}
        </div>
      </div>
    </>
  );

  return (
    <li>
      {onClick ? (
        <button
          type="button"
          onClick={onClick}
          className={LIVE_QUEUE_ITEM_BUTTON_CLASS}
        >
          {content}
        </button>
      ) : (
        <div className={LIVE_QUEUE_ITEM_STATIC_CLASS}>{content}</div>
      )}
    </li>
  );
}

function ActiveOperationProgress({
  operation,
  nowMs,
}: {
  operation: AdminActiveOperation;
  nowMs: number;
}) {
  const progress = operationProgress(operation, nowMs);
  return (
    <div className="mt-2 space-y-1">
      <div className="flex items-center justify-between gap-2 text-[10px] text-muted-foreground">
        <span className="min-w-0 truncate">{operationLabel(operation)}</span>
        <span className="shrink-0 tabular-nums text-foreground">
          {progress.label}
        </span>
      </div>
      {progress.hasTotal && (
        <div
          role="progressbar"
          aria-label={`${operationLabel(operation)} progress`}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={progress.percent}
          className="h-1.5 overflow-hidden rounded-full bg-muted"
        >
          <div
            className="h-full rounded-full bg-primary"
            style={{ width: `${progress.percent}%` }}
          />
        </div>
      )}
      <div className="flex items-center justify-between gap-2 text-[10px] tabular-nums text-muted-foreground">
        <span>{progress.size}</span>
        <span>{progress.rate}</span>
      </div>
    </div>
  );
}

function bestActiveOperationForFeed(
  name: string,
  operations: AdminActiveOperation[],
): AdminActiveOperation | undefined {
  const candidates = operations.filter((operation) => operation.feed === name);
  if (candidates.length === 0) return undefined;
  return candidates.sort(
    (left, right) => operationScore(right) - operationScore(left),
  )[0];
}

function operationScore(operation: AdminActiveOperation): number {
  let score = 0;
  if ((operation.total ?? 0) > 0) score += 100;
  if ((operation.current ?? 0) > 0) score += 10;
  if (operation.operation === "retention.reconcile_cohorts") score += 5;
  return score;
}

function operationProgress(operation: AdminActiveOperation, nowMs: number) {
  const current = Math.max(0, operation.current ?? 0);
  const total = Math.max(0, operation.total ?? 0);
  const hasTotal = true;
  const percent =
    operation.completion_pct ??
    (hasTotal
      ? Math.min(100, Math.max(0, Math.round((current / total) * 100)))
      : 0);
  const elapsedMs = operationElapsedMs(operation, nowMs);
  const unit = operation.unit || "items";
  const size = `${formatWorkCount(current)} / ${formatWorkCount(total)} ${unit}`;
  return {
    hasTotal,
    percent,
    label: hasTotal ? `${percent}%` : formatDuration(elapsedMs),
    size,
    rate: operationRate(operation.rate_per_second, current, elapsedMs, unit),
  };
}

function operationElapsedMs(
  operation: AdminActiveOperation,
  nowMs: number,
): number {
  const startedAt = parseGoTime(operation.started_at);
  if (startedAt > 0 && nowMs > 0) {
    return Math.max(0, nowMs - startedAt * 1000);
  }
  return Math.max(0, operation.elapsed_ms ?? 0);
}

function operationRate(
  backendRate: number | undefined,
  current: number,
  elapsedMs: number,
  unit: string,
): string {
  const perSecond =
    backendRate !== undefined && backendRate >= 0
      ? backendRate
      : elapsedMs > 0
        ? current / (elapsedMs / 1000)
        : 0;
  if (!Number.isFinite(perSecond) || perSecond <= 0) return "rate —";
  const formatted =
    perSecond < 10 ? perSecond.toFixed(1) : Math.round(perSecond).toString();
  return `${formatted} ${unit}/s`;
}

function operationLabel(operation: AdminActiveOperation): string {
  switch (operation.operation) {
    case "sources.process_feed":
      return "Processing feed";
    case "sources.update_retention":
      return "Updating retention";
    case "retention.reconcile_cohorts":
      return "Scanning retention cohorts";
    default:
      return operation.stage
        ? `${operation.operation} · ${operation.stage}`
        : operation.operation;
  }
}

function formatWorkCount(value: number): string {
  return new Intl.NumberFormat("en-US", { maximumFractionDigits: 0 }).format(
    value,
  );
}

function QueueFeedItem({
  name,
  sublabel,
  right,
  blocked,
  blockedParents,
  onClick,
}: {
  name: string;
  sublabel?: string;
  right: string;
  blocked?: boolean;
  blockedParents?: string[];
  onClick?: () => void;
}) {
  const content = (
    <>
      <div className="min-w-0 flex-1">
        <div className="font-mono text-[12px] text-foreground truncate flex items-center gap-1.5">
          {blocked && (
            <span
              className="text-status-warning text-[10px]"
              title="Waiting for parent download"
            >
              ⏳
            </span>
          )}
          {name}
        </div>
        {blocked && blockedParents && blockedParents.length > 0 && (
          <div className="mt-0.5 truncate text-[10px] text-status-warning tabular-nums">
            waiting for: {blockedParents.join(", ")}
          </div>
        )}
        {sublabel && (
          <div className="mt-0.5 truncate text-[10px] tabular-nums text-muted-foreground">
            {sublabel}
          </div>
        )}
      </div>
      <div className="shrink-0 text-right">
        <div className="text-[11px] text-foreground">{right}</div>
      </div>
    </>
  );

  return (
    <li>
      {onClick ? (
        <button
          type="button"
          onClick={onClick}
          className={LIVE_QUEUE_ITEM_BUTTON_CLASS}
        >
          {content}
        </button>
      ) : (
        <div className={LIVE_QUEUE_ITEM_STATIC_CLASS}>{content}</div>
      )}
    </li>
  );
}

function queueSublabel(
  item:
    | Pick<
        AdminQueueItem,
        "reason" | "status" | "status_label" | "problem_class" | "detail"
      >
    | Pick<
        AdminActiveQueueItem,
        "reason" | "status" | "status_label" | "problem_class" | "detail"
      >,
  prefix?: string,
): string | undefined {
  const parts = [prefix];
  if (item.reason) {
    parts.push(formatRunReason(item.reason));
  }
  const problemLabel = problemClassLabel(item.problem_class);
  if (problemLabel) {
    parts.push(problemLabel);
  }
  if (
    item.detail &&
    item.detail !== item.status &&
    item.detail !== item.status_label &&
    item.detail !== "downloading" &&
    item.detail !== "processing"
  ) {
    parts.push(item.detail);
  } else if (
    item.status_label &&
    item.status_label !== "Downloader working" &&
    item.status_label !== "Processing local input"
  ) {
    parts.push(item.status_label);
  }
  const out = parts.filter(Boolean).join(" · ");
  return out || undefined;
}

function formatQueueAge(queuedAt: string | undefined): string {
  const ts = parseGoTime(queuedAt);
  if (!ts) return "—";
  return relativeTime(ts);
}

function emptyProcessingText(currentPhase: string | undefined): string {
  switch (currentPhase) {
    case "geoip":
      return "GeoIP database work is running in this phase.";
    case "asn":
      return "ASN database work is running in this phase.";
    default:
      return "This phase is running background work without per-feed queue entries.";
  }
}
