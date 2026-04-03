import { cn } from "@/lib/utils";

/**
 * Thin horizontal accent stroke. Used to anchor a section header
 * (above the title) or as a section divider. The default colour is
 * the primary accent (FireHOL red) but a neutral variant is available
 * for less prominent uses.
 */
export function AccentBar({
  className,
  variant = "primary",
}: {
  className?: string;
  variant?: "primary" | "neutral";
}) {
  return (
    <div
      className={cn(
        "h-[3px] w-12",
        variant === "primary" ? "bg-primary" : "bg-border",
        className,
      )}
    />
  );
}
