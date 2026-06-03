import { useMemo, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { Download, ExternalLink } from "lucide-react";
import { Area, AreaChart } from "recharts";
import type { FeedMetadata } from "@/lib/api-types";
import { CategoryBadge } from "@/components/category-badge";
import { AccentBar } from "@/components/editorial/accent-bar";
import { AutoFitText } from "@/components/editorial/auto-fit-text";
import { HoverTip } from "@/components/editorial/hover-tip";
import { FeedHealthTip } from "@/components/feed-health-tip";
import { feedHealthLabel } from "@/lib/feed-health";
import { useChartTheme } from "@/lib/chart-theme";
import { parseHistoryCSV } from "@/lib/feed-history";
import { historyOptions } from "@/lib/queries/feed";
import { safeExternalUrl } from "@/lib/safe-url";
import { formatFreq, formatIPs, formatNum, timeAgo } from "@/lib/utils";

/**
 * Detail-page hero — the loudest thing on the page.
 *
 *   - Full-bleed dark surface
 *   - Tiny eyebrow (the feed category)
 *   - Massive display number for the IP count, auto-fitted to the
 *     right-column width so it never clips
 *   - Feed name in display font alongside
 *   - Auxiliary stats in a thin row underneath
 *   - A large primary CTA plus an upstream-source secondary action
 *
 * Mirrors the Apple iPhone product page hero structure: huge stat, then
 * supporting facts laid out as a thin grid, then a direct CTA row.
 */
export function FeedHero({ feed }: { feed: FeedMetadata }) {
  const dontRedistribute = !!feed.dont_redistribute;
  const rawFeedAvailable = Boolean(feed.file) && !dontRedistribute;
  const primaryHref = rawFeedAvailable ? `/${feed.file}` : `/${feed.name}.json`;
  const primaryLabel = rawFeedAvailable ? `Download ${feed.file}` : "View metadata";
  const PrimaryIcon = rawFeedAvailable ? Download : ExternalLink;
  const maintainerUrl = safeExternalUrl(feed.maintainer_url);
  const sourceUrl = safeExternalUrl(feed.source);
  const officialName = feed.official_name?.trim();
  const tagline =
    feed.short_description?.trim() || feed.enrichment?.short_description?.trim() || "";
  const roleLabels =
    feed.enrichment?.roles
      .slice(0, 3)
      .map((role) => role.role.replace(/_/g, " "))
      .filter(Boolean) ?? [];
  return (
    <section className="bg-display py-24 text-display-fg md:py-32">
      <div className="page-container">
        <div className="grid grid-cols-1 gap-16 lg:grid-cols-12 lg:gap-12">
          <div className="lg:col-span-7">
            <AccentBar />
            <div className="eyebrow mt-6 text-display-muted">
              {feed.category}
            </div>
            {/* AutoFitText keeps the feed name on a single line and
                scales the font to whatever fits inside the 7-col
                hero column. Short names render at the display-hero
                max (144px); long names like `ri_connect_proxies_30d`
                shrink until they fit without overlapping the stats
                column on the right. */}
            <h1 className="mt-3 font-mono font-bold text-display-fg">
              <AutoFitText text={feed.name} maxPx={144} minPx={40} />
            </h1>
            {officialName && officialName !== feed.name && (
              <p className="mt-4 text-[22px] leading-snug text-display-fg/90">
                {officialName}
              </p>
            )}
            {tagline && (
              <p className="mt-6 max-w-[60ch] text-[20px] leading-snug text-display-fg/80">
                {tagline}
              </p>
            )}
            {/* No inner width cap: the left column is already 7/12 of
                the container (col-span-7 above), so the paragraph
                naturally wraps at a comfortable width without a
                redundant 52ch clamp on top of the column constraint. */}
            <p className="mt-6 text-[15px] leading-relaxed text-display-muted">
              by{" "}
              {maintainerUrl ? (
                <a
                  className="text-display-fg underline-offset-4 hover:underline"
                  href={maintainerUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  {feed.maintainer}
                </a>
              ) : (
                <span className="text-display-fg">{feed.maintainer}</span>
              )}
              {feed.started ? (
                <>
                  {" "}
                  · tracking since{" "}
                  <span className="text-display-fg">{formatStartYear(feed.started)}</span>
                  {" "}
                  · <span className="text-display-fg">{formatTrackingAge(feed.started)}</span>
                </>
              ) : null}
              .
            </p>

            {/* CTAs. The download is the loudest call-to-action on the
                page: a solid primary button for redistributable feeds,
                and a metadata fallback for non-redistributable feeds. */}
            <div className="mt-10 flex flex-wrap items-center gap-4">
              <a
                href={primaryHref}
                {...(rawFeedAvailable ? { download: true } : {})}
                className="inline-flex items-center gap-2 rounded-lg bg-primary px-6 py-3 text-[15px] font-semibold text-primary-foreground shadow-lg shadow-primary/20 transition-all hover:-translate-y-0.5 hover:shadow-xl hover:shadow-primary/30"
              >
                <PrimaryIcon className="h-4 w-4" />
                {primaryLabel}
              </a>
              {sourceUrl && (
                <a
                  href={sourceUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-2 rounded-lg border border-display-border px-6 py-3 text-[14px] font-medium text-display-fg transition-colors hover:border-display-fg/60 hover:bg-display-border/40"
                >
                  <ExternalLink className="h-4 w-4" />
                  Upstream source
                </a>
              )}
            </div>

            {/* Secondary meta row — category pill and redistributable
                status. Visually demoted from the CTAs so the download
                button reads as the primary action. */}
            <div className="mt-6 flex flex-wrap items-center gap-3">
              <CategoryBadge category={feed.category} />
              <HoverTip text={<FeedHealthTip health={feed.health} />}>
                <span className="rounded-sm border border-display-border px-2 py-1 text-[10px] uppercase tracking-[0.1em] text-display-muted">
                  {feedHealthLabel(feed.health.class)}
                </span>
              </HoverTip>
              {feed.health.class === "archived" && (
                <span className="rounded-sm border border-display-border px-2 py-1 text-[10px] uppercase tracking-[0.1em] text-display-muted">
                  raw access disabled
                </span>
              )}
              {dontRedistribute && (
                <span className="rounded-sm border border-display-border px-2 py-1 text-[10px] uppercase tracking-[0.1em] text-display-muted">
                  not redistributable
                </span>
              )}
              {roleLabels.map((role) => (
                <span
                  key={role}
                  className="rounded-sm border border-display-border px-2 py-1 text-[10px] uppercase tracking-[0.1em] text-display-muted"
                >
                  {role}
                </span>
              ))}
            </div>
          </div>

          <HeroRightVisual feed={feed} />
        </div>
      </div>
    </section>
  );
}

function HeroFact({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-white/10 bg-display/45 px-5 py-5 shadow-[0_12px_32px_rgba(0,0,0,0.18)] backdrop-blur-md">
      <div className="eyebrow text-display-muted">{label}</div>
      <div className="num mt-2 font-display text-[20px] font-semibold text-display-fg">
        {value}
      </div>
    </div>
  );
}

function HeroRightVisual({ feed }: { feed: FeedMetadata }) {
  const historyQuery = useQuery({
    ...historyOptions(feed.name),
  });
  const points = useMemo(() => parseHistoryCSV(historyQuery.data), [historyQuery.data]);
  const statusMessage = heroEvolutionStatusMessage(historyQuery, points);
  const uniqueIPs = formatIPs(feed.ips);

  return (
    <div className="lg:col-span-5">
      <div className="relative min-h-[38rem] overflow-hidden rounded-xl border border-display-border/70 bg-white/[0.03] px-6 py-6 sm:px-8 sm:py-8">
        <div className="pointer-events-none absolute inset-0">
          <HeroEvolutionBackground
            points={points}
            loading={historyQuery.isLoading}
            available={!historyQuery.isError && points.length >= 2}
          />
        </div>
        <div className="absolute inset-0 bg-[radial-gradient(circle_at_top_right,rgba(255,255,255,0.12),transparent_38%)]" />
        <div className="absolute inset-0 bg-gradient-to-b from-display/82 via-display/38 to-display/92" />

        <div className="relative z-10 flex h-full flex-col justify-between">
          <div className="max-w-[24rem]">
            <div className="eyebrow text-display-muted">All-time evolution</div>
            <p className="mt-4 text-sm leading-relaxed text-display-muted">
              {statusMessage}
            </p>
          </div>

          <div className="mt-12">
            <div className="border-t border-display-border pt-10">
              <div className="eyebrow text-display-muted">
                Unique IPs tracked
              </div>
              <div className="mt-4">
                <AutoFitText
                  text={uniqueIPs}
                  maxPx={112}
                  minPx={36}
                  className="num font-display font-semibold text-display-fg"
                />
              </div>
              <div className="mt-3 text-[15px] text-display-muted">
                across {formatNum(feed.entries)} entries
              </div>
            </div>
            <div className="mt-12 grid grid-cols-2 gap-3">
              <HeroFact label="Frequency" value={formatFreq(feed.frequency)} />
              <HeroFact
                label="Health"
                value={feedHealthLabel(feed.health.class)}
              />
              <HeroFact label="Updated" value={timeAgo(feed.updated)} />
              <HeroFact label="IP version" value={feed.ipv || "—"} />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function HeroEvolutionBackground({
  points,
  loading,
  available,
}: {
  points: ReturnType<typeof parseHistoryCSV>;
  loading: boolean;
  available: boolean;
}) {
  const theme = useChartTheme();
  if (loading) {
    return <div className="h-full w-full animate-pulse bg-white/[0.04]" />;
  }
  if (!available) {
    return <div className="h-full w-full bg-white/[0.02]" />;
  }
  return (
    <div className="h-full w-full px-3 py-3 sm:px-4 sm:py-4">
      <AreaChart
        responsive
        style={{ width: "100%", height: "100%", minWidth: 0 }}
        data={points}
        margin={{ top: 0, right: 0, bottom: 0, left: 0 }}
      >
        <defs>
          <linearGradient id="heroEvolutionGradient" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor={theme.accent} stopOpacity={0.38} />
            <stop offset="100%" stopColor={theme.accent} stopOpacity={0.04} />
          </linearGradient>
        </defs>
        <Area
          type="monotone"
          dataKey="ips"
          stroke={theme.accent}
          strokeWidth={2}
          fill="url(#heroEvolutionGradient)"
          dot={false}
          activeDot={false}
          isAnimationActive
          animationDuration={800}
          animationEasing="ease-out"
        />
      </AreaChart>
    </div>
  );
}

function heroEvolutionStatusMessage(
  historyQuery: {
    isLoading: boolean;
    isError: boolean;
    error: unknown;
  },
  points: ReturnType<typeof parseHistoryCSV>,
): ReactNode {
  if (historyQuery.isLoading) {
    return "Loading all-time evolution…";
  }
  if (historyQuery.isError) {
    return historyQuery.error instanceof Error && historyQuery.error.message
      ? `Evolution history could not be loaded. ${historyQuery.error.message}`
      : "Evolution history could not be loaded.";
  }
  if (points.length < 2) {
    return "All-time evolution appears after at least two recorded updates.";
  }
  return buildEvolutionHeadline(points);
}

function buildEvolutionHeadline(
  points: ReturnType<typeof parseHistoryCSV>,
): ReactNode {
  const current = points[points.length - 1]?.ips ?? 0;
  const low = Math.min(...points.map((point) => point.ips));
  const high = Math.max(...points.map((point) => point.ips));
  const spanMs = Math.max(0, (points[points.length - 1]?.ts ?? 0) - (points[0]?.ts ?? 0));
  return (
    <>
      {formatNum(current)} IPs today.
      <br />
      {formatHistorySpan(spanMs)} range: {formatNum(low)} – {formatNum(high)}.
    </>
  );
}

function formatHistorySpan(spanMs: number): string {
  const day = 24 * 60 * 60 * 1000;
  const days = spanMs / day;
  if (days >= 365) {
    const years = Math.max(1, Math.round(days / 365.25));
    return years === 1 ? "1-year" : `${years}-year`;
  }
  if (days >= 30) {
    const months = Math.max(1, Math.round(days / 30.44));
    return months === 1 ? "1-month" : `${months}-month`;
  }
  const roundedDays = Math.max(1, Math.round(days));
  return roundedDays === 1 ? "1-day" : `${roundedDays}-day`;
}

function formatStartYear(input: number): string {
  const millis = input < 1e12 ? input * 1000 : input;
  return String(new Date(millis).getUTCFullYear());
}

function formatTrackingAge(input: number): string {
  const millis = input < 1e12 ? input * 1000 : input;
  const started = new Date(millis);
  const now = new Date();
  let years = now.getUTCFullYear() - started.getUTCFullYear();
  let months = now.getUTCMonth() - started.getUTCMonth();
  if (now.getUTCDate() < started.getUTCDate()) {
    months -= 1;
  }
  if (months < 0) {
    years -= 1;
    months += 12;
  }
  if (years <= 0) {
    return months <= 1 ? "1 month" : `${months} months`;
  }
  if (months <= 0) {
    return years === 1 ? "1 year" : `${years} years`;
  }
  return `${years}y ${months}mo`;
}
