package engine

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/iprange"
)

func TestCountryDetailIncludesOlderFeedsAndComputesTruthfulTopASNs(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }

	seedDetailFixturesForEntityTests(t, eng, webDir, libDir, now)

	payload, err := eng.CountryDetail("us")
	if err != nil {
		t.Fatal(err)
	}

	if payload.Totals.FeedsMatching != 2 {
		t.Fatalf("feeds matching = %d, want 2", payload.Totals.FeedsMatching)
	}
	if payload.Totals.Categories != 2 {
		t.Fatalf("categories = %d, want 2", payload.Totals.Categories)
	}
	if payload.Totals.Maintainers != 2 {
		t.Fatalf("maintainers = %d, want 2", payload.Totals.Maintainers)
	}
	if payload.Totals.ASNs != 1 {
		t.Fatalf("asns = %d, want 1", payload.Totals.ASNs)
	}
	if payload.Totals.AttributedIPsInFeed != 512 {
		t.Fatalf("attributed_ips_in_feeds = %d, want 512", payload.Totals.AttributedIPsInFeed)
	}

	names := make([]string, 0, len(payload.Feeds))
	healthByFeed := make(map[string]string, len(payload.Feeds))
	for _, row := range payload.Feeds {
		names = append(names, row.Name)
		healthByFeed[row.Name] = row.HealthClass
	}
	slices.Sort(names)
	if want := []string{"alpha", "gamma"}; !slices.Equal(names, want) {
		t.Fatalf("feeds = %v, want %v", names, want)
	}
	if healthByFeed["gamma"] != "unmaintained" {
		t.Fatalf("gamma health = %q, want unmaintained", healthByFeed["gamma"])
	}
	if len(payload.FeedsByCategory["intrusion"]) != 1 || len(payload.FeedsByCategory["malware_infrastructure"]) != 1 {
		t.Fatalf("unexpected grouped feeds: %+v", payload.FeedsByCategory)
	}
	if len(payload.TopASNs) != 1 {
		t.Fatalf("top_asns len = %d, want 1: %+v", len(payload.TopASNs), payload.TopASNs)
	}
	if payload.TopASNs[0].ASN != 13335 || payload.TopASNs[0].AttributedIPs != 512 || payload.TopASNs[0].FeedCount != 2 {
		t.Fatalf("unexpected top ASN row: %+v", payload.TopASNs[0])
	}
	if payload.ASNProvider.Name != "iptoasn" {
		t.Fatalf("asn provider = %q, want iptoasn", payload.ASNProvider.Name)
	}
}

func TestASNDetailBuildsTruthfulCountryDistribution(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }

	seedDetailFixturesForEntityTests(t, eng, webDir, libDir, now)

	payload, err := eng.ASNDetail(15169)
	if err != nil {
		t.Fatal(err)
	}

	if payload.Totals.FeedsMatching != 2 {
		t.Fatalf("feeds matching = %d, want 2", payload.Totals.FeedsMatching)
	}
	if payload.Totals.Categories != 1 {
		t.Fatalf("categories = %d, want 1", payload.Totals.Categories)
	}
	if payload.Totals.Maintainers != 2 {
		t.Fatalf("maintainers = %d, want 2", payload.Totals.Maintainers)
	}
	if payload.Totals.Countries != 2 {
		t.Fatalf("countries = %d, want 2", payload.Totals.Countries)
	}
	if payload.GeoProvider.Name != "dbip_country" {
		t.Fatalf("geo provider = %q, want dbip_country", payload.GeoProvider.Name)
	}
	if payload.CountryDistribution == nil {
		t.Fatal("expected country distribution payload")
	}
	if payload.CountryDistribution.TotalMapped != 768 {
		t.Fatalf("total_mapped = %d, want 768", payload.CountryDistribution.TotalMapped)
	}

	dist := make(map[string]uint64, len(payload.CountryDistribution.Countries))
	for _, row := range payload.CountryDistribution.Countries {
		dist[row.Code] = row.Value
	}
	if len(dist) != 2 || dist["DE"] != 512 || dist["FR"] != 256 {
		t.Fatalf("unexpected country distribution: %+v", payload.CountryDistribution.Countries)
	}
	if _, ok := dist["US"]; ok {
		t.Fatalf("unexpected US in ASN country distribution: %+v", payload.CountryDistribution.Countries)
	}
	if len(payload.TopCountries) != 2 {
		t.Fatalf("top_countries len = %d, want 2", len(payload.TopCountries))
	}
	if payload.TopCountries[0].Code != "DE" || payload.TopCountries[0].AttributedIPs != 512 || payload.TopCountries[0].FeedCount != 2 {
		t.Fatalf("unexpected top country row: %+v", payload.TopCountries[0])
	}
}

