import { http, HttpResponse, type HttpHandler } from "msw";
import type {
  AdminFeed,
  ASNProvider,
  BogonProvider,
  CriticalInfrastructureProvider,
  FeedSummary,
  GeoProvider,
} from "@/lib/api-types";
import {
  emptyWorldTopology,
  sampleAdminFeed,
  sampleAdminStatus,
  sampleCategories,
  sampleFeedManifest,
  sampleFeedMetadata,
  sampleFeedSummary,
  sampleHealth,
  sampleSearchResult,
} from "./fixtures";

export function homePageHandlers(
  feeds: FeedSummary[] = sampleExplorerFeeds(),
): HttpHandler[] {
  return [
    http.get("/api/v1/categories", () => HttpResponse.json(sampleCategories)),
    http.get("/api/v1/client-ip", () =>
      HttpResponse.json({ ip: "203.0.113.10" }),
    ),
    http.get("/world/countries-110m.json", () =>
      HttpResponse.json(emptyWorldTopology),
    ),
    http.get("/api/v1/sets", () => HttpResponse.json(feeds)),
    http.get("/api/v1/search", ({ request }) => {
      const url = new URL(request.url);
      const ip = url.searchParams.get("ip") ?? "";
      if (ip === sampleSearchResult.ip) {
        return HttpResponse.json(sampleSearchResult);
      }
      return HttpResponse.json({ ip, scope: "global", matches: [] });
    }),
  ];
}

export function adminPageHandlers(
  feeds: AdminFeed[] = sampleAdminFeeds(),
): HttpHandler[] {
  return [
    http.get("/api/v1/admin/status", () =>
      HttpResponse.json(
        sampleAdminStatus({
          feeds: {
            total_configured: feeds.length,
            total_enabled: feeds.filter((feed) => feed.enabled).length,
            total_entries: feeds.reduce((sum, feed) => sum + feed.entries, 0),
            total_unique_ips: feeds.reduce(
              (sum, feed) => sum + feed.unique_ips,
              0,
            ),
            healthy: feeds.filter((feed) => feed.health.class === "healthy")
              .length,
          },
        }),
      ),
    ),
    http.get("/api/v1/admin/feeds", () => HttpResponse.json(feeds)),
    http.get("/api/v1/admin/integrity", () =>
      HttpResponse.json({
        status: "clean",
        running: false,
        count: 0,
        findings: [],
      }),
    ),
    http.get("/api/v1/admin/integrity/entities", () =>
      HttpResponse.json({
        status: "clean",
        running: false,
        count: 0,
        findings: [],
      }),
    ),
    http.get("/api/v1/admin/feeds/:name/manifest", ({ params }) => {
      const name = String(params.name ?? "");
      return HttpResponse.json(
        sampleFeedManifest({
          feed: name,
          files: [
            {
              rel: `${name}.ipset`,
              path: `/tmp/update-ipsets/web/${name}.ipset`,
              kind: "canonical",
              required: true,
              exists: true,
              size: 4096,
              mtime: 1_700_000_000,
            },
          ],
          summary: {
            total: 1,
            present: 1,
            missing: 0,
            stale: 0,
            required: 1,
          },
        }),
      );
    }),
  ];
}

export function adminWriteActionHandlers(
  requests: string[] = [],
): HttpHandler[] {
  const record = (request: Request) => {
    const url = new URL(request.url);
    requests.push(`${request.method} ${url.pathname}${url.search}`);
  };

  return [
    http.post("/api/v1/admin/feeds/:name/recheck", ({ request, params }) => {
      record(request);
      return HttpResponse.json({
        status: "scheduled",
        name: String(params.name ?? ""),
      });
    }),
    http.post("/api/v1/admin/feeds/:name/reprocess", ({ request, params }) => {
      record(request);
      return HttpResponse.json({
        status: "scheduled",
        name: String(params.name ?? ""),
      });
    }),
    http.post("/api/v1/admin/feeds/:name/enable", ({ request, params }) => {
      record(request);
      return HttpResponse.json({
        status: "enabled",
        name: String(params.name ?? ""),
      });
    }),
    http.post("/api/v1/admin/feeds/:name/disable", ({ request, params }) => {
      record(request);
      return HttpResponse.json({
        status: "disabled",
        name: String(params.name ?? ""),
      });
    }),
    http.post("/api/v1/admin/integrity/reprocess", ({ request }) => {
      record(request);
      return HttpResponse.json({
        status: "scheduled",
        count: 1,
        names: ["beta_malware"],
      });
    }),
    http.post("/api/v1/admin/integrity/refresh", ({ request }) => {
      record(request);
      return HttpResponse.json({
        status: "scheduled",
      });
    }),
    http.post("/api/v1/admin/integrity/entities/refresh", ({ request }) => {
      record(request);
      return HttpResponse.json({
        status: "scheduled",
      });
    }),
    http.post("/api/v1/admin/integrity/entities/rebuild", ({ request }) => {
      record(request);
      return HttpResponse.json({
        status: "scheduled",
      });
    }),
  ];
}

