package engine

import (
	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
)

func validJSONUnixSeconds(ts int64) bool {
	return cache.ValidJSONUnixSeconds(ts)
}

func invalidJSONUnixSeconds(ts int64) bool {
	return cache.InvalidJSONUnixSeconds(ts)
}

func (e *Engine) repairInvalidEntryTimestamps() error {
	if e == nil || e.cfg == nil || e.state == nil {
		return nil
	}

	repaired := 0
	for _, name := range config.SortedSourceNames(e.cfg) {
		src := e.cfg.Sources[name]
		if src == nil {
			continue
		}
		snap := e.state.EntrySnapshot(name)
		if snap == nil {
			continue
		}
		entry := *snap
		if !e.repairEntryTimestampsFromDisk(name, src, &entry) {
			continue
		}
		e.state.ReplaceEntry(name, entry)
		repaired++
	}
	if repaired == 0 {
		return nil
	}
	if err := cache.Save(e.cachePath, e.state); err != nil {
		e.logger.Warn("repair: failed to persist repaired entry timestamps", "count", repaired, "path", e.cachePath, "error", err)
		return nil
	}
	e.logger.Info("repaired invalid entry timestamps from disk evidence", "count", repaired, "path", e.cachePath)
	return nil
}

func (e *Engine) repairEntryTimestampsFromDisk(name string, src *config.Source, entry *cache.Entry) bool {
	if entry == nil {
		return false
	}
	if !entryNeedsTimestampRepair(entry) {
		return false
	}

	latestObserved, haveLatest := e.latestObservedTimestamp(name, src)
	firstObserved, haveFirst := e.firstObservedTimestamp(name)
	return entry.RepairInvalidTimestamps(cache.TimestampRepairEvidence{
		LatestUnix: latestObserved,
		HaveLatest: haveLatest,
		FirstUnix:  firstObserved,
		HaveFirst:  haveFirst,
	})
}

func entryNeedsTimestampRepair(entry *cache.Entry) bool {
	if entry == nil {
		return false
	}
	return invalidJSONUnixSeconds(entry.SourceDate) ||
		invalidJSONUnixSeconds(entry.ProcessedDate) ||
		invalidJSONUnixSeconds(entry.CheckedDate) ||
		invalidJSONUnixSeconds(entry.StartedDate) ||
		invalidJSONUnixSeconds(entry.FailureStartedDate)
}

func (e *Engine) latestObservedTimestamp(name string, src *config.Source) (int64, bool) {
	var latest int64
	if points := e.bootstrapHistoryPoints(name); len(points) > 0 {
		for i := len(points) - 1; i >= 0; i-- {
			if validJSONUnixSeconds(points[i].Timestamp) {
				latest = points[i].Timestamp
				break
			}
		}
	}
	if stats, ok := e.currentSetStats(name, src); ok && validJSONUnixSeconds(stats.mtime) && stats.mtime > latest {
		latest = stats.mtime
	}
	return latest, latest > 0
}

func (e *Engine) firstObservedTimestamp(name string) (int64, bool) {
	points := e.bootstrapHistoryPoints(name)
	for _, point := range points {
		if validJSONUnixSeconds(point.Timestamp) {
			return point.Timestamp, true
		}
	}
	return 0, false
}
