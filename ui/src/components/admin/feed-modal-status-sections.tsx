import type { AdminFeed } from "@/lib/api-types";
import { HoverTip } from "@/components/editorial/hover-tip";
import { useNow } from "@/lib/use-now";
import { cn } from "@/lib/utils";
import {
  absoluteTime,
  formatFrequency,
  formatNumber,
  relativeTime,
  scheduleDetail,
  scheduleNextCheckLabel,
  scheduleNextCheckTooltip,
  usesInputTriggeredSchedule,
} from "@/lib/admin-format";
import {
  formatMinutes,
  thresholdBasisLabel,
} from "@/lib/feed-health";
import { KV, ModalSection } from "@/components/admin/feed-modal-primitives";

export function FeedModalSchedule({ feed }: { feed: AdminFeed }) {
  const scheduled = feed.frequency_minutes;
  const actual = feed.avg_update_mins ?? 0;
  const delta = cadenceDelta(scheduled, actual);
  const nowMs = useNow();
  const inputTriggered = usesInputTriggeredSchedule(feed);
  const overdue =
    !inputTriggered &&
    nowMs > 0 &&
    feed.next_check > 0 &&
    feed.next_check * 1000 <
      nowMs - Math.max(60_000, (feed.frequency_minutes || 0) * 6_000);
  const scheduleState = scheduleDetail(feed);

  return (
    <ModalSection title="Schedule">
      <KV label="Scheduled frequency" value={formatFrequency(scheduled)} />
      <KV
        label="Actual avg interval"
        value={
          actual > 0 ? (
            <span className="tabular-nums">
              {formatFrequency(actual).replace("every ", "")}
              {delta && (
                <span className={cn("ml-2 text-xs", delta.color)}>
                  {delta.label}
                </span>
              )}
            </span>
          ) : (
            <span className="text-muted-foreground">no data yet</span>
          )
        }
        span2
      />
      {feed.min_update_mins && feed.min_update_mins > 0 ? (
        <KV
          label="Observed min / max"
          value={
            <span className="tabular-nums">
              min {formatFrequency(feed.min_update_mins).replace("every ", "")}
              {" · "}
              max{" "}
              {formatFrequency(feed.max_update_mins ?? 0).replace("every ", "")}
            </span>
          }
        />
      ) : null}
      <KV
        label="Healthy cadence floor"
        value={
          feed.health.effective_healthy_gap_mins ? (
            <span className="tabular-nums">
              {formatMinutes(feed.health.effective_healthy_gap_mins)}
            </span>
          ) : (
            <span className="text-muted-foreground">—</span>
          )
        }
      />
      <KV
        label="Risky cadence"
        value={
          feed.health.risky_cadence_mins ? (
            <span className="tabular-nums">
              {formatMinutes(feed.health.risky_cadence_mins)}
            </span>
          ) : (
            <span className="text-muted-foreground">—</span>
          )
        }
      />
      <KV
        label="Abandoned at"
        value={
          feed.health.unmaintained_threshold_mins ? (
            <span className="tabular-nums">
              {formatMinutes(feed.health.unmaintained_threshold_mins)}
            </span>
          ) : (
            <span className="text-muted-foreground">—</span>
          )
        }
      />
      <KV
        label="Archived after"
        value={
          feed.health.archival_threshold_mins ? (
            <span className="tabular-nums">
              {formatMinutes(feed.health.archival_threshold_mins)}
            </span>
          ) : (
            <span className="text-muted-foreground">—</span>
          )
        }
      />
      <KV
        label="Single-update grace"
        value={
          feed.health.single_observation_grace_mins ? (
            <span className="tabular-nums">
              {formatMinutes(feed.health.single_observation_grace_mins)}
            </span>
          ) : (
            <span className="text-muted-foreground">—</span>
          )
        }
      />
      <KV
        label="Health basis"
        value={thresholdBasisLabel(feed.health.threshold_basis)}
      />
      <KV
        label="Observed updates"
        value={
          feed.health.observed_updates && feed.health.observed_updates > 0
            ? String(feed.health.observed_updates)
            : "—"
        }
      />
      <KV
        label="Time since last change"
        value={
          feed.health.time_since_last_change_mins ? (
            <span className="tabular-nums">
              {formatMinutes(feed.health.time_since_last_change_mins)}
            </span>
          ) : (
            "—"
          )
        }
      />
      <KV
        label="Next check"
        value={
          <HoverTip text={scheduleNextCheckTooltip(feed)}>
            <span
              className={cn(
                "tabular-nums",
                overdue ? "text-destructive" : "",
              )}
            >
              {scheduleNextCheckLabel(feed)}
              {!inputTriggered && (
                <span className="ml-2 text-xs text-muted-foreground">
                  {absoluteTime(feed.next_check)}
                </span>
              )}
            </span>
          </HoverTip>
        }
        span2
      />
      {scheduleState && (
        <KV
          label="Scheduler state"
          value={
            <span className="tabular-nums text-foreground">
              {scheduleState}
            </span>
          }
          span2
        />
      )}
      {feed.download_failures > 0 && (
        <KV
          label="Retry / backoff"
          value={
            <span className="text-destructive tabular-nums">
              {feed.download_failures} consecutive failures — scheduler applying
              linear backoff
            </span>
          }
          span2
        />
      )}
    </ModalSection>
  );
}

