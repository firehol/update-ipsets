package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/scheduler"
)

func TestCountryEndpointNormalizesLegacyArrayPayload(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	runtimeWebDir := filepath.Join(root, "runtime-web")
	servedWebDir := filepath.Join(root, "served-web")
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
sources:
  sample:
    url: https://example.test/sample.txt
    frequency: 1
    ipv: ipv4
    output: ip
  geolite2_country:
    url: https://example.test/geo.csv
    frequency: 1
    use: [geoip]
    format: maxmind_country_csv
`, baseDir, filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), runtimeWebDir, filepath.Join(root, "cache"))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(runtimeWebDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(servedWebDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(servedWebDir, "sample_geolite2_country.json"), []byte(`[
  {"code":"US","value":5},
  {"code":"DE","value":2}
]`), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	handler := newHandler(eng, Options{EnableAll: true, WebDir: servedWebDir}, scheduler.New(eng, true, nil))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sets/sample/countries/geolite2_country", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		TotalMapped int `json:"total_mapped"`
		Countries   []struct {
			Code  string `json:"code"`
			Value int    `json:"value"`
		} `json:"countries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v body=%s", err, rec.Body.String())
	}
	if payload.TotalMapped != 7 {
		t.Fatalf("expected total_mapped 7, got %d", payload.TotalMapped)
	}
	if len(payload.Countries) != 2 {
		t.Fatalf("expected 2 countries, got %d", len(payload.Countries))
	}
}