func TestMaintainerDetailMissingUsesSentinel(t *testing.T) {
	eng, _, _ := newDetailEngineForTest(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }
	configureDetailFeedState(t, eng, "alpha", "intrusion", "Alpha Maintainer", "https://alpha.test", now.Unix(), 256)

	_, err := eng.MaintainerDetail("missing-maintainer")
	if !errors.Is(err, ErrMaintainerNotFound) {
		t.Fatalf("error = %v, want ErrMaintainerNotFound", err)
	}
}

func TestMaterializeASNDetailUsesBatchHealthClassifierSnapshot(t *testing.T) {
	eng, _, _ := newDetailEngineForTest(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }
	configureDetailFeedState(t, eng, "alpha", "intrusion", "Alpha Maintainer", "https://alpha.test", now.Unix(), 256)

	health := eng.newFeedHealthClassifier()

	alpha := eng.state.Entry("alpha")
	alpha.SourceDate = now.Add(-48 * time.Hour).Unix()
	alpha.ProcessedDate = alpha.SourceDate

	sidecar := &asnDetailSidecar{
		ASN: 13335,
		Provider: HomeSummaryProvider{
			Name: "iptoasn",
		},
		Feeds: []asnDetailFeedBase{
			{
				Name:          "alpha",
				Category:      "intrusion",
				AttributedIPs: 256,
				UniqueIPs:     256,
				LastChangeTS:  now.Unix(),
			},
		},
	}

	batchPayload := eng.materializeASNDetailWithHealth(sidecar, health)
	if got := batchPayload.Feeds[0].HealthClass; got != "healthy" {
		t.Fatalf("batch snapshot health = %q, want healthy", got)
	}

	freshPayload := eng.materializeASNDetail(sidecar)
	if got := freshPayload.Feeds[0].HealthClass; got != "unmaintained" {
		t.Fatalf("fresh health = %q, want unmaintained", got)
	}
}

func TestCountryIndexIncludesAllPublicContributors(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }

	seedDetailFixturesForEntityTests(t, eng, webDir, libDir, now)

	payload, err := eng.CountryIndex()
	if err != nil {
		t.Fatal(err)
	}
	if payload.Provider.Name != "dbip_country" {
		t.Fatalf("provider = %q, want dbip_country", payload.Provider.Name)
	}
	if len(payload.Countries) != 3 {
		t.Fatalf("countries len = %d, want 3", len(payload.Countries))
	}

	got := map[string]CountryIndexEntry{}
	for _, row := range payload.Countries {
		got[row.Code] = row
	}
	if got["DE"].FeedCount != 2 || got["DE"].AttributedIPs != 512 {
		t.Fatalf("DE row = %+v, want feed_count=2 attributed_ips=512", got["DE"])
	}
	if got["US"].FeedCount != 2 || got["US"].AttributedIPs != 512 {
		t.Fatalf("US row = %+v, want feed_count=2 attributed_ips=512", got["US"])
	}
	if got["FR"].FeedCount != 1 || got["FR"].AttributedIPs != 256 {
		t.Fatalf("FR row = %+v, want feed_count=1 attributed_ips=256", got["FR"])
	}
}

func TestASNIndexIncludesAllPublicContributors(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }

	seedDetailFixturesForEntityTests(t, eng, webDir, libDir, now)

	payload, err := eng.ASNIndex()
	if err != nil {
		t.Fatal(err)
	}
	if payload.Provider.Name != "iptoasn" {
		t.Fatalf("provider = %q, want iptoasn", payload.Provider.Name)
	}
	if len(payload.ASNs) != 2 {
		t.Fatalf("asns len = %d, want 2", len(payload.ASNs))
	}

	got := map[uint32]ASNIndexEntry{}
	for _, row := range payload.ASNs {
		got[row.ASN] = row
	}
	if got[15169].FeedCount != 2 || got[15169].AttributedIPs != 768 {
		t.Fatalf("AS15169 row = %+v, want feed_count=2 attributed_ips=768", got[15169])
	}
	if got[13335].FeedCount != 2 || got[13335].AttributedIPs != 512 {
		t.Fatalf("AS13335 row = %+v, want feed_count=2 attributed_ips=512", got[13335])
	}
}

