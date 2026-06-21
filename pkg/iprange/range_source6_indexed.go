package iprange

import "fmt"

type indexedRangeSource6 struct {
	ranges      []Range6
	bytes       []byte
	file        FileSet6
	length      int
	uniqueIPs   Uint128
	knownUnique bool
}

func indexedRangeSources6(sources []RangeSource6) ([]indexedRangeSource6, func(), bool, error) {
	unlock, err := lockMmapRangeSources6(sources)
	if err != nil {
		return nil, nil, true, err
	}

	indexed := make([]indexedRangeSource6, 0, len(sources))
	for _, src := range sources {
		if src == nil {
			if unlock != nil {
				unlock()
			}
			return nil, nil, true, fmt.Errorf("nil range source6")
		}
		if set, ok := src.(*IPSet6); ok {
			set.Optimize()
			indexed = append(indexed, indexedRangeSource6{
				ranges:      set.Ranges,
				length:      len(set.Ranges),
				uniqueIPs:   set.UniqueIPs,
				knownUnique: true,
			})
			continue
		}
		if mapped, ok := mmapIndexedRangeSource6(src); ok {
			indexed = append(indexed, mapped)
			continue
		}
		if fs, ok := src.(FileSet6); ok {
			indexed = append(indexed, indexedRangeSource6{
				file:        fs,
				length:      fs.Len(),
				uniqueIPs:   fs.UniqueIPs(),
				knownUnique: true,
			})
			continue
		}
		if unlock != nil {
			unlock()
		}
		return nil, nil, false, nil
	}
	return indexed, unlock, true, nil
}

func (s indexedRangeSource6) len() int {
	return s.length
}

func (s indexedRangeSource6) at(i int) (Range6, error) {
	switch {
	case s.ranges != nil:
		return s.ranges[i], nil
	case s.bytes != nil:
		return decodeRange6At(s.bytes, i), nil
	case s.file != nil:
		return s.file.Range(i)
	default:
		return Range6{}, fmt.Errorf("empty indexed range source6")
	}
}

func (s indexedRangeSource6) uniqueCount() (Uint128, bool) {
	return s.uniqueIPs, s.knownUnique
}

func decodeRange6At(data []byte, i int) Range6 {
	off := i * 32
	return decodeRange6(data[off : off+32])
}
