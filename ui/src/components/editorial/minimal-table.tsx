import { type ReactNode } from "react";
import { cn } from "@/lib/utils";

/**
 * Editorial table primitives. Replaces the shadcn Table chrome with a
 * minimal hairline-only design: no row stripes, no card backplate,
 * generous row height, tabular numerals, uppercase eyebrow headers.
 *
 * The goal is "Apple specs table" not "SaaS data grid". Use these as
 * the building blocks for any table-like layout in the rewrite.
 */
export function MinimalTable({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div className={cn("overflow-x-auto", className)}>
      <table className="w-full border-collapse text-[15px]">{children}</table>
    </div>
  );
}

export function MinimalTableHead({ children }: { children: ReactNode }) {
  return (
    <thead>
      <tr className="border-b border-border">{children}</tr>
    </thead>
  );
}

export function MinimalTableHeader({
  children,
  align = "left",
  className,
}: {
  children: ReactNode;
  align?: "left" | "right" | "center";
  className?: string;
}) {
  return (
    <th
      className={cn(
        "eyebrow py-4 px-3 first:pl-0 last:pr-0",
        align === "right" && "text-right",
        align === "center" && "text-center",
        align === "left" && "text-left",
        className,
      )}
    >
      {children}
    </th>
  );
}

export function MinimalTableBody({ children }: { children: ReactNode }) {
  return <tbody>{children}</tbody>;
}

export function MinimalTableRow({
  children,
  onClick,
  className,
}: {
  children: ReactNode;
  onClick?: () => void;
  className?: string;
}) {
  return (
    <tr
      onClick={onClick}
      className={cn(
        "border-b border-border/60 transition-colors hover:bg-muted/40",
        onClick && "cursor-pointer",
        className,
      )}
    >
      {children}
    </tr>
  );
}

export function MinimalTableCell({
  children,
  align = "left",
  mono = false,
  num = false,
  muted = false,
  className,
}: {
  children: ReactNode;
  align?: "left" | "right" | "center";
  mono?: boolean;
  num?: boolean;
  muted?: boolean;
  className?: string;
}) {
  return (
    <td
      className={cn(
        "py-5 px-3 first:pl-0 last:pr-0",
        align === "right" && "text-right",
        align === "center" && "text-center",
        align === "left" && "text-left",
        mono && "font-mono",
        num && "num tabular-nums",
        muted && "text-muted-foreground",
        className,
      )}
    >
      {children}
    </td>
  );
}
