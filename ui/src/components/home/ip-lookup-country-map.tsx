import { useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useChartTheme } from "@/lib/chart-theme";
import { ISO_NUM_TO_A2 } from "@/lib/iso-codes";
import { buildWorldPaths } from "@/lib/world-geographies";
import { useWorldTopology } from "@/lib/world-topology";

const MAP_WIDTH = 980;

export function IPLookupCountryMap({
  countryCode,
  height = 220,
}: {
  countryCode?: string;
  height?: number;
}) {
  const topologyQuery = useWorldTopology();
  const navigate = useNavigate();
  const chartTheme = useChartTheme();
  const [hoveredCode, setHoveredCode] = useState<string | null>(null);
  const activeCode = countryCode?.trim().toUpperCase() ?? "";
  const mappableCountryCode = /^[A-Z]{2}$/.test(activeCode) ? activeCode : "";
  const geographies = useMemo(
    () =>
      topologyQuery.data
        ? buildWorldPaths({
            topology: topologyQuery.data,
            width: MAP_WIDTH,
            height,
            scale: 150,
          })
        : [],
    [height, topologyQuery.data],
  );

  if (!mappableCountryCode) {
    return (
      <div
        className="flex items-center justify-center px-4 text-sm text-muted-foreground"
        style={{ height }}
      >
        No geographic position is available for this IP.
      </div>
    );
  }

  if (!topologyQuery.data) {
    return (
      <div
        className="flex items-center justify-center px-4 text-sm text-muted-foreground"
        style={{ height }}
      >
        {topologyQuery.isError
          ? "Country boundaries are unavailable."
          : "Loading map…"}
      </div>
    );
  }

  return (
    <svg
      viewBox={`0 0 ${MAP_WIDTH} ${height}`}
      width="100%"
      height={height}
      role="img"
      aria-label="IP country map"
    >
      {geographies.map((geo) => {
        const code = ISO_NUM_TO_A2[geo.id as keyof typeof ISO_NUM_TO_A2];
        const active = code === mappableCountryCode;
        const hovered = code === hoveredCode;
        const handleActivate = () => {
          if (!code) return;
          navigate(`/countries/${code}`);
        };
        const handleKeyDown = (e: React.KeyboardEvent<SVGPathElement>) => {
          if (!code) return;
          if (e.key !== "Enter" && e.key !== " ") return;
          e.preventDefault();
          handleActivate();
        };
        return (
          <path
            key={geo.key}
            d={geo.path}
            fill={
              active
                ? chartTheme.accent
                : hovered
                  ? chartTheme.grid
                  : chartTheme.context
            }
            stroke={active || hovered ? chartTheme.accent : chartTheme.grid}
            strokeWidth={active ? 0.8 : 0.45}
            onMouseEnter={code ? () => setHoveredCode(code) : undefined}
            onMouseLeave={code ? () => setHoveredCode(null) : undefined}
            onClick={code ? handleActivate : undefined}
            onKeyDown={code ? handleKeyDown : undefined}
            role={code ? "link" : undefined}
            tabIndex={code ? 0 : undefined}
            aria-label={code ? `Open ${code} country detail` : undefined}
            style={{ cursor: code ? "pointer" : undefined, outline: "none" }}
          />
        );
      })}
    </svg>
  );
}
