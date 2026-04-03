import type { FeedHealthClass, FeedHealthSnapshot } from "./api-types";

export function feedHealthLabel(value: FeedHealthClass | undefined): string {
  switch (value) {
    case "unavailable":
      return "unavailable";
    case "archived":
      return "archived";
    case "empty":
      return "empty";
    case "delayed":
      return "delayed";
    case "risky":
      return "risky";
    case "unmaintained":
      return "unmaintained";
    case "healthy":
    default:
      return "healthy";
  }
}

export function feedHealthColor(value: FeedHealthClass | undefined): string {
  switch (value) {
    case "unavailable":
      return "text-destructive";
    case "archived":
      return "text-status-archived";
    case "empty":
      return "text-status-warning";
    case "delayed":
      return "text-status-delayed";
    case "risky":
      return "text-status-risky";
    case "unmaintained":
      return "text-destructive";
    case "healthy":
    default:
      return "text-status-healthy";
  }
}

export function feedHealthDotColor(value: FeedHealthClass | undefined): string {
  switch (value) {
    case "unavailable":
      return "hsl(var(--destructive))";
    case "archived":
      return "var(--status-archived)";
    case "empty":
      return "var(--status-warning)";
    case "delayed":
      return "var(--status-delayed)";
    case "risky":
      return "var(--status-risky)";
    case "unmaintained":
      return "hsl(var(--destructive))";
    case "healthy":
    default:
      return "var(--status-healthy)";
  }
}

export function feedHealthDescription(
  snapshot: FeedHealthSnapshot | undefined,
): string {
  if (!snapshot) return "No health data";
  switch (snapshot.class) {
    case "unavailable":
      return snapshot.threshold_mins && snapshot.time_since_failure_mins
        ? `Download has been failing for ${formatMinutes(snapshot.time_since_failure_mins)}; threshold ${formatMinutes(snapshot.threshold_mins)}`
        : "Download is currently unavailable";
    case "archived":
      return snapshot.archival_threshold_mins &&
        snapshot.time_since_failure_mins &&
        snapshot.threshold_mins
        ? `Feed has remained unavailable for ${formatMinutes(snapshot.time_since_failure_mins - snapshot.threshold_mins)} beyond the unavailable threshold; archival threshold ${formatMinutes(snapshot.archival_threshold_mins)}`
        : "Feed has remained unavailable long enough to stop automatic retries";
    case "empty":
      return "Download works, but the current feed has zero entries";
    case "delayed":
      return snapshot.time_since_last_change_mins &&
        snapshot.effective_healthy_gap_mins &&
        snapshot.risky_cadence_mins
        ? `Last change ${formatMinutes(snapshot.time_since_last_change_mins)} ago; healthy cadence ${formatMinutes(snapshot.effective_healthy_gap_mins)}, risky at ${formatMinutes(snapshot.risky_cadence_mins)}`
        : "Feed is behind its healthy cadence";
    case "risky":
      return snapshot.time_since_last_change_mins &&
        snapshot.risky_cadence_mins &&
        snapshot.unmaintained_threshold_mins
        ? `Last change ${formatMinutes(snapshot.time_since_last_change_mins)} ago; risky after ${formatMinutes(snapshot.risky_cadence_mins)}, unmaintained at ${formatMinutes(snapshot.unmaintained_threshold_mins)}`
        : "Feed is older than its risky cadence";
    case "unmaintained":
      return snapshot.unmaintained_threshold_mins &&
        snapshot.time_since_last_change_mins
        ? `No observed content change for ${formatMinutes(snapshot.time_since_last_change_mins)}; unmaintained threshold ${formatMinutes(snapshot.unmaintained_threshold_mins)}`
        : "Feed is older than its observed maintenance cadence";
    case "healthy":
    default:
      if (snapshot.exclude_from_unmaintained) {
        return "Excluded from age-based maintenance classification";
      }
      if (
        snapshot.threshold_basis === "single_observation_grace" &&
        snapshot.single_observation_grace_mins &&
        snapshot.time_since_last_change_mins
      ) {
        return `Single-observation grace: last change ${formatMinutes(snapshot.time_since_last_change_mins)} ago; age-based health starts after ${formatMinutes(snapshot.single_observation_grace_mins)}`;
      }
      if (
        snapshot.time_since_last_change_mins &&
        snapshot.effective_healthy_gap_mins &&
        snapshot.risky_cadence_mins
      ) {
        return `Last change ${formatMinutes(snapshot.time_since_last_change_mins)} ago; healthy cadence ${formatMinutes(snapshot.effective_healthy_gap_mins)}, risky at ${formatMinutes(snapshot.risky_cadence_mins)}`;
      }
      return "Within expected maintenance cadence";
  }
}

export function thresholdBasisLabel(
  basis: FeedHealthSnapshot["threshold_basis"],
): string {
  switch (basis) {
    case "single_observation_grace":
      return "single-observation grace";
    case "category_cadence":
      return "category cadence";
    default:
      return "—";
  }
}

export function formatMinutes(minutes: number | undefined): string {
  if (!minutes || minutes <= 0) return "—";
  if (minutes < 60) return `${minutes}m`;
  if (minutes < 60 * 24) {
    const hours = Math.round(minutes / 60);
    return `${hours}h`;
  }
  const days = Math.round(minutes / (60 * 24));
  return `${days}d`;
}
