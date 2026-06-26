package engine

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/firehol/update-ipsets/pkg/iprange"
)

var (
	runtimeLedgerLoadHookMu sync.Mutex
	runtimeLedgerLoadHook   func(kind, name string)
)

func setRuntimeLedgerLoadHookForTest(fn func(kind, name string)) func() {
	runtimeLedgerLoadHookMu.Lock()
	previous := runtimeLedgerLoadHook
	runtimeLedgerLoadHook = fn
	runtimeLedgerLoadHookMu.Unlock()

	return func() {
		runtimeLedgerLoadHookMu.Lock()
		runtimeLedgerLoadHook = previous
		runtimeLedgerLoadHookMu.Unlock()
	}
}

func runtimeLedgerLoadHookForTest() func(kind, name string) {
	runtimeLedgerLoadHookMu.Lock()
	defer runtimeLedgerLoadHookMu.Unlock()
	return runtimeLedgerLoadHook
}

func runRuntimeLedgerLoadHook(kind, name string) {
	if hook := runtimeLedgerLoadHookForTest(); hook != nil {
		hook(kind, name)
	}
}

func appendHistoryTail(tail []HistoryPoint, point HistoryPoint, limit int) []HistoryPoint {
	if limit <= 0 {
		return nil
	}
	tail = append(tail, point)
	if len(tail) > limit {
		tail = append([]HistoryPoint(nil), tail[len(tail)-limit:]...)
	}
	return tail
}

func appendChangesTail(tail []ChangesetPoint, point ChangesetPoint, limit int) []ChangesetPoint {
	if limit <= 0 {
		return nil
	}
	tail = append(tail, point)
	if len(tail) > limit {
		tail = append([]ChangesetPoint(nil), tail[len(tail)-limit:]...)
	}
	return tail
}

func readCSVLinesContext(ctx context.Context, path string, fn func(string) error) error {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return err
	}
	file, err := openFilePathUnderRoot(filepath.Dir(path), path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	reader := bufio.NewReader(file)
	first := true
	for {
		if err := contextErr(ctx); err != nil {
			return err
		}
		line, err := reader.ReadString('\n')
		if errors.Is(err, io.EOF) && line == "" {
			return nil
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			if errors.Is(err, io.EOF) {
				return nil
			}
			continue
		}
		if first {
			first = false
			if err := fn(line); err != nil {
				return err
			}
			if errors.Is(err, io.EOF) {
				return nil
			}
			continue
		}
		if err := fn(line); err != nil {
			return err
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
	}
}

func loadHistoryLedgerStateContext(ctx context.Context, path, name string, limit int) (historyLedgerStats, []HistoryPoint, error) {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return historyLedgerStats{}, nil, err
	}
	runRuntimeLedgerLoadHook("history", name)
	var stats historyLedgerStats
	if limit < 1 {
		limit = 1
	}
	tail := make([]HistoryPoint, 0, limit)
	var lastPoint HistoryPoint
	haveLast := false
	flushLast := func() {
		if !haveLast {
			return
		}
		stats.observe(lastPoint)
		tail = appendObservedHistoryTail(tail, lastPoint, limit)
	}
	lineNum := 0
	err := readCSVLinesContext(ctx, path, func(line string) error {
		lineNum++
		if lineNum == 1 && strings.EqualFold(line, "DateTime,Entries,UniqueIPs") {
			return nil
		}
		parts := strings.Split(line, ",")
		if len(parts) != 3 {
			return nil
		}
		ts, err1 := strconv.ParseInt(parts[0], 10, 64)
		entries, err2 := strconv.Atoi(parts[1])
		ips, err3 := strconv.ParseUint(parts[2], 10, 64)
		if err1 != nil || err2 != nil || err3 != nil || !validHistoryTimestamp(ts) {
			return nil
		}
		point := HistoryPoint{Timestamp: ts, Name: name, Entries: entries, UniqueIPs: ips}
		if haveLast && point.Timestamp > lastPoint.Timestamp {
			flushLast()
			haveLast = false
		}
		lastPoint = point
		haveLast = true
		return nil
	})
	if err != nil {
		return historyLedgerStats{}, nil, err
	}
	flushLast()
	return stats, tail, nil
}

func loadHistoryTailCSV(path, name string, limit int) ([]HistoryPoint, error) {
	return loadHistoryTailCSVContext(context.Background(), path, name, limit)
}

