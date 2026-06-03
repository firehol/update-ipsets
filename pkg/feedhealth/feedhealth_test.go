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

func TestPolicyFromRuntimeAndCategoryFallback(t *testing.T) {
	runtimeConfig := config.RuntimeConfig{
		FeedHealthSingleObservationGraceMins: 30,
		FeedHealthDefaultHealthyCadenceMins:  60,
		FeedHealthDefaultRiskyCadenceMins:    180,
		FeedHealthArchivalThresholdMins:      1440,
		FeedHealthCategoryThresholds: map[string]config.FeedHealthCategoryThresholds{
			"fast": {
				HealthyCadenceMins: 10,
				RiskyCadenceMins:   20,
			},
		},
	}

	policy := feedhealth.PolicyFromRuntime(runtimeConfig)

	if got, want := policy.SingleObservationGraceMins, 30; got != want {
		t.Fatalf("single observation grace = %d, want %d", got, want)
	}
	if got, want := policy.ArchivalThresholdMins, 1440; got != want {
		t.Fatalf("archival threshold = %d, want %d", got, want)
	}
	if got, want := policy.DefaultThresholds, (config.FeedHealthCategoryThresholds{HealthyCadenceMins: 60, RiskyCadenceMins: 180}); got != want {
		t.Fatalf("default thresholds = %+v, want %+v", got, want)
	}
	if got, want := policy.ThresholdsForCategory("fast"), (config.FeedHealthCategoryThresholds{HealthyCadenceMins: 10, RiskyCadenceMins: 20}); got != want {
		t.Fatalf("fast thresholds = %+v, want %+v", got, want)
	}
	if got, want := policy.ThresholdsForCategory("unknown"), policy.DefaultThresholds; got != want {
		t.Fatalf("unknown thresholds = %+v, want %+v", got, want)
	}
}

func TestClassifyAgeClasses(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	policy := feedhealth.Policy{
		DefaultThresholds: config.FeedHealthCategoryThresholds{
			HealthyCadenceMins: 60,
			RiskyCadenceMins:   180,
		},
	}

	for _, tc := range []struct {
		name string
		gap  time.Duration
		want feedhealth.Class
	}{
		{name: "within healthy cadence", gap: 30 * time.Minute, want: feedhealth.ClassHealthy},
		{name: "past healthy cadence", gap: 61 * time.Minute, want: feedhealth.ClassDelayed},
		{name: "at risky cadence", gap: 180 * time.Minute, want: feedhealth.ClassRisky},
		{name: "at unmaintained cadence", gap: 360 * time.Minute, want: feedhealth.ClassUnmaintained},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entry := &cache.Entry{
				Name:              "sample",
				ProcessedDate:     now.Add(-tc.gap).Unix(),
				SourceDate:        now.Add(-tc.gap).Unix(),
				CheckedDate:       now.Unix(),
				Category:          "intrusion",
				Entries:           10,
				Version:           3,
				AverageUpdateMins: 60,
			}

			snap := feedhealth.Classify(entry, nil, policy, now)
			if got := snap.Class; got != tc.want {
				t.Fatalf("class = %q, want %q", got, tc.want)
			}
			if got, want := snap.ThresholdBasis, feedhealth.ThresholdCategoryCadence; got != want {
				t.Fatalf("threshold basis = %q, want %q", got, want)
			}
			if got, want := snap.ThresholdMins, 360; got != want {
				t.Fatalf("threshold mins = %d, want %d", got, want)
			}
			if got, want := snap.EffectiveHealthyGapMins, 60; got != want {
				t.Fatalf("effective healthy gap = %d, want %d", got, want)
			}
		})
	}
}

func TestClassifySingleObservationGraceUsesEntryFrequency(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	entry := &cache.Entry{
		Name:             "new_feed",
		ProcessedDate:    now.Add(-90 * time.Minute).Unix(),
		SourceDate:       now.Add(-90 * time.Minute).Unix(),
		CheckedDate:      now.Unix(),
		Entries:          10,
		Version:          1,
		FrequencyMinutes: 45,
	}
	src := &config.Source{Name: "new_feed", Category: "intrusion", Frequency: 30}
	policy := feedhealth.Policy{
		SingleObservationGraceMins: 120,
		DefaultThresholds: config.FeedHealthCategoryThresholds{
			HealthyCadenceMins: 60,
			RiskyCadenceMins:   180,
		},
	}

	snap := feedhealth.Classify(entry, src, policy, now)

	if got, want := snap.Class, feedhealth.ClassHealthy; got != want {
		t.Fatalf("class = %q, want %q", got, want)
	}
	if got, want := snap.ThresholdBasis, feedhealth.ThresholdSingleObservation; got != want {
		t.Fatalf("threshold basis = %q, want %q", got, want)
	}
	if got, want := snap.ThresholdMins, 120; got != want {
		t.Fatalf("threshold mins = %d, want %d", got, want)
	}
	if got, want := snap.ObservedUpdates, 1; got != want {
		t.Fatalf("observed updates = %d, want %d", got, want)
	}
	if got, want := snap.AvgUpdateMins, 45; got != want {
		t.Fatalf("average update mins = %d, want %d", got, want)
	}
	if got, want := snap.MinUpdateMins, 45; got != want {
		t.Fatalf("minimum update mins = %d, want %d", got, want)
	}
	if got, want := snap.MaxUpdateMins, 45; got != want {
		t.Fatalf("maximum update mins = %d, want %d", got, want)
	}
}

