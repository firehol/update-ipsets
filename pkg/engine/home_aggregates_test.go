package engine

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
)

func TestHomeSummaryReadsPrecomputedAggregateWithoutPerFeedArtifacts(t *testing.T) {
	eng := newEngineFixture(t, withConfig(homeAggregateTestConfig()))
	artifact := homeAggregatesPayload{
		Version: homeAggregatesVersion,
		Providers: HomeSummaryProviders{
			Geo: HomeSummaryProvider{Name: "geolite2_country", Label: "GeoLite2 Country"},
			ASN: HomeSummaryProvider{Name: "iptoasn", Label: "IPtoASN"},
		},
		Categories: []homeCategoryAggregate{
			{
				Category:          "intrusion",
				EligibleFeeds:     1,
				ContributingFeeds: 1,
				UniqueIPs:         10,
				Countries: []HomeSummaryCountry{
					{Code: "US", FeedCount: 1, AttributedIPs: 10},
				},
				ASNs: []HomeSummaryASN{
					{ASN: 13335, Name: "CLOUDFLARENET", FeedCount: 1, AttributedIPs: 10},
				},
				Maintainers: []HomeSummaryMaintainer{
					{
						Slug:              "example",
						Name:              "Example",
						FeedCount:         1,
						UniqueIPs:         10,
						CategoryBreakdown: map[string]int{"intrusion": 1},
					},
				},
			},
		},
	}
	if err := writeJSONFile(filepath.Join(eng.outputDir(), eng.publicHomeAggregatesRelPath()), artifact); err != nil {
		t.Fatal(err)
	}

	summary, err := eng.HomeSummaryInDir([]string{"intrusion"}, 20, eng.outputDir())
	if err != nil {
		t.Fatal(err)
	}
	if summary.EligibleFeeds != 1 || summary.ContributingFeeds != 1 {
		t.Fatalf("unexpected summary counts: eligible=%d contributing=%d", summary.EligibleFeeds, summary.ContributingFeeds)
	}
	if len(summary.TopCountries) != 1 || summary.TopCountries[0].Code != "US" {
		t.Fatalf("summary did not use aggregate countries: %+v", summary.TopCountries)
	}
	if len(summary.TopASNs) != 1 || summary.TopASNs[0].ASN != 13335 {
		t.Fatalf("summary did not use aggregate ASNs: %+v", summary.TopASNs)
	}

	globe, err := eng.HomeGlobeInDir([]string{"intrusion"}, eng.outputDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(globe.Countries) != 1 || globe.Countries[0].Code != "US" {
		t.Fatalf("globe did not use aggregate countries: %+v", globe.Countries)
	}
}

func TestHomeSummaryMissingAggregateReturnsNotReady(t *testing.T) {
	eng := newEngineFixture(t, withConfig(homeAggregateTestConfig()))
	_, err := eng.HomeSummaryInDir(nil, 20, eng.outputDir())
	if !errors.Is(err, ErrHomeAggregatesNotReady) {
		t.Fatalf("error = %v, want ErrHomeAggregatesNotReady", err)
	}
}

func TestStageHomeAggregatesPrefersStagedArtifactsAndFallsBackToLive(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	cfg := homeAggregateTestConfig()
	cfg.Sources["beta"] = &config.Source{
		Name:          "beta",
		URL:           "https://example.test/beta.txt",
		Frequency:     60,
		IPV:           "ipv4",
		Output:        "ip",
		Category:      "intrusion",
		Info:          "beta",
		Maintainer:    "Example",
		MaintainerURL: "https://example.test",
	}
	eng := newEngineFixture(t, withConfig(cfg), withNow(func() time.Time { return now }))
	for _, name := range []string{"alpha", "beta"} {
		entry := eng.state.Entry(name)
		entry.Name = name
		entry.Category = "intrusion"
		entry.SourceDate = now.Add(-30 * time.Minute).Unix()
		entry.ProcessedDate = entry.SourceDate
		entry.FrequencyMinutes = 60
		entry.AverageUpdateMins = 60
		entry.MinUpdateMins = 60
		entry.MaxUpdateMins = 60
		entry.Version = 2
		entry.Entries = 10
		entry.UniqueIPs = 10
		entry.Maintainer = "Example"
		entry.MaintainerURL = "https://example.test"
	}

	writeHomeCountryPayload(t, eng.outputDir(), "alpha", "geolite2_country", []CountryValue{{Code: "US", Value: 10}})
	writeHomeCountryPayload(t, eng.outputDir(), "beta", "geolite2_country", []CountryValue{{Code: "DE", Value: 10}})
	stageDir := filepath.Join(t.TempDir(), "stage")
	writeHomeCountryPayload(t, stageDir, "alpha", "geolite2_country", []CountryValue{{Code: "FR", Value: 10}})

	generated, err := eng.stageHomeAggregates(t.Context(), stageDir, stageDir)
	if err != nil {
		t.Fatal(err)
	}
	if generated.Path != filepath.Join(eng.outputDir(), eng.publicHomeAggregatesRelPath()) {
		t.Fatalf("generated path = %q, want %q", generated.Path, filepath.Join(eng.outputDir(), eng.publicHomeAggregatesRelPath()))
	}
	if !generated.Timestamp.Equal(now) {
		t.Fatalf("generated timestamp = %s, want %s", generated.Timestamp, now)
	}
	data, err := os.ReadFile(filepath.Join(stageDir, eng.publicHomeAggregatesRelPath()))
	if err != nil {
		t.Fatal(err)
	}
	var artifact homeAggregatesPayload
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatal(err)
	}
	summary := composeHomeSummaryFromAggregates(&artifact, normalizeCategoryFilter([]string{"intrusion"}), 20)
	codes := make(map[string]struct{}, len(summary.TopCountries))
	for _, row := range summary.TopCountries {
		codes[row.Code] = struct{}{}
	}
	if _, ok := codes["FR"]; !ok {
		t.Fatalf("staged alpha country was not used: %+v", summary.TopCountries)
	}
	if _, ok := codes["DE"]; !ok {
		t.Fatalf("live beta country fallback was not used: %+v", summary.TopCountries)
	}
	if _, ok := codes["US"]; ok {
		t.Fatalf("live alpha country should be hidden by staged replacement: %+v", summary.TopCountries)
	}
}

