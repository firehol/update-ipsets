import { Award, ChevronDown, MessageSquareQuote, Sparkles } from "lucide-react";
import type { FeedMetadata } from "@/lib/api-types";
import { MarkdownText } from "./markdown-text";

/**
 * Reputation and community signals as a quiet footer-style section,
 * below the technical specs. Three small disclosures: positive
 * signals, past complaints, maintainer engagement. Collapsed by
 * default so this section never dominates the page — these are
 * supporting signals, not primary content.
 */
export function SectionReputation({ feed }: { feed: FeedMetadata }) {
  const community = feed.enrichment?.community;
  if (!community) return null;
  const awards = community.awards?.trim();
  const criticism = community.criticism?.trim();
  const engagement = community.engagement?.trim();
  if (!awards && !criticism && !engagement) return null;

  return (
    <section className="scroll-mt-24 border-t border-border py-12">
      <div className="flex items-center gap-2 eyebrow">
        <MessageSquareQuote className="h-3.5 w-3.5" aria-hidden="true" />
        <span>Reputation &amp; community signals</span>
      </div>
      <p className="mt-2 max-w-[60ch] text-[13px] leading-relaxed text-muted-foreground">
        Supplementary signals gathered while researching this feed. Not used
        for blocking decisions.
      </p>
      <div className="mt-5 grid grid-cols-1 gap-3 md:grid-cols-3">
        <FooterSignal
          icon={Award}
          title="Positive signals"
          body={awards}
          tone="positive"
        />
        <FooterSignal
          icon={MessageSquareQuote}
          title="Past complaints"
          body={criticism}
          tone="neutral"
        />
        <FooterSignal
          icon={Sparkles}
          title="Maintainer engagement"
          body={engagement}
          tone="neutral"
        />
      </div>
    </section>
  );
}

function FooterSignal({
  icon: Icon,
  title,
  body,
  tone,
}: {
  icon: typeof Award;
  title: string;
  body: string | undefined;
  tone: "positive" | "neutral";
}) {
  if (!body) return null;
  return (
    <details className="group border-l-[2px] border-border/70 pl-4">
      <summary className="flex cursor-pointer list-none items-center justify-between gap-3 text-[12px] font-semibold text-foreground">
        <span className="inline-flex items-center gap-2">
          <Icon
            className={
              tone === "positive"
                ? "h-3.5 w-3.5 text-emerald-500"
                : "h-3.5 w-3.5 text-muted-foreground"
            }
            aria-hidden="true"
          />
          {title}
        </span>
        <ChevronDown className="h-3.5 w-3.5 transition-transform group-open:rotate-180" />
      </summary>
      <div className="mt-2 text-[13px] leading-relaxed">
        <MarkdownText value={body} className="text-[13px] leading-relaxed" />
      </div>
    </details>
  );
}
