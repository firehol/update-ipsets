package engine

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestProviderOnlyRunDefersEntitySidecarStagingWhileFullRebuildActive(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }

	seedDetailFixturesForEntityTests(t, eng, webDir, libDir, now)
	task := eng.beginBackgroundTask("Entity artifacts rebuild", "startup", "planning", "building full country and ASN entity artifacts", 0, 0)
	defer task.Finish()

	report, err := eng.RunOnce(t.Context(), RunOptions{
		Selected:  []string{"dbip_country"},
		Reprocess: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(report.EntityRefreshTargets, []string{"alpha", "beta", "gamma"}) {
		t.Fatalf("entity refresh targets = %v, want alpha beta gamma", report.EntityRefreshTargets)
	}
	if _, err := os.Stat(filepath.Join(libDir, "entities", "feeds-pending", "alpha.json")); !os.IsNotExist(err) {
		t.Fatalf("pending alpha feed sidecar should be deferred while full rebuild is active, got err=%v", err)
	}
}

func TestProviderOnlyRunDefersEntitySidecarStagingWhileFullRebuildQueued(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }

	seedDetailFixturesForEntityTests(t, eng, webDir, libDir, now)
	if !eng.tryMarkEntityArtifactFullRebuildQueued() {
		t.Fatal("tryMarkEntityArtifactFullRebuildQueued() returned false")
	}
	defer eng.clearEntityArtifactFullRebuildQueued()

	report, err := eng.RunOnce(t.Context(), RunOptions{
		Selected:  []string{"dbip_country"},
		Reprocess: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(report.EntityRefreshTargets, []string{"alpha", "beta", "gamma"}) {
		t.Fatalf("entity refresh targets = %v, want alpha beta gamma", report.EntityRefreshTargets)
	}
	if _, err := os.Stat(filepath.Join(libDir, "entities", "feeds-pending", "alpha.json")); !os.IsNotExist(err) {
		t.Fatalf("pending alpha feed sidecar should be deferred while full rebuild is queued, got err=%v", err)
	}
}
