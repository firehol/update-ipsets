import { useMemo, type ReactNode } from "react";
import { Link } from "react-router-dom";
import type {
  FeedHealthClass,
  IPSearchContext,
  IPSearchMatch,
  IPSearchResult,
} from "@/lib/api-types";
import { CategoryBadge } from "@/components/category-badge";
import { FeedRef } from "@/components/feed-detail/feed-ref";
import { FeedHealthTip } from "@/components/feed-health-tip";
import { ProvenanceBadge } from "@/components/provenance-badge";
import { HoverTip } from "@/components/editorial/hover-tip";
import { feedHealthColor, feedHealthLabel } from "@/lib/feed-health";
import { orderCategories, useCategoriesQuery } from "@/lib/categories";
import { maintainerSlug } from "@/lib/maintainers";
import { IPLookupCountryMap } from "@/components/home/ip-lookup-country-map";
import { cn } from "@/lib/utils";

export function IPSearchResults({
  ip,
  scope,
  result,
  loading,
  error,
  variant,
}: {
  ip: string;
  scope: "global" | "feed";
  result: IPSearchResult | undefined;
  loading: boolean;
  error?: string;
  variant: "hero" | "section" | "header";
}) {
  if (!ip) return null;

  if (loading) {
    return (
      <div className={resultTextClass(variant)}>
        Looking up <span className="font-mono">{ip}</span>…
      </div>
    );
  }

  if (error) {
    return (
      <div className={cn(resultTextClass(variant), "text-destructive")}>
        {error}
      </div>
    );
  }

  const matches = normalizeMatches(result);
  if (variant === "section" && result) {
    return <DetailedSectionResults ip={ip} result={result} scope={scope} />;
  }
  if (matches.length === 0) {
    return scope === "feed" ? (
      <div className={resultTextClass(variant)}>
        <div>
          <span className="font-mono">{ip}</span> is not present in this feed.
        </div>
        <Link
          to={`/?ip=${encodeURIComponent(ip)}`}
          className={emptyLinkClass(variant)}
        >
          Search all feeds
        </Link>
      </div>
    ) : (
      <div className={resultTextClass(variant)}>
        <span className="font-mono">{ip}</span> is not present in the tracked
        feeds.
      </div>
    );
  }

  return (
    <div className="space-y-3">
      <div className={resultTextClass(variant)}>
        {scope === "feed"
          ? `${ip} is present in this feed.`
          : `${matches.length} tracked ${matches.length === 1 ? "feed" : "feeds"} currently match ${ip}.`}
      </div>
      <div className={cardGridClass(variant)}>
        {matches.map((match) => (
          <ResultCard key={match.name} match={match} variant={variant} />
        ))}
      </div>
    </div>
  );
}

