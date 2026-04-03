package insights

import "fmt"

func init() {
	catalog = append(catalog,
		ruleCurrentlyListedAtObservationWall(),
		ruleCurrentlyListedAgeP75(),
		ruleCurrentlyListedAgeP100(),
		ruleRemovedAgeP75(),
		ruleMultipleRetentionPolicies(),
		rulePermanentBans(),
	)
}

// ruleCurrentlyListedAtObservationWall is the "smart" replacement for
// the awkward case where the regular p75/p100 rules would all produce
// identical headlines. It fires when most of the currently-listed IPs
// sit at the edge of our observation window — i.e. they were already
// on the list when we started tracking the feed, and we cannot measure
// their true age.
//
// Without this rule a 7-day-old feed where every IP predates tracking
// produces three near-duplicate headlines:
//
//   - "75% of currently-listed IPs have been here for at most 7 d 15 h."
//   - "Oldest currently-listed IP has been here for 7 d 15 h."
//   - (no useful information about actual age, just a tautology)
//
// With this rule the same feed produces ONE accurate sentence and the
// regular percentile rules stay silent (they detect the same condition
// and self-suppress).
func ruleCurrentlyListedAtObservationWall() Rule {
	return Rule{
		Code:    "currently_listed_at_observation_wall",
		Name:    "Currently-listed IPs predate observation",
		Section: SectionRetention,
		MinSamples: func(s SignalSnapshot) bool {
			return s.AgeOfListed.Total >= 100 && observationHours(s) > 0
		},
		Compute: func(s SignalSnapshot) (Insight, bool) {
			obs := observationHours(s)
			share := walledShare(s.AgeOfListed, obs)
			if share < observationWallShareTrigger {
				return Insight{}, false
			}
			pct := share * 100
			var headline string
			if share >= 0.99 {
				headline = fmt.Sprintf(
					"Tracked for %s. Every currently-listed IP was already on the list when we started observing — we cannot measure how long they had been listed before that.",
					formatDuration(obs),
				)
			} else {
				headline = fmt.Sprintf(
					"Tracked for %s. %.0f%% of currently-listed IPs were already on the list when we started observing — we cannot measure how long they had been listed before that.",
					formatDuration(obs), pct,
				)
			}
			return Insight{
				Headline: headline,
				Evidence: map[string]any{
					"observation_hours": obs,
					"walled_share":      share,
					"total_ips":         s.AgeOfListed.Total,
				},
			}, true
		},
		Methodology: "/methodology/observation-wall",
	}
}

// ruleCurrentlyListedAgeP75 (R02) reports the p75 of the currently
// listed IP age distribution. It self-suppresses when the data is
// "at the wall" — see ruleCurrentlyListedAtObservationWall for the
// reasoning. Sample guard: at least 100 currently-listed IPs.
func ruleCurrentlyListedAgeP75() Rule {
	return Rule{
		Code:    "currently_listed_age_p75",
		Name:    "Currently listed age (p75)",
		Section: SectionRetention,
		MinSamples: func(s SignalSnapshot) bool {
			return s.AgeOfListed.Total >= 100
		},
		Compute: func(s SignalSnapshot) (Insight, bool) {
			p75 := percentileHours(s.AgeOfListed, 0.75)
			if p75 <= 0 {
				return Insight{}, false
			}
			// Suppress when the wall rule is already firing — saying
			// "at most X" when most IPs ARE X is meaningless.
			obs := observationHours(s)
			if walledShare(s.AgeOfListed, obs) >= observationWallShareTrigger {
				return Insight{}, false
			}
			return Insight{
				Headline: fmt.Sprintf(
					"75%% of currently-listed IPs have been here for at most %s.",
					formatDuration(p75),
				),
				Evidence: map[string]any{
					"p75_hours": p75,
					"total_ips": s.AgeOfListed.Total,
				},
			}, true
		},
		Methodology: "/methodology/currently-listed-age-p75",
	}
}

// ruleCurrentlyListedAgeP100 (R03) reports the largest populated bucket
// in the currently listed age distribution. Self-suppresses in two
// cases:
//
//  1. The data is at the observation wall (the wall rule says it).
//  2. p100 == p75 (the p75 rule already conveys the same value;
//     "the oldest is X" when "75% are X" is redundant).
//
// Sample guard: at least 100 listed IPs.
func ruleCurrentlyListedAgeP100() Rule {
	return Rule{
		Code:    "currently_listed_age_p100",
		Name:    "Oldest currently listed IP",
		Section: SectionRetention,
		MinSamples: func(s SignalSnapshot) bool {
			return s.AgeOfListed.Total >= 100
		},
		Compute: func(s SignalSnapshot) (Insight, bool) {
			p100 := percentileHours(s.AgeOfListed, 1.0)
			if p100 <= 0 {
				return Insight{}, false
			}
			obs := observationHours(s)
			if walledShare(s.AgeOfListed, obs) >= observationWallShareTrigger {
				return Insight{}, false
			}
			if ageHistogramSaturated(s.AgeOfListed) {
				return Insight{}, false
			}
			return Insight{
				Headline: fmt.Sprintf(
					"Oldest currently-listed IP has been here for %s.",
					formatDuration(p100),
				),
				Evidence: map[string]any{
					"p100_hours": p100,
					"total_ips":  s.AgeOfListed.Total,
				},
			}, true
		},
		Methodology: "/methodology/currently-listed-age-p100",
	}
}

