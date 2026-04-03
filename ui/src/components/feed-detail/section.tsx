import { type ComponentType, type ReactNode, type SVGProps } from "react";
import { cn } from "@/lib/utils";
import { AccentBar } from "@/components/editorial/accent-bar";

type SectionIcon = ComponentType<SVGProps<SVGSVGElement>>;

/**
 * Detail-page section wrapper. Each section renders as a generously
 * spaced editorial unit with:
 *   - Thin accent bar (optionally tinted per category)
 *   - Tracked uppercase eyebrow
 *   - Display-titled section header (optionally preceded by a small icon)
 *   - Optional narrow lede paragraph
 *   - Section body at full container width
 *
 * The first section after the hero gets reduced top padding so it
 * doesn't fight the hero's bottom edge. Pass `tight` to enable that.
 *
 * `accentColor` (a hex string from CategoryMeta.color) tints the accent
 * bar and the icon swatch so each section reads as its own chapter
 * instead of a copy of the previous one.
 */
export function DetailSection({
  eyebrow,
  title,
  lede,
  children,
  id,
  className,
  tight = false,
  icon: Icon,
  accentColor,
  density = "normal",
}: {
  eyebrow?: string;
  title: string;
  lede?: ReactNode;
  children: ReactNode;
  id?: string;
  className?: string;
  tight?: boolean;
  icon?: SectionIcon;
  accentColor?: string | null;
  density?: "normal" | "footer";
}) {
  const bar =
    accentColor && /^#[0-9a-fA-F]{6}$/.test(accentColor) ? (
      <div className="h-[3px] w-12" style={{ backgroundColor: accentColor }} />
    ) : (
      <AccentBar />
    );
  const swatch = Icon ? (
    <span
      className="inline-flex h-9 w-9 items-center justify-center rounded-md border border-border"
      style={
        accentColor && /^#[0-9a-fA-F]{6}$/.test(accentColor)
          ? {
              color: accentColor,
              backgroundColor: `${accentColor}1f`,
              borderColor: `${accentColor}55`,
            }
          : undefined
      }
    >
      <Icon className="h-4 w-4" aria-hidden="true" />
    </span>
  ) : null;
  const padding =
    density === "footer" ? "py-12 md:py-14" : tight ? "py-20" : "py-24 md:py-28";
  return (
    <section id={id} className={cn("scroll-mt-24", padding, className)}>
      <header className={density === "footer" ? "mb-6" : "mb-12"}>
        {density === "footer" ? null : bar}
        {eyebrow && (
          <div className={cn("eyebrow", density === "footer" ? "" : "mt-6")}>
            {eyebrow}
          </div>
        )}
        <div className={cn("flex items-center gap-3", density === "footer" ? "mt-1" : "mt-3")}>
          {swatch}
          <h2 className={density === "footer" ? "text-base font-semibold text-foreground" : "display-title"}>
            {title}
          </h2>
        </div>
        {lede && (
          <p className={cn(density === "footer" ? "mt-3 text-sm text-muted-foreground" : "lede mt-6")}>
            {lede}
          </p>
        )}
      </header>
      {children}
    </section>
  );
}

/**
 * A subsection inside a DetailSection. Lighter chrome — no accent bar,
 * just a generous top-margin and a thin top hairline. Used to stack
 * the rfc_reserved + third-party-bogons subsections inside the Bogons
 * section, etc.
 */
export function DetailSubsection({
  title,
  description,
  children,
  className,
  icon: Icon,
  accentColor,
}: {
  title: string;
  description?: ReactNode;
  children: ReactNode;
  className?: string;
  icon?: SectionIcon;
  accentColor?: string | null;
}) {
  return (
    <div className={cn("mt-20 border-t border-border pt-12 first:mt-0 first:border-t-0 first:pt-0", className)}>
      <div className="flex items-center gap-3">
        {Icon ? (
          <span
            className="inline-flex h-7 w-7 items-center justify-center rounded-md border border-border"
            style={
              accentColor && /^#[0-9a-fA-F]{6}$/.test(accentColor)
                ? {
                    color: accentColor,
                    backgroundColor: `${accentColor}1f`,
                    borderColor: `${accentColor}55`,
                  }
                : undefined
            }
          >
            <Icon className="h-3.5 w-3.5" aria-hidden="true" />
          </span>
        ) : null}
        <h3 className="display-subtitle">{title}</h3>
      </div>
      {description && <p className="mt-3 text-base text-muted-foreground">{description}</p>}
      <div className="mt-8">{children}</div>
    </div>
  );
}

