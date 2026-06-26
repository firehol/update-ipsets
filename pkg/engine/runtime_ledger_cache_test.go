package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
)

func TestHistoryLedgerCacheAppliesAndObserves(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.LibDir = root
		rt.WebChartsEntries = 2
	}))
	const (
		name = "sample"
		base = int64(1700000000)
	)
	historyPath := filepath.Join(root, name, "history.csv")
	if err := appendCSV(historyPath, "DateTime,Entries,UniqueIPs\n", "1700000000,10,100\n"); err != nil {
		t.Fatalf("append history row 1: %v", err)
	}
	if err := appendCSV(historyPath, "DateTime,Entries,UniqueIPs\n", "1700003600,20,200\n"); err != nil {
		t.Fatalf("append history row 2: %v", err)
	}
	if err := appendCSV(historyPath, "DateTime,Entries,UniqueIPs\n", "1700007200,15,150\n"); err != nil {
		t.Fatalf("append history row 3: %v", err)
	}
	if info, err := os.Stat(historyPath); err != nil {
		t.Fatal(err)
	} else if got := info.Mode().Perm(); got != generatedFileMode {
		t.Fatalf("history ledger mode = %04o, want %04o", got, generatedFileMode)
	}

	entry := &cache.Entry{}
	if !eng.refreshHistoryStatsFromLedger(name, entry, 60) {
		t.Fatal("expected cached history stats refresh to succeed")
	}
	if got, want := entry.Version, 3; got != want {
		t.Fatalf("version = %d, want %d", got, want)
	}
	if got, want := entry.StartedDate, base; got != want {
		t.Fatalf("started = %d, want %d", got, want)
	}
	if got, want := entry.AverageUpdateMins, 60; got != want {
		t.Fatalf("average update mins = %d, want %d", got, want)
	}
	if got, want := len(eng.historyTailFromRuntime(name)), 2; got != want {
		t.Fatalf("history tail len = %d, want %d", got, want)
	}

	next := HistoryPoint{
		Timestamp: base + 3*3600,
		Name:      name,
		Entries:   25,
		UniqueIPs: 250,
	}
	if err := appendCSV(historyPath, "DateTime,Entries,UniqueIPs\n", "1700010800,25,250\n"); err != nil {
		t.Fatalf("append history row 4: %v", err)
	}
	entry = &cache.Entry{}
	if !eng.observeHistoryPoint(name, next, entry, nil, 60) {
		t.Fatal("expected observed history point to refresh cached stats")
	}
	if got, want := entry.Version, 4; got != want {
		t.Fatalf("version after observe = %d, want %d", got, want)
	}
	if got, want := entry.Entries, 25; got != want {
		t.Fatalf("entries after observe = %d, want %d", got, want)
	}
	if got, want := entry.UniqueIPs, uint64(250); got != want {
		t.Fatalf("unique IPs after observe = %d, want %d", got, want)
	}
	tail := eng.historyTailFromRuntime(name)
	if got, want := len(tail), 2; got != want {
		t.Fatalf("tail len after observe = %d, want %d", got, want)
	}
	if got, want := tail[0].Timestamp, base+2*3600; got != want {
		t.Fatalf("tail[0].timestamp = %d, want %d", got, want)
	}
	if got, want := tail[1].Timestamp, next.Timestamp; got != want {
		t.Fatalf("tail[1].timestamp = %d, want %d", got, want)
	}
}

