package feedhealth

import (
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
)

type Class string

const (
	ClassUnavailable  Class = "unavailable"
	ClassArchived     Class = "archived"
	ClassEmpty        Class = "empty"
	ClassDelayed      Class = "delayed"
	ClassRisky        Class = "risky"
	ClassUnmaintained Class = "unmaintained"
	ClassHealthy      Class = "healthy"
)

type ThresholdBasis string

const (
	ThresholdCategoryCadence   ThresholdBasis = "category_cadence"
	ThresholdSingleObservation ThresholdBasis = "single_observation_grace"
)

type Policy struct {
	SingleObservationGraceMins int                                            `json:"single_observation_grace_mins"`
	DefaultThresholds          config.FeedHealthCategoryThresholds            `json:"default_thresholds"`
	ArchivalThresholdMins      int                                            `json:"archival_threshold_mins"`
	CategoryThresholds         map[string]config.FeedHealthCategoryThresholds `json:"category_thresholds,omitempty"`
}

type Snapshot struct {
	Class                      Class          `json:"class"`
	ThresholdBasis             ThresholdBasis `json:"threshold_basis,omitempty"`
	ThresholdMins              int            `json:"threshold_mins,omitempty"`
	AvgUpdateMins              int            `json:"avg_update_mins,omitempty"`
	MinUpdateMins              int            `json:"min_update_mins,omitempty"`
	MaxUpdateMins              int            `json:"max_update_mins,omitempty"`
	ObservedUpdates            int            `json:"observed_updates,omitempty"`
	FirstObservedAt            int64          `json:"first_observed_at,omitempty"`
	LastChangeAt               int64          `json:"last_change_at,omitempty"`
	TimeSinceLastChangeMins    int            `json:"time_since_last_change_mins,omitempty"`
	FailureStartedAt           int64          `json:"failure_started_at,omitempty"`
	TimeSinceFailureMins       int            `json:"time_since_failure_mins,omitempty"`
	DownloadFailures           int            `json:"download_failures,omitempty"`
	ExcludeFromUnmaintained    bool           `json:"exclude_from_unmaintained,omitempty"`
	HealthyCadenceMins         int            `json:"healthy_cadence_mins,omitempty"`
	RiskyCadenceMins           int            `json:"risky_cadence_mins,omitempty"`
	EffectiveHealthyGapMins    int            `json:"effective_healthy_gap_mins,omitempty"`
	UnmaintainedThresholdMins  int            `json:"unmaintained_threshold_mins,omitempty"`
	ArchivalThresholdMins      int            `json:"archival_threshold_mins,omitempty"`
	SingleObservationGraceMins int            `json:"single_observation_grace_mins,omitempty"`
}

func PolicyFromRuntime(rt config.RuntimeConfig) Policy {
	return Policy{
		SingleObservationGraceMins: rt.FeedHealthSingleObservationGraceMins,
		DefaultThresholds: config.FeedHealthCategoryThresholds{
			HealthyCadenceMins: rt.FeedHealthDefaultHealthyCadenceMins,
			RiskyCadenceMins:   rt.FeedHealthDefaultRiskyCadenceMins,
		},
		ArchivalThresholdMins: rt.FeedHealthArchivalThresholdMins,
		CategoryThresholds:    rt.FeedHealthCategoryThresholds,
	}
}

func (p Policy) ThresholdsForCategory(category string) config.FeedHealthCategoryThresholds {
	if threshold, ok := p.CategoryThresholds[category]; ok {
		return threshold
	}
	return p.DefaultThresholds
}

