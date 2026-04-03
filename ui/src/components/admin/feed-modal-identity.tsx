import { Copy } from "lucide-react";
import { toast } from "sonner";
import type { AdminFeed } from "@/lib/api-types";
import { formatFrequency, kindLabel } from "@/lib/admin-format";
import { KV, ModalSection } from "@/components/admin/feed-modal-primitives";
import { safeExternalUrl } from "@/lib/safe-url";

export function FeedModalIdentity({ feed }: { feed: AdminFeed }) {
  const maintainerUrl = safeExternalUrl(feed.maintainer_url);
  return (
    <ModalSection title="Identity">
      <KV label="Kind" value={kindLabel(feed.kind)} />
      {feed.uses && feed.uses.length > 0 && (
        <KV label="Roles" value={feed.uses.join(", ")} />
      )}
      {feed.url && (
        <KV label="Download URL" value={<CopyableURL url={feed.url} />} span2 />
      )}
      {feed.public_url && feed.public_url !== feed.url && (
        <KV
          label="Public URL"
          value={<CopyableURL url={feed.public_url} />}
          span2
        />
      )}
      {feed.maintainer && (
        <KV
          label="Maintainer"
          value={
            feed.maintainer_url ? (
              maintainerUrl ? (
                <a
                  href={maintainerUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="underline decoration-dotted underline-offset-4 hover:decoration-solid"
                >
                  {feed.maintainer}
                </a>
              ) : (
                feed.maintainer
              )
            ) : (
              feed.maintainer
            )
          }
        />
      )}
      {feed.license && <KV label="License" value={feed.license} />}
      {feed.attribution && (
        <KV label="Attribution" value={feed.attribution} span2 />
      )}
      <KV
        label="Redistribution"
        value={
          feed.redistributable ? (
            <span className="text-status-healthy">allowed</span>
          ) : (
            <span className="text-status-warning">not redistributed</span>
          )
        }
      />
      <KV
        label="Public visibility"
        value={
          feed.hidden ? (
            <span className="text-muted-foreground">hidden from catalog</span>
          ) : (
            <span className="text-status-healthy">visible on site</span>
          )
        }
      />
      {feed.ipv && <KV label="IPv" value={feed.ipv} />}
      {feed.output && <KV label="Output" value={feed.output} />}
      {feed.hash && <KV label="Hash" value={feed.hash} />}
      {feed.processor_raw && (
        <KV
          label="Processor"
          value={
            <code className="font-mono text-xs">{feed.processor_raw}</code>
          }
          span2
        />
      )}
      {feed.downloader && <KV label="Downloader" value={feed.downloader} />}
      {feed.downloader_options && (
        <KV
          label="Downloader options"
          value={
            <code className="font-mono text-xs">{feed.downloader_options}</code>
          }
          span2
        />
      )}
      {feed.derived_from && feed.derived_from.length > 0 && (
        <KV
          label="Derived from"
          value={<NamePills names={feed.derived_from} />}
          span2
        />
      )}
      {feed.merge_included && feed.merge_included.length > 0 && (
        <KV
          label="Merge included"
          value={<NamePills names={feed.merge_included.map((input) => input.name)} />}
          span2
        />
      )}
      {feed.merge_subtracted && feed.merge_subtracted.length > 0 && (
        <KV
          label="Merge subtracted"
          value={<NamePills names={feed.merge_subtracted.map((input) => input.name)} />}
          span2
        />
      )}
      {feed.merge_excluded && feed.merge_excluded.length > 0 && (
        <KV
          label="Merge excluded"
          value={
            <div className="flex flex-wrap gap-1">
              {feed.merge_excluded.map((input) => (
                <span
                  key={input.name}
                  className="inline-flex items-center rounded-sm border border-border bg-card px-2 py-0.5 font-mono text-[11px]"
                >
                  {input.name}: {mergeReasonLabel(input.reason)}
                </span>
              ))}
            </div>
          }
          span2
        />
      )}
      {feed.history_minutes && feed.history_minutes.length > 0 && (
        <KV
          label="Retention windows"
          value={feed.history_minutes
            .map((m) => formatFrequency(m).replace("every ", ""))
            .join(" · ")}
        />
      )}
      {feed.accept_empty && <KV label="Accept empty" value="yes" />}
      {feed.file && <KV label="Output file" value={feed.file} />}
      {feed.source && <KV label="Source file" value={feed.source} />}
    </ModalSection>
  );
}

function NamePills({ names }: { names: string[] }) {
  return (
    <div className="flex flex-wrap gap-1">
      {names.map((name) => (
        <span
          key={name}
          className="inline-flex items-center rounded-sm border border-border bg-card px-2 py-0.5 font-mono text-[11px]"
        >
          {name}
        </span>
      ))}
    </div>
  );
}

function CopyableURL({ url }: { url: string }) {
  const safeUrl = safeExternalUrl(url);
  return (
    <div className="flex items-center gap-2">
      {safeUrl ? (
        <a
          href={safeUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="break-all font-mono text-xs text-foreground underline decoration-dotted underline-offset-4 hover:decoration-solid"
        >
          {url}
        </a>
      ) : (
        <span className="break-all font-mono text-xs text-foreground">
          {url}
        </span>
      )}
      <button
        type="button"
        onClick={() => {
          navigator.clipboard.writeText(url);
          toast.success("URL copied");
        }}
        className="shrink-0 text-muted-foreground hover:text-foreground"
        aria-label="Copy URL"
      >
        <Copy className="h-3.5 w-3.5" />
      </button>
    </div>
  );
}

function mergeReasonLabel(reason: string | undefined): string {
  switch (reason) {
    case "disabled":
      return "disabled";
    case "archived":
      return "archived";
    case "unmaintained":
      return "unmaintained";
    case "missing_local_feed_body":
      return "missing local feed body";
    default:
      return "excluded";
  }
}
