import { cn } from "@/lib/utils";

/**
 * BigNumber is the centrepiece of every hero. The number itself is
 * rendered in display-stat (32-48px) for the ordinary case and
 * display-hero (64-144px) for the headline use. The eyebrow label
 * sits above in tracked uppercase, the auxiliary line sits below in
 * muted body text.
 *
 * Critical: numbers always use tabular figures so they line up across
 * adjacent BigNumbers in a row. The .num utility class wires that up.
 */
export function BigNumber({
  value,
  label,
  caption,
  size = "md",
  align = "left",
  accent = false,
  className,
}: {
  value: React.ReactNode;
  label: string;
  caption?: React.ReactNode;
  size?: "md" | "lg" | "xl";
  align?: "left" | "right" | "center";
  accent?: boolean;
  className?: string;
}) {
  const sizeClass =
    size === "xl" ? "display-hero" : size === "lg" ? "display-title" : "display-stat";
  const alignClass = align === "right" ? "text-right" : align === "center" ? "text-center" : "text-left";
  return (
    <div className={cn(alignClass, className)}>
      <div className="eyebrow">{label}</div>
      <div
        className={cn(
          "num mt-2",
          sizeClass,
          accent ? "text-primary" : "text-foreground",
        )}
      >
        {value}
      </div>
      {caption && (
        <div className="mt-2 text-sm text-muted-foreground">{caption}</div>
      )}
    </div>
  );
}
