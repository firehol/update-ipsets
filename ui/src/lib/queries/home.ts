import { queryOptions } from "@tanstack/react-query";
import * as home from "@/lib/api-client/home";
import { queryKeys } from "@/lib/query-keys";

export const clientIPOptions = () =>
  queryOptions({
    queryKey: queryKeys.clientIP(),
    queryFn: ({ signal }) => home.getClientIP(signal),
  });
