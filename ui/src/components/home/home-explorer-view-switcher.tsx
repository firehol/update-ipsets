import type { SortKey, ViewMode } from "@/lib/explorer-state";
import { cn } from "@/lib/utils";

const SORT_OPTIONS: Array<{ id: SortKey; label: string }> = [
  { id: "freshest", label: "Freshest" },
  { id: "coverage", label: "Largest coverage" },
  { id: "unique", label: "Most unique" },
  { id: "name", label: "Name" },
  { id: "maintainer", label: "Maintainer" },
];

const VIEW_OPTIONS: Array<{ id: ViewMode; label: string }> = [
  { id: "cards", label: "Cards" },
  { id: "table", label: "Table" },
  { id: "treemap", label: "Treemap" },
  { id: "timeline", label: "Timeline" },
  { id: "maintainers", label: "By maintainer" },
];

export function HomeExplorerViewSwitcher({
  sort,
  view,
  onSortChange,
  onViewChange,
}: {
  sort: SortKey;
  view: ViewMode;
  onSortChange: (sort: SortKey) => void;
  onViewChange: (view: ViewMode) => void;
}) {
  const sortDisabled = view === "treemap" || view === "timeline" || view === "maintainers";
  return (
    <div className="flex flex-wrap items-center justify-between gap-4 border-b border-border pb-4">
      <div
        className={cn(
          "flex items-center gap-3",
          sortDisabled && "opacity-40",
        )}
        aria-hidden={sortDisabled}
      >
        <span className="eyebrow text-muted-foreground">Sort</span>
        <div className="flex flex-wrap gap-1">
          {SORT_OPTIONS.map((opt) => {
            const active = sort === opt.id;
            return (
              <button
                key={opt.id}
                type="button"
                onClick={() => !sortDisabled && onSortChange(opt.id)}
                disabled={sortDisabled}
                className={cn(
                  "inline-flex h-8 items-center border px-3 text-[12px] font-medium transition",
                  active
                    ? "border-primary bg-primary/10 text-foreground"
                    : "border-transparent text-muted-foreground hover:border-border hover:text-foreground",
                  sortDisabled && "cursor-not-allowed",
                )}
              >
                {opt.label}
              </button>
            );
          })}
        </div>
      </div>

      <div className="flex items-center gap-3">
        <span className="eyebrow text-muted-foreground">View</span>
        <div className="flex flex-wrap gap-1">
          {VIEW_OPTIONS.map((opt) => {
            const active = view === opt.id;
            return (
              <button
                key={opt.id}
                type="button"
                onClick={() => onViewChange(opt.id)}
                className={cn(
                  "inline-flex h-8 items-center border px-3 text-[12px] font-medium transition",
                  active
                    ? "border-primary bg-primary/10 text-foreground"
                    : "border-transparent text-muted-foreground hover:border-border hover:text-foreground",
                )}
              >
                {opt.label}
              </button>
            );
          })}
        </div>
      </div>
    </div>
  );
}
