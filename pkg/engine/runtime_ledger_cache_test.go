package engine

import (
	"path/filepath"
	"testing"

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
