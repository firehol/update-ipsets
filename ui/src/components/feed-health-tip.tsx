import { Link } from "react-router-dom";
import type { FeedHealthSnapshot } from "@/lib/api-types";
import {
  feedHealthDescription,
  feedHealthLabel,
  formatMinutes,
} from "@/lib/feed-health";

export function FeedHealthTip({
  health,
  compact,
}: {
  health: FeedHealthSnapshot | undefined;
  compact?: boolean;
}) {
  if (!health) return null;
  return (
    <div className="flex w-[300px] flex-col gap-2">
      <div className="font-medium text-popover-foreground">
        {feedHealthLabel(health.class)}
      </div>
      <div className="text-[12px] leading-relaxed text-popover-foreground/85">
        {feedHealthDescription(health)}
      </div>
      {!compact && (
        <div className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-[10px]">
          <span className="uppercase tracking-[0.08em] text-popover-foreground/55">
            Healthy
          </span>
          <span className="text-popover-foreground">
            {formatMinutes(health.effective_healthy_gap_mins)}
          </span>
          <span className="uppercase tracking-[0.08em] text-popover-foreground/55">
            Risky
          </span>
          <span className="text-popover-foreground">
            {formatMinutes(health.risky_cadence_mins)}
          </span>
          <span className="uppercase tracking-[0.08em] text-popover-foreground/55">
            Abandoned
          </span>
          <span className="text-popover-foreground">
            {formatMinutes(health.unmaintained_threshold_mins)}
          </span>
          <span className="uppercase tracking-[0.08em] text-popover-foreground/55">
            Archived
          </span>
          <span className="text-popover-foreground">
            {formatMinutes(health.archival_threshold_mins)}
          </span>
        </div>
      )}
      <div className="pt-1 text-[11px] text-popover-foreground/75">
        <Link
          className="underline underline-offset-4"
          to="/methodology/feed-health"
        >
          Methodology
        </Link>
      </div>
    </div>
  );
}
