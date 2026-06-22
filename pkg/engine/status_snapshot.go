package engine

import "time"

func (e *Engine) StatusSnapshot() StatusSnapshot {
	return e.statusSnapshot()
}

func (e *Engine) StatusSnapshotLight() StatusSnapshotLight {
	engineLane := LaneSnapshot{}
	if e.engineLane != nil {
		engineLane = e.engineLane.Snapshot()
	}
	pipelineIntegrityCache, entityIntegrityCache := e.IntegrityCacheStatus()
	e.mu.RLock()
	defer e.mu.RUnlock()
	return StatusSnapshotLight{
		Running:                      e.running,
		LastStarted:                  e.lastStarted,
		LastEnded:                    e.lastEnded,
		LastError:                    e.lastError,
		LastReport:                   e.lastReport,
		CurrentReason:                e.currentReason,
		LastReason:                   e.lastReason,
		CurrentPhase:                 e.currentPhase,
		CurrentBatch:                 e.snapshotRunBatchLocked(),
		PhasePlan:                    e.snapshotRunPhasePlanLocked(),
		ActiveFeeds:                  e.snapshotActiveFeedsLocked(),
		ActiveOperations:             e.snapshotActiveOperationsLocked(time.Now().UTC()),
		BackgroundTasks:              e.snapshotBackgroundTasksLocked(),
		EngineLane:                   engineLane,
		PipelineIntegrityCache:       pipelineIntegrityCache,
		EntityIntegrityCache:         entityIntegrityCache,
		BackgroundLimit:              engineLane.Limit,
		BackgroundRunning:            engineLane.ActiveCount,
		MaxIngestWorkers:             e.runtime.MaxIngestWorkers,
		ParallelDownloads:            e.runtime.ParallelDownloads,
		ParallelDNSQueries:           e.runtime.ParallelDNSQueries,
		MaxProcessingWorkers:         e.runtime.MaxProcessingWorkers,
		MaxHeavyPhaseWorkers:         e.runtime.HeavyPhaseWorkers(),
		MaxBackgroundWorkers:         e.runtime.BackgroundWorkers(),
		MaxEngineLaneWorkers:         e.runtime.EngineLaneWorkers(),
		SourceCount:                  len(e.cfg.Sources),
		MergeCount:                   e.mergeCount(),
		EntityRefreshPending:         len(e.entityRefreshPending),
		EntityHealthPending:          len(e.entityHealthPending),
		EntityRebuildPending:         e.entityRebuildQueued,
		LastConfigReload:             e.lastConfigReload,
		ConfigReloadCount:            e.configReloadCount,
		LastConfigReloadError:        e.lastConfigReloadError,
		StartupRepairDeferred:        e.startupRepairDeferred,
		StartupRepairDeferredTargets: e.startupRepairDeferredTargets,
	}
}

func (e *Engine) statusSnapshot() StatusSnapshot {
	engineLane := LaneSnapshot{}
	if e.engineLane != nil {
		engineLane = e.engineLane.Snapshot()
	}
	pipelineIntegrityCache, entityIntegrityCache := e.IntegrityCacheStatus()
	e.mu.RLock()
	defer e.mu.RUnlock()
	var currentMetrics *RunMetricsSnapshot
	if e.currentMetrics != nil {
		snap := e.currentMetrics.snapshot(true)
		currentMetrics = &snap
	}
	var lastMetrics *RunMetricsSnapshot
	if e.lastMetrics != nil {
		snap := *e.lastMetrics
		lastMetrics = &snap
	}
	backgroundLimit, backgroundRunning := engineLane.Limit, engineLane.ActiveCount
	var lifetimeMetrics *LifetimeMetricsSnapshot
	lifetimeMetrics = e.lifetimeMetricsSnapshot()
	return StatusSnapshot{
		Running:                      e.running,
		LastStarted:                  e.lastStarted,
		LastEnded:                    e.lastEnded,
		LastError:                    e.lastError,
		LastReport:                   e.lastReport,
		CurrentReason:                e.currentReason,
		LastReason:                   e.lastReason,
		CurrentPhase:                 e.currentPhase,
		CurrentBatch:                 e.snapshotRunBatchLocked(),
		PhasePlan:                    e.snapshotRunPhasePlanLocked(),
		ActiveFeeds:                  e.snapshotActiveFeedsLocked(),
		ActiveOperations:             e.snapshotActiveOperationsLocked(time.Now().UTC()),
		BackgroundTasks:              e.snapshotBackgroundTasksLocked(),
		EngineLane:                   engineLane,
		PipelineIntegrityCache:       pipelineIntegrityCache,
		EntityIntegrityCache:         entityIntegrityCache,
		BackgroundLimit:              backgroundLimit,
		BackgroundRunning:            backgroundRunning,
		CurrentMetrics:               currentMetrics,
		LastMetrics:                  lastMetrics,
		LifetimeMetrics:              lifetimeMetrics,
		ConfigPath:                   e.runtime.ConfigPath,
		BaseDir:                      e.runtime.BaseDir,
		MaxIngestWorkers:             e.runtime.MaxIngestWorkers,
		ParallelDownloads:            e.runtime.ParallelDownloads,
		ParallelDNSQueries:           e.runtime.ParallelDNSQueries,
		MaxProcessingWorkers:         e.runtime.MaxProcessingWorkers,
		MaxHeavyPhaseWorkers:         e.runtime.HeavyPhaseWorkers(),
		MaxBackgroundWorkers:         e.runtime.BackgroundWorkers(),
		MaxEngineLaneWorkers:         e.runtime.EngineLaneWorkers(),
		SourceCount:                  len(e.cfg.Sources),
		MergeCount:                   e.mergeCount(),
		EntityRefreshPending:         len(e.entityRefreshPending),
		EntityHealthPending:          len(e.entityHealthPending),
		EntityRebuildPending:         e.entityRebuildQueued,
		LastConfigReload:             e.lastConfigReload,
		ConfigReloadCount:            e.configReloadCount,
		LastConfigReloadError:        e.lastConfigReloadError,
		StartupRepairDeferred:        e.startupRepairDeferred,
		StartupRepairDeferredTargets: e.startupRepairDeferredTargets,
	}
}
