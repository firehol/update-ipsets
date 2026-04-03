package insights

import "testing"

func TestRuleSizeVariation_Fires(t *testing.T) {
	snap := buildSnap().withSizeSeries(rangingSizeSeries(60, 500_000, 1_500_000)).toSnap()
	got := NewEngine().Derive(snap)
	if !containsCode(got, "size_variation") {
		t.Fatalf("expected size_variation to fire; got %v", insightCodes(got))
	}
}

func TestRuleSizeVariation_SuppressedWhenSteady(t *testing.T) {
	// min == max: no range to report.
	snap := buildSnap().withSizeSeries(steadySizeSeries(60, 1_000_000)).toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "size_variation") {
		t.Fatalf("expected size_variation to stay silent on steady series; got %v", insightCodes(got))
	}
}

func TestRuleSizeVariation_SuppressedBySample(t *testing.T) {
	// Only 10 points — below the 50-point guard.
	snap := buildSnap().withSizeSeries(rangingSizeSeries(10, 500_000, 1_500_000)).toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "size_variation") {
		t.Fatalf("expected size_variation to stay silent with 10 points; got %v", insightCodes(got))
	}
}
