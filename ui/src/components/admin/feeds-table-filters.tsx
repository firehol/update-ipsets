import { memo } from "react";
import { X } from "lucide-react";
import { cn } from "@/lib/utils";

export const MultiSelectChipRow = memo(function MultiSelectChipRow({
  label,
  filters,
  selected,
  onToggle,
  onClear,
  counts,
}: {
  label: string;
  filters: { id: string; label: string }[];
  selected: string[];
  onToggle: (id: string) => void;
  onClear: () => void;
  counts: Record<string, number>;
}) {
  if (filters.length === 0) return null;
  return (
    <div className="flex flex-wrap items-center gap-2">
      <span className="eyebrow min-w-[72px]">{label}</span>
      <div className="flex flex-wrap gap-1.5">
        {filters.map((f) => {
          const count = counts[f.id] ?? 0;
          const isActive = selected.includes(f.id);
          return (
            <button
              key={f.id}
              type="button"
              onClick={() => onToggle(f.id)}
              className={cn(
                "inline-flex items-center gap-1.5 rounded-sm border px-3 py-1.5 text-xs font-medium transition-colors",
                isActive
                  ? "border-primary bg-primary text-primary-foreground"
                  : "border-border bg-card text-muted-foreground hover:border-foreground hover:text-foreground",
              )}
            >
              <span>{f.label}</span>
              <span
                className={cn(
                  "tabular-nums",
                  isActive
                    ? "text-primary-foreground/80"
                    : "text-muted-foreground",
                )}
              >
                {count}
              </span>
            </button>
          );
        })}
      </div>
      {selected.length > 0 && (
        <button
          type="button"
          onClick={onClear}
          className="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
        >
          <X className="h-3 w-3" />
          clear
        </button>
      )}
    </div>
  );
});
