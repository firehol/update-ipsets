package config

import (
	"fmt"
	"slices"
	"strings"
)

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
