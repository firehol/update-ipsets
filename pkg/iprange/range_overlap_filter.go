package iprange

type rangePrefixBitmap [rangeSummaryPrefixWords]uint64

type rangeSparsePrefixSet struct {
	prefixes []uint32
}

type rangeCompactPrefixSet struct {
	indexes []uint16
	words   []uint64
}

type rangeSparsePrefixBuilder struct {
	inline    [rangeSummarySparsePrefixLimit]uint32
	prefixLen int
	last      uint32
	haveLast  bool
	overflow  bool
}

type rangeCompactPrefixBuilder struct {
	indexes  [rangeSummaryCompactWordLimit]uint16
	words    [rangeSummaryCompactWordLimit]uint64
	wordLen  int
	lastWord uint32
	active   bool
}

// RangeOverlapFilter is a conservative zero-overlap proof for two range
// sources. A false disjoint result means "unknown; run the exact scan".
type RangeOverlapFilter struct {
	valid        bool
	hasRange     bool
	bounds       Range
	prefixBitmap *rangePrefixBitmap
	compact      *rangeCompactPrefixSet
	sparsePrefix *rangeSparsePrefixSet
}

// Valid reports whether the filter was built from a known source.
func (f RangeOverlapFilter) Valid() bool {
	return f.valid
}

// HasRange reports whether the source had at least one range when the filter was
// built. A valid filter with no ranges represents a known-empty source.
func (f RangeOverlapFilter) HasRange() bool {
	return f.hasRange
}

// BoundsDisjoint reports whether min/max bounds prove zero overlap.
func (f RangeOverlapFilter) BoundsDisjoint(other RangeOverlapFilter) bool {
	if !f.valid || !other.valid {
		return false
	}
	if !f.hasRange || !other.hasRange {
		return true
	}
	return f.bounds.Hi < other.bounds.Lo || other.bounds.Hi < f.bounds.Lo
}

// SparsePrefixesDisjoint reports whether sparse occupied-prefix sets prove zero
// overlap. It returns false when either filter lacks precise sparse evidence.
func (f RangeOverlapFilter) SparsePrefixesDisjoint(other RangeOverlapFilter) bool {
	if !f.valid || !other.valid {
		return false
	}
	if !f.hasRange || !other.hasRange {
		return true
	}
	return !rangeSparsePrefixOverlap(f.sparsePrefix, other.sparsePrefix)
}

// PrefixesDisjoint reports whether coarse occupied-prefix bitmaps prove zero
// overlap. It returns false when either filter lacks prefix evidence.
func (f RangeOverlapFilter) PrefixesDisjoint(other RangeOverlapFilter) bool {
	if !f.valid || !other.valid {
		return false
	}
	if !f.hasRange || !other.hasRange {
		return true
	}
	return !rangeCoarsePrefixOverlap(f, other)
}

// Disjoint reports whether any conservative filter proves zero overlap.
func (f RangeOverlapFilter) Disjoint(other RangeOverlapFilter) bool {
	if f.BoundsDisjoint(other) {
		return true
	}
	if f.SparsePrefixesDisjoint(other) {
		return true
	}
	return f.PrefixesDisjoint(other)
}

func (b *rangeSparsePrefixBuilder) addRange(start, end uint32) {
	if b.overflow {
		return
	}
	if b.haveLast && start <= b.last {
		if end <= b.last {
			return
		}
		start = b.last + 1
	}
	count := uint64(end) - uint64(start) + 1
	if uint64(b.prefixLen)+count > rangeSummarySparsePrefixLimit {
		b.prefixLen = 0
		b.overflow = true
		return
	}
	for prefix := start; prefix <= end; prefix++ {
		b.inline[b.prefixLen] = prefix
		b.prefixLen++
		if prefix == end {
			break
		}
	}
	b.last = end
	b.haveLast = true
}

func (b *rangeSparsePrefixBuilder) wouldOverflow(start, end uint32) bool {
	if b == nil || b.overflow {
		return false
	}
	if b.haveLast && start <= b.last {
		if end <= b.last {
			return false
		}
		start = b.last + 1
	}
	count := uint64(end) - uint64(start) + 1
	return uint64(b.prefixLen)+count > rangeSummarySparsePrefixLimit
}

func (b *rangeSparsePrefixBuilder) set() *rangeSparsePrefixSet {
	if b == nil || b.overflow || b.prefixLen == 0 {
		return nil
	}
	prefixes := make([]uint32, b.prefixLen)
	copy(prefixes, b.inline[:b.prefixLen])
	return &rangeSparsePrefixSet{prefixes: prefixes}
}

