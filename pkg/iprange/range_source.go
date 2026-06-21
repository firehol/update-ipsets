package iprange

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"iter"
	"sync"
)

const (
	rangeSummaryPrefixBits        = 20
	rangeSummaryPrefixShift       = 32 - rangeSummaryPrefixBits
	rangeSummaryPrefixWords       = 1 << (rangeSummaryPrefixBits - 6)
	rangeSummaryCompactWordLimit  = 2048
	rangeSummarySparsePrefixBits  = 24
	rangeSummarySparsePrefixShift = 32 - rangeSummarySparsePrefixBits
	rangeSummarySparsePrefixLimit = 8192
)

type rangeSourceFunc struct {
	seq func(yield func(Range) bool)
	len int
}

type rangeSourceErrFunc struct {
	seq func(yield func(Range) bool) error
	len int
	mu  sync.Mutex
	err error
}

// RangeSourceFromIter wraps a range iterator as a RangeSource. The len argument
// is a best-effort range count; pass -1 when it is unknown.
func RangeSourceFromIter(seq func(yield func(Range) bool), len int) RangeSource {
	return rangeSourceFunc{seq: seq, len: len}
}

// RangeSourceFromIterErr wraps an error-returning range walker as a RangeSource.
// The last completed walk error is exposed through RangeSourceErr.
func RangeSourceFromIterErr(seq func(yield func(Range) bool) error, len int) RangeSource {
	return &rangeSourceErrFunc{seq: seq, len: len}
}

func (s rangeSourceFunc) Len() int {
	if s.len < 0 {
		return 0
	}
	return s.len
}

func (s rangeSourceFunc) Iter() func(yield func(Range) bool) {
	if s.seq == nil {
		return func(func(Range) bool) {}
	}
	return s.seq
}

func (s *rangeSourceErrFunc) Len() int {
	if s == nil || s.len < 0 {
		return 0
	}
	return s.len
}

func (s *rangeSourceErrFunc) Iter() func(yield func(Range) bool) {
	return func(yield func(Range) bool) {
		if s == nil {
			return
		}
		var err error
		if s.seq != nil {
			err = s.seq(yield)
		}
		s.mu.Lock()
		s.err = err
		s.mu.Unlock()
	}
}

func (s *rangeSourceErrFunc) Err() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// CollectIterContext materializes a range iterator into an optimized IPSet.
func CollectIterContext(ctx context.Context, name string, seq func(yield func(Range) bool)) (*IPSet, error) {
	ctx = rangeSourceContext(ctx)
	set := New(name)
	var count int
	for r := range seq {
		count++
		if count%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, fmt.Errorf("collect ranges %s: %w", name, err)
			}
		}
		if err := set.AddRange(r); err != nil {
			return nil, fmt.Errorf("collect ranges %s add %s: %w", name, r.String(), err)
		}
	}
	set.Optimize()
	return set, ctx.Err()
}

// CountIterContext counts unique IPs yielded by a range iterator.
func CountIterContext(ctx context.Context, name string, seq func(yield func(Range) bool)) (uint64, error) {
	ctx = rangeSourceContext(ctx)
	var total uint64
	var count int
	for r := range seq {
		count++
		if count%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return 0, fmt.Errorf("count ranges %s: %w", name, err)
			}
		}
		total += r.Size()
	}
	return total, ctx.Err()
}

// RangeSourceErr returns the last recorded error for sources that expose Err.
func RangeSourceErr(src RangeSource) error {
	if withErr, ok := src.(interface{ Err() error }); ok {
		return withErr.Err()
	}
	return nil
}

// RangeSourceUniqueIPs returns the unique IP count for a RangeSource, using
// source metadata when available and falling back to bounded iteration.
func RangeSourceUniqueIPs(ctx context.Context, src RangeSource) (uint64, error) {
	ctx = rangeSourceContext(ctx)
	if src == nil {
		return 0, nil
	}
	if counter, ok := src.(interface{ UniqueIPs() uint64 }); ok {
		return counter.UniqueIPs(), nil
	}
	if counter, ok := src.(interface{ UniqueCount() uint64 }); ok {
		return counter.UniqueCount(), nil
	}
	total, err := CountIterContext(ctx, DefaultName, src.Iter())
	if err != nil {
		return 0, err
	}
	return total, RangeSourceErr(src)
}

