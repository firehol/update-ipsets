package engine

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/firehol/update-ipsets/internal/telemetry"
	"github.com/firehol/update-ipsets/pkg/asnloc"
	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/downloader"
	"github.com/firehol/update-ipsets/pkg/markdown"
	"github.com/firehol/update-ipsets/pkg/runreason"
)

type Engine struct {
	cfg                         *config.Config
	runtime                     Runtime
	cachePath                   string
	state                       *cache.State
	downloads                   *downloader.Client
	logger                      *slog.Logger
	now                         func() time.Time
	reloadMu                    sync.Mutex
	mu                          sync.RWMutex
	running                     bool
	runState                    RunState
	lastStarted                 time.Time
	lastEnded                   time.Time
	lastError                   string
	lastReport                  *Report
	currentReason               runreason.Reason
	lastReason                  runreason.Reason
	currentPhase                RunPhase
	currentBatch                *runBatchState
	currentPhasePlan            []RunPhase
	currentPhasePlanFinal       bool
	activeFeedsMu               sync.RWMutex
	activeFeeds                 map[string]ActiveFeed
	activeOperationsMu          sync.RWMutex
	activeOperations            map[string]ActiveOperation
	backgroundTasksMu           sync.RWMutex
	backgroundTaskSeq           uint64
	backgroundTasks             map[string]backgroundTaskState
	engineLane                  *WorkLane
	gitLane                     *WorkLane
	gitSyncSeq                  atomic.Uint64
	engineLaneDiagnosticsOnce   sync.Once
	engineLaneLongHoldWarningMu sync.RWMutex
	engineLaneLongHoldWarning   *LaneLongHoldWarning
	cachePersistenceMu          sync.Mutex
	cachePersistence            *cachePersistenceWorker
	runtimeStatsMu              sync.RWMutex
	runtimeStatsSamplerOnce     sync.Once
	runtimeStatsSampledAt       time.Time
	runtimeStats                engineRuntimeStats
	pipelineIntegrityCacheMu    sync.RWMutex
	pipelineIntegrityCaches     map[pipelineIntegrityCacheKey]*pipelineIntegrityCacheState
	entityIntegrityCacheMu      sync.RWMutex
	entityIntegrityCache        entityIntegrityCacheState
	// Entity artifact publication takes entityArtifactPublishMu as the
	// serialization lease. If both locks are needed, take this lock before
	// entityArtifactsMu; entityArtifactsMu protects generation only.
	entityArtifactPublishMu   sync.Mutex
	entityArtifactsMu         sync.Mutex
	entityArtifactsGeneration uint64
	entityRebuildQueued       bool
	entityRefreshPending      map[string]struct{}
	entityRefreshRunning      bool
	entityRefreshContinuation int
	entityHealthPending       map[string]struct{}
	entityHealthRunning       bool
	entityHealthContinuation  int
	currentMetricsPtr         atomic.Pointer[runMetrics]
	currentMetrics            *runMetrics
	lastMetrics               *RunMetricsSnapshot
	lifetimeOperations        telemetry.TimingBook
	lifetimeCounters          telemetry.CounterBook
	criticalProviderSetMu     sync.RWMutex
	criticalProviderSetID     string
	criticalProviderSetCached bool
	geoProviders              *geoProviderCache
	asnLookupCache            *asnDatabaseCache
	ledgerCache               *runtimeLedgerCache
	querySetCache             *sharedLatestSetCache
	runtimeOverrideWebDir     string
	runtimeOverrideFilesDir   string
	// retentionMaxWindow maps a parent source name to the longest
	// downloader-owned history-snapshot window any of its history
	// derivatives declares. It is used to prune old snapshots from
	// runtime.HistoryDir once they cannot affect any derivative any
	// more.
	retentionMaxWindow           map[string]time.Duration
	lastConfigReload             time.Time
	configReloadCount            int
	lastConfigReloadError        string
	startupRepairDeferred        bool
	startupRepairDeferredTargets int
	markdownTemplates            *markdown.TemplateStore
}

