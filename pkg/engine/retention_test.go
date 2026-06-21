package engine

import (
	"bytes"
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
	if equal, err := iprange.RangeSourcesEqualContext(t.Context(), got.newSet, want.newSet); err != nil || !equal {
		if err != nil {
			t.Fatalf("RangeSourcesEqualContext() error = %v", err)
		}
		t.Fatalf("new set from file-backed diff does not match in-memory diff")
	}
}

func TestReconcileRetentionCohortUsesFileBackedCompare(t *testing.T) {
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
	started := addedAt - 3600
	updatedAt := addedAt + 7200
	result, err := eng.reconcileRetentionCohorts(t.Context(), "sample", paths, started, updatedAt, current, map[int]uint64{})
	if err != nil {
		t.Fatalf("reconcileRetentionCohorts() error = %v", err)
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
	if equal, err := iprange.RangeSourcesEqualContext(t.Context(), reloaded, current); err != nil || !equal {
		if err != nil {
			t.Fatalf("RangeSourcesEqualContext() error = %v", err)
		}
		t.Fatalf("rewritten cohort does not match still-listed current set")
	}
}

func TestReconcileRetentionCohortsAcrossCompareBatches(t *testing.T) {
	root := t.TempDir()
	libDir := filepath.Join(root, "lib")
	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.LibDir = libDir
	}))
	paths, err := eng.prepareRetentionUpdatePaths("sample")
	if err != nil {
		t.Fatalf("prepareRetentionUpdatePaths() error = %v", err)
	}

	baseAt := int64(1_700_000_000)
	total := retentionReconcileCompareBatchSize + 7
	deleteIndex := retentionReconcileCompareBatchSize - 1
	rewriteIndex := retentionReconcileCompareBatchSize + 3
	for i := 0; i < total; i++ {
		lo := uint32(1000 + i*10)
		addedAt := baseAt + int64(i*3600)
		writeRetentionTestCohort(t, paths.newDir, addedAt, []iprange.Range{{Lo: lo, Hi: lo + 4}})
	}

	unchangedAt := baseAt
	unchangedPath := filepath.Join(paths.newDir, strconvFormatInt(unchangedAt))
	unchangedBefore, err := os.ReadFile(unchangedPath)
	if err != nil {
		t.Fatalf("read unchanged cohort before reconcile: %v", err)
	}
	unchangedInfoBefore, err := os.Stat(unchangedPath)
	if err != nil {
		t.Fatalf("stat unchanged cohort before reconcile: %v", err)
	}

	current := iprange.New("sample")
	for i := 0; i < total; i++ {
		if i == deleteIndex {
			continue
		}
		lo := uint32(1000 + i*10)
		r := iprange.Range{Lo: lo, Hi: lo + 4}
		if i == rewriteIndex {
			r.Lo = lo + 2
		}
		if err := current.AddRange(r); err != nil {
			t.Fatalf("current AddRange(%v) error = %v", r, err)
		}
	}
	current.Optimize()

	started := baseAt - 3600
	updatedAt := baseAt + int64((total+1)*3600)
	result, err := eng.reconcileRetentionCohorts(t.Context(), "sample", paths, started, updatedAt, current, map[int]uint64{})
	if err != nil {
		t.Fatalf("reconcileRetentionCohorts() error = %v", err)
	}

	if got, want := len(result.cohorts), total-1; got != want {
		t.Fatalf("cohort count = %d, want %d", got, want)
	}
	deletedAt := baseAt + int64(deleteIndex*3600)
	if _, ok := result.cohorts[deletedAt]; ok {
		t.Fatalf("deleted cohort %d still present", deletedAt)
	}
	if _, err := os.Stat(filepath.Join(paths.newDir, strconvFormatInt(deletedAt))); !os.IsNotExist(err) {
		t.Fatalf("deleted cohort stat error = %v, want not exist", err)
	}

	rewrittenAt := baseAt + int64(rewriteIndex*3600)
	if got, want := result.cohorts[rewrittenAt], uint64(3); got != want {
		t.Fatalf("rewritten cohort count = %d, want %d", got, want)
	}
	rewritten, err := loadSnapshotSet(t.Context(), "sample", libDir, filepath.Join("sample", "new", strconvFormatInt(rewrittenAt)))
	if err != nil {
		t.Fatalf("load rewritten cohort: %v", err)
	}
	wantRewritten := iprange.New("sample")
	rewriteLo := uint32(1000 + rewriteIndex*10)
	if err := wantRewritten.AddRange(iprange.Range{Lo: rewriteLo + 2, Hi: rewriteLo + 4}); err != nil {
		t.Fatalf("want rewritten AddRange() error = %v", err)
	}
	wantRewritten.Optimize()
	if equal, err := iprange.RangeSourcesEqualContext(t.Context(), rewritten, wantRewritten); err != nil || !equal {
		if err != nil {
			t.Fatalf("RangeSourcesEqualContext() error = %v", err)
		}
		t.Fatalf("rewritten cohort does not contain only retained IPs")
	}

	unchangedAfter, err := os.ReadFile(unchangedPath)
	if err != nil {
		t.Fatalf("read unchanged cohort after reconcile: %v", err)
	}
	unchangedInfoAfter, err := os.Stat(unchangedPath)
	if err != nil {
		t.Fatalf("stat unchanged cohort after reconcile: %v", err)
	}
	if !bytes.Equal(unchangedAfter, unchangedBefore) {
		t.Fatalf("unchanged cohort body was rewritten")
	}
	if !unchangedInfoAfter.ModTime().Equal(unchangedInfoBefore.ModTime()) {
		t.Fatalf("unchanged cohort mtime = %s, want %s", unchangedInfoAfter.ModTime(), unchangedInfoBefore.ModTime())
	}
}

