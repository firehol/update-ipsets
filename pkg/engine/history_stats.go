package engine

import (
	"path/filepath"

	"github.com/firehol/update-ipsets/pkg/cache"
)

func (e *Engine) refreshHistoryStatsFromLedger(name string, entry *cache.Entry, frequency int) bool {
	if e == nil || entry == nil {
		return false
	}
	if e.historyStatsFromRuntime(name, entry, frequency) {
		return true
	}
	if e.runtime.LibDir == "" {
		return false
	}
	points := parseHistoryCSV(filepath.Join(e.runtime.LibDir, name, "history.csv"), name)
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