func TestObserveHistoryPointBootstrapsFromCacheEntryAndPublicTail(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	libDir := filepath.Join(root, "lib")
	webDir := filepath.Join(root, "web")
	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.LibDir = libDir
		rt.WebDir = webDir
		rt.WebChartsEntries = 2
	}))
	const (
		name = "sample"
		base = int64(1700000000)
	)
	if err := appendCSV(filepath.Join(webDir, name+"_history.csv"), "DateTime,Entries,UniqueIPs\n", "1700003600,20,200\n"); err != nil {
		t.Fatalf("append public history row 1: %v", err)
	}
	if err := appendCSV(filepath.Join(webDir, name+"_history.csv"), "DateTime,Entries,UniqueIPs\n", "1700007200,15,150\n"); err != nil {
		t.Fatalf("append public history row 2: %v", err)
	}

	baseline := &cache.Entry{
		Name:                name,
		Version:             3,
		StartedDate:         base,
		SourceDate:          base + 2*3600,
		Entries:             15,
		UniqueIPs:           150,
		EntriesMin:          10,
		EntriesMax:          20,
		IPsMin:              100,
		IPsMax:              200,
		AverageUpdateMins:   60,
		MinUpdateMins:       60,
		MaxUpdateMins:       60,
		HistoryTotalGapSecs: 7200,
		HistoryMinGapSecs:   3600,
		HistoryMaxGapSecs:   3600,
	}
	entry := &cache.Entry{}
	next := HistoryPoint{
		Timestamp: base + 3*3600,
		Name:      name,
		Entries:   25,
		UniqueIPs: 250,
	}
	if !eng.observeHistoryPoint(name, next, entry, baseline, 60) {
		t.Fatal("expected bootstrap from cached entry to succeed without full ledger")
	}
	if got, want := entry.Version, 4; got != want {
		t.Fatalf("version after bootstrap observe = %d, want %d", got, want)
	}
	if got, want := entry.AverageUpdateMins, 60; got != want {
		t.Fatalf("average update mins after bootstrap observe = %d, want %d", got, want)
	}
	if got, want := entry.HistoryTotalGapSecs, int64(10800); got != want {
		t.Fatalf("history total gap secs after bootstrap observe = %d, want %d", got, want)
	}
	tail := eng.historyTailFromRuntime(name)
	if got, want := len(tail), 2; got != want {
		t.Fatalf("tail len after bootstrap observe = %d, want %d", got, want)
	}
	if got, want := tail[0].Timestamp, base+2*3600; got != want {
		t.Fatalf("tail[0].timestamp after bootstrap observe = %d, want %d", got, want)
	}
	if got, want := tail[1].Timestamp, next.Timestamp; got != want {
		t.Fatalf("tail[1].timestamp after bootstrap observe = %d, want %d", got, want)
	}
}

