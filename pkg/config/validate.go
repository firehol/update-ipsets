package config

import (
	"fmt"
	"net/url"
	"strings"
)

// validOutputs enumerates the authoritative Source.Output values.
// The engine supports exactly two kernel ipset types — the file
// extension and print format follow from this field:
//
//	ipset  → writes {name}.ipset  (hash:ip — individual IPs, one
//	         per line, CIDRs are expanded into their member IPs)
//	netset → writes {name}.netset (hash:net — CIDR blocks, one
//	         per line, individual IPs render as /32)
//
// Operators should use "netset" for feeds whose source publishes
// CIDR ranges and "ipset" for feeds that publish individual IPs.
// Mis-picking "ipset" for a CIDR-heavy feed silently loses most
// of the address space when iprange expands the CIDRs into
// individual host bits — that's exactly the bug that broke the
// MISP warninglists in April 2026.
//
// Legacy aliases accepted by canonicalizeOutput() (for YAML
// backward compatibility):
//
//	"ip"   → "ipset"
//	"net"  → "netset"
//	"both" → "netset"
//
// The old "both" value produced one .netset file with all
// prefixes (including /32). "netset" does the same thing — there
// was never a mode that wrote two files on disk.
var validOutputs = []string{"ipset", "netset"}

// canonicalizeOutput translates legacy `output:` values to the
// canonical two. Called by LoadYAML for every source (and during
// ExpandDerivatives for every new synthetic source) so internal
// code only ever sees "ipset" or "netset".
//
// Unknown values pass through unchanged and fail Validate() with
// a clear error, pointing the operator at the two accepted
// values.
func canonicalizeOutput(output string) string {
	switch output {
	case "ip", "ips":
		return "ipset"
	case "net", "nets", "both", "all":
		return "netset"
	default:
		return output
	}
}

// validUseRoles enumerates the engine roles a source may declare via
// `use:`. Empty `use:` is equivalent to "ipset" (the default plain
// download → parse → write `.ipset` path) and is intentionally NOT in
// this list — empty Use is the documented "I'm a regular ipset"
// signal.
var validUseRoles = map[string]struct{}{
	UseBogons:                 {},
	UseASN:                    {},
	UseGeoIP:                  {},
	UseCriticalInfrastructure: {},
	UseProviderContext:        {},
}

var validCriticalTiers = stringSet(CriticalTiers())

// CriticalTiers returns the configured presentation order for
// critical-infrastructure tiers.
func CriticalTiers() []string {
	return []string{"hard", "soft", "contextual"}
}

func stringSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

var validCriticalSourceQualities = map[string]struct{}{
	"A": {},
	"B": {},
	"C": {},
	"D": {},
}

var validCriticalSourceTypes = map[string]struct{}{
	"authoritative_provider_json":    {},
	"authoritative_provider_api":     {},
	"authoritative_plain_text":       {},
	"authoritative_service_tag_json": {},
	"authoritative_static_docs":      {},
	"authoritative_root_hints":       {},
	"authoritative_rfc":              {},
	"authoritative_geofeed_csv":      {},
	"curated_static":                 {},
	"secondary":                      {},
	"generated_bgp":                  {},
	"dns_derived":                    {},
	"analytical_only":                {},
}

var validCriticalRoles = map[string]struct{}{
	"public_dns_core":          {},
	"public_dns_extended":      {},
	"dns_root":                 {},
	"dns_sink_infrastructure":  {},
	"public_time":              {},
	"cdn_edge":                 {},
	"cdn_edge_shared":          {},
	"cloud_provider":           {},
	"cloud_customer_hosting":   {},
	"cloud_service_edge":       {},
	"cloud_service_tag":        {},
	"cloud_control_plane":      {},
	"cloud_service_google":     {},
	"cloud_proxy":              {},
	"developer_platform":       {},
	"dev_platform_saas":        {},
	"container_registry":       {},
	"payment_or_commerce":      {},
	"certificate_validation":   {},
	"software_update":          {},
	"identity_saas":            {},
	"saas_productivity":        {},
	"saas_productivity_devops": {},
	"saas_control_plane":       {},
	"saas_crm_platform":        {},
	"email_delivery":           {},
	"email_delivery_saas":      {},
	"observability_saas":       {},
	"synthetic_monitoring":     {},
	"local_control_plane":      {},
	"social_platform":          {},
}

var reservedCriticalInfrastructureProviderNames = map[string]struct{}{
	"infrastructure": {},
	"providers":      {},
}

var validArtifactTypes = map[string]struct{}{
	ArtifactTypeDroneBLBuildzone: {},
}

// sourceProducesIPSet reports whether the source's role mix means it
// produces an `.ipset` file (and therefore needs IPV/Output set). The
// rule is: bogons and critical_infrastructure roles still produce
// ipsets in addition to participating in attribution; asn and geoip
// roles do not. A source with no Use is a plain ipset.
func sourceProducesIPSet(s *Source) bool {
	if s == nil {
		return false
	}
	if len(s.Use) == 0 {
		return true
	}
	for _, role := range s.Use {
		switch role {
		case UseBogons, UseCriticalInfrastructure, UseProviderContext:
			return true
		}
	}
	return false
}

// validFeedName checks that name is safe for use as a filename component and
// internal merge URL token: no separators, commas, control characters,
// non-ASCII, or empty names.
func validFeedName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if r < 0x20 || r > 0x7E {
			return false
		}
		if strings.ContainsRune(`/\:*?"<>|,`, r) {
			return false
		}
	}
	return true
}

