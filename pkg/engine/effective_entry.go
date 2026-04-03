package engine

import (
	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
)

// effectiveEntryResolver derives operator-facing "last upstream change"
// timestamps for local derivatives from their parents instead of from the
// local rebuild time. Integrity continues to use ProcessedDate.
type effectiveEntryResolver struct {
	cfg      *config.Config
	entries  map[string]cache.Entry
	memo     map[string]int64
	visiting map[string]bool
}

func newEffectiveEntryResolver(cfg *config.Config, entries map[string]cache.Entry) *effectiveEntryResolver {
	return &effectiveEntryResolver{
		cfg:      cfg,
		entries:  entries,
		memo:     make(map[string]int64, len(entries)),
		visiting: make(map[string]bool),
	}
}

func (e *Engine) effectiveEntryResolverFromFreshStateSnapshot() *effectiveEntryResolver {
	if e == nil || e.cfg == nil {
		return nil
	}
	entries := map[string]cache.Entry{}
	if e.state != nil {
		entries = e.state.SnapshotEntries()
	}
	return newEffectiveEntryResolver(e.cfg, entries)
}

func (r *effectiveEntryResolver) entry(name string, raw *cache.Entry) *cache.Entry {
	if raw == nil {
		return nil
	}
	clone := raw.Snapshot()
	if src := r.lookupSource(name); inheritsSingleParentHealth(src) {
		if parent := r.parentEntry(src); parent != nil {
			clone.StartedDate = parent.StartedDate
			clone.SourceDate = parent.SourceDate
			clone.Version = parent.Version
			clone.FrequencyMinutes = parent.FrequencyMinutes
			clone.AverageUpdateMins = parent.AverageUpdateMins
			clone.MinUpdateMins = parent.MinUpdateMins
			clone.MaxUpdateMins = parent.MaxUpdateMins
			clone.DownloadFailures = parent.DownloadFailures
			clone.FailureStartedDate = parent.FailureStartedDate
			clone.CheckedDate = parent.CheckedDate
			clone.LastStatus = parent.LastStatus
		}
	}
	if ts := r.lastChange(name); ts > 0 {
		clone.SourceDate = ts
	}
	return &clone
}

func (r *effectiveEntryResolver) entryFromSnapshot(name string) *cache.Entry {
	if r == nil {
		return nil
	}
	entry, ok := r.entries[name]
	if !ok {
		return nil
	}
	return r.entry(name, &entry)
}

func (r *effectiveEntryResolver) lastChange(name string) int64 {
	if name == "" {
		return 0
	}
	if ts, ok := r.memo[name]; ok {
		return ts
	}
	if r.visiting[name] {
		return r.baseLastChange(name)
	}
	r.visiting[name] = true
	defer delete(r.visiting, name)

	ts := r.baseLastChange(name)
	if src := r.lookupSource(name); src != nil && len(src.DerivedFrom) > 0 &&
		(src.Provenance == config.ProvenanceSecondaryMerge || src.Provenance == config.ProvenanceSecondaryRetention) {
		derivedTS := int64(0)
		for _, parent := range src.DerivedFrom {
			if parentTS := r.lastChange(parent); parentTS > derivedTS {
				derivedTS = parentTS
			}
		}
		if derivedTS > 0 {
			ts = derivedTS
		}
	}

	r.memo[name] = ts
	return ts
}

func (r *effectiveEntryResolver) lookupSource(name string) *config.Source {
	if r == nil || r.cfg == nil {
		return nil
	}
	return r.cfg.SourceByName(name)
}

func (r *effectiveEntryResolver) parentEntry(src *config.Source) *cache.Entry {
	if r == nil || src == nil || len(src.DerivedFrom) != 1 {
		return nil
	}
	parent, ok := r.entries[src.DerivedFrom[0]]
	if !ok {
		return nil
	}
	clone := parent
	return &clone
}

func inheritsSingleParentHealth(src *config.Source) bool {
	return src != nil &&
		src.Provenance == config.ProvenanceSecondaryRetention &&
		len(src.DerivedFrom) == 1
}

func (r *effectiveEntryResolver) baseLastChange(name string) int64 {
	entry, ok := r.entries[name]
	if !ok {
		return 0
	}
	if entry.SourceDate > 0 {
		return entry.SourceDate
	}
	return entry.ProcessedDate
}

// entryViewFromFreshStateSnapshot takes a full cache snapshot. Use it only for
// single-entry code paths; loop/batch paths must reuse an effectiveEntryResolver.
func (e *Engine) entryViewFromFreshStateSnapshot(name string, raw *cache.Entry) *cache.Entry {
	if e == nil || raw == nil || e.cfg == nil || e.state == nil {
		return raw
	}
	return e.effectiveEntryResolverFromFreshStateSnapshot().entry(name, raw)
}

func (e *Engine) EntrySnapshot(name string) *cache.Entry {
	if e == nil {
		return nil
	}
	return e.entryViewFromFreshStateSnapshot(name, e.state.EntrySnapshot(name))
}
