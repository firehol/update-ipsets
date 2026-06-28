package engine

import "github.com/firehol/update-ipsets/pkg/config"

type SchedulerConfigSnapshot struct {
	ProviderDatabases map[string]struct{}
	SourceParents     map[string][]string
	Artifacts         map[string]struct{}
}

func (s SchedulerConfigSnapshot) IsProviderDatabase(name string) bool {
	_, ok := s.ProviderDatabases[name]
	return ok
}

func (s SchedulerConfigSnapshot) DerivedFrom(name string) []string {
	parents := s.SourceParents[name]
	if len(parents) == 0 {
		return nil
	}
	return append([]string(nil), parents...)
}

func (s SchedulerConfigSnapshot) IsArtifact(name string) bool {
	_, ok := s.Artifacts[name]
	return ok
}

func (e *Engine) SchedulerConfigSnapshot() SchedulerConfigSnapshot {
	if e == nil {
		return SchedulerConfigSnapshot{}
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return SchedulerConfigSnapshotForConfig(e.cfg)
}

func (e *Engine) TrySchedulerConfigSnapshot() (SchedulerConfigSnapshot, bool) {
	if e == nil {
		return SchedulerConfigSnapshot{}, true
	}
	if !e.mu.TryRLock() {
		return SchedulerConfigSnapshot{}, false
	}
	defer e.mu.RUnlock()
	return SchedulerConfigSnapshotForConfig(e.cfg), true
}

func SchedulerConfigSnapshotForConfig(cfg *config.Config) SchedulerConfigSnapshot {
	snapshot := SchedulerConfigSnapshot{}
	if cfg == nil {
		return snapshot
	}
	if len(cfg.Sources) > 0 {
		snapshot.ProviderDatabases = make(map[string]struct{})
		snapshot.SourceParents = make(map[string][]string)
		for name, src := range cfg.Sources {
			if src == nil {
				continue
			}
			if src.HasUse(config.UseASN) || src.HasUse(config.UseGeoIP) {
				snapshot.ProviderDatabases[name] = struct{}{}
			}
			if len(src.DerivedFrom) > 0 {
				snapshot.SourceParents[name] = append([]string(nil), src.DerivedFrom...)
			}
		}
	}
	if len(cfg.Artifacts) > 0 {
		snapshot.Artifacts = make(map[string]struct{}, len(cfg.Artifacts))
		for name, artifact := range cfg.Artifacts {
			if artifact != nil {
				snapshot.Artifacts[name] = struct{}{}
			}
		}
	}
	return snapshot
}
