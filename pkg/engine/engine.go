package engine

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/firehol/update-ipsets/internal/telemetry"
	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/downloader"
	"github.com/firehol/update-ipsets/pkg/feedhealth"
	"github.com/firehol/update-ipsets/pkg/markdown"
	"github.com/firehol/update-ipsets/pkg/runreason"
)

type RunOptions struct {
	Selected   []string
	EnableAll  bool
	Recheck    bool
	Reprocess  bool
	Manual     bool
	CleanupOld bool
	Reason     runreason.Reason
	// BeforePublish runs after processing/local artifact generation has
	// completed successfully and immediately before public publication
	// begins. The scheduler uses this hook to promote only the staged
	// downloader outputs that correspond to feeds whose processing
	// completed successfully in the current batch.
	BeforePublish func(report *Report) error
}

type Report struct {
	StartedAt            time.Time         `json:"started_at"`
	EndedAt              time.Time         `json:"ended_at"`
	Updated              []string          `json:"updated,omitempty"`
	EntityRefreshTargets []string          `json:"entity_refresh_targets,omitempty"`
	Skipped              []string          `json:"skipped,omitempty"`
	Failed               []string          `json:"failed,omitempty"`
	Messages             map[string]string `json:"messages,omitempty"`
	Statuses             map[string]string `json:"statuses,omitempty"`
}

type QueryMatch struct {
	Name       string              `json:"name"`
	File       string              `json:"file,omitempty"`
	Category   string              `json:"category,omitempty"`
	Provenance string              `json:"provenance,omitempty"`
	Info       string              `json:"info,omitempty"`
	Maintainer string              `json:"maintainer,omitempty"`
	FirstSeen  int64               `json:"first_seen,omitempty"`
	LastSeen   int64               `json:"last_seen,omitempty"`
	Health     feedhealth.Snapshot `json:"health"`
	Error      string              `json:"error,omitempty"`
}

type HistoryPoint struct {
	Timestamp int64  `json:"timestamp"`
	Name      string `json:"name"`
	Entries   int    `json:"entries"`
	UniqueIPs uint64 `json:"unique_ips"`
}

// ChangesetPoint is a single "what changed between update N-1 and N" record
// for a feed. Added + Removed are the raw counts from the engine's
// changesets.csv, and their sum is non-zero. Consumers typically render these
// as a bidirectional bar chart (added above the axis, removed below) so both
// the net delta and the true churn are visible at a glance — the net can be
// zero while the underlying list was completely refreshed.
type ChangesetPoint struct {
	Timestamp int64  `json:"timestamp"`
	Added     uint64 `json:"added"`
	Removed   uint64 `json:"removed"`
}

type CompareRow struct {
	Name     string `json:"name"`
	Category string `json:"category,omitempty"`
	IPs      uint64 `json:"ips"`
	// Common is always positive in public comparison artifacts. A zero
	// overlap is represented by the absence of a row.
	Common uint64 `json:"common"`
	// Related marks rows that belong to the same derivative
	// family as the containing feed — retention variants of the
	// same parent, merges whose inputs include this feed,
	// variants of such inputs, or the primary source from which
	// this feed derives. The set is computed via leafAncestors()
	// in output.go: two feeds are "related" iff their leaf-
	// ancestor sets intersect.
	//
	// The public UI's overlap-facts tiles (UNIQUE, INCLUDED IN,
	// INCLUDES, ≥50% OVERLAP) exclude related rows from their
	// math because a retention variant trivially overlaps its
	// parent 100% and a merge trivially contains every one of
	// its inputs — including those rows in the tiles makes the
	// "unique IPs" count drop to zero for every feed with any
	// derivative.
	Related bool `json:"related,omitempty"`
}

type RetentionSeries struct {
	Hours []int    `json:"hours"`
	IPs   []uint64 `json:"ips"`
	Total uint64   `json:"total"`
}

type RetentionData struct {
	Name       string          `json:"ipset"`
	Started    int64           `json:"started"`
	Updated    int64           `json:"updated"`
	Incomplete int             `json:"incomplete"`
	Past       RetentionSeries `json:"past"`
	Current    RetentionSeries `json:"current"`
}