func New(configPath string, logger *slog.Logger) (*Engine, error) {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
	// Load .update-ipsets.env before config so env vars are available
	// for URL template expansion (API keys, license keys, etc.).
	loadEnvFile(logger)

	resolvedPath, err := resolveConfigPath(configPath)
	if err != nil {
		return nil, err
	}
	logger.Info("loading configuration", "path", resolvedPath)
	cfg, err := config.Load(resolvedPath)
	if err != nil {
		logger.Error("failed to load configuration", "path", resolvedPath, "error", err)
		return nil, err
	}
	now := time.Now().UTC()
	rt, err := resolveRuntime(cfg, now)
	if err != nil {
		return nil, err
	}
	rt.ConfigPath = resolvedPath
	for _, dir := range []string{rt.DistributionSuppliedIPSets, rt.AdminSuppliedIPSets, rt.UserSuppliedIPSets} {
		extra, err := config.LoadDirectory(dir)
		if err != nil {
			logger.Error("failed to load supplemental config directory", "dir", dir, "error", err)
			return nil, err
		}
		cfg.Merge(extra)
	}
	if err := config.Validate(cfg); err != nil {
		logger.Error("configuration validation failed", "error", err)
		return nil, err
	}
	logger.Info("configuration loaded",
		"sources", len(cfg.Sources),
		"merges", len(cfg.Merges),
		"geolocation_providers", len(cfg.SourcesWithUse(config.UseGeoIP)),
		"asn_providers", len(cfg.SourcesWithUse(config.UseASN)),
		"bogon_providers", len(cfg.SourcesWithUse(config.UseBogons)),
		"critical_infrastructure_providers", len(cfg.SourcesWithUse(config.UseCriticalInfrastructure)),
		"base_dir", rt.BaseDir,
		"lib_dir", rt.LibDir,
		"web_dir", rt.WebDir,
	)

	cachePath := filepath.Join(rt.BaseDir, ".cache.json")
	legacyCachePath := filepath.Join(rt.BaseDir, ".cache")
	st, err := cache.LoadWithMigration(cachePath, legacyCachePath)
	if err != nil {
		logger.Error("failed to load cache", "path", cachePath, "error", err)
		return nil, err
	}
	logger.Info("cache loaded", "entries", len(st.SnapshotEntries()), "path", cachePath)

	e := &Engine{
		cfg:            cfg,
		runtime:        rt,
		cachePath:      cachePath,
		state:          st,
		downloads:      downloader.New(rt.MaxConnectTime, rt.MaxDownloadTime),
		logger:         logger,
		now:            time.Now,
		engineLane:     NewWorkLane(rt.EngineLaneWorkers()),
		gitLane:        NewWorkLane(1),
		geoProviders:   newGeoProviderCache(),
		asnLookupCache: newASNDatabaseCache(),
		ledgerCache:    newRuntimeLedgerCache(),
	}
	e.querySetCache = newSharedLatestSetCache(e)
	if err := e.ensureDirectories(); err != nil {
		return nil, err
	}
	e.registerSyntheticInternalSources()
	e.reconcileEntriesFromSourceConfig()
	if err := e.bootstrapMissingEntriesFromDisk(); err != nil {
		return nil, err
	}
	if err := e.repairInvalidEntryTimestamps(); err != nil {
		return nil, err
	}
	if err := e.bootstrapLegacyFailureStarts(); err != nil {
		return nil, err
	}
	// Build the parent → longest-window map used by downloader-owned
	// history-snapshot pruning.
	e.buildRetentionMaxWindow()
	e.refreshCriticalInfrastructureProviderSetID()
	e.initMarkdownTemplates()
	return e, nil
}

