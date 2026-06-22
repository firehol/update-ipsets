import type {
  AdminArtifact,
  AdminFeed,
  AdminStatus,
  EntityIntegrityActionResult,
  EntityIntegrityReport,
  FeedManifest,
  IntegrityReport,
  IntegrityReprocessResult,
} from "../api-types";
import { fetchJSON, signalInit } from "./http";

export async function adminStatus(signal?: AbortSignal): Promise<AdminStatus> {
  return fetchJSON<AdminStatus>(
    "/api/v1/admin/status?mode=light",
    signalInit(signal),
  );
}

export async function adminFeeds(signal?: AbortSignal): Promise<AdminFeed[]> {
  return fetchJSON<AdminFeed[]>("/api/v1/admin/feeds", signalInit(signal));
}

export async function adminArtifacts(
  signal?: AbortSignal,
): Promise<AdminArtifact[]> {
  return fetchJSON<AdminArtifact[]>(
    "/api/v1/admin/artifacts",
    signalInit(signal),
  );
}

export async function adminRunAll(opts?: {
  recheck?: boolean;
  reprocess?: boolean;
}) {
  const params = new URLSearchParams();
  if (opts?.recheck) params.set("recheck", "true");
  if (opts?.reprocess) params.set("reprocess", "true");
  const queryString = params.toString();
  return fetchJSON<{ status: string }>(
    `/api/v1/admin/run${queryString ? `?${queryString}` : ""}`,
    { method: "POST" },
  );
}

export async function adminRecheckFeed(name: string) {
  return fetchJSON<{ status: string; name: string }>(
    `/api/v1/admin/feeds/${encodeURIComponent(name)}/recheck`,
    { method: "POST" },
  );
}

export async function adminReprocessFeed(name: string) {
  return fetchJSON<{ status: string; name: string }>(
    `/api/v1/admin/feeds/${encodeURIComponent(name)}/reprocess`,
    { method: "POST" },
  );
}

export async function adminEnableFeed(name: string) {
  return fetchJSON<{ status: string; name: string }>(
    `/api/v1/admin/feeds/${encodeURIComponent(name)}/enable`,
    { method: "POST" },
  );
}

export async function adminDisableFeed(name: string) {
  return fetchJSON<{ status: string; name: string }>(
    `/api/v1/admin/feeds/${encodeURIComponent(name)}/disable`,
    { method: "POST" },
  );
}

export async function adminRecheckArtifact(name: string) {
  return fetchJSON<{ status: string; name: string }>(
    `/api/v1/admin/artifacts/${encodeURIComponent(name)}/recheck`,
    { method: "POST" },
  );
}

export async function adminEnableArtifact(name: string) {
  return fetchJSON<{ status: string; name: string }>(
    `/api/v1/admin/artifacts/${encodeURIComponent(name)}/enable`,
    { method: "POST" },
  );
}

export async function adminDisableArtifact(name: string) {
  return fetchJSON<{ status: string; name: string }>(
    `/api/v1/admin/artifacts/${encodeURIComponent(name)}/disable`,
    { method: "POST" },
  );
}

export async function adminFeedManifest(
  name: string,
  signal?: AbortSignal,
): Promise<FeedManifest> {
  return fetchJSON<FeedManifest>(
    `/api/v1/admin/feeds/${encodeURIComponent(name)}/manifest`,
    signalInit(signal),
  );
}

export async function adminIntegrity(
  opts?: { includeArchived?: boolean },
  signal?: AbortSignal,
): Promise<IntegrityReport> {
  const params = new URLSearchParams();
  if (opts?.includeArchived) params.set("include_archived", "true");
  const queryString = params.toString();
  return fetchJSON<IntegrityReport>(
    `/api/v1/admin/integrity${queryString ? `?${queryString}` : ""}`,
    signalInit(signal),
  );
}

export async function adminIntegrityReprocess(opts?: {
  includeArchived?: boolean;
}): Promise<IntegrityReprocessResult> {
  const params = new URLSearchParams();
  if (opts?.includeArchived) params.set("include_archived", "true");
  const queryString = params.toString();
  return fetchJSON<IntegrityReprocessResult>(
    `/api/v1/admin/integrity/reprocess${queryString ? `?${queryString}` : ""}`,
    { method: "POST" },
  );
}

export async function adminEntityIntegrity(
  signal?: AbortSignal,
): Promise<EntityIntegrityReport> {
  return fetchJSON<EntityIntegrityReport>(
    "/api/v1/admin/integrity/entities",
    signalInit(signal),
  );
}

export async function adminRebuildEntityArtifacts(): Promise<EntityIntegrityActionResult> {
  return fetchJSON<EntityIntegrityActionResult>(
    "/api/v1/admin/integrity/entities/rebuild",
    { method: "POST" },
  );
}
