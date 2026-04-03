package engine

import (
	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/feedhealth"
)

func (e *Engine) classifyEffectiveEntryHealth(name string, entry *cache.Entry) feedhealth.Snapshot {
	if e == nil || e.cfg == nil || e.now == nil {
		return feedhealth.Snapshot{Class: feedhealth.ClassHealthy}
	}
	return feedhealth.Classify(entry, e.lookupSource(name), feedhealth.PolicyFromRuntime(e.cfg.Runtime), e.now().UTC())
}

func (e *Engine) healthSnapshotFromFreshStateSnapshot(name string, entry *cache.Entry) feedhealth.Snapshot {
	return e.classifyEffectiveEntryHealth(name, e.entryViewFromFreshStateSnapshot(name, entry))
}
