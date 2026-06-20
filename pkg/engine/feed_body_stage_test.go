package engine

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

func TestAppendHistorySnapshotSameTimestampNoChangeReturnsFalse(t *testing.T) {
	root := t.TempDir()
	set, err := parseFeedBodyBytes(t.Context(), "sample", []byte("1.2.3.4\n"), 1)
	if err != nil {
		t.Fatal(err)
	}
	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.HistoryDir = filepath.Join(root, "history")
	}))
	eng.retentionMaxWindow = map[string]time.Duration{"sample": 7 * 24 * time.Hour}
	observedAt := time.Date(2026, 4, 21, 9, 0, 0, 0, time.UTC)

	changed, err := eng.appendHistorySnapshot(t.Context(), "sample", set, observedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected first history snapshot write to report change")
	}

	changed, err = eng.appendHistorySnapshot(t.Context(), "sample", set, observedAt)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("expected identical snapshot update at the same timestamp to be a no-op")
	}

	changed, err = eng.appendHistorySnapshot(t.Context(), "sample", set, observedAt.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected a later successful observation to create a new history snapshot")
	}
	files, err := os.ReadDir(filepath.Join(root, "history", "sample"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 history snapshots, got %d", len(files))
	}
}

func TestComposeHistoryDerivativeUsesParentTimestampInsteadOfWallClock(t *testing.T) {
	root := t.TempDir()
	parentObserved := time.Date(2026, 4, 21, 9, 0, 0, 0, time.UTC)
	olderSnapshot := parentObserved.Add(-23 * time.Hour)

	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.BaseDir = filepath.Join(root, "base")
		rt.HistoryDir = filepath.Join(root, "history")
	}), withNow(func() time.Time { return parentObserved.Add(14 * 24 * time.Hour) }))
	eng.retentionMaxWindow = map[string]time.Duration{"sample": 48 * time.Hour}
	if err := os.MkdirAll(eng.runtime.BaseDir, 0o700); err != nil {
		t.Fatal(err)
	}

	parentBody := filepath.Join(eng.runtime.BaseDir, "sample.ipset")
	if err := os.WriteFile(parentBody, []byte("4.5.6.7\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(parentBody, parentObserved, parentObserved); err != nil {
		t.Fatal(err)
	}

	oldSet, err := parseFeedBodyBytes(t.Context(), "sample_old", []byte("1.2.3.4\n"), 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(eng.runtime.HistoryDir, "sample"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeBinaryPath(filepath.Join(eng.runtime.HistoryDir, "sample", fmt.Sprintf("%d.set", olderSnapshot.Unix())), oldSet, olderSnapshot); err != nil {
		t.Fatal(err)
	}

	src := &config.Source{
		Name:              "sample_2d",
		URL:               "internal://retention_window?parent=sample&minutes=2880",
		DerivedFrom:       []string{"sample"},
		Provenance:        config.ProvenanceSecondaryRetention,
		HistoryWindowDays: 2,
		Output:            "ipset",
	}
	body, _, err := eng.composeHistoryDerivativeBody(t.Context(), src)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "1.2.3.4") || !strings.Contains(text, "4.5.6.7") {
		t.Fatalf("expected derivative body to include current parent and in-window snapshot, got %q", text)
	}
}

func TestComposeMergeBodySubtractsConfiguredExclude(t *testing.T) {
	now := time.Date(2026, 4, 27, 9, 0, 0, 0, time.UTC)
	eng, merge := newMergeSubtractTestEngine(t, now)
	writeMergeParentBody(t, eng, "included", "10.0.0.0/24\n10.0.1.0/24\n192.0.2.1\n")
	writeMergeParentBody(t, eng, "subtracted", "10.0.0.0/25\n")
	markMergeParentHealthy(eng, "included", now)
	markMergeParentHealthy(eng, "subtracted", now)

	body, set, disabled, err := eng.composeMergeBody(t.Context(), merge, true)
	if err != nil {
		t.Fatal(err)
	}
	if disabled != "" {
		t.Fatalf("unexpected disabled message: %q", disabled)
	}
	if len(body) == 0 {
		t.Fatal("expected rendered merge body")
	}
	assertSetContains(t, set, "10.0.0.129")
	assertSetContains(t, set, "10.0.1.1")
	assertSetDoesNotContain(t, set, "10.0.0.1")
}

func TestComposeMergeBodySubtractsMultipleConfiguredExcludes(t *testing.T) {
	now := time.Date(2026, 4, 27, 9, 0, 0, 0, time.UTC)
	eng, merge := newMergeSubtractTestEngine(t, now)
	eng.cfg.Sources["subtracted_second"] = &config.Source{Name: "subtracted_second", Frequency: 60, IPV: "ipv4", Output: "netset"}
	merge.DerivedFrom = append(merge.DerivedFrom, "subtracted_second")
	merge.MergeExclude = append(merge.MergeExclude, "subtracted_second")
	writeMergeParentBody(t, eng, "included", "10.0.0.0/24\n192.0.2.0/24\n203.0.113.0/24\n")
	writeMergeParentBody(t, eng, "subtracted", "10.0.0.0/24\n")
	writeMergeParentBody(t, eng, "subtracted_second", "192.0.2.0/24\n")
	markMergeParentHealthy(eng, "included", now)
	markMergeParentHealthy(eng, "subtracted", now)
	markMergeParentHealthy(eng, "subtracted_second", now)

	_, set, disabled, err := eng.composeMergeBody(t.Context(), merge, true)
	if err != nil {
		t.Fatal(err)
	}
	if disabled != "" {
		t.Fatalf("unexpected disabled message: %q", disabled)
	}
	assertSetContains(t, set, "203.0.113.1")
	assertSetDoesNotContain(t, set, "10.0.0.1")
	assertSetDoesNotContain(t, set, "192.0.2.1")
}

func TestComposeMergeBodyAllowsEmptyConfiguredExclude(t *testing.T) {
	now := time.Date(2026, 4, 27, 9, 0, 0, 0, time.UTC)
	eng, merge := newMergeSubtractTestEngine(t, now)
	writeMergeParentBody(t, eng, "included", "10.0.0.0/24\n")
	writeMergeParentBody(t, eng, "subtracted", "")
	markMergeParentHealthy(eng, "included", now)
	markMergeParentHealthy(eng, "subtracted", now)

	_, set, disabled, err := eng.composeMergeBody(t.Context(), merge, true)
	if err != nil {
		t.Fatal(err)
	}
	if disabled != "" {
		t.Fatalf("unexpected disabled message: %q", disabled)
	}
	assertSetContains(t, set, "10.0.0.1")
	assertSetContains(t, set, "10.0.0.255")
}

func TestComposeMergeBodyReturnsDisabledWhenNoAdditiveParentsAreEligible(t *testing.T) {
	now := time.Date(2026, 4, 27, 9, 0, 0, 0, time.UTC)
	eng, merge := newMergeSubtractTestEngine(t, now)
	writeMergeParentBody(t, eng, "included", "10.0.0.0/24\n")
	writeMergeParentBody(t, eng, "subtracted", "10.0.0.0/25\n")
	markMergeParentHealthy(eng, "included", now)
	markMergeParentHealthy(eng, "subtracted", now)

	body, set, disabled, err := eng.composeMergeBody(t.Context(), merge, false)
	if err != nil {
		t.Fatal(err)
	}
	if disabled != "merge disabled: no currently eligible inputs" {
		t.Fatalf("disabled message = %q", disabled)
	}
	if body != nil || set != nil {
		t.Fatalf("expected no body/set for disabled merge, got body=%q set=%v", body, set)
	}
}

func TestComposeMergeBodyFailsWhenConfiguredExcludeBodyIsMissing(t *testing.T) {
	now := time.Date(2026, 4, 27, 9, 0, 0, 0, time.UTC)
	eng, merge := newMergeSubtractTestEngine(t, now)
	writeMergeParentBody(t, eng, "included", "10.0.0.0/24\n")
	markMergeParentHealthy(eng, "included", now)
	markMergeParentHealthy(eng, "subtracted", now)

	_, _, _, err := eng.composeMergeBody(t.Context(), merge, true)
	if err == nil {
		t.Fatal("expected missing subtractive input to fail the merge")
	}
	if !strings.Contains(err.Error(), "subtracted") {
		t.Fatalf("expected error to name missing subtractive input, got %v", err)
	}
}

func TestComposeMergeBodyFailsWhenConfiguredExcludeIsDisabled(t *testing.T) {
	now := time.Date(2026, 4, 27, 9, 0, 0, 0, time.UTC)
	eng, merge := newMergeSubtractTestEngine(t, now)
	writeMergeParentBody(t, eng, "included", "10.0.0.0/24\n")
	writeMergeParentBody(t, eng, "subtracted", "10.0.0.0/25\n")
	enableMergeParent(t, eng, "included")
	markMergeParentHealthy(eng, "included", now)
	markMergeParentHealthy(eng, "subtracted", now)

	_, _, _, err := eng.composeMergeBody(t.Context(), merge, false)
	if err == nil {
		t.Fatal("expected disabled subtractive input to fail the merge")
	}
	if !strings.Contains(err.Error(), "subtracted(disabled)") {
		t.Fatalf("expected error to name disabled subtractive input, got %v", err)
	}
}

func TestComposeMergeBodyFailsWhenConfiguredExcludeIsArchived(t *testing.T) {
	now := time.Date(2026, 4, 27, 9, 0, 0, 0, time.UTC)
	eng, merge := newMergeSubtractTestEngine(t, now)
	writeMergeParentBody(t, eng, "included", "10.0.0.0/24\n")
	writeMergeParentBody(t, eng, "subtracted", "10.0.0.0/25\n")
	markMergeParentHealthy(eng, "included", now)
	markMergeParentArchived(eng, "subtracted", now)

	_, _, _, err := eng.composeMergeBody(t.Context(), merge, true)
	if err == nil {
		t.Fatal("expected archived subtractive input to fail the merge")
	}
	if !strings.Contains(err.Error(), "subtracted(archived)") {
		t.Fatalf("expected error to name archived subtractive input, got %v", err)
	}
}

func TestComposeMergeBodyFailsWhenConfiguredExcludeIsUnmaintained(t *testing.T) {
	now := time.Date(2026, 4, 27, 9, 0, 0, 0, time.UTC)
	eng, merge := newMergeSubtractTestEngine(t, now)
	writeMergeParentBody(t, eng, "included", "10.0.0.0/24\n")
	writeMergeParentBody(t, eng, "subtracted", "10.0.0.0/25\n")
	markMergeParentHealthy(eng, "included", now)
	markMergeParentUnmaintained(eng, "subtracted", now)

	_, _, _, err := eng.composeMergeBody(t.Context(), merge, true)
	if err == nil {
		t.Fatal("expected unmaintained subtractive input to fail the merge")
	}
	if !strings.Contains(err.Error(), "subtracted(unmaintained)") {
		t.Fatalf("expected error to name unmaintained subtractive input, got %v", err)
	}
}

func newMergeSubtractTestEngine(t *testing.T, now time.Time) (*Engine, *config.Source) {
	t.Helper()
	baseDir := filepath.Join(t.TempDir(), "base")
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := config.New()
	cfg.Runtime.FeedHealthSingleObservationGraceMins = 60
	cfg.Runtime.FeedHealthDefaultHealthyCadenceMins = 60
	cfg.Runtime.FeedHealthDefaultRiskyCadenceMins = 60
	cfg.Runtime.FeedHealthArchivalThresholdMins = 600
	cfg.Sources["included"] = &config.Source{Name: "included", Frequency: 60, IPV: "ipv4", Output: "netset"}
	cfg.Sources["subtracted"] = &config.Source{Name: "subtracted", Frequency: 60, IPV: "ipv4", Output: "netset"}
	cfg.Sources["merged"] = &config.Source{
		Name:         "merged",
		Frequency:    60,
		IPV:          "ipv4",
		Output:       "netset",
		DerivedFrom:  []string{"included", "subtracted"},
		MergeSources: []string{"included"},
		MergeExclude: []string{"subtracted"},
		Provenance:   config.ProvenanceSecondaryMerge,
	}
	eng := newEngineFixture(t, withConfig(cfg), withRuntime(func(rt *Runtime) {
		rt.BaseDir = baseDir
	}), withNow(func() time.Time { return now }))
	return eng, cfg.Sources["merged"]
}

func enableMergeParent(t *testing.T, eng *Engine, name string) {
	t.Helper()
	if err := os.WriteFile(sourceEnablePathForRuntime(eng.runtime, name), nil, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeMergeParentBody(t *testing.T, eng *Engine, name, body string) {
	t.Helper()
	if err := os.WriteFile(eng.feedBodyPath(name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func markMergeParentHealthy(eng *Engine, name string, now time.Time) {
	entry := eng.state.Entry(name)
	entry.Name = name
	entry.ProcessedDate = now.Add(-10 * time.Minute).Unix()
	entry.SourceDate = now.Add(-10 * time.Minute).Unix()
	entry.CheckedDate = now.Unix()
	entry.Entries = 1
	entry.Version = 1
}

func markMergeParentArchived(eng *Engine, name string, now time.Time) {
	entry := eng.state.Entry(name)
	entry.Name = name
	entry.ProcessedDate = now.Add(-24 * time.Hour).Unix()
	entry.SourceDate = now.Add(-24 * time.Hour).Unix()
	entry.CheckedDate = now.Unix()
	entry.DownloadFailures = 5
	entry.FailureStartedDate = now.Add(-24 * time.Hour).Unix()
	entry.LastStatus = "download_failed"
	entry.Version = 1
}

func markMergeParentUnmaintained(eng *Engine, name string, now time.Time) {
	entry := eng.state.Entry(name)
	entry.Name = name
	entry.ProcessedDate = now.Add(-3 * time.Hour).Unix()
	entry.SourceDate = now.Add(-3 * time.Hour).Unix()
	entry.CheckedDate = now.Unix()
	entry.Entries = 1
	entry.Version = 3
}

func assertSetContains(t *testing.T, set *iprange.IPSet, ip string) {
	t.Helper()
	parsed, err := iprange.ParseIPv4Token(ip)
	if err != nil {
		t.Fatal(err)
	}
	if !set.Contains(parsed) {
		t.Fatalf("expected set to contain %s", ip)
	}
}

func assertSetDoesNotContain(t *testing.T, set *iprange.IPSet, ip string) {
	t.Helper()
	parsed, err := iprange.ParseIPv4Token(ip)
	if err != nil {
		t.Fatal(err)
	}
	if set.Contains(parsed) {
		t.Fatalf("expected set not to contain %s", ip)
	}
}

func TestFetchAndStageArtifactChildExtendsHistoryDerivativeDecision(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	cfg := `
runtime:
  base_dir: "` + filepath.Join(root, "base") + `"
  history_dir: "` + filepath.Join(root, "history") + `"
  lib_dir: "` + filepath.Join(root, "lib") + `"
  errors_dir: "` + filepath.Join(root, "errors") + `"
  web_dir: "` + filepath.Join(root, "web") + `"
  cache_dir: "` + filepath.Join(root, "cache") + `"
  ipsets_apply: false
artifacts:
  dronebl:
    type: dronebl_buildzone
    frequency: 60
    info: dronebl
    maintainer: dronebl
    maintainer_url: https://example.test
    rsync_url: rsync://example.test/dronebl/
sources:
  child:
    url: artifact://dronebl?parts=auto_botnets
    frequency: 0
    history: [1440]
    ipv: ipv4
    output: ipset
    processor:
      - passthrough
    category: attacks
    info: child feed
    maintainer: test
    maintainer_url: https://example.test
`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	eng.state = cache.New()
	if err := touchFileAt(eng.artifactEnablePath("dronebl"), eng.now().UTC()); err != nil {
		t.Fatal(err)
	}
	childPath := filepath.Join(root, "base", "child.source")
	if err := os.MkdirAll(filepath.Dir(childPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(childPath, []byte("1.2.3.4\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	decision, err := eng.FetchAndStage(t.Context(), "child", true, true)
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(decision.ProcessingNames, "child") {
		t.Fatalf("expected child in processing decision, got %#v", decision.ProcessingNames)
	}
	if !containsString(decision.ProcessingNames, "child_1d") {
		t.Fatalf("expected child history derivative in processing decision, got %#v", decision.ProcessingNames)
	}
	if !fileExists(stagedPath(filepath.Join(root, "base", "child_1d.ipset"))) {
		t.Fatalf("expected staged history-derivative feed body to exist")
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestManualRecheckRebuildsRetainedRawOnStatusSame(t *testing.T) {
	const rawBody = `{"list":["103.21.244.0/22","104.16.0.0/13"]}`
	modified := time.Date(2026, 4, 24, 9, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", modified.Format(http.TimeFormat))
		_, _ = w.Write([]byte(rawBody))
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
      - extract_ipv4_cidr
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
	rawPath := eng.sourcePath("sample")
	if err := os.WriteFile(rawPath, []byte(rawBody), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(rawPath, modified, modified); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(eng.feedBodyPath("sample"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	decision, err := eng.FetchAndStage(t.Context(), "sample", true, true)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Status != DownloadStatusDownloaded {
		t.Fatalf("decision status = %q, want %q", decision.Status, DownloadStatusDownloaded)
	}
	if !containsString(decision.ProcessingNames, "sample") {
		t.Fatalf("expected sample queued for processing, got %#v", decision.ProcessingNames)
	}
	stagedBody, err := os.ReadFile(stagedPath(eng.feedBodyPath("sample")))
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(stagedBody))
	want := "103.21.244.0/22\n104.16.0.0/13"
	if got != want {
		t.Fatalf("staged canonical body = %q, want %q", got, want)
	}
}

func TestManualRecheckRebuildsRetainedRawOnNotModified(t *testing.T) {
	const rawBody = `{"list":["103.21.244.0/22","104.16.0.0/13"]}`
	modified := time.Date(2026, 4, 24, 9, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-Modified-Since") == "" {
			t.Fatal("expected conditional recheck request to send If-Modified-Since")
		}
		w.WriteHeader(http.StatusNotModified)
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
      - extract_ipv4_cidr
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
	rawPath := eng.sourcePath("sample")
	if err := os.WriteFile(rawPath, []byte(rawBody), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(rawPath, modified, modified); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(eng.feedBodyPath("sample"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	decision, err := eng.FetchAndStage(t.Context(), "sample", true, true)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Status != DownloadStatusDownloaded {
		t.Fatalf("decision status = %q, want %q", decision.Status, DownloadStatusDownloaded)
	}
	entry := eng.state.EntrySnapshot("sample")
	if entry == nil {
		t.Fatal("missing cache entry")
	}
	if got, want := entry.SourceDate, modified.Unix(); got != want {
		t.Fatalf("source_date = %d, want raw source mtime %d", got, want)
	}
	stagedBody, err := os.ReadFile(stagedPath(eng.feedBodyPath("sample")))
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(stagedBody))
	want := "103.21.244.0/22\n104.16.0.0/13"
	if got != want {
		t.Fatalf("staged canonical body = %q, want %q", got, want)
	}
}
