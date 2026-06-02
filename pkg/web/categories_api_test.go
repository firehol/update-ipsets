package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/scheduler"
)

func TestCategoriesAndProvenanceAPI(t *testing.T) {
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("1.2.3.4\n"))
	}))
	defer sourceServer.Close()

	root := t.TempDir()
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
categories:
  intrusion:
    label: Intrusion
    description: Active hostile access attempts.
    color: "#dc2626"
    sort_order: 10
  asn:
    label: ASN
    description: Provider attribution datasets.
    public: false
sources:
  sample:
    url: %q
    frequency: 1
    ipv: ipv4
    output: ip
    processor:
      - passthrough
    category: intrusion
    provenance: secondary_upstream
    info: sample feed
    maintainer: test
    maintainer_url: https://example.test
    license: Proprietary test license
    redistributable: false
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"), sourceServer.URL)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runSchedulerStyleOnce(t, eng, engine.RunOptions{EnableAll: true, Manual: true, CleanupOld: true}); err != nil {
		t.Fatal(err)
	}

	runner := scheduler.New(eng, true, nil)
	handler := newHandler(eng, Options{EnableAll: true}, runner)
	server := newWebHTTPTestServer(t, handler)

	var categories []engine.PublicCategory
	status, _ := server.getJSON(t, "/api/v1/categories", &categories)
	if status != http.StatusOK {
		t.Fatalf("unexpected categories status: got %d", status)
	}
	var intrusion *engine.PublicCategory
	for i := range categories {
		if categories[i].Name == "intrusion" {
			intrusion = &categories[i]
		}
		if categories[i].Name == "asn" {
			t.Fatalf("non-public category leaked into categories payload: %+v", categories)
		}
	}
	if intrusion == nil {
		t.Fatalf("missing intrusion category in payload: %+v", categories)
	}
	if intrusion.Label != "Intrusion" {
		t.Fatalf("intrusion label = %q, want Intrusion", intrusion.Label)
	}

	var feeds []engine.PublicFeedSummary
	status, _ = server.getJSON(t, "/api/v1/sets", &feeds)
	if status != http.StatusOK {
		t.Fatalf("unexpected sets status: got %d", status)
	}
	var sample *engine.PublicFeedSummary
	for i := range feeds {
		if feeds[i].Name == "sample" {
			sample = &feeds[i]
			break
		}
	}
	if sample == nil {
		t.Fatalf("missing sample feed summary in payload: %+v", feeds)
	}
	if sample.Category != "intrusion" {
		t.Fatalf("sample category = %q, want intrusion", sample.Category)
	}
	if sample.Provenance != "secondary_upstream" {
		t.Fatalf("sample provenance = %q, want secondary_upstream", sample.Provenance)
	}
	if sample.License != "Proprietary test license" {
		t.Fatalf("sample license = %q, want Proprietary test license", sample.License)
	}
	if sample.Redistributable {
		t.Fatal("sample redistributable = true, want false")
	}
}