// RangeSourceContains checks whether src contains ip and returns plain local
// counters for the lookup. Known indexed sources use binary search; arbitrary
// RangeSource values fall back to a sorted linear scan.
func RangeSourceContains(ctx context.Context, src RangeSource, ip uint32) (bool, OperationStats, error) {
	ctx = rangeSourceContext(ctx)
	if src == nil {
		return false, OperationStats{}, nil
	}
	if err := ctx.Err(); err != nil {
		return false, OperationStats{}, err
	}
	switch set := src.(type) {
	case *IPSet:
		ok, stats := set.containsWithStats(ip)
		return ok, stats, nil
	case FileSet:
		ok, stats := fileSetContainsWithStats(ip, set.Len(), set.Range)
		return ok, stats, RangeSourceErr(set)
	}

	stats := OperationStats{Lookups: 1}
	for r := range src.Iter() {
		stats.RangesScanned++
		stats.Comparisons++
		if r.Hi < ip {
			continue
		}
		stats.Comparisons++
		return r.Lo <= ip, stats, RangeSourceErr(src)
	}
	if err := RangeSourceErr(src); err != nil {
		return false, stats, err
	}
	return false, stats, ctx.Err()
}

// RangeSourceBoundsContext returns the first and last IPv4 values covered by
// src. ok is false for nil or empty sources.
func RangeSourceBoundsContext(ctx context.Context, src RangeSource) (bounds Range, ok bool, err error) {
	ctx = rangeSourceContext(ctx)
	if src == nil {
		return Range{}, false, nil
	}
	if err := ctx.Err(); err != nil {
		return Range{}, false, err
	}
	switch set := src.(type) {
	case FileSet:
		if set.Len() == 0 {
			return Range{}, false, RangeSourceErr(src)
		}
		first, err := set.Range(0)
		if err != nil {
			return Range{}, false, err
		}
		last, err := set.Range(set.Len() - 1)
		if err != nil {
			return Range{}, false, err
		}
		return Range{Lo: first.Lo, Hi: last.Hi}, true, RangeSourceErr(src)
	case *IPSet:
		set.Optimize()
		if len(set.Ranges) == 0 {
			return Range{}, false, nil
		}
		return Range{Lo: set.Ranges[0].Lo, Hi: set.Ranges[len(set.Ranges)-1].Hi}, true, nil
	default:
		first := true
		for r := range set.Iter() {
			if err := ctx.Err(); err != nil {
				return Range{}, false, err
			}
			if first {
				bounds.Lo = r.Lo
				first = false
			}
			bounds.Hi = r.Hi
		}
		if err := RangeSourceErr(src); err != nil {
			return Range{}, false, err
		}
		if err := ctx.Err(); err != nil {
			return Range{}, false, err
		}
		return bounds, !first, nil
	}
}

// RangeContentHash is a stable hash of a normalized range stream.
type RangeContentHash struct {
	Sum   [sha256.Size]byte
	Valid bool
}

// Equal reports whether both hashes are valid and identical.
func (h RangeContentHash) Equal(other RangeContentHash) bool {
	return h.Valid && other.Valid && h.Sum == other.Sum
}

// Hex returns the lowercase hexadecimal representation of a valid hash.
func (h RangeContentHash) Hex() string {
	if !h.Valid {
		return ""
	}
	return hex.EncodeToString(h.Sum[:])
}

// RangeSourceContentHashContext returns a stable hash of the normalized range
// stream.
func RangeSourceContentHashContext(ctx context.Context, src RangeSource) (RangeContentHash, error) {
	ctx = rangeSourceContext(ctx)
	if src == nil {
		return RangeContentHash{}, nil
	}
	h := sha256.New()
	var buf [8]byte
	var count int
	for r := range src.Iter() {
		count++
		if count%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return RangeContentHash{}, err
			}
		}
		binary.BigEndian.PutUint32(buf[0:4], r.Lo)
		binary.BigEndian.PutUint32(buf[4:8], r.Hi)
		_, _ = h.Write(buf[:])
	}
	if err := RangeSourceErr(src); err != nil {
		return RangeContentHash{}, err
	}
	if err := ctx.Err(); err != nil {
		return RangeContentHash{}, err
	}
	var out RangeContentHash
	copy(out.Sum[:], h.Sum(nil))
	out.Valid = true
	return out, nil
}

// RangeSourceSummary is a reusable summary for fast conservative range-source
// comparisons.
type RangeSourceSummary struct {
	Bounds      Range
	HasRange    bool
	ContentHash RangeContentHash
	filter      RangeOverlapFilter
}

// OverlapFilter returns the conservative overlap filter derived from the
// summary.
func (s RangeSourceSummary) OverlapFilter() RangeOverlapFilter {
	return s.filter
}

