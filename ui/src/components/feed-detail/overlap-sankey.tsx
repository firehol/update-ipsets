import { useCallback, useMemo, useRef, useState } from "react";
import { sankey, sankeyJustify, sankeyLinkHorizontal } from "d3-sankey";
import { ParentSize } from "@visx/responsive";
import type { ComparisonRow } from "@/lib/api-types";
import { useChartTheme } from "@/lib/chart-theme";
import { formatIPs } from "@/lib/utils";
import { useClearOnExit } from "@/components/editorial/clear-on-exit";
import { CursorTip } from "@/components/editorial/cursor-tip";

/** Hover state. Discriminated union — link hovers carry source/target,
 *  node hovers carry just the feed name. */
type SankeyHover =
  | {
      kind: "link";
      source: string;
      target: string;
      value: number;
      percent: number;
      x: number;
      y: number;
    }
  | {
      kind: "node";
      label: string;
      value: number;
      x: number;
      y: number;
    };

/**
 * Sankey diagram of overlap between THIS feed and the top-N feeds it
 * shares IPs with. The left node is this feed (size = total feed IPs);
 * each right node is a feed it overlaps with (size = the other feed's
 * total). Link thickness = number of shared IPs.
 *
 * Restrained palette: every link uses the primary accent at varying
 * opacity proportional to the link's relative size, so the dominant
 * relationships pop without introducing rainbow colours. Nodes are
 * filled with the secondary accent (blue) and hairline-bordered.
 *
 * The full pairwise table sits underneath this diagram in the parent
 * section — Sankey is meant to highlight the half-dozen biggest
 * overlaps, not enumerate every relationship.
 */
export function OverlapSankey({
  feedName,
  feedIPs,
  rows,
  topN = 12,
  height = 460,
}: {
  feedName: string;
  feedIPs: number;
  rows: ComparisonRow[];
  topN?: number;
  height?: number;
}) {
  if (rows.length === 0) {
    return (
      <div className="flex items-center justify-center text-sm text-muted-foreground" style={{ height }}>
        No overlaps to plot.
      </div>
    );
  }
  return (
    <div style={{ height }}>
      <ParentSize>
        {({ width, height: h }) => (
          <SankeyInner
            feedName={feedName}
            feedIPs={feedIPs}
            rows={rows}
            topN={topN}
            width={width}
            height={h}
          />
        )}
      </ParentSize>
    </div>
  );
}

interface SNode {
  id: string;
  label: string;
  side: "left" | "right";
}

interface SLink {
  source: string;
  target: string;
  value: number;
}

