package engine

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/firehol/update-ipsets/internal/runtimeinfo"
	"github.com/firehol/update-ipsets/pkg/runreason"
)

const (
	engineProgressLogInterval        = time.Minute
	engineRuntimeStatsSampleInterval = 5 * time.Second
)

var (
	engineRuntimeStatsCaptureMu sync.Mutex
	engineRuntimeStatsCapture   = runtimeinfo.Capture
)

func setEngineRuntimeStatsCaptureForTest(fn func() runtimeinfo.Snapshot) func() {
	engineRuntimeStatsCaptureMu.Lock()
	old := engineRuntimeStatsCapture
	if fn == nil {
		engineRuntimeStatsCapture = runtimeinfo.Capture
	} else {
		engineRuntimeStatsCapture = fn
	}
	engineRuntimeStatsCaptureMu.Unlock()
	return func() {
		engineRuntimeStatsCaptureMu.Lock()
		engineRuntimeStatsCapture = old
		engineRuntimeStatsCaptureMu.Unlock()
	}
}

func engineRuntimeStatsCaptureSnapshot() func() runtimeinfo.Snapshot {
	engineRuntimeStatsCaptureMu.Lock()
	defer engineRuntimeStatsCaptureMu.Unlock()
	return engineRuntimeStatsCapture
}

type engineRunDiagnostics struct {
	reason     runreason.Reason
	selected   int
	recheck    bool
	reprocess  bool
	manual     bool
	startedAt  time.Time
	startStats engineRuntimeStats
}

type engineRuntimeStats struct {
	Goroutines              int    `json:"goroutines"`
	HeapAlloc               uint64 `json:"heap_alloc"`
	HeapSys                 uint64 `json:"heap_sys"`
	HeapInuse               uint64 `json:"heap_inuse"`
	HeapIdle                uint64 `json:"heap_idle"`
	HeapReleased            uint64 `json:"heap_released"`
	HeapObjects             uint64 `json:"heap_objects"`
	NumGC                   uint32 `json:"num_gc"`
	PauseTotalMS            int64  `json:"pause_total_ms"`
	GoMemLimit              int64  `json:"go_mem_limit"`
	RSSKB                   uint64 `json:"rss_kb,omitempty"`
	VMSKB                   uint64 `json:"vms_kb,omitempty"`
	DataKB                  uint64 `json:"data_kb,omitempty"`
	CPUUserMS               int64  `json:"cpu_user_ms,omitempty"`
	CPUSystemMS             int64  `json:"cpu_system_ms,omitempty"`
	CPUTotalMS              int64  `json:"cpu_total_ms,omitempty"`
	ProcReadBytes           uint64 `json:"proc_read_bytes,omitempty"`
	ProcWriteBytes          uint64 `json:"proc_write_bytes,omitempty"`
	ProcCancelledWriteBytes uint64 `json:"proc_cancelled_write_bytes,omitempty"`
	ProcReadSyscalls        uint64 `json:"proc_read_syscalls,omitempty"`
	ProcWriteSyscalls       uint64 `json:"proc_write_syscalls,omitempty"`
	OpenFDs                 int    `json:"open_fds,omitempty"`
}

type engineRuntimeDelta = runtimeinfo.Delta

