import { useState } from "react";
import { Link } from "react-router-dom";
import { useQueries, useQuery } from "@tanstack/react-query";
import { Network } from "lucide-react";
import type { ASNFeedPayload } from "@/lib/api-types";
import { asnFeedOptions, asnProvidersOptions } from "@/lib/queries/feed";
import { useCategoryAccent } from "@/lib/categories";
import { StatRow, StatTile } from "@/components/editorial/stat-row";
import { DataTable, type DataTableColumn } from "@/components/editorial/data-table";
import { ASNBubbleChart } from "./asn-bubble-chart";
import { ASNTreemap } from "./asn-treemap";
import { DetailNotice, DetailSection } from "./section";
import { ProviderTabBar, ProviderTab, ViewTabBar, ViewTab } from "./provider-tabs";
import { formatIPs, formatNum } from "@/lib/utils";

type ASNEntry = NonNullable<ASNFeedPayload["by_asn"]>[number];

// Single source of truth for the AS Composition view height. Treemap,
// Bubble, and List all render at this exact height so switching tabs
// cannot resize the section.
const ASN_VIEW_HEIGHT = 640;

/**
 * AS Composition section. Editorial layout:
 *   1. Provider tab strip (one tab per configured ASN source)
 *   2. Stat tile row: ASNs / Attributed / RFC reserved / Unknown
 *   3. View tabs: Treemap | Bubble | List
 *   4. The active visualization
 */
export function SectionASN({
  feedName,
  category,
}: {
  feedName: string;
  category?: string | null;
}) {
  const accent = useCategoryAccent(category);
  const providersQuery = useQuery(asnProvidersOptions(feedName));

  const providers = providersQuery.data ?? [];

  const dataQueries = useQueries({
    queries: providers.map((p) => asnFeedOptions(feedName, p.name)),
  });

  const dataByProvider: Record<string, ASNFeedPayload | undefined> = {};
  const queryByProvider: Record<string, (typeof dataQueries)[number] | undefined> = {};
  providers.forEach((p, i) => {
    dataByProvider[p.name] = dataQueries[i]?.data;
    queryByProvider[p.name] = dataQueries[i];
  });

  const [selectedProvider, setSelectedProvider] = useState<string>("");
  const active = providers.some((p) => p.name === selectedProvider)
    ? selectedProvider
    : (providers[0]?.name ?? "");
  // Default to treemap because it carries proportional composition
  // information better than the bubble pack.
  const [view, setView] = useState<"treemap" | "bubble" | "table">("treemap");

  if (providersQuery.isLoading) {
    return (
      <DetailSection
        eyebrow="Composition"
        title="AS attribution"
        lede="Loading…"
        icon={Network}
        accentColor={accent}
      >
        <div className="h-72 animate-pulse bg-muted/40" />
      </DetailSection>
    );
  }
  if (providers.length === 0) {
    return (
      <DetailSection
        eyebrow="Composition"
        title="AS attribution"
        lede="No ASN attribution providers are configured for this feed."
        icon={Network}
        accentColor={accent}
      >
        <div className="border border-border bg-card py-16 text-center text-sm text-muted-foreground">
          No ASN provider is configured for public AS attribution.
        </div>
      </DetailSection>
    );
  }

  const activeProvider = providers.find((provider) => provider.name === active);
  const activeQuery = queryByProvider[active];
  const activeData = dataByProvider[active];
  const activeLabel = activeProvider?.label || activeProvider?.name || active;
  const providerError =
    activeQuery?.isError && activeQuery.error instanceof Error
      ? activeQuery.error.message
      : null;
  const providerLoading = Boolean(activeQuery?.isLoading);
  const providerEmpty =
    !providerLoading &&
    !providerError &&
    Boolean(activeData) &&
    (!activeData?.by_asn || activeData.by_asn.length === 0);
  const treemapTruncated = (activeData?.by_asn?.length ?? 0) > 80;
  const bubbleTruncated = (activeData?.by_asn?.length ?? 0) > 60;

  return (
    <DetailSection
      eyebrow="Composition"
      title="Where the IPs come from"
      lede="Which Autonomous Systems operate the addresses in this list, with unknown and bogon space separated from attributed networks."
      icon={Network}
      accentColor={accent}
    >
      <ProviderTabBar>
        {providers.map((p) => (
          <ProviderTab
            key={p.name}
            label={p.label || p.name}
            active={active === p.name}
            onClick={() => setSelectedProvider(p.name)}
          />
        ))}
      </ProviderTabBar>

      {providerLoading && (
        <DetailNotice title={`${activeLabel} is still loading`} className="mt-8">
          ASN attribution from this provider has not finished loading yet. The
          other configured provider tabs remain available.
        </DetailNotice>
      )}
      {providerError && (
        <DetailNotice
          title={`${activeLabel} could not be loaded`}
          tone="danger"
          className="mt-8"
        >
          This provider is configured, but its per-feed ASN payload could not be
          loaded. This is different from "no ASN data for this feed".
          <div className="mt-2 font-mono text-xs text-foreground/80">
            {providerError}
          </div>
        </DetailNotice>
      )}
      {providerEmpty && (
        <DetailNotice title={`${activeLabel} found no ASN attributions`} className="mt-8">
          This provider did not attribute any ASNs for this feed. The feed may
          consist entirely of unattributed or reserved space under this
          provider's view.
        </DetailNotice>
      )}

      {!providerLoading && !providerError && activeData && (
        <div className="mt-10">
          <ASNStats data={activeData} />
        </div>
      )}

      {!providerLoading && !providerError && activeData && (
        <div className="mt-12">
          <div className="mb-6 flex items-center justify-between">
            <div className="eyebrow">Visualisation</div>
            <ViewTabBar>
              <ViewTab
                label="Treemap"
                active={view === "treemap"}
                onClick={() => setView("treemap")}
              />
              <ViewTab
                label="Bubble"
                active={view === "bubble"}
                onClick={() => setView("bubble")}
              />
              <ViewTab
                label="List"
                active={view === "table"}
                onClick={() => setView("table")}
              />
            </ViewTabBar>
          </div>
          {view === "treemap" && treemapTruncated && (
            <DetailNotice title="Top 80 ASNs only" className="mb-6">
              The treemap shows the 80 largest ASNs by IP count for readability.
              Use the List view for the complete provider breakdown.
            </DetailNotice>
          )}
          {view === "bubble" && bubbleTruncated && (
            <DetailNotice title="Top 60 ASNs only" className="mb-6">
              The bubble chart shows the 60 largest ASNs by IP count for
              readability. Use the List view for the complete provider breakdown.
            </DetailNotice>
          )}
          {view === "treemap" && <ASNTreemap data={activeData} height={ASN_VIEW_HEIGHT} />}
          {view === "bubble" && <ASNBubbleChart data={activeData} height={ASN_VIEW_HEIGHT} />}
          {view === "table" && <ASNFullTable data={activeData} />}
        </div>
      )}
    </DetailSection>
  );
}

