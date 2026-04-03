import { useQuery } from "@tanstack/react-query";

export const WORLD_TOPOLOGY_PATH = "/world/countries-110m.json";

export function useWorldTopology() {
  return useQuery({
    queryKey: ["world-topology"],
    queryFn: async ({ signal }) => {
      const response = await fetch(WORLD_TOPOLOGY_PATH, { signal });
      if (!response.ok) {
        throw new Error(`failed to load world topology: ${response.status}`);
      }
      return response.json() as Promise<Record<string, unknown>>;
    },
    staleTime: Infinity,
    gcTime: Infinity,
    retry: 2,
    refetchOnMount: false,
    refetchOnWindowFocus: false,
  });
}
