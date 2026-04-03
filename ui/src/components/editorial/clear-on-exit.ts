import { type RefObject, useEffect } from "react";

/**
 * Bulletproof "clear hover when cursor leaves the chart" hook.
 *
 * A sticky-tooltip bug made SVG chart tooltips sometimes stay visible
 * after the cursor exited the chart area.
 * Several attempts to fix it via `onMouseLeave` (per-element,
 * per-svg, per-wrapping-div) all turned out to be unreliable in
 * different ways:
 *
 *   - Per-element handlers can be dropped by the browser when the
 *     cursor moves quickly across small SVG primitives.
 *   - Per-SVG handlers do not fire when an SVG disables pointer
 *     events on the root and re-enables them only on children.
 *   - Per-div handlers fire only when the cursor exits the div's
 *     bounding box, which can be wider than the visible chart
 *     when the wrapper div spans the full container width with no
 *     explicit width.
 *
 * This hook installs a single global `mousemove` listener WHILE
 * hover is active and clears the state the moment the cursor's
 * `clientX` / `clientY` are outside the referenced element's
 * bounding box. It does not depend on the React event system at
 * all, so it cannot be defeated by any of the above quirks.
 *
 * The listener is only attached while `active` is `true` so the
 * background CPU cost is zero when no chart is being hovered.
 */
export function useClearOnExit(
  ref: RefObject<HTMLElement | null>,
  active: boolean,
  clear: () => void,
): void {
  useEffect(() => {
    if (!active) return;
    const onMove = (e: MouseEvent) => {
      const el = ref.current;
      if (!el) return;
      const r = el.getBoundingClientRect();
      if (
        e.clientX < r.left ||
        e.clientX > r.right ||
        e.clientY < r.top ||
        e.clientY > r.bottom
      ) {
        clear();
      }
    };
    window.addEventListener("mousemove", onMove);
    return () => window.removeEventListener("mousemove", onMove);
  }, [active, ref, clear]);
}
