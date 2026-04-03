import type { ChangeEvent } from "react";
import type { CategoryMeta, FeedHealthClass } from "@/lib/api-types";
import type {
  CadenceBucket,
  CriticalTierFilter,
  ExplorerState,
  FreshnessBucket,
  RedistributionBucket,
  UniquenessBucket,
} from "@/lib/explorer-state";
import { defaultHealthSelection } from "@/lib/explorer-state";
import { cn } from "@/lib/utils";

const HEALTH_CLASSES: Array<{ id: FeedHealthClass; label: string }> = [
  { id: "healthy", label: "Healthy" },
  { id: "delayed", label: "Delayed" },
  { id: "risky", label: "Risky" },
  { id: "archived", label: "Archived" },
  { id: "unmaintained", label: "Unmaintained" },
  { id: "empty", label: "Empty" },
  { id: "unavailable", label: "Unavailable" },
];

const PROVENANCE_CLASSES: Array<{ id: string; label: string }> = [
  { id: "primary", label: "Primary" },
  { id: "secondary_upstream", label: "Upstream" },
  { id: "secondary_merge", label: "Merge" },
  { id: "secondary_retention", label: "Retention" },
];

const FRESHNESS_BUCKETS: Array<{ id: FreshnessBucket; label: string }> = [
  { id: "hour", label: "Past hour" },
  { id: "day", label: "Past day" },
  { id: "week", label: "Past week" },
  { id: "month", label: "Past month" },
  { id: "older", label: "Older" },
];

const CADENCE_BUCKETS: Array<{ id: CadenceBucket; label: string }> = [
  { id: "hourly", label: "Hourly or faster" },
  { id: "daily", label: "Daily" },
  { id: "weekly", label: "Weekly" },
  { id: "monthly", label: "Monthly" },
  { id: "slower", label: "Slower" },
  { id: "unknown", label: "Unknown" },
];

const UNIQUENESS_BUCKETS: Array<{ id: UniquenessBucket; label: string }> = [
  { id: "very_high", label: ">=50% unique" },
  { id: "high", label: "20-50% unique" },
  { id: "medium", label: "5-20% unique" },
  { id: "low", label: "<5% unique" },
  { id: "unknown", label: "Unknown" },
];

const REDISTRIBUTION_BUCKETS: Array<{
  id: RedistributionBucket;
  label: string;
}> = [
  { id: "redistributable", label: "Redistributable" },
  { id: "restricted", label: "Not redistributable" },
];

const CRITICAL_TIERS: Array<{ id: CriticalTierFilter; label: string }> = [
  { id: "hard", label: "Hard" },
  { id: "soft", label: "Soft" },
  { id: "contextual", label: "Contextual" },
];

