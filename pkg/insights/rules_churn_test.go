package insights

import "testing"

func TestRuleChurnHigh_Fires(t *testing.T) {
	// 60 points at 60% churn (added=300k, removed=300k, size=1M).
	snap := buildSnap().withChurnSeries(steadyChurnSeries(60, 1_000_000, 300_000, 300_000)).toSnap()
	got := NewEngine().Derive(snap)
	if !containsCode(got, "churn_high") {
		t.Fatalf("expected churn_high to fire at 60%% churn; got %v", insightCodes(got))
	}
}

func TestRuleChurnHigh_SuppressedBelowThreshold(t *testing.T) {
	// 60 points at 20% churn — under 50%.
	snap := buildSnap().withChurnSeries(steadyChurnSeries(60, 1_000_000, 100_000, 100_000)).toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "churn_high") {
		t.Fatalf("expected churn_high silent at 20%%; got %v", insightCodes(got))
	}
}

func TestRuleChurnHigh_SuppressedBySample(t *testing.T) {
	// Only 10 points — below the 50-point guard.
	snap := buildSnap().withChurnSeries(steadyChurnSeries(10, 1_000_000, 300_000, 300_000)).toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "churn_high") {
		t.Fatalf("expected churn_high silent with 10 points; got %v", insightCodes(got))
	}
}

func TestRuleChurnLow_Fires(t *testing.T) {
	// 60 points at 2% churn.
	snap := buildSnap().withChurnSeries(steadyChurnSeries(60, 1_000_000, 10_000, 10_000)).toSnap()
	got := NewEngine().Derive(snap)
	if !containsCode(got, "churn_low") {
		t.Fatalf("expected churn_low to fire at 2%% churn; got %v", insightCodes(got))
	}
}

func TestRuleChurnLow_SuppressedAboveThreshold(t *testing.T) {
	snap := buildSnap().withChurnSeries(steadyChurnSeries(60, 1_000_000, 100_000, 100_000)).toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "churn_low") {
		t.Fatalf("expected churn_low silent at 20%%; got %v", insightCodes(got))
	}
}

func TestRuleChurnLow_SuppressedBySample(t *testing.T) {
	snap := buildSnap().withChurnSeries(steadyChurnSeries(10, 1_000_000, 10_000, 10_000)).toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "churn_low") {
		t.Fatalf("expected churn_low silent with 10 points; got %v", insightCodes(got))
	}
}
