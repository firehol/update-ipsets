package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/feedhealth"
	"github.com/firehol/update-ipsets/pkg/scheduler"
)

func TestDeriveFeedStatusUsesLiveProcessingState(t *testing.T) {
	feed := &adminFeed{
		Name:             "sample",
		Enabled:          true,
		LastStatus:       "same",
		LastCheck:        1,
		FrequencyMinutes: 1,
	}

	if got := deriveFeedStatus(feed, adminLiveState{ProcessingActive: true}); got != "processing" {
		t.Fatalf("expected processing for active feed, got %q", got)
	}
}

func TestDeriveFeedStatusUsesLiveDownloadQueueState(t *testing.T) {
	feed := &adminFeed{
		Name:             "sample",
		Enabled:          true,
		LastStatus:       "same",
		LastCheck:        1,
		FrequencyMinutes: 1,
	}

	if got := deriveFeedStatus(feed, adminLiveState{DownloadWaiting: true}); got != "waiting_download" {
		t.Fatalf("expected waiting_download for queued feed, got %q", got)
	}
}

func TestDeriveFeedStatusFallsBackToErrorWhenIdle(t *testing.T) {
	feed := &adminFeed{
		Name:             "sample",
		Enabled:          true,
		LastStatus:       "same",
		LastCheck:        1,
		LastError:        "boom",
		FrequencyMinutes: 1,
	}

	if got := deriveFeedStatus(feed, adminLiveState{}); got != "error" {
		t.Fatalf("expected error for idle failing feed, got %q", got)
	}
}

func TestPopulateAdminFeedStatusMetaUsesOperatorFacingProcessingMeaning(t *testing.T) {
	feed := &adminFeed{
		Name:       "sample",
		LastStatus: "parse_failed",
		LastError:  "bad input",
	}

	populateAdminFeedStatusMeta(feed)

	if got := feed.LastStatusLabel; got != "Processing could not parse local input" {
		t.Fatalf("last_status_label = %q, want processing parse label", got)
	}
	if got := feed.LastProblemClass; got != adminProblemClassProcessing {
		t.Fatalf("last_problem_class = %q, want %q", got, adminProblemClassProcessing)
	}
}

func TestPopulateAdminFeedStatusMetaUsesOperatorFacingDownloaderMeaning(t *testing.T) {
	feed := &adminFeed{
		Name:             "sample",
		LastStatus:       "history_snapshot_failed",
		LastError:        "disk full",
		DownloadFailures: 2,
	}

	populateAdminFeedStatusMeta(feed)

	if got := feed.LastStatusLabel; got != "Downloader could not update retained history snapshot" {
		t.Fatalf("last_status_label = %q, want downloader snapshot label", got)
	}
	if got := feed.LastProblemClass; got != adminProblemClassDownloader {
		t.Fatalf("last_problem_class = %q, want %q", got, adminProblemClassDownloader)
	}
}

func TestPopulateAdminFeedStatusMetaResolvesAmbiguousFailedStatusByFailureStreak(t *testing.T) {
	downloaderFeed := &adminFeed{
		Name:             "download-failure",
		LastStatus:       "failed",
		LastError:        "fetch failed",
		DownloadFailures: 3,
	}
	populateAdminFeedStatusMeta(downloaderFeed)
	if got := downloaderFeed.LastProblemClass; got != adminProblemClassDownloader {
		t.Fatalf("downloader last_problem_class = %q, want %q", got, adminProblemClassDownloader)
	}

	processingFeed := &adminFeed{
		Name:       "processing-failure",
		LastStatus: "failed",
		LastError:  "source file does not exist",
	}
	populateAdminFeedStatusMeta(processingFeed)
	if got := processingFeed.LastProblemClass; got != adminProblemClassProcessing {
		t.Fatalf("processing last_problem_class = %q, want %q", got, adminProblemClassProcessing)
	}
}

