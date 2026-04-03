import { queryOptions } from "@tanstack/react-query";
import * as entities from "@/lib/api-client/entities";
import { queryKeys } from "@/lib/query-keys";

export const countriesOptions = () =>
  queryOptions({
    queryKey: queryKeys.countries(),
    queryFn: ({ signal }) => entities.listCountries(signal),
  });

export const countryOptions = (code: string) =>
  queryOptions({
    queryKey: queryKeys.country(code),
    queryFn: ({ signal }) => entities.getCountryDetail(code, signal),
  });

export const asnsOptions = () =>
  queryOptions({
    queryKey: queryKeys.asns(),
    queryFn: ({ signal }) => entities.listASNs(signal),
  });

export const asnOptions = (asn: number | string) =>
  queryOptions({
    queryKey: queryKeys.asn(asn),
    queryFn: ({ signal }) => entities.getASNDetail(asn, signal),
  });

export const maintainersOptions = (categories: string[] = []) =>
  queryOptions({
    queryKey: queryKeys.maintainers(categories),
    queryFn: ({ signal }) => entities.listMaintainers(categories, signal),
  });

export const maintainerOptions = (slug: string) =>
  queryOptions({
    queryKey: queryKeys.maintainer(slug),
    queryFn: ({ signal }) => entities.getMaintainerDetail(slug, signal),
  });
