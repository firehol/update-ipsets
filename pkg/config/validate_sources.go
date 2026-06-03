package config

import (
	"fmt"
	"net/netip"
	"net/url"
	"slices"
	"strings"

	"github.com/firehol/update-ipsets/pkg/enrichment"
)

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

func validateSources(cfg *Config) error {
	for name, src := range cfg.Sources {
		if err := validateSource(cfg, name, src); err != nil {
			return err
		}
	}
	return nil
}

func validateSource(cfg *Config, name string, src *Source) error {
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
	if err := validateSourceRoleCompatibility(name, src); err != nil {
		return err
	}
	producesIPSet := sourceProducesIPSet(src)
	if err := validateStaticSourceBody(name, src, producesIPSet, src.HasUse(UseCriticalInfrastructure)); err != nil {
		return err
	}
	if err := validateSourceIPSetShape(name, src, producesIPSet); err != nil {
		return err
	}
	if err := validateSourceProviderFormat(name, src); err != nil {
		return err
	}
	return validateSourceURL(cfg, name, src)
}

func validateSourceRoleCompatibility(name string, src *Source) error {
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

func validateSourceIPSetShape(name string, src *Source, producesIPSet bool) error {
	if !producesIPSet {
		return nil
	}
	if src.IPV != "ipv4" && src.IPV != "ipv6" {
		return fmt.Errorf("source %q has invalid ip version %q", name, src.IPV)
	}
	if !slices.Contains(validOutputs, src.Output) {
		return fmt.Errorf("source %q has invalid output %q", name, src.Output)
	}
	return nil
}

func validateSourceProviderFormat(name string, src *Source) error {
	// Format is required for asn and geoip roles since the engine
	// dispatches the parser by format. ipset sources may omit it
	// (the default text parser is implied by the processor chain).
	if !src.HasUse(UseASN) && !src.HasUse(UseGeoIP) {
		return nil
	}
	if src.Format == "" {
		return fmt.Errorf("source %q has use:[%s] but no format set", name, strings.Join(src.Use, ","))
	}
	return nil
}

func validateSourceURL(cfg *Config, name string, src *Source) error {
	if src.URL == "" {
		return nil
	}
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
	return nil
}