func TestPopulateAdminArtifactStatusMetaUsesOperatorFacingDownloaderMeaning(t *testing.T) {
	artifact := &adminArtifact{
		Name:       "dronebl",
		LastStatus: "download_failed",
		LastError:  "403 forbidden",
	}

	populateAdminArtifactStatusMeta(artifact)

	if got := artifact.LastStatusLabel; got != "Downloader fetch or materialization failed" {
		t.Fatalf("last_status_label = %q, want downloader failure label", got)
	}
	if got := artifact.LastProblemClass; got != adminProblemClassDownloader {
		t.Fatalf("last_problem_class = %q, want %q", got, adminProblemClassDownloader)
	}
}

func TestSanitizeSchedulerSnapshotClampsOutOfRangeTimes(t *testing.T) {
	snap := scheduler.Snapshot{
		GeneratedAt: time.Date(12000, time.January, 1, 0, 0, 0, 0, time.UTC),
		Items: []scheduler.Item{{
			Name:      "sample",
			CheckedAt: time.Date(12001, time.January, 1, 0, 0, 0, 0, time.UTC),
			NextDue:   time.Date(12002, time.January, 1, 0, 0, 0, 0, time.UTC),
		}},
	}

	sanitized := sanitizeSchedulerSnapshot(snap)
	if !sanitized.GeneratedAt.IsZero() {
		t.Fatalf("expected generated_at to be zero, got %v", sanitized.GeneratedAt)
	}
	if !sanitized.Items[0].CheckedAt.IsZero() {
		t.Fatalf("expected checked_at to be zero, got %v", sanitized.Items[0].CheckedAt)
	}
	if !sanitized.Items[0].NextDue.IsZero() {
		t.Fatalf("expected next_due to be zero, got %v", sanitized.Items[0].NextDue)
	}
	if _, err := json.Marshal(sanitized); err != nil {
		t.Fatalf("expected sanitized snapshot to marshal, got %v", err)
	}
}

func TestAdminStatusModeSelectsLightOnlyForExactLightMode(t *testing.T) {
	eng, handler := testHandler(t, Options{
		EnableAll:                 true,
		AdminAuthMode:             AdminAuthModeDisabled,
		AllowUnauthenticatedAdmin: true,
	})
	eng.ObserveOperation("metadata.comparison_pair_overlap", time.Second)
	server := newWebHTTPTestServer(t, handler)

	var payload map[string]any
	status, _ := server.getJSON(t, "/api/v1/admin/status", &payload)
	if status != http.StatusOK {
		t.Fatalf("admin status HTTP status = %d, want 200", status)
	}
	enginePayload, ok := payload["engine"].(map[string]any)
	if !ok {
		t.Fatalf("admin status engine payload has type %T", payload["engine"])
	}
	for _, field := range []string{"current_metrics", "last_metrics", "lifetime_metrics", "config_path", "base_dir", "last_report"} {
		if _, ok := enginePayload[field]; ok {
			t.Fatalf("default admin status included %s; default polling must use the lightweight engine snapshot", field)
		}
	}
	if _, ok := enginePayload["engine_lane"]; !ok {
		t.Fatal("default admin status omitted engine_lane")
	}
	if _, ok := enginePayload["git_lane"]; !ok {
		t.Fatal("default admin status omitted git_lane")
	}

	var lightPayload map[string]any
	lightStatus, _ := server.getJSON(t, "/api/v1/admin/status?mode=light", &lightPayload)
	if lightStatus != http.StatusOK {
		t.Fatalf("admin light status HTTP status = %d, want 200", lightStatus)
	}
	lightEnginePayload, ok := lightPayload["engine"].(map[string]any)
	if !ok {
		t.Fatalf("admin light status engine payload has type %T", lightPayload["engine"])
	}
	for _, field := range []string{"current_metrics", "last_metrics", "lifetime_metrics", "config_path", "base_dir", "last_report"} {
		if _, ok := lightEnginePayload[field]; ok {
			t.Fatalf("admin light status included %s; live polling must use the lightweight engine snapshot", field)
		}
	}

	var fullPayload map[string]any
	fullStatus, _ := server.getJSON(t, "/api/v1/admin/status?mode=full", &fullPayload)
	if fullStatus != http.StatusOK {
		t.Fatalf("admin full status HTTP status = %d, want 200", fullStatus)
	}
	fullEnginePayload, ok := fullPayload["engine"].(map[string]any)
	if !ok {
		t.Fatalf("admin full status engine payload has type %T", fullPayload["engine"])
	}
	for _, field := range []string{"config_path", "base_dir", "lifetime_metrics"} {
		if _, ok := fullEnginePayload[field]; !ok {
			t.Fatalf("admin full status omitted full field %s", field)
		}
	}

	var unknownPayload map[string]any
	unknownStatus, _ := server.getJSON(t, "/api/v1/admin/status?mode=unexpected", &unknownPayload)
	if unknownStatus != http.StatusOK {
		t.Fatalf("admin unknown-mode status HTTP status = %d, want 200", unknownStatus)
	}
	unknownEnginePayload, ok := unknownPayload["engine"].(map[string]any)
	if !ok {
		t.Fatalf("admin unknown-mode status engine payload has type %T", unknownPayload["engine"])
	}
	for _, field := range []string{"current_metrics", "last_metrics", "lifetime_metrics", "config_path", "base_dir", "last_report"} {
		if _, ok := unknownEnginePayload[field]; ok {
			t.Fatalf("admin unknown-mode status included %s; unknown modes must fall back to light", field)
		}
	}
}

