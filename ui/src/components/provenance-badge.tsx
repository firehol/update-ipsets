import type { FeedProvenance } from "@/lib/api-types";
import { cn } from "@/lib/utils";

function provenanceLabel(value: FeedProvenance | undefined): string | null {
  switch (value) {
    case "secondary_upstream":
      return "Upstream aggregate";
    case "secondary_merge":
      return "Merged";
    case "secondary_retention":
      return "Retention";
    case "primary":
    default:
      return null;
  }
}

export function ProvenanceBadge({
  provenance,
  className,
}: {
  provenance?: FeedProvenance;
  className?: string;
}) {
  const label = provenanceLabel(provenance);
  if (!label) return null;
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-md border border-border bg-muted px-2 py-0.5 text-[10px] font-medium uppercase tracking-[0.08em] text-muted-foreground",
        className,
      )}
    >
      {label}
    </span>
  );
}
