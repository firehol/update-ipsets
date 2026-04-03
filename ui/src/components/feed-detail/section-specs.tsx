import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import type { FeedMetadata, FeedSummary } from "@/lib/api-types";
import {
  feedHealthDescription,
  feedHealthLabel,
  formatMinutes,
  thresholdBasisLabel,
} from "@/lib/feed-health";
import { feedsOptions } from "@/lib/queries/catalog";
import { Settings2 } from "lucide-react";
import { useCategoryAccent } from "@/lib/categories";
import { DetailSection, DetailSubsection } from "./section";
import { FeedRef } from "./feed-ref";
import { safeExternalUrl } from "@/lib/safe-url";
import { formatFreq, formatIPs, formatNum, timeAgo } from "@/lib/utils";

/**
 * Technical specifications — public fact sheet in three groups:
 *
 *   1. Identification — origin, maintainer, licensing, provenance.
 *   2. Data — the technical shape of the canonical feed body.
 *   3. Updates — cadence, timestamps, health, and failures.
 *
 * The older Access / Processing groups were explicitly pruned from the
 * public spec table. Keep this section factual and public-facing; do not
 * reintroduce internal pipeline plumbing here without explicit direction.
 */
export function SectionSpecs({ feed }: { feed: FeedMetadata }) {
  const accent = useCategoryAccent(feed.category);
  const catalogQuery = useQuery({
    ...feedsOptions(),
    staleTime: 5 * 60 * 1000,
  });
  const summaryByFeed = useMemo(() => {
    const out = new Map<string, FeedSummary>();
    for (const item of catalogQuery.data ?? []) {
      out.set(item.name, item);
    }
    return out;
  }, [catalogQuery.data]);

  return (
    <DetailSection
      eyebrow="Specs"
      title="Technical specifications"
      icon={Settings2}
      accentColor={accent}
      lede="The public fact sheet for this feed: where it comes from, what shape the data has, and how it behaves over time."
    >
      <div className="space-y-12">
        <DetailSubsection title="Identification">
          <SpecGrid rows={identification(feed)} />
        </DetailSubsection>

        {(feed.merge_included?.length ||
          feed.merge_subtracted?.length ||
          feed.merge_excluded?.length) && (
          <DetailSubsection title="Merge composition">
            <MergeCompositionGrid feed={feed} summaryByFeed={summaryByFeed} />
          </DetailSubsection>
        )}

        <DetailSubsection title="Data">
          <SpecGrid rows={dataRows(feed)} />
        </DetailSubsection>

        <DetailSubsection title="Updates">
          <SpecGrid rows={updatesRows(feed)} />
        </DetailSubsection>
      </div>
    </DetailSection>
  );
}

/* -------------------------------------------------------------------------- */

type SpecRow = { label: string; value: React.ReactNode };

function SpecGrid({ rows }: { rows: SpecRow[] }) {
  return (
    <dl className="grid grid-cols-1 gap-x-12 gap-y-0 md:grid-cols-2">
      {rows.map((row) => (
        <div
          key={row.label}
          className="flex items-start justify-between gap-6 border-b border-border py-4 last:border-b-0 md:even:[&:nth-last-child(-n+2)]:border-b-0"
        >
          <dt className="eyebrow whitespace-nowrap">{row.label}</dt>
          <dd className="min-w-0 text-right text-[15px] tabular-nums text-foreground [word-break:break-word]">
            {row.value}
          </dd>
        </div>
      ))}
    </dl>
  );
}

/* -------------------------------------------------------------------------- */

const DASH = <span className="text-muted-foreground">—</span>;

function identification(feed: FeedMetadata): SpecRow[] {
  const maintainerUrl = safeExternalUrl(feed.maintainer_url);
  const sourceUrl = safeExternalUrl(feed.source);
  return [
    { label: "Name", value: <span className="font-mono">{feed.name}</span> },
    { label: "Category", value: feed.category },
    { label: "Provenance", value: provenanceLabel(feed.provenance) },
    { label: "Maintainer", value: feed.maintainer || DASH },
    {
      label: "Homepage",
      value: feed.maintainer_url ? (
        maintainerUrl ? (
          <a
            className="text-primary hover:underline [word-break:break-all]"
            href={maintainerUrl}
            target="_blank"
            rel="noopener noreferrer"
          >
            {feed.maintainer_url}
          </a>
        ) : (
          <span className="[word-break:break-all]">{feed.maintainer_url}</span>
        )
      ) : (
        DASH
      ),
    },
    {
      label: "Source URL",
      value: feed.source ? (
        sourceUrl ? (
          <a
            className="font-mono text-primary hover:underline [word-break:break-all]"
            href={sourceUrl}
            target="_blank"
            rel="noopener noreferrer"
          >
            {feed.source}
          </a>
        ) : (
          <span className="font-mono [word-break:break-all]">
            {feed.source}
          </span>
        )
      ) : (
        DASH
      ),
    },
    { label: "License", value: feed.license || DASH },
    { label: "Attribution", value: feed.attribution || DASH },
    {
      label: "Commit history",
      value: feed.commit_history ? (
        <a
          className="text-primary hover:underline [word-break:break-all]"
          href={feed.commit_history}
          target="_blank"
          rel="noopener noreferrer"
        >
          View revision history
        </a>
      ) : (
        DASH
      ),
    },
    {
      label: "Redistributable",
      value: feed.dont_redistribute ? "no" : "yes",
    },
    {
      label: "Redistribution terms",
      value:
        feed.enrichment?.redistribution.terms ||
        feed.enrichment?.redistribution.attribution_required ||
        DASH,
    },
    {
      label: "Operational feed URLs",
      value: operationalFeedURLsLabel(feed),
    },
  ];
}

