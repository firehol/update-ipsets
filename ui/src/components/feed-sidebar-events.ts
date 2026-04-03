import { useEffect } from "react";

export const OVERLAY_OPEN_EVENT = "firehol:sidebar:open";

export function openFeedSidebarOverlay(): void {
  window.dispatchEvent(new CustomEvent(OVERLAY_OPEN_EVENT));
}

/** Registers the global Cmd+K / Ctrl+K shortcut. Opens the overlay on
 *  small screens; on xl:+ it focuses the inline sidebar's search box
 *  without having to open anything. */
export function useFeedSidebarShortcut(): void {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (!((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k")) return;
      e.preventDefault();
      const inlineSearch = document.querySelector<HTMLInputElement>(
        'aside[aria-label="Feed navigator"] input[type="search"]',
      );
      if (inlineSearch && inlineSearch.offsetParent !== null) {
        inlineSearch.focus();
        inlineSearch.select();
        return;
      }
      openFeedSidebarOverlay();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);
}