// BuildRangeSourceSummaryContext scans src once to produce bounds, prefix
// filters, and a normalized content hash.
func BuildRangeSourceSummaryContext(ctx context.Context, src RangeSource) (RangeSourceSummary, error) {
	ctx = rangeSourceContext(ctx)
	if src == nil {
		return RangeSourceSummary{}, nil
	}
	if indexed, unlock, ok, err := indexedRangeSources([]RangeSource{src}); ok {
		if unlock != nil {
			defer unlock()
		}
		if err != nil {
			return RangeSourceSummary{}, err
		}
		return buildRangeSourceSummaryIndexed(ctx, indexed[0], src)
	}

	var bitmap *rangePrefixBitmap
	compact := rangeCompactPrefixBuilder{}
	hasRanges := false
	hasher := sha256.New()
	var hashBuf [8]byte
	sparse := rangeSparsePrefixBuilder{}
	var bounds Range

	for r := range src.Iter() {
		if err := ctx.Err(); err != nil {
			return RangeSourceSummary{}, err
		}
		if !hasRanges {
			bounds.Lo = r.Lo
			hasRanges = true
		}
		bounds.Hi = r.Hi
		binary.BigEndian.PutUint32(hashBuf[0:4], r.Lo)
		binary.BigEndian.PutUint32(hashBuf[4:8], r.Hi)
		_, _ = hasher.Write(hashBuf[:])
		start := r.Lo >> rangeSummaryPrefixShift
		end := r.Hi >> rangeSummaryPrefixShift
		sparseStart := r.Lo >> rangeSummarySparsePrefixShift
		sparseEnd := r.Hi >> rangeSummarySparsePrefixShift
		if bitmap == nil && sparse.wouldOverflow(sparseStart, sparseEnd) && !compact.active {
			if !compact.addSparsePrefixes(&sparse) {
				bitmap = &rangePrefixBitmap{}
				bitmap.addSparsePrefixes(&sparse)
			}
		}
		if bitmap != nil {
			bitmap.addRange(start, end)
		} else if compact.active {
			if !compact.addRange(start, end) {
				bitmap = &rangePrefixBitmap{}
				compact.addToBitmap(bitmap)
				bitmap.addRange(start, end)
			}
		}
		sparse.addRange(sparseStart, sparseEnd)
	}
	if err := RangeSourceErr(src); err != nil {
		return RangeSourceSummary{}, err
	}
	if err := ctx.Err(); err != nil {
		return RangeSourceSummary{}, err
	}
	if !hasRanges {
		return RangeSourceSummary{
			filter: RangeOverlapFilter{valid: true},
		}, nil
	}

	sum := hasher.Sum(nil)
	var contentHash RangeContentHash
	copy(contentHash.Sum[:], sum)
	contentHash.Valid = true
	filter := RangeOverlapFilter{
		valid:        true,
		hasRange:     true,
		bounds:       bounds,
		prefixBitmap: bitmap,
		compact:      compact.set(bitmap == nil),
		sparsePrefix: sparse.set(),
	}
	return RangeSourceSummary{
		Bounds:      bounds,
		HasRange:    true,
		ContentHash: contentHash,
		filter:      filter,
	}, nil
}

// BuildRangeOverlapFilterContext scans src once and returns only its
// conservative overlap filter.
func BuildRangeOverlapFilterContext(ctx context.Context, src RangeSource) (RangeOverlapFilter, error) {
	ctx = rangeSourceContext(ctx)
	if src == nil {
		return RangeOverlapFilter{}, nil
	}
	if indexed, unlock, ok, err := indexedRangeSources([]RangeSource{src}); ok {
		if unlock != nil {
			defer unlock()
		}
		if err != nil {
			return RangeOverlapFilter{}, err
		}
		return buildRangeOverlapFilterIndexed(ctx, indexed[0], src)
	}

	var bitmap *rangePrefixBitmap
	compact := rangeCompactPrefixBuilder{}
	hasRanges := false
	sparse := rangeSparsePrefixBuilder{}
	var bounds Range

	for r := range src.Iter() {
		if err := ctx.Err(); err != nil {
			return RangeOverlapFilter{}, err
		}
		if !hasRanges {
			bounds.Lo = r.Lo
			hasRanges = true
		}
		bounds.Hi = r.Hi
		start := r.Lo >> rangeSummaryPrefixShift
		end := r.Hi >> rangeSummaryPrefixShift
		sparseStart := r.Lo >> rangeSummarySparsePrefixShift
		sparseEnd := r.Hi >> rangeSummarySparsePrefixShift
		if bitmap == nil && sparse.wouldOverflow(sparseStart, sparseEnd) && !compact.active {
			if !compact.addSparsePrefixes(&sparse) {
				bitmap = &rangePrefixBitmap{}
				bitmap.addSparsePrefixes(&sparse)
			}
		}
		if bitmap != nil {
			bitmap.addRange(start, end)
		} else if compact.active {
			if !compact.addRange(start, end) {
				bitmap = &rangePrefixBitmap{}
				compact.addToBitmap(bitmap)
				bitmap.addRange(start, end)
			}
		}
		sparse.addRange(sparseStart, sparseEnd)
	}
	if err := RangeSourceErr(src); err != nil {
		return RangeOverlapFilter{}, err
	}
	if err := ctx.Err(); err != nil {
		return RangeOverlapFilter{}, err
	}
	if !hasRanges {
		return RangeOverlapFilter{valid: true}, nil
	}
	return RangeOverlapFilter{
		valid:        true,
		hasRange:     true,
		bounds:       bounds,
		prefixBitmap: bitmap,
		compact:      compact.set(bitmap == nil),
		sparsePrefix: sparse.set(),
	}, nil
}