/**
 * Small editorial notice box for feed-detail sections.
 *
 * Used for chart/visualization states that are meaningful to the reader:
 * loading failures, empty-but-valid states, partiality warnings, and similar.
 * Keep the copy factual and local to the section.
 */
export function DetailNotice({
  title,
  children,
  tone = "info",
  className,
}: {
  title: ReactNode;
  children: ReactNode;
  tone?: "info" | "warning" | "danger";
  className?: string;
}) {
  return (
    <div
      className={cn(
        "border-l-[3px] px-5 py-4",
        tone === "danger"
          ? "border-destructive/80 bg-destructive/5"
          : tone === "warning"
            ? "border-amber-500/80 bg-amber-500/5"
            : "border-primary/70 bg-muted/30",
        className,
      )}
    >
      <h4 className="text-[14px] font-semibold text-foreground">{title}</h4>
      <div className="mt-2 text-sm leading-relaxed text-muted-foreground">{children}</div>
    </div>
  );
}

/**
 * Two-column editorial panel grid. Each column can contribute:
 *   - title
 *   - optional description
 *   - optional local notices
 *   - body
 *
 * When both columns are shown side-by-side, the rows align across the pair so
 * one column's notices cannot push its chart lower than the sibling column.
 */
export function DetailTwoColumnPanels({
  left,
  right,
  className,
}: {
  left: DetailPanelSpec;
  right: DetailPanelSpec;
  className?: string;
}) {
  const showDescriptions = Boolean(left.description || right.description);
  const showNotices = (left.notices?.length ?? 0) > 0 || (right.notices?.length ?? 0) > 0;
  const rowTemplate = [
    "auto",
    showDescriptions ? "auto" : null,
    showNotices ? "auto" : null,
    "minmax(0, 1fr)",
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <div
      className={cn("grid grid-cols-1 gap-12 lg:grid-cols-2 lg:gap-x-12 lg:gap-y-6", className)}
      style={{ gridTemplateRows: rowTemplate }}
    >
      <DetailPanelColumn
        title={left.title}
        description={left.description}
        notices={left.notices}
        bodyClassName={left.bodyClassName}
        showDescriptions={showDescriptions}
        showNotices={showNotices}
      >
        {left.children}
      </DetailPanelColumn>
      <DetailPanelColumn
        title={right.title}
        description={right.description}
        notices={right.notices}
        bodyClassName={right.bodyClassName}
        showDescriptions={showDescriptions}
        showNotices={showNotices}
      >
        {right.children}
      </DetailPanelColumn>
    </div>
  );
}

interface DetailPanelSpec {
  title: ReactNode;
  description?: ReactNode;
  notices?: ReactNode[];
  children: ReactNode;
  bodyClassName?: string;
}

function DetailPanelColumn({
  title,
  description,
  notices,
  children,
  bodyClassName,
  showDescriptions,
  showNotices,
}: DetailPanelSpec & {
  showDescriptions: boolean;
  showNotices: boolean;
}) {
  const hasNotices = (notices?.length ?? 0) > 0;

  return (
    <div className="flex min-w-0 flex-col gap-6 lg:row-span-full lg:grid lg:grid-rows-subgrid lg:gap-0">
      <div>
        <h3 className="display-subtitle">{title}</h3>
      </div>
      {showDescriptions && (
        <div>
          {description ? (
            <p className="text-base leading-relaxed text-muted-foreground">{description}</p>
          ) : null}
        </div>
      )}
      {showNotices && <div className={hasNotices ? "space-y-4" : undefined}>{notices}</div>}
      <div className={cn("min-w-0", bodyClassName)}>{children}</div>
    </div>
  );
}
