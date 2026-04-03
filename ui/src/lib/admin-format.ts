/**
 * Shared formatting helpers for the operator admin console.
 *
 * These are called from every admin panel — keep them pure, fast,
 * and free of component dependencies so they can be shared without
 * creating import cycles. Anything that needs React state goes in
 * the component that uses it, not here.
 */

import type { AdminFeed, AdminProblemClass } from "./api-types";
import { feedHealthColor, feedHealthLabel } from "./feed-health";

/* ------------------------------------------------------------------------ */
/* Time formatting                                                          */
/* ------------------------------------------------------------------------ */

/**
 * Render a Unix-seconds timestamp as a short human-relative delta,
 * always directional (past = "5m ago", future = "in 5m", zero = "—").
 * Used everywhere the admin shows "when did X happen" or "when
 * will Y happen". Longest result is "Nh ago" / "in Nh"; anything
 * over 24h gets calendar days.
 */
export function relativeTime(unixSeconds: number): string {
  if (!unixSeconds || unixSeconds <= 0) return "—";
  const deltaMs = unixSeconds * 1000 - Date.now();
  const abs = Math.abs(deltaMs);
  const past = deltaMs < 0;
  const mag = formatDuration(abs);
  return past ? `${mag} ago` : `in ${mag}`;
}

/**
 * Format an absolute Unix-seconds timestamp as "YYYY-MM-DD HH:MM"
 * in the viewer's local timezone. For row expansion and tooltips
 * where operators want to cross-reference log lines and precise
 * diagnostic data — relative time is not enough when correlating.
 */
export function absoluteTime(unixSeconds: number): string {
  if (!unixSeconds || unixSeconds <= 0) return "—";
  const d = new Date(unixSeconds * 1000);
  if (Number.isNaN(d.getTime())) return "—";
  const pad = (n: number) => String(n).padStart(2, "0");
  return (
    `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ` +
    `${pad(d.getHours())}:${pad(d.getMinutes())}`
  );
}

/**
 * Format a bare duration in milliseconds as a compact rounded
 * string: "42s", "3m", "1h", "2d". Precision steps down as
 * magnitude grows — sub-minute resolution for sub-minute deltas,
 * minute resolution up to an hour, hour resolution up to a day,
 * day resolution beyond. Kept deliberately short so it fits in
 * table cells.
 */
