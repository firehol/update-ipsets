import { type ReactNode } from "react";
import { cn } from "@/lib/utils";

/**
 * Horizontal row of editorial stat tiles. Each tile is divided from the
 * next by a thin vertical hairline rather than card chrome. The first
 * tile is identified as the "count" by a thin accent rule above its
 * eyebrow label.
 *
 * This replaces the shadcn Card-based provider tile row with something
 * more refined and Apple-feel.
 *
 * `cols` controls how many tiles fit in a row on md+ screens. Defaults
 * to 4 (the original four-up layout). Use `cols={2}` for sections that
 * have two structural facts that deserve equal weight.
 */
export function StatRow({
  children,
  className,
  cols = 4,
}: {
  children: ReactNode;
  className?: string;
  cols?: 2 | 3 | 4;
}) {
  // Tailwind needs literal class names so JIT can find them — switch
  // on the prop instead of building the class string dynamically.
  const colClass =
    cols === 2 ? "md:grid-cols-2" : cols === 3 ? "md:grid-cols-3" : "md:grid-cols-4";
  return (
    <div
      className={cn(
        "grid grid-cols-2 gap-px overflow-hidden rounded-sm border border-border bg-border",
        colClass,
        className,
      )}
    >
      {children}
    </div>
  );
}

export function StatTile({
  label,
  value,
  caption,
  accent = false,
  className,
}: {
  label: string;
  value: ReactNode;
  caption?: ReactNode;
  accent?: boolean;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "relative bg-card px-6 py-7",
        className,
      )}
    >
      {accent && <span className="absolute left-0 top-0 h-[3px] w-10 bg-primary" />}
      <div className="eyebrow">{label}</div>
      <div className="num mt-3 display-stat text-foreground">{value}</div>
      {caption && (
        <div className="mt-2 text-sm text-muted-foreground tabular-nums">{caption}</div>
      )}
    </div>
  );
}
