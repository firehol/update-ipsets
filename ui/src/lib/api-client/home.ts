import type { ClientIPPayload } from "../api-types";
import { fetchJSON, signalInit } from "./http";

export async function getClientIP(
  signal?: AbortSignal,
): Promise<ClientIPPayload> {
  return fetchJSON<ClientIPPayload>("/api/v1/client-ip", signalInit(signal));
}
