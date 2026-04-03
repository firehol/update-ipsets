import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { useCategoriesQuery } from "@/lib/categories";
import { publicExplorerFeeds } from "@/lib/explorer-state";
import { feedsOptions } from "@/lib/queries/catalog";
import { HomeHero } from "@/components/home/home-hero";
import { HomeIPLookup } from "@/components/home/home-ip-lookup";
import { HomeExplorer } from "@/components/home/home-explorer";

export function HomePage() {
  const feedsQuery = useQuery(feedsOptions());

  const categoriesQuery = useCategoriesQuery();

  const feeds = useMemo(() => feedsQuery.data ?? [], [feedsQuery.data]);
  const categories = useMemo(
    () => categoriesQuery.data ?? [],
    [categoriesQuery.data],
  );

  const overview = useMemo(() => {
    const publicFeeds = publicExplorerFeeds(feeds, categories);

    const maintainerSet = new Set<string>();
    const categorySet = new Set<string>();
    for (const feed of publicFeeds) {
      if (feed.maintainer) maintainerSet.add(feed.maintainer.trim());
      if (feed.category) categorySet.add(feed.category);
    }

    return {
      trackedFeeds: publicFeeds.length,
      maintainers: maintainerSet.size,
      categoryCount: categorySet.size,
    };
  }, [feeds, categories]);

  return (
    <div>
      <HomeHero
        loading={feedsQuery.isLoading || categoriesQuery.isLoading}
        trackedFeeds={overview.trackedFeeds}
        maintainers={overview.maintainers}
        categoryCount={overview.categoryCount}
      />

      <HomeIPLookup />

      <HomeExplorer
        feeds={feeds}
        categories={categories}
        loading={feedsQuery.isLoading || categoriesQuery.isLoading}
      />
    </div>
  );
}
