import { Fragment, useState } from "react";
import {
  AlertTriangle,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Play,
  RefreshCw,
  ShieldCheck,
} from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import type { IntegrityFinding } from "@/lib/api-types";
import {
  adminIntegrityRefresh,
  adminIntegrityReprocess,
} from "@/lib/api-client/admin";
import { queryKeys } from "@/lib/query-keys";
import { adminIntegrityOptions } from "@/lib/queries/admin";
import { Button } from "@/components/ui/button";
import { absoluteTime } from "@/lib/admin-format";

/**
 * Integrity panel — surfaces feeds whose pipeline ran but whose
 * public outputs no longer match the last successful local
 * publication.
 *
 * The backend's CheckIntegrity() uses cache.Entry.ProcessedDate
 * as its reference (NOT the on-disk source file mtime, which
 * can be in the future because it's set from upstream
 * Last-Modified headers). See pkg/engine/integrity.go for the
 * rationale.
 *
 * Three visual states:
 *
 *   1. Loading — quiet placeholder
 *   2. Clean — collapsed to a single-line status strip so it
 *      stops burning vertical space when there is nothing to
 *      report
 *   3. Dirty — red-accented findings table with the exact
 *      recovery class and targets for each finding plus a
 *      global recovery CTA
 *
 * Rows are click-to-expand to reveal the full missing / stale /
 * malformed evidence plus the planned recovery targets.
 *
 * Refresh is manual (no polling). Integrity runs walk every
 * secondary file in webDir — expensive.
 *
 * One exception: while the backend reports queued/running work,
 * the panel polls briefly until the active run settles. Without
 * that, the UI can get stuck showing the waiting message from
 * an old response forever.
 */
