package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCheckEntityArtifactsIntegrityFlagsMissingCountryPublicJSON(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }

	seedDetailFixturesForEntityTests(t, eng, webDir, libDir, now)
	if err := eng.RebuildEntityArtifacts(); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(webDir, "countries", "US.json")
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}

	findings, plan, err := eng.CheckEntityArtifactsIntegrity()
	if err != nil {
		t.Fatal(err)
	}
	if plan.full {
		t.Fatal("expected targeted repair plan, got full rebuild")
	}
	if _, ok := plan.countryCodes["US"]; !ok {
		t.Fatalf("expected US country refresh in plan, got %+v", plan)
	}

	var found bool
	for _, finding := range findings {
		if finding.Kind == "detail_public_missing" && finding.Country == "US" {
			found = true
			if finding.RepairAction != "refresh_entity" {
				t.Fatalf("repair action = %q, want refresh_entity", finding.RepairAction)
			}
		}
	}
	if !found {
		t.Fatalf("expected missing country public finding, got %+v", findings)
	}
}

func TestCheckEntityArtifactsIntegrityRepairsMissingHomeAggregate(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }

	seedDetailFixturesForEntityTests(t, eng, webDir, libDir, now)
	if err := eng.RebuildEntityArtifactsWithTrigger(t.Context(), "test"); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(webDir, "home", "aggregates.json")
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}

	findings, plan, err := eng.CheckEntityArtifactsIntegrity()
	if err != nil {
		t.Fatal(err)
	}
	if !plan.rebuildHomeAggregate {
		t.Fatalf("expected homepage aggregate repair plan, got %+v", plan)
	}
	var found bool
	for _, finding := range findings {
		if finding.Kind == "home_aggregates_missing" && finding.Subject == "home" {
			found = true
			if finding.RepairAction != "refresh_home_aggregates" {
				t.Fatalf("repair action = %q, want refresh_home_aggregates", finding.RepairAction)
			}
		}
	}
	if !found {
		t.Fatalf("expected missing homepage aggregate finding, got %+v", findings)
	}

	if err := eng.EnsureEntityArtifactsCurrent(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected homepage aggregate repair to republish artifact: %v", err)
	}
	findings, plan, err = eng.CheckEntityArtifactsIntegrity()
	if err != nil {
		t.Fatal(err)
	}
	if plan.rebuildHomeAggregate {
		t.Fatalf("expected clean homepage aggregate after repair, got plan %+v findings %+v", plan, findings)
	}
}

func TestCheckEntityArtifactsIntegrityFlagsStaleHomeAggregate(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }

	seedDetailFixturesForEntityTests(t, eng, webDir, libDir, now)
	if err := eng.RebuildEntityArtifactsWithTrigger(t.Context(), "test"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(webDir, "home", "aggregates.json")
	older := now.Add(-2 * time.Hour)
	if err := os.Chtimes(target, older, older); err != nil {
		t.Fatal(err)
	}

	findings, plan, err := eng.CheckEntityArtifactsIntegrity()
	if err != nil {
		t.Fatal(err)
	}
	if !plan.rebuildHomeAggregate {
		t.Fatalf("expected homepage aggregate repair plan, got %+v", plan)
	}
	for _, finding := range findings {
		if finding.Kind == "home_aggregates_stale" && finding.Subject == "home" {
			return
		}
	}
	t.Fatalf("expected stale homepage aggregate finding, got %+v", findings)
}

func TestCheckEntityArtifactsIntegrityFlagsMalformedHomeAggregate(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }

	seedDetailFixturesForEntityTests(t, eng, webDir, libDir, now)
	if err := eng.RebuildEntityArtifactsWithTrigger(t.Context(), "test"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(webDir, "home", "aggregates.json")
	if err := os.WriteFile(target, []byte(`{`), 0o600); err != nil {
		t.Fatal(err)
	}

	findings, plan, err := eng.CheckEntityArtifactsIntegrity()
	if err != nil {
		t.Fatal(err)
	}
	if !plan.rebuildHomeAggregate {
		t.Fatalf("expected homepage aggregate repair plan, got %+v", plan)
	}
	for _, finding := range findings {
		if finding.Kind == "home_aggregates_malformed" && finding.Subject == "home" {
			return
		}
	}
	t.Fatalf("expected malformed homepage aggregate finding, got %+v", findings)
}

