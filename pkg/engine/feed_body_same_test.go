package engine

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFetchAndStageSkipsProcessingWhenRawBytesChangeButCanonicalBodySame(t *testing.T) {
	firstModified := time.Date(2026, 4, 24, 9, 0, 0, 0, time.UTC)
	secondModified := firstModified.Add(time.Hour)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch requests {
		case 1:
			w.Header().Set("Last-Modified", firstModified.Format(http.TimeFormat))
			_, _ = w.Write([]byte("198.51.100.0/24\n192.0.2.0/24\n"))
		default:
			w.Header().Set("Last-Modified", secondModified.Format(http.TimeFormat))
			_, _ = w.Write([]byte("# upstream comment changed\n192.0.2.0/24\n198.51.100.0/24\n192.0.2.0/24\n"))
		}
	}))
	defer server.Close()

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
    output: netset
    processor:
      - passthrough
    category: tests
    info: sample feed
    maintainer: test
    maintainer_url: https://example.test
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"), server.URL)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	report, err := runSchedulerStyleOnce(t, eng, RunOptions{Selected: []string{"sample"}, EnableAll: true, CleanupOld: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Updated) != 1 || report.Updated[0] != "sample" {
		t.Fatalf("expected initial sample processing, got %#v", report.Updated)
	}

	decision, err := eng.FetchAndStage(t.Context(), "sample", false, true)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Status != DownloadStatusSame {
		t.Fatalf("decision status = %q, want %q", decision.Status, DownloadStatusSame)
	}
	if len(decision.ProcessingNames) != 0 {
		t.Fatalf("same canonical body should not queue processing, got %#v", decision.ProcessingNames)
	}
	if _, err := os.Stat(stagedPath(eng.feedBodyPath("sample"))); !os.IsNotExist(err) {
		t.Fatalf("same canonical body left staged processing body or stat failed with %v", err)
	}
}
