import type {
  AdminStatus,
  AdminFeed,
  CategoryMeta,
  FeedHealthClass,
  FeedHealthSnapshot,
  FeedManifest,
  FeedMetadata,
  FeedSummary,
  IPSearchResult,
} from "@/lib/api-types";

export const sampleCategories: CategoryMeta[] = [
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

export function sampleHealth(
  className: FeedHealthClass = "healthy",
): FeedHealthSnapshot {
  return {
    class: className,
    threshold_basis: "category_cadence",
    threshold_mins: 120,
    observed_updates: 3,
  };
}

export function sampleFeedSummary(
  overrides: Partial<FeedSummary> = {},
): FeedSummary {
  return {
    name: "alpha_feed",
    category: "intrusion",
    provenance: "primary",
    maintainer: "Alpha Maintainer",
    redistributable: true,
    started_date: 1_700_000_000,
    source_date: 1_700_000_000,
    processed_date: 1_700_000_000,
    checked_date: 1_700_000_000,
    unique_ips: 1200,
    entries: 1200,
    frequency_minutes: 60,
    unique_share_pct: 25,
    health: sampleHealth(),
    ...overrides,
  };
}

export function sampleAdminFeed(
  overrides: Partial<AdminFeed> = {},
): AdminFeed {
  return {
    name: "alpha_feed",
    kind: "source",
    category: "intrusion",
    enabled: true,
    status: "ok",
    health: sampleHealth(),
    last_status: "updated",
    last_status_label: "Updated",
    last_run_reason: "scheduled",
    last_processing_ms: 84,
    last_check: 1_700_000_000,
    last_update: 1_700_000_000,
    processed_date: 1_700_000_000,
    started_date: 1_700_000_000,
    next_check: 1_700_003_600,
    entries: 1200,
    unique_ips: 1200,
    download_failures: 0,
    frequency_minutes: 60,
    maintainer: "Alpha Maintainer",
    info: "A test feed used by admin page behavior tests.",
    license: "Test License",
    redistributable: true,
    url: "https://example.invalid/alpha.txt",
    file: "alpha_feed.ipset",
    source: "alpha_feed.raw",
    output: "ipset",
    ipv: "ipv4",
    ...overrides,
  };
}

type AdminStatusOverrides = Omit<
  Partial<AdminStatus>,
  "system" | "engine" | "feeds" | "queues"
> & {
  system?: Partial<AdminStatus["system"]>;
  engine?: Partial<AdminStatus["engine"]>;
  feeds?: Partial<AdminStatus["feeds"]>;
  queues?: Partial<NonNullable<AdminStatus["queues"]>>;
};

export function sampleAdminStatus(
  overrides: AdminStatusOverrides = {},
): AdminStatus {
  const base: AdminStatus = {
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
      last_gc_unix: 1_700_000_000,
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
        updated: ["alpha_feed"],
        failed: [],
      },
      background_tasks: [],
      background_limit: 1,
      background_running: 0,
      max_ingest_workers: 1,
      parallel_downloads: 1,
      parallel_dns_queries: 1,
      max_processing_workers: 1,
      max_heavy_phase_workers: 1,
      max_background_workers: 1,
    },
    queues: {
      download_waiting: [],
      download_active: [],
      processing_waiting: [],
      processing_active: [],
    },
    feeds: {
      total_configured: 2,
      total_enabled: 2,
      total_entries: 2100,
      total_unique_ips: 2100,
      healthy: 2,
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

  return {
    ...base,
    ...overrides,
    system: { ...base.system, ...overrides.system },
    engine: { ...base.engine, ...overrides.engine },
    feeds: { ...base.feeds, ...overrides.feeds },
    queues: { ...base.queues, ...overrides.queues },
  };
}

export function sampleFeedManifest(
  overrides: Partial<FeedManifest> = {},
): FeedManifest {
  return {
    feed: "alpha_feed",
    processed_date: 1_700_000_000,
    files: [
      {
        rel: "alpha_feed.ipset",
        path: "/tmp/update-ipsets/web/alpha_feed.ipset",
        kind: "canonical",
        required: true,
        exists: true,
        size: 4096,
        mtime: 1_700_000_000,
      },
      {
        rel: "alpha_feed.json",
        path: "/tmp/update-ipsets/web/alpha_feed.json",
        kind: "metadata",
        required: true,
        exists: true,
        size: 2048,
        mtime: 1_700_000_000,
      },
    ],
    summary: {
      total: 2,
      present: 2,
      missing: 0,
      stale: 0,
      required: 2,
    },
    ...overrides,
  };
}

export function sampleFeedMetadata(
  overrides: Partial<FeedMetadata> = {},
): FeedMetadata {
  const enrichment = sampleFeedEnrichment();
  return {
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
    started: 1_700_000_000_000,
    updated: 1_700_000_000_000,
    processed: 1_700_000_000_000,
    checked: 1_700_000_000_000,
    clock_skew: 0,
    category: "intrusion",
    provenance: "primary",
    maintainer: "Alpha Maintainer",
    maintainer_url: "https://example.invalid/alpha",
    license: "Test License",
    attribution: "Test Attribution",
    official_name: enrichment.official_name ?? undefined,
    short_description: enrichment.short_description ?? undefined,
    current_status: enrichment.current_status,
    enrichment,
    info: "Test feed used by frontend behavior tests.",
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
    health: sampleHealth(),
    downloader: "http",
    redistributable: true,
    ...overrides,
  };
}

export function sampleFeedEnrichment() {
  return {
    enrichment_schema_version: 2 as const,
    run_at: "2026-05-26T00:00:00Z",
    official_name: "Known Test Feed",
    official_url: "https://example.invalid/known-feed",
    short_description: "Research-backed context for the test feed.",
    long_description:
      "This feed is used by frontend behavior tests to verify researched feed context rendering.",
    roles: [
      {
        role: "maintainer" as const,
        name: "Alpha Maintainer",
        official_url: "https://example.invalid/alpha",
      },
    ],
    derivation: {
      type: "original" as const,
      description: "The maintainer publishes this feed directly.",
      source_feeds: [],
    },
    listing_policy: {
      summary: "Entries are listed when the maintainer observes unwanted traffic.",
      criteria: ["Observed unwanted traffic"],
    },
    unlisting_policy: {
      summary: "Entries are removed when the maintainer no longer sees the signal.",
      criteria: ["Signal no longer observed"],
    },
    unlist_request: {
      url: "https://example.invalid/unlist",
      instructions: "Use the maintainer's request page.",
    },
    update_frequency: {
      frequency: "1h",
      human_readable: "The maintainer describes hourly updates.",
    },
    detection_classification: {
      primary_method: "unknown" as const,
      secondary_methods: [],
      description: "The maintainer does not publish detailed detection mechanics.",
    },
    scope_and_intent: {
      description: "Use this fixture for UI behavior tests.",
      intended_for: ["Frontend tests"],
      not_intended_for: ["Production decisions"],
    },
    license: "Test License",
    redistribution: {
      allowed: true,
      terms: "Fixture redistribution terms.",
    },
    current_status: {
      state: "active" as const,
      description: "Active fixture.",
    },
    community: {
      awards: "Used in tests.",
      criticism: "No real-world signal.",
      engagement: "Maintainer engagement is simulated.",
    },
    sources_consulted: [
      {
        url: "https://example.invalid/source",
        validation_date: "2026-05-26",
      },
    ],
  };
}

export const sampleSearchResult: IPSearchResult = {
  ip: "1.1.1.1",
  scope: "global",
  matches: [
    {
      name: "alpha_feed",
      category: "intrusion",
      provenance: "primary",
      info: "Research-backed context for the alpha fixture.",
      maintainer: "Alpha Maintainer",
      health: sampleHealth(),
    },
  ],
  context: {
    ip: "1.1.1.1",
    asn: 13335,
    asn_name: "Cloudflare, Inc.",
    asn_provider_label: "ip2asn.com",
  },
};

export const emptyWorldTopology = {
  type: "Topology",
  objects: {
    countries: {
      type: "GeometryCollection",
      geometries: [],
    },
  },
  arcs: [],
};