function dataRows(feed: FeedMetadata): SpecRow[] {
  return [
    { label: "IP version", value: feed.ipv || DASH },
    {
      label: "Format",
      value: feed.format ? (
        <span className="font-mono">{feed.format}</span>
      ) : (
        DASH
      ),
    },
    { label: "Unique IPs", value: formatIPs(feed.ips) },
    { label: "Entries", value: formatNum(feed.entries) },
    {
      label: "IPs (min — max)",
      value:
        feed.ips_max > 0 ? (
          <>
            {formatIPs(feed.ips_min)} — {formatIPs(feed.ips_max)}
          </>
        ) : (
          DASH
        ),
    },
    {
      label: "Entries (min — max)",
      value:
        feed.entries_max > 0 ? (
          <>
            {formatNum(feed.entries_min)} — {formatNum(feed.entries_max)}
          </>
        ) : (
          DASH
        ),
    },
    {
      label: "Aggregation",
      value: feed.aggregation > 0 ? `${feed.aggregation}m` : DASH,
    },
    {
      label: "Hash",
      value: feed.hash ? (
        <code className="font-mono text-[12px]">{feed.hash}</code>
      ) : (
        DASH
      ),
    },
  ];
}

function updatesRows(feed: FeedMetadata): SpecRow[] {
  return [
    {
      label: "Configured frequency",
      value: feed.frequency > 0 ? formatFreq(feed.frequency) : DASH,
    },
    {
      label: "Average interval",
      value:
        feed.average_update > 0 ? formatMinutes(feed.average_update) : DASH,
    },
    {
      label: "Min interval",
      value: feed.min_update > 0 ? formatMinutes(feed.min_update) : DASH,
    },
    {
      label: "Max interval",
      value: feed.max_update > 0 ? formatMinutes(feed.max_update) : DASH,
    },
    {
      label: "Health",
      value: feedHealthLabel(feed.health.class),
    },
    {
      label: "Healthy cadence floor",
      value: feed.health.effective_healthy_gap_mins
        ? formatMinutes(feed.health.effective_healthy_gap_mins)
        : DASH,
    },
    {
      label: "Risky cadence",
      value: feed.health.risky_cadence_mins
        ? formatMinutes(feed.health.risky_cadence_mins)
        : DASH,
    },
    {
      label: "Abandoned at",
      value: feed.health.unmaintained_threshold_mins
        ? formatMinutes(feed.health.unmaintained_threshold_mins)
        : DASH,
    },
    {
      label: "Archived after",
      value: feed.health.archival_threshold_mins
        ? formatMinutes(feed.health.archival_threshold_mins)
        : DASH,
    },
    {
      label: "Single-update grace",
      value: feed.health.single_observation_grace_mins
        ? formatMinutes(feed.health.single_observation_grace_mins)
        : DASH,
    },
    {
      label: "Health basis",
      value: thresholdBasisLabel(feed.health.threshold_basis),
    },
    {
      label: "Observed updates",
      value:
        feed.health.observed_updates && feed.health.observed_updates > 0
          ? formatNum(feed.health.observed_updates)
          : DASH,
    },
    {
      label: "Time since last change",
      value: feed.health.time_since_last_change_mins
        ? formatMinutes(feed.health.time_since_last_change_mins)
        : DASH,
    },
    {
      label: "Time since failure",
      value: feed.health.time_since_failure_mins
        ? formatMinutes(feed.health.time_since_failure_mins)
        : DASH,
    },
    {
      label: "Age-based maintenance",
      value: feed.health.exclude_from_unmaintained ? "excluded" : "enabled",
    },
    {
      label: "Health detail",
      value: feedHealthDescription(feed.health),
    },
    {
      label: "Tracked since",
      value: feed.started ? formatDateTime(feed.started) : DASH,
    },
    {
      label: "Last updated",
      value: feed.updated ? (
        <>
          {formatDateTime(feed.updated)}{" "}
          <span className="text-muted-foreground">
            ({timeAgo(feed.updated)})
          </span>
        </>
      ) : (
        DASH
      ),
    },
    {
      label: "Last processed",
      value: feed.processed ? formatDateTime(feed.processed) : DASH,
    },
    {
      label: "Last checked",
      value: feed.checked ? (
        <>
          {formatDateTime(feed.checked)}{" "}
          <span className="text-muted-foreground">
            ({timeAgo(feed.checked)})
          </span>
        </>
      ) : (
        DASH
      ),
    },
    {
      label: "Clock skew",
      value: formatClockSkew(feed.clock_skew),
    },
    {
      label: "Download failures",
      value: feed.errors > 0 ? formatNum(feed.errors) : DASH,
    },
    { label: "Version", value: feed.version ? `v${feed.version}` : DASH },
  ];
}

