export type EnrichmentRoleName =
  | "maintainer"
  | "publisher"
  | "aggregator"
  | "source_contributor"
  | "original_author"
  | "successor";

export type EnrichmentOrganizationType =
  | "non_profit"
  | "commercial"
  | "research_institution"
  | "government"
  | "individual"
  | "informal_collective"
  | "unknown";

export type EnrichmentDerivationType =
  | "original"
  | "derivative"
  | "extraction"
  | "partial_mirror"
  | "aggregate_merge"
  | "reformat"
  | "fork"
  | "unknown";

export type EnrichmentSourceRelationship =
  | "subset"
  | "superset"
  | "filtered"
  | "enriched"
  | "mirror"
  | "aggregate_component"
  | "fork";

export type EnrichmentDetectionMethod =
  | "honeypot"
  | "network_telescope"
  | "active_scanning"
  | "user_submission"
  | "malware_analysis"
  | "reputation_aggregation"
  | "policy_assignment"
  | "commercial_threat_intel"
  | "mixed"
  | "unknown";

export type EnrichmentCurrentStatusState =
  | "active"
  | "discontinued"
  | "merged"
  | "forked"
  | "reformatted"
  | "altered_scope"
  | "unknown";

export interface FeedEnrichmentRole {
  role: EnrichmentRoleName;
  name: string;
  organization_type?: EnrichmentOrganizationType | null;
  official_url?: string | null;
  contact_email?: string | null;
  based_in?: string | null;
  active_since?: string | null;
  notes?: string | null;
}

export interface FeedEnrichmentSourceFeed {
  identifier: string;
  relationship?: EnrichmentSourceRelationship | null;
  notes?: string | null;
}

export interface FeedEnrichmentDerivation {
  type: EnrichmentDerivationType;
  description: string;
  source_feeds?: FeedEnrichmentSourceFeed[];
}

export interface FeedEnrichmentPolicy {
  summary?: string;
  criteria?: string[];
}

export interface FeedEnrichmentUnlistRequest {
  url?: string | null;
  email?: string | null;
  instructions?: string | null;
}

export interface FeedEnrichmentUpdateFrequency {
  frequency?: string | null;
  human_readable?: string | null;
}

export interface FeedEnrichmentDetectionClassification {
  primary_method: EnrichmentDetectionMethod;
  secondary_methods?: Exclude<
    EnrichmentDetectionMethod,
    "mixed" | "unknown"
  >[];
  description: string;
}

export interface FeedEnrichmentScopeAndIntent {
  description?: string | null;
  intended_for?: string[];
  not_intended_for?: string[];
}

export interface FeedEnrichmentRedistribution {
  allowed?: boolean | null;
  commercial_use_allowed?: boolean | null;
  attribution_required?: string | null;
  terms?: string | null;
}

export interface FeedEnrichmentCurrentStatus {
  state: EnrichmentCurrentStatusState;
  description: string;
  successor?: {
    name?: string | null;
    url?: string | null;
  } | null;
  announcement_date?: string | null;
}

export interface FeedEnrichmentCommunity {
  awards?: string | null;
  criticism?: string | null;
  engagement?: string | null;
}

export interface FeedEnrichmentSourceConsulted {
  url: string;
  document_date?: string | null;
  validation_date?: string | null;
}

export interface FeedEnrichment {
  enrichment_schema_version: 2;
  run_at: string;
  official_name?: string | null;
  official_url?: string | null;
  short_description?: string | null;
  long_description?: string | null;
  roles: FeedEnrichmentRole[];
  derivation: FeedEnrichmentDerivation;
  listing_policy?: FeedEnrichmentPolicy | null;
  unlisting_policy?: FeedEnrichmentPolicy | null;
  unlist_request?: FeedEnrichmentUnlistRequest | null;
  update_frequency?: FeedEnrichmentUpdateFrequency | null;
  detection_classification: FeedEnrichmentDetectionClassification;
  scope_and_intent?: FeedEnrichmentScopeAndIntent | null;
  license?: string | null;
  redistribution: FeedEnrichmentRedistribution;
  current_status: FeedEnrichmentCurrentStatus;
  community: FeedEnrichmentCommunity;
  sources_consulted: FeedEnrichmentSourceConsulted[];
}
