package engine

import (
	"context"
	"path/filepath"

	"github.com/firehol/update-ipsets/pkg/cache"
)

func (e *Engine) refreshHistoryStatsFromLedger(name string, entry *cache.Entry, frequency int) bool {
	return e.refreshHistoryStatsFromLedgerWithSnapshot(e.operationSnapshot(), name, entry, frequency)
}

func (e *Engine) refreshHistoryStatsFromLedgerWithRuntime(rt Runtime, name string, entry *cache.Entry, frequency int) bool {
	return e.refreshHistoryStatsFromLedgerWithSnapshot(operationSnapshot{runtime: rt}, name, entry, frequency)
}

func (e *Engine) refreshHistoryStatsFromLedgerWithSnapshot(snap operationSnapshot, name string, entry *cache.Entry, frequency int) bool {
	if e == nil || entry == nil {
		return false
	}
	if e.historyStatsFromSnapshot(context.Background(), snap, name, entry, frequency) {
		return true
	}
	if snap.runtime.LibDir == "" {
		return false
	}
	points := parseHistoryCSVInRoot(snap.runtime.LibDir, filepath.Join(name, "history.csv"), name)
	return applyHistoryPointsToEntry(entry, points, frequency)
}

func applyHistoryPointsToEntry(entry *cache.Entry, points []HistoryPoint, frequency int) bool {
	if entry == nil || len(points) == 0 {
		return false
	}
	var stats historyLedgerStats
	for _, point := range points {
		stats.observe(point)
	}
	return stats.apply(entry, frequency)
}

func roundSecondsToMinutes(seconds int64) int {
	if seconds <= 0 {
		return 0
	}
	minutes := int((seconds + 30) / 60)
	if minutes < 1 {
		return 1
	}
	return minutes
}
