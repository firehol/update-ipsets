package web

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/feedhealth"
	"github.com/firehol/update-ipsets/pkg/scheduler"
)

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

	var fullPayload adminStatus
	fullStatus, _ := server.getJSON(t, "/api/v1/admin/status?mode=full", &fullPayload)
	if fullStatus != http.StatusOK {
		t.Fatalf("admin full status HTTP status = %d, want 200", fullStatus)
	}
	if got := fullPayload.Scheduler.Items; len(got) != 1 || got[0].Name != "cached-sentinel" {
		t.Fatalf("full status scheduler items = %+v, want cached sentinel without rebuild", got)
	}
}

func TestAdminScheduleUsesCachedSchedulerSnapshotWithoutRebuild(t *testing.T) {
	opts := Options{
		EnableAll:                 true,
		AdminAuthMode:             AdminAuthModeDisabled,
		AllowUnauthenticatedAdmin: true,
	}
	eng, _ := testHandler(t, opts)
	stale := scheduler.Snapshot{
		GeneratedAt: time.Unix(1_700_000_000, 0).UTC(),
		Items: []scheduler.Item{{
			Name:             "cached-schedule-sentinel",
			Kind:             "source",
			Enabled:          true,
			FrequencyMinutes: 30,
			Failures:         2,
			CheckedAt:        time.Unix(1_700_000_001, 0).UTC(),
			NextDue:          time.Unix(1_700_000_002, 0).UTC(),
			AvgUpdateMins:    13,
			MinUpdateMins:    7,
			MaxUpdateMins:    19,
			LastError:        "cached schedule error",
			Detail:           "cached schedule detail",
		}},
	}
	if err := scheduler.SaveSnapshot(filepath.Join(eng.Runtime().CacheDir, "scheduler-state.json"), stale); err != nil {
		t.Fatalf("write scheduler snapshot: %v", err)
	}
	runner := scheduler.New(eng, true, nil)
	server := newWebHTTPTestServer(t, newHandler(eng, opts, runner))

	var payload struct {
		GeneratedAt int64                `json:"generated_at"`
		Items       []adminScheduleEntry `json:"items"`
	}
	status, _ := server.getJSON(t, "/api/v1/admin/schedule", &payload)
	if status != http.StatusOK {
		t.Fatalf("admin schedule HTTP status = %d, want 200", status)
	}
	if len(payload.Items) != 1 || payload.Items[0].Name != "cached-schedule-sentinel" {
		t.Fatalf("admin schedule items = %+v, want cached sentinel without rebuild", payload.Items)
	}
	got := payload.Items[0]
	if got.AvgUpdateMins != 13 || got.MinUpdateMins != 7 || got.MaxUpdateMins != 19 || got.LastError != "cached schedule error" {
		t.Fatalf("admin schedule did not preserve cached cadence/error fields: %+v", got)
	}
	if got.Detail != "cached schedule detail" || got.NextDue != time.Unix(1_700_000_002, 0).Unix() {
		t.Fatalf("admin schedule did not preserve cached schedule state: %+v", got)
	}
}

