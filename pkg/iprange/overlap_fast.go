package iprange

import "context"

const overlapContextCheckEvery = 4096

func overlapCountFastPath(ctx context.Context, a, b RangeSource) (uint64, bool, error) {
	if left, ok := a.(*IPSet); ok {
		left.Optimize()
		if right, ok := b.(*IPSet); ok {
			right.Optimize()
			count, err := overlapCountRangesContext(ctx, left.Ranges, right.Ranges)
			return count, true, err
		}
	}

	return overlapCountFastPathPlatform(ctx, a, b)
}

func overlapCountRangesContext(ctx context.Context, a, b []Range) (uint64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	var count uint64
	i, j := 0, 0
	steps := 0
	for i < len(a) && j < len(b) {
		steps++
		if steps&(overlapContextCheckEvery-1) == 0 {
			if err := ctx.Err(); err != nil {
				return count, err
			}
		}

		ra := a[i]
		rb := b[j]
		if ra.Hi < rb.Lo {
			i++
			continue
		}
		if rb.Hi < ra.Lo {
			j++
			continue
		}

		lo := ra.Lo
		if rb.Lo > lo {
			lo = rb.Lo
		}
		hi := ra.Hi
		if rb.Hi < hi {
			hi = rb.Hi
		}
		count += uint64(hi) - uint64(lo) + 1

		switch {
		case ra.Hi < rb.Hi:
			i++
		case rb.Hi < ra.Hi:
			j++
		default:
			i++
			j++
		}
	}

	return count, ctx.Err()
}

func overlapCountRangeBytesContext(ctx context.Context, a, b []byte) (uint64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	var count uint64
	i, j := 0, 0
	n, m := len(a)/8, len(b)/8
	steps := 0
	for i < n && j < m {
		steps++
		if steps&(overlapContextCheckEvery-1) == 0 {
			if err := ctx.Err(); err != nil {
				return count, err
			}
		}

		ra := decodeRangeAt(a, i)
		rb := decodeRangeAt(b, j)
		if ra.Hi < rb.Lo {
			i++
			continue
		}
		if rb.Hi < ra.Lo {
			j++
			continue
		}

		lo := ra.Lo
		if rb.Lo > lo {
			lo = rb.Lo
		}
		hi := ra.Hi
		if rb.Hi < hi {
			hi = rb.Hi
		}
		count += uint64(hi) - uint64(lo) + 1

		switch {
		case ra.Hi < rb.Hi:
			i++
		case rb.Hi < ra.Hi:
			j++
		default:
			i++
			j++
		}
	}

	return count, ctx.Err()
}

func overlapCountRangesAndBytesContext(ctx context.Context, a []Range, b []byte) (uint64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	var count uint64
	i, j := 0, 0
	m := len(b) / 8
	steps := 0
	for i < len(a) && j < m {
		steps++
		if steps&(overlapContextCheckEvery-1) == 0 {
			if err := ctx.Err(); err != nil {
				return count, err
			}
		}

		ra := a[i]
		rb := decodeRangeAt(b, j)
		if ra.Hi < rb.Lo {
			i++
			continue
		}
		if rb.Hi < ra.Lo {
			j++
			continue
		}

		lo := ra.Lo
		if rb.Lo > lo {
			lo = rb.Lo
		}
		hi := ra.Hi
		if rb.Hi < hi {
			hi = rb.Hi
		}
		count += uint64(hi) - uint64(lo) + 1

		switch {
		case ra.Hi < rb.Hi:
			i++
		case rb.Hi < ra.Hi:
			j++
		default:
			i++
			j++
		}
	}

	return count, ctx.Err()
}

func overlapCountBytesAndRangesContext(ctx context.Context, a []byte, b []Range) (uint64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	var count uint64
	i, j := 0, 0
	n := len(a) / 8
	steps := 0
	for i < n && j < len(b) {
		steps++
		if steps&(overlapContextCheckEvery-1) == 0 {
			if err := ctx.Err(); err != nil {
				return count, err
			}
		}

		ra := decodeRangeAt(a, i)
		rb := b[j]
		if ra.Hi < rb.Lo {
			i++
			continue
		}
		if rb.Hi < ra.Lo {
			j++
			continue
		}

		lo := ra.Lo
		if rb.Lo > lo {
			lo = rb.Lo
		}
		hi := ra.Hi
		if rb.Hi < hi {
			hi = rb.Hi
		}
		count += uint64(hi) - uint64(lo) + 1

		switch {
		case ra.Hi < rb.Hi:
			i++
		case rb.Hi < ra.Hi:
			j++
		default:
			i++
			j++
		}
	}

	return count, ctx.Err()
}

func decodeRangeAt(data []byte, i int) Range {
	off := i * 8
	return Range{
		Lo: nativeEndian.Uint32(data[off : off+4]),
		Hi: nativeEndian.Uint32(data[off+4 : off+8]),
	}
}
