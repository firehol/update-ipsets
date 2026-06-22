import type { Page, Route } from "@playwright/test";

const now = 1_700_000_000;

const categories = [
  {
    name: "intrusion",
    label: "Intrusion",
    description: "Active attack and scanner sources.",
    sort_order: 10,
  },
  {
    name: "malware_infrastructure",
    label: "Malware Infrastructure",
    description: "Infrastructure associated with malware operations.",
    sort_order: 20,
  },
];

const health = {
  class: "healthy",
  threshold_basis: "category_cadence",
  threshold_mins: 120,
  observed_updates: 3,
};

const alphaFeedSummary = {
  name: "alpha_feed",
  category: "intrusion",
  provenance: "primary",
  maintainer: "Alpha Maintainer",
  redistributable: true,
  started_date: now,
  source_date: now,
  processed_date: now,
  checked_date: now,
  unique_ips: 1200,
  entries: 1200,
  frequency_minutes: 60,
  unique_share_pct: 25,
  health,
  critical: {
    tier: "hard",
    role: "public_dns",
  },
};

const betaFeedSummary = {
  ...alphaFeedSummary,
  name: "beta_malware",
  category: "malware_infrastructure",
  maintainer: "Beta Maintainer",
  unique_ips: 900,
  entries: 900,
  critical: undefined,
  critical_overlap_tiers: ["soft"],
};

const knownFeedMetadata = {
  name: "known_feed",
  entries: 1200,
  entries_min: 1200,
  entries_max: 1200,
  ips: 1200,
  ips_min: 1200,
  ips_max: 1200,
  ipv: "ipv4",
  hash: "abc123",
  frequency: 60,
  aggregation: 0,
  started: now * 1000,
  updated: now * 1000,
  processed: now * 1000,
  checked: now * 1000,
  clock_skew: 0,
  category: "intrusion",
  provenance: "primary",
  maintainer: "Alpha Maintainer",
  maintainer_url: "https://example.invalid/alpha",
  license: "Test License",
  attribution: "Test Attribution",
  info: "Test feed used by frontend browser smoke tests.",
  source: "https://example.invalid/feed.txt",
  file: "known_feed.ipset",
  history: "known_feed_history.csv",
  comparison: "known_feed_comparison.json",
  file_local: "known_feed.ipset",
  commit_history: "known_feed_changesets.json",
  errors: 0,
  version: 1,
  average_update: 60,
  min_update: 60,
  max_update: 60,
  health,
  downloader: "http",
  redistributable: true,
};

const adminFeed = {
  name: "beta_malware",
  kind: "source",
  category: "malware_infrastructure",
  enabled: true,
  status: "ok",
  health,
  last_status: "updated",
  last_status_label: "Updated",
  last_run_reason: "scheduled",
  last_processing_ms: 84,
  last_check: now,
  last_update: now,
  processed_date: now,
  started_date: now,
  next_check: now + 3600,
  entries: 900,
  unique_ips: 900,
  download_failures: 0,
  frequency_minutes: 60,
  maintainer: "Beta Maintainer",
  info: "Beta feed modal content used by browser smoke tests.",
  license: "Test License",
  redistributable: true,
  url: "https://example.invalid/beta.txt",
  file: "beta_malware.ipset",
  source: "beta_malware.raw",
  output: "ipset",
  ipv: "ipv4",
};

const worldTopology = {
  type: "Topology",
  objects: {
    countries: {
      type: "GeometryCollection",
      geometries: [
        {
          type: "Polygon",
          id: 840,
          properties: { name: "United States" },
          arcs: [[0]],
        },
      ],
    },
  },
  arcs: [
    [
      [-125, 25],
      [60, 0],
      [0, 25],
      [-60, 0],
      [0, -25],
    ],
  ],
};

const providerFeed = {
  name: "alpha_feed",
  category: "intrusion",
  provenance: "primary",
  maintainer: "Alpha Maintainer",
  attributed_ips: 1200,
  unique_ips: 1200,
  health_class: "healthy",
  last_change_ts: now,
};

