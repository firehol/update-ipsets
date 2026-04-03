import type {
  MethodologyIndexPayload,
  MethodologyPagePayload,
} from "../api-types";
import { fetchJSON, signalInit } from "./http";

export async function listMethodology(
  signal?: AbortSignal,
): Promise<MethodologyIndexPayload> {
  return fetchJSON<MethodologyIndexPayload>(
    "/api/v1/methodology",
    signalInit(signal),
  );
}

export async function getMethodologyPage(
  slug: string,
  signal?: AbortSignal,
): Promise<MethodologyPagePayload> {
  return fetchJSON<MethodologyPagePayload>(
    `/api/v1/methodology/${encodeURIComponent(slug)}`,
    signalInit(signal),
  );
}