export function formatDuration(ms: number): string {
  if (ms < 1000) return "<1s";
  const seconds = Math.round(ms / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours}h`;
  const days = Math.round(hours / 24);
  return `${days}d`;
}

/**
 * Format a frequency in minutes as "every Nm" / "every Nh" /
 * "every Nd". Used in the Schedule column of the feeds table.
 */
export function formatFrequency(minutes: number): string {
  if (!minutes || minutes <= 0) return "—";
  if (minutes < 60) return `every ${minutes}m`;
  if (minutes < 60 * 24) {
    const h = Math.round(minutes / 60);
    return `every ${h}h`;
  }
  const d = Math.round(minutes / (60 * 24));
  return `every ${d}d`;
}

const INPUT_TRIGGERED_SCHEDULE_LABEL = "triggered by inputs";

/**
 * Retention windows and merges are derived feeds. They are rebuilt
 * when their inputs change, not by a fixed wall-clock cadence.
 */
export function usesInputTriggeredSchedule(
  feed: Pick<AdminFeed, "kind" | "frequency_minutes">,
): boolean {
  return (
    feed.frequency_minutes === 0 &&
    (feed.kind === "retention" || feed.kind === "merge")
  );
}

/**
 * Human-readable scheduler-state string for operators. Derived feeds
 * should explain their trigger model instead of leaking the scheduler's
 * internal far-future sentinel timestamp.
 */
export function scheduleDetail(
  feed: Pick<AdminFeed, "kind" | "frequency_minutes" | "scheduler_detail">,
): string | undefined {
  if (usesInputTriggeredSchedule(feed)) {
    return INPUT_TRIGGERED_SCHEDULE_LABEL;
  }
  return feed.scheduler_detail;
}

/**
 * Primary label shown in the "next check" slot of admin views.
 */
export function scheduleNextCheckLabel(
  feed: Pick<AdminFeed, "kind" | "frequency_minutes" | "next_check">,
): string {
  if (usesInputTriggeredSchedule(feed)) {
    return INPUT_TRIGGERED_SCHEDULE_LABEL;
  }
  return relativeTime(feed.next_check);
}

/**
 * Tooltip/secondary explanation for the "next check" slot.
 */
export function scheduleNextCheckTooltip(
  feed: Pick<
    AdminFeed,
    "kind" | "frequency_minutes" | "next_check" | "scheduler_detail"
  >,
): string {
  if (usesInputTriggeredSchedule(feed)) {
    return "Triggered by parent/input feeds; no fixed wall-clock schedule.";
  }
  const detail = scheduleDetail(feed);
  return `${absoluteTime(feed.next_check)}${detail ? ` — ${detail}` : ""}`;
}

/* ------------------------------------------------------------------------ */
/* Number formatting                                                        */
/* ------------------------------------------------------------------------ */

/**
 * Compact number formatting for admin tiles and rows. Uses SI
 * suffixes (K/M/B) for brevity so "5,120" becomes "5.1K" and
 * "1,234,567" becomes "1.2M". The operator always has the exact
 * number available in the row expansion / tooltip; here we
 * optimise for scanning.
 */
export function formatNumber(n: number | undefined | null): string {
  if (n === undefined || n === null || !Number.isFinite(n)) return "—";
  const abs = Math.abs(n);
  if (abs >= 1_000_000_000) return `${(n / 1_000_000_000).toFixed(1)}B`;
  if (abs >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (abs >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return String(Math.round(n));
}

/**
 * Format a raw byte count as "X.Y MiB" / "X.Y GiB". Used for
 * heap/sys tiles in the heartbeat strip.
 */
export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return "—";
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let value = bytes;
  let i = 0;
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024;
    i++;
  }
  return `${value.toFixed(value >= 100 ? 0 : 1)} ${units[i]}`;
}

/* ------------------------------------------------------------------------ */
/* Feed health / filter helpers                                             */
/* ------------------------------------------------------------------------ */

export type FeedHealthFilter =
  | "healthy"
  | "delayed"
  | "risky"
  | "archived"
  | "unavailable"
  | "empty"
  | "unmaintained";

export function feedHealth(
  feed: Pick<AdminFeed, "health">,
): "healthy" | "delayed" | "risky" | "archived" | "unavailable" | "empty" | "unmaintained" {
  return feed.health?.class ?? "healthy";
}

export { feedHealthColor as healthColor, feedHealthLabel as healthLabel };

export function lastStatusLabel(
  item: Pick<AdminFeed, "last_status" | "last_status_label">,
): string {
  return item.last_status_label || item.last_status || "—";
}

export function problemClassLabel(
  problemClass?: AdminProblemClass,
): string | null {
  switch (problemClass) {
    case "downloader":
      return "Downloader-stage failure";
    case "processing":
      return "Processing-stage exception";
    default:
      return null;
  }
}

export function problemClassTone(
  problemClass?: AdminProblemClass,
): string {
  switch (problemClass) {
    case "downloader":
      return "text-status-warning";
    case "processing":
      return "text-destructive";
    default:
      return "text-muted-foreground";
  }
}

/** Short human-readable label for a feed kind. Covers the seven
 *  kinds the backend's adminKindForSource emits. */
export function kindLabel(kind: string): string {
  switch (kind) {
    case "source":
      return "src";
    case "merge":
      return "merge";
    case "retention":
      return "ret";
    case "asn":
      return "asn";
    case "geolocation":
      return "geo";
    case "bogon":
      return "bogon";
    default:
      return kind;
  }
}
