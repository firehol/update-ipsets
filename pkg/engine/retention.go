package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/pkg/iprange"
)

// loadSnapshotSet reads a stored set from disk, transparently handling
// both the iprange binary format written by the bash and Go engines and
// text fallback files that may exist from manual repairs or earlier
// migration experiments. We use iprange.ParseReader because it peeks at
// the first bytes to detect the binary header and falls back to text
// parsing otherwise.
//
// Without this, the retention loader was calling ReadBinary directly
// and crashing with "expecting binary header but found …" when a
// snapshot is not binary. The failure killed the entire feed update
// with last_status: retention_failed.
func loadSnapshotSet(ctx context.Context, name, path string) (*iprange.IPSet, error) {
	ctx = nonNilContext(ctx)
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	return iprange.ParseReader(ctx, name, file, iprange.DefaultParseOptions())
}

// loadLatestSet reads the previous latest snapshot for a given name.
// Returns an empty set if the file does not exist. This must be called
// before finalize() overwrites latest with the new data.
func (e *Engine) loadLatestSet(ctx context.Context, name string) (*iprange.IPSet, error) {
	latestPath := filepath.Join(e.runtime.LibDir, name, "latest")
	if !fileExists(latestPath) {
		// Fallback: earlier Go builds used "latest.set".
		latestPath = filepath.Join(e.runtime.LibDir, name, "latest.set")
		if !fileExists(latestPath) {
			return iprange.New(name), nil
		}
	}
	return loadSnapshotSet(ctx, name, latestPath)
}

func isIgnoredRetentionSnapshotName(name string) bool {
	return strings.HasPrefix(name, ".")
}

