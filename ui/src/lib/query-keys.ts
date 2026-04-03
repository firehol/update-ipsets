export const queryKeys = {
  feeds: () => ["feeds"] as const,
  categories: () => ["categories"] as const,
  feed: (name: string) => ["feed", name] as const,
  asnProviders: (name: string) => ["feed", name, "asn", "providers"] as const,
  asnFeed: (name: string, provider: string) =>
    ["feed", name, "asn", provider] as const,
  geoProviders: (name: string) => ["feed", name, "geo", "providers"] as const,
  geoFeed: (name: string, provider: string) =>
    ["feed", name, "geo", provider] as const,
  bogonProviders: (name: string) =>
    ["feed", name, "bogons", "providers"] as const,
  bogonFeed: (name: string, provider?: string) =>
    ["feed", name, "bogons", provider ?? "none"] as const,
  criticalProviders: (name: string) =>
    ["feed", name, "critical", "providers"] as const,
  criticalAggregate: (name: string) =>
    ["feed", name, "critical", "aggregate"] as const,
  criticalProvider: (name: string, provider: string) =>
    ["feed", name, "critical", provider] as const,
  comparison: (name: string) => ["feed", name, "comparison"] as const,
  history: (name: string) => ["feed", name, "history"] as const,
  changesets: (name: string) => ["feed", name, "changesets"] as const,
  retention: (name: string) => ["feed", name, "retention"] as const,
  insights: (name: string) => ["feed", name, "insights"] as const,
  ipSearch: (scope: string, ip: string, details: boolean) =>
    ["search", scope, ip, details] as const,
  clientIP: () => ["client-ip"] as const,
  countries: () => ["countries"] as const,
  country: (code: string) => ["country", code.toUpperCase()] as const,
  asns: () => ["asns"] as const,
  asn: (asn: number | string) => ["asn", String(asn)] as const,
  maintainers: (categories: readonly string[]) =>
    ["maintainers", [...categories]] as const,
  maintainer: (slug: string) => ["maintainer", slug] as const,
  methodology: () => ["methodology"] as const,
  methodologyPage: (slug: string) => ["methodology", slug] as const,
  adminStatus: () => ["admin", "status"] as const,
  adminFeeds: () => ["admin", "feeds"] as const,
  adminArtifacts: () => ["admin", "artifacts"] as const,
  adminManifest: (name: string) => ["admin", "feed", name, "manifest"] as const,
  adminIntegrityRoot: () => ["admin", "integrity"] as const,
  adminIntegrity: (includeArchived: boolean) =>
    ["admin", "integrity", includeArchived] as const,
  adminEntityIntegrity: () => ["admin", "entity-integrity"] as const,
};
