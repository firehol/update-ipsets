import { useState } from "react";
import { CheckCircle2, Play, RefreshCw, Wrench } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import type { EntityIntegrityFinding } from "@/lib/api-types";
import { adminRebuildEntityArtifacts } from "@/lib/api-client/admin";
import { queryKeys } from "@/lib/query-keys";
import { adminEntityIntegrityOptions } from "@/lib/queries/admin";
import { Button } from "@/components/ui/button";
import { absoluteTime } from "@/lib/admin-format";

export function EntityIntegrityPanel() {
  const queryClient = useQueryClient();
  const [confirmRebuild, setConfirmRebuild] = useState(false);
  const query = useQuery({
    ...adminEntityIntegrityOptions(),
    retry: false,
    refetchInterval: (state) =>
      state.state.data?.status === "in_progress" ? 5000 : false,
  });

  const rebuildAll = useMutation({
    mutationFn: adminRebuildEntityArtifacts,
    onSuccess: (result) => {
      if (result.status === "in_progress") {
        toast.info("Entity rebuild is already queued or running");
      } else {
        toast.success("Queued full country and ASN rebuild");
      }
      setConfirmRebuild(false);
      queryClient.invalidateQueries({ queryKey: queryKeys.adminStatus() });
      queryClient.invalidateQueries({ queryKey: queryKeys.adminEntityIntegrity() });
    },
    onError: (e: Error) => {
      toast.error(`Entity rebuild failed: ${e.message}`);
      setConfirmRebuild(false);
    },
  });

  const findings = query.data?.findings ?? [];
  const count = query.data?.count ?? 0;
  const status = query.data?.status ?? "clean";

  return (
    <section id="admin-entity-integrity-panel" className="mb-12">
      <div className="mb-5 flex items-center gap-2">
        <Wrench className="h-4 w-4 text-muted-foreground" />
        <span className="eyebrow">Entity Artifact Integrity</span>
        <div className="ml-auto flex items-center gap-2">
          <button
            type="button"
            onClick={() => query.refetch()}
            disabled={query.isFetching}
            className="inline-flex items-center gap-2 border-b border-border pb-1 text-[13px] text-muted-foreground transition-colors hover:border-foreground hover:text-foreground disabled:opacity-50"
          >
            <RefreshCw
              className={`h-3.5 w-3.5 ${query.isFetching ? "animate-spin" : ""}`}
            />
            Re-check
          </button>
          {confirmRebuild ? (
            <div className="inline-flex items-center gap-2">
              <Button
                type="button"
                onClick={() => rebuildAll.mutate()}
                disabled={rebuildAll.isPending || status === "in_progress"}
                className="inline-flex items-center gap-2"
              >
                {rebuildAll.isPending ? (
                  <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Play className="h-3.5 w-3.5" />
                )}
                Confirm Full Rebuild
              </Button>
              <button
                type="button"
                onClick={() => setConfirmRebuild(false)}
                className="text-xs text-muted-foreground hover:text-foreground"
              >
                cancel
              </button>
            </div>
          ) : (
            <Button
              type="button"
              onClick={() => setConfirmRebuild(true)}
              disabled={rebuildAll.isPending || status === "in_progress"}
              className="inline-flex items-center gap-2"
            >
              <Play className="h-3.5 w-3.5" />
              Rebuild All
            </Button>
          )}
        </div>
      </div>

      {query.isLoading && <div className="h-16 animate-pulse bg-muted/40" />}

      {!query.isLoading && query.error && (
        <p className="border border-border bg-card px-6 py-5 text-sm text-destructive">
          Entity artifact integrity check failed: {(query.error as Error).message}
        </p>
      )}

      {!query.isLoading && !query.error && count === 0 && (
        <div className="flex items-center gap-3 border border-border bg-card px-6 py-3 text-sm">
          {status === "in_progress" ? (
            <>
              <RefreshCw className="h-4 w-4 animate-spin text-muted-foreground" />
              <span className="text-muted-foreground">
                Entity artifact repair is running in the background.
              </span>
            </>
          ) : (
            <>
              <CheckCircle2 className="h-4 w-4 text-status-healthy" />
              <span className="text-muted-foreground">
                Country and ASN artifacts are current and readable.
              </span>
            </>
          )}
        </div>
      )}

      {!query.isLoading && !query.error && count > 0 && (
        <EntityIntegrityFindingsTable findings={findings} />
      )}
    </section>
  );
}

