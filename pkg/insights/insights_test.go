package insights

import (
	"slices"
	"testing"
)

// TestCatalogCompleteness asserts that every rule referenced in the
// spec is present in the catalog. This is the single canary that
// breaks when somebody accidentally deletes a rule.
func TestCatalogCompleteness(t *testing.T) {
	want := []string{
		"bogon_present",
		"churn_high",
		"churn_low",
		"country_concentrated",
		"country_diverse",
		"cross_category_overlap",
		"currently_listed_age_p100",
		"currently_listed_age_p75",
		"currently_listed_at_observation_wall",
		"independent",
		"infrastructure_present",
		"multiple_retention_policies",
		"permanent_bans",
		"removed_age_p75",
		"single_country",
		"size_variation",
		"subset_of",
	}
	have := make([]string, 0, len(catalog))
	for _, r := range catalog {
		have = append(have, r.Code)
	}
	slices.Sort(have)
	slices.Sort(want)
	if len(have) != len(want) {
		t.Fatalf("catalog size mismatch: want %d, have %d (%v vs %v)", len(want), len(have), want, have)
	}
	for i := range want {
		if have[i] != want[i] {
			t.Fatalf("catalog mismatch at %d: want %q, have %q", i, want[i], have[i])
		}
	}
}

// TestRuleMetadata asserts every rule has a populated Code, Name,
// Methodology path, and non-nil Compute function. This catches missing
// methodology pages and typo'd registration bugs.
func TestRuleMetadata(t *testing.T) {
	for _, r := range catalog {
		if r.Code == "" {
			t.Errorf("rule with empty code: %+v", r)
		}
		if r.Name == "" {
			t.Errorf("rule %q: missing Name", r.Code)
		}
		if r.Methodology == "" {
			t.Errorf("rule %q: missing Methodology", r.Code)
		}
		if r.Compute == nil {
			t.Errorf("rule %q: nil Compute", r.Code)
		}
		if r.MinSamples == nil {
			t.Errorf("rule %q: nil MinSamples", r.Code)
		}
	}
}

// TestKnownGoodSnapshot runs the full catalog against a carefully
// constructed snapshot that is designed to fire an exact set of rules.
// When a threshold changes unintentionally, this test breaks — the
// break forces whoever is changing the threshold to update the expected
// output as part of the same commit.
func TestKnownGoodSnapshot(t *testing.T) {
	// Deliberately constructed to fire: size_variation, churn_high,
	// country_concentrated, bogon_present, infrastructure_present,
	// subset_of. Everything else must stay silent.
	s := SignalSnapshot{
		Name:     "canary",
		Category: "attacks",
		TotalIPs: 1_000_000,
		// Size series: 60 points ranging 500k-1.5M (fires R01).
		SizeSeries: rangingSizeSeries(60, 500_000, 1_500_000),
		// Churn series: 60 points at 60% churn (fires R13).
		ChurnSeries: steadyChurnSeries(60, 1_000_000, 300_000, 300_000),
		// Currently-listed age: below guard, R02/R03 stay silent.
		AgeOfListed: AgeHistogram{
			BucketsHours: []int{1, 24},
			Counts:       []uint64{10, 5},
			Total:        15,
		},
		// Removed age: below 1000 guard, R10/R11/R12 stay silent.
		AgeOfRemoved: AgeHistogram{
			BucketsHours: []int{1, 24},
			Counts:       []uint64{10, 5},
			Total:        15,
		},
		// Country distribution: top3 = 0.40+0.20+0.15 = 0.75 > 0.70
		// but top1 <= 0.95 so R05 fires and R07 does not.
		TopCountries: []CountryShare{
			{Code: "FR", Name: "France", IPs: 400_000, Share: 0.40},
			{Code: "DE", Name: "Germany", IPs: 200_000, Share: 0.20},
			{Code: "RU", Name: "Russia", IPs: 150_000, Share: 0.15},
			{Code: "US", IPs: 150_000, Share: 0.15},
			{Code: "BR", IPs: 100_000, Share: 0.10},
		},
		BogonShare: 0.001,
		InfraShare: 0.0001,
		// 5 overlaps: one at 0.98 with OlderThanThis=true (R15 fires).
		// Max OurShare = 0.98 so R04 stays silent.
		Overlaps: []FeedOverlap{
			{Other: "a", Category: "attacks", OurShare: 0.98, OlderThanThis: true},
			{Other: "b", Category: "attacks", OurShare: 0.05},
			{Other: "c", Category: "attacks", OurShare: 0.03},
			{Other: "d", Category: "malware", OurShare: 0.02},
			{Other: "e", Category: "malware", OurShare: 0.01},
		},
		OverlapsByCat: map[string]CategoryStat{
			"attacks": {MaxShare: 0.98, FeedCount: 3},
			"malware": {MaxShare: 0.02, FeedCount: 2},
		},
	}
	got := NewEngine().Derive(s)
	gotCodes := insightCodes(got)
	slices.Sort(gotCodes)

	want := []string{
		"bogon_present",
		"churn_high",
		"country_concentrated",
		"infrastructure_present",
		"size_variation",
		"subset_of",
	}
	slices.Sort(want)

	if len(gotCodes) != len(want) {
		t.Fatalf("unexpected insight set: got %v, want %v", gotCodes, want)
	}
	for i := range want {
		if gotCodes[i] != want[i] {
			t.Fatalf("insight set mismatch: got %v, want %v", gotCodes, want)
		}
	}
}

// TestDeriveFillsMetadata asserts that the engine populates Code,
// Section, and Methodology on every returned insight from the Rule
// definition (rules should not duplicate these fields themselves).
func TestDeriveFillsMetadata(t *testing.T) {
	s := buildSnap().
		withSizeSeries(rangingSizeSeries(60, 500_000, 1_500_000)).
		toSnap()
	got := NewEngine().Derive(s)
	var found bool
	for _, ins := range got {
		if ins.Code == "size_variation" {
			found = true
			if ins.Section != SectionOverview {
				t.Errorf("size_variation: expected SectionOverview, got %v", ins.Section)
			}
			if ins.Methodology == "" {
				t.Errorf("size_variation: methodology not populated")
			}
			if ins.Headline == "" {
				t.Errorf("size_variation: headline empty")
			}
		}
	}
	if !found {
		t.Fatalf("size_variation not in derived insights")
	}
}
