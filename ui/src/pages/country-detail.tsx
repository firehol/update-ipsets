import { type ReactNode, useMemo } from "react";
import { Link, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { AccentBar } from "@/components/editorial/accent-bar";
import { CategoryBadge } from "@/components/category-badge";
import { ProvenanceBadge } from "@/components/provenance-badge";
import { FeedRef } from "@/components/feed-detail/feed-ref";
import { useFeedRefDescriptorMap } from "@/components/feed-detail/feed-ref-descriptor";
import { countryOptions } from "@/lib/queries/entities";
import {
  feedHealthColor,
  feedHealthDotColor,
  feedHealthLabel,
} from "@/lib/feed-health";
import { orderCategories, useCategoriesQuery } from "@/lib/categories";
import { IPLookupCountryMap } from "@/components/home/ip-lookup-country-map";
import { formatCountryLabel } from "@/lib/countries";
import { maintainerSlug } from "@/lib/maintainers";
import { formatNum, timeAgo } from "@/lib/utils";
import type {
  CountryDetailASN,
  CountryDetailFeed,
} from "@/lib/api-types";

const DETAIL_SUMMARY_VIEWPORT_CLASS =
  "mt-5 max-h-[24rem] space-y-3 overflow-y-auto pr-2";
const DETAIL_FEEDS_VIEWPORT_CLASS =
  "max-h-[46rem] overflow-auto pr-2";

export function CountryDetailPage() {
  const { code } = useParams<{ code: string }>();
  const normalized = (code ?? "").toUpperCase();
  const categoriesQuery = useCategoriesQuery();

  const query = useQuery({
    ...countryOptions(normalized),
    enabled: !!normalized,
  });

  const payload = query.data;
  const countryName = formatCountryLabel(payload?.code ?? normalized);
  const feedsByCategory = useMemo(
    () =>
      orderCategories(
        categoriesQuery.data ?? [],
        Object.keys(payload?.feeds_by_category ?? {}),
      ).map((category) => [category, payload?.feeds_by_category?.[category] ?? []] as const),
    [categoriesQuery.data, payload],
  );

  if (query.isLoading) {
    return (
      <div className="page-container py-20 md:py-24 text-[13px] text-muted-foreground">
        Loading country detail…
      </div>
    );
  }

  if (query.isError || !payload) {
    return (
      <div className="page-container py-20 md:py-24">
        <AccentBar />
        <div className="eyebrow mt-6 text-muted-foreground">Country</div>
        <h1 className="display-title mt-4 text-foreground">
          {normalized || "Unknown"}
        </h1>
        <p className="lede mt-5 max-w-[62ch] text-muted-foreground">
          No public feeds attribute IPs to this country yet, or the
          geolocation provider has not finished its first run.
        </p>
        <div className="mt-8 text-[13px]">
          <Link to="/#explorer" className="text-primary hover:underline">
            ← Back to the feed explorer
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="page-container py-20 md:py-24">
      <AccentBar />

      <section className="mt-8 grid gap-10 xl:grid-cols-[minmax(0,1.25fr)_380px] xl:items-start">
        <div>
          <div className="eyebrow text-muted-foreground">Country</div>
          <h1 className="display-title mt-4 text-foreground">
            {countryName}
            <span className="ml-4 align-middle font-mono text-[0.5em] uppercase tracking-[0.18em] text-muted-foreground">
              {payload.code}
            </span>
          </h1>
          <p className="lede mt-5 max-w-[68ch] text-muted-foreground">
            Public feeds that currently attribute IPs to this country under the{" "}
            <span className="font-semibold text-foreground">
              {payload.provider.label ?? payload.provider.name}
            </span>{" "}
            geolocation provider. Category and maintainer blocks below help
            explain where this country appears in the catalog, not just which
            feed names match it.
          </p>

          <div className="mt-8 flex flex-wrap gap-3 text-[11px] uppercase tracking-[0.08em] text-muted-foreground">
            <ProviderChip label="Geo provider" value={payload.provider.label ?? payload.provider.name} />
            {payload.asn_provider?.name && (
              <ProviderChip
                label="ASN provider"
                value={payload.asn_provider.label ?? payload.asn_provider.name}
              />
            )}
          </div>

          <div className="mt-10 grid grid-cols-2 gap-px overflow-hidden border-t border-b border-border xl:grid-cols-5">
            <Stat label="Matching feeds" value={formatNum(payload.totals.feeds_matching)} accent />
            <Stat label="IPs in feeds" value={formatNum(payload.totals.attributed_ips_in_feeds)} />
            <Stat label="Categories" value={formatNum(payload.totals.categories)} />
            <Stat label="Maintainers" value={formatNum(payload.totals.maintainers)} />
            <Stat label="ASNs" value={formatNum(payload.totals.asns)} />
          </div>
        </div>

        <div className="border border-border bg-muted/20 p-4">
          <div className="eyebrow text-muted-foreground">Map</div>
          <h2 className="mt-2 text-[18px] font-semibold tracking-tight text-foreground">
            Country outline
          </h2>
          <p className="mt-2 text-[13px] leading-relaxed text-muted-foreground">
            The map is intentionally simple here: this page is about one country
            as an entity, not about geographic distribution inside the country.
          </p>
          <div className="mt-4">
            <IPLookupCountryMap countryCode={payload.code} height={250} />
          </div>
        </div>
      </section>

      <section className="mt-16 grid gap-8 xl:grid-cols-3">
        <SummaryPanel
          title="Top Categories"
          description="Which feed categories contribute the most attributed IPs for this country."
          empty="No public categories contribute to this country yet."
          contentClassName={DETAIL_SUMMARY_VIEWPORT_CLASS}
        >
          {(payload.top_categories ?? []).map((row) => (
            <SummaryRow
              key={row.category}
              label={<CategoryBadge category={row.category} />}
              meta={`${formatNum(row.feed_count)} feed${row.feed_count === 1 ? "" : "s"}`}
              value={`${formatNum(row.attributed_ips)} IPs`}
            />
          ))}
        </SummaryPanel>

        <SummaryPanel
          title="Top Maintainers"
          description="Who publishes the feeds that most often attribute IPs to this country."
          empty="No maintainer metadata is available for these feeds."
          contentClassName={DETAIL_SUMMARY_VIEWPORT_CLASS}
        >
          {(payload.top_maintainers ?? []).map((row) => (
            <SummaryRow
              key={row.slug}
              label={
                <Link
                  to={`/maintainers/${row.slug}`}
                  className="font-medium text-foreground hover:text-primary"
                >
                  {row.name}
                </Link>
              }
              meta={`${formatNum(row.feed_count)} feed${row.feed_count === 1 ? "" : "s"}`}
              value={`${formatNum(row.attributed_ips)} IPs`}
            />
          ))}
        </SummaryPanel>

        <SummaryPanel
          title="Top ASNs In This Country"
          description="Observed from the feed ranges themselves, not inferred from whole-feed ASN rankings."
          empty="No ASN attribution is available for this country yet."
          contentClassName={DETAIL_SUMMARY_VIEWPORT_CLASS}
        >
          {(payload.top_asns_in_country ?? []).map((row: CountryDetailASN) => (
            <SummaryRow
              key={row.asn}
              label={
                <div className="min-w-0">
                  <Link
                    to={`/asns/${row.asn}`}
                    className="font-mono font-medium text-foreground hover:text-primary"
                  >
                    AS{row.asn}
                  </Link>
                  {row.name && (
                    <div className="mt-1 text-muted-foreground">
                      <span className="truncate text-[11px] tracking-normal">
                        {row.name}
                      </span>
                    </div>
                  )}
                </div>
              }
              meta={`${formatNum(row.feed_count)} feed${row.feed_count === 1 ? "" : "s"}`}
              value={`${formatNum(row.attributed_ips)} IPs`}
            />
          ))}
        </SummaryPanel>
      </section>

      <section className="mt-20">
        <header className="border-b border-border pb-4">
          <div className="eyebrow text-muted-foreground">Feeds</div>
          <h2 className="display-subtitle mt-2 text-foreground">
            Feeds Grouped By Category
          </h2>
          <p className="mt-3 max-w-[72ch] text-[14px] leading-relaxed text-muted-foreground">
            This shows where the country appears across the public feed
            inventory, with each feed’s health and provenance left visible so
            stale or derived sources are not hidden.
          </p>
        </header>

        <div className="mt-10 border border-border bg-card/40 p-5 md:p-6">
          <div className={DETAIL_FEEDS_VIEWPORT_CLASS}>
            <div className="space-y-12">
              {feedsByCategory.map(([category, rows]) => {
                const summary = (payload.top_categories ?? []).find((row) => row.category === category);
                return (
                  <section key={category}>
                    <header className="flex flex-wrap items-baseline justify-between gap-4 border-b border-border pb-3">
                      <div className="flex items-baseline gap-3">
                        <CategoryBadge category={category} />
                        <span className="text-[12px] text-muted-foreground">
                          {rows.length} feed{rows.length === 1 ? "" : "s"}
                        </span>
                      </div>
                      {summary && (
                        <div className="num text-[12px] text-muted-foreground">
                          {formatNum(summary.attributed_ips)} IPs in this country
                        </div>
                      )}
                    </header>

                    <ul className="mt-3 divide-y divide-border">
                      {rows.map((feed) => (
                        <CountryFeedRow
                          key={feed.name}
                          feed={feed}
                          totalAttributed={payload.totals.attributed_ips_in_feeds}
                        />
                      ))}
                    </ul>
                  </section>
                );
              })}
            </div>
          </div>
        </div>
      </section>
    </div>
  );
}

function CountryFeedRow({
  feed,
  totalAttributed,
}: {
  feed: CountryDetailFeed;
  totalAttributed: number;
}) {
  const refMap = useFeedRefDescriptorMap();
  const share = totalAttributed > 0 ? (feed.attributed_ips / totalAttributed) * 100 : 0;
  return (
    <li className="grid w-max min-w-full gap-3 py-4 md:grid-cols-[minmax(24rem,1fr)_max-content_max-content_max-content] md:items-center md:gap-x-6">
      <div className="grid min-w-0 grid-cols-[auto_minmax(0,1fr)] gap-x-3">
        <span
          role="img"
          className="row-span-2 mt-1.5 inline-block h-2 w-2 shrink-0 rounded-full"
          style={{ backgroundColor: feedHealthDotColor(feed.health_class) }}
          aria-label={feedHealthLabel(feed.health_class)}
        />
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <FeedRef
            name={feed.name}
            feed={refMap.get(feed.name)}
            className="min-w-0 font-mono text-[13px] font-semibold text-foreground hover:text-primary md:whitespace-nowrap"
          >
            {feed.name}
          </FeedRef>
          <ProvenanceBadge provenance={feed.provenance} />
        </div>
        <div className="mt-1 flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1 text-[12px] text-muted-foreground md:flex-nowrap md:whitespace-nowrap">
          <span className={`font-medium ${feedHealthColor(feed.health_class)}`}>
            {feedHealthLabel(feed.health_class)}
          </span>
          {feed.maintainer ? (
            <Link to={`/maintainers/${maintainerSlug(feed.maintainer)}`} className="hover:text-primary">
              {feed.maintainer}
            </Link>
          ) : (
            <span>Unknown maintainer</span>
          )}
          <span>{share.toFixed(1)}% of this country page</span>
        </div>
      </div>
      <div className="min-w-0 text-[12px] text-muted-foreground md:text-right">
        <span className="num block font-semibold leading-tight text-foreground md:whitespace-nowrap">
          {formatNum(feed.attributed_ips)}
        </span>
        <div>attributed IPs</div>
      </div>
      <div className="min-w-0 text-[12px] text-muted-foreground md:text-right">
        <span className="num block font-semibold leading-tight text-foreground md:whitespace-nowrap">
          {formatNum(feed.unique_ips)}
        </span>
        <div>IPs in feed</div>
      </div>
      <div className="num text-[12px] text-muted-foreground md:text-right md:whitespace-nowrap">
        {timeAgo(feed.last_change_ts)}
      </div>
    </li>
  );
}

function ProviderChip({ label, value }: { label: string; value?: string }) {
  if (!value) return null;
  return (
    <span className="inline-flex items-center gap-2 border border-border bg-muted/30 px-3 py-1">
      <span>{label}</span>
      <span className="text-foreground">{value}</span>
    </span>
  );
}

function SummaryPanel({
  title,
  description,
  empty,
  children,
  contentClassName,
}: {
  title: string;
  description: string;
  empty: string;
  children: ReactNode;
  contentClassName?: string;
}) {
  const hasChildren = Array.isArray(children) ? children.length > 0 : Boolean(children);
  return (
    <section className="border border-border bg-card/70 p-5">
      <h2 className="display-subtitle text-foreground">{title}</h2>
      <p className="mt-2 text-[13px] leading-relaxed text-muted-foreground">
        {description}
      </p>
      <div className={contentClassName ?? "mt-5 space-y-3"}>
        {hasChildren ? (
          children
        ) : (
          <div className="text-[13px] text-muted-foreground">{empty}</div>
        )}
      </div>
    </section>
  );
}

function SummaryRow({
  label,
  meta,
  value,
}: {
  label: ReactNode;
  meta: string;
  value: string;
}) {
  return (
    <div className="flex items-start justify-between gap-4 border-b border-border pb-3 last:border-b-0 last:pb-0">
      <div className="min-w-0">
        <div className="min-w-0 text-[13px] leading-tight">{label}</div>
        <div className="mt-1 text-[11px] text-muted-foreground">{meta}</div>
      </div>
      <div className="num shrink-0 text-right text-[12px] leading-tight text-foreground [overflow-wrap:anywhere]">
        {value}
      </div>
    </div>
  );
}

function Stat({
  label,
  value,
  accent = false,
}: {
  label: string;
  value: string;
  accent?: boolean;
}) {
  return (
    <div className="min-w-0 border-r border-border px-2 py-8 last:border-r-0 md:px-3">
      <div className="eyebrow text-muted-foreground">{label}</div>
      <div
        className={
          "num display-stat-card mt-3 [overflow-wrap:anywhere] " +
          (accent ? "text-primary" : "text-foreground")
        }
      >
        {value}
      </div>
    </div>
  );
}
