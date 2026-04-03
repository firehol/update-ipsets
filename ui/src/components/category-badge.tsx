import { cn } from "@/lib/utils";
import { useCategoryMap } from "@/lib/categories";

function alpha(hex: string, opacity: number): string {
  const value = hex.trim();
  if (!/^#[0-9a-fA-F]{6}$/.test(value)) {
    return "";
  }
  const r = Number.parseInt(value.slice(1, 3), 16);
  const g = Number.parseInt(value.slice(3, 5), 16);
  const b = Number.parseInt(value.slice(5, 7), 16);
  return `rgba(${r}, ${g}, ${b}, ${opacity})`;
}

export function CategoryBadge({
  category,
  className,
}: {
  category: string;
  className?: string;
}) {
  const categories = useCategoryMap();
  const meta = categories.get(category);
  const color = meta?.color;
  const label = meta?.label ?? category;
  const style =
    color && /^#[0-9a-fA-F]{6}$/.test(color)
      ? {
          color,
          backgroundColor: alpha(color, 0.18),
          borderColor: alpha(color, 0.4),
        }
      : undefined;

  return (
    <span
      className={cn(
        "inline-flex items-center rounded-md border px-2.5 py-0.5 text-[11px] font-semibold uppercase tracking-[0.1em] leading-tight",
        !style && "bg-muted text-muted-foreground border-border",
        className,
      )}
      style={style}
    >
      {label}
    </span>
  );
}