// HistoryLabel returns the suffix appended to an ipset name for a given
// history window (in minutes). For example, 1440 -> "_1d", 10080 -> "_7d".
// This must match the engine's historyLabel() exactly.
func HistoryLabel(minutes int) string {
	if minutes >= 24*60 {
		days := minutes / (24 * 60)
		rem := minutes - days*(24*60)
		if rem == 0 {
			return fmt.Sprintf("_%dd", days)
		}
		return fmt.Sprintf("_%dd%dh", days, rem/60)
	}
	return fmt.Sprintf("_%dh", minutes/60)
}

// GeneratedNames returns the set of all feed names that exist at runtime:
// source names, built-in synthetic sources, history-windowed variants, and
// legacy merge names prior to expansion.
func GeneratedNames(cfg *Config) map[string]struct{} {
	names := map[string]struct{}{}
	for name, src := range cfg.Sources {
		names[name] = struct{}{}
		if src == nil {
			continue
		}
		for _, mins := range src.History {
			names[name+HistoryLabel(mins)] = struct{}{}
		}
	}
	for _, src := range builtInSyntheticSourceDefs(cfg) {
		if src != nil {
			names[src.Name] = struct{}{}
		}
	}
	for name := range cfg.Merges {
		names[name] = struct{}{}
	}
	return names
}

// ValidateDeep performs full cross-referencing validation: everything
// Validate does, plus merge source reference checks.
func ValidateDeep(cfg *Config) error {
	if err := Validate(cfg); err != nil {
		return err
	}
	return validateMergeReferences(cfg)
}

func validateMergeReferences(cfg *Config) error {
	if cfg == nil {
		return nil
	}
	known := GeneratedNames(cfg)
	for name, merge := range cfg.Merges {
		if merge == nil {
			return fmt.Errorf("merge %q is nil", name)
		}
		sources := cleanMergeRefs(merge.Sources)
		exclude := cleanMergeRefs(merge.Exclude)
		if len(sources) == 0 {
			return fmt.Errorf("merge %q has no sources", name)
		}
		if err := validateSignedMergeRefs(name, sources, exclude); err != nil {
			return err
		}
		for _, ref := range sources {
			if _, ok := known[ref]; !ok {
				return fmt.Errorf("merge %q references unknown source %q", name, ref)
			}
		}
		for _, ref := range exclude {
			if _, ok := known[ref]; !ok {
				return fmt.Errorf("merge %q references unknown source %q", name, ref)
			}
		}
	}
	return nil
}

func Validate(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if err := validateRuntimeURLs(cfg.Runtime); err != nil {
		return err
	}
	if err := validateRuntimeResourceControls(cfg.Runtime); err != nil {
		return err
	}
	if err := validateCategoryRegistry(cfg); err != nil {
		return err
	}
	if err := validateFeedHealthThresholds(cfg.Runtime); err != nil {
		return err
	}
	if len(cfg.InfrastructureASNs) > 0 {
		return fmt.Errorf("infrastructure_asns is no longer supported; define critical infrastructure as sources or merges with use:[critical_infrastructure] and critical metadata")
	}
	if err := validateCriticalASNContext(cfg.CriticalASNContext); err != nil {
		return err
	}
	if err := validateDefaultProviders(cfg); err != nil {
		return err
	}
	if err := validateArtifacts(cfg); err != nil {
		return err
	}
	if err := validateSources(cfg); err != nil {
		return err
	}
	if err := validateMerges(cfg); err != nil {
		return err
	}
	return validateCriticalArtifactNamespace(cfg)
}

func validateUseRoles(kind, name string, roles []string) error {
	// Validate every Use entry against the known role set so a typo in YAML
	// surfaces here instead of silently routing the feed to the wrong path.
	for _, role := range roles {
		if _, ok := validUseRoles[role]; !ok {
			return fmt.Errorf("%s %q has unknown use role %q (valid: bogons, asn, geoip, critical_infrastructure, provider_context)", kind, name, role)
		}
	}
	return nil
}

func validateRuntimeURLs(runtime RuntimeConfig) error {
	for field, raw := range map[string]string{
		"runtime.web_url":            runtime.WebURL,
		"runtime.public_base_url":    runtime.PublicBaseURL,
		"runtime.local_copy_url":     runtime.LocalCopyURL,
		"runtime.github_changes_url": runtime.GitHubChangesURL,
		"runtime.github_setinfo":     runtime.GitHubSetInfo,
	} {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil {
			return fmt.Errorf("%s has invalid url %q: %w", field, raw, err)
		}
		switch strings.ToLower(u.Scheme) {
		case "http", "https":
		default:
			return fmt.Errorf("%s has invalid scheme %q (only http and https are permitted)", field, u.Scheme)
		}
		if u.Host == "" {
			return fmt.Errorf("%s has invalid url %q: host is required", field, raw)
		}
		if field == "runtime.public_base_url" && (u.RawQuery != "" || u.Fragment != "") {
			return fmt.Errorf("%s has invalid url %q: query and fragment are not allowed", field, raw)
		}
	}
	return nil
}

func validateRuntimeResourceControls(runtime RuntimeConfig) error {
	for field, value := range map[string]int{
		"runtime.web_artifact_cache_max_entries": runtime.WebArtifactCacheMaxEntries,
		"runtime.max_ingest_workers":             runtime.MaxIngestWorkers,
	} {
		if value < 0 {
			return fmt.Errorf("%s must be zero or positive, got %d", field, value)
		}
	}
	for field, value := range map[string]int64{
		"runtime.web_artifact_cache_max_bytes":      runtime.WebArtifactCacheMaxBytes,
		"runtime.web_artifact_cache_max_file_bytes": runtime.WebArtifactCacheMaxFileBytes,
	} {
		if value < 0 {
			return fmt.Errorf("%s must be zero or positive, got %d", field, value)
		}
	}
	return nil
}
