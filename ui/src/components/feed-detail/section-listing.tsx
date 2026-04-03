import { ListChecks, Mail, MinusCircle, ScrollText } from "lucide-react";
import type { FeedMetadata } from "@/lib/api-types";
import type {
  FeedEnrichmentPolicy,
  FeedEnrichmentUnlistRequest,
} from "@/lib/enrichment-types";
import { useCategoryAccent } from "@/lib/categories";
import { safeExternalUrl } from "@/lib/safe-url";
import { DetailSection } from "./section";
import { MarkdownText } from "./markdown-text";

/**
 * Listing rules and removal as three visually distinct cards.
 * Listing and Unlisting use a neutral rail (informational). Removal
 * uses the primary accent (loud) because it is the only actionable
 * card on the page: it carries the contact URL and email for operators
 * who want their IPs delisted.
 */
export function SectionListing({ feed }: { feed: FeedMetadata }) {
  const enrichment = feed.enrichment;
  const accent = useCategoryAccent(feed.category);
  if (!enrichment) return null;
  const hasAnything =
    enrichment.listing_policy ||
    enrichment.unlisting_policy ||
    enrichment.unlist_request;
  if (!hasAnything) return null;

  return (
    <DetailSection
      eyebrow="Operator rules"
      title="How IPs get on and off this list"
      icon={ScrollText}
      accentColor={accent}
    >
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <PolicyCard
          icon={ListChecks}
          title="Listing policy"
          tone="neutral"
          accent={accent}
          policy={enrichment.listing_policy}
        />
        <PolicyCard
          icon={MinusCircle}
          title="Unlisting policy"
          tone="neutral"
          accent={accent}
          policy={enrichment.unlisting_policy}
        />
        <RemovalCard
          request={enrichment.unlist_request}
          accent={accent}
        />
      </div>
    </DetailSection>
  );
}

function PolicyCard({
  icon: Icon,
  title,
  tone,
  accent,
  policy,
}: {
  icon: typeof ListChecks;
  title: string;
  tone: "neutral" | "loud";
  accent: string | undefined;
  policy: FeedEnrichmentPolicy | null | undefined;
}) {
  const hasContent =
    (policy?.summary && policy.summary.trim().length > 0) ||
    (policy?.criteria?.length ?? 0) > 0;
  if (!hasContent) return null;
  const railStyle =
    tone === "loud" && accent
      ? { borderColor: accent, backgroundColor: `${accent}0d` }
      : undefined;
  const iconStyle = tone === "loud" && accent ? { color: accent } : undefined;
  return (
    <div
      className={
        tone === "loud"
          ? "border-l-[4px] border-primary bg-primary/5 p-5"
          : "border border-border p-5"
      }
      style={railStyle}
    >
      <div className="flex items-center gap-2">
        <Icon className="h-4 w-4" style={iconStyle} aria-hidden="true" />
        <h3 className="text-sm font-semibold text-foreground">{title}</h3>
      </div>
      <MarkdownText value={policy?.summary} className="mt-4" />
      {(policy?.criteria?.length ?? 0) > 0 && (
        <ul className="mt-4 list-disc space-y-2 pl-5 text-sm text-muted-foreground">
          {policy?.criteria?.map((item) => <li key={item}>{item}</li>)}
        </ul>
      )}
    </div>
  );
}

function RemovalCard({
  request,
  accent,
}: {
  request: FeedEnrichmentUnlistRequest | null | undefined;
  accent: string | undefined;
}) {
  const requestURL = safeExternalUrl(request?.url);
  const emailHref = request?.email ? `mailto:${request.email}` : undefined;
  const hasContent =
    !!request?.instructions?.trim() || !!requestURL || !!emailHref;
  if (!hasContent) return null;
  return (
    <div
      className="relative border-l-[4px] border-primary bg-primary/5 p-5"
      style={accent ? { borderColor: accent, backgroundColor: `${accent}0d` } : undefined}
    >
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <Mail
            className="h-4 w-4"
            style={accent ? { color: accent } : undefined}
            aria-hidden="true"
          />
          <h3 className="text-sm font-semibold text-foreground">Removal request</h3>
        </div>
        <span
          className="text-[10px] font-semibold uppercase tracking-[0.1em] text-primary"
          style={accent ? { color: accent } : undefined}
        >
          Actionable
        </span>
      </div>
      <MarkdownText value={request?.instructions} className="mt-4" />
      <div className="mt-5 flex flex-wrap gap-3">
        {requestURL && (
          <a
            className="inline-flex items-center gap-2 border border-primary/40 bg-primary/10 px-3 py-2 text-sm font-semibold text-primary hover:bg-primary/20"
            href={requestURL}
            target="_blank"
            rel="noopener noreferrer"
            style={
              accent
                ? { borderColor: `${accent}66`, color: accent, backgroundColor: `${accent}14` }
                : undefined
            }
          >
            Request page
          </a>
        )}
        {emailHref && (
          <a
            className="inline-flex items-center gap-2 border border-border px-3 py-2 text-sm text-foreground hover:bg-muted/50"
            href={emailHref}
          >
            <Mail className="h-4 w-4" aria-hidden="true" />
            Email
          </a>
        )}
      </div>
    </div>
  );
}