func TestAdminStatusLightUsesRuntimeStatsSampler(t *testing.T) {
	setDetailedStatusCacheForTest(t, time.Now().UTC(), detailedSystemInfo{
		Goroutines: 321,
		HeapSys:    654_321,
		DiskFree:   "cached sentinel",
		RSSKB:      987,
	})

	_, handler := testHandler(t, Options{
		EnableAll:                 true,
		AdminAuthMode:             AdminAuthModeDisabled,
		AllowUnauthenticatedAdmin: true,
	})
	server := newWebHTTPTestServer(t, handler)

	var lightPayload adminStatusLight
	status, _ := server.getJSON(t, "/api/v1/admin/status?mode=light", &lightPayload)
	if status != http.StatusOK {
		t.Fatalf("admin light status HTTP status = %d, want 200", status)
	}
	if lightPayload.System.Goroutines != 321 {
		t.Fatalf("admin light status goroutines = %d, want cached sampler value 321", lightPayload.System.Goroutines)
	}
	if lightPayload.System.HeapSys != 654_321 {
		t.Fatalf("admin light status heap_sys = %d, want cached sampler value 654321", lightPayload.System.HeapSys)
	}
	if lightPayload.System.DiskFree != "cached sentinel" {
		t.Fatalf("admin light status disk_free = %q, want cached sampler value", lightPayload.System.DiskFree)
	}
	if lightPayload.System.RSSKB != 987 {
		t.Fatalf("admin light status rss_kb = %d, want cached sampler value 987", lightPayload.System.RSSKB)
	}
}

