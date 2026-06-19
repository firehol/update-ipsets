package engine

import (
	"context"
	"os"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/firehol/update-ipsets/pkg/runreason"
)

const engineProgressLogInterval = time.Minute

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

type engineRuntimeDelta struct {
	NumGC                   int64  `json:"num_gc,omitempty"`
	PauseTotalMS            int64  `json:"pause_total_ms,omitempty"`
	CPUUserMS               int64  `json:"cpu_user_ms,omitempty"`
	CPUSystemMS             int64  `json:"cpu_system_ms,omitempty"`
	CPUTotalMS              int64  `json:"cpu_total_ms,omitempty"`
	ProcReadBytes           uint64 `json:"proc_read_bytes,omitempty"`
	ProcWriteBytes          uint64 `json:"proc_write_bytes,omitempty"`
	ProcCancelledWriteBytes uint64 `json:"proc_cancelled_write_bytes,omitempty"`
	ProcReadSyscalls        uint64 `json:"proc_read_syscalls,omitempty"`
	ProcWriteSyscalls       uint64 `json:"proc_write_syscalls,omitempty"`
}

type activeOperationHandle struct {
	e  *Engine
	id string
}

func newEngineRunDiagnostics(reason runreason.Reason, opts RunOptions, startedAt time.Time) engineRunDiagnostics {
	return engineRunDiagnostics{
		reason:     reason,
		selected:   len(opts.Selected),
		recheck:    opts.Recheck,
		reprocess:  opts.Reprocess,
		manual:     opts.Manual,
		startedAt:  startedAt,
		startStats: captureEngineRuntimeStats(),
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
	stats := captureEngineRuntimeStats()
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
	status := "ok"
	if runErr != nil {
		status = "error"
	}
	stats := captureEngineRuntimeStats()
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
	e.mu.RLock()
	current := e.currentMetrics
	e.mu.RUnlock()
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
	current := e.currentMetrics
	activeFeeds := e.snapshotActiveFeedsLocked()
	activeOps := e.snapshotActiveOperationsLocked(time.Now().UTC())
	phase := e.currentPhase
	e.mu.RUnlock()
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
	e.mu.Lock()
	if e.activeOperations == nil {
		e.activeOperations = make(map[string]ActiveOperation)
	}
	e.activeOperations[id] = ActiveOperation{
		Operation: operation,
		Phase:     e.currentPhase,
		Feed:      feed,
		Stage:     stage,
		Unit:      unit,
		StartedAt: startedAt,
		Total:     total,
	}
	e.mu.Unlock()
	return &activeOperationHandle{e: e, id: id}
}

func (h *activeOperationHandle) Update(current, total int64, counters map[string]int64) {
	if h == nil || h.e == nil || h.id == "" {
		return
	}
	if current < 0 && total < 0 && len(counters) == 0 {
		return
	}
	h.e.mu.Lock()
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
	h.e.mu.Unlock()
}

func (h *activeOperationHandle) Finish() {
	if h == nil || h.e == nil || h.id == "" {
		return
	}
	h.e.mu.Lock()
	delete(h.e.activeOperations, h.id)
	h.e.mu.Unlock()
}

func (e *Engine) snapshotActiveOperationsLocked(now time.Time) []ActiveOperation {
	if len(e.activeOperations) == 0 {
		return nil
	}
	out := make([]ActiveOperation, 0, len(e.activeOperations))
	for _, op := range e.activeOperations {
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

func captureEngineRuntimeStats() engineRuntimeStats {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	stats := engineRuntimeStats{
		Goroutines:   runtime.NumGoroutine(),
		HeapAlloc:    mem.HeapAlloc,
		HeapSys:      mem.HeapSys,
		HeapInuse:    mem.HeapInuse,
		HeapIdle:     mem.HeapIdle,
		HeapReleased: mem.HeapReleased,
		HeapObjects:  mem.HeapObjects,
		NumGC:        mem.NumGC,
		PauseTotalMS: int64(mem.PauseTotalNs / uint64(time.Millisecond)),
		GoMemLimit:   engineGoMemLimit(),
	}
	readEngineProcessMemory(&stats)
	readEngineProcessUsage(&stats)
	readEngineProcessIO(&stats)
	readEngineOpenFDs(&stats)
	return stats
}

func diffEngineRuntimeStats(start, end engineRuntimeStats) engineRuntimeDelta {
	return engineRuntimeDelta{
		NumGC:                   int64(end.NumGC) - int64(start.NumGC),
		PauseTotalMS:            end.PauseTotalMS - start.PauseTotalMS,
		CPUUserMS:               end.CPUUserMS - start.CPUUserMS,
		CPUSystemMS:             end.CPUSystemMS - start.CPUSystemMS,
		CPUTotalMS:              end.CPUTotalMS - start.CPUTotalMS,
		ProcReadBytes:           unsignedDelta(start.ProcReadBytes, end.ProcReadBytes),
		ProcWriteBytes:          unsignedDelta(start.ProcWriteBytes, end.ProcWriteBytes),
		ProcCancelledWriteBytes: unsignedDelta(start.ProcCancelledWriteBytes, end.ProcCancelledWriteBytes),
		ProcReadSyscalls:        unsignedDelta(start.ProcReadSyscalls, end.ProcReadSyscalls),
		ProcWriteSyscalls:       unsignedDelta(start.ProcWriteSyscalls, end.ProcWriteSyscalls),
	}
}

func unsignedDelta(start, end uint64) uint64 {
	if end <= start {
		return 0
	}
	return end - start
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

func engineGoMemLimit() int64 {
	limit := debug.SetMemoryLimit(-1)
	if limit <= 0 || limit >= 1<<62 {
		return -1
	}
	return limit
}

func readEngineProcessMemory(stats *engineRuntimeStats) {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields := strings.Fields(strings.TrimSpace(value))
		if len(fields) == 0 {
			continue
		}
		n, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			continue
		}
		switch key {
		case "VmRSS":
			stats.RSSKB = n
		case "VmSize":
			stats.VMSKB = n
		case "VmData":
			stats.DataKB = n
		}
	}
}

func readEngineProcessUsage(stats *engineRuntimeStats) {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return
	}
	user := usage.Utime.Sec*1000 + int64(usage.Utime.Usec)/1000
	system := usage.Stime.Sec*1000 + int64(usage.Stime.Usec)/1000
	stats.CPUUserMS = user
	stats.CPUSystemMS = system
	stats.CPUTotalMS = user + system
}

func readEngineProcessIO(stats *engineRuntimeStats) {
	data, err := os.ReadFile("/proc/self/io")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		n, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
		if err != nil {
			continue
		}
		switch key {
		case "read_bytes":
			stats.ProcReadBytes = n
		case "write_bytes":
			stats.ProcWriteBytes = n
		case "cancelled_write_bytes":
			stats.ProcCancelledWriteBytes = n
		case "syscr":
			stats.ProcReadSyscalls = n
		case "syscw":
			stats.ProcWriteSyscalls = n
		}
	}
}

func readEngineOpenFDs(stats *engineRuntimeStats) {
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return
	}
	stats.OpenFDs = len(entries)
}
