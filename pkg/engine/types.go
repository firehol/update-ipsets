package engine

import (
	"time"

	"github.com/firehol/update-ipsets/internal/telemetry"
	"github.com/firehol/update-ipsets/pkg/feedhealth"
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
	// AsyncCachePersistence lets daemon scheduler runs return after the cache
	// save is accepted by the persistence worker. Direct callers leave this
	// false so one-shot runs do not exit before their cache state is durable.
	AsyncCachePersistence bool
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

type RunState string

const (
	RunStateIdle       RunState = "idle"
	RunStateRunning    RunState = "running"
	RunStateFinalizing RunState = "finalizing"
)

type CachePersistenceState string

const (
	CachePersistenceIdle    CachePersistenceState = "idle"
	CachePersistencePending CachePersistenceState = "pending"
	CachePersistenceSaving  CachePersistenceState = "saving"
	CachePersistenceFailed  CachePersistenceState = "failed"
	CachePersistenceStopped CachePersistenceState = "stopped"
)

type CachePersistenceSnapshot struct {
	State       CachePersistenceState `json:"state"`
	Pending     bool                  `json:"pending,omitempty"`
	Saving      bool                  `json:"saving,omitempty"`
	LastStarted time.Time             `json:"last_started,omitempty"`
	LastSaved   time.Time             `json:"last_saved,omitempty"`
	LastError   string                `json:"last_error,omitempty"`
	Accepted    uint64                `json:"accepted,omitempty"`
	Completed   uint64                `json:"completed,omitempty"`
	Failed      uint64                `json:"failed,omitempty"`
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
// the net delta and the true churn are visible at a glance - the net can be
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
	// family as the containing feed - retention variants of the
	// same parent, merges whose inputs include this feed,
	// variants of such inputs, or the primary source from which
	// this feed derives. The set is computed via leafAncestors()
	// in output.go: two feeds are "related" iff their leaf-
	// ancestor sets intersect.
	//
	// The public UI's overlap-facts tiles (UNIQUE, INCLUDED IN,
	// INCLUDES, >=50% OVERLAP) exclude related rows from their
	// math because a retention variant trivially overlaps its
	// parent 100% and a merge trivially contains every one of
	// its inputs - including those rows in the tiles makes the
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
	Running                      bool                         `json:"running"`
	RunState                     RunState                     `json:"run_state"`
	LastStarted                  time.Time                    `json:"last_started,omitempty"`
	LastEnded                    time.Time                    `json:"last_ended,omitempty"`
	LastError                    string                       `json:"last_error,omitempty"`
	LastReport                   *Report                      `json:"last_report,omitempty"`
	CurrentReason                runreason.Reason             `json:"current_reason,omitempty"`
	LastReason                   runreason.Reason             `json:"last_reason,omitempty"`
	CurrentPhase                 RunPhase                     `json:"current_phase,omitempty"`
	CurrentBatch                 *RunBatchSnapshot            `json:"current_batch,omitempty"`
	PhasePlan                    *RunPhasePlanSnapshot        `json:"phase_plan,omitempty"`
	ActiveFeeds                  []ActiveFeed                 `json:"active_feeds,omitempty"`
	ActiveOperations             []ActiveOperation            `json:"active_operations,omitempty"`
	BackgroundTasks              []BackgroundTaskSnapshot     `json:"background_tasks,omitempty"`
	EngineLane                   LaneSnapshot                 `json:"engine_lane"`
	GitLane                      LaneSnapshot                 `json:"git_lane"`
	CachePersistence             CachePersistenceSnapshot     `json:"cache_persistence"`
	PipelineIntegrityCache       PipelineIntegrityCacheStatus `json:"pipeline_integrity_cache"`
	EntityIntegrityCache         EntityIntegrityCacheStatus   `json:"entity_integrity_cache"`
	BackgroundLimit              int                          `json:"background_limit,omitempty"`
	BackgroundRunning            int                          `json:"background_running,omitempty"`
	CurrentMetrics               *RunMetricsSnapshot          `json:"current_metrics,omitempty"`
	LastMetrics                  *RunMetricsSnapshot          `json:"last_metrics,omitempty"`
	LifetimeMetrics              *LifetimeMetricsSnapshot     `json:"lifetime_metrics,omitempty"`
	ConfigPath                   string                       `json:"config_path"`
	BaseDir                      string                       `json:"base_dir"`
	MaxIngestWorkers             int                          `json:"max_ingest_workers,omitempty"`
	ParallelDownloads            int                          `json:"parallel_downloads"`
	ParallelDNSQueries           int                          `json:"parallel_dns_queries"`
	MaxProcessingWorkers         int                          `json:"max_processing_workers"`
	MaxHeavyPhaseWorkers         int                          `json:"max_heavy_phase_workers"`
	MaxBackgroundWorkers         int                          `json:"max_background_workers"`
	MaxEngineLaneWorkers         int                          `json:"max_engine_lane_workers,omitempty"`
	SourceCount                  int                          `json:"source_count"`
	MergeCount                   int                          `json:"merge_count"`
	EntityRefreshPending         int                          `json:"entity_refresh_pending,omitempty"`
	EntityHealthPending          int                          `json:"entity_health_pending,omitempty"`
	EntityRebuildPending         bool                         `json:"entity_rebuild_pending,omitempty"`
	LastConfigReload             time.Time                    `json:"last_config_reload,omitempty"`
	ConfigReloadCount            int                          `json:"config_reload_count,omitempty"`
	LastConfigReloadError        string                       `json:"last_config_reload_error,omitempty"`
	StartupRepairDeferred        bool                         `json:"startup_repair_deferred,omitempty"`
	StartupRepairDeferredTargets int                          `json:"startup_repair_deferred_targets,omitempty"`
}

type StatusSnapshotLight struct {
	Running                      bool                         `json:"running"`
	RunState                     RunState                     `json:"run_state"`
	LastStarted                  time.Time                    `json:"last_started,omitempty"`
	LastEnded                    time.Time                    `json:"last_ended,omitempty"`
	LastError                    string                       `json:"last_error,omitempty"`
	CurrentReason                runreason.Reason             `json:"current_reason,omitempty"`
	LastReason                   runreason.Reason             `json:"last_reason,omitempty"`
	CurrentPhase                 RunPhase                     `json:"current_phase,omitempty"`
	CurrentBatch                 *RunBatchSnapshot            `json:"current_batch,omitempty"`
	PhasePlan                    *RunPhasePlanSnapshot        `json:"phase_plan,omitempty"`
	ActiveFeeds                  []ActiveFeed                 `json:"active_feeds,omitempty"`
	ActiveOperations             []ActiveOperation            `json:"active_operations,omitempty"`
	BackgroundTasks              []BackgroundTaskSnapshot     `json:"background_tasks,omitempty"`
	EngineLane                   LaneSnapshot                 `json:"engine_lane"`
	GitLane                      LaneSnapshot                 `json:"git_lane"`
	CachePersistence             CachePersistenceSnapshot     `json:"cache_persistence"`
	PipelineIntegrityCache       PipelineIntegrityCacheStatus `json:"pipeline_integrity_cache"`
	EntityIntegrityCache         EntityIntegrityCacheStatus   `json:"entity_integrity_cache"`
	BackgroundLimit              int                          `json:"background_limit,omitempty"`
	BackgroundRunning            int                          `json:"background_running,omitempty"`
	MaxIngestWorkers             int                          `json:"max_ingest_workers,omitempty"`
	ParallelDownloads            int                          `json:"parallel_downloads"`
	ParallelDNSQueries           int                          `json:"parallel_dns_queries"`
	MaxProcessingWorkers         int                          `json:"max_processing_workers"`
	MaxHeavyPhaseWorkers         int                          `json:"max_heavy_phase_workers"`
	MaxBackgroundWorkers         int                          `json:"max_background_workers"`
	MaxEngineLaneWorkers         int                          `json:"max_engine_lane_workers,omitempty"`
	SourceCount                  int                          `json:"source_count"`
	MergeCount                   int                          `json:"merge_count"`
	EntityRefreshPending         int                          `json:"entity_refresh_pending,omitempty"`
	EntityHealthPending          int                          `json:"entity_health_pending,omitempty"`
	EntityRebuildPending         bool                         `json:"entity_rebuild_pending,omitempty"`
	LastConfigReload             time.Time                    `json:"last_config_reload,omitempty"`
	ConfigReloadCount            int                          `json:"config_reload_count,omitempty"`
	LastConfigReloadError        string                       `json:"last_config_reload_error,omitempty"`
	StartupRepairDeferred        bool                         `json:"startup_repair_deferred,omitempty"`
	StartupRepairDeferredTargets int                          `json:"startup_repair_deferred_targets,omitempty"`
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

type RunBatchSnapshot struct {
	Total            int       `json:"total"`
	Completed        int       `json:"completed"`
	Active           int       `json:"active"`
	Pending          int       `json:"pending"`
	Names            []string  `json:"names,omitempty"`
	CompletedNames   []string  `json:"completed_names,omitempty"`
	ActiveNames      []string  `json:"active_names,omitempty"`
	PendingNames     []string  `json:"pending_names,omitempty"`
	SourceTotal      int       `json:"source_total,omitempty"`
	SourceCompleted  int       `json:"source_completed,omitempty"`
	HistoryTotal     int       `json:"history_total,omitempty"`
	HistoryCompleted int       `json:"history_completed,omitempty"`
	MergeTotal       int       `json:"merge_total,omitempty"`
	MergeCompleted   int       `json:"merge_completed,omitempty"`
	StartedAt        time.Time `json:"started_at,omitempty"`
}

type RunPhasePlanSnapshot struct {
	Phases          []RunPhase `json:"phases,omitempty"`
	Current         RunPhase   `json:"current,omitempty"`
	CurrentPosition int        `json:"current_position,omitempty"`
	Total           int        `json:"total,omitempty"`
	Final           bool       `json:"final"`
}

type ActiveOperation struct {
	Operation     string           `json:"operation"`
	Phase         RunPhase         `json:"phase,omitempty"`
	Feed          string           `json:"feed,omitempty"`
	Stage         string           `json:"stage,omitempty"`
	Unit          string           `json:"unit"`
	StartedAt     time.Time        `json:"started_at,omitempty"`
	ElapsedMS     int64            `json:"elapsed_ms"`
	Current       int64            `json:"current"`
	Total         int64            `json:"total"`
	CompletionPct int              `json:"completion_pct"`
	RatePerSecond float64          `json:"rate_per_second"`
	Counters      map[string]int64 `json:"counters,omitempty"`
}