type StatusSnapshot struct {
	Running                      bool                     `json:"running"`
	LastStarted                  time.Time                `json:"last_started,omitempty"`
	LastEnded                    time.Time                `json:"last_ended,omitempty"`
	LastError                    string                   `json:"last_error,omitempty"`
	LastReport                   *Report                  `json:"last_report,omitempty"`
	CurrentReason                runreason.Reason         `json:"current_reason,omitempty"`
	LastReason                   runreason.Reason         `json:"last_reason,omitempty"`
	CurrentPhase                 RunPhase                 `json:"current_phase,omitempty"`
	ActiveFeeds                  []ActiveFeed             `json:"active_feeds,omitempty"`
	BackgroundTasks              []BackgroundTaskSnapshot `json:"background_tasks,omitempty"`
	BackgroundLimit              int                      `json:"background_limit,omitempty"`
	BackgroundRunning            int                      `json:"background_running,omitempty"`
	CurrentMetrics               *RunMetricsSnapshot      `json:"current_metrics,omitempty"`
	LastMetrics                  *RunMetricsSnapshot      `json:"last_metrics,omitempty"`
	LifetimeMetrics              *LifetimeMetricsSnapshot `json:"lifetime_metrics,omitempty"`
	ConfigPath                   string                   `json:"config_path"`
	BaseDir                      string                   `json:"base_dir"`
	MaxIngestWorkers             int                      `json:"max_ingest_workers,omitempty"`
	ParallelDownloads            int                      `json:"parallel_downloads"`
	ParallelDNSQueries           int                      `json:"parallel_dns_queries"`
	MaxProcessingWorkers         int                      `json:"max_processing_workers"`
	MaxHeavyPhaseWorkers         int                      `json:"max_heavy_phase_workers"`
	MaxBackgroundWorkers         int                      `json:"max_background_workers"`
	SourceCount                  int                      `json:"source_count"`
	MergeCount                   int                      `json:"merge_count"`
	EntityRefreshPending         int                      `json:"entity_refresh_pending,omitempty"`
	EntityHealthPending          int                      `json:"entity_health_pending,omitempty"`
	EntityRebuildPending         bool                     `json:"entity_rebuild_pending,omitempty"`
	LastConfigReload             time.Time                `json:"last_config_reload,omitempty"`
	ConfigReloadCount            int                      `json:"config_reload_count,omitempty"`
	LastConfigReloadError        string                   `json:"last_config_reload_error,omitempty"`
	StartupRepairDeferred        bool                     `json:"startup_repair_deferred,omitempty"`
	StartupRepairDeferredTargets int                      `json:"startup_repair_deferred_targets,omitempty"`
}

type LifetimeMetricsSnapshot struct {
	Operations []telemetry.TimingStatSnapshot  `json:"operations,omitempty"`
	Counters   []telemetry.CounterStatSnapshot `json:"counters,omitempty"`
}

type ActiveFeed struct {
	Name      string           `json:"name"`
	Reason    runreason.Reason `json:"reason,omitempty"`
	StartedAt time.Time        `json:"started_at,omitempty"`
}

type Engine struct {
	cfg                       *config.Config
	runtime                   Runtime
	cachePath                 string
	state                     *cache.State
	downloads                 *downloader.Client
	logger                    *slog.Logger
	now                       func() time.Time
	mu                        sync.RWMutex
	running                   bool
	lastStarted               time.Time
	lastEnded                 time.Time
	lastError                 string
	lastReport                *Report
	currentReason             runreason.Reason
	lastReason                runreason.Reason
	currentPhase              RunPhase
	activeFeeds               map[string]ActiveFeed
	backgroundTaskSeq         uint64
	backgroundTasks           map[string]backgroundTaskState
	backgroundLimiter         *backgroundLimiter
	entityArtifactsMu         sync.Mutex
	entityRebuildQueued       bool
	entityRefreshPending      map[string]struct{}
	entityRefreshRunning      bool
	entityHealthPending       map[string]struct{}
	entityHealthRunning       bool
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
		cfg:               cfg,
		runtime:           rt,
		cachePath:         cachePath,
		state:             st,
		downloads:         downloader.New(rt.MaxConnectTime, rt.MaxDownloadTime),
		logger:            logger,
		now:               time.Now,
		backgroundLimiter: newBackgroundLimiter(rt.BackgroundWorkers()),
		geoProviders:      newGeoProviderCache(),
		asnLookupCache:    newASNDatabaseCache(),
		ledgerCache:       newRuntimeLedgerCache(),
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
	e.runtime.PushToGit = enabled
}

