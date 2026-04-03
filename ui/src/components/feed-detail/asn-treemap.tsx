import { useCallback, useMemo, useRef, useState, type KeyboardEvent } from "react";
import { useNavigate } from "react-router-dom";
import { Group } from "@visx/group";
import { Treemap, hierarchy, treemapSquarify } from "@visx/hierarchy";
import { ParentSize } from "@visx/responsive";
import type { ASNFeedPayload } from "@/lib/api-types";
import { formatIPs } from "@/lib/utils";
import { useChartTheme } from "@/lib/chart-theme";
import { useClearOnExit } from "@/components/editorial/clear-on-exit";
import { CursorTip } from "@/components/editorial/cursor-tip";

/** Hover state for the treemap. SVG `<rect>` cannot be a Radix Tooltip
 *  trigger, so we track the hovered ASN + cursor coordinates ourselves
 *  and render a `<CursorTip>` next to the SVG. */
interface ASNHover {
  asn: number;
  label: string;
  count: number;
  percent: number;
  x: number;
  y: number;
}

/**
 * Treemap view of the per-provider ASN distribution. Default tab for
 * the AS Composition section — the shape carries proportion information
 * better than the bubble pack for users who read rectangles faster
 * than circles. Each tile is labeled with the AS number and, when it
 * fits, the organisation name.
 *
 * Restrained palette: ONE accent colour. Opacity falls with rank so the
 * top ASNs stand out against the long tail without turning the chart
 * into a rainbow.
 */
export function ASNTreemap({
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
          <TreemapInner data={data} width={width} height={h} navigate={navigate} />
        )}
      </ParentSize>
    </div>
  );
}

type Datum = {
  name: string;
  children?: Datum[];
  asn?: number;
  count?: number;
  label?: string;
};

function TreemapInner({
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
  const theme = useChartTheme();
  const [hover, setHover] = useState<ASNHover | null>(null);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const clearHover = useCallback(() => setHover(null), []);
  // Bulletproof cursor-exit detection — see useClearOnExit doc.
  useClearOnExit(containerRef, hover != null, clearHover);

  const top = useMemo(
    () => [...(data.by_asn ?? [])].sort((a, b) => b.count - a.count).slice(0, 80),
    [data.by_asn],
  );

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
      })
        .sum((d) => d.count ?? 0)
        .sort((a, b) => (b.value ?? 0) - (a.value ?? 0)),
    [top],
  );

  // Lookup percent by ASN — the by_asn array has it but the
  // hierarchy nodes only carry asn/count/label.
  const percentByASN = useMemo(() => {
    const m = new Map<number, number>();
    for (const a of data.by_asn) m.set(a.asn, a.percent);
    return m;
  }, [data.by_asn]);

  if (width === 0 || height === 0) return null;

  return (
    <div ref={containerRef} style={{ width, height, position: "relative" }}>
      <svg
        width={width}
        height={height}
        role="img"
        aria-label="ASN distribution treemap"
      >
        <Treemap<Datum>
          root={root}
          size={[width, height]}
          tile={treemapSquarify}
          round
        >
          {(treemap) => (
            <Group>
              {treemap
                .descendants()
                .filter((node) => node.depth > 0)
                .map((node, i) => {
                  const w = node.x1 - node.x0;
                  const h = node.y1 - node.y0;
                  if (w < 2 || h < 2) return null;
                  // Rank-based opacity — top 3 stand out against the tail.
                  const opacity = i < 3 ? 0.92 : i < 8 ? 0.78 : i < 20 ? 0.6 : 0.4;
                  // Label only if the tile has room: AS number for small
                  // tiles, AS number + org name for larger ones.
                  const showNumber = w > 40 && h > 18;
                  const showName = w > 110 && h > 36 && !!node.data.label;
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
                      key={`tm-${i}`}
                      transform={`translate(${node.x0},${node.y0})`}
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
                      <rect
                        width={w}
                        height={h}
                        fill={theme.accent}
                        fillOpacity={opacity}
                        stroke={theme.tooltipBg}
                        strokeWidth={1}
                      />
                      {showNumber && (
                        <text
                          x={8}
                          y={16}
                          fill="white"
                          fontSize={11}
                          fontFamily="JetBrains Mono, monospace"
                          fontWeight={600}
                          style={{ pointerEvents: "none", letterSpacing: "-0.01em" }}
                        >
                          AS{asn}
                        </text>
                      )}
                      {showName && (
                        <text
                          x={8}
                          y={32}
                          fill="white"
                          fillOpacity={0.82}
                          fontSize={11}
                          style={{ pointerEvents: "none" }}
                        >
                          {truncate(label, Math.max(0, Math.floor(w / 7) - 2))}
                        </text>
                      )}
                      {showNumber && h > 54 && (
                        <text
                          x={8}
                          y={h - 8}
                          fill="white"
                          fillOpacity={0.7}
                          fontSize={10}
                          style={{ pointerEvents: "none" }}
                        >
                          {formatIPs(count)}
                        </text>
                      )}
                    </g>
                  );
                })}
            </Group>
          )}
        </Treemap>
      </svg>
      <CursorTip x={hover?.x} y={hover?.y}>
        {hover && <ASNHoverBody hover={hover} />}
      </CursorTip>
    </div>
  );
}

/** Shared tooltip body for the ASN treemap and bubble chart. Same shape
 *  in both so the user gets the same details regardless of which view
 *  they switched to. */
function ASNHoverBody({ hover }: { hover: ASNHover }) {
  return (
    <>
      <div className="font-semibold text-popover-foreground">
        <span className="font-mono text-popover-foreground/70">AS{hover.asn}</span>
        <span className="mx-1.5 text-popover-foreground/40">·</span>
        <span>{hover.label}</span>
      </div>
      <div className="mt-1 tabular-nums text-popover-foreground/85">
        {formatIPs(hover.count)} IPs
        <span className="mx-1.5 text-popover-foreground/40">·</span>
        {hover.percent.toFixed(2)}% of feed
      </div>
    </>
  );
}

export { ASNHoverBody };
export type { ASNHover };

function truncate(s: string, max: number): string {
  if (s.length <= max) return s;
  if (max <= 1) return "";
  return s.slice(0, max - 1) + "…";
}
