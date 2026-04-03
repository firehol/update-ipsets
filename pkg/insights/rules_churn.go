package insights

import (
	"fmt"
	"sort"
)

func init() {
	catalog = append(catalog,
		ruleChurnHigh(),
		ruleChurnLow(),
	)
}

// ruleChurnHigh (R13) fires when the median churn ratio is greater
// than 0.50: more than half of the list changes every recorded update.
// Sample guard: at least 50 churn points.
func ruleChurnHigh() Rule {
	return Rule{
		Code:    "churn_high",
		Name:    "High churn",
		Section: SectionTrends,
		MinSamples: func(s SignalSnapshot) bool {
			return len(s.ChurnSeries) >= 50
		},
		Compute: func(s SignalSnapshot) (Insight, bool) {
			ratios := churnRatios(s.ChurnSeries)
			if len(ratios) == 0 {
				return Insight{}, false
			}
			median := medianFloat64(ratios)
			if median <= 0.50 {
				return Insight{}, false
			}
			return Insight{
				Headline: fmt.Sprintf(
					"Median churn over the last %d updates: %s of the list changes per update.",
					len(ratios), formatPercent(median),
				),
				Evidence: map[string]any{
					"updates":       len(ratios),
					"median_churn":  median,
					"series_length": len(s.ChurnSeries),
				},
			}, true
		},
		Methodology: "/methodology/churn-high",
	}
}

// ruleChurnLow (R14) fires when the median churn ratio is less than
// 0.05: the list is very stable, almost nothing changes update to
// update. Sample guard: at least 50 churn points.
func ruleChurnLow() Rule {
	return Rule{
		Code:    "churn_low",
		Name:    "Low churn",
		Section: SectionTrends,
		MinSamples: func(s SignalSnapshot) bool {
			return len(s.ChurnSeries) >= 50
		},
		Compute: func(s SignalSnapshot) (Insight, bool) {
			ratios := churnRatios(s.ChurnSeries)
			if len(ratios) == 0 {
				return Insight{}, false
			}
			median := medianFloat64(ratios)
			if median >= 0.05 {
				return Insight{}, false
			}
			return Insight{
				Headline: fmt.Sprintf(
					"Median churn over the last %d updates: %s of the list changes per update.",
					len(ratios), formatPercent(median),
				),
				Evidence: map[string]any{
					"updates":       len(ratios),
					"median_churn":  median,
					"series_length": len(s.ChurnSeries),
				},
			}, true
		},
		Methodology: "/methodology/churn-low",
	}
}

// churnRatios computes (added+removed)/size for every churn point
// whose Size is non-zero. Points with Size == 0 are skipped so an
// empty feed at a single recorded moment cannot divide by zero.
func churnRatios(series []ChurnPoint) []float64 {
	out := make([]float64, 0, len(series))
	for _, pt := range series {
		if pt.Size == 0 {
			continue
		}
		out = append(out, float64(pt.Added+pt.Removed)/float64(pt.Size))
	}
	return out
}

// medianFloat64 returns the median of a slice of float64 values. The
// input is copied before sorting so the caller's slice is not mutated.
// For an even number of samples the lower midpoint is returned so the
// result is deterministic.
func medianFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	cp := make([]float64, len(values))
	copy(cp, values)
	sort.Float64s(cp)
	return cp[len(cp)/2]
}
