package engine

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
)

func TestBashCompatibleHistoryChangesetAndRetentionFiles(t *testing.T) {
	mods := []time.Time{
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC),
	}
	bodies := []string{
		"1.1.1.1\n",
		"1.1.1.1\n2.2.2.2\n",
		"2.2.2.2\n3.3.3.3\n",
	}
	var request int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := request
		request++
		if idx >= len(bodies) {
			idx = len(bodies) - 1
		}
		w.Header().Set("Last-Modified", mods[idx].Format(http.TimeFormat))
		_, _ = w.Write([]byte(bodies[idx]))
	}))
	defer server.Close()

	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	webDir := filepath.Join(root, "web")
	libDir := filepath.Join(root, "lib")
	cfgPath := filepath.Join(root, "config.yaml")
	cfg := fmt.Sprintf(`
runtime:
  base_dir: %q
  history_dir: %q
  lib_dir: %q
  errors_dir: %q
  web_dir: %q
  cache_dir: %q
  web_charts_entries: 2
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
    info: file contract source
    maintainer: test
    maintainer_url: https://example.test
`, baseDir, filepath.Join(root, "history"), libDir, filepath.Join(root, "errors"), webDir, filepath.Join(root, "cache"), server.URL)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := range bodies {
		now := mods[i].Add(12 * time.Hour)
		eng.now = func() time.Time { return now }
		if _, err := runSchedulerStyleOnce(t, eng, RunOptions{EnableAll: true, Manual: true, Recheck: true, CleanupOld: true}); err != nil {
			t.Fatal(err)
		}
	}

	internalHistory := readNonEmptyLines(t, filepath.Join(libDir, "sample", "history.csv"))
	if got, want := len(internalHistory), 4; got != want {
		t.Fatalf("expected full internal history ledger with %d lines, got %d: %v", want, got, internalHistory)
	}
	if !strings.HasPrefix(internalHistory[1], fmt.Sprintf("%d,", mods[0].Unix())) ||
		!strings.HasPrefix(internalHistory[3], fmt.Sprintf("%d,", mods[2].Unix())) {
		t.Fatalf("expected internal history timestamps from source mtimes, got %v", internalHistory)
	}

	publicHistory := readNonEmptyLines(t, filepath.Join(webDir, "sample_history.csv"))
	if got, want := len(publicHistory), 3; got != want {
		t.Fatalf("expected public history window with %d lines, got %d: %v", want, got, publicHistory)
	}
	if strings.Contains(publicHistory[1], fmt.Sprint(mods[0].Unix())) ||
		!strings.HasPrefix(publicHistory[1], fmt.Sprintf("%d,", mods[1].Unix())) ||
		!strings.HasPrefix(publicHistory[2], fmt.Sprintf("%d,", mods[2].Unix())) {
		t.Fatalf("expected public history to contain last 2 rows, got %v", publicHistory)
	}

	internalChangesets := readNonEmptyLines(t, filepath.Join(libDir, "sample", "changesets.csv"))
	if got, want := len(internalChangesets), 4; got != want {
		t.Fatalf("expected full internal changeset ledger with %d lines, got %d: %v", want, got, internalChangesets)
	}
	if internalChangesets[0] != "DateTime,IPsAdded,IPsRemoved" {
		t.Fatalf("unexpected internal changeset header: %q", internalChangesets[0])
	}

	publicChangesets := readNonEmptyLines(t, filepath.Join(webDir, "sample_changesets.csv"))
	if got, want := len(publicChangesets), 3; got != want {
		t.Fatalf("expected public changeset window with %d lines, got %d: %v", want, got, publicChangesets)
	}
	if publicChangesets[0] != "DateTime,AddedIPs,RemovedIPs" {
		t.Fatalf("unexpected public changeset header: %q", publicChangesets[0])
	}
	if strings.Contains(publicChangesets[1], fmt.Sprint(mods[0].Unix())) ||
		!strings.HasPrefix(publicChangesets[1], fmt.Sprintf("%d,", mods[1].Unix())) ||
		!strings.HasPrefix(publicChangesets[2], fmt.Sprintf("%d,", mods[2].Unix())) {
		t.Fatalf("expected public changesets to skip bootstrap and contain last rows, got %v", publicChangesets)
	}

	if _, err := os.Stat(filepath.Join(webDir, "sample_retention.json")); err != nil {
		t.Fatalf("expected public retention JSON: %v", err)
	}
	expectedProcessedMTime := time.Unix(mods[len(mods)-1].Add(12*time.Hour).Unix(), 0).UTC()
	for _, path := range []string{
		filepath.Join(webDir, "sample.json"),
		filepath.Join(webDir, "sample_history.csv"),
		filepath.Join(webDir, "sample_changesets.csv"),
		filepath.Join(webDir, "sample_retention.json"),
	} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat generated file %s: %v", path, err)
		}
		if info.ModTime().Before(expectedProcessedMTime) {
			t.Fatalf("generated file %s mtime = %s, want at least processed timestamp %s", path, info.ModTime(), expectedProcessedMTime)
		}
	}
	if _, err := os.Stat(filepath.Join(libDir, "sample", "latest")); err != nil {
		t.Fatalf("expected bash-compatible latest binary: %v", err)
	}
	if _, err := os.Stat(filepath.Join(libDir, "sample", "latest.set")); !os.IsNotExist(err) {
		t.Fatalf("expected no new latest.set file, got err=%v", err)
	}
	for _, mod := range mods[1:] {
		if _, err := os.Stat(filepath.Join(libDir, "sample", "new", fmt.Sprint(mod.Unix()))); err != nil {
			t.Fatalf("expected bash-compatible new/%d file: %v", mod.Unix(), err)
		}
		if _, err := os.Stat(filepath.Join(libDir, "sample", "new", fmt.Sprintf("%d.set", mod.Unix()))); !os.IsNotExist(err) {
			t.Fatalf("expected no new/%d.set file, got err=%v", mod.Unix(), err)
		}
	}
	histogram, err := os.ReadFile(filepath.Join(libDir, "sample", "histogram"))
	if err != nil {
		t.Fatalf("expected bash-compatible histogram cache: %v", err)
	}
	if !strings.Contains(string(histogram), `declare -- RETENTION_HISTOGRAM_STARTED="`) ||
		!strings.Contains(string(histogram), `declare -a RETENTION_HISTOGRAM_REST=`) {
		t.Fatalf("unexpected histogram cache contents: %s", histogram)
	}
	retentionData, err := os.ReadFile(filepath.Join(webDir, "sample_retention.json"))
	if err != nil {
		t.Fatal(err)
	}
	var retention RetentionData
	if err := json.Unmarshal(retentionData, &retention); err != nil {
		t.Fatal(err)
	}
	if retention.Past.Total != 0 {
		t.Fatalf("expected bash histogram to exclude removals from bootstrap row, got past total %d", retention.Past.Total)
	}
	if retention.Current.Total != 2 {
		t.Fatalf("expected current retention total 2, got %d", retention.Current.Total)
	}
	metadataData, err := os.ReadFile(filepath.Join(webDir, "sample.json"))
	if err != nil {
		t.Fatal(err)
	}
	var metadata setMetadata
	if err := json.Unmarshal(metadataData, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.Started != mods[0].Unix()*1000 {
		t.Fatalf("expected started from source mtime, got %d want %d", metadata.Started, mods[0].Unix()*1000)
	}
	if metadata.RotationSamples != 2 {
		t.Fatalf("expected rotation stats to include the latest changeset immediately, got samples=%d", metadata.RotationSamples)
	}
	if metadata.ChangeRatioSamples != 2 {
		t.Fatalf("expected change-ratio stats to include the latest changeset immediately, got samples=%d", metadata.ChangeRatioSamples)
	}
	staged, err := filepath.Glob(filepath.Join(webDir, ".update-ipsets-web-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(staged) != 0 {
		t.Fatalf("expected staging directories to be cleaned up, got %v", staged)
	}
}

func TestPublicChangesetsNormalizesOldGoInternalHeader(t *testing.T) {
	root := t.TempDir()
	libDir := filepath.Join(root, "lib")
	webDir := filepath.Join(root, "web")
	if err := os.MkdirAll(filepath.Join(libDir, "sample"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(webDir, 0o755); err != nil {
		t.Fatal(err)
	}
	internalPath := filepath.Join(libDir, "sample", "changesets.csv")
	if err := os.WriteFile(internalPath, []byte("DateTime,AddedIPs,RemovedIPs\n1,10,0\n2,1,1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.LibDir = libDir
		rt.WebChartsEntries = 500
	}))
	if err := eng.writePublicChangesetsCSV("sample", webDir); err != nil {
		t.Fatal(err)
	}

	internal := readNonEmptyLines(t, internalPath)
	if internal[0] != "DateTime,IPsAdded,IPsRemoved" {
		t.Fatalf("expected normalized internal header, got %q", internal[0])
	}
	public := readNonEmptyLines(t, filepath.Join(webDir, "sample_changesets.csv"))
	if public[0] != "DateTime,AddedIPs,RemovedIPs" {
		t.Fatalf("unexpected public header: %q", public[0])
	}
	if got, want := len(public), 2; got != want {
		t.Fatalf("expected public bootstrap skip to leave %d lines, got %d: %v", want, got, public)
	}
	if public[1] != "2,1,1" {
		t.Fatalf("expected public file to skip bootstrap row, got %v", public)
	}
}

func TestPublicMetadataUsesBashClockSkewMilliseconds(t *testing.T) {
	observed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	sourceTime := observed.Add(2 * time.Hour)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", sourceTime.Format(http.TimeFormat))
		_, _ = w.Write([]byte("1.1.1.1\n"))
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
    category: tests
    info: skew source
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
	eng.now = func() time.Time { return observed }
	if _, err := runSchedulerStyleOnce(t, eng, RunOptions{EnableAll: true, Manual: true, CleanupOld: true}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(webDir, "sample.json"))
	if err != nil {
		t.Fatal(err)
	}
	var metadata setMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.ClockSkew != int64((2*time.Hour)/time.Millisecond) {
		t.Fatalf("expected bash clock_skew milliseconds, got %d", metadata.ClockSkew)
	}
}

func TestNoUpdateDoesNotRepublishWebFiles(t *testing.T) {
	mod := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", mod.Format(http.TimeFormat))
		_, _ = w.Write([]byte("1.1.1.1\n"))
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
    frequency: 1440
    ipv: ipv4
    output: ip
    processor:
      - passthrough
    category: tests
    info: no update source
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
	eng.now = func() time.Time { return mod.Add(time.Hour) }
	if _, err := runSchedulerStyleOnce(t, eng, RunOptions{EnableAll: true, Manual: true, CleanupOld: true}); err != nil {
		t.Fatal(err)
	}
	metaPath := filepath.Join(webDir, "sample.json")
	oldMTime := mod.Add(-24 * time.Hour)
	if err := os.Chtimes(metaPath, oldMTime, oldMTime); err != nil {
		t.Fatal(err)
	}
	eng.now = func() time.Time { return mod.Add(2 * time.Hour) }
	report, err := runSchedulerStyleOnce(t, eng, RunOptions{EnableAll: true, CleanupOld: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Updated) != 0 {
		t.Fatalf("expected no updated feeds, got %#v", report.Updated)
	}
	info, err := os.Stat(metaPath)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(oldMTime) {
		t.Fatalf("expected no web republish, mtime got %s want %s", info.ModTime(), oldMTime)
	}
}

func TestCopyUpdatedIPSetsToWebDirForIPSets(t *testing.T) {
	mod := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", mod.Format(http.TimeFormat))
		_, _ = w.Write([]byte("1.1.1.1\n"))
	}))
	defer server.Close()

	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	webDir := filepath.Join(root, "web")
	filesDir := filepath.Join(root, "files")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
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
  web_dir_for_ipsets: %q
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
    category: tests
    info: copied source
    maintainer: test
    maintainer_url: https://example.test
`, baseDir, filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), webDir, filesDir, filepath.Join(root, "cache"), server.URL)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	eng.now = func() time.Time { return mod.Add(time.Hour) }
	if _, err := runSchedulerStyleOnce(t, eng, RunOptions{EnableAll: true, Manual: true, CleanupOld: true}); err != nil {
		t.Fatal(err)
	}
	copiedPath := filepath.Join(filesDir, "sample.ipset")
	if _, err := os.Stat(copiedPath); err != nil {
		t.Fatalf("expected copied public ipset file: %v", err)
	}
	if _, err := os.Stat(copiedPath + ".new"); !os.IsNotExist(err) {
		t.Fatalf("expected no stale .new file, got err=%v", err)
	}
}

func TestRenameAndDeleteCleanupPublicSecondaryFiles(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	webDir := filepath.Join(root, "web")
	libDir := filepath.Join(root, "lib")
	historyDir := filepath.Join(root, "history")
	for _, dir := range []string{baseDir, webDir, libDir, historyDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.New()
	cfg.Sources["geo_provider"] = &config.Source{Name: "geo_provider", Use: []string{config.UseGeoIP}}
	cfg.Sources["asn_provider"] = &config.Source{Name: "asn_provider", Use: []string{config.UseASN}}
	cfg.Sources["bogon_provider"] = &config.Source{Name: "bogon_provider", Use: []string{config.UseBogons}}
	eng := newEngineFixture(t, withConfig(cfg), withRuntime(func(rt *Runtime) {
		rt.BaseDir = baseDir
		rt.WebDir = webDir
		rt.LibDir = libDir
		rt.HistoryDir = historyDir
	}))
	baseSuffixes := []string{".source", ".ipset", ".netset", ".split", ".setinfo"}
	for _, suffix := range baseSuffixes {
		if err := os.WriteFile(filepath.Join(baseDir, "old"+suffix), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	webSuffixes := eng.publicArtifactSuffixes()
	for _, suffix := range webSuffixes {
		if err := os.WriteFile(filepath.Join(webDir, "old"+suffix), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := eng.renameIPSet("old", "new"); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range baseSuffixes {
		if _, err := os.Stat(filepath.Join(baseDir, "new"+suffix)); err != nil {
			t.Fatalf("expected renamed base suffix %s: %v", suffix, err)
		}
	}
	for _, suffix := range webSuffixes {
		if _, err := os.Stat(filepath.Join(webDir, "new"+suffix)); err != nil {
			t.Fatalf("expected renamed web suffix %s: %v", suffix, err)
		}
	}
	if err := eng.deleteIPSet("new"); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range baseSuffixes {
		if _, err := os.Stat(filepath.Join(baseDir, "new"+suffix)); !os.IsNotExist(err) {
			t.Fatalf("expected deleted base suffix %s, got err=%v", suffix, err)
		}
	}
	for _, suffix := range webSuffixes {
		if _, err := os.Stat(filepath.Join(webDir, "new"+suffix)); !os.IsNotExist(err) {
			t.Fatalf("expected deleted web suffix %s, got err=%v", suffix, err)
		}
	}
}

func readNonEmptyLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw := strings.Split(strings.TrimSpace(string(data)), "\n")
	out := raw[:0]
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