func buildRangeSourceSummaryIndexed(ctx context.Context, src indexedRangeSource, original RangeSource) (RangeSourceSummary, error) {
	if err := ctx.Err(); err != nil {
		return RangeSourceSummary{}, err
	}

	var bitmap *rangePrefixBitmap
	compact := rangeCompactPrefixBuilder{}
	hasRanges := false
	hasher := sha256.New()
	var hashBuf [8]byte
	sparse := rangeSparsePrefixBuilder{}
	var bounds Range

	for i := 0; i < src.len(); i++ {
		if i&(materializeContextCheckEvery-1) == 0 {
			if err := ctx.Err(); err != nil {
				return RangeSourceSummary{}, err
			}
		}
		r, err := src.at(i)
		if err != nil {
			return RangeSourceSummary{}, err
		}
		if !hasRanges {
			bounds.Lo = r.Lo
			hasRanges = true
		}
		bounds.Hi = r.Hi
		binary.BigEndian.PutUint32(hashBuf[0:4], r.Lo)
		binary.BigEndian.PutUint32(hashBuf[4:8], r.Hi)
		_, _ = hasher.Write(hashBuf[:])
		start := r.Lo >> rangeSummaryPrefixShift
		end := r.Hi >> rangeSummaryPrefixShift
		sparseStart := r.Lo >> rangeSummarySparsePrefixShift
		sparseEnd := r.Hi >> rangeSummarySparsePrefixShift
		if bitmap == nil && sparse.wouldOverflow(sparseStart, sparseEnd) && !compact.active {
			if !compact.addSparsePrefixes(&sparse) {
				bitmap = &rangePrefixBitmap{}
				bitmap.addSparsePrefixes(&sparse)
			}
		}
		if bitmap != nil {
			bitmap.addRange(start, end)
		} else if compact.active {
			if !compact.addRange(start, end) {
				bitmap = &rangePrefixBitmap{}
				compact.addToBitmap(bitmap)
				bitmap.addRange(start, end)
			}
		}
		sparse.addRange(sparseStart, sparseEnd)
	}
	if err := RangeSourceErr(original); err != nil {
		return RangeSourceSummary{}, err
	}
	if err := ctx.Err(); err != nil {
		return RangeSourceSummary{}, err
	}
	if !hasRanges {
		return RangeSourceSummary{
			filter: RangeOverlapFilter{valid: true},
		}, nil
	}

	sum := hasher.Sum(nil)
	var contentHash RangeContentHash
	copy(contentHash.Sum[:], sum)
	contentHash.Valid = true
	filter := RangeOverlapFilter{
		valid:        true,
		hasRange:     true,
		bounds:       bounds,
		prefixBitmap: bitmap,
		compact:      compact.set(bitmap == nil),
		sparsePrefix: sparse.set(),
	}
	return RangeSourceSummary{
		Bounds:      bounds,
		HasRange:    true,
		ContentHash: contentHash,
		filter:      filter,
	}, nil
}

