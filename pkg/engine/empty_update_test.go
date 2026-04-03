package engine

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/feedhealth"
)

func TestRunOnceEmptyUpdateReplacesPreviousSet(t *testing.T) {
	var mu sync.RWMutex
	body := "1.2.3.4\n5.6.7.8\n"
	modified := time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		defer mu.RUnlock()
		w.Header().Set("Last-Modified", modified.Format(http.TimeFormat))
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	webDir := filepath.Join(root, "web")
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
    category: intrusion
    info: sample feed
    maintainer: test
    maintainer_url: https://example.test
`, baseDir, filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), webDir, filepath.Join(root, "cache"), server.URL)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}

	report, err := runSchedulerStyleOnce(t, eng, RunOptions{
		EnableAll:  true,
		Manual:     true,
		CleanupOld: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Failed) != 0 {
		t.Fatalf("unexpected initial failures: %#v", report)
	}

	entry := eng.state.EntrySnapshot("sample")
	if entry == nil {
		t.Fatal("missing cache entry after initial run")
	}
	if got, want := entry.UniqueIPs, uint64(2); got != want {
		t.Fatalf("unexpected initial unique IPs: got %d want %d", got, want)
	}

	mu.Lock()
	body = ""
	modified = modified.Add(time.Hour)
	mu.Unlock()

	report, err = runSchedulerStyleOnce(t, eng, RunOptions{
		EnableAll:  true,
		Manual:     true,
		CleanupOld: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Failed) != 0 {
		t.Fatalf("unexpected second-run failures: %#v", report)
	}

	entry = eng.state.EntrySnapshot("sample")
	if entry == nil {
		t.Fatal("missing cache entry after empty run")
	}
	if got := entry.Entries; got != 0 {
		t.Fatalf("expected entries to be zero after empty run, got %d", got)
	}
	if got := entry.UniqueIPs; got != 0 {
		t.Fatalf("expected unique IPs to be zero after empty run, got %d", got)
	}
	if entry.LastStatus != "empty" {
		t.Fatalf("expected last status empty, got %q", entry.LastStatus)
	}
	if entry.LastError != "" {
		t.Fatalf("expected no last error for valid empty run, got %q", entry.LastError)
	}

	summaries := eng.PublicFeedSummaries()
	if len(summaries) != 1 {
		t.Fatalf("expected one public summary, got %d", len(summaries))
	}
	if summaries[0].Health.Class != feedhealth.ClassEmpty {
		t.Fatalf("expected public health class empty, got %q", summaries[0].Health.Class)
	}
	if summaries[0].UniqueIPs != 0 {
		t.Fatalf("expected public unique IPs to be zero, got %d", summaries[0].UniqueIPs)
	}

	matches, err := eng.QueryIP(t.Context(), "1.2.3.4")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected old IPs to disappear after empty run, got %#v", matches)
	}
}
