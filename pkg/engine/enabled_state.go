package engine

import "github.com/firehol/update-ipsets/pkg/config"

// EffectiveArtifactEnabled reports whether the artifact parent is enabled by
// operator policy. Artifact parents have their own enable marker and do not
// inherit enablement from child feeds.
func EffectiveArtifactEnabled(cfg *config.Config, rt Runtime, name string, enableAll bool) bool {
	return effectiveArtifactEnabled(cfg, rt, name, enableAll, false, map[string]bool{})
}

// EffectiveSourceEnabled reports whether a feed is effectively enabled after
// applying parent constraints:
//   - artifact-backed feeds require their artifact parent to be enabled
//   - retention derivatives require their parent feed to be enabled
//   - merges require at least one enabled input
//
// This variant never overrides the feed's own enable marker.
func EffectiveSourceEnabled(cfg *config.Config, rt Runtime, name string, enableAll bool) bool {
	return effectiveSourceEnabled(cfg, rt, name, enableAll, false, map[string]bool{}, map[string]bool{})
}

// EffectiveSourceEnabledForRun is the processing-time variant. It can override
// the root feed's own enable marker for explicit operator requests while still
// enforcing parent constraints.
func EffectiveSourceEnabledForRun(cfg *config.Config, rt Runtime, name string, enableAll, overrideRoot bool) bool {
	return effectiveSourceEnabled(cfg, rt, name, enableAll, overrideRoot, map[string]bool{}, map[string]bool{})
}

func effectiveSourceEnabled(cfg *config.Config, rt Runtime, name string, enableAll, explicitOverride bool, sourceSeen, artifactSeen map[string]bool) bool {
	if cfg == nil || cfg.Sources == nil {
		return false
	}
	src := cfg.Sources[name]
	if src == nil {
		return false
	}
	if sourceSeen[name] {
		return false
	}
	sourceSeen[name] = true
	defer delete(sourceSeen, name)

	explicit := enableAll || explicitOverride || fileExists(enableMarkerPathForSource(rt, name, src))
	if !explicit {
		return false
	}
	if src.ArtifactParent != "" {
		return effectiveArtifactEnabled(cfg, rt, src.ArtifactParent, enableAll, false, artifactSeen)
	}
	if src.Provenance == config.ProvenanceSecondaryRetention {
		for _, parent := range src.DerivedFrom {
			if effectiveSourceEnabled(cfg, rt, parent, enableAll, false, sourceSeen, artifactSeen) {
				return true
			}
		}
		return false
	}
	if src.Provenance == config.ProvenanceSecondaryMerge {
		for _, parent := range mergeSourceNames(src) {
			if effectiveSourceEnabled(cfg, rt, parent, enableAll, false, sourceSeen, artifactSeen) {
				return true
			}
		}
		return false
	}
	if src.URL == "" {
		return true
	}
	return true
}

func enableMarkerPathForSource(rt Runtime, name string, src *config.Source) string {
	_ = src
	return sourceEnablePathForRuntime(rt, name)
}

func effectiveArtifactEnabled(cfg *config.Config, rt Runtime, name string, enableAll, explicitOverride bool, artifactSeen map[string]bool) bool {
	if cfg == nil || cfg.ArtifactByName(name) == nil {
		return false
	}
	if artifactSeen[name] {
		return false
	}
	artifactSeen[name] = true
	defer delete(artifactSeen, name)

	return enableAll || explicitOverride || fileExists(artifactEnablePathForRuntime(rt, name))
}
