package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHomeGlobeAggregatesSelectedCategories(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	webDir := filepath.Join(root, "web")
	cfgPath := filepath.Join(root, "config.yaml")
	now := time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC)

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
  feed_health_category_thresholds:
    intrusion:
      healthy_cadence_minutes: 60
      risky_cadence_minutes: 120
    malware_infrastructure:
      healthy_cadence_minutes: 60
      risky_cadence_minutes: 120
sources:
  alpha:
    url: https://example.test/alpha.txt
    frequency: 60
    ipv: ipv4
    output: ip
    category: intrusion
    info: alpha
    maintainer: test
    maintainer_url: https://example.test
  beta:
    url: https://example.test/beta.txt
    frequency: 60
    ipv: ipv4
    output: ip
    category: malware_infrastructure
    info: beta
    maintainer: test
    maintainer_url: https://example.test
  gamma:
    url: https://example.test/gamma.txt
    frequency: 60
    ipv: ipv4
    output: ip
    category: intrusion
    info: gamma
    maintainer: test
    maintainer_url: https://example.test
  delta:
    url: https://example.test/delta.txt
    frequency: 60
    ipv: ipv4
    output: ip
    category: intrusion
    provenance: secondary_merge
    info: delta
    maintainer: test
    maintainer_url: https://example.test
  geolite2_country:
    url: https://example.test/geo.csv
    frequency: 1440
    use: [geoip]
    format: maxmind_country_csv
    label: GeoLite2 Country
`, baseDir, filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), webDir, filepath.Join(root, "cache"))
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
	eng.now = func() time.Time { return now }

	for _, state := range []struct {
		name       string
		category   string
		sourceDate int64
	}{
		{name: "alpha", category: "intrusion", sourceDate: now.Add(-30 * time.Minute).Unix()},
		{name: "beta", category: "malware_infrastructure", sourceDate: now.Add(-90 * time.Minute).Unix()},
		{name: "gamma", category: "intrusion", sourceDate: now.Add(-6 * time.Hour).Unix()},
		{name: "delta", category: "intrusion", sourceDate: now.Add(-20 * time.Minute).Unix()},
	} {
		entry := eng.state.Entry(state.name)
		entry.Name = state.name
		entry.Category = state.category
		entry.SourceDate = state.sourceDate
		entry.FrequencyMinutes = 60
		entry.AverageUpdateMins = 60
		entry.MinUpdateMins = 60
		entry.MaxUpdateMins = 60
		entry.Version = 2
		entry.Entries = 10
		entry.UniqueIPs = 10
		entry.StartedDate = now.Add(-7 * 24 * time.Hour).Unix()
	}

	writeCountryPayload := func(name string, rows []CountryValue) {
		t.Helper()
		data, err := json.Marshal(CountryComparisonPayload{
			TotalMapped: 10,
			Countries:   rows,
		})
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(webDir, name+"_geolite2_country.json")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeCountryPayload("alpha", []CountryValue{
		{Code: "US", Value: 8},
		{Code: "DE", Value: 2},
	})
	writeCountryPayload("beta", []CountryValue{
		{Code: "US", Value: 3},
		{Code: "FR", Value: 7},
	})
	writeCountryPayload("gamma", []CountryValue{
		{Code: "CN", Value: 10},
	})
	writeCountryPayload("delta", []CountryValue{
		{Code: "IT", Value: 10},
	})
	if _, err := eng.stageHomeAggregates(t.Context(), webDir, webDir); err != nil {
		t.Fatal(err)
	}

	payload, err := eng.HomeGlobe([]string{"intrusion", "malware_infrastructure"})
	if err != nil {
		t.Fatal(err)
	}
	if payload.Provider != "geolite2_country" {
		t.Fatalf("provider = %q, want geolite2_country", payload.Provider)
	}
	if payload.ProviderLabel != "GeoLite2 Country" {
		t.Fatalf("provider label = %q", payload.ProviderLabel)
	}
	if payload.EligibleFeeds != 2 {
		t.Fatalf("eligible feeds = %d, want 2", payload.EligibleFeeds)
	}
	if payload.ContributingFeeds != 2 {
		t.Fatalf("contributing feeds = %d, want 2", payload.ContributingFeeds)
	}
	if len(payload.Countries) != 3 {
		t.Fatalf("countries = %d, want 3", len(payload.Countries))
	}
	if payload.Countries[0].Code != "US" || payload.Countries[0].FeedCount != 2 {
		t.Fatalf("unexpected top country: %+v", payload.Countries[0])
	}
	for _, country := range payload.Countries {
		if country.Code == "CN" {
			t.Fatalf("unexpected excluded-country entry: %+v", country)
		}
		if country.Code == "IT" {
			t.Fatalf("unexpected secondary-provenance country entry: %+v", country)
		}
	}
}
