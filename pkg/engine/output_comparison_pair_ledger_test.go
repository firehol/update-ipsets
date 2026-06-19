package engine

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
)

func TestWriteComparisonFilesUsesPairLedgerForUnchangedUpdatedFeed(t *testing.T) {
	eng, webDir := newComparisonLedgerFixture(t, map[string]string{
		"alpha": "10.0.0.0/24\n",
		"beta":  "10.0.0.128/25\n",
		"gamma": "10.0.0.64/26\n",
	})

	if err := eng.writeComparisonFiles(t.Context(), nil, webDir, nil); err != nil {
		t.Fatal(err)
	}
	overlapBefore := lifetimeCounterCount(t, eng, "metadata.comparison_pair_overlap")

	if err := eng.writeComparisonFiles(t.Context(), []string{"alpha"}, webDir, nil); err != nil {
		t.Fatal(err)
	}

	if got, want := lifetimeCounterCount(t, eng, "metadata.comparison_pair_ledger_hit"), int64(3); got != want {
		t.Fatalf("ledger hits = %d, want %d", got, want)
	}
	if got := lifetimeCounterCount(t, eng, "metadata.comparison_pair_overlap"); got != overlapBefore {
		t.Fatalf("overlap counter changed from %d to %d on cached unchanged run", overlapBefore, got)
	}
}

func TestComparisonPairLedgerIncrementalReplacementPreservesUnchangedEntries(t *testing.T) {
	eng, webDir := newComparisonLedgerFixture(t, map[string]string{
		"alpha": "10.0.0.0/24\n",
		"beta":  "10.0.0.128/25\n",
		"gamma": "10.0.0.64/26\n",
	})

	if err := eng.writeComparisonFiles(t.Context(), nil, webDir, nil); err != nil {
		t.Fatal(err)
	}
	rewriteComparisonLedgerFeed(t, eng, "alpha", "10.0.1.0/24\n")
	if err := eng.writeComparisonFiles(t.Context(), []string{"alpha"}, webDir, nil); err != nil {
		t.Fatal(err)
	}

	entries := readComparisonPairLedgerEntries(t, eng)
	if got, want := len(entries), 3; got != want {
		t.Fatalf("ledger entry count = %d, want %d; entries=%+v", got, want, entries)
	}
	if !comparisonPairLedgerEntryExists(entries, "beta", "gamma") {
		t.Fatalf("unchanged beta/gamma entry was not retained: %+v", entries)
	}
}

