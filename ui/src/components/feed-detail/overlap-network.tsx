import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  forceCenter,
  forceCollide,
  forceLink,
  forceManyBody,
  forceSimulation,
  type SimulationLinkDatum,
  type SimulationNodeDatum,
} from "d3-force";
import { ParentSize } from "@visx/responsive";
import type { ComparisonRow } from "@/lib/api-types";
import { useChartTheme } from "@/lib/chart-theme";
import { formatIPs } from "@/lib/utils";
import { useClearOnExit } from "@/components/editorial/clear-on-exit";
import { CursorTip } from "@/components/editorial/cursor-tip";

/** Hover state for the network graph. Mirrors overlap-sankey.tsx so
 *  the two views show the same content for the same hover targets. */
type NetworkHover =
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
      isCentre: boolean;
      x: number;
      y: number;
    };

/**
 * Force-directed graph of overlap between THIS feed and the top-N feeds
 * it shares IPs with. Star topology: this feed sits in the centre as
 * the larger node; each overlapping feed orbits it. Edge thickness =
 * shared IP count, node radius = total IPs (log-scaled so 4M and 16M
 * don't drown out 4k).
 *
 * Restrained palette: every edge uses the primary accent. Nodes use
 * the secondary accent for the centre and a muted neutral for the
 * neighbours, so the "this feed vs the rest" relationship is visible.
 *
 * The simulation runs entirely on the client. We tick it for ~250
 * iterations on first render and re-tick whenever the rows or
 * dimensions change. Mouse interaction is intentionally absent —
 * the section's full table below is the place to inspect specific
 * rows; this graph is for at-a-glance pattern recognition.
 */
