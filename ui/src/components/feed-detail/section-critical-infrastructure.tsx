import { Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { ShieldAlert } from "lucide-react";
import type {
  CriticalASNContextMatch,
  CriticalInfrastructureOverlap,
  CriticalInfrastructureTierSummary,
} from "@/lib/api-types";
import {
  criticalAggregateOptions,
  criticalProvidersOptions,
} from "@/lib/queries/feed";
import { useCategoryAccent } from "@/lib/categories";
import { DataTable, type DataTableColumn } from "@/components/editorial/data-table";
import { StatRow, StatTile } from "@/components/editorial/stat-row";
import { formatIPs, formatNum } from "@/lib/utils";
import { DetailNotice, DetailSection, DetailSubsection } from "./section";

export function SectionCriticalInfrastructure({
  feedName,
  family,
  feedIPs,
  isReferenceFeed = false,
  isProviderContext = false,
  category,
}: {
  feedName: string;
  family?: string;
  feedIPs: number;
  isReferenceFeed?: boolean;
  isProviderContext?: boolean;
  category?: string | null;
}) {
  const accent = useCategoryAccent(category);
  const isIPv6 = family === "ipv6";
  const providersQuery = useQuery({
    ...criticalProvidersOptions(feedName),
    retry: false,
  });
  const aggregateQuery = useQuery({
    ...criticalAggregateOptions(feedName),
    enabled: !isReferenceFeed && !isProviderContext && !isIPv6,
    retry: false,
  });

  const providers = providersQuery.data ?? [];
  const referenceProvider = providers.find((provider) => provider.name === feedName);
  const payload = aggregateQuery.data;
  const loading = providersQuery.isLoading || aggregateQuery.isLoading;
  // The public API is cache-first: a 404 here means the artifact has never
  // been published for this feed yet (either because the feed is new, or
  // because no critical-overlap reference set has run against it). The
  // public surface does NOT distinguish between "never built" and "older
  // catalog identity" — both are simply "not yet published". Drift detection
  // is the admin integrity path's concern, not a user-facing concept.
  const aggregateMissing =
    !isReferenceFeed && !isProviderContext && errorStatus(aggregateQuery.error) === 404;

  return (
    <DetailSection
      eyebrow="Operational risk"
      title="Critical infrastructure overlap"
      icon={ShieldAlert}
      accentColor={accent}
      lede={
        <>
          Configured IPv4 reference feeds for operator-sensitive infrastructure
          are checked against this feed before publication. See the{" "}
          <Link to="/methodology/infrastructure-asns" className="text-primary hover:text-foreground">
            methodology
          </Link>
          .
        </>
      }
    >
      {loading && <div className="h-36 animate-pulse bg-muted/40" />}

      {!loading && providersQuery.isError && (
        <DetailNotice title="Critical-infrastructure providers could not be loaded" tone="danger">
          The provider catalog endpoint failed. The feed page will not infer or
          recompute this data in the browser.
        </DetailNotice>
      )}

      {!loading && !providersQuery.isError && providers.length === 0 && (
        <DetailNotice title="No critical-infrastructure reference feeds are configured">
          The backend did not expose any reference feeds with
          <span className="font-mono text-foreground"> use: critical_infrastructure</span>.
        </DetailNotice>
      )}

      {!loading && !providersQuery.isError && isReferenceFeed && (
        <DetailNotice title="This feed is a critical-infrastructure reference feed">
          {referenceProvider ? (
            <>
              It is tagged as <span className="font-mono text-foreground">{referenceProvider.tier}</span>{" "}
              / <span className="font-mono text-foreground">{referenceProvider.role}</span>.{" "}
              {referenceProvider.rationale}
            </>
          ) : (
            <>This feed is tagged for critical-infrastructure overlap checks.</>
          )}
        </DetailNotice>
      )}

      {!loading && !providersQuery.isError && isProviderContext && (
        <DetailNotice title="This feed is provider context">
          Broad provider and customer-hosting ranges are published for operator
          context, but they are not used as critical-infrastructure warning
          truth. Use them to understand possible collateral impact, not to judge
          feed quality.
        </DetailNotice>
      )}

      {!loading && !providersQuery.isError && !isReferenceFeed && !isProviderContext && isIPv6 && (
        <DetailNotice title="Critical-infrastructure overlap is IPv4-only in this release">
          This feed is IPv6, so no critical-infrastructure overlap artifact is
          expected yet.
        </DetailNotice>
      )}

      {!loading && !providersQuery.isError && aggregateMissing && (
        <DetailNotice title="Overlap artifacts are not published yet">
          The configured reference feeds exist, but this feed does not have a
          published critical-infrastructure overlap artifact yet.
        </DetailNotice>
      )}

      {!loading && !isReferenceFeed && !isProviderContext && !aggregateMissing && aggregateQuery.isError && (
        <DetailNotice title="Critical-infrastructure overlap could not be loaded" tone="danger">
          The aggregate artifact endpoint failed. Public pages do not recompute
          overlap data on demand.
        </DetailNotice>
      )}

      {!loading && !isReferenceFeed && !isProviderContext && payload && (
        <>
          {payload.complete === false && (
            <DetailNotice title="Critical-infrastructure overlap is incomplete" tone="warning">
              {payload.missing_providers && payload.missing_providers.length > 0 ? (
                <>
                  These configured reference feeds were unavailable during the
                  last overlap build:{" "}
                  <span className="font-mono text-foreground">
                    {payload.missing_providers.map((provider) => provider.name).join(", ")}
                  </span>
                  . Treat zero-overlap results on this page as incomplete.
                </>
              ) : (
                <>One or more configured reference feeds were unavailable during the last overlap build.</>
              )}
            </DetailNotice>
          )}
          <CriticalInfrastructureStats
            feedIPs={payload.feed_ips || feedIPs}
            criticalIPs={payload.critical_ips}
            percent={payload.percent}
            providerCount={providers.length}
            tiers={payload.tiers ?? []}
          />
          {payload.critical_ips > 0 ? (
            <DetailSubsection
              title="Matched reference feeds"
              description="Only reference feeds with a positive overlap are shown here. Rows are ordered by criticality first, then by matched IPs inside each tier."
            >
              <CriticalInfrastructureTable rows={payload.providers ?? []} />
            </DetailSubsection>
          ) : (
            <DetailNotice title="No configured critical infrastructure overlaps this feed" className="mt-10">
              {payload.complete === false ? (
                <>
                  No loaded reference feed matched this feed. Because the
                  overlap artifact is incomplete, this is not evidence that all
                  configured reference feeds were checked.
                </>
              ) : (
                <>
                  This means the currently configured reference feeds did not match
                  any IPs in this feed. It does not prove the feed is safe to block.
                </>
              )}
            </DetailNotice>
          )}
          {payload.asn_context && (payload.asn_context.matches?.length ?? 0) > 0 && (
            <DetailSubsection
              title="Matched ASN context"
              description={`Secondary signal from ${payload.asn_context.provider || "the configured ASN provider"}. These rows do not increase the reference-feed overlap count.`}
            >
              <CriticalASNContextTable rows={payload.asn_context.matches ?? []} />
            </DetailSubsection>
          )}
        </>
      )}
    </DetailSection>
  );
}

function CriticalInfrastructureStats({
  feedIPs,
  criticalIPs,
  percent,
  providerCount,
  tiers,
}: {
  feedIPs: number;
  criticalIPs: number;
  percent: number;
  providerCount: number;
  tiers: CriticalInfrastructureTierSummary[];
}) {
  const byTier = new Map(tiers.map((tier) => [tier.tier, tier]));
  const hard = byTier.get("hard");
  const soft = byTier.get("soft");
  const contextual = byTier.get("contextual");
  const tiersHit = tiers.filter((tier) => (tier.critical_ips ?? 0) > 0).length;
  return (
    <div className="space-y-4">
      <StatRow>
        <StatTile
          label="Matched IPs"
          value={formatIPs(criticalIPs)}
          caption={`${(percent || 0).toFixed(2)}% of feed`}
          accent={criticalIPs > 0}
        />
        <StatTile label="Feed size" value={formatIPs(feedIPs)} />
        <StatTile label="Reference feeds" value={formatNum(providerCount)} />
        <StatTile label="Tiers hit" value={formatNum(tiersHit)} caption="of 3" />
      </StatRow>
      <StatRow cols={3}>
        <CriticalTierTile
          label="Hard"
          tier={hard}
          emptyCaption="no hard-tier overlap"
          hitCaption="investigate immediately"
        />
        <CriticalTierTile
          label="Soft"
          tier={soft}
          emptyCaption="no soft-tier overlap"
          hitCaption="review with service context"
        />
        <CriticalTierTile
          label="Contextual"
          tier={contextual}
          emptyCaption="no contextual overlap"
          hitCaption="policy-dependent"
        />
      </StatRow>
    </div>
  );
}

function CriticalTierTile({
  label,
  tier,
  emptyCaption,
  hitCaption,
}: {
  label: string;
  tier?: CriticalInfrastructureTierSummary;
  emptyCaption: string;
  hitCaption: string;
}) {
  const ips = tier?.critical_ips ?? 0;
  const caption =
    ips > 0
      ? `${(tier?.percent ?? 0).toFixed(2)}% / ${tier?.providers ?? 0} provider(s), ${hitCaption}`
      : emptyCaption;
  return (
    <StatTile
      label={label}
      value={formatIPs(ips)}
      caption={caption}
      accent={ips > 0 && label === "Hard"}
    />
  );
}

function CriticalInfrastructureTable({
  rows,
}: {
  rows: CriticalInfrastructureOverlap[];
}) {
  const columns: DataTableColumn<CriticalInfrastructureOverlap>[] = [
    {
      key: "provider",
      label: "Reference feed",
      sortValue: (row) => row.provider.label || row.provider.name,
      searchValue: (row) =>
        [
          row.provider.name,
          row.provider.label,
          row.provider.role,
          row.provider.rationale,
        ]
          .filter(Boolean)
          .join(" "),
      render: (row) => (
        <div>
          <div className="font-medium text-foreground">
            {row.provider.label || row.provider.name}
          </div>
          <div className="mt-0.5 font-mono text-xs text-muted-foreground">
            {row.provider.name}
          </div>
        </div>
      ),
    },
    {
      key: "tier",
      label: "Tier",
      sortValue: (row) => row.provider.tier,
      compare: compareCriticalInfrastructurePriority,
      render: (row) => <TierLabel tier={row.provider.tier} />,
    },
    {
      key: "role",
      label: "Role",
      sortValue: (row) => row.provider.role,
      render: (row) => (
        <span className="text-muted-foreground">
          {row.provider.role.replace(/_/g, " ")}
        </span>
      ),
    },
    {
      key: "critical_ips",
      label: "IPs",
      align: "right",
      sortValue: (row) => row.critical_ips,
      render: (row) => formatIPs(row.critical_ips),
    },
    {
      key: "percent",
      label: "% of feed",
      align: "right",
      sortValue: (row) => row.percent,
      render: (row) => (
        <span className="text-muted-foreground">
          {(row.percent || 0).toFixed(2)}%
        </span>
      ),
    },
    {
      key: "rationale",
      label: "Why it matters",
      sortValue: (row) => row.provider.rationale,
      render: (row) => (
        <span className="text-muted-foreground">{row.provider.rationale}</span>
      ),
    },
  ];

  return (
    <DataTable
      rows={rows}
      columns={columns}
      rowKey={(row) => row.provider.name}
      initialSortKey="tier"
      initialSortDir="asc"
      exportFilename="critical-infrastructure-overlap"
      searchPlaceholder="Filter by provider, tier, or role..."
      maxHeight={420}
    />
  );
}

function CriticalASNContextTable({
  rows,
}: {
  rows: CriticalASNContextMatch[];
}) {
  const columns: DataTableColumn<CriticalASNContextMatch>[] = [
    {
      key: "asn",
      label: "ASN",
      sortValue: (row) => row.asn,
      searchValue: (row) => [`AS${row.asn}`, row.name, row.role, row.rationale].join(" "),
      render: (row) => (
        <div>
          <div className="font-medium text-foreground">AS{row.asn}</div>
          <div className="mt-0.5 text-xs text-muted-foreground">{row.name}</div>
        </div>
      ),
    },
    {
      key: "tier",
      label: "Tier",
      sortValue: (row) => row.tier,
      compare: compareCriticalASNContextPriority,
      render: (row) => <TierLabel tier={row.tier} />,
    },
    {
      key: "role",
      label: "Role",
      sortValue: (row) => row.role,
      render: (row) => (
        <span className="text-muted-foreground">
          {row.role.replace(/_/g, " ")}
        </span>
      ),
    },
    {
      key: "ips",
      label: "IPs",
      align: "right",
      sortValue: (row) => row.ips,
      render: (row) => formatIPs(row.ips),
    },
    {
      key: "percent",
      label: "% of feed",
      align: "right",
      sortValue: (row) => row.percent,
      render: (row) => (
        <span className="text-muted-foreground">
          {(row.percent || 0).toFixed(2)}%
        </span>
      ),
    },
    {
      key: "rationale",
      label: "Why it matters",
      sortValue: (row) => row.rationale,
      render: (row) => (
        <span className="text-muted-foreground">{row.rationale}</span>
      ),
    },
  ];

  return (
    <DataTable
      rows={rows}
      columns={columns}
      rowKey={(row) => String(row.asn)}
      initialSortKey="tier"
      initialSortDir="asc"
      exportFilename="critical-asn-context"
      searchPlaceholder="Filter by ASN, tier, or role..."
      maxHeight={360}
    />
  );
}

function TierLabel({ tier }: { tier: string }) {
  const color =
    tier === "hard"
      ? "border-destructive/50 text-destructive"
      : tier === "soft"
        ? "border-amber-500/60 text-amber-600"
        : "border-border text-muted-foreground";
  return (
    <span className={`inline-flex rounded-sm border px-2 py-1 text-xs font-medium ${color}`}>
      {tier}
    </span>
  );
}

function tierRank(tier: string) {
  switch (tier) {
    case "hard":
      return 0;
    case "soft":
      return 1;
    case "contextual":
      return 2;
    default:
      return 3;
  }
}

function compareCriticalASNContextPriority(
  left: CriticalASNContextMatch,
  right: CriticalASNContextMatch,
) {
  const tierDelta = tierRank(left.tier) - tierRank(right.tier);
  if (tierDelta !== 0) return tierDelta;

  const ipDelta = right.ips - left.ips;
  if (ipDelta !== 0) return ipDelta;

  const percentDelta = right.percent - left.percent;
  if (percentDelta !== 0) return percentDelta;

  return left.asn - right.asn;
}

function compareCriticalInfrastructurePriority(
  left: CriticalInfrastructureOverlap,
  right: CriticalInfrastructureOverlap,
) {
  const tierDelta = tierRank(left.provider.tier) - tierRank(right.provider.tier);
  if (tierDelta !== 0) return tierDelta;

  const ipDelta = right.critical_ips - left.critical_ips;
  if (ipDelta !== 0) return ipDelta;

  const percentDelta = right.percent - left.percent;
  if (percentDelta !== 0) return percentDelta;

  const leftName = left.provider.label || left.provider.name;
  const rightName = right.provider.label || right.provider.name;
  return leftName.localeCompare(rightName);
}

function errorStatus(error: unknown): number | undefined {
  if (typeof error !== "object" || error === null || !("status" in error)) {
    return undefined;
  }
  const status = (error as { status?: unknown }).status;
  return typeof status === "number" ? status : undefined;
}
