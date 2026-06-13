package engine

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/output"
)

func TestStagedPublishBatchPublishesNestedFiles(t *testing.T) {
	liveDir := t.TempDir()
	batch, err := newStagedPublishBatch(liveDir, "", ".test-web-*")
	if err != nil {
		t.Fatalf("newStagedPublishBatch() error = %v", err)
	}
	defer batch.cleanup()
	if info, err := os.Stat(batch.stageDir); err != nil {
		t.Fatalf("Stat(stageDir) error = %v", err)
	} else if got := info.Mode().Perm(); got != generatedDirMode {
		t.Fatalf("stage dir mode = %04o, want %04o", got, generatedDirMode)
	}

	stageFiles := map[string]string{
		"countries/index.json": `{"countries":[]}` + "\n",
		"countries/US.json":    `{"code":"US"}` + "\n",
		"asns/index.json":      `{"asns":[]}` + "\n",
	}
	for rel, body := range stageFiles {
		path := filepath.Join(batch.stageDir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", path, err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", path, err)
		}
	}

	published, err := batch.publish()
	if err != nil {
		t.Fatalf("publish() error = %v", err)
	}
	slices.Sort(published)
	want := []string{
		filepath.Join(liveDir, "asns", "index.json"),
		filepath.Join(liveDir, "countries", "US.json"),
		filepath.Join(liveDir, "countries", "index.json"),
	}
	if len(published) != len(want) {
		t.Fatalf("published len = %d, want %d: %v", len(published), len(want), published)
	}
	for i := range want {
		if published[i] != want[i] {
			t.Fatalf("published[%d] = %q, want %q", i, published[i], want[i])
		}
	}
	for rel, body := range stageFiles {
		path := filepath.Join(liveDir, rel)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", rel, err)
		}
		if string(data) != body {
			t.Fatalf("body for %q = %q, want %q", rel, string(data), body)
		}
		if info, err := os.Stat(path); err != nil {
			t.Fatalf("Stat(%q) error = %v", rel, err)
		} else if got := info.Mode().Perm(); got != generatedFileMode {
			t.Fatalf("mode for %q = %04o, want %04o", rel, got, generatedFileMode)
		}
	}
	for _, rel := range []string{"countries", "asns"} {
		path := filepath.Join(liveDir, rel)
		if info, err := os.Stat(path); err != nil {
			t.Fatalf("Stat(%q) error = %v", rel, err)
		} else if got := info.Mode().Perm(); got != generatedDirMode {
			t.Fatalf("mode for %q = %04o, want %04o", rel, got, generatedDirMode)
		}
	}
}

func TestStagedPublishBatchDeletesMarkedFilesAndPrunesParents(t *testing.T) {
	liveDir := t.TempDir()
	oldPath := filepath.Join(liveDir, "countries", "ZZ.json")
	if err := os.MkdirAll(filepath.Dir(oldPath), 0o700); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(oldPath), err)
	}
	if err := os.WriteFile(oldPath, []byte(`{"code":"ZZ"}`+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", oldPath, err)
	}

	batch, err := newStagedPublishBatch(liveDir, "", ".test-web-*")
	if err != nil {
		t.Fatalf("newStagedPublishBatch() error = %v", err)
	}
	defer batch.cleanup()
	batch.markDelete("countries/ZZ.json")

	published, err := batch.publish()
	if err != nil {
		t.Fatalf("publish() error = %v", err)
	}
	if len(published) != 1 || published[0] != oldPath {
		t.Fatalf("publish() touched files = %v, want deleted path %q", published, oldPath)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("expected %q to be deleted, stat err = %v", oldPath, err)
	}
	if _, err := os.Stat(filepath.Join(liveDir, "countries")); !os.IsNotExist(err) {
		t.Fatalf("expected empty countries dir to be pruned, stat err = %v", err)
	}
}

