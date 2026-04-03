package config

import (
	"fmt"
	"net/netip"
	"net/url"
	"slices"
	"strings"

	"github.com/firehol/update-ipsets/pkg/enrichment"
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
	for name, artifact := range cfg.Artifacts {
		if artifact == nil {
			return fmt.Errorf("artifact %q is nil", name)
		}
		if !validFeedName(name) {
			return fmt.Errorf("artifact name %q contains invalid characters (path separators, commas, control chars, or non-ASCII)", name)
		}
		if _, ok := cfg.Sources[name]; ok {
			return fmt.Errorf("artifact %q collides with source %q", name, name)
		}
		if artifact.Frequency < 0 {
			return fmt.Errorf("artifact %q has invalid frequency %d", name, artifact.Frequency)
		}
		if artifact.MaxDownloadSize < -1 {
			return fmt.Errorf("artifact %q has invalid max_download_size %d", name, artifact.MaxDownloadSize)
		}
		if _, ok := validArtifactTypes[artifact.Type]; !ok {
			return fmt.Errorf("artifact %q has unknown type %q", name, artifact.Type)
		}
	}
	for name, src := range cfg.Sources {
		if src == nil {
			return fmt.Errorf("source %q is nil", name)
		}
		if !validFeedName(name) {
			return fmt.Errorf("source name %q contains invalid characters (path separators, commas, control chars, or non-ASCII)", name)
		}
		if src.Frequency < 0 {
			return fmt.Errorf("source %q has invalid frequency %d", name, src.Frequency)
		}
		if src.Provenance != "" && !src.Provenance.Valid() {
			return fmt.Errorf("source %q has invalid provenance %q", name, src.Provenance)
		}
		if err := validateUseRoles("source", name, src.Use); err != nil {
			return err
		}
		if err := enrichment.ValidateNamed("source", name, src.Enrichment); err != nil {
			return err
		}
		if err := validateCriticalMetadata("source", name, src.HasUse(UseCriticalInfrastructure), src.Critical); err != nil {
			return err
		}
		if src.HasUse(UseCriticalInfrastructure) && src.HasUse(UseBogons) {
			return fmt.Errorf("source %q combines use:[critical_infrastructure] with use:[bogons]; critical infrastructure references and bogon references have different public artifact semantics", name)
		}
		if src.HasUse(UseCriticalInfrastructure) && src.HasUse(UseProviderContext) {
			return fmt.Errorf("source %q combines use:[critical_infrastructure] with use:[provider_context]; critical warnings and broad provider context have different public artifact semantics", name)
		}
		if src.HasUse(UseCriticalInfrastructure) && (src.HasUse(UseASN) || src.HasUse(UseGeoIP)) {
			return fmt.Errorf("source %q combines use:[critical_infrastructure] with a provider database role; critical infrastructure references must produce ipset/netset artifacts", name)
		}
		if src.HasUse(UseCriticalInfrastructure) && src.IPV != "" && src.IPV != "ipv4" {
			return fmt.Errorf("source %q has use:[critical_infrastructure] with ipv %q; critical infrastructure overlap is IPv4-only in this release", name, src.IPV)
		}
		if src.HasUse(UseCriticalInfrastructure) && len(src.History) > 0 {
			return fmt.Errorf("source %q has use:[critical_infrastructure] with history windows; critical infrastructure reference feeds must not generate retention variants", name)
		}
		producesIPSet := sourceProducesIPSet(src)
		if err := validateStaticSourceBody(name, src, producesIPSet, src.HasUse(UseCriticalInfrastructure)); err != nil {
			return err
		}
		if producesIPSet {
			if src.IPV != "ipv4" && src.IPV != "ipv6" {
				return fmt.Errorf("source %q has invalid ip version %q", name, src.IPV)
			}
			if !slices.Contains(validOutputs, src.Output) {
				return fmt.Errorf("source %q has invalid output %q", name, src.Output)
			}
		}
		// Format is required for asn and geoip roles since the engine
		// dispatches the parser by format. ipset sources may omit it
		// (the default text parser is implied by the processor chain).
		if src.HasUse(UseASN) || src.HasUse(UseGeoIP) {
			if src.Format == "" {
				return fmt.Errorf("source %q has use:[%s] but no format set", name, strings.Join(src.Use, ","))
			}
		}
		if src.URL != "" {
			parsed, err := url.Parse(src.URL)
			if err != nil {
				return fmt.Errorf("source %q has invalid url %q: %w", name, src.URL, err)
			}
			scheme := strings.ToLower(parsed.Scheme)
			// Reject non-HTTP(S) schemes to prevent SSRF via file://, gopher://, etc.
			// Template URLs (containing {}) are checked at expansion time.
			// The internal:// scheme is reserved for synthetic sources
			// whose data is built into the binary.
			if scheme != "" && scheme != "http" && scheme != "https" && scheme != "internal" && scheme != "file" && scheme != ArtifactScheme {
				return fmt.Errorf("source %q has disallowed url scheme %q (only http, https, file, internal, and artifact are permitted)", name, parsed.Scheme)
			}
			if scheme == "file" {
				if parsed.Host != "" {
					return fmt.Errorf("source %q has invalid file url %q: host component is not allowed", name, src.URL)
				}
				if !strings.HasPrefix(parsed.Path, "/") {
					return fmt.Errorf("source %q has invalid file url %q: absolute path required", name, src.URL)
				}
			}
			if scheme == ArtifactScheme {
				ref, err := ParseArtifactURL(src.URL)
				if err != nil {
					return fmt.Errorf("source %q: %w", name, err)
				}
				if cfg.ArtifactByName(ref.Artifact) == nil {
					return fmt.Errorf("source %q references unknown artifact %q", name, ref.Artifact)
				}
				if src.Frequency != 0 {
					return fmt.Errorf("source %q is artifact-backed and must have frequency 0", name)
				}
			}
		}
	}
	// LoadYAML/LoadDirectory validate merge references before expansion, then
	// ExpandDerivatives clears cfg.Merges. This loop remains for programmatic
	// callers and tests that call Validate directly on an unexpanded Config.
	for name, merge := range cfg.Merges {
		if merge == nil {
			return fmt.Errorf("merge %q is nil", name)
		}
		if !validFeedName(name) {
			return fmt.Errorf("merge name %q contains invalid characters (path separators, commas, control chars, or non-ASCII)", name)
		}
		if merge.IPV != "ipv4" && merge.IPV != "ipv6" {
			return fmt.Errorf("merge %q has invalid ip version %q", name, merge.IPV)
		}
		if merge.Output == "" {
			return fmt.Errorf("merge %q has empty output", name)
		}
		if !slices.Contains(validOutputs, merge.Output) {
			return fmt.Errorf("merge %q has invalid output %q", name, merge.Output)
		}
		if merge.Frequency < 0 {
			return fmt.Errorf("merge %q has invalid frequency %d", name, merge.Frequency)
		}
		sources := cleanMergeRefs(merge.Sources)
		exclude := cleanMergeRefs(merge.Exclude)
		if len(sources) == 0 {
			return fmt.Errorf("merge %q has no sources", name)
		}
		if err := validateSignedMergeRefs(name, sources, exclude); err != nil {
			return err
		}
		if err := validateUseRoles("merge", name, merge.Use); err != nil {
			return err
		}
		if err := enrichment.ValidateNamed("merge", name, merge.Enrichment); err != nil {
			return err
		}
		if err := validateCriticalMetadata("merge", name, slices.Contains(merge.Use, UseCriticalInfrastructure), merge.Critical); err != nil {
			return err
		}
		if slices.Contains(merge.Use, UseCriticalInfrastructure) && slices.Contains(merge.Use, UseBogons) {
			return fmt.Errorf("merge %q combines use:[critical_infrastructure] with use:[bogons]; critical infrastructure references and bogon references have different public artifact semantics", name)
		}
		if slices.Contains(merge.Use, UseCriticalInfrastructure) && slices.Contains(merge.Use, UseProviderContext) {
			return fmt.Errorf("merge %q combines use:[critical_infrastructure] with use:[provider_context]; critical warnings and broad provider context have different public artifact semantics", name)
		}
		if slices.Contains(merge.Use, UseCriticalInfrastructure) && merge.IPV != "" && merge.IPV != "ipv4" {
			return fmt.Errorf("merge %q has use:[critical_infrastructure] with ipv %q; critical infrastructure overlap is IPv4-only in this release", name, merge.IPV)
		}
		for _, role := range merge.Use {
			switch role {
			case UseASN, UseGeoIP:
				return fmt.Errorf("merge %q has unsupported use role %q (merges produce ipsets; valid merge roles: bogons, critical_infrastructure, provider_context)", name, role)
			}
		}
		if merge.Provenance != "" && !merge.Provenance.Valid() {
			return fmt.Errorf("merge %q has invalid provenance %q", name, merge.Provenance)
		}
	}
	return validateCriticalArtifactNamespace(cfg)
}

