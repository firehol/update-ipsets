package engine

import (
	"context"

	"github.com/firehol/update-ipsets/pkg/insights"
)

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
	return e.readInsightsHistoryPointsWithSnapshot(e.operationSnapshot(), name, outDir)
}

func (e *Engine) readInsightsHistoryPointsWithSnapshot(snap operationSnapshot, name, outDir string) []HistoryPoint {
	data, err := readFirstExisting(
		singleCandidatePath(outDir, name+"_history.csv"),
		singleCandidatePath(outputDirForRuntime(snap.runtime), name+"_history.csv"),
	)
	if err == nil {
		return parseHistoryCSVData(data, name)
	}
	return e.historyFromLedgerCSVContextWithRuntime(context.Background(), snap.runtime, name)
}

// readInsightsChangesets mirrors readInsightsHistoryPoints for churn:
// prefer the already-bounded public changeset artifact, then fall back to the
// internal ledger only when the public file is absent. This keeps the insights
// hot path off the full-history internals.
func (e *Engine) readInsightsChangesets(name, outDir string) []ChangesetPoint {
	return e.readInsightsChangesetsWithSnapshot(e.operationSnapshot(), name, outDir)
}

func (e *Engine) readInsightsChangesetsWithSnapshot(snap operationSnapshot, name, outDir string) []ChangesetPoint {
	data, err := readFirstExisting(
		singleCandidatePath(outDir, name+"_changesets.csv"),
		singleCandidatePath(outputDirForRuntime(snap.runtime), name+"_changesets.csv"),
	)
	if err == nil {
		return parseChangesetCSVData(data)
	}
	points, err := e.ChangesetSeriesContextWithRuntime(context.Background(), snap.runtime, name)
	if err != nil {
		return nil
	}
	return points
}

// readInsightsSizeSeries loads the last WebChartsEntries points for the feed's
// history using bounded public artifacts first. Returns nil if history is
// unavailable.
func (e *Engine) readInsightsSizeSeries(name, outDir string) []insights.SizePoint {
	return e.readInsightsSizeSeriesWithSnapshot(e.operationSnapshot(), name, outDir)
}

func (e *Engine) readInsightsSizeSeriesWithSnapshot(snap operationSnapshot, name, outDir string) []insights.SizePoint {
	points := e.readInsightsHistoryPointsWithSnapshot(snap, name, outDir)
	if len(points) == 0 {
		return nil
	}
	points = trimHistoryWindow(points, webChartsEntriesFromRuntime(snap.runtime))
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
	return e.readInsightsChurnSeriesWithSnapshot(e.operationSnapshot(), name, outDir, sizes)
}

func (e *Engine) readInsightsChurnSeriesWithSnapshot(snap operationSnapshot, name, outDir string, sizes []insights.SizePoint) []insights.ChurnPoint {
	sizesByTS := make(map[int64]uint64, len(sizes))
	for _, p := range sizes {
		sizesByTS[p.TS] = p.Size
	}
	changes := e.readInsightsChangesetsWithSnapshot(snap, name, outDir)
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
	return e.readSizeSeriesWithSnapshot(e.operationSnapshot(), name)
}

func (e *Engine) readSizeSeriesWithRuntime(rt Runtime, name string) []insights.SizePoint {
	return e.readSizeSeriesWithSnapshot(operationSnapshot{runtime: rt}, name)
}

func (e *Engine) readSizeSeriesWithSnapshot(snap operationSnapshot, name string) []insights.SizePoint {
	points := e.historyTailFromSnapshot(context.Background(), snap, name)
	if len(points) == 0 {
		points = e.historyFromLedgerCSVContextWithRuntime(context.Background(), snap.runtime, name)
		if len(points) == 0 {
			points = e.historyFromWebCSVContextWithRuntime(context.Background(), snap.runtime, name)
		}
		if len(points) == 0 {
			return nil
		}
		window := webChartsEntriesFromRuntime(snap.runtime)
		if len(points) > window {
			points = points[len(points)-window:]
		}
	}
	out := make([]insights.SizePoint, 0, len(points))
	for _, p := range points {
		out = append(out, insights.SizePoint{TS: p.Timestamp, Size: p.UniqueIPs})
	}
	return out
}

func (e *Engine) readChurnSeries(name string, sizes []insights.SizePoint) []insights.ChurnPoint {
	return e.readChurnSeriesWithSnapshot(e.operationSnapshot(), name, sizes)
}

func (e *Engine) readChurnSeriesWithRuntime(rt Runtime, name string, sizes []insights.SizePoint) []insights.ChurnPoint {
	return e.readChurnSeriesWithSnapshot(operationSnapshot{runtime: rt}, name, sizes)
}

func (e *Engine) readChurnSeriesWithSnapshot(snap operationSnapshot, name string, sizes []insights.SizePoint) []insights.ChurnPoint {
	sizesByTS := make(map[int64]uint64, len(sizes))
	for _, p := range sizes {
		sizesByTS[p.TS] = p.Size
	}
	changes := e.changesetTailFromSnapshot(context.Background(), snap, name)
	if len(changes) == 0 {
		var err error
		changes, err = e.ChangesetSeriesContextWithRuntime(context.Background(), snap.runtime, name)
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