func (e *Engine) updateRetention(ctx context.Context, name string, previous, current *iprange.IPSet, updatedAt time.Time) error {
	ctx = nonNilContext(ctx)
	dir := filepath.Join(e.runtime.LibDir, name)
	newDir := filepath.Join(dir, "new")
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		return err
	}
	updatedAtUnix := updatedAt.UTC().Unix()

	newSet := iprange.Exclude(current, previous)
	removedSet := iprange.Exclude(previous, current)
	added := newSet.UniqueCount()
	removed := removedSet.UniqueCount()
	if added > 0 || removed > 0 {
		changesetsPath := filepath.Join(dir, "changesets.csv")
		if err := normalizeChangesetLedgerHeader(changesetsPath); err != nil {
			return err
		}
		if err := appendCSV(changesetsPath, changesetLedgerHeader,
			fmt.Sprintf("%d,%d,%d\n", updatedAtUnix, added, removed)); err != nil {
			return err
		}
		e.observeChangesetPoint(name, ChangesetPoint{
			Timestamp: updatedAtUnix,
			Added:     added,
			Removed:   removed,
		})
	}
	if added > 0 {
		if err := writeBinaryPath(filepath.Join(newDir, fmt.Sprintf("%d", updatedAtUnix)), newSet, updatedAt); err != nil {
			return err
		}
		e.observeRetentionCohort(name, updatedAtUnix, added)
	}
	if err := ensureCSVHeader(filepath.Join(dir, "retention.csv"), "date_removed,date_added,hours,ips\n"); err != nil {
		return err
	}
	started := e.state.Entry(name).Snapshot().StartedDate
	if started == 0 {
		started = updatedAtUnix
	}
	past := e.retentionPastFromRuntime(name, started)

	if removed == 0 {
		cohorts := e.retentionCohortsFromRuntime(ctx, name)
		currentBuckets, incomplete := buildCurrentRetentionBuckets(cohorts, updatedAtUnix, started)
		retention := buildRetentionDataFromBuckets(name, started, updatedAtUnix, incomplete, past, currentBuckets)
		data, err := jsonMarshalTabIndent(retention)
		if err != nil {
			return err
		}
		if err := writeRetentionCohortIndex(filepath.Join(dir, "retention_cohorts.csv"), cohorts); err != nil {
			return err
		}
		if err := writeRetentionHistogramCache(filepath.Join(dir, "histogram"), retention); err != nil {
			return err
		}
		return writeFileAtomic(filepath.Join(dir, "retention.json"), append(data, '\n'), 0o644)
	}

	currentBuckets := map[int]uint64{}
	incomplete := 0
	cohorts := make(map[int64]uint64)

	files, err := os.ReadDir(newDir)
	if err != nil {
		return err
	}
	for _, entry := range files {
		if entry.IsDir() {
			continue
		}
		if isIgnoredRetentionSnapshotName(entry.Name()) {
			continue
		}
		// Accept both "1234567.set" (Go convention) and "1234567" (bash convention).
		baseName := entry.Name()
		tsStr := strings.TrimSuffix(baseName, ".set")
		addedAt, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			e.logger.Warn("retention: skipping malformed filename", "source", name, "file", baseName, "error", err)
			continue
		}
		path := filepath.Join(newDir, baseName)
		// Use loadSnapshotSet (ParseReader-backed) so legacy text-format
		// snapshots from the bash version are accepted alongside the
		// binary format the Go engine writes for new snapshots. The
		// bogons feed in particular has retention snapshot files going
		// back to 2015 that are still in plain text and must keep
		// loading correctly.
		oldSet, err := loadSnapshotSet(ctx, entry.Name(), path)
		if err != nil {
			return err
		}
		still := iprange.Intersect(oldSet, current)
		removedSet := iprange.Exclude(oldSet, still)
		stillCount := still.UniqueCount()
		removedCount := removedSet.UniqueCount()
		hours := int((updatedAtUnix + 1800 - addedAt) / 3600)
		if removedCount > 0 {
			if err := appendCSV(filepath.Join(dir, "retention.csv"), "date_removed,date_added,hours,ips\n",
				fmt.Sprintf("%d,%d,%d,%d\n", updatedAtUnix, addedAt, hours, removedCount)); err != nil {
				return err
			}
			if addedAt > started {
				past[hours] += removedCount
				e.observeRetentionPast(name, started, hours, removedCount)
			}
		}
		if stillCount == 0 {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			continue
		}
		cohorts[addedAt] = stillCount
		currentBuckets[hours] += stillCount
		if addedAt <= started {
			incomplete = 1
		}
		if removedCount == 0 {
			continue
		}
		if err := writeBinaryPath(path, still, time.Unix(addedAt, 0).UTC()); err != nil {
			return err
		}
	}
	e.replaceRetentionCohorts(name, cohorts)
	if err := writeRetentionCohortIndex(filepath.Join(dir, "retention_cohorts.csv"), cohorts); err != nil {
		return err
	}

	retention := buildRetentionDataFromBuckets(name, started, updatedAtUnix, incomplete, past, currentBuckets)
	data, err := jsonMarshalTabIndent(retention)
	if err != nil {
		return err
	}
	if err := writeRetentionHistogramCache(filepath.Join(dir, "histogram"), retention); err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(dir, "retention.json"), append(data, '\n'), 0o644)
}