func TestObserveHistoryPointContextHonorsCanceledContext(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.LibDir = root
		rt.WebChartsEntries = 2
	}))
	const name = "sample"
	if err := appendCSV(filepath.Join(root, name, "history.csv"), "DateTime,Entries,UniqueIPs\n", "1700000000,10,100\n"); err != nil {
		t.Fatalf("append history row: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	entry := &cache.Entry{Name: name}
	if eng.observeHistoryPointContext(ctx, name, HistoryPoint{
		Timestamp: 1700003600,
		Name:      name,
		Entries:   20,
		UniqueIPs: 200,
	}, entry, nil, 60) {
		t.Fatal("observeHistoryPointContext() succeeded with canceled context")
	}
	if entry.Version != 0 || entry.Entries != 0 || entry.UniqueIPs != 0 {
		t.Fatalf("entry was updated despite canceled context: %+v", entry)
	}
}

func TestObserveHistoryPointReloadsSameTimestampCorrectionFromLedger(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	libDir := filepath.Join(root, "lib")
	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.LibDir = libDir
		rt.WebChartsEntries = 3
	}))
	const (
		name = "sample"
		base = int64(1700000000)
	)
	historyPath := filepath.Join(libDir, name, "history.csv")
	if err := appendCSV(historyPath, "DateTime,Entries,UniqueIPs\n", "1700000000,10,100\n"); err != nil {
		t.Fatalf("append history row 1: %v", err)
	}
	if err := appendCSV(historyPath, "DateTime,Entries,UniqueIPs\n", "1700003600,0,0\n"); err != nil {
		t.Fatalf("append bad history row: %v", err)
	}
	if err := appendCSV(historyPath, "DateTime,Entries,UniqueIPs\n", "1700003600,20,200\n"); err != nil {
		t.Fatalf("append corrected history row: %v", err)
	}

	baseline := &cache.Entry{
		Name:                name,
		Version:             2,
		StartedDate:         base,
		SourceDate:          base + 3600,
		Entries:             0,
		UniqueIPs:           0,
		EntriesMin:          0,
		EntriesMax:          10,
		IPsMin:              0,
		IPsMax:              100,
		AverageUpdateMins:   60,
		MinUpdateMins:       60,
		MaxUpdateMins:       60,
		HistoryTotalGapSecs: 3600,
		HistoryMinGapSecs:   3600,
		HistoryMaxGapSecs:   3600,
	}
	entry := &cache.Entry{}
	point := HistoryPoint{
		Timestamp: base + 3600,
		Name:      name,
		Entries:   20,
		UniqueIPs: 200,
	}
	if !eng.observeHistoryPoint(name, point, entry, baseline, 60) {
		t.Fatal("expected same-timestamp correction to reload from ledger")
	}
	if got, want := entry.Version, 2; got != want {
		t.Fatalf("version after correction = %d, want %d", got, want)
	}
	if got, want := entry.Entries, 20; got != want {
		t.Fatalf("entries after correction = %d, want %d", got, want)
	}
	if got, want := entry.UniqueIPs, uint64(200); got != want {
		t.Fatalf("unique IPs after correction = %d, want %d", got, want)
	}
	if got, want := entry.EntriesMin, 10; got != want {
		t.Fatalf("entries_min after correction = %d, want %d", got, want)
	}
	if got, want := entry.IPsMin, uint64(100); got != want {
		t.Fatalf("ips_min after correction = %d, want %d", got, want)
	}
	tail := eng.historyTailFromRuntime(name)
	if got, want := len(tail), 2; got != want {
		t.Fatalf("tail len after correction = %d, want %d", got, want)
	}
	if got, want := tail[1].Entries, 20; got != want {
		t.Fatalf("tail corrected entries = %d, want %d", got, want)
	}
	if got, want := tail[1].UniqueIPs, uint64(200); got != want {
		t.Fatalf("tail corrected unique IPs = %d, want %d", got, want)
	}
}

func TestObserveHistoryPointSameTimestampSameCountsUsesCachedStats(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	libDir := filepath.Join(root, "lib")
	webDir := filepath.Join(root, "web")
	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.LibDir = libDir
		rt.WebDir = webDir
		rt.WebChartsEntries = 2
	}))
	const (
		name = "sample"
		base = int64(1700000000)
	)
	if err := appendCSV(filepath.Join(webDir, name+"_history.csv"), "DateTime,Entries,UniqueIPs\n", "1700000000,10,100\n"); err != nil {
		t.Fatalf("append public history row 1: %v", err)
	}
	if err := appendCSV(filepath.Join(webDir, name+"_history.csv"), "DateTime,Entries,UniqueIPs\n", "1700003600,20,200\n"); err != nil {
		t.Fatalf("append public history row 2: %v", err)
	}

	baseline := &cache.Entry{
		Name:                name,
		Version:             2,
		StartedDate:         base,
		SourceDate:          base + 3600,
		Entries:             20,
		UniqueIPs:           200,
		EntriesMin:          10,
		EntriesMax:          20,
		IPsMin:              100,
		IPsMax:              200,
		AverageUpdateMins:   60,
		MinUpdateMins:       60,
		MaxUpdateMins:       60,
		HistoryTotalGapSecs: 3600,
		HistoryMinGapSecs:   3600,
		HistoryMaxGapSecs:   3600,
	}
	entry := &cache.Entry{}
	point := HistoryPoint{
		Timestamp: base + 3600,
		Name:      name,
		Entries:   20,
		UniqueIPs: 200,
	}
	if !eng.observeHistoryPoint(name, point, entry, baseline, 60) {
		t.Fatal("expected same-timestamp no-op to use cached stats without internal ledger")
	}
	if got, want := entry.Version, 2; got != want {
		t.Fatalf("version after no-op = %d, want %d", got, want)
	}
	if got, want := entry.Entries, 20; got != want {
		t.Fatalf("entries after no-op = %d, want %d", got, want)
	}
	tail := eng.historyTailFromRuntime(name)
	if got, want := len(tail), 2; got != want {
		t.Fatalf("tail len after no-op = %d, want %d", got, want)
	}
	if got, want := tail[1].Timestamp, point.Timestamp; got != want {
		t.Fatalf("tail timestamp after no-op = %d, want %d", got, want)
	}
}

