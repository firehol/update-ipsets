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
		data, err := os.ReadFile(filepath.Join(liveDir, rel))
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", rel, err)
		}
		if string(data) != body {
			t.Fatalf("body for %q = %q, want %q", rel, string(data), body)
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