func (e *Engine) buildRetentionData(ctx context.Context, name string, updatedAt int64) (*RetentionData, error) {
	ctx = nonNilContext(ctx)
	dir := filepath.Join(e.runtime.LibDir, name)
	entry := e.state.Entry(name)
	started := entry.StartedDate
	if started == 0 {
		started = updatedAt
	}
	past := map[int]uint64{}
	current := map[int]uint64{}
	// Bash sets incomplete=0 before scanning, then sets it to 1 if any
	// retention file's date <= started. We use int (0/1) to match the JSON.
	incomplete := 0

	retentionCSV := filepath.Join(dir, "retention.csv")
	if data, err := os.ReadFile(retentionCSV); err == nil {
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		for _, line := range lines[1:] {
			if strings.TrimSpace(line) == "" {
				continue
			}
			parts := strings.Split(line, ",")
			if len(parts) != 4 {
				e.logger.Warn("retention: skipping malformed CSV line", "source", name, "line", line)
				continue
			}
			addedAt, err1 := strconv.ParseInt(parts[1], 10, 64)
			hours, err2 := strconv.Atoi(parts[2])
			ips, err3 := strconv.ParseUint(parts[3], 10, 64)
			if err1 != nil || err2 != nil || err3 != nil {
				e.logger.Warn("retention: skipping unparseable CSV values", "source", name, "line", line)
				continue
			}
			if addedAt > started {
				past[hours] += ips
			}
		}
	}

	newDir := filepath.Join(dir, "new")
	files, err := os.ReadDir(newDir)
	if err == nil {
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			if isIgnoredRetentionSnapshotName(file.Name()) {
				continue
			}
			// Accept both "1234567.set" (Go) and "1234567" (bash).
			tsStr := strings.TrimSuffix(file.Name(), ".set")
			addedAt, err := strconv.ParseInt(tsStr, 10, 64)
			if err != nil {
				e.logger.Warn("retention: skipping malformed new-set filename", "source", name, "file", file.Name(), "error", err)
				continue
			}
			// loadSnapshotSet handles both binary and legacy text
			// snapshots so the bash-era files in lib/{name}/new/
			// keep loading correctly across the format migration.
			set, err := loadSnapshotSet(ctx, file.Name(), filepath.Join(newDir, file.Name()))
			if err != nil {
				return nil, err
			}
			hours := int((updatedAt + 1800 - addedAt) / 3600)
			current[hours] += set.UniqueCount()
			if addedAt <= started {
				incomplete = 1
			}
		}
	}

	return &RetentionData{
		Name:       name,
		Started:    millis(started),
		Updated:    millis(updatedAt),
		Incomplete: incomplete,
		Past:       retentionSeries(past),
		Current:    retentionSeries(current),
	}, nil
}

func buildRetentionDataFromBuckets(name string, started, updatedAt int64, incomplete int, past, current map[int]uint64) *RetentionData {
	if past == nil {
		past = map[int]uint64{}
	}
	if current == nil {
		current = map[int]uint64{}
	}
	return &RetentionData{
		Name:       name,
		Started:    millis(started),
		Updated:    millis(updatedAt),
		Incomplete: incomplete,
		Past:       retentionSeries(past),
		Current:    retentionSeries(current),
	}
}

func writeRetentionHistogramCache(path string, retention *RetentionData) error {
	if retention == nil {
		return nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "declare -- RETENTION_HISTOGRAM_STARTED=\"%d\"\n", retention.Started/1000)
	fmt.Fprintf(&b, "declare -- RETENTION_HISTOGRAM_INCOMPLETE=\"%d\"\n", retention.Incomplete)
	writeBashArrayDeclare(&b, "RETENTION_HISTOGRAM", retention.Past)
	writeBashArrayDeclare(&b, "RETENTION_HISTOGRAM_REST", retention.Current)
	return writeFileAtomic(path, []byte(b.String()), 0o644)
}

func writeBashArrayDeclare(b *strings.Builder, name string, series RetentionSeries) {
	fmt.Fprintf(b, "declare -a %s=(", name)
	wrote := false
	for i, hour := range series.Hours {
		if i >= len(series.IPs) || series.IPs[i] == 0 {
			continue
		}
		if wrote {
			b.WriteByte(' ')
		}
		fmt.Fprintf(b, "[%d]=\"%d\"", hour, series.IPs[i])
		wrote = true
	}
	b.WriteString(")\n")
}

func retentionSeries(values map[int]uint64) RetentionSeries {
	hours := make([]int, 0, len(values))
	for hour, count := range values {
		if count == 0 {
			continue
		}
		hours = append(hours, hour)
	}
	slices.Sort(hours)
	ips := make([]uint64, 0, len(hours))
	var total uint64
	for _, hour := range hours {
		ips = append(ips, values[hour])
		total += values[hour]
	}
	return RetentionSeries{
		Hours: hours,
		IPs:   ips,
		Total: total,
	}
}
