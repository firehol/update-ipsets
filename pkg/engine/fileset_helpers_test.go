package engine

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/iprange"
)

// newTestEngine creates a minimal Engine with a single source that
// serves the given body. It returns the engine and the root temp dir.
func newTestEngine(t *testing.T, body string) (*Engine, string) {
	t.Helper()

	modified := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", modified.Format(http.TimeFormat))
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

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
  alpha:
    url: %q
    frequency: 1
    ipv: ipv4
    output: ip
    processor:
      - passthrough
    category: attacks
    info: alpha feed
    maintainer: test
    maintainer_url: https://example.test
  beta:
    url: %q
    frequency: 1
    ipv: ipv4
    output: ip
    processor:
      - passthrough
    category: malware
    info: beta feed
    maintainer: test
    maintainer_url: https://example.test
`, filepath.Join(root, "base"), filepath.Join(root, "history"),
		filepath.Join(root, "lib"), filepath.Join(root, "errors"),
		filepath.Join(root, "web"), filepath.Join(root, "cache"),
		server.URL, server.URL)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	eng.now = func() time.Time { return modified.Add(time.Hour) }
	return eng, root
}

// runOnce runs the engine once with all sources enabled.
func runOnce(t *testing.T, eng *Engine) {
	t.Helper()
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
}

func TestQueryIPFallbackTextOnly(t *testing.T) {
	// After RunOnce, lib/{name}/latest is created by retention.
	// Delete it to force the text fallback path.
	eng, root := newTestEngine(t, "1.2.3.4\n5.6.7.0/30\n")
	runOnce(t, eng)

	// Remove the binary files to force text fallback.
	latestAlpha := filepath.Join(root, "lib", "alpha", "latest")
	latestBeta := filepath.Join(root, "lib", "beta", "latest")
	if err := os.Remove(latestAlpha); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.Remove(latestBeta); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	matches, err := eng.QueryIP(t.Context(), "5.6.7.2")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("expected at least one match via text fallback")
	}
	found := false
	for _, m := range matches {
		if m.Name == "alpha" || m.Name == "beta" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected alpha or beta in matches, got %v", matches)
	}
}

func TestQueryIPWithBinarySet(t *testing.T) {
	eng, _ := newTestEngine(t, "1.2.3.4\n5.6.7.0/30\n")
	runOnce(t, eng)

	// Binary .set files should exist after RunOnce (written by retention).
	matches, err := eng.QueryIP(t.Context(), "5.6.7.2")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("expected matches with binary .set files")
	}
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		names = append(names, m.Name)
	}
	got := strings.Join(names, ",")
	if !strings.Contains(got, "alpha") {
		t.Fatalf("expected alpha in matches, got %v", names)
	}
}

func TestQueryIPNoMatch(t *testing.T) {
	eng, _ := newTestEngine(t, "1.2.3.4\n")
	runOnce(t, eng)

	matches, err := eng.QueryIP(t.Context(), "9.9.9.9")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected no matches, got %v", matches)
	}
}

func TestQueryFeedIPScopedMatch(t *testing.T) {
	eng, _ := newTestEngine(t, "1.2.3.4\n5.6.7.0/30\n")
	runOnce(t, eng)

	match, found, err := eng.QueryFeedIP(t.Context(), "alpha", "5.6.7.2")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected scoped match")
	}
	if match == nil || match.Name != "alpha" {
		t.Fatalf("unexpected scoped match: %+v", match)
	}
	if match.Provenance == "" {
		t.Fatalf("expected provenance in scoped match: %+v", match)
	}
}

func TestQueryFeedIPScopedMiss(t *testing.T) {
	eng, _ := newTestEngine(t, "1.2.3.4\n")
	runOnce(t, eng)

	match, found, err := eng.QueryFeedIP(t.Context(), "alpha", "9.9.9.9")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatalf("expected miss, got %+v", match)
	}
	if match == nil || match.Name != "alpha" {
		t.Fatalf("expected feed metadata even on miss, got %+v", match)
	}
}

func TestQueryFeedIPInvalidatesCachedLatestAfterFinalize(t *testing.T) {
	var (
		bodyMu   sync.RWMutex
		bodyText = "1.2.3.4\n"
		modified = time.Date(2026, 4, 23, 10, 0, 0, 0, time.UTC)
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyMu.RLock()
		defer bodyMu.RUnlock()
		w.Header().Set("Last-Modified", modified.Format(http.TimeFormat))
		_, _ = w.Write([]byte(bodyText))
	}))
	t.Cleanup(server.Close)

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
  alpha:
    url: %q
    frequency: 1
    ipv: ipv4
    output: ip
    processor:
      - passthrough
    category: attacks
    info: alpha feed
    maintainer: test
    maintainer_url: https://example.test
`, filepath.Join(root, "base"), filepath.Join(root, "history"),
		filepath.Join(root, "lib"), filepath.Join(root, "errors"),
		filepath.Join(root, "web"), filepath.Join(root, "cache"),
		server.URL)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	runOnce(t, eng)

	if _, found, err := eng.QueryFeedIP(t.Context(), "alpha", "1.2.3.4"); err != nil || !found {
		t.Fatalf("expected initial cached match, found=%v err=%v", found, err)
	}

	bodyMu.Lock()
	bodyText = "9.9.9.9\n"
	modified = modified.Add(time.Hour)
	bodyMu.Unlock()
	runOnce(t, eng)

	if _, found, err := eng.QueryFeedIP(t.Context(), "alpha", "1.2.3.4"); err != nil {
		t.Fatal(err)
	} else if found {
		t.Fatal("expected old IP to disappear after finalize invalidated the cached latest set")
	}

	if _, found, err := eng.QueryFeedIP(t.Context(), "alpha", "9.9.9.9"); err != nil {
		t.Fatal(err)
	} else if !found {
		t.Fatal("expected refreshed latest set to match the new IP")
	}
}

