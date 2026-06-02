package web

import (
	"encoding/json"
	"fmt"
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

	feed := buildSourceFeed(nil, "sample_1d", src, nil, schedIdx, nil, feedhealth.Policy{}, now, false, nil)

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

	feed := buildSourceFeed(eng, "merged", eng.Config().Sources["merged"], nil, nil, nil, feedhealth.Policy{}, time.Now().UTC(), true, nil)

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

	feed := buildSourceFeed(nil, "sample", src, nil, schedIdx, nil, feedhealth.Policy{}, now, false, nil)

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

	feed := buildSourceFeed(nil, "c2_tracker", src, nil, nil, nil, feedhealth.Policy{}, now, false, nil)

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

	feed := buildSourceFeed(nil, "critical_infra", src, nil, nil, nil, feedhealth.Policy{}, now, false, nil)

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