/* -------------------------------------------------------------------------- */

function ASNStats({ data }: { data: ASNFeedPayload | undefined }) {
  if (!data || data.feed_ips <= 0) return null;
  const feed = data.feed_ips;
  const pct = (n: number) => ((n / feed) * 100).toFixed(2) + "%";
  return (
    <StatRow>
      <StatTile
        label="Distinct ASNs"
        value={formatNum((data.by_asn ?? []).length)}
        accent
      />
      <StatTile
        label="Attributed"
        value={formatIPs(data.attributed_ips)}
        caption={pct(data.attributed_ips) + " of feed"}
      />
      <StatTile
        label="RFC reserved"
        value={formatIPs(data.bogon_ips)}
        caption={pct(data.bogon_ips) + " of feed"}
      />
      <StatTile
        label="Unknown"
        value={formatIPs(data.unknown_ips)}
        caption={pct(data.unknown_ips) + " of feed"}
      />
    </StatRow>
  );
}

/* -------------------------------------------------------------------------- */

function ASNFullTable({ data }: { data: ASNFeedPayload | undefined }) {
  if (!data || !data.by_asn || data.by_asn.length === 0) {
    return (
      <div className="py-16 text-center text-sm text-muted-foreground">
        No ASN data for this provider.
      </div>
    );
  }

  const columns: DataTableColumn<ASNEntry>[] = [
    {
      key: "asn",
      label: "ASN",
      sortValue: (row) => row.asn,
      searchValue: (row) => `AS${row.asn}`,
      render: (row) => (
        <Link
          to={`/asns/${row.asn}`}
          className="font-mono text-foreground hover:text-primary"
        >
          AS{row.asn}
        </Link>
      ),
    },
    {
      key: "name",
      label: "Organization",
      sortValue: (row) => row.name || "",
      render: (row) =>
        row.name ? row.name : <span className="text-muted-foreground">(unknown)</span>,
    },
    {
      key: "count",
      label: "IPs",
      align: "right",
      sortValue: (row) => row.count,
      render: (row) => formatIPs(row.count),
    },
    {
      key: "percent",
      label: "% of feed",
      align: "right",
      sortValue: (row) => row.percent ?? 0,
      render: (row) => (
        <span className="text-muted-foreground">
          {(row.percent ?? 0).toFixed(2)}%
        </span>
      ),
    },
  ];

  return (
    <DataTable
      rows={data.by_asn}
      columns={columns}
      rowKey={(row) => row.asn}
      initialSortKey="count"
      initialSortDir="desc"
      exportFilename="asn-composition"
      searchPlaceholder="Filter by ASN or organisation…"
      viewportHeight={ASN_VIEW_HEIGHT}
    />
  );
}
