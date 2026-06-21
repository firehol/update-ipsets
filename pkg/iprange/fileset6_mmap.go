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

type mmapFileSet6 struct {
	data       []byte
	records    int
	uniqueIPs_ Uint128
	rangesData []byte
	closed     atomic.Bool
	mu         sync.RWMutex
}

func openFileSet6Platform(f *os.File, path string, fileSize int64, hdr binaryHeader6) (FileSet6, error) {
	if fileSize == 0 || hdr.records == 0 {
		_ = f.Close()
		return &emptyFileSet6{}, nil
	}

	fs, err := openFileSet6Mmap(f, path, fileSize, hdr)
	if err != nil {
		var mmapErr *errMmapSyscall
		if errors.As(err, &mmapErr) {
			return openFileSet6Pread(f, path, fileSize, hdr)
		}
		return nil, err
	}
	return fs, nil
}

func openFileSet6Mmap(f *os.File, path string, fileSize int64, hdr binaryHeader6) (FileSet6, error) {
	data, err := unix.Mmap(int(f.Fd()), 0, int(fileSize), unix.PROT_READ, unix.MAP_PRIVATE)
	if err != nil {
		return nil, &errMmapSyscall{err: fmt.Errorf("%s: mmap: %w", path, err)}
	}
	_ = f.Close()

	rangeStart := int(hdr.dataOffset) + 4
	_ = unix.Madvise(data[rangeStart:], unix.MADV_RANDOM)

	markerSlice := data[hdr.dataOffset : hdr.dataOffset+4]
	if err := validateEndianness(markerSlice); err != nil {
		_ = unix.Munmap(data)
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	rangesData := data[rangeStart : rangeStart+hdr.records*32]
	mmapRead := func(i int) (Range6, error) {
		off := i * 32
		return decodeRange6(rangesData[off : off+32]), nil
	}
	if err := validateSortedRanges6(hdr.records, mmapRead); err != nil {
		_ = unix.Munmap(data)
		return nil, fmt.Errorf("%s: payload validation: %w", path, err)
	}

	return &mmapFileSet6{
		data:       data,
		records:    hdr.records,
		uniqueIPs_: hdr.uniqueIPs,
		rangesData: rangesData,
	}, nil
}

func (m *mmapFileSet6) Len() int { return m.records }

func (m *mmapFileSet6) Range(i int) (Range6, error) {
	if m.closed.Load() {
		return Range6{}, ErrFileSet6Closed
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed.Load() {
		return Range6{}, ErrFileSet6Closed
	}
	if i < 0 || i >= m.records {
		return Range6{}, fmt.Errorf("index %d out of range [0, %d)", i, m.records)
	}
	off := i * 32
	return decodeRange6(m.rangesData[off : off+32]), nil
}

func (m *mmapFileSet6) Contains(ip Uint128) bool {
	if m.closed.Load() {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed.Load() {
		return false
	}
	return fileSetContains6(ip, m.records, m.rangeAt)
}

func (m *mmapFileSet6) rangeAt(i int) (Range6, error) {
	if i < 0 || i >= m.records {
		return Range6{}, fmt.Errorf("index %d out of range [0, %d)", i, m.records)
	}
	off := i * 32
	return decodeRange6(m.rangesData[off : off+32]), nil
}

func (m *mmapFileSet6) UniqueIPs() Uint128 { return m.uniqueIPs_ }

func (m *mmapFileSet6) Iter() func(yield func(Range6) bool) {
	return func(yield func(Range6) bool) {
		if m.closed.Load() {
			return
		}
		m.mu.RLock()
		defer m.mu.RUnlock()
		if m.closed.Load() {
			return
		}
		for i := 0; i < m.records; i++ {
			off := i * 32
			r := decodeRange6(m.rangesData[off : off+32])
			if !yield(r) {
				return
			}
		}
	}
}

func (m *mmapFileSet6) Err() error {
	if m.closed.Load() {
		return ErrFileSet6Closed
	}
	return nil
}

func (m *mmapFileSet6) lockFastReader() error {
	if m.closed.Load() {
		return ErrFileSet6Closed
	}
	m.mu.RLock()
	if m.closed.Load() {
		m.mu.RUnlock()
		return ErrFileSet6Closed
	}
	return nil
}

func (m *mmapFileSet6) Close() error {
	if m.closed.Swap(true) {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data == nil {
		return nil
	}
	return unix.Munmap(m.data)
}