func TestRebuildEntityArtifactsWritesPrecomputedCountryAndASNPayloads(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }

	seedDetailFixturesForEntityTests(t, eng, webDir, libDir, now)

	if err := eng.RebuildEntityArtifacts(); err != nil {
		t.Fatal(err)
	}

	required := []string{
		filepath.Join(webDir, "countries", "index.json"),
		filepath.Join(webDir, "countries", "US.json"),
		filepath.Join(webDir, "asns", "index.json"),
		filepath.Join(webDir, "asns", "13335.json"),
		filepath.Join(libDir, "entities", "feeds", "alpha.json"),
		filepath.Join(libDir, "entities", "countries", "US.json"),
		filepath.Join(libDir, "entities", "asns", "13335.json"),
		filepath.Join(libDir, "entities", "version"),
	}
	for _, path := range required {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected artifact %s: %v", path, err)
		}
	}

	indexData, err := os.ReadFile(filepath.Join(webDir, "countries", "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	var countryIndex CountryIndexPayload
	if err := json.Unmarshal(indexData, &countryIndex); err != nil {
		t.Fatalf("unmarshal country index: %v", err)
	}
	if len(countryIndex.Countries) != 3 {
		t.Fatalf("country index countries = %d, want 3", len(countryIndex.Countries))
	}

	detailData, err := os.ReadFile(filepath.Join(webDir, "asns", "13335.json"))
	if err != nil {
		t.Fatal(err)
	}
	var asnDetail ASNDetailPayload
	if err := json.Unmarshal(detailData, &asnDetail); err != nil {
		t.Fatalf("unmarshal ASN detail: %v", err)
	}
	if asnDetail.ASN != 13335 {
		t.Fatalf("ASN detail ASN = %d, want 13335", asnDetail.ASN)
	}
	if asnDetail.Totals.FeedsMatching != 2 {
		t.Fatalf("ASN detail feeds = %d, want 2", asnDetail.Totals.FeedsMatching)
	}

	var sidecar feedEntitySidecar
	loadJSONForTest(t, filepath.Join(libDir, "entities", "feeds", "alpha.json"), &sidecar)
	if sidecar.Feed != "alpha" {
		t.Fatalf("sidecar feed = %q, want alpha", sidecar.Feed)
	}
	if len(sidecar.Countries) != 2 {
		t.Fatalf("sidecar countries len = %d, want 2", len(sidecar.Countries))
	}
	if len(sidecar.ASNs) != 2 {
		t.Fatalf("sidecar ASNs len = %d, want 2", len(sidecar.ASNs))
	}
	gotJoint := map[string]map[uint32]uint64{}
	for _, country := range sidecar.Countries {
		if gotJoint[country.Code] == nil {
			gotJoint[country.Code] = map[uint32]uint64{}
		}
		for _, row := range country.ASNs {
			gotJoint[country.Code][row.ASN] = row.Count
		}
	}
	if gotJoint["US"][13335] != 256 {
		t.Fatalf("US/AS13335 joint count = %d, want 256", gotJoint["US"][13335])
	}
	if gotJoint["DE"][15169] != 256 {
		t.Fatalf("DE/AS15169 joint count = %d, want 256", gotJoint["DE"][15169])
	}
}

func TestRefreshEntityArtifactsForFeedUpdatesSurgicallyPatchesAggregates(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }

	seedDetailFixturesForEntityTests(t, eng, webDir, libDir, now)
	if err := eng.RebuildEntityArtifacts(); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(libDir, "geolocation")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(libDir, "asn")); err != nil {
		t.Fatal(err)
	}

	configureDetailFeedState(t, eng, "alpha", "intrusion", "Alpha Maintainer", "https://alpha.test", now.Add(5*time.Minute).Unix(), 256)
	writeLatestSetForDetailTest(t, libDir, "alpha", now.Add(5*time.Minute), "9.9.9.0/24\n")
	writeCountryPayloadForDetailTest(t, webDir, "alpha", "dbip_country", []CountryValue{
		{Code: "FR", Value: 256},
	})
	writeASNPayloadForDetailTest(t, webDir, "alpha", "iptoasn", map[uint32]asnEntryJSON{
		15169: {ASN: 15169, Name: "GOOGLE", Count: 256},
	})
	writePendingFeedEntitySidecarForDetailTest(t, libDir, feedEntitySidecar{
		Feed:         "alpha",
		Category:     "intrusion",
		Provenance:   string(publicProvenance(eng.lookupSource("alpha"))),
		Maintainer:   "Alpha Maintainer",
		UniqueIPs:    256,
		LastChangeTS: now.Add(5 * time.Minute).Unix(),
		GeoProvider:  "dbip_country",
		ASNProvider:  "iptoasn",
		Countries: []feedEntityCountryContribution{
			{
				Code:          "FR",
				AttributedIPs: 256,
				ASNs: []feedEntityJointASN{
					{ASN: 15169, Name: "GOOGLE", Count: 256},
				},
			},
		},
		ASNs: []feedEntityASNContribution{
			{ASN: 15169, Name: "GOOGLE", AttributedIPs: 256},
		},
	})

	if err := eng.refreshEntityArtifactsForFeedUpdates(t.Context(), []string{"alpha"}, nil); err != nil {
		t.Fatal(err)
	}

	us := loadCountryDetailPayloadForTest(t, filepath.Join(webDir, "countries", "US.json"))
	if us.Totals.FeedsMatching != 1 || us.Totals.AttributedIPsInFeed != 256 {
		t.Fatalf("US totals = %+v, want one remaining gamma contribution", us.Totals)
	}
	if len(us.Feeds) != 1 || us.Feeds[0].Name != "gamma" {
		t.Fatalf("US feeds = %+v, want gamma only", us.Feeds)
	}
	if len(us.TopASNs) != 1 || us.TopASNs[0].ASN != 13335 || us.TopASNs[0].FeedCount != 1 || us.TopASNs[0].AttributedIPs != 256 {
		t.Fatalf("US top ASNs = %+v, want AS13335 feed_count=1 attributed=256", us.TopASNs)
	}

	fr := loadCountryDetailPayloadForTest(t, filepath.Join(webDir, "countries", "FR.json"))
	if fr.Totals.FeedsMatching != 2 || fr.Totals.AttributedIPsInFeed != 512 {
		t.Fatalf("FR totals = %+v, want alpha+beta", fr.Totals)
	}
	if len(fr.TopMaintainers) != 2 {
		t.Fatalf("FR maintainers = %+v, want alpha and beta", fr.TopMaintainers)
	}
	if len(fr.TopASNs) != 1 || fr.TopASNs[0].ASN != 15169 || fr.TopASNs[0].FeedCount != 2 || fr.TopASNs[0].AttributedIPs != 512 {
		t.Fatalf("FR top ASNs = %+v, want AS15169 feed_count=2 attributed=512", fr.TopASNs)
	}

	as13335 := loadASNDetailPayloadForTest(t, filepath.Join(webDir, "asns", "13335.json"))
	if as13335.Totals.FeedsMatching != 1 || as13335.Totals.AttributedIPs != 256 {
		t.Fatalf("AS13335 totals = %+v, want gamma only", as13335.Totals)
	}
	if len(as13335.TopCountries) != 1 || as13335.TopCountries[0].Code != "US" || as13335.TopCountries[0].FeedCount != 1 || as13335.TopCountries[0].AttributedIPs != 256 {
		t.Fatalf("AS13335 countries = %+v, want US only", as13335.TopCountries)
	}

	as15169 := loadASNDetailPayloadForTest(t, filepath.Join(webDir, "asns", "15169.json"))
	if as15169.Totals.FeedsMatching != 2 || as15169.Totals.AttributedIPs != 768 || as15169.Totals.Countries != 2 {
		t.Fatalf("AS15169 totals = %+v, want alpha+beta across DE/FR", as15169.Totals)
	}
	if as15169.CountryDistribution == nil || as15169.CountryDistribution.TotalMapped != 768 {
		t.Fatalf("AS15169 distribution = %+v, want total_mapped=768", as15169.CountryDistribution)
	}
	gotCountries := map[string]ASNDetailCountry{}
	for _, row := range as15169.TopCountries {
		gotCountries[row.Code] = row
	}
	if gotCountries["FR"].FeedCount != 2 || gotCountries["FR"].AttributedIPs != 512 {
		t.Fatalf("AS15169 FR row = %+v, want feed_count=2 attributed=512", gotCountries["FR"])
	}
	if gotCountries["DE"].FeedCount != 1 || gotCountries["DE"].AttributedIPs != 256 {
		t.Fatalf("AS15169 DE row = %+v, want feed_count=1 attributed=256", gotCountries["DE"])
	}

	countryIndex := loadCountryIndexPayloadForTest(t, filepath.Join(webDir, "countries", "index.json"))
	gotCountryIndex := map[string]CountryIndexEntry{}
	for _, row := range countryIndex.Countries {
		gotCountryIndex[row.Code] = row
	}
	if gotCountryIndex["US"].FeedCount != 1 || gotCountryIndex["US"].AttributedIPs != 256 {
		t.Fatalf("country index US = %+v, want feed_count=1 attributed=256", gotCountryIndex["US"])
	}
	if gotCountryIndex["FR"].FeedCount != 2 || gotCountryIndex["FR"].AttributedIPs != 512 {
		t.Fatalf("country index FR = %+v, want feed_count=2 attributed=512", gotCountryIndex["FR"])
	}

	asnIndex := loadASNIndexPayloadForTest(t, filepath.Join(webDir, "asns", "index.json"))
	gotASNIndex := map[uint32]ASNIndexEntry{}
	for _, row := range asnIndex.ASNs {
		gotASNIndex[row.ASN] = row
	}
	if gotASNIndex[13335].FeedCount != 1 || gotASNIndex[13335].AttributedIPs != 256 {
		t.Fatalf("ASN index AS13335 = %+v, want feed_count=1 attributed=256", gotASNIndex[13335])
	}
	if gotASNIndex[15169].FeedCount != 2 || gotASNIndex[15169].AttributedIPs != 768 {
		t.Fatalf("ASN index AS15169 = %+v, want feed_count=2 attributed=768", gotASNIndex[15169])
	}
	if _, err := os.Stat(filepath.Join(libDir, "entities", "feeds-pending", "alpha.json")); !os.IsNotExist(err) {
		t.Fatalf("pending alpha feed sidecar should be consumed, got err=%v", err)
	}
	var promoted feedEntitySidecar
	loadJSONForTest(t, filepath.Join(libDir, "entities", "feeds", "alpha.json"), &promoted)
	if len(promoted.Countries) != 1 || promoted.Countries[0].Code != "FR" {
		t.Fatalf("promoted alpha feed sidecar = %+v, want FR-only", promoted.Countries)
	}
}

