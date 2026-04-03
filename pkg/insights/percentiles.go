package insights

// percentileHours returns the lowest bucket H from an AgeHistogram such
// that the cumulative count of all buckets with BucketsHours <= H is at
// least q × Total. q must be in [0, 1]; callers routinely pass 0.50,
// 0.75, 0.90, and 1.00.
//
// The function assumes BucketsHours is sorted ascending. If Total is
// zero, or q is outside [0, 1], the function returns 0. For q == 1 the
// result is the largest populated bucket (the "maximum observed age"),
// which the spec calls p100.
//
// Rules use this helper as the single source of truth for every age
// percentile claim so that slight differences in rounding do not
// produce mismatched numbers between the headline and the evidence.
func percentileHours(h AgeHistogram, q float64) int {
	if h.Total == 0 {
		return 0
	}
	if q <= 0 {
		// Return the first populated bucket for q<=0. A rule passing
		// q=0 is a programming error but the result is still a valid
		// bucket from the distribution.
		for i, count := range h.Counts {
			if count > 0 {
				return h.BucketsHours[i]
			}
		}
		return 0
	}
	if q > 1 {
		q = 1
	}
	// Compute the cumulative threshold. We use float64 multiplication
	// and round up to the nearest uint64 so q=1 always requires the
	// full total (no off-by-one that would let p100 land early on the
	// second-largest bucket).
	threshold := uint64(float64(h.Total) * q)
	if threshold < 1 {
		threshold = 1
	}
	if q >= 1.0 {
		threshold = h.Total
	}
	var cumulative uint64
	lastPopulated := 0
	for i, count := range h.Counts {
		if count == 0 {
			continue
		}
		cumulative += count
		lastPopulated = h.BucketsHours[i]
		if cumulative >= threshold {
			return h.BucketsHours[i]
		}
	}
	// We exhausted every bucket without meeting the threshold (can
	// happen only if Total disagreed with sum(Counts)). Return the
	// largest populated bucket as a safe fallback.
	return lastPopulated
}