func TestComparisonPairLedgerCorruptFallbackRebuildsSparseLedger(t *testing.T) {
	eng, webDir := newComparisonLedgerFixture(t, map[string]string{
		"alpha": "10.0.0.0/24\n",
		"beta":  "10.0.0.128/25\n",
		"gamma": "10.0.0.64/26\n",
	})
	if err := os.MkdirAll(eng.runtime.CacheDir, generatedDirMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(eng.comparisonPairLedgerPath(), []byte("{broken"), generatedFileMode); err != nil {
		t.Fatal(err)
	}

	if err := eng.writeComparisonFiles(t.Context(), []string{"alpha"}, webDir, nil); err != nil {
		t.Fatal(err)
	}

	if got, want := lifetimeCounterCount(t, eng, "metadata.comparison_pair_ledger_ignored"), int64(1); got != want {
		t.Fatalf("ignored corrupt ledger counter = %d, want %d", got, want)
	}
	entries := readComparisonPairLedgerEntries(t, eng)
	if got, want := len(entries), 2; got != want {
		t.Fatalf("sparse rebuilt ledger entries = %d, want %d; entries=%+v", got, want, entries)
	}
	if comparisonPairLedgerEntryExists(entries, "beta", "gamma") {
		t.Fatalf("miss+unchanged beta/gamma pair should not be recomputed after corrupt ledger: %+v", entries)
	}
}

func TestComparisonPairLedgerVersionMismatchFallbackRebuildsSparseLedger(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*comparisonPairLedgerFile)
	}{
		{
			name: "format version",
			mutate: func(disk *comparisonPairLedgerFile) {
				disk.Version = comparisonPairLedgerFormatVersion + 1
			},
		},
		{
			name: "algorithm version",
			mutate: func(disk *comparisonPairLedgerFile) {
				disk.AlgorithmVersion = comparisonPairLedgerAlgorithmVersion + 1
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			eng, webDir := newComparisonLedgerFixture(t, map[string]string{
				"alpha": "10.0.0.0/24\n",
				"beta":  "10.0.0.128/25\n",
				"gamma": "10.0.0.64/26\n",
			})
			if err := eng.writeComparisonFiles(t.Context(), nil, webDir, nil); err != nil {
				t.Fatal(err)
			}
			overlapBefore := lifetimeCounterCount(t, eng, "metadata.comparison_pair_overlap")
			disk := readComparisonPairLedgerFile(t, eng)
			tc.mutate(&disk)
			writeComparisonPairLedgerFile(t, eng, disk)

			if err := eng.writeComparisonFiles(t.Context(), []string{"alpha"}, webDir, nil); err != nil {
				t.Fatal(err)
			}

			if got, want := lifetimeCounterCount(t, eng, "metadata.comparison_pair_ledger_ignored"), int64(1); got != want {
				t.Fatalf("ignored mismatched ledger counter = %d, want %d", got, want)
			}
			if got := lifetimeCounterCount(t, eng, "metadata.comparison_pair_overlap"); got <= overlapBefore {
				t.Fatalf("overlap counter did not increase after mismatched ledger fallback: before=%d after=%d", overlapBefore, got)
			}
			entries := readComparisonPairLedgerEntries(t, eng)
			if got, want := len(entries), 2; got != want {
				t.Fatalf("sparse rebuilt ledger entries = %d, want %d; entries=%+v", got, want, entries)
			}
			if comparisonPairLedgerEntryExists(entries, "beta", "gamma") {
				t.Fatalf("miss+unchanged beta/gamma pair should not be recomputed after mismatched ledger: %+v", entries)
			}
		})
	}
}

func TestComparisonPairLedgerPrunesRemovedFeeds(t *testing.T) {
	eng, webDir := newComparisonLedgerFixture(t, map[string]string{
		"alpha": "10.0.0.0/24\n",
		"beta":  "10.0.0.128/25\n",
		"gamma": "203.0.113.1\n",
	})
	if err := eng.writeComparisonFiles(t.Context(), nil, webDir, nil); err != nil {
		t.Fatal(err)
	}
	if entries := readComparisonPairLedgerEntries(t, eng); len(entries) != 3 {
		t.Fatalf("initial ledger entries = %d, want 3: %+v", len(entries), entries)
	}

	delete(eng.cfg.Sources, "gamma")
	if err := eng.writeComparisonFiles(t.Context(), []string{"alpha"}, webDir, nil); err != nil {
		t.Fatal(err)
	}

	entries := readComparisonPairLedgerEntries(t, eng)
	if got, want := len(entries), 1; got != want {
		t.Fatalf("pruned ledger entries = %d, want %d: %+v", got, want, entries)
	}
	if comparisonPairLedgerContainsFeed(entries, "gamma") {
		t.Fatalf("removed feed survived in ledger entries: %+v", entries)
	}
}

func TestComparisonPairLedgerCachedZeroDeletesStaleRowsOnBothPeers(t *testing.T) {
	eng, webDir := newComparisonLedgerFixture(t, map[string]string{
		"alpha": "10.0.0.1\n",
		"beta":  "192.0.2.1\n",
		"gamma": "203.0.113.1\n",
	})
	if err := eng.writeComparisonFiles(t.Context(), nil, webDir, nil); err != nil {
		t.Fatal(err)
	}
	writeComparisonRows(t, webDir, "alpha", []CompareRow{{Name: "beta", Category: "test", IPs: 1, Common: 1}})
	writeComparisonRows(t, webDir, "beta", []CompareRow{{Name: "alpha", Category: "test", IPs: 1, Common: 1}})

	if err := eng.writeComparisonFiles(t.Context(), []string{"gamma"}, webDir, nil); err != nil {
		t.Fatal(err)
	}

	if rows := readComparisonRows(t, webDir, "alpha"); comparisonRowExists(rows, "beta") {
		t.Fatalf("stale alpha->beta row survived cached common=0: %+v", rows)
	}
	if rows := readComparisonRows(t, webDir, "beta"); comparisonRowExists(rows, "alpha") {
		t.Fatalf("stale beta->alpha row survived cached common=0: %+v", rows)
	}
}

