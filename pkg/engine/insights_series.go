package engine

import "github.com/firehol/update-ipsets/pkg/insights"

func singleCandidatePath(baseDir string, elems ...string) []rootedCandidatePath {
	if baseDir == "" {
		return nil
	}
	return []rootedCandidatePath{{rootDir: baseDir, rel: joinRelativePath(elems...)}}
}

// readInsightsHistoryPoints is the waste-elimination path for insights:
// prefer the already-bounded public history artifact from the staged or live
// web output, then fall back to the internal ledger only when the public file
// does not exist. Unlike HistorySeries(), this never scans history snapshots.
func (e *Engine) readInsightsHistoryPoints(name, outDir string) []HistoryPoint {
	data, err := readFirstExisting(
		singleCandidatePath(outDir, name+"_history.csv"),
		singleCandidatePath(e.outputDir(), name+"_history.csv"),
	)
	if err == nil {
		return parseHistoryCSVData(data, name)
	}
	return e.historyFromLedgerCSV(name)
}

// readInsightsChangesets mirrors readInsightsHistoryPoints for churn:
// prefer the already-bounded public changeset artifact, then fall back to the
// internal ledger only when the public file is absent. This keeps the insights
// hot path off the full-history internals.
func (e *Engine) readInsightsChangesets(name, outDir string) []ChangesetPoint {
	data, err := readFirstExisting(
		singleCandidatePath(outDir, name+"_changesets.csv"),
		singleCandidatePath(e.outputDir(), name+"_changesets.csv"),
	)
	if err == nil {
		return parseChangesetCSVData(data)
	}
	points, err := e.ChangesetSeries(name)
	if err != nil {
		return nil
	}
	return points
}

// readInsightsSizeSeries loads the last WebChartsEntries points for the feed's
// history using bounded public artifacts first. Returns nil if history is
// unavailable.
func (e *Engine) readInsightsSizeSeries(name, outDir string) []insights.SizePoint {
	points := e.readInsightsHistoryPoints(name, outDir)
	if len(points) == 0 {
		return nil
	}
	points = trimHistoryWindow(points, e.webChartsEntries())
	out := make([]insights.SizePoint, 0, len(points))
	for _, p := range points {
		out = append(out, insights.SizePoint{TS: p.Timestamp, Size: p.UniqueIPs})
	}
	return out
}

// readInsightsChurnSeries derives churn points from the staged/live bounded
// public changeset artifact first, falling back to the internal ledger only
// when that public artifact is missing.
func (e *Engine) readInsightsChurnSeries(name, outDir string, sizes []insights.SizePoint) []insights.ChurnPoint {
	sizesByTS := make(map[int64]uint64, len(sizes))
	for _, p := range sizes {
		sizesByTS[p.TS] = p.Size
	}
	changes := e.readInsightsChangesets(name, outDir)
	if len(changes) == 0 {
		return nil
	}
	out := make([]insights.ChurnPoint, 0, len(changes))
	for _, change := range changes {
		size := sizesByTS[change.Timestamp]
		if size == 0 {
			// Changeset may be slightly off-grid from the history
			// timestamp - skip until we can correlate it.
			continue
		}
		kept := size
		if change.Removed <= kept {
			kept = size - change.Removed
		}
		out = append(out, insights.ChurnPoint{
			TS:      change.Timestamp,
			Added:   change.Added,
			Removed: change.Removed,
			Kept:    kept,
			Size:    size,
		})
	}
	return out
}

// readSizeSeries loads the last WebChartsEntries points of the feed's history
// as SizePoints. Returns nil if history is unavailable.
func (e *Engine) readSizeSeries(name string) []insights.SizePoint {
	points := e.historyTailFromRuntime(name)
	if len(points) == 0 {
		var err error
		points, err = e.HistorySeries(name)
		if err != nil || len(points) == 0 {
			return nil
		}
		points = trimHistoryWindow(points, e.webChartsEntries())
	}
	out := make([]insights.SizePoint, 0, len(points))
	for _, p := range points {
		out = append(out, insights.SizePoint{TS: p.Timestamp, Size: p.UniqueIPs})
	}
	return out
}

// readChurnSeries derives churn points from changesets.csv, correlating each
// recorded content change with the size reported by the corresponding history
// entry. Missing changesets mean there is no churn series yet.
func (e *Engine) readChurnSeries(name string, sizes []insights.SizePoint) []insights.ChurnPoint {
	sizesByTS := make(map[int64]uint64, len(sizes))
	for _, p := range sizes {
		sizesByTS[p.TS] = p.Size
	}
	changes := e.changesetTailFromRuntime(name)
	if len(changes) == 0 {
		var err error
		changes, err = e.ChangesetSeries(name)
		if err != nil || len(changes) == 0 {
			return nil
		}
	}
	out := make([]insights.ChurnPoint, 0, len(changes))
	for _, change := range changes {
		size := sizesByTS[change.Timestamp]
		if size == 0 {
			// Changeset may be slightly off-grid from the history
			// timestamp - skip until we can correlate it.
			continue
		}
		kept := size
		if change.Removed <= kept {
			kept = size - change.Removed
		}
		out = append(out, insights.ChurnPoint{
			TS:      change.Timestamp,
			Added:   change.Added,
			Removed: change.Removed,
			Kept:    kept,
			Size:    size,
		})
	}
	return out
}