func TestObserveHistoryPointBootstrapsFromRoundedEntryStats(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	libDir := filepath.Join(root, "lib")
	webDir := filepath.Join(root, "web")
	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.LibDir = libDir
		rt.WebDir = webDir
		rt.WebChartsEntries = 2
	}))
	const (
		name = "rounded"
		base = int64(1700000000)
	)
	if err := appendCSV(filepath.Join(webDir, name+"_history.csv"), "DateTime,Entries,UniqueIPs\n", "1700003600,20,200\n"); err != nil {
		t.Fatalf("append public history row 1: %v", err)
	}
	if err := appendCSV(filepath.Join(webDir, name+"_history.csv"), "DateTime,Entries,UniqueIPs\n", "1700007200,15,150\n"); err != nil {
		t.Fatalf("append public history row 2: %v", err)
	}

	baseline := &cache.Entry{
		Name:              name,
		Version:           3,
		StartedDate:       base,
		SourceDate:        base + 2*3600,
		Entries:           15,
		UniqueIPs:         150,
		EntriesMin:        10,
		EntriesMax:        20,
		IPsMin:            100,
		IPsMax:            200,
		AverageUpdateMins: 60,
		MinUpdateMins:     60,
		MaxUpdateMins:     60,
	}
	entry := &cache.Entry{}
	next := HistoryPoint{
		Timestamp: base + 3*3600,
		Name:      name,
		Entries:   25,
		UniqueIPs: 250,
	}
	if !eng.observeHistoryPoint(name, next, entry, baseline, 60) {
		t.Fatal("expected rounded cache stats bootstrap to succeed without full ledger")
	}
	if got, want := entry.Version, 4; got != want {
		t.Fatalf("version after rounded bootstrap observe = %d, want %d", got, want)
	}
	if got, want := entry.AverageUpdateMins, 60; got != want {
		t.Fatalf("average update mins after rounded bootstrap observe = %d, want %d", got, want)
	}
	if got, want := entry.HistoryTotalGapSecs, int64(10800); got != want {
		t.Fatalf("history total gap secs after rounded bootstrap observe = %d, want %d", got, want)
	}
}

func TestChangesetTailFromRuntimeDropsBootstrapAndKeepsWindow(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.LibDir = root
		rt.WebChartsEntries = 2
	}))
	const name = "sample"
	path := filepath.Join(root, name, "changesets.csv")
	rows := []string{
		"1700000000,1,1\n",
		"1700003600,2,0\n",
		"1700007200,3,0\n",
		"1700010800,4,0\n",
	}
	for _, row := range rows {
		if err := appendCSV(path, changesetLedgerHeader, row); err != nil {
			t.Fatalf("append changeset row %q: %v", row, err)
		}
	}

	tail := eng.changesetTailFromRuntime(name)
	if got, want := len(tail), 2; got != want {
		t.Fatalf("changeset tail len = %d, want %d", got, want)
	}
	if got, want := tail[0].Timestamp, int64(1700007200); got != want {
		t.Fatalf("tail[0].timestamp = %d, want %d", got, want)
	}
	if got, want := tail[1].Timestamp, int64(1700010800); got != want {
		t.Fatalf("tail[1].timestamp = %d, want %d", got, want)
	}

	if err := appendCSV(path, changesetLedgerHeader, "1700014400,5,0\n"); err != nil {
		t.Fatalf("append changeset row 5: %v", err)
	}
	eng.observeChangesetPoint(name, ChangesetPoint{
		Timestamp: 1700014400,
		Added:     5,
		Removed:   0,
	})
	tail = eng.changesetTailFromRuntime(name)
	if got, want := len(tail), 2; got != want {
		t.Fatalf("changeset tail len after observe = %d, want %d", got, want)
	}
	if got, want := tail[0].Timestamp, int64(1700010800); got != want {
		t.Fatalf("tail[0].timestamp after observe = %d, want %d", got, want)
	}
	if got, want := tail[1].Timestamp, int64(1700014400); got != want {
		t.Fatalf("tail[1].timestamp after observe = %d, want %d", got, want)
	}
}

