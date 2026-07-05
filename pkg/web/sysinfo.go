package web

import (
	"context"
	"fmt"
	"sync"
	"syscall"
	"time"

	"github.com/firehol/update-ipsets/internal/runtimeinfo"
)

var startedAt = time.Now()

const runtimeStatsSampleInterval = 5 * time.Second
const detailedStatusSampleMaxAge = runtimeStatsSampleInterval

var (
	detailedStatusCaptureMu sync.Mutex
	detailedStatusCapture   = runtimeinfo.Capture
)

func setDetailedStatusCaptureForTest(fn func() runtimeinfo.Snapshot) func() {
	detailedStatusCaptureMu.Lock()
	old := detailedStatusCapture
	if fn == nil {
		detailedStatusCapture = runtimeinfo.Capture
	} else {
		detailedStatusCapture = fn
	}
	detailedStatusCaptureMu.Unlock()
	return func() {
		detailedStatusCaptureMu.Lock()
		detailedStatusCapture = old
		detailedStatusCaptureMu.Unlock()
	}
}

func detailedStatusCaptureSnapshot() func() runtimeinfo.Snapshot {
	detailedStatusCaptureMu.Lock()
	defer detailedStatusCaptureMu.Unlock()
	return detailedStatusCapture
}

var detailedStatusCache struct {
	mu        sync.RWMutex
	sampledAt time.Time
	info      detailedSystemInfo
}

type runtimeStatsSampler struct {
	once sync.Once
}

func newRuntimeStatsSampler() *runtimeStatsSampler {
	return &runtimeStatsSampler{}
}

func humanBytes(value uint64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	div, exp := uint64(unit), 0
	for n := value / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(value)/float64(div), "KMGTPE"[exp])
}

// detailedSystemInfo contains comprehensive system and process status used by
// the authenticated admin surface. Public status is intentionally narrower.
type detailedSystemInfo struct {
	UptimeSeconds float64 `json:"uptime_seconds"`
	Uptime        string  `json:"uptime"`
	Goroutines    int     `json:"goroutines"`
	DiskFree      string  `json:"disk_free"`

	// Go heap stats
	HeapAlloc    uint64 `json:"heap_alloc"`
	HeapSys      uint64 `json:"heap_sys"`
	HeapInuse    uint64 `json:"heap_inuse"`
	HeapIdle     uint64 `json:"heap_idle"`
	HeapReleased uint64 `json:"heap_released"`
	HeapObjects  uint64 `json:"heap_objects"`
	StackInuse   uint64 `json:"stack_inuse"`
	Sys          uint64 `json:"sys"`

	// GC stats
	NumGC        uint32 `json:"num_gc"`
	PauseTotalNs uint64 `json:"pause_total_ns"`
	LastGCUnix   int64  `json:"last_gc_unix"`

	// Go memory limit (GOMEMLIMIT)
	GoMemLimit int64 `json:"go_mem_limit"` // -1 if not set

	// Process stats collected without procfs reads from liveness-sensitive samplers.
	RSSKB              uint64  `json:"rss_kb,omitempty"`
	VMSKB              uint64  `json:"vms_kb,omitempty"`
	DataKB             uint64  `json:"data_kb,omitempty"`
	CPUUserSeconds     float64 `json:"cpu_user_seconds,omitempty"`
	CPUSystemSeconds   float64 `json:"cpu_system_seconds,omitempty"`
	CPUTotalSeconds    float64 `json:"cpu_total_seconds,omitempty"`
	ProcReadBytes      uint64  `json:"proc_read_bytes,omitempty"`
	ProcWriteBytes     uint64  `json:"proc_write_bytes,omitempty"`
	ProcCancelledWrite uint64  `json:"proc_cancelled_write_bytes,omitempty"`
	ProcReadSyscalls   uint64  `json:"proc_read_syscalls,omitempty"`
	ProcWriteSyscalls  uint64  `json:"proc_write_syscalls,omitempty"`
	OpenFDs            int     `json:"open_fds,omitempty"`
}