func engineRuntimeStatsFromSample(sample runtimeinfo.Snapshot) engineRuntimeStats {
	return engineRuntimeStats{
		Goroutines:              sample.Goroutines,
		HeapAlloc:               sample.HeapAlloc,
		HeapSys:                 sample.HeapSys,
		HeapInuse:               sample.HeapInuse,
		HeapIdle:                sample.HeapIdle,
		HeapReleased:            sample.HeapReleased,
		HeapObjects:             sample.HeapObjects,
		NumGC:                   sample.NumGC,
		PauseTotalMS:            sample.PauseTotalMS,
		GoMemLimit:              sample.GoMemLimit,
		RSSKB:                   sample.RSSKB,
		VMSKB:                   sample.VMSKB,
		DataKB:                  sample.DataKB,
		CPUUserMS:               sample.CPUUserMS,
		CPUSystemMS:             sample.CPUSystemMS,
		CPUTotalMS:              sample.CPUTotalMS,
		ProcReadBytes:           sample.ProcReadBytes,
		ProcWriteBytes:          sample.ProcWriteBytes,
		ProcCancelledWriteBytes: sample.ProcCancelledWriteBytes,
		ProcReadSyscalls:        sample.ProcReadSyscalls,
		ProcWriteSyscalls:       sample.ProcWriteSyscalls,
		OpenFDs:                 sample.OpenFDs,
	}
}

func (stats engineRuntimeStats) runtimeInfoSnapshot() runtimeinfo.Snapshot {
	return runtimeinfo.Snapshot{
		Goroutines:              stats.Goroutines,
		HeapAlloc:               stats.HeapAlloc,
		HeapSys:                 stats.HeapSys,
		HeapInuse:               stats.HeapInuse,
		HeapIdle:                stats.HeapIdle,
		HeapReleased:            stats.HeapReleased,
		HeapObjects:             stats.HeapObjects,
		NumGC:                   stats.NumGC,
		PauseTotalMS:            stats.PauseTotalMS,
		GoMemLimit:              stats.GoMemLimit,
		RSSKB:                   stats.RSSKB,
		VMSKB:                   stats.VMSKB,
		DataKB:                  stats.DataKB,
		CPUUserMS:               stats.CPUUserMS,
		CPUSystemMS:             stats.CPUSystemMS,
		CPUTotalMS:              stats.CPUTotalMS,
		ProcReadBytes:           stats.ProcReadBytes,
		ProcWriteBytes:          stats.ProcWriteBytes,
		ProcCancelledWriteBytes: stats.ProcCancelledWriteBytes,
		ProcReadSyscalls:        stats.ProcReadSyscalls,
		ProcWriteSyscalls:       stats.ProcWriteSyscalls,
		OpenFDs:                 stats.OpenFDs,
	}
}

type activeOperationHandle struct {
	e  *Engine
	id string
}

func (e *Engine) newEngineRunDiagnostics(reason runreason.Reason, opts RunOptions, startedAt time.Time) engineRunDiagnostics {
	return engineRunDiagnostics{
		reason:     reason,
		selected:   len(opts.Selected),
		recheck:    opts.Recheck,
		reprocess:  opts.Reprocess,
		manual:     opts.Manual,
		startedAt:  startedAt,
		startStats: e.cachedEngineRuntimeStats(),
	}
}

func (e *Engine) startRunProgressLogger(ctx context.Context, diag engineRunDiagnostics) func() {
	if e == nil || e.logger == nil {
		return func() {}
	}
	ctx = nonNilContext(ctx)
	done := make(chan struct{})
	stopped := make(chan struct{})
	var once sync.Once
	go func() {
		defer close(stopped)
		timer := time.NewTimer(engineProgressLogInterval)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-timer.C:
				e.logRunProgress(diag)
				timer.Reset(engineProgressLogInterval)
			}
		}
	}()
	return func() {
		once.Do(func() {
			close(done)
			<-stopped
		})
	}
}

func (e *Engine) logRunProgress(diag engineRunDiagnostics) {
	if e == nil || e.logger == nil {
		return
	}
	metrics, activeFeeds, activeOps, phase := e.currentRunDiagnosticsSnapshot()
	now := time.Now()
	stats := e.cachedEngineRuntimeStats()
	e.logger.Info("engine run progress",
		"reason", diag.reason.String(),
		"selected", diag.selected,
		"recheck", diag.recheck,
		"reprocess", diag.reprocess,
		"manual", diag.manual,
		"elapsed_ms", telemetryDurationMillis(now.Sub(diag.startedAt)),
		"phase", phase,
		"active_feeds", activeFeeds,
		"active_operations", activeOps,
		"phase_times", metrics.PhaseTimes,
		"phases", metrics.Phases,
		"feeds", metrics.Feeds,
		"operations", metrics.Operations,
		"counters", metrics.Counters,
		"slow_feeds", metrics.SlowFeeds,
		"runtime", stats,
		"runtime_delta", diffEngineRuntimeStats(diag.startStats, stats),
	)
}