func TestChangesetTailFromRuntimeReadsLargeLedgerTail(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.LibDir = root
		rt.WebChartsEntries = 3
	}))
	const (
		name = "large"
		base = int64(1700000000)
	)
	path := filepath.Join(root, name, "changesets.csv")
	if err := os.MkdirAll(filepath.Dir(path), generatedDirMode); err != nil {
		t.Fatalf("mkdir changeset dir: %v", err)
	}
	var b strings.Builder
	b.WriteString(changesetLedgerHeader)
	fmt.Fprintf(&b, "%d,1,1\n", base)
	for i := int64(1); i <= 8000; i++ {
		fmt.Fprintf(&b, "%d,%d,0\n", base+i*3600, i+1)
	}
	b.WriteString("not,a,valid,row\n")
	fmt.Fprintf(&b, "%d,0,0\n", base+9000*3600)
	if err := writeFileAtomic(path, []byte(b.String()), generatedFileMode); err != nil {
		t.Fatalf("write changesets ledger: %v", err)
	}

	tail := eng.changesetTailFromRuntime(name)
	if got, want := len(tail), 3; got != want {
		t.Fatalf("changeset tail len = %d, want %d", got, want)
	}
	for i, point := range tail {
		wantTS := base + int64(7998+i)*3600
		if point.Timestamp != wantTS {
			t.Fatalf("tail[%d].timestamp = %d, want %d", i, point.Timestamp, wantTS)
		}
	}
}

func BenchmarkLoadChangesetTailLargeLedger(b *testing.B) {
	root := b.TempDir()
	const (
		name = "large"
		base = int64(1700000000)
	)
	path := filepath.Join(root, name, "changesets.csv")
	if err := os.MkdirAll(filepath.Dir(path), generatedDirMode); err != nil {
		b.Fatalf("mkdir changeset dir: %v", err)
	}
	var body strings.Builder
	body.WriteString(changesetLedgerHeader)
	fmt.Fprintf(&body, "%d,1,1\n", base)
	for i := int64(1); i <= 100_000; i++ {
		fmt.Fprintf(&body, "%d,%d,0\n", base+i*3600, i+1)
	}
	if err := writeFileAtomic(path, []byte(body.String()), generatedFileMode); err != nil {
		b.Fatalf("write changesets ledger: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		tail, err := loadChangesetTail(path, 200)
		if err != nil {
			b.Fatal(err)
		}
		if len(tail) != 200 {
			b.Fatalf("tail len = %d, want 200", len(tail))
		}
	}
}

func TestRuntimeLedgerHistoryLoadDoesNotHoldFeedLock(t *testing.T) {
	root := t.TempDir()
	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.LibDir = root
		rt.WebChartsEntries = 2
	}))
	const name = "sample"
	if err := appendCSV(filepath.Join(root, name, "history.csv"), "DateTime,Entries,UniqueIPs\n", "1700000000,10,100\n"); err != nil {
		t.Fatalf("append history row: %v", err)
	}

	loadStarted := make(chan struct{})
	releaseLoad := make(chan struct{})
	var loadStartedOnce sync.Once
	restore := setRuntimeLedgerLoadHookForTest(func(kind, feed string) {
		if kind != "history" || feed != name {
			return
		}
		loadStartedOnce.Do(func() { close(loadStarted) })
		<-releaseLoad
	})
	defer restore()

	tailDone := make(chan struct{})
	go func() {
		_ = eng.historyTailFromRuntime(name)
		close(tailDone)
	}()

	select {
	case <-loadStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("history ledger load did not start")
	}

	observerDone := make(chan struct{})
	go func() {
		eng.observeRetentionCohort(name, 1700000000, 1)
		close(observerDone)
	}()

	select {
	case <-observerDone:
	case <-time.After(200 * time.Millisecond):
		close(releaseLoad)
		t.Fatal("history ledger load held the per-feed ledger lock")
	}

	close(releaseLoad)
	select {
	case <-tailDone:
	case <-time.After(2 * time.Second):
		t.Fatal("history ledger load did not finish")
	}
}