func TestProviderOnlyRunReportsEntityRefreshTargets(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }

	seedDetailFixturesForEntityTests(t, eng, webDir, libDir, now)

	report, err := eng.RunOnce(t.Context(), RunOptions{
		Selected:  []string{"dbip_country"},
		Reprocess: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Updated) != 0 {
		t.Fatalf("provider-only run updated feeds = %v, want none", report.Updated)
	}
	if !slices.Equal(report.EntityRefreshTargets, []string{"alpha", "beta", "gamma"}) {
		t.Fatalf("entity refresh targets = %v, want alpha beta gamma", report.EntityRefreshTargets)
	}
	if _, err := os.Stat(filepath.Join(libDir, "entities", "feeds-pending", "alpha.json")); err != nil {
		t.Fatalf("expected pending alpha feed sidecar: %v", err)
	}
}

func seedDetailFixturesForEntityTests(t *testing.T, eng *Engine, webDir, libDir string, now time.Time) {
	t.Helper()

	configureDetailFeedState(t, eng, "alpha", "intrusion", "Alpha Maintainer", "https://alpha.test", now.Add(-30*time.Minute).Unix(), 512)
	configureDetailFeedState(t, eng, "beta", "intrusion", "Beta Maintainer", "https://beta.test", now.Add(-20*time.Minute).Unix(), 512)
	configureDetailFeedState(t, eng, "gamma", "malware_infrastructure", "Gamma Maintainer", "https://gamma.test", now.Add(-10*time.Hour).Unix(), 256)

	writeLatestSetForDetailTest(t, libDir, "alpha", now, "1.1.1.0/24\n8.8.8.0/24\n")
	writeLatestSetForDetailTest(t, libDir, "beta", now, "8.8.8.0/24\n9.9.9.0/24\n")
	writeLatestSetForDetailTest(t, libDir, "gamma", now, "1.1.1.0/24\n")

	writeCountryPayloadForDetailTest(t, webDir, "alpha", "dbip_country", []CountryValue{
		{Code: "US", Value: 256},
		{Code: "DE", Value: 256},
	})
	writeCountryPayloadForDetailTest(t, webDir, "beta", "dbip_country", []CountryValue{
		{Code: "DE", Value: 256},
		{Code: "FR", Value: 256},
	})
	writeCountryPayloadForDetailTest(t, webDir, "gamma", "dbip_country", []CountryValue{
		{Code: "US", Value: 256},
	})

	writeASNPayloadForDetailTest(t, webDir, "alpha", "iptoasn", map[uint32]asnEntryJSON{
		13335: {ASN: 13335, Name: "CLOUDFLARENET", Count: 256},
		15169: {ASN: 15169, Name: "GOOGLE", Count: 256},
	})
	writeASNPayloadForDetailTest(t, webDir, "beta", "iptoasn", map[uint32]asnEntryJSON{
		15169: {ASN: 15169, Name: "GOOGLE", Count: 512},
	})
	writeASNPayloadForDetailTest(t, webDir, "gamma", "iptoasn", map[uint32]asnEntryJSON{
		13335: {ASN: 13335, Name: "CLOUDFLARENET", Count: 256},
	})

	writeDetailGeoProviderForTest(t, filepath.Join(libDir, "geolocation", "dbip_country.source"))
	writeDetailASNProviderForTest(t, filepath.Join(libDir, "asn", "iptoasn", "database.tsv"))
}

