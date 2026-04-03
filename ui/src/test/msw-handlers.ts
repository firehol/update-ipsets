import { http, HttpResponse } from "msw";
import {
  emptyWorldTopology,
  sampleCategories,
  sampleFeedMetadata,
  sampleFeedSummary,
  sampleSearchResult,
} from "./fixtures";

export const handlers = [
  http.get("/api/v1/categories", () => HttpResponse.json(sampleCategories)),
  http.get("/api/v1/client-ip", () => HttpResponse.json({ ip: "203.0.113.10" })),
  http.get("/world/countries-110m.json", () =>
    HttpResponse.json(emptyWorldTopology),
  ),
  http.get("/api/v1/sets", () =>
    HttpResponse.json([
      sampleFeedSummary(),
      sampleFeedSummary({
        name: "beta_malware",
        category: "malware_infrastructure",
        maintainer: "Beta Maintainer",
        unique_ips: 900,
        entries: 900,
      }),
    ]),
  ),
  http.get("/api/v1/search", ({ request }) => {
    const url = new URL(request.url);
    const ip = url.searchParams.get("ip") ?? "";
    if (ip === sampleSearchResult.ip) {
      return HttpResponse.json(sampleSearchResult);
    }
    return HttpResponse.json({ ip, scope: "global", matches: [] });
  }),
  http.get("/api/v1/sets/:name", ({ params }) => {
    const name = String(params.name ?? "");
    if (name === "known_feed") {
      return HttpResponse.json(sampleFeedMetadata({ name }));
    }
    return HttpResponse.json({ error: "not found" }, { status: 404 });
  }),
];