function DetailedSectionResults({
  ip,
  result,
  scope,
}: {
  ip: string;
  result: IPSearchResult;
  scope: "global" | "feed";
}) {
  const categoriesQuery = useCategoriesQuery();
  const matches = normalizeMatches(result);
  const ctx = result.context;
  const grouped = groupMatchesByCategory(matches);
  const orderedCategories = useMemo(
    () => orderCategories(categoriesQuery.data ?? [], Object.keys(grouped)),
    [categoriesQuery.data, grouped],
  );
  const countryCode = ctx?.country_code?.trim().toUpperCase() ?? "";
  const regionCode = isISORegionCode(countryCode) ? countryCode : "";
  const countryLabel = countryCode ? formatCountryLabel(countryCode) : null;

  return (
    <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.6fr)]">
      <div className="space-y-4">
        <div className="grid gap-px bg-border">
          <ContextTile
            label="IP address"
            value={<span className="font-mono">{ip}</span>}
          />
          <ContextTile
            label="Country"
            value={
              countryLabel && regionCode ? (
                <Link
                  to={`/countries/${regionCode}`}
                  className="inline-flex items-center gap-2 hover:text-primary"
                >
                  <span className="text-2xl leading-none">
                    {countryFlag(regionCode)}
                  </span>
                  <span>
                    {countryLabel}
                    <span className="ml-2 text-sm text-muted-foreground">
                      {regionCode}
                    </span>
                  </span>
                </Link>
              ) : countryLabel ? (
                <span>{countryLabel}</span>
              ) : (
                "—"
              )
            }
            provider={ctx?.geo_provider_label}
          />
          <ContextTile
            label="ASN"
            value={<ASNValue ctx={ctx} />}
            provider={ctx?.asn_provider_label}
          />
          <ContextTile
            label={scope === "feed" ? "Match status" : "Feeds matched"}
            value={
              scope === "feed" ? (
                <span className={matches.length > 0 ? "text-primary" : ""}>
                  {matches.length > 0 ? "Present in this feed" : "Not present in this feed"}
                </span>
              ) : (
                <span className={matches.length > 0 ? "text-primary" : ""}>
                  {matches.length}
                </span>
              )
            }
          />
        </div>

        <div className="overflow-hidden border border-border bg-card">
          <div className="border-b border-border px-4 py-3">
            <div className="eyebrow text-muted-foreground">
              Geographic position
            </div>
            <div className="mt-1 text-[13px] text-muted-foreground">
              {regionCode
                ? "Highlighting the resolved country on the world map."
                : "A map is available only when the lookup resolves to a specific ISO country."}
            </div>
          </div>
          <IPLookupCountryMap countryCode={regionCode} />
        </div>
      </div>

      <div>
        {matches.length === 0 ? (
          <div className="border border-dashed border-border p-8 text-[13px] text-muted-foreground">
            <span className="font-mono">{ip}</span>{" "}
            {scope === "feed"
              ? "is not present in this feed."
              : "is not in any currently tracked public feed."}
            {(countryLabel || ctx?.country_code || ctx?.asn || ctx?.asn_name) && (
              <>
                {" "}
                It still resolves to{" "}
                <strong>{formatContextSummary(ctx, countryLabel)}</strong> via
                the configured providers.
              </>
            )}
            {scope === "feed" && (
              <>
                {" "}
                <Link
                  to={`/?ip=${encodeURIComponent(ip)}#ip-lookup`}
                  className="font-medium text-foreground hover:text-primary"
                >
                  Search all feeds
                </Link>
                .
              </>
            )}
          </div>
        ) : (
          <div className="space-y-8">
            {orderedCategories.map((category) => {
              const rows = grouped[category] ?? [];
              return (
                <section key={category}>
                  <header className="flex items-baseline justify-between gap-4 border-b border-border pb-2">
                    <div className="flex items-baseline gap-3">
                      {scope === "global" ? (
                        <>
                          <CategoryBadge category={category} />
                          <span className="text-[12px] text-muted-foreground">
                            {rows.length} feed{rows.length === 1 ? "" : "s"}
                          </span>
                        </>
                      ) : (
                        <>
                          <span className="eyebrow text-muted-foreground">
                            This feed
                          </span>
                          <span className="text-[12px] text-muted-foreground">
                            membership match
                          </span>
                        </>
                      )}
                    </div>
                  </header>
                  <ul className="mt-3 divide-y divide-border">
                    {rows.map((row) => (
                      <li key={row.name} className="flex items-start gap-4 py-3">
                        <div className="min-w-0 flex-1">
                          <FeedRef
                            name={row.name}
                            feed={matchFeedDescriptor(row)}
                            className="block truncate font-mono text-[13px] font-semibold text-foreground hover:text-primary"
                          />
                          {(row.maintainer || row.first_seen || row.last_seen) && (
                            <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-muted-foreground">
                              {row.maintainer && (
                                <Link
                                  to={`/maintainers/${maintainerSlug(row.maintainer)}`}
                                  className="truncate hover:text-primary"
                                >
                                  {row.maintainer}
                                </Link>
                              )}
                              <MatchTiming row={row} />
                            </div>
                          )}
                        </div>
                        {row.provenance && (
                          <span className="shrink-0 text-[10px] uppercase tracking-[0.08em] text-muted-foreground">
                            {row.provenance.replace("secondary_", "")}
                          </span>
                        )}
                      </li>
                    ))}
                  </ul>
                </section>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}

function ResultCard({
  match,
  variant,
}: {
  match: IPSearchMatch;
  variant: "hero" | "section" | "header";
}) {
  const isDisplay = variant === "hero" || variant === "header";
  return (
    <FeedRef
      name={match.name}
      feed={matchFeedDescriptor(match)}
      className={cn(
        "block border px-4 py-4 transition-colors",
        isDisplay
          ? "border-display-border bg-white/[0.04] hover:border-primary/60"
          : "border-border bg-card hover:border-primary",
      )}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div
            className={cn(
              "truncate font-mono text-sm font-semibold",
              isDisplay ? "text-display-fg" : "text-foreground",
            )}
          >
            {match.name}
          </div>
          <div className="mt-2 flex flex-wrap items-center gap-2">
            {match.category && <CategoryBadge category={match.category} />}
            <ProvenanceBadge provenance={match.provenance} />
            {match.health && (
              <HoverTip text={<FeedHealthTip health={match.health} compact />}>
                <span className={feedHealthColor(match.health.class)}>
                  {feedHealthLabel(match.health.class)}
                </span>
              </HoverTip>
            )}
          </div>
        </div>
      </div>
      {(match.info || match.maintainer) && (
        <div
          className={cn(
            "mt-3 text-sm leading-relaxed",
            isDisplay ? "text-display-muted" : "text-muted-foreground",
          )}
        >
          {match.info || match.maintainer}
        </div>
      )}
    </FeedRef>
  );
}

function matchFeedDescriptor(match: IPSearchMatch) {
  return {
    name: match.name,
    short_description: match.info,
    maintainer: match.maintainer,
  };
}

function ASNValue({ ctx }: { ctx?: IPSearchContext }) {
  if (!ctx?.asn) return <>—</>;
  return (
    <Link to={`/asns/${ctx.asn}`} className="inline-flex flex-col hover:text-primary">
      <span>{ctx.asn_name || `AS${ctx.asn}`}</span>
      {ctx.asn_name && (
        <span className="mt-1 text-sm text-muted-foreground">
          AS{ctx.asn}
        </span>
      )}
    </Link>
  );
}

function normalizeMatches(result: IPSearchResult | undefined): IPSearchMatch[] {
  const raw = result?.matches ?? [];
  return raw
    .map((match) =>
      typeof match === "string" ? { name: match } : (match as IPSearchMatch),
    )
    .sort(compareMatches);
}

function compareMatches(left: IPSearchMatch, right: IPSearchMatch): number {
  const leftRank = healthRank(left.health?.class);
  const rightRank = healthRank(right.health?.class);
  if (leftRank !== rightRank) return leftRank - rightRank;
  return left.name.localeCompare(right.name);
}

function healthRank(value: FeedHealthClass | undefined) {
  switch (value) {
    case "healthy":
      return 0;
    case "delayed":
      return 1;
    case "risky":
      return 2;
    case "unmaintained":
      return 3;
    case "empty":
      return 4;
    case "unavailable":
      return 5;
    case "archived":
      return 6;
    default:
      return 7;
  }
}

function resultTextClass(variant: "hero" | "section" | "header") {
  return cn(
    "text-sm leading-relaxed",
    variant === "hero" || variant === "header"
      ? "text-display-muted"
      : "text-muted-foreground",
  );
}

function cardGridClass(variant: "hero" | "section" | "header") {
  return cn(
    "grid gap-3",
    variant === "header" ? "max-h-[24rem] overflow-auto" : "md:grid-cols-2",
  );
}

function emptyLinkClass(variant: "hero" | "section" | "header") {
  return cn(
    "inline-flex items-center text-sm font-medium transition-colors",
    variant === "hero" || variant === "header"
      ? "text-display-fg hover:text-primary"
      : "text-foreground hover:text-primary",
  );
}

function ContextTile({
  label,
  value,
  provider,
}: {
  label: string;
  value: ReactNode;
  provider?: string;
}) {
  return (
    <div className="bg-card px-4 py-5">
      <div className="eyebrow text-muted-foreground">{label}</div>
      <div className="mt-2 text-[18px] font-semibold text-foreground">
        {value}
      </div>
      {provider && (
        <div className="mt-1 text-[11px] text-muted-foreground">
          via {provider}
        </div>
      )}
    </div>
  );
}

function groupMatchesByCategory(
  matches: IPSearchMatch[],
): Record<string, IPSearchMatch[]> {
  const out: Record<string, IPSearchMatch[]> = {};
  for (const match of matches) {
    const key = match.category || "other";
    if (!out[key]) out[key] = [];
    out[key].push(match);
  }
  return out;
}

function MatchTiming({ row }: { row: IPSearchMatch }) {
  if (!row.first_seen && !row.last_seen) {
    return null;
  }

  return (
    <>
      {row.first_seen && (
        <span>First seen {formatSeenTimestamp(row.first_seen)}</span>
      )}
      {row.last_seen && (
        <span>Last seen {formatSeenTimestamp(row.last_seen)}</span>
      )}
    </>
  );
}

const countryDisplayNames =
  typeof Intl !== "undefined" && typeof Intl.DisplayNames === "function"
    ? new Intl.DisplayNames(["en"], { type: "region" })
    : null;

const seenTimestampFormatter =
  typeof Intl !== "undefined"
    ? new Intl.DateTimeFormat("en", {
        dateStyle: "medium",
        timeStyle: "short",
        timeZone: "UTC",
      })
    : null;

function formatCountryLabel(code: string): string {
  if (code === "COUNTRYLESS") {
    return "Countryless";
  }
  if (!isISORegionCode(code)) {
    return prettifyCountryMarker(code);
  }
  try {
    return countryDisplayNames?.of(code) ?? code;
  } catch {
    return code;
  }
}

function formatSeenTimestamp(ts: number): string {
  if (!seenTimestampFormatter) {
    return `${ts}`;
  }
  return `${seenTimestampFormatter.format(new Date(ts * 1000))} UTC`;
}

function countryFlag(code: string): string {
  if (!isISORegionCode(code)) return code;
  return String.fromCodePoint(
    ...code.split("").map((char) => 0x1f1a5 + char.charCodeAt(0)),
  );
}

function isISORegionCode(code: string): boolean {
  return /^[A-Z]{2}$/.test(code);
}

function prettifyCountryMarker(code: string): string {
  return code
    .trim()
    .replace(/[_-]+/g, " ")
    .toLowerCase()
    .replace(/\b\w/g, (char) => char.toUpperCase());
}

function formatContextSummary(
  ctx: IPSearchContext | undefined,
  countryLabel: string | null,
): string {
  const parts: string[] = [];
  if (countryLabel) {
    const code = ctx?.country_code?.trim().toUpperCase();
    parts.push(code ? `${countryLabel} (${code})` : countryLabel);
  } else if (ctx?.country_code) {
    parts.push(ctx.country_code.trim().toUpperCase());
  }
  if (ctx?.asn_name && ctx?.asn) {
    parts.push(`${ctx.asn_name} (AS${ctx.asn})`);
  } else if (ctx?.asn_name) {
    parts.push(ctx.asn_name);
  } else if (ctx?.asn) {
    parts.push(`AS${ctx.asn}`);
  }
  return parts.join(", ");
}
