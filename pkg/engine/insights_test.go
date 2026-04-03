package engine

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestInsightTargetNamesScopesToAffectedFeedsAndMissingFiles(t *testing.T) {
	cfg := newTestConfigForFanOut()
	stageDir := t.TempDir()
	liveDir := t.TempDir()
	outputNames := []string{"feed_a", "feed_b"}

	writeTestInsightsFile(t, liveDir, "feed_b")
	got := insightTargetNames(cfg, []string{"feed_a"}, outputNames, stageDir, liveDir)
	if want := []string{"feed_a"}; !slices.Equal(got, want) {
		t.Fatalf("affected feed target mismatch: got %#v want %#v", got, want)
	}

	if err := os.Remove(filepath.Join(liveDir, "feed_b_insights.json")); err != nil {
		t.Fatal(err)
	}
	got = insightTargetNames(cfg, []string{"feed_a"}, outputNames, stageDir, liveDir)
	if want := []string{"feed_a", "feed_b"}; !slices.Equal(got, want) {
		t.Fatalf("missing insights target mismatch: got %#v want %#v", got, want)
	}

	writeTestInsightsFile(t, liveDir, "feed_a")
	writeTestInsightsFile(t, liveDir, "feed_b")
	got = insightTargetNames(cfg, []string{"maxmind_geolite2"}, outputNames, stageDir, liveDir)
	if want := []string{"feed_a", "feed_b"}; !slices.Equal(got, want) {
		t.Fatalf("provider update target mismatch: got %#v want %#v", got, want)
	}
}

func writeTestInsightsFile(t *testing.T, dir string, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name+"_insights.json"), []byte(`{"items":[]}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