// ruleRemovedAgeP75 (R10) reports the p75 of how long removed IPs
// stayed in the list before removal. Sample guard: at least 1000
// removed IPs so the percentile is stable.
//
// When the observation window is short (≤30 days) the headline
// includes the window so the user knows the percentile is bounded by
// what we've observed, not by the maintainer's policy.
func ruleRemovedAgeP75() Rule {
	return Rule{
		Code:    "removed_age_p75",
		Name:    "Removed IP duration (p75)",
		Section: SectionRetention,
		MinSamples: func(s SignalSnapshot) bool {
			return s.AgeOfRemoved.Total >= 1000
		},
		Compute: func(s SignalSnapshot) (Insight, bool) {
			p75 := percentileHours(s.AgeOfRemoved, 0.75)
			if p75 <= 0 {
				return Insight{}, false
			}
			obs := observationHours(s)
			var headline string
			if obs > 0 && obs <= 24*30 {
				headline = fmt.Sprintf(
					"Over the past %s, 75%% of removed IPs were kept for at most %s before being dropped.",
					formatDuration(obs), formatDuration(p75),
				)
			} else {
				headline = fmt.Sprintf(
					"75%% of removed IPs were kept for at most %s before being dropped.",
					formatDuration(p75),
				)
			}
			return Insight{
				Headline: headline,
				Evidence: map[string]any{
					"p75_hours":         p75,
					"removed_total":     s.AgeOfRemoved.Total,
					"observation_hours": obs,
				},
			}, true
		},
		Methodology: "/methodology/removed-age-p75",
	}
}

// ruleMultipleRetentionPolicies (R11) fires when the removed-IP age
// distribution shows two distinct retention windows: p90 / p50 > 5.0.
// The headline quotes both percentiles so the user can see the bimodal
// split directly without inspecting the histogram.
func ruleMultipleRetentionPolicies() Rule {
	return Rule{
		Code:    "multiple_retention_policies",
		Name:    "Multiple retention policies",
		Section: SectionRetention,
		MinSamples: func(s SignalSnapshot) bool {
			return s.AgeOfRemoved.Total >= 1000
		},
		Compute: func(s SignalSnapshot) (Insight, bool) {
			p50 := percentileHours(s.AgeOfRemoved, 0.50)
			p90 := percentileHours(s.AgeOfRemoved, 0.90)
			if p50 <= 0 || p90 <= 0 {
				return Insight{}, false
			}
			ratio := float64(p90) / float64(p50)
			if ratio < 5.0 {
				return Insight{}, false
			}
			return Insight{
				Headline: fmt.Sprintf(
					"Half of removed IPs were dropped within %s; the slower 10%% were kept up to %s (%.1fx longer).",
					formatDuration(p50), formatDuration(p90), ratio,
				),
				Evidence: map[string]any{
					"p50_hours":     p50,
					"p90_hours":     p90,
					"ratio":         ratio,
					"removed_total": s.AgeOfRemoved.Total,
				},
			}, true
		},
		Methodology: "/methodology/multiple-retention-policies",
	}
}

// rulePermanentBans (R12) fires when the removed-IP distribution has a
// very long tail: p100 / p90 > 10.0. That indicates a small population
// of IPs kept effectively forever while most were aged out quickly —
// the "permanent ban" pattern.
func rulePermanentBans() Rule {
	return Rule{
		Code:    "permanent_bans",
		Name:    "Permanent bans",
		Section: SectionRetention,
		MinSamples: func(s SignalSnapshot) bool {
			return s.AgeOfRemoved.Total >= 1000
		},
		Compute: func(s SignalSnapshot) (Insight, bool) {
			p90 := percentileHours(s.AgeOfRemoved, 0.90)
			p100 := percentileHours(s.AgeOfRemoved, 1.0)
			if p90 <= 0 || p100 <= 0 {
				return Insight{}, false
			}
			ratio := float64(p100) / float64(p90)
			if ratio < 10.0 {
				return Insight{}, false
			}
			return Insight{
				Headline: fmt.Sprintf(
					"10%% of removed IPs were kept up to %s; the longest-held was %s (%.1fx).",
					formatDuration(p90), formatDuration(p100), ratio,
				),
				Evidence: map[string]any{
					"p90_hours":     p90,
					"p100_hours":    p100,
					"ratio":         ratio,
					"removed_total": s.AgeOfRemoved.Total,
				},
			}, true
		},
		Methodology: "/methodology/permanent-bans",
	}
}
