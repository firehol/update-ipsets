package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/iprange"
)

func TestRetentionIgnoresAtomicTempFilesInNewDir(t *testing.T) {
	root := t.TempDir()
	newDir := filepath.Join(root, "lib", "sample", "new")
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newDir, ".tmp-123456"), []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}

	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.LibDir = filepath.Join(root, "lib")
	}), withNow(func() time.Time { return time.Date(2026, 4, 11, 1, 0, 0, 0, time.UTC) }))

	previous := iprange.New("sample")
	current := iprange.New("sample")
	if err := current.Add(10, 10); err != nil {
		t.Fatal(err)
	}

	updatedAt := time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC)
	if err := eng.updateRetention(t.Context(), "sample", previous, current, updatedAt); err != nil {
		t.Fatal(err)
	}
	retention, err := eng.buildRetentionData(t.Context(), "sample", updatedAt.Unix())
	if err != nil {
		t.Fatal(err)
	}
	if got, want := retention.Current.Total, uint64(1); got != want {
		t.Fatalf("current retention total = %d, want %d", got, want)
	}
	if got := retention.Past.Total; got != 0 {
		t.Fatalf("past retention total = %d, want 0", got)
	}
}

func TestLoadRetentionCohortsFromIndex(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "lib", "sample")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cohorts := map[int64]uint64{
		1700000000: 11,
		1700003600: 22,
	}
	if err := writeRetentionCohortIndex(filepath.Join(dir, "retention_cohorts.csv"), cohorts); err != nil {
		t.Fatalf("write retention cohort index: %v", err)
	}
	loaded, err := loadRetentionCohorts(t.Context(), dir)
	if err != nil {
		t.Fatalf("load retention cohorts: %v", err)
	}
	if len(loaded) != len(cohorts) {
		t.Fatalf("loaded cohort len = %d, want %d", len(loaded), len(cohorts))
	}
	for addedAt, want := range cohorts {
		if got := loaded[addedAt]; got != want {
			t.Fatalf("loaded cohort %d = %d, want %d", addedAt, got, want)
		}
	}
}
