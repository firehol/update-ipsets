package insights

import "testing"

func TestRuleCurrentlyListedAgeP75_Fires(t *testing.T) {
	snap := buildSnap().withAgeOfListed(AgeHistogram{
		BucketsHours: []int{1, 24, 168},
		Counts:       []uint64{100, 80, 40},
		Total:        220,
	}).toSnap()
	got := NewEngine().Derive(snap)
	if !containsCode(got, "currently_listed_age_p75") {
		t.Fatalf("expected currently_listed_age_p75 to fire; got %v", insightCodes(got))
	}
}

func TestRuleCurrentlyListedAgeP75_SuppressedBySample(t *testing.T) {
	snap := buildSnap().withAgeOfListed(AgeHistogram{
		BucketsHours: []int{1, 24},
		Counts:       []uint64{5, 3},
		Total:        8,
	}).toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "currently_listed_age_p75") {
		t.Fatalf("expected currently_listed_age_p75 to stay silent on tiny sample; got %v", insightCodes(got))
	}
}

func TestRuleCurrentlyListedAgeP100_Fires(t *testing.T) {
	// Histogram with REAL spread between p75 and p100 — most IPs are
	// in the 24h bucket, a long tail extends to 720h. The new
	// "p75 != p100" suppression must NOT trigger here.
	snap := buildSnap().withAgeOfListed(AgeHistogram{
		BucketsHours: []int{1, 24, 168, 720},
		Counts:       []uint64{100, 50, 30, 30},
		Total:        210,
	}).toSnap()
	got := NewEngine().Derive(snap)
	if !containsCode(got, "currently_listed_age_p100") {
		t.Fatalf("expected currently_listed_age_p100 to fire; got %v", insightCodes(got))
	}
}

func TestRuleCurrentlyListedAgeP100_SuppressedWhenEqualsP75(t *testing.T) {
	// All 220 IPs in the same bucket: p75 == p100 == 720h. The new
	// logic must suppress p100 because p75 already conveys the value.
	// p75 should still fire (we have not crossed the observation wall
	// — observation hours is unset on this snapshot, so the wall
	// trigger stays inactive).
	snap := buildSnap().withAgeOfListed(AgeHistogram{
		BucketsHours: []int{720},
		Counts:       []uint64{220},
		Total:        220,
	}).toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "currently_listed_age_p100") {
		t.Fatalf("expected currently_listed_age_p100 to stay silent when equal to p75; got %v", insightCodes(got))
	}
	if !containsCode(got, "currently_listed_age_p75") {
		t.Fatalf("expected currently_listed_age_p75 to fire; got %v", insightCodes(got))
	}
}

// TestRuleCurrentlyListedAtObservationWall_Fires_AndSuppressesPercentiles
// is the canonical "Costa's complaint" test: a 7-day-old feed where
// every currently-listed IP has been on the list since we started
// observing. The previous behaviour was three near-duplicate insights:
//
//	"75% of currently-listed IPs have been here for at most 7 d 15 h."
//	"Oldest currently-listed IP has been here for 7 d 15 h."
//
// The new behaviour is one combined insight admitting we cannot measure
// the true age, and zero p75/p100 noise.
func TestRuleCurrentlyListedAtObservationWall_Fires_AndSuppressesPercentiles(t *testing.T) {
	const hours7d15h = 24*7 + 15
	// 220 IPs all in the bucket at 7d15h, observation window also 7d15h.
	snap := buildSnap().
		withAgeOfListed(AgeHistogram{
			BucketsHours: []int{hours7d15h},
			Counts:       []uint64{220},
			Total:        220,
		}).
		toSnap()
	// Set the observation window directly: tracked since exactly
	// 7d15h before SnapshotTS. (buildSnap leaves TrackedSinceTS at 0.)
	snap.TrackedSinceTS = snap.SnapshotTS - int64(hours7d15h*3600)
	got := NewEngine().Derive(snap)
	if !containsCode(got, "currently_listed_at_observation_wall") {
		t.Fatalf("expected currently_listed_at_observation_wall to fire; got %v", insightCodes(got))
	}
	if containsCode(got, "currently_listed_age_p75") {
		t.Fatalf("expected currently_listed_age_p75 to stay silent at the wall; got %v", insightCodes(got))
	}
	if containsCode(got, "currently_listed_age_p100") {
		t.Fatalf("expected currently_listed_age_p100 to stay silent at the wall; got %v", insightCodes(got))
	}
}

// TestRuleCurrentlyListedAtObservationWall_SuppressedAtRest verifies
// that a feed with normal age spread does NOT trigger the wall rule —
// the regular percentile rules should remain active.
func TestRuleCurrentlyListedAtObservationWall_SuppressedAtRest(t *testing.T) {
	snap := buildSnap().withAgeOfListed(AgeHistogram{
		BucketsHours: []int{1, 24, 168, 720},
		Counts:       []uint64{100, 50, 30, 30},
		Total:        210,
	}).toSnap()
	// Long observation window so nothing sits at the wall.
	snap.TrackedSinceTS = snap.SnapshotTS - int64(365*24*3600)
	got := NewEngine().Derive(snap)
	if containsCode(got, "currently_listed_at_observation_wall") {
		t.Fatalf("expected wall rule to stay silent on a healthy histogram; got %v", insightCodes(got))
	}
}

