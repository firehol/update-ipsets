package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

func TestStagedPublishBatchPublishWorkTotalCountsFilesAndDeletes(t *testing.T) {
	liveDir := t.TempDir()
	batch, err := newStagedPublishBatch(liveDir, "", ".test-web-*")
	if err != nil {
		t.Fatalf("newStagedPublishBatch() error = %v", err)
	}
	defer batch.cleanup()

	stageFiles := []string{
		"countries/index.json",
		"countries/US.json",
		"asns/index.json",
	}
	for _, rel := range stageFiles {
		path := filepath.Join(batch.stageDir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", path, err)
		}
		if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", path, err)
		}
	}
	batch.markDelete("old/feed.json")

	total, err := batch.publishWorkTotal(t.Context())
	if err != nil {
		t.Fatalf("publishWorkTotal() error = %v", err)
	}
	if total != 4 {
		t.Fatalf("publishWorkTotal() = %d, want 4", total)
	}
}

func TestStagedPublishBatchPublishContextCancelledBeforeStartLeavesLiveUntouched(t *testing.T) {
	liveDir := t.TempDir()
	rel := "sample.json"
	livePath := filepath.Join(liveDir, rel)
	if err := os.WriteFile(livePath, []byte("old\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(live) error = %v", err)
	}

	batch, err := newStagedPublishBatch(liveDir, "", ".test-web-*")
	if err != nil {
		t.Fatalf("newStagedPublishBatch() error = %v", err)
	}
	defer batch.cleanup()
	stagePath := filepath.Join(batch.stageDir, rel)
	if err := os.WriteFile(stagePath, []byte("new\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(stage) error = %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	published, err := batch.publishContext(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("publishContext() error = %v, want context.Canceled", err)
	}
	if len(published) != 0 {
		t.Fatalf("publishContext() published = %v, want none", published)
	}
	body, err := os.ReadFile(livePath)
	if err != nil {
		t.Fatalf("ReadFile(live) error = %v", err)
	}
	if got, want := string(body), "old\n"; got != want {
		t.Fatalf("live body = %q, want %q", got, want)
	}
	if _, err := os.Stat(stagePath); err != nil {
		t.Fatalf("stage file should remain for caller cleanup, stat err = %v", err)
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

func TestSameRegularFileContentContextCancelled(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "left")
	right := filepath.Join(dir, "right")
	if err := os.WriteFile(left, []byte("same\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(left) error = %v", err)
	}
	if err := os.WriteFile(right, []byte("same\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(right) error = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	same, err := sameRegularFileContentContext(ctx, left, right)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("sameRegularFileContentContext() error = %v, want context.Canceled", err)
	}
	if same {
		t.Fatal("sameRegularFileContentContext() returned true for cancelled context")
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

func TestStagedPublishBatchTouchesIdenticalLiveFileInPlace(t *testing.T) {
	liveDir := t.TempDir()
	rel := "sample.json"
	livePath := filepath.Join(liveDir, rel)
	body := []byte(`{"name":"sample"}` + "\n")
	if err := os.WriteFile(livePath, body, 0o600); err != nil {
		t.Fatalf("WriteFile(live) error = %v", err)
	}
	oldTime := time.Date(2026, 4, 28, 9, 0, 0, 0, time.UTC)
	if err := os.Chtimes(livePath, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes(live) error = %v", err)
	}
	before, err := os.Stat(livePath)
	if err != nil {
		t.Fatalf("Stat(live before) error = %v", err)
	}

	batch, err := newStagedPublishBatch(liveDir, "", ".test-web-*")
	if err != nil {
		t.Fatalf("newStagedPublishBatch() error = %v", err)
	}
	defer batch.cleanup()
	stagePath := filepath.Join(batch.stageDir, rel)
	if err := os.WriteFile(stagePath, body, 0o600); err != nil {
		t.Fatalf("WriteFile(stage) error = %v", err)
	}
	logical := oldTime.Add(time.Hour)
	if err := os.Chtimes(stagePath, logical, logical); err != nil {
		t.Fatalf("Chtimes(stage) error = %v", err)
	}

	published, err := batch.publish()
	if err != nil {
		t.Fatalf("publish() error = %v", err)
	}
	if len(published) != 1 || published[0] != livePath {
		t.Fatalf("published = %v, want [%s]", published, livePath)
	}
	after, err := os.Stat(livePath)
	if err != nil {
		t.Fatalf("Stat(live after) error = %v", err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("identical live file was replaced instead of touched in place")
	}
	if got := after.ModTime().UTC(); !got.Equal(logical) {
		t.Fatalf("live mtime = %s, want %s", got, logical)
	}
	if _, err := os.Stat(stagePath); !os.IsNotExist(err) {
		t.Fatalf("stage file still exists or stat failed with %v", err)
	}
}

func TestEntityPublishBatchTouchesIdenticalLiveFileInPlace(t *testing.T) {
	eng := newEngineFixture(t)
	batch, err := eng.newEntityPublishBatch()
	if err != nil {
		t.Fatalf("newEntityPublishBatch() error = %v", err)
	}
	defer batch.cleanup()

	livePath := eng.entityVersionPath()
	body := []byte(entityArtifactsVersion + "\n")
	if err := os.WriteFile(livePath, body, 0o600); err != nil {
		t.Fatalf("WriteFile(live) error = %v", err)
	}
	oldTime := time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC)
	if err := os.Chtimes(livePath, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes(live) error = %v", err)
	}
	before, err := os.Stat(livePath)
	if err != nil {
		t.Fatalf("Stat(live before) error = %v", err)
	}

	stagePath := filepath.Join(batch.stageDir, "version")
	if err := os.WriteFile(stagePath, body, 0o600); err != nil {
		t.Fatalf("WriteFile(stage) error = %v", err)
	}
	logical := oldTime.Add(time.Hour)
	if err := os.Chtimes(stagePath, logical, logical); err != nil {
		t.Fatalf("Chtimes(stage) error = %v", err)
	}

	published, err := batch.publish()
	if err != nil {
		t.Fatalf("publish() error = %v", err)
	}
	if len(published) != 1 || published[0] != livePath {
		t.Fatalf("published = %v, want [%s]", published, livePath)
	}
	after, err := os.Stat(livePath)
	if err != nil {
		t.Fatalf("Stat(live after) error = %v", err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("identical entity file was replaced instead of touched in place")
	}
	if got := after.ModTime().UTC(); !got.Equal(logical) {
		t.Fatalf("live mtime = %s, want %s", got, logical)
	}
	if _, err := os.Stat(stagePath); !os.IsNotExist(err) {
		t.Fatalf("stage file still exists or stat failed with %v", err)
	}
}

func TestStagedPublishBatchPublishesChangedLiveFile(t *testing.T) {
	liveDir := t.TempDir()
	rel := "sample.json"
	livePath := filepath.Join(liveDir, rel)
	if err := os.WriteFile(livePath, []byte("old\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(live) error = %v", err)
	}

	batch, err := newStagedPublishBatch(liveDir, "", ".test-web-*")
	if err != nil {
		t.Fatalf("newStagedPublishBatch() error = %v", err)
	}
	defer batch.cleanup()
	stagePath := filepath.Join(batch.stageDir, rel)
	if err := os.WriteFile(stagePath, []byte("new\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(stage) error = %v", err)
	}

	published, err := batch.publish()
	if err != nil {
		t.Fatalf("publish() error = %v", err)
	}
	if len(published) != 1 || published[0] != livePath {
		t.Fatalf("published = %v, want [%s]", published, livePath)
	}
	body, err := os.ReadFile(livePath)
	if err != nil {
		t.Fatalf("ReadFile(live) error = %v", err)
	}
	if got, want := string(body), "new\n"; got != want {
		t.Fatalf("live body = %q, want %q", got, want)
	}
	if _, err := os.Stat(stagePath); !os.IsNotExist(err) {
		t.Fatalf("stage file still exists or stat failed with %v", err)
	}
}

func TestSameRegularFileContent(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", name, err)
		}
		return path
	}
	largeBody := strings.Repeat("x", 32*1024+7)
	symlinkTarget := write("symlink-target", "body\n")
	symlinkPath := filepath.Join(dir, "symlink-right")
	if err := os.Symlink(symlinkTarget, symlinkPath); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}
	cases := []struct {
		name  string
		left  string
		right string
		want  bool
	}{
		{
			name:  "missing right file",
			left:  write("missing-left", "body\n"),
			right: filepath.Join(dir, "missing-right"),
			want:  false,
		},
		{
			name:  "different size",
			left:  write("different-size-left", "short\n"),
			right: write("different-size-right", "longer\n"),
			want:  false,
		},
		{
			name:  "same size different content",
			left:  write("same-size-left", "old\n"),
			right: write("same-size-right", "new\n"),
			want:  false,
		},
		{
			name:  "right side symlink",
			left:  write("symlink-left", "body\n"),
			right: symlinkPath,
			want:  false,
		},
		{
			name:  "identical empty files",
			left:  write("empty-left", ""),
			right: write("empty-right", ""),
			want:  true,
		},
		{
			name:  "identical small files",
			left:  write("small-left", "body\n"),
			right: write("small-right", "body\n"),
			want:  true,
		},
		{
			name:  "identical file across buffer boundary",
			left:  write("large-left", largeBody),
			right: write("large-right", largeBody),
			want:  true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sameRegularFileContent(tc.left, tc.right)
			if err != nil {
				t.Fatalf("sameRegularFileContent() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("sameRegularFileContent() = %v, want %v", got, tc.want)
			}
		})
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

func TestEngineCleanupPublishStagesBeforeKeepsCurrentProcessStages(t *testing.T) {
	cutoff := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	eng := newEngineFixture(t)
	if err := eng.ensureDirectories(); err != nil {
		t.Fatalf("ensureDirectories() error = %v", err)
	}
	oldWebStage := filepath.Join(eng.outputDir(), ".update-ipsets-web-old-process")
	currentWebStage := filepath.Join(eng.outputDir(), ".update-ipsets-web-current-process")
	oldEntityStage := filepath.Join(eng.entitiesDir(), ".update-ipsets-entities-old-process")
	currentEntityStage := filepath.Join(eng.entitiesDir(), ".update-ipsets-entities-current-process")
	for _, dir := range []string{oldWebStage, currentWebStage, oldEntityStage, currentEntityStage} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", dir, err)
		}
	}
	for _, dir := range []string{oldWebStage, oldEntityStage} {
		oldTime := cutoff.Add(-time.Minute)
		if err := os.Chtimes(dir, oldTime, oldTime); err != nil {
			t.Fatalf("Chtimes(%q) error = %v", dir, err)
		}
	}
	for _, dir := range []string{currentWebStage, currentEntityStage} {
		currentTime := cutoff.Add(time.Minute)
		if err := os.Chtimes(dir, currentTime, currentTime); err != nil {
			t.Fatalf("Chtimes(%q) error = %v", dir, err)
		}
	}

	result, err := eng.CleanupPublishStagesBefore(cutoff)
	if err != nil {
		t.Fatalf("CleanupPublishStagesBefore() error = %v", err)
	}
	if got, want := result.WebRemoved, 1; got != want {
		t.Fatalf("web removed = %d, want %d", got, want)
	}
	if got, want := result.EntityRemoved, 1; got != want {
		t.Fatalf("entity removed = %d, want %d", got, want)
	}
	for _, path := range []string{oldWebStage, oldEntityStage} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected %q to be removed, stat error = %v", path, err)
		}
	}
	for _, path := range []string{currentWebStage, currentEntityStage} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %q to remain, stat error = %v", path, err)
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