func (e *Engine) Reload() error {
	e.logger.Info("reloading configuration", "path", e.runtime.ConfigPath)
	cfg, err := config.Load(e.runtime.ConfigPath)
	if err != nil {
		e.logger.Error("config reload failed", "error", err)
		e.mu.Lock()
		e.configReloadCount++
		e.lastConfigReloadError = err.Error()
		e.mu.Unlock()
		return err
	}
	for _, dir := range []string{e.runtime.DistributionSuppliedIPSets, e.runtime.AdminSuppliedIPSets, e.runtime.UserSuppliedIPSets} {
		extra, err := config.LoadDirectory(dir)
		if err != nil {
			e.logger.Error("config reload: failed to load supplemental dir", "dir", dir, "error", err)
			e.mu.Lock()
			e.configReloadCount++
			e.lastConfigReloadError = err.Error()
			e.mu.Unlock()
			return err
		}
		cfg.Merge(extra)
	}
	if err := config.Validate(cfg); err != nil {
		e.logger.Error("config reload: validation failed", "error", err)
		e.mu.Lock()
		e.configReloadCount++
		e.lastConfigReloadError = err.Error()
		e.mu.Unlock()
		return err
	}
	rt, err := resolveRuntime(cfg, e.now().UTC())
	if err != nil {
		e.mu.Lock()
		e.configReloadCount++
		e.lastConfigReloadError = err.Error()
		e.mu.Unlock()
		return err
	}
	rt.ConfigPath = e.runtime.ConfigPath
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cfg = cfg
	e.runtime = rt
	e.applyRuntimeOverridesLocked()
	e.downloads = downloader.New(rt.MaxConnectTime, rt.MaxDownloadTime)
	if e.backgroundLimiter == nil {
		e.backgroundLimiter = newBackgroundLimiter(rt.BackgroundWorkers())
	} else {
		e.backgroundLimiter.SetLimit(rt.BackgroundWorkers())
	}
	e.geoProviders = newGeoProviderCache()
	e.asnLookupCache = newASNDatabaseCache()
	e.ledgerCache = newRuntimeLedgerCache()
	if err := e.ensureDirectories(); err != nil {
		e.configReloadCount++
		e.lastConfigReloadError = err.Error()
		return err
	}
	e.registerSyntheticInternalSources()
	e.reconcileEntriesFromSourceConfig()
	e.buildRetentionMaxWindow()
	if err := e.bootstrapMissingEntriesFromDisk(); err != nil {
		e.configReloadCount++
		e.lastConfigReloadError = err.Error()
		return err
	}
	if err := e.repairInvalidEntryTimestamps(); err != nil {
		e.configReloadCount++
		e.lastConfigReloadError = err.Error()
		return err
	}
	if err := e.bootstrapLegacyFailureStarts(); err != nil {
		e.configReloadCount++
		e.lastConfigReloadError = err.Error()
		return err
	}
	e.refreshCriticalInfrastructureProviderSetID()
	e.configReloadCount++
	e.lastConfigReload = e.now()
	e.lastConfigReloadError = ""
	if err := e.CleanupStaleCriticalInfrastructureArtifacts(); err != nil {
		e.lastConfigReloadError = err.Error()
		return err
	}
	return nil
}

func (e *Engine) AcquireLock() (*FileLock, error) {
	return acquireLock(e.runtime.LockFile)
}

func (e *Engine) Config() *config.Config {
	return e.cfg
}

func (e *Engine) Runtime() Runtime {
	return e.runtime
}

func (e *Engine) Enable(names []string, all bool) error {
	if all {
		// After config.ExpandDerivatives, every feed (including
		// merges and retention variants) is in cfg.Sources. The
		// old sortedMergeNames walk is redundant.
		names = config.SortedSourceNames(e.cfg)
	}
	for _, name := range names {
		if name == "" {
			continue
		}
		path := e.sourceEnablePath(name)
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
	if all {
		names = config.SortedArtifactNames(e.cfg)
	}
	for _, name := range names {
		if name == "" || !e.isArtifact(name) {
			continue
		}
		path := e.artifactEnablePath(name)
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
	if all {
		// After config.ExpandDerivatives, every feed (including
		// merges and retention variants) is in cfg.Sources. The
		// old sortedMergeNames walk is redundant.
		names = config.SortedSourceNames(e.cfg)
	}
	for _, name := range names {
		if name == "" {
			continue
		}
		if err := os.Remove(e.sourceEnablePath(name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (e *Engine) DisableArtifacts(names []string, all bool) error {
	if all {
		names = config.SortedArtifactNames(e.cfg)
	}
	for _, name := range names {
		if name == "" || !e.isArtifact(name) {
			continue
		}
		if err := os.Remove(e.artifactEnablePath(name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}
