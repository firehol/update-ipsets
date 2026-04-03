package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/insights"
)

func TestReadInsightsSizeSeriesPrefersStagedPublicHistory(t *testing.T) {
	root := t.TempDir()
	stageDir := filepath.Join(root, "stage")
	webDir := filepath.Join(root, "web")
	libDir := filepath.Join(root, "lib")
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(webDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(libDir, "sample"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(stageDir, "sample_history.csv"), []byte("DateTime,Entries,UniqueIPs\n1700000200,20,2000\n1700000201,21,2100\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "sample_history.csv"), []byte("DateTime,Entries,UniqueIPs\n1700000100,10,1000\n1700000101,11,1100\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(libDir, "sample", "history.csv"), []byte("DateTime,Entries,UniqueIPs\n1700000300,30,3000\n1700000301,31,3100\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.WebDir = webDir
		rt.LibDir = libDir
		rt.WebChartsEntries = 500
	}))
	got := eng.readInsightsSizeSeries("sample", stageDir)
	want := []insights.SizePoint{
		{TS: 1700000200, Size: 2000},
		{TS: 1700000201, Size: 2100},
	}
	if len(got) != len(want) {
		t.Fatalf("size series length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("size series[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestReadInsightsSizeSeriesFallsBackToInternalLedgerWithoutSnapshots(t *testing.T) {
	root := t.TempDir()
	historyDir := filepath.Join(root, "history")
	libDir := filepath.Join(root, "lib")
	if err := os.MkdirAll(filepath.Join(libDir, "sample"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(libDir, "sample", "history.csv"), []byte("DateTime,Entries,UniqueIPs\n1700000100,10,1000\n1700000101,11,1100\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// If the insights hot path still used HistorySeries(), this extra snapshot
	// would be merged into the size series. The optimized path must ignore it.
	writeSnapshotForTest(t, historyDir, "sample", time.Unix(1700000150, 0).UTC(), "2.2.2.0/24")

	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.HistoryDir = historyDir
		rt.LibDir = libDir
		rt.WebChartsEntries = 500
	}))
	got := eng.readInsightsSizeSeries("sample", "")
	if len(got) != 2 {
		t.Fatalf("expected insights size series to come from the internal ledger only, got %#v", got)
	}
	if got[0].TS != 1700000100 || got[1].TS != 1700000101 {
		t.Fatalf("unexpected size series timestamps: %#v", got)
	}
}

func TestReadInsightsChurnSeriesPrefersStagedPublicChangesets(t *testing.T) {
	root := t.TempDir()
	stageDir := filepath.Join(root, "stage")
	webDir := filepath.Join(root, "web")
	libDir := filepath.Join(root, "lib")
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(webDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(libDir, "sample"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(stageDir, "sample_changesets.csv"), []byte("DateTime,AddedIPs,RemovedIPs\n1700000201,4,1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "sample_changesets.csv"), []byte("DateTime,AddedIPs,RemovedIPs\n1700000101,8,2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(libDir, "sample", "changesets.csv"), []byte("DateTime,IPsAdded,IPsRemoved\n1700000300,1,0\n1700000301,2,1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.WebDir = webDir
		rt.LibDir = libDir
		rt.WebChartsEntries = 500
	}))
	sizes := []insights.SizePoint{
		{TS: 1700000201, Size: 10},
	}
	got := eng.readInsightsChurnSeries("sample", stageDir, sizes)
	if len(got) != 1 {
		t.Fatalf("expected staged public changesets to drive churn, got %#v", got)
	}
	if got[0].TS != 1700000201 || got[0].Added != 4 || got[0].Removed != 1 || got[0].Kept != 9 || got[0].Size != 10 {
		t.Fatalf("unexpected churn point: %#v", got[0])
	}
}
