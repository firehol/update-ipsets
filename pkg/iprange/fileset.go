package iprange

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"sync/atomic"
)

var (
	// ErrFileSetClosed is returned when operating on a closed FileSet.
	ErrFileSetClosed = errors.New("fileset is closed")

	// ErrNotOptimized is returned by OpenFileSet when the binary file's
	// header indicates non-optimized data. Binary search requires sorted,
	// non-overlapping ranges which only optimized sets guarantee.
	ErrNotOptimized = errors.New("binary file is not optimized; Contains requires sorted, non-overlapping ranges")
)

// FileSet provides read-only access to a binary .set file without loading the
// full range array into memory. Implementations back the data with either mmap
// or pread, selected automatically by OpenFileSet.
type FileSet interface {
	// Len returns the number of ranges in the set.
	Len() int

	// Range returns the i-th range. Returns an error if i is out of bounds
	// or if the FileSet has been closed.
	Range(i int) (Range, error)

	// Contains checks whether ip falls within any range, using binary search
	// directly on the file-backed data. Returns false if the FileSet is
	// closed or an I/O error occurs; callers that need to distinguish "not
	// found" from "error" should check Err() after the call.
	Contains(ip uint32) bool

	// UniqueIPs returns the total count of unique IPs (read from the header).
	UniqueIPs() uint64

	// Iter returns a Go 1.23-style push iterator over all ranges. If an I/O
	// error occurs during iteration, the iterator stops early; callers
	// should check Err() afterwards.
	Iter() func(yield func(Range) bool)

	// Err returns the last I/O error encountered by Contains or Iter, or
	// ErrFileSetClosed if the FileSet has been closed. Returns nil if no
	// error has occurred.
	Err() error

	// Close releases underlying resources (unmaps memory or closes the file).
	// After Close, all read operations return zero values / false and Err()
	// returns ErrFileSetClosed.
	Close() error
}

// binaryHeader holds metadata parsed from the text header of a .set file.
type binaryHeader struct {
	optimized  bool
	recordSize int
	records    int
	bytes      int
	lines      int
	uniqueIPs  uint64
	// dataOffset is the byte offset in the file where the endianness marker
	// starts (immediately after the last header line).
	dataOffset int64
}

// parseBinaryHeader reads the text header from r and returns structured
// metadata. The reader is left positioned right before the endianness marker.
func parseBinaryHeader(r io.Reader) (binaryHeader, error) {
	br := bufio.NewReader(r)
	var consumed int64

	// line 1: format identifier
	line, err := br.ReadString('\n')
	if err != nil {
		return binaryHeader{}, fmt.Errorf("reading header: %w", err)
	}
	consumed += int64(len(line))
	if line != BinaryHeaderV10 {
		return binaryHeader{}, fmt.Errorf("expecting binary header but found %q", strings.TrimSpace(line))
	}

	// line 2: optimized / non-optimized
	line, err = br.ReadString('\n')
	if err != nil {
		return binaryHeader{}, fmt.Errorf("reading optimization marker: %w", err)
	}
	consumed += int64(len(line))
	mode := strings.TrimSpace(line)
	if mode != "optimized" && mode != "non-optimized" {
		return binaryHeader{}, fmt.Errorf("invalid optimization marker %q", mode)
	}

	// line 3: record size N
	recordSize, n, err := readHeaderInt(br, "record size ")
	if err != nil {
		return binaryHeader{}, err
	}
	consumed += int64(n)
	if recordSize != 8 {
		return binaryHeader{}, fmt.Errorf("invalid record size %d", recordSize)
	}

	// line 4: records N
	records, n, err := readHeaderInt(br, "records ")
	if err != nil {
		return binaryHeader{}, err
	}
	consumed += int64(n)

	// line 5: bytes N
	payloadBytes, n, err := readHeaderInt(br, "bytes ")
	if err != nil {
		return binaryHeader{}, err
	}
	consumed += int64(n)

	// line 6: lines N
	lines, n, err := readHeaderInt(br, "lines ")
	if err != nil {
		return binaryHeader{}, err
	}
	consumed += int64(n)

	// line 7: unique ips N
	uniqueIPs, n, err := readHeaderUint64(br, "unique ips ")
	if err != nil {
		return binaryHeader{}, err
	}
	consumed += int64(n)

	// Validate consistency
	if payloadBytes != records*8+4 {
		return binaryHeader{}, fmt.Errorf("invalid payload size %d (expected %d)", payloadBytes, records*8+4)
	}
	if uniqueIPs < uint64(records) || lines < records {
		return binaryHeader{}, fmt.Errorf("inconsistent binary counters")
	}

	return binaryHeader{
		optimized:  mode == "optimized",
		recordSize: recordSize,
		records:    records,
		bytes:      payloadBytes,
		lines:      lines,
		uniqueIPs:  uniqueIPs,
		dataOffset: consumed,
	}, nil
}

