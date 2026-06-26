package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (e *Engine) HistorySeries(name string) ([]HistoryPoint, error) {
	return e.HistorySeriesContext(context.Background(), name)
}

func (e *Engine) HistorySeriesContext(ctx context.Context, name string) ([]HistoryPoint, error) {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	// Read from the internal full ledger first. Bash keeps
	// LIB_DIR/<feed>/history.csv append-only and generates public
	// <feed>_history.csv as a last-N window from it.
	points := e.historyFromLedgerCSVContext(ctx, name)
	if len(points) == 0 {
		// Compatibility fallback for Go rewrite data written before the
		// internal ledger was restored.
		points = e.historyFromWebCSVContext(ctx, name)
	}

	if len(points) == 0 {
		return nil, nil
	}
	return points, nil
}

// ChangesetSeries returns the (timestamp, added, removed) tuples written to
// <LibDir>/<name>/changesets.csv by the retention step. Each tuple corresponds
// to exactly one successful update where the binary set changed. A missing file
// returns an empty slice and a nil error - young feeds or feeds that never
// changed since tracking started simply have no changesets yet.
//
// The results match the bash public changeset window: ignore historical
// zero-delta rows, drop the bootstrap row, then return the last
// WebChartsEntries real changes.
func (e *Engine) ChangesetSeries(name string) ([]ChangesetPoint, error) {
	return e.ChangesetSeriesContext(context.Background(), name)
}

func (e *Engine) ChangesetSeriesContext(ctx context.Context, name string) ([]ChangesetPoint, error) {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	out, err := e.readChangesetLedgerContext(ctx, name)
	if err != nil {
		return nil, err
	}
	if len(out) > 0 {
		out = out[1:]
	}
	window := e.webChartsEntries()
	if len(out) > window {
		out = out[len(out)-window:]
	}
	return out, nil
}

// PublicChangesetSeries reads the already-published web changeset artifact.
// It intentionally does not fall back to the internal ledger because public
// requests must not regenerate missing artifacts.
func (e *Engine) PublicChangesetSeries(name string) ([]ChangesetPoint, error) {
	return e.PublicChangesetSeriesInDir(name, e.outputDir())
}

func (e *Engine) PublicChangesetSeriesInDir(name, dir string) ([]ChangesetPoint, error) {
	if _, err := e.Entry(name); err != nil {
		return nil, err
	}
	if dir == "" {
		dir = e.outputDir()
	}
	data, err := readFileInRoot(dir, name+"_changesets.csv")
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no changeset data for %q", name)
		}
		return nil, err
	}
	return parseChangesetCSVData(data), nil
}

func (e *Engine) readChangesetLedgerContext(ctx context.Context, name string) ([]ChangesetPoint, error) {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if e == nil {
		return nil, nil
	}
	rt := e.Runtime()
	if rt.LibDir == "" {
		return nil, nil
	}
	data, err := readFileInRoot(rt.LibDir, filepath.Join(name, "changesets.csv"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	return parseChangesetCSVData(data), nil
}

// historyFromLedgerCSV reads the internal append-only history ledger.
func (e *Engine) historyFromLedgerCSV(name string) []HistoryPoint {
	return e.historyFromLedgerCSVContext(context.Background(), name)
}

func (e *Engine) historyFromLedgerCSVContext(ctx context.Context, name string) []HistoryPoint {
	ctx = nonNilContext(ctx)
	if contextErr(ctx) != nil {
		return nil
	}
	if e == nil {
		return nil
	}
	rt := e.Runtime()
	if rt.LibDir == "" {
		return nil
	}
	return parseHistoryCSVInRootContext(ctx, rt.LibDir, filepath.Join(name, "history.csv"), name)
}

func (e *Engine) historyFromWebCSVContext(ctx context.Context, name string) []HistoryPoint {
	ctx = nonNilContext(ctx)
	if contextErr(ctx) != nil {
		return nil
	}
	dir := e.outputDir()
	if dir == "" {
		return nil
	}
	return parseHistoryCSVInRootContext(ctx, dir, name+"_history.csv", name)
}

// parseHistoryCSV reads a CSV with header "DateTime,Entries,UniqueIPs".
func parseHistoryCSVInRoot(rootDir, rel, name string) []HistoryPoint {
	return parseHistoryCSVInRootContext(context.Background(), rootDir, rel, name)
}

func parseHistoryCSVInRootContext(ctx context.Context, rootDir, rel, name string) []HistoryPoint {
	ctx = nonNilContext(ctx)
	if contextErr(ctx) != nil {
		return nil
	}
	data, err := readFileInRoot(rootDir, rel)
	if err != nil {
		return nil
	}
	if contextErr(ctx) != nil {
		return nil
	}
	return parseHistoryCSVData(data, name)
}

func parseHistoryCSVData(data []byte, name string) []HistoryPoint {
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) <= 1 {
		return nil
	}
	points := make([]HistoryPoint, 0, len(lines)-1)
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) != 3 {
			continue
		}
		ts, err1 := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
		entries, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		ips, err3 := strconv.ParseUint(strings.TrimSpace(parts[2]), 10, 64)
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		if !validHistoryTimestamp(ts) {
			continue
		}
		points = append(points, HistoryPoint{
			Timestamp: ts,
			Name:      name,
			Entries:   entries,
			UniqueIPs: ips,
		})
	}
	return points
}

func parseChangesetCSVData(data []byte) []ChangesetPoint {
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) <= 1 {
		return nil
	}
	out := make([]ChangesetPoint, 0, len(lines)-1)
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if point, ok := parseChangesetCSVLine(line); ok {
			out = append(out, point)
		}
	}
	return out
}

func parseChangesetCSVLine(line string) (ChangesetPoint, bool) {
	tsText, rest, ok := strings.Cut(line, ",")
	if !ok {
		return ChangesetPoint{}, false
	}
	addedText, removedText, ok := strings.Cut(rest, ",")
	if !ok || strings.Contains(removedText, ",") {
		return ChangesetPoint{}, false
	}
	ts, err := parseInt64(tsText)
	if err != nil {
		return ChangesetPoint{}, false
	}
	added, err := parseUint64(addedText)
	if err != nil {
		return ChangesetPoint{}, false
	}
	removed, err := parseUint64(removedText)
	if err != nil {
		return ChangesetPoint{}, false
	}
	if added == 0 && removed == 0 {
		return ChangesetPoint{}, false
	}
	return ChangesetPoint{
		Timestamp: ts,
		Added:     added,
		Removed:   removed,
	}, true
}

func validHistoryTimestamp(ts int64) bool {
	const (
		minHistoryUnix = 946684800  // 2000-01-01T00:00:00Z
		maxHistoryUnix = 4102444800 // 2100-01-01T00:00:00Z
	)
	return ts >= minHistoryUnix && ts <= maxHistoryUnix
}
