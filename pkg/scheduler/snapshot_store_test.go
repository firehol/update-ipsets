package scheduler

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStoreSnapshotKeepsCachedSnapshotReadableWhilePersisting(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	oldSave := saveSchedulerSnapshot
	saveSchedulerSnapshot = func(path string, snapshot Snapshot) error {
		close(started)
		<-release
		return nil
	}
	t.Cleanup(func() {
		saveSchedulerSnapshot = oldSave
	})

	now := time.Unix(1_700_000_000, 0).UTC()
	runner := &Runner{statePath: filepath.Join(t.TempDir(), "scheduler.json")}
	snapshot := Snapshot{
		GeneratedAt: now,
		Items: []Item{{
			Name:    "sample",
			Enabled: true,
		}},
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		runner.storeSnapshot(snapshot)
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("snapshot persistence did not start")
	}

	readDone := make(chan Snapshot, 1)
	go func() {
		readDone <- runner.CachedSnapshot()
	}()
	select {
	case cached := <-readDone:
		if !cached.GeneratedAt.Equal(now) || len(cached.Items) != 1 || cached.Items[0].Name != "sample" {
			t.Fatalf("cached snapshot while persistence blocked = %#v", cached)
		}
	case <-time.After(200 * time.Millisecond):
		close(release)
		t.Fatal("CachedSnapshot blocked behind snapshot persistence")
	}

	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("storeSnapshot did not finish after persistence was released")
	}
}