// readHeaderInt reads a line of the form "<prefix><int>\n" and returns the
// integer value and the number of bytes consumed (including the newline).
func readHeaderInt(br *bufio.Reader, prefix string) (int, int, error) {
	line, err := br.ReadString('\n')
	if err != nil {
		return 0, 0, fmt.Errorf("reading %q line: %w", prefix, err)
	}
	if !strings.HasPrefix(line, prefix) {
		return 0, 0, fmt.Errorf("expected %q line, got %q", prefix, strings.TrimSpace(line))
	}
	val, err := parseInt(strings.TrimSpace(strings.TrimPrefix(line, prefix)))
	if err != nil {
		return 0, 0, fmt.Errorf("parsing %q value: %w", prefix, err)
	}
	return val, len(line), nil
}

// readHeaderUint64 reads a line of the form "<prefix><uint64>\n".
func readHeaderUint64(br *bufio.Reader, prefix string) (uint64, int, error) {
	line, err := br.ReadString('\n')
	if err != nil {
		return 0, 0, fmt.Errorf("reading %q line: %w", prefix, err)
	}
	if !strings.HasPrefix(line, prefix) {
		return 0, 0, fmt.Errorf("expected %q line, got %q", prefix, strings.TrimSpace(line))
	}
	val, err := parseUint64(strings.TrimSpace(strings.TrimPrefix(line, prefix)))
	if err != nil {
		return 0, 0, fmt.Errorf("parsing %q value: %w", prefix, err)
	}
	return val, len(line), nil
}

// parseInt is a minimal integer parser that avoids importing strconv in this
// file (strconv is already used by range.go). We reuse strconv via the
// existing helpers — but since this file needs to be self-contained for the
// shared header parser, we import it directly.
func parseInt(s string) (int, error) {
	// Simple decimal parser — header values are always small positive ints.
	if s == "" {
		return 0, fmt.Errorf("empty integer string")
	}
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("non-digit %q in integer", ch)
		}
		digit := int(ch - '0')
		if n > (math.MaxInt-digit)/10 {
			return 0, fmt.Errorf("integer overflow parsing %q", s)
		}
		n = n*10 + digit
	}
	return n, nil
}

func parseUint64(s string) (uint64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty uint64 string")
	}
	var n uint64
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("non-digit %q in uint64", ch)
		}
		digit := uint64(ch - '0')
		if n > (math.MaxUint64-digit)/10 {
			return 0, fmt.Errorf("uint64 overflow parsing %q", s)
		}
		n = n*10 + digit
	}
	return n, nil
}

// OpenFileSet opens a binary .set file for read-only, out-of-core access.
// On Linux and macOS it tries mmap for zero-copy access and falls back to
// pread if mmap fails. On other platforms it uses pread directly.
//
// The file must contain an optimized (sorted, non-overlapping) range set;
// non-optimized files are rejected with ErrNotOptimized.
//
// A zero-byte file is treated as an empty set (0 ranges, 0 unique IPs).
func OpenFileSet(path string) (FileSet, error) {
	f, err := os.Open(path) // nosemgrep: exported local fileset API; callers intentionally provide the binary set path.
	if err != nil {
		return nil, err
	}

	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	fileSize := fi.Size()

	// Empty file => empty set. WriteBinary writes nothing for empty sets,
	// so this is the canonical on-disk representation of an empty IPSet.
	if fileSize == 0 {
		_ = f.Close()
		return &emptyFileSet{}, nil
	}

	hdr, err := parseBinaryHeader(f)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	// Reject non-optimized data: binary search requires sorted, non-overlapping
	// ranges which only the optimized format guarantees.
	if !hdr.optimized {
		_ = f.Close()
		return nil, fmt.Errorf("%s: %w", path, ErrNotOptimized)
	}

	// The binary payload is: 4 bytes endianness marker + records*8 bytes.
	expectedSize := hdr.dataOffset + 4 + int64(hdr.records)*8
	if fileSize != expectedSize {
		_ = f.Close()
		return nil, fmt.Errorf("%s: file size %d does not match expected %d", path, fileSize, expectedSize)
	}

	fs, err := openFileSetPlatform(f, path, fileSize, hdr)
	if err != nil {
		return nil, err
	}
	return fs, nil
}

