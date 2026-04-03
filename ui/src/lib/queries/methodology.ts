import { queryOptions } from "@tanstack/react-query";
import * as methodology from "@/lib/api-client/methodology";
import { queryKeys } from "@/lib/query-keys";

export const methodologyOptions = () =>
  queryOptions({
    queryKey: queryKeys.methodology(),
    queryFn: ({ signal }) => methodology.listMethodology(signal),
  });

export const methodologyPageOptions = (slug: string) =>
  queryOptions({
    queryKey: queryKeys.methodologyPage(slug),
    queryFn: ({ signal }) => methodology.getMethodologyPage(slug, signal),
    enabled: slug.length > 0,
  });
