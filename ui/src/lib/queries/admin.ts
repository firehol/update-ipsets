import { queryOptions } from "@tanstack/react-query";
import * as admin from "@/lib/api-client/admin";
import { queryKeys } from "@/lib/query-keys";

export const adminStatusOptions = () =>
  queryOptions({
    queryKey: queryKeys.adminStatus(),
    queryFn: ({ signal }) => admin.adminStatus(signal),
    refetchInterval: 3000,
  });

export const adminFeedsOptions = () =>
  queryOptions({
    queryKey: queryKeys.adminFeeds(),
    queryFn: ({ signal }) => admin.adminFeeds(signal),
  });

export const adminArtifactsOptions = () =>
  queryOptions({
    queryKey: queryKeys.adminArtifacts(),
    queryFn: ({ signal }) => admin.adminArtifacts(signal),
  });

export const adminManifestOptions = (name: string) =>
  queryOptions({
    queryKey: queryKeys.adminManifest(name),
    queryFn: ({ signal }) => admin.adminFeedManifest(name, signal),
    enabled: name.length > 0,
  });

export const adminIntegrityOptions = (includeArchived = false) =>
  queryOptions({
    queryKey: queryKeys.adminIntegrity(includeArchived),
    queryFn: ({ signal }) =>
      admin.adminIntegrity({ includeArchived }, signal),
    refetchInterval: 5000,
  });

export const adminEntityIntegrityOptions = () =>
  queryOptions({
    queryKey: queryKeys.adminEntityIntegrity(),
    queryFn: ({ signal }) => admin.adminEntityIntegrity(signal),
  });
