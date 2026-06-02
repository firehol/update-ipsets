package scheduler

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/internal/fileutil"
)

func TestSaveSnapshotWritesGeneratedFileMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scheduler-state.json")
	snapshot := Snapshot{
		GeneratedAt: time.Unix(1700000000, 0).UTC(),
		Items: []Item{{
			Name:             "sample",
			Kind:             "feed",
			Enabled:          true,
			FrequencyMinutes: 60,
		}},
	}

	if err := SaveSnapshot(path, snapshot); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}
	if got := info.Mode().Perm(); got != fileutil.GeneratedFileMode {
		t.Fatalf("scheduler snapshot mode = %04o, want %04o", got, fileutil.GeneratedFileMode)
	}
}
