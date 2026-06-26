package engine

import "time"

func (e *Engine) StatusSnapshot() StatusSnapshot {
	return e.statusSnapshot()
}

func (e *Engine) StatusSnapshotLight() StatusSnapshotLight {
	engineLane := LaneSnapshot{}
	if e.engineLane != nil {
		engineLane = e.attachEngineLaneWarning(e.engineLane.Snapshot())
	}
	gitLane := LaneSnapshot{}
	if e.gitLane != nil {
		gitLane = e.gitLane.Snapshot()
	}
	pipelineIntegrityCache, entityIntegrityCache := e.IntegrityCacheStatus()
	now := time.Now().UTC()
	cachePersistence := e.cachePersistenceSnapshot()
	e.mu.RLock()
	runState := normalizeRunStateLocked(e.runState, e.running)
	running := runState != RunStateIdle
	lastStarted := e.lastStarted
	lastEnded := e.lastEnded
	lastError := e.lastError
	currentReason := e.currentReason
	lastReason := e.lastReason
	currentPhase := e.currentPhase
	currentBatch := e.copyRunBatchLocked()
	phasePlan := e.snapshotRunPhasePlanLocked()
	maxIngestWorkers := e.runtime.MaxIngestWorkers
	parallelDownloads := e.runtime.ParallelDownloads
	parallelDNSQueries := e.runtime.ParallelDNSQueries
	maxProcessingWorkers := e.runtime.MaxProcessingWorkers
	maxHeavyPhaseWorkers := e.runtime.HeavyPhaseWorkers()
	maxBackgroundWorkers := e.runtime.BackgroundWorkers()
	maxEngineLaneWorkers := e.runtime.EngineLaneWorkers()
	sourceCount := len(e.cfg.Sources)
	mergeCount := mergeCountForConfig(e.cfg)
	entityRefreshPending := len(e.entityRefreshPending)
	entityHealthPending := len(e.entityHealthPending)
	entityRebuildPending := e.entityRebuildQueued
	lastConfigReload := e.lastConfigReload
	configReloadCount := e.configReloadCount
	lastConfigReloadError := e.lastConfigReloadError
	startupRepairDeferred := e.startupRepairDeferred
	startupRepairDeferredTargets := e.startupRepairDeferredTargets
	e.mu.RUnlock()
	activeFeeds := e.snapshotActiveFeeds()
	return StatusSnapshotLight{
		Running:                      running,
		RunState:                     runState,
		LastStarted:                  lastStarted,
		LastEnded:                    lastEnded,
		LastError:                    lastError,
		CurrentReason:                currentReason,
		LastReason:                   lastReason,
		CurrentPhase:                 currentPhase,
		CurrentBatch:                 snapshotRunBatch(currentBatch, activeFeeds),
		PhasePlan:                    phasePlan,
		ActiveFeeds:                  activeFeeds,
		ActiveOperations:             e.snapshotActiveOperations(now),
		BackgroundTasks:              e.snapshotBackgroundTasks(),
		EngineLane:                   engineLane,
		GitLane:                      gitLane,
		CachePersistence:             cachePersistence,
		PipelineIntegrityCache:       pipelineIntegrityCache,
		EntityIntegrityCache:         entityIntegrityCache,
		BackgroundLimit:              engineLane.Limit,
		BackgroundRunning:            engineLane.ActiveCount,
		MaxIngestWorkers:             maxIngestWorkers,
		ParallelDownloads:            parallelDownloads,
		ParallelDNSQueries:           parallelDNSQueries,
		MaxProcessingWorkers:         maxProcessingWorkers,
		MaxHeavyPhaseWorkers:         maxHeavyPhaseWorkers,
		MaxBackgroundWorkers:         maxBackgroundWorkers,
		MaxEngineLaneWorkers:         maxEngineLaneWorkers,
		SourceCount:                  sourceCount,
		MergeCount:                   mergeCount,
		EntityRefreshPending:         entityRefreshPending,
		EntityHealthPending:          entityHealthPending,
		EntityRebuildPending:         entityRebuildPending,
		LastConfigReload:             lastConfigReload,
		ConfigReloadCount:            configReloadCount,
		LastConfigReloadError:        lastConfigReloadError,
		StartupRepairDeferred:        startupRepairDeferred,
		StartupRepairDeferredTargets: startupRepairDeferredTargets,
	}
}

