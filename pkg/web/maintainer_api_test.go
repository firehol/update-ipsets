package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/scheduler"
)

func TestMaintainerAPIEndpoints(t *testing.T) {
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("1.2.3.4\n5.6.7.0/30\n"))
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
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"), sourceServer.URL)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
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

	var index engine.MaintainerIndexPayload
	status, _ := server.getJSON(t, "/api/v1/maintainers", &index)
	if status != http.StatusOK {
		t.Fatalf("unexpected maintainer index status: got %d", status)
	}
	if len(index.Maintainers) != 1 {
		t.Fatalf("maintainer count = %d, want 1: %+v", len(index.Maintainers), index.Maintainers)
	}
	if got, want := index.Maintainers[0].Slug, "example-org"; got != want {
		t.Fatalf("maintainer slug = %q, want %q", got, want)
	}
	if got, want := index.Maintainers[0].FeedCount, 1; got != want {
		t.Fatalf("maintainer feed count = %d, want %d", got, want)
	}

	var detail engine.MaintainerDetailPayload
	status, _ = server.getJSON(t, "/api/v1/maintainers/example-org", &detail)
	if status != http.StatusOK {
		t.Fatalf("unexpected maintainer detail status: got %d", status)
	}
	if got, want := detail.Name, "Example Org"; got != want {
		t.Fatalf("maintainer detail name = %q, want %q", got, want)
	}
	if got, want := detail.Totals.Feeds, 1; got != want {
		t.Fatalf("maintainer feeds total = %d, want %d", got, want)
	}
	if got := detail.FeedsByCategory["attacks"]; len(got) != 1 || got[0].Name != "sample" {
		t.Fatalf("maintainer feeds by category = %+v, want sample under attacks", detail.FeedsByCategory)
	}

	var missingBody map[string]string
	status, _ = server.getJSON(t, "/api/v1/maintainers/missing-org", &missingBody)
	if status != http.StatusNotFound {
		t.Fatalf("unexpected missing maintainer status: got %d", status)
	}
	if missingBody["error"] != "maintainer not found" {
		t.Fatalf("missing maintainer error = %q", missingBody["error"])
	}
}

func TestMaintainerDetailBackendErrorIsNotMappedToNotFound(t *testing.T) {
	handler := newHandler(&engine.Engine{}, Options{EnableAll: true}, nil)
	server := newWebHTTPTestServer(t, handler)

	var body map[string]string
	status, _, rawBody := server.get(t, "/api/v1/maintainers/example-org")
	if status != http.StatusInternalServerError {
		t.Fatalf("unexpected backend error status: got %d", status)
	}
	if err := json.Unmarshal(rawBody, &body); err != nil {
		t.Fatalf("decode backend error response: %v", err)
	}
	if !strings.Contains(body["error"], "engine is not configured") {
		t.Fatalf("backend error body = %q", body["error"])
	}
}