func TestComparisonPairLedgerRepairsMissingArtifactWithCachedRows(t *testing.T) {
	eng, webDir := newComparisonLedgerFixture(t, map[string]string{
		"alpha": "10.0.0.0/24\n",
		"beta":  "10.0.0.128/25\n",
		"gamma": "203.0.113.1\n",
	})
	if err := eng.writeComparisonFiles(t.Context(), nil, webDir, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(webDir, "alpha_comparison.json")); err != nil {
		t.Fatal(err)
	}

	if err := eng.writeComparisonFiles(t.Context(), []string{"gamma"}, webDir, nil); err != nil {
		t.Fatal(err)
	}

	if got, want := lifetimeCounterCount(t, eng, "metadata.comparison_pair_ledger_hit"), int64(3); got != want {
		t.Fatalf("ledger hits = %d, want %d", got, want)
	}
	row := findComparisonRow(t, readComparisonRows(t, webDir, "alpha"), "beta")
	if row.Common != 128 {
		t.Fatalf("repaired cached alpha->beta common = %d, want 128", row.Common)
	}
}

func TestComparisonPairLedgerPreservesUniqueShareEquivalence(t *testing.T) {
	eng, webDir := newComparisonLedgerFixture(t, map[string]string{
		"alpha": "10.0.0.0/24\n",
		"beta":  "10.0.0.128/25\n",
		"gamma": "203.0.113.1\n",
	})
	eng.state.Entry("alpha").UniqueIPs = 256
	eng.state.Entry("beta").UniqueIPs = 128
	eng.state.Entry("gamma").UniqueIPs = 1
	if err := eng.writeComparisonFiles(t.Context(), nil, webDir, nil); err != nil {
		t.Fatal(err)
	}
	eng.updateUniqueShares(nil, webDir)
	want := uniqueShareSnapshotForFeeds(t, eng, "alpha", "beta", "gamma")

	for _, name := range []string{"alpha", "beta", "gamma"} {
		eng.state.Entry(name).SetUniqueShare(0, 0)
	}
	if err := eng.writeComparisonFiles(t.Context(), []string{"gamma"}, webDir, nil); err != nil {
		t.Fatal(err)
	}
	eng.updateUniqueShares(nil, webDir)
	got := uniqueShareSnapshotForFeeds(t, eng, "alpha", "beta", "gamma")

	for name, wantEntry := range want {
		gotEntry := got[name]
		if math.Abs(gotEntry.pct-wantEntry.pct) > 0.000001 || gotEntry.samples != wantEntry.samples {
			t.Fatalf("%s unique share after cached reuse = pct %.6f samples %d, want pct %.6f samples %d", name, gotEntry.pct, gotEntry.samples, wantEntry.pct, wantEntry.samples)
		}
	}
}

func TestComparisonPairLedgerRecomputesCategoryAndRelatednessWithCachedCommon(t *testing.T) {
	eng, webDir := newComparisonLedgerFixture(t, map[string]string{
		"alpha": "10.0.0.0/24\n",
		"beta":  "10.0.0.128/25\n",
		"gamma": "203.0.113.1\n",
	})
	if err := eng.writeComparisonFiles(t.Context(), nil, webDir, nil); err != nil {
		t.Fatal(err)
	}

	eng.cfg.Sources["beta"].Category = "updated-category"
	eng.cfg.Sources["beta"].DerivedFrom = []string{"alpha"}
	entry := eng.state.Entry("beta")
	entry.Category = "updated-category"
	if err := eng.writeComparisonFiles(t.Context(), []string{"gamma"}, webDir, nil); err != nil {
		t.Fatal(err)
	}

	row := findComparisonRow(t, readComparisonRows(t, webDir, "alpha"), "beta")
	if row.Category != "updated-category" {
		t.Fatalf("cached row category = %q, want updated-category", row.Category)
	}
	if !row.Related {
		t.Fatalf("cached row related = false, want true: %+v", row)
	}
}

