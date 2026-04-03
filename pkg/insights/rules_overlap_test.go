package insights

import "testing"

func TestRuleIndependent_Fires(t *testing.T) {
	// 6 feeds all with very low OurShare => independent.
	overlaps := []FeedOverlap{
		{Other: "a", Category: "attacks", OurShare: 0.01},
		{Other: "b", Category: "attacks", OurShare: 0.02},
		{Other: "c", Category: "attacks", OurShare: 0.04},
		{Other: "d", Category: "malware", OurShare: 0.05},
		{Other: "e", Category: "malware", OurShare: 0.03},
		{Other: "f", Category: "attacks", OurShare: 0.08},
	}
	snap := buildSnap().withOverlaps(overlaps).toSnap()
	got := NewEngine().Derive(snap)
	if !containsCode(got, "independent") {
		t.Fatalf("expected independent to fire; got %v", insightCodes(got))
	}
}

func TestRuleIndependent_SuppressedOnHighOverlap(t *testing.T) {
	// One overlap above 10% — silent.
	overlaps := []FeedOverlap{
		{Other: "a", OurShare: 0.01},
		{Other: "b", OurShare: 0.02},
		{Other: "c", OurShare: 0.20},
		{Other: "d", OurShare: 0.05},
		{Other: "e", OurShare: 0.03},
	}
	snap := buildSnap().withOverlaps(overlaps).toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "independent") {
		t.Fatalf("expected independent silent with 20%% overlap; got %v", insightCodes(got))
	}
}

func TestRuleIndependent_SuppressedBySample(t *testing.T) {
	// Only 3 overlaps — below the 5-feed requirement.
	overlaps := []FeedOverlap{
		{Other: "a", OurShare: 0.01},
		{Other: "b", OurShare: 0.02},
		{Other: "c", OurShare: 0.04},
	}
	snap := buildSnap().withOverlaps(overlaps).toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "independent") {
		t.Fatalf("expected independent silent with 3 feeds; got %v", insightCodes(got))
	}
}

func TestRuleSubsetOf_Fires(t *testing.T) {
	overlaps := []FeedOverlap{
		{Other: "spamhaus_drop", Category: "abuse", OurShare: 0.98, TheirShare: 0.10, OlderThanThis: true},
	}
	snap := buildSnap().withOverlaps(overlaps).toSnap()
	got := NewEngine().Derive(snap)
	if !containsCode(got, "subset_of") {
		t.Fatalf("expected subset_of to fire; got %v", insightCodes(got))
	}
}

func TestRuleSubsetOf_SuppressedWhenNotOlder(t *testing.T) {
	// 98% overlap but the other feed is NOT older — we cannot
	// meaningfully call ourselves a subset of it.
	overlaps := []FeedOverlap{
		{Other: "new_feed", Category: "abuse", OurShare: 0.98, OlderThanThis: false},
	}
	snap := buildSnap().withOverlaps(overlaps).toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "subset_of") {
		t.Fatalf("expected subset_of silent when candidate is not older; got %v", insightCodes(got))
	}
}

func TestRuleSubsetOf_SuppressedBySample(t *testing.T) {
	snap := buildSnap().withTotalIPs(50).withOverlaps([]FeedOverlap{
		{Other: "foo", OurShare: 0.99, OlderThanThis: true},
	}).toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "subset_of") {
		t.Fatalf("expected subset_of silent on 50 IPs; got %v", insightCodes(got))
	}
}

func TestRuleCrossCategoryOverlap_Fires(t *testing.T) {
	snap := buildSnap().withCategory("attacks").withOverlapsByCat(map[string]CategoryStat{
		"attacks": {MaxShare: 0.40, FeedCount: 5},
		"malware": {MaxShare: 0.45, FeedCount: 4, ExampleFeed: "zeus_tracker"},
	}).toSnap()
	got := NewEngine().Derive(snap)
	if !containsCode(got, "cross_category_overlap") {
		t.Fatalf("expected cross_category_overlap to fire; got %v", insightCodes(got))
	}
}

func TestRuleCrossCategoryOverlap_SuppressedBelowThreshold(t *testing.T) {
	snap := buildSnap().withCategory("attacks").withOverlapsByCat(map[string]CategoryStat{
		"malware": {MaxShare: 0.20, FeedCount: 4},
	}).toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "cross_category_overlap") {
		t.Fatalf("expected cross_category_overlap silent at 20%%; got %v", insightCodes(got))
	}
}

func TestRuleCrossCategoryOverlap_SuppressedBySample(t *testing.T) {
	snap := buildSnap().withCategory("attacks").withOverlapsByCat(map[string]CategoryStat{
		// Good overlap but only 1 feed in that category.
		"malware": {MaxShare: 0.50, FeedCount: 1},
	}).toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "cross_category_overlap") {
		t.Fatalf("expected cross_category_overlap silent with 1 feed in target category; got %v", insightCodes(got))
	}
}