func (e *Engine) logRunDiagnosticSummary(report *Report, runErr error, diag engineRunDiagnostics) {
	if e == nil || e.logger == nil || report == nil {
		return
	}
	metrics, activeFeeds, activeOps, phase := e.currentRunDiagnosticsSnapshot()
	e.logRunDiagnosticSummarySnapshot(report, runErr, diag, metrics, activeFeeds, activeOps, phase)
}

func (e *Engine) logRunDiagnosticSummarySnapshot(report *Report, runErr error, diag engineRunDiagnostics, metrics RunMetricsSnapshot, activeFeeds []ActiveFeed, activeOps []ActiveOperation, phase RunPhase) {
	if e == nil || e.logger == nil || report == nil {
		return
	}
	status := "ok"
	if runErr != nil {
		status = "error"
	}
	stats := e.cachedEngineRuntimeStats()
	e.logger.Info("engine run diagnostic summary",
		"reason", diag.reason.String(),
		"status", status,
		"selected", diag.selected,
		"recheck", diag.recheck,
		"reprocess", diag.reprocess,
		"manual", diag.manual,
		"updated", len(report.Updated),
		"skipped", len(report.Skipped),
		"failed", len(report.Failed),
		"elapsed_ms", telemetryDurationMillis(report.EndedAt.Sub(report.StartedAt)),
		"phase", phase,
		"active_feeds", activeFeeds,
		"active_operations", activeOps,
		"phase_times", metrics.PhaseTimes,
		"phases", metrics.Phases,
		"feeds", metrics.Feeds,
		"operations", metrics.Operations,
		"counters", metrics.Counters,
		"slow_feeds", metrics.SlowFeeds,
		"runtime", stats,
		"runtime_delta", diffEngineRuntimeStats(diag.startStats, stats),
	)
}

func (e *Engine) logRunPhaseSummary(completed RunPhaseTimingSnapshot) {
	if e == nil || e.logger == nil || !completed.Phase.Valid() {
		return
	}
	metrics, activeFeeds, activeOps, currentPhase := e.currentRunDiagnosticsSnapshot()
	var phaseMetrics RunPhaseMetricsSnapshot
	current := e.currentRunMetrics()
	if current != nil {
		phaseMetrics = current.phaseSnapshot(completed.Phase, completed.DurationMS)
	}
	var feeds []FeedWorkSnapshot
	if completed.Phase == RunPhaseSources {
		feeds = metrics.Feeds
	}
	e.logger.Info("engine phase completed",
		"phase", completed.Phase,
		"duration_ms", completed.DurationMS,
		"work", phaseMetrics.Work,
		"feeds", feeds,
		"next_phase", currentPhase,
		"active_feeds", activeFeeds,
		"active_operations", activeOps,
		"phase_times", metrics.PhaseTimes,
		"operations", phaseMetrics.Operations,
		"counters", phaseMetrics.Counters,
	)
}

func (e *Engine) logFeedProcessingSummary(name string, elapsed time.Duration, result FeedProcessingResult) {
	if e == nil || e.logger == nil || name == "" {
		return
	}
	feedMetrics, _ := e.feedMetricsSnapshot(name)
	elapsedMS := telemetryDurationMillis(elapsed)
	work := result.Work
	status := result.StatusString()
	e.logger.Info("feed processing summary",
		"source", name,
		"status", status,
		"processed", result.Processed,
		"exception", result.Exception.String(),
		"input_bytes", work.InputBytes,
		"entries", work.Entries,
		"unique_ips", work.UniqueIPs,
		"elapsed_ms", elapsedMS,
		"input_bytes_per_second", ratePerSecond(work.InputBytes, elapsedMS),
		"entries_per_second", ratePerSecond(work.Entries, elapsedMS),
		"unique_ips_per_second", ratePerSecond(work.UniqueIPs, elapsedMS),
		"operations", feedMetrics.Operations,
	)
}

