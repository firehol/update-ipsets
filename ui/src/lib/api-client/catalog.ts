import type { CategoryMeta, FeedSummary } from "../api-types";
import { fetchJSON, signalInit } from "./http";

export async function listFeeds(signal?: AbortSignal): Promise<FeedSummary[]> {
  return fetchJSON<FeedSummary[]>("/api/v1/sets", signalInit(signal));
}

export async function listCategories(
  signal?: AbortSignal,
): Promise<CategoryMeta[]> {
  return fetchJSON<CategoryMeta[]>("/api/v1/categories", signalInit(signal));
}
