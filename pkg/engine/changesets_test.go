package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/firehol/update-ipsets/pkg/insights"
)

func TestChangesetReadersIgnoreZeroDeltaRows(t *testing.T) {
	root := t.TempDir()
	libDir := filepath.Join(root, "lib")
	feedDir := filepath.Join(libDir, "sample")
	if err := os.MkdirAll(feedDir, 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(feedDir, "changesets.csv"),
		[]byte("DateTime,AddedIPs,RemovedIPs\n100,4,0\n101,0,0\n102,2,2\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.LibDir = libDir
	}))
	points, err := eng.ChangesetSeries("sample")
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 real changeset point after bootstrap filtering, got %#v", points)
	}
	if points[0].Timestamp != 102 {
		t.Fatalf("expected zero-delta and bootstrap rows to be filtered, got %#v", points)
	}

	churn := eng.readChurnSeries("sample", []insights.SizePoint{
		{TS: 100, Size: 10},
		{TS: 101, Size: 10},
		{TS: 102, Size: 12},
	})
	if len(churn) != 1 {
		t.Fatalf("expected 1 real churn point after bootstrap filtering, got %#v", churn)
	}
	if churn[0].TS != 102 {
		t.Fatalf("expected zero-delta and bootstrap churn rows to be filtered, got %#v", churn)
	}
}
