package engine

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
)

func TestProviderListsUseConfigDefaults(t *testing.T) {
	cfg := config.New()
	cfg.Defaults.ASNProvider = "iptoasn"
	cfg.Defaults.GeoProvider = "dbip_country"
	cfg.Sources["caida_prefix2as"] = &config.Source{Name: "caida_prefix2as", Label: "CAIDA prefix2as", Use: []string{config.UseASN}, Format: "caida_prefix2as"}
	cfg.Sources["iptoasn"] = &config.Source{Name: "iptoasn", Label: "iptoasn.com", Use: []string{config.UseASN}, Format: "iptoasn_combined_tsv"}
	cfg.Sources["geolite2_country"] = &config.Source{Name: "geolite2_country", Label: "GeoLite2", Use: []string{config.UseGeoIP}, Format: "maxmind_country_csv"}
	cfg.Sources["dbip_country"] = &config.Source{Name: "dbip_country", Label: "DB-IP", Use: []string{config.UseGeoIP}, Format: "dbip_country_csv"}
	eng := newEngineFixture(t, withConfig(cfg))

	asnProviders := eng.ASNProviders()
	if len(asnProviders) < 2 || asnProviders[0].Name != "iptoasn" || asnProviders[1].Name != "caida_prefix2as" {
		t.Fatalf("ASN providers = %+v, want default first then catalog order", asnProviders)
	}
	geoProviders := eng.GeoProviders()
	if len(geoProviders) < 2 || geoProviders[0].Name != "dbip_country" || geoProviders[1].Name != "geolite2_country" {
		t.Fatalf("geo providers = %+v, want default first then catalog order", geoProviders)
	}
}

func TestProviderDefaultsMarkerDetectsConfigDrift(t *testing.T) {
	cfg := config.New()
	cfg.Defaults.ASNProvider = "iptoasn"
	cfg.Defaults.GeoProvider = "dbip_country"
	cfg.Sources["iptoasn"] = &config.Source{Name: "iptoasn", Label: "iptoasn.com", Use: []string{config.UseASN}, Format: "iptoasn_combined_tsv"}
	cfg.Sources["caida_prefix2as"] = &config.Source{Name: "caida_prefix2as", Label: "CAIDA prefix2as", Use: []string{config.UseASN}, Format: "caida_prefix2as"}
	cfg.Sources["dbip_country"] = &config.Source{Name: "dbip_country", Label: "DB-IP", Use: []string{config.UseGeoIP}, Format: "dbip_country_csv"}

	root := t.TempDir()
	eng := newEngineFixture(t, withConfig(cfg), withRuntime(func(rt *Runtime) {
		rt.LibDir = filepath.Join(root, "lib")
	}))
	if !eng.ProviderDefaultsChanged() {
		t.Fatal("missing marker should be treated as provider-default drift")
	}
	if err := eng.writeProviderDefaultsMarker(); err != nil {
		t.Fatal(err)
	}
	if eng.ProviderDefaultsChanged() {
		t.Fatal("matching marker should not be treated as provider-default drift")
	}
	cfg.Defaults.ASNProvider = "caida_prefix2as"
	if !eng.ProviderDefaultsChanged() {
		t.Fatal("changed default ASN provider should be treated as provider-default drift")
	}
}

func TestCriticalOverlapSummaryStoredForPublicFeedSummaries(t *testing.T) {
	cfg := config.New()
	cfg.Sources["sample"] = &config.Source{Name: "sample"}
	cfg.Sources["critical_dns"] = &config.Source{
		Name:     "critical_dns",
		Use:      []string{config.UseCriticalInfrastructure},
		Critical: testCriticalMetadata("hard", "public_dns_core"),
	}
	cfg.Sources["cloud_context"] = &config.Source{
		Name: "cloud_context",
		Use:  []string{config.UseProviderContext},
	}

	state := cache.New()
	state.ReplaceEntry("sample", cache.Entry{Name: "sample", CriticalOverlapTiers: []string{"hard", "soft"}})
	state.ReplaceEntry("critical_dns", cache.Entry{Name: "critical_dns"})
	state.ReplaceEntry("cloud_context", cache.Entry{Name: "cloud_context", CriticalOverlapTiers: []string{"soft"}})
	eng := newEngineFixture(t, withConfig(cfg), withState(state), withNow(func() time.Time {
		return time.Unix(1000, 0).UTC()
	}))

	summaries := summariesByName(eng.PublicFeedSummaries())
	summary := summaries["sample"]
	if len(summary.CriticalOverlapTiers) != 2 || summary.CriticalOverlapTiers[0] != "hard" {
		t.Fatalf("critical overlap tiers = %+v, want cached tiers", summary.CriticalOverlapTiers)
	}
	refSummary := summaries["critical_dns"]
	if refSummary.Critical == nil || refSummary.Critical.Tier != "hard" {
		t.Fatalf("critical reference summary = %+v, want hard metadata", refSummary.Critical)
	}
	if len(refSummary.CriticalOverlapTiers) != 0 {
		t.Fatalf("critical reference should not expose overlap tiers, got %+v", refSummary.CriticalOverlapTiers)
	}
	contextSummary := summaries["cloud_context"]
	if len(contextSummary.CriticalOverlapTiers) != 0 {
		t.Fatalf("provider context should not expose critical overlap tiers, got %+v", contextSummary.CriticalOverlapTiers)
	}
}

func summariesByName(summaries []PublicFeedSummary) map[string]PublicFeedSummary {
	out := make(map[string]PublicFeedSummary, len(summaries))
	for _, summary := range summaries {
		out[summary.Name] = summary
	}
	return out
}

func TestProviderDefaultsMarkerPathEmptyWithoutLibDir(t *testing.T) {
	if got := ProviderDefaultsSetMarkerPath(Runtime{}); got != "" {
		t.Fatalf("marker path = %q, want empty without LibDir", got)
	}
}
