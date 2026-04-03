import { type ReactNode, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Area,
  Bar,
  CartesianGrid,
  ComposedChart,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { RetentionData, RetentionWindow } from "@/lib/api-types";
import { useChartTheme, type ChartTheme } from "@/lib/chart-theme";
import { retentionOptions } from "@/lib/queries/feed";
import { Clock3 } from "lucide-react";
import { useCategoryAccent } from "@/lib/categories";
import { DetailNotice, DetailSection, DetailTwoColumnPanels } from "./section";

const CLOCK_SKEW_WARNING_HOURS = 2;
const LOG_SUB_HOUR_BUCKET = 0.5;

const absoluteUTCFormatter =
  typeof Intl !== "undefined"
    ? new Intl.DateTimeFormat("en", {
        dateStyle: "medium",
        timeStyle: "short",
        timeZone: "UTC",
      })
    : null;

/**
 * Retention section. Two histograms derived from the engine's
 * _retention.json:
 *
 *   - "Currently listed" (= freshness): the `current` window — age
 *     distribution of IPs that are on the list now. This view is aged
 *     forward to the viewer's current time when the browser clock looks
 *     sane.
 *
 *   - "Removed age" (= retention policy): the `past` window — how long
 *     removed IPs stayed on the list before being dropped.
 *
 * Both charts mark p75 prominently, with p90/p100 as secondary marks.
 * The key contract here is interpretation safety: load failure, empty,
 * partial observation, and "not enough history" are distinct states and
 * must stay distinct in the UI.
 */
export function SectionRetention({
  feedName,
  category,
}: {
  feedName: string;
  category?: string | null;
}) {
  const accent = useCategoryAccent(category);
  const [nowMs] = useState(() => Date.now());
  const retentionQuery = useQuery({
    ...retentionOptions(feedName),
    retry: false,
  });

  if (retentionQuery.isLoading) {
    return (
      <DetailSection
        eyebrow="Retention"
        title="How long IPs stay on the list"
        icon={Clock3}
        accentColor={accent}
      >
        <div className="h-96 animate-pulse bg-muted/40" />
      </DetailSection>
    );
  }

  if (retentionQuery.isError) {
    const message =
      retentionQuery.error instanceof Error
        ? retentionQuery.error.message
        : "Unknown error";
    return (
      <DetailSection
        eyebrow="Retention"
        title="How long IPs stay on the list"
        icon={Clock3}
        accentColor={accent}
        lede="Two windows into the feed's lifecycle. Freshness shows how old the current entries are; removed age shows how long previous entries were kept before being dropped."
      >
        <DetailNotice title="Retention data could not be loaded" tone="danger">
          The published retention artifact for this feed was unavailable or
          malformed. This is different from "no retention data yet".
          <div className="mt-2 font-mono text-xs text-foreground/80">{message}</div>
        </DetailNotice>
      </DetailSection>
    );
  }

  const data = retentionQuery.data;
  const updatedMs = data?.updated ?? 0;
  const startedMs = data?.started ?? 0;
  const hourDelta =
    updatedMs > 0 ? Math.floor((nowMs - updatedMs) / 3_600_000) : 0;
  const browserClockWrong = updatedMs > 0 && hourDelta < -CLOCK_SKEW_WARNING_HOURS;
  const currentAgeShiftHours = browserClockWrong ? 0 : Math.max(0, hourDelta);
  const incomplete = Boolean(data?.incomplete);

  return (
    <DetailSection
      eyebrow="Retention"
      title="How long IPs stay on the list"
      icon={Clock3}
      accentColor={accent}
      lede="Two windows into the feed's lifecycle. Freshness shows how old the current entries are; removed age shows how long previous entries were kept before being dropped. The p75 mark is the default percentile — it shows what most of the distribution looks like without letting the long tail dominate."
    >
      {browserClockWrong && (
        <DetailNotice title="Your browser clock looks wrong" tone="warning" className="mb-8">
          Freshness is normally aged forward to your current time. Your local
          clock appears to be more than {CLOCK_SKEW_WARNING_HOURS} hours behind
          the published artifact, so the chart below stays anchored to the last
          publication time instead.
        </DetailNotice>
      )}

      <DetailTwoColumnPanels
        left={{
          title: "Freshness — currently listed",
          description:
            "Age distribution of every IP currently on the list. A steep left slope means most IPs are new; a flatter curve means the list keeps entries for a long time.",
          notices: freshnessNotices(
            currentAgeShiftHours,
            browserClockWrong,
            incomplete,
            startedMs,
            updatedMs,
          ),
          children: (
            <CurrentRetentionBody
              data={data}
              ageShiftHours={currentAgeShiftHours}
              incomplete={incomplete}
            />
          ),
        }}
        right={{
          title: "Retention — age at removal",
          description:
            "Distribution of how long removed IPs were listed before the maintainer dropped them. This covers observed removals only, so it describes the policy we have seen rather than claiming complete lifetime ground truth.",
          notices: retentionNotices(incomplete, startedMs),
          children: <PastRetentionBody data={data} />,
        }}
      />
    </DetailSection>
  );
}

function CurrentRetentionBody({
  data,
  ageShiftHours,
  incomplete,
}: {
  data: RetentionData | undefined;
  ageShiftHours: number;
  incomplete: boolean;
}) {
  const window = data?.current;
  const total = windowTotal(window);
  if (!window || total === 0) {
    return (
      <DetailNotice title="No currently listed IPs yet">
        This feed does not currently publish any IPs, so there is no freshness
        distribution to show yet.
      </DetailNotice>
    );
  }
  return (
    <RetentionHistogram
      window={window}
      ageShiftHours={ageShiftHours}
      oldestOpenEnded={incomplete}
    />
  );
}

function PastRetentionBody({ data }: { data: RetentionData | undefined }) {
  const window = data?.past;
  const total = windowTotal(window);
  if (!window || total === 0) {
    return (
      <DetailNotice title="No observed removals yet">
        We have not yet observed IPs enter and later leave this feed within our
        retention window, so the removal-age distribution is still empty.
      </DetailNotice>
    );
  }
  return <RetentionHistogram window={window} />;
}

function freshnessNotices(
  ageShiftHours: number,
  browserClockWrong: boolean,
  incomplete: boolean,
  startedMs: number,
  updatedMs: number,
) {
  const notices: ReactNode[] = [];
  if (updatedMs > 0) {
    notices.push(
      <DetailNotice key="updated" title="Time anchor" tone="info">
        {browserClockWrong ? (
          <>
            Showing ages as of the last published artifact at{" "}
            <span className="font-mono text-foreground">
              {formatAbsoluteUTC(updatedMs)}
            </span>
            .
          </>
        ) : ageShiftHours > 0 ? (
          <>
            Aged forward to now from the last published artifact at{" "}
            <span className="font-mono text-foreground">
              {formatAbsoluteUTC(updatedMs)}
            </span>{" "}
            (+{formatHours(ageShiftHours)}).
          </>
        ) : (
          <>
            The latest published artifact is from{" "}
            <span className="font-mono text-foreground">
              {formatAbsoluteUTC(updatedMs)}
            </span>
            . No additional aging was needed yet.
          </>
        )}
      </DetailNotice>,
    );
  }
  if (incomplete) {
    notices.push(
      <DetailNotice key="incomplete" title="Partial observation window" tone="warning">
        Some currently listed IPs were already present when tracking began
        {startedMs > 0 ? (
          <>
            {" "}
            on{" "}
            <span className="font-mono text-foreground">
              {formatAbsoluteUTC(startedMs)}
            </span>
          </>
        ) : null}
        . Their true age is older than the oldest bucket shown here.
      </DetailNotice>,
    );
  }
  return notices;
}

function retentionNotices(incomplete: boolean, startedMs: number) {
  if (!incomplete) return [];
  return [
    <DetailNotice key="partial" title="Observed removals only" tone="warning">
      This histogram only covers IPs whose removal we have actually observed.
      Some currently listed IPs predate the observation window
      {startedMs > 0 ? (
        <>
          {" "}
          that began on{" "}
          <span className="font-mono text-foreground">
            {formatAbsoluteUTC(startedMs)}
          </span>
        </>
      ) : null}
      , so the historical retention picture is necessarily partial.
    </DetailNotice>,
  ];
}

/* -------------------------------------------------------------------------- */

interface Bin {
  xHours: number;
  displayHours: number;
  label: string;
  pc: number;
  pcc: number;
}

function RetentionHistogram({
  window: w,
  ageShiftHours = 0,
  oldestOpenEnded = false,
}: {
  window: RetentionWindow;
  ageShiftHours?: number;
  oldestOpenEnded?: boolean;
}) {
  const theme = useChartTheme();

  const {
    bins,
    labelByX,
    p75,
    p90,
    p100,
    total,
    xMin,
    xMax,
    openEndedP100,
  } = useMemo(() => {
    const hours = w.hours ?? [];
    const ips = w.ips ?? [];
    const n = Math.min(hours.length, ips.length);
    const total = windowTotal(w);

    const bins: Bin[] = [];
    const labelByX = new Map<number, string>();
    let xMin = Number.POSITIVE_INFINITY;
    let xMax = Number.NEGATIVE_INFINITY;
    let cum = 0;

    for (let i = 0; i < n; i++) {
      const displayHours = Math.max(0, hours[i] + ageShiftHours);
      const xHours = displayHours === 0 ? LOG_SUB_HOUR_BUCKET : displayHours;
      const openEnded = oldestOpenEnded && i === n - 1;
      const label = formatDisplayHours(displayHours, { openEnded });
      cum += ips[i];
      bins.push({
        xHours,
        displayHours,
        label,
        pc: total > 0 ? (ips[i] * 100) / total : 0,
        pcc: total > 0 ? (cum * 100) / total : 0,
      });
      labelByX.set(xHours, label);
      if (xHours < xMin) xMin = xHours;
      if (xHours > xMax) xMax = xHours;
    }

    const safeMin = Number.isFinite(xMin) ? xMin : 1;
    const safeMax =
      Number.isFinite(xMax) && xMax > safeMin
        ? xMax
        : safeMin < 1
          ? 1
          : safeMin * 2;
    const adjustedHours = hours.slice(0, n).map((hour) => Math.max(0, hour + ageShiftHours));

    return {
      bins,
      labelByX,
      p75: percentileHours(adjustedHours, ips, total, 0.75),
      p90: percentileHours(adjustedHours, ips, total, 0.9),
      p100: adjustedHours.length > 0 ? adjustedHours[adjustedHours.length - 1] : 0,
      total,
      xMin: safeMin,
      xMax: safeMax,
      openEndedP100: oldestOpenEnded,
    };
  }, [ageShiftHours, oldestOpenEnded, w]);

  if (bins.length === 0 || total === 0) {
    return (
      <DetailNotice title="Not enough retention history yet">
        This part of the distribution has not accumulated enough observed
        samples to render a meaningful histogram.
      </DetailNotice>
    );
  }

  return (
    <div>
      <div className="h-60 w-full">
        <ComposedChart
          responsive
          style={{ width: "100%", height: "100%", minWidth: 0 }}
          data={bins}
          margin={{ top: 16, right: 16, bottom: 4, left: 4 }}
        >
          <defs>
            <linearGradient id="retentionCumulativeFill" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={theme.accent} stopOpacity={0.32} />
              <stop offset="100%" stopColor={theme.accent} stopOpacity={0.04} />
            </linearGradient>
          </defs>
          <CartesianGrid stroke={theme.grid} strokeDasharray="2 4" vertical={false} />
          <XAxis
            dataKey="xHours"
            type="number"
            scale="log"
            domain={[xMin, xMax]}
            ticks={logTicks(xMin, xMax)}
            tickFormatter={(v) => labelByX.get(Number(v)) ?? formatDisplayHours(Number(v))}
            stroke={theme.axis}
            fontSize={11}
            tickLine={false}
            allowDataOverflow={false}
          />
          <YAxis
            type="number"
            scale="linear"
            domain={[0, 100]}
            ticks={[0, 25, 50, 75, 100]}
            tickFormatter={(v) => `${Math.round(Number(v))}%`}
            stroke={theme.axis}
            fontSize={11}
            tickLine={false}
            width={42}
            allowDataOverflow={false}
          />
          <Tooltip
            labelFormatter={(v) =>
              `${labelByX.get(Number(v)) ?? formatDisplayHours(Number(v))} bucket`
            }
            formatter={(value, name) => {
              const pct = Number(value);
              return [
                `${pct.toFixed(2)}%`,
                name === "pc" ? "in this bucket" : "cumulative",
              ];
            }}
            contentStyle={tooltipStyle(theme)}
            labelStyle={{ color: theme.tooltipFg }}
            itemStyle={{ color: theme.tooltipFg }}
            cursor={{ fill: theme.accent, fillOpacity: 0.08 }}
          />
          <Area
            dataKey="pcc"
            type="monotoneX"
            stroke={theme.accent}
            strokeWidth={1.5}
            fill="url(#retentionCumulativeFill)"
            isAnimationActive={false}
            dot={false}
            activeDot={false}
          />
          <Bar
            dataKey="pc"
            fill={theme.accent}
            fillOpacity={0.85}
            barSize={10}
            radius={[2, 2, 0, 0]}
            isAnimationActive={false}
          />
        </ComposedChart>
      </div>
      <div className="mt-6 grid grid-cols-3 gap-px overflow-hidden rounded-sm border border-border bg-border">
        <StatCell label="p75" value={formatDisplayHours(p75)} accent />
        <StatCell label="p90" value={formatDisplayHours(p90)} />
        <StatCell
          label="p100 (oldest)"
          value={formatDisplayHours(p100, { openEnded: openEndedP100 })}
        />
      </div>
    </div>
  );
}

/**
 * Build tick positions on a log10 scale for the time axis. Returns
 * decade markers (1, 10, 100, 1000…) bounded by [min, max], with the
 * min and max always included so the axis ends are labelled.
 */
function logTicks(min: number, max: number): number[] {
  const ticks = new Set<number>();
  ticks.add(min);
  ticks.add(max);
  let p = 1;
  while (p < min) p *= 10;
  while (p <= max) {
    ticks.add(p);
    p *= 10;
  }
  return Array.from(ticks).sort((a, b) => a - b);
}

function StatCell({
  label,
  value,
  accent,
}: {
  label: string;
  value: string;
  accent?: boolean;
}) {
  return (
    <div className="relative bg-card px-5 py-4">
      {accent && <span className="absolute left-0 top-0 h-[2px] w-8 bg-primary" />}
      <div className="eyebrow">{label}</div>
      <div className="num mt-1 font-display text-[18px] font-semibold tabular-nums text-foreground">
        {value}
      </div>
    </div>
  );
}

/* -------------------------------------------------------------------------- */

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

function windowTotal(window: RetentionWindow | undefined): number {
  if (!window) return 0;
  if (typeof window.total === "number") return window.total;
  return (window.ips ?? []).reduce((sum, value) => sum + value, 0);
}

/** Walk the cumulative distribution until we cross the target share. */
function percentileHours(
  hours: number[],
  ips: number[],
  total: number,
  fraction: number,
): number {
  if (total === 0 || hours.length === 0) return 0;
  const target = total * fraction;
  let cum = 0;
  for (let i = 0; i < hours.length; i++) {
    cum += ips[i];
    if (cum >= target) return hours[i];
  }
  return hours[hours.length - 1];
}

function formatDisplayHours(
  hours: number,
  options?: { openEnded?: boolean },
): string {
  const prefix = options?.openEnded ? "> " : "";
  if (hours < 1) return `${prefix}<1h`;
  if (hours < 24) return `${prefix}${Math.round(hours)}h`;
  const days = hours / 24;
  if (days < 30) return `${prefix}${Math.round(days)}d`;
  const months = days / 30.44;
  if (months < 12) return `${prefix}${Math.round(months)}mo`;
  const years = days / 365.25;
  return years < 10 ? `${prefix}${years.toFixed(1)}yr` : `${prefix}${Math.round(years)}yr`;
}

function formatHours(hours: number): string {
  if (hours < 1) {
    return `${Math.max(1, Math.round(hours * 60))}m`;
  }
  if (hours < 24) {
    return `${Math.round(hours)}h`;
  }
  const days = hours / 24;
  if (days < 30) {
    return `${Math.round(days)}d`;
  }
  const months = days / 30.44;
  if (months < 12) {
    return `${Math.round(months)}mo`;
  }
  const years = days / 365.25;
  return years < 10 ? `${years.toFixed(1)}yr` : `${Math.round(years)}yr`;
}

function formatAbsoluteUTC(tsMs: number): string {
  if (!absoluteUTCFormatter) return `${tsMs}`;
  return `${absoluteUTCFormatter.format(new Date(tsMs))} UTC`;
}