func TestComparisonPairLedgerFullReprocessBypassesPoisonedLedger(t *testing.T) {
	eng, webDir := newComparisonLedgerFixture(t, map[string]string{
		"alpha": "10.0.0.0/24\n",
		"beta":  "10.0.0.128/25\n",
	})
	if err := eng.writeComparisonFiles(t.Context(), nil, webDir, nil); err != nil {
		t.Fatal(err)
	}
	poisonComparisonPairLedgerCommon(t, eng, 999)

	if err := eng.writeComparisonFiles(t.Context(), nil, webDir, nil); err != nil {
		t.Fatal(err)
	}

	row := findComparisonRow(t, readComparisonRows(t, webDir, "alpha"), "beta")
	if row.Common == 999 {
		t.Fatalf("full reprocess reused poisoned ledger row: %+v", row)
	}
	if row.Common != 128 {
		t.Fatalf("full reprocess common = %d, want 128", row.Common)
	}
}

func BenchmarkRunComparisonPairsPairLedgerHits(b *testing.B) {
	eng := newEngineFixture(b)
	infos := make([]comparisonSetInfo, 400)
	for i := range infos {
		infos[i] = comparisonPairLedgerBenchmarkInfo(i)
	}
	results := make([]comparisonPairResult, 0, len(infos)*(len(infos)-1)/2)
	for i := 0; i < len(infos); i++ {
		for j := i + 1; j < len(infos); j++ {
			results = append(results, comparisonPairResult{i: i, j: j, common: 1})
		}
	}
	ledger := newComparisonPairLedgerSnapshot()
	for _, result := range results {
		ledger.entries[comparisonPairLedgerKeyForInfos(infos[result.i], infos[result.j])] = result.common
	}
	updated := newComparisonUpdateFilter([]string{infos[0].name})

	b.ReportAllocs()
	for b.Loop() {
		got, stats := eng.runComparisonPairs(b.Context(), infos, updated, nil, ledger)
		if len(got) != len(results) {
			b.Fatalf("result count = %d, want %d", len(got), len(results))
		}
		if stats.ledgerHit.Load() != int64(len(results)) {
			b.Fatalf("ledger hits = %d, want %d", stats.ledgerHit.Load(), len(results))
		}
	}
}

