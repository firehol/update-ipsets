import type { CategoryMeta, FeedHealthClass, FeedSummary } from "@/lib/api-types";

export type FreshnessBucket = "hour" | "day" | "week" | "month" | "older";
export type CadenceBucket =
  | "hourly"
  | "daily"
  | "weekly"
  | "monthly"
  | "slower"
  | "unknown";
export type UniquenessBucket =
  | "very_high"
  | "high"
  | "medium"
  | "low"
  | "unknown";
export type RedistributionBucket = "redistributable" | "restricted";
export type CriticalTierFilter = "hard" | "soft" | "contextual";

export type SortKey =
  | "freshest"
  | "coverage"
  | "unique"
  | "name"
  | "maintainer";

export type ViewMode =
  | "cards"
  | "table"
  | "treemap"
  | "timeline"
  | "maintainers";

export interface ExplorerState {
  q: string;
  categories: Set<string>;
  maintainers: Set<string>;
  health: Set<FeedHealthClass>;
  provenance: Set<string>;
  cadence: Set<CadenceBucket>;
  uniqueness: Set<UniquenessBucket>;
  license: string | null;
  redistribution: Set<RedistributionBucket>;
  criticalReference: Set<CriticalTierFilter>;
  criticalOverlap: Set<CriticalTierFilter>;
  sizeMin: number | null;
  sizeMax: number | null;
  freshness: FreshnessBucket | null;
  sort: SortKey;
  view: ViewMode;
  lens: string | null;
}

const VIEW_STORAGE_KEY = "update-ipsets.home.explorer.view";

const FRESHNESS_VALUES: FreshnessBucket[] = ["hour", "day", "week", "month", "older"];
const CADENCE_VALUES: CadenceBucket[] = [
  "hourly",
  "daily",
  "weekly",
  "monthly",
  "slower",
  "unknown",
];
const UNIQUENESS_VALUES: UniquenessBucket[] = [
  "very_high",
  "high",
  "medium",
  "low",
  "unknown",
];
const REDISTRIBUTION_VALUES: RedistributionBucket[] = [
  "redistributable",
  "restricted",
];
const CRITICAL_TIER_VALUES: CriticalTierFilter[] = ["hard", "soft", "contextual"];
const SORT_VALUES: SortKey[] = [
  "freshest",
  "coverage",
  "unique",
  "name",
  "maintainer",
];
const VIEW_VALUES: ViewMode[] = [
  "cards",
  "table",
  "treemap",
  "timeline",
  "maintainers",
];
const HEALTH_VALUES: FeedHealthClass[] = [
  "healthy",
  "delayed",
  "risky",
  "archived",
  "unmaintained",
  "empty",
  "unavailable",
];
const DEFAULT_HEALTH_VALUES: FeedHealthClass[] = [
  "healthy",
  "delayed",
  "risky",
  "unavailable",
];
const HEALTH_ALL_TOKEN = "all";

export function defaultHealthSelection(): Set<FeedHealthClass> {
  return new Set(DEFAULT_HEALTH_VALUES);
}

function createExplorerState(): ExplorerState {
  return {
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
    sort: "freshest",
    view: "cards",
    lens: null,
  };
}

function parseCSV(value: string | null): Set<string> {
  if (!value) return new Set();
  const items = value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
  return new Set(items);
}

function parseSubset<T extends string>(value: string | null, allowed: readonly T[]): Set<T> {
  const raw = parseCSV(value);
  const next = new Set<T>();
  for (const item of raw) {
    if ((allowed as readonly string[]).includes(item)) {
      next.add(item as T);
    }
  }
  return next;
}

function parseHealthSelection(value: string | null): Set<FeedHealthClass> {
  if (value === null) return defaultHealthSelection();
  if (value.trim() === HEALTH_ALL_TOKEN) return new Set();
  return parseSubset(value, HEALTH_VALUES);
}

function sameSet<T extends string>(left: Set<T>, right: Set<T>): boolean {
  if (left.size !== right.size) return false;
  for (const item of left) {
    if (!right.has(item)) return false;
  }
  return true;
}

function parseNumber(value: string | null): number | null {
  if (!value) return null;
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : null;
}

function parseFreshness(value: string | null): FreshnessBucket | null {
  if (!value) return null;
  return (FRESHNESS_VALUES as readonly string[]).includes(value)
    ? (value as FreshnessBucket)
    : null;
}

function parseSort(value: string | null): SortKey {
  return value && (SORT_VALUES as readonly string[]).includes(value)
    ? (value as SortKey)
    : createExplorerState().sort;
}

function parseViewValue(value: string | null): ViewMode | null {
  return value && (VIEW_VALUES as readonly string[]).includes(value)
    ? (value as ViewMode)
    : null;
}

