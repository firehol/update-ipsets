package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSurgicalRefreshRebuildsAffectedDetailsFromFeedSidecars(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }
	seedDetailFixturesForEntityTests(t, eng, webDir, libDir, now)

	if err := eng.RebuildEntityArtifactsWithTrigger(t.Context(), "test"); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(libDir, "entities", "countries", "US.json"), []byte(`{"code":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(libDir, "entities", "asns", "13335.json"), []byte(`{"asn":`), 0o600); err != nil {
		t.Fatal(err)
	}

	alpha, err := eng.loadCommittedFeedEntitySidecar("alpha")
	if err != nil {
		t.Fatal(err)
	}
	alpha.UniqueIPs = 640
	alpha.LastChangeTS = now.Unix()
	for i := range alpha.Countries {
		if alpha.Countries[i].Code == "US" {
			alpha.Countries[i].AttributedIPs = 384
			for j := range alpha.Countries[i].ASNs {
				if alpha.Countries[i].ASNs[j].ASN == 13335 {
					alpha.Countries[i].ASNs[j].Count = 384
				}
			}
		}
	}
	for i := range alpha.ASNs {
		if alpha.ASNs[i].ASN == 13335 {
			alpha.ASNs[i].AttributedIPs = 384
		}
	}
	writePendingFeedEntitySidecarForDetailTest(t, libDir, *alpha)

	if err := eng.refreshEntityArtifactsForFeedUpdates(t.Context(), []string{"alpha"}, nil); err != nil {
		t.Fatalf("refreshEntityArtifactsForFeedUpdates() error = %v", err)
	}
	if got := lifetimeCounterCount(t, eng, "entity.refresh.country_sidecar_read"); got != 0 {
		t.Fatalf("country actor sidecar reads = %d, want 0", got)
	}
	if got := lifetimeCounterCount(t, eng, "entity.refresh.asn_sidecar_read"); got != 0 {
		t.Fatalf("ASN actor sidecar reads = %d, want 0", got)
	}

	country := loadCountryDetailPayloadForTest(t, filepath.Join(webDir, "countries", "US.json"))
	if got, want := attributedIPsForCountryFeed(country, "alpha"), uint64(384); got != want {
		t.Fatalf("US alpha attributed IPs = %d, want %d", got, want)
	}
	if got, want := attributedIPsForCountryFeed(country, "gamma"), uint64(256); got != want {
		t.Fatalf("US gamma attributed IPs = %d, want %d", got, want)
	}

	asn := loadASNDetailPayloadForTest(t, filepath.Join(webDir, "asns", "13335.json"))
	if got, want := attributedIPsForASNFeed(asn, "alpha"), uint64(384); got != want {
		t.Fatalf("ASN 13335 alpha attributed IPs = %d, want %d", got, want)
	}
	if got, want := attributedIPsForASNFeed(asn, "gamma"), uint64(256); got != want {
		t.Fatalf("ASN 13335 gamma attributed IPs = %d, want %d", got, want)
	}
}

func attributedIPsForCountryFeed(payload CountryDetailPayload, name string) uint64 {
	for _, row := range payload.Feeds {
		if row.Name == name {
			return row.AttributedIPs
		}
	}
	return 0
}

func assertFileMTimeForTest(t *testing.T, path string, want time.Time) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", path, err)
	}
	if got := info.ModTime().UTC(); !got.Equal(want.UTC()) {
		t.Fatalf("%s mtime = %s, want %s", path, got, want.UTC())
	}
}

func attributedIPsForASNFeed(payload ASNDetailPayload, name string) uint64 {
	for _, row := range payload.Feeds {
		if row.Name == name {
			return row.AttributedIPs
		}
	}
	return 0
}
