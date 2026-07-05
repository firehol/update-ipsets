package runtimeinfo

import (
	"runtime/metrics"
	"syscall"
)

type Snapshot struct {
	Goroutines              int
	HeapAlloc               uint64
	HeapSys                 uint64
	HeapInuse               uint64
	HeapIdle                uint64
	HeapReleased            uint64
	HeapObjects             uint64
	StackInuse              uint64
	Sys                     uint64
	NumGC                   uint32
	PauseTotalNS            uint64
	PauseTotalMS            int64
	LastGCUnix              int64
	GoMemLimit              int64
	RSSKB                   uint64
	VMSKB                   uint64
	DataKB                  uint64
	CPUUserSeconds          float64
	CPUSystemSeconds        float64
	CPUTotalSeconds         float64
	CPUUserMS               int64
	CPUSystemMS             int64
	CPUTotalMS              int64
	ProcReadBytes           uint64
	ProcWriteBytes          uint64
	ProcCancelledWriteBytes uint64
	ProcReadSyscalls        uint64
	ProcWriteSyscalls       uint64
	OpenFDs                 int
}

type Delta struct {
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

const (
	metricGoroutines = iota
	metricHeapObjectsBytes
	metricHeapFreeBytes
	metricHeapReleasedBytes
	metricHeapUnusedBytes
	metricHeapStacksBytes
	metricOSStacksBytes
	metricTotalBytes
	metricHeapObjects
	metricGCCycles
	metricGoMemLimit
	metricGCPauseCPUSeconds
)

// Capture intentionally avoids runtime.ReadMemStats and procfs reads. It runs
// from liveness-sensitive samplers, so optional process details stay unknown
// rather than risking stop-the-world or kernel file I/O stalls.
func Capture() Snapshot {
	samples := [...]metrics.Sample{
		{Name: "/sched/goroutines:goroutines"},
		{Name: "/memory/classes/heap/objects:bytes"},
		{Name: "/memory/classes/heap/free:bytes"},
		{Name: "/memory/classes/heap/released:bytes"},
		{Name: "/memory/classes/heap/unused:bytes"},
		{Name: "/memory/classes/heap/stacks:bytes"},
		{Name: "/memory/classes/os-stacks:bytes"},
		{Name: "/memory/classes/total:bytes"},
		{Name: "/gc/heap/objects:objects"},
		{Name: "/gc/cycles/total:gc-cycles"},
		{Name: "/gc/gomemlimit:bytes"},
		{Name: "/cpu/classes/gc/pause:cpu-seconds"},
	}
	metrics.Read(samples[:])

	heapObjectsBytes := metricUint64(samples[:], metricHeapObjectsBytes)
	heapFreeBytes := metricUint64(samples[:], metricHeapFreeBytes)
	heapReleasedBytes := metricUint64(samples[:], metricHeapReleasedBytes)
	heapUnusedBytes := metricUint64(samples[:], metricHeapUnusedBytes)
	heapStacksBytes := metricUint64(samples[:], metricHeapStacksBytes)
	osStacksBytes := metricUint64(samples[:], metricOSStacksBytes)
	pauseTotalSeconds := metricFloat64(samples[:], metricGCPauseCPUSeconds)
	pauseTotalNS := uint64(pauseTotalSeconds * 1_000_000_000)

	out := Snapshot{
		Goroutines:   int(metricUint64(samples[:], metricGoroutines)),
		HeapAlloc:    heapObjectsBytes,
		HeapSys:      heapObjectsBytes + heapFreeBytes + heapReleasedBytes + heapUnusedBytes,
		HeapInuse:    heapObjectsBytes + heapUnusedBytes,
		HeapIdle:     heapFreeBytes + heapReleasedBytes,
		HeapReleased: heapReleasedBytes,
		HeapObjects:  metricUint64(samples[:], metricHeapObjects),
		StackInuse:   heapStacksBytes + osStacksBytes,
		Sys:          metricUint64(samples[:], metricTotalBytes),
		NumGC:        uint32(metricUint64(samples[:], metricGCCycles)),
		PauseTotalNS: pauseTotalNS,
		PauseTotalMS: int64(pauseTotalNS / 1_000_000),
		GoMemLimit:   normalizeMemoryLimit(metricUint64(samples[:], metricGoMemLimit)),
	}
	readProcessUsage(&out)
	return out
}

func Diff(start, end Snapshot) Delta {
	return Delta{
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

func GoMemLimit() int64 {
	samples := [...]metrics.Sample{{Name: "/gc/gomemlimit:bytes"}}
	metrics.Read(samples[:])
	return normalizeMemoryLimit(metricUint64(samples[:], 0))
}

func unsignedDelta(start, end uint64) uint64 {
	if end <= start {
		return 0
	}
	return end - start
}

func normalizeMemoryLimit(limit uint64) int64 {
	if limit == 0 || limit >= 1<<62 {
		return -1
	}
	return int64(limit)
}

func metricUint64(samples []metrics.Sample, index int) uint64 {
	if index < 0 || index >= len(samples) {
		return 0
	}
	value := samples[index].Value
	if value.Kind() != metrics.KindUint64 {
		return 0
	}
	return value.Uint64()
}

func metricFloat64(samples []metrics.Sample, index int) float64 {
	if index < 0 || index >= len(samples) {
		return 0
	}
	value := samples[index].Value
	if value.Kind() != metrics.KindFloat64 {
		return 0
	}
	return value.Float64()
}

func readProcessUsage(out *Snapshot) {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return
	}
	userSeconds := float64(usage.Utime.Sec) + float64(usage.Utime.Usec)/1_000_000
	systemSeconds := float64(usage.Stime.Sec) + float64(usage.Stime.Usec)/1_000_000
	out.CPUUserSeconds = userSeconds
	out.CPUSystemSeconds = systemSeconds
	out.CPUTotalSeconds = userSeconds + systemSeconds
	out.CPUUserMS = usage.Utime.Sec*1000 + int64(usage.Utime.Usec)/1000
	out.CPUSystemMS = usage.Stime.Sec*1000 + int64(usage.Stime.Usec)/1000
	out.CPUTotalMS = out.CPUUserMS + out.CPUSystemMS
}