func validateDefaultProviders(cfg *Config) error {
	if cfg == nil {
		return nil
	}
	if err := validateDefaultProvider(cfg, "asn_provider", cfg.Defaults.ASNProvider, UseASN); err != nil {
		return err
	}
	if err := validateDefaultProvider(cfg, "geo_provider", cfg.Defaults.GeoProvider, UseGeoIP); err != nil {
		return err
	}
	return nil
}

func validateDefaultProvider(cfg *Config, field, name, role string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	src := cfg.SourceByName(name)
	if src == nil {
		return fmt.Errorf("defaults.%s references unknown source %q", field, name)
	}
	if !src.HasUse(role) {
		return fmt.Errorf("defaults.%s references source %q without use:[%s]", field, name, role)
	}
	return nil
}

func validateStaticSourceBody(name string, src *Source, producesIPSet, criticalInfrastructure bool) error {
	if len(src.Static) == 0 {
		return nil
	}
	if src.URL != "" {
		return fmt.Errorf("source %q has both url and static body configured", name)
	}
	if !producesIPSet {
		return fmt.Errorf("source %q has static body but does not produce an ipset", name)
	}
	for i, line := range src.Static {
		if strings.TrimSpace(line) == "" {
			return fmt.Errorf("source %q static line %d is empty", name, i+1)
		}
		if line != strings.TrimSpace(line) {
			return fmt.Errorf("source %q static line %d has leading or trailing whitespace", name, i+1)
		}
		if strings.ContainsAny(line, "\r\n\x00") {
			return fmt.Errorf("source %q static line %d contains a newline or NUL byte", name, i+1)
		}
		if criticalInfrastructure {
			if err := validateCriticalInfrastructureStaticLine(line); err != nil {
				return fmt.Errorf("source %q static line %d has invalid critical infrastructure IP/CIDR %q: %w", name, i+1, line, err)
			}
		}
	}
	return nil
}

