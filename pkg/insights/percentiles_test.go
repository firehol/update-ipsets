package insights

import "testing"

func TestPercentileHours_EmptyHistogram(t *testing.T) {
	if got := percentileHours(AgeHistogram{}, 0.75); got != 0 {
		t.Errorf("empty histogram: expected 0, got %d", got)
	}
}

func TestPercentileHours_SingleBucket(t *testing.T) {
	h := AgeHistogram{
		BucketsHours: []int{24},
		Counts:       []uint64{500},
		Total:        500,
	}
	for _, q := range []float64{0.1, 0.5, 0.75, 0.9, 1.0} {
		if got := percentileHours(h, q); got != 24 {
			t.Errorf("q=%v: expected 24, got %d", q, got)
		}
	}
}

func TestPercentileHours_P75Math(t *testing.T) {
	// cumulative: 100, 200, 300. Total 300. p75 threshold = 225.
	// cum at bucket 24 = 200 (not yet). cum at bucket 168 = 300 (>= 225).
	// Result: 168.
	h := AgeHistogram{
		BucketsHours: []int{1, 24, 168},
		Counts:       []uint64{100, 100, 100},
		Total:        300,
	}
	if got := percentileHours(h, 0.75); got != 168 {
		t.Errorf("expected 168, got %d", got)
	}
}

func TestPercentileHours_P100ReturnsMax(t *testing.T) {
	h := AgeHistogram{
		BucketsHours: []int{1, 24, 720},
		Counts:       []uint64{800, 150, 50},
		Total:        1000,
	}
	if got := percentileHours(h, 1.0); got != 720 {
		t.Errorf("p100: expected 720, got %d", got)
	}
}

func TestPercentileHours_P50OnSkewedDistribution(t *testing.T) {
	// Heavy at the first bucket. p50 threshold = 500. cum at 1h = 800.
	h := AgeHistogram{
		BucketsHours: []int{1, 24, 720},
		Counts:       []uint64{800, 150, 50},
		Total:        1000,
	}
	if got := percentileHours(h, 0.5); got != 1 {
		t.Errorf("p50: expected 1, got %d", got)
	}
}