func TestAdminFeedsUsesCachedSchedulerSnapshot(t *testing.T) {
	opts := Options{
		EnableAll:                 true,
		AdminAuthMode:             AdminAuthModeDisabled,
		AllowUnauthenticatedAdmin: true,
	}
	eng, _ := testHandler(t, opts)
	snapshot := scheduler.Snapshot{
		GeneratedAt: time.Unix(1_700_000_000, 0).UTC(),
		Items: []scheduler.Item{{
			Name:             "sample",
			Enabled:          true,
			HealthClass:      string(feedhealth.ClassHealthy),
			FrequencyMinutes: 1,
			File:             "sample.ipset",
			Source:           "sample.source",
			PublicURL:        "https://example.test/sample.ipset",
			Hash:             "cached-hash",
			Entries:          7,
			EntriesMin:       5,
			EntriesMax:       9,
			UniqueIPs:        11,
			IPsMin:           10,
			IPsMax:           12,
			AvgUpdateMins:    13,
			MinUpdateMins:    7,
			MaxUpdateMins:    19,
			Version:          23,
			Failures:         3,
			CheckedAt:        time.Unix(1_700_000_000, 0).UTC(),
			UpdatedAt:        time.Unix(1_700_000_100, 0).UTC(),
			ProcessedAt:      time.Unix(1_700_000_150, 0).UTC(),
			StartedAt:        time.Unix(1_700_000_140, 0).UTC(),
			ClockSkewSeconds: 17,
			LastStatus:       "download_failed",
			LastRunReason:    "manual_run",
			LastError:        "cached scheduler sentinel",
			LastProcessingMS: 29,
			NextDue:          time.Unix(1_700_000_200, 0).UTC(),
			Detail:           "cached scheduler detail",
		}},
	}
	if err := scheduler.SaveSnapshot(filepath.Join(eng.Runtime().CacheDir, "scheduler-state.json"), snapshot); err != nil {
		t.Fatalf("write scheduler snapshot: %v", err)
	}
	runner := scheduler.New(eng, true, nil)
	server := newWebHTTPTestServer(t, newHandler(eng, opts, runner))

	var feeds []adminFeed
	status, _ := server.getJSON(t, "/api/v1/admin/feeds", &feeds)
	if status != http.StatusOK {
		t.Fatalf("admin feeds HTTP status = %d, want 200", status)
	}
	var feed adminFeed
	found := false
	for _, row := range feeds {
		if row.Name == "sample" {
			feed = row
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("admin feeds did not include sample row: %+v", feeds)
	}
	if feed.Entries != 7 || feed.UniqueIPs != 11 || feed.LastRunReason != "manual_run" || feed.LastError != "cached scheduler sentinel" {
		t.Fatalf("admin feed row did not use cached scheduler state: %+v", feed)
	}
	if feed.LastCheck != 1_700_000_000 || feed.LastUpdate != 1_700_000_100 || feed.NextCheck != 1_700_000_200 {
		t.Fatalf("admin feed timestamps did not use cached scheduler state: %+v", feed)
	}
	if feed.Health.Class != feedhealth.ClassHealthy {
		t.Fatalf("admin feed health = %q, want cached healthy", feed.Health.Class)
	}

	var detail adminFeedDetail
	detailStatus, _ := server.getJSON(t, "/api/v1/admin/feeds/sample", &detail)
	if detailStatus != http.StatusOK {
		t.Fatalf("admin feed detail HTTP status = %d, want 200", detailStatus)
	}
	if detail.File != "sample.ipset" || detail.Hash != "cached-hash" || detail.PublicURL != "https://example.test/sample.ipset" {
		t.Fatalf("admin feed detail did not use cached file identity: %+v", detail)
	}
	if detail.ProcessedDate != 1_700_000_150 || detail.StartedDate != 1_700_000_140 || detail.Version != 23 {
		t.Fatalf("admin feed detail did not use cached publication state: %+v", detail)
	}
	if detail.EntriesMin != 5 || detail.EntriesMax != 9 || detail.IPsMin != 10 || detail.IPsMax != 12 {
		t.Fatalf("admin feed detail did not use cached range statistics: %+v", detail)
	}
	if detail.AvgUpdateMins != 13 || detail.MinUpdateMins != 7 || detail.MaxUpdateMins != 19 {
		t.Fatalf("admin feed detail did not use cached cadence statistics: %+v", detail)
	}
}

func TestAdminFeedDetailUsesCachedSchedulerRowsForMergeComposition(t *testing.T) {
	opts := Options{
		EnableAll:                 true,
		AdminAuthMode:             AdminAuthModeDisabled,
		AllowUnauthenticatedAdmin: true,
	}
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
  feed_health_single_observation_grace_minutes: 60
  feed_health_default_healthy_cadence_minutes: 60
  feed_health_default_risky_cadence_minutes: 120
  feed_health_archival_threshold_minutes: 60
sources:
  included:
    frequency: 60
    ipv: ipv4
    output: netset
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
	now := time.Unix(1_800_000_000, 0).UTC()
	snapshot := scheduler.Snapshot{
		GeneratedAt: now,
		Items: []scheduler.Item{
			{Name: "included", Enabled: true, HealthClass: string(feedhealth.ClassHealthy), Entries: 1, UpdatedAt: now, ProcessedAt: now},
			{Name: "subtracted", Enabled: true, HealthClass: string(feedhealth.ClassHealthy), Entries: 1, UpdatedAt: now, ProcessedAt: now},
			{Name: "merged", Enabled: true, HealthClass: string(feedhealth.ClassHealthy), Entries: 1, UpdatedAt: now, ProcessedAt: now},
		},
	}
	if err := scheduler.SaveSnapshot(filepath.Join(eng.Runtime().CacheDir, "scheduler-state.json"), snapshot); err != nil {
		t.Fatalf("write scheduler snapshot: %v", err)
	}
	runner := scheduler.New(eng, true, nil)
	server := newWebHTTPTestServer(t, newHandler(eng, opts, runner))

	var detail adminFeedDetail
	status, _ := server.getJSON(t, "/api/v1/admin/feeds/merged", &detail)
	if status != http.StatusOK {
		t.Fatalf("admin feed detail HTTP status = %d, want 200", status)
	}
	if len(detail.MergeIncluded) != 1 || detail.MergeIncluded[0].Name != "included" {
		t.Fatalf("merge included = %+v, want cached included parent", detail.MergeIncluded)
	}
	if len(detail.MergeSubtracted) != 1 || detail.MergeSubtracted[0].Name != "subtracted" {
		t.Fatalf("merge subtracted = %+v, want cached subtractive parent", detail.MergeSubtracted)
	}
	if len(detail.MergeExcluded) != 0 {
		t.Fatalf("merge excluded = %+v, want none", detail.MergeExcluded)
	}
}

func TestAdminArtifactsUsesCachedSchedulerArtifactSnapshot(t *testing.T) {
	opts := Options{
		EnableAll:                 true,
		AdminAuthMode:             AdminAuthModeDisabled,
		AllowUnauthenticatedAdmin: true,
	}
	eng, _ := testHandlerWithArtifactCatalog(t, opts)
	snapshot := scheduler.Snapshot{
		GeneratedAt: time.Unix(1_700_000_000, 0).UTC(),
		ArtifactItems: []scheduler.Item{{
			Name:             "dronebl",
			Enabled:          true,
			FrequencyMinutes: 60,
			Failures:         4,
			CheckedAt:        time.Unix(1_700_000_000, 0).UTC(),
			UpdatedAt:        time.Unix(1_700_000_100, 0).UTC(),
			LastStatus:       "download_failed",
			LastError:        "cached artifact sentinel",
			NextDue:          time.Unix(1_700_000_200, 0).UTC(),
			Detail:           "cached artifact detail",
		}},
	}
	if err := scheduler.SaveSnapshot(filepath.Join(eng.Runtime().CacheDir, "scheduler-state.json"), snapshot); err != nil {
		t.Fatalf("write scheduler snapshot: %v", err)
	}
	runner := scheduler.New(eng, true, nil)
	server := newWebHTTPTestServer(t, newHandler(eng, opts, runner))

	var artifacts []adminArtifact
	status, _ := server.getJSON(t, "/api/v1/admin/artifacts", &artifacts)
	if status != http.StatusOK {
		t.Fatalf("admin artifacts HTTP status = %d, want 200", status)
	}
	if len(artifacts) != 1 {
		t.Fatalf("admin artifact rows = %d, want 1: %+v", len(artifacts), artifacts)
	}
	artifact := artifacts[0]
	if artifact.Name != "dronebl" || artifact.DownloadFailures != 4 || artifact.LastError != "cached artifact sentinel" {
		t.Fatalf("admin artifact row did not use cached scheduler state: %+v", artifact)
	}
	if artifact.LastCheck != 1_700_000_000 || artifact.LastUpdate != 1_700_000_100 || artifact.NextCheck != 1_700_000_200 {
		t.Fatalf("admin artifact timestamps did not use cached scheduler state: %+v", artifact)
	}
	if artifact.SchedulerDetail != "cached artifact detail" {
		t.Fatalf("admin artifact scheduler detail = %q, want cached detail", artifact.SchedulerDetail)
	}
}
