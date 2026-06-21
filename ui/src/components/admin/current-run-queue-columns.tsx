import type {
  AdminActiveOperation,
  AdminActiveQueueItem,
  AdminFeed,
  AdminQueueItem,
  AdminRunBatch,
  AdminRunPhasePlan,
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
  LIVE_QUEUE_TILE_CLASS,
  LIVE_QUEUE_TILE_VIEWPORT_CLASS,
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
}: {
  title: string;
  items: AdminQueueItem[];
  feedIndex: Map<string, AdminFeed>;
  onFeedClick: (feed: AdminFeed) => void;
  emptyText: string;
  itemLabel: "feed" | "item";
}) {
  return (
    <div className={LIVE_QUEUE_TILE_CLASS}>
      <div className="flex items-baseline justify-between border-b border-border/60 px-6 py-3">
        <div className="eyebrow">{title}</div>
        <div className="text-[11px] tabular-nums text-muted-foreground">
          {items.length} {items.length === 1 ? itemLabel : `${itemLabel}s`}
        </div>
      </div>
      <div
        aria-label={`${title} queue`}
        className={LIVE_QUEUE_TILE_VIEWPORT_CLASS}
        role="region"
      >
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
    <div className={LIVE_QUEUE_TILE_CLASS}>
      <div className="flex items-baseline justify-between border-b border-border/60 px-6 py-3">
        <div className="eyebrow">Being Downloaded Now</div>
        <div className="text-[11px] tabular-nums text-muted-foreground">
          {items.length} {items.length === 1 ? "item" : "items"}
        </div>
      </div>
      <div
        aria-label="Being Downloaded Now queue"
        className={LIVE_QUEUE_TILE_VIEWPORT_CLASS}
        role="region"
      >
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
  currentBatch,
  phasePlan,
  activeOperations,
  processingBatch,
  feedIndex,
  nowMs,
  onFeedClick,
}: {
  running: boolean;
  currentPhase: string | undefined;
  activeOperations: AdminActiveOperation[];
  currentBatch?: AdminRunBatch;
  phasePlan?: AdminRunPhasePlan;
  processingBatch: NonNullable<AdminStatus["queues"]>["processing_active"];
  feedIndex: Map<string, AdminFeed>;
  nowMs: number;
  onFeedClick: (feed: AdminFeed) => void;
}) {
  const phaseOperations = activeOperations
    .filter(
      (operation) =>
        !operation.feed &&
        (!currentPhase || !operation.phase || operation.phase === currentPhase),
    )
    .sort(
      (left, right) =>
        parseGoTime(left.started_at) - parseGoTime(right.started_at),
    );

  return (
    <div className={LIVE_QUEUE_TILE_CLASS}>
      <div className="flex items-baseline justify-between border-b border-border/60 px-6 py-3">
        <div className="eyebrow">Being Processed Now</div>
        <div className="text-[11px] tabular-nums text-muted-foreground">
          {currentBatch?.total ?? processingBatch?.length ?? 0}{" "}
          {(currentBatch?.total ?? processingBatch?.length ?? 0) === 1
            ? "feed"
            : "feeds"}
        </div>
      </div>
      <div className="border-b border-border/40 px-6 py-3">
        {currentBatch && <RunBatchSummary batch={currentBatch} />}
        <HoverTip text={describeRunPhase(currentPhase)}>
          <div className="mt-3 flex items-center justify-between gap-3 text-xs">
            <span className="text-muted-foreground">Phase</span>
            <span className="font-medium text-foreground tabular-nums">
              {running
                ? phasePlanLabel(phasePlan, currentPhase)
                : "Idle"}
            </span>
          </div>
        </HoverTip>
        {phasePlan?.phases && phasePlan.phases.length > 0 && (
          <PhasePlanStrip phasePlan={phasePlan} currentPhase={currentPhase} />
        )}
        {phaseOperations.length > 0 && (
          <div className="mt-3 space-y-3">
            {phaseOperations.slice(0, 3).map((operation) => (
              <ActiveOperationProgress
                key={`${operation.operation}:${operation.stage ?? ""}`}
                operation={operation}
                nowMs={nowMs}
              />
            ))}
          </div>
        )}
      </div>
      <div
        aria-label="Being Processed Now queue"
        className={LIVE_QUEUE_TILE_VIEWPORT_CLASS}
        role="region"
      >
        {!processingBatch || processingBatch.length === 0 ? (
          running ? null : (
            <div className={LIVE_QUEUE_EMPTY_CLASS}>
              No batch is active right now.
            </div>
          )
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

function RunBatchSummary({
  batch,
}: {
  batch: AdminRunBatch;
}) {
  const names = batch.names ?? [];
  const visibleNames = names.slice(0, 8);
  const hidden = Math.max(0, names.length - visibleNames.length);
  return (
    <div className="space-y-2">
      <HoverTip text={names.length > 0 ? names.join(", ") : "No feeds"}>
        <div className="flex items-center justify-between gap-3 text-xs">
          <span className="text-muted-foreground">Batch</span>
          <span className="font-medium text-foreground tabular-nums">
            {formatWorkCount(batch.completed)} done ·{" "}
            {formatWorkCount(batch.active)} active ·{" "}
            {formatWorkCount(batch.pending)} pending
          </span>
        </div>
      </HoverTip>
      {names.length > 0 && (
        <div className="truncate font-mono text-[10px] text-muted-foreground">
          {visibleNames.join(", ")}
          {hidden > 0 && ` +${hidden} more`}
        </div>
      )}
      <div className="flex flex-wrap gap-2 text-[10px] tabular-nums text-muted-foreground">
        <span>
          sources {formatWorkCount(batch.source_completed ?? 0)}/
          {formatWorkCount(batch.source_total ?? 0)}
        </span>
        {(batch.history_total ?? 0) > 0 && (
          <span>
            retention {formatWorkCount(batch.history_completed ?? 0)}/
            {formatWorkCount(batch.history_total ?? 0)}
          </span>
        )}
        {(batch.merge_total ?? 0) > 0 && (
          <span>
            merges {formatWorkCount(batch.merge_completed ?? 0)}/
            {formatWorkCount(batch.merge_total ?? 0)}
          </span>
        )}
      </div>
    </div>
  );
}

function PhasePlanStrip({
  phasePlan,
  currentPhase,
}: {
  phasePlan: AdminRunPhasePlan;
  currentPhase: string | undefined;
}) {
  const phases = phasePlan.phases ?? [];
  const currentIndex = phases.findIndex((phase) => phase === currentPhase);
  return (
    <div className="mt-2 flex flex-wrap gap-1">
      {phases.map((phase, index) => {
        const active = phase === currentPhase;
        const done = currentIndex >= 0 && index < currentIndex;
        return (
          <HoverTip key={phase} text={describeRunPhase(phase)}>
            <span
              className={[
                "rounded-sm border px-1.5 py-0.5 text-[9px] uppercase tracking-wider",
                active
                  ? "border-primary bg-primary text-primary-foreground"
                  : done
                    ? "border-status-healthy/40 text-status-healthy"
                    : "border-border text-muted-foreground",
              ].join(" ")}
            >
              {formatRunPhase(phase)}
            </span>
          </HoverTip>
        );
      })}
      {!phasePlan.final && (
        <span className="rounded-sm border border-border px-1.5 py-0.5 text-[9px] uppercase tracking-wider text-muted-foreground">
          planning
        </span>
      )}
    </div>
  );
}

function phasePlanLabel(
  phasePlan: AdminRunPhasePlan | undefined,
  currentPhase: string | undefined,
): string {
  if (!phasePlan || !phasePlan.total || !phasePlan.current_position) {
    return formatRunPhase(currentPhase);
  }
  return `${phasePlan.current_position}/${phasePlan.total} · ${formatRunPhase(
    currentPhase,
  )}`;
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
  const hasTotal = total > 0;
  const percent =
    operation.completion_pct ??
    (hasTotal
      ? Math.min(100, Math.max(0, Math.round((current / total) * 100)))
      : 0);
  const elapsedMs = operationElapsedMs(operation, nowMs);
  const unit = operation.unit || "items";
  const size = hasTotal
    ? `${formatWorkCount(current)} / ${formatWorkCount(total)} ${unit}`
    : `${formatWorkCount(current)} ${unit}`;
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
    case "sources.parse_feed_body":
      return "Parsing source body";
    case "sources.resolve_hostnames":
      return "Resolving hostnames";
    case "sources.diff_previous_latest":
      return "Diffing previous latest";
    case "sources.finalize":
      return "Finalizing feed";
    case "sources.update_retention":
      return "Updating retention";
    case "sources.refresh_rotation":
      return "Refreshing rotation stats";
    case "retention.reconcile_cohorts":
      return "Scanning retention cohorts";
    case "geoip.load_providers":
      return "Loading GeoIP providers";
    case "geoip.write_comparisons":
      return "Writing GeoIP comparisons";
    case "bogons.load_providers":
      return "Loading bogon providers";
    case "bogons.write_comparisons":
      return "Writing bogon comparisons";
    case "bogons.build_union":
      return "Building bogon union";
    case "asn.load_providers":
      return "Loading ASN providers";
    case "asn.precompute_bogon_splits":
      return "Precomputing ASN bogon splits";
    case "asn.write_comparisons":
      return "Writing ASN comparisons";
    case "critical.load_providers":
      return "Loading critical providers";
    case "critical.write_comparisons":
      return "Writing critical comparisons";
    case "entities.stage_feed_sidecars":
      return "Building entity feed sidecars";
    case "metadata.prepare_comparison_sets":
      return "Preparing comparison sets";
    case "metadata.scan_comparison_pairs":
      return "Scanning comparison pairs";
    case "metadata.compare_pairs":
      return "Comparing candidate pairs";
    case "metadata.write_comparison_rows":
      return "Writing comparison rows";
    case "metadata.update_unique_shares":
      return "Updating unique-share metrics";
    case "metadata.write_home_aggregates":
      return "Writing homepage aggregates";
    case "metadata.write_public_metadata":
      return "Writing public metadata";
    case "metadata.write_per_feed_outputs":
      return "Writing feed metadata";
    case "metadata.write_indexes":
      return "Writing metadata indexes";
    case "metadata.write_git_artifacts":
      return "Writing Git artifacts";
    case "metadata.write_markdown":
      return "Writing markdown pages";
    case "insights.write_feeds":
      return "Writing insights";
    case "publish.apply_timestamps":
      return "Applying artifact timestamps";
    case "publish.promote_web_artifacts":
      return "Publishing web artifacts";
    case "publish.promote_entity_artifacts":
      return "Publishing entity artifacts";
    case "publish.copy_raw_ipsets":
      return "Copying raw ipsets";
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
              className="rounded-sm border border-status-warning/40 px-1 py-0.5 text-[9px] uppercase tracking-wider text-status-warning"
              title="Blocked waiting queue item"
            >
              blocked
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
