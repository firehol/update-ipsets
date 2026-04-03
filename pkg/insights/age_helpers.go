package insights

// Helpers for age-related rules. Centralised so the "is the data
// degenerate?" decision is in one place — the regular p75/p100 rules
// and the "at observation wall" rule must agree on the same threshold,
// or they'll fire together and produce contradictory headlines.

const (
	// observationWallRatio is the share of the observation window above
	// which an age bucket is considered "at the wall". When more than
	// this fraction of the window has elapsed, we cannot distinguish
	// "the IP is exactly N hours old" from "the IP was already in the
	// list when we started observing". 0.9 catches buckets that landed
	// just inside the observation window because of coarse bucketing.
	observationWallRatio = 0.9

	// observationWallShareTrigger is the share of currently-listed IPs
	// that must sit at the wall before we collapse the regular age
	// rules into the single "observation wall" headline. Below this
	// share the regular p75/p100 still produce useful information.
	observationWallShareTrigger = 0.5
)

// observationHours returns the duration in hours we have been tracking
// the feed (SnapshotTS - TrackedSinceTS, both unix seconds). Returns 0
// when either timestamp is missing or implausible — callers must treat
// 0 as "unknown" and skip any observation-aware logic.
func observationHours(s SignalSnapshot) int {
	if s.TrackedSinceTS <= 0 || s.SnapshotTS <= 0 || s.SnapshotTS <= s.TrackedSinceTS {
		return 0
	}
	return int((s.SnapshotTS - s.TrackedSinceTS) / 3600)
}

// walledShare returns the fraction of IPs in the histogram whose age
// bucket is at or beyond the observation wall. A value of 1.0 means
// every IP in the list predates our observation window; 0.0 means none
// do. Returns 0 when obsHours is unknown.
func walledShare(h AgeHistogram, obsHours int) float64 {
	if obsHours <= 0 || h.Total == 0 {
		return 0
	}
	threshold := int(observationWallRatio * float64(obsHours))
	if threshold <= 0 {
		threshold = 1
	}
	var atWall uint64
	for i, hours := range h.BucketsHours {
		if i >= len(h.Counts) {
			break
		}
		if hours >= threshold {
			atWall += h.Counts[i]
		}
	}
	return float64(atWall) / float64(h.Total)
}

// ageHistogramSaturated reports whether the histogram has effectively
// no spread between p75 and p100 — i.e. the percentile rules would
// produce identical numbers. Used by the p100 rule to suppress itself
// when p75 already conveys the value.
func ageHistogramSaturated(h AgeHistogram) bool {
	if h.Total == 0 || len(h.Counts) == 0 {
		return false
	}
	p75 := percentileHours(h, 0.75)
	p100 := percentileHours(h, 1.0)
	return p75 > 0 && p100 > 0 && p75 == p100
}
