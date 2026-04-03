import { type ReactNode } from "react";
import { cn } from "@/lib/utils";

/**
 * Editorial provider tab strip. Used by the AS Composition and
 * Geographic Coverage sections to switch between data providers.
 *
 * Visual style:
 *   - Thin hairline base, no card chrome
 *   - Active tab gets a 2px primary underline AND bolder type
 *   - Inactive tabs are muted, lifting on hover
 *   - Generous gap between tabs
 *
 * Mirrors Apple's tab patterns: restrained, deliberate, no fill.
 */
export function ProviderTabBar({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
      role="tablist"
      className={cn("flex flex-wrap gap-x-8 gap-y-2 border-b border-border", className)}
    >
      {children}
    </div>
  );
}

export function ProviderTab({
  label,
  active,
  onClick,
}: {
  label: ReactNode;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      role="tab"
      aria-selected={active}
      onClick={onClick}
      className={cn(
        "group relative -mb-px inline-flex items-center gap-2 border-b-2 pb-4 pt-2 text-[14px] transition-colors",
        active
          ? "border-primary font-semibold text-foreground"
          : "border-transparent text-muted-foreground hover:text-foreground",
      )}
    >
      <span>{label}</span>
    </button>
  );
}

/**
 * View tabs — the secondary tab strip below the provider strip
 * (Treemap | Bubble | List, Map | List, etc). Smaller and tighter
 * than the provider tabs.
 */
export function ViewTabBar({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
      role="tablist"
      className={cn("flex gap-1 rounded-sm bg-muted p-1", className)}
    >
      {children}
    </div>
  );
}

export function ViewTab({
  label,
  active,
  onClick,
}: {
  label: string;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      role="tab"
      aria-selected={active}
      onClick={onClick}
      className={cn(
        "rounded-[3px] px-4 py-1.5 text-[12px] font-medium transition-colors",
        active
          ? "bg-card text-foreground shadow-[0_1px_0_rgba(0,0,0,0.04)]"
          : "text-muted-foreground hover:text-foreground",
      )}
    >
      {label}
    </button>
  );
}
