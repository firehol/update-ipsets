import { ArrowDown, ArrowUp, ArrowUpDown } from "lucide-react";
import { HoverTip } from "@/components/editorial/hover-tip";
import { cn } from "@/lib/utils";
import type {
  SortDir,
  SortKey,
} from "@/components/admin/feeds-table-model";

export function FeedsTableHeader({
  sortKey,
  sortDir,
  onSort,
}: {
  sortKey: SortKey | null;
  sortDir: SortDir;
  onSort: (key: SortKey) => void;
}) {
  return (
    <thead className="sticky top-0 z-10 bg-card">
      <tr className="border-b border-border">
        <SortHeader
          label="Feed"
          sortKey="name"
          current={sortKey}
          dir={sortDir}
          onClick={onSort}
          className="w-[220px] pl-3 pr-3 text-left"
        />
        <SortHeader
          label="Category"
          sortKey="category"
          current={sortKey}
          dir={sortDir}
          onClick={onSort}
          className="w-[110px] text-left"
        />
        <th className="eyebrow w-[60px] py-3 px-2 text-center">Vis</th>
        <SortHeader
          label="Sched"
          sortKey="frequency"
          current={sortKey}
          dir={sortDir}
          onClick={onSort}
          className="w-[90px] text-right"
          hint="Configured frequency"
        />
        <SortHeader
          label="Actual"
          sortKey="actual_freq"
          current={sortKey}
          dir={sortDir}
          onClick={onSort}
          className="w-[140px] text-right"
          hint="Observed average update interval"
        />
        <SortHeader
          label="Next"
          sortKey="next_check"
          current={sortKey}
          dir={sortDir}
          onClick={onSort}
          className="w-[100px] text-right"
          hint="Next scheduled check"
        />
        <SortHeader
          label="Processed"
          sortKey="processed_date"
          current={sortKey}
          dir={sortDir}
          onClick={onSort}
          className="w-[110px] text-right"
          hint="Wall clock of last finalize()"
        />
        <SortHeader
          label="Why"
          sortKey="last_run_reason"
          current={sortKey}
          dir={sortDir}
          onClick={onSort}
          className="w-[120px] text-left"
          hint="Why this feed last ran"
        />
        <SortHeader
          label="Took"
          sortKey="last_processing_ms"
          current={sortKey}
          dir={sortDir}
          onClick={onSort}
          className="w-[80px] text-right"
          hint="How long the last run attempt took"
        />
        <SortHeader
          label="Upstream"
          sortKey="last_update"
          current={sortKey}
          dir={sortDir}
          onClick={onSort}
          className="w-[110px] text-right"
          hint="Last-Modified from upstream"
        />
        <SortHeader
          label="IPs"
          sortKey="unique_ips"
          current={sortKey}
          dir={sortDir}
          onClick={onSort}
          className="w-[80px] text-right"
        />
        <SortHeader
          label="Entries"
          sortKey="entries"
          current={sortKey}
          dir={sortDir}
          onClick={onSort}
          className="w-[80px] text-right"
        />
        <th className="eyebrow py-3 px-3 text-left">
          <HoverTip text="Scheduler state / last outcome / error text">
            <span className="cursor-help">State</span>
          </HoverTip>
        </th>
        <SortHeader
          label="Fail"
          sortKey="download_failures"
          current={sortKey}
          dir={sortDir}
          onClick={onSort}
          className="w-[50px] text-right"
          hint="Consecutive download failures"
        />
        <th className="eyebrow w-[70px] py-3 px-2 text-left">Files</th>
        <th className="eyebrow w-[40px] py-3 pr-3 text-right">
          <span className="sr-only">Public page</span>
        </th>
      </tr>
    </thead>
  );
}

function SortHeader({
  label,
  sortKey,
  current,
  dir,
  onClick,
  className,
  hint,
}: {
  label: string;
  sortKey: SortKey;
  current: SortKey | null;
  dir: SortDir;
  onClick: (key: SortKey) => void;
  className?: string;
  hint?: string;
}) {
  const active = current === sortKey;
  const nextSortDir = active && dir === "asc" ? "descending" : "ascending";
  const content = (
    <button
      type="button"
      onClick={() => onClick(sortKey)}
      aria-label={`Sort by ${label} ${nextSortDir}`}
      className={cn(
        "eyebrow inline-flex items-center gap-1 transition-colors",
        active ? "text-foreground" : "hover:text-foreground",
      )}
    >
      {label}
      {active ? (
        dir === "asc" ? (
          <ArrowUp className="h-3 w-3" />
        ) : (
          <ArrowDown className="h-3 w-3" />
        )
      ) : (
        <ArrowUpDown className="h-3 w-3 opacity-30" />
      )}
    </button>
  );
  return (
    <th
      scope="col"
      aria-sort={
        active
          ? dir === "asc"
            ? "ascending"
            : "descending"
          : "none"
      }
      className={cn("py-3 px-2", className)}
    >
      {hint ? <HoverTip text={hint}>{content}</HoverTip> : content}
    </th>
  );
}
