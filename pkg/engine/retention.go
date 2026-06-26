package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

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
func loadSnapshotSet(ctx context.Context, name, rootDir, rel string) (*iprange.IPSet, error) {
	ctx = nonNilContext(ctx)
	file, err := openFileInRoot(rootDir, rel)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	return iprange.ParseReader(ctx, name, file, iprange.DefaultParseOptions())
}

func (e *Engine) openPreviousLatestSet(ctx context.Context, name string) (*closableSource, error) {
	return e.openPreviousLatestSetWithRuntime(ctx, e.Runtime(), name)
}

func (e *Engine) openPreviousLatestSetWithRuntime(ctx context.Context, rt Runtime, name string) (*closableSource, error) {
	for _, filename := range []string{"latest", "latest.set"} {
		rel := filepath.Join(name, filename)
		path := filepath.Join(rt.LibDir, rel)
		if !fileExists(path) {
			continue
		}
		fs, err := iprange.OpenFileSet(path)
		if err == nil {
			return &closableSource{RangeSource: fs, close: fs.Close}, nil
		}
		set, loadErr := loadSnapshotSet(ctx, name, rt.LibDir, rel)
		if loadErr != nil {
			return nil, loadErr
		}
		return &closableSource{RangeSource: set, close: nil}, nil
	}
	return &closableSource{RangeSource: iprange.New(name), close: nil}, nil
}

func isIgnoredRetentionSnapshotName(name string) bool {
	return strings.HasPrefix(name, ".")
}

func (e *Engine) buildRetentionData(ctx context.Context, name string, updatedAt int64) (*RetentionData, error) {
	return e.buildRetentionDataWithRuntime(ctx, e.Runtime(), name, updatedAt)
}

func (e *Engine) buildRetentionDataWithRuntime(ctx context.Context, rt Runtime, name string, updatedAt int64) (*RetentionData, error) {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	dir := filepath.Join(rt.LibDir, name)
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

	if data, err := readFileInRoot(rt.LibDir, filepath.Join(name, "retention.csv")); err == nil {
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		for _, line := range lines[1:] {
			if err := contextErr(ctx); err != nil {
				return nil, err
			}
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
			if err := contextErr(ctx); err != nil {
				return nil, err
			}
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
			set, err := loadSnapshotSet(ctx, file.Name(), rt.LibDir, filepath.Join(name, "new", file.Name()))
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
	return writeFileAtomic(path, []byte(b.String()), generatedFileMode)
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
