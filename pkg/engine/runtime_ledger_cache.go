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

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

type historyLedgerStats struct {
	firstTS       int64
	lastTS        int64
	count         int
	totalGapSecs  int64
	minGapSecs    int64
	maxGapSecs    int64
	entriesMin    int
	entriesMax    int
	ipsMin        uint64
	ipsMax        uint64
	lastEntries   int
	lastUniqueIPs uint64
}

func (s *historyLedgerStats) observe(point HistoryPoint) {
	if !validHistoryTimestamp(point.Timestamp) {
		return
	}
	if s.count == 0 {
		s.firstTS = point.Timestamp
		s.lastTS = point.Timestamp
		s.count = 1
		s.entriesMin = point.Entries
		s.entriesMax = point.Entries
		s.ipsMin = point.UniqueIPs
		s.ipsMax = point.UniqueIPs
		s.lastEntries = point.Entries
		s.lastUniqueIPs = point.UniqueIPs
		return
	}
	if point.Timestamp <= s.lastTS {
		return
	}
	gap := point.Timestamp - s.lastTS
	s.lastTS = point.Timestamp
	s.count++
	s.totalGapSecs += gap
	if s.minGapSecs == 0 || gap < s.minGapSecs {
		s.minGapSecs = gap
	}
	if gap > s.maxGapSecs {
		s.maxGapSecs = gap
	}
	if point.Entries < s.entriesMin {
		s.entriesMin = point.Entries
	}
	if point.Entries > s.entriesMax {
		s.entriesMax = point.Entries
	}
	if point.UniqueIPs < s.ipsMin {
		s.ipsMin = point.UniqueIPs
	}
	if point.UniqueIPs > s.ipsMax {
		s.ipsMax = point.UniqueIPs
	}
	s.lastEntries = point.Entries
	s.lastUniqueIPs = point.UniqueIPs
}

func (s historyLedgerStats) apply(entry *cache.Entry, frequency int) bool {
	if entry == nil || s.count == 0 {
		return false
	}
	snapshot := cache.HistoryLedgerStatsSnapshot{
		Version:             s.count,
		StartedUnix:         s.firstTS,
		Entries:             s.lastEntries,
		UniqueIPs:           s.lastUniqueIPs,
		EntriesMin:          s.entriesMin,
		EntriesMax:          s.entriesMax,
		IPsMin:              s.ipsMin,
		IPsMax:              s.ipsMax,
		HistoryTotalGapSecs: s.totalGapSecs,
		HistoryMinGapSecs:   s.minGapSecs,
		HistoryMaxGapSecs:   s.maxGapSecs,
	}
	if s.count <= 1 {
		snapshot.HistoryTotalGapSecs = 0
		snapshot.HistoryMinGapSecs = 0
		snapshot.HistoryMaxGapSecs = 0
		if frequency > 0 {
			snapshot.AverageUpdateMinutes = frequency
			snapshot.MinUpdateMinutes = frequency
			snapshot.MaxUpdateMinutes = frequency
		}
		return entry.ApplyHistoryLedgerStats(snapshot)
	}
	snapshot.AverageUpdateMinutes = roundSecondsToMinutes(s.totalGapSecs / int64(s.count-1))
	snapshot.MinUpdateMinutes = roundSecondsToMinutes(s.minGapSecs)
	snapshot.MaxUpdateMinutes = roundSecondsToMinutes(s.maxGapSecs)
	return entry.ApplyHistoryLedgerStats(snapshot)
}

func historyLedgerStatsFromEntry(entry *cache.Entry) (historyLedgerStats, bool) {
	if entry == nil || entry.Version <= 0 || entry.StartedDate <= 0 || entry.SourceDate <= 0 {
		return historyLedgerStats{}, false
	}

	stats := historyLedgerStats{
		firstTS:       entry.StartedDate,
		lastTS:        entry.SourceDate,
		count:         entry.Version,
		totalGapSecs:  entry.HistoryTotalGapSecs,
		minGapSecs:    entry.HistoryMinGapSecs,
		maxGapSecs:    entry.HistoryMaxGapSecs,
		entriesMin:    entry.EntriesMin,
		entriesMax:    entry.EntriesMax,
		ipsMin:        entry.IPsMin,
		ipsMax:        entry.IPsMax,
		lastEntries:   entry.Entries,
		lastUniqueIPs: entry.UniqueIPs,
	}
	if stats.count == 1 {
		stats.totalGapSecs = 0
		stats.minGapSecs = 0
		stats.maxGapSecs = 0
		return stats, true
	}
	if stats.totalGapSecs <= 0 || stats.minGapSecs <= 0 || stats.maxGapSecs <= 0 {
		if entry.AverageUpdateMins <= 0 || entry.MinUpdateMins <= 0 || entry.MaxUpdateMins <= 0 {
			return historyLedgerStats{}, false
		}
		stats.totalGapSecs = int64(entry.AverageUpdateMins) * 60 * int64(stats.count-1)
		stats.minGapSecs = int64(entry.MinUpdateMins) * 60
		stats.maxGapSecs = int64(entry.MaxUpdateMins) * 60
	}
	return stats, true
}