func TestCompareSetWithBinaryFiles(t *testing.T) {
	eng, _ := newTestEngine(t, "1.2.3.4\n5.6.7.0/30\n")
	runOnce(t, eng)

	rows, err := eng.CompareSet(t.Context(), "alpha")
	if err != nil {
		t.Fatal(err)
	}
	// alpha and beta have identical data, so there should be 100% overlap.
	found := false
	for _, row := range rows {
		if row.Name == "beta" {
			found = true
			if row.Common == 0 {
				t.Fatalf("expected non-zero common IPs between identical sets, got %d", row.Common)
			}
			if row.Related {
				t.Fatalf("expected independent peer beta not to be marked related: %+v", row)
			}
		}
	}
	if !found {
		t.Fatalf("expected beta in comparison results, got %v", rows)
	}
}

func TestComposeWithBinaryFiles(t *testing.T) {
	eng, _ := newTestEngine(t, "1.2.3.4\n5.6.7.0/30\n")
	runOnce(t, eng)

	data, err := eng.Compose(t.Context(), []string{"alpha", "beta"}, nil, "cidr")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "1.2.3.4") {
		t.Fatalf("expected 1.2.3.4 in composed output, got %s", text)
	}
}

func TestComposeWithExclude(t *testing.T) {
	eng, _ := newTestEngine(t, "1.2.3.4\n5.6.7.0/30\n")
	runOnce(t, eng)

	// Exclude beta (which has the same data) — the result should be empty.
	data, err := eng.Compose(t.Context(), []string{"alpha"}, []string{"beta"}, "cidr")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.TrimSpace(string(data))
	if text != "" {
		t.Fatalf("expected empty compose when excluding identical set, got %q", text)
	}
}

func TestComposeRejectsUnknownSetWithoutCreatingCacheEntry(t *testing.T) {
	eng, _ := newTestEngine(t, "1.2.3.4\n")
	runOnce(t, eng)

	if _, err := eng.Compose(t.Context(), []string{"../missing"}, nil, "cidr"); err == nil {
		t.Fatal("expected compose to reject unknown set")
	}
	if entry := eng.state.EntrySnapshot("../missing"); entry != nil {
		t.Fatalf("unknown compose input created cache entry: %+v", entry)
	}
}

func TestOpenLatestSetBinaryPath(t *testing.T) {
	eng, root := newTestEngine(t, "1.2.3.4\n5.6.7.0/30\n")
	runOnce(t, eng)

	// Verify the binary latest file exists.
	latestPath := filepath.Join(root, "lib", "alpha", "latest")
	if _, err := os.Stat(latestPath); err != nil {
		t.Fatalf("expected latest to exist: %v", err)
	}

	src, err := eng.openLatestSet(t.Context(), "alpha")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Close() }()

	if src.UniqueIPs() == 0 {
		t.Fatal("expected non-zero unique IPs from binary set")
	}
	// Check Contains works
	ipv4, _ := iprange.ParseIPv4Token("1.2.3.4")
	if !src.Contains(ipv4) {
		t.Fatal("expected Contains to return true for 1.2.3.4")
	}
	ipNotIn, _ := iprange.ParseIPv4Token("9.9.9.9")
	if src.Contains(ipNotIn) {
		t.Fatal("expected Contains to return false for 9.9.9.9")
	}
}

func TestOpenLatestSetTextFallback(t *testing.T) {
	eng, root := newTestEngine(t, "1.2.3.4\n5.6.7.0/30\n")
	runOnce(t, eng)

	// Remove the binary file to test text fallback.
	latestPath := filepath.Join(root, "lib", "alpha", "latest")
	if err := os.Remove(latestPath); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	src, err := eng.openLatestSet(t.Context(), "alpha")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Close() }()

	if src.UniqueIPs() == 0 {
		t.Fatal("expected non-zero unique IPs from text fallback")
	}
}

func TestClosableSourceFromIPSet(t *testing.T) {
	set := iprange.New("test")
	_ = set.Add(10, 20)
	_ = set.Add(100, 200)
	set.Optimize()

	src := &closableSource{RangeSource: set}
	defer func() { _ = src.Close() }()

	if src.UniqueIPs() != set.UniqueCount() {
		t.Fatalf("UniqueIPs mismatch: got %d, want %d", src.UniqueIPs(), set.UniqueCount())
	}
	if !src.Contains(15) {
		t.Fatal("expected Contains(15) to be true")
	}
	if src.Contains(50) {
		t.Fatal("expected Contains(50) to be false")
	}
}