func (b *rangeCompactPrefixBuilder) addSparsePrefixes(prefixes *rangeSparsePrefixBuilder) bool {
	if b == nil || prefixes == nil {
		return true
	}
	for i := 0; i < prefixes.prefixLen; i++ {
		prefix := prefixes.inline[i] >> (rangeSummarySparsePrefixBits - rangeSummaryPrefixBits)
		if !b.addPrefix(prefix) {
			return false
		}
	}
	return true
}

func (b *rangeCompactPrefixBuilder) addRange(start, end uint32) bool {
	if b == nil {
		return true
	}
	b.active = true
	startWord := start >> 6
	endWord := end >> 6
	for word := startWord; word <= endWord; word++ {
		firstBit := uint(0)
		if word == startWord {
			firstBit = uint(start & 63)
		}
		lastBit := uint(63)
		if word == endWord {
			lastBit = uint(end & 63)
		}

		mask := ^uint64(0)
		if firstBit > 0 {
			mask &^= (uint64(1) << firstBit) - 1
		}
		if lastBit < 63 {
			mask &= (uint64(1) << (lastBit + 1)) - 1
		}
		if !b.addWord(word, mask) {
			return false
		}
		if word == endWord {
			break
		}
	}
	return true
}

func (b *rangeCompactPrefixBuilder) addPrefix(prefix uint32) bool {
	if b == nil {
		return true
	}
	b.active = true
	return b.addWord(prefix>>6, uint64(1)<<(prefix&63))
}

func (b *rangeCompactPrefixBuilder) addWord(word uint32, mask uint64) bool {
	if b == nil || mask == 0 {
		return true
	}
	if b.wordLen > 0 && uint32(b.indexes[b.wordLen-1]) == word {
		b.words[b.wordLen-1] |= mask
		b.lastWord = word
		return true
	}
	if b.wordLen >= rangeSummaryCompactWordLimit {
		return false
	}
	b.indexes[b.wordLen] = uint16(word)
	b.words[b.wordLen] = mask
	b.wordLen++
	b.lastWord = word
	return true
}

func (b *rangeCompactPrefixBuilder) addToBitmap(bitmap *rangePrefixBitmap) {
	if b == nil || bitmap == nil {
		return
	}
	for i := 0; i < b.wordLen; i++ {
		bitmap[b.indexes[i]] |= b.words[i]
	}
}

func (b *rangeCompactPrefixBuilder) set(allow bool) *rangeCompactPrefixSet {
	if b == nil || !allow || !b.active || b.wordLen == 0 {
		return nil
	}
	indexes := make([]uint16, b.wordLen)
	copy(indexes, b.indexes[:b.wordLen])
	words := make([]uint64, b.wordLen)
	copy(words, b.words[:b.wordLen])
	return &rangeCompactPrefixSet{indexes: indexes, words: words}
}

func rangeSparsePrefixOverlap(a, b *rangeSparsePrefixSet) bool {
	if a == nil || b == nil {
		return true
	}
	i, j := 0, 0
	for i < len(a.prefixes) && j < len(b.prefixes) {
		switch {
		case a.prefixes[i] == b.prefixes[j]:
			return true
		case a.prefixes[i] < b.prefixes[j]:
			i++
		default:
			j++
		}
	}
	return false
}

func rangeCoarsePrefixOverlap(a, b RangeOverlapFilter) bool {
	switch {
	case a.prefixBitmap != nil:
		return rangePrefixBitmapEvidenceOverlap(a.prefixBitmap, b.prefixBitmap, b.compact, b.sparsePrefix)
	case a.compact != nil:
		return rangeCompactEvidenceOverlap(a.compact, b.prefixBitmap, b.compact, b.sparsePrefix)
	case a.sparsePrefix != nil:
		return rangeSparseCoarseEvidenceOverlap(a.sparsePrefix, b.prefixBitmap, b.compact, b.sparsePrefix)
	default:
		return true
	}
}

func rangePrefixBitmapEvidenceOverlap(a *rangePrefixBitmap, bitmap *rangePrefixBitmap, compact *rangeCompactPrefixSet, sparse *rangeSparsePrefixSet) bool {
	switch {
	case a == nil:
		return true
	case bitmap != nil:
		return rangePrefixOverlap(a, bitmap)
	case compact != nil:
		return rangeBitmapCompactOverlap(a, compact)
	case sparse != nil:
		return rangeBitmapSparseCoarsePrefixOverlap(a, sparse)
	default:
		return true
	}
}

