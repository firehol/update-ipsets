package config

import "fmt"

type FeedHealthCategoryThresholds struct {
	HealthyCadenceMins int `yaml:"healthy_cadence_minutes,omitempty"`
	RiskyCadenceMins   int `yaml:"risky_cadence_minutes,omitempty"`
}

func DefaultFeedHealthCategoryThresholds() map[string]FeedHealthCategoryThresholds {
	return map[string]FeedHealthCategoryThresholds{}
}

func cloneFeedHealthCategoryThresholds(in map[string]FeedHealthCategoryThresholds) map[string]FeedHealthCategoryThresholds {
	if len(in) == 0 {
		return map[string]FeedHealthCategoryThresholds{}
	}
	out := make(map[string]FeedHealthCategoryThresholds, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func mergeFeedHealthCategoryThresholds(
	base map[string]FeedHealthCategoryThresholds,
	overrides map[string]FeedHealthCategoryThresholds,
) map[string]FeedHealthCategoryThresholds {
	out := cloneFeedHealthCategoryThresholds(base)
	for category, override := range overrides {
		current := out[category]
		if override.HealthyCadenceMins > 0 {
			current.HealthyCadenceMins = override.HealthyCadenceMins
		}
		if override.RiskyCadenceMins > 0 {
			current.RiskyCadenceMins = override.RiskyCadenceMins
		}
		out[category] = current
	}
	return out
}

func normalizeFeedHealthThresholds(rt *RuntimeConfig) {
	if rt == nil {
		return
	}
	if rt.FeedHealthDefaultHealthyCadenceMins <= 0 {
		rt.FeedHealthDefaultHealthyCadenceMins = 7 * 24 * 60
	}
	if rt.FeedHealthDefaultRiskyCadenceMins <= 0 {
		rt.FeedHealthDefaultRiskyCadenceMins = 30 * 24 * 60
	}
	if rt.FeedHealthArchivalThresholdMins <= 0 {
		rt.FeedHealthArchivalThresholdMins = 60 * 24 * 60
	}
	rt.FeedHealthCategoryThresholds = mergeFeedHealthCategoryThresholds(
		DefaultFeedHealthCategoryThresholds(),
		rt.FeedHealthCategoryThresholds,
	)
}

func validateFeedHealthThresholds(rt RuntimeConfig) error {
	if rt.FeedHealthSingleObservationGraceMins <= 0 {
		return fmt.Errorf("runtime.feed_health_single_observation_grace_minutes must be > 0, got %d", rt.FeedHealthSingleObservationGraceMins)
	}
	if rt.FeedHealthDefaultHealthyCadenceMins <= 0 {
		return fmt.Errorf("runtime.feed_health_default_healthy_cadence_minutes must be > 0, got %d", rt.FeedHealthDefaultHealthyCadenceMins)
	}
	if rt.FeedHealthDefaultRiskyCadenceMins <= 0 {
		return fmt.Errorf("runtime.feed_health_default_risky_cadence_minutes must be > 0, got %d", rt.FeedHealthDefaultRiskyCadenceMins)
	}
	if rt.FeedHealthArchivalThresholdMins <= 0 {
		return fmt.Errorf("runtime.feed_health_archival_threshold_minutes must be > 0, got %d", rt.FeedHealthArchivalThresholdMins)
	}
	if rt.FeedHealthDefaultHealthyCadenceMins >= rt.FeedHealthDefaultRiskyCadenceMins {
		return fmt.Errorf("runtime.feed_health_default_healthy_cadence_minutes must be < runtime.feed_health_default_risky_cadence_minutes (%d >= %d)", rt.FeedHealthDefaultHealthyCadenceMins, rt.FeedHealthDefaultRiskyCadenceMins)
	}
	for category, threshold := range rt.FeedHealthCategoryThresholds {
		if threshold.HealthyCadenceMins <= 0 {
			return fmt.Errorf("runtime.feed_health_category_thresholds.%s.healthy_cadence_minutes must be > 0, got %d", category, threshold.HealthyCadenceMins)
		}
		if threshold.RiskyCadenceMins <= 0 {
			return fmt.Errorf("runtime.feed_health_category_thresholds.%s.risky_cadence_minutes must be > 0, got %d", category, threshold.RiskyCadenceMins)
		}
		if threshold.HealthyCadenceMins >= threshold.RiskyCadenceMins {
			return fmt.Errorf("runtime.feed_health_category_thresholds.%s.healthy_cadence_minutes must be < risky_cadence_minutes (%d >= %d)", category, threshold.HealthyCadenceMins, threshold.RiskyCadenceMins)
		}
	}
	return nil
}

func (rt RuntimeConfig) FeedHealthThresholdsForCategory(category string) FeedHealthCategoryThresholds {
	if threshold, ok := rt.FeedHealthCategoryThresholds[category]; ok {
		return threshold
	}
	return FeedHealthCategoryThresholds{
		HealthyCadenceMins: rt.FeedHealthDefaultHealthyCadenceMins,
		RiskyCadenceMins:   rt.FeedHealthDefaultRiskyCadenceMins,
	}
}