func TestReconcileRetentionCohortsOnlyRewritesAffectedFiles(t *testing.T) {
	root := t.TempDir()
	libDir := filepath.Join(root, "lib")
	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.LibDir = libDir
	}))
	paths, err := eng.prepareRetentionUpdatePaths("sample")
	if err != nil {
		t.Fatalf("prepareRetentionUpdatePaths() error = %v", err)
	}

	unchangedAt := int64(1_700_000_000)
	rewriteAt := unchangedAt + 3600
	deleteAt := unchangedAt + 7200
	writeRetentionTestCohort(t, paths.newDir, unchangedAt, []iprange.Range{{Lo: 10, Hi: 19}})
	writeRetentionTestCohort(t, paths.newDir, rewriteAt, []iprange.Range{{Lo: 30, Hi: 39}})
	writeRetentionTestCohort(t, paths.newDir, deleteAt, []iprange.Range{{Lo: 50, Hi: 59}})

	unchangedPath := filepath.Join(paths.newDir, strconvFormatInt(unchangedAt))
	unchangedBefore, err := os.ReadFile(unchangedPath)
	if err != nil {
		t.Fatalf("read unchanged cohort before reconcile: %v", err)
	}
	unchangedInfoBefore, err := os.Stat(unchangedPath)
	if err != nil {
		t.Fatalf("stat unchanged cohort before reconcile: %v", err)
	}

	current := iprange.New("sample")
	for _, r := range []iprange.Range{{Lo: 10, Hi: 19}, {Lo: 35, Hi: 39}} {
		if err := current.AddRange(r); err != nil {
			t.Fatalf("current AddRange(%v) error = %v", r, err)
		}
	}
	current.Optimize()

	started := unchangedAt - 3600
	updatedAt := unchangedAt + 3*3600
	result, err := eng.reconcileRetentionCohorts(t.Context(), "sample", paths, started, updatedAt, current, map[int]uint64{})
	if err != nil {
		t.Fatalf("reconcileRetentionCohorts() error = %v", err)
	}

	unchangedAfter, err := os.ReadFile(unchangedPath)
	if err != nil {
		t.Fatalf("read unchanged cohort after reconcile: %v", err)
	}
	unchangedInfoAfter, err := os.Stat(unchangedPath)
	if err != nil {
		t.Fatalf("stat unchanged cohort after reconcile: %v", err)
	}
	if !bytes.Equal(unchangedAfter, unchangedBefore) {
		t.Fatalf("unchanged cohort body was rewritten")
	}
	if !unchangedInfoAfter.ModTime().Equal(unchangedInfoBefore.ModTime()) {
		t.Fatalf("unchanged cohort mtime = %s, want %s", unchangedInfoAfter.ModTime(), unchangedInfoBefore.ModTime())
	}

	if got, want := result.cohorts[unchangedAt], uint64(10); got != want {
		t.Fatalf("unchanged cohort count = %d, want %d", got, want)
	}
	if got, want := result.cohorts[rewriteAt], uint64(5); got != want {
		t.Fatalf("rewritten cohort count = %d, want %d", got, want)
	}
	if _, ok := result.cohorts[deleteAt]; ok {
		t.Fatalf("deleted cohort still present in result: %+v", result.cohorts)
	}
	if got, want := result.currentBuckets[3], uint64(10); got != want {
		t.Fatalf("current bucket for unchanged cohort = %d, want %d", got, want)
	}
	if got, want := result.currentBuckets[2], uint64(5); got != want {
		t.Fatalf("current bucket for rewritten cohort = %d, want %d", got, want)
	}

	rewrittenPath := filepath.Join(paths.newDir, strconvFormatInt(rewriteAt))
	rewritten, err := loadSnapshotSet(t.Context(), "sample", libDir, filepath.Join("sample", "new", strconvFormatInt(rewriteAt)))
	if err != nil {
		t.Fatalf("load rewritten cohort: %v", err)
	}
	expectedRewrite := iprange.New("sample")
	if err := expectedRewrite.AddRange(iprange.Range{Lo: 35, Hi: 39}); err != nil {
		t.Fatalf("expected rewrite AddRange() error = %v", err)
	}
	expectedRewrite.Optimize()
	if equal, err := iprange.RangeSourcesEqualContext(t.Context(), rewritten, expectedRewrite); err != nil || !equal {
		if err != nil {
			t.Fatalf("RangeSourcesEqualContext() error = %v", err)
		}
		t.Fatalf("rewritten cohort does not contain only retained IPs")
	}
	rewrittenInfo, err := os.Stat(rewrittenPath)
	if err != nil {
		t.Fatalf("stat rewritten cohort: %v", err)
	}
	if got, want := rewrittenInfo.ModTime().Unix(), rewriteAt; got != want {
		t.Fatalf("rewritten cohort mtime = %d, want %d", got, want)
	}
	if _, err := os.Stat(filepath.Join(paths.newDir, strconvFormatInt(deleteAt))); !os.IsNotExist(err) {
		t.Fatalf("deleted cohort stat error = %v, want not exist", err)
	}

	if err := writeRetentionCohortIndex(filepath.Join(paths.dir, "retention_cohorts.csv"), result.cohorts); err != nil {
		t.Fatalf("write retention cohort index: %v", err)
	}
	resetRetentionCohortCacheForTest(eng, "sample")
	if got := eng.queryMatchFirstSeen(t.Context(), "sample", 10); got != unchangedAt {
		t.Fatalf("first_seen for unchanged IP = %d, want %d", got, unchangedAt)
	}
	if got := eng.queryMatchFirstSeen(t.Context(), "sample", 35); got != rewriteAt {
		t.Fatalf("first_seen for retained rewritten IP = %d, want %d", got, rewriteAt)
	}
	if got := eng.queryMatchFirstSeen(t.Context(), "sample", 30); got != 0 {
		t.Fatalf("first_seen for removed rewritten IP = %d, want 0", got)
	}
	if got := eng.queryMatchFirstSeen(t.Context(), "sample", 50); got != 0 {
		t.Fatalf("first_seen for deleted cohort IP = %d, want 0", got)
	}
}

