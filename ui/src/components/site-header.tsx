import { Link, useLocation, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import type { FeedSummary } from "@/lib/api-types";
import { CategoryBadge } from "@/components/category-badge";
import { ThemeToggle } from "./theme-toggle";
import { FeedSidebarHamburger } from "./feed-sidebar";
import { IPSearchSurface } from "@/components/ip-search/ip-search-surface";
import { feedsOptions } from "@/lib/queries/catalog";

/**
 * Editorial site header. Inspired by Apple's product page chrome:
 *   - Thin, dark, opaque
 *   - Wordmark on the left, feed orientation in the middle (when
 *     viewing a feed), search and nav on the right
 *   - No drop shadow, just a single hairline border at the bottom
 *
 * On feed-detail pages the header gains a "current feed" strip in the
 * middle: the feed name (mono) and its category badge. The persistent
 * sidebar on xl:+ screens handles feed switching; below xl the
 * hamburger in this header opens the overlay version of that sidebar.
 * ⌘K focuses the sidebar's search input (or opens the overlay on small
 * screens) — the global shortcut is registered once in Layout.
 */
export function SiteHeader() {
  const { pathname } = useLocation();

  // useParams() returns the route param when we're on /ipsets/:name,
  // and undefined otherwise. We don't need to know "are we actually
  // on a feed page" beyond that — if there's a name, render the
  // feed-aware chrome; if not, the standard header.
  const { name: routeFeedName } = useParams<{ name: string }>();

  // Pull the feed summary from the catalog cache so we can render the
  // category badge inline. The catalog query is shared with the
  // sidebar and the catalog page, so this never causes a round trip
  // on its own.
  const catalogQuery = useQuery({
    ...feedsOptions(),
    staleTime: 5 * 60 * 1000,
    enabled: !!routeFeedName,
  });
  const feed: FeedSummary | undefined = routeFeedName
    ? catalogQuery.data?.find((f) => f.name === routeFeedName)
    : undefined;

  const showInlineSearch = pathname !== "/";

  return (
    <header className="sticky top-0 z-50 border-b border-display-border/60 bg-display/95 backdrop-blur supports-[backdrop-filter]:bg-display/80">
      <div className="page-container flex h-16 items-center gap-4">
        {/* Hamburger — only visible below xl: where the inline sidebar
            is hidden. Rendered as null on catalog / methodology / admin. */}
        <FeedSidebarHamburger />

        <Link
          to="/"
          className="flex items-baseline gap-2 text-display-fg transition-colors hover:opacity-90"
        >
          <span className="font-display text-[19px] font-bold tracking-tight">
            FireHOL
          </span>
          <span className="hidden text-[11px] uppercase tracking-[0.18em] text-display-muted sm:inline">
            IP Lists
          </span>
        </Link>

        {/*
          Feed orientation strip — only rendered when the URL says
          we're on /ipsets/:name. Shows the current feed name (mono)
          and its category badge so the user never loses track of
          which feed they're viewing while scrolling.
        */}
        {routeFeedName && (
          <div className="hidden min-w-0 items-center gap-3 lg:flex">
            <span aria-hidden="true" className="text-display-muted">
              /
            </span>
            <span className="truncate font-mono text-[14px] font-semibold text-display-fg">
              {routeFeedName}
            </span>
            {feed && <CategoryBadge category={feed.category} />}
          </div>
        )}

        {showInlineSearch && (
          <div
            className={
              routeFeedName
                ? "ml-auto hidden min-w-0 md:block md:flex-1 md:max-w-[28rem] xl:max-w-[30rem]"
                : "ml-auto hidden min-w-0 sm:block sm:flex-1 sm:max-w-[30rem] lg:max-w-[32rem]"
            }
          >
            <IPSearchSurface
              scope={{ kind: "global" }}
              variant="header"
              placeholder="Search any IPv4 address"
            />
          </div>
        )}

        <nav className="flex shrink-0 items-center gap-1 text-[13px]">
          <Link
            to="/#explorer"
            className="rounded-sm px-3 py-2 text-display-muted transition-colors hover:text-display-fg"
          >
            Explore
          </Link>
          <Link
            to="/countries"
            className="rounded-sm px-3 py-2 text-display-muted transition-colors hover:text-display-fg"
          >
            Countries
          </Link>
          <Link
            to="/asns"
            className="rounded-sm px-3 py-2 text-display-muted transition-colors hover:text-display-fg"
          >
            ASNs
          </Link>
          <Link
            to="/maintainers"
            className="rounded-sm px-3 py-2 text-display-muted transition-colors hover:text-display-fg"
          >
            Maintainers
          </Link>
          <Link
            to="/methodology"
            className="rounded-sm px-3 py-2 text-display-muted transition-colors hover:text-display-fg"
          >
            Methodology
          </Link>
          {/* Admin is reachable via direct URL (/admin) — not advertised
              in the public header so casual visitors don't get prompted
              for credentials they don't have. */}
          <div className="ml-1">
            <ThemeToggle />
          </div>
        </nav>
      </div>
    </header>
  );
}
