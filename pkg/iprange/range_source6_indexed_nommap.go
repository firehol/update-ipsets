//go:build !linux && !darwin

package iprange

func mmapIndexedRangeSource6(RangeSource6) (indexedRangeSource6, bool) {
	return indexedRangeSource6{}, false
}

func lockMmapRangeSources6([]RangeSource6) (func(), error) {
	return nil, nil
}