function readPersistedView(): ViewMode | null {
  if (typeof window === "undefined") return null;
  try {
    return parseViewValue(window.localStorage.getItem(VIEW_STORAGE_KEY));
  } catch {
    return null;
  }
}

export function rememberExplorerView(view: ViewMode): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(VIEW_STORAGE_KEY, view);
  } catch {
    // Ignore storage failures; URL state still preserves the current view.
  }
}

export function readExplorerState(params: URLSearchParams): ExplorerState {
  const defaults = createExplorerState();
  const viewFromURL = parseViewValue(params.get("view"));
  return normalizeExplorerState({
    q: params.get("q")?.trim() ?? "",
    categories: parseCSV(params.get("category")),
    maintainers: parseCSV(params.get("maintainer")),
    health: parseHealthSelection(params.get("health")),
    provenance: parseCSV(params.get("provenance")),
    cadence: parseSubset(params.get("cadence"), CADENCE_VALUES),
    uniqueness: parseSubset(params.get("uniqueness"), UNIQUENESS_VALUES),
    license: params.get("license")?.trim() || null,
    redistribution: parseSubset(
      params.get("redistribution"),
      REDISTRIBUTION_VALUES,
    ),
    criticalReference: parseSubset(params.get("critical"), CRITICAL_TIER_VALUES),
    criticalOverlap: parseSubset(
      params.get("critical_overlap"),
      CRITICAL_TIER_VALUES,
    ),
    sizeMin: parseNumber(params.get("size_min")),
    sizeMax: parseNumber(params.get("size_max")),
    freshness: parseFreshness(params.get("fresh")),
    sort: parseSort(params.get("sort")),
    view: viewFromURL ?? readPersistedView() ?? defaults.view,
    lens: params.get("lens")?.trim() || null,
  });
}

function writeSet(params: URLSearchParams, key: string, value: Set<string>): void {
  if (value.size === 0) {
    params.delete(key);
    return;
  }
  params.set(key, Array.from(value).sort().join(","));
}

function writeNumber(params: URLSearchParams, key: string, value: number | null): void {
  if (value === null) {
    params.delete(key);
    return;
  }
  params.set(key, String(value));
}

export function writeExplorerState(
  base: URLSearchParams,
  state: ExplorerState,
): URLSearchParams {
  state = normalizeExplorerState(state);
  const defaults = createExplorerState();
  const next = new URLSearchParams(base);
  if (state.q) next.set("q", state.q);
  else next.delete("q");
  writeSet(next, "category", state.categories);
  writeSet(next, "maintainer", state.maintainers);
  if (sameSet(state.health, defaults.health)) {
    next.delete("health");
  } else if (state.health.size === 0) {
    next.set("health", HEALTH_ALL_TOKEN);
  } else {
    next.set("health", Array.from(state.health).sort().join(","));
  }
  writeSet(next, "provenance", state.provenance);
  writeSet(next, "cadence", state.cadence);
  writeSet(next, "uniqueness", state.uniqueness);
  writeSet(next, "redistribution", state.redistribution);
  writeSet(next, "critical", state.criticalReference);
  writeSet(next, "critical_overlap", state.criticalOverlap);
  if (state.license) next.set("license", state.license);
  else next.delete("license");
  writeNumber(next, "size_min", state.sizeMin);
  writeNumber(next, "size_max", state.sizeMax);
  if (state.freshness) next.set("fresh", state.freshness);
  else next.delete("fresh");
  if (state.sort !== defaults.sort) next.set("sort", state.sort);
  else next.delete("sort");
  if (state.view !== defaults.view) next.set("view", state.view);
  else next.delete("view");
  if (state.lens) next.set("lens", state.lens);
  else next.delete("lens");
  return next;
}

function nowSeconds(): number {
  return Math.floor(Date.now() / 1000);
}

export function explorerTimestamp(feed: FeedSummary): number {
  return feed.source_date || feed.processed_date || feed.checked_date || 0;
}

function freshnessBucket(feed: FeedSummary): FreshnessBucket {
  const ts = explorerTimestamp(feed);
  if (!ts) return "older";
  const ageSeconds = nowSeconds() - ts;
  if (ageSeconds < 60 * 60) return "hour";
  if (ageSeconds < 60 * 60 * 24) return "day";
  if (ageSeconds < 60 * 60 * 24 * 7) return "week";
  if (ageSeconds < 60 * 60 * 24 * 30) return "month";
  return "older";
}

