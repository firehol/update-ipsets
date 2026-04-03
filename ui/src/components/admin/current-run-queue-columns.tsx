import type {
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
const LIVE_QUEUE_ITEM_STATIC_CLASS = "flex min-h-14 items-start gap-3 px-6 py-2";

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
  processingBatch,
  feedIndex,
  nowMs,
  onFeedClick,
}: {
  running: boolean;
  currentPhase: string | undefined;
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
  right,
  onClick,
}: {
  name: string;
  reason?: string;
  startedAt: number;
  detail?: string;
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
            <span className="text-status-warning text-[10px]" title="Waiting for parent download">⏳</span>
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
