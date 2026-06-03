package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHealthTransitionRefreshRewritesPublishedEntityDetails(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }

	seedDetailFixturesForEntityTests(t, eng, webDir, libDir, now)
	if err := eng.RebuildEntityArtifacts(); err != nil {
		t.Fatal(err)
	}

	countryPath := filepath.Join(webDir, "countries", "US.json")
	asnPath := filepath.Join(webDir, "asns", "13335.json")
	if err := os.WriteFile(countryPath, []byte(`{"code":"US","feeds":[]}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(asnPath, []byte(`{"asn":13335,"feeds":[]}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := eng.refreshEntityArtifactsForHealthTransitions(t.Context(), []string{"alpha"}, nil); err != nil {
		t.Fatal(err)
	}

	country := loadCountryDetailPayloadForTest(t, countryPath)
	foundAlphaCountry := false
	for _, row := range country.Feeds {
		if row.Name == "alpha" {
			foundAlphaCountry = true
			if row.HealthClass == "" {
				t.Fatalf("alpha country row missing health class: %+v", row)
			}
		}
	}
	if !foundAlphaCountry {
		t.Fatalf("alpha was not restored in US country detail: %+v", country.Feeds)
	}

	asn := loadASNDetailPayloadForTest(t, asnPath)
	foundAlphaASN := false
	for _, row := range asn.Feeds {
		if row.Name == "alpha" {
			foundAlphaASN = true
			if row.HealthClass == "" {
				t.Fatalf("alpha ASN row missing health class: %+v", row)
			}
		}
	}
	if !foundAlphaASN {
		t.Fatalf("alpha was not restored in AS13335 detail: %+v", asn.Feeds)
	}

	publicCountryInfo, err := os.Stat(countryPath)
	if err != nil {
		t.Fatal(err)
	}
	privateCountryInfo, err := os.Stat(filepath.Join(libDir, "entities", "countries", "US.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !publicCountryInfo.ModTime().UTC().Equal(privateCountryInfo.ModTime().UTC()) {
		t.Fatalf("country detail mtime = %s, want private sidecar mtime %s", publicCountryInfo.ModTime(), privateCountryInfo.ModTime())
	}

	publicASNInfo, err := os.Stat(asnPath)
	if err != nil {
		t.Fatal(err)
	}
	privateASNInfo, err := os.Stat(filepath.Join(libDir, "entities", "asns", "13335.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !publicASNInfo.ModTime().UTC().Equal(privateASNInfo.ModTime().UTC()) {
		t.Fatalf("ASN detail mtime = %s, want private sidecar mtime %s", publicASNInfo.ModTime(), privateASNInfo.ModTime())
	}
}