func validateCriticalInfrastructureStaticLine(line string) error {
	line = strings.TrimSpace(line)
	if strings.Contains(line, "/") {
		prefix, err := netip.ParsePrefix(line)
		if err != nil {
			return err
		}
		if !prefix.Addr().Is4() {
			return fmt.Errorf("expected IPv4 prefix")
		}
		return nil
	}
	addr, err := netip.ParseAddr(line)
	if err != nil {
		return err
	}
	if !addr.Is4() {
		return fmt.Errorf("expected IPv4 address")
	}
	return nil
}

func validateCriticalArtifactNamespace(cfg *Config) error {
	if cfg == nil {
		return nil
	}
	publicFeeds := map[string]struct{}{}
	targetFeeds := map[string]struct{}{}
	providers := make([]string, 0)
	for name, src := range cfg.Sources {
		if src == nil {
			continue
		}
		if isPublicFeedForCriticalArtifactNamespace(src) {
			publicFeeds[name] = struct{}{}
			if !src.HasUse(UseCriticalInfrastructure) && !src.HasUse(UseProviderContext) && (src.IPV == "" || src.IPV == "ipv4") {
				targetFeeds[name] = struct{}{}
			}
		}
		if src.HasUse(UseCriticalInfrastructure) {
			providers = append(providers, name)
		}
	}
	for name, merge := range cfg.Merges {
		if merge == nil {
			continue
		}
		hasCriticalRole := slices.Contains(merge.Use, UseCriticalInfrastructure)
		publicFeeds[name] = struct{}{}
		if !hasCriticalRole && (merge.IPV == "" || merge.IPV == "ipv4") {
			targetFeeds[name] = struct{}{}
		}
		if hasCriticalRole {
			providers = append(providers, name)
		}
	}
	targetNames := sortedStringKeys(targetFeeds)
	slices.Sort(providers)
	for _, target := range targetNames {
		aggregate := target + "_critical_infrastructure"
		if _, ok := publicFeeds[aggregate]; ok {
			return fmt.Errorf("public feed %q collides with generated critical infrastructure aggregate artifact for feed %q", aggregate, target)
		}
		for _, provider := range providers {
			providerArtifact := target + "_critical_" + provider
			if _, ok := publicFeeds[providerArtifact]; ok {
				return fmt.Errorf("public feed %q collides with generated critical infrastructure provider artifact for feed %q and provider %q", providerArtifact, target, provider)
			}
		}
	}
	return nil
}

func sortedStringKeys(in map[string]struct{}) []string {
	out := make([]string, 0, len(in))
	for key := range in {
		out = append(out, key)
	}
	slices.Sort(out)
	return out
}

