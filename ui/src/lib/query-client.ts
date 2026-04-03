import { QueryClient } from "@tanstack/react-query";

/**
 * Single QueryClient for the whole app. The defaults trade memory for fewer
 * network requests — feed metadata changes infrequently and the worst case
 * is showing a stale page for a few minutes, never wrong data.
 */
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 2 * 60 * 1000, // 2 minutes
      gcTime: 30 * 60 * 1000,    // 30 minutes
      refetchOnWindowFocus: false,
      retry: 1,
    },
  },
});