func TestStageHomeAggregatesRejectsMalformedInputArtifacts(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		path    string
		wantErr string
	}{
		{
			name:    "country",
			path:    "alpha_geolite2_country.json",
			wantErr: "read homepage country aggregate input for alpha/geolite2_country",
		},
		{
			name:    "asn",
			path:    "alpha_asn_iptoasn.json",
			wantErr: "read homepage ASN aggregate input for alpha/iptoasn",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eng := newEngineFixture(t, withConfig(homeAggregateTestConfig()), withNow(func() time.Time { return now }))
			setHomeAggregateTestEntry(eng, "alpha", now)
			stageDir := t.TempDir()
			if err := os.WriteFile(filepath.Join(stageDir, tt.path), []byte(`{`), 0o644); err != nil {
				t.Fatal(err)
			}

			_, err := eng.stageHomeAggregates(t.Context(), stageDir, stageDir)
			if err == nil {
				t.Fatal("expected malformed homepage aggregate input to fail")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want context %q", err, tt.wantErr)
			}
		})
	}
}

func TestHealthTransitionRefreshesHomeAggregateWithoutEntitySidecars(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	eng := newEngineFixture(t, withConfig(homeAggregateTestConfig()), withNow(func() time.Time { return now }))
	setHomeAggregateTestEntry(eng, "alpha", now)

	if err := eng.refreshEntityArtifactsForHealthTransitions(t.Context(), []string{"alpha"}, nil); err != nil {
		t.Fatal(err)
	}
	summary, err := eng.HomeSummaryInDir(nil, 20, eng.outputDir())
	if err != nil {
		t.Fatal(err)
	}
	if summary.EligibleFeeds != 1 || summary.Totals.Feeds != 1 {
		t.Fatalf("home aggregate was not refreshed from health transition: %+v", summary.Totals)
	}
}

func homeAggregateTestConfig() *config.Config {
	cfg := config.New()
	cfg.Runtime.FeedHealthSingleObservationGraceMins = 60
	cfg.Runtime.FeedHealthDefaultHealthyCadenceMins = 60
	cfg.Runtime.FeedHealthDefaultRiskyCadenceMins = 120
	cfg.Sources["alpha"] = &config.Source{
		Name:          "alpha",
		URL:           "https://example.test/alpha.txt",
		Frequency:     60,
		IPV:           "ipv4",
		Output:        "ip",
		Category:      "intrusion",
		Info:          "alpha",
		Maintainer:    "Example",
		MaintainerURL: "https://example.test",
	}
	cfg.Sources["geolite2_country"] = &config.Source{
		Name:      "geolite2_country",
		URL:       "https://example.test/geo.csv",
		Frequency: 1440,
		Use:       []string{config.UseGeoIP},
		Format:    "maxmind_country_csv",
		Label:     "GeoLite2 Country",
	}
	cfg.Sources["iptoasn"] = &config.Source{
		Name:      "iptoasn",
		URL:       "https://example.test/iptoasn.tsv",
		Frequency: 1440,
		Use:       []string{config.UseASN},
		Format:    "iptoasn_tsv",
		Label:     "IPtoASN",
	}
	return cfg
}

func setHomeAggregateTestEntry(eng *Engine, name string, now time.Time) {
	entry := eng.state.Entry(name)
	entry.Name = name
	entry.Category = "intrusion"
	entry.SourceDate = now.Add(-30 * time.Minute).Unix()
	entry.ProcessedDate = entry.SourceDate
	entry.FrequencyMinutes = 60
	entry.AverageUpdateMins = 60
	entry.MinUpdateMins = 60
	entry.MaxUpdateMins = 60
	entry.Version = 2
	entry.Entries = 10
	entry.UniqueIPs = 10
	entry.Maintainer = "Example"
	entry.MaintainerURL = "https://example.test"
}

func writeHomeCountryPayload(t *testing.T, dir, feed, provider string, rows []CountryValue) {
	t.Helper()
	data, err := json.Marshal(CountryComparisonPayload{
		TotalMapped: 10,
		Countries:   rows,
	})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, feed+"_"+provider+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