function cadenceDelta(scheduledMins: number, actualMins: number) {
  if (!scheduledMins || !actualMins) return null;
  const ratio = actualMins / scheduledMins;
  if (ratio < 0.5) {
    return {
      label: `${ratio.toFixed(1)}× schedule (feed changes faster than we check)`,
      color: "text-status-warning",
    };
  }
  if (ratio > 3) {
    return {
      label: `${ratio.toFixed(1)}× schedule (checking ${Math.round(ratio)}× too often)`,
      color: "text-status-warning",
    };
  }
  if (ratio >= 0.8 && ratio <= 1.5) {
    return { label: "in sync", color: "text-status-healthy" };
  }
  return {
    label: `${ratio.toFixed(1)}× schedule`,
    color: "text-muted-foreground",
  };
}

export function FeedModalTimeline({ feed }: { feed: AdminFeed }) {
  return (
    <ModalSection title="Timeline">
      {feed.started_date > 0 && (
        <KV
          label="Tracking since"
          value={absoluteTime(feed.started_date)}
          span2
        />
      )}
      <TimelineRow
        label="Last checked"
        ts={feed.last_check}
        description="Last time we asked upstream (HTTP probe)"
      />
      <TimelineRow
        label="Last upstream change"
        ts={feed.last_update}
        description="Upstream's own Last-Modified header"
      />
      <TimelineRow
        label="Last processed"
        ts={feed.processed_date}
        description="Wall clock when finalize() last ran (our authoritative timestamp)"
      />
      <TimelineRow
        label="Next check"
        ts={feed.next_check}
        description="Scheduler's next attempt"
      />
      {feed.clock_skew_seconds && feed.clock_skew_seconds !== 0 ? (
        <KV
          label="Clock skew"
          value={
            <span className="text-status-warning tabular-nums">
              {feed.clock_skew_seconds > 0 ? "+" : ""}
              {feed.clock_skew_seconds}s (upstream clock vs ours)
            </span>
          }
          span2
        />
      ) : null}
    </ModalSection>
  );
}

function TimelineRow({
  label,
  ts,
  description,
}: {
  label: string;
  ts: number;
  description: string;
}) {
  return (
    <div className="col-span-2 grid grid-cols-[180px_1fr] items-baseline gap-3 text-xs">
      <div className="text-muted-foreground">{label}</div>
      <div>
        <span className="tabular-nums text-foreground">{relativeTime(ts)}</span>
        <span className="ml-2 tabular-nums text-muted-foreground">
          {absoluteTime(ts)}
        </span>
        <div className="mt-0.5 text-[11px] text-muted-foreground">
          {description}
        </div>
      </div>
    </div>
  );
}

export function FeedModalContent({ feed }: { feed: AdminFeed }) {
  return (
    <ModalSection title="Content">
      <KV
        label="Unique IPs"
        value={
          <span className="tabular-nums">{formatNumber(feed.unique_ips)}</span>
        }
      />
      <KV
        label="Entries"
        value={
          <span className="tabular-nums">{formatNumber(feed.entries)}</span>
        }
      />
      {(feed.ips_min ?? 0) > 0 && (
        <KV
          label="IPs range"
          value={
            <span className="tabular-nums">
              {formatNumber(feed.ips_min)} – {formatNumber(feed.ips_max)}
            </span>
          }
        />
      )}
      {(feed.entries_min ?? 0) > 0 && (
        <KV
          label="Entries range"
          value={
            <span className="tabular-nums">
              {formatNumber(feed.entries_min)} –{" "}
              {formatNumber(feed.entries_max)}
            </span>
          }
        />
      )}
      {(feed.version ?? 0) > 0 && (
        <KV label="Version" value={`v${feed.version}`} />
      )}
    </ModalSection>
  );
}