// emptyFileSet is returned for zero-byte (empty) .set files.
type emptyFileSet struct {
	closed atomic.Bool
}

func (e *emptyFileSet) Len() int { return 0 }
func (e *emptyFileSet) Range(int) (Range, error) {
	return Range{}, fmt.Errorf("index 0 out of range [0, 0)")
}
func (e *emptyFileSet) Contains(uint32) bool               { return false }
func (e *emptyFileSet) UniqueIPs() uint64                  { return 0 }
func (e *emptyFileSet) Iter() func(yield func(Range) bool) { return func(func(Range) bool) {} }

func (e *emptyFileSet) Err() error {
	if e.closed.Load() {
		return ErrFileSetClosed
	}
	return nil
}

func (e *emptyFileSet) Close() error {
	e.closed.Store(true)
	return nil
}

// fileSetContains implements binary search over file-backed range data.
// It is shared by both the mmap and pread backends via the rangeReader func.
// On I/O errors it returns false; the pread backend captures the error
// via setErr() so callers can check Err() to distinguish "not found" from
// "read failed".
func fileSetContains(ip uint32, n int, readRange func(int) (Range, error)) bool {
	ok, _ := fileSetContainsWithStats(ip, n, readRange)
	return ok
}

func fileSetContainsWithStats(ip uint32, n int, readRange func(int) (Range, error)) (bool, OperationStats) {
	stats := OperationStats{Lookups: 1, BinarySearches: 1}
	i := sort.Search(n, func(i int) bool {
		stats.Comparisons++
		r, err := readRange(i)
		if err != nil {
			return true // treat read errors as "past the target"
		}
		return r.Hi >= ip
	})
	if i >= n {
		return false, stats
	}
	r, err := readRange(i)
	if err != nil {
		return false, stats
	}
	stats.Comparisons++
	return r.Lo <= ip, stats
}

// validateEndianness reads the 4-byte endianness marker at the given offset
// and verifies it matches the host byte order.
func validateEndianness(data []byte) error {
	if len(data) < 4 {
		return fmt.Errorf("endianness marker too short")
	}
	marker := nativeEndian.Uint32(data[:4])
	if marker != binaryEndiannessMarker {
		return fmt.Errorf("incompatible endianness")
	}
	return nil
}

// decodeRange extracts a Range from an 8-byte slice in native byte order.
func decodeRange(buf []byte) Range {
	return Range{
		Lo: nativeEndian.Uint32(buf[0:4]),
		Hi: nativeEndian.Uint32(buf[4:8]),
	}
}

// readRangeAt reads a single Range from f at the given file offset of the
// range data area (after the endianness marker). Used by the pread backend.
func readRangeAt(f *os.File, rangesOffset int64, i int) (Range, error) {
	var buf [8]byte
	offset := rangesOffset + int64(i)*8
	n, err := f.ReadAt(buf[:], offset)
	if err != nil {
		return Range{}, fmt.Errorf("reading range %d: %w", i, err)
	}
	if n != 8 {
		return Range{}, fmt.Errorf("short read for range %d: got %d bytes", i, n)
	}
	return decodeRange(buf[:]), nil
}

// validateEndiannessAt reads and validates the endianness marker from a file.
func validateEndiannessAt(f *os.File, offset int64) error {
	var buf [4]byte
	n, err := f.ReadAt(buf[:], offset)
	if err != nil {
		return fmt.Errorf("reading endianness marker: %w", err)
	}
	if n != 4 {
		return fmt.Errorf("short read for endianness marker: got %d bytes", n)
	}
	marker := binary.NativeEndian.Uint32(buf[:])
	if marker != binaryEndiannessMarker {
		return fmt.Errorf("incompatible endianness")
	}
	return nil
}
