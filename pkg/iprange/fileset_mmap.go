//go:build linux || darwin

package iprange

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"golang.org/x/sys/unix"
)

// mmapFileSet reads ranges directly from an mmap'd region of the .set file.
// The entire file is mapped read-only; individual Range values are decoded on
// demand without copying the full array into Go heap memory.
type mmapFileSet struct {
	data       []byte // mmap'd region covering the whole file
	records    int
	uniqueIPs_ uint64
	// rangesData is a sub-slice of data pointing to the first Range byte
	// (past the endianness marker). Length == records * 8.
	rangesData []byte
	closed     atomic.Bool
	mu         sync.RWMutex // protects against concurrent Close/read races
}

// openFileSetPlatform tries mmap first; falls back to pread on mmap
// syscall failure only. Data validation errors are not retried because
// the data is corrupt regardless of the I/O backend.
func openFileSetPlatform(f *os.File, path string, fileSize int64, hdr binaryHeader) (FileSet, error) {
	if fileSize == 0 || hdr.records == 0 {
		_ = f.Close()
		return &emptyFileSet{}, nil
	}

	fs, err := openFileSetMmap(f, path, fileSize, hdr)
	if err != nil {
		// Only fall back to pread when the mmap syscall itself failed
		// (fd still open). Validation errors mean corrupt data.
		var mmapErr *errMmapSyscall
		if errors.As(err, &mmapErr) {
			return openFileSetPread(f, path, fileSize, hdr)
		}
		return nil, err
	}
	return fs, nil
}

// errMmapSyscall wraps mmap system call errors to distinguish them from
// data validation errors. Only mmap syscall failures trigger the pread
// fallback — validation errors mean the data is corrupt regardless of backend.
type errMmapSyscall struct{ err error }

func (e *errMmapSyscall) Error() string { return e.err.Error() }
func (e *errMmapSyscall) Unwrap() error { return e.err }

// openFileSetMmap maps the entire file read-only. On mmap syscall failure
// the fd is left open for pread fallback. On data validation failure the
// fd is closed (data is bad regardless of backend).
func openFileSetMmap(f *os.File, path string, fileSize int64, hdr binaryHeader) (FileSet, error) {
	data, err := unix.Mmap(int(f.Fd()), 0, int(fileSize), unix.PROT_READ, unix.MAP_PRIVATE)
	if err != nil {
		// Mmap syscall failed — fd still open, caller can fall back to pread.
		return nil, &errMmapSyscall{err: fmt.Errorf("%s: mmap: %w", path, err)}
	}
	// File descriptor is no longer needed after successful mmap.
	_ = f.Close()

	// Advise the kernel we'll access the range data randomly (binary search).
	rangeStart := int(hdr.dataOffset) + 4 // skip endianness marker
	_ = unix.Madvise(data[rangeStart:], unix.MADV_RANDOM)

	// Validate endianness marker from the mapped data.
	markerSlice := data[hdr.dataOffset : hdr.dataOffset+4]
	if err := validateEndianness(markerSlice); err != nil {
		_ = unix.Munmap(data)
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	rangesData := data[rangeStart : rangeStart+hdr.records*8]

	// Validate that ranges are sorted and non-overlapping. For mmap this
	// is just pointer arithmetic over the mapped region — no extra I/O.
	mmapRead := func(i int) (Range, error) {
		off := i * 8
		return decodeRange(rangesData[off : off+8]), nil
	}
	if err := validateSortedRanges(hdr.records, mmapRead); err != nil {
		_ = unix.Munmap(data)
		return nil, fmt.Errorf("%s: payload validation: %w", path, err)
	}

	return &mmapFileSet{
		data:       data,
		records:    hdr.records,
		uniqueIPs_: hdr.uniqueIPs,
		rangesData: rangesData,
	}, nil
}

func (m *mmapFileSet) Len() int { return m.records }

func (m *mmapFileSet) Range(i int) (Range, error) {
	if m.closed.Load() {
		return Range{}, ErrFileSetClosed
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed.Load() {
		return Range{}, ErrFileSetClosed
	}
	if i < 0 || i >= m.records {
		return Range{}, fmt.Errorf("index %d out of range [0, %d)", i, m.records)
	}
	off := i * 8
	return decodeRange(m.rangesData[off : off+8]), nil
}

func (m *mmapFileSet) Contains(ip uint32) bool {
	if m.closed.Load() {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed.Load() {
		return false
	}
	return fileSetContains(ip, m.records, m.rangeAt)
}

// rangeAt reads the i-th range without acquiring the lock (caller holds it).
func (m *mmapFileSet) rangeAt(i int) (Range, error) {
	if i < 0 || i >= m.records {
		return Range{}, fmt.Errorf("index %d out of range [0, %d)", i, m.records)
	}
	off := i * 8
	return decodeRange(m.rangesData[off : off+8]), nil
}

func (m *mmapFileSet) UniqueIPs() uint64 {
	return m.uniqueIPs_
}

func (m *mmapFileSet) Iter() func(yield func(Range) bool) {
	return func(yield func(Range) bool) {
		if m.closed.Load() {
			return
		}
		m.mu.RLock()
		defer m.mu.RUnlock()
		if m.closed.Load() {
			return
		}
		for i := 0; i < m.records; i++ {
			off := i * 8
			r := decodeRange(m.rangesData[off : off+8])
			if !yield(r) {
				return
			}
		}
	}
}

func (m *mmapFileSet) Err() error {
	if m.closed.Load() {
		return ErrFileSetClosed
	}
	// mmap backend: data is memory-resident after open, no I/O errors possible.
	return nil
}

func (m *mmapFileSet) lockFastReader() error {
	if m.closed.Load() {
		return ErrFileSetClosed
	}
	m.mu.RLock()
	if m.closed.Load() {
		m.mu.RUnlock()
		return ErrFileSetClosed
	}
	return nil
}

func (m *mmapFileSet) Close() error {
	if m.closed.Swap(true) {
		return nil // already closed
	}
	// Acquire write lock to ensure all in-flight reads complete before
	// unmapping the memory region.
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data == nil {
		return nil
	}
	return unix.Munmap(m.data)
}
