import type {
  ASNDetailPayload,
  ASNIndexPayload,
  CountryDetailPayload,
  CountryIndexPayload,
  MaintainerDetailPayload,
  MaintainerIndexPayload,
} from "../api-types";
import { fetchJSON, signalInit } from "./http";

export async function listCountries(
  signal?: AbortSignal,
): Promise<CountryIndexPayload> {
  return fetchJSON<CountryIndexPayload>("/api/v1/countries", signalInit(signal));
}

export async function getCountryDetail(
  code: string,
  signal?: AbortSignal,
): Promise<CountryDetailPayload> {
  return fetchJSON<CountryDetailPayload>(
    `/api/v1/countries/${encodeURIComponent(code.toUpperCase())}`,
    signalInit(signal),
  );
}

export async function listASNs(signal?: AbortSignal): Promise<ASNIndexPayload> {
  return fetchJSON<ASNIndexPayload>("/api/v1/asns", signalInit(signal));
}

export async function getASNDetail(
  asn: number | string,
  signal?: AbortSignal,
): Promise<ASNDetailPayload> {
  return fetchJSON<ASNDetailPayload>(
    `/api/v1/asns/${encodeURIComponent(String(asn))}`,
    signalInit(signal),
  );
}

export async function listMaintainers(
  categories: string[] = [],
  signal?: AbortSignal,
): Promise<MaintainerIndexPayload> {
  const query = new URLSearchParams();
  if (categories.length > 0) query.set("categories", categories.join(","));
  const queryString = query.toString();
  return fetchJSON<MaintainerIndexPayload>(
    `/api/v1/maintainers${queryString ? `?${queryString}` : ""}`,
    signalInit(signal),
  );
}

export async function getMaintainerDetail(
  slug: string,
  signal?: AbortSignal,
): Promise<MaintainerDetailPayload> {
  return fetchJSON<MaintainerDetailPayload>(
    `/api/v1/maintainers/${encodeURIComponent(slug)}`,
    signalInit(signal),
  );
}