func TestCheckEntityArtifactsIntegrityFlagsHealthTransitionDrift(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }

	seedDetailFixturesForEntityTests(t, eng, webDir, libDir, now)
	configureDetailFeedState(t, eng, "gamma", "malware_infrastructure", "Gamma Maintainer", "https://gamma.test", now.Add(-30*time.Minute).Unix(), 256)
	if err := eng.RebuildEntityArtifacts(); err != nil {
		t.Fatal(err)
	}

	later := now.Add(3 * time.Hour)
	eng.now = func() time.Time { return later }

	findings, plan, err := eng.CheckEntityArtifactsIntegrity()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := plan.healthFeeds["gamma"]; !ok {
		t.Fatalf("expected gamma health refresh in plan, got %+v", plan)
	}

	var found bool
	for _, finding := range findings {
		if finding.Kind == "detail_health_stale" && finding.Feed == "gamma" {
			found = true
			if finding.RepairAction != "refresh_health" {
				t.Fatalf("repair action = %q, want refresh_health", finding.RepairAction)
			}
			if finding.AffectedCountries == 0 && finding.AffectedASNs == 0 {
				t.Fatalf("expected affected entities in finding, got %+v", finding)
			}
		}
	}
	if !found {
		t.Fatalf("expected health-stale entity finding, got %+v", findings)
	}
}

func TestCheckEntityArtifactsIntegrityAllowsUnchangedActorOlderThanFeedSidecar(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }

	seedDetailFixturesForEntityTests(t, eng, webDir, libDir, now)
	if err := eng.RebuildEntityArtifacts(); err != nil {
		t.Fatal(err)
	}

	feedSidecar := filepath.Join(libDir, "entities", "feeds", "alpha.json")
	newer := now.Add(2 * time.Hour)
	if err := os.Chtimes(feedSidecar, newer, newer); err != nil {
		t.Fatal(err)
	}

	findings, plan, err := eng.CheckEntityArtifactsIntegrity()
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.countryCodes) != 0 || len(plan.asns) != 0 {
		t.Fatalf("expected no country/ASN mtime-only repair plan, got %+v", plan)
	}
	for _, finding := range findings {
		if finding.Kind == "detail_sidecar_stale" {
			t.Fatalf("unexpected mtime-only actor stale finding: %+v", finding)
		}
	}
}

func TestCheckEntityArtifactsIntegrityRequiresFeedSidecarsForEmptyEntityInputs(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }

	for _, name := range []string{"alpha", "beta", "gamma"} {
		configureDetailFeedState(t, eng, name, "intrusion", "Maintainer", "https://example.test", now.Unix(), 0)
		writeLatestSetForDetailTest(t, libDir, name, now, "")
		writeCountryPayloadForDetailTest(t, webDir, name, "dbip_country", nil)
		writeASNPayloadForDetailTest(t, webDir, name, "iptoasn", nil)
	}
	writeDetailGeoProviderForTest(t, filepath.Join(libDir, "geolocation", "dbip_country.source"))
	writeDetailASNProviderForTest(t, filepath.Join(libDir, "asn", "iptoasn", "database.tsv"))

	if err := eng.RebuildEntityArtifacts(); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"alpha", "beta", "gamma"} {
		sidecarPath := filepath.Join(libDir, "entities", "feeds", name+".json")
		var sidecar feedEntitySidecar
		loadJSONForTest(t, sidecarPath, &sidecar)
		if sidecar.Feed != name {
			t.Fatalf("%s sidecar feed = %q, want %q", name, sidecar.Feed, name)
		}
		if len(sidecar.Countries) != 0 || len(sidecar.ASNs) != 0 {
			t.Fatalf("%s sidecar has countries=%v asns=%v, want explicit empty contribution state", name, sidecar.Countries, sidecar.ASNs)
		}
	}

	missingPath := filepath.Join(libDir, "entities", "feeds", "alpha.json")
	if err := os.Remove(missingPath); err != nil {
		t.Fatal(err)
	}

	findings, plan, err := eng.CheckEntityArtifactsIntegrity()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := plan.feedNames["alpha"]; !ok {
		t.Fatalf("expected alpha feed-sidecar repair plan for missing empty sidecar, got %+v", plan)
	}
	var found bool
	for _, finding := range findings {
		if finding.Kind == "feed_sidecar_missing" && finding.Feed == "alpha" {
			found = true
			if finding.RepairAction != "refresh_feed" {
				t.Fatalf("repair action = %q, want refresh_feed", finding.RepairAction)
			}
		}
	}
	if !found {
		t.Fatalf("expected missing alpha feed sidecar finding, got %+v", findings)
	}
}

