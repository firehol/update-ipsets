package engine

import (
	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/feedhealth"
)

func (e *Engine) classifyEffectiveEntryHealth(name string, entry *cache.Entry) feedhealth.Snapshot {
	if e == nil || e.now == nil {
		return feedhealth.Snapshot{Class: feedhealth.ClassHealthy}
	}
	cfg, _, policy := e.configRuntimePolicySnapshot()
	return e.classifyEffectiveEntryHealthWithConfigPolicy(cfg, policy, name, entry)
}

func (e *Engine) classifyEffectiveEntryHealthWithSnapshot(snap operationSnapshot, name string, entry *cache.Entry) feedhealth.Snapshot {
	if e == nil || e.now == nil {
		return feedhealth.Snapshot{Class: feedhealth.ClassHealthy}
	}
	return e.classifyEffectiveEntryHealthWithConfigPolicy(snap.cfg, snap.feedHealthPolicy, name, entry)
}

func (e *Engine) classifyEffectiveEntryHealthWithConfigPolicy(cfg *config.Config, policy feedhealth.Policy, name string, entry *cache.Entry) feedhealth.Snapshot {
	if cfg == nil {
		return feedhealth.Snapshot{Class: feedhealth.ClassHealthy}
	}
	return feedhealth.Classify(entry, lookupSourceForConfig(cfg, name), policy, e.now().UTC())
}

func (e *Engine) healthSnapshotFromFreshStateSnapshot(name string, entry *cache.Entry) feedhealth.Snapshot {
	return e.classifyEffectiveEntryHealth(name, e.entryViewFromFreshStateSnapshot(name, entry))
}
