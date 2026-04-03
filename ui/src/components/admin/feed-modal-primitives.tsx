import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

export function ModalSection({
  title,
  right,
  children,
}: {
  title: string;
  right?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="p-6">
      <div className="mb-4 flex items-center justify-between">
        <h3 className="eyebrow">{title}</h3>
        {right}
      </div>
      <div className="grid grid-cols-1 gap-x-8 gap-y-3 md:grid-cols-2">
        {children}
      </div>
    </div>
  );
}

export function KV({
  label,
  value,
  span2,
}: {
  label: string;
  value: ReactNode;
  span2?: boolean;
}) {
  return (
    <div
      className={cn(
        "grid grid-cols-[180px_1fr] items-baseline gap-3 text-xs",
        span2 && "md:col-span-2",
      )}
    >
      <div className="text-muted-foreground">{label}</div>
      <div className="min-w-0 text-foreground">{value}</div>
    </div>
  );
}
