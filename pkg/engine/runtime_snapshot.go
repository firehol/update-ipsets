package engine

import (
	"path/filepath"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/downloader"
	"github.com/firehol/update-ipsets/pkg/feedhealth"
)

type operationSnapshot struct {
	cfg                *config.Config
	runtime            Runtime
	downloads          *downloader.Client
	geoProviders       *geoProviderCache
	ledgerCache        *runtimeLedgerCache
	retentionMaxWindow map[string]time.Duration
	asnLookupCache     *asnDatabaseCache
	feedHealthPolicy   feedhealth.Policy
}

func (e *Engine) operationSnapshot() operationSnapshot {
	if e == nil {
		return operationSnapshot{}
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return operationSnapshot{
		cfg:                e.cfg,
		runtime:            e.runtime,
		downloads:          e.downloads,
		geoProviders:       e.geoProviders,
		ledgerCache:        e.ledgerCache,
		retentionMaxWindow: cloneRetentionWindowMap(e.retentionMaxWindow),
		asnLookupCache:     e.asnLookupCache,
		feedHealthPolicy:   feedhealth.PolicyFromConfig(e.cfg),
	}
}

func (e *Engine) tryOperationSnapshot() (operationSnapshot, bool) {
	if e == nil {
		return operationSnapshot{}, true
	}
	if !e.mu.TryRLock() {
		return operationSnapshot{}, false
	}
	defer e.mu.RUnlock()
	return operationSnapshot{
		cfg:                e.cfg,
		runtime:            e.runtime,
		downloads:          e.downloads,
		geoProviders:       e.geoProviders,
		ledgerCache:        e.ledgerCache,
		retentionMaxWindow: cloneRetentionWindowMap(e.retentionMaxWindow),
		asnLookupCache:     e.asnLookupCache,
		feedHealthPolicy:   feedhealth.PolicyFromConfig(e.cfg),
	}, true
}

func (e *Engine) configRuntimePolicySnapshot() (*config.Config, Runtime, feedhealth.Policy) {
	if e == nil {
		return nil, Runtime{}, feedhealth.Policy{}
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.cfg, e.runtime, feedhealth.PolicyFromConfig(e.cfg)
}

func (e *Engine) ConfigRuntimePolicySnapshot() (*config.Config, Runtime, feedhealth.Policy) {
	return e.configRuntimePolicySnapshot()
}

func (e *Engine) TryConfigRuntimePolicySnapshot() (*config.Config, Runtime, feedhealth.Policy, bool) {
	if e == nil {
		return nil, Runtime{}, feedhealth.Policy{}, true
	}
	if !e.mu.TryRLock() {
		return nil, Runtime{}, feedhealth.Policy{}, false
	}
	defer e.mu.RUnlock()
	return e.cfg, e.runtime, feedhealth.PolicyFromConfig(e.cfg), true
}

func cloneRetentionWindowMap(in map[string]time.Duration) map[string]time.Duration {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]time.Duration, len(in))
	for name, window := range in {
		out[name] = window
	}
	return out
}

func (s operationSnapshot) sourcePath(name string) string {
	return sourcePathForRuntime(s.runtime, name)
}

func (s operationSnapshot) feedBodyPath(name string) string {
	if s.cfg == nil {
		return filepath.Join(s.runtime.BaseDir, name+".ipset")
	}
	src := s.cfg.Sources[name]
	if src == nil {
		return filepath.Join(s.runtime.BaseDir, name+".ipset")
	}
	return finalPathForRuntime(s.runtime, name, src.Output)
}

func (s operationSnapshot) isEnabled(name string, opts RunOptions) bool {
	return EffectiveSourceEnabledForRun(
		s.cfg,
		s.runtime,
		name,
		opts.EnableAll,
		opts.Manual || isSelected(name, opts.Selected),
	)
}

func (s operationSnapshot) isDownloadable(name string) bool {
	if s.cfg != nil && s.cfg.ArtifactByName(name) != nil {
		return true
	}
	if s.cfg == nil {
		return false
	}
	src := s.cfg.Sources[name]
	if src == nil {
		return false
	}
	if src.ArtifactParent != "" {
		return true
	}
	return src.URL != "" || len(src.Static) > 0
}

func (s operationSnapshot) isMerge(name string) bool {
	if s.cfg == nil {
		return false
	}
	src := s.cfg.Sources[name]
	return src != nil && src.Provenance == config.ProvenanceSecondaryMerge
}

func (s operationSnapshot) isHistoryDerivative(name string) bool {
	if s.cfg == nil {
		return false
	}
	src := s.cfg.Sources[name]
	return src != nil && src.Provenance == config.ProvenanceSecondaryRetention
}