func TestAdminStatusLightIncludesEngineLane(t *testing.T) {
	eng, handler := testHandler(t, Options{
		EnableAll:                 true,
		AdminAuthMode:             AdminAuthModeDisabled,
		AllowUnauthenticatedAdmin: true,
	})
	server := newWebHTTPTestServer(t, handler)

	var lightPayload adminStatusLight
	status, _ := server.getJSON(t, "/api/v1/admin/status?mode=light", &lightPayload)
	if status != http.StatusOK {
		t.Fatalf("admin light status HTTP status = %d, want 200", status)
	}
	wantLimit := eng.Runtime().EngineLaneWorkers()
	if lightPayload.Engine.EngineLane.Limit != wantLimit {
		t.Fatalf("admin light status engine lane limit = %d, want %d", lightPayload.Engine.EngineLane.Limit, wantLimit)
	}
	if lightPayload.Engine.MaxEngineLaneWorkers != wantLimit {
		t.Fatalf("admin light status max_engine_lane_workers = %d, want %d", lightPayload.Engine.MaxEngineLaneWorkers, wantLimit)
	}
	if lightPayload.Engine.BackgroundLimit != lightPayload.Engine.EngineLane.Limit {
		t.Fatalf("admin light status background_limit = %d, want engine lane limit %d", lightPayload.Engine.BackgroundLimit, lightPayload.Engine.EngineLane.Limit)
	}
	if lightPayload.Engine.BackgroundRunning != lightPayload.Engine.EngineLane.ActiveCount {
		t.Fatalf("admin light status background_running = %d, want engine lane active count %d", lightPayload.Engine.BackgroundRunning, lightPayload.Engine.EngineLane.ActiveCount)
	}
}

func TestAdminStatusLightIncludesIntegritySummary(t *testing.T) {
	eng, handler := testHandler(t, Options{
		EnableAll:                 true,
		AdminAuthMode:             AdminAuthModeDisabled,
		AllowUnauthenticatedAdmin: true,
	})
	eng.StorePipelineIntegrityFindings(engine.IntegrityOptions{}, []engine.IntegrityFinding{{
		Feed:   "sample",
		Reason: "missing secondary files",
	}}, nil)
	eng.StoreEntityIntegrityFindings([]engine.EntityIntegrityFinding{{
		Scope:   "global",
		Kind:    "version_missing",
		Subject: "entity_artifacts",
		Reason:  "missing entity artifact version",
	}}, nil)
	server := newWebHTTPTestServer(t, handler)

	var lightPayload adminStatusLight
	status, _ := server.getJSON(t, "/api/v1/admin/status?mode=light", &lightPayload)
	if status != http.StatusOK {
		t.Fatalf("admin light status HTTP status = %d, want 200", status)
	}
	if lightPayload.Engine.PipelineIntegrityCache.CacheState != engine.IntegrityCacheFresh {
		t.Fatalf("pipeline integrity cache state = %q, want %q", lightPayload.Engine.PipelineIntegrityCache.CacheState, engine.IntegrityCacheFresh)
	}
	if lightPayload.Engine.PipelineIntegrityCache.Count != 1 {
		t.Fatalf("pipeline integrity cache count = %d, want 1", lightPayload.Engine.PipelineIntegrityCache.Count)
	}
	if lightPayload.Engine.EntityIntegrityCache.CacheState != engine.IntegrityCacheFresh {
		t.Fatalf("entity integrity cache state = %q, want %q", lightPayload.Engine.EntityIntegrityCache.CacheState, engine.IntegrityCacheFresh)
	}
	if lightPayload.Engine.EntityIntegrityCache.Count != 1 {
		t.Fatalf("entity integrity cache count = %d, want 1", lightPayload.Engine.EntityIntegrityCache.Count)
	}
}

func TestAdminStatusLightIncludesFeedHealthSummary(t *testing.T) {
	_, handler := testHandlerWithCatalog(t, Options{
		EnableAll:                 true,
		AdminAuthMode:             AdminAuthModeDisabled,
		AllowUnauthenticatedAdmin: true,
	})
	server := newWebHTTPTestServer(t, handler)

	var feeds []adminFeed
	feedsStatus, _ := server.getJSON(t, "/api/v1/admin/feeds", &feeds)
	if feedsStatus != http.StatusOK {
		t.Fatalf("admin feeds HTTP status = %d, want 200", feedsStatus)
	}
	if len(feeds) == 0 {
		t.Fatal("admin feeds fixture returned no rows")
	}
	want := summarizeAdminFeeds(len(feeds), feeds)
	if want.TotalEnabled == 0 {
		t.Fatalf("admin feeds fixture produced empty enabled summary: %+v", want)
	}

	var lightPayload adminStatusLight
	lightStatus, _ := server.getJSON(t, "/api/v1/admin/status?mode=light", &lightPayload)
	if lightStatus != http.StatusOK {
		t.Fatalf("admin light status HTTP status = %d, want 200", lightStatus)
	}
	if lightPayload.Feeds != want {
		t.Fatalf("admin light status feeds summary = %+v, want %+v", lightPayload.Feeds, want)
	}
}

