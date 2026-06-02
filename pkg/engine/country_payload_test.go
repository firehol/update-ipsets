package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCountryComparisonAcceptsLegacyArray(t *testing.T) {
	dir := t.TempDir()
	webDir := filepath.Join(dir, "web")
	if err := os.MkdirAll(webDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(webDir, "sample_geolite2_country.json")
	if err := os.WriteFile(path, []byte(`[
  {"code":"US","value":5},
  {"code":"DE","value":2}
]`), 0o600); err != nil {
		t.Fatal(err)
	}

	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.WebDir = webDir
	}))
	payload, err := eng.CountryComparison("sample", "geolite2_country")
	if err != nil {
		t.Fatal(err)
	}
	if payload.TotalMapped != 7 {
		t.Fatalf("expected total_mapped 7, got %d", payload.TotalMapped)
	}
	if len(payload.Countries) != 2 {
		t.Fatalf("expected 2 countries, got %d", len(payload.Countries))
	}
	if payload.Countries[0].Code != "DE" || payload.Countries[1].Code != "US" {
		t.Fatalf("expected countries sorted by code, got %+v", payload.Countries)
	}
}
