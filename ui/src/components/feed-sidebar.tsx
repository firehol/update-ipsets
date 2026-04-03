import {
  memo,
  useCallback,
  useDeferredValue,
  useEffect,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";
import { createPortal } from "react-dom";
import { Link, useLocation, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { Menu, Search, X } from "lucide-react";
import type { CategoryMeta, FeedSummary } from "@/lib/api-types";
import { orderCategories, useCategoriesQuery } from "@/lib/categories";
import { feedHealthLabel } from "@/lib/feed-health";
import { usePrefetchFeedDetail } from "@/lib/feed-prefetch";
import { cn, formatIPs, timeAgo } from "@/lib/utils";
import { feedsOptions } from "@/lib/queries/catalog";
import { Input } from "@/components/ui/input";
import { HoverTip } from "@/components/editorial/hover-tip";
import { CategoryBadge } from "@/components/category-badge";
import {
  OVERLAY_OPEN_EVENT,
  openFeedSidebarOverlay,
} from "@/components/feed-sidebar-events";

/**
 * Persistent feed-switching sidebar.
 *
 * Inspired by the old bash iplists site which had a three-tab left nav
 * (By category / By maintainer / Alphabetically). A modal palette is
 * always one click away but still requires opening; a persistent sidebar
 * makes neighbouring feeds visible and reachable in zero clicks.
 *
 * Visibility rules:
 *   - xl: (≥1280px) AND /ipsets/:name route → inline sidebar in the gutter
 *   - smaller screens OR /ipsets/:name → hamburger overlay
 *   - On /, /methodology*, /admin* the sidebar is suppressed entirely
 *     because those routes already have their own full-width chrome.
 *
 * Keep every row cheap:
 *   - Catalog fetched once via TanStack Query, shared cache with header
 *   - Rows are memoised so filter keystrokes only re-render the rows
 *     whose visibility actually flips
 *   - Sorting / grouping happens in useMemo so filter input doesn't
 *     re-sort a 180-entry array on every keystroke
 */

/* ============================================================================
   Freshness classification — same five-bucket mapping as the old bash
   site. Colours are mapped to category tokens so the sidebar inherits
   the same palette as the rest of the UI (no new colour variables).
   ========================================================================== */

type Freshness = "now" | "hour" | "fourHours" | "today" | "week" | "older";

function classifyFreshness(sourceDateSec: number | undefined): Freshness {
  if (!sourceDateSec) return "older";
  const ageMin = (Date.now() / 1000 - sourceDateSec) / 60;
  if (ageMin <= 15) return "now";
  if (ageMin <= 60) return "hour";
  if (ageMin <= 240) return "fourHours";
  if (ageMin <= 1440) return "today";
  if (ageMin <= 1440 * 7) return "week";
  return "older";
}

/** LED-style freshness dot. Small, unobtrusive; colour mirrors the old
 *  bash site's badge palette so returning users recognise it. */
function FreshnessDot({ freshness }: { freshness: Freshness }) {
  const cls: Record<Freshness, string> = {
    now: "bg-rose-500 shadow-[0_0_6px_rgba(244,63,94,0.6)]",
    hour: "bg-amber-400",
    fourHours: "bg-sky-400",
    today: "bg-emerald-500",
    week: "bg-slate-400",
    older: "bg-slate-600",
  };
  const description: Record<Freshness, string> = {
    now: "Updated in the last 15 minutes",
    hour: "Updated in the last hour",
    fourHours: "Updated in the last 4 hours",
    today: "Updated in the last 24 hours",
    week: "Updated in the last 7 days",
    older: "Older than a week",
  };
  return (
    <HoverTip text={description[freshness]} side="right">
      <span
        role="img"
        aria-label={description[freshness]}
        className={cn(
          "inline-block h-2 w-2 shrink-0 rounded-full",
          cls[freshness],
        )}
      />
    </HoverTip>
  );
}

/* ============================================================================
   Core content (search + tabs + list). Rendered in two places: the
   inline xl: sidebar and the hamburger overlay.
   ========================================================================== */

type TabKey = "category" | "maintainer" | "alpha";

interface Grouped {
  key: string;
  heading: string;
  feeds: FeedSummary[];
}

function groupByCategory(
  feeds: FeedSummary[],
  categories: CategoryMeta[],
): Grouped[] {
  const map = new Map<string, FeedSummary[]>();
  for (const f of feeds) {
    const key = f.category || "uncategorised";
    const list = map.get(key) ?? [];
    list.push(f);
    map.set(key, list);
  }
  const meta = new Map(categories.map((category) => [category.name, category]));
  const headings = orderCategories(categories, map.keys());
  return headings.map((heading) => ({
    key: heading,
    heading: meta.get(heading)?.label ?? heading,
    feeds: (map.get(heading) ?? [])
      .slice()
      .sort((a, b) => a.name.localeCompare(b.name)),
  }));
}

function groupByMaintainer(feeds: FeedSummary[]): Grouped[] {
  const map = new Map<string, FeedSummary[]>();
  for (const f of feeds) {
    const key = f.maintainer || "Unknown";
    const list = map.get(key) ?? [];
    list.push(f);
    map.set(key, list);
  }
  const headings = Array.from(map.keys()).sort((a, b) =>
    a.toLowerCase().localeCompare(b.toLowerCase()),
  );
  return headings.map((heading) => ({
    key: heading,
    heading,
    feeds: (map.get(heading) ?? [])
      .slice()
      .sort((a, b) => a.name.localeCompare(b.name)),
  }));
}

function sortAlpha(feeds: FeedSummary[]): Grouped[] {
  return [
    {
      key: "__alpha",
      heading: "",
      feeds: feeds.slice().sort((a, b) => a.name.localeCompare(b.name)),
    },
  ];
}

/* ============================================================================
   Row component. Memoised so filtering does not re-render every row.
   ========================================================================== */

const SidebarRow = memo(function SidebarRow({
  feed,
  isActive,
  onNavigate,
  compact,
}: {
  feed: FeedSummary;
  isActive: boolean;
  onNavigate: () => void;
  /** Drop the inline IP-count label for narrow variants (the 200px
   *  inline sidebar). The overlay has room for it and keeps it on. */
  compact?: boolean;
}) {
  const freshness = classifyFreshness(feed.source_date);
  const prefetch = usePrefetchFeedDetail(feed.name);
  // Editorial tooltip body: unique IPs, category, maintainer, last
  // updated. The Radix surface gives us real layout freedom that the
  // browser-default `title=` never could.
  const tipBody = (
    <div className="flex w-[260px] flex-col gap-2">
      <div className="font-mono text-[11px] font-semibold text-popover-foreground">
        {feed.name}
      </div>
      {feed.official_name && feed.official_name !== feed.name && (
        <div className="text-[11px] font-semibold leading-snug text-popover-foreground">
          {feed.official_name}
        </div>
      )}
      {feed.short_description && (
        <p className="text-[11px] leading-relaxed text-popover-foreground/85">
          {feed.short_description}
        </p>
      )}
      <div className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-[10px]">
        <span className="uppercase tracking-[0.08em] text-popover-foreground/55">
          IPs
        </span>
        <span className="tabular-nums text-popover-foreground">
          {formatIPs(feed.unique_ips)}
        </span>
        <span className="uppercase tracking-[0.08em] text-popover-foreground/55">
          Category
        </span>
        <span>
          <CategoryBadge category={feed.category} />
        </span>
        <span className="uppercase tracking-[0.08em] text-popover-foreground/55">
          Maintainer
        </span>
        <span className="truncate text-popover-foreground">
          {feed.maintainer || "Unknown"}
        </span>
        <span className="uppercase tracking-[0.08em] text-popover-foreground/55">
          Updated
        </span>
        <span className="text-popover-foreground">
          {feed.source_date ? timeAgo(feed.source_date) : "—"}
        </span>
        <span className="uppercase tracking-[0.08em] text-popover-foreground/55">
          Health
        </span>
        <span className="text-popover-foreground">
          {feedHealthLabel(feed.health?.class)}
        </span>
      </div>
    </div>
  );
  return (
    // delayDuration={120}: the sidebar is a hover-heavy navigation
    // area where the user is scanning many feed rows quickly. The
    // global 400ms delay feels sluggish here. 120ms is fast enough
    // to feel responsive without flashing on accidental cursor
    // sweeps. Other tooltips in the app keep the global delay.
    <HoverTip text={tipBody} side="right" align="start" delayDuration={120}>
      <Link
        to={`/ipsets/${encodeURIComponent(feed.name)}`}
        onFocus={prefetch}
        onMouseEnter={prefetch}
        onClick={onNavigate}
        className={cn(
          "group flex items-center gap-2 rounded-sm px-2 py-1 text-[12px] leading-tight transition-colors",
          "hover:bg-muted/60",
          isActive && "bg-primary/[0.12] text-foreground",
        )}
      >
        <FreshnessDot freshness={freshness} />
        <span
          className={cn(
            "flex-1 truncate font-mono",
            isActive
              ? "font-semibold text-foreground"
              : "text-muted-foreground group-hover:text-foreground",
          )}
        >
          {feed.name}
        </span>
        {!compact && (
          <span className="shrink-0 text-[10px] tabular-nums text-muted-foreground/70 group-hover:text-muted-foreground">
            {formatIPs(feed.unique_ips)}
          </span>
        )}
      </Link>
    </HoverTip>
  );
});

/* ============================================================================
   The sidebar content. Identical for inline + overlay, parameterised by
   a tiny wrapper that handles layout + close behaviour.
   ========================================================================== */

interface FeedSidebarContentProps {
  /** Called whenever the user picks a row, so the overlay variant can
   *  close itself. The inline variant passes a noop. */
  onNavigate: () => void;
  /** Ref for the search input — parent uses this to focus on open /
   *  on ⌘K. */
  searchRef?: React.RefObject<HTMLInputElement | null>;
  /** Current active feed name from the route, for highlighting. */
  activeFeedName?: string;
  /** Narrow variant — drops the inline IP-count label on each row so
   *  the long feed names (`ri_connect_proxies_30d`) still fit. */
  compact?: boolean;
}

function FeedSidebarContent({
  onNavigate,
  searchRef,
  activeFeedName,
  compact,
}: FeedSidebarContentProps) {
  const catalogQuery = useQuery({
    ...feedsOptions(),
    staleTime: 5 * 60 * 1000,
  });

  const feeds = useMemo(() => catalogQuery.data ?? [], [catalogQuery.data]);
  const categoriesQuery = useCategoriesQuery();

  const [tab, setTab] = useState<TabKey>("category");
  const [query, setQuery] = useState("");
  const deferredQuery = useDeferredValue(query);
  const normalisedQuery = deferredQuery.trim().toLowerCase();

  // Filter once by name + maintainer (same semantics as the old bash
  // typeahead). The filtered list feeds into every tab, so switching
  // tabs with an active query doesn't require re-typing.
  const filteredFeeds = useMemo(() => {
    if (!normalisedQuery) return feeds;
    return feeds.filter((f) => {
      const hay = `${f.name} ${f.maintainer ?? ""}`.toLowerCase();
      return hay.includes(normalisedQuery);
    });
  }, [feeds, normalisedQuery]);

  // Group by the active tab. Memoised on filteredFeeds so typing
  // doesn't re-group the full catalogue.
  const groups = useMemo(() => {
    switch (tab) {
      case "category":
        return groupByCategory(filteredFeeds, categoriesQuery.data ?? []);
      case "maintainer":
        return groupByMaintainer(filteredFeeds);
      case "alpha":
        return sortAlpha(filteredFeeds);
    }
  }, [tab, filteredFeeds, categoriesQuery.data]);

  const total = feeds.length;
  const shown = filteredFeeds.length;

  return (
    // flex-1 + min-h-0 (not h-full) so we size against the flex parent's
    // remaining space, not 100% of the parent's total height. The
    // overlay's drawer wraps this in a flex-col parent with a 48px
    // header above us — h-full would double-count that header and push
    // the scroll area below the viewport bottom.
    <div className="flex min-h-0 flex-1 flex-col">
      {/* Search input — always visible at the top. */}
      <div className="shrink-0 border-b border-border px-3 py-2.5">
        <div className="relative">
          <Search className="pointer-events-none absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            ref={searchRef}
            type="search"
            placeholder={`Filter ${total} feeds…`}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            className="h-8 pl-7 pr-2 text-[12px]"
            aria-label="Filter feeds by name or maintainer"
          />
        </div>
        <div className="mt-2 flex items-center justify-between text-[10px] uppercase tracking-[0.1em] text-muted-foreground">
          <span>
            {shown === total ? `${total} feeds` : `${shown} / ${total} match`}
          </span>
        </div>
      </div>

      {/* Tab bar. Bottom-border underline for the active tab echoes
          the editorial tab pattern used elsewhere in the app. */}
      <div className="shrink-0 border-b border-border px-2">
        <nav className="flex gap-0.5" role="tablist">
          <TabButton
            active={tab === "category"}
            onClick={() => setTab("category")}
          >
            Category
          </TabButton>
          <TabButton
            active={tab === "maintainer"}
            onClick={() => setTab("maintainer")}
          >
            Maintainer
          </TabButton>
          <TabButton active={tab === "alpha"} onClick={() => setTab("alpha")}>
            A–Z
          </TabButton>
        </nav>
      </div>

      {/* Scrollable list. `flex-1 min-h-0` is the idiomatic trick
          that lets a flex child actually scroll inside a flex parent
          (without min-h-0, the child's implicit min-height is auto,
          i.e. the min-content size of its children, so flex-1 grows
          to fit all 180 rows and the overflow never engages).
          `overscroll-contain` prevents a mobile rubber-band at the
          top/bottom from propagating scroll to the body. */}
      <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain px-2 py-2">
        {catalogQuery.isLoading && (
          <div className="px-2 py-4 text-[11px] text-muted-foreground">
            Loading catalogue…
          </div>
        )}
        {!catalogQuery.isLoading && shown === 0 && (
          <div className="px-2 py-4 text-[11px] text-muted-foreground">
            No feeds match.
          </div>
        )}
        {groups.map((g) => (
          <div key={g.key} className="mb-4 last:mb-0">
            {g.heading && (
              <div className="mb-1 px-2 pt-1 text-[10px] font-semibold uppercase tracking-[0.12em] text-muted-foreground">
                {g.heading}
              </div>
            )}
            <div className="flex flex-col">
              {g.feeds.map((f) => (
                <SidebarRow
                  key={f.name}
                  feed={f}
                  isActive={f.name === activeFeedName}
                  onNavigate={onNavigate}
                  compact={compact}
                />
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function TabButton({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      role="tab"
      aria-selected={active}
      onClick={onClick}
      className={cn(
        "relative px-3 py-2 text-[11px] font-semibold uppercase tracking-[0.08em] transition-colors",
        active
          ? "text-foreground"
          : "text-muted-foreground hover:text-foreground",
      )}
    >
      {children}
      {active && (
        <span className="absolute inset-x-2 bottom-0 h-[2px] bg-primary" />
      )}
    </button>
  );
}

/* ============================================================================
   Layout wrappers — inline (desktop) and overlay (mobile / small screens).
   ========================================================================== */

/** Returns true if the sidebar should render at all on the current
 *  route. Keeps sidebar mounting / unmounting in one place so every
 *  caller sees consistent behaviour. */
function useShouldShowSidebar(): boolean {
  const { pathname } = useLocation();
  // Excluded routes: homepage, catalog (it IS the feed list),
  // methodology, admin.
  if (pathname === "/" || pathname === "") return false;
  if (pathname.startsWith("/catalog")) return false;
  if (pathname.startsWith("/methodology")) return false;
  if (pathname.startsWith("/admin")) return false;
  return true;
}

/**
 * Returns true when the viewport is wide enough for the inline
 * sidebar to fit in the left gutter without overlapping the centred
 * page-container (see FeedSidebarInline for the math: 1280 + 2*200
 * = 1680px). Below that width, the overlay drawer is used instead.
 *
 * Previously the two variants were both mounted unconditionally and
 * switched via `hidden` / `min-[1680px]:block` CSS classes. That made
 * every feed-detail page ship 360 list rows in the DOM — 180 for the
 * inline sidebar and another 180 for the overlay — so the first time
 * the user opened the overlay on a narrow screen, the browser had to
 * style+layout+paint a tree that had never been visible before, and
 * the stall was long enough to feel like the hamburger was broken.
 * Gating with a JS media-query hook means exactly one variant is
 * mounted at any viewport width, so there is never a "cold" subtree
 * waiting to be painted on first open.
 */
function useIsWideViewport(): boolean {
  return useSyncExternalStore(
    subscribeWideViewport,
    getWideViewportSnapshot,
    getWideViewportServerSnapshot,
  );
}

function subscribeWideViewport(callback: () => void): () => void {
  if (typeof window === "undefined") return () => {};
  const mq = window.matchMedia("(min-width: 1680px)");
  mq.addEventListener("change", callback);
  return () => mq.removeEventListener("change", callback);
}

function getWideViewportSnapshot(): boolean {
  if (typeof window === "undefined") return false;
  return window.matchMedia("(min-width: 1680px)").matches;
}

function getWideViewportServerSnapshot(): boolean {
  return false;
}

/**
 * Inline sidebar — `position: fixed` in the left viewport gutter
 * under the header. Because it's fixed (not a flex child), the main
 * content's centering is untouched: the centred page-container stays
 * centred and the sidebar floats on top of the otherwise empty gutter
 * space to the left of it.
 *
 * Width is FLUID via `clamp()`, tracking the available gutter so the
 * sidebar grows with the viewport but never eats into the content:
 *
 *   width = clamp(200px, (100vw - 1280px) / 2, 260px)
 *
 * The middle term is the exact left-gutter width of a max-w-[1280px]
 * centred container, so the sidebar is always precisely as wide as
 * the empty space it sits in (until it hits the 260px ceiling).
 *
 * Visibility threshold: 1680px. That's the smallest viewport where
 * (100vw - 1280px) / 2 ≥ 200px — i.e. where the gutter can hold the
 * minimum sidebar width. Below that the inline sidebar stays hidden
 * and the hamburger + overlay variant takes over.
 *
 *   1680px → gutter  200px → sidebar 200px (floor)
 *   1740px → gutter  230px → sidebar 230px
 *   1800px → gutter  260px → sidebar 260px (ceiling reached)
 *   2000px → gutter  360px → sidebar 260px (capped)
 *
 * The inline rows use the `compact` prop (no IP count label) so even
 * at the 200px minimum the longest feed names like
 * `ri_connect_proxies_30d` still fit. The overlay drawer on smaller
 * screens keeps the IP count because its 86vw width has room for it.
 */
export function FeedSidebarInline() {
  const show = useShouldShowSidebar();
  const wide = useIsWideViewport();
  const { name: routeFeedName } = useParams<{ name: string }>();

  if (!show || !wide) return null;
  return (
    <aside
      aria-label="Feed navigator"
      className="fixed left-0 top-16 bottom-0 z-30 flex w-[clamp(200px,calc((100vw-1280px)/2),260px)] flex-col border-r border-border bg-card"
    >
      <FeedSidebarContent
        onNavigate={() => {
          /* inline sidebar stays open across navigations */
        }}
        activeFeedName={routeFeedName}
        compact
      />
    </aside>
  );
}

/* ============================================================================
   Overlay for small screens + ⌘K. Manages its own open state via a
   tiny custom-event bus so any component (site header hamburger,
   keyboard shortcut) can open it without prop-drilling.
   ========================================================================== */

export function FeedSidebarOverlay() {
  const wide = useIsWideViewport();
  const [open, setOpen] = useState(false);
  const searchRef = useRef<HTMLInputElement | null>(null);
  const { name: routeFeedName } = useParams<{ name: string }>();

  // Listen for programmatic opens from the header hamburger and from
  // the ⌘K global shortcut.
  useEffect(() => {
    const onOpen = () => setOpen(true);
    window.addEventListener(OVERLAY_OPEN_EVENT, onOpen);
    return () => window.removeEventListener(OVERLAY_OPEN_EVENT, onOpen);
  }, []);

  // Focus the filter input as soon as the overlay opens so the user
  // can start typing immediately. Deferred to the next frame so the
  // mount is committed before we try to focus.
  useEffect(() => {
    if (!open) return;
    requestAnimationFrame(() => {
      searchRef.current?.focus();
      searchRef.current?.select();
    });
  }, [open]);

  // Close on Escape — mirrors the cmdk picker we replaced.
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        setOpen(false);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open]);

  const close = useCallback(() => setOpen(false), []);

  // Gate 1: never mount on wide viewports — the inline sidebar takes
  // over there.
  if (wide) return null;

  // Gate 2: only mount the overlay subtree while it is actually open.
  //
  // The previous version kept the overlay always in the tree and
  // toggled visibility via `display: none` ↔ `display: block`. That
  // put FeedSidebarContent and its 180 rows inside Layout's flex-col
  // ancestor chain on every narrow-viewport page, which meant any
  // layout pass on Layout (from theme toggles, window resizes, or
  // the fixed sidebar's clamp() width recomputing on resize) had to
  // walk through the hidden overlay subtree as well — and on feed
  // detail pages that subtree sits next to AutoFitText's
  // ResizeObserver in the hero, which turned a one-shot size
  // recompute into a loop.
  //
  // Mount-on-open + a portal to document.body breaks that cascade:
  //   - while closed: the subtree does not exist, no layout cost
  //   - while open: the subtree lives under <body>, NOT under
  //     Layout's flex container, so its layout is independent of
  //     every other element on the page.
  //
  // The tradeoff is that first-open now pays a mount cost (180 rows
  // rendering for the first time), but TanStack Query already has
  // the catalog cached from the inline sidebar's query on wider
  // routes, or from the site header's query on feed-detail pages,
  // so there is no network round trip and the mount is close to
  // instant in practice.
  if (!open) return null;

  const drawer = (
    <div role="presentation">
      {/* Backdrop. Fills the whole viewport and closes the drawer on
          click. Separate element so it can have its own backdrop blur
          without affecting the drawer panel. */}
      <div
        className="fixed inset-0 z-40 bg-black/70 backdrop-blur-sm"
        onClick={close}
      />
      {/* Drawer panel. Explicit h-screen rather than `inset-y-0` so
          the element has a resolved height immediately on mount,
          which is required for the inner `flex-1 min-h-0` scroll
          area to compute its own resolved height on first paint. */}
      <aside
        role="dialog"
        aria-label="Feed navigator"
        className="fixed left-0 top-0 z-50 flex h-screen w-[86vw] max-w-[320px] flex-col border-r border-border bg-card shadow-2xl"
      >
        <div className="flex h-12 shrink-0 items-center justify-between border-b border-border px-3">
          <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-muted-foreground">
            Feeds
          </span>
          <button
            type="button"
            onClick={close}
            aria-label="Close feed navigator"
            className="rounded-sm p-1 text-muted-foreground hover:bg-muted hover:text-foreground"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
        <FeedSidebarContent
          onNavigate={close}
          searchRef={searchRef}
          activeFeedName={routeFeedName}
        />
      </aside>
    </div>
  );

  // Portal to <body> so the drawer is a sibling of the entire React
  // root, not a descendant of Layout's flex column. Any layout pass
  // triggered inside Layout cannot propagate into the drawer, and
  // vice versa.
  return createPortal(drawer, document.body);
}

/* ============================================================================
   Hamburger trigger for the site header, plus the global ⌘K shortcut.
   ========================================================================== */

/** Small hamburger button for the site header — only visible below
 *  the inline sidebar threshold (1680px) where the overlay variant
 *  is the active navigator. */
export function FeedSidebarHamburger() {
  const show = useShouldShowSidebar();
  const wide = useIsWideViewport();
  if (!show || wide) return null;
  return (
    <button
      type="button"
      onClick={openFeedSidebarOverlay}
      aria-label="Open feed navigator"
      className="inline-flex items-center justify-center rounded-sm border border-display-border p-1.5 text-display-muted transition-colors hover:border-display-fg/40 hover:text-display-fg"
    >
      <Menu className="h-4 w-4" />
    </button>
  );
}