export async function installApiFixtures(page: Page) {
  await page.route("**/*", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname;

    if (request.method() !== "GET") {
      await route.continue();
      return;
    }

    if (path === "/api/v1/categories") {
      await json(route, categories);
      return;
    }
    if (path === "/api/v1/client-ip") {
      await json(route, { ip: "203.0.113.10" });
      return;
    }
    if (path === "/world/countries-110m.json") {
      await json(route, worldTopology);
      return;
    }
    if (path === "/api/v1/sets") {
      await json(route, [alphaFeedSummary, betaFeedSummary]);
      return;
    }
    if (path === "/api/v1/search") {
      await json(route, searchResult(url.searchParams.get("ip") ?? ""));
      return;
    }
    if (path === "/api/v1/sets/known_feed") {
      await json(route, knownFeedMetadata);
      return;
    }
    if (path === "/api/v1/sets/about/known_feed") {
      await text(
        route,
        "<p>This list documents a known test feed for browser smoke coverage.</p>",
      );
      return;
    }
    if (path === "/api/v1/sets/known_feed/insights") {
      await json(route, {
        name: "known_feed",
        computed: now,
        items: [
          {
            code: "stable_size",
            section: "overview",
            headline: "The feed has a stable observed size in the fixture.",
          },
        ],
      });
      return;
    }
    if (path === "/api/v1/sets/known_feed/infrastructure/providers") {
      await json(route, [criticalProvider]);
      return;
    }
    if (path === "/api/v1/sets/known_feed/infrastructure") {
      await json(route, {
        feed: "known_feed",
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
      });
      return;
    }
    if (path === "/api/v1/sets/known_feed/asn") {
      await json(route, [asnProvider]);
      return;
    }
    if (path === "/api/v1/sets/known_feed/asn/ip2asn") {
      await json(route, {
        provider: "ip2asn",
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
      });
      return;
    }
    if (path === "/api/v1/sets/known_feed/countries") {
      await json(route, [geoProvider]);
      return;
    }
    if (path === "/api/v1/sets/known_feed/countries/dbip") {
      await json(route, {
        total_mapped: 1200,
        countries: [{ code: "US", name: "United States", value: 1200 }],
      });
      return;
    }
    if (path === "/api/v1/sets/known_feed/bogons") {
      await json(route, [bogonProvider]);
      return;
    }
    if (path === "/api/v1/sets/known_feed/bogons/rfc_reserved") {
      await json(route, {
        provider: "rfc_reserved",
        feed_ips: 1200,
        bogon_ips: 0,
        percent: 0,
        by_range: [],
      });
      return;
    }
    if (path === "/api/v1/sets/known_feed/compare") {
      await json(route, [
        {
          name: "beta_malware",
          category: "malware_infrastructure",
          ips: 900,
          common: 150,
        },
      ]);
      return;
    }
    if (path === "/api/v1/sets/known_feed/history") {
      await text(
        route,
        "timestamp,entries,ips\n1699989200,980,980\n1699992800,1050,1050\n1699996400,1100,1100\n1700000000,1200,1200\n",
      );
      return;
    }
    if (path === "/api/v1/sets/known_feed/changesets") {
      await json(route, [
        { timestamp: now - 7200, added: 90, removed: 20 },
        { timestamp: now - 3600, added: 70, removed: 20 },
        { timestamp: now, added: 120, removed: 20 },
      ]);
      return;
    }
    if (path === "/api/v1/sets/known_feed/retention") {
      await json(route, {
        started: (now - 3600) * 1000,
        updated: now * 1000,
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
      });
      return;
    }
    if (path === "/api/v1/admin/status") {
      await json(route, adminStatus);
      return;
    }
    if (path === "/api/v1/admin/feeds") {
      await json(route, [adminFeed]);
      return;
    }
    if (path === "/api/v1/admin/integrity") {
      await json(route, {
        status: "clean",
        running: false,
        count: 0,
        findings: [],
      });
      return;
    }
    if (path === "/api/v1/admin/integrity/entities") {
      await json(route, {
        status: "clean",
        running: false,
        count: 0,
        findings: [],
      });
      return;
    }
    if (path === "/api/v1/admin/feeds/beta_malware/manifest") {
      await json(route, {
        feed: "beta_malware",
        processed_date: now,
        files: [
          {
            rel: "beta_malware.ipset",
            path: "/tmp/update-ipsets/web/beta_malware.ipset",
            kind: "canonical",
            required: true,
            exists: true,
            size: 4096,
            mtime: now,
          },
        ],
        summary: {
          total: 1,
          present: 1,
          missing: 0,
          stale: 0,
          required: 1,
        },
      });
      return;
    }
    if (path === "/api/v1/countries/US") {
      await json(route, countryDetail);
      return;
    }

    if (path.startsWith("/api/") || path.startsWith("/world/")) {
      const message = `unhandled browser-test fixture route: ${path}`;
      await json(
        route,
        { error: message },
        500,
      );
      throw new Error(message);
    }

    await route.continue();
  });
}

