import { type ReactNode, useState } from "react";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";

type HoverTipText = ReactNode | (() => ReactNode);

function renderTipText(text: HoverTipText): ReactNode {
  return typeof text === "function" ? text() : text;
}

/**
 * The single tooltip primitive for the entire app.
 *
 * Wraps the four-element Radix triplet (Tooltip / TooltipTrigger asChild
 * / TooltipContent / TooltipProvider — the provider is mounted once at
 * the Layout root) into one tag so use sites stay readable:
 *
 *   <HoverTip text="Switch to dark mode">
 *     <Button>…</Button>
 *   </HoverTip>
 *
 * vs the raw Radix form:
 *
 *   <Tooltip>
 *     <TooltipTrigger asChild>
 *       <Button>…</Button>
 *     </TooltipTrigger>
 *     <TooltipContent>Switch to dark mode</TooltipContent>
 *   </Tooltip>
 *
 * Why this is the only tooltip API the rest of the codebase should know
 * about: see `AGENTS.md` "Tooltips" subsection. Browser-default `title=`
 * attributes and SVG `<title>` elements are forbidden — they render the
 * OS-native tooltip which has none of the editorial chrome and a
 * different delay budget than the rest of the app.
 *
 * Implementation notes:
 *   - The trigger uses `asChild` so the child element receives the
 *     Radix props directly (no extra wrapper span). Children must be a
 *     single React element that accepts a ref.
 *   - `text` accepts a string OR a ReactNode for occasional rich
 *     content (mono font, line breaks, links inside the tooltip body).
 *   - `side` and `align` map straight through to Radix; the defaults
 *     (`top` / `center`) are right for almost every use site.
 *   - The hover delay is configured ONCE on the TooltipProvider in
 *     Layout — do not override it per use site.
 */
export function HoverTip({
  text,
  children,
  side,
  align,
  delayDuration,
  lazy,
}: {
  /** Tooltip body. String for the common case, ReactNode for rich content. */
  text: HoverTipText;
  /** The element that triggers the tooltip on hover / focus. Must be
   *  a single React element that accepts a ref (Button, span, a, etc). */
  children: ReactNode;
  /** Radix side preference. Default `top`. */
  side?: "top" | "right" | "bottom" | "left";
  /** Radix alignment along the chosen side. Default `center`. */
  align?: "start" | "center" | "end";
  /** Override the global hover delay (set to 400ms in App.tsx). Use a
   *  shorter value on hover-heavy navigation areas like the feed
   *  sidebar where the user is scanning many rows quickly and the
   *  default delay feels sluggish. Leave undefined for everywhere
   *  else so the rest of the app shares one delay budget. */
  delayDuration?: number;
  /** When true, delay tooltip-body rendering until the tooltip is
   *  actually open. Use this on dense tables where many tooltips are
   *  present but only a tiny fraction are ever opened. */
  lazy?: boolean;
}) {
  const [open, setOpen] = useState(false);
  // Empty tooltips are a code smell — render the child unchanged so
  // a missing string never produces a stray empty bubble.
  if (text === undefined || text === null || text === "") {
    return <>{children}</>;
  }
  return (
    <Tooltip delayDuration={delayDuration} onOpenChange={setOpen}>
      <TooltipTrigger asChild>{children}</TooltipTrigger>
      {(!lazy || open) && (
        <TooltipContent side={side} align={align}>
          {renderTipText(text)}
        </TooltipContent>
      )}
    </Tooltip>
  );
}