func (e *Engine) statusSnapshot() StatusSnapshot {
	engineLane := LaneSnapshot{}
	if e.engineLane != nil {
		engineLane = e.attachEngineLaneWarning(e.engineLane.Snapshot())
	}
	gitLane := LaneSnapshot{}
	if e.gitLane != nil {
		gitLane = e.gitLane.Snapshot()
	}
	pipelineIntegrityCache, entityIntegrityCache := e.IntegrityCacheStatus()
	now := time.Now().UTC()
	cachePersistence := e.cachePersistenceSnapshot()
	e.mu.RLock()
	runState := normalizeRunStateLocked(e.runState, e.running)
	running := runState != RunStateIdle
	lastStarted := e.lastStarted
	lastEnded := e.lastEnded
	lastError := e.lastError
	lastReport := e.lastReport
	currentReason := e.currentReason
	lastReason := e.lastReason
	currentPhase := e.currentPhase
	currentBatch := e.copyRunBatchLocked()
	phasePlan := e.snapshotRunPhasePlanLocked()
	var lastMetrics *RunMetricsSnapshot
	if e.lastMetrics != nil {
		snap := *e.lastMetrics
		lastMetrics = &snap
	}
	configPath := e.runtime.ConfigPath
	baseDir := e.runtime.BaseDir
	maxIngestWorkers := e.runtime.MaxIngestWorkers
	parallelDownloads := e.runtime.ParallelDownloads
	parallelDNSQueries := e.runtime.ParallelDNSQueries
	maxProcessingWorkers := e.runtime.MaxProcessingWorkers
	maxHeavyPhaseWorkers := e.runtime.HeavyPhaseWorkers()
	maxBackgroundWorkers := e.runtime.BackgroundWorkers()
	maxEngineLaneWorkers := e.runtime.EngineLaneWorkers()
	sourceCount := len(e.cfg.Sources)
	mergeCount := mergeCountForConfig(e.cfg)
	entityRefreshPending := len(e.entityRefreshPending)
	entityHealthPending := len(e.entityHealthPending)
	entityRebuildPending := e.entityRebuildQueued
	lastConfigReload := e.lastConfigReload
	configReloadCount := e.configReloadCount
	lastConfigReloadError := e.lastConfigReloadError
	startupRepairDeferred := e.startupRepairDeferred
	startupRepairDeferredTargets := e.startupRepairDeferredTargets
	e.mu.RUnlock()
	var currentMetrics *RunMetricsSnapshot
	if current := e.currentRunMetrics(); current != nil {
		snap := current.snapshot(true)
		currentMetrics = &snap
	}
	activeFeeds := e.snapshotActiveFeeds()
	backgroundLimit, backgroundRunning := engineLane.Limit, engineLane.ActiveCount
	lifetimeMetrics := e.lifetimeMetricsSnapshot()
	return StatusSnapshot{
		Running:                      running,
		RunState:                     runState,
		LastStarted:                  lastStarted,
		LastEnded:                    lastEnded,
		LastError:                    lastError,
		LastReport:                   lastReport,
		CurrentReason:                currentReason,
		LastReason:                   lastReason,
		CurrentPhase:                 currentPhase,
		CurrentBatch:                 snapshotRunBatch(currentBatch, activeFeeds),
		PhasePlan:                    phasePlan,
		ActiveFeeds:                  activeFeeds,
		ActiveOperations:             e.snapshotActiveOperations(now),
		BackgroundTasks:              e.snapshotBackgroundTasks(),
		EngineLane:                   engineLane,
		GitLane:                      gitLane,
		CachePersistence:             cachePersistence,
		PipelineIntegrityCache:       pipelineIntegrityCache,
		EntityIntegrityCache:         entityIntegrityCache,
		BackgroundLimit:              backgroundLimit,
		BackgroundRunning:            backgroundRunning,
		CurrentMetrics:               currentMetrics,
		LastMetrics:                  lastMetrics,
		LifetimeMetrics:              lifetimeMetrics,
		ConfigPath:                   configPath,
		BaseDir:                      baseDir,
		MaxIngestWorkers:             maxIngestWorkers,
		ParallelDownloads:            parallelDownloads,
		ParallelDNSQueries:           parallelDNSQueries,
		MaxProcessingWorkers:         maxProcessingWorkers,
		MaxHeavyPhaseWorkers:         maxHeavyPhaseWorkers,
		MaxBackgroundWorkers:         maxBackgroundWorkers,
		MaxEngineLaneWorkers:         maxEngineLaneWorkers,
		SourceCount:                  sourceCount,
		MergeCount:                   mergeCount,
		EntityRefreshPending:         entityRefreshPending,
		EntityHealthPending:          entityHealthPending,
		EntityRebuildPending:         entityRebuildPending,
		LastConfigReload:             lastConfigReload,
		ConfigReloadCount:            configReloadCount,
		LastConfigReloadError:        lastConfigReloadError,
		StartupRepairDeferred:        startupRepairDeferred,
		StartupRepairDeferredTargets: startupRepairDeferredTargets,
	}
}

func normalizeRunStateLocked(state RunState, running bool) RunState {
	switch state {
	case RunStateRunning, RunStateFinalizing, RunStateIdle:
		return state
	default:
		if running {
			return RunStateRunning
		}
		return RunStateIdle
	}
}
