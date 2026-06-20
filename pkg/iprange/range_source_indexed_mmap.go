//go:build linux || darwin

package iprange

import (
	"slices"
	"unsafe"
)

func mmapIndexedRangeSource(src RangeSource) (indexedRangeSource, bool) {
	mapped, ok := src.(*mmapFileSet)
	if !ok {
		return indexedRangeSource{}, false
	}
	return indexedRangeSource{
		bytes:       mapped.rangesData,
		length:      mapped.records,
		uniqueIPs:   mapped.uniqueIPs_,
		knownUnique: true,
	}, true
}

func lockMmapRangeSources(sources []RangeSource) (func(), error) {
	if len(sources) == 0 {
		return func() {}, nil
	}
	var mapped []*mmapFileSet
	var seen map[*mmapFileSet]struct{}
	for _, src := range sources {
		m, ok := src.(*mmapFileSet)
		if !ok {
			continue
		}
		if seen == nil {
			seen = make(map[*mmapFileSet]struct{}, len(sources))
			mapped = make([]*mmapFileSet, 0, len(sources))
		}
		if _, exists := seen[m]; exists {
			continue
		}
		seen[m] = struct{}{}
		mapped = append(mapped, m)
	}
	if len(mapped) == 0 {
		return func() {}, nil
	}
	slices.SortFunc(mapped, func(a, b *mmapFileSet) int {
		ap := uintptr(unsafe.Pointer(a))
		bp := uintptr(unsafe.Pointer(b))
		switch {
		case ap < bp:
			return -1
		case ap > bp:
			return 1
		default:
			return 0
		}
	})

	locked := make([]*mmapFileSet, 0, len(mapped))
	for _, m := range mapped {
		if err := m.lockFastReader(); err != nil {
			for i := len(locked) - 1; i >= 0; i-- {
				locked[i].mu.RUnlock()
			}
			return nil, err
		}
		locked = append(locked, m)
	}
	return func() {
		for i := len(locked) - 1; i >= 0; i-- {
			locked[i].mu.RUnlock()
		}
	}, nil
}
