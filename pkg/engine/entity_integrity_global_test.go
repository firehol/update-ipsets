package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCheckEntityArtifactsIntegrityFlagsMissingVersionMarker(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }

	seedDetailFixturesForEntityTests(t, eng, webDir, libDir, now)
	if err := eng.RebuildEntityArtifacts(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(eng.entityVersionPath()); err != nil {
		t.Fatal(err)
	}

	findings, plan, err := eng.CheckEntityArtifactsIntegrity()
	if err != nil {
		t.Fatal(err)
	}
	if !plan.full {
		t.Fatalf("expected full rebuild plan, got %+v", plan)
	}
	finding, ok := findEntityIntegrityFinding(findings, "global", "version_missing", "entity_artifacts")
	if !ok {
		t.Fatalf("expected missing version finding, got %+v", findings)
	}
	if finding.RepairAction != "full_rebuild" {
		t.Fatalf("repair action = %q, want full_rebuild", finding.RepairAction)
	}
}

func TestCheckEntityArtifactsIntegrityFlagsMismatchedVersionMarker(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }

	seedDetailFixturesForEntityTests(t, eng, webDir, libDir, now)
	if err := eng.RebuildEntityArtifacts(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(eng.entityVersionPath(), []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	findings, plan, err := eng.CheckEntityArtifactsIntegrity()
	if err != nil {
		t.Fatal(err)
	}
	if !plan.full {
		t.Fatalf("expected full rebuild plan, got %+v", plan)
	}
	finding, ok := findEntityIntegrityFinding(findings, "global", "version_mismatch", "entity_artifacts")
	if !ok {
		t.Fatalf("expected mismatched version finding, got %+v", findings)
	}
	if finding.RepairAction != "full_rebuild" {
		t.Fatalf("repair action = %q, want full_rebuild", finding.RepairAction)
	}
}

func TestCheckEntityArtifactsIntegrityRequiresFeedPresenceIndex(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }

	seedDetailFixturesForEntityTests(t, eng, webDir, libDir, now)
	if err := eng.RebuildEntityArtifacts(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(eng.entityFeedPresenceIndexPath()); err != nil {
		t.Fatal(err)
	}

	findings, plan, err := eng.CheckEntityArtifactsIntegrity()
	if err != nil {
		t.Fatal(err)
	}
	if !plan.full {
		t.Fatalf("expected full rebuild plan, got %+v", plan)
	}
	finding, ok := findEntityIntegrityFinding(findings, "global", "feed_presence_index_missing", "entity_artifacts")
	if !ok {
		t.Fatalf("expected missing feed presence index finding, got %+v", findings)
	}
	if finding.RepairAction != "full_rebuild" {
		t.Fatalf("repair action = %q, want full_rebuild", finding.RepairAction)
	}
}

func TestRefreshEntityArtifactsRebuildsPreviousVersionPartialFeedSidecarStore(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }

	seedDetailFixturesForEntityTests(t, eng, webDir, libDir, now)
	if err := eng.RebuildEntityArtifacts(); err != nil {
		t.Fatal(err)
	}
	writePreviousVersionPartialFeedSidecarStoreForTest(t, eng, []string{"alpha"})

	findings, plan, err := eng.CheckEntityArtifactsIntegrity()
	if err != nil {
		t.Fatal(err)
	}
	if !plan.full {
		t.Fatalf("expected old partial store to require full rebuild, got plan %+v findings %+v", plan, findings)
	}
	if _, ok := findEntityIntegrityFinding(findings, "global", "version_mismatch", "entity_artifacts"); !ok {
		t.Fatalf("expected version mismatch for old partial store, got %+v", findings)
	}

	if err := eng.RefreshEntityArtifactsForFeedUpdates(t.Context(), []string{"alpha"}, "test"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"alpha", "beta", "gamma"} {
		if _, err := os.Stat(filepath.Join(libDir, "entities", "feeds", name+".json")); err != nil {
			t.Fatalf("expected full rebuild to restore %s feed sidecar: %v", name, err)
		}
	}
	findings, plan, err = eng.CheckEntityArtifactsIntegrity()
	if err != nil {
		t.Fatal(err)
	}
	if plan.hasWork() || len(findings) > 0 {
		t.Fatalf("expected clean integrity after full rebuild, got plan %+v findings %+v", plan, findings)
	}
}

func TestCheckEntityArtifactsIntegrityFlagsMissingASNPublicJSON(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }

	seedDetailFixturesForEntityTests(t, eng, webDir, libDir, now)
	if err := eng.RebuildEntityArtifacts(); err != nil {
		t.Fatal(err)
	}
	target := eng.PublicASNDetailPath(13335)
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}

	findings, plan, err := eng.CheckEntityArtifactsIntegrity()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := plan.asns[13335]; !ok {
		t.Fatalf("expected ASN 13335 refresh in plan, got %+v", plan)
	}
	finding, ok := findEntityIntegrityFinding(findings, "asn", "detail_public_missing", "13335")
	if !ok {
		t.Fatalf("expected missing ASN public finding, got %+v", findings)
	}
	if finding.RepairAction != "refresh_entity" {
		t.Fatalf("repair action = %q, want refresh_entity", finding.RepairAction)
	}
}

func TestCheckEntityArtifactsIntegrityFlagsMalformedASNPrivateSidecar(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }

	seedDetailFixturesForEntityTests(t, eng, webDir, libDir, now)
	if err := eng.RebuildEntityArtifacts(); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(eng.entityASNsDir(), "13335.json")
	if err := os.WriteFile(target, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}

	findings, plan, err := eng.CheckEntityArtifactsIntegrity()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := plan.asns[13335]; !ok {
		t.Fatalf("expected ASN 13335 refresh in plan, got %+v", plan)
	}
	finding, ok := findEntityIntegrityFinding(findings, "asn", "detail_sidecar_malformed", "13335")
	if !ok {
		t.Fatalf("expected malformed ASN sidecar finding, got %+v", findings)
	}
	if finding.RepairAction != "refresh_entity" {
		t.Fatalf("repair action = %q, want refresh_entity", finding.RepairAction)
	}
}

func findEntityIntegrityFinding(findings []EntityIntegrityFinding, scope, kind, subject string) (EntityIntegrityFinding, bool) {
	for _, finding := range findings {
		if finding.Scope == scope && finding.Kind == kind && finding.Subject == subject {
			return finding, true
		}
	}
	return EntityIntegrityFinding{}, false
}

func writePreviousVersionPartialFeedSidecarStoreForTest(t *testing.T, eng *Engine, keep []string) {
	t.Helper()
	keepSet := stringExactSet(keep)
	feedDir := eng.entityFeedsDir()
	entries, err := os.ReadDir(feedDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		if _, ok := keepSet[name]; ok {
			continue
		}
		if err := os.Remove(filepath.Join(feedDir, entry.Name())); err != nil {
			t.Fatal(err)
		}
	}
	data, err := marshalEntityFeedPresenceIndex(keep)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeFileAtomic(eng.entityFeedPresenceIndexPath(), data, generatedFileMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(eng.entityVersionPath(), []byte("3\n"), generatedFileMode); err != nil {
		t.Fatal(err)
	}
}
