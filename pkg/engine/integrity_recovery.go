package engine

import (
	"slices"

	"github.com/firehol/update-ipsets/pkg/config"
)

// IntegrityRecoveryPlan splits integrity findings into the two recovery modes
// the split pipeline supports:
//   - recheck: feeds/artifacts that must refresh their staged input first
//   - reprocess: feeds that can be regenerated from already-available local state
//
// Download-backed sources need a recheck only when no durable canonical feed
// body exists locally. Otherwise a processing-only reprocess is enough.
// Merge sources use the same recheck queue entry, but their "recheck" is a
// local recomposition from parent feed bodies rather than an upstream fetch.
// Artifact-backed children follow the same rule, but when their retained
// materialized local input still exists they can recheck directly; otherwise
// their recovery trigger is the artifact parent.
func (e *Engine) IntegrityRecoveryPlan(findings []IntegrityFinding) (recheck []string, reprocess []string) {
	if e == nil || e.cfg == nil || len(findings) == 0 {
		return nil, nil
	}
	recheckSet := map[string]struct{}{}
	reprocessSet := map[string]struct{}{}
	for _, finding := range findings {
		recheckTargets, reprocessTargets := e.integrityRecoveryForFinding(finding)
		for _, target := range recheckTargets {
			recheckSet[target] = struct{}{}
		}
		for _, target := range reprocessTargets {
			reprocessSet[target] = struct{}{}
		}
	}
	for name := range reprocessSet {
		if _, ok := recheckSet[name]; ok {
			delete(reprocessSet, name)
		}
	}
	return sortedNames(recheckSet), sortedNames(reprocessSet)
}

func (e *Engine) integrityRecoveryForFinding(finding IntegrityFinding) (recheck []string, reprocess []string) {
	if e == nil || e.cfg == nil {
		return nil, nil
	}
	findingSource := e.cfg.Sources[finding.Feed]
	if findingSource == nil {
		return nil, nil
	}

	if len(finding.BlockedFeeds) > 0 {
		recheckSet := map[string]struct{}{}
		for _, blocked := range finding.BlockedFeeds {
			if findingSource.Provenance == config.ProvenanceSecondaryRetention {
				recheckSet[blocked] = struct{}{}
				continue
			}
			if target, ok := e.integrityRecheckTarget(blocked); ok {
				recheckSet[target] = struct{}{}
			}
		}
		if len(recheckSet) > 0 {
			recheck = sortedNames(recheckSet)
			if findingSource.Provenance == config.ProvenanceSecondaryMerge || findingSource.Provenance == config.ProvenanceSecondaryRetention {
				return recheck, nil
			}
		}
	}

	if target, ok := e.integrityRecheckTarget(finding.Feed); ok {
		return appendUniqueStrings(recheck, []string{target}), nil
	}

	return recheck, []string{finding.Feed}
}

func (e *Engine) integrityRecheckTarget(name string) (string, bool) {
	if e == nil || e.cfg == nil {
		return "", false
	}
	src := e.cfg.Sources[name]
	if src == nil {
		return "", false
	}
	if src.ArtifactParent != "" {
		if fileExists(preferStagedPath(e.sourcePath(name))) {
			return name, true
		}
		if e.integrityHasCommittedOrStagedSource(name) {
			return "", false
		}
		if e.cfg.ArtifactByName(src.ArtifactParent) != nil {
			return src.ArtifactParent, true
		}
		return "", false
	}
	if e.IsDownloadable(name) && !e.integrityHasCommittedOrStagedSource(name) {
		return name, true
	}
	return "", false
}

func (e *Engine) integrityHasCommittedOrStagedSource(name string) bool {
	if e == nil || e.cfg == nil {
		return false
	}
	src := e.cfg.Sources[name]
	if src == nil {
		return false
	}
	if e.IsProviderDatabase(name) {
		return fileExists(preferStagedPath(e.providerArchivePath(name, src)))
	}
	return fileExists(latestFeedBodyPath(e.feedBodyPath(name)))
}

func sortedNames(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
