package config

import (
	"fmt"
	"slices"

	"github.com/firehol/update-ipsets/pkg/enrichment"
)

func validateMerges(cfg *Config) error {
	// LoadYAML/LoadDirectory validate merge references before expansion, then
	// ExpandDerivatives clears cfg.Merges. This loop remains for programmatic
	// callers and tests that call Validate directly on an unexpanded Config.
	for name, merge := range cfg.Merges {
		if err := validateMerge(name, merge); err != nil {
			return err
		}
	}
	return nil
}

func validateMerge(name string, merge *Merge) error {
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
	if err := validateMergeRoleCompatibility(name, merge); err != nil {
		return err
	}
	if merge.Provenance != "" && !merge.Provenance.Valid() {
		return fmt.Errorf("merge %q has invalid provenance %q", name, merge.Provenance)
	}
	return nil
}

func validateMergeRoleCompatibility(name string, merge *Merge) error {
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
	return nil
}
