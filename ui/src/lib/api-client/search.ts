import type { IPSearchResult } from "../api-types";
import { fetchJSON, signalInit } from "./http";

export async function searchIP(
  ip: string,
  details = false,
  signal?: AbortSignal,
): Promise<IPSearchResult> {
  const query = new URLSearchParams({ ip });
  if (details) query.set("details", "true");
  return fetchJSON<IPSearchResult>(
    `/api/v1/search?${query.toString()}`,
    signalInit(signal),
  );
}

export async function searchIPInFeed(
  name: string,
  ip: string,
  details = false,
  signal?: AbortSignal,
): Promise<IPSearchResult> {
  const query = new URLSearchParams({ ip });
  if (details) query.set("details", "true");
  return fetchJSON<IPSearchResult>(
    `/api/v1/sets/${encodeURIComponent(name)}/search?${query.toString()}`,
    signalInit(signal),
  );
}
