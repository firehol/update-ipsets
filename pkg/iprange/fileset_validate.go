package iprange

import "fmt"

// validateSortedRanges verifies that ranges are well-formed (Lo <= Hi),
// sorted, and non-overlapping. This catches corrupt files that claim
// "optimized" in their header but have shuffled or invalid data.
func validateSortedRanges(count int, readRange func(int) (Range, error)) error {
	var prev Range
	for i := 0; i < count; i++ {
		r, err := readRange(i)
		if err != nil {
			return fmt.Errorf("reading range %d: %w", i, err)
		}
		if r.Lo > r.Hi {
			return fmt.Errorf("range %d: Lo (%d) > Hi (%d)", i, r.Lo, r.Hi)
		}
		if i > 0 && prev.Hi >= r.Lo {
			return fmt.Errorf("range %d: not sorted or overlapping (prev.Hi=%d >= Lo=%d)", i, prev.Hi, r.Lo)
		}
		prev = r
	}
	return nil
}

func validateSortedRanges6(count int, readRange func(int) (Range6, error)) error {
	var prevHi Uint128
	for i := 0; i < count; i++ {
		r, err := readRange(i)
		if err != nil {
			return fmt.Errorf("reading range6 %d: %w", i, err)
		}
		if r.Lo.GreaterThan(r.Hi) {
			return fmt.Errorf("range6 %d: Lo > Hi", i)
		}
		if i > 0 && !prevHi.LessThan(r.Lo) {
			return fmt.Errorf("range6 %d: not sorted or overlapping", i)
		}
		prevHi = r.Hi
	}
	return nil
}