func (e *Engine) currentRunDiagnosticsSnapshot() (RunMetricsSnapshot, []ActiveFeed, []ActiveOperation, RunPhase) {
	if e == nil {
		return RunMetricsSnapshot{}, nil, nil, RunPhaseUnknown
	}
	e.mu.RLock()
	phase := e.currentPhase
	e.mu.RUnlock()
	current := e.currentRunMetrics()
	activeFeeds := e.snapshotActiveFeeds()
	activeOps := e.snapshotActiveOperations(time.Now().UTC())
	if current == nil {
		return RunMetricsSnapshot{}, activeFeeds, activeOps, phase
	}
	return current.snapshot(true), activeFeeds, activeOps, phase
}

func (e *Engine) beginActiveOperation(operation, feed, stage, unit string, total int64) *activeOperationHandle {
	if e == nil || operation == "" {
		return nil
	}
	if total < 0 {
		total = 0
	}
	if unit == "" {
		unit = "items"
	}
	id := activeOperationKey(operation, feed, stage)
	startedAt := time.Now().UTC()
	e.mu.RLock()
	phase := e.currentPhase
	e.mu.RUnlock()
	e.activeOperationsMu.Lock()
	if e.activeOperations == nil {
		e.activeOperations = make(map[string]ActiveOperation)
	}
	e.activeOperations[id] = ActiveOperation{
		Operation: operation,
		Phase:     phase,
		Feed:      feed,
		Stage:     stage,
		Unit:      unit,
		StartedAt: startedAt,
		Total:     total,
	}
	e.activeOperationsMu.Unlock()
	return &activeOperationHandle{e: e, id: id}
}

func (h *activeOperationHandle) Update(current, total int64, counters map[string]int64) {
	if h == nil || h.e == nil || h.id == "" {
		return
	}
	if current < 0 && total < 0 && len(counters) == 0 {
		return
	}
	h.e.activeOperationsMu.Lock()
	op, ok := h.e.activeOperations[h.id]
	if ok {
		if current >= 0 {
			op.Current = current
		}
		if total >= 0 {
			op.Total = total
		}
		if len(counters) > 0 {
			op.Counters = copyInt64Map(counters)
		}
		h.e.activeOperations[h.id] = op
	}
	h.e.activeOperationsMu.Unlock()
}

func (h *activeOperationHandle) Add(delta, total int64, counters map[string]int64) {
	if h == nil || h.e == nil || h.id == "" {
		return
	}
	if delta == 0 && total < 0 && len(counters) == 0 {
		return
	}
	h.e.activeOperationsMu.Lock()
	op, ok := h.e.activeOperations[h.id]
	if ok {
		op.Current += delta
		if op.Current < 0 {
			op.Current = 0
		}
		if total >= 0 {
			op.Total = total
		}
		if len(counters) > 0 {
			op.Counters = copyInt64Map(counters)
		}
		h.e.activeOperations[h.id] = op
	}
	h.e.activeOperationsMu.Unlock()
}

func (h *activeOperationHandle) Finish() {
	if h == nil || h.e == nil || h.id == "" {
		return
	}
	h.e.activeOperationsMu.Lock()
	delete(h.e.activeOperations, h.id)
	h.e.activeOperationsMu.Unlock()
}

