import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { Ban } from "lucide-react";
import type {
  BogonFeedPayload,
  BogonProvider,
  ComparisonRow,
} from "@/lib/api-types";
import {
  bogonFeedOptions,
  bogonProvidersOptions,
  comparisonOptions,
} from "@/lib/queries/feed";
import { useCategoryAccent } from "@/lib/categories";
import {
  MinimalTable,
  MinimalTableBody,
  MinimalTableCell,
  MinimalTableHead,
  MinimalTableHeader,
  MinimalTableRow,
} from "@/components/editorial/minimal-table";
import { DetailSection, DetailSubsection } from "./section";
import { FeedRef } from "./feed-ref";
import { useFeedRefDescriptorMap } from "./feed-ref-descriptor";
import { formatIPs } from "@/lib/utils";

/**
 * Bogons section. Two subsections stacked:
 *   1. Authoritative — the bogon source the backend tags as
 *      `authoritative: true` (the RFC reserved baseline). The
 *      frontend never names this source — the YAML config drives
 *      which source carries the IETF reservations, the engine
 *      surfaces the flag in the provider listing, and we read it.
 *   2. Cross-reference — every OTHER bogon source listed in the
 *      provider response, with labels and names coming straight
 *      from the YAML. Adding a new third-party bogon source to
 *      `configs/firehol/` makes it appear here automatically;
 *      removing one removes it. There is no hardcoded list of
 *      tracked third-party feeds in this file anymore.
 */
export function SectionBogons({
  feedName,
  feedIPs,
  category,
}: {
  feedName: string;
  feedIPs: number;
  category?: string | null;
}) {
  const accent = useCategoryAccent(category);
  const providersQuery = useQuery(bogonProvidersOptions(feedName));

  // Identify the authoritative provider (RFC reserved baseline) and
  // the third-party providers from the same response. Both come from
  // the YAML config — the backend just tags one with `authoritative`.
  const { authoritative, thirdParty } = useMemo(() => {
    const providers = providersQuery.data ?? [];
    return {
      authoritative: providers.find((p) => p.authoritative),
      thirdParty: providers.filter((p) => !p.authoritative),
    };
  }, [providersQuery.data]);

  // Per-feed payload for the authoritative source. Disabled until we
  // know which source name to ask for.
  const rfcQuery = useQuery({
    ...bogonFeedOptions(feedName, authoritative?.name ?? ""),
    enabled: !!authoritative,
    retry: false,
  });

  const comparisonQuery = useQuery(comparisonOptions(feedName));

  return (
    <DetailSection
      eyebrow="Bogons"
      title="Private and Unassigned IP Address Space"
      lede='RFC-reserved address space plus public "bogons" cross-checks shown for comparison.'
      icon={Ban}
      accentColor={accent}
    >
      <DetailSubsection
        title={authoritative?.label || "Authoritative bogons (RFC reserved)"}
        description="The IETF-defined reserved IPv4 ranges — private space, loopback, link-local, multicast, documentation, and future-use. Fixed forever by RFC."
      >
        <RfcBogonBlock
          data={rfcQuery.data}
          loading={providersQuery.isLoading || rfcQuery.isLoading}
          missing={!providersQuery.isLoading && !authoritative}
        />
      </DetailSubsection>
      <DetailSubsection
        title="Cross-reference with third-party bogon lists"
        description='Public "bogons" lists add unallocated IANA space to the RFC baseline. They are useful cross-validation, but they become stale as IANA allocates new blocks — so they are shown for comparison, not as authority.'
      >
        <ThirdPartyBogonTable
          providers={thirdParty}
          comparison={comparisonQuery.data ?? []}
          feedIPs={feedIPs}
          loading={providersQuery.isLoading || comparisonQuery.isLoading}
        />
      </DetailSubsection>
    </DetailSection>
  );
}

