import { queryOptions } from "@tanstack/react-query";
import * as feed from "@/lib/api-client/feed";
import { queryKeys } from "@/lib/query-keys";

export const asnProvidersOptions = (name: string) =>
  queryOptions({
    queryKey: queryKeys.asnProviders(name),
    queryFn: ({ signal }) => feed.listASNProviders(name, signal),
    staleTime: 5 * 60 * 1000,
  });

export const asnFeedOptions = (name: string, provider: string) =>
  queryOptions({
    queryKey: queryKeys.asnFeed(name, provider),
    queryFn: ({ signal }) => feed.getASNFeed(name, provider, signal),
    staleTime: 5 * 60 * 1000,
  });

export const geoProvidersOptions = (name: string) =>
  queryOptions({
    queryKey: queryKeys.geoProviders(name),
    queryFn: ({ signal }) => feed.listGeoProviders(name, signal),
    staleTime: 5 * 60 * 1000,
  });

export const geoFeedOptions = (name: string, provider: string) =>
  queryOptions({
    queryKey: queryKeys.geoFeed(name, provider),
    queryFn: ({ signal }) => feed.getGeoFeed(name, provider, signal),
    staleTime: 5 * 60 * 1000,
  });

export const bogonProvidersOptions = (name: string) =>
  queryOptions({
    queryKey: queryKeys.bogonProviders(name),
    queryFn: ({ signal }) => feed.listBogonProviders(name, signal),
    staleTime: 5 * 60 * 1000,
  });

export const bogonFeedOptions = (name: string, provider: string) =>
  queryOptions({
    queryKey: queryKeys.bogonFeed(name, provider),
    queryFn: ({ signal }) => feed.getBogonFeed(name, provider, signal),
    staleTime: 5 * 60 * 1000,
  });

export const criticalProvidersOptions = (name: string) =>
  queryOptions({
    queryKey: queryKeys.criticalProviders(name),
    queryFn: ({ signal }) =>
      feed.listCriticalInfrastructureProviders(name, signal),
  });

export const criticalAggregateOptions = (name: string) =>
  queryOptions({
    queryKey: queryKeys.criticalAggregate(name),
    queryFn: ({ signal }) => feed.getCriticalInfrastructure(name, signal),
  });

export const criticalProviderOptions = (name: string, provider: string) =>
  queryOptions({
    queryKey: queryKeys.criticalProvider(name, provider),
    queryFn: ({ signal }) =>
      feed.getCriticalInfrastructureProvider(name, provider, signal),
  });

export const comparisonOptions = (name: string) =>
  queryOptions({
    queryKey: queryKeys.comparison(name),
    queryFn: ({ signal }) => feed.getComparison(name, signal),
  });

export const historyOptions = (name: string) =>
  queryOptions({
    queryKey: queryKeys.history(name),
    queryFn: ({ signal }) => feed.getHistoryCSV(name, signal),
  });

export const changesetsOptions = (name: string) =>
  queryOptions({
    queryKey: queryKeys.changesets(name),
    queryFn: ({ signal }) => feed.getChangesets(name, signal),
  });

export const retentionOptions = (name: string) =>
  queryOptions({
    queryKey: queryKeys.retention(name),
    queryFn: ({ signal }) => feed.getRetention(name, signal),
  });

export const insightsOptions = (name: string) =>
  queryOptions({
    queryKey: queryKeys.insights(name),
    queryFn: ({ signal }) => feed.getInsights(name, signal),
  });
