package engine

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/firehol/update-ipsets/pkg/cache"
)

func TestRefreshRotationStatsFromLedgerComputesBoundedChangeRatio(t *testing.T) {
	root := t.TempDir()
	libDir := filepath.Join(root, "lib")
	feedDir := filepath.Join(libDir, "sample")
	if err := os.MkdirAll(feedDir, 0o755); err != nil {
		t.Fatal(err)
	}

	historyCSV := "DateTime,Entries,UniqueIPs\n1711929600,10,10\n1711933200,10,10\n1711936800,10,10\n"
	if err := os.WriteFile(filepath.Join(feedDir, "history.csv"), []byte(historyCSV), 0o644); err != nil {
		t.Fatal(err)
	}
	changesetsCSV := "DateTime,AddedIPs,RemovedIPs\n1711929600,1,0\n1711933200,5,5\n1711936800,10,10\n"
	if err := os.WriteFile(filepath.Join(feedDir, "changesets.csv"), []byte(changesetsCSV), 0o644); err != nil {
		t.Fatal(err)
	}

	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.LibDir = libDir
	}))
	entry := &cache.Entry{Name: "sample"}
	if !eng.refreshRotationStatsFromLedger("sample", entry) {
		t.Fatal("expected rotation stats refresh to succeed")
	}

	if got, want := entry.RotationSamples, 2; got != want {
		t.Fatalf("unexpected turnover sample count: got %d want %d", got, want)
	}
	if got, want := entry.ChangeRatioSamples, 2; got != want {
		t.Fatalf("unexpected change-ratio sample count: got %d want %d", got, want)
	}
	if got, want := entry.RotationMedianPct, 150.0; !almostEqual(got, want) {
		t.Fatalf("unexpected turnover median: got %.2f want %.2f", got, want)
	}
	if got, want := entry.ChangeRatioMedianPct, 83.33; !almostEqual(got, want) {
		t.Fatalf("unexpected change-ratio median: got %.2f want %.2f", got, want)
	}
	if got := entry.ChangeRatioP75Pct; got > 100 {
		t.Fatalf("change-ratio percentile must stay bounded, got %.2f", got)
	}
}

func almostEqual(got, want float64) bool {
	return math.Abs(got-want) < 0.01
}