func isPublicFeedForCriticalArtifactNamespace(src *Source) bool {
	if src == nil || src.Hidden {
		return false
	}
	if src.HasUse(UseASN) || src.HasUse(UseGeoIP) {
		return false
	}
	return sourceProducesIPSet(src)
}

func validateCriticalMetadata(kind, name string, hasRole bool, meta *CriticalMetadata) error {
	if hasRole && meta == nil {
		return fmt.Errorf("%s %q has use:[critical_infrastructure] but no critical metadata", kind, name)
	}
	if hasRole {
		if _, ok := reservedCriticalInfrastructureProviderNames[name]; ok {
			return fmt.Errorf("%s %q uses reserved critical infrastructure provider name %q", kind, name, name)
		}
	}
	if !hasRole && meta != nil {
		return fmt.Errorf("%s %q has critical metadata but no use:[critical_infrastructure]", kind, name)
	}
	if meta == nil {
		return nil
	}
	if _, ok := validCriticalTiers[meta.Tier]; !ok {
		return fmt.Errorf("%s %q has invalid critical tier %q (valid: hard, soft, contextual)", kind, name, meta.Tier)
	}
	if _, ok := validCriticalRoles[meta.Role]; !ok {
		return fmt.Errorf("%s %q has invalid critical role %q", kind, name, meta.Role)
	}
	if _, ok := validCriticalSourceTypes[meta.SourceType]; !ok {
		return fmt.Errorf("%s %q has invalid critical source_type %q", kind, name, meta.SourceType)
	}
	if _, ok := validCriticalSourceQualities[meta.SourceQuality]; !ok {
		return fmt.Errorf("%s %q has invalid critical source_quality %q (valid: A, B, C, D)", kind, name, meta.SourceQuality)
	}
	if strings.TrimSpace(meta.Rationale) == "" {
		return fmt.Errorf("%s %q has empty critical rationale", kind, name)
	}
	return nil
}

func validateCriticalASNContext(entries []CriticalASNContext) error {
	seen := make(map[uint32]struct{}, len(entries))
	for i, entry := range entries {
		if entry.ASN == 0 {
			return fmt.Errorf("critical_asn_context entry %d has invalid asn 0", i+1)
		}
		if _, ok := seen[entry.ASN]; ok {
			return fmt.Errorf("critical_asn_context entry AS%d is duplicated", entry.ASN)
		}
		seen[entry.ASN] = struct{}{}
		if reason, ok := disallowedCriticalASNContext[entry.ASN]; ok {
			return fmt.Errorf("critical_asn_context entry AS%d is disallowed: %s", entry.ASN, reason)
		}
		if strings.TrimSpace(entry.Name) == "" {
			return fmt.Errorf("critical_asn_context entry AS%d has empty name", entry.ASN)
		}
		switch entry.Tier {
		case "soft", "contextual":
		default:
			return fmt.Errorf("critical_asn_context entry AS%d has invalid tier %q (valid: soft, contextual)", entry.ASN, entry.Tier)
		}
		if _, ok := validCriticalRoles[entry.Role]; !ok {
			return fmt.Errorf("critical_asn_context entry AS%d has invalid role %q", entry.ASN, entry.Role)
		}
		if _, ok := validCriticalSourceQualities[entry.SourceQuality]; !ok {
			return fmt.Errorf("critical_asn_context entry AS%d has invalid source_quality %q (valid: A, B, C, D)", entry.ASN, entry.SourceQuality)
		}
		if strings.TrimSpace(entry.Rationale) == "" {
			return fmt.Errorf("critical_asn_context entry AS%d has empty rationale", entry.ASN)
		}
	}
	return nil
}

var disallowedCriticalASNContext = map[uint32]string{
	13335:  "Cloudflare mixes CDN edge with customer Workers, Pages, R2, and tenant-controlled workloads; use exact reference feeds",
	14618:  "AWS mixes provider services with customer workloads; use exact service feeds instead of ASN-wide context",
	15169:  "Google AS15169 is not a clean services-only boundary for GCP and Google-hosted workloads; use exact service feeds",
	16509:  "AWS mixes provider services with customer workloads; use exact service feeds instead of ASN-wide context",
	396982: "Google Cloud Platform is customer workload space; use provider_context or exact service feeds, not critical ASN context",
	8075:   "Microsoft AS8075 mixes Azure customer workloads with Microsoft services; use service tags or exact service feeds",
	8068:   "Microsoft AS8068 mixes Azure and Microsoft services; use service tags or exact service feeds",
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
