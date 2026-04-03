package feedhealth_test

import (
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/feedhealth"
)

func TestClassifyUnavailableBecomesArchivedAfterConfiguredArchivedWindow(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	entry := &cache.Entry{
		Name:               "sample",
		ProcessedDate:      now.Add(-200 * time.Minute).Unix(),
		SourceDate:         now.Add(-200 * time.Minute).Unix(),
		CheckedDate:        now.Unix(),
		FailureStartedDate: now.Add(-200 * time.Minute).Unix(),
		DownloadFailures:   5,
		LastStatus:         "download_failed",
		Version:            3,
	}
	src := &config.Source{Name: "sample", Category: "intrusion", Frequency: 60}
	policy := feedhealth.Policy{
		SingleObservationGraceMins: 60,
		ArchivalThresholdMins:      60,
		DefaultThresholds: config.FeedHealthCategoryThresholds{
			HealthyCadenceMins: 60,
			RiskyCadenceMins:   60,
		},
	}

	snap := feedhealth.Classify(entry, src, policy, now)
	if got, want := snap.Class, feedhealth.ClassArchived; got != want {
		t.Fatalf("class = %q, want %q", got, want)
	}
}

func TestClassifyDoesNotArchiveImmediatelyWhenFeedFirstBecomesUnavailable(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	entry := &cache.Entry{
		Name:               "sample",
		ProcessedDate:      now.Add(-120 * time.Minute).Unix(),
		SourceDate:         now.Add(-120 * time.Minute).Unix(),
		CheckedDate:        now.Unix(),
		FailureStartedDate: now.Add(-120 * time.Minute).Unix(),
		DownloadFailures:   2,
		LastStatus:         "download_failed",
		Version:            2,
	}
	src := &config.Source{Name: "sample", Category: "intrusion", Frequency: 60}
	policy := feedhealth.Policy{
		SingleObservationGraceMins: 60,
		ArchivalThresholdMins:      60,
		DefaultThresholds: config.FeedHealthCategoryThresholds{
			HealthyCadenceMins: 60,
			RiskyCadenceMins:   60,
		},
	}

	snap := feedhealth.Classify(entry, src, policy, now)
	if got, want := snap.Class, feedhealth.ClassUnavailable; got != want {
		t.Fatalf("class = %q, want %q", got, want)
	}
}

func TestClassifyArchivesUnavailableFeedWhenLastUsableRefreshIsAlreadyTooOld(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	entry := &cache.Entry{
		Name:               "sample",
		ProcessedDate:      now.Add(-200 * 24 * time.Hour).Unix(),
		SourceDate:         now.Add(-200 * 24 * time.Hour).Unix(),
		CheckedDate:        now.Unix(),
		FailureStartedDate: now.Add(-2 * 24 * time.Hour).Unix(),
		DownloadFailures:   2,
		LastStatus:         "download_failed",
		Version:            25,
		Entries:            10,
		AverageUpdateMins:  60,
	}
	src := &config.Source{Name: "sample", Category: "intrusion", Frequency: 60}
	policy := feedhealth.Policy{
		SingleObservationGraceMins: 60,
		ArchivalThresholdMins:      60 * 24 * 60,
		DefaultThresholds: config.FeedHealthCategoryThresholds{
			HealthyCadenceMins: 60,
			RiskyCadenceMins:   60 * 24 * 14,
		},
	}

	snap := feedhealth.Classify(entry, src, policy, now)
	if got, want := snap.Class, feedhealth.ClassArchived; got != want {
		t.Fatalf("class = %q, want %q", got, want)
	}
}

func TestClassifySuppressesAgeForReferenceAndProviderRoles(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	policy := feedhealth.Policy{
		SingleObservationGraceMins: 60,
		DefaultThresholds: config.FeedHealthCategoryThresholds{
			HealthyCadenceMins: 60,
			RiskyCadenceMins:   120,
		},
	}
	entry := &cache.Entry{
		Name:              "sample",
		ProcessedDate:     now.Add(-10 * time.Hour).Unix(),
		SourceDate:        now.Add(-10 * time.Hour).Unix(),
		CheckedDate:       now.Unix(),
		Entries:           10,
		Version:           5,
		AverageUpdateMins: 60,
	}

	for _, tc := range []struct {
		name string
		use  []string
	}{
		{name: "critical reference", use: []string{config.UseCriticalInfrastructure}},
		{name: "provider context", use: []string{config.UseProviderContext}},
		{name: "asn provider", use: []string{config.UseASN}},
		{name: "geo provider", use: []string{config.UseGeoIP}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := &config.Source{Name: "sample", Category: "provider_infrastructure", Frequency: 60, Use: tc.use}
			snap := feedhealth.Classify(entry, src, policy, now)
			if got, want := snap.Class, feedhealth.ClassHealthy; got != want {
				t.Fatalf("class = %q, want %q", got, want)
			}
			if !snap.ExcludeFromUnmaintained {
				t.Fatal("expected age-based maintenance exclusion to be visible")
			}
		})
	}
}

func TestClassifyKeepsEmptyForAgeExcludedRoles(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	entry := &cache.Entry{
		Name:          "critical_empty",
		ProcessedDate: now.Add(-10 * time.Hour).Unix(),
		SourceDate:    now.Add(-10 * time.Hour).Unix(),
		CheckedDate:   now.Unix(),
		Entries:       0,
		Version:       1,
	}
	src := &config.Source{Name: "critical_empty", Category: "provider_infrastructure", Frequency: 60, Use: []string{config.UseCriticalInfrastructure}}
	policy := feedhealth.Policy{
		DefaultThresholds: config.FeedHealthCategoryThresholds{
			HealthyCadenceMins: 60,
			RiskyCadenceMins:   120,
		},
	}

	snap := feedhealth.Classify(entry, src, policy, now)
	if got, want := snap.Class, feedhealth.ClassEmpty; got != want {
		t.Fatalf("class = %q, want %q", got, want)
	}
}