func newDetailEngineForTest(t *testing.T) (*Engine, string, string) {
	t.Helper()

	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	libDir := filepath.Join(root, "lib")
	webDir := filepath.Join(root, "web")
	cfgPath := filepath.Join(root, "config.yaml")
	cfg := fmt.Sprintf(`
runtime:
  base_dir: %q
  history_dir: %q
  lib_dir: %q
  errors_dir: %q
  web_dir: %q
  cache_dir: %q
  ipsets_apply: false
  feed_health_single_observation_grace_minutes: 60
  feed_health_default_healthy_cadence_minutes: 60
  feed_health_default_risky_cadence_minutes: 120
sources:
  alpha:
    url: https://example.test/alpha.txt
    frequency: 60
    ipv: ipv4
    output: ip
    category: intrusion
    info: alpha
    maintainer: Alpha Maintainer
    maintainer_url: https://alpha.test
  beta:
    url: https://example.test/beta.txt
    frequency: 60
    ipv: ipv4
    output: ip
    category: intrusion
    info: beta
    maintainer: Beta Maintainer
    maintainer_url: https://beta.test
  gamma:
    url: https://example.test/gamma.txt
    frequency: 60
    ipv: ipv4
    output: ip
    category: malware_infrastructure
    info: gamma
    maintainer: Gamma Maintainer
    maintainer_url: https://gamma.test
  dbip_country:
    url: https://example.test/dbip.csv.gz
    frequency: 1440
    use: [geoip]
    format: dbip_country_csv
    label: DB-IP Country
  iptoasn:
    url: https://example.test/iptoasn.tsv
    frequency: 1440
    use: [asn]
    format: iptoasn_combined_tsv
    label: IPtoASN
`, baseDir, filepath.Join(root, "history"), libDir, filepath.Join(root, "errors"), webDir, filepath.Join(root, "cache"))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(webDir, 0o700); err != nil {
		t.Fatal(err)
	}
	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	return eng, webDir, libDir
}

