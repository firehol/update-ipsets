package iprange

import "fmt"

type indexedRangeSource struct {
	ranges      []Range
	bytes       []byte
	file        FileSet
	length      int
	uniqueIPs   uint64
	knownUnique bool
}

func indexedRangeSources(sources []RangeSource) ([]indexedRangeSource, func(), bool, error) {
	unlock, err := lockMmapRangeSources(sources)
	if err != nil {
		return nil, nil, true, err
	}
	if unlock == nil {
		unlock = func() {}
	}

	indexed := make([]indexedRangeSource, 0, len(sources))
	for _, src := range sources {
		if src == nil {
			unlock()
			return nil, nil, true, fmt.Errorf("nil range source")
		}
		if set, ok := src.(*IPSet); ok {
			set.Optimize()
			indexed = append(indexed, indexedRangeSource{
				ranges:      set.Ranges,
				length:      len(set.Ranges),
				uniqueIPs:   set.UniqueIPs,
				knownUnique: true,
			})
			continue
		}
		if mapped, ok := mmapIndexedRangeSource(src); ok {
			indexed = append(indexed, mapped)
			continue
		}
		if fs, ok := src.(FileSet); ok {
			indexed = append(indexed, indexedRangeSource{
				file:        fs,
				length:      fs.Len(),
				uniqueIPs:   fs.UniqueIPs(),
				knownUnique: true,
			})
			continue
		}
		unlock()
		return nil, nil, false, nil
	}
	return indexed, unlock, true, nil
}

func (s indexedRangeSource) len() int {
	return s.length
}

func (s indexedRangeSource) at(i int) (Range, error) {
	switch {
	case s.ranges != nil:
		return s.ranges[i], nil
	case s.bytes != nil:
		return decodeRangeAt(s.bytes, i), nil
	case s.file != nil:
		return s.file.Range(i)
	default:
		return Range{}, fmt.Errorf("empty indexed range source")
	}
}

func (s indexedRangeSource) uniqueCount() (uint64, bool) {
	return s.uniqueIPs, s.knownUnique
}