function cadenceBucket(feed: FeedSummary): CadenceBucket {
  const minutes =
    feed.average_update_mins && feed.average_update_mins > 0
      ? feed.average_update_mins
      : feed.frequency_minutes && feed.frequency_minutes > 0
        ? feed.frequency_minutes
        : null;
  if (minutes === null) return "unknown";
  if (minutes <= 60) return "hourly";
  if (minutes <= 60 * 24) return "daily";
  if (minutes <= 60 * 24 * 7) return "weekly";
  if (minutes <= 60 * 24 * 30) return "monthly";
  return "slower";
}

function uniquenessBucket(feed: FeedSummary): UniquenessBucket {
  const share = feed.unique_share_pct;
  if (share === undefined || share === null || !Number.isFinite(share)) {
    return "unknown";
  }
  if (share >= 50) return "very_high";
  if (share >= 20) return "high";
  if (share >= 5) return "medium";
  return "low";
}

function redistributionBucket(feed: FeedSummary): RedistributionBucket {
  return feed.redistributable === false ? "restricted" : "redistributable";
}

function feedCriticalTier(feed: FeedSummary): CriticalTierFilter | null {
  const tier = feed.critical?.tier;
  return isCriticalTierFilter(tier) ? tier : null;
}

function feedCriticalOverlapTiers(feed: FeedSummary): Set<CriticalTierFilter> {
  const out = new Set<CriticalTierFilter>();
  for (const tier of feed.critical_overlap_tiers ?? []) {
    if (isCriticalTierFilter(tier)) out.add(tier);
  }
  return out;
}

function isCriticalTierFilter(value: string | undefined | null): value is CriticalTierFilter {
  return (
    value === "hard" ||
    value === "soft" ||
    value === "contextual"
  );
}

function matchesFreeText(feed: FeedSummary, q: string): boolean {
  if (!q) return true;
  const needle = q.toLowerCase();
  if (feed.name.toLowerCase().includes(needle)) return true;
  if (feed.official_name?.toLowerCase().includes(needle)) return true;
  if (feed.short_description?.toLowerCase().includes(needle)) return true;
  if (feed.maintainer?.toLowerCase().includes(needle)) return true;
  if (feed.info?.toLowerCase().includes(needle)) return true;
  return false;
}

export function applyFilters(
  feeds: FeedSummary[],
  state: ExplorerState,
): FeedSummary[] {
  return feeds.filter((feed) => {
    if (!matchesFreeText(feed, state.q)) return false;
    if (state.categories.size > 0 && !state.categories.has(feed.category)) {
      return false;
    }
    if (
      state.maintainers.size > 0 &&
      (!feed.maintainer || !state.maintainers.has(feed.maintainer))
    ) {
      return false;
    }
    if (state.health.size > 0 && !state.health.has(feed.health?.class ?? "")) {
      return false;
    }
    if (state.provenance.size > 0) {
      const prov = feed.provenance ?? "primary";
      if (!state.provenance.has(prov)) return false;
    }
    if (state.cadence.size > 0 && !state.cadence.has(cadenceBucket(feed))) {
      return false;
    }
    if (
      state.uniqueness.size > 0 &&
      !state.uniqueness.has(uniquenessBucket(feed))
    ) {
      return false;
    }
    if (state.license && (feed.license?.trim() ?? "") !== state.license) {
      return false;
    }
    if (
      state.redistribution.size > 0 &&
      !state.redistribution.has(redistributionBucket(feed))
    ) {
      return false;
    }
    if (state.criticalReference.size > 0) {
      const tier = feedCriticalTier(feed);
      if (!tier || !state.criticalReference.has(tier)) return false;
    }
    if (state.criticalOverlap.size > 0) {
      const tiers = feedCriticalOverlapTiers(feed);
      let matched = false;
      for (const tier of state.criticalOverlap) {
        if (tiers.has(tier)) {
          matched = true;
          break;
        }
      }
      if (!matched) return false;
    }
    const ips = feed.unique_ips ?? 0;
    if (state.sizeMin !== null && ips < state.sizeMin) return false;
    if (state.sizeMax !== null && ips > state.sizeMax) return false;
    if (state.freshness && freshnessBucket(feed) !== state.freshness) {
      return false;
    }
    return true;
  });
}

export function applySort(feeds: FeedSummary[], sort: SortKey): FeedSummary[] {
  const sorted = [...feeds];
  switch (sort) {
    case "freshest":
      sorted.sort((a, b) => explorerTimestamp(b) - explorerTimestamp(a));
      break;
    case "coverage":
      sorted.sort((a, b) => (b.unique_ips ?? 0) - (a.unique_ips ?? 0));
      break;
    case "unique":
      sorted.sort((a, b) => {
        const uq = (b.unique_share_pct ?? 0) - (a.unique_share_pct ?? 0);
        if (uq !== 0) return uq;
        return (b.unique_ips ?? 0) - (a.unique_ips ?? 0);
      });
      break;
    case "name":
      sorted.sort((a, b) => a.name.localeCompare(b.name));
      break;
    case "maintainer":
      sorted.sort((a, b) => {
        const m = (a.maintainer ?? "").localeCompare(b.maintainer ?? "");
        return m !== 0 ? m : a.name.localeCompare(b.name);
      });
      break;
  }
  return sorted;
}