func activeOperationsFromMap(in map[string]ActiveOperation, now time.Time) []ActiveOperation {
	if len(in) == 0 {
		return nil
	}
	out := make([]ActiveOperation, 0, len(in))
	for _, op := range in {
		if !op.StartedAt.IsZero() {
			op.ElapsedMS = telemetryDurationMillis(now.Sub(op.StartedAt))
		}
		if op.Total > 0 {
			op.CompletionPct = completionPct(op.Current, op.Total)
		}
		op.RatePerSecond = ratePerSecond(op.Current, op.ElapsedMS)
		if len(op.Counters) > 0 {
			op.Counters = copyInt64Map(op.Counters)
		}
		out = append(out, op)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].StartedAt.Equal(out[j].StartedAt) {
			if out[i].Operation == out[j].Operation {
				return out[i].Feed < out[j].Feed
			}
			return out[i].Operation < out[j].Operation
		}
		return out[i].StartedAt.Before(out[j].StartedAt)
	})
	return out
}

func (e *Engine) snapshotActiveOperations(now time.Time) []ActiveOperation {
	if e == nil {
		return nil
	}
	e.activeOperationsMu.RLock()
	defer e.activeOperationsMu.RUnlock()
	return activeOperationsFromMap(e.activeOperations, now)
}

func (e *Engine) trySnapshotActiveOperations(now time.Time) ([]ActiveOperation, bool) {
	if e == nil {
		return nil, true
	}
	if !e.activeOperationsMu.TryRLock() {
		return nil, false
	}
	defer e.activeOperationsMu.RUnlock()
	return activeOperationsFromMap(e.activeOperations, now), true
}

func activeOperationKey(operation, feed, stage string) string {
	return operation + "\x00" + feed + "\x00" + stage
}

func copyInt64Map(in map[string]int64) map[string]int64 {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]int64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (e *Engine) startEngineRuntimeStatsSampler(ctx context.Context) {
	if e == nil || ctx == nil {
		return
	}
	e.runtimeStatsSamplerOnce.Do(func() {
		go func() {
			e.refreshEngineRuntimeStatsSafely()
			ticker := time.NewTicker(engineRuntimeStatsSampleInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					e.refreshEngineRuntimeStatsSafely()
				}
			}
		}()
	})
}

func (e *Engine) refreshEngineRuntimeStatsSafely() {
	defer func() {
		if recovered := recover(); recovered != nil && e != nil && e.logger != nil {
			e.logger.Error("engine runtime stats sampler panic recovered", "error", fmt.Sprint(recovered))
		}
	}()
	e.refreshEngineRuntimeStats()
}

func (e *Engine) refreshEngineRuntimeStats() {
	if e == nil {
		return
	}
	stats := engineRuntimeStatsFromSample(engineRuntimeStatsCaptureSnapshot()())
	e.runtimeStatsMu.Lock()
	e.runtimeStats = stats
	e.runtimeStatsSampledAt = time.Now().UTC()
	e.runtimeStatsMu.Unlock()
}

func (e *Engine) cachedEngineRuntimeStats() engineRuntimeStats {
	if e == nil {
		return engineRuntimeStats{}
	}
	e.runtimeStatsMu.RLock()
	stats := e.runtimeStats
	sampledAt := e.runtimeStatsSampledAt
	e.runtimeStatsMu.RUnlock()
	if !sampledAt.IsZero() {
		return stats
	}
	return engineRuntimeStats{
		GoMemLimit: runtimeinfo.GoMemLimit(),
	}
}

func diffEngineRuntimeStats(start, end engineRuntimeStats) engineRuntimeDelta {
	return runtimeinfo.Diff(start.runtimeInfoSnapshot(), end.runtimeInfoSnapshot())
}

func ratePerSecond(completed, elapsedMS int64) float64 {
	if completed <= 0 || elapsedMS <= 0 {
		return 0
	}
	return float64(completed) / (float64(elapsedMS) / 1000)
}

func completionPct(completed, total int64) int {
	if total <= 0 {
		return 0
	}
	return int(clampInt64((completed*100+total/2)/total, 0, 100))
}

func clampInt64(value, min, max int64) int64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