/* -------------------------------------------------------------------------- */

function formatDateTime(input: number): string {
  // FeedMetadata timestamps are Unix millis, but some fall back to seconds
  // when the daemon serialises cache.Entry directly — normalize both.
  const ms = input < 1e12 ? input * 1000 : input;
  const d = new Date(ms);
  return d
    .toISOString()
    .replace("T", " ")
    .replace(/\.\d+Z$/, " UTC");
}

function formatClockSkew(milliseconds: number): string {
  if (!milliseconds) return "0s";
  const seconds = Math.round(milliseconds / 1000);
  return `${seconds}s`;
}

function MergeCompositionGrid({
  feed,
  summaryByFeed,
}: {
  feed: FeedMetadata;
  summaryByFeed: Map<string, FeedSummary>;
}) {
  const included = feed.merge_included ?? [];
  const subtracted = feed.merge_subtracted ?? [];
  const excluded = feed.merge_excluded ?? [];

  return (
    <div className="grid grid-cols-1 gap-8 md:grid-cols-2 lg:grid-cols-3">
      <div className="space-y-3">
        <div className="eyebrow">Included now</div>
        {included.length === 0 ? (
          <div className="text-sm text-muted-foreground">No inputs are currently included.</div>
        ) : (
          <div className="space-y-2">
            {included.map((input) => (
              <div
                key={input.name}
                className="flex items-center justify-between gap-4 border-b border-border py-2 text-sm last:border-b-0"
              >
                <FeedRef
                  name={input.name}
                  feed={summaryByFeed.get(input.name)}
                  className="font-mono text-foreground hover:text-primary"
                />
                <span className="text-muted-foreground">
                  {input.health_class ?? "healthy"}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="space-y-3">
        <div className="eyebrow">Subtracted now</div>
        {subtracted.length === 0 ? (
          <div className="text-sm text-muted-foreground">No inputs are currently subtracted.</div>
        ) : (
          <div className="space-y-2">
            {subtracted.map((input) => (
              <div
                key={input.name}
                className="flex items-center justify-between gap-4 border-b border-border py-2 text-sm last:border-b-0"
              >
                <FeedRef
                  name={input.name}
                  feed={summaryByFeed.get(input.name)}
                  className="font-mono text-foreground hover:text-primary"
                />
                <span className="text-muted-foreground">
                  {input.health_class ?? "healthy"}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="space-y-3">
        <div className="eyebrow">Excluded now</div>
        {excluded.length === 0 ? (
          <div className="text-sm text-muted-foreground">No inputs are currently excluded.</div>
        ) : (
          <div className="space-y-2">
            {excluded.map((input) => (
              <div
                key={input.name}
                className="flex items-center justify-between gap-4 border-b border-border py-2 text-sm last:border-b-0"
              >
                <FeedRef
                  name={input.name}
                  feed={summaryByFeed.get(input.name)}
                  className="font-mono text-foreground hover:text-primary"
                />
                <span className="text-right text-muted-foreground">
                  {mergeReasonLabel(input.reason)}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function mergeReasonLabel(reason: string | undefined): string {
  switch (reason) {
    case "disabled":
      return "disabled";
    case "archived":
      return "archived";
    case "unmaintained":
      return "unmaintained";
    case "missing_local_feed_body":
      return "missing local feed body";
    default:
      return "excluded";
  }
}

function provenanceLabel(value: FeedMetadata["provenance"]): string {
  switch (value) {
    case "secondary_upstream":
      return "upstream aggregate";
    case "secondary_merge":
      return "merged feed";
    case "secondary_retention":
      return "retention derivative";
    case "primary":
    default:
      return "primary source";
  }
}

function operationalFeedURLsLabel(feed: FeedMetadata): string {
  if (feed.health.class === "archived") {
    return "disabled while archived";
  }
  if (feed.dont_redistribute) {
    return "metadata only";
  }
  if (feed.file || feed.file_local) {
    return "enabled";
  }
  return "not published";
}