export function integrityIssueHandlers(): HttpHandler[] {
  return [
    http.get("/api/v1/admin/integrity", () =>
      HttpResponse.json({
        status: "issues",
        running: false,
        count: 1,
        findings: [
          {
            feed: "beta_malware",
            source_path: "/tmp/update-ipsets/lib/beta_malware.ipset",
            source_mtime: "2023-11-14T22:13:20Z",
            source_file_mtime: "2023-11-14T22:13:20Z",
            processed_at: "2023-11-14T22:13:20Z",
            stale_files: ["/tmp/update-ipsets/web/beta_malware.json"],
            recovery_action: "reprocess",
            recovery_targets: ["beta_malware"],
            reason: "published metadata is older than the processed feed",
          },
        ],
      }),
    ),
  ];
}

export function entityPageHandlers(): HttpHandler[] {
  const feed = {
    name: "alpha_feed",
    category: "intrusion",
    provenance: "primary",
    maintainer: "Alpha Maintainer",
    attributed_ips: 1200,
    unique_ips: 1200,
    health_class: "healthy",
    last_change_ts: 1_700_000_000,
  };

  return [
    http.get("/api/v1/categories", () => HttpResponse.json(sampleCategories)),
    http.get("/world/countries-110m.json", () =>
      HttpResponse.json(emptyWorldTopology),
    ),
    http.get("/api/v1/countries/US", () =>
      HttpResponse.json({
        code: "US",
        provider: { name: "dbip", label: "DB-IP" },
        asn_provider: { name: "ip2asn", label: "ip2asn.com" },
        totals: {
          feeds_matching: 1,
          attributed_ips_in_feeds: 1200,
          categories: 1,
          maintainers: 1,
          asns: 1,
        },
        feeds: [feed],
        feeds_by_category: { intrusion: [feed] },
        top_categories: [
          { category: "intrusion", feed_count: 1, attributed_ips: 1200 },
        ],
        top_maintainers: [
          {
            slug: "alpha-maintainer",
            name: "Alpha Maintainer",
            feed_count: 1,
            attributed_ips: 1200,
          },
        ],
        top_asns_in_country: [
          {
            asn: 13335,
            name: "Cloudflare, Inc.",
            feed_count: 1,
            attributed_ips: 1200,
          },
        ],
      }),
    ),
    http.get("/api/v1/asns/13335", () =>
      HttpResponse.json({
        asn: 13335,
        name: "Cloudflare, Inc.",
        description: "Fixture ASN used by page behavior tests.",
        provider: { name: "ip2asn", label: "ip2asn.com" },
        geo_provider: { name: "dbip", label: "DB-IP" },
        totals: {
          feeds_matching: 1,
          attributed_ips: 1200,
          categories: 1,
          maintainers: 1,
          countries: 1,
        },
        feeds: [feed],
        feeds_by_category: { intrusion: [feed] },
        top_categories: [
          { category: "intrusion", feed_count: 1, attributed_ips: 1200 },
        ],
        top_maintainers: [
          {
            slug: "alpha-maintainer",
            name: "Alpha Maintainer",
            feed_count: 1,
            attributed_ips: 1200,
          },
        ],
        top_countries: [{ code: "US", feed_count: 1, attributed_ips: 1200 }],
        country_distribution: {
          total_mapped: 1200,
          countries: [{ code: "US", name: "United States", value: 1200 }],
        },
      }),
    ),
    http.get("/api/v1/maintainers/alpha-maintainer", () =>
      HttpResponse.json({
        slug: "alpha-maintainer",
        name: "Alpha Maintainer",
        url: "https://example.invalid/alpha",
        totals: { feeds: 1, unique_ips: 1200, categories: 1 },
        feeds_by_category: {
          intrusion: [
            {
              name: "alpha_feed",
              category: "intrusion",
              provenance: "primary",
              unique_ips: 1200,
              health_class: "healthy",
              last_change_ts: 1_700_000_000,
            },
          ],
        },
      }),
    ),
  ];
}