func TestClassifyUsesSourceCategoryThresholdsAndFallbackDates(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	startedAt := now.Add(-200 * time.Minute).Unix()
	processedAt := now.Add(-100 * time.Minute).Unix()
	entry := &cache.Entry{
		Name:          "sample",
		ProcessedDate: processedAt,
		StartedDate:   startedAt,
		Category:      "slow",
		Entries:       10,
		Version:       2,
	}
	src := &config.Source{Name: "sample", Category: "fast", Frequency: 15}
	policy := feedhealth.Policy{
		DefaultThresholds: config.FeedHealthCategoryThresholds{
			HealthyCadenceMins: 90,
			RiskyCadenceMins:   180,
		},
		CategoryThresholds: map[string]config.FeedHealthCategoryThresholds{
			"fast": {
				HealthyCadenceMins: 30,
				RiskyCadenceMins:   60,
			},
		},
	}

	snap := feedhealth.Classify(entry, src, policy, now)

	if got, want := snap.Class, feedhealth.ClassRisky; got != want {
		t.Fatalf("class = %q, want %q", got, want)
	}
	if got, want := snap.HealthyCadenceMins, 30; got != want {
		t.Fatalf("healthy cadence = %d, want %d", got, want)
	}
	if got, want := snap.RiskyCadenceMins, 60; got != want {
		t.Fatalf("risky cadence = %d, want %d", got, want)
	}
	if got, want := snap.AvgUpdateMins, 15; got != want {
		t.Fatalf("average update mins = %d, want %d", got, want)
	}
	if got, want := snap.FirstObservedAt, startedAt; got != want {
		t.Fatalf("first observed at = %d, want %d", got, want)
	}
	if got, want := snap.LastChangeAt, processedAt; got != want {
		t.Fatalf("last change at = %d, want %d", got, want)
	}
}

func TestClassifyUnavailableStatusTakesPrecedenceOverEmpty(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	checkedAt := now.Add(-300 * time.Minute).Unix()
	entry := &cache.Entry{
		Name:          "missing_env_feed",
		ProcessedDate: now.Add(-300 * time.Minute).Unix(),
		SourceDate:    now.Add(-300 * time.Minute).Unix(),
		CheckedDate:   checkedAt,
		LastStatus:    "missing_env",
		Entries:       0,
		Version:       2,
	}
	policy := feedhealth.Policy{
		DefaultThresholds: config.FeedHealthCategoryThresholds{
			HealthyCadenceMins: 30,
			RiskyCadenceMins:   60,
		},
	}

	snap := feedhealth.Classify(entry, nil, policy, now)

	if got, want := snap.Class, feedhealth.ClassUnavailable; got != want {
		t.Fatalf("class = %q, want %q", got, want)
	}
	if got, want := snap.FailureStartedAt, checkedAt; got != want {
		t.Fatalf("failure started at = %d, want %d", got, want)
	}
	if got, want := snap.TimeSinceFailureMins, 300; got != want {
		t.Fatalf("time since failure = %d, want %d", got, want)
	}
	if got, want := snap.ThresholdMins, 120; got != want {
		t.Fatalf("threshold mins = %d, want %d", got, want)
	}
	if got, want := snap.DownloadFailures, 0; got != want {
		t.Fatalf("download failures = %d, want %d", got, want)
	}
}

func TestClassifyUnavailableForNilAndNeverPublishedEntries(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	policy := feedhealth.Policy{
		DefaultThresholds: config.FeedHealthCategoryThresholds{
			HealthyCadenceMins: 60,
			RiskyCadenceMins:   120,
		},
	}

	if snap := feedhealth.Classify(nil, nil, policy, now); snap.Class != feedhealth.ClassUnavailable {
		t.Fatalf("nil entry class = %q, want %q", snap.Class, feedhealth.ClassUnavailable)
	}

	entry := &cache.Entry{
		Name:        "never_published",
		StartedDate: now.Add(-10 * time.Minute).Unix(),
	}
	snap := feedhealth.Classify(entry, nil, policy, now)
	if got, want := snap.Class, feedhealth.ClassUnavailable; got != want {
		t.Fatalf("never-published class = %q, want %q", got, want)
	}
	if got, want := snap.FirstObservedAt, entry.StartedDate; got != want {
		t.Fatalf("first observed at = %d, want %d", got, want)
	}
}