export function HomeExplorerFilterRail({
  state,
  onChange,
  categories,
  maintainers,
  licenses,
  totalCount,
  visibleCount,
}: {
  state: ExplorerState;
  onChange: (patch: Partial<ExplorerState>) => void;
  categories: CategoryMeta[];
  maintainers: string[];
  licenses: string[];
  totalCount: number;
  visibleCount: number;
}) {
  const toggleInSet = <T extends string>(set: Set<T>, id: T): Set<T> => {
    const next = new Set(set);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    return next;
  };

  const onQ = (event: ChangeEvent<HTMLInputElement>) =>
    onChange({ q: event.target.value });

  const onSizeMin = (event: ChangeEvent<HTMLInputElement>) => {
    const parsed = event.target.value ? Number(event.target.value) : null;
    onChange({ sizeMin: Number.isFinite(parsed) ? parsed : null });
  };
  const onSizeMax = (event: ChangeEvent<HTMLInputElement>) => {
    const parsed = event.target.value ? Number(event.target.value) : null;
    onChange({ sizeMax: Number.isFinite(parsed) ? parsed : null });
  };

  const onMaintainer = (event: ChangeEvent<HTMLSelectElement>) => {
    const value = event.target.value;
    if (!value) {
      onChange({ maintainers: new Set() });
      return;
    }
    onChange({ maintainers: new Set([value]) });
  };

  const onLicense = (event: ChangeEvent<HTMLSelectElement>) => {
    const value = event.target.value.trim();
    onChange({ license: value || null });
  };

  const clearAll = () =>
    onChange({
      q: "",
      categories: new Set(),
      maintainers: new Set(),
      health: defaultHealthSelection(),
      provenance: new Set(),
      cadence: new Set(),
      uniqueness: new Set(),
      license: null,
      redistribution: new Set(),
      criticalReference: new Set(),
      criticalOverlap: new Set(),
      sizeMin: null,
      sizeMax: null,
      freshness: null,
      lens: null,
    });

  return (
    <aside className="space-y-8">
      <div className="flex items-baseline justify-between">
        <div className="eyebrow text-muted-foreground">Filters</div>
        <button
          type="button"
          onClick={clearAll}
          className="text-[12px] font-medium text-muted-foreground transition hover:text-foreground"
        >
          Clear all
        </button>
      </div>

      <div className="text-[12px] text-muted-foreground">
        Showing <span className="font-semibold text-foreground">{visibleCount}</span>{" "}
        of <span className="font-semibold text-foreground">{totalCount}</span>{" "}
        feeds
      </div>

      <FilterGroup label="Search">
        <input
          type="search"
          aria-label="Filter feeds"
          value={state.q}
          onChange={onQ}
          placeholder="Name, maintainer, description…"
          className="h-10 w-full border border-border bg-background px-3 text-[13px] focus:border-primary/60 focus:outline-none"
        />
      </FilterGroup>

      <FilterGroup label="Category">
        <div className="flex flex-wrap gap-2">
          {categories.map((cat) => {
            const active = state.categories.has(cat.name);
            return (
              <Chip
                key={cat.name}
                label={cat.label ?? cat.name}
                active={active}
                onClick={() =>
                  onChange({
                    categories: toggleInSet(state.categories, cat.name),
                  })
                }
              />
            );
          })}
        </div>
      </FilterGroup>

      <FilterGroup label="Health">
        <div className="flex flex-wrap gap-2">
          {HEALTH_CLASSES.map((item) => (
            <Chip
              key={item.id}
              label={item.label}
              active={state.health.has(item.id)}
              onClick={() =>
                onChange({ health: toggleInSet(state.health, item.id) })
              }
            />
          ))}
        </div>
      </FilterGroup>

      <FilterGroup label="Provenance">
        <div className="flex flex-wrap gap-2">
          {PROVENANCE_CLASSES.map((item) => (
            <Chip
              key={item.id}
              label={item.label}
              active={state.provenance.has(item.id)}
              onClick={() =>
                onChange({
                  provenance: toggleInSet(state.provenance, item.id),
                })
              }
            />
          ))}
        </div>
      </FilterGroup>

      <FilterGroup label="Freshness">
        <div className="flex flex-wrap gap-2">
          {FRESHNESS_BUCKETS.map((item) => (
            <Chip
              key={item.id}
              label={item.label}
              active={state.freshness === item.id}
              onClick={() =>
                onChange({
                  freshness: state.freshness === item.id ? null : item.id,
                })
              }
            />
          ))}
        </div>
      </FilterGroup>

      <FilterGroup label="Update cadence">
        <div className="flex flex-wrap gap-2">
          {CADENCE_BUCKETS.map((item) => (
            <Chip
              key={item.id}
              label={item.label}
              active={state.cadence.has(item.id)}
              onClick={() =>
                onChange({
                  cadence: toggleInSet(state.cadence, item.id),
                })
              }
            />
          ))}
        </div>
      </FilterGroup>

      <FilterGroup label="Uniqueness">
        <div className="flex flex-wrap gap-2">
          {UNIQUENESS_BUCKETS.map((item) => (
            <Chip
              key={item.id}
              label={item.label}
              active={state.uniqueness.has(item.id)}
              onClick={() =>
                onChange({
                  uniqueness: toggleInSet(state.uniqueness, item.id),
                })
              }
            />
          ))}
        </div>
      </FilterGroup>

      <FilterGroup label="Size (IPs)">
        <div className="grid grid-cols-2 gap-2">
          <input
            type="number"
            min={0}
            aria-label="Minimum feed size"
            placeholder="Min"
            value={state.sizeMin ?? ""}
            onChange={onSizeMin}
            className="h-10 w-full border border-border bg-background px-3 text-[13px] focus:border-primary/60 focus:outline-none"
          />
          <input
            type="number"
            min={0}
            aria-label="Maximum feed size"
            placeholder="Max"
            value={state.sizeMax ?? ""}
            onChange={onSizeMax}
            className="h-10 w-full border border-border bg-background px-3 text-[13px] focus:border-primary/60 focus:outline-none"
          />
        </div>
      </FilterGroup>

      <FilterGroup label="Maintainer">
        <select
          aria-label="Filter by maintainer"
          value={
            state.maintainers.size === 1
              ? Array.from(state.maintainers)[0]
              : ""
          }
          onChange={onMaintainer}
          className="h-10 w-full border border-border bg-background px-3 text-[13px] focus:border-primary/60 focus:outline-none"
        >
          <option value="">Any maintainer</option>
          {maintainers.map((m) => (
            <option key={m} value={m}>
              {m}
            </option>
          ))}
        </select>
      </FilterGroup>

      <FilterGroup label="License">
        <select
          aria-label="Filter by license"
          value={state.license ?? ""}
          onChange={onLicense}
          className="h-10 w-full border border-border bg-background px-3 text-[13px] focus:border-primary/60 focus:outline-none"
        >
          <option value="">Any license</option>
          {licenses.map((license) => (
            <option key={license} value={license}>
              {license}
            </option>
          ))}
        </select>
      </FilterGroup>

      <FilterGroup label="Redistribution">
        <div className="flex flex-wrap gap-2">
          {REDISTRIBUTION_BUCKETS.map((item) => (
            <Chip
              key={item.id}
              label={item.label}
              active={state.redistribution.has(item.id)}
              onClick={() =>
                onChange({
                  redistribution: toggleInSet(state.redistribution, item.id),
                })
              }
            />
          ))}
        </div>
      </FilterGroup>

      <FilterGroup label="Critical references">
        <div className="flex flex-wrap gap-2">
          {CRITICAL_TIERS.map((item) => (
            <Chip
              key={item.id}
              label={item.label}
              active={state.criticalReference.has(item.id)}
              onClick={() =>
                onChange({
                  criticalReference: toggleInSet(state.criticalReference, item.id),
                })
              }
            />
          ))}
        </div>
      </FilterGroup>

      <FilterGroup label="Critical overlap">
        <div className="flex flex-wrap gap-2">
          {CRITICAL_TIERS.map((item) => (
            <Chip
              key={item.id}
              label={item.label}
              active={state.criticalOverlap.has(item.id)}
              onClick={() =>
                onChange({
                  criticalOverlap: toggleInSet(state.criticalOverlap, item.id),
                })
              }
            />
          ))}
        </div>
      </FilterGroup>
    </aside>
  );
}

function FilterGroup({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-3">
      <div className="eyebrow text-muted-foreground">{label}</div>
      {children}
    </div>
  );
}

function Chip({
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
      onClick={onClick}
      className={cn(
        "inline-flex h-8 items-center border px-3 text-[12px] font-medium transition",
        active
          ? "border-primary bg-primary/10 text-foreground"
          : "border-border bg-background text-muted-foreground hover:border-primary/60 hover:text-foreground",
      )}
    >
      {label}
    </button>
  );
}
