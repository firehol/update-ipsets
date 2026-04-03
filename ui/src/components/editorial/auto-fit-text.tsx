import { useLayoutEffect, useRef, useState } from "react";

/**
 * AutoFitText renders a single line of text at the largest font size
 * that still fits its container. It solves the "long feed name
 * overlaps the stats column" problem on the feed detail hero: short
 * names like `dshield` render at the full display-hero size, long
 * names like `ri_connect_proxies_30d` shrink to fit on the same line.
 *
 * Implementation detail: there are TWO span children inside the
 * container — a hidden one fixed at `maxPx` used only for
 * measurement, and a visible one whose font size is state-driven.
 * Measuring via a separate element means the hook never mutates the
 * visible element's inline style directly, so Strict Mode's
 * double-invocation of effects does not leave a stale font size
 * behind (a problem the earlier single-span implementation had —
 * on the second effect run the state was already at the correct
 * value and the no-op setState skipped the re-render that would
 * have restored the visible span's size).
 */
export function AutoFitText({
  text,
  maxPx = 144,
  minPx = 32,
  className,
  style,
}: {
  text: string;
  /** Largest font size (px) to try. Never exceeded even when there's room. */
  maxPx?: number;
  /** Smallest font size (px). Won't shrink below this. */
  minPx?: number;
  className?: string;
  style?: React.CSSProperties;
}) {
  const containerRef = useRef<HTMLSpanElement | null>(null);
  const measureRef = useRef<HTMLSpanElement | null>(null);
  const [fontPx, setFontPx] = useState(maxPx);

  useLayoutEffect(() => {
    const container = containerRef.current;
    const measurer = measureRef.current;
    if (!container || !measurer) return;

    function fit() {
      if (!container || !measurer) return;
      const containerWidth = container.clientWidth;
      // The measurer is fixed at maxPx, so scrollWidth here is the
      // reference "text width at maximum size". containerWidth /
      // refWidth gives us the fit ratio directly.
      const refWidth = measurer.scrollWidth;
      if (containerWidth === 0 || refWidth === 0) return;
      // 0.995 fudge prevents sub-pixel clipping.
      const scale = (containerWidth * 0.995) / refWidth;
      const next = Math.floor(Math.min(maxPx, Math.max(minPx, maxPx * scale)));
      setFontPx(next);
    }

    fit();
    const ro = new ResizeObserver(fit);
    ro.observe(container);
    return () => ro.disconnect();
  }, [text, maxPx, minPx]);

  return (
    <span
      ref={containerRef}
      className={className}
      style={{
        display: "block",
        position: "relative",
        overflow: "hidden",
        ...style,
      }}
    >
      {/* Hidden reference: always at maxPx, invisible, absolutely
          positioned so it does not contribute to layout. Its
          scrollWidth is the text's natural width at maximum size. */}
      <span
        ref={measureRef}
        aria-hidden="true"
        style={{
          position: "absolute",
          visibility: "hidden",
          pointerEvents: "none",
          whiteSpace: "nowrap",
          fontSize: `${maxPx}px`,
          lineHeight: 0.92,
          letterSpacing: "-0.04em",
          left: 0,
          top: 0,
        }}
      >
        {text}
      </span>
      {/* Visible text at the state-driven font size. */}
      <span
        style={{
          display: "inline-block",
          whiteSpace: "nowrap",
          fontSize: `${fontPx}px`,
          lineHeight: 0.92,
          letterSpacing: "-0.04em",
        }}
      >
        {text}
      </span>
    </span>
  );
}