export interface LensDefinition {
  id: string;
  label: string;
  description: string;
  apply: (state: ExplorerState) => ExplorerState;
}

function lensState(patch: Partial<ExplorerState>): ExplorerState {
  return {
    ...createExplorerState(),
    ...patch,
  };
}

export const LENSES: LensDefinition[] = [
  {
    id: "freshest",
    label: "Freshest",
    description: "Feeds updated most recently.",
    apply: () =>
      lensState({
        sort: "freshest",
        view: "cards",
        lens: "freshest",
      }),
  },
  {
    id: "coverage",
    label: "Largest coverage",
    description: "Feeds with the broadest IP-address coverage.",
    apply: () =>
      lensState({
        sort: "coverage",
        view: "cards",
        lens: "coverage",
      }),
  },
  {
    id: "unique",
    label: "Most unique",
    description:
      "Feeds whose IPs are least covered by their closest independent peer.",
    apply: () =>
      lensState({
        sort: "unique",
        view: "cards",
        lens: "unique",
      }),
  },
  {
    id: "starter",
    label: "Primary starters",
    description:
      "Primary feeds whose raw lists can be redistributed directly.",
    apply: () =>
      lensState({
        sort: "freshest",
        view: "cards",
        provenance: new Set(["primary"]),
        redistribution: new Set(["redistributable"]),
        lens: "starter",
      }),
  },
  {
    id: "by-threat",
    label: "By threat type",
    description: "Explore feeds grouped by category.",
    apply: () =>
      lensState({
        sort: "name",
        view: "treemap",
        lens: "by-threat",
      }),
  },
  {
    id: "by-maintainer",
    label: "By maintainer",
    description: "Explore feeds grouped by the maintainer that publishes them.",
    apply: () =>
      lensState({
        sort: "maintainer",
        view: "maintainers",
        lens: "by-maintainer",
      }),
  },
];

function sameExplorerStateIgnoringLens(
  left: ExplorerState,
  right: ExplorerState,
): boolean {
  return (
    left.q === right.q &&
    sameSet(left.categories, right.categories) &&
    sameSet(left.maintainers, right.maintainers) &&
    sameSet(left.health, right.health) &&
    sameSet(left.provenance, right.provenance) &&
    sameSet(left.cadence, right.cadence) &&
    sameSet(left.uniqueness, right.uniqueness) &&
    left.license === right.license &&
    sameSet(left.redistribution, right.redistribution) &&
    sameSet(left.criticalReference, right.criticalReference) &&
    sameSet(left.criticalOverlap, right.criticalOverlap) &&
    left.sizeMin === right.sizeMin &&
    left.sizeMax === right.sizeMax &&
    left.freshness === right.freshness &&
    left.sort === right.sort &&
    left.view === right.view
  );
}

function lensPreset(id: string): ExplorerState | null {
  const lens = LENSES.find((item) => item.id === id);
  if (!lens) return null;
  return lens.apply(createExplorerState());
}

export function normalizeExplorerState(state: ExplorerState): ExplorerState {
  if (!state.lens) return state;
  const preset = lensPreset(state.lens);
  if (preset && sameExplorerStateIgnoringLens(state, preset)) {
    return state;
  }
  return { ...state, lens: null };
}

export function distinctMaintainers(feeds: FeedSummary[]): string[] {
  const set = new Set<string>();
  for (const feed of feeds) {
    if (feed.maintainer) {
      set.add(feed.maintainer.trim());
    }
  }
  return Array.from(set).sort((a, b) => a.localeCompare(b));
}

export function distinctLicenses(feeds: FeedSummary[]): string[] {
  const set = new Set<string>();
  for (const feed of feeds) {
    const license = feed.license?.trim();
    if (license) {
      set.add(license);
    }
  }
  return Array.from(set).sort((a, b) => a.localeCompare(b));
}

function publicCategoryNames(categories: CategoryMeta[]): Set<string> {
  const names = new Set<string>();
  for (const category of categories) {
    const name = category.name?.trim();
    if (name) {
      names.add(name);
    }
  }
  return names;
}

export function publicExplorerFeeds(
  feeds: FeedSummary[],
  categories: CategoryMeta[],
): FeedSummary[] {
  const allowedCategories = publicCategoryNames(categories);
  return feeds.filter((feed) => allowedCategories.has(feed.category));
}
