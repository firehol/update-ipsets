import type {
  ASNFeedPayload,
  ASNProvider,
  BogonFeedPayload,
  BogonProvider,
  ChangesetPoint,
  ComparisonRow,
  CountryComparisonPayload,
  CriticalInfrastructureOverlap,
  CriticalInfrastructurePayload,
  CriticalInfrastructureProvider,
  GeoProvider,
  Insight,
  InsightsPayload,
  RetentionData,
} from "../api-types";
import { fetchJSON, fetchText, signalInit } from "./http";

export async function listASNProviders(
  name: string,
  signal?: AbortSignal,
): Promise<ASNProvider[]> {
  return fetchJSON<ASNProvider[]>(
    `/api/v1/sets/${encodeURIComponent(name)}/asn`,
    signalInit(signal),
  );
}

export async function getASNFeed(
  name: string,
  provider: string,
  signal?: AbortSignal,
): Promise<ASNFeedPayload> {
  return fetchJSON<ASNFeedPayload>(
    `/api/v1/sets/${encodeURIComponent(name)}/asn/${encodeURIComponent(provider)}`,
    signalInit(signal),
  );
}

export async function listGeoProviders(
  name: string,
  signal?: AbortSignal,
): Promise<GeoProvider[]> {
  return fetchJSON<GeoProvider[]>(
    `/api/v1/sets/${encodeURIComponent(name)}/countries`,
    signalInit(signal),
  );
}

export async function getGeoFeed(
  name: string,
  provider: string,
  signal?: AbortSignal,
): Promise<CountryComparisonPayload> {
  return fetchJSON<CountryComparisonPayload>(
    `/api/v1/sets/${encodeURIComponent(name)}/countries/${encodeURIComponent(provider)}`,
    signalInit(signal),
  );
}

export async function listBogonProviders(
  name: string,
  signal?: AbortSignal,
): Promise<BogonProvider[]> {
  return fetchJSON<BogonProvider[]>(
    `/api/v1/sets/${encodeURIComponent(name)}/bogons`,
    signalInit(signal),
  );
}

export async function getBogonFeed(
  name: string,
  provider: string,
  signal?: AbortSignal,
): Promise<BogonFeedPayload> {
  return fetchJSON<BogonFeedPayload>(
    `/api/v1/sets/${encodeURIComponent(name)}/bogons/${encodeURIComponent(provider)}`,
    signalInit(signal),
  );
}

export async function listCriticalInfrastructureProviders(
  name: string,
  signal?: AbortSignal,
): Promise<CriticalInfrastructureProvider[]> {
  return fetchJSON<CriticalInfrastructureProvider[]>(
    `/api/v1/sets/${encodeURIComponent(name)}/infrastructure/providers`,
    signalInit(signal),
  );
}

export async function getCriticalInfrastructure(
  name: string,
  signal?: AbortSignal,
): Promise<CriticalInfrastructurePayload> {
  return fetchJSON<CriticalInfrastructurePayload>(
    `/api/v1/sets/${encodeURIComponent(name)}/infrastructure`,
    signalInit(signal),
  );
}

export async function getCriticalInfrastructureProvider(
  name: string,
  provider: string,
  signal?: AbortSignal,
): Promise<CriticalInfrastructureOverlap> {
  return fetchJSON<CriticalInfrastructureOverlap>(
    `/api/v1/sets/${encodeURIComponent(name)}/infrastructure/${encodeURIComponent(provider)}`,
    signalInit(signal),
  );
}

export async function getComparison(
  name: string,
  signal?: AbortSignal,
): Promise<ComparisonRow[]> {
  return fetchJSON<ComparisonRow[]>(
    `/api/v1/sets/${encodeURIComponent(name)}/compare`,
    signalInit(signal),
  );
}

export async function getHistoryCSV(
  name: string,
  signal?: AbortSignal,
): Promise<string> {
  return fetchText(
    `/api/v1/sets/${encodeURIComponent(name)}/history`,
    signalInit(signal),
  );
}

export async function getChangesets(
  name: string,
  signal?: AbortSignal,
): Promise<ChangesetPoint[]> {
  return fetchJSON<ChangesetPoint[]>(
    `/api/v1/sets/${encodeURIComponent(name)}/changesets`,
    signalInit(signal),
  );
}

export async function getRetention(
  name: string,
  signal?: AbortSignal,
): Promise<RetentionData> {
  return fetchJSON<RetentionData>(
    `/api/v1/sets/${encodeURIComponent(name)}/retention`,
    signalInit(signal),
  );
}

export async function getInsights(
  name: string,
  signal?: AbortSignal,
): Promise<Insight[]> {
  const payload = await fetchJSON<InsightsPayload>(
    `/api/v1/sets/${encodeURIComponent(name)}/insights`,
    signalInit(signal),
  );
  return payload.items ?? [];
}