func TestRuntimeLedgerLoadersHonorCancelledContext(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	historyPath := filepath.Join(root, "sample", "history.csv")
	if err := appendCSV(historyPath, "DateTime,Entries,UniqueIPs\n", "1700000000,10,100\n"); err != nil {
		t.Fatalf("append history row: %v", err)
	}
	if _, _, err := loadHistoryLedgerStateContext(ctx, historyPath, "sample", 10); !errors.Is(err, context.Canceled) {
		t.Fatalf("loadHistoryLedgerStateContext() error = %v, want context.Canceled", err)
	}

	changesPath := filepath.Join(root, "sample", "changesets.csv")
	if err := appendCSV(changesPath, changesetLedgerHeader, "1700000000,1,0\n"); err != nil {
		t.Fatalf("append changeset row: %v", err)
	}
	if _, err := loadChangesetTailContext(ctx, changesPath, 10); !errors.Is(err, context.Canceled) {
		t.Fatalf("loadChangesetTailContext() error = %v, want context.Canceled", err)
	}

	retentionPath := filepath.Join(root, "sample", "retention.csv")
	if err := appendCSV(retentionPath, "date_removed,date_added,hours,ips\n", "1700003600,1700000000,1,10\n"); err != nil {
		t.Fatalf("append retention row: %v", err)
	}
	if _, err := loadRetentionPastContext(ctx, retentionPath, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("loadRetentionPastContext() error = %v, want context.Canceled", err)
	}

	cohortDir := filepath.Join(root, "sample")
	if err := appendCSV(filepath.Join(cohortDir, "retention_cohorts.csv"), "date_added,ips\n", "1700000000,10\n"); err != nil {
		t.Fatalf("append retention cohort row: %v", err)
	}
	if _, err := loadRetentionCohorts(ctx, cohortDir); !errors.Is(err, context.Canceled) {
		t.Fatalf("loadRetentionCohorts() error = %v, want context.Canceled", err)
	}
}

func TestBuildCurrentRetentionBuckets(t *testing.T) {
	t.Parallel()

	cohorts := map[int64]uint64{
		1700000000: 10,
		1700003600: 20,
		1700007200: 30,
	}
	current, incomplete := buildCurrentRetentionBuckets(cohorts, 1700010800, 1700003600)
	if got, want := incomplete, 1; got != want {
		t.Fatalf("incomplete = %d, want %d", got, want)
	}
	if got, want := current[3], uint64(10); got != want {
		t.Fatalf("bucket[3] = %d, want %d", got, want)
	}
	if got, want := current[2], uint64(20); got != want {
		t.Fatalf("bucket[2] = %d, want %d", got, want)
	}
	if got, want := current[1], uint64(30); got != want {
		t.Fatalf("bucket[1] = %d, want %d", got, want)
	}
}
