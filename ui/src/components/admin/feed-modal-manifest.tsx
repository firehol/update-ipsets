import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  CheckCircle2,
  FileText,
  RefreshCw,
  ShieldAlert,
  X,
} from "lucide-react";
import type { AdminFeed, ManifestFile } from "@/lib/api-types";
import { HoverTip } from "@/components/editorial/hover-tip";
import { cn } from "@/lib/utils";
import { adminManifestOptions } from "@/lib/queries/admin";
import {
  absoluteTime,
  formatBytes,
  relativeTime,
} from "@/lib/admin-format";
import { ModalSection } from "@/components/admin/feed-modal-primitives";

export function FeedModalManifest({ feed }: { feed: AdminFeed }) {
  const manifestQuery = useQuery({
    ...adminManifestOptions(feed.name),
    retry: false,
    refetchOnMount: "always",
  });

  const manifest = manifestQuery.data;
  const files = useMemo(() => {
    const list = manifest?.files ?? [];
    return [...list].sort((a, b) => {
      const aPri = priority(a);
      const bPri = priority(b);
      if (aPri !== bPri) return aPri - bPri;
      if (a.kind !== b.kind) return a.kind.localeCompare(b.kind);
      return a.rel.localeCompare(b.rel);
    });
  }, [manifest]);

  return (
    <ModalSection
      title="File manifest"
      right={
        <div className="flex items-center gap-3">
          {manifest && (
            <span className="text-xs tabular-nums text-muted-foreground">
              {manifest.summary.present}/{manifest.summary.total} present
              {manifest.summary.missing > 0 && (
                <span className="text-destructive">
                  {" · "}
                  {manifest.summary.missing} missing
                </span>
              )}
              {manifest.summary.stale > 0 && (
                <span className="text-status-warning">
                  {" · "}
                  {manifest.summary.stale} stale
                </span>
              )}
            </span>
          )}
          <button
            type="button"
            onClick={() => manifestQuery.refetch()}
            disabled={manifestQuery.isFetching}
            className="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground disabled:opacity-40"
          >
            <RefreshCw
              className={cn(
                "h-3 w-3",
                manifestQuery.isFetching && "animate-spin",
              )}
            />
            Refresh
          </button>
        </div>
      }
    >
      {manifestQuery.isLoading && (
        <div className="col-span-2 h-24 animate-pulse bg-muted/40" />
      )}
      {manifestQuery.error && (
        <div className="col-span-2 text-sm text-destructive">
          Failed to load manifest: {(manifestQuery.error as Error).message}
        </div>
      )}
      {manifest && (
        <div className="col-span-2">
          <ManifestTable files={files} />
        </div>
      )}
    </ModalSection>
  );
}

function priority(f: ManifestFile): number {
  if (f.required && !f.exists) return 0;
  if (f.stale) return 1;
  if (!f.exists) return 3;
  return 2;
}

function ManifestTable({ files }: { files: ManifestFile[] }) {
  return (
    <table className="w-full border-collapse text-[11px]">
      <thead>
        <tr className="border-b border-border">
          <th className="w-5 py-1 pl-2" />
          <th className="eyebrow w-[70px] py-1 px-2 text-left">Kind</th>
          <th className="eyebrow py-1 px-2 text-left">File</th>
          <th className="eyebrow w-[100px] py-1 px-2 text-left">Provider</th>
          <th className="eyebrow w-[60px] py-1 px-2 text-right">Size</th>
          <th className="eyebrow w-[140px] py-1 pr-2 text-right">Modified</th>
        </tr>
      </thead>
      <tbody>
        {files.map((f) => (
          <tr
            key={f.path}
            className={cn(
              "border-b border-border/40",
              f.required && !f.exists && "bg-destructive/[0.05]",
              f.stale && "bg-status-warning/[0.06]",
            )}
          >
            <td className="py-0.5 pl-2">
              {f.exists ? (
                f.stale ? (
                  <HoverTip text="Older than last processed_date — pipeline fan-out did not refresh this file">
                    <ShieldAlert className="h-3 w-3 text-status-warning" />
                  </HoverTip>
                ) : (
                  <CheckCircle2 className="h-3 w-3 text-status-healthy" />
                )
              ) : f.required ? (
                <HoverTip text="Required but missing on disk">
                  <X className="h-3 w-3 text-destructive" />
                </HoverTip>
              ) : (
                <HoverTip text="Optional, not on disk">
                  <FileText className="h-3 w-3 text-muted-foreground/30" />
                </HoverTip>
              )}
            </td>
            <td className="py-0.5 px-2 text-[10px] uppercase tracking-wider text-muted-foreground">
              {f.kind}
            </td>
            <td className="py-0.5 px-2 font-mono text-[10px] break-all text-foreground">
              {f.rel}
              {f.required && (
                <span className="ml-1 text-[9px] text-muted-foreground">
                  req
                </span>
              )}
            </td>
            <td className="py-0.5 px-2 text-[10px] text-muted-foreground">
              {f.provider || ""}
            </td>
            <td className="py-0.5 px-2 text-right tabular-nums text-muted-foreground">
              {f.exists && f.size !== undefined ? formatBytes(f.size) : "—"}
            </td>
            <td className="py-0.5 pr-2 text-right tabular-nums text-muted-foreground">
              {f.exists && f.mtime ? (
                <HoverTip text={absoluteTime(f.mtime)}>
                  <span>{relativeTime(f.mtime)}</span>
                </HoverTip>
              ) : (
                "—"
              )}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