func configureDetailFeedState(t *testing.T, eng *Engine, name, category, maintainer, maintainerURL string, sourceDate int64, uniqueIPs uint64) {
	t.Helper()
	entry := eng.state.Entry(name)
	entry.Name = name
	entry.Category = category
	entry.Maintainer = maintainer
	entry.MaintainerURL = maintainerURL
	entry.SourceDate = sourceDate
	entry.ProcessedDate = sourceDate
	entry.CheckedDate = sourceDate
	entry.FrequencyMinutes = 60
	entry.AverageUpdateMins = 60
	entry.MinUpdateMins = 60
	entry.MaxUpdateMins = 60
	entry.Version = 2
	entry.Entries = int(uniqueIPs)
	entry.UniqueIPs = uniqueIPs
	entry.StartedDate = sourceDate - 86400
}

func writeLatestSetForDetailTest(t *testing.T, libDir, feed string, mod time.Time, body string) {
	t.Helper()
	set, err := iprange.ParseReader(t.Context(), feed, strings.NewReader(body), iprange.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse latest set for %s: %v", feed, err)
	}
	set.Optimize()
	path := filepath.Join(libDir, feed, "latest")
	if err := writeBinaryPath(path, set, mod); err != nil {
		t.Fatalf("write latest set for %s: %v", feed, err)
	}
}