export function entityIndexHandlers(): HttpHandler[] {
  return [
    http.get("/api/v1/countries", () =>
      HttpResponse.json({
        provider: { name: "dbip", label: "DB-IP" },
        countries: [
          { code: "US", feed_count: 2, attributed_ips: 1200 },
        ],
      }),
    ),
    http.get("/api/v1/asns", () =>
      HttpResponse.json({
        provider: { name: "ip2asn", label: "ip2asn.com" },
        asns: [
          {
            asn: 13335,
            name: "Cloudflare, Inc.",
            feed_count: 2,
            attributed_ips: 1200,
          },
        ],
      }),
    ),
    http.get("/api/v1/maintainers", () =>
      HttpResponse.json({
        maintainers: [
          {
            slug: "alpha-maintainer",
            name: "Alpha Maintainer",
            url: "https://example.invalid/alpha",
            feed_count: 2,
            unique_ips: 1200,
            categories: ["intrusion", "malware_infrastructure"],
          },
        ],
      }),
    ),
  ];
}

export function feedDetailPageHandlers(name = "known_feed"): HttpHandler[] {
  const asnProvider: ASNProvider = {
    name: "ip2asn",
    label: "ip2asn.com",
    type: "asn",
    redistributable: true,
  };
  const geoProvider: GeoProvider = {
    name: "dbip",
    label: "DB-IP",
    type: "geoip",
    redistributable: true,
  };
  const bogonProvider: BogonProvider = {
    name: "rfc_reserved",
    label: "RFC reserved",
    type: "bogons",
    authoritative: true,
  };
  const criticalProvider = criticalReferenceProvider("critical_dns_core", {
    label: "Core public DNS",
    tier: "hard",
    role: "public_dns",
  });

  return [
    http.get("/api/v1/categories", () => HttpResponse.json(sampleCategories)),
    http.get("/world/countries-110m.json", () =>
      HttpResponse.json(emptyWorldTopology),
    ),
    http.get("/api/v1/sets", () =>
      HttpResponse.json([
        sampleFeedSummary({ name }),
        sampleFeedSummary({
          name: "beta_malware",
          category: "malware_infrastructure",
          maintainer: "Beta Maintainer",
          official_name: "Beta Malware Fixture",
          short_description: "Research-backed context for the beta fixture.",
          unique_ips: 900,
          entries: 900,
        }),
      ]),
    ),
    http.get("/api/v1/sets/:feedName", ({ params }) => {
      if (String(params.feedName ?? "") !== name) {
        return HttpResponse.json({ error: "not found" }, { status: 404 });
      }
      return HttpResponse.json(sampleFeedMetadata({ name }));
    }),
    http.get("/api/v1/sets/:feedName/insights", () =>
      HttpResponse.json({
        name,
        computed: 1_700_000_000,
        items: [
          {
            code: "stable_size",
            section: "overview",
            headline: "The feed has a stable observed size in the fixture.",
          },
        ],
      }),
    ),
    http.get("/api/v1/sets/:feedName/infrastructure/providers", () =>
      HttpResponse.json([criticalProvider]),
    ),
    http.get("/api/v1/sets/:feedName/infrastructure", () =>
      HttpResponse.json({
        feed: name,
        family: "ipv4",
        feed_ips: 1200,
        critical_ips: 2,
        percent: 0.17,
        complete: true,
        provider_set_id: "test-provider-set",
        configured_providers: [criticalProvider.name],
        tiers: [
          {
            tier: "hard",
            critical_ips: 2,
            percent: 0.17,
            providers: 1,
          },
        ],
        providers: [
          {
            provider: criticalProvider,
            provider_set_id: "test-provider-set",
            feed_ips: 1200,
            critical_ips: 2,
            percent: 0.17,
          },
        ],
      }),
    ),
    http.get("/api/v1/sets/:feedName/asn", () =>
      HttpResponse.json([asnProvider]),
    ),
    http.get("/api/v1/sets/:feedName/asn/:provider", () =>
      HttpResponse.json({
        provider: asnProvider.name,
        feed_ips: 1200,
        attributed_ips: 1190,
        bogon_ips: 0,
        unknown_ips: 10,
        by_asn: [
          {
            asn: 13335,
            name: "Cloudflare, Inc.",
            count: 1200,
            percent: 100,
          },
        ],
      }),
    ),
    http.get("/api/v1/sets/:feedName/countries", () =>
      HttpResponse.json([geoProvider]),
    ),
    http.get("/api/v1/sets/:feedName/countries/:provider", () =>
      HttpResponse.json({
        total_mapped: 1200,
        countries: [{ code: "US", name: "United States", value: 1200 }],
      }),
    ),
    http.get("/api/v1/sets/:feedName/bogons", () =>
      HttpResponse.json([bogonProvider]),
    ),
    http.get("/api/v1/sets/:feedName/bogons/:provider", () =>
      HttpResponse.json({
        provider: bogonProvider.name,
        feed_ips: 1200,
        bogon_ips: 0,
        percent: 0,
        by_range: [],
      }),
    ),
    http.get("/api/v1/sets/:feedName/compare", () =>
      HttpResponse.json([
        {
          name: "beta_malware",
          category: "malware_infrastructure",
          ips: 900,
          common: 150,
        },
      ]),
    ),
    http.get("/api/v1/sets/:feedName/history", () =>
      HttpResponse.text(
        "timestamp,entries,ips\n1699996400,1100,1100\n1700000000,1200,1200\n",
      ),
    ),
    http.get("/api/v1/sets/:feedName/changesets", () =>
      HttpResponse.json([
        { timestamp: 1_700_000_000, added: 120, removed: 20 },
      ]),
    ),
    http.get("/api/v1/sets/:feedName/retention", () =>
      HttpResponse.json({
        started: 1_699_996_400_000,
        updated: 1_700_000_000_000,
        incomplete: 0,
        current: {
          hours: [1, 6, 24],
          ips: [800, 300, 100],
          total: 1200,
        },
        past: {
          hours: [1, 6, 24],
          ips: [100, 80, 20],
          total: 200,
        },
      }),
    ),
  ];
}

