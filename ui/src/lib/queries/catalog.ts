import { queryOptions } from "@tanstack/react-query";
import * as catalog from "@/lib/api-client/catalog";
import { queryKeys } from "@/lib/query-keys";

export const feedsOptions = () =>
  queryOptions({
    queryKey: queryKeys.feeds(),
    queryFn: ({ signal }) => catalog.listFeeds(signal),
  });

export const categoriesOptions = () =>
  queryOptions({
    queryKey: queryKeys.categories(),
    queryFn: ({ signal }) => catalog.listCategories(signal),
    staleTime: 5 * 60 * 1000,
  });
