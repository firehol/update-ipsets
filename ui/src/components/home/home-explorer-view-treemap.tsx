import { useMemo } from "react";
import { Link } from "react-router-dom";
import { Group } from "@visx/group";
import { Treemap, hierarchy, treemapSquarify } from "@visx/hierarchy";
import { ParentSize } from "@visx/responsive";
import type { CategoryMeta, FeedSummary } from "@/lib/api-types";
import { formatNum } from "@/lib/utils";

interface TreemapLeaf {
  kind: "leaf";
  name: string;
  value: number;
  feedName: string;
  category: string;
  color: string;
  shortDescription?: string;
  officialName?: string;
  maintainer?: string;
}

interface TreemapBranch {
  kind: "branch";
  name: string;
  children: Array<TreemapLeaf>;
}

interface TreemapRoot {
  kind: "root";
  name: string;
  children: Array<TreemapBranch>;
}

type TreemapNode = TreemapRoot | TreemapBranch | TreemapLeaf;

export function HomeExplorerViewTreemap({
  feeds,
  categories,
}: {
  feeds: FeedSummary[];
  categories: CategoryMeta[];
}) {
  const root = useMemo<TreemapRoot>(() => {
    const palette = new Map(
      categories.map((c) => [c.name, c.color ?? "var(--chart-accent)"]),
    );
    const byCategory = new Map<string, TreemapLeaf[]>();
    for (const feed of feeds) {
      const value = feed.unique_ips ?? 0;
      if (value <= 0) continue;
      const leaf: TreemapLeaf = {
        kind: "leaf",
        name: feed.name,
        feedName: feed.name,
        value,
        category: feed.category,
        color: palette.get(feed.category) ?? "var(--chart-accent)",
        shortDescription: feed.short_description,
        officialName: feed.official_name,
        maintainer: feed.maintainer,
      };
      let leaves = byCategory.get(feed.category);
      if (!leaves) {
        leaves = [];
        byCategory.set(feed.category, leaves);
      }
      leaves.push(leaf);
    }
    const children: TreemapBranch[] = Array.from(byCategory.entries()).map(
      ([cat, leaves]) => ({
        kind: "branch",
        name: cat,
        children: leaves.sort((a, b) => b.value - a.value),
      }),
    );
    children.sort((a, b) => {
      const sumA = a.children.reduce((acc, l) => acc + l.value, 0);
      const sumB = b.children.reduce((acc, l) => acc + l.value, 0);
      return sumB - sumA;
    });
    return { kind: "root", name: "All feeds", children };
  }, [feeds, categories]);

  if (root.children.length === 0) {
    return (
      <div className="border border-dashed border-border py-24 text-center text-[13px] text-muted-foreground">
        No feeds with measurable size match the current filter.
      </div>
    );
  }

  return (
    <div className="space-y-3">
      <div className="text-[12px] text-muted-foreground">
        Tile area = unique IPs. Tile colour follows the configured category.
      </div>
      <div className="border border-border bg-card" style={{ height: 640 }}>
        <ParentSize>
          {({ width, height }) =>
            width > 0 && height > 0 ? (
              <TreemapChart root={root} width={width} height={height} />
            ) : null
          }
        </ParentSize>
      </div>
    </div>
  );
}

function TreemapChart({
  root,
  width,
  height,
}: {
  root: TreemapRoot;
  width: number;
  height: number;
}) {
  const data = useMemo(() => {
    return hierarchy<TreemapNode>(root, (node) =>
      node.kind === "leaf" ? null : node.children,
    )
      .sum((node) => (node.kind === "leaf" ? node.value : 0))
      .sort((a, b) => (b.value ?? 0) - (a.value ?? 0));
  }, [root]);

  return (
    <Treemap<TreemapNode>
      top={0}
      root={data}
      size={[width, height]}
      tile={treemapSquarify}
      round
      paddingInner={1}
      paddingOuter={2}
    >
      {(treemap) => {
        const nodes = treemap.descendants().reverse();
        return (
          <div className="relative h-full w-full">
            <svg
              width={width}
              height={height}
              role="img"
              aria-label="Feed size treemap"
              className="absolute inset-0"
            >
              <Group>
                {nodes.map((node, i) => {
                  const nodeWidth = node.x1 - node.x0;
                  const nodeHeight = node.y1 - node.y0;
                  const payload = node.data;
                  if (payload.kind === "root") return null;
                  if (payload.kind === "branch") {
                    return (
                      <text
                        key={`label-${i}`}
                        x={node.x0 + 6}
                        y={node.y0 + 14}
                        className="pointer-events-none fill-foreground/75 font-mono text-[10px] uppercase tracking-[0.08em]"
                      >
                        {nodeWidth > 60 && nodeHeight > 20 ? payload.name : ""}
                      </text>
                    );
                  }
                  const leafColor = payload.color;
                  const showLabel = nodeWidth > 60 && nodeHeight > 30;
                  return (
                    <g key={`leaf-${i}`}>
                      <rect
                        x={node.x0}
                        y={node.y0}
                        width={nodeWidth}
                        height={nodeHeight}
                        fill={leafColor}
                        fillOpacity={0.65}
                        stroke="hsl(var(--background))"
                        strokeOpacity={0.15}
                      />
                      {showLabel && (
                        <>
                          <text
                            x={node.x0 + 6}
                            y={node.y0 + 18}
                            className="pointer-events-none fill-white font-mono text-[11px] font-semibold"
                          >
                            {payload.feedName.length > 24
                              ? payload.feedName.slice(0, 22) + "…"
                              : payload.feedName}
                          </text>
                          <text
                            x={node.x0 + 6}
                            y={node.y0 + 32}
                            className="pointer-events-none fill-white/80 text-[10px] tabular-nums"
                          >
                            {formatNum(payload.value)}
                          </text>
                        </>
                      )}
                    </g>
                  );
                })}
              </Group>
            </svg>
            {nodes.map((node, i) => {
              const nodeWidth = node.x1 - node.x0;
              const nodeHeight = node.y1 - node.y0;
              const payload = node.data;
              if (payload.kind !== "leaf" || nodeWidth < 2 || nodeHeight < 2) {
                return null;
              }
              return (
                <Link
                  key={`leaf-link-${i}`}
                  to={`/ipsets/${encodeURIComponent(payload.feedName)}`}
                  aria-label={`Open ${payload.feedName} feed details`}
                  title={treemapTooltip(payload)}
                  className="absolute block focus:outline-none focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary"
                  style={{
                    left: node.x0,
                    top: node.y0,
                    width: nodeWidth,
                    height: nodeHeight,
                  }}
                />
              );
            })}
          </div>
        );
      }}
    </Treemap>
  );
}

function treemapTooltip(leaf: TreemapLeaf): string {
  const parts: string[] = [leaf.officialName?.trim() || leaf.feedName];
  if (leaf.shortDescription?.trim()) parts.push(leaf.shortDescription.trim());
  if (leaf.maintainer?.trim()) parts.push(`Maintainer: ${leaf.maintainer.trim()}`);
  return parts.join("\n");
}
