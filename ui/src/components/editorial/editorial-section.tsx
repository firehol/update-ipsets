import { type ReactNode } from "react";
import { cn } from "@/lib/utils";

/**
 * Top-level page section in the editorial layout. Each section feels
 * anchored: a small eyebrow, a generous display title, an optional narrow
 * intro lede, then the section body at full container width. Sections
 * are separated by generous vertical breathing room (no card chrome).
 *
 * This is the primitive that replaces shadcn Card across the rewrite —
 * the goal is "Apple product page section", not "SaaS dashboard card".
 */
export function EditorialSection({
  eyebrow,
  title,
  lede,
  children,
  id,
  className,
  bleed = false,
  dark = false,
}: {
  eyebrow?: string;
  title: string;
  lede?: ReactNode;
  children: ReactNode;
  id?: string;
  className?: string;
  /** Removes the page container so children can extend full-bleed. */
  bleed?: boolean;
  /** Switches the section to a dark display surface for visual rhythm. */
  dark?: boolean;
}) {
  const surfaceClass = dark
    ? "bg-display text-display-fg border-y border-display-border"
    : "";
  return (
    <section
      id={id}
      className={cn("scroll-mt-24 py-20 md:py-28", surfaceClass, className)}
    >
      <div className={bleed ? "" : "page-container"}>
        <header className="mb-12">
          {eyebrow && (
            <div
              className={cn(
                "eyebrow mb-3",
                dark ? "text-display-muted" : undefined,
              )}
            >
              {eyebrow}
            </div>
          )}
          <h2
            className={cn(
              "display-title",
              dark ? "text-display-fg" : "text-foreground",
            )}
          >
            {title}
          </h2>
          {lede && (
            <p
              className={cn(
                "lede mt-6",
                dark ? "text-display-muted" : undefined,
              )}
            >
              {lede}
            </p>
          )}
        </header>
        {children}
      </div>
    </section>
  );
}

/**
 * Subsection inside an editorial section. Uses tighter typography and
 * a thin top border for separation. Mirrors how Apple stacks supporting
 * panels under a primary section title.
 */
export function EditorialSubsection({
  title,
  description,
  children,
  className,
}: {
  title: string;
  description?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("mt-16 border-t border-border pt-12 first:mt-0 first:border-t-0 first:pt-0", className)}>
      <h3 className="display-subtitle text-foreground">{title}</h3>
      {description && (
        <p className="mt-3 text-base text-muted-foreground">{description}</p>
      )}
      <div className="mt-8">{children}</div>
    </div>
  );
}