func loadHistoryTailCSVContext(ctx context.Context, path, name string, limit int) ([]HistoryPoint, error) {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if limit < 1 {
		limit = 1
	}
	tail := make([]HistoryPoint, 0, limit)
	var lastPoint HistoryPoint
	haveLast := false
	flushLast := func() {
		if !haveLast {
			return
		}
		tail = appendObservedHistoryTail(tail, lastPoint, limit)
	}
	lineNum := 0
	err := readCSVLinesContext(ctx, path, func(line string) error {
		lineNum++
		if lineNum == 1 && strings.EqualFold(line, "DateTime,Entries,UniqueIPs") {
			return nil
		}
		parts := strings.Split(line, ",")
		if len(parts) != 3 {
			return nil
		}
		ts, err1 := strconv.ParseInt(parts[0], 10, 64)
		entries, err2 := strconv.Atoi(parts[1])
		ips, err3 := strconv.ParseUint(parts[2], 10, 64)
		if err1 != nil || err2 != nil || err3 != nil || !validHistoryTimestamp(ts) {
			return nil
		}
		point := HistoryPoint{
			Timestamp: ts,
			Name:      name,
			Entries:   entries,
			UniqueIPs: ips,
		}
		if haveLast && point.Timestamp > lastPoint.Timestamp {
			flushLast()
			haveLast = false
		}
		lastPoint = point
		haveLast = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	flushLast()
	return tail, nil
}

func (e *Engine) historyTailBootstrap(name string) []HistoryPoint {
	if e == nil {
		return nil
	}
	rt := e.Runtime()
	limit := webChartsEntriesFromRuntime(rt)
	if limit < 1 {
		return nil
	}
	if rt.WebDir != "" {
		path := filepath.Join(rt.WebDir, name+"_history.csv")
		if tail, err := loadHistoryTailCSV(path, name, limit); err == nil && len(tail) > 0 {
			return tail
		}
	}
	if rt.LibDir != "" {
		path := filepath.Join(rt.LibDir, name, "history.csv")
		if tail, err := loadHistoryTailCSV(path, name, limit); err == nil && len(tail) > 0 {
			return tail
		}
	}
	return nil
}

func appendObservedHistoryTail(tail []HistoryPoint, point HistoryPoint, limit int) []HistoryPoint {
	if len(tail) > 0 {
		last := tail[len(tail)-1]
		if last.Timestamp == point.Timestamp {
			tail[len(tail)-1] = point
			return tail
		}
		if last.Timestamp > point.Timestamp {
			return tail
		}
	}
	return appendHistoryTail(tail, point, limit)
}

func loadRetentionPastContext(ctx context.Context, path string, started int64) (map[int]uint64, error) {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	runRuntimeLedgerLoadHook("retention_past", "")
	past := make(map[int]uint64)
	lineNum := 0
	err := readCSVLinesContext(ctx, path, func(line string) error {
		lineNum++
		if lineNum == 1 && strings.EqualFold(line, "date_removed,date_added,hours,ips") {
			return nil
		}
		parts := strings.Split(line, ",")
		if len(parts) != 4 {
			return nil
		}
		addedAt, err1 := strconv.ParseInt(parts[1], 10, 64)
		hours, err2 := strconv.Atoi(parts[2])
		ips, err3 := strconv.ParseUint(parts[3], 10, 64)
		if err1 != nil || err2 != nil || err3 != nil || addedAt <= started {
			return nil
		}
		past[hours] += ips
		return nil
	})
	if err != nil {
		return nil, err
	}
	return past, nil
}

func copyRetentionPast(in map[int]uint64) map[int]uint64 {
	if len(in) == 0 {
		return map[int]uint64{}
	}
	out := make(map[int]uint64, len(in))
	for hour, ips := range in {
		out[hour] = ips
	}
	return out
}

func loadRetentionCohortIndexContext(ctx context.Context, path string) (map[int64]uint64, error) {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	runRuntimeLedgerLoadHook("retention_cohorts", "")
	out := map[int64]uint64{}
	lineNum := 0
	err := readCSVLinesContext(ctx, path, func(line string) error {
		lineNum++
		if lineNum == 1 && strings.EqualFold(line, "date_added,ips") {
			return nil
		}
		parts := strings.Split(line, ",")
		if len(parts) != 2 {
			return nil
		}
		addedAt, err1 := strconv.ParseInt(parts[0], 10, 64)
		ips, err2 := strconv.ParseUint(parts[1], 10, 64)
		if err1 != nil || err2 != nil || addedAt <= 0 || ips == 0 {
			return nil
		}
		out[addedAt] = ips
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func writeRetentionCohortIndex(path string, cohorts map[int64]uint64) error {
	var lines []string
	lines = append(lines, "date_added,ips")
	keys := make([]int64, 0, len(cohorts))
	for addedAt, ips := range cohorts {
		if addedAt <= 0 || ips == 0 {
			continue
		}
		keys = append(keys, addedAt)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, addedAt := range keys {
		lines = append(lines, fmt.Sprintf("%d,%d", addedAt, cohorts[addedAt]))
	}
	return writeFileAtomic(path, []byte(strings.Join(lines, "\n")+"\n"), generatedFileMode)
}

func loadRetentionCohorts(ctx context.Context, dir string) (map[int64]uint64, error) {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	indexPath := filepath.Join(dir, "retention_cohorts.csv")
	if cohorts, err := loadRetentionCohortIndexContext(ctx, indexPath); err == nil && len(cohorts) > 0 {
		return cohorts, nil
	} else if err != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
		return nil, err
	}

	newDir := filepath.Join(dir, "new")
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(newDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[int64]uint64{}, nil
		}
		return nil, err
	}
	out := make(map[int64]uint64, len(entries))
	for _, entry := range entries {
		if err := contextErr(ctx); err != nil {
			return nil, err
		}
		if entry.IsDir() || isIgnoredRetentionSnapshotName(entry.Name()) {
			continue
		}
		tsStr := strings.TrimSuffix(entry.Name(), ".set")
		addedAt, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			continue
		}
		filePath := filepath.Join(newDir, entry.Name())
		meta, err := iprange.ReadFileSetMetadata(filePath)
		if err == nil {
			out[addedAt] = meta.UniqueIPs
			continue
		}
		set, err := loadSnapshotSet(ctx, entry.Name(), dir, filepath.Join("new", entry.Name()))
		if err != nil {
			return nil, err
		}
		out[addedAt] = set.UniqueCount()
	}
	return out, nil
}

func buildCurrentRetentionBuckets(cohorts map[int64]uint64, updatedAt, started int64) (map[int]uint64, int) {
	current := make(map[int]uint64, len(cohorts))
	incomplete := 0
	for addedAt, ips := range cohorts {
		if ips == 0 {
			continue
		}
		hours := int((updatedAt + 1800 - addedAt) / 3600)
		current[hours] += ips
		if addedAt <= started {
			incomplete = 1
		}
	}
	return current, incomplete
}
