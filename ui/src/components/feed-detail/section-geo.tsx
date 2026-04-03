import { useState } from "react";
import { useQueries, useQuery } from "@tanstack/react-query";
import { Globe2 } from "lucide-react";
import type {
  CountryComparisonPayload,
  FeedMetadata,
  GeoProvider,
} from "@/lib/api-types";
import {
  bogonFeedOptions,
  bogonProvidersOptions,
  geoFeedOptions,
  geoProvidersOptions,
} from "@/lib/queries/feed";
import { useCategoryAccent } from "@/lib/categories";
import { StatRow, StatTile } from "@/components/editorial/stat-row";
import { DataTable, type DataTableColumn } from "@/components/editorial/data-table";
import { GeoMap } from "./geo-map";
import { DetailNotice, DetailSection } from "./section";
import { ProviderTabBar, ProviderTab, ViewTabBar, ViewTab } from "./provider-tabs";
import { formatIPs, formatNum } from "@/lib/utils";

// Single source of truth for the Geographic Coverage view height. Map
// and List render at exactly this number so switching tabs cannot
// resize the section.
const GEO_VIEW_HEIGHT = 640;

/**
 * Geographic Coverage section. Same editorial layout as the AS section:
 *   1. Provider tab strip
 *   2. Stat tile row
 *   3. View tabs (Map | List)
 *   4. Active visualisation
 */
export function SectionGeo({
  feedName,
  feed,
}: {
  feedName: string;
  feed: FeedMetadata;
}) {
  const accent = useCategoryAccent(feed.category);
  const providersQuery = useQuery(geoProvidersOptions(feedName));

  const providers = providersQuery.data ?? [];

  const dataQueries = useQueries({
    queries: providers.map((p) => geoFeedOptions(feedName, p.name)),
  });

  const dataByProvider: Record<string, CountryComparisonPayload | undefined> = {};
  const queryByProvider: Record<string, (typeof dataQueries)[number] | undefined> = {};
  providers.forEach((p, i) => {
    dataByProvider[p.name] = dataQueries[i]?.data;
    queryByProvider[p.name] = dataQueries[i];
  });

  // Discover the authoritative bogon source name from the backend
  // instead of hardcoding "rfc_reserved" — the YAML config decides
  // which source is the IETF baseline, the engine flags it as
  // `authoritative: true`, and we read the flag. Renaming the source
  // in the YAML keeps the geo section working with no code change.
  const bogonProvidersQuery = useQuery(bogonProvidersOptions(feedName));
  const authoritativeBogon = (bogonProvidersQuery.data ?? []).find(
    (p) => p.authoritative,
  );
  const bogonQuery = useQuery({
    ...bogonFeedOptions(feedName, authoritativeBogon?.name ?? ""),
    enabled: !!authoritativeBogon,
    retry: false,
  });
  const rfcReserved = bogonQuery.data?.bogon_ips ?? 0;

  const [selectedProvider, setSelectedProvider] = useState<string>("");
  const active = providers.some((p) => p.name === selectedProvider)
    ? selectedProvider
    : (providers[0]?.name ?? "");
  const [view, setView] = useState<"map" | "table">("map");

  if (providersQuery.isLoading) {
    return (
      <DetailSection
        eyebrow="Coverage"
        title="Geographic distribution"
        lede="Loading…"
        icon={Globe2}
        accentColor={accent}
      >
        <div className="h-96 animate-pulse bg-muted/40" />
      </DetailSection>
    );
  }
  if (providers.length === 0) {
    return (
      <DetailSection
        eyebrow="Coverage"
        title="Geographic distribution"
        lede="No country attribution providers are configured for this feed."
        icon={Globe2}
        accentColor={accent}
      >
        <div className="border border-border bg-card py-16 text-center text-sm text-muted-foreground">
          No geolocation provider is configured for public country attribution.
        </div>
      </DetailSection>
    );
  }

  // Normalise the API shape: zero-coverage feeds may return an object
  // with total_mapped:0 and no countries key, so always pass through a
  // payload with a guaranteed countries array. Also defends against
  // intermediate loading states.
  const activeProvider = providers.find((provider) => provider.name === active);
  const activeQuery = queryByProvider[active];
  const raw = dataByProvider[active];
  const safePayload = raw
    ? { ...raw, countries: raw.countries ?? [], total_mapped: raw.total_mapped ?? 0 }
    : undefined;
  const activeLabel = activeProvider?.label || activeProvider?.name || active;
  const providerError =
    activeQuery?.isError && activeQuery.error instanceof Error
      ? activeQuery.error.message
      : null;
  const providerLoading = Boolean(activeQuery?.isLoading);
  const providerEmpty =
    !providerLoading &&
    !providerError &&
    !!safePayload &&
    safePayload.total_mapped === 0 &&
    safePayload.countries.length === 0;

  return (
    <DetailSection
      eyebrow="Coverage"
      title="Where they live on the map"
      icon={Globe2}
      accentColor={accent}
      lede="Which countries host the addresses in this list, attributed by the configured geolocation providers. Switch providers to compare."
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
          Country attribution from this provider has not finished loading yet.
          The other provider tabs remain available.
        </DetailNotice>
      )}
      {providerError && (
        <DetailNotice
          title={`${activeLabel} could not be loaded`}
          tone="danger"
          className="mt-8"
        >
          This provider is configured, but its per-feed country payload could
          not be loaded. This is different from "no country data for this
          feed".
          <div className="mt-2 font-mono text-xs text-foreground/80">
            {providerError}
          </div>
        </DetailNotice>
      )}
      {providerEmpty && (
        <DetailNotice title={`${activeLabel} found no attributable countries`} className="mt-8">
          This feed has no country-attributed IPs under this provider. That may
          mean the feed has no attributable public IPs, or simply that this
          provider could not map them.
        </DetailNotice>
      )}

      {!providerLoading && !providerError && safePayload && (
        <div className="mt-10">
          <GeoStats payload={safePayload} feedIPs={feed.ips} rfcReserved={rfcReserved} />
        </div>
      )}

      {!providerLoading && !providerError && (
        <div className="mt-12">
          <div className="mb-6 flex items-center justify-between">
            <div className="eyebrow">Visualisation</div>
            <ViewTabBar>
              <ViewTab label="Map" active={view === "map"} onClick={() => setView("map")} />
              <ViewTab
                label="List"
                active={view === "table"}
                onClick={() => setView("table")}
              />
            </ViewTabBar>
          </div>
          {view === "map" ? (
            <GeoMap payload={safePayload} feedIPs={feed.ips} height={GEO_VIEW_HEIGHT} />
          ) : (
            <CountryTable payload={safePayload} />
          )}
        </div>
      )}
    </DetailSection>
  );
}

