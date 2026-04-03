import { type ReactNode } from "react";
import { Activity, AlertCircle } from "lucide-react";
import { cn } from "@/lib/utils";
import type { AdminStatus } from "@/lib/api-types";
import { HoverTip } from "@/components/editorial/hover-tip";
import { formatBytes, formatNumber, relativeTime } from "@/lib/admin-format";
import type { FeedHealthFilter } from "@/lib/admin-format";
import { parseGoTime } from "@/components/admin/current-run-shared";

/**
 * Heartbeat strip — the operator's "is anything on fire" panel.
 *
 * Renders a single row of hairline-divided tiles:
 *
 *   Daemon state · Healthy · Delayed · Risky · Unavailable · Archived · Empty · Unmaintained
 *
 * Followed by a compact row of runtime vitals (uptime, memory,
 * goroutines, disk, integrity).
 *
 * Every health tile in the first row is clickable and drives the
 * feeds-table filter so operators can go from "3 errors" to "here
 * are the 3 errors" in one click. The Daemon tile is highlighted
 * by the accent rule when a run is in progress so "the daemon is
 * doing something right now" is the most visually prominent pixel
 * on the page.
 *
 * Uses a local AdminTile grid instead of the shared StatRow /
 * StatTile because the admin needs 8 columns and the public
 * editorial StatRow is capped at 4.
 */
export function HeartbeatPanel({
  data,
  loading,
  error,
  integrityCount,
  onFilterByHealth,
}: {
  data: AdminStatus | undefined;
  loading: boolean;
  error: unknown;
  integrityCount: number;
  onFilterByHealth: (h: FeedHealthFilter | null) => void;
}) {
  if (loading) {
    return <div className="mb-12 h-32 animate-pulse bg-muted/40" />;
  }
  if (error) {
    return (
      <div className="mb-12 flex items-center gap-3 border border-destructive/40 bg-destructive/[0.03] px-6 py-6 text-sm text-destructive">
        <AlertCircle className="h-5 w-5" />
        <span>Could not reach the admin API: {(error as Error).message}</span>
      </div>
    );
  }
  if (!data) return null;

  const f = data.feeds;
  const running = data.engine.running;
  const lastReport = data.engine.last_report;
  const lastRunSize =
    (lastReport?.updated?.length ?? 0) +
    (lastReport?.skipped?.length ?? 0) +
    (lastReport?.failed?.length ?? 0);
  const heapPercent = data.system.heap_alloc / (2 * 1024 * 1024 * 1024);

  return (
    <section className="mb-12">
      <div className="mb-5 flex items-center gap-2">
        <Activity className="h-4 w-4 text-muted-foreground" />
        <span className="eyebrow">Heartbeat</span>
        <span className="ml-auto text-xs text-muted-foreground">
          {data.system.goos}/{data.system.goarch} · {data.system.go_version}
        </span>
      </div>

      {/* Row A — requested feed-health classes. Eight tiles, clickable. */}
      <AdminTileGrid cols={8}>
        <AdminTile
          label="Daemon"
          value={
            <span className={running ? "text-status-healthy" : "text-foreground"}>
              {running ? "RUNNING" : "Idle"}
            </span>
          }
          caption={
            running
              ? `${lastRunSize} feeds in last run`
              : "waiting for next tick"
          }
          accent={running}
        />
        <AdminTile
          label="Healthy"
          value={formatNumber(f.healthy)}
          caption={`of ${f.total_enabled} enabled`}
          valueClass="text-status-healthy"
          onClick={() => onFilterByHealth("healthy")}
        />
        <AdminTile
          label="Delayed"
          value={formatNumber(f.delayed)}
          caption="behind healthy cadence"
          valueClass={f.delayed > 0 ? "text-status-delayed" : "text-foreground"}
          onClick={() => onFilterByHealth("delayed")}
        />
        <AdminTile
          label="Risky"
          value={formatNumber(f.risky)}
          caption="past risky cadence"
          valueClass={f.risky > 0 ? "text-status-risky" : "text-foreground"}
          onClick={() => onFilterByHealth("risky")}
        />
        <AdminTile
          label="Unavailable"
          value={formatNumber(f.unavailable)}
          caption="no successful local publication yet or beyond recovery threshold"
          valueClass={
            f.unavailable > 0 ? "text-destructive" : "text-foreground"
          }
          onClick={() => onFilterByHealth("unavailable")}
        />
        <AdminTile
          label="Archived"
          value={formatNumber(f.archived)}
          caption="automatic retries disabled"
          valueClass={f.archived > 0 ? "text-slate-500" : "text-foreground"}
          onClick={() => onFilterByHealth("archived")}
        />
        <AdminTile
          label="Empty"
          value={formatNumber(f.empty)}
          caption="download works, zero entries"
          valueClass={f.empty > 0 ? "text-status-warning" : "text-foreground"}
          onClick={() => onFilterByHealth("empty")}
        />
        <AdminTile
          label="Unmaintained"
          value={formatNumber(f.unmaintained)}
          caption="past abandonment threshold"
          valueClass={
            f.unmaintained > 0 ? "text-destructive" : "text-foreground"
          }
          onClick={() => onFilterByHealth("unmaintained")}
        />
      </AdminTileGrid>

      {/* Row B — runtime vitals. Six tiles, tighter padding. */}
      <div className="mt-px">
        <AdminTileGrid cols={6} dense>
          <AdminTile
            dense
            label="Uptime"
            value={data.system.uptime}
            caption={
              <>
                {f.total_configured} feeds configured
                {data.metrics?.snapshot_persist_errors != null &&
                  data.metrics.snapshot_persist_errors > 0 && (
                    <span className="text-status-warning">
                      {" "}
                      · {data.metrics.snapshot_persist_errors} snapshot errors
                    </span>
                  )}
              </>
            }
          />
          <AdminTile
            dense
            label="Config"
            value={
              data.engine.config_reload_count != null &&
              data.engine.config_reload_count > 0
                ? String(data.engine.config_reload_count)
                : "—"
            }
            caption={
              data.engine.last_config_reload_error ? (
                <span className="text-destructive">
                  {data.engine.last_config_reload_error}
                </span>
              ) : data.engine.last_config_reload ? (
                `last: ${relativeTime(parseGoTime(data.engine.last_config_reload))}`
              ) : (
                "no reloads"
              )
            }
            valueClass={
              data.engine.last_config_reload_error
                ? "text-destructive"
                : undefined
            }
          />
          <AdminTile
            dense
            label="Heap"
            value={formatBytes(data.system.heap_alloc)}
            caption={`${(heapPercent * 100).toFixed(0)}% of 2 GiB · sys ${formatBytes(data.system.sys)}`}
          />
          <AdminTile
            dense
            label="Goroutines"
            value={formatNumber(data.system.goroutines)}
            caption={`GC ${data.system.num_gc}`}
          />
          <AdminTile
            dense
            label="Disk free"
            value={data.system.disk_free}
            caption="data volume"
          />
          <AdminTile
            dense
            label="Integrity"
            value={formatNumber(integrityCount)}
            caption={
              data.engine.startup_repair_deferred ? (
                <span className="text-status-warning">
                  {data.engine.startup_repair_deferred_targets} repairs deferred
                </span>
              ) : integrityCount > 0 ? (
                "broken pipeline runs"
              ) : (
                "all clean"
              )
            }
            valueClass={
              integrityCount > 0 ? "text-destructive" : "text-status-healthy"
            }
            onClick={() => {
              document
                .getElementById("admin-integrity-panel")
                ?.scrollIntoView({ behavior: "smooth", block: "start" });
            }}
          />
        </AdminTileGrid>
      </div>
    </section>
  );
}

