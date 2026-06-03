import { BookOpen, Check, ExternalLink, Quote, Users } from "lucide-react";
import type { FeedMetadata } from "@/lib/api-types";
import type { FeedEnrichmentRole } from "@/lib/enrichment-types";
import { useCategoryAccent } from "@/lib/categories";
import { safeExternalUrl } from "@/lib/safe-url";
import { cn } from "@/lib/utils";
import { DetailSection } from "./section";
import { MarkdownText } from "./markdown-text";
import { extractPullQuote } from "./pull-quote";

const ROLE_LABELS: Record<string, string> = {
  maintainer: "Maintainer",
  publisher: "Publisher",
  aggregator: "Aggregator",
  source_contributor: "Source contributor",
  original_author: "Original author",
  successor: "Successor",
};

/**
 * Editorial-style opening section for the feed-detail page. Treats the
 * researched `long_description` like a magazine article opening:
 *   - drop-cap on the first paragraph
 *   - a pull-quote callout extracted from the article body
 *   - a right-rail sidebar with operator, upstream link, scope lede,
 *     and an "Intended for" checklist
 */
export function SectionEditorial({ feed }: { feed: FeedMetadata }) {
  const enrichment = feed.enrichment;
  const accent = useCategoryAccent(feed.category);
  if (!enrichment?.long_description) {
    return <FallbackAbout feed={feed} accent={accent} />;
  }
  const pullQuote = extractPullQuote(enrichment.long_description);
  const scopeLede = truncateSentences(enrichment.scope_and_intent?.description, 2);
  const intendedFor = enrichment.scope_and_intent?.intended_for ?? [];
  const roles = (enrichment.roles ?? []).slice(0, 2);
  const sourceUrl = safeExternalUrl(feed.source);

  return (
    <DetailSection
      eyebrow="The story"
      title={feed.official_name?.trim() || enrichment.official_name || feed.name}
      icon={BookOpen}
      accentColor={accent}
      tight
    >
      <div className="grid grid-cols-1 gap-12 lg:grid-cols-12 lg:gap-16">
        <article className="lg:col-span-7">
          <MarkdownText
            value={enrichment.long_description}
            dropCap
            prose="editorial"
            className="max-w-[60ch]"
          />
          {pullQuote && (
            <figure className="mt-12 max-w-[60ch] border-l-[3px] pl-6" style={borderStyle(accent)}>
              <Quote
                className="mb-3 h-5 w-5"
                style={accent ? { color: accent } : undefined}
                aria-hidden="true"
              />
              <blockquote className="font-display text-[24px] leading-[1.35] text-foreground">
                &ldquo;{pullQuote}&rdquo;
              </blockquote>
            </figure>
          )}
        </article>
        <aside className="space-y-8 lg:col-span-5 lg:pl-8 lg:border-l lg:border-border">
          {roles.length > 0 && <OperatedBy roles={roles} accent={accent} />}
          {sourceUrl && (
            <SidebarLink
              icon={ExternalLink}
              label="Upstream source"
              value={feed.source ?? sourceUrl}
              href={sourceUrl}
              accent={accent}
            />
          )}
          {scopeLede && (
            <p className="text-[14px] italic leading-relaxed text-muted-foreground">
              {scopeLede}
            </p>
          )}
          {intendedFor.length > 0 && (
            <IntendedForList items={intendedFor} accent={accent} />
          )}
        </aside>
      </div>
    </DetailSection>
  );
}

function FallbackAbout({
  feed,
  accent,
}: {
  feed: FeedMetadata;
  accent: string | undefined;
}) {
  const body = feed.info?.trim() || "No description.";
  return (
    <DetailSection
      eyebrow="The story"
      title={feed.official_name?.trim() || feed.name}
      icon={BookOpen}
      accentColor={accent}
      tight
    >
      <MarkdownText value={body} className="max-w-3xl" />
    </DetailSection>
  );
}

function OperatedBy({
  roles,
  accent,
}: {
  roles: FeedEnrichmentRole[];
  accent: string | undefined;
}) {
  return (
    <div>
      <div className="flex items-center gap-2 eyebrow">
        <Users className="h-3.5 w-3.5" aria-hidden="true" />
        <span>Operated by</span>
      </div>
      <ul className="mt-4 space-y-3">
        {roles.map((role, index) => {
          const href = safeExternalUrl(role.official_url);
          return (
            <li
              key={`${role.role}-${role.name}-${index}`}
              className="border-l-[3px] pl-4"
              style={borderStyle(accent)}
            >
              <div className="text-[10px] uppercase tracking-[0.12em] text-muted-foreground">
                {ROLE_LABELS[role.role] ?? role.role.replace(/_/g, " ")}
              </div>
              <div className="mt-1 text-[15px] font-semibold text-foreground">
                {href ? (
                  <a
                    className="inline-flex items-center gap-1 text-foreground hover:text-primary"
                    href={href}
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    {role.name}
                    <ExternalLink className="h-3 w-3 opacity-70" />
                  </a>
                ) : (
                  role.name
                )}
              </div>
              {role.notes && (
                <p className="mt-1 text-[13px] leading-relaxed text-muted-foreground">
                  {role.notes}
                </p>
              )}
            </li>
          );
        })}
      </ul>
    </div>
  );
}

function SidebarLink({
  icon: Icon,
  label,
  value,
  href,
  accent,
}: {
  icon: typeof ExternalLink;
  label: string;
  value: string;
  href: string;
  accent: string | undefined;
}) {
  return (
    <div>
      <div className="flex items-center gap-2 eyebrow">
        <Icon className="h-3.5 w-3.5" aria-hidden="true" />
        <span>{label}</span>
      </div>
      <a
        href={href}
        target="_blank"
        rel="noopener noreferrer"
        className={cn(
          "mt-2 inline-flex max-w-full items-center gap-1 truncate text-[14px] text-foreground hover:underline",
        )}
        style={accent ? { textDecorationColor: accent } : undefined}
      >
        <span className="truncate">{value}</span>
      </a>
    </div>
  );
}

function IntendedForList({
  items,
  accent,
}: {
  items: string[];
  accent: string | undefined;
}) {
  return (
    <div>
      <div className="flex items-center gap-2 eyebrow">
        <Check className="h-3.5 w-3.5" aria-hidden="true" />
        <span>Intended for</span>
      </div>
      <ul className="mt-3 space-y-2">
        {items.map((item) => (
          <li key={item} className="flex gap-2 text-[14px] leading-relaxed text-muted-foreground">
            <Check
              className="mt-1 h-3.5 w-3.5 shrink-0"
              style={accent ? { color: accent } : undefined}
              aria-hidden="true"
            />
            <span>{item}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}

function truncateSentences(value: string | null | undefined, max: number): string {
  const trimmed = value?.trim();
  if (!trimmed) return "";
  const sentences = trimmed
    .replace(/\s+/g, " ")
    .split(/(?<=[.!?])\s+(?=[A-Z“"'])/)
    .map((s) => s.trim())
    .filter(Boolean);
  return sentences.slice(0, max).join(" ");
}

function borderStyle(accent: string | undefined) {
  if (!accent) return { borderColor: "hsl(var(--border))" };
  return { borderColor: accent };
}
