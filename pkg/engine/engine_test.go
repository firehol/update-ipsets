package engine

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunOnceAndQuery(t *testing.T) {
	modified := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", modified.Format(http.TimeFormat))
		_, _ = w.Write([]byte("# comment\n1.2.3.4\n5.6.7.0/30\n"))
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
    history: [60]
    ipv: ipv4
    output: ip
    processor:
      - remove_comments
    category: attacks
    info: sample feed
    maintainer: test
    maintainer_url: https://example.test
merges:
  merged:
    ipv: ipv4
    output: ip
    category: attacks
    info: merged feed
    maintainer: test
    maintainer_url: https://example.test
    sources: [sample]
`, baseDir, filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), webDir, filepath.Join(root, "cache"), server.URL)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	eng.now = func() time.Time { return modified.Add(time.Hour) }

	report, err := runSchedulerStyleOnce(t, eng, RunOptions{
		Selected:   []string{"sample"},
		EnableAll:  true,
		Manual:     true,
		CleanupOld: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Failed) != 0 {
		t.Fatalf("unexpected failures: %#v", report)
	}
	report, err = runSchedulerStyleOnce(t, eng, RunOptions{
		Selected:   []string{"merged"},
		EnableAll:  true,
		Manual:     true,
		CleanupOld: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Failed) != 0 {
		t.Fatalf("unexpected merge failures: %#v", report)
	}

	for _, path := range []string{
		filepath.Join(baseDir, "sample.ipset"),
		filepath.Join(baseDir, "sample_1h.ipset"),
		filepath.Join(baseDir, "merged.ipset"),
		filepath.Join(webDir, "index.json"),
		filepath.Join(root, "lib", "sample", "retention.json"),
		filepath.Join(root, "lib", "sample", "changesets.csv"),
		filepath.Join(root, "lib", "sample", "history.csv"),
		filepath.Join(webDir, "sample_history.csv"),
		filepath.Join(webDir, "sample_changesets.csv"),
		filepath.Join(webDir, "sample_retention.json"),
		filepath.Join(webDir, "sample_insights.json"),
		filepath.Join(webDir, "merged_insights.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected generated file %s: %v", path, err)
		}
	}
	for _, path := range []string{
		filepath.Join(baseDir, "sample.setinfo"),
		filepath.Join(baseDir, "README.md"),
		filepath.Join(baseDir, ".gitignore"),
		filepath.Join(baseDir, "set_file_timestamps.sh"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected no git support file without base .git dir at %s, got err=%v", path, err)
		}
	}
	changesets, err := os.ReadFile(filepath.Join(root, "lib", "sample", "changesets.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(changesets), "DateTime,IPsAdded,IPsRemoved\n") {
		t.Fatalf("unexpected changesets header: %s", changesets)
	}
	olderSeen := modified.Add(-30 * time.Minute)
	writeSnapshotForTest(t, filepath.Join(root, "history"), "sample", modified.Add(-15*time.Minute), "5.6.7.0/30")
	writeRetentionCohortForTest(t, filepath.Join(root, "lib"), "sample", olderSeen, "5.6.7.0/30")
	resetRetentionCohortCacheForTest(eng, "sample")

	matches, err := eng.QueryIP(t.Context(), "5.6.7.2")
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(matches))
	timings := make(map[string]QueryMatch, len(matches))
	for _, match := range matches {
		names = append(names, match.Name)
		timings[match.Name] = match
	}
	got := strings.Join(names, ",")
	if !strings.Contains(got, "sample") || !strings.Contains(got, "merged") {
		t.Fatalf("unexpected matches: %v", names)
	}
	if match := timings["sample"]; match.FirstSeen != olderSeen.Unix() || match.LastSeen != modified.Unix() {
		t.Fatalf("unexpected sample timing: %+v", match)
	}
	if match := timings["merged"]; match.LastSeen != modified.Unix() {
		t.Fatalf("unexpected merged last_seen: %+v", match)
	}
}

func TestQueryIPFirstSeenUsesRetentionCohortsNotDownloaderHistory(t *testing.T) {
	eng, root := newTestEngine(t, "5.6.7.0/30\n")
	runOnce(t, eng)

	oldestSeen := time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)
	middleSeen := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	writeSnapshotForTest(t, filepath.Join(root, "history"), "alpha", middleSeen, "5.6.7.0/30")
	writeRetentionCohortForTest(t, filepath.Join(root, "lib"), "alpha", oldestSeen, "5.6.7.0/30")
	resetRetentionCohortCacheForTest(eng, "alpha")

	match, found, err := eng.QueryFeedIP(t.Context(), "alpha", "5.6.7.2")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected scoped match")
	}
	if match.FirstSeen != oldestSeen.Unix() {
		t.Fatalf("expected oldest matching snapshot %d, got %+v", oldestSeen.Unix(), match)
	}
}

// TestSplitSourceDownloadsOnce was removed when the legacy
// bash-era `output: split` mode was dropped. Only ipset and
// netset are supported now.

func TestGeolocationCountryComparison(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	geoPath := filepath.Join(root, "ipdeny.tar.gz")

	var archive bytes.Buffer
	gw := gzip.NewWriter(&archive)
	tw := tar.NewWriter(gw)
	body := []byte("5.6.7.0/24\n")
	if err := tw.WriteHeader(&tar.Header{Name: "us.zone", Mode: 0o600, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(geoPath, archive.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

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
  ipdeny_country:
    url: https://example.test/ignored
    frequency: 1
    use: [geoip]
    format: ipdeny_country_tar_gz
    info: geo
    maintainer: geo
    maintainer_url: https://example.test
`, baseDir, filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"), httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("5.6.7.0/24\n"))
	})).URL)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	geoSrc := eng.Config().Sources["ipdeny_country"]
	geoSrc.URL = "https://example.test/ignored"
	geoSrc.Format = "ipdeny_country_tar_gz"
	geoSrc.Maintainer = "geo"
	geoSrc.MaintainerURL = "https://example.test"
	entry := eng.state.Entry("ipdeny_country")
	entry.Downloader = "copyfile"
	entry.DownloaderOptions = geoPath
	geoSrc.URL = "copy://local"
	geoSrc.Downloader = "copyfile"
	geoSrc.DownloaderOptions = geoPath

	if _, err := runSchedulerStyleOnce(t, eng, RunOptions{
		EnableAll:  true,
		Manual:     true,
		CleanupOld: true,
	}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "web", "sample_ipdeny_country.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"code": "US"`) {
		t.Fatalf("expected US country comparison in %s", data)
	}
}