func TestCollectIter(t *testing.T) {
	set := iprange.New("source")
	_ = set.Add(10, 20)
	_ = set.Add(30, 40)
	set.Optimize()

	collected, err := collectIter(t.Context(), "result", set.Iter())
	if err != nil {
		t.Fatal(err)
	}
	if collected.UniqueCount() != set.UniqueCount() {
		t.Fatalf("collected unique IPs %d != source %d", collected.UniqueCount(), set.UniqueCount())
	}
	if collected.Entries() != set.Entries() {
		t.Fatalf("collected entries %d != source %d", collected.Entries(), set.Entries())
	}
}

func TestCollectIterWithUnion(t *testing.T) {
	setA := iprange.New("a")
	_ = setA.Add(10, 20)
	setA.Optimize()

	setB := iprange.New("b")
	_ = setB.Add(15, 30)
	setB.Optimize()

	unionIter := iprange.UnionIter(setA, setB)
	result, err := collectIter(t.Context(), "union", unionIter)
	if err != nil {
		t.Fatal(err)
	}

	// Union of [10,20] and [15,30] should be [10,30] = 21 IPs.
	if result.UniqueCount() != 21 {
		t.Fatalf("expected 21 unique IPs in union, got %d", result.UniqueCount())
	}
}

func TestWriteComparisonFilesWithFileSets(t *testing.T) {
	eng, root := newTestEngine(t, "1.2.3.4\n5.6.7.0/30\n")
	runOnce(t, eng)

	// The comparison file should have been generated during RunOnce.
	compPath := filepath.Join(root, "web", "alpha_comparison.json")
	if _, err := os.Stat(compPath); err != nil {
		t.Fatalf("expected comparison file: %v", err)
	}
	data, err := os.ReadFile(compPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"name": "beta"`) {
		t.Fatalf("expected beta in alpha's comparison file: %s", data)
	}
}

func TestOpenLatestSetEmptyBinaryFile(t *testing.T) {
	eng, root := newTestEngine(t, "1.2.3.4\n")
	runOnce(t, eng)

	// Overwrite latest with an empty file (represents empty set).
	latestPath := filepath.Join(root, "lib", "alpha", "latest")
	if err := os.WriteFile(latestPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	src, err := eng.openLatestSet(t.Context(), "alpha")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Close() }()

	if src.UniqueIPs() != 0 {
		t.Fatalf("expected 0 unique IPs from empty binary, got %d", src.UniqueIPs())
	}
}

func TestOpenLatestSetCorruptBinaryFallsBackToText(t *testing.T) {
	eng, root := newTestEngine(t, "1.2.3.4\n")
	runOnce(t, eng)

	// Overwrite latest with garbage to force fallback to text.
	latestPath := filepath.Join(root, "lib", "alpha", "latest")
	if err := os.WriteFile(latestPath, []byte("garbage data"), 0o644); err != nil {
		t.Fatal(err)
	}

	src, err := eng.openLatestSet(t.Context(), "alpha")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Close() }()

	if src.UniqueIPs() == 0 {
		t.Fatal("expected non-zero unique IPs from text fallback after corrupt binary")
	}
}

func TestComposeWithTextFallback(t *testing.T) {
	eng, root := newTestEngine(t, "1.2.3.4\n5.6.7.0/30\n")
	runOnce(t, eng)

	// Remove binary files to force text fallback in Compose.
	for _, name := range []string{"alpha", "beta"} {
		latestPath := filepath.Join(root, "lib", name, "latest")
		_ = os.Remove(latestPath)
	}

	data, err := eng.Compose(t.Context(), []string{"alpha"}, nil, "cidr")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "1.2.3.4") {
		t.Fatalf("expected 1.2.3.4 in composed output via text fallback")
	}
}

func TestClosableSourceIterMatchesIPSet(t *testing.T) {
	// Create a binary .set file and open it via OpenFileSet, then
	// verify that iterating produces the same ranges.
	set := iprange.New("test")
	_ = set.Add(10, 20)
	_ = set.Add(100, 200)
	_ = set.Add(1000, 2000)
	set.Optimize()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.set")
	var buf bytes.Buffer
	if err := iprange.WriteBinary(&buf, set); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	fs, err := iprange.OpenFileSet(path)
	if err != nil {
		t.Fatal(err)
	}
	src := &closableSource{RangeSource: fs, close: fs.Close}
	defer func() { _ = src.Close() }()

	// Count via iterator.
	collected, err := collectIter(t.Context(), "collected", fs.Iter())
	if err != nil {
		t.Fatal(err)
	}
	if collected.UniqueCount() != set.UniqueCount() {
		t.Fatalf("iterator unique IPs %d != original %d", collected.UniqueCount(), set.UniqueCount())
	}
}
