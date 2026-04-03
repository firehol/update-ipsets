import * as React from "react";
import * as TooltipPrimitive from "@radix-ui/react-tooltip";

import { cn } from "@/lib/utils";

/**
 * Radix Tooltip primitive, restyled for the editorial design system.
 *
 * The shadcn default uses `bg-primary` (FireHOL red) which clashes with
 * every other surface in the app. Editorial tooltips should look like
 * the popover / dropdown surfaces — `bg-popover` background with a
 * hairline `border-border` and `text-popover-foreground` text. Same
 * radius (`rounded-sm`) as MinimalTable / FeedSidebar so tooltips feel
 * like a stamp on top of the existing chrome rather than a foreign
 * element.
 *
 * Application code should NOT import these primitives directly. Use
 * `<HoverTip>` from `@/components/editorial/hover-tip` instead — it
 * collapses the four-element Radix triplet into a single tag and is
 * the only place that should know about Radix Tooltip.
 */
const TooltipProvider = TooltipPrimitive.Provider;
const Tooltip = TooltipPrimitive.Root;
const TooltipTrigger = TooltipPrimitive.Trigger;

const TooltipContent = React.forwardRef<
  React.ComponentRef<typeof TooltipPrimitive.Content>,
  React.ComponentPropsWithoutRef<typeof TooltipPrimitive.Content>
>(({ className, sideOffset = 6, ...props }, ref) => (
  <TooltipPrimitive.Portal>
    <TooltipPrimitive.Content
      ref={ref}
      sideOffset={sideOffset}
      className={cn(
        // Surface — matches popover / dropdown chrome
        "z-50 max-w-xs rounded-sm border border-border bg-popover px-2.5 py-1.5",
        // Text — small, tight, popover foreground colour
        "text-[11px] leading-snug text-popover-foreground",
        // Subtle drop shadow so the tooltip detaches from the underlying surface
        "shadow-md",
        // Radix slide-in / fade animation, kept verbatim from the shadcn defaults
        "animate-in fade-in-0 zoom-in-95",
        "data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95",
        "data-[side=bottom]:slide-in-from-top-2 data-[side=left]:slide-in-from-right-2",
        "data-[side=right]:slide-in-from-left-2 data-[side=top]:slide-in-from-bottom-2",
        "origin-[--radix-tooltip-content-transform-origin]",
        className,
      )}
      {...props}
    />
  </TooltipPrimitive.Portal>
));
TooltipContent.displayName = TooltipPrimitive.Content.displayName;

export { Tooltip, TooltipTrigger, TooltipContent, TooltipProvider };
