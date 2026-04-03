import { Outlet } from "react-router-dom";
import { SiteHeader } from "./site-header";
import { SiteFooter } from "./site-footer";
import {
  FeedSidebarInline,
  FeedSidebarOverlay,
} from "./feed-sidebar";
import { useFeedSidebarShortcut } from "./feed-sidebar-events";
import { Toaster } from "@/components/ui/sonner";

/**
 * Shared layout for every page on the public site. The admin SPA uses its
 * own layout because it needs different chrome (no public footer, basic-auth
 * gating, condensed header).
 *
 * The feed sidebar is rendered as a sibling of `<main>` rather than a
 * flex child because it uses `position: fixed` and therefore does NOT
 * participate in the normal flow. The centred page-container inside
 * `<main>` stays centred in the viewport regardless of sidebar
 * visibility — the sidebar simply floats on top of the otherwise empty
 * left gutter space when the viewport is wide enough (≥1680px). On
 * narrower screens it stays hidden and the hamburger in the site
 * header opens an overlay drawer variant instead.
 */
export function Layout() {
  // Wire the global ⌘K shortcut once at the layout root so every
  // public page inherits it without having to re-register.
  useFeedSidebarShortcut();

  return (
    <div className="flex min-h-screen flex-col">
      <SiteHeader />
      <main className="flex-1">
        <Outlet />
      </main>
      <SiteFooter />
      <FeedSidebarInline />
      <FeedSidebarOverlay />
      <Toaster richColors closeButton />
    </div>
  );
}