function EntityIntegrityFindingsTable({
  findings,
}: {
  findings: EntityIntegrityFinding[];
}) {
  const groups = groupEntityFindings(findings);
  return (
    <div className="rounded-md border border-border bg-card">
      <div className="border-b border-border px-6 py-3 text-sm text-muted-foreground">
        <strong className="font-semibold text-foreground">{findings.length}</strong>{" "}
        entity artifact finding{findings.length === 1 ? "" : "s"} across feed
        sidecars, country/ASN pages, indexes, or global files.
      </div>
      <div className="max-h-[34rem] divide-y divide-border/70 overflow-y-auto">
        {groups.map((group) => (
          <div key={group.scope}>
            <div className="sticky top-0 z-20 flex items-center justify-between border-b border-border/70 bg-card px-6 py-3">
              <div>
                <div className="eyebrow">{group.label}</div>
                <div className="mt-1 text-xs text-muted-foreground">
                  {group.findings.length} finding
                  {group.findings.length === 1 ? "" : "s"}
                </div>
              </div>
            </div>
            <div className="overflow-x-auto">
              <table className="w-full border-collapse text-[14px]">
                <thead>
                  <tr className="border-y border-border/70 bg-muted/20">
                    <th className="eyebrow px-4 py-3 text-left">Subject</th>
                    <th className="eyebrow px-4 py-3 text-left">Reason</th>
                    <th className="eyebrow px-4 py-3 text-left">Repair</th>
                    <th className="eyebrow px-4 py-3 text-left">Current</th>
                    <th className="eyebrow px-4 py-3 text-left">Reference</th>
                  </tr>
                </thead>
                <tbody>
                  {group.findings.map((finding, index) => (
                    <EntityIntegrityFindingRow
                      key={`${finding.kind}-${finding.subject ?? index}`}
                      finding={finding}
                    />
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function EntityIntegrityFindingRow({
  finding,
}: {
  finding: EntityIntegrityFinding;
}) {
  return (
    <tr className="border-b border-border/70 last:border-b-0">
      <td className="px-4 py-3 align-top text-sm text-foreground">
        {finding.subject || "global"}
      </td>
      <td className="px-4 py-3 align-top text-sm text-muted-foreground">
        <div>{finding.reason}</div>
        {(finding.affected_countries || finding.affected_asns) && (
          <div className="mt-1 text-xs">
            {finding.affected_countries
              ? `${finding.affected_countries} countries`
              : "0 countries"}
            {" · "}
            {finding.affected_asns ? `${finding.affected_asns} ASNs` : "0 ASNs"}
          </div>
        )}
      </td>
      <td className="px-4 py-3 align-top text-sm text-foreground">
        {formatRepairAction(finding.repair_action)}
      </td>
      <td className="px-4 py-3 align-top text-xs text-muted-foreground">
        {finding.path ? (
          <>
            <div className="break-all">{finding.path}</div>
            {finding.path_mtime && (
              <div className="mt-1">
                {absoluteTime(parseGoTime(finding.path_mtime))}
              </div>
            )}
          </>
        ) : (
          "—"
        )}
      </td>
      <td className="px-4 py-3 align-top text-xs text-muted-foreground">
        {finding.reference_path || finding.reference_mtime ? (
          <>
            {finding.reference_path && (
              <div className="break-all">{finding.reference_path}</div>
            )}
            {finding.reference_mtime && (
              <div className="mt-1">
                {absoluteTime(parseGoTime(finding.reference_mtime))}
              </div>
            )}
          </>
        ) : (
          "—"
        )}
      </td>
    </tr>
  );
}

function groupEntityFindings(findings: EntityIntegrityFinding[]) {
  const order = ["feed", "country", "asn", "index", "global"];
  const groups = new Map<string, EntityIntegrityFinding[]>();
  for (const finding of findings) {
    const scope = finding.scope || "global";
    groups.set(scope, [...(groups.get(scope) ?? []), finding]);
  }
  return [...groups.entries()]
    .sort(([left], [right]) => {
      const leftRank = order.indexOf(left);
      const rightRank = order.indexOf(right);
      return (
        (leftRank === -1 ? order.length : leftRank) -
          (rightRank === -1 ? order.length : rightRank) ||
        left.localeCompare(right)
      );
    })
    .map(([scope, groupFindings]) => ({
      scope,
      label: formatEntityScope({ scope } as EntityIntegrityFinding),
      findings: groupFindings,
    }));
}

function formatEntityScope(finding: EntityIntegrityFinding) {
  switch (finding.scope) {
    case "feed":
      return "Feed sidecar";
    case "country":
      return "Country page";
    case "asn":
      return "ASN page";
    case "index":
      return "Index";
    case "global":
      return "Global";
    default:
      return finding.scope;
  }
}

function formatRepairAction(action?: string) {
  switch (action) {
    case "full_rebuild":
      return "Full rebuild";
    case "refresh_feed":
      return "Refresh feed";
    case "refresh_entity":
      return "Refresh entity";
    case "refresh_index":
      return "Refresh index";
    case "refresh_health":
      return "Rewrite health";
    default:
      return "—";
  }
}

function parseGoTime(value?: string) {
  if (!value) {
    return 0;
  }
  const ms = Date.parse(value);
  return Number.isFinite(ms) ? Math.floor(ms / 1000) : 0;
}