type feedLedgerState struct {
	mu sync.Mutex

	historyLoaded bool
	historyStats  historyLedgerStats
	historyTail   []HistoryPoint
	historyLimit  int

	changesLoaded bool
	changesTail   []ChangesetPoint
	changesLimit  int

	retentionLoaded  bool
	retentionStarted int64
	retentionPast    map[int]uint64

	cohortsLoaded bool
	cohorts       map[int64]uint64
}

type runtimeLedgerCache struct {
	mu    sync.Mutex
	feeds map[string]*feedLedgerState
}

func newRuntimeLedgerCache() *runtimeLedgerCache {
	return &runtimeLedgerCache{feeds: make(map[string]*feedLedgerState)}
}

func (c *runtimeLedgerCache) feed(name string) *feedLedgerState {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if st := c.feeds[name]; st != nil {
		return st
	}
	st := &feedLedgerState{}
	c.feeds[name] = st
	return st
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

func readCSVLines(path string, fn func(string) error) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	reader := bufio.NewReader(file)
	first := true
	for {
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

func loadHistoryLedgerState(path, name string, limit int) (historyLedgerStats, []HistoryPoint, error) {
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
	err := readCSVLines(path, func(line string) error {
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

func loadChangesetTail(path string, limit int) ([]ChangesetPoint, error) {
	if limit < 1 {
		limit = 1
	}
	tail := make([]ChangesetPoint, 0, limit)
	count := 0
	lineNum := 0
	err := readCSVLines(path, func(line string) error {
		lineNum++
		if lineNum == 1 && (strings.EqualFold(line, strings.TrimSpace(changesetLedgerHeader)) || strings.EqualFold(line, strings.TrimSpace(oldChangesetLedgerHeader))) {
			return nil
		}
		parts := strings.Split(line, ",")
		if len(parts) != 3 {
			return nil
		}
		ts, err := parseInt64(parts[0])
		if err != nil {
			return nil
		}
		added, err := parseUint64(parts[1])
		if err != nil {
			return nil
		}
		removed, err := parseUint64(parts[2])
		if err != nil {
			return nil
		}
		if added == 0 && removed == 0 {
			return nil
		}
		count++
		tail = appendChangesTail(tail, ChangesetPoint{
			Timestamp: ts,
			Added:     added,
			Removed:   removed,
		}, limit)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if count <= limit {
		if len(tail) <= 1 {
			return nil, nil
		}
		return append([]ChangesetPoint(nil), tail[1:]...), nil
	}
	return append([]ChangesetPoint(nil), tail...), nil
}

func loadHistoryTailCSV(path, name string, limit int) ([]HistoryPoint, error) {
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
	err := readCSVLines(path, func(line string) error {
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
	limit := e.webChartsEntries()
	if limit < 1 {
		return nil
	}
	if e.runtime.WebDir != "" {
		path := filepath.Join(e.runtime.WebDir, name+"_history.csv")
		if tail, err := loadHistoryTailCSV(path, name, limit); err == nil && len(tail) > 0 {
			return tail
		}
	}
	if e.runtime.LibDir != "" {
		path := filepath.Join(e.runtime.LibDir, name, "history.csv")
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

func loadRetentionPast(path string, started int64) (map[int]uint64, error) {
	past := make(map[int]uint64)
	lineNum := 0
	err := readCSVLines(path, func(line string) error {
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

func loadRetentionCohortIndex(path string) (map[int64]uint64, error) {
	out := map[int64]uint64{}
	lineNum := 0
	err := readCSVLines(path, func(line string) error {
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
	return writeFileAtomic(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}

func loadRetentionCohorts(ctx context.Context, dir string) (map[int64]uint64, error) {
	ctx = nonNilContext(ctx)
	indexPath := filepath.Join(dir, "retention_cohorts.csv")
	if cohorts, err := loadRetentionCohortIndex(indexPath); err == nil && len(cohorts) > 0 {
		return cohorts, nil
	}

	newDir := filepath.Join(dir, "new")
	entries, err := os.ReadDir(newDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[int64]uint64{}, nil
		}
		return nil, err
	}
	out := make(map[int64]uint64, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || isIgnoredRetentionSnapshotName(entry.Name()) {
			continue
		}
		tsStr := strings.TrimSuffix(entry.Name(), ".set")
		addedAt, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			continue
		}
		filePath := filepath.Join(newDir, entry.Name())
		fs, err := iprange.OpenFileSet(filePath)
		if err == nil {
			out[addedAt] = fs.UniqueIPs()
			_ = fs.Close()
			continue
		}
		set, err := loadSnapshotSet(ctx, entry.Name(), filePath)
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

func (e *Engine) historyStatsFromRuntime(name string, entry *cache.Entry, frequency int) bool {
	if e == nil || entry == nil || e.runtime.LibDir == "" {
		return false
	}
	st := e.ledgerCache.feed(name)
	if st == nil {
		return false
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if !st.historyLoaded || st.historyLimit < e.webChartsEntries() {
		stats, tail, err := loadHistoryLedgerState(filepath.Join(e.runtime.LibDir, name, "history.csv"), name, e.webChartsEntries())
		if err != nil {
			return false
		}
		st.historyLoaded = true
		st.historyStats = stats
		st.historyTail = tail
		st.historyLimit = e.webChartsEntries()
	}
	return st.historyStats.apply(entry, frequency)
}

func (e *Engine) observeHistoryPoint(name string, point HistoryPoint, entry, baseline *cache.Entry, frequency int) bool {
	if e == nil || entry == nil || e.runtime.LibDir == "" {
		return false
	}
	st := e.ledgerCache.feed(name)
	if st == nil {
		return false
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if !st.historyLoaded || st.historyLimit < e.webChartsEntries() {
		if stats, ok := historyLedgerStatsFromEntry(baseline); ok {
			st.historyLoaded = true
			st.historyStats = stats
			st.historyTail = e.historyTailBootstrap(name)
			st.historyLimit = e.webChartsEntries()
		} else {
			stats, tail, err := loadHistoryLedgerState(filepath.Join(e.runtime.LibDir, name, "history.csv"), name, e.webChartsEntries())
			if err != nil {
				return false
			}
			st.historyLoaded = true
			st.historyStats = stats
			st.historyTail = tail
			st.historyLimit = e.webChartsEntries()
		}
	}
	if point.Timestamp <= st.historyStats.lastTS {
		if point.Timestamp == st.historyStats.lastTS {
			stats, tail, err := loadHistoryLedgerState(filepath.Join(e.runtime.LibDir, name, "history.csv"), name, e.webChartsEntries())
			if err == nil {
				st.historyLoaded = true
				st.historyStats = stats
				st.historyTail = tail
				st.historyLimit = e.webChartsEntries()
				return st.historyStats.apply(entry, frequency)
			}
		}
		return st.historyStats.apply(entry, frequency)
	}
	st.historyStats.observe(point)
	st.historyTail = appendObservedHistoryTail(st.historyTail, point, e.webChartsEntries())
	st.historyLimit = e.webChartsEntries()
	return st.historyStats.apply(entry, frequency)
}

func (e *Engine) historyTailFromRuntime(name string) []HistoryPoint {
	if e == nil || e.runtime.LibDir == "" {
		return nil
	}
	st := e.ledgerCache.feed(name)
	if st == nil {
		return nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if !st.historyLoaded || st.historyLimit < e.webChartsEntries() {
		stats, tail, err := loadHistoryLedgerState(filepath.Join(e.runtime.LibDir, name, "history.csv"), name, e.webChartsEntries())
		if err != nil {
			return nil
		}
		st.historyLoaded = true
		st.historyStats = stats
		st.historyTail = tail
		st.historyLimit = e.webChartsEntries()
	}
	return append([]HistoryPoint(nil), st.historyTail...)
}

func (e *Engine) changesetTailFromRuntime(name string) []ChangesetPoint {
	if e == nil || e.runtime.LibDir == "" {
		return nil
	}
	st := e.ledgerCache.feed(name)
	if st == nil {
		return nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if !st.changesLoaded || st.changesLimit < e.webChartsEntries() {
		points, err := loadChangesetTail(filepath.Join(e.runtime.LibDir, name, "changesets.csv"), e.webChartsEntries())
		if err != nil {
			return nil
		}
		st.changesLoaded = true
		st.changesTail = points
		st.changesLimit = e.webChartsEntries()
	}
	return append([]ChangesetPoint(nil), st.changesTail...)
}

func (e *Engine) observeChangesetPoint(name string, point ChangesetPoint) {
	if e == nil || e.runtime.LibDir == "" || (point.Added == 0 && point.Removed == 0) {
		return
	}
	st := e.ledgerCache.feed(name)
	if st == nil {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if !st.changesLoaded || st.changesLimit < e.webChartsEntries() {
		points, err := loadChangesetTail(filepath.Join(e.runtime.LibDir, name, "changesets.csv"), e.webChartsEntries())
		if err != nil {
			return
		}
		st.changesLoaded = true
		st.changesTail = points
		st.changesLimit = e.webChartsEntries()
		return
	}
	st.changesTail = appendChangesTail(st.changesTail, point, e.webChartsEntries())
	st.changesLimit = e.webChartsEntries()
}

func (e *Engine) retentionPastFromRuntime(name string, started int64) map[int]uint64 {
	if e == nil || e.runtime.LibDir == "" {
		return map[int]uint64{}
	}
	st := e.ledgerCache.feed(name)
	if st == nil {
		return map[int]uint64{}
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if !st.retentionLoaded || st.retentionStarted != started {
		past, err := loadRetentionPast(filepath.Join(e.runtime.LibDir, name, "retention.csv"), started)
		if err != nil {
			return map[int]uint64{}
		}
		st.retentionLoaded = true
		st.retentionStarted = started
		st.retentionPast = past
	}
	return copyRetentionPast(st.retentionPast)
}

func (e *Engine) observeRetentionPast(name string, started int64, hours int, ips uint64) {
	if e == nil || e.runtime.LibDir == "" || ips == 0 {
		return
	}
	st := e.ledgerCache.feed(name)
	if st == nil {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if !st.retentionLoaded || st.retentionStarted != started {
		past, err := loadRetentionPast(filepath.Join(e.runtime.LibDir, name, "retention.csv"), started)
		if err != nil {
			return
		}
		st.retentionLoaded = true
		st.retentionStarted = started
		st.retentionPast = past
		return
	}
	if st.retentionPast == nil {
		st.retentionPast = make(map[int]uint64)
	}
	st.retentionPast[hours] += ips
}

func (e *Engine) retentionCohortsFromRuntime(ctx context.Context, name string) map[int64]uint64 {
	if e == nil || e.runtime.LibDir == "" {
		return map[int64]uint64{}
	}
	st := e.ledgerCache.feed(name)
	if st == nil {
		return map[int64]uint64{}
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if !st.cohortsLoaded {
		cohorts, err := loadRetentionCohorts(ctx, filepath.Join(e.runtime.LibDir, name))
		if err != nil {
			return map[int64]uint64{}
		}
		st.cohortsLoaded = true
		st.cohorts = cohorts
	}
	out := make(map[int64]uint64, len(st.cohorts))
	for addedAt, ips := range st.cohorts {
		out[addedAt] = ips
	}
	return out
}

func (e *Engine) observeRetentionCohort(name string, addedAt int64, ips uint64) {
	if e == nil || e.runtime.LibDir == "" || ips == 0 {
		return
	}
	st := e.ledgerCache.feed(name)
	if st == nil {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if !st.cohortsLoaded {
		return
	}
	if st.cohorts == nil {
		st.cohorts = make(map[int64]uint64)
	}
	st.cohorts[addedAt] += ips
}

func (e *Engine) replaceRetentionCohorts(name string, cohorts map[int64]uint64) {
	if e == nil || e.runtime.LibDir == "" {
		return
	}
	st := e.ledgerCache.feed(name)
	if st == nil {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	st.cohortsLoaded = true
	st.cohorts = make(map[int64]uint64, len(cohorts))
	for addedAt, ips := range cohorts {
		if ips == 0 {
			continue
		}
		st.cohorts[addedAt] = ips
	}
}
