import {
  Activity,
  ChevronDown,
  Clock3,
  GitBranch,
  Microscope,
  MinusCircle,
  Network,
  Sparkles,
  Users,
} from "lucide-react";
import type { FeedMetadata } from "@/lib/api-types";
import type {
  EnrichmentDerivationType,
  EnrichmentDetectionMethod,
  FeedEnrichmentRole,
} from "@/lib/enrichment-types";
import { useCategoryAccent } from "@/lib/categories";
import { safeExternalUrl } from "@/lib/safe-url";
import { DetailSection } from "./section";
import { FeedRef } from "./feed-ref";
import { useFeedRefDescriptorMap } from "./feed-ref-descriptor";
import { MarkdownText } from "./markdown-text";

const METHOD_LABELS: Record<EnrichmentDetectionMethod, string> = {
  honeypot: "Honeypot telemetry",
  network_telescope: "Network telescope",
  active_scanning: "Active scanning",
  user_submission: "User submission",
  malware_analysis: "Malware analysis",
  reputation_aggregation: "Reputation aggregation",
  policy_assignment: "Policy assignment",
  commercial_threat_intel: "Commercial threat intel",
  mixed: "Mixed methods",
  unknown: "Unknown",
};

const DERIVATION_LABELS: Record<EnrichmentDerivationType, string> = {
  original: "Original first-party",
  derivative: "Derived from another feed",
  extraction: "Extracted from a dataset",
  partial_mirror: "Partial mirror",
  aggregate_merge: "Aggregate merge",
  reformat: "Reformatted source",
  fork: "Forked feed",
  unknown: "Unknown",
};

const ROLE_LABELS: Record<string, string> = {
  maintainer: "Maintainer",
  publisher: "Publisher",
  aggregator: "Aggregator",
  source_contributor: "Source contributor",
  original_author: "Original author",
  successor: "Successor",
};

/**
 * "How this feed is built" rendered as an infographic fact-card section.
 * Three big chips up top (Method · Source-of-source · Cadence) make
 * the key facts scannable in one glance. The longer derivation and
 * detection-classification prose live behind a single disclosure so the
 * page does not become a wall of explanation when the chips already
 * carry the headline.
 */