function sampleExplorerFeeds(): FeedSummary[] {
  return [
    sampleFeedSummary({
      name: "alpha_feed",
      maintainer: "Alpha Maintainer",
      critical: {
        tier: "hard",
        role: "public_dns",
      },
    }),
    sampleFeedSummary({
      name: "beta_malware",
      category: "malware_infrastructure",
      maintainer: "Beta Maintainer",
      official_name: "Beta Malware Fixture",
      short_description: "Research-backed context for the beta fixture.",
      unique_ips: 900,
      entries: 900,
      critical_overlap_tiers: ["soft"],
    }),
  ];
}

function sampleAdminFeeds(): AdminFeed[] {
  return [
    sampleAdminFeed({ name: "alpha_feed", maintainer: "Alpha Maintainer" }),
    sampleAdminFeed({
      name: "beta_malware",
      category: "malware_infrastructure",
      maintainer: "Beta Maintainer",
      info: "Beta feed modal content used by the page-level admin test.",
      unique_ips: 900,
      entries: 900,
      file: "beta_malware.ipset",
      source: "beta_malware.raw",
      health: sampleHealth("healthy"),
    }),
  ];
}

function criticalReferenceProvider(
  name: string,
  overrides: Partial<CriticalInfrastructureProvider> = {},
): CriticalInfrastructureProvider {
  return {
    name,
    label: name,
    type: "source",
    tier: "hard",
    role: "public_dns",
    source_type: "curated_static",
    source_quality: "A",
    rationale: "Core resolver service address space should not be blocked accidentally.",
    redistributable: true,
    ...overrides,
  };
}
