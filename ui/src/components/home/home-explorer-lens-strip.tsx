import { LENSES, type LensDefinition } from "@/lib/explorer-state";
import { cn } from "@/lib/utils";

export function HomeExplorerLensStrip({
  activeLens,
  onSelect,
}: {
  activeLens: string | null;
  onSelect: (lens: LensDefinition) => void;
}) {
  return (
    <div className="relative border-b border-border">
      <div className="grid grid-cols-1 gap-2 pb-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
        {LENSES.map((lens) => {
          const active = activeLens === lens.id;
          return (
            <button
              key={lens.id}
              type="button"
              onClick={() => onSelect(lens)}
              className={cn(
                "group flex min-h-[9.25rem] w-full flex-col items-start gap-1 border px-4 py-4 text-left transition",
                active
                  ? "border-primary bg-primary/5 text-foreground"
                  : "border-border bg-card text-foreground hover:border-primary/60",
              )}
            >
              <span className="eyebrow text-muted-foreground">Lens</span>
              <span className="text-[14px] font-semibold tracking-tight">
                {lens.label}
              </span>
              <span className="text-[12px] leading-snug text-muted-foreground whitespace-normal text-pretty">
                {lens.description}
              </span>
            </button>
          );
        })}
      </div>
    </div>
  );
}
