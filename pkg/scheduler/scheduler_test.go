package scheduler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/runreason"
)

func TestBuildSnapshotOrdersDueFirst(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	cfg := config.New()
	cfg.Sources["due"] = &config.Source{Name: "due", Frequency: 60}
	cfg.Sources["later"] = &config.Source{Name: "later", Frequency: 60}
	rt := engine.Runtime{BaseDir: t.TempDir(), IgnoreRepeatingDownloadErrors: 10}

	entries := []cache.Entry{
		{Name: "due", CheckedDate: now.Add(-2 * time.Hour).Unix()},
		{Name: "later", CheckedDate: now.Unix()},
	}

	snapshot := BuildSnapshot(cfg, rt, entries, true, now)
	if len(snapshot.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(snapshot.Items))
	}
	if snapshot.Items[0].Name != "due" {
		t.Fatalf("expected due item first, got %q", snapshot.Items[0].Name)
	}
}

func TestDueNames(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	snapshot := Snapshot{
		Items: []Item{
			{Name: "due", Enabled: true, NextDue: now},
			{Name: "later", Enabled: true, NextDue: now.Add(time.Hour)},
			{Name: "disabled", Enabled: false, NextDue: now},
		},
	}
	got := DueNames(snapshot, now)
	if len(got) != 1 || got[0] != "due" {
		t.Fatalf("unexpected due names: %#v", got)
	}
}

func TestHealthTransitionNamesOnlyReturnsChangedFeedHealth(t *testing.T) {
	prev := Snapshot{
		Items: []Item{
			{Name: "alpha", Kind: "source", HealthClass: "healthy"},
			{Name: "beta", Kind: "source", HealthClass: "delayed"},
			{Name: "db", Kind: "artifact", HealthClass: "healthy"},
		},
	}
	next := Snapshot{
		Items: []Item{
			{Name: "alpha", Kind: "source", HealthClass: "healthy"},
			{Name: "beta", Kind: "source", HealthClass: "risky"},
			{Name: "gamma", Kind: "source", HealthClass: "healthy"},
			{Name: "db", Kind: "artifact", HealthClass: "archived"},
		},
	}

	got := healthTransitionNames(prev, next)
	want := []string{"beta", "gamma"}
	if len(got) != len(want) {
		t.Fatalf("health transition names = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("health transition names = %v, want %v", got, want)
		}
	}
}

func TestHealthTransitionNamesSkipsColdStartSnapshot(t *testing.T) {
	next := Snapshot{
		Items: []Item{
			{Name: "alpha", Kind: "source", HealthClass: "healthy"},
			{Name: "beta", Kind: "source", HealthClass: "risky"},
		},
	}

	if got := healthTransitionNames(Snapshot{}, next); len(got) != 0 {
		t.Fatalf("expected no cold-start health transitions, got %v", got)
	}
}

