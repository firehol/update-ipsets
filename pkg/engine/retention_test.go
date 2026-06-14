package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/iprange"
)

func TestRetentionIgnoresAtomicTempFilesInNewDir(t *testing.T) {
	root := t.TempDir()
	newDir := filepath.Join(root, "lib", "sample", "new")
	if err := os.MkdirAll(newDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newDir, ".tmp-123456"), []byte("partial"), 0o600); err != nil {
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

func TestRetentionDiffUsesFileBackedPreviousLatest(t *testing.T) {
	root := t.TempDir()
	libDir := filepath.Join(root, "lib")
	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.LibDir = libDir
	}))

	previous := iprange.New("sample")
	for _, r := range []iprange.Range{{Lo: 10, Hi: 19}, {Lo: 30, Hi: 39}} {
		if err := previous.AddRange(r); err != nil {
			t.Fatalf("previous AddRange(%v) error = %v", r, err)
		}
	}
	previous.Optimize()
	if err := writeBinaryPath(filepath.Join(libDir, "sample", "latest"), previous, time.Date(2026, 6, 14, 8, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("write previous latest: %v", err)
	}

	current := iprange.New("sample")
	for _, r := range []iprange.Range{{Lo: 15, Hi: 24}, {Lo: 30, Hi: 39}, {Lo: 50, Hi: 59}} {
		if err := current.AddRange(r); err != nil {
			t.Fatalf("current AddRange(%v) error = %v", r, err)
		}
	}
	current.Optimize()

	previousSource, err := eng.openPreviousLatestSet(t.Context(), "sample")
	if err != nil {
		t.Fatalf("openPreviousLatestSet() error = %v", err)
	}
	defer func() { _ = previousSource.Close() }()
	if _, ok := previousSource.RangeSource.(iprange.FileSet); !ok {
		t.Fatalf("previous source type = %T, want iprange.FileSet", previousSource.RangeSource)
	}

	got, err := eng.retentionDiffFromSources(t.Context(), "sample", previousSource.RangeSource, current)
	if err != nil {
		t.Fatalf("retentionDiffFromSources() error = %v", err)
	}
	want := retentionDiff(previous, current)
	if got.added != want.added || got.removed != want.removed {
		t.Fatalf("diff added/removed = %d/%d, want %d/%d", got.added, got.removed, want.added, want.removed)
	}
	if !rangeSourcesEqual(got.newSet, want.newSet) {
		t.Fatalf("new set from file-backed diff does not match in-memory diff")
	}
}

func TestReconcileRetentionCohortUsesFileBackedSource(t *testing.T) {
	root := t.TempDir()
	libDir := filepath.Join(root, "lib")
	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.LibDir = libDir
	}))
	paths, err := eng.prepareRetentionUpdatePaths("sample")
	if err != nil {
		t.Fatalf("prepareRetentionUpdatePaths() error = %v", err)
	}

	addedAt := int64(1_700_000_000)
	baseName := "1700000000"
	cohort := iprange.New("sample")
	for _, r := range []iprange.Range{{Lo: 10, Hi: 19}, {Lo: 30, Hi: 39}} {
		if err := cohort.AddRange(r); err != nil {
			t.Fatalf("cohort AddRange(%v) error = %v", r, err)
		}
	}
	cohort.Optimize()
	cohortPath := filepath.Join(paths.newDir, baseName)
	if err := writeBinaryPath(cohortPath, cohort, time.Unix(addedAt, 0).UTC()); err != nil {
		t.Fatalf("write cohort: %v", err)
	}
	source, err := openRetentionCohortSet(t.Context(), "sample", libDir, filepath.Join("sample", "new", baseName), cohortPath)
	if err != nil {
		t.Fatalf("openRetentionCohortSet() error = %v", err)
	}
	if _, ok := source.RangeSource.(iprange.FileSet); !ok {
		t.Fatalf("cohort source type = %T, want iprange.FileSet", source.RangeSource)
	}
	if err := source.Close(); err != nil {
		t.Fatalf("close cohort source: %v", err)
	}

	current := iprange.New("sample")
	for _, r := range []iprange.Range{{Lo: 15, Hi: 19}, {Lo: 30, Hi: 39}} {
		if err := current.AddRange(r); err != nil {
			t.Fatalf("current AddRange(%v) error = %v", r, err)
		}
	}
	current.Optimize()
	result := retentionReconcileResult{
		cohorts:        map[int64]uint64{},
		currentBuckets: map[int]uint64{},
	}
	started := addedAt - 3600
	updatedAt := addedAt + 7200
	if err := eng.reconcileRetentionCohort(t.Context(), "sample", paths, started, updatedAt, current, map[int]uint64{}, baseName, addedAt, &result); err != nil {
		t.Fatalf("reconcileRetentionCohort() error = %v", err)
	}

	if got, want := result.cohorts[addedAt], uint64(15); got != want {
		t.Fatalf("cohort count = %d, want %d", got, want)
	}
	if got, want := result.currentBuckets[2], uint64(15); got != want {
		t.Fatalf("current bucket = %d, want %d", got, want)
	}
	body, err := os.ReadFile(filepath.Join(paths.dir, "retention.csv"))
	if err != nil {
		t.Fatalf("read retention.csv: %v", err)
	}
	if !strings.Contains(string(body), "1700007200,1700000000,2,5\n") {
		t.Fatalf("retention.csv missing removal row, got:\n%s", body)
	}
	reloaded, err := loadSnapshotSet(t.Context(), "sample", libDir, filepath.Join("sample", "new", baseName))
	if err != nil {
		t.Fatalf("load rewritten cohort: %v", err)
	}
	if !rangeSourcesEqual(reloaded, current) {
		t.Fatalf("rewritten cohort does not match still-listed current set")
	}
}

func TestLoadRetentionCohortsFromIndex(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "lib", "sample")
	if err := os.MkdirAll(dir, 0o700); err != nil {
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