func TestRunOnceUsesStagedSourceFile(t *testing.T) {
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
    url: https://example.test/list.txt
    frequency: 60
    ipv: ipv4
    output: ipset
    processor:
      - passthrough
    category: attacks
    info: sample feed
    maintainer: test
    maintainer_url: https://example.test
`, baseDir, filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		t.Fatal(err)
	}
	finalBody := filepath.Join(baseDir, "sample.ipset")
	stagedBody := finalBody + ".new"
	if err := os.WriteFile(finalBody, []byte("1.1.1.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stagedBody, []byte("2.2.2.2\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	report, err := eng.RunOnce(t.Context(), RunOptions{
		Selected:   []string{"sample"},
		EnableAll:  true,
		Manual:     true,
		CleanupOld: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Updated) != 1 || report.Updated[0] != "sample" {
		t.Fatalf("expected sample to update from staged source, got %#v", report.Updated)
	}
	rendered, err := os.ReadFile(filepath.Join(baseDir, "sample.ipset"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rendered), "2.2.2.2\n") {
		t.Fatalf("expected final ipset from staged source, got %q", string(rendered))
	}
	if entry := eng.state.Entry("sample"); entry.CheckedDate != 0 {
		t.Fatalf("expected processing-only run to preserve checked_date, got %d", entry.CheckedDate)
	}
}

func TestRunOnceBeforePublishHookRunsBeforePublication(t *testing.T) {
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
    url: https://example.test/list.txt
    frequency: 60
    ipv: ipv4
    output: ipset
    processor:
      - passthrough
    category: attacks
    info: sample feed
    maintainer: test
    maintainer_url: https://example.test
`, baseDir, filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), webDir, filepath.Join(root, "cache"))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		t.Fatal(err)
	}
	stagedBody := filepath.Join(baseDir, "sample.ipset.new")
	if err := os.WriteFile(stagedBody, []byte("2.2.2.2\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	beforePublishCalled := false
	report, err := eng.RunOnce(t.Context(), RunOptions{
		Selected:   []string{"sample"},
		EnableAll:  true,
		Manual:     true,
		CleanupOld: true,
		BeforePublish: func(report *Report) error {
			beforePublishCalled = true
			if _, err := os.Stat(filepath.Join(webDir, "index.json")); !os.IsNotExist(err) {
				t.Fatalf("expected publication to start after before-publish hook, got err=%v", err)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !beforePublishCalled {
		t.Fatal("expected before-publish hook to be called")
	}
	if len(report.Updated) != 1 || report.Updated[0] != "sample" {
		t.Fatalf("expected sample update, got %#v", report.Updated)
	}
	if _, err := os.Stat(filepath.Join(baseDir, "sample.ipset")); err != nil {
		t.Fatalf("expected committed canonical feed body after processing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(webDir, "index.json")); err != nil {
		t.Fatalf("expected publication after before-publish hook: %v", err)
	}
}

func TestRunOnceRebuildsInternalHistorySourceWhenLocalSourceMissing(t *testing.T) {
	modified := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
	observed := modified.Add(30 * time.Minute)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", modified.Format(http.TimeFormat))
		_, _ = w.Write([]byte("1.2.3.4\n1.2.3.0/30\n"))
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
    history: [60]
    ipv: ipv4
    output: ipset
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
	eng.now = func() time.Time { return observed }
	if _, err := runSchedulerStyleOnce(t, eng, RunOptions{
		EnableAll:  true,
		Manual:     true,
		CleanupOld: true,
	}); err != nil {
		t.Fatal(err)
	}

	historySourcePath := filepath.Join(baseDir, "sample_1h.ipset")
	if err := os.Remove(historySourcePath); err != nil {
		t.Fatal(err)
	}

	report, err := runSchedulerStyleOnce(t, eng, RunOptions{
		Selected:   []string{"sample_1h"},
		EnableAll:  true,
		Manual:     true,
		Reprocess:  true,
		CleanupOld: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Updated) != 1 || report.Updated[0] != "sample_1h" {
		t.Fatalf("expected sample_1h to rebuild from internal provider, got %#v", report.Updated)
	}
	if _, err := os.Stat(historySourcePath); err != nil {
		t.Fatalf("expected rebuilt history source at %s: %v", historySourcePath, err)
	}
	rendered, err := os.ReadFile(filepath.Join(baseDir, "sample_1h.ipset"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rendered), "1.2.3.4") {
		t.Fatalf("expected rebuilt history ipset to contain parent data, got %q", string(rendered))
	}
}

func TestResolveRecheckTargetFallsBackToParentWhenHistoryRollupsMissing(t *testing.T) {
	modified := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
	observed := modified.Add(30 * time.Minute)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", modified.Format(http.TimeFormat))
		_, _ = w.Write([]byte("1.2.3.4\n1.2.3.0/30\n"))
	}))
	defer server.Close()

	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	historyDir := filepath.Join(root, "history")
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
    history: [60]
    ipv: ipv4
    output: ipset
    processor:
      - passthrough
    category: attacks
    info: sample feed
    maintainer: test
    maintainer_url: https://example.test
`, baseDir, historyDir, filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"), server.URL)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	eng.now = func() time.Time { return observed }
	if _, err := runSchedulerStyleOnce(t, eng, RunOptions{
		EnableAll:  true,
		Manual:     true,
		CleanupOld: true,
	}); err != nil {
		t.Fatal(err)
	}

	snapshots, err := filepath.Glob(filepath.Join(historyDir, "sample", "*.set"))
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) == 0 {
		t.Fatal("expected history snapshots to exist")
	}
	if err := os.WriteFile(snapshots[0], []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}

	if got := eng.ResolveRecheckTarget(t.Context(), "sample_1h"); got != "sample" {
		t.Fatalf("expected derivative recheck to target parent, got %q", got)
	}
}

func TestFullFeedReprocessTargetsIncludeHiddenFeeds(t *testing.T) {
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
  visible:
    url: https://example.test/visible.txt
    frequency: 60
    ipv: ipv4
    output: ipset
    processor:
      - passthrough
    category: attacks
    info: visible
    maintainer: test
    maintainer_url: https://example.test
  hidden_feed:
    url: https://example.test/hidden.txt
    frequency: 60
    hidden: true
    ipv: ipv4
    output: ipset
    processor:
      - passthrough
    category: attacks
    info: hidden
    maintainer: test
    maintainer_url: https://example.test
  asn_db:
    url: https://example.test/asn.tsv
    frequency: 1440
    use: [asn]
    format: iptoasn_combined_tsv
    info: asn
    maintainer: test
    maintainer_url: https://example.test
`, baseDir, filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"visible", "hidden_feed"} {
		if err := os.WriteFile(filepath.Join(baseDir, name+".ipset"), []byte("1.2.3.4\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got := eng.FullFeedReprocessTargets(true)
	want := map[string]bool{
		"visible":     true,
		"hidden_feed": true,
	}
	if len(got) != len(want) {
		t.Fatalf("unexpected reprocess targets: got %v want %v", got, want)
	}
	for _, name := range got {
		if !want[name] {
			t.Fatalf("unexpected reprocess target %q in %v", name, got)
		}
		delete(want, name)
	}
	if len(want) != 0 {
		t.Fatalf("missing reprocess targets: %v", want)
	}
}

func TestOutputArtifactsRespectDontRedistribute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("9.9.9.9\n"))
	}))
	defer server.Close()

	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	webDir := filepath.Join(root, "web")
	if err := os.MkdirAll(filepath.Join(baseDir, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
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
  private_feed:
    url: %q
    frequency: 1
    ipv: ipv4
    output: ip
    processor:
      - passthrough
    category: tests
    info: private feed
    maintainer: test
    maintainer_url: https://example.test
    redistributable: false
`, baseDir, filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), webDir, filepath.Join(root, "cache"), server.URL)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runSchedulerStyleOnce(t, eng, RunOptions{EnableAll: true, Manual: true, CleanupOld: true}); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{
		filepath.Join(baseDir, "README.md"),
		filepath.Join(baseDir, ".gitignore"),
		filepath.Join(baseDir, "set_file_timestamps.sh"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected artifact %s: %v", path, err)
		}
	}

	data, err := os.ReadFile(filepath.Join(webDir, "private_feed.json"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	// Fields are always present (no omitempty, matching bash), but for
	// non-redistributable sets they must be empty strings.
	if !strings.Contains(text, `"source": ""`) {
		t.Fatalf("non-redistributable set should have empty source, got: %s", text)
	}
	if !strings.Contains(text, `"file_local": ""`) {
		t.Fatalf("non-redistributable set should have empty file_local, got: %s", text)
	}
	if !strings.Contains(text, `"commit_history": ""`) {
		t.Fatalf("non-redistributable set should have empty commit_history, got: %s", text)
	}
	ignoreData, err := os.ReadFile(filepath.Join(baseDir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ignoreData), "private_feed.ipset") {
		t.Fatalf("expected private feed in .gitignore: %s", string(ignoreData))
	}
}

func TestHistoryUsesSourceTimestamp(t *testing.T) {
	modified := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
	observed := modified.Add(30 * time.Minute)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", modified.Format(http.TimeFormat))
		_, _ = w.Write([]byte("1.2.3.4\n1.2.3.0/30\n"))
	}))
	defer server.Close()

	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	historyDir := filepath.Join(root, "history")
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
    history: [60]
    ipv: ipv4
    output: ip
    processor:
      - passthrough
    category: tests
    info: history source
    maintainer: test
    maintainer_url: https://example.test
`, baseDir, historyDir, filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"), server.URL)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	eng.now = func() time.Time { return observed }

	if _, err := runSchedulerStyleOnce(t, eng, RunOptions{EnableAll: true, Manual: true, CleanupOld: true}); err != nil {
		t.Fatal(err)
	}

	files, err := os.ReadDir(filepath.Join(historyDir, "sample"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 history snapshot, got %d", len(files))
	}
	if got, want := files[0].Name(), fmt.Sprintf("%d.set", modified.Unix()); got != want {
		t.Fatalf("unexpected history snapshot name: got %q want %q", got, want)
	}
	points, err := eng.HistorySeries("sample")
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 || points[0].Timestamp != modified.Unix() || points[0].UniqueIPs == 0 {
		t.Fatalf("expected non-empty history series, got %#v", points)
	}

	data, err := os.ReadFile(filepath.Join(baseDir, "sample_1h.ipset"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "1.2.3.4") {
		t.Fatalf("expected aggregated history variant to retain observed data, got %s", data)
	}
}

func TestRebuildContinuesAfterNotModified(t *testing.T) {
	modified := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("If-Modified-Since") != "" {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Last-Modified", modified.Format(http.TimeFormat))
		_, _ = w.Write([]byte("1.2.3.4\n1.2.3.0/30\n"))
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
    history: [60]
    ipv: ipv4
    output: ip
    processor:
      - passthrough
    category: tests
    info: rebuild source
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
	observed := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return observed }
	if _, err := runSchedulerStyleOnce(t, eng, RunOptions{EnableAll: true, Manual: true, CleanupOld: true}); err != nil {
		t.Fatal(err)
	}

	eng.now = func() time.Time { return observed.Add(2 * time.Minute) }
	report, err := runSchedulerStyleOnce(t, eng, RunOptions{Selected: []string{"sample"}, EnableAll: true, Reprocess: true, CleanupOld: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Updated) == 0 || report.Updated[0] != "sample" {
		t.Fatalf("expected rebuild to reprocess sample after 304, got %#v", report)
	}
	if requests < 2 {
		t.Fatalf("expected conditional revalidation request, got %d requests", requests)
	}

	changesets, err := os.ReadFile(filepath.Join(root, "lib", "sample", "changesets.csv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(changesets)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected rebuild to leave one real changeset row, got %d rows: %s", len(lines)-1, changesets)
	}
	if strings.Contains(string(changesets), ",0,0") {
		t.Fatalf("expected no zero-delta changeset rows after identical rebuild, got %s", changesets)
	}
}

func TestRunOnceGeneratesHeaderWithRawURL(t *testing.T) {
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
    url: "%s?token=${TEST_API_KEY}"
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

	_, err = runSchedulerStyleOnce(t, eng, RunOptions{Selected: []string{"sample"}, EnableAll: true, Manual: true, CleanupOld: true})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(baseDir, "sample.ipset"))
	if err != nil {
		t.Fatal(err)
	}

	// Verify the file starts with a header.
	if !bytes.HasPrefix(data, []byte("#\n# sample\n")) {
		t.Fatalf("expected header at start of file, got:\n%s", data)
	}

	// Verify the raw URL (with ${TEST_API_KEY}) appears in the header, not an expanded value.
	if !bytes.Contains(data, []byte("?token=${TEST_API_KEY}")) {
		t.Fatalf("expected raw URL with ${TEST_API_KEY} in header, got:\n%s", data)
	}

	// Verify no real token value appears (server.URL does not contain a real token).
	if bytes.Contains(data, []byte("?token=real")) {
		t.Fatalf("unexpected expanded token in header:\n%s", data)
	}
}

func TestRunOnceGeneratesHeadersForMergeAndHistoryDerivative(t *testing.T) {
	modified := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", modified.Format(http.TimeFormat))
		_, _ = w.Write([]byte("1.2.3.4\n5.6.7.8\n"))
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
    history: [60]
    ipv: ipv4
    output: ip
    processor:
      - passthrough
    category: attacks
    info: sample feed
    maintainer: test
    maintainer_url: https://example.test
merges:
  merged:
    ipv: ipv4
    output: ip
    category: attacks
    info: merged feed
    maintainer: test
    maintainer_url: https://example.test
    sources: [sample]
`, baseDir, filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"), server.URL)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	eng.now = func() time.Time { return modified.Add(time.Hour) }

	_, err = runSchedulerStyleOnce(t, eng, RunOptions{Selected: []string{"sample"}, EnableAll: true, Manual: true, CleanupOld: true})
	if err != nil {
		t.Fatal(err)
	}

	// Run merge after parent exists.
	_, err = runSchedulerStyleOnce(t, eng, RunOptions{Selected: []string{"merged"}, EnableAll: true, Manual: true, CleanupOld: true})
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name   string
		expect string
	}{
		{"sample", "#\n# sample\n"},
		{"sample_1h", "#\n# sample_1h\n"},
		{"merged", "#\n# merged\n"},
	} {
		path := filepath.Join(baseDir, tc.name+".ipset")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", tc.name, err)
		}
		if !bytes.HasPrefix(data, []byte(tc.expect)) {
			t.Fatalf("expected header for %s, got:\n%s", tc.name, data)
		}
		// Verify body is present after the header.
		if !bytes.Contains(data, []byte("1.2.3.4")) {
			t.Fatalf("expected body IP in %s, got:\n%s", tc.name, data)
		}
	}

	// Verify history derivative has the correct aggregation.
	histData, err := os.ReadFile(filepath.Join(baseDir, "sample_1h.ipset"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(histData, []byte("Aggregation     : 1 hour ")) {
		t.Fatalf("expected 'Aggregation: 1 hour' in sample_1h header, got:\n%s", histData)
	}
}
