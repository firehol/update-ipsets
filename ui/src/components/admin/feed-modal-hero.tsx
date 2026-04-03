import type { ReactNode } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  ExternalLink,
  Pause,
  Play,
  RefreshCw,
  RotateCcw,
  ShieldAlert,
} from "lucide-react";
import { toast } from "sonner";
import type { AdminFeed } from "@/lib/api-types";
import {
  adminDisableFeed,
  adminEnableFeed,
  adminRecheckFeed,
  adminReprocessFeed,
} from "@/lib/api-client/admin";
import { queryKeys } from "@/lib/query-keys";
import { CategoryBadge } from "@/components/category-badge";
import { HoverTip } from "@/components/editorial/hover-tip";
import { publicFeedURL } from "@/lib/public-url";
import { cn } from "@/lib/utils";
import {
  feedHealth,
  healthColor,
  healthLabel,
  kindLabel,
} from "@/lib/admin-format";
import { feedHealthDescription } from "@/lib/feed-health";

export function FeedModalHero({
  feed,
  publicBaseURL,
}: {
  feed: AdminFeed;
  publicBaseURL?: string | null;
}) {
  const health = feedHealth(feed);
  return (
    <div className="p-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="min-w-0 flex-1">
          <div className="mb-2 flex items-center gap-2">
            <span className={cn("text-base leading-none", healthColor(health))}>
              ●
            </span>
            <span className="eyebrow">{healthLabel(health)}</span>
            <span className="text-xs text-muted-foreground">
              · {kindLabel(feed.kind)}
              {feed.health.class === "archived" && " · archived"}
              {feed.hidden && " · hidden"}
              {!feed.redistributable && " · private"}
              {feed.version ? ` · v${feed.version}` : ""}
            </span>
          </div>
          <h2 className="font-mono text-2xl font-bold tracking-tight text-foreground break-all">
            {feed.name}
          </h2>
          {feed.category && (
            <div className="mt-3">
              <CategoryBadge category={feed.category} />
            </div>
          )}
          {feed.info && (
            <p className="mt-3 max-w-3xl text-sm leading-relaxed text-muted-foreground">
              {feed.info}
            </p>
          )}
          <p className="mt-3 max-w-3xl text-sm leading-relaxed text-muted-foreground">
            {feedHealthDescription(feed.health)}
          </p>
        </div>
        <FeedModalActions feed={feed} publicBaseURL={publicBaseURL} />
      </div>
    </div>
  );
}

function FeedModalActions({
  feed,
  publicBaseURL,
}: {
  feed: AdminFeed;
  publicBaseURL?: string | null;
}) {
  const queryClient = useQueryClient();
  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: queryKeys.adminFeeds() });
    queryClient.invalidateQueries({ queryKey: queryKeys.adminStatus() });
    queryClient.invalidateQueries({ queryKey: queryKeys.adminManifest(feed.name) });
    queryClient.invalidateQueries({ queryKey: queryKeys.adminIntegrityRoot() });
    queryClient.invalidateQueries({ queryKey: queryKeys.adminEntityIntegrity() });
  };

  const recheck = useMutation({
    mutationFn: () => adminRecheckFeed(feed.name),
    onSuccess: () => {
      toast.success(`${feed.name}: recheck scheduled`);
      invalidate();
    },
    onError: (e: Error) => toast.error(`Recheck failed: ${e.message}`),
  });
  const reprocess = useMutation({
    mutationFn: () => adminReprocessFeed(feed.name),
    onSuccess: () => {
      toast.success(`${feed.name}: reprocess scheduled`);
      invalidate();
    },
    onError: (e: Error) => toast.error(`Reprocess failed: ${e.message}`),
  });
  const enableDisable = useMutation({
    mutationFn: () =>
      feed.enabled
        ? adminDisableFeed(feed.name)
        : adminEnableFeed(feed.name),
    onSuccess: () => {
      toast.success(`${feed.name}: ${feed.enabled ? "disabled" : "enabled"}`);
      invalidate();
    },
    onError: (e: Error) => toast.error(`Failed: ${e.message}`),
  });
  const publicHref = publicFeedURL(publicBaseURL, feed.name);

  return (
    <div className="flex flex-wrap items-center gap-2">
      <ActionButton
        label="Recheck"
        description="Refresh downloader-stage input or local composition now, then queue processing even if the feed body does not change."
        icon={<RotateCcw className="h-3.5 w-3.5" />}
        pending={recheck.isPending}
        onClick={() => recheck.mutate()}
      />
      <ActionButton
        label="Reprocess"
        description="Rerun processing from existing staged or committed local feed-body state without fetching."
        icon={<ShieldAlert className="h-3.5 w-3.5" />}
        pending={reprocess.isPending}
        onClick={() => reprocess.mutate()}
        accent
      />
      <ActionButton
        label={feed.enabled ? "Disable" : "Enable"}
        description={
          feed.enabled
            ? "Stop scheduling this feed. Existing files are kept."
            : "Add this feed back to the scheduler."
        }
        icon={
          feed.enabled ? (
            <Pause className="h-3.5 w-3.5" />
          ) : (
            <Play className="h-3.5 w-3.5" />
          )
        }
        pending={enableDisable.isPending}
        onClick={() => enableDisable.mutate()}
      />
      {publicHref && (
        <a
          href={publicHref}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-1.5 rounded-sm border border-border bg-card px-3 py-2 text-[13px] font-medium text-foreground transition-colors hover:border-foreground"
        >
          <ExternalLink className="h-3.5 w-3.5" />
          Public page
        </a>
      )}
    </div>
  );
}

function ActionButton({
  label,
  description,
  icon,
  pending,
  onClick,
  accent,
}: {
  label: string;
  description: string;
  icon: ReactNode;
  pending: boolean;
  onClick: () => void;
  accent?: boolean;
}) {
  return (
    <HoverTip text={description}>
      <button
        type="button"
        onClick={onClick}
        disabled={pending}
        className={cn(
          "inline-flex items-center gap-1.5 rounded-sm border px-3 py-2 text-[13px] font-medium transition-colors",
          accent
            ? "border-primary bg-primary text-primary-foreground hover:opacity-90"
            : "border-border bg-card text-foreground hover:border-foreground",
          pending && "opacity-50",
        )}
      >
        {pending ? <RefreshCw className="h-3.5 w-3.5 animate-spin" /> : icon}
        {label}
      </button>
    </HoverTip>
  );
}
