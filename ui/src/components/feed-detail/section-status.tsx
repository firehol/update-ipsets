import { AlertCircle, type LucideIcon } from "lucide-react";
import type { FeedHealthClass, FeedHealthSnapshot, FeedMetadata } from "@/lib/api-types";
import { feedHealthDescription } from "@/lib/feed-health";
import { safeExternalUrl } from "@/lib/safe-url";
import { FeedRef } from "./feed-ref";
import { useFeedRefDescriptor } from "./feed-ref-descriptor";

const RESEARCH_LABEL: Record<string, string> = {
  discontinued: "Discontinued",
  merged: "Merged",
  forked: "Forked",
  reformatted: "Reformatted",
  altered_scope: "Altered scope",
  unknown: "Unknown",
};

const RESEARCH_LEAD: Record<string, string> = {
  discontinued: "The official status of this feed is discontinued:",
  merged: "The official status of this feed is merged:",
  forked: "The official status of this feed has been forked:",
  reformatted: "The official status of this feed is reformatted:",
  altered_scope: "The official status of this feed has been altered:",
  unknown: "The official status of this feed is unknown:",
};

const HEALTH_LABEL: Partial<Record<FeedHealthClass, string>> = {
  archived: "Archived",
  unmaintained: "Unmaintained",
  empty: "Empty",
};

const HEALTH_LEAD: Partial<Record<FeedHealthClass, string>> = {
  archived: "Our health automation has archived this feed:",
  unmaintained: "Our health automation has flagged this feed as unmaintained:",
  empty: "This feed currently contains no entries:",
};

/**
 * Renders below "The Story" whenever the feed has something we want
 * to call out before the reader scans the charts:
 *   - the researched `current_status` (AI lifecycle) — anything other
 *     than `active`, including `unknown` so the reader knows we tried
 *     and could not confirm
 *   - our internal health-class observation for `archived`,
 *     `unmaintained`, `empty` — these are derived deterministically
 *     from feed cadence and content, not from research narrative
 *
 * Both panels share the same chrome so the page has one consistent
 * "status" vocabulary even though the two signals come from different
 * sources.
 */
export function SectionStatus({ feed }: { feed: FeedMetadata }) {
  const research = feed.current_status;
  const showResearch = research && research.state !== "active";
  const health = feed.health;
  const showHealth = health && HEALTH_LABEL[health.class] !== undefined;
  if (!showResearch && !showHealth) return null;

  return (
    <section className="scroll-mt-24 pb-12">
      <div className="page-container space-y-4">
        {showHealth && health && (
          <StatusPanel
            label={HEALTH_LABEL[health.class] ?? health.class}
            lead={HEALTH_LEAD[health.class] ?? "Our health automation has flagged this feed:"}
            description={feedHealthDescription(health as FeedHealthSnapshot)}
          />
        )}
        {showResearch && research && (
          <StatusPanel
            label={RESEARCH_LABEL[research.state] ?? research.state.replace(/_/g, " ")}
            lead={RESEARCH_LEAD[research.state] ?? `The official status of this feed is ${research.state.replace(/_/g, " ")}:`}
            description={research.description?.trim()}
            footer={
              <>
                {research.successor && <SuccessorLine successor={research.successor} />}
                {research.announcement_date && (
                  <p className="mt-2 text-[12px] text-muted-foreground">
                    Announced {research.announcement_date}.
                  </p>
                )}
              </>
            }
          />
        )}
      </div>
    </section>
  );
}

function StatusPanel({
  label,
  lead,
  description,
  footer,
  icon: Icon = AlertCircle,
}: {
  label: string;
  lead: string;
  description?: string;
  footer?: React.ReactNode;
  icon?: LucideIcon;
}) {
  return (
    <div className="border-l-[3px] border-amber-500/70 bg-amber-500/[0.04] px-6 py-5">
      <div className="flex items-center gap-2">
        <Icon className="h-4 w-4 text-amber-500" aria-hidden="true" />
        <h3 className="text-[11px] font-semibold uppercase tracking-[0.14em] text-amber-700 dark:text-amber-300">
          Status: {label}
        </h3>
      </div>
      {description && (
        <p className="mt-3 text-[15px] leading-relaxed text-foreground/85">
          <span className="font-semibold text-foreground">{lead}</span>{" "}
          {description}
        </p>
      )}
      {footer}
    </div>
  );
}

function SuccessorLine({
  successor,
}: {
  successor: { name?: string | null; url?: string | null };
}) {
  const successorName = successor.name?.trim();
  const successorUrl = safeExternalUrl(successor.url);
  if (!successorName && !successorUrl) return null;
  return (
    <p className="mt-3 text-[14px] leading-relaxed text-muted-foreground">
      Successor:{" "}
      {successorName ? (
        <SuccessorFeed name={successorName} url={successor.url ?? null} />
      ) : (
        successorUrl && (
          <a
            href={successorUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="font-semibold underline-offset-4 hover:underline"
          >
            {successor.url}
          </a>
        )
      )}
    </p>
  );
}

function SuccessorFeed({
  name,
  url,
}: {
  name: string;
  url: string | null;
}) {
  const descriptor = useFeedRefDescriptor(name);
  return (
    <FeedRef
      name={name}
      feed={descriptor}
      description={url}
      className="font-mono font-semibold underline-offset-4 hover:underline"
    />
  );
}
