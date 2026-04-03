package iprange

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
)

type preadFileSet6 struct {
	f            *os.File
	records      int
	uniqueIPs_   Uint128
	rangesOffset int64
	closed       atomic.Bool
	rwmu         sync.RWMutex
	mu           sync.Mutex
	lastErr      error
}

func openFileSet6Pread(f *os.File, path string, fileSize int64, hdr binaryHeader6) (FileSet6, error) {
	if fileSize == 0 || hdr.records == 0 {
		_ = f.Close()
		return &emptyFileSet6{}, nil
	}

	if err := validateEndiannessAt(f, hdr.dataOffset); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	rangesOffset := hdr.dataOffset + 4
	preadRead := func(i int) (Range6, error) {
		return readRange6At(f, rangesOffset, i)
	}
	if err := validateSortedRanges6(hdr.records, preadRead); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("%s: payload validation: %w", path, err)
	}

	return &preadFileSet6{
		f:            f,
		records:      hdr.records,
		uniqueIPs_:   hdr.uniqueIPs,
		rangesOffset: rangesOffset,
	}, nil
}

func (p *preadFileSet6) Len() int { return p.records }

func (p *preadFileSet6) Range(i int) (Range6, error) {
	if p.closed.Load() {
		return Range6{}, ErrFileSet6Closed
	}
	p.rwmu.RLock()
	defer p.rwmu.RUnlock()
	if p.closed.Load() {
		return Range6{}, ErrFileSet6Closed
	}
	if i < 0 || i >= p.records {
		return Range6{}, fmt.Errorf("index %d out of range [0, %d)", i, p.records)
	}
	r, err := readRange6At(p.f, p.rangesOffset, i)
	if err != nil {
		p.setErr(err)
		return Range6{}, err
	}
	return r, nil
}

func (p *preadFileSet6) Contains(ip Uint128) bool {
	if p.closed.Load() {
		return false
	}
	p.rwmu.RLock()
	defer p.rwmu.RUnlock()
	if p.closed.Load() {
		return false
	}
	return fileSetContains6(ip, p.records, p.rangeAt)
}

func (p *preadFileSet6) rangeAt(i int) (Range6, error) {
	if i < 0 || i >= p.records {
		return Range6{}, fmt.Errorf("index %d out of range [0, %d)", i, p.records)
	}
	r, err := readRange6At(p.f, p.rangesOffset, i)
	if err != nil {
		p.setErr(err)
		return Range6{}, err
	}
	return r, nil
}

func (p *preadFileSet6) UniqueIPs() Uint128 { return p.uniqueIPs_ }

func (p *preadFileSet6) Iter() func(yield func(Range6) bool) {
	return func(yield func(Range6) bool) {
		if p.closed.Load() {
			return
		}
		p.rwmu.RLock()
		defer p.rwmu.RUnlock()
		if p.closed.Load() {
			return
		}
		for i := 0; i < p.records; i++ {
			r, err := readRange6At(p.f, p.rangesOffset, i)
			if err != nil {
				p.setErr(err)
				return
			}
			if !yield(r) {
				return
			}
		}
	}
}

func (p *preadFileSet6) Err() error {
	if p.closed.Load() {
		return ErrFileSet6Closed
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastErr
}

func (p *preadFileSet6) Close() error {
	if p.closed.Swap(true) {
		return nil
	}
	p.rwmu.Lock()
	defer p.rwmu.Unlock()
	if p.f == nil {
		return nil
	}
	err := p.f.Close()
	p.f = nil
	return err
}

func (p *preadFileSet6) setErr(err error) {
	p.mu.Lock()
	p.lastErr = err
	p.mu.Unlock()
}
