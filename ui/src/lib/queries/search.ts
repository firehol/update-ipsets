import { queryOptions } from "@tanstack/react-query";
import * as search from "@/lib/api-client/search";
import { queryKeys } from "@/lib/query-keys";

export const ipSearchOptions = (
  ip: string,
  scope: { kind: "global" } | { kind: "feed"; feedName: string },
  details = true,
) =>
  queryOptions({
    queryKey: queryKeys.ipSearch(
      scope.kind === "feed" ? scope.feedName : "global",
      ip,
      details,
    ),
    queryFn: ({ signal }) =>
      scope.kind === "feed"
        ? search.searchIPInFeed(scope.feedName, ip, details, signal)
        : search.searchIP(ip, details, signal),
    enabled: ip.length > 0,
  });
