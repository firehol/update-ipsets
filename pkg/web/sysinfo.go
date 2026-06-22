package web

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var startedAt = time.Now()

const detailedStatusSampleMaxAge = time.Second
const runtimeStatsSampleInterval = 5 * time.Second

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

	// Process stats (populated on Linux, zero elsewhere)
	RSSKB              uint64  `json:"rss_kb"`
	VMSKB              uint64  `json:"vms_kb"`
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
		Goroutines:    runtime.NumGoroutine(),
		DiskFree:      "unknown",
		GoMemLimit:    goMemLimit(),
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
			refreshDetailedStatus()
			ticker := time.NewTicker(runtimeStatsSampleInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					refreshDetailedStatus()
				}
			}
		}()
	})
}

func captureDetailedStatus(now time.Time) detailedSystemInfo {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	up := now.Sub(startedAt)

	diskFree := "unknown"
	var stat syscall.Statfs_t
	if err := syscall.Statfs(".", &stat); err == nil {
		diskFree = humanBytes(stat.Bavail * uint64(stat.Bsize))
	}

	info := detailedSystemInfo{
		UptimeSeconds: up.Seconds(),
		Uptime:        up.Truncate(time.Second).String(),
		Goroutines:    runtime.NumGoroutine(),
		DiskFree:      diskFree,

		HeapAlloc:    mem.HeapAlloc,
		HeapSys:      mem.HeapSys,
		HeapInuse:    mem.HeapInuse,
		HeapIdle:     mem.HeapIdle,
		HeapReleased: mem.HeapReleased,
		HeapObjects:  mem.HeapObjects,
		StackInuse:   mem.StackInuse,
		Sys:          mem.Sys,

		NumGC:        mem.NumGC,
		PauseTotalNs: mem.PauseTotalNs,
		LastGCUnix:   int64(mem.LastGC),

		GoMemLimit: goMemLimit(),
	}

	readProcessMemory(&info)
	readProcessUsage(&info)
	readProcessIO(&info)
	readOpenFDs(&info)
	return info
}

func withCurrentUptime(info detailedSystemInfo, now time.Time) detailedSystemInfo {
	up := now.Sub(startedAt)
	info.UptimeSeconds = up.Seconds()
	info.Uptime = up.Truncate(time.Second).String()
	return info
}

// goMemLimit returns the current GOMEMLIMIT value, or -1 if unset / math.MaxInt64.
func goMemLimit() int64 {
	limit := debug.SetMemoryLimit(-1) // read current without changing
	if limit <= 0 || limit >= 1<<62 {
		return -1
	}
	return limit
}

// readProcessMemory populates RSS/VMS from /proc/self/status on Linux.
// On other platforms this is a no-op.
func readProcessMemory(info *detailedSystemInfo) {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		// Values are typically "12345 kB".
		numStr := strings.Fields(value)
		if len(numStr) == 0 {
			continue
		}
		n, err := strconv.ParseUint(numStr[0], 10, 64)
		if err != nil {
			continue
		}
		switch key {
		case "VmRSS":
			info.RSSKB = n
		case "VmSize":
			info.VMSKB = n
		case "VmData":
			info.DataKB = n
		}
	}
}

func readProcessUsage(info *detailedSystemInfo) {
	if info == nil {
		return
	}
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return
	}
	user := float64(usage.Utime.Sec) + float64(usage.Utime.Usec)/1_000_000
	system := float64(usage.Stime.Sec) + float64(usage.Stime.Usec)/1_000_000
	info.CPUUserSeconds = user
	info.CPUSystemSeconds = system
	info.CPUTotalSeconds = user + system
}

func readProcessIO(info *detailedSystemInfo) {
	if info == nil {
		return
	}
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
			info.ProcReadBytes = n
		case "write_bytes":
			info.ProcWriteBytes = n
		case "cancelled_write_bytes":
			info.ProcCancelledWrite = n
		case "syscr":
			info.ProcReadSyscalls = n
		case "syscw":
			info.ProcWriteSyscalls = n
		}
	}
}

func readOpenFDs(info *detailedSystemInfo) {
	if info == nil {
		return
	}
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return
	}
	info.OpenFDs = len(entries)
}
