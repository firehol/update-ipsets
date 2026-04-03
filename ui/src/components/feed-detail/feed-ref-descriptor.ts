import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import type { FeedSummary } from "@/lib/api-types";
import { feedsOptions } from "@/lib/queries/catalog";

export type FeedRefDescriptor = Pick<
  FeedSummary,
  "name" | "official_name" | "short_description" | "maintainer"
>;

/**
 * Resolve a feed's tooltip descriptor by name from the global feeds
 * catalog. Used by call sites that only know the feed name (overlap
 * rows, source feeds, in-feed-lookup result rows).
 */
export function useFeedRefDescriptor(name: string | null | undefined): FeedRefDescriptor | null {
  const map = useFeedRefDescriptorMap();
  if (!name) return null;
  return map.get(name) ?? null;
}

/**
 * Build the descriptor map once per render. Lists that render many
 * feed references should call this at the top and pass descriptors
 * down per row instead of calling `useFeedRefDescriptor` per row.
 */
export function useFeedRefDescriptorMap(): Map<string, FeedRefDescriptor> {
  const query = useQuery({ ...feedsOptions(), staleTime: 5 * 60 * 1000 });
  return useMemo(() => {
    const out = new Map<string, FeedRefDescriptor>();
    for (const feed of query.data ?? []) {
      out.set(feed.name, {
        name: feed.name,
        official_name: feed.official_name,
        short_description: feed.short_description,
        maintainer: feed.maintainer,
      });
    }
    return out;
  }, [query.data]);
}