/* -------------------------------------------------------------------------- */

function GeoStats({
  payload,
  feedIPs,
  rfcReserved,
}: {
  payload: CountryComparisonPayload | undefined;
  feedIPs: number;
  rfcReserved: number;
}) {
  if (!payload) return null;
  const countries = payload.countries ?? [];
  if (feedIPs <= 0) return null;
  const mapped = Math.min(payload.total_mapped ?? 0, Math.max(0, feedIPs - rfcReserved));
  const unmapped = Math.max(0, feedIPs - mapped - rfcReserved);
  const pct = (n: number) => ((n / feedIPs) * 100).toFixed(2) + "%";
  return (
    <StatRow>
      <StatTile label="Distinct countries" value={formatNum(countries.length)} accent />
      <StatTile label="Mapped" value={formatIPs(mapped)} caption={pct(mapped) + " of feed"} />
      <StatTile label="RFC reserved" value={formatIPs(rfcReserved)} caption={pct(rfcReserved) + " of feed"} />
      <StatTile label="Unmapped" value={formatIPs(unmapped)} caption={pct(unmapped) + " of feed"} />
    </StatRow>
  );
}

/* -------------------------------------------------------------------------- */

type CountryEntry = NonNullable<CountryComparisonPayload["countries"]>[number];

/**
 * ISO 3166-1 alpha-2 → English country name. Uses the browser's
 * built-in `Intl.DisplayNames` so we ship zero hardcoded country
 * tables AND the data stays consistent with the same names the
 * browser uses elsewhere (the language picker, the OS locale, etc).
 *
 * Falls back to the raw code for non-standard or unknown values
 * (Kosovo's "XK", "EU", custom region codes — `Intl.DisplayNames`
 * sometimes returns the same string back unchanged).
 *
 * The instance is created at module load time once because
 * constructing `Intl.DisplayNames` is non-trivial. Wrapped in a
 * try/catch because some older browsers do not have it.
 */
const countryNameLookup = (() => {
  try {
    const dn = new Intl.DisplayNames(["en"], { type: "region" });
    return (code: string): string => {
      if (!code) return "";
      try {
        const name = dn.of(code.toUpperCase());
        return name && name !== code.toUpperCase() ? name : code;
      } catch {
        return code;
      }
    };
  } catch {
    // Pre-2021 browser: `Intl.DisplayNames` is not supported. Fall
    // back to the bare code so the table is at least consistent.
    return (code: string) => code;
  }
})();

function CountryTable({ payload }: { payload: CountryComparisonPayload | undefined }) {
  const countries = payload?.countries ?? [];
  if (countries.length === 0) {
    return (
      <div className="py-16 text-center text-sm text-muted-foreground">
        No country data for this provider.
      </div>
    );
  }
  const total =
    payload?.total_mapped ||
    countries.reduce((acc, c) => acc + c.value, 0) ||
    1;

  const columns: DataTableColumn<CountryEntry>[] = [
    {
      key: "name",
      label: "Country",
      // The backend never populates `name` on country entries —
      // every record only carries the ISO2 code. Resolve the
      // English country name from the browser's `Intl.DisplayNames`
      // so the table actually shows readable names instead of the
      // code twice. Sort uses the resolved name too so alphabetical
      // ordering is by country name, not by code.
      sortValue: (row) => countryNameLookup(row.code) || row.code || "",
      render: (row) => countryNameLookup(row.code) || row.code || "",
    },
    {
      key: "code",
      label: "Code",
      sortValue: (row) => row.code || "",
      render: (row) => (
        <span className="font-mono text-muted-foreground">{row.code || ""}</span>
      ),
    },
    {
      key: "value",
      label: "IPs",
      align: "right",
      sortValue: (row) => row.value,
      render: (row) => formatIPs(row.value),
    },
    {
      key: "percent",
      label: "% of mapped",
      align: "right",
      sortValue: (row) => row.value / total,
      render: (row) => (
        <span className="text-muted-foreground">
          {((row.value / total) * 100).toFixed(2)}%
        </span>
      ),
    },
  ];

  return (
    <DataTable
      rows={countries}
      columns={columns}
      rowKey={(row) => row.code}
      initialSortKey="value"
      initialSortDir="desc"
      exportFilename="country-distribution"
      searchPlaceholder="Filter by country or code…"
      viewportHeight={GEO_VIEW_HEIGHT}
    />
  );
}

// Keep types accessible for callers that pass through.
export type { GeoProvider };