/* -------------------------------------------------------------------------- */

/**
 * Horizontal row of hairline-divided admin tiles. Local to the
 * admin console — the public editorial StatRow caps at 4 cols
 * and the operator needs more density. `dense` shaves the vertical
 * padding for secondary rows so primary and secondary rows have
 * a clear visual hierarchy.
 */
function AdminTileGrid({
  children,
	cols,
	dense,
}: {
	children: ReactNode;
	cols: 5 | 6 | 7 | 8;
	dense?: boolean;
}) {
	const colClass =
		cols === 8
			? "md:grid-cols-4 xl:grid-cols-8"
			: cols === 7
			? "md:grid-cols-7"
			: cols === 6
				? "md:grid-cols-6"
        : "md:grid-cols-5";
  return (
    <div
      className={cn(
        "grid grid-cols-2 gap-px overflow-hidden rounded-sm border border-border bg-border",
        colClass,
        dense && "opacity-95",
      )}
    >
      {children}
    </div>
  );
}

/**
 * Admin-local stat tile. Visually matches the editorial StatTile
 * but supports click-through for filter shortcuts and a `dense`
 * variant with tighter padding. When clickable, shows explicit
 * affordance: hover border, pointer cursor, action chevron in
 * the top-right corner so operators can tell at a glance which
 * tiles are interactive.
 */
function AdminTile({
  label,
  value,
  caption,
  accent = false,
  valueClass,
  onClick,
  dense,
}: {
  label: string;
  value: ReactNode;
  caption?: ReactNode;
  accent?: boolean;
  valueClass?: string;
  onClick?: () => void;
  dense?: boolean;
}) {
  const isClickable = Boolean(onClick);
  const body = (
    <div
      className={cn(
        "group relative h-full bg-card",
        dense ? "px-5 py-5" : "px-6 py-7",
        isClickable &&
          "cursor-pointer transition-all hover:bg-muted/40 hover:ring-2 hover:ring-inset hover:ring-primary/60",
      )}
    >
      {accent && (
        <span className="absolute left-0 top-0 h-[3px] w-10 bg-primary" />
      )}
      {isClickable && (
        <span className="absolute right-3 top-3 text-[10px] uppercase tracking-wider text-muted-foreground/60 opacity-0 transition-opacity group-hover:opacity-100">
          filter ›
        </span>
      )}
      <div className="eyebrow">{label}</div>
      <div
        className={cn(
          "num mt-3 text-foreground tabular-nums",
          dense ? "text-[28px] font-semibold leading-none" : "display-stat",
        )}
      >
        {valueClass ? <span className={valueClass}>{value}</span> : value}
      </div>
      {caption && (
        <div
          className={cn(
            "mt-2 tabular-nums text-muted-foreground",
            dense ? "text-xs" : "text-sm",
          )}
        >
          {caption}
        </div>
      )}
    </div>
  );

  if (!onClick) return body;

  return (
    <HoverTip text={`Click to filter the table: ${label.toLowerCase()}`}>
      <button
        type="button"
        onClick={onClick}
        className="block h-full w-full text-left focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        {body}
      </button>
    </HoverTip>
  );
}