func TestAdminStatusLightUsesCachedSchedulerSnapshotWithoutRebuild(t *testing.T) {
	opts := Options{
		EnableAll:                 true,
		AdminAuthMode:             AdminAuthModeDisabled,
		AllowUnauthenticatedAdmin: true,
	}
	eng, _ := testHandler(t, opts)
	stale := scheduler.Snapshot{
		GeneratedAt: time.Unix(1_700_000_000, 0).UTC(),
		Items: []scheduler.Item{{
			Name:        "cached-sentinel",
			Enabled:     true,
			HealthClass: string(feedhealth.ClassHealthy),
			Entries:     7,
			UniqueIPs:   11,
			CheckedAt:   time.Unix(1_700_000_000, 0).UTC(),
			NextDue:     time.Unix(1_700_000_001, 0).UTC(),
		}},
	}
	if err := scheduler.SaveSnapshot(filepath.Join(eng.Runtime().CacheDir, "scheduler-state.json"), stale); err != nil {
		t.Fatalf("write scheduler snapshot: %v", err)
	}
	runner := scheduler.New(eng, true, nil)
	server := newWebHTTPTestServer(t, newHandler(eng, opts, runner))

	var lightPayload adminStatusLight
	status, _ := server.getJSON(t, "/api/v1/admin/status?mode=light", &lightPayload)
	if status != http.StatusOK {
		t.Fatalf("admin light status HTTP status = %d, want 200", status)
	}
	if got := lightPayload.Scheduler.Items; len(got) != 1 || got[0].Name != "cached-sentinel" {
		t.Fatalf("light status scheduler items = %+v, want cached sentinel without rebuild", got)
	}
	if lightPayload.Feeds.Healthy != 1 || lightPayload.Feeds.TotalEntries != 7 || lightPayload.Feeds.TotalUniqueIPs != 11 {
		t.Fatalf("light status cached feed summary = %+v, want healthy cached sentinel counters", lightPayload.Feeds)
	}
}

func TestAdminStatusLightRespondsWhileEngineLaneBusy(t *testing.T) {
	eng, handler := testHandler(t, Options{
		EnableAll:                 true,
		AdminAuthMode:             AdminAuthModeDisabled,
		AllowUnauthenticatedAdmin: true,
	})
	eng.StorePipelineIntegrityFindings(engine.IntegrityOptions{}, []engine.IntegrityFinding{{Feed: "sample"}}, nil)

	releaseLane := make(chan struct{})
	laneStarted := make(chan struct{})
	_, err := eng.QueuePipelineIntegrityReprocess(t.Context(), engine.IntegrityOptions{}, "test_block", func(ctx context.Context, _ []engine.IntegrityFinding) error {
		close(laneStarted)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-releaseLane:
			return nil
		}
	})
	if err != nil {
		t.Fatalf("queue blocking integrity reprocess: %v", err)
	}
	<-laneStarted
	t.Cleanup(func() {
		close(releaseLane)
		waitForEngineLaneClear(t, eng, 2*time.Second)
	})

	server := newWebHTTPTestServer(t, handler)
	var lightPayload map[string]any
	status, _ := server.getJSON(t, "/api/v1/admin/status?mode=light", &lightPayload)
	if status != http.StatusOK {
		t.Fatalf("admin light status while engine lane busy = %d, want 200", status)
	}
	enginePayload, ok := lightPayload["engine"].(map[string]any)
	if !ok {
		t.Fatalf("admin light status engine payload has type %T", lightPayload["engine"])
	}
	if _, ok := enginePayload["engine_lane"]; !ok {
		t.Fatalf("admin light status omitted engine_lane while lane was busy: %#v", enginePayload)
	}
	if _, ok := enginePayload["git_lane"]; !ok {
		t.Fatalf("admin light status omitted git_lane while lane was busy: %#v", enginePayload)
	}
}

