package engine

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRebuildEntityArtifactsWritesFeedPresenceIndex(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }
	seedDetailFixturesForEntityTests(t, eng, webDir, libDir, now)

	if err := eng.RebuildEntityArtifactsWithTrigger(t.Context(), "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(libDir, "entities", "feed-presence-v1.bin")); err != nil {
		t.Fatalf("feed presence index missing after rebuild: %v", err)
	}

	presence := newEntityArtifactFeedPresence(eng)
	ok, err := presence.contains("alpha")
	if err != nil {
		t.Fatalf("presence contains alpha: %v", err)
	}
	if !ok {
		t.Fatal("presence index did not report alpha")
	}
	if got := lifetimeCounterCount(t, eng, "entity.repair_feed_scan.country_sidecar_read"); got != 0 {
		t.Fatalf("presence lookup scanned country sidecars = %d, want 0", got)
	}
	if got := lifetimeCounterCount(t, eng, "entity.repair_feed_scan.asn_sidecar_read"); got != 0 {
		t.Fatalf("presence lookup scanned ASN sidecars = %d, want 0", got)
	}
}

func TestMissingCommittedFeedSidecarUsesPresenceIndexForFullRebuildProof(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }
	seedDetailFixturesForEntityTests(t, eng, webDir, libDir, now)

	if err := eng.RebuildEntityArtifactsWithTrigger(t.Context(), "test"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(libDir, "entities", "feeds", "gamma.json")); err != nil {
		t.Fatal(err)
	}

	_, err := eng.buildFeedEntityDeltaWithPresence("gamma", newEntityArtifactFeedPresence(eng))
	if !errors.Is(err, errEntitySurgicalNeedsFullRebuild) {
		t.Fatalf("missing committed gamma sidecar error = %v, want full rebuild fallback", err)
	}
	if got := lifetimeCounterCount(t, eng, "entity.repair_feed_scan.country_sidecar_read"); got != 0 {
		t.Fatalf("full-rebuild proof scanned country sidecars = %d, want 0", got)
	}
	if got := lifetimeCounterCount(t, eng, "entity.repair_feed_scan.asn_sidecar_read"); got != 0 {
		t.Fatalf("full-rebuild proof scanned ASN sidecars = %d, want 0", got)
	}
}