function searchResult(ip: string) {
  if (ip !== "1.1.1.1") {
    return { ip, scope: "global", matches: [] };
  }
  return {
    ip,
    scope: "global",
    matches: [
      {
        name: "alpha_feed",
        category: "intrusion",
        provenance: "primary",
        maintainer: "Alpha Maintainer",
        health,
      },
    ],
    context: {
      ip,
      country_code: "US",
      geo_provider_label: "DB-IP",
      asn: 13335,
      asn_name: "Cloudflare, Inc.",
      asn_provider_label: "ip2asn.com",
    },
  };
}

const asnProvider = {
  name: "ip2asn",
  label: "ip2asn.com",
  type: "asn",
  redistributable: true,
};

const geoProvider = {
  name: "dbip",
  label: "DB-IP",
  type: "geoip",
  redistributable: true,
};

const bogonProvider = {
  name: "rfc_reserved",
  label: "RFC reserved",
  type: "bogons",
  authoritative: true,
};

const criticalProvider = {
  name: "critical_dns_core",
  label: "Core public DNS",
  type: "source",
  tier: "hard",
  role: "public_dns",
  source_type: "curated_static",
  source_quality: "A",
  rationale: "Core resolver service address space should not be blocked accidentally.",
  redistributable: true,
};

const adminStatus = {
  public_base_url: "https://iplists.firehol.org",
  system: {
    uptime: "1h",
    go_version: "go1.26.0",
    goos: "linux",
    goarch: "amd64",
    goroutines: 12,
    heap_alloc: 32 * 1024 * 1024,
    heap_sys: 64 * 1024 * 1024,
    heap_inuse: 40 * 1024 * 1024,
    stack_inuse: 512 * 1024,
    sys: 96 * 1024 * 1024,
    num_gc: 3,
    last_gc_unix: now,
    gc_pause_total_ns: 1_000_000,
    disk_free: "10 GiB",
  },
  engine: {
    running: false,
    last_started: "2023-11-14T22:13:20Z",
    last_ended: "2023-11-14T22:14:20Z",
    last_report: {
      started_at: "2023-11-14T22:13:20Z",
      ended_at: "2023-11-14T22:14:20Z",
      skipped: [],
      updated: ["beta_malware"],
      failed: [],
    },
    background_tasks: [],
    background_limit: 1,
    background_running: 0,
    engine_lane: {
      limit: 1,
      active_count: 0,
      waiting_count: 0,
      active: [],
      waiting: [],
    },
    pipeline_integrity_cache: {
      generation: 0,
      cache_state: "cold",
      count: 0,
    },
    entity_integrity_cache: {
      generation: 0,
      cache_state: "cold",
      count: 0,
    },
    max_ingest_workers: 1,
    parallel_downloads: 1,
    parallel_dns_queries: 1,
    max_processing_workers: 1,
    max_heavy_phase_workers: 1,
    max_background_workers: 1,
    max_engine_lane_workers: 1,
  },
  queues: {
    download_waiting: [],
    download_active: [],
    processing_waiting: [],
    processing_active: [],
  },
  feeds: {
    total_configured: 1,
    total_enabled: 1,
    total_entries: 900,
    total_unique_ips: 900,
    healthy: 1,
    delayed: 0,
    risky: 0,
    unavailable: 0,
    archived: 0,
    empty: 0,
    unmaintained: 0,
    stale: 0,
    errors: 0,
    running: 0,
    never_run: 0,
    disabled: 0,
    hidden: 0,
  },
  artifacts: [],
};

const countryDetail = {
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
  feeds: [providerFeed],
  feeds_by_category: { intrusion: [providerFeed] },
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
};

async function json(route: Route, body: unknown, status = 200) {
  await route.fulfill({
    status,
    contentType: "application/json",
    body: JSON.stringify(body),
  });
}

async function text(route: Route, body: string, status = 200) {
  await route.fulfill({
    status,
    contentType: "text/plain; charset=utf-8",
    body,
  });
}
