import { useCallback, useMemo, useRef, useState, type KeyboardEvent } from "react";
import { useNavigate } from "react-router-dom";
import { Group } from "@visx/group";
import { Pack, hierarchy } from "@visx/hierarchy";
import { ParentSize } from "@visx/responsive";
import type { ASNFeedPayload } from "@/lib/api-types";
import { useChartTheme } from "@/lib/chart-theme";
import { useClearOnExit } from "@/components/editorial/clear-on-exit";
import { CursorTip } from "@/components/editorial/cursor-tip";
import { ASNHoverBody, type ASNHover } from "./asn-treemap";

/**
 * Bubble (circle pack) chart for the per-provider ASN distribution.
 *
 * Restrained palette: ONE accent colour for every bubble. Size carries
 * the magnitude information; opacity differentiates the largest from
 * the long tail. No rainbow.
 *
 * Built with @visx/hierarchy (D3-pack under the hood) so the geometry
 * matches what users see in the existing site.
 */
export function ASNBubbleChart({
  data,
  height = 420,
}: {
  data: ASNFeedPayload | null | undefined;
  height?: number;
}) {
  const navigate = useNavigate();
  if (!data || !data.by_asn || data.by_asn.length === 0) {
    return (
      <div
        className="flex items-center justify-center text-sm text-muted-foreground"
        style={{ height }}
      >
        No ASN data for this provider.
      </div>
    );
  }
  return (
    <div style={{ height }}>
      <ParentSize>
        {({ width, height: h }) => (
          <BubbleChartInner data={data} width={width} height={h} navigate={navigate} />
        )}
      </ParentSize>
    </div>
  );
}

function BubbleChartInner({
  data,
  width,
  height,
  navigate,
}: {
  data: ASNFeedPayload;
  width: number;
  height: number;
  navigate: ReturnType<typeof useNavigate>;
}) {
  const [hover, setHover] = useState<ASNHover | null>(null);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const clearHover = useCallback(() => setHover(null), []);
  // Bulletproof cursor-exit detection — see useClearOnExit doc.
  // Per-element / per-svg onMouseLeave proved unreliable across the
  // 5 SVG charts; this hook is the never-stuck guarantee.
  useClearOnExit(containerRef, hover != null, clearHover);

  const top = useMemo(
    () => [...(data.by_asn ?? [])].sort((a, b) => b.count - a.count).slice(0, 60),
    [data.by_asn],
  );

  type Datum = {
    name: string;
    children?: Datum[];
    asn?: number;
    count?: number;
    label?: string;
  };

  const root = useMemo(
    () =>
      hierarchy<Datum>({
        name: "root",
        children: top.map((a) => ({
          name: String(a.asn),
          asn: a.asn,
          count: a.count,
          label: a.name,
        })),
      }).sum((d) => d.count ?? 0),
    [top],
  );

  const theme = useChartTheme();

  // Lookup percent by ASN — same dance as in asn-treemap.tsx.
  // CRITICAL: this useMemo MUST stay above the `if (width === 0 …)`
  // early return below. Hooks have to be called in the same order
  // on every render. Putting it after the early return changes the
  // hook count between the first paint (width=0 → 4 hooks) and the
  // subsequent paints (width>0 → 5 hooks), which trips React error
  // #310 and crashes the chart.
  const percentByASN = useMemo(() => {
    const m = new Map<number, number>();
    for (const a of data.by_asn) m.set(a.asn, a.percent);
    return m;
  }, [data.by_asn]);

  if (width === 0 || height === 0) return null;

  // Single accent palette: every bubble is the same hue, opacity falls
  // off with rank.
  const accent = theme.accent;
  const defaultStroke = theme.grid;

  return (
    // Wrapping div carries the ref consumed by useClearOnExit.
    // Sized by its content (the SVG below), so the bounding-box
    // check tracks the actual rendered chart area exactly.
    <div ref={containerRef} style={{ width, height, position: "relative" }}>
      <svg
        width={width}
        height={height}
        role="img"
        aria-label="ASN distribution bubble chart"
      >
        <Pack<Datum> root={root} size={[width, height]} padding={5}>
          {(packData) => {
            const circles = packData.descendants().slice(1);
            return (
              <Group>
                {circles.map((node, i) => {
                  const r = node.r;
                  if (r < 2) return null;
                  // Top 5 are stronger; the rest fall off to a quiet wash.
                  const opacity = i < 3 ? 0.92 : i < 8 ? 0.78 : i < 20 ? 0.55 : 0.32;
                  const asn = node.data.asn ?? 0;
                  const count = node.data.count ?? 0;
                  const label = node.data.label || "Unknown";
                  const percent = percentByASN.get(asn) ?? 0;
                  const handleEnter = (e: React.MouseEvent<SVGGElement>) => {
                    setHover({
                      asn,
                      label,
                      count,
                      percent,
                      x: e.clientX,
                      y: e.clientY,
                    });
                  };
                  const handleMove = (e: React.MouseEvent<SVGGElement>) => {
                    setHover((prev) =>
                      prev && prev.asn === asn
                        ? { ...prev, x: e.clientX, y: e.clientY }
                        : prev,
                    );
                  };
                  const handleLeave = () => {
                    setHover((prev) => (prev && prev.asn === asn ? null : prev));
                  };
                  const handleActivate = () => {
                    if (!asn) return;
                    setHover(null);
                    navigate(`/asns/${asn}`);
                  };
                  const handleKeyDown = (e: KeyboardEvent<SVGGElement>) => {
                    if (!asn) return;
                    if (e.key !== "Enter" && e.key !== " ") return;
                    e.preventDefault();
                    handleActivate();
                  };
                  return (
                    <g
                      key={`asn-${i}`}
                      transform={`translate(${node.x},${node.y})`}
                      onMouseEnter={handleEnter}
                      onMouseMove={handleMove}
                      onMouseLeave={handleLeave}
                      onClick={asn ? handleActivate : undefined}
                      onKeyDown={asn ? handleKeyDown : undefined}
                      role={asn ? "link" : undefined}
                      tabIndex={asn ? 0 : undefined}
                      aria-label={asn ? `Open AS${asn} detail` : undefined}
                      style={{ cursor: asn ? "pointer" : undefined }}
                    >
                      <circle
                        r={r}
                        fill={accent}
                        fillOpacity={opacity}
                        stroke={defaultStroke}
                        strokeWidth={0.5}
                      />
                      {r > 26 && (
                        <text
                          textAnchor="middle"
                          dy="0.3em"
                          fill="white"
                          fontSize={Math.min(r / 2.6, 14)}
                          fontWeight={600}
                          style={{ pointerEvents: "none", letterSpacing: "-0.01em" }}
                        >
                          AS{asn}
                        </text>
                      )}
                    </g>
                  );
                })}
              </Group>
            );
          }}
        </Pack>
      </svg>
      <CursorTip x={hover?.x} y={hover?.y}>
        {hover && <ASNHoverBody hover={hover} />}
      </CursorTip>
    </div>
  );
}