func writeCountryPayloadForDetailTest(t *testing.T, webDir, feed, provider string, rows []CountryValue) {
	t.Helper()
	var total uint64
	for _, row := range rows {
		total += row.Value
	}
	data, err := json.Marshal(CountryComparisonPayload{
		TotalMapped: total,
		Countries:   rows,
	})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(webDir, fmt.Sprintf("%s_%s.json", feed, provider))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeASNPayloadForDetailTest(t *testing.T, webDir, feed, provider string, rows map[uint32]asnEntryJSON) {
	t.Helper()
	payload := asnFeedJSON{
		Provider: provider,
	}
	for _, row := range rows {
		payload.ByASN = append(payload.ByASN, row)
		payload.AttributedIPs += row.Count
		payload.FeedIPs += row.Count
	}
	slices.SortFunc(payload.ByASN, func(a, b asnEntryJSON) int {
		switch {
		case a.Count > b.Count:
			return -1
		case a.Count < b.Count:
			return 1
		case a.ASN < b.ASN:
			return -1
		case a.ASN > b.ASN:
			return 1
		default:
			return 0
		}
	})
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(webDir, fmt.Sprintf("%s_asn_%s.json", feed, provider))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeDetailGeoProviderForTest(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	var payload bytes.Buffer
	gw := gzip.NewWriter(&payload)
	_, _ = gw.Write([]byte("1.1.1.0,1.1.1.255,US\n8.8.8.0,8.8.8.255,DE\n9.9.9.0,9.9.9.255,FR\n"))
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, payload.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeDetailASNProviderForTest(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	body := strings.Join([]string{
		"1.1.1.0\t1.1.1.255\t13335\tUS\tCLOUDFLARENET",
		"8.8.8.0\t8.8.8.255\t15169\tDE\tGOOGLE",
		"9.9.9.0\t9.9.9.255\t15169\tFR\tGOOGLE",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writePendingFeedEntitySidecarForDetailTest(t *testing.T, libDir string, sidecar feedEntitySidecar) {
	t.Helper()
	path := filepath.Join(libDir, "entities", "feeds-pending", sidecar.Feed+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(sidecar)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func loadCountryDetailPayloadForTest(t *testing.T, path string) CountryDetailPayload {
	t.Helper()
	var payload CountryDetailPayload
	loadJSONForTest(t, path, &payload)
	return payload
}

func loadASNDetailPayloadForTest(t *testing.T, path string) ASNDetailPayload {
	t.Helper()
	var payload ASNDetailPayload
	loadJSONForTest(t, path, &payload)
	return payload
}

func loadCountryIndexPayloadForTest(t *testing.T, path string) CountryIndexPayload {
	t.Helper()
	var payload CountryIndexPayload
	loadJSONForTest(t, path, &payload)
	return payload
}

func loadASNIndexPayloadForTest(t *testing.T, path string) ASNIndexPayload {
	t.Helper()
	var payload ASNIndexPayload
	loadJSONForTest(t, path, &payload)
	return payload
}

func loadJSONForTest(t *testing.T, path string, dst any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
}