export function SectionMethod({ feed }: { feed: FeedMetadata }) {
  const accent = useCategoryAccent(feed.category);
  const refMap = useFeedRefDescriptorMap();
  const enrichment = feed.enrichment;
  if (!enrichment) return null;
  const primaryMethod = enrichment.detection_classification.primary_method;
  const derivationType = enrichment.derivation.type;
  const cadence =
    enrichment.update_frequency?.human_readable?.trim() ||
    enrichment.update_frequency?.frequency?.trim() ||
    (feed.frequency ? `Every ${feed.frequency} minutes` : "Unknown");
  const allSourceFeeds = enrichment.derivation.source_feeds ?? [];
  // Show only additive entries here. Subtractive entries (carried with
  // relationship: "filtered" + notes about subtraction by the static
  // merge enricher) are surfaced separately below using the
  // authoritative `merge_excluded` metadata.
  const sourceFeeds = allSourceFeeds.filter(
    (s) => s.relationship !== "filtered" || !(s.notes ?? "").toLowerCase().includes("subtract"),
  );
  const mergeExcluded = feed.merge_excluded ?? [];
  const detailsBody =
    enrichment.derivation.description?.trim() ||
    enrichment.detection_classification.description?.trim();
  const roles = (enrichment.roles ?? []).slice(0, 3);

  return (
    <DetailSection
      eyebrow="Method"
      title="How this feed is built"
      icon={Microscope}
      accentColor={accent}
    >
      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        <FactChip
          icon={Sparkles}
          label="Primary method"
          value={METHOD_LABELS[primaryMethod] ?? primaryMethod.replaceAll("_", " ")}
          accent={accent}
        />
        <FactChip
          icon={derivationType === "original" ? Sparkles : GitBranch}
          label="Source of source"
          value={DERIVATION_LABELS[derivationType] ?? derivationType.replaceAll("_", " ")}
          accent={accent}
        />
        <FactChip
          icon={Clock3}
          label="Update cadence"
          value={cadence}
          accent={accent}
        />
      </div>

      {sourceFeeds.length > 0 && (
        <div className="mt-10">
          <div className="flex items-center gap-2 eyebrow">
            <Network className="h-3.5 w-3.5" aria-hidden="true" />
            <span>Built from</span>
          </div>
          <ul className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {sourceFeeds.map((source) => (
              <li
                key={source.identifier}
                className="border-l-[3px] border-border pl-4 text-sm"
              >
                <FeedRef
                  name={source.identifier}
                  feed={refMap.get(source.identifier) ?? { name: source.identifier }}
                  className="font-mono text-[14px] font-semibold text-foreground hover:text-primary"
                >
                  {source.identifier}
                </FeedRef>
                {source.relationship && (
                  <span className="ml-2 text-[10px] uppercase tracking-[0.1em] text-muted-foreground">
                    {source.relationship.replaceAll("_", " ")}
                  </span>
                )}
                {source.notes && (
                  <p className="mt-1 text-[13px] leading-relaxed text-muted-foreground">
                    {source.notes}
                  </p>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}

      {mergeExcluded.length > 0 && (
        <div className="mt-10">
          <div className="flex items-center gap-2 eyebrow">
            <MinusCircle className="h-3.5 w-3.5" aria-hidden="true" />
            <span>Subtracted from the result</span>
          </div>
          <p className="mt-2 max-w-[68ch] text-[13px] leading-relaxed text-muted-foreground">
            IPs that appear in the additive components above but also in any of these feeds are removed before the merge is published.
          </p>
          <ul className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {mergeExcluded.map((entry) => (
              <li
                key={entry.name}
                className="border-l-[3px] border-amber-500/50 pl-4 text-sm"
              >
                <FeedRef
                  name={entry.name}
                  feed={refMap.get(entry.name) ?? { name: entry.name }}
                  className="font-mono text-[14px] font-semibold text-foreground hover:text-primary"
                >
                  {entry.name}
                </FeedRef>
                {entry.reason && (
                  <span className="ml-2 text-[10px] uppercase tracking-[0.1em] text-muted-foreground">
                    {entry.reason.replaceAll("_", " ")}
                  </span>
                )}
                {entry.health_class && entry.health_class !== "healthy" && (
                  <p className="mt-1 text-[12px] leading-relaxed text-muted-foreground">
                    Currently {entry.health_class}; subtraction may be skipped until the feed recovers.
                  </p>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}

      {roles.length > 0 && (
        <div className="mt-10">
          <div className="flex items-center gap-2 eyebrow">
            <Users className="h-3.5 w-3.5" aria-hidden="true" />
            <span>People &amp; organizations behind the feed</span>
          </div>
          <ul className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {roles.map((role, index) => (
              <RoleCard
                key={`${role.role}-${role.name}-${index}`}
                role={role}
                accent={accent}
              />
            ))}
          </ul>
        </div>
      )}

      {detailsBody && (
        <details className="group mt-10 border-l-[3px] pl-5" style={borderStyle(accent)}>
          <summary className="flex cursor-pointer list-none items-center justify-between text-sm font-semibold text-foreground">
            <span className="inline-flex items-center gap-2">
              <Activity className="h-4 w-4" aria-hidden="true" />
              Method details
            </span>
            <ChevronDown className="h-4 w-4 transition-transform group-open:rotate-180" />
          </summary>
          <div className="mt-4 space-y-5">
            <MarkdownText value={enrichment.derivation.description} />
            <MarkdownText value={enrichment.detection_classification.description} />
          </div>
        </details>
      )}
    </DetailSection>
  );
}

function FactChip({
  icon: Icon,
  label,
  value,
  accent,
}: {
  icon: typeof Clock3;
  label: string;
  value: string;
  accent: string | undefined;
}) {
  return (
    <div
      className="border border-border bg-muted/30 p-5"
      style={
        accent
          ? { borderColor: `${accent}55`, backgroundColor: `${accent}0d` }
          : undefined
      }
    >
      <div className="flex items-center gap-2 eyebrow">
        <Icon
          className="h-3.5 w-3.5"
          style={accent ? { color: accent } : undefined}
          aria-hidden="true"
        />
        <span>{label}</span>
      </div>
      <div className="mt-3 font-display text-[20px] font-semibold leading-snug text-foreground">
        {value}
      </div>
    </div>
  );
}

function RoleCard({
  role,
  accent,
}: {
  role: FeedEnrichmentRole;
  accent: string | undefined;
}) {
  const href = safeExternalUrl(role.official_url);
  return (
    <li className="border border-border p-4" style={accent ? { borderColor: `${accent}33` } : undefined}>
      <div className="text-[10px] uppercase tracking-[0.1em] text-muted-foreground">
        {ROLE_LABELS[role.role] ?? role.role.replaceAll("_", " ")}
      </div>
      <div className="mt-1 text-[15px] font-semibold text-foreground">
        {href ? (
          <a
            className="text-foreground hover:text-primary"
            href={href}
            target="_blank"
            rel="noopener noreferrer"
          >
            {role.name}
          </a>
        ) : (
          role.name
        )}
      </div>
      {role.notes && (
        <p className="mt-2 text-[13px] leading-relaxed text-muted-foreground">
          {role.notes}
        </p>
      )}
    </li>
  );
}

function borderStyle(accent: string | undefined) {
  if (!accent) return { borderColor: "hsl(var(--border))" };
  return { borderColor: accent };
}
