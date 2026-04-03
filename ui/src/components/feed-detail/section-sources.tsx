import { ChevronDown, BookMarked } from "lucide-react";
import type { FeedMetadata } from "@/lib/api-types";
import { safeExternalUrl } from "@/lib/safe-url";

/**
 * Researcher sources, folded by default, rendered at the very end of
 * the feed presentation. The list is auditable evidence for the
 * researched context surfaced higher on the page — useful to readers
 * who want to verify or follow the research, but never primary content.
 */
export function SectionSourcesConsulted({ feed }: { feed: FeedMetadata }) {
  const sources = feed.enrichment?.sources_consulted ?? [];
  const runAt = feed.enrichment?.run_at;
  if (sources.length === 0) return null;
  return (
    <section className="scroll-mt-24 border-t border-border py-10">
      <details className="group">
        <summary className="flex cursor-pointer list-none items-center justify-between gap-3">
          <span className="inline-flex items-center gap-2 eyebrow">
            <BookMarked className="h-3.5 w-3.5" aria-hidden="true" />
            <span>Sources consulted</span>
            <span className="ml-2 text-[11px] font-normal normal-case tracking-normal text-muted-foreground">
              {sources.length} {sources.length === 1 ? "source" : "sources"}
              {runAt ? ` · last researched ${formatResearchDate(runAt)}` : ""}
            </span>
          </span>
          <ChevronDown className="h-4 w-4 text-muted-foreground transition-transform group-open:rotate-180" />
        </summary>
        <ul className="mt-5 space-y-2 text-[13px] leading-relaxed text-muted-foreground">
          {sources.map((source) => {
            const href = safeExternalUrl(source.url);
            return (
              <li key={`${source.url}-${source.validation_date ?? ""}`}>
                {href ? (
                  <a
                    className="text-primary hover:underline [word-break:break-all]"
                    href={href}
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    {source.url}
                  </a>
                ) : (
                  <span className="[word-break:break-all]">{source.url}</span>
                )}
                {source.document_date && (
                  <span className="ml-2 text-muted-foreground">({source.document_date})</span>
                )}
              </li>
            );
          })}
        </ul>
      </details>
    </section>
  );
}

function formatResearchDate(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}
