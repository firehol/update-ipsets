import type { FeedMetadata } from "../api-types";
import { fetchJSON, signalInit } from "./http";

export async function getFeed(
  name: string,
  signal?: AbortSignal,
): Promise<FeedMetadata> {
  return fetchJSON<FeedMetadata>(
    `/api/v1/sets/${encodeURIComponent(name)}`,
    signalInit(signal),
  );
}