func waitForEngineLaneClear(t *testing.T, eng *engine.Engine, timeout time.Duration) {
	t.Helper()

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		snap := eng.StatusSnapshotLight().EngineLane
		if snap.ActiveCount == 0 && snap.WaitingCount == 0 {
			return
		}
		select {
		case <-deadline.C:
			t.Fatalf("engine lane did not clear: %+v", snap)
		case <-ticker.C:
		}
	}
}

func setDetailedStatusCacheForTest(t *testing.T, sampledAt time.Time, info detailedSystemInfo) {
	t.Helper()
	detailedStatusCache.mu.Lock()
	oldSampledAt := detailedStatusCache.sampledAt
	oldInfo := detailedStatusCache.info
	detailedStatusCache.sampledAt = sampledAt
	detailedStatusCache.info = info
	detailedStatusCache.mu.Unlock()
	t.Cleanup(func() {
		detailedStatusCache.mu.Lock()
		detailedStatusCache.sampledAt = oldSampledAt
		detailedStatusCache.info = oldInfo
		detailedStatusCache.mu.Unlock()
	})
}

func TestBuildSourceFeedUsesInputTriggeredScheduleLabelForDerivedFeeds(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	next := now.Add(365 * 24 * time.Hour)
	schedIdx := map[string]*scheduler.Item{
		"sample_1d": {
			Name:    "sample_1d",
			NextDue: next,
			Detail:  "static source (never expires)",
		},
	}
	src := &config.Source{
		Name:       "sample_1d",
		URL:        config.InternalRetentionWindowScheme + "?minutes=1440&parent=sample",
		Frequency:  0,
		Provenance: config.ProvenanceSecondaryRetention,
	}

	feed := buildSourceFeed("sample_1d", src, nil, nil, schedIdx, nil, feedhealth.Policy{}, now, nil)

	if feed.Kind != "retention" {
		t.Fatalf("expected retention kind, got %q", feed.Kind)
	}
	if feed.SchedulerDetail != adminDerivedScheduleLabel {
		t.Fatalf("expected scheduler detail %q, got %q", adminDerivedScheduleLabel, feed.SchedulerDetail)
	}
	if feed.NextCheck != next.Unix() {
		t.Fatalf("expected next_check to preserve scheduler value %d, got %d", next.Unix(), feed.NextCheck)
	}
}

