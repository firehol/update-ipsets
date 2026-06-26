package web

import (
	"runtime"
	"strings"

	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/feedhealth"
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
}

func buildAdminStatusLight(eng *engine.Engine, runner *scheduler.Runner) adminStatusLight {
	sys := detailedStatusCached()
	cfg, rt := eng.ConfigRuntimeSnapshot()
	totalConfigured := 0
	if cfg != nil {
		totalConfigured = len(cfg.Sources)
	}
	activity := runner.ActivitySnapshotLight()
	snapshot := runner.CachedSnapshot()
	return adminStatusLight{
		PublicBaseURL: strings.TrimSpace(rt.PublicBaseURL),
		System:        adminSystemFromDetailed(sys),
		Engine:        eng.StatusSnapshotLight(),
		Scheduler:     sanitizeSchedulerSnapshot(snapshot),
		Queues:        activity,
		Metrics:       runner.MetricsSnapshot(),
		Feeds:         summarizeAdminFeedsFromSchedulerSnapshot(totalConfigured, snapshot, activity),
	}
}

func summarizeAdminFeedsFromSchedulerSnapshot(totalConfigured int, snap scheduler.Snapshot, activity scheduler.ActivitySnapshot) adminFeedsSummary {
	summary := adminFeedsSummary{TotalConfigured: totalConfigured}
	if len(snap.Items) == 0 {
		return summary
	}
	active := make(map[string]struct{}, len(activity.DownloadActive)+len(activity.ProcessingActive))
	for _, item := range activity.DownloadActive {
		active[item.Name] = struct{}{}
	}
	for _, item := range activity.ProcessingActive {
		active[item.Name] = struct{}{}
	}
	for _, item := range snap.Items {
		if item.Hidden {
			summary.Hidden++
		}
		summary.TotalEntries += item.Entries
		summary.TotalUniqueIPs += item.UniqueIPs
		if item.Enabled {
			summary.TotalEnabled++
		} else {
			summary.Disabled++
			continue
		}
		switch item.HealthClass {
		case string(feedhealth.ClassHealthy):
			summary.Healthy++
		case string(feedhealth.ClassDelayed):
			summary.Delayed++
		case string(feedhealth.ClassRisky):
			summary.Risky++
		case string(feedhealth.ClassUnavailable):
			summary.Unavailable++
			summary.Errors++
		case string(feedhealth.ClassArchived):
			summary.Archived++
		case string(feedhealth.ClassEmpty):
			summary.Empty++
		case string(feedhealth.ClassUnmaintained):
			summary.Unmaintained++
			summary.Stale++
		}
		if _, ok := active[item.Name]; ok {
			summary.Running++
			continue
		}
		if item.NeverRun {
			summary.NeverRun++
		}
	}
	return summary
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