func Classify(entry *cache.Entry, src *config.Source, policy Policy, now time.Time) Snapshot {
	if entry == nil {
		return Snapshot{Class: ClassUnavailable}
	}

	now = now.UTC()
	snap := Snapshot{
		Class:                      ClassHealthy,
		AvgUpdateMins:              positiveOrZero(entry.AverageUpdateMins),
		MinUpdateMins:              positiveOrZero(entry.MinUpdateMins),
		MaxUpdateMins:              positiveOrZero(entry.MaxUpdateMins),
		ObservedUpdates:            observedUpdates(entry),
		FirstObservedAt:            firstObservedAt(entry),
		LastChangeAt:               lastChangeAt(entry),
		DownloadFailures:           entry.DownloadFailures,
		ExcludeFromUnmaintained:    excludesAgeBasedHealth(src),
		ArchivalThresholdMins:      positiveOrZero(policy.ArchivalThresholdMins),
		SingleObservationGraceMins: positiveOrZero(policy.SingleObservationGraceMins),
	}

	if neverPublished(entry) {
		snap.Class = ClassUnavailable
		return snap
	}

	if snap.AvgUpdateMins == 0 {
		snap.AvgUpdateMins = configuredFrequency(entry, src)
	}
	if snap.MinUpdateMins == 0 {
		snap.MinUpdateMins = configuredFrequency(entry, src)
	}
	if snap.MaxUpdateMins == 0 {
		snap.MaxUpdateMins = configuredFrequency(entry, src)
	}

	if snap.LastChangeAt > 0 {
		snap.TimeSinceLastChangeMins = elapsedMinutes(now.Unix() - snap.LastChangeAt)
	}
	if failureStart := currentFailureStartedAt(entry); failureStart > 0 {
		snap.FailureStartedAt = failureStart
		snap.TimeSinceFailureMins = elapsedMinutes(now.Unix() - failureStart)
	}

	thresholds := policy.ThresholdsForCategory(categoryFor(entry, src))
	snap.HealthyCadenceMins = positiveOrZero(thresholds.HealthyCadenceMins)
	snap.RiskyCadenceMins = positiveOrZero(thresholds.RiskyCadenceMins)
	snap.EffectiveHealthyGapMins = effectiveHealthyGapMins(snap.AvgUpdateMins, snap.HealthyCadenceMins)
	snap.UnmaintainedThresholdMins = snap.RiskyCadenceMins * 2

	if graceActive(snap) {
		snap.ThresholdBasis = ThresholdSingleObservation
		snap.ThresholdMins = snap.SingleObservationGraceMins
	} else {
		snap.ThresholdBasis = ThresholdCategoryCadence
		snap.ThresholdMins = snap.UnmaintainedThresholdMins
	}

	if failureThreshold := unavailableThresholdMins(snap); isCurrentlyUnavailable(entry) &&
		failureThreshold > 0 &&
		unavailableByFailureOrStaleness(snap, failureThreshold) {
		snap.Class = unavailableOrArchivedClass(snap, failureThreshold)
		snap.ThresholdMins = failureThreshold
		return snap
	}
	if entry.Entries == 0 {
		snap.Class = ClassEmpty
		return snap
	}
	if snap.ExcludeFromUnmaintained {
		snap.Class = ClassHealthy
		return snap
	}
	if graceActive(snap) {
		snap.Class = ClassHealthy
		return snap
	}
	switch classifyAge(snap.TimeSinceLastChangeMins, snap.EffectiveHealthyGapMins, snap.RiskyCadenceMins, snap.UnmaintainedThresholdMins) {
	case ClassUnmaintained:
		snap.Class = ClassUnmaintained
	case ClassRisky:
		snap.Class = ClassRisky
	case ClassDelayed:
		snap.Class = ClassDelayed
	default:
		snap.Class = ClassHealthy
	}
	return snap
}

func unavailableOrArchivedClass(snap Snapshot, unavailableThreshold int) Class {
	if archivalThresholdReached(snap, unavailableThreshold) {
		return ClassArchived
	}
	return ClassUnavailable
}

func archivalThresholdReached(snap Snapshot, unavailableThreshold int) bool {
	if snap.ArchivalThresholdMins <= 0 {
		return false
	}
	unavailableFor := timeInUnavailableMins(snap, unavailableThreshold)
	return unavailableFor >= snap.ArchivalThresholdMins
}

func timeInUnavailableMins(snap Snapshot, unavailableThreshold int) int {
	// There are two independently useful lower bounds for how long the feed
	// has been unusable:
	// 1. the active failure streak (`TimeSinceFailureMins`)
	// 2. how long it has been since the last usable local refresh beyond the
	//    ordinary unavailable threshold
	//
	// We intentionally take the larger bound. This allows obviously dead feeds
	// with very old last successful publications to move to `archived`
	// immediately once they are in a live unavailable state, instead of waiting
	// an extra archival window from the most recent failure event.
	failureBeyondThreshold := 0
	if snap.TimeSinceFailureMins > unavailableThreshold {
		failureBeyondThreshold = snap.TimeSinceFailureMins - unavailableThreshold
	}
	staleWithoutRefresh := 0
	if snap.LastChangeAt > 0 && snap.TimeSinceLastChangeMins > unavailableThreshold {
		staleWithoutRefresh = snap.TimeSinceLastChangeMins - unavailableThreshold
	}
	if failureBeyondThreshold > staleWithoutRefresh {
		return failureBeyondThreshold
	}
	return staleWithoutRefresh
}

