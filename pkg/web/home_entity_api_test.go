package web

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/scheduler"
)

func TestCountryAndASNAPIEndpointsServePrecomputedArtifacts(t *testing.T) {
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("1.1.1.0/24\n"))
	}))
	defer sourceServer.Close()

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
    url: %q
    frequency: 1
    ipv: ipv4
    output: ip
    processor:
      - passthrough
    category: attacks
    info: sample feed
    maintainer: Example Org
    maintainer_url: https://example.test
  dbip_country:
    url: https://example.test/geo.csv
    frequency: 1440
    hidden: true
    use: [geoip]
    format: maxmind_country_csv
  iptoasn:
    url: https://example.test/asn.tsv
    frequency: 1440
    hidden: true
    use: [asn]
    format: iptoasn_tsv
`, baseDir, filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), runtimeWebDir, filepath.Join(root, "cache"), sourceServer.URL)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runSchedulerStyleOnce(t, eng, engine.RunOptions{
		Selected:   []string{"sample"},
		EnableAll:  true,
		Manual:     true,
		CleanupOld: true,
	}); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(runtimeWebDir, "sample_dbip_country.json"), []byte(`{"total_mapped":256,"countries":[{"code":"US","value":256}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtimeWebDir, "sample_asn_iptoasn.json"), []byte(`{"by_asn":[{"asn":13335,"name":"CLOUDFLARENET","count":256}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := eng.RebuildEntityArtifacts(); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(servedWebDir, "countries"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(servedWebDir, "asns"), 0o755); err != nil {
		t.Fatal(err)
	}
	servedArtifacts := map[string]string{
		filepath.Join(servedWebDir, "countries", "index.json"): `{"countries":[{"code":"ZZ","name":"Served Country"}]}`,
		filepath.Join(servedWebDir, "countries", "ZZ.json"):    `{"code":"ZZ","totals":{"feeds_matching":99}}`,
		filepath.Join(servedWebDir, "asns", "index.json"):      `{"asns":[{"asn":64512,"name":"SERVED-ASN"}]}`,
		filepath.Join(servedWebDir, "asns", "64512.json"):      `{"asn":64512,"name":"SERVED-ASN","totals":{"feeds_matching":99}}`,
	}
	for path, body := range servedArtifacts {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	handler := newHandler(eng, Options{EnableAll: true, WebDir: servedWebDir}, scheduler.New(eng, true, nil))

	for _, tc := range []struct {
		path string
		want string
	}{
		{path: "/api/v1/countries", want: `"code":"ZZ"`},
		{path: "/api/v1/countries/ZZ", want: `"feeds_matching":99`},
		{path: "/api/v1/asns", want: `"asn":64512`},
		{path: "/api/v1/asns/64512", want: `"name":"SERVED-ASN"`},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: unexpected status %d body=%s", tc.path, rec.Code, rec.Body.String())
		}
		body, _ := io.ReadAll(rec.Result().Body)
		if !strings.Contains(string(body), tc.want) {
			t.Fatalf("%s: expected %q in body, got %s", tc.path, tc.want, body)
		}
	}

	for _, path := range []string{
		filepath.Join(servedWebDir, "countries", "index.json"),
		filepath.Join(servedWebDir, "countries", "US.json"),
		filepath.Join(servedWebDir, "asns", "index.json"),
		filepath.Join(servedWebDir, "asns", "13335.json"),
	} {
		_ = os.Remove(path)
	}
	for _, tc := range []struct {
		path       string
		rejectCode int
	}{
		{path: "/api/v1/countries", rejectCode: http.StatusOK},
		{path: "/api/v1/countries/US", rejectCode: http.StatusOK},
		{path: "/api/v1/asns", rejectCode: http.StatusOK},
		{path: "/api/v1/asns/13335", rejectCode: http.StatusOK},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code == tc.rejectCode {
			t.Fatalf("%s: unexpectedly served runtime/live entity data: status=%d body=%s", tc.path, rec.Code, rec.Body.String())
		}
	}

	if err := os.WriteFile(filepath.Join(servedWebDir, "ESCAPED.json"), []byte(`{"escaped":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/countries/../escaped", nil)
	rec := httptest.NewRecorder()
	handleCountryDetail(eng, newFileCache(), servedWebDir).ServeHTTP(rec, req)
	if rec.Code == http.StatusOK || strings.Contains(rec.Body.String(), "escaped") {
		t.Fatalf("country detail path traversal served outside countries dir: status=%d body=%s", rec.Code, rec.Body.String())
	}
}