export function IntegrityPanel() {
  const queryClient = useQueryClient();
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [includeArchived, setIncludeArchived] = useState(false);

  const integrityQuery = useQuery({
    ...adminIntegrityOptions(includeArchived),
    retry: false,
    refetchInterval: (query) =>
      query.state.data?.running ||
      query.state.data?.queued ||
      query.state.data?.startup_scan_running
        ? 5000
        : false,
  });

  const refreshIntegrity = useMutation({
    mutationFn: () => adminIntegrityRefresh({ includeArchived }),
    onSuccess: (result) => {
      if (result.status === "in_progress") {
        toast.info("Integrity check is already queued or running");
      } else {
        toast.success("Queued integrity re-check");
      }
      queryClient.invalidateQueries({
        queryKey: queryKeys.adminIntegrity(includeArchived),
      });
      queryClient.invalidateQueries({ queryKey: queryKeys.adminStatus() });
    },
    onError: (e: Error) =>
      toast.error(`Integrity re-check failed: ${e.message}`),
  });

  const recoverAll = useMutation({
    mutationFn: () => adminIntegrityReprocess({ includeArchived }),
    onSuccess: (result) => {
      if (result.status === "clean") {
        toast.success("Integrity clean — nothing to recover");
      } else if (result.status === "in_progress") {
        toast.info("Integrity check is waiting for the active run to finish");
      } else {
        toast.success(
          `Scheduled integrity recovery for ${result.count} item(s)`,
        );
      }
      queryClient.invalidateQueries({
        queryKey: queryKeys.adminIntegrityRoot(),
      });
      queryClient.invalidateQueries({
        queryKey: queryKeys.adminEntityIntegrity(),
      });
      queryClient.invalidateQueries({ queryKey: queryKeys.adminStatus() });
    },
    onError: (e: Error) => toast.error(`Recovery failed: ${e.message}`),
  });

  const findings = integrityQuery.data?.findings ?? [];
  const count = integrityQuery.data?.count ?? 0;
  const status = integrityQuery.data?.status ?? "clean";
  const integrityWorkActive = Boolean(
    integrityQuery.data?.running ||
    integrityQuery.data?.queued ||
    integrityQuery.data?.startup_scan_running,
  );

  return (
    <section id="admin-integrity-panel" className="mb-12">
      <div className="mb-5 flex items-center gap-2">
        <ShieldCheck className="h-4 w-4 text-muted-foreground" />
        <span className="eyebrow">Pipeline integrity</span>
        <label className="ml-4 inline-flex items-center gap-2 text-[12px] text-muted-foreground">
          <input
            type="checkbox"
            checked={includeArchived}
            onChange={(e) => {
              setExpanded({});
              setIncludeArchived(e.target.checked);
            }}
            className="h-3.5 w-3.5 border-border"
          />
          include archived feeds
        </label>
        <div className="ml-auto flex items-center gap-2">
          <button
            type="button"
            onClick={() => refreshIntegrity.mutate()}
            disabled={refreshIntegrity.isPending || integrityWorkActive}
            className="inline-flex items-center gap-2 border-b border-border pb-1 text-[13px] text-muted-foreground transition-colors hover:border-foreground hover:text-foreground disabled:opacity-50"
          >
            <RefreshCw
              className={`h-3.5 w-3.5 ${refreshIntegrity.isPending || integrityWorkActive ? "animate-spin" : ""}`}
            />
            Re-check
          </button>
          {count > 0 && (
            <button
              type="button"
              onClick={() => recoverAll.mutate()}
              disabled={recoverAll.isPending || integrityQuery.isFetching}
              className="inline-flex items-center gap-2 bg-primary px-5 py-2.5 text-[13px] font-semibold uppercase tracking-[0.08em] text-primary-foreground transition-opacity hover:opacity-90 disabled:opacity-50"
            >
              {recoverAll.isPending ? (
                <RefreshCw className="h-3.5 w-3.5 animate-spin" />
              ) : (
                <Play className="h-3.5 w-3.5" />
              )}
              Recover all {count}
            </button>
          )}
        </div>
      </div>

      {integrityQuery.isLoading && (
        <div className="h-16 animate-pulse bg-muted/40" />
      )}

      {!integrityQuery.isLoading && integrityQuery.error && (
        <p className="border border-border bg-card px-6 py-5 text-sm text-destructive">
          Integrity check failed: {(integrityQuery.error as Error).message}
        </p>
      )}

      {!integrityQuery.isLoading && !integrityQuery.error && count === 0 && (
        <div className="flex items-center gap-3 border border-border bg-card px-6 py-3 text-sm">
          {status === "in_progress" ? (
            <>
              <RefreshCw
                className={`h-4 w-4 text-muted-foreground ${integrityWorkActive ? "animate-spin" : ""}`}
              />
              <span className="text-muted-foreground">
                {integrityWorkActive
                  ? "Integrity check is queued or running."
                  : "Integrity cache is stale; current findings are hidden until a fresh check settles."}
              </span>
            </>
          ) : (
            <>
              <CheckCircle2 className="h-4 w-4 text-status-healthy" />
              <span className="text-muted-foreground">
                All feeds have up-to-date and readable secondary files.
              </span>
            </>
          )}
        </div>
      )}

      {!integrityQuery.isLoading && !integrityQuery.error && count > 0 && (
        <IntegrityFindingsTable
          findings={findings}
          expanded={expanded}
          onToggle={(feed) =>
            setExpanded((prev) => ({ ...prev, [feed]: !prev[feed] }))
          }
        />
      )}
    </section>
  );
}

/* -------------------------------------------------------------------------- */

