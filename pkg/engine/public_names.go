package engine

import (
	"slices"

	"github.com/firehol/update-ipsets/pkg/config"
)

func (e *Engine) outputDir() string {
	return outputDirForRuntime(e.Runtime())
}

func outputDirForRuntime(rt Runtime) string {
	if rt.WebDir != "" {
		return rt.WebDir
	}
	return rt.BaseDir
}

func (e *Engine) outputNames() []string {
	snapMap := e.state.SnapshotEntries()
	configured := e.configuredNames()
	names := make([]string, 0, len(snapMap))
	for name := range snapMap {
		if !configured[name] || !e.hasUsableSet(name) {
			continue
		}
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func (e *Engine) outputNamesForSnapshot(snap operationSnapshot) []string {
	snapMap := e.state.SnapshotEntries()
	configured := configuredNamesForConfig(snap.cfg)
	names := make([]string, 0, len(snapMap))
	for name := range snapMap {
		if !configured[name] || !e.hasUsableSetForSnapshot(snap, name) {
			continue
		}
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func (e *Engine) publicOutputNames() []string {
	all := e.outputNames()
	out := make([]string, 0, len(all))
	for _, name := range all {
		if e.isPublicFeedName(name) {
			out = append(out, name)
		}
	}
	return out
}

func (e *Engine) publicOutputNamesForSnapshot(snap operationSnapshot) []string {
	all := e.outputNamesForSnapshot(snap)
	out := make([]string, 0, len(all))
	for _, name := range all {
		if isPublicFeedNameForConfig(snap.cfg, name) {
			out = append(out, name)
		}
	}
	return out
}

// configuredNames returns the set of all names that correspond to configured
// feeds: sources plus their history window variants (_1d/_7d/etc.). After
// source unification, ASN/GeoIP/Bogon database "providers" are themselves
// regular sources and so are already included via the sources walk.
// Unconfigured/stale cache entries are excluded. The legacy bash "split"
// output mode (which produced separate _ip and _net variants from one source)
// is no longer supported; only ipset and netset remain.
func (e *Engine) configuredNames() map[string]bool {
	return configuredNamesForConfig(e.Config())
}

func configuredNamesForConfig(cfg *config.Config) map[string]bool {
	if cfg == nil {
		return map[string]bool{}
	}
	names := make(map[string]bool, len(cfg.Sources)+len(cfg.Merges))
	for name, src := range cfg.Sources {
		names[name] = true
		for _, minutes := range src.History {
			names[name+historyLabel(minutes)] = true
		}
	}
	for name := range cfg.Merges {
		names[name] = true
	}
	return names
}

func configuredNamesWithArtifactsForConfig(cfg *config.Config) map[string]bool {
	names := configuredNamesForConfig(cfg)
	if cfg == nil {
		return names
	}
	for name := range cfg.Artifacts {
		names[name] = true
	}
	return names
}
