import { queryOptions } from "@tanstack/react-query";
import { getFeed } from "@/lib/api-client/feed-core";
import { queryKeys } from "@/lib/query-keys";

export const feedOptions = (name: string) =>
  queryOptions({
    queryKey: queryKeys.feed(name),
    queryFn: ({ signal }) => getFeed(name, signal),
  });
