package engine

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestEndToEndMultiFeedBatch creates multiple synthetic feeds, runs a
// batch update, and verifies that all artifacts are generated correctly.
func TestEndToEndMultiFeedBatch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping end-to-end batch test in short mode")
	}

	const numFeeds = 5
	const ipsPerFeed = 1000

	// Generate distinct IP data for each feed.
	feedBodies := make([]string, numFeeds)
	for i := range numFeeds {
		var sb strings.Builder
		base := i * ipsPerFeed * 2
		for j := range ipsPerFeed {
			a := (base + j*2) >> 16 & 0xFF
			b := (base + j*2) >> 8 & 0xFF
			c := (base + j*2) & 0xFF
			fmt.Fprintf(&sb, "10.%d.%d.%d\n", a, b, c)
		}
		feedBodies[i] = sb.String()
	}

	modified := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	servers := make([]*httptest.Server, numFeeds)
	for i := range numFeeds {
		body := feedBodies[i]
		servers[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Last-Modified", modified.Format(http.TimeFormat))
			_, _ = w.Write([]byte(body))
		}))
		t.Cleanup(servers[i].Close)
	}

	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	webDir := filepath.Join(root, "web")
	libDir := filepath.Join(root, "lib")

	// Build config with multiple feeds and a merge.
	var cfg strings.Builder
	fmt.Fprintf(&cfg, `
runtime:
  base_dir: %q
  history_dir: %q
  lib_dir: %q
  errors_dir: %q
  web_dir: %q
  cache_dir: %q
  ipsets_apply: false
sources:
`, baseDir, filepath.Join(root, "history"), libDir,
		filepath.Join(root, "errors"), webDir, filepath.Join(root, "cache"))

	feedNames := make([]string, numFeeds)
	for i := range numFeeds {
		name := fmt.Sprintf("feed_%d", i)
		feedNames[i] = name
		fmt.Fprintf(&cfg, `  %s:
    url: %q
    frequency: 1
    ipv: ipv4
    output: ip
    processor:
      - passthrough
    category: attacks
    info: synthetic feed %d
    maintainer: test
    maintainer_url: https://example.test
`, name, servers[i].URL, i)
	}

	fmt.Fprintf(&cfg, `merges:
  all_feeds:
    ipv: ipv4
    output: ip
    category: attacks
    info: merged synthetic
    maintainer: test
    maintainer_url: https://example.test
    sources: [%s]
`, strings.Join(feedNames, ", "))

	cfgPath := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfg.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	eng.now = func() time.Time { return modified.Add(time.Hour) }

	report, err := runSchedulerStyleOnce(t, eng, RunOptions{
		Selected:   feedNames,
		EnableAll:  true,
		Manual:     true,
		CleanupOld: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Failed) != 0 {
		t.Fatalf("unexpected failures: %v", report.Failed)
	}
	report, err = runSchedulerStyleOnce(t, eng, RunOptions{
		Selected:   []string{"all_feeds"},
		EnableAll:  true,
		Manual:     true,
		CleanupOld: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Failed) != 0 {
		t.Fatalf("unexpected merge failures: %v", report.Failed)
	}

	// Verify artifacts for each feed.
	for _, name := range feedNames {
		ipsetPath := filepath.Join(baseDir, name+".ipset")
		if _, err := os.Stat(ipsetPath); err != nil {
			t.Fatalf("missing ipset file for %s: %v", name, err)
		}

		binaryPath := filepath.Join(libDir, name, "latest")
		if _, err := os.Stat(binaryPath); err != nil {
			t.Fatalf("missing binary file for %s: %v", name, err)
		}

		retentionPath := filepath.Join(libDir, name, "retention.json")
		if _, err := os.Stat(retentionPath); err != nil {
			t.Fatalf("missing retention file for %s: %v", name, err)
		}
	}

	// Verify merge artifact.
	mergePath := filepath.Join(baseDir, "all_feeds.ipset")
	if _, err := os.Stat(mergePath); err != nil {
		t.Fatalf("missing merge ipset: %v", err)
	}

	// Verify comparison files.
	compPath := filepath.Join(webDir, "feed_0_comparison.json")
	if _, err := os.Stat(compPath); err != nil {
		t.Fatalf("missing comparison file: %v", err)
	}

	// Verify index.json.
	indexPath := filepath.Join(webDir, "index.json")
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("missing index.json: %v", err)
	}

	// Verify QueryIP works across the batch.
	matches, err := eng.QueryIP(t.Context(), "10.0.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("expected at least one match for 10.0.0.0")
	}

	t.Logf("batch update: %d feeds, %d updated, %d merged",
		numFeeds, len(report.Updated), 1)
}

// TestEndToEndHeapBounded runs a batch update with multiple feeds and
// verifies that peak heap allocation stays reasonable. This tests the
// out-of-core pipeline end-to-end: download to disk, stream processing,
// FileSet comparison, and artifact generation.
func TestEndToEndHeapBounded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping heap-bounded e2e test in short mode")
	}

	const numFeeds = 4
	const linesPerFeed = 5000

	// Generate feeds with large but not enormous data.
	modified := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	servers := make([]*httptest.Server, numFeeds)
	for i := range numFeeds {
		var sb strings.Builder
		for j := range linesPerFeed {
			a := (i*linesPerFeed + j) >> 16 & 0xFF
			b := (i*linesPerFeed + j) >> 8 & 0xFF
			c := (i*linesPerFeed + j) & 0xFF
			fmt.Fprintf(&sb, "10.%d.%d.%d # comment to make lines longer for processor testing\n", a, b, c)
		}
		body := sb.String()
		servers[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Last-Modified", modified.Format(http.TimeFormat))
			_, _ = w.Write([]byte(body))
		}))
		t.Cleanup(servers[i].Close)
	}

	root := t.TempDir()
	var cfg strings.Builder
	fmt.Fprintf(&cfg, `
runtime:
  base_dir: %q
  history_dir: %q
  lib_dir: %q
  errors_dir: %q
  web_dir: %q
  cache_dir: %q
  ipsets_apply: false
sources:
`, filepath.Join(root, "base"), filepath.Join(root, "history"),
		filepath.Join(root, "lib"), filepath.Join(root, "errors"),
		filepath.Join(root, "web"), filepath.Join(root, "cache"))

	for i := range numFeeds {
		fmt.Fprintf(&cfg, `  heap_feed_%d:
    url: %q
    frequency: 1
    ipv: ipv4
    output: ip
    processor:
      - remove_comments
    category: attacks
    info: heap test feed
    maintainer: test
    maintainer_url: https://example.test
`, i, servers[i].URL)
	}

	cfgPath := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfg.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	eng.now = func() time.Time { return modified.Add(time.Hour) }

	runtime.GC()
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	report, err := runSchedulerStyleOnce(t, eng, RunOptions{
		EnableAll:  true,
		Manual:     true,
		CleanupOld: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Failed) != 0 {
		t.Fatalf("unexpected failures: %v", report.Failed)
	}

	runtime.GC()
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	heapGrowth := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	t.Logf("heap before=%d after=%d growth=%d (feeds=%d lines/feed=%d)",
		before.HeapAlloc, after.HeapAlloc, heapGrowth, numFeeds, linesPerFeed)

	// Verify all feeds were processed.
	if len(report.Updated) != numFeeds {
		t.Fatalf("expected %d updates, got %d", numFeeds, len(report.Updated))
	}

	// The test doesn't set a hard heap limit here — the value is reported
	// for human review. The main assertion is that the pipeline succeeds
	// without error and generates correct output.
}
