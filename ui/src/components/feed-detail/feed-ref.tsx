import { Link } from "react-router-dom";
import { forwardRef, type ComponentProps, type ReactNode } from "react";
import { HoverTip } from "@/components/editorial/hover-tip";
import { cn } from "@/lib/utils";
import type { FeedRefDescriptor } from "./feed-ref-descriptor";

type FeedRefProps = Omit<ComponentProps<typeof Link>, "to"> & {
  name: string;
  feed?: Partial<FeedRefDescriptor> | null;
  description?: ReactNode;
  fallbackDescription?: ReactNode;
};

function feedRefDescription(
  feed?: Partial<FeedRefDescriptor> | null,
  fallback?: ReactNode,
): ReactNode {
  if (!feed) return fallback ?? null;
  const title = feed.official_name?.trim() || feed.name?.trim() || "";
  const short = feed.short_description?.trim() || "";
  const maintainer = feed.maintainer?.trim() || "";
  if (!title && !short && !maintainer) return fallback ?? null;
  return (
    <span className="block max-w-xs text-left">
      {title && <span className="block font-semibold text-foreground">{title}</span>}
      {short && <span className="mt-1 block leading-relaxed">{short}</span>}
      {maintainer && (
        <span className="mt-1 block text-[11px] uppercase tracking-[0.08em] text-muted-foreground">
          Maintainer: {maintainer}
        </span>
      )}
    </span>
  );
}

export const FeedRef = forwardRef<HTMLAnchorElement, FeedRefProps>(function FeedRef(
  {
    name,
    feed,
    description,
    fallbackDescription,
    className,
    children,
    ...linkProps
  },
  ref,
) {
  const tooltip = description ?? feedRefDescription(feed, fallbackDescription);
  const link = (
    <Link
      ref={ref}
      to={`/ipsets/${encodeURIComponent(name)}`}
      className={cn(
        children ? "" : "font-mono text-primary underline-offset-4 hover:underline",
        className,
      )}
      {...linkProps}
    >
      {children ?? name}
    </Link>
  );
  if (!tooltip) return link;
  return <HoverTip text={tooltip}>{link}</HoverTip>;
});
