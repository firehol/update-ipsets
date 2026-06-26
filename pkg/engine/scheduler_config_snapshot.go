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
	snapshot := SchedulerConfigSnapshot{}
	if e == nil {
		return snapshot
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.cfg == nil {
		return snapshot
	}
	if len(e.cfg.Sources) > 0 {
		snapshot.ProviderDatabases = make(map[string]struct{})
		snapshot.SourceParents = make(map[string][]string)
		for name, src := range e.cfg.Sources {
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
	if len(e.cfg.Artifacts) > 0 {
		snapshot.Artifacts = make(map[string]struct{}, len(e.cfg.Artifacts))
		for name, artifact := range e.cfg.Artifacts {
			if artifact != nil {
				snapshot.Artifacts[name] = struct{}{}
			}
		}
	}
	return snapshot
}
