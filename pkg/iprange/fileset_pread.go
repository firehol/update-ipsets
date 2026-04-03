package iprange

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
)

// preadFileSet reads ranges from a .set file using pread (ReadAt) syscalls.
// This is the portable fallback used on all platforms. On Linux/macOS it
// serves as the fallback when mmap fails.
type preadFileSet struct {
	f            *os.File
	records      int
	uniqueIPs_   uint64
	rangesOffset int64 // file offset of first Range byte (past endianness marker)
	closed       atomic.Bool
	rwmu         sync.RWMutex // protects against concurrent Close/read races
	mu           sync.Mutex   // protects lastErr
	lastErr      error
}

// openFileSetPread opens a pread-backed FileSet. Available on all platforms.
func openFileSetPread(f *os.File, path string, fileSize int64, hdr binaryHeader) (FileSet, error) {
	if fileSize == 0 || hdr.records == 0 {
		_ = f.Close()
		return &emptyFileSet{}, nil
	}

	// Validate endianness marker via pread.
	if err := validateEndiannessAt(f, hdr.dataOffset); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	rangesOffset := hdr.dataOffset + 4 // skip the 4-byte marker

	// Validate that ranges are sorted and non-overlapping.
	preadRead := func(i int) (Range, error) {
		return readRangeAt(f, rangesOffset, i)
	}
	if err := validateSortedRanges(hdr.records, preadRead); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("%s: payload validation: %w", path, err)
	}

	return &preadFileSet{
		f:            f,
		records:      hdr.records,
		uniqueIPs_:   hdr.uniqueIPs,
		rangesOffset: rangesOffset,
	}, nil
}

func (p *preadFileSet) Len() int { return p.records }

func (p *preadFileSet) Range(i int) (Range, error) {
	if p.closed.Load() {
		return Range{}, ErrFileSetClosed
	}
	p.rwmu.RLock()
	defer p.rwmu.RUnlock()
	if p.closed.Load() {
		return Range{}, ErrFileSetClosed
	}
	if i < 0 || i >= p.records {
		return Range{}, fmt.Errorf("index %d out of range [0, %d)", i, p.records)
	}
	r, err := readRangeAt(p.f, p.rangesOffset, i)
	if err != nil {
		p.setErr(err)
		return Range{}, err
	}
	return r, nil
}

func (p *preadFileSet) Contains(ip uint32) bool {
	if p.closed.Load() {
		return false
	}
	p.rwmu.RLock()
	defer p.rwmu.RUnlock()
	if p.closed.Load() {
		return false
	}
	return fileSetContains(ip, p.records, p.rangeAt)
}

// rangeAt reads the i-th range without acquiring the lock (caller holds it).
func (p *preadFileSet) rangeAt(i int) (Range, error) {
	if i < 0 || i >= p.records {
		return Range{}, fmt.Errorf("index %d out of range [0, %d)", i, p.records)
	}
	r, err := readRangeAt(p.f, p.rangesOffset, i)
	if err != nil {
		p.setErr(err)
		return Range{}, err
	}
	return r, nil
}

func (p *preadFileSet) UniqueIPs() uint64 {
	return p.uniqueIPs_
}

func (p *preadFileSet) Iter() func(yield func(Range) bool) {
	return func(yield func(Range) bool) {
		if p.closed.Load() {
			return
		}
		p.rwmu.RLock()
		defer p.rwmu.RUnlock()
		if p.closed.Load() {
			return
		}
		for i := 0; i < p.records; i++ {
			r, err := readRangeAt(p.f, p.rangesOffset, i)
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

func (p *preadFileSet) Err() error {
	if p.closed.Load() {
		return ErrFileSetClosed
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastErr
}

func (p *preadFileSet) Close() error {
	if p.closed.Swap(true) {
		return nil // already closed
	}
	// Acquire write lock to ensure all in-flight reads complete before
	// closing the file descriptor.
	p.rwmu.Lock()
	defer p.rwmu.Unlock()
	if p.f == nil {
		return nil
	}
	err := p.f.Close()
	p.f = nil
	return err
}

func (p *preadFileSet) setErr(err error) {
	p.mu.Lock()
	p.lastErr = err
	p.mu.Unlock()
}