func rangeCompactEvidenceOverlap(a *rangeCompactPrefixSet, bitmap *rangePrefixBitmap, compact *rangeCompactPrefixSet, sparse *rangeSparsePrefixSet) bool {
	switch {
	case a == nil:
		return true
	case bitmap != nil:
		return rangeBitmapCompactOverlap(bitmap, a)
	case compact != nil:
		return rangeCompactPrefixOverlap(a, compact)
	case sparse != nil:
		return rangeCompactSparseCoarsePrefixOverlap(a, sparse)
	default:
		return true
	}
}

func rangeSparseCoarseEvidenceOverlap(a *rangeSparsePrefixSet, bitmap *rangePrefixBitmap, compact *rangeCompactPrefixSet, sparse *rangeSparsePrefixSet) bool {
	switch {
	case a == nil:
		return true
	case bitmap != nil:
		return rangeBitmapSparseCoarsePrefixOverlap(bitmap, a)
	case compact != nil:
		return rangeCompactSparseCoarsePrefixOverlap(compact, a)
	case sparse != nil:
		return rangeSparseCoarsePrefixOverlap(a, sparse)
	default:
		return true
	}
}

func rangeSparseCoarsePrefixOverlap(a, b *rangeSparsePrefixSet) bool {
	if a == nil || b == nil {
		return true
	}
	const shift = rangeSummarySparsePrefixBits - rangeSummaryPrefixBits
	i, j := 0, 0
	for i < len(a.prefixes) && j < len(b.prefixes) {
		left := a.prefixes[i] >> shift
		right := b.prefixes[j] >> shift
		switch {
		case left == right:
			return true
		case left < right:
			i++
		default:
			j++
		}
	}
	return false
}

func rangeBitmapCompactOverlap(bitmap *rangePrefixBitmap, compact *rangeCompactPrefixSet) bool {
	if bitmap == nil || compact == nil {
		return true
	}
	for i, wordIndex := range compact.indexes {
		if bitmap[wordIndex]&compact.words[i] != 0 {
			return true
		}
	}
	return false
}

func rangeBitmapSparseCoarsePrefixOverlap(bitmap *rangePrefixBitmap, sparse *rangeSparsePrefixSet) bool {
	if bitmap == nil || sparse == nil {
		return true
	}
	const shift = rangeSummarySparsePrefixBits - rangeSummaryPrefixBits
	for _, sparsePrefix := range sparse.prefixes {
		prefix := sparsePrefix >> shift
		if bitmap[prefix>>6]&(uint64(1)<<(prefix&63)) != 0 {
			return true
		}
	}
	return false
}

func rangeCompactPrefixOverlap(a, b *rangeCompactPrefixSet) bool {
	if a == nil || b == nil {
		return true
	}
	i, j := 0, 0
	for i < len(a.indexes) && j < len(b.indexes) {
		switch {
		case a.indexes[i] == b.indexes[j]:
			if a.words[i]&b.words[j] != 0 {
				return true
			}
			i++
			j++
		case a.indexes[i] < b.indexes[j]:
			i++
		default:
			j++
		}
	}
	return false
}

func rangeCompactSparseCoarsePrefixOverlap(compact *rangeCompactPrefixSet, sparse *rangeSparsePrefixSet) bool {
	if compact == nil || sparse == nil {
		return true
	}
	const shift = rangeSummarySparsePrefixBits - rangeSummaryPrefixBits
	wordIndex := 0
	for _, sparsePrefix := range sparse.prefixes {
		prefix := sparsePrefix >> shift
		word := uint16(prefix >> 6)
		for wordIndex < len(compact.indexes) && compact.indexes[wordIndex] < word {
			wordIndex++
		}
		if wordIndex >= len(compact.indexes) {
			return false
		}
		if compact.indexes[wordIndex] == word && compact.words[wordIndex]&(uint64(1)<<(prefix&63)) != 0 {
			return true
		}
	}
	return false
}

func rangePrefixOverlap(a, b *rangePrefixBitmap) bool {
	if a == nil || b == nil {
		return true
	}
	for i := range a {
		if a[i]&b[i] != 0 {
			return true
		}
	}
	return false
}

func (b *rangePrefixBitmap) addRange(start, end uint32) {
	if b == nil {
		return
	}
	for prefix := start; prefix <= end; prefix++ {
		b[prefix>>6] |= uint64(1) << (prefix & 63)
		if prefix == end {
			break
		}
	}
}

func (b *rangePrefixBitmap) addSparsePrefixes(prefixes *rangeSparsePrefixBuilder) {
	if b == nil || prefixes == nil {
		return
	}
	for i := 0; i < prefixes.prefixLen; i++ {
		sparsePrefix := prefixes.inline[i]
		prefix := sparsePrefix >> (rangeSummarySparsePrefixBits - rangeSummaryPrefixBits)
		b[prefix>>6] |= uint64(1) << (prefix & 63)
	}
}
