import { type ReactNode, useLayoutEffect, useRef, useState } from "react";

/**
 * Mouse-following tooltip surface for SVG charts.
 *
 * `<HoverTip>` (Radix-based) cannot be used inside SVGs because Radix
 * Tooltip needs an HTML element trigger that accepts a ref — `<rect>`,
 * `<circle>`, `<path>` and friends do not. Charts that render their
 * data as SVG primitives (geo choropleth, ASN treemap / bubble pack,
 * overlap sankey / network) therefore need a different mechanism.
 *
 * `<CursorTip>` is that mechanism. Each chart tracks the hovered
 * datum + cursor coordinates in React state and renders one of these
 * components as a sibling of the SVG. The visual chrome is identical
 * to the editorial Radix tooltip — same `bg-popover`, hairline
 * `border-border`, `rounded-sm`, 11px text — so the user cannot tell
 * the two systems apart.
 *
 * The component:
 *   - Renders at `position: fixed` against the viewport so it follows
 *     the cursor regardless of page scroll.
 *   - Defaults to a 14px down-and-right offset from the cursor so the
 *     panel does not block the next datum the user is trying to
 *     hover.
 *   - Measures itself after mount and flips horizontally / vertically
 *     when the chosen position would put the panel partially off-
 *     screen near the right / bottom edge.
 *   - Sets `pointer-events: none` so hovering the tooltip itself does
 *     not block the underlying chart's hover events.
 *   - Renders nothing when `x` or `y` are undefined.
 *
 * Charts must clear their hover state on `onMouseLeave` of the
 * triggering element so the panel disappears when the cursor moves
 * off the data.
 */
export function CursorTip({
  x,
  y,
  children,
  offset = 14,
}: {
  /** Viewport-relative cursor X (e.g. `event.clientX`). */
  x: number | undefined;
  /** Viewport-relative cursor Y (e.g. `event.clientY`). */
  y: number | undefined;
  /** Tooltip body. Use the editorial typography classes
   *  (`text-popover-foreground`, `tabular-nums`, `font-mono`) so the
   *  body matches `<HoverTip>` content visually. */
  children: ReactNode;
  /** Pixel gap between the cursor and the tooltip edge. Default 14. */
  offset?: number;
}) {
  const ref = useRef<HTMLDivElement | null>(null);
  const [size, setSize] = useState<{ w: number; h: number } | null>(null);

  // Measure after the rendered body changes so the edge-flip logic
  // uses the actual rendered dimensions, not a guess. The bail-out
  // (only call setSize when the value really changed) is critical:
  // without it, every parent re-render creates a fresh `{ w, h }`
  // object literal, React's Object.is() bailout fails, and we get
  // an extra re-render per mouse move. The double-render made the
  // tooltip flicker / appear to "travel" between positions during
  // continuous cursor motion.
  useLayoutEffect(() => {
    if (!ref.current) return;
    const rect = ref.current.getBoundingClientRect();
    setSize((prev) => {
      if (prev && prev.w === rect.width && prev.h === rect.height) return prev;
      return { w: rect.width, h: rect.height };
    });
  }, [children]);

  if (x === undefined || y === undefined) return null;

  // Default position: down-and-right of cursor.
  let left = x + offset;
  let top = y + offset;

  // Flip horizontally when the panel would extend past the right edge.
  if (size && typeof window !== "undefined") {
    if (left + size.w > window.innerWidth - 8) {
      left = x - offset - size.w;
    }
    if (top + size.h > window.innerHeight - 8) {
      top = y - offset - size.h;
    }
    // After flipping, clamp to a small viewport-edge margin so the
    // tooltip never disappears entirely on tiny mobile screens.
    if (left < 8) left = 8;
    if (top < 8) top = 8;
  }

  return (
    <div
      ref={ref}
      role="tooltip"
      className="pointer-events-none fixed left-0 top-0 z-50 max-w-xs rounded-sm border border-border bg-popover px-2.5 py-1.5 text-[11px] leading-snug text-popover-foreground shadow-md"
      // Use translate3d() instead of left/top so position updates run
      // on the compositor (GPU) rather than triggering layout +
      // paint on every mouse move. Explicit `transition: none` is a
      // belt-and-suspenders guard against any future global rule
      // that might add a transition to all `[role="tooltip"]`
      // elements — without it the tooltip would smoothly animate
      // between cursor positions, which we never want for a
      // mouse-following indicator.
      style={{
        transform: `translate3d(${left}px, ${top}px, 0)`,
        transition: "none",
        willChange: "transform",
      }}
    >
      {children}
    </div>
  );
}
