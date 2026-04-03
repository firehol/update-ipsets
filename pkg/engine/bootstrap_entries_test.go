package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

func TestNewBootstrapsRestoredFeedFromDisk(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	libDir := filepath.Join(root, "lib")
	historyDir := filepath.Join(root, "history")
	errorsDir := filepath.Join(root, "errors")
	webDir := filepath.Join(root, "web")
	cacheDir := filepath.Join(root, "cache")

	for _, dir := range []string{baseDir, libDir, historyDir, errorsDir, webDir, cacheDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	name := "restored"
	feedDir := filepath.Join(libDir, name)
	if err := os.MkdirAll(feedDir, 0o755); err != nil {
		t.Fatal(err)
	}

	historyCSV := "DateTime,Entries,UniqueIPs\n1711929600,1,1\n1711933200,2,5\n171193320012345,999,999\n"
	if err := os.WriteFile(filepath.Join(feedDir, "history.csv"), []byte(historyCSV), 0o644); err != nil {
		t.Fatal(err)
	}

	set := iprange.New(name)
	ip := mustIPv4(t, "1.2.3.4")
	if err := set.Add(ip, ip); err != nil {
		t.Fatal(err)
	}
	lo := iprange.Network(mustIPv4(t, "5.6.7.8"), 30)
	hi := iprange.Broadcast(mustIPv4(t, "5.6.7.8"), 30)
	if err := set.Add(lo, hi); err != nil {
		t.Fatal(err)
	}

	latestPath := filepath.Join(feedDir, "latest")
	f, err := os.Create(latestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := iprange.WriteBinary(f, set); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
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
  restored:
    url: https://example.test/restored.txt
    frequency: 60
    ipv: ipv4
    output: ip
    processor:
      - passthrough
    category: attacks
    info: restored feed
    maintainer: test
    maintainer_url: https://example.test
`, baseDir, historyDir, libDir, errorsDir, webDir, cacheDir)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}

	entry := eng.state.EntrySnapshot(name)
	if entry == nil {
		t.Fatalf("expected bootstrapped cache entry for %q", name)
	}
	if entry.File != "" {
		t.Fatalf("expected no public text file for bootstrapped feed, got %q", entry.File)
	}
	if got, want := entry.Version, 2; got != want {
		t.Fatalf("unexpected version: got %d want %d", got, want)
	}
	if got, want := entry.Entries, 2; got != want {
		t.Fatalf("unexpected entries: got %d want %d", got, want)
	}
	if got, want := entry.UniqueIPs, uint64(5); got != want {
		t.Fatalf("unexpected unique IPs: got %d want %d", got, want)
	}
	if got, want := entry.AverageUpdateMins, 60; got != want {
		t.Fatalf("unexpected average cadence: got %d want %d", got, want)
	}
	if got, want := entry.MaxUpdateMins, 60; got != want {
		t.Fatalf("unexpected max cadence: got %d want %d", got, want)
	}
	if !eng.hasUsableSet(name) {
		t.Fatalf("expected %q to have a usable set via lib/latest", name)
	}

	found := false
	for _, candidate := range eng.outputNames() {
		if candidate == name {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected %q in outputNames()", name)
	}

	publicFeeds := eng.PublicFeedSummaries()
	found = false
	for _, summary := range publicFeeds {
		if summary.Name == name {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected %q in public summaries", name)
	}

	matches, err := eng.QueryIP(t.Context(), "5.6.7.9")
	if err != nil {
		t.Fatal(err)
	}
	found = false
	for _, match := range matches {
		if match.Name == name {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected QueryIP() to match %q from lib/latest without a text file", name)
	}
}

func TestHistoryAndChangesetParsersAcceptCRLF(t *testing.T) {
	points := parseHistoryCSVData([]byte("DateTime,Entries,UniqueIPs\r\n1711929600,1,1\r\n1711933200,2,5\r\n"), "sample")
	if got, want := len(points), 2; got != want {
		t.Fatalf("history points len = %d, want %d", got, want)
	}
	if got, want := points[1].UniqueIPs, uint64(5); got != want {
		t.Fatalf("history unique IPs = %d, want %d", got, want)
	}

	changes := parseChangesetCSVData([]byte("DateTime,AddedIPs,RemovedIPs\r\n1711929600,7,0\r\n1711933200,0,2\r\n"))
	if got, want := len(changes), 2; got != want {
		t.Fatalf("changeset rows len = %d, want %d", got, want)
	}
	if got, want := changes[1].Removed, uint64(2); got != want {
		t.Fatalf("changeset removed = %d, want %d", got, want)
	}
}

func mustIPv4(t *testing.T, raw string) uint32 {
	t.Helper()
	ip, err := iprange.ParseIPv4Token(raw)
	if err != nil {
		t.Fatalf("parse IPv4 %q: %v", raw, err)
	}
	return ip
}

func TestNewReconcilesCachedMetadataFromConfigWithoutRefreshingStats(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	historyDir := filepath.Join(root, "history")
	libDir := filepath.Join(root, "lib")
	errorsDir := filepath.Join(root, "errors")
	webDir := filepath.Join(root, "web")
	cacheDir := filepath.Join(root, "cache")

	for _, dir := range []string{baseDir, historyDir, libDir, errorsDir, webDir, cacheDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
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
categories:
  intrusion:
    label: Intrusion
    description: Active hostile access attempts.
sources:
  sample:
    url: https://example.test/sample.txt
    frequency: 60
    ipv: ipv4
    output: ip
    processor:
      - passthrough
    category: intrusion
    info: sample feed
    maintainer: test
    maintainer_url: https://example.test
`, baseDir, historyDir, libDir, errorsDir, webDir, cacheDir)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	feedDir := filepath.Join(libDir, "sample")
	if err := os.MkdirAll(feedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	historyCSV := "DateTime,Entries,UniqueIPs\n1711929600,1,1\n1711933200,2,5\n"
	if err := os.WriteFile(filepath.Join(feedDir, "history.csv"), []byte(historyCSV), 0o644); err != nil {
		t.Fatal(err)
	}

	st := cache.New()
	entry := st.Entry("sample")
	entry.Name = "sample"
	entry.Category = "attacks"
	entry.Info = "stale info"
	entry.Maintainer = "stale maintainer"
	entry.MaintainerURL = "https://stale.example.test"
	entry.AverageUpdateMins = 999999
	entry.MinUpdateMins = 999999
	entry.MaxUpdateMins = 999999
	entry.SourceDate = 1711929600
	entry.ProcessedDate = 1711929600
	entry.CheckedDate = 1711929600
	cachePath := filepath.Join(baseDir, ".cache.json")
	if err := cache.Save(cachePath, st); err != nil {
		t.Fatal(err)
	}

	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}

	got := eng.state.EntrySnapshot("sample")
	if got == nil {
		t.Fatalf("expected cache entry for sample")
	}
	if got.Category != "intrusion" {
		t.Fatalf("expected category to be reconciled from config, got %q", got.Category)
	}
	if got.Info != "sample feed" {
		t.Fatalf("expected info to be reconciled from config, got %q", got.Info)
	}
	if got.Maintainer != "test" {
		t.Fatalf("expected maintainer to be reconciled from config, got %q", got.Maintainer)
	}
	if got.AverageUpdateMins != 999999 || got.MinUpdateMins != 999999 || got.MaxUpdateMins != 999999 {
		t.Fatalf("expected startup reconcile to leave cached cadence untouched, got avg=%d min=%d max=%d", got.AverageUpdateMins, got.MinUpdateMins, got.MaxUpdateMins)
	}

	summaries := eng.PublicFeedSummaries()
	if len(summaries) != 1 {
		t.Fatalf("expected one public summary, got %d", len(summaries))
	}
	if summaries[0].Category != "intrusion" {
		t.Fatalf("expected public summary category intrusion, got %q", summaries[0].Category)
	}
	if summaries[0].AverageUpdateMins != 999999 {
		t.Fatalf("expected public summary avg cadence to stay cached at startup, got %d", summaries[0].AverageUpdateMins)
	}
}

func TestNewRepairsInvalidCachedTimestampsFromDisk(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	historyDir := filepath.Join(root, "history")
	libDir := filepath.Join(root, "lib")
	errorsDir := filepath.Join(root, "errors")
	webDir := filepath.Join(root, "web")
	cacheDir := filepath.Join(root, "cache")

	for _, dir := range []string{baseDir, historyDir, libDir, errorsDir, webDir, cacheDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
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
categories:
  provider_infrastructure:
    label: Provider Infrastructure
    description: Benign provider and platform networks.
sources:
  sample:
    url: https://example.test/sample.txt
    frequency: 1440
    ipv: ipv4
    output: ip
    processor:
      - passthrough
    category: provider_infrastructure
    info: sample feed
    maintainer: test
    maintainer_url: https://example.test
`, baseDir, historyDir, libDir, errorsDir, webDir, cacheDir)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	feedDir := filepath.Join(libDir, "sample")
	if err := os.MkdirAll(feedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	historyCSV := "DateTime,Entries,UniqueIPs\n1709549889,1,1\n1709584328,2,2\n"
	if err := os.WriteFile(filepath.Join(feedDir, "history.csv"), []byte(historyCSV), 0o644); err != nil {
		t.Fatal(err)
	}
	set := iprange.New("sample")
	ip := mustIPv4(t, "1.2.3.4")
	if err := set.Add(ip, ip); err != nil {
		t.Fatal(err)
	}
	latestPath := filepath.Join(feedDir, "latest")
	f, err := os.Create(latestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := iprange.WriteBinary(f, set); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	mtime := time.Unix(1709584328, 0).UTC()
	if err := os.Chtimes(latestPath, mtime, mtime); err != nil {
		t.Fatal(err)
	}

	st := cache.New()
	entry := st.Entry("sample")
	entry.Name = "sample"
	entry.SourceDate = 1_521_527_945_506
	entry.ProcessedDate = 1_521_527_945_506
	entry.CheckedDate = 1709590000
	entry.StartedDate = 1_521_527_945_506
	cachePath := filepath.Join(baseDir, ".cache.json")
	if err := cache.Save(cachePath, st); err != nil {
		t.Fatal(err)
	}

	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}

	got := eng.state.EntrySnapshot("sample")
	if got == nil {
		t.Fatalf("expected repaired cache entry for sample")
	}
	if got.SourceDate != 1709584328 {
		t.Fatalf("source_date = %d, want 1709584328", got.SourceDate)
	}
	if got.ProcessedDate != 1709584328 {
		t.Fatalf("processed_date = %d, want 1709584328", got.ProcessedDate)
	}
	if got.StartedDate != 1709549889 {
		t.Fatalf("started_date = %d, want 1709549889", got.StartedDate)
	}
	if got.CheckedDate != 1709590000 {
		t.Fatalf("checked_date = %d, want unchanged 1709590000", got.CheckedDate)
	}
}

func TestRepairEntryTimestampsFromDiskSkipsCleanEntries(t *testing.T) {
	var eng *Engine
	entry := &cache.Entry{
		SourceDate:         1709584328,
		ProcessedDate:      1709584328,
		CheckedDate:        1709590000,
		StartedDate:        1709549889,
		FailureStartedDate: 0,
	}

	if changed := eng.repairEntryTimestampsFromDisk("sample", &config.Source{}, entry); changed {
		t.Fatalf("expected clean entry to skip timestamp repair")
	}
}