func TestNextDueUsesSourceTimestampWhenUnchecked(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.source")
	ts := now.Add(-10 * time.Minute)
	if err := os.WriteFile(path, []byte("test\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := os.Chtimes(path, ts, ts); err != nil {
		t.Fatalf("Chtimes returned error: %v", err)
	}

	next, _ := nextDue(cache.Entry{Name: "sample"}, 30, path, now, 10, false, false)
	if next.IsZero() {
		t.Fatal("expected non-zero next due time")
	}
	if !next.After(ts) {
		t.Fatalf("expected next due after source timestamp, got %s", next)
	}
}

func TestBuildSnapshotStaticSourceConfigChangesAreDue(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	baseDir := t.TempDir()
	sourcePath := filepath.Join(baseDir, "static_reference.source")
	cfg := config.New()
	cfg.Sources["static_reference"] = &config.Source{
		Name:      "static_reference",
		Static:    []string{"1.1.1.1", "8.8.8.8"},
		Frequency: 0,
		IPV:       "ipv4",
		Output:    "netset",
	}
	rt := engine.Runtime{BaseDir: baseDir, IgnoreRepeatingDownloadErrors: 10}
	entries := []cache.Entry{{Name: "static_reference", CheckedDate: now.Add(-time.Hour).Unix()}}

	if err := os.WriteFile(sourcePath, []byte("1.1.1.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	snapshot := BuildSnapshot(cfg, rt, entries, true, now)
	if got := DueNames(snapshot, now); len(got) != 1 || got[0] != "static_reference" {
		t.Fatalf("due static sources after config change = %v, want static_reference", got)
	}
	if snapshot.Items[0].Detail != "static source config changed" {
		t.Fatalf("detail = %q, want static source config changed", snapshot.Items[0].Detail)
	}

	if err := os.WriteFile(sourcePath, []byte("1.1.1.1\n8.8.8.8\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	snapshot = BuildSnapshot(cfg, rt, entries, true, now)
	if got := DueNames(snapshot, now); len(got) != 0 {
		t.Fatalf("due static sources after matching config = %v, want none", got)
	}
	if snapshot.Items[0].Detail != "static source (never expires)" {
		t.Fatalf("detail = %q, want static source (never expires)", snapshot.Items[0].Detail)
	}
}

func TestBuildSnapshotCriticalProviderSetChangesAreForcedDue(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	libDir := filepath.Join(root, "lib")
	sourcePath := filepath.Join(baseDir, "critical_dns.source")
	cfg := config.New()
	cfg.Sources["critical_dns"] = &config.Source{
		Name:      "critical_dns",
		Static:    []string{"1.1.1.1"},
		Frequency: 0,
		IPV:       "ipv4",
		Output:    "netset",
		Use:       []string{config.UseCriticalInfrastructure},
		Critical: &config.CriticalMetadata{
			Tier:          "hard",
			Role:          "public_dns_core",
			SourceType:    "curated_static",
			SourceQuality: "C",
			Rationale:     "test provider",
		},
	}
	rt := engine.Runtime{BaseDir: baseDir, LibDir: libDir, IgnoreRepeatingDownloadErrors: 10}
	entries := []cache.Entry{{
		Name:          "critical_dns",
		CheckedDate:   now.Add(-time.Hour).Unix(),
		Hash:          "provider-body",
		ContentHash:   "ranges-a",
		File:          "critical_dns.ipset",
		Entries:       1,
		Version:       2,
		ProcessedDate: now.Add(-time.Hour).Unix(),
		UniqueIPs:     1,
	}}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourcePath, []byte("1.1.1.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	snapshot := BuildSnapshot(cfg, rt, entries, true, now)
	if got := DueNames(snapshot, now); len(got) != 1 || got[0] != "critical_dns" {
		t.Fatalf("due critical provider after missing marker = %v, want critical_dns", got)
	}
	if snapshot.Items[0].Detail != detailCriticalProviderSetChanged {
		t.Fatalf("detail = %q, want %q", snapshot.Items[0].Detail, detailCriticalProviderSetChanged)
	}

	markerPath := engine.CriticalInfrastructureProviderSetMarkerPath(rt)
	if err := os.MkdirAll(filepath.Dir(markerPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(markerPath, []byte(engine.CriticalInfrastructureProviderSetIDForSnapshot(cfg)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	snapshot = BuildSnapshot(cfg, rt, entries, true, now)
	if got := DueNames(snapshot, now); len(got) != 0 {
		t.Fatalf("due critical provider after matching marker = %v, want none", got)
	}
	if snapshot.Items[0].Detail != "static source (never expires)" {
		t.Fatalf("detail = %q, want static source (never expires)", snapshot.Items[0].Detail)
	}

	// Volatile cache fields (Version, ProcessedDate, ContentHash, Entries,
	// UniqueIPs) must not mark the critical provider as due. The provider-set
	// identity is catalog-only; content drift is absorbed by the per-feed
	// reprocess flow on the parent feed's next change.
	entries[0].Version++
	entries[0].ProcessedDate = now.Unix()
	entries[0].CheckedDate = now.Unix()
	entries[0].ContentHash = "ranges-b"
	entries[0].Entries = 99
	entries[0].UniqueIPs = 99
	snapshot = BuildSnapshot(cfg, rt, entries, true, now)
	if got := DueNames(snapshot, now); len(got) != 0 {
		t.Fatalf("due critical provider after volatile cache fields changed = %v, want none", got)
	}

	// Catalog (config) changes still flip the identity and force the provider due.
	cfg.Sources["critical_dns"].Processor = []config.ProcessorStep{{Name: "passthrough"}}
	snapshot = BuildSnapshot(cfg, rt, entries, true, now)
	if got := DueNames(snapshot, now); len(got) != 1 || got[0] != "critical_dns" {
		t.Fatalf("due critical provider after processing config changed = %v, want critical_dns", got)
	}
}

func TestAutomaticDueSkipsCriticalProviderSetRefreshWhileEngineRuns(t *testing.T) {
	item := Item{
		Name:   "critical_dns",
		Detail: detailCriticalProviderSetChanged,
	}
	if shouldEnqueueAutomaticDue(item, true) {
		t.Fatal("provider-set refresh should not be automatically requeued while an engine run is active")
	}
	if !shouldEnqueueAutomaticDue(item, false) {
		t.Fatal("provider-set refresh should be automatically queued when no engine run is active")
	}
	if !shouldEnqueueAutomaticDue(Item{Name: "regular", Detail: "due now"}, true) {
		t.Fatal("ordinary due work should not be suppressed by the provider-set guard")
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scheduler.json")
	snapshot := Snapshot{
		GeneratedAt: time.Unix(1_700_000_000, 0).UTC(),
		Items:       []Item{{Name: "sample", Kind: "source", Enabled: true}},
	}
	if err := SaveSnapshot(path, snapshot); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}
	got, err := LoadSnapshot(path)
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Name != "sample" {
		t.Fatalf("unexpected snapshot %#v", got)
	}
}

// TestBuildSnapshotUsesSplitChildEntryTimestamps was removed
// when the legacy bash-era `output: split` mode was dropped.
// snapshotEntryForSource is now a straight lookup — the split
// child-merge logic no longer has a reason to exist.

// TestMergeDueDoesNotStayOverdueFromHistoricSourceTimestamp was
// deleted with mergeDue itself. After the 2026-04 pipeline rewrite,
// merges are standalone Source entries with internal://merge URLs;
// the scheduler treats them identically to any other source and no
// longer special-cases "is this merge due based on input file
// mtimes?" logic.

func TestNextDueAppliesFrequencyMarginAndFailureBackoff(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()

	next, _ := nextDue(cache.Entry{Name: "sample", CheckedDate: now.Unix()}, 120, "", now, 10, false, false)
	wantMargin := now.Add(121 * time.Minute)
	if !next.Equal(wantMargin) {
		t.Fatalf("unexpected margin-adjusted next due: got %s want %s", next, wantMargin)
	}

	next, _ = nextDue(cache.Entry{Name: "sample", CheckedDate: now.Unix(), DownloadFailures: 3}, 60, "", now, 10, false, false)
	wantRetry := now.Add(16 * time.Minute)
	if !next.Equal(wantRetry) {
		t.Fatalf("unexpected pre-unmaintained retry next due: got %s want %s", next, wantRetry)
	}

	next, _ = nextDue(cache.Entry{Name: "sample", CheckedDate: now.Unix(), DownloadFailures: 12}, 60, "", now, 10, false, false)
	wantBackoff := now.Add(60 * time.Minute)
	if !next.Equal(wantBackoff) {
		t.Fatalf("unexpected capped retry next due: got %s want %s", next, wantBackoff)
	}

	next, _ = nextDue(cache.Entry{Name: "sample", CheckedDate: now.Unix(), DownloadFailures: 6}, 60, "", now, 10, true, false)
	wantUnmaintained := now.Add(128 * time.Minute)
	if !next.Equal(wantUnmaintained) {
		t.Fatalf("unexpected unmaintained retry next due: got %s want %s", next, wantUnmaintained)
	}
}

func TestBuildSnapshotIgnoresOutOfRangeCheckedDate(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	cfg := config.New()
	cfg.Sources["sample"] = &config.Source{Name: "sample", Frequency: 60}
	rt := engine.Runtime{BaseDir: t.TempDir(), IgnoreRepeatingDownloadErrors: 10}

	snapshot := BuildSnapshot(cfg, rt, []cache.Entry{{
		Name:        "sample",
		CheckedDate: 1_521_527_945_506,
	}}, true, now)
	if len(snapshot.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(snapshot.Items))
	}
	if !snapshot.Items[0].CheckedAt.IsZero() {
		t.Fatalf("expected checked_at to be zero, got %v", snapshot.Items[0].CheckedAt)
	}
	if !snapshot.Items[0].NextDue.Equal(now) {
		t.Fatalf("expected invalid checked date to fall back to due now, got %v want %v", snapshot.Items[0].NextDue, now)
	}
	if err := SaveSnapshot(filepath.Join(t.TempDir(), "scheduler.json"), snapshot); err != nil {
		t.Fatalf("expected snapshot with invalid cached checked_date to save, got %v", err)
	}
}

func TestBuildSnapshotUsesExplicitMergeCadence(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	cfg := config.New()
	cfg.Sources["merged"] = &config.Source{
		Name:       "merged",
		URL:        config.InternalMergeScheme + "?inputs=alpha,beta",
		Frequency:  42,
		Provenance: config.ProvenanceSecondaryMerge,
	}
	rt := engine.Runtime{
		BaseDir:                       t.TempDir(),
		IgnoreRepeatingDownloadErrors: 10,
	}
	snapshot := BuildSnapshot(cfg, rt, []cache.Entry{{
		Name:        "merged",
		CheckedDate: now.Unix(),
	}}, true, now)
	if len(snapshot.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(snapshot.Items))
	}
	if snapshot.Items[0].FrequencyMinutes != 42 {
		t.Fatalf("expected merge frequency 42 mins, got %d", snapshot.Items[0].FrequencyMinutes)
	}
	want := now.Add(42 * time.Minute)
	if !snapshot.Items[0].NextDue.Equal(want) {
		t.Fatalf("unexpected merge next due: got %v want %v", snapshot.Items[0].NextDue, want)
	}
}

func TestNextWaitUsesArtifactDeadlines(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	wait := nextWait(now,
		[]Item{{Name: "plain", Enabled: true, NextDue: now.Add(10 * time.Minute)}},
		[]Item{{Name: "artifact", Enabled: true, NextDue: now.Add(2 * time.Minute)}},
	)
	if wait != 2*time.Minute {
		t.Fatalf("expected artifact deadline to win, got %s", wait)
	}
}

func TestBuildSnapshotUsesProviderEnableStateAndArchivePath(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	root := t.TempDir()
	cfg := config.New()
	cfg.Sources["geo_db"] = &config.Source{
		Name:      "geo_db",
		URL:       "https://example.test/geo.mmdb",
		Frequency: 1440,
		Use:       []string{config.UseGeoIP},
	}
	rt := engine.Runtime{
		BaseDir:                       filepath.Join(root, "base"),
		LibDir:                        filepath.Join(root, "lib"),
		IgnoreRepeatingDownloadErrors: 10,
	}
	if err := os.MkdirAll(rt.BaseDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	archivePath := filepath.Join(rt.LibDir, "geolocation", "geo_db.source")
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	ts := now.Add(-2 * time.Hour)
	if err := os.WriteFile(archivePath, []byte("provider body"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := os.Chtimes(archivePath, ts, ts); err != nil {
		t.Fatalf("Chtimes returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rt.BaseDir, "geo_db.enabled"), []byte{}, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	snapshot := BuildSnapshot(cfg, rt, nil, false, now)
	if len(snapshot.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(snapshot.Items))
	}
	if !snapshot.Items[0].Enabled {
		t.Fatalf("expected enabled provider snapshot item, got %#v", snapshot.Items[0])
	}
	if snapshot.Items[0].NextDue.IsZero() {
		t.Fatalf("expected next due to use provider archive mtime, got zero")
	}
	if !snapshot.Items[0].NextDue.After(ts) {
		t.Fatalf("expected next due after provider archive timestamp, got %v", snapshot.Items[0].NextDue)
	}
}

func TestBuildSnapshotMarksProviderDisabledWithoutArchiveMarker(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	cfg := config.New()
	cfg.Sources["geo_db"] = &config.Source{
		Name:      "geo_db",
		URL:       "https://example.test/geo.mmdb",
		Frequency: 1440,
		Use:       []string{config.UseGeoIP},
	}
	rt := engine.Runtime{BaseDir: t.TempDir(), LibDir: t.TempDir(), IgnoreRepeatingDownloadErrors: 10}

	snapshot := BuildSnapshot(cfg, rt, nil, false, now)
	if len(snapshot.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(snapshot.Items))
	}
	if snapshot.Items[0].Enabled {
		t.Fatalf("expected disabled provider snapshot item, got %#v", snapshot.Items[0])
	}
}

func TestSourceKindMatchesAdminTaxonomy(t *testing.T) {
	tests := []struct {
		name string
		src  *config.Source
		want string
	}{
		{
			name: "plain source",
			src:  &config.Source{Name: "plain"},
			want: "source",
		},
		{
			name: "retention derivative",
			src:  &config.Source{Name: "hist", Provenance: config.ProvenanceSecondaryRetention},
			want: "retention",
		},
		{
			name: "merge derivative",
			src:  &config.Source{Name: "merge", Provenance: config.ProvenanceSecondaryMerge},
			want: "merge",
		},
		{
			name: "asn provider",
			src:  &config.Source{Name: "asn", Use: []string{config.UseASN}},
			want: "asn",
		},
		{
			name: "geolocation provider",
			src:  &config.Source{Name: "geo", Use: []string{config.UseGeoIP}},
			want: "geolocation",
		},
		{
			name: "bogon source",
			src:  &config.Source{Name: "bogon", Use: []string{config.UseBogons}},
			want: "bogon",
		},
		{
			name: "critical infrastructure remains source",
			src:  &config.Source{Name: "infra", Use: []string{config.UseCriticalInfrastructure}},
			want: "source",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := sourceKind(tc.src); got != tc.want {
				t.Fatalf("sourceKind() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestManualRecheckQueuesDownloadWork(t *testing.T) {
	var blockDownload atomic.Bool
	downloadStarted := make(chan struct{}, 1)
	releaseDownload := make(chan struct{})
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if blockDownload.Load() {
			select {
			case downloadStarted <- struct{}{}:
			default:
			}
			select {
			case <-releaseDownload:
			case <-r.Context().Done():
				return
			}
		}
		_, _ = w.Write([]byte("1.2.3.4\n"))
	}))
	defer sourceServer.Close()
	defer close(releaseDownload)

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
    frequency: 9999
    ipv: ipv4
    output: ipset
    processor:
      - passthrough
    category: attacks
    info: sample feed
    maintainer: test
    maintainer_url: https://example.test
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"), sourceServer.URL)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runSchedulerStyleOnce(t, eng, engine.RunOptions{
		EnableAll: true, Manual: true, CleanupOld: true,
	}); err != nil {
		t.Fatal(err)
	}
	runner := New(eng, true, nil)
	startSchedulerRunner(t, runner)
	blockDownload.Store(true)

	if !runner.TriggerQueuedAction(PendingAction{
		Names:   []string{"sample"},
		Recheck: true,
		Reason:  runreason.ReasonManualRecheck,
	}) {
		t.Fatal("expected manual recheck action to be accepted")
	}

	select {
	case <-downloadStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for manual recheck download to start")
	}

	activity := waitForSchedulerActivity(t, runner, 2*time.Second, func(activity ActivitySnapshot) bool {
		return activityHasDownloadActive(activity, "sample")
	})
	if len(activity.ProcessingWaiting) != 0 {
		t.Fatalf("expected no direct processing queue entry, got %#v", activity.ProcessingWaiting)
	}
}

func TestFinishProcessingRequeuePreservesOriginalQueuedAt(t *testing.T) {
	runner := &Runner{
		processing: processingLoopState{
			waiting: map[string]queuedWork{},
			active:  map[string]ActiveQueueFeed{},
		},
		now: func() time.Time { return time.Unix(1_700_000_100, 0).UTC() },
	}
	queuedAt := time.Unix(1_700_000_000, 0).UTC()
	items := []queuedWork{{
		Name:     "sample",
		Reason:   runreason.ReasonScheduledDue,
		QueuedAt: queuedAt,
	}}

	runner.finishProcessing(items, true)

	got, ok := runner.processing.waiting["sample"]
	if !ok {
		t.Fatalf("expected sample to be requeued")
	}
	if !got.QueuedAt.Equal(queuedAt) {
		t.Fatalf("queued_at = %v, want %v", got.QueuedAt, queuedAt)
	}
}

func TestRunQueuedProcessingPromotesSuccessfulItemsAndRequeuesFailures(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	baseDir := filepath.Join(root, "base")
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
  good:
    url: https://example.test/good.txt
    frequency: 60
    ipv: ipv4
    output: ipset
    processor:
      - passthrough
    category: attacks
    info: good
    maintainer: test
    maintainer_url: https://example.test
  bad:
    url: https://example.test/bad.txt
    frequency: 60
    ipv: ipv4
    output: ipset
    processor:
      - passthrough
    category: attacks
    info: bad
    maintainer: test
    maintainer_url: https://example.test
`, baseDir, filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	runner := New(eng, true, nil)
	queuedAt := time.Unix(1_700_000_100, 0).UTC()

	if err := os.WriteFile(filepath.Join(baseDir, "good.ipset.new"), []byte("1.2.3.4\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	runner.processing.waiting["good"] = queuedWork{
		Name:     "good",
		Reason:   runreason.ReasonScheduledDue,
		QueuedAt: queuedAt,
		Promote:  []string{"good"},
	}
	runner.processing.waiting["bad"] = queuedWork{
		Name:     "bad",
		Reason:   runreason.ReasonScheduledDue,
		QueuedAt: queuedAt,
		Promote:  []string{"bad"},
	}

	runner.runQueuedProcessing(t.Context())

	activity := runner.ActivitySnapshot()
	if len(activity.ProcessingWaiting) != 1 || activity.ProcessingWaiting[0].Name != "bad" {
		t.Fatalf("expected only bad feed requeued, got %#v", activity.ProcessingWaiting)
	}
	if !activity.ProcessingWaiting[0].QueuedAt.Equal(queuedAt) {
		t.Fatalf("expected bad feed queued_at preserved, got %v want %v", activity.ProcessingWaiting[0].QueuedAt, queuedAt)
	}

	if _, err := os.Stat(filepath.Join(baseDir, "good.ipset.new")); !os.IsNotExist(err) {
		t.Fatalf("expected good staged source promoted away, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(baseDir, "bad.ipset.new")); err != nil {
		if !os.IsNotExist(err) {
			t.Fatalf("expected bad staged source to stay absent, got %v", err)
		}
	}
	if _, err := os.Stat(filepath.Join(baseDir, "good.ipset")); err != nil {
		t.Fatalf("expected good feed outputs committed, got %v", err)
	}
}

func TestScheduledDownloadWithProcessingWorkWakesProcessLoop(t *testing.T) {
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("1.2.3.4\n"))
	}))
	defer sourceServer.Close()

	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	baseDir := filepath.Join(root, "base")
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
  scheduled:
    url: %q
    frequency: 60
    ipv: ipv4
    output: ipset
    processor:
      - passthrough
    category: attacks
    info: scheduled feed
    maintainer: test
    maintainer_url: https://example.test
`, baseDir, filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"), sourceServer.URL)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	runner := New(eng, true, nil)
	stop := startSchedulerRunner(t, runner)

	waitForFileContent(t, filepath.Join(baseDir, "scheduled.ipset"), 5*time.Second, "1.2.3.4")
	stop()
}

func TestStartNextDownloadDefersDerivedDownloadUntilParentSettles(t *testing.T) {
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
  anonymous:
    url: https://example.test/anonymous.txt
    frequency: 60
    ipv: ipv4
    output: ipset
    processor:
      - passthrough
    category: attacks
    info: synthetic parent
    maintainer: test
    maintainer_url: https://example.test
merges:
  firehol_anonymous:
    ipv: ipv4
    output: ipset
    sources: [anonymous]
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	runner := New(eng, true, nil)
	queuedAt := time.Unix(1_700_000_000, 0).UTC()
	runner.download.waiting["anonymous"] = queuedWork{Name: "anonymous", QueuedAt: queuedAt}
	runner.download.waiting["firehol_anonymous"] = queuedWork{Name: "firehol_anonymous", QueuedAt: queuedAt}

	item, ok := runner.startNextDownload()
	if !ok {
		t.Fatal("expected a download candidate")
	}
	if item.Name != "anonymous" {
		t.Fatalf("expected parent download first, got %q", item.Name)
	}
}

func TestBuildSnapshotDisablesArtifactChildWhenParentDisabled(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	root := t.TempDir()

	cfg := config.New()
	cfg.Artifacts["dronebl"] = &config.Artifact{
		Name:      "dronebl",
		Type:      config.ArtifactTypeDroneBLBuildzone,
		Frequency: 60,
	}
	cfg.Sources["child"] = &config.Source{
		Name:           "child",
		URL:            "artifact://dronebl?parts=auto_botnets",
		ArtifactParent: "dronebl",
		Frequency:      0,
		IPV:            "ipv4",
		Output:         "ipset",
	}
	rt := engine.Runtime{
		BaseDir:                       filepath.Join(root, "base"),
		LibDir:                        filepath.Join(root, "lib"),
		IgnoreRepeatingDownloadErrors: 10,
		ProcessingIntervalMinutes:     10,
	}
	if err := os.MkdirAll(rt.BaseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rt.BaseDir, "child.source"), []byte("1.2.3.4\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	snapshot := BuildSnapshot(cfg, rt, nil, false, now)
	if len(snapshot.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(snapshot.Items))
	}
	if snapshot.Items[0].Enabled {
		t.Fatalf("expected artifact child to be disabled when parent marker is absent: %#v", snapshot.Items[0])
	}
}

func TestBuildArtifactItemsIncludesEnabledArtifact(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	root := t.TempDir()

	cfg := config.New()
	cfg.Artifacts["dronebl"] = &config.Artifact{
		Name:      "dronebl",
		Type:      config.ArtifactTypeDroneBLBuildzone,
		Frequency: 60,
	}
	rt := engine.Runtime{
		BaseDir:                       filepath.Join(root, "base"),
		LibDir:                        filepath.Join(root, "lib"),
		IgnoreRepeatingDownloadErrors: 10,
	}
	if err := os.MkdirAll(filepath.Join(rt.LibDir, "artifacts", "dronebl"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rt.LibDir, "artifacts", "dronebl", "enabled"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	items := BuildArtifactItems(cfg, rt, []cache.Entry{{
		Name:        "dronebl",
		CheckedDate: now.Add(-2 * time.Hour).Unix(),
	}}, false, now)
	if len(items) != 1 {
		t.Fatalf("expected 1 artifact item, got %d", len(items))
	}
	if items[0].Name != "dronebl" || items[0].Kind != "artifact" {
		t.Fatalf("unexpected artifact item: %#v", items[0])
	}
	if !items[0].Enabled {
		t.Fatalf("expected artifact to be enabled: %#v", items[0])
	}
	if !items[0].NextDue.Equal(now) {
		t.Fatalf("expected overdue artifact to be due now, got %v", items[0].NextDue)
	}
}

func TestBuildArtifactItemsUsesArtifactFailureStateForRetryBackoff(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	root := t.TempDir()

	cfg := config.New()
	cfg.Artifacts["dronebl"] = &config.Artifact{
		Name:      "dronebl",
		Type:      config.ArtifactTypeDroneBLBuildzone,
		Frequency: 60,
	}
	rt := engine.Runtime{
		BaseDir:                       filepath.Join(root, "base"),
		LibDir:                        filepath.Join(root, "lib"),
		IgnoreRepeatingDownloadErrors: 10,
	}
	if err := os.MkdirAll(filepath.Join(rt.LibDir, "artifacts", "dronebl"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rt.LibDir, "artifacts", "dronebl", "enabled"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	items := BuildArtifactItems(cfg, rt, []cache.Entry{{
		Name:             "dronebl",
		CheckedDate:      now.Add(-15 * time.Minute).Unix(),
		DownloadFailures: 3,
	}}, false, now)
	if len(items) != 1 {
		t.Fatalf("expected 1 artifact item, got %d", len(items))
	}
	if got := items[0].Detail; !strings.Contains(got, "retry in") {
		t.Fatalf("scheduler detail = %q, want retry/backoff detail", got)
	}
	if got := items[0].Failures; got != 3 {
		t.Fatalf("failures = %d, want 3", got)
	}
	if items[0].CheckedAt.IsZero() {
		t.Fatal("expected non-zero checked_at for artifact retry state")
	}
}

func TestManualRecheckArtifactChildWithoutLocalInputQueuesParentArtifact(t *testing.T) {
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
    ipv: ipv4
    output: ipset
    processor:
      - passthrough
    category: attacks
    info: child feed
    maintainer: test
    maintainer_url: https://example.test
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	runner := New(eng, true, nil)

	runner.handleAction(t.Context(), PendingAction{
		Names:   []string{"child"},
		Recheck: true,
		Reason:  runreason.ReasonManualRecheck,
	})

	activity := runner.ActivitySnapshot()
	if len(activity.DownloadWaiting) != 1 {
		t.Fatalf("expected 1 queued downloader-stage item for artifact parent, got %#v", activity.DownloadWaiting)
	}
	if activity.DownloadWaiting[0].Name != "dronebl" {
		t.Fatalf("expected parent artifact queued in downloader stage, got %#v", activity.DownloadWaiting)
	}
	if len(activity.ProcessingWaiting) != 0 {
		t.Fatalf("expected no processing queue entry before downloader-stage child materialization, got %#v", activity.ProcessingWaiting)
	}
}

func TestManualRecheckArtifactChildWithLocalInputQueuesChild(t *testing.T) {
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
    ipv: ipv4
    output: ipset
    processor:
      - passthrough
    category: attacks
    info: child feed
    maintainer: test
    maintainer_url: https://example.test
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	childPath := filepath.Join(root, "base", "child.source")
	if err := os.MkdirAll(filepath.Dir(childPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(childPath, []byte("1.2.3.4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := New(eng, true, nil)

	runner.handleAction(t.Context(), PendingAction{
		Names:   []string{"child"},
		Recheck: true,
		Reason:  runreason.ReasonManualRecheck,
	})

	activity := runner.ActivitySnapshot()
	if len(activity.DownloadWaiting) != 1 {
		t.Fatalf("expected 1 queued downloader-stage item for artifact child, got %#v", activity.DownloadWaiting)
	}
	if activity.DownloadWaiting[0].Name != "child" {
		t.Fatalf("expected child queued in downloader stage, got %#v", activity.DownloadWaiting)
	}
}

func TestManualRecheckWithoutNamesDoesNothing(t *testing.T) {
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
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	runner := New(eng, true, nil)

	runner.handleAction(t.Context(), PendingAction{
		Recheck: true,
		Reason:  runreason.ReasonManualRecheck,
	})

	activity := runner.ActivitySnapshot()
	if len(activity.DownloadWaiting) != 0 || len(activity.ProcessingWaiting) != 0 {
		t.Fatalf("expected no queued work for unnamed manual recheck, got %#v", activity)
	}
}

func TestManualReprocessWithoutLocalStateDoesNotQueue(t *testing.T) {
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
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	runner := New(eng, true, nil)

	runner.handleAction(t.Context(), PendingAction{
		Names:     []string{"sample"},
		Reprocess: true,
		Reason:    runreason.ReasonManualReprocess,
	})

	activity := runner.ActivitySnapshot()
	if len(activity.ProcessingWaiting) != 0 {
		t.Fatalf("expected no processing queue entry without local input, got %#v", activity.ProcessingWaiting)
	}
}

func TestEnqueueProcessingWhileActiveDefersUntilBatchCompletes(t *testing.T) {
	runner := &Runner{
		processing: processingLoopState{
			waiting:  map[string]queuedWork{},
			active:   map[string]ActiveQueueFeed{"sample": {Name: "sample"}},
			deferred: map[string]queuedWork{},
		},
	}
	queuedAt := time.Unix(1_700_000_000, 0).UTC()
	runner.enqueueProcessing(queuedWork{
		Name:     "sample",
		Reason:   runreason.ReasonManualReprocess,
		QueuedAt: queuedAt,
	})
	if len(runner.processing.waiting) != 0 {
		t.Fatalf("expected no waiting entry while sample is active, got %#v", runner.processing.waiting)
	}
	if len(runner.processing.deferred) != 1 {
		t.Fatalf("expected deferred reprocess entry, got %#v", runner.processing.deferred)
	}

	released := runner.finishProcessing([]queuedWork{{Name: "sample", QueuedAt: queuedAt}}, false)
	if !released {
		t.Fatalf("expected deferred processing request to be released")
	}
	if len(runner.processing.waiting) != 1 || runner.processing.waiting["sample"].Name != "sample" {
		t.Fatalf("expected deferred work to move into waiting queue, got %#v", runner.processing.waiting)
	}
}

func TestActivitySnapshotHidesDatabaseSourcesFromProcessingQueue(t *testing.T) {
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
  geolite2_country:
    url: https://example.test/geolite2-country.zip
    frequency: 1440
    use: [geoip]
    format: maxmind_country_csv
    info: geo provider
    maintainer: test
    maintainer_url: https://example.test
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	runner := New(eng, true, nil)
	runner.enqueueProcessing(queuedWork{Name: "sample", Reason: runreason.ReasonScheduledDue, QueuedAt: time.Now().UTC()})
	runner.enqueueProcessing(queuedWork{Name: "geolite2_country", Reason: runreason.ReasonScheduledDue, QueuedAt: time.Now().UTC()})

	activity := runner.ActivitySnapshot()
	if len(activity.ProcessingWaiting) != 1 {
		t.Fatalf("expected only real feeds in processing queue snapshot, got %#v", activity.ProcessingWaiting)
	}
	if activity.ProcessingWaiting[0].Name != "sample" {
		t.Fatalf("expected sample to remain visible, got %#v", activity.ProcessingWaiting)
	}
}

func TestActivitySnapshotExposesProcessingBatchWithoutDatabaseSources(t *testing.T) {
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
  geolite2_country:
    url: https://example.test/geolite2-country.zip
    frequency: 1440
    use: [geoip]
    format: maxmind_country_csv
    info: geo provider
    maintainer: test
    maintainer_url: https://example.test
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	runner := New(eng, true, nil)
	startedAt := time.Now().UTC()
	runner.activeFeedSnapshot = func() []engine.ActiveFeed {
		return []engine.ActiveFeed{
			{Name: "sample", Reason: runreason.ReasonManualRun, StartedAt: startedAt},
			{Name: "geolite2_country", Reason: runreason.ReasonManualRun, StartedAt: startedAt},
		}
	}

	activity := runner.ActivitySnapshot()
	if len(activity.ProcessingActive) != 1 {
		t.Fatalf("expected only real feeds in processing batch snapshot, got %#v", activity.ProcessingActive)
	}
	if activity.ProcessingActive[0].Name != "sample" {
		t.Fatalf("expected sample to remain visible, got %#v", activity.ProcessingActive)
	}
}

func TestActivitySnapshotUsesRealActiveFeedsInsteadOfDrainedBatchLedger(t *testing.T) {
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
  sample_1d:
    url: internal://retention_window?minutes=1440&parent=sample
    frequency: 0
    ipv: ipv4
    output: ipset
    category: attacks
    info: sample derivative
    maintainer: test
    maintainer_url: https://example.test
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	runner := New(eng, true, nil)
	startedAt := time.Now().UTC()
	runner.processing.active["sample"] = ActiveQueueFeed{Name: "sample", Reason: runreason.ReasonManualRun, StartedAt: startedAt}
	runner.processing.active["sample_1d"] = ActiveQueueFeed{Name: "sample_1d", Reason: runreason.ReasonDependencyUpdate, StartedAt: startedAt}
	runner.activeFeedSnapshot = func() []engine.ActiveFeed {
		return []engine.ActiveFeed{
			{Name: "sample_1d", Reason: runreason.ReasonDependencyUpdate, StartedAt: startedAt},
		}
	}

	activity := runner.ActivitySnapshot()
	if len(activity.ProcessingActive) != 1 {
		t.Fatalf("expected only the real in-flight feed, got %#v", activity.ProcessingActive)
	}
	if activity.ProcessingActive[0].Name != "sample_1d" {
		t.Fatalf("expected dynamic derivative to be exposed as active, got %#v", activity.ProcessingActive)
	}
}