function RfcBogonBlock({
  data,
  loading,
  missing,
}: {
  data: BogonFeedPayload | undefined;
  loading: boolean;
  missing: boolean;
}) {
  if (loading) return <div className="h-24 animate-pulse bg-muted/40" />;
  if (missing) {
    return (
      <p className="text-base text-muted-foreground">
        No authoritative bogon source is configured.
      </p>
    );
  }
  if (!data || data.bogon_ips === 0) {
    return (
      <p className="text-base text-muted-foreground">
        None of this feed's IPs fall in IETF-reserved ranges.
      </p>
    );
  }
  return (
    <div>
      <div className="mb-10 flex flex-col gap-3 border-l-[3px] border-primary pl-6">
        <div className="eyebrow">IPs in IETF-reserved ranges</div>
        <div className="num display-title text-primary">{formatIPs(data.bogon_ips)}</div>
        <div className="text-[15px] text-muted-foreground">
          {(data.percent || 0).toFixed(2)}% of this feed
        </div>
      </div>
      {data.by_range && data.by_range.length > 0 && (
        <MinimalTable>
          <MinimalTableHead>
            <MinimalTableHeader>CIDR</MinimalTableHeader>
            <MinimalTableHeader>Reserved for</MinimalTableHeader>
            <MinimalTableHeader>RFC</MinimalTableHeader>
            <MinimalTableHeader align="right">IPs</MinimalTableHeader>
          </MinimalTableHead>
          <MinimalTableBody>
            {data.by_range.map((row) => (
              <MinimalTableRow key={row.cidr}>
                <MinimalTableCell mono>{row.cidr}</MinimalTableCell>
                <MinimalTableCell>{row.name}</MinimalTableCell>
                <MinimalTableCell muted>{row.rfc || ""}</MinimalTableCell>
                <MinimalTableCell align="right" num>{formatIPs(row.count)}</MinimalTableCell>
              </MinimalTableRow>
            ))}
          </MinimalTableBody>
        </MinimalTable>
      )}
    </div>
  );
}

function ThirdPartyBogonTable({
  providers,
  comparison,
  feedIPs,
  loading,
}: {
  providers: BogonProvider[];
  comparison: ComparisonRow[];
  feedIPs: number;
  loading: boolean;
}) {
  const refMap = useFeedRefDescriptorMap();
  if (loading) return <div className="h-24 animate-pulse bg-muted/40" />;

  // Provider name → display label, sourced entirely from the YAML
  // via the backend's BogonProviders endpoint. The previous version
  // of this component had a hardcoded `THIRD_PARTY_LABELS` map of
  // five well-known feed names; that map silently filtered out any
  // bogon source the curator added to the YAML afterwards. Now the
  // displayed list IS the configured list, with no code change
  // required to add or remove sources.
  const labelByName = new Map<string, string>();
  for (const p of providers) {
    labelByName.set(p.name, p.label || p.name);
  }

  const rows = comparison
    .filter((r) => labelByName.has(r.name) && r.common > 0)
    .map((r) => ({
      ...r,
      label: labelByName.get(r.name) || r.name,
      pct: feedIPs > 0 ? (r.common / feedIPs) * 100 : 0,
    }))
    .sort((a, b) => b.common - a.common);

  if (rows.length === 0) {
    return (
      <p className="text-base text-muted-foreground">
        No overlap with any tracked third-party bogon list.
      </p>
    );
  }
  return (
    <MinimalTable>
      <MinimalTableHead>
        <MinimalTableHeader>List</MinimalTableHeader>
        <MinimalTableHeader align="right">Overlap</MinimalTableHeader>
        <MinimalTableHeader align="right">% of feed</MinimalTableHeader>
      </MinimalTableHead>
      <MinimalTableBody>
        {rows.map((row) => (
          <MinimalTableRow key={row.name}>
            <MinimalTableCell>
              <FeedRef
                name={row.name}
                feed={refMap.get(row.name)}
                className="hover:text-primary"
              >
                {row.label}
              </FeedRef>
            </MinimalTableCell>
            <MinimalTableCell align="right" num>{formatIPs(row.common)}</MinimalTableCell>
            <MinimalTableCell align="right" num muted>{row.pct.toFixed(2)}%</MinimalTableCell>
          </MinimalTableRow>
        ))}
      </MinimalTableBody>
    </MinimalTable>
  );
}
