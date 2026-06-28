package web

import (
	"time"

	"github.com/firehol/update-ipsets/pkg/engine"
)

type publicStatus struct {
	Engine publicEngineStatus `json:"engine"`
	System publicSystemStatus `json:"system"`
}

type publicEngineStatus struct {
	Running     bool      `json:"running"`
	LastStarted time.Time `json:"last_started,omitempty"`
	LastEnded   time.Time `json:"last_ended,omitempty"`
	SourceCount int       `json:"source_count"`
	MergeCount  int       `json:"merge_count"`
}

type publicSystemStatus struct {
	UptimeSeconds float64 `json:"uptime_seconds"`
	Uptime        string  `json:"uptime"`
}

func buildPublicStatus(eng *engine.Engine) publicStatus {
	snap, _ := eng.TryStatusSnapshotLight()
	sys := detailedStatusCached()
	return publicStatus{
		Engine: publicEngineStatus{
			Running:     snap.Running,
			LastStarted: sanitizeJSONTime(snap.LastStarted),
			LastEnded:   sanitizeJSONTime(snap.LastEnded),
			SourceCount: snap.SourceCount,
			MergeCount:  snap.MergeCount,
		},
		System: publicSystemStatus{
			UptimeSeconds: sys.UptimeSeconds,
			Uptime:        sys.Uptime,
		},
	}
}