func TestCheckEntityArtifactsIntegrityAllowsFeedSidecarWithFutureDatedLatestSource(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Now().UTC().Truncate(time.Second)
	eng.now = func() time.Time { return now }

	seedDetailFixturesForEntityTests(t, eng, webDir, libDir, now)
	if err := eng.RebuildEntityArtifacts(); err != nil {
		t.Fatal(err)
	}

	futureSource := time.Now().UTC().Truncate(time.Second).Add(5 * time.Minute)
	latestPath := filepath.Join(libDir, "alpha", "latest")
	if err := os.Chtimes(latestPath, futureSource, futureSource); err != nil {
		t.Fatal(err)
	}
	sidecarPath := filepath.Join(libDir, "entities", "feeds", "alpha.json")
	var sidecar feedEntitySidecar
	loadJSONForTest(t, sidecarPath, &sidecar)
	sidecar.LastChangeTS = futureSource.Unix()
	if err := writeJSONFile(sidecarPath, &sidecar); err != nil {
		t.Fatal(err)
	}
	older := now.Add(-time.Minute)
	if err := os.Chtimes(sidecarPath, older, older); err != nil {
		t.Fatal(err)
	}

	findings, plan, err := eng.CheckEntityArtifactsIntegrity()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := plan.feedNames["alpha"]; ok {
		t.Fatalf("expected no alpha feed-sidecar repair plan for matching source timestamp, got %+v", plan)
	}
	for _, finding := range findings {
		if finding.Kind == "feed_sidecar_stale" && finding.Feed == "alpha" {
			t.Fatalf("unexpected stale feed sidecar finding for future-dated source timestamp: %+v", finding)
		}
	}
}

func TestRebuildEntityArtifactsForFeedsDoesNotDeleteFeedWithoutPendingSidecar(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }

	seedDetailFixturesForEntityTests(t, eng, webDir, libDir, now)
	if err := eng.RebuildEntityArtifacts(); err != nil {
		t.Fatal(err)
	}

	alphaSidecar := filepath.Join(libDir, "entities", "feeds", "alpha.json")
	oldMTime := now.Add(-2 * time.Hour)
	if err := os.Chtimes(alphaSidecar, oldMTime, oldMTime); err != nil {
		t.Fatal(err)
	}
	if err := eng.rebuildEntityArtifactsForFeeds(t.Context(), []string{"alpha"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(alphaSidecar); err != nil {
		t.Fatalf("expected committed alpha feed sidecar to remain: %v", err)
	}
	info, err := os.Stat(alphaSidecar)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().After(oldMTime) {
		t.Fatalf("expected unchanged alpha feed sidecar freshness touch, got %s <= %s", info.ModTime(), oldMTime)
	}

	us := loadCountryDetailPayloadForTest(t, filepath.Join(webDir, "countries", "US.json"))
	names := map[string]struct{}{}
	for _, row := range us.Feeds {
		names[row.Name] = struct{}{}
	}
	if _, ok := names["alpha"]; !ok {
		t.Fatalf("alpha disappeared from US country detail after feed-sidecar repair: %+v", us.Feeds)
	}
}

func TestRebuildEntityArtifactsForFeedsStampsUnchangedSidecarToProviderReference(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Now().UTC().Truncate(time.Second).Add(-2 * time.Hour)
	eng.now = func() time.Time { return now }

	seedDetailFixturesForEntityTests(t, eng, webDir, libDir, now)
	for _, path := range []string{
		filepath.Join(webDir, "alpha_dbip_country.json"),
		filepath.Join(webDir, "alpha_asn_iptoasn.json"),
		filepath.Join(libDir, "geolocation", "dbip_country.source"),
		filepath.Join(libDir, "asn", "iptoasn", "database.tsv"),
	} {
		if err := os.Chtimes(path, now, now); err != nil {
			t.Fatal(err)
		}
	}
	if err := eng.RebuildEntityArtifacts(); err != nil {
		t.Fatal(err)
	}

	later := now.Add(time.Hour)
	alphaASN := filepath.Join(webDir, "alpha_asn_iptoasn.json")
	if err := os.Chtimes(alphaASN, later, later); err != nil {
		t.Fatal(err)
	}
	alphaSidecar := filepath.Join(libDir, "entities", "feeds", "alpha.json")
	oldMTime := now.Add(-time.Hour)
	if err := os.Chtimes(alphaSidecar, oldMTime, oldMTime); err != nil {
		t.Fatal(err)
	}

	if err := eng.rebuildEntityArtifactsForFeeds(t.Context(), []string{"alpha"}, nil); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{
		alphaSidecar,
		filepath.Join(libDir, "entities", "countries", "US.json"),
		filepath.Join(webDir, "countries", "US.json"),
		filepath.Join(libDir, "entities", "asns", "13335.json"),
		filepath.Join(webDir, "asns", "13335.json"),
	} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.ModTime().UTC().Before(later) {
			t.Fatalf("%s mtime = %s, want at least %s", path, info.ModTime().UTC(), later)
		}
	}

	findings, plan, err := eng.CheckEntityArtifactsIntegrity()
	if err != nil {
		t.Fatal(err)
	}
	if plan.hasWork() {
		t.Fatalf("expected no repair plan after provider-reference stamp, got %+v with findings %+v", plan, findings)
	}
	for _, finding := range findings {
		if finding.Feed == "alpha" || finding.Country == "US" || finding.ASN == 13335 {
			t.Fatalf("unexpected stale finding after provider-reference stamp: %+v", finding)
		}
	}
}