// buildRetentionMaxWindow walks every history derivative and records, for each
// parent, the longest semantic retention window any derivative declares. The
// result feeds downloader-owned history-snapshot pruning.
//
// Safe to call multiple times (e.g. after Reload) — the result
// replaces the previous map wholesale.
func (e *Engine) buildRetentionMaxWindow() {
	if e.cfg == nil {
		return
	}
	m := make(map[string]time.Duration, len(e.cfg.Sources))
	for _, src := range e.cfg.Sources {
		if src == nil || src.Provenance != config.ProvenanceSecondaryRetention || len(src.DerivedFrom) == 0 {
			continue
		}
		window := e.historyDerivativeWindowDuration(src)
		if window <= 0 {
			e.logger.Warn("retention max-window: invalid derivative window", "source", src.Name)
			continue
		}
		parent := src.DerivedFrom[0]
		if existing := m[parent]; window > existing {
			m[parent] = window
		}
	}
	e.retentionMaxWindow = m
}

func (e *Engine) SetPushToGit(enabled bool) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.runtime.PushToGit = enabled
}

func (e *Engine) Reload() error {
	return e.ReloadContext(context.Background())
}

func (e *Engine) ReloadContext(ctx context.Context) error {
	if e == nil {
		return nil
	}
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return err
	}
	e.reloadMu.Lock()
	defer e.reloadMu.Unlock()

	currentRuntime := e.Runtime()
	e.logger.Info("reloading configuration", "path", currentRuntime.ConfigPath)
	cfg, err := config.Load(currentRuntime.ConfigPath)
	if err != nil {
		e.logger.Error("config reload failed", "error", err)
		e.recordConfigReloadError(err)
		return err
	}
	for _, dir := range []string{currentRuntime.DistributionSuppliedIPSets, currentRuntime.AdminSuppliedIPSets, currentRuntime.UserSuppliedIPSets} {
		extra, err := config.LoadDirectory(dir)
		if err != nil {
			e.logger.Error("config reload: failed to load supplemental dir", "dir", dir, "error", err)
			e.recordConfigReloadError(err)
			return err
		}
		cfg.Merge(extra)
	}
	if err := config.Validate(cfg); err != nil {
		e.logger.Error("config reload: validation failed", "error", err)
		e.recordConfigReloadError(err)
		return err
	}
	rt, err := resolveRuntime(cfg, e.now().UTC())
	if err != nil {
		e.recordConfigReloadError(err)
		return err
	}
	rt.ConfigPath = currentRuntime.ConfigPath
	var staleASNLookups map[string]*asnloc.Database
	webDirChanged := false
	var effectiveRuntime Runtime
	var lane *WorkLane
	e.mu.Lock()
	previousWebDir := e.runtime.WebDir
	e.cfg = cfg
	e.runtime = rt
	e.applyRuntimeOverridesLocked()
	effectiveRuntime = e.runtime
	webDirChanged = previousWebDir != effectiveRuntime.WebDir
	e.downloads = downloader.New(effectiveRuntime.MaxConnectTime, effectiveRuntime.MaxDownloadTime)
	if e.engineLane == nil {
		e.engineLane = NewWorkLane(effectiveRuntime.EngineLaneWorkers())
	}
	lane = e.engineLane
	e.geoProviders = newGeoProviderCache()
	if e.asnLookupCache == nil {
		e.asnLookupCache = newASNDatabaseCache()
	} else {
		staleASNLookups = e.asnLookupCache.retireAll()
	}
	e.ledgerCache = newRuntimeLedgerCache()
	e.registerSyntheticInternalSources()
	e.buildRetentionMaxWindow()
	e.mu.Unlock()

	if lane != nil {
		lane.SetLimit(effectiveRuntime.EngineLaneWorkers())
	}
	e.reconcileEntriesFromSourceConfigForSnapshot(cfg, effectiveRuntime)
	if err := ensureDirectoriesForRuntime(effectiveRuntime); err != nil {
		closeASNLookupDatabases(staleASNLookups, e.logger)
		e.recordConfigReloadError(err)
		return err
	}
	if err := e.bootstrapMissingEntriesFromDisk(); err != nil {
		closeASNLookupDatabases(staleASNLookups, e.logger)
		e.recordConfigReloadError(err)
		return err
	}
	if err := e.repairInvalidEntryTimestamps(); err != nil {
		closeASNLookupDatabases(staleASNLookups, e.logger)
		e.recordConfigReloadError(err)
		return err
	}
	if err := e.bootstrapLegacyFailureStarts(); err != nil {
		closeASNLookupDatabases(staleASNLookups, e.logger)
		e.recordConfigReloadError(err)
		return err
	}
	e.refreshCriticalInfrastructureProviderSetID()
	e.mu.Lock()
	e.configReloadCount++
	e.lastConfigReload = e.now()
	e.lastConfigReloadError = ""
	e.mu.Unlock()
	closeASNLookupDatabases(staleASNLookups, e.logger)
	if webDirChanged {
		e.MarkIntegrityCachesStale()
	}
	if _, err := e.QueueCriticalInfrastructureCleanup(ctx, "reload"); err != nil {
		e.setConfigReloadError(err)
		return err
	}
	return nil
}