func TestStagedPublishBatchAppliesGeneratedFileTimestamps(t *testing.T) {
	liveDir := t.TempDir()
	batch, err := newStagedPublishBatch(liveDir, "", ".test-web-*")
	if err != nil {
		t.Fatalf("newStagedPublishBatch() error = %v", err)
	}
	defer batch.cleanup()

	rel := "sample.json"
	stagePath := filepath.Join(batch.stageDir, rel)
	if err := os.WriteFile(stagePath, []byte(`{"name":"sample"}`+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", stagePath, err)
	}
	logical := time.Date(2026, 4, 29, 9, 34, 19, 0, time.UTC)
	if err := batch.applyGeneratedFileTimestamps([]output.GeneratedFile{{
		Path:      filepath.Join(liveDir, rel),
		Timestamp: logical,
	}}); err != nil {
		t.Fatalf("applyGeneratedFileTimestamps() error = %v", err)
	}

	if _, err := batch.publish(); err != nil {
		t.Fatalf("publish() error = %v", err)
	}
	info, err := os.Stat(filepath.Join(liveDir, rel))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.ModTime().UTC(); !got.Equal(logical) {
		t.Fatalf("published mtime = %s, want logical timestamp %s", got, logical)
	}
}

func TestCleanupStalePublishStageDirsOnlyRemovesOldKnownStages(t *testing.T) {
	root := t.TempDir()
	cutoff := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	oldStage := filepath.Join(root, ".update-ipsets-web-old")
	activeStage := filepath.Join(root, ".update-ipsets-web-active")
	otherHidden := filepath.Join(root, ".other-stage")
	publishedDir := filepath.Join(root, "countries")
	publishedFile := filepath.Join(root, "sample.json")
	for _, dir := range []string{oldStage, activeStage, otherHidden, publishedDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(oldStage, "leftover.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(old stage) error = %v", err)
	}
	if err := os.WriteFile(publishedFile, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(published) error = %v", err)
	}
	oldTime := cutoff.Add(-time.Hour)
	futureTime := cutoff.Add(time.Hour)
	if err := os.Chtimes(oldStage, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes(old stage) error = %v", err)
	}
	if err := os.Chtimes(activeStage, futureTime, futureTime); err != nil {
		t.Fatalf("Chtimes(active stage) error = %v", err)
	}
	if err := os.Chtimes(otherHidden, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes(other hidden) error = %v", err)
	}

	removed, err := cleanupStalePublishStageDirs(root, webPublishStagePrefix, cutoff)
	if err != nil {
		t.Fatalf("cleanupStalePublishStageDirs() error = %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if _, err := os.Stat(oldStage); !os.IsNotExist(err) {
		t.Fatalf("old stage still exists or unexpected stat error: %v", err)
	}
	for _, path := range []string{activeStage, otherHidden, publishedDir, publishedFile} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %q to remain, stat error = %v", path, err)
		}
	}
}

func TestEngineCleanupStalePublishStagesCoversWebAndEntities(t *testing.T) {
	cutoff := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	eng := newEngineFixture(t, withNow(func() time.Time { return cutoff }))
	if err := eng.ensureDirectories(); err != nil {
		t.Fatalf("ensureDirectories() error = %v", err)
	}
	webStage := filepath.Join(eng.outputDir(), ".update-ipsets-web-old")
	entityStage := filepath.Join(eng.entitiesDir(), ".update-ipsets-entities-old")
	for _, dir := range []string{webStage, entityStage} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", dir, err)
		}
		oldTime := cutoff.Add(-time.Hour)
		if err := os.Chtimes(dir, oldTime, oldTime); err != nil {
			t.Fatalf("Chtimes(%q) error = %v", dir, err)
		}
	}

	result, err := eng.CleanupStalePublishStages()
	if err != nil {
		t.Fatalf("CleanupStalePublishStages() error = %v", err)
	}
	if got, want := result.WebRemoved, 1; got != want {
		t.Fatalf("web removed = %d, want %d", got, want)
	}
	if got, want := result.EntityRemoved, 1; got != want {
		t.Fatalf("entity removed = %d, want %d", got, want)
	}
	for _, path := range []string{webStage, entityStage} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected %q to be removed, stat error = %v", path, err)
		}
	}
}

func TestEngineCleanupStalePublishStagesKeepsRecentStages(t *testing.T) {
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	eng := newEngineFixture(t, withNow(func() time.Time { return now }))
	if err := eng.ensureDirectories(); err != nil {
		t.Fatalf("ensureDirectories() error = %v", err)
	}
	webStage := filepath.Join(eng.outputDir(), ".update-ipsets-web-recent")
	entityStage := filepath.Join(eng.entitiesDir(), ".update-ipsets-entities-recent")
	recentTime := now.Add(-time.Minute)
	for _, dir := range []string{webStage, entityStage} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", dir, err)
		}
		if err := os.Chtimes(dir, recentTime, recentTime); err != nil {
			t.Fatalf("Chtimes(%q) error = %v", dir, err)
		}
	}

	result, err := eng.CleanupStalePublishStages()
	if err != nil {
		t.Fatalf("CleanupStalePublishStages() error = %v", err)
	}
	if result.TotalRemoved() != 0 {
		t.Fatalf("removed = %+v, want no recent stage removals", result)
	}
	for _, path := range []string{webStage, entityStage} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected recent stage %q to remain, stat error = %v", path, err)
		}
	}
}
