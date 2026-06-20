//go:build linux || darwin

package iprange

import (
	"slices"
	"unsafe"
)

func mmapIndexedRangeSource6(src RangeSource6) (indexedRangeSource6, bool) {
	mapped, ok := src.(*mmapFileSet6)
	if !ok {
		return indexedRangeSource6{}, false
	}
	return indexedRangeSource6{
		bytes:       mapped.rangesData,
		length:      mapped.records,
		uniqueIPs:   mapped.uniqueIPs_,
		knownUnique: true,
	}, true
}

func lockMmapRangeSources6(sources []RangeSource6) (func(), error) {
	if len(sources) == 0 {
		return func() {}, nil
	}
	var mapped []*mmapFileSet6
	var seen map[*mmapFileSet6]struct{}
	for _, src := range sources {
		m, ok := src.(*mmapFileSet6)
		if !ok {
			continue
		}
		if seen == nil {
			seen = make(map[*mmapFileSet6]struct{}, len(sources))
			mapped = make([]*mmapFileSet6, 0, len(sources))
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
	slices.SortFunc(mapped, func(a, b *mmapFileSet6) int {
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

	locked := make([]*mmapFileSet6, 0, len(mapped))
	for _, m := range mapped {
		if m.closed.Load() {
			for i := len(locked) - 1; i >= 0; i-- {
				locked[i].mu.RUnlock()
			}
			return nil, ErrFileSet6Closed
		}
		m.mu.RLock()
		if m.closed.Load() {
			m.mu.RUnlock()
			for i := len(locked) - 1; i >= 0; i-- {
				locked[i].mu.RUnlock()
			}
			return nil, ErrFileSet6Closed
		}
		locked = append(locked, m)
	}
	return func() {
		for i := len(locked) - 1; i >= 0; i-- {
			locked[i].mu.RUnlock()
		}
	}, nil
}