function SankeyInner({
  feedName,
  feedIPs,
  rows,
  topN,
  width,
  height,
}: {
  feedName: string;
  feedIPs: number;
  rows: ComparisonRow[];
  topN: number;
  width: number;
  height: number;
}) {
  const theme = useChartTheme();
  const [hover, setHover] = useState<SankeyHover | null>(null);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const clearHover = useCallback(() => setHover(null), []);
  // Bulletproof cursor-exit detection — see useClearOnExit doc.
  useClearOnExit(containerRef, hover != null, clearHover);

  const layout = useMemo(() => {
    if (width === 0 || height === 0) return null;
    const top = [...rows]
      .filter((r) => r.common > 0)
      .sort((a, b) => b.common - a.common)
      .slice(0, topN);
    if (top.length === 0) return null;

    const nodes: SNode[] = [
      { id: feedName, label: feedName, side: "left" },
      ...top.map<SNode>((r) => ({ id: r.name, label: r.name, side: "right" })),
    ];
    const links: SLink[] = top.map((r) => ({
      source: feedName,
      target: r.name,
      value: r.common,
    }));

    const generator = sankey<SNode, SLink>()
      .nodeId((d) => d.id)
      .nodeAlign(sankeyJustify)
      .nodeWidth(14)
      .nodePadding(10)
      .extent([
        [4, 4],
        [width - 4, height - 4],
      ]);

    // d3-sankey mutates the input — clone so re-renders are deterministic.
    const graph = generator({
      nodes: nodes.map((n) => ({ ...n })),
      links: links.map((l) => ({ ...l })),
    });
    return { graph, top, totalCommon: top.reduce((a, b) => a + b.common, 0) };
  }, [feedName, rows, topN, width, height]);

  if (!layout) return null;

  const { graph } = layout;
  const linkPath = sankeyLinkHorizontal<SNode, SLink>();
  const maxLinkValue = Math.max(...graph.links.map((l) => l.value || 0), 1);

  return (
    <div ref={containerRef} style={{ width, height, position: "relative" }}>
      <svg
        width={width}
        height={height}
        role="img"
        aria-label="Overlap sankey"
      >
        <defs>
          {/* Link gradient: starts at the source side in the primary
              accent and fades to the secondary accent. Same gradient for
              every link — opacity is what differentiates loud links from
              quiet ones. */}
          <linearGradient id="overlap-sankey-gradient" x1="0" y1="0" x2="1" y2="0">
            <stop offset="0%" stopColor={theme.accent} stopOpacity={0.85} />
            <stop offset="100%" stopColor={theme.secondary} stopOpacity={0.55} />
          </linearGradient>
        </defs>
        <g>
          {graph.links.map((link, i) => {
            const path = linkPath(link) || "";
            const ratio = (link.value || 0) / maxLinkValue;
            // Opacity scale: dominant link gets 0.85, mid links 0.5, tail 0.25.
            const opacity = 0.25 + ratio * 0.6;
            const value = link.value || 0;
            const sourceId = (link.source as SNode).id;
            const targetId = (link.target as SNode).id;
            const percent = feedIPs > 0 ? (value / feedIPs) * 100 : 0;
            const handleEnter = (e: React.MouseEvent<SVGPathElement>) => {
              setHover({
                kind: "link",
                source: sourceId,
                target: targetId,
                value,
                percent,
                x: e.clientX,
                y: e.clientY,
              });
            };
            const handleMove = (e: React.MouseEvent<SVGPathElement>) => {
              setHover((prev) =>
                prev && prev.kind === "link" && prev.target === targetId
                  ? { ...prev, x: e.clientX, y: e.clientY }
                  : prev,
              );
            };
            const handleLeave = () => {
              setHover((prev) =>
                prev && prev.kind === "link" && prev.target === targetId ? null : prev,
              );
            };
            return (
              <path
                key={`l-${i}`}
                d={path}
                fill="none"
                stroke="url(#overlap-sankey-gradient)"
                strokeOpacity={opacity}
                strokeWidth={Math.max(1, link.width || 0)}
                onMouseEnter={handleEnter}
                onMouseMove={handleMove}
                onMouseLeave={handleLeave}
                style={{ cursor: "pointer" }}
              />
            );
          })}
        </g>
        <g>
          {graph.nodes.map((node, i) => {
            const x0 = node.x0 ?? 0;
            const x1 = node.x1 ?? 0;
            const y0 = node.y0 ?? 0;
            const y1 = node.y1 ?? 0;
            const w = x1 - x0;
            const h = y1 - y0;
            if (h <= 0) return null;
            const isSource = node.side === "left";
            // Source node uses the secondary accent (blue) so the "this
            // feed" anchor reads differently from the destination feeds
            // — they get the primary accent (red).
            const fill = isSource ? theme.secondary : theme.accent;
            const labelX = isSource ? x1 + 6 : x0 - 6;
            const labelAnchor = isSource ? "start" : "end";
            // Label only if there's room; otherwise hover carries it.
            const showLabel = h >= 12;
            const label = node.label;
            const value = node.value || 0;
            const handleEnter = (e: React.MouseEvent<SVGRectElement>) => {
              setHover({ kind: "node", label, value, x: e.clientX, y: e.clientY });
            };
            const handleMove = (e: React.MouseEvent<SVGRectElement>) => {
              setHover((prev) =>
                prev && prev.kind === "node" && prev.label === label
                  ? { ...prev, x: e.clientX, y: e.clientY }
                  : prev,
              );
            };
            const handleLeave = () => {
              setHover((prev) =>
                prev && prev.kind === "node" && prev.label === label ? null : prev,
              );
            };
            return (
              <g key={`n-${i}`}>
                <rect
                  x={x0}
                  y={y0}
                  width={w}
                  height={h}
                  fill={fill}
                  fillOpacity={0.85}
                  stroke={theme.tooltipBg}
                  strokeWidth={0.5}
                  onMouseEnter={handleEnter}
                  onMouseMove={handleMove}
                  onMouseLeave={handleLeave}
                  style={{ cursor: "pointer" }}
                />
                {showLabel && (
                  <text
                    x={labelX}
                    y={(y0 + y1) / 2}
                    dy="0.35em"
                    textAnchor={labelAnchor}
                    fontSize={11}
                    fontFamily="JetBrains Mono, monospace"
                    fill={theme.tooltipFg}
                    style={{ pointerEvents: "none" }}
                  >
                    {label}
                  </text>
                )}
              </g>
            );
          })}
        </g>
        <text
          x={width / 2}
          y={height - 6}
          textAnchor="middle"
          fontSize={10}
          fill={theme.axis}
        >
          Top {graph.nodes.length - 1} feeds by shared IP count · this feed has {formatIPs(feedIPs)} IPs
        </text>
      </svg>
      <CursorTip x={hover?.x} y={hover?.y}>
        {hover && hover.kind === "link" && (
          <>
            <div className="font-semibold text-popover-foreground">
              <span className="font-mono text-popover-foreground/85">{hover.source}</span>
              <span className="mx-1.5 text-popover-foreground/40">→</span>
              <span className="font-mono text-popover-foreground/85">{hover.target}</span>
            </div>
            <div className="mt-1 tabular-nums text-popover-foreground/85">
              {formatIPs(hover.value)} shared IPs
              <span className="mx-1.5 text-popover-foreground/40">·</span>
              {hover.percent.toFixed(2)}% of feed
            </div>
          </>
        )}
        {hover && hover.kind === "node" && (
          <>
            <div className="font-mono font-semibold text-popover-foreground">
              {hover.label}
            </div>
            <div className="mt-1 tabular-nums text-popover-foreground/85">
              {formatIPs(hover.value)} IPs
            </div>
          </>
        )}
      </CursorTip>
    </div>
  );
}
