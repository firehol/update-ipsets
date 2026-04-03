import { useMutation, useQueryClient } from "@tanstack/react-query";
import { RefreshCw } from "lucide-react";
import { toast } from "sonner";
import type { AdminFeed, AdminStatus } from "@/lib/api-types";
import {
  adminDisableArtifact,
  adminEnableArtifact,
  adminRecheckArtifact,
} from "@/lib/api-client/admin";
import { queryKeys } from "@/lib/query-keys";
import {
  absoluteTime,
  lastStatusLabel,
  problemClassLabel,
  problemClassTone,
  relativeTime,
} from "@/lib/admin-format";

type ArtifactAction = "recheck" | "enable" | "disable";

export function ArtifactsPanel({
  status,
  feeds,
  onFeedClick,
}: {
  status: AdminStatus | undefined;
  feeds: AdminFeed[];
  onFeedClick: (feed: AdminFeed) => void;
}) {
  const queryClient = useQueryClient();
  const feedIndex = new Map(feeds.map((feed) => [feed.name, feed]));
  const artifacts = status?.artifacts ?? [];

  const action = useMutation({
    mutationFn: async ({
      name,
      action,
    }: {
      name: string;
      action: ArtifactAction;
    }) => {
      switch (action) {
        case "recheck":
          return adminRecheckArtifact(name);
        case "enable":
          return adminEnableArtifact(name);
        case "disable":
          return adminDisableArtifact(name);
      }
    },
    onSuccess: (_, vars) => {
      toast.success(`Artifact ${vars.action} scheduled for ${vars.name}`);
      queryClient.invalidateQueries({ queryKey: queryKeys.adminStatus() });
      queryClient.invalidateQueries({ queryKey: queryKeys.adminFeeds() });
    },
    onError: (error: Error) => {
      toast.error(error.message);
    },
  });

  if (artifacts.length === 0) {
    return null;
  }

  return (
    <section className="mb-10 overflow-hidden rounded-sm border border-border bg-card">
      <div className="flex items-center justify-between border-b border-border/60 px-6 py-4">
        <div>
          <div className="eyebrow">Artifacts</div>
          <div className="mt-1 text-sm text-muted-foreground">
            Shared download parents managed separately from feeds.
          </div>
        </div>
        <div className="text-xs tabular-nums text-muted-foreground">
          {artifacts.length} {artifacts.length === 1 ? "artifact" : "artifacts"}
        </div>
      </div>

      <div className="overflow-x-auto">
        <table className="w-full border-collapse text-sm">
          <thead>
            <tr className="border-b border-border/60 text-left text-[11px] uppercase tracking-wider text-muted-foreground">
              <th className="px-6 py-3">Artifact</th>
              <th className="px-4 py-3">Status</th>
              <th className="px-4 py-3">Last check</th>
              <th className="px-4 py-3">Next check</th>
              <th className="px-4 py-3">Children</th>
              <th className="px-6 py-3 text-right">Actions</th>
            </tr>
          </thead>
          <tbody>
            {artifacts.map((artifact) => {
              const busy =
                action.isPending && action.variables?.name === artifact.name;
              return (
                <tr
                  key={artifact.name}
                  className="border-b border-border/40 align-top"
                >
                  <td className="px-6 py-4">
                    <div className="font-medium text-foreground">
                      {artifact.name}
                    </div>
                    <div className="mt-1 text-xs text-muted-foreground">
                      {artifact.type}
                    </div>
                    {artifact.info && (
                      <div className="mt-2 max-w-xl text-xs text-muted-foreground">
                        {artifact.info}
                      </div>
                    )}
                  </td>
                  <td className="px-4 py-4">
                    <div className={artifactStatusClass(artifact.status)}>
                      {artifactCurrentStatusLabel(artifact.status)}
                    </div>
                    <div className="mt-1 text-xs text-muted-foreground">
                      {lastStatusLabel(artifact)}
                    </div>
                    {problemClassLabel(artifact.last_problem_class) && (
                      <div
                        className={[
                          "mt-1 text-[10px] font-medium uppercase tracking-[0.08em]",
                          problemClassTone(artifact.last_problem_class),
                        ].join(" ")}
                      >
                        {problemClassLabel(artifact.last_problem_class)}
                      </div>
                    )}
                    {artifact.last_error && (
                      <div className="mt-2 max-w-xs text-xs text-destructive">
                        {artifact.last_error}
                      </div>
                    )}
                    {artifact.download_failures > 0 && (
                      <div className="mt-1 text-xs text-muted-foreground">
                        failures: {artifact.download_failures}
                      </div>
                    )}
                  </td>
                  <td className="px-4 py-4 text-xs text-muted-foreground">
                    {artifact.last_check > 0 ? (
                      <>
                        <div>{relativeTime(artifact.last_check)}</div>
                        <div className="mt-1">{absoluteTime(artifact.last_check)}</div>
                      </>
                    ) : (
                      "never"
                    )}
                  </td>
                  <td className="px-4 py-4 text-xs text-muted-foreground">
                    {artifact.enabled ? (
                      <>
                        <div>
                          {artifact.next_check > 0
                            ? relativeTime(artifact.next_check)
                            : "due now"}
                        </div>
                        {artifact.scheduler_detail && (
                          <div className="mt-1">{artifact.scheduler_detail}</div>
                        )}
                      </>
                    ) : (
                      "disabled"
                    )}
                  </td>
                  <td className="px-4 py-4">
                    <div className="flex max-w-lg flex-wrap gap-2">
                      {(artifact.child_feeds ?? []).map((child) => {
                        const feed = feedIndex.get(child);
                        return (
                          <button
                            key={child}
                            type="button"
                            className="rounded-sm border border-border px-2 py-1 text-xs text-muted-foreground transition-colors hover:border-foreground hover:text-foreground"
                            onClick={() => {
                              if (feed) onFeedClick(feed);
                            }}
                            disabled={!feed}
                          >
                            {child}
                          </button>
                        );
                      })}
                    </div>
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex justify-end gap-2">
                      <ArtifactActionButton
                        label="Recheck"
                        busy={busy && action.variables?.action === "recheck"}
                        disabled={!artifact.enabled}
                        onClick={() =>
                          action.mutate({ name: artifact.name, action: "recheck" })
                        }
                      />
                      {artifact.enabled ? (
                        <ArtifactActionButton
                          label="Disable"
                          busy={busy && action.variables?.action === "disable"}
                          tone="danger"
                          onClick={() =>
                            action.mutate({
                              name: artifact.name,
                              action: "disable",
                            })
                          }
                        />
                      ) : (
                        <ArtifactActionButton
                          label="Enable"
                          busy={busy && action.variables?.action === "enable"}
                          onClick={() =>
                            action.mutate({
                              name: artifact.name,
                              action: "enable",
                            })
                          }
                        />
                      )}
                    </div>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function ArtifactActionButton({
  label,
  busy,
  disabled,
  onClick,
  tone = "default",
}: {
  label: string;
  busy: boolean;
  disabled?: boolean;
  onClick: () => void;
  tone?: "default" | "danger";
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled || busy}
      className={[
        "inline-flex items-center gap-1.5 rounded-sm border px-3 py-1.5 text-xs font-medium transition-colors disabled:opacity-40",
        tone === "danger"
          ? "border-destructive/30 text-destructive hover:border-destructive"
          : "border-border text-foreground hover:border-foreground",
      ].join(" ")}
    >
      {busy && <RefreshCw className="h-3 w-3 animate-spin" />}
      {label}
    </button>
  );
}

function artifactStatusClass(status: string) {
  switch (status) {
    case "error":
      return "text-sm font-medium text-destructive";
    case "downloading":
      return "text-sm font-medium text-status-info";
    case "queued":
      return "text-sm font-medium text-status-warning";
    case "disabled":
      return "text-sm font-medium text-muted-foreground";
    case "stale":
      return "text-sm font-medium text-status-warning";
    default:
      return "text-sm font-medium text-status-healthy";
  }
}

function artifactCurrentStatusLabel(status: string): string {
  switch (status) {
    case "healthy":
      return "Healthy";
    case "queued":
      return "Waiting to download";
    case "downloading":
      return "Downloading now";
    case "stale":
      return "Schedule overdue";
    case "disabled":
      return "Disabled";
    case "unavailable":
      return "Never completed";
    case "error":
      return "Needs attention";
    default:
      return status;
  }
}