func detailedStatus() detailedSystemInfo {
	now := time.Now()
	detailedStatusCache.mu.RLock()
	if !detailedStatusCache.sampledAt.IsZero() && now.Sub(detailedStatusCache.sampledAt) < detailedStatusSampleMaxAge {
		info := detailedStatusCache.info
		detailedStatusCache.mu.RUnlock()
		return withCurrentUptime(info, now)
	}
	detailedStatusCache.mu.RUnlock()

	detailedStatusCache.mu.Lock()
	defer detailedStatusCache.mu.Unlock()
	now = time.Now()
	if !detailedStatusCache.sampledAt.IsZero() && now.Sub(detailedStatusCache.sampledAt) < detailedStatusSampleMaxAge {
		return withCurrentUptime(detailedStatusCache.info, now)
	}
	info := captureDetailedStatus(now)
	detailedStatusCache.sampledAt = now
	detailedStatusCache.info = info
	return withCurrentUptime(info, now)
}

func detailedStatusCached() detailedSystemInfo {
	now := time.Now()
	detailedStatusCache.mu.RLock()
	if !detailedStatusCache.sampledAt.IsZero() {
		info := detailedStatusCache.info
		detailedStatusCache.mu.RUnlock()
		return withCurrentUptime(info, now)
	}
	detailedStatusCache.mu.RUnlock()
	return detailedSystemInfo{
		UptimeSeconds: now.Sub(startedAt).Seconds(),
		Uptime:        now.Sub(startedAt).Truncate(time.Second).String(),
		DiskFree:      "unknown",
	}
}

func refreshDetailedStatus() detailedSystemInfo {
	now := time.Now()
	info := captureDetailedStatus(now)
	detailedStatusCache.mu.Lock()
	detailedStatusCache.sampledAt = now
	detailedStatusCache.info = info
	detailedStatusCache.mu.Unlock()
	return withCurrentUptime(info, now)
}

func (s *runtimeStatsSampler) Start(ctx context.Context) {
	if s == nil || ctx == nil {
		return
	}
	s.once.Do(func() {
		go func() {
			refreshDetailedStatusSafely()
			ticker := time.NewTicker(runtimeStatsSampleInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					refreshDetailedStatusSafely()
				}
			}
		}()
	})
}

func refreshDetailedStatusSafely() {
	defer recoverDaemonControlPanic("runtime_stats_sampler")
	refreshDetailedStatus()
}

func captureDetailedStatus(now time.Time) detailedSystemInfo {
	up := now.Sub(startedAt)
	sample := detailedStatusCaptureSnapshot()()
	info := detailedSystemInfo{
		UptimeSeconds: up.Seconds(),
		Uptime:        up.Truncate(time.Second).String(),
		Goroutines:    sample.Goroutines,
		DiskFree:      "unknown",

		HeapAlloc:    sample.HeapAlloc,
		HeapSys:      sample.HeapSys,
		HeapInuse:    sample.HeapInuse,
		HeapIdle:     sample.HeapIdle,
		HeapReleased: sample.HeapReleased,
		HeapObjects:  sample.HeapObjects,
		StackInuse:   sample.StackInuse,
		Sys:          sample.Sys,

		NumGC:        sample.NumGC,
		PauseTotalNs: sample.PauseTotalNS,
		LastGCUnix:   sample.LastGCUnix,

		GoMemLimit: sample.GoMemLimit,

		RSSKB:              sample.RSSKB,
		VMSKB:              sample.VMSKB,
		DataKB:             sample.DataKB,
		CPUUserSeconds:     sample.CPUUserSeconds,
		CPUSystemSeconds:   sample.CPUSystemSeconds,
		CPUTotalSeconds:    sample.CPUTotalSeconds,
		ProcReadBytes:      sample.ProcReadBytes,
		ProcWriteBytes:     sample.ProcWriteBytes,
		ProcCancelledWrite: sample.ProcCancelledWriteBytes,
		ProcReadSyscalls:   sample.ProcReadSyscalls,
		ProcWriteSyscalls:  sample.ProcWriteSyscalls,
		OpenFDs:            sample.OpenFDs,
	}
	// Keep disk space separate from runtimeinfo because it depends on the
	// current working directory used by the web process.
	info.DiskFree = currentDiskFree()
	return info
}

func withCurrentUptime(info detailedSystemInfo, now time.Time) detailedSystemInfo {
	up := now.Sub(startedAt)
	info.UptimeSeconds = up.Seconds()
	info.Uptime = up.Truncate(time.Second).String()
	return info
}

func currentDiskFree() string {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(".", &stat); err != nil {
		return "unknown"
	}
	return humanBytes(stat.Bavail * uint64(stat.Bsize))
}