func classifyAge(gap, healthy, risky, unmaintained int) Class {
	if unmaintained > 0 && gap >= unmaintained {
		return ClassUnmaintained
	}
	if risky > 0 && gap >= risky {
		return ClassRisky
	}
	if healthy > 0 && gap > healthy {
		return ClassDelayed
	}
	return ClassHealthy
}

func excludesAgeBasedHealth(src *config.Source) bool {
	if src == nil {
		return false
	}
	if src.ExcludeFromUnmaintained {
		return true
	}
	return src.HasUse(config.UseCriticalInfrastructure) ||
		src.HasUse(config.UseProviderContext) ||
		src.HasUse(config.UseASN) ||
		src.HasUse(config.UseGeoIP)
}

func unavailableThresholdMins(snap Snapshot) int {
	threshold := snap.UnmaintainedThresholdMins
	if snap.ObservedUpdates <= 1 && snap.SingleObservationGraceMins > threshold {
		threshold = snap.SingleObservationGraceMins
	}
	return threshold
}

func unavailableByFailureOrStaleness(snap Snapshot, threshold int) bool {
	if threshold <= 0 {
		return false
	}
	if snap.TimeSinceFailureMins >= threshold {
		return true
	}
	return snap.LastChangeAt > 0 && snap.TimeSinceLastChangeMins >= threshold
}

func graceActive(snap Snapshot) bool {
	return snap.ObservedUpdates <= 1 &&
		snap.SingleObservationGraceMins > 0 &&
		snap.TimeSinceLastChangeMins > 0 &&
		snap.TimeSinceLastChangeMins < snap.SingleObservationGraceMins
}

func effectiveHealthyGapMins(avgGap, healthyCadence int) int {
	if avgGap > healthyCadence {
		return avgGap
	}
	return healthyCadence
}

func categoryFor(entry *cache.Entry, src *config.Source) string {
	if src != nil && src.Category != "" {
		return src.Category
	}
	if entry != nil {
		return entry.Category
	}
	return ""
}

func configuredFrequency(entry *cache.Entry, src *config.Source) int {
	if entry != nil && entry.FrequencyMinutes > 0 {
		return entry.FrequencyMinutes
	}
	if src != nil && src.Frequency > 0 {
		return src.Frequency
	}
	return 0
}

func observedUpdates(entry *cache.Entry) int {
	if entry == nil {
		return 0
	}
	if entry.Version > 0 {
		return entry.Version
	}
	if entry.SourceDate > 0 || entry.ProcessedDate > 0 || entry.StartedDate > 0 {
		return 1
	}
	return 0
}

func firstObservedAt(entry *cache.Entry) int64 {
	if entry == nil {
		return 0
	}
	switch {
	case entry.StartedDate > 0:
		return entry.StartedDate
	case entry.SourceDate > 0:
		return entry.SourceDate
	case entry.ProcessedDate > 0:
		return entry.ProcessedDate
	default:
		return 0
	}
}

func lastChangeAt(entry *cache.Entry) int64 {
	if entry == nil {
		return 0
	}
	if entry.SourceDate > 0 {
		return entry.SourceDate
	}
	return entry.ProcessedDate
}

func isCurrentlyUnavailable(entry *cache.Entry) bool {
	if entry == nil {
		return false
	}
	if entry.DownloadFailures > 0 {
		return true
	}
	switch entry.LastStatus {
	case "download_failed", "missing_env", "url_resolve_failed", "unavailable":
		return true
	default:
		return false
	}
}

func currentFailureStartedAt(entry *cache.Entry) int64 {
	if entry == nil || !isCurrentlyUnavailable(entry) {
		return 0
	}
	if entry.FailureStartedDate > 0 {
		return entry.FailureStartedDate
	}
	if entry.CheckedDate > 0 {
		return entry.CheckedDate
	}
	return 0
}

func neverPublished(entry *cache.Entry) bool {
	if entry == nil {
		return true
	}
	return entry.Version <= 0 && entry.ProcessedDate <= 0
}

func positiveOrZero(v int) int {
	if v > 0 {
		return v
	}
	return 0
}

func elapsedMinutes(seconds int64) int {
	if seconds <= 0 {
		return 0
	}
	return int(seconds / 60)
}