func buildRangeOverlapFilterIndexed(ctx context.Context, src indexedRangeSource, original RangeSource) (RangeOverlapFilter, error) {
	if err := ctx.Err(); err != nil {
		return RangeOverlapFilter{}, err
	}

	var bitmap *rangePrefixBitmap
	compact := rangeCompactPrefixBuilder{}
	hasRanges := false
	sparse := rangeSparsePrefixBuilder{}
	var bounds Range

	for i := 0; i < src.len(); i++ {
		if i&(materializeContextCheckEvery-1) == 0 {
			if err := ctx.Err(); err != nil {
				return RangeOverlapFilter{}, err
			}
		}
		r, err := src.at(i)
		if err != nil {
			return RangeOverlapFilter{}, err
		}
		if !hasRanges {
			bounds.Lo = r.Lo
			hasRanges = true
		}
		bounds.Hi = r.Hi
		start := r.Lo >> rangeSummaryPrefixShift
		end := r.Hi >> rangeSummaryPrefixShift
		sparseStart := r.Lo >> rangeSummarySparsePrefixShift
		sparseEnd := r.Hi >> rangeSummarySparsePrefixShift
		if bitmap == nil && sparse.wouldOverflow(sparseStart, sparseEnd) && !compact.active {
			if !compact.addSparsePrefixes(&sparse) {
				bitmap = &rangePrefixBitmap{}
				bitmap.addSparsePrefixes(&sparse)
			}
		}
		if bitmap != nil {
			bitmap.addRange(start, end)
		} else if compact.active {
			if !compact.addRange(start, end) {
				bitmap = &rangePrefixBitmap{}
				compact.addToBitmap(bitmap)
				bitmap.addRange(start, end)
			}
		}
		sparse.addRange(sparseStart, sparseEnd)
	}
	if err := RangeSourceErr(original); err != nil {
		return RangeOverlapFilter{}, err
	}
	if err := ctx.Err(); err != nil {
		return RangeOverlapFilter{}, err
	}
	if !hasRanges {
		return RangeOverlapFilter{valid: true}, nil
	}
	return RangeOverlapFilter{
		valid:        true,
		hasRange:     true,
		bounds:       bounds,
		prefixBitmap: bitmap,
		compact:      compact.set(bitmap == nil),
		sparsePrefix: sparse.set(),
	}, nil
}

// RangeSourcesEqualContext compares two normalized range streams exactly.
func RangeSourcesEqualContext(ctx context.Context, left, right RangeSource) (bool, error) {
	ctx = rangeSourceContext(ctx)
	if left == nil || right == nil {
		return left == nil && right == nil, nil
	}
	if l, ok := rangeSourceKnownUniqueIPs(left); ok {
		if r, ok := rangeSourceKnownUniqueIPs(right); ok && l != r {
			return false, nil
		}
	}
	if indexed, unlock, ok, err := indexedRangeSources([]RangeSource{left, right}); ok {
		if unlock != nil {
			defer unlock()
		}
		if err != nil {
			return false, err
		}
		return rangeSourcesEqualIndexed(ctx, indexed[0], indexed[1], left, right)
	}

	nextLeft, stopLeft := iter.Pull(left.Iter())
	defer stopLeft()
	nextRight, stopRight := iter.Pull(right.Iter())
	defer stopRight()

	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		lr, lok := nextLeft()
		rr, rok := nextRight()
		if lok != rok {
			return false, firstRangeSourceErr(left, right)
		}
		if !lok {
			return true, firstRangeSourceErr(left, right)
		}
		if lr != rr {
			return false, firstRangeSourceErr(left, right)
		}
	}
}

func rangeSourcesEqualIndexed(ctx context.Context, left, right indexedRangeSource, originalLeft, originalRight RangeSource) (bool, error) {
	if left.len() != right.len() {
		return false, firstRangeSourceErr(originalLeft, originalRight)
	}
	if leftUnique, ok := left.uniqueCount(); ok {
		if rightUnique, ok := right.uniqueCount(); ok && leftUnique != rightUnique {
			return false, firstRangeSourceErr(originalLeft, originalRight)
		}
	}
	for i := 0; i < left.len(); i++ {
		if i&(materializeContextCheckEvery-1) == 0 {
			if err := ctx.Err(); err != nil {
				return false, err
			}
		}
		leftRange, err := left.at(i)
		if err != nil {
			return false, err
		}
		rightRange, err := right.at(i)
		if err != nil {
			return false, err
		}
		if leftRange != rightRange {
			return false, firstRangeSourceErr(originalLeft, originalRight)
		}
	}
	if err := firstRangeSourceErr(originalLeft, originalRight); err != nil {
		return false, err
	}
	return true, ctx.Err()
}

func rangeSourceKnownUniqueIPs(src RangeSource) (uint64, bool) {
	if src == nil {
		return 0, true
	}
	if counter, ok := src.(interface{ UniqueIPs() uint64 }); ok {
		return counter.UniqueIPs(), true
	}
	if counter, ok := src.(interface{ UniqueCount() uint64 }); ok {
		return counter.UniqueCount(), true
	}
	return 0, false
}

func rangeSourceContext(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
}

func firstRangeSourceErr(sources ...RangeSource) error {
	for _, src := range sources {
		if err := RangeSourceErr(src); err != nil {
			return err
		}
	}
	return nil
}
