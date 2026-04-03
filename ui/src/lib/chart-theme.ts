import { useSyncExternalStore } from "react";

/**
 * Theme-aware colour tokens for chart libraries.
 *
 * All values are CSS custom-property references (e.g. `var(--chart-accent)`).
 * Both Recharts and raw SVG pass them straight through as `fill=` /
 * `stroke=` attribute values, and the browser resolves them at PAINT time.
 * Theme switching therefore becomes a pure CSS repaint — zero React work,
 * zero chart re-renders. Previously we returned resolved hex values,
 * which meant every `.dark` class toggle fired ~10 simultaneous Recharts
 * re-renders and the visible UI lagged 2–3s per toggle.
 *
 * The only caller that cannot use CSS variables is `GeoMap`, because
 * d3-scale `scaleSqrt` needs parseable hex strings for colour
 * interpolation. That one uses the `useIsDark()` hook below to pick
 * from a hardcoded palette per theme.
 */
export interface ChartTheme {
  /** Primary accent — the single highlight colour in every chart. */
  accent: string;
  /** Second accent hue (used sparingly, e.g. the negative-delta bars). */
  secondary: string;
  /** Neutral context fill for non-highlight chart marks. */
  context: string;
  /** Thin grid lines — should almost blend with the background. */
  grid: string;
  /** Axis labels and tick marks. */
  axis: string;
  /** Tooltip background (solid, not transparent). */
  tooltipBg: string;
  /** Tooltip border. */
  tooltipBorder: string;
  /** Tooltip text colour. */
  tooltipFg: string;
  /** Tooltip elevation shadow. */
  tooltipShadow: string;
}

/*
  Static frozen object. Every token is a CSS variable reference, so the
  resolved colour depends on whether `.dark` is present on <html>. The
  object reference NEVER changes — no subscription, no re-render cost.
  Keep it frozen so an accidental mutation anywhere in the app fails
  loudly rather than corrupting every chart at once.
*/
const CHART_THEME: ChartTheme = Object.freeze({
  accent: "var(--chart-accent)",
  secondary: "var(--chart-secondary)",
  context: "var(--chart-context)",
  grid: "var(--chart-grid)",
  axis: "var(--chart-axis)",
  tooltipBg: "var(--chart-tooltip-bg)",
  tooltipBorder: "var(--chart-tooltip-border)",
  tooltipFg: "var(--chart-tooltip-fg)",
  tooltipShadow: "var(--chart-tooltip-shadow)",
});

/**
 * The old hook is preserved as a simple getter so existing callers keep
 * working unchanged. It is NOT a React subscription anymore — the
 * returned object is identity-stable forever, so a component that reads
 * it will never re-render due to a theme change. The hook form is kept
 * only to avoid a sweeping find/replace in every chart component.
 */
export function useChartTheme(): ChartTheme {
  return CHART_THEME;
}

/* ============================================================================
   useIsDark — a tiny boolean subscription for the handful of callers that
   need an actual JS-side theme flag (GeoMap uses d3-scale to interpolate
   between two parseable hex values, which CSS variables cannot express).

   One MutationObserver is shared by every subscriber, and the snapshot is
   just a boolean so React bails out via Object.is() unless the theme
   actually flips. Components that only use `useChartTheme()` do NOT
   subscribe here.
   ========================================================================== */

let cachedIsDark: boolean = readIsDarkFromDOM();
const darkListeners = new Set<() => void>();
let darkObserver: MutationObserver | null = null;

function readIsDarkFromDOM(): boolean {
  if (typeof document === "undefined") return true; // SSR / pre-hydrate → default dark
  return document.documentElement.classList.contains("dark");
}

function ensureDarkObserver(): void {
  if (darkObserver !== null || typeof document === "undefined") return;
  darkObserver = new MutationObserver(() => {
    const next = readIsDarkFromDOM();
    if (next === cachedIsDark) return;
    cachedIsDark = next;
    for (const listener of darkListeners) listener();
  });
  darkObserver.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ["class"],
  });
}

function subscribeDark(listener: () => void): () => void {
  ensureDarkObserver();
  darkListeners.add(listener);
  return () => {
    darkListeners.delete(listener);
  };
}

function getDarkSnapshot(): boolean {
  // Re-read the DOM on every snapshot call so the first render after
  // module load sees the real state even if ThemeProvider's effect has
  // already committed `.dark` by then. Reference-stable booleans keep
  // useSyncExternalStore from triggering spurious renders.
  const current = readIsDarkFromDOM();
  if (current !== cachedIsDark) cachedIsDark = current;
  return cachedIsDark;
}

function getDarkServerSnapshot(): boolean {
  return true;
}

/**
 * Returns `true` while `.dark` is present on <html>. Reserved for callers
 * that need a real JS-side theme flag — primarily `GeoMap`, which feeds
 * d3-scale interpolation with hardcoded hex endpoints per theme. Normal
 * chart components should use `useChartTheme()` instead and let the CSS
 * variables do the work.
 */
export function useIsDark(): boolean {
  return useSyncExternalStore(subscribeDark, getDarkSnapshot, getDarkServerSnapshot);
}
