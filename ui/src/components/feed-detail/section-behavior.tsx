import { type ReactNode, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  Line,
  LineChart,
  ReferenceLine,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { Activity } from "lucide-react";
import { useCategoryAccent } from "@/lib/categories";
import { DetailNotice, DetailSection, DetailTwoColumnPanels } from "./section";
import { formatIPs } from "@/lib/utils";
import { useChartTheme, type ChartTheme } from "@/lib/chart-theme";
import type { ChangesetPoint, FeedMetadata } from "@/lib/api-types";
import { changesetsOptions, historyOptions } from "@/lib/queries/feed";
import { parseHistoryCSV, type HistoryPoint } from "@/lib/feed-history";

/**
 * Per-update churn computed from changesets — the SIGNED ratio has no
 * meaning, we want the magnitude of change even when the net delta is
 * zero. Churn = (added + removed) / previous_size so a full refresh
 * (1000 removed then 1000 added on a 1000-IP list) reads as 200% while
 * the net delta is 0.
 */
interface ChurnPoint {
  ts: number;
  churn: number;
}

/**
 * Build a tooltipStyle object from the active chart theme. Recharts
 * accepts style objects — CSS variables won't work because Recharts
 * serialises to SVG attributes, not CSS.
 */
function tooltipStyle(theme: ChartTheme) {
  return {
    background: theme.tooltipBg,
    border: `1px solid ${theme.tooltipBorder}`,
    borderRadius: "10px",
    color: theme.tooltipFg,
    fontSize: "12px",
    padding: "8px 12px",
    boxShadow: theme.tooltipShadow,
  };
}

/**
 * Behavior section: four small charts showing how the feed moves over
 * the last 500 recorded updates.
 *
 * The four charts intentionally use ONE accent + ONE secondary colour.
 * Added and removed are the only time we use two hues in a single
 * chart: added is the primary accent (red), removed is the secondary
 * (blue). Everything else stays mono-accent. This is the "no circus
 * of colours" rule with a single exception for bidirectional data
 * where the semantic difference is load-bearing.
 */
export function SectionBehavior({
  feedName,
  feed,
}: {
  feedName: string;
  feed: FeedMetadata;
}) {
  const accent = useCategoryAccent(feed.category);
  const historyQuery = useQuery({
    ...historyOptions(feedName),
  });

  const changesetsQuery = useQuery({
    ...changesetsOptions(feedName),
    // Changesets only exist for feeds that have actually changed since
    // we started tracking; young feeds return an empty array. Empty is
    // not an error.
    retry: false,
  });

  const points = useMemo(() => parseHistoryCSV(historyQuery.data), [historyQuery.data]);
  const historyError = queryErrorMessage(historyQuery.error, "History data could not be loaded.");
  const changesetsError = queryErrorMessage(
    changesetsQuery.error,
    "Changeset data could not be loaded.",
  );

  // Correlate each changeset with the size of the list at that point
  // in time so churn = (added + removed) / size is accurate. Falls back
  // to the previous history point's size when the timestamps don't
  // line up exactly.
  const churnPoints = useMemo<ChurnPoint[]>(() => {
    const changesets = changesetsQuery.data ?? [];
    if (changesets.length === 0 || points.length === 0) return [];
    // Build a sorted array of (ts, ips) tuples for binary search.
    const sorted = [...points].sort((a, b) => a.ts - b.ts);
    const ipsAt = (tsMs: number): number => {
      // Find the most-recent history point at or before tsMs.
      let lo = 0;
      let hi = sorted.length - 1;
      let best = -1;
      while (lo <= hi) {
        const mid = (lo + hi) >> 1;
        if (sorted[mid].ts <= tsMs) {
          best = mid;
          lo = mid + 1;
        } else {
          hi = mid - 1;
        }
      }
      return best >= 0 ? sorted[best].ips : 0;
    };
    const out: ChurnPoint[] = [];
    for (const c of changesets) {
      const tsMs = c.timestamp * 1000;
      const size = ipsAt(tsMs);
      if (size === 0) continue;
      out.push({ ts: tsMs, churn: ((c.added + c.removed) / size) * 100 });
    }
    return out;
  }, [changesetsQuery.data, points]);

  return (
    <DetailSection
      eyebrow="Behaviour"
      title="How the list moves over time"
      lede="Four angles on how this feed changes between updates. Every chart spans the last 500 recorded runs, not a fixed calendar window."
      icon={Activity}
      accentColor={accent}
    >
      <DetailTwoColumnPanels
        left={{
          title: "Churn over the last 500 updates",
          children: (
            <ChartPanelBody
              title="Churn over the last 500 updates"
              loading={historyQuery.isLoading || changesetsQuery.isLoading}
              error={historyError ?? changesetsError}
            >
              <ChurnChart points={churnPoints} />
            </ChartPanelBody>
          ),
        }}
        right={{
          title: "IP count evolution",
          children: (
            <ChartPanelBody
              title="IP count evolution"
              loading={historyQuery.isLoading}
              error={historyError}
            >
              <EvolutionChart points={points} />
            </ChartPanelBody>
          ),
        }}
      />
      <DetailTwoColumnPanels
        className="mt-12"
        left={{
          title: "Cadence between updates",
          children: (
            <ChartPanelBody
              title="Cadence between updates"
              loading={historyQuery.isLoading}
              error={historyError}
            >
              <CadenceChart points={points} />
            </ChartPanelBody>
          ),
        }}
        right={{
          title: "IPs added vs removed per update",
          children: (
            <ChartPanelBody
              title="IPs added vs removed per update"
              loading={changesetsQuery.isLoading}
              error={changesetsError}
            >
              <AddedRemovedChart changesets={changesetsQuery.data ?? []} />
            </ChartPanelBody>
          ),
        }}
      />
    </DetailSection>
  );
}

function ChartPanelBody({
  title,
  loading,
  error,
  children,
}: {
  title: string;
  loading: boolean;
  error?: string | null;
  children: ReactNode;
}) {
  return (
    <div className="min-w-0">
      {loading ? (
        <div className="h-48 animate-pulse bg-muted/40" />
      ) : error ? (
        <div className="h-48">
          <DetailNotice title={title} tone="danger" className="flex h-full flex-col justify-center">
            {error}
          </DetailNotice>
        </div>
      ) : (
        <div className="h-48 w-full">{children}</div>
      )}
    </div>
  );
}

function ChurnChart({ points }: { points: ChurnPoint[] }) {
  const theme = useChartTheme();
  if (points.length < 2) {
    return <EmptyState message="Churn appears once this feed has at least two recorded changes." />;
  }
  return (
    <LineChart
      responsive
      style={{ width: "100%", height: "100%", minWidth: 0 }}
      data={points}
      margin={{ top: 4, right: 4, bottom: 4, left: 4 }}
    >
      <CartesianGrid stroke={theme.grid} strokeDasharray="2 4" />
      <XAxis dataKey="ts" hide />
      <YAxis tickFormatter={(v) => `${v.toFixed(0)}%`} stroke={theme.axis} fontSize={11} />
      <Tooltip
        labelFormatter={(v) => new Date(v).toLocaleString()}
        formatter={(v) => [`${Number(v).toFixed(2)}%`, "Churn"]}
        contentStyle={tooltipStyle(theme)}
        labelStyle={{ color: theme.tooltipFg }}
        itemStyle={{ color: theme.tooltipFg }}
      />
      <Line type="monotone" dataKey="churn" stroke={theme.accent} strokeWidth={1.5} dot={false} />
    </LineChart>
  );
}

function EvolutionChart({ points }: { points: HistoryPoint[] }) {
  const theme = useChartTheme();
  if (points.length < 2) return <EmptyState />;
  return (
    <AreaChart
      aria-label="IP count evolution chart"
      responsive
      style={{ width: "100%", height: "100%", minWidth: 0 }}
      data={points}
      margin={{ top: 4, right: 4, bottom: 4, left: 4 }}
    >
      <defs>
        <linearGradient id="evoGradient" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={theme.accent} stopOpacity={0.28} />
          <stop offset="100%" stopColor={theme.accent} stopOpacity={0.02} />
        </linearGradient>
      </defs>
      <CartesianGrid stroke={theme.grid} strokeDasharray="2 4" />
      <XAxis dataKey="ts" hide />
      <YAxis tickFormatter={(v) => formatIPs(v)} stroke={theme.axis} fontSize={11} />
      <Tooltip
        labelFormatter={(v) => new Date(v).toLocaleString()}
        formatter={(v) => [formatIPs(Number(v)), "IPs"]}
        contentStyle={tooltipStyle(theme)}
        labelStyle={{ color: theme.tooltipFg }}
        itemStyle={{ color: theme.tooltipFg }}
      />
      <Area type="monotone" dataKey="ips" stroke={theme.accent} strokeWidth={1.5} fill="url(#evoGradient)" />
    </AreaChart>
  );
}

function CadenceChart({ points }: { points: HistoryPoint[] }) {
  const theme = useChartTheme();
  if (points.length < 2) return <EmptyState />;
  const intervals: number[] = [];
  for (let i = 1; i < points.length; i++) {
    const dt = (points[i].ts - points[i - 1].ts) / 60000;
    if (dt > 0) intervals.push(dt);
  }
  if (intervals.length === 0) return <EmptyState />;
  const buckets = 20;
  const max = Math.max(...intervals);
  const width = max / buckets || 1;
  const data: { range: number; count: number }[] = Array.from({ length: buckets }, (_, i) => ({
    range: Math.round(width * (i + 0.5)),
    count: 0,
  }));
  for (const v of intervals) {
    const idx = Math.min(buckets - 1, Math.floor(v / width));
    data[idx].count += 1;
  }
  return (
    <AreaChart
      responsive
      style={{ width: "100%", height: "100%", minWidth: 0 }}
      data={data}
      margin={{ top: 4, right: 4, bottom: 4, left: 4 }}
    >
      <CartesianGrid stroke={theme.grid} strokeDasharray="2 4" />
      <XAxis
        dataKey="range"
        tickFormatter={(v) => formatMinutesCompact(Number(v))}
        stroke={theme.axis}
        fontSize={11}
      />
      <YAxis stroke={theme.axis} fontSize={11} />
      <Tooltip
        formatter={(v) => [String(v), "updates"]}
        labelFormatter={(v) => `~${formatMinutesCompact(Number(v))} interval`}
        contentStyle={tooltipStyle(theme)}
        labelStyle={{ color: theme.tooltipFg }}
        itemStyle={{ color: theme.tooltipFg }}
      />
      <Area type="step" dataKey="count" stroke={theme.accent} fill={theme.accent} fillOpacity={0.22} />
    </AreaChart>
  );
}

/**
 * Two-dimensional "per-update delta" chart. Added IPs plot above the
 * x-axis in the primary accent, removed IPs plot below the x-axis in
 * the secondary accent. The net delta is implicitly visible as the
 * asymmetry between the two — but unlike a simple signed-delta line,
 * this shows the true churn when the list is refreshed (large added
 * and large removed on the same update reads as loud bars in both
 * directions while the net delta is near zero).
 *
 * Rendered as a BarChart with the "removed" series negated so Recharts
 * plots it below zero. A ReferenceLine at y=0 makes the axis explicit.
 */
function AddedRemovedChart({ changesets }: { changesets: ChangesetPoint[] }) {
  const theme = useChartTheme();
  if (changesets.length < 1) {
    return <EmptyState message="No recorded changes yet." />;
  }
  const data = changesets.map((c) => ({
    ts: c.timestamp * 1000,
    added: c.added,
    // Negated for below-axis rendering.
    removed: -c.removed,
  }));
  return (
    <BarChart
      responsive
      style={{ width: "100%", height: "100%", minWidth: 0 }}
      data={data}
      margin={{ top: 4, right: 4, bottom: 4, left: 4 }}
      stackOffset="sign"
    >
      <CartesianGrid stroke={theme.grid} strokeDasharray="2 4" />
      <XAxis dataKey="ts" hide />
      <YAxis
        tickFormatter={(v) => formatIPs(Math.abs(Number(v)))}
        stroke={theme.axis}
        fontSize={11}
      />
      <ReferenceLine y={0} stroke={theme.axis} strokeWidth={1} />
      <Tooltip
        labelFormatter={(v) => new Date(v).toLocaleString()}
        formatter={(v, name) => {
          const n = Number(v);
          const label = name === "added" ? "Added" : "Removed";
          return [formatIPs(Math.abs(n)), label];
        }}
        contentStyle={tooltipStyle(theme)}
        labelStyle={{ color: theme.tooltipFg }}
        itemStyle={{ color: theme.tooltipFg }}
      />
      <Bar dataKey="added" stackId="delta" fill={theme.accent} maxBarSize={6} />
      <Bar dataKey="removed" stackId="delta" fill={theme.secondary} maxBarSize={6} />
    </BarChart>
  );
}

function EmptyState({ message = "Not enough history yet." }: { message?: string }) {
  return (
    <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
      {message}
    </div>
  );
}

function queryErrorMessage(error: unknown, fallback: string): string | null {
  if (!error) return null;
  if (error instanceof Error && error.message) {
    return `${fallback} ${error.message}`;
  }
  return fallback;
}

function formatMinutesCompact(minutes: number): string {
  if (!Number.isFinite(minutes) || minutes <= 0) return "0m";
  if (minutes < 60) return `${Math.round(minutes)}m`;
  if (minutes < 1440) return `${Math.round(minutes / 60)}h`;
  const days = minutes / 1440;
  if (days < 14) return `${Math.round(days)}d`;
  const weeks = days / 7;
  if (weeks < 10) return `${Math.round(weeks)}w`;
  const months = days / 30.44;
  if (months < 12) return `${Math.round(months)}mo`;
  const years = days / 365.25;
  return years < 10 ? `${years.toFixed(1)}yr` : `${Math.round(years)}yr`;
}
