import { AlertCircle, ShieldCheck } from "lucide-react";
import type { AdminFeed, IntegrityFinding } from "@/lib/api-types";
import { HoverTip } from "@/components/editorial/hover-tip";
import {
  feedHealth,
  formatDuration,
  healthColor,
  healthLabel,
  lastStatusLabel,
  problemClassLabel,
  problemClassTone,
} from "@/lib/admin-format";
import { describeRunReason, formatRunReason } from "@/lib/admin-run-reason";
import { feedHealthDescription } from "@/lib/feed-health";
import { KV, ModalSection } from "@/components/admin/feed-modal-primitives";

export function FeedModalDiagnostics({
  feed,
  integrityFinding,
}: {
  feed: AdminFeed;
  integrityFinding: IntegrityFinding | undefined;
}) {
  const statusLabel = lastStatusLabel(feed);
  const problemLabel = integrityFinding
    ? "Integrity recovery needed"
    : problemClassLabel(feed.last_problem_class);

  return (
    <ModalSection title="Diagnostics">
      <KV
        label="Current status"
        value={
          <span className={healthColor(feedHealth(feed))}>
            {healthLabel(feedHealth(feed))}
          </span>
        }
      />
      <KV
        label="Health detail"
        value={feedHealthDescription(feed.health)}
        span2
      />
      <KV
        label="Last reason"
        value={
          <HoverTip text={describeRunReason(feed.last_run_reason)}>
            <span>{formatRunReason(feed.last_run_reason)}</span>
          </HoverTip>
        }
      />
      <KV
        label="Last processing time"
        value={
          feed.last_processing_ms && feed.last_processing_ms > 0 ? (
            <span className="tabular-nums">
              {formatDuration(feed.last_processing_ms)}
            </span>
          ) : (
            "—"
          )
        }
      />
      <KV label="Last status" value={statusLabel} />
      {problemLabel && (
        <KV
          label="Problem class"
          value={
            <span
              className={
                integrityFinding
                  ? "text-destructive"
                  : problemClassTone(feed.last_problem_class)
              }
            >
              {problemLabel}
            </span>
          }
        />
      )}
      <KV
        label="Download failures"
        value={
          feed.download_failures > 0 ? (
            <span className="text-destructive tabular-nums">
              {feed.download_failures}
            </span>
          ) : (
            <span className="tabular-nums">0</span>
          )
        }
      />
      {feed.last_error && (
        <div className="col-span-2">
          <div className="text-xs text-muted-foreground">Last error</div>
          <pre className="mt-1 max-h-40 select-text overflow-auto whitespace-pre-wrap break-all rounded-sm border border-destructive/30 bg-destructive/[0.03] p-3 font-mono text-[11px] text-destructive">
            {feed.last_error}
          </pre>
        </div>
      )}
      {integrityFinding ? (
        <IntegrityFindingDetails finding={integrityFinding} />
      ) : (
        <div className="col-span-2 flex items-center gap-2 text-xs text-status-healthy">
          <ShieldCheck className="h-4 w-4" />
          No integrity findings — secondary files are up to date.
        </div>
      )}
    </ModalSection>
  );
}

function IntegrityFindingDetails({ finding }: { finding: IntegrityFinding }) {
  return (
    <div className="col-span-2">
      <div className="mb-2 flex items-center gap-2">
        <AlertCircle className="h-4 w-4 text-destructive" />
        <span className="eyebrow text-destructive">Integrity issues</span>
      </div>
      <div className="rounded-sm border border-destructive/30 bg-destructive/[0.03] p-3 text-xs">
        <div className="mb-2 text-destructive">{finding.reason}</div>
        {finding.recovery_action && (
          <div className="mb-2">
            <div className="text-[10px] uppercase tracking-wider text-muted-foreground">
              Recovery
            </div>
            <div className="mt-1 text-foreground">
              {finding.recovery_action === "recheck" ? "Recheck" : "Reprocess"}
              {finding.recovery_targets &&
                finding.recovery_targets.length > 0 &&
                `: ${finding.recovery_targets.join(", ")}`}
            </div>
          </div>
        )}
        <FindingFileList label="Missing" files={finding.missing_files} />
        <FindingFileList label="Malformed" files={finding.malformed_files} />
        <FindingFileList label="Stale" files={finding.stale_files} />
        {finding.blocked_feeds && finding.blocked_feeds.length > 0 && (
          <div>
            <div className="text-[10px] uppercase tracking-wider text-muted-foreground">
              Blocked feeds ({finding.blocked_feeds.length})
            </div>
            <ul className="mt-1 space-y-0.5">
              {finding.blocked_feeds.map((f) => (
                <li key={f} className="break-all font-mono text-[11px]">
                  {f}
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>
    </div>
  );
}

function FindingFileList({
  label,
  files,
}: {
  label: string;
  files: string[] | undefined;
}) {
  if (!files || files.length === 0) return null;
  return (
    <div className="mb-2">
      <div className="text-[10px] uppercase tracking-wider text-muted-foreground">
        {label} ({files.length})
      </div>
      <ul className="mt-1 space-y-0.5">
        {files.map((f) => (
          <li key={f} className="break-all font-mono text-[11px]">
            {f}
          </li>
        ))}
      </ul>
    </div>
  );
}