func newComparisonLedgerFixture(t *testing.T, bodies map[string]string) (*Engine, string) {
	t.Helper()
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	webDir := filepath.Join(root, "web")
	cacheDir := filepath.Join(root, "cache")
	for _, dir := range []string{baseDir, webDir, cacheDir} {
		if err := os.MkdirAll(dir, generatedDirMode); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.New()
	eng := newEngineFixture(t, withConfig(cfg), withRuntime(func(rt *Runtime) {
		rt.BaseDir = baseDir
		rt.CacheDir = cacheDir
		rt.WebDir = webDir
		rt.LibDir = filepath.Join(root, "lib")
		rt.MaxHeavyPhaseWorkers = 1
	}))
	logical := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	for name, body := range bodies {
		cfg.Sources[name] = &config.Source{Name: name, Frequency: 60, IPV: "ipv4", Output: "ip", Category: "test"}
		writeLedgerFixtureFeed(t, eng, name, body, logical)
	}
	return eng, webDir
}

func writeLedgerFixtureFeed(t *testing.T, eng *Engine, name, body string, logical time.Time) {
	t.Helper()
	file := name + ".ipset"
	if err := os.WriteFile(filepath.Join(eng.runtime.BaseDir, file), []byte(body), generatedFileMode); err != nil {
		t.Fatal(err)
	}
	entry := eng.state.Entry(name)
	entry.Name = name
	entry.File = file
	entry.Category = eng.cfg.Sources[name].Category
	entry.ProcessedDate = logical.Unix()
	entry.CheckedDate = entry.ProcessedDate
	entry.SourceDate = entry.ProcessedDate
}

func rewriteComparisonLedgerFeed(t *testing.T, eng *Engine, name, body string) {
	t.Helper()
	next := time.Unix(eng.state.Entry(name).ProcessedDate, 0).Add(time.Minute).UTC()
	writeLedgerFixtureFeed(t, eng, name, body, next)
}

func readComparisonPairLedgerEntries(t *testing.T, eng *Engine) []comparisonPairLedgerEntry {
	t.Helper()
	return readComparisonPairLedgerFile(t, eng).Entries
}

func readComparisonPairLedgerFile(t *testing.T, eng *Engine) comparisonPairLedgerFile {
	t.Helper()
	data, err := os.ReadFile(eng.comparisonPairLedgerPath())
	if err != nil {
		t.Fatal(err)
	}
	var disk comparisonPairLedgerFile
	if err := json.Unmarshal(data, &disk); err != nil {
		t.Fatal(err)
	}
	return disk
}

func writeComparisonPairLedgerFile(t *testing.T, eng *Engine, disk comparisonPairLedgerFile) {
	t.Helper()
	data, err := json.MarshalIndent(disk, "", "\t")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(eng.comparisonPairLedgerPath(), data, generatedFileMode); err != nil {
		t.Fatal(err)
	}
}

func comparisonPairLedgerEntryExists(entries []comparisonPairLedgerEntry, left, right string) bool {
	for _, entry := range entries {
		if entry.Left == left && entry.Right == right {
			return true
		}
		if entry.Left == right && entry.Right == left {
			return true
		}
	}
	return false
}

func comparisonPairLedgerContainsFeed(entries []comparisonPairLedgerEntry, name string) bool {
	for _, entry := range entries {
		if entry.Left == name || entry.Right == name {
			return true
		}
	}
	return false
}

func writeComparisonRows(t *testing.T, webDir, name string, rows []CompareRow) {
	t.Helper()
	data, err := jsonMarshalTabIndent(rows)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(webDir, name+"_comparison.json"), data, generatedFileMode); err != nil {
		t.Fatal(err)
	}
}

func readComparisonRows(t *testing.T, webDir, name string) []CompareRow {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(webDir, name+"_comparison.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []CompareRow
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	return rows
}

func comparisonRowExists(rows []CompareRow, name string) bool {
	for _, row := range rows {
		if row.Name == name {
			return true
		}
	}
	return false
}

func findComparisonRow(t *testing.T, rows []CompareRow, name string) CompareRow {
	t.Helper()
	for _, row := range rows {
		if row.Name == name {
			return row
		}
	}
	t.Fatalf("missing comparison row %q in %+v", name, rows)
	return CompareRow{}
}

func poisonComparisonPairLedgerCommon(t *testing.T, eng *Engine, common uint64) {
	t.Helper()
	disk := readComparisonPairLedgerFile(t, eng)
	for i := range disk.Entries {
		disk.Entries[i].Common = common
	}
	writeComparisonPairLedgerFile(t, eng, disk)
}

type uniqueShareSnapshotEntry struct {
	pct     float64
	samples int
}

func uniqueShareSnapshotForFeeds(t *testing.T, eng *Engine, names ...string) map[string]uniqueShareSnapshotEntry {
	t.Helper()
	out := make(map[string]uniqueShareSnapshotEntry, len(names))
	for _, name := range names {
		entry := eng.state.Entry(name)
		if entry == nil {
			t.Fatalf("missing state entry %q", name)
		}
		out[name] = uniqueShareSnapshotEntry{
			pct:     entry.UniqueSharePct,
			samples: entry.UniqueShareSamples,
		}
	}
	return out
}

func comparisonPairLedgerBenchmarkInfo(i int) comparisonSetInfo {
	name := fmt.Sprintf("feed_%03d", i)
	var hash comparisonContentHash
	hash.valid = true
	hash.sum[0] = byte(i >> 24)
	hash.sum[1] = byte(i >> 16)
	hash.sum[2] = byte(i >> 8)
	hash.sum[3] = byte(i)
	return comparisonSetInfo{name: name, ips: 1, category: "test", contentHash: hash}
}
