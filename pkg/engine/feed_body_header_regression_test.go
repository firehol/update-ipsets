package engine

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCanonicalFeedBodySameIgnoresHeaderComments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.ipset")
	body := []byte("1.2.3.4\n5.6.7.8\n")
	headered := "#\n# sample\n# " + strings.Repeat("x", 128*1024) + "\n" + string(body)
	if err := os.WriteFile(path, []byte(headered), 0o600); err != nil {
		t.Fatal(err)
	}

	same, err := canonicalFeedBodySame(path, body)
	if err != nil {
		t.Fatal(err)
	}
	if !same {
		t.Fatal("expected headered committed body to match canonical body")
	}

	same, err = canonicalFeedBodySame(path, []byte("1.2.3.4\n"))
	if err != nil {
		t.Fatal(err)
	}
	if same {
		t.Fatal("expected different canonical body to be detected")
	}
}

func TestRunOnceReprocessFromHeaderedCommittedFileDoesNotDuplicateHeader(t *testing.T) {
	modified := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", modified.Format(http.TimeFormat))
		_, _ = w.Write([]byte("1.2.3.4\n"))
	}))
	defer server.Close()

	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
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
    maintainer: test
    maintainer_url: https://example.test
`, baseDir, filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"), server.URL)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	eng.now = func() time.Time { return modified.Add(time.Hour) }
	if _, err := runSchedulerStyleOnce(t, eng, RunOptions{Selected: []string{"sample"}, EnableAll: true, Manual: true, CleanupOld: true}); err != nil {
		t.Fatal(err)
	}

	report, err := eng.RunOnce(t.Context(), RunOptions{Selected: []string{"sample"}, EnableAll: true, Reprocess: true, Manual: true, CleanupOld: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Updated) != 1 || report.Updated[0] != "sample" {
		t.Fatalf("expected direct local reprocess to update sample, got %#v", report.Updated)
	}

	data, err := os.ReadFile(filepath.Join(baseDir, "sample.ipset"))
	if err != nil {
		t.Fatal(err)
	}
	if got := bytes.Count(data, []byte("#\n# sample\n#\n")); got != 1 {
		t.Fatalf("expected exactly one header block after reprocess, got %d:\n%s", got, data)
	}
	if got := bytes.Count(data, []byte("# List source URL : ")); got != 1 {
		t.Fatalf("expected exactly one source URL header after reprocess, got %d:\n%s", got, data)
	}
	if got := bytes.Count(data, []byte("1.2.3.4\n")); got != 1 {
		t.Fatalf("expected body entry once after reprocess, got %d:\n%s", got, data)
	}
}
