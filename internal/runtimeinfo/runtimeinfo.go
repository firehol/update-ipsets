package runtimeinfo

import (
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"
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

func Capture() Snapshot {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	out := Snapshot{
		Goroutines:   runtime.NumGoroutine(),
		HeapAlloc:    mem.HeapAlloc,
		HeapSys:      mem.HeapSys,
		HeapInuse:    mem.HeapInuse,
		HeapIdle:     mem.HeapIdle,
		HeapReleased: mem.HeapReleased,
		HeapObjects:  mem.HeapObjects,
		StackInuse:   mem.StackInuse,
		Sys:          mem.Sys,
		NumGC:        mem.NumGC,
		PauseTotalNS: mem.PauseTotalNs,
		PauseTotalMS: int64(mem.PauseTotalNs / uint64(time.Millisecond)),
		LastGCUnix:   int64(mem.LastGC),
		GoMemLimit:   GoMemLimit(),
	}
	readProcessMemory(&out)
	readProcessUsage(&out)
	readProcessIO(&out)
	readOpenFDs(&out)
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
	limit := debug.SetMemoryLimit(-1)
	if limit <= 0 || limit >= 1<<62 {
		return -1
	}
	return limit
}

func unsignedDelta(start, end uint64) uint64 {
	if end <= start {
		return 0
	}
	return end - start
}

func readProcessMemory(out *Snapshot) {
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
			out.RSSKB = n
		case "VmSize":
			out.VMSKB = n
		case "VmData":
			out.DataKB = n
		}
	}
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

func readProcessIO(out *Snapshot) {
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
			out.ProcReadBytes = n
		case "write_bytes":
			out.ProcWriteBytes = n
		case "cancelled_write_bytes":
			out.ProcCancelledWriteBytes = n
		case "syscr":
			out.ProcReadSyscalls = n
		case "syscw":
			out.ProcWriteSyscalls = n
		}
	}
}

func readOpenFDs(out *Snapshot) {
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return
	}
	out.OpenFDs = len(entries)
}
