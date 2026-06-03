package engine

import (
	"os"
	"path/filepath"
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