func TestRuleCurrentlyListedAgeP100_SuppressedBySample(t *testing.T) {
	snap := buildSnap().withAgeOfListed(AgeHistogram{
		BucketsHours: []int{1, 720},
		Counts:       []uint64{2, 1},
		Total:        3,
	}).toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "currently_listed_age_p100") {
		t.Fatalf("expected currently_listed_age_p100 to stay silent; got %v", insightCodes(got))
	}
}

func TestRuleRemovedAgeP75_Fires(t *testing.T) {
	snap := buildSnap().withAgeOfRemoved(AgeHistogram{
		BucketsHours: []int{1, 24, 168},
		Counts:       []uint64{500, 800, 700},
		Total:        2000,
	}).toSnap()
	got := NewEngine().Derive(snap)
	if !containsCode(got, "removed_age_p75") {
		t.Fatalf("expected removed_age_p75 to fire; got %v", insightCodes(got))
	}
}

func TestRuleRemovedAgeP75_SuppressedBySample(t *testing.T) {
	snap := buildSnap().withAgeOfRemoved(AgeHistogram{
		BucketsHours: []int{1, 24},
		Counts:       []uint64{100, 50},
		Total:        150, // < 1000 guard
	}).toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "removed_age_p75") {
		t.Fatalf("expected removed_age_p75 to stay silent; got %v", insightCodes(got))
	}
}

func TestRuleMultipleRetentionPolicies_Fires(t *testing.T) {
	// Bimodal: 1000 IPs at 24h (fast), 500 IPs at 720h (slow).
	// cumulative at 24h = 1000, total = 1500, so p50 (cum >= 750) = 24h.
	// p90 (cum >= 1350) = 720h. ratio = 720/24 = 30 > 5.
	snap := buildSnap().withAgeOfRemoved(bimodalAgeHistogram(24, 720, 1000, 500)).toSnap()
	got := NewEngine().Derive(snap)
	if !containsCode(got, "multiple_retention_policies") {
		t.Fatalf("expected multiple_retention_policies to fire; got %v", insightCodes(got))
	}
}

func TestRuleMultipleRetentionPolicies_SuppressedOnUniform(t *testing.T) {
	// A uniform distribution across 24 hours: p50 and p90 are both
	// within the same order of magnitude so the ratio stays below 5.
	snap := buildSnap().withAgeOfRemoved(uniformAgeHistogram(24, 2400)).toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "multiple_retention_policies") {
		t.Fatalf("expected multiple_retention_policies to stay silent on uniform dist; got %v", insightCodes(got))
	}
}

func TestRuleMultipleRetentionPolicies_SuppressedBySample(t *testing.T) {
	// Would have a ratio > 5 but total is below 1000.
	snap := buildSnap().withAgeOfRemoved(bimodalAgeHistogram(1, 720, 5, 2)).toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "multiple_retention_policies") {
		t.Fatalf("expected multiple_retention_policies to stay silent with 7 samples; got %v", insightCodes(got))
	}
}

func TestRulePermanentBans_Fires(t *testing.T) {
	// Three modes: fast (1000 at 1h), medium (800 at 24h), rare (200 at 2400h).
	// cum at 1h=1000, at 24h=1800, at 2400h=2000. total = 2000.
	// p90 threshold = 1800; hits at 24h. p100 = 2400h. ratio = 100 > 10.
	snap := buildSnap().withAgeOfRemoved(AgeHistogram{
		BucketsHours: []int{1, 24, 2400},
		Counts:       []uint64{1000, 800, 200},
		Total:        2000,
	}).toSnap()
	got := NewEngine().Derive(snap)
	if !containsCode(got, "permanent_bans") {
		t.Fatalf("expected permanent_bans to fire; got %v", insightCodes(got))
	}
}

func TestRulePermanentBans_SuppressedOnShortTail(t *testing.T) {
	// p90/p100 ratio close to 1 — no long tail.
	snap := buildSnap().withAgeOfRemoved(AgeHistogram{
		BucketsHours: []int{1, 24, 48},
		Counts:       []uint64{800, 600, 600},
		Total:        2000,
	}).toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "permanent_bans") {
		t.Fatalf("expected permanent_bans to stay silent without a long tail; got %v", insightCodes(got))
	}
}

func TestRulePermanentBans_SuppressedBySample(t *testing.T) {
	snap := buildSnap().withAgeOfRemoved(AgeHistogram{
		BucketsHours: []int{1, 2400},
		Counts:       []uint64{10, 1},
		Total:        11,
	}).toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "permanent_bans") {
		t.Fatalf("expected permanent_bans to stay silent on 11 samples; got %v", insightCodes(got))
	}
}