export function OverlapNetwork({
  feedName,
  feedIPs,
  rows,
  topN = 18,
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
          <NetworkInner
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

interface FNode extends SimulationNodeDatum {
  id: string;
  label: string;
  ips: number;
  isCentre: boolean;
}

interface FLink extends SimulationLinkDatum<FNode> {
  source: string | FNode;
  target: string | FNode;
  value: number;
}

function NetworkInner({
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
  const simRef = useRef<ReturnType<typeof forceSimulation<FNode, FLink>> | null>(null);
  const [tick, setTick] = useState(0);
  const [hover, setHover] = useState<NetworkHover | null>(null);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const clearHover = useCallback(() => setHover(null), []);
  // Bulletproof cursor-exit detection — see useClearOnExit doc.
  useClearOnExit(containerRef, hover != null, clearHover);

  const data = useMemo(() => {
    const top = [...rows]
      .filter((r) => r.common > 0)
      .sort((a, b) => b.common - a.common)
      .slice(0, topN);
    const centre: FNode = {
      id: feedName,
      label: feedName,
      ips: feedIPs,
      isCentre: true,
      fx: width / 2,
      fy: height / 2,
    };
    const others: FNode[] = top.map((r) => ({
      id: r.name,
      label: r.name,
      ips: r.ips,
      isCentre: false,
    }));
    const nodes: FNode[] = [centre, ...others];
    const links: FLink[] = top.map((r) => ({
      source: feedName,
      target: r.name,
      value: r.common,
    }));
    return { nodes, links, top };
  }, [feedName, feedIPs, rows, topN, width, height]);

  // Build / restart the simulation whenever data or geometry changes.
  useEffect(() => {
    if (width === 0 || height === 0 || data.nodes.length < 2) return;
    const maxLinkValue = Math.max(...data.links.map((l) => l.value), 1);
    // Strength normalised to [0, 1] so the strongest overlap pulls the
    // hardest. Distance is inverse so high-overlap neighbours sit closer.
    const linkStrength = (l: FLink) => 0.05 + 0.4 * (l.value / maxLinkValue);
    const linkDistance = (l: FLink) =>
      40 + 120 * (1 - l.value / maxLinkValue);

    const sim = forceSimulation<FNode>(data.nodes)
      .force(
        "link",
        forceLink<FNode, FLink>(data.links)
          .id((d) => d.id)
          .strength(linkStrength)
          .distance(linkDistance),
      )
      .force("charge", forceManyBody<FNode>().strength(-180))
      .force(
        "collide",
        forceCollide<FNode>().radius((d) => nodeRadius(d) + 4),
      )
      .force("center", forceCenter(width / 2, height / 2))
      .alpha(1)
      .alphaDecay(0.04);

    // Manual ticking — render whenever the simulation moves.
    sim.on("tick", () => setTick((t) => t + 1));
    simRef.current = sim;

    return () => {
      sim.stop();
      simRef.current = null;
    };
  }, [data, width, height]);

  if (width === 0 || height === 0 || data.nodes.length < 2) return null;

  const maxLinkValue = Math.max(...data.links.map((l) => l.value), 1);

  return (
    <div ref={containerRef} style={{ width, height, position: "relative" }}>
      <svg
        width={width}
        height={height}
        role="img"
        aria-label="Overlap network graph"
        data-tick={tick}
      >
        <g>
          {data.links.map((link, i) => {
            const s = link.source as FNode;
            const t = link.target as FNode;
            if (s.x == null || s.y == null || t.x == null || t.y == null) return null;
            const ratio = link.value / maxLinkValue;
            const strokeWidth = 0.5 + ratio * 4.5;
            const opacity = 0.2 + ratio * 0.6;
            const value = link.value;
            const sourceId = s.id;
            const targetId = t.id;
            const percent = feedIPs > 0 ? (value / feedIPs) * 100 : 0;
            const handleEnter = (e: React.MouseEvent<SVGLineElement>) => {
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
            const handleMove = (e: React.MouseEvent<SVGLineElement>) => {
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
              <line
                key={`l-${i}`}
                x1={s.x}
                y1={s.y}
                x2={t.x}
                y2={t.y}
                stroke={theme.accent}
                strokeWidth={Math.max(strokeWidth, 4)}
                strokeOpacity={opacity}
                onMouseEnter={handleEnter}
                onMouseMove={handleMove}
                onMouseLeave={handleLeave}
                style={{ cursor: "pointer" }}
              />
            );
          })}
        </g>
        <g>
          {data.nodes.map((node) => {
            if (node.x == null || node.y == null) return null;
            const r = nodeRadius(node);
            const fill = node.isCentre ? theme.secondary : theme.accent;
            const labelOpacity = node.isCentre ? 1 : 0.85;
            const label = node.label;
            const ips = node.ips;
            const isCentre = node.isCentre;
            const handleEnter = (e: React.MouseEvent<SVGCircleElement>) => {
              setHover({ kind: "node", label, value: ips, isCentre, x: e.clientX, y: e.clientY });
            };
            const handleMove = (e: React.MouseEvent<SVGCircleElement>) => {
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
              <g key={node.id} transform={`translate(${node.x},${node.y})`}>
                <circle
                  r={r}
                  fill={fill}
                  fillOpacity={node.isCentre ? 0.95 : 0.78}
                  stroke={theme.tooltipBg}
                  strokeWidth={1}
                  onMouseEnter={handleEnter}
                  onMouseMove={handleMove}
                  onMouseLeave={handleLeave}
                  style={{ cursor: "pointer" }}
                />
                {(node.isCentre || r > 9) && (
                  <text
                    x={0}
                    y={r + 12}
                    textAnchor="middle"
                    fontSize={node.isCentre ? 12 : 10}
                    fontFamily="JetBrains Mono, monospace"
                    fontWeight={node.isCentre ? 600 : 400}
                    fill={theme.tooltipFg}
                    fillOpacity={labelOpacity}
                    style={{ pointerEvents: "none" }}
                  >
                    {label}
                  </text>
                )}
              </g>
            );
          })}
        </g>
      </svg>
      <CursorTip x={hover?.x} y={hover?.y}>
        {hover && hover.kind === "link" && (
          <>
            <div className="font-semibold text-popover-foreground">
              <span className="font-mono text-popover-foreground/85">{hover.source}</span>
              <span className="mx-1.5 text-popover-foreground/40">↔</span>
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
              {hover.isCentre && (
                <span className="ml-1.5 text-[10px] uppercase tracking-[0.08em] text-popover-foreground/55">
                  (this feed)
                </span>
              )}
            </div>
          </>
        )}
      </CursorTip>
    </div>
  );
}

/**
 * Log-scaled radius so a 16M-IP feed and a 4k-IP feed both render at
 * legible sizes. The centre node is fixed slightly larger so it always
 * reads as the anchor.
 */
function nodeRadius(node: FNode): number {
  if (node.isCentre) return 16;
  const ips = Math.max(1, node.ips);
  const log = Math.log10(ips);
  // log10 ranges roughly 1 (10) to 9 (1B). Map to [4, 14] px.
  return 4 + Math.min(10, Math.max(0, (log - 1) * 1.4));
}
