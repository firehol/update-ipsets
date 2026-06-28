package engine

import "time"

func (e *Engine) StatusSnapshot() StatusSnapshot {
	return e.statusSnapshot()
}

func (e *Engine) TryStatusSnapshot() (StatusSnapshot, bool) {
	return e.statusSnapshotBestEffort(false)
}

func (e *Engine) StatusSnapshotLight() StatusSnapshotLight {
	snapshot, _ := e.statusSnapshotLightBestEffort(true)
	return snapshot
}

func (e *Engine) TryStatusSnapshotLight() (StatusSnapshotLight, bool) {
	return e.statusSnapshotLightBestEffort(false)
}

func (e *Engine) statusSnapshotLightBestEffort(wait bool) (StatusSnapshotLight, bool) {
	engineLane, gitLane, ok := e.statusLaneSnapshots(wait)
	now := time.Now().UTC()
	cachePersistence, cacheOK := e.statusCachePersistenceSnapshot(wait)
	ok = ok && cacheOK
	if wait {
		e.mu.RLock()
	} else if !e.mu.TryRLock() {
		return StatusSnapshotLight{
			Running:                engineLane.ActiveCount > 0 || engineLane.WaitingCount > 0,
			EngineLane:             engineLane,
			GitLane:                gitLane,
			CachePersistence:       cachePersistence,
			PipelineIntegrityCache: PipelineIntegrityCacheStatus{CacheState: IntegrityCacheCold},
			EntityIntegrityCache:   EntityIntegrityCacheStatus{CacheState: IntegrityCacheCold},
			BackgroundLimit:        engineLane.Limit,
			BackgroundRunning:      engineLane.ActiveCount,
		}, false
	}
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
	integrityOpts := IntegrityOptions{WebDir: e.runtime.WebDir}
	lastConfigReload := e.lastConfigReload
	configReloadCount := e.configReloadCount
	lastConfigReloadError := e.lastConfigReloadError
	startupRepairDeferred := e.startupRepairDeferred
	startupRepairDeferredTargets := e.startupRepairDeferredTargets
	e.mu.RUnlock()
	pipelineIntegrityCache, entityIntegrityCache, integrityOK := e.statusIntegrityCacheSnapshots(integrityOpts, wait)
	activeFeeds, activeFeedsOK := e.statusActiveFeeds(wait)
	activeOperations, activeOperationsOK := e.statusActiveOperations(wait, now)
	backgroundTasks, backgroundTasksOK := e.statusBackgroundTasks(wait)
	ok = ok && integrityOK && activeFeedsOK && activeOperationsOK && backgroundTasksOK
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
		ActiveOperations:             activeOperations,
		BackgroundTasks:              backgroundTasks,
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
	}, ok
}

func (e *Engine) statusSnapshot() StatusSnapshot {
	snapshot, _ := e.statusSnapshotBestEffort(true)
	return snapshot
}

func (e *Engine) statusSnapshotBestEffort(wait bool) (StatusSnapshot, bool) {
	engineLane, gitLane, ok := e.statusLaneSnapshots(wait)
	now := time.Now().UTC()
	cachePersistence, cacheOK := e.statusCachePersistenceSnapshot(wait)
	ok = ok && cacheOK
	if wait {
		e.mu.RLock()
	} else if !e.mu.TryRLock() {
		return StatusSnapshot{
			Running:                engineLane.ActiveCount > 0 || engineLane.WaitingCount > 0,
			EngineLane:             engineLane,
			GitLane:                gitLane,
			CachePersistence:       cachePersistence,
			PipelineIntegrityCache: PipelineIntegrityCacheStatus{CacheState: IntegrityCacheCold},
			EntityIntegrityCache:   EntityIntegrityCacheStatus{CacheState: IntegrityCacheCold},
			BackgroundLimit:        engineLane.Limit,
			BackgroundRunning:      engineLane.ActiveCount,
		}, false
	}
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
	integrityOpts := IntegrityOptions{WebDir: e.runtime.WebDir}
	lastConfigReload := e.lastConfigReload
	configReloadCount := e.configReloadCount
	lastConfigReloadError := e.lastConfigReloadError
	startupRepairDeferred := e.startupRepairDeferred
	startupRepairDeferredTargets := e.startupRepairDeferredTargets
	e.mu.RUnlock()
	pipelineIntegrityCache, entityIntegrityCache, integrityOK := e.statusIntegrityCacheSnapshots(integrityOpts, wait)
	var currentMetrics *RunMetricsSnapshot
	if current := e.currentRunMetrics(); current != nil {
		if snap, ok := current.trySnapshot(true); ok {
			currentMetrics = &snap
		}
	}
	activeFeeds, activeFeedsOK := e.statusActiveFeeds(wait)
	activeOperations, activeOperationsOK := e.statusActiveOperations(wait, now)
	backgroundTasks, backgroundTasksOK := e.statusBackgroundTasks(wait)
	ok = ok && integrityOK && activeFeedsOK && activeOperationsOK && backgroundTasksOK
	backgroundLimit, backgroundRunning := engineLane.Limit, engineLane.ActiveCount
	lifetimeMetrics := e.tryLifetimeMetricsSnapshot()
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
		ActiveOperations:             activeOperations,
		BackgroundTasks:              backgroundTasks,
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
	}, ok
}

func (e *Engine) statusLaneSnapshots(wait bool) (LaneSnapshot, LaneSnapshot, bool) {
	ok := true
	engineLane := LaneSnapshot{}
	if e.engineLane != nil {
		if wait {
			engineLane = e.attachEngineLaneWarning(e.engineLane.Snapshot())
		} else if snap, snapOK := e.engineLane.TrySnapshot(); snapOK {
			var warningOK bool
			engineLane, warningOK = e.tryAttachEngineLaneWarning(snap)
			ok = ok && warningOK
		} else {
			ok = false
		}
	}
	gitLane := LaneSnapshot{}
	if e.gitLane != nil {
		if wait {
			gitLane = e.gitLane.Snapshot()
		} else if snap, snapOK := e.gitLane.TrySnapshot(); snapOK {
			gitLane = snap
		} else {
			ok = false
		}
	}
	return engineLane, gitLane, ok
}

func (e *Engine) statusCachePersistenceSnapshot(wait bool) (CachePersistenceSnapshot, bool) {
	if wait {
		return e.cachePersistenceSnapshot(), true
	}
	return e.tryCachePersistenceSnapshot()
}

func (e *Engine) statusIntegrityCacheSnapshots(opts IntegrityOptions, wait bool) (PipelineIntegrityCacheStatus, EntityIntegrityCacheStatus, bool) {
	if wait {
		pipeline, entity := e.integrityCacheStatusForNormalizedOptions(opts)
		return pipeline, entity, true
	}
	return e.tryIntegrityCacheStatusForNormalizedOptions(opts)
}

func (e *Engine) statusActiveFeeds(wait bool) ([]ActiveFeed, bool) {
	if wait {
		return e.snapshotActiveFeeds(), true
	}
	return e.trySnapshotActiveFeeds()
}

func (e *Engine) statusActiveOperations(wait bool, now time.Time) ([]ActiveOperation, bool) {
	if wait {
		return e.snapshotActiveOperations(now), true
	}
	return e.trySnapshotActiveOperations(now)
}

func (e *Engine) statusBackgroundTasks(wait bool) ([]BackgroundTaskSnapshot, bool) {
	if wait {
		return e.snapshotBackgroundTasks(), true
	}
	return e.trySnapshotBackgroundTasks()
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
