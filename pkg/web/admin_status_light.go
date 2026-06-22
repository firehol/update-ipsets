package web

import (
	"runtime"
	"strings"

	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/scheduler"
)

type adminStatusLight struct {
	PublicBaseURL string                     `json:"public_base_url,omitempty"`
	System        adminSystemInfo            `json:"system"`
	Engine        engine.StatusSnapshotLight `json:"engine"`
	Scheduler     scheduler.Snapshot         `json:"scheduler"`
	Queues        scheduler.ActivitySnapshot `json:"queues"`
	Metrics       scheduler.MetricsSnapshot  `json:"metrics"`
	Feeds         adminFeedsSummary          `json:"feeds"`
	Artifacts     []adminArtifact            `json:"artifacts,omitempty"`
}

func buildAdminStatusLight(eng *engine.Engine, runner *scheduler.Runner) adminStatusLight {
	sys := detailedStatusCached()
	cfg := eng.Config()
	snapshot := runner.Snapshot()
	return adminStatusLight{
		PublicBaseURL: strings.TrimSpace(eng.Runtime().PublicBaseURL),
		System:        adminSystemFromDetailed(sys),
		Engine:        eng.StatusSnapshotLight(),
		Scheduler:     sanitizeSchedulerSnapshot(snapshot),
		Queues:        runner.ActivitySnapshot(),
		Metrics:       runner.MetricsSnapshot(),
		Feeds: adminFeedsSummary{
			TotalConfigured: len(cfg.Sources),
		},
	}
}

func adminSystemFromDetailed(sys detailedSystemInfo) adminSystemInfo {
	return adminSystemInfo{
		Uptime:       sys.Uptime,
		GoVersion:    runtime.Version(),
		GOOS:         runtime.GOOS,
		GOARCH:       runtime.GOARCH,
		Goroutines:   sys.Goroutines,
		HeapAlloc:    sys.HeapAlloc,
		HeapSys:      sys.HeapSys,
		HeapInuse:    sys.HeapInuse,
		StackInuse:   sys.StackInuse,
		Sys:          sys.Sys,
		NumGC:        sys.NumGC,
		LastGC:       sys.LastGCUnix,
		GCPauseTotal: sys.PauseTotalNs,
		DiskFree:     sys.DiskFree,
		RSSKB:        sys.RSSKB,
		VMSKB:        sys.VMSKB,
		DataKB:       sys.DataKB,

		CPUUserSeconds:     sys.CPUUserSeconds,
		CPUSystemSeconds:   sys.CPUSystemSeconds,
		CPUTotalSeconds:    sys.CPUTotalSeconds,
		ProcReadBytes:      sys.ProcReadBytes,
		ProcWriteBytes:     sys.ProcWriteBytes,
		ProcCancelledWrite: sys.ProcCancelledWrite,
		ProcReadSyscalls:   sys.ProcReadSyscalls,
		ProcWriteSyscalls:  sys.ProcWriteSyscalls,
		OpenFDs:            sys.OpenFDs,
	}
}