func (e *Engine) recordConfigReloadError(err error) {
	if e == nil || err == nil {
		return
	}
	e.mu.Lock()
	e.configReloadCount++
	e.lastConfigReloadError = err.Error()
	e.mu.Unlock()
}

func (e *Engine) setConfigReloadError(err error) {
	if e == nil || err == nil {
		return
	}
	e.mu.Lock()
	e.lastConfigReloadError = err.Error()
	e.mu.Unlock()
}

func (e *Engine) AcquireLock() (*FileLock, error) {
	return acquireLock(e.Runtime().LockFile)
}

func (e *Engine) Config() *config.Config {
	if e == nil {
		return nil
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.cfg
}

func (e *Engine) Runtime() Runtime {
	if e == nil {
		return Runtime{}
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.runtime
}

func (e *Engine) configRuntimeSnapshot() (*config.Config, Runtime) {
	if e == nil {
		return nil, Runtime{}
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.cfg, e.runtime
}

func (e *Engine) Enable(names []string, all bool) error {
	if e == nil {
		return nil
	}
	cfg, rt := e.configRuntimeSnapshot()
	if all {
		// After config.ExpandDerivatives, every feed (including
		// merges and retention variants) is in cfg.Sources. The
		// old sortedMergeNames walk is redundant.
		names = config.SortedSourceNames(cfg)
	}
	for _, name := range names {
		if name == "" {
			continue
		}
		path := sourceEnablePathForRuntime(rt, name)
		if err := os.MkdirAll(filepath.Dir(path), generatedDirMode); err != nil {
			return err
		}
		if err := touchFileAt(path, time.Unix(0, 0)); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) EnableArtifacts(names []string, all bool) error {
	if e == nil {
		return nil
	}
	cfg, rt := e.configRuntimeSnapshot()
	if all {
		names = config.SortedArtifactNames(cfg)
	}
	for _, name := range names {
		if name == "" || cfg == nil || cfg.ArtifactByName(name) == nil {
			continue
		}
		path := artifactEnablePathForRuntime(rt, name)
		if err := os.MkdirAll(filepath.Dir(path), generatedDirMode); err != nil {
			return err
		}
		if err := touchFileAt(path, time.Unix(0, 0)); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) Disable(names []string, all bool) error {
	if e == nil {
		return nil
	}
	cfg, rt := e.configRuntimeSnapshot()
	if all {
		// After config.ExpandDerivatives, every feed (including
		// merges and retention variants) is in cfg.Sources. The
		// old sortedMergeNames walk is redundant.
		names = config.SortedSourceNames(cfg)
	}
	for _, name := range names {
		if name == "" {
			continue
		}
		if err := os.Remove(sourceEnablePathForRuntime(rt, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (e *Engine) DisableArtifacts(names []string, all bool) error {
	if e == nil {
		return nil
	}
	cfg, rt := e.configRuntimeSnapshot()
	if all {
		names = config.SortedArtifactNames(cfg)
	}
	for _, name := range names {
		if name == "" || cfg == nil || cfg.ArtifactByName(name) == nil {
			continue
		}
		if err := os.Remove(artifactEnablePathForRuntime(rt, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}
