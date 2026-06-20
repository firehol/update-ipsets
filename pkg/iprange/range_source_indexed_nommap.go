//go:build !linux && !darwin

package iprange

func mmapIndexedRangeSource(RangeSource) (indexedRangeSource, bool) {
	return indexedRangeSource{}, false
}

func lockMmapRangeSources([]RangeSource) (func(), error) {
	return func() {}, nil
}