func TestBuildSourceFeedUsesRunnerEnableAllForMergeComposition(t *testing.T) {
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
  included:
    frequency: 60
    ipv: ipv4
    output: netset
    redistributable: false
  subtracted:
    frequency: 60
    ipv: ipv4
    output: netset
merges:
  merged:
    frequency: 60
    ipv: ipv4
    output: netset
    sources: [included]
    exclude: [subtracted]
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(eng.FeedBodyPath("included"), []byte("10.0.0.0/24\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(eng.FeedBodyPath("subtracted"), []byte("10.0.0.0/25\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfgSnap, rt, policy := eng.ConfigRuntimePolicySnapshot()
	mergeCompositions := eng.MergeCompositionsForConfigRuntimePolicy(cfgSnap, rt, policy, true)
	feed := buildSourceFeed("merged", cfgSnap.Sources["merged"], cfgSnap, nil, nil, nil, policy, time.Now().UTC(), mergeCompositions)

	if len(feed.MergeIncluded) != 1 || feed.MergeIncluded[0].Name != "included" {
		t.Fatalf("merge included = %+v, want included", feed.MergeIncluded)
	}
	if len(feed.MergeSubtracted) != 1 || feed.MergeSubtracted[0].Name != "subtracted" {
		t.Fatalf("merge subtracted = %+v, want subtracted", feed.MergeSubtracted)
	}
	if len(feed.MergeExcluded) != 0 {
		t.Fatalf("merge excluded = %+v, want none", feed.MergeExcluded)
	}
	if feed.Redistributable {
		t.Fatal("merge admin row should be non-redistributable when an additive parent is non-redistributable")
	}
}

func TestBuildSourceFeedKeepsOriginalScheduleDetailForPlainSources(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	schedIdx := map[string]*scheduler.Item{
		"sample": {
			Name:    "sample",
			NextDue: now.Add(30 * time.Minute),
			Detail:  "next check in 30 mins (base 30 mins)",
		},
	}
	src := &config.Source{
		Name:      "sample",
		URL:       "https://example.com/list.txt",
		Frequency: 30,
	}

	feed := buildSourceFeed("sample", src, nil, nil, schedIdx, nil, feedhealth.Policy{}, now, nil)

	if feed.Kind != "source" {
		t.Fatalf("expected source kind, got %q", feed.Kind)
	}
	if feed.SchedulerDetail != schedIdx["sample"].Detail {
		t.Fatalf("expected scheduler detail %q, got %q", schedIdx["sample"].Detail, feed.SchedulerDetail)
	}
}

func TestBuildSourceFeedKeepsInfrastructureCategoriesAsSourceKind(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	src := &config.Source{
		Name:      "c2_tracker",
		URL:       "https://example.com/infrastructure.txt",
		Frequency: 60,
		Category:  "malware_infrastructure",
	}

	feed := buildSourceFeed("c2_tracker", src, nil, nil, nil, nil, feedhealth.Policy{}, now, nil)

	if feed.Kind != "source" {
		t.Fatalf("expected source kind, got %q", feed.Kind)
	}
}

func TestBuildSourceFeedKeepsCriticalInfrastructureRoleAsSourceKind(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	src := &config.Source{
		Name:      "critical_infra",
		URL:       "https://example.com/infrastructure.txt",
		Frequency: 60,
		Use:       []string{config.UseCriticalInfrastructure},
	}

	feed := buildSourceFeed("critical_infra", src, nil, nil, nil, nil, feedhealth.Policy{}, now, nil)

	if feed.Kind != "source" {
		t.Fatalf("expected source kind, got %q", feed.Kind)
	}
}

func TestSummarizeAdminFeedsCountsHiddenFeedsInHealthBuckets(t *testing.T) {
	feeds := []adminFeed{
		{
			Name:      "hidden-unmaintained",
			Hidden:    true,
			Enabled:   true,
			Entries:   10,
			UniqueIPs: 20,
			Health:    feedhealth.Snapshot{Class: feedhealth.ClassUnmaintained},
		},
		{
			Name:      "visible-unmaintained",
			Enabled:   true,
			Entries:   5,
			UniqueIPs: 8,
			Health:    feedhealth.Snapshot{Class: feedhealth.ClassUnmaintained},
		},
	}

	got := summarizeAdminFeeds(2, feeds)
	if got.Hidden != 1 {
		t.Fatalf("hidden = %d, want 1", got.Hidden)
	}
	if got.TotalEnabled != 2 {
		t.Fatalf("total_enabled = %d, want 2", got.TotalEnabled)
	}
	if got.Unmaintained != 2 {
		t.Fatalf("unmaintained = %d, want 2", got.Unmaintained)
	}
	if got.Stale != 2 {
		t.Fatalf("stale = %d, want 2", got.Stale)
	}
	if got.TotalEntries != 15 {
		t.Fatalf("total_entries = %d, want 15", got.TotalEntries)
	}
	if got.TotalUniqueIPs != 28 {
		t.Fatalf("total_unique_ips = %d, want 28", got.TotalUniqueIPs)
	}
}
