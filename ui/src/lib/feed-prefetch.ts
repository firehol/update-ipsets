import { useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { feedOptions } from "@/lib/queries/feed-core";

export function usePrefetchFeedDetail(name: string) {
  const queryClient = useQueryClient();
  return useCallback(() => {
    if (!name) return;
    void queryClient.prefetchQuery(feedOptions(name));
  }, [name, queryClient]);
}