function IntegrityFindingsTable({
  findings,
  expanded,
  onToggle,
}: {
  findings: IntegrityFinding[];
  expanded: Record<string, boolean>;
  onToggle: (feed: string) => void;
}) {
  return (
    <div className="rounded-md border border-destructive/40 bg-destructive/[0.03]">
      <div className="flex items-center gap-3 border-b border-destructive/30 bg-destructive/[0.05] px-6 py-3">
        <AlertTriangle className="h-4 w-4 text-destructive" />
        <span className="text-sm text-foreground">
          <strong className="font-semibold">{findings.length}</strong> feed
          {findings.length === 1 ? "" : "s"} with broken pipeline runs — local
          outputs or required local inputs are inconsistent with the last
          successful finalize.
        </span>
      </div>
      <div className="max-h-[30rem] overflow-auto">
        <table className="w-full border-collapse text-[14px]">
          <thead className="sticky top-0 z-10 bg-card">
            <tr className="border-b border-border">
              <th className="eyebrow w-8 py-3 pl-5">
                <span className="sr-only">Details</span>
              </th>
              <th className="eyebrow py-3 px-3 text-left">Feed</th>
              <th className="eyebrow py-3 px-3 text-left">Reason</th>
              <th className="eyebrow py-3 px-3 text-right">Missing</th>
              <th className="eyebrow py-3 px-3 text-right">Stale</th>
              <th className="eyebrow py-3 px-3 text-right">Malformed</th>
              <th className="eyebrow py-3 px-3 text-left">Processed at</th>
              <th className="eyebrow py-3 pr-5 text-left">Recovery</th>
            </tr>
          </thead>
          <tbody>
            {findings.map((finding) => {
              const isOpen = Boolean(expanded[finding.feed]);
              const missingCount = finding.missing_files?.length ?? 0;
              const staleCount = finding.stale_files?.length ?? 0;
              const malformedCount = finding.malformed_files?.length ?? 0;
              const processedTs = parseGoTime(finding.processed_at);
              const detailId = integrityFindingDetailId(finding.feed);
              return (
                <Fragment key={finding.feed}>
                  <tr className="border-b border-border/60 transition-colors hover:bg-muted/40">
                    <td className="py-3 pl-5">
                      <button
                        type="button"
                        onClick={() => onToggle(finding.feed)}
                        aria-expanded={isOpen}
                        aria-controls={detailId}
                        aria-label={`${isOpen ? "Collapse" : "Expand"} ${finding.feed} integrity finding`}
                        className="inline-flex h-7 w-7 items-center justify-center text-muted-foreground transition hover:text-foreground focus:outline-none focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary"
                      >
                        {isOpen ? (
                          <ChevronDown className="h-4 w-4" />
                        ) : (
                          <ChevronRight className="h-4 w-4" />
                        )}
                      </button>
                    </td>
                    <td className="py-3 px-3 font-mono text-sm">
                      {finding.feed}
                    </td>
                    <td className="py-3 px-3 text-sm text-muted-foreground">
                      {finding.reason}
                    </td>
                    <td className="py-3 px-3 text-right tabular-nums">
                      {renderFindingCount(missingCount)}
                    </td>
                    <td className="py-3 px-3 text-right tabular-nums">
                      {renderFindingCount(staleCount)}
                    </td>
                    <td className="py-3 px-3 text-right tabular-nums">
                      {renderFindingCount(malformedCount)}
                    </td>
                    <td className="py-3 px-3 text-sm text-muted-foreground">
                      {absoluteTime(processedTs)}
                    </td>
                    <td className="py-3 pr-5 text-sm">
                      <RecoverySummary finding={finding} />
                    </td>
                  </tr>
                  {isOpen && (
                    <tr
                      id={detailId}
                      className="border-b border-border/60 bg-muted/20"
                    >
                      <td colSpan={8} className="px-5 py-5">
                        <IntegrityFindingDetail finding={finding} />
                      </td>
                    </tr>
                  )}
                </Fragment>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function integrityFindingDetailId(feed: string): string {
  return `integrity-finding-${feed.replace(/[^a-zA-Z0-9_-]/g, "-")}`;
}

function IntegrityFindingDetail({ finding }: { finding: IntegrityFinding }) {
  const missing = finding.missing_files ?? [];
  const stale = finding.stale_files ?? [];
  const malformed = finding.malformed_files ?? [];
  const blocked = finding.blocked_feeds ?? [];
  const recoveryTargets = finding.recovery_targets ?? [];

  return (
    <div className="grid gap-6 md:grid-cols-2">
      <div className="md:col-span-2">
        <div className="eyebrow mb-1">Source output</div>
        <div className="font-mono text-xs text-muted-foreground break-all">
          {finding.source_path || "(no source output file)"}
        </div>
        <div className="mt-2 grid gap-2 text-xs text-muted-foreground md:grid-cols-2">
          <div>
            <span className="eyebrow mr-2">Processed at</span>
            {absoluteTime(parseGoTime(finding.processed_at))}
          </div>
          <div>
            <span className="eyebrow mr-2">Source file mtime</span>
            {absoluteTime(parseGoTime(finding.source_file_mtime))}
          </div>
        </div>
      </div>
      {(finding.recovery_action || recoveryTargets.length > 0) && (
        <div className="md:col-span-2">
          <div className="eyebrow mb-2">Recovery</div>
          <div className="text-sm">
            <RecoverySummary finding={finding} />
          </div>
          {recoveryTargets.length > 0 && (
            <ul className="mt-2 space-y-1">
              {recoveryTargets.map((target) => (
                <li key={target} className="font-mono text-xs break-all">
                  {target}
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
      {missing.length > 0 && (
        <FindingFileList
          title={`Missing (${missing.length})`}
          items={missing}
        />
      )}
      {stale.length > 0 && (
        <FindingFileList title={`Stale (${stale.length})`} items={stale} />
      )}
      {malformed.length > 0 && (
        <FindingFileList
          title={`Malformed (${malformed.length})`}
          items={malformed}
        />
      )}
      {blocked.length > 0 && (
        <FindingFileList
          title={`Blocked feeds (${blocked.length})`}
          items={blocked}
        />
      )}
    </div>
  );
}

function FindingFileList({ title, items }: { title: string; items: string[] }) {
  return (
    <div>
      <div className="eyebrow mb-2 text-destructive">{title}</div>
      <ul className="space-y-1">
        {items.map((item) => (
          <li key={item} className="font-mono text-xs break-all">
            {item}
          </li>
        ))}
      </ul>
    </div>
  );
}

function RecoverySummary({ finding }: { finding: IntegrityFinding }) {
  if (finding.recovery_action === "recheck") {
    return (
      <span className="inline-flex items-center gap-2">
        <Button variant="outline" size="sm" className="pointer-events-none h-7">
          Recheck
        </Button>
        <span className="text-muted-foreground">
          {formatRecoveryTargets(finding.recovery_targets)}
        </span>
      </span>
    );
  }
  if (finding.recovery_action === "reprocess") {
    return (
      <span className="inline-flex items-center gap-2">
        <Button variant="outline" size="sm" className="pointer-events-none h-7">
          Reprocess
        </Button>
        <span className="text-muted-foreground">
          {formatRecoveryTargets(finding.recovery_targets)}
        </span>
      </span>
    );
  }
  return <span className="text-muted-foreground">No recovery plan</span>;
}

function formatRecoveryTargets(targets: string[] | undefined): string {
  if (!targets || targets.length === 0) {
    return "no targets";
  }
  if (targets.length === 1) {
    return targets[0];
  }
  return `${targets.length} targets`;
}

function renderFindingCount(count: number) {
  if (count <= 0) {
    return <span className="text-muted-foreground">—</span>;
  }
  return <span className="text-destructive">{count}</span>;
}

/**
 * Convert a Go-encoded RFC3339 time string to Unix seconds so
 * the admin-format helpers can work with it.
 */
function parseGoTime(s: string): number {
  if (!s || s.startsWith("0001-01-01")) return 0;
  const d = new Date(s);
  if (Number.isNaN(d.getTime())) return 0;
  return Math.floor(d.getTime() / 1000);
}