func TestRetentionReconcileUsesIPrangeComparePairsBeforeMaterializing(t *testing.T) {
	sourceBytes, err := os.ReadFile("retention_update.go")
	if err != nil {
		t.Fatalf("read retention_update.go: %v", err)
	}
	source := string(sourceBytes)
	if !strings.Contains(source, "iprange.CompareSourcePairs") {
		t.Fatalf("retention reconciliation must use pkg/iprange CompareSourcePairs")
	}
	if strings.Contains(source, "iprange.CompareNextSources") {
		t.Fatalf("retention reconciliation must not use one CompareNextSources call per cohort")
	}
	if strings.Contains(source, "func (e *Engine) reconcileRetentionCohort(") {
		t.Fatalf("retention reconciliation must not restore the old engine-owned per-cohort comparison")
	}
	sectionStart := strings.Index(source, "func (e *Engine) applyRetentionCohortCompare")
	sectionEnd := strings.Index(source, "func retentionHours")
	if sectionStart < 0 || sectionEnd < sectionStart {
		t.Fatalf("cannot locate applyRetentionCohortCompare cost-shape section")
	}
	section := source[sectionStart:sectionEnd]
	noChangeBranch := strings.Index(section, "if removedCount == 0")
	materialize := strings.Index(section, "IntersectSourcesContext(")
	if noChangeBranch < 0 || materialize < 0 || noChangeBranch > materialize {
		t.Fatalf("unchanged cohorts must return before materializing the intersection")
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
