package insights

import (
	"fmt"
	"sort"
	"time"
)

func init() {
	catalog = append(catalog, ruleSizeVariation())
}

// ruleSizeVariation (R01) reports the min/max range of the size series
// over the last N recorded updates. It fires whenever there is a real
// range to show — min != max and median > 0. The spec does not define
// a ratio threshold for this rule because the numeric range is the
// entire point of the headline: even a small spread (±3%) is a factual
// observation about feed stability. The sample-size guard of ≥50 points
// is the only gate.
func ruleSizeVariation() Rule {
	return Rule{
		Code:    "size_variation",
		Name:    "Size variation",
		Section: SectionOverview,
		MinSamples: func(s SignalSnapshot) bool {
			return len(s.SizeSeries) >= 50
		},
		Compute: func(s SignalSnapshot) (Insight, bool) {
			sizes := make([]uint64, 0, len(s.SizeSeries))
			var minTS, maxTS int64
			minSize := s.SizeSeries[0].Size
			maxSize := s.SizeSeries[0].Size
			for i, pt := range s.SizeSeries {
				sizes = append(sizes, pt.Size)
				if i == 0 || pt.TS < minTS {
					minTS = pt.TS
				}
				if pt.TS > maxTS {
					maxTS = pt.TS
				}
				if pt.Size < minSize {
					minSize = pt.Size
				}
				if pt.Size > maxSize {
					maxSize = pt.Size
				}
			}
			median := medianUint64(sizes)
			if median == 0 || minSize == maxSize {
				return Insight{}, false
			}
			ratio := float64(maxSize-minSize) / float64(median)
			span := ""
			if minTS > 0 && maxTS > minTS {
				span = formatDuration(int(time.Unix(maxTS, 0).Sub(time.Unix(minTS, 0)).Hours()))
			}
			headline := fmt.Sprintf(
				"Over the last %d updates, the list size ranged from %s to %s.",
				len(s.SizeSeries), formatCount(minSize), formatCount(maxSize),
			)
			if span != "" {
				headline = fmt.Sprintf(
					"Over the last %d updates (%s), the list size ranged from %s to %s.",
					len(s.SizeSeries), span, formatCount(minSize), formatCount(maxSize),
				)
			}
			return Insight{
				Headline: headline,
				Evidence: map[string]any{
					"updates":     len(s.SizeSeries),
					"min_size":    minSize,
					"max_size":    maxSize,
					"median_size": median,
					"ratio":       ratio,
					"from_ts":     minTS,
					"to_ts":       maxTS,
				},
			}, true
		},
		Methodology: "/methodology/size-variation",
	}
}

// medianUint64 returns the median of a slice of uint64 values. The
// input is copied before sorting so the caller's slice is not mutated.
// For an even number of samples the lower midpoint is returned, which
// is deterministic and matches the "at most" phrasing used by the
// headlines that depend on this helper.
func medianUint64(values []uint64) uint64 {
	if len(values) == 0 {
		return 0
	}
	cp := make([]uint64, len(values))
	copy(cp, values)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	return cp[len(cp)/2]
}
