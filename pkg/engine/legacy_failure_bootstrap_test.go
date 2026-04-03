package engine

import (
	"path/filepath"
	"testing"

	"github.com/firehol/update-ipsets/pkg/cache"
)

func TestBootstrapLegacyFailureStartsUsesImportedCheckedDate(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "data")
	importDir := filepath.Join(root, "import-bash-version")
	cachePath := filepath.Join(baseDir, ".cache.json")
	legacyPath := filepath.Join(importDir, "merged-cache.json")

	st := cache.New()
	entry := st.Entry("sample")
	entry.Name = "sample"
	entry.DownloadFailures = 3
	entry.FailureStartedDate = 1_776_000_000

	legacy := cache.New()
	legacyEntry := legacy.Entry("sample")
	legacyEntry.Name = "sample"
	legacyEntry.DownloadFailures = 1
	legacyEntry.CheckedDate = 1_745_090_223

	if err := cache.Save(legacyPath, legacy); err != nil {
		t.Fatalf("cache.Save(legacy) returned error: %v", err)
	}

	e := newEngineFixture(t, withState(st), withRuntime(func(rt *Runtime) {
		rt.BaseDir = baseDir
	}))
	e.cachePath = cachePath

	if err := e.bootstrapLegacyFailureStarts(); err != nil {
		t.Fatalf("bootstrapLegacyFailureStarts returned error: %v", err)
	}

	got := e.state.EntrySnapshot("sample")
	if got == nil {
		t.Fatal("missing entry after bootstrap")
	}
	if got.FailureStartedDate != legacyEntry.CheckedDate {
		t.Fatalf("failure_started_date = %d, want %d", got.FailureStartedDate, legacyEntry.CheckedDate)
	}

	persisted, err := cache.Load(cachePath)
	if err != nil {
		t.Fatalf("cache.Load returned error: %v", err)
	}
	saved := persisted.EntrySnapshot("sample")
	if saved == nil {
		t.Fatal("missing persisted entry after bootstrap")
	}
	if saved.FailureStartedDate != legacyEntry.CheckedDate {
		t.Fatalf("persisted failure_started_date = %d, want %d", saved.FailureStartedDate, legacyEntry.CheckedDate)
	}
}

func TestBootstrapLegacyFailureStartsSkipsRecoveredFeed(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "data")
	importDir := filepath.Join(root, "import-bash-version")

	st := cache.New()
	entry := st.Entry("sample")
	entry.Name = "sample"
	entry.DownloadFailures = 2
	entry.FailureStartedDate = 1_775_906_038
	entry.ProcessedDate = 1_775_000_000

	legacy := cache.New()
	legacyEntry := legacy.Entry("sample")
	legacyEntry.Name = "sample"
	legacyEntry.DownloadFailures = 1
	legacyEntry.CheckedDate = 1_745_090_223

	if err := cache.Save(filepath.Join(importDir, "merged-cache.json"), legacy); err != nil {
		t.Fatalf("cache.Save(legacy) returned error: %v", err)
	}

	e := newEngineFixture(t, withState(st), withRuntime(func(rt *Runtime) {
		rt.BaseDir = baseDir
	}))

	if err := e.bootstrapLegacyFailureStarts(); err != nil {
		t.Fatalf("bootstrapLegacyFailureStarts returned error: %v", err)
	}

	got := e.state.EntrySnapshot("sample")
	if got == nil {
		t.Fatal("missing entry after bootstrap")
	}
	if got.FailureStartedDate != 1_775_906_038 {
		t.Fatalf("failure_started_date = %d, want unchanged %d", got.FailureStartedDate, int64(1_775_906_038))
	}
}

func TestBootstrapLegacyFailureStartsFallsBackToImportD1(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "data")
	importDir := filepath.Join(root, "import-d1")

	st := cache.New()
	entry := st.Entry("sample")
	entry.Name = "sample"
	entry.DownloadFailures = 2

	legacy := cache.New()
	legacyEntry := legacy.Entry("sample")
	legacyEntry.Name = "sample"
	legacyEntry.DownloadFailures = 1
	legacyEntry.CheckedDate = 1_745_090_223

	if err := cache.Save(filepath.Join(importDir, "merged-cache.json"), legacy); err != nil {
		t.Fatalf("cache.Save(legacy) returned error: %v", err)
	}

	e := newEngineFixture(t, withState(st), withRuntime(func(rt *Runtime) {
		rt.BaseDir = baseDir
	}))

	if err := e.bootstrapLegacyFailureStarts(); err != nil {
		t.Fatalf("bootstrapLegacyFailureStarts returned error: %v", err)
	}

	got := e.state.EntrySnapshot("sample")
	if got == nil {
		t.Fatal("missing entry after bootstrap")
	}
	if got.FailureStartedDate != legacyEntry.CheckedDate {
		t.Fatalf("failure_started_date = %d, want %d", got.FailureStartedDate, legacyEntry.CheckedDate)
	}
}
