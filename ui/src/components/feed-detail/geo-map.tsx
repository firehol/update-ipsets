import { useCallback, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { scaleSqrt } from "d3-scale";
import type { CountryComparisonPayload } from "@/lib/api-types";
import { ISO_NUM_TO_A2 } from "@/lib/iso-codes";
import { useIsDark } from "@/lib/chart-theme";
import { formatIPs } from "@/lib/utils";
import { buildWorldPaths } from "@/lib/world-geographies";
import { useWorldTopology } from "@/lib/world-topology";
import { useClearOnExit } from "@/components/editorial/clear-on-exit";
import { CursorTip } from "@/components/editorial/cursor-tip";
const EMPTY_COUNTRIES: CountryComparisonPayload["countries"] = [];
const MAP_WIDTH = 980;

/**
 * The world-atlas TopoJSON file is vendored under ui/public so public
 * browsing never depends on a third-party CDN. The parsed topology is
 * cached in TanStack Query so switching between Map and List view (or
 * navigating between feed-detail pages) does not repeat that fetch work.
 *
 * The cached TopoJSON is converted into SVG paths locally with d3-geo
 * and topojson-client. This keeps the public bundle on maintained D3
 * packages while preserving the same map projection and interaction
 * contract.
 */

/** Hover state for the geo map. SVG `<path>` cannot be a Radix
 *  Tooltip trigger, so we track the hovered country + cursor
 *  coordinates ourselves and render a `<CursorTip>` next to the SVG. */
interface GeoHover {
  code: string;
  name: string;
  value: number;
  percent: number;
  x: number;
  y: number;
}

/**
 * World choropleth coloured by per-country IP count for the active geo
 * provider. Restrained palette:
 *
 *   - Countries with NO IPs in this feed: very muted theme-aware fill
 *   - Countries with IPs: low-saturation → FireHOL red gradient
 *   - Borders: thin hairlines matching the current theme
 *
 * The map renders on a TRANSPARENT background — no bordered card, no
 * boxed container. It blends with the surrounding section so the
 * choropleth reads as a data layer, not a widget. One accent hue per
 * run (red), neutral context everywhere else. No rainbow.
 */
export function GeoMap({
  payload,
  feedIPs,
  percentLabel = "of feed",
  height = 540,
}: {
  payload: CountryComparisonPayload | null | undefined;
  /** Total unique IPs in the parent feed. Used as the denominator for
   *  the per-country percentage shown in the hover tooltip. Falls back
   *  to `payload.total_mapped` (the sum of all country values on the
   *  map) when not provided, so the percentage still has a meaningful
   *  base in standalone usage. */
  feedIPs?: number;
  percentLabel?: string;
  height?: number;
}) {
  const isDark = useIsDark();
  const navigate = useNavigate();
  const [hover, setHover] = useState<GeoHover | null>(null);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const clearHover = useCallback(() => setHover(null), []);
  // Bulletproof cursor-exit detection — see useClearOnExit doc.
  useClearOnExit(containerRef, hover != null, clearHover);
  // Cached world TopoJSON — fetched once per page lifetime via
  // TanStack Query, shared across every GeoMap instance.
  const topologyQuery = useWorldTopology();
  const geographies = useMemo(
    () =>
      topologyQuery.data
        ? buildWorldPaths({
            topology: topologyQuery.data,
            width: MAP_WIDTH,
            height,
            scale: 170,
            center: [10, 25],
          })
        : [],
    [height, topologyQuery.data],
  );

  // Theme-aware colour tokens. The other charts all consume
  // `useChartTheme()`, which returns CSS variable strings and lets the
  // browser resolve colours at paint time — zero React re-render on
  // theme toggle. The map is the one exception because d3-scale
  // `scaleSqrt` interpolates between two PARSEABLE hex endpoints and
  // cannot consume `var(--chart-accent)`. So we read a single boolean
  // (`useIsDark`) and pick from hardcoded palettes per theme.
  const palette = useMemo(() => {
    if (isDark) {
      return {
        // No-data countries: a clearly muted neutral, lifted from the
        // page background by ~6 lightness steps so shapes are visible
        // but unmistakably "empty". Previously equalled the bg
        // exactly, which made the entire map look like only a handful
        // of countries had data.
        fillZero: "#1a2230",
        // Country borders. Lifted from fillZero by another ~5 steps so
        // shapes stay crisp on both fillZero and the brighter data fills.
        stroke: "#3a4663",
        // Data range: starts BRIGHT enough that the lowest data value
        // is unambiguously "has data" (not "almost the same as
        // no-data"). Previously the start was a near-navy red that
        // blended into the dark bg for any country below the top one.
        // The new start is a clear medium red — still dark enough to
        // distinguish from the bright top, light enough to read as a
        // data point.
        fillStart: "#7f1d1d",     // tailwind red-900
        fillEnd: "#ef4444",       // tailwind red-500
        hover: "#f8fafc",         // near-white hover to invert the choropleth
      };
    }
    return {
      fillZero: "#eef2f7",
      fillStart: "#fca5a5",
      fillEnd: "#dc2626",
      stroke: "#cbd5e1",
      hover: "#0f131b",
    };
  }, [isDark]);

  // Defensive: the daemon may serve a payload with `total_mapped: 0` but
  // no `countries` array at all when a feed has zero geo coverage. Treat
  // every "no countries" shape uniformly.
  const countries = payload?.countries ?? EMPTY_COUNTRIES;

  const { lookup, max } = useMemo(() => {
    const m = new Map<string, number>();
    let max = 0;
    for (const c of countries) {
      m.set(c.code.toUpperCase(), c.value);
      if (c.value > max) max = c.value;
    }
    return { lookup: m, max };
  }, [countries]);

  // Square-root scale instead of linear so the long tail of small
  // countries gets visual weight. Threat-feed geo distributions are
  // heavy-tailed: one or two countries hold most of the IPs and the
  // rest hold a few each. With a linear scale every non-top country
  // collapses onto the start of the range and looks identical to
  // "no data". Sqrt pulls the mid-range values toward the brighter
  // end of the gradient so they read as graded data, not flat noise.
  const colorScale = useMemo(
    () =>
      scaleSqrt<string>()
        .domain([0, Math.max(1, max)])
        .range([palette.fillStart, palette.fillEnd])
        .clamp(true),
    [max, palette],
  );

  // Denominator for the per-country percentage shown in the tooltip.
  // Prefer the parent feed's total IP count when supplied; fall back
  // to the sum of all country values (`total_mapped`) so standalone
  // usage of <GeoMap> still produces a meaningful number.
  const percentDenominator =
    feedIPs && feedIPs > 0 ? feedIPs : payload?.total_mapped ?? 0;

  if (!payload || countries.length === 0) {
    return (
      <div
        className="flex items-center justify-center text-sm text-muted-foreground"
        style={{ height }}
      >
        No country data for this provider.
      </div>
    );
  }

  if (!topologyQuery.data) {
    return (
      <div
        className="flex items-center justify-center text-sm text-muted-foreground"
        style={{ height }}
      >
        {topologyQuery.isError
          ? "Country boundaries are unavailable."
          : "Loading country boundaries."}
      </div>
    );
  }

  return (
    // useClearOnExit (above) installs a global mousemove listener
    // that clears the hover state the moment the cursor's
    // clientX/clientY are outside this div's bounding box. The
    // wrapper div carries the ref consumed by that hook.
    <div ref={containerRef} className="relative" style={{ height }}>
      <svg
        viewBox={`0 0 ${MAP_WIDTH} ${height}`}
        width="100%"
        height="100%"
        role="img"
        aria-label="Country distribution map"
      >
        {geographies.map((geo) => {
          const a2 = ISO_NUM_TO_A2[geo.id as keyof typeof ISO_NUM_TO_A2];
          const value = a2 ? lookup.get(a2) || 0 : 0;
          const fill = value > 0 ? colorScale(value) : palette.fillZero;
          const name = geo.name || a2 || "Unknown";
          const hovered = hover?.code === a2;
          const handleEnter = (e: React.MouseEvent<SVGPathElement>) => {
            if (!a2) return;
            setHover({
              code: a2,
              name,
              value,
              percent:
                percentDenominator > 0
                  ? (value / percentDenominator) * 100
                  : 0,
              x: e.clientX,
              y: e.clientY,
            });
          };
          const handleMove = (e: React.MouseEvent<SVGPathElement>) => {
            // Only update coordinates if we are already showing this
            // country, avoiding overlay rerenders across no-data paths.
            setHover((prev) =>
              prev && prev.code === a2
                ? { ...prev, x: e.clientX, y: e.clientY }
                : prev,
            );
          };
          const handleLeave = () => {
            setHover((prev) => (prev && prev.code === a2 ? null : prev));
          };
          const handleActivate = () => {
            if (!a2) return;
            setHover(null);
            navigate(`/countries/${a2}`);
          };
          const handleKeyDown = (e: React.KeyboardEvent<SVGPathElement>) => {
            if (!a2) return;
            if (e.key !== "Enter" && e.key !== " ") return;
            e.preventDefault();
            handleActivate();
          };
          return (
            <path
              key={geo.key}
              d={geo.path}
              fill={hovered ? palette.hover : fill}
              stroke={palette.stroke}
              strokeWidth={0.4}
              onMouseEnter={handleEnter}
              onMouseMove={handleMove}
              onMouseLeave={handleLeave}
              onClick={a2 ? handleActivate : undefined}
              onKeyDown={a2 ? handleKeyDown : undefined}
              role={a2 ? "link" : undefined}
              tabIndex={a2 ? 0 : undefined}
              aria-label={a2 ? `Open ${name} country detail` : undefined}
              style={{ cursor: a2 ? "pointer" : undefined, outline: "none" }}
            />
          );
        })}
      </svg>
      <div className="pointer-events-none absolute bottom-3 left-3 rounded-sm border border-border/80 bg-background/85 px-3 py-2 text-[10px] text-muted-foreground shadow-sm backdrop-blur">
        <div>Colour = attributed IPs per country</div>
        <div className="mt-1 flex items-center gap-2">
          <span>low</span>
          <span
            className="h-2 w-20 rounded-full"
            style={{
              background: `linear-gradient(90deg, ${palette.fillStart}, ${palette.fillEnd})`,
            }}
          />
          <span>high</span>
        </div>
        <div className="mt-1">Square-root scale so mid-range countries stay visible.</div>
      </div>
      <CursorTip x={hover?.x} y={hover?.y}>
        {hover && (
          <>
            <div className="font-semibold text-popover-foreground">
              <span className="font-mono text-popover-foreground/70">{hover.code}</span>
              <span className="mx-1.5 text-popover-foreground/40">·</span>
              <span>{hover.name}</span>
            </div>
            <div className="mt-1 tabular-nums text-popover-foreground/85">
              {formatIPs(hover.value)} IPs
              <span className="mx-1.5 text-popover-foreground/40">·</span>
              {hover.percent.toFixed(2)}% {percentLabel}
            </div>
          </>
        )}
      </CursorTip>
    </div>
  );
}
