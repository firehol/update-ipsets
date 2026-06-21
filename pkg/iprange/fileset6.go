package iprange

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync/atomic"
)

var ErrFileSet6Closed = errors.New("fileset6 is closed")

type FileSet6 interface {
	Len() int
	Range(i int) (Range6, error)
	Contains(ip Uint128) bool
	UniqueIPs() Uint128
	Iter() func(yield func(Range6) bool)
	Err() error
	Close() error
}

type binaryHeader6 struct {
	optimized  bool
	recordSize int
	records    int
	bytes      int
	lines      int
	uniqueIPs  Uint128
	dataOffset int64
}

func parseBinaryHeader6(r io.Reader) (binaryHeader6, error) {
	br := bufio.NewReader(r)
	var consumed int64

	line, err := br.ReadString('\n')
	if err != nil {
		return binaryHeader6{}, fmt.Errorf("reading header: %w", err)
	}
	consumed += int64(len(line))
	if line != BinaryHeaderV20IPv6 {
		return binaryHeader6{}, fmt.Errorf("expecting binary v2 header but found %q", strings.TrimSpace(line))
	}

	family, err := br.ReadString('\n')
	if err != nil {
		return binaryHeader6{}, fmt.Errorf("reading family: %w", err)
	}
	consumed += int64(len(family))
	if strings.TrimSpace(family) != "ipv6" {
		return binaryHeader6{}, fmt.Errorf("expected family 'ipv6' but found %q", strings.TrimSpace(family))
	}

	mode, err := br.ReadString('\n')
	if err != nil {
		return binaryHeader6{}, fmt.Errorf("reading optimization marker: %w", err)
	}
	consumed += int64(len(mode))
	modeStr := strings.TrimSpace(mode)
	if modeStr != "optimized" && modeStr != "non-optimized" {
		return binaryHeader6{}, fmt.Errorf("invalid optimization marker %q", modeStr)
	}

	recordSize, n, err := readHeaderInt(br, "record size ")
	if err != nil {
		return binaryHeader6{}, err
	}
	consumed += int64(n)
	if recordSize != 32 {
		return binaryHeader6{}, fmt.Errorf("invalid record size %d (expected 32)", recordSize)
	}

	records, n, err := readHeaderInt(br, "records ")
	if err != nil {
		return binaryHeader6{}, err
	}
	consumed += int64(n)

	payloadBytes, n, err := readHeaderInt(br, "bytes ")
	if err != nil {
		return binaryHeader6{}, err
	}
	consumed += int64(n)

	lines, n, err := readHeaderInt(br, "lines ")
	if err != nil {
		return binaryHeader6{}, err
	}
	consumed += int64(n)

	uniqueLine, err := br.ReadString('\n')
	if err != nil {
		return binaryHeader6{}, fmt.Errorf("reading unique ips: %w", err)
	}
	consumed += int64(len(uniqueLine))
	if !strings.HasPrefix(uniqueLine, "unique ips ") {
		return binaryHeader6{}, fmt.Errorf("expected 'unique ips' line, got %q", strings.TrimSpace(uniqueLine))
	}
	uniqueIPs, err := parseUint128(strings.TrimSpace(strings.TrimPrefix(uniqueLine, "unique ips ")))
	if err != nil {
		return binaryHeader6{}, fmt.Errorf("parsing unique ips: %w", err)
	}

	if payloadBytes != records*32+4 {
		return binaryHeader6{}, fmt.Errorf("invalid payload size %d (expected %d)", payloadBytes, records*32+4)
	}

	return binaryHeader6{
		optimized:  modeStr == "optimized",
		recordSize: recordSize,
		records:    records,
		bytes:      payloadBytes,
		lines:      lines,
		uniqueIPs:  uniqueIPs,
		dataOffset: consumed,
	}, nil
}

func parseBinaryHeader6File(f *os.File) (binaryHeader6, error) {
	var buf [1024]byte
	n, err := f.ReadAt(buf[:], 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return binaryHeader6{}, fmt.Errorf("reading header: %w", err)
	}
	return parseBinaryHeader6Bytes(buf[:n])
}

func parseBinaryHeader6Bytes(data []byte) (binaryHeader6, error) {
	p := headerByteParser{data: data}

	line, err := p.nextLine("header")
	if err != nil {
		return binaryHeader6{}, err
	}
	if !headerLineEqual(line, strings.TrimSuffix(BinaryHeaderV20IPv6, "\n")) {
		return binaryHeader6{}, fmt.Errorf("expecting binary v2 header but found %q", string(line))
	}

	family, err := p.nextLine("family")
	if err != nil {
		return binaryHeader6{}, err
	}
	if !headerLineEqual(family, "ipv6") {
		return binaryHeader6{}, fmt.Errorf("expected family 'ipv6' but found %q", string(family))
	}

	mode, err := p.nextLine("optimization marker")
	if err != nil {
		return binaryHeader6{}, err
	}
	if !headerLineEqual(mode, "optimized") && !headerLineEqual(mode, "non-optimized") {
		return binaryHeader6{}, fmt.Errorf("invalid optimization marker %q", string(mode))
	}
	optimized := headerLineEqual(mode, "optimized")

	recordSize, err := readHeaderIntBytes(&p, "record size ")
	if err != nil {
		return binaryHeader6{}, err
	}
	if recordSize != 32 {
		return binaryHeader6{}, fmt.Errorf("invalid record size %d (expected 32)", recordSize)
	}

	records, err := readHeaderIntBytes(&p, "records ")
	if err != nil {
		return binaryHeader6{}, err
	}
	payloadBytes, err := readHeaderIntBytes(&p, "bytes ")
	if err != nil {
		return binaryHeader6{}, err
	}
	lines, err := readHeaderIntBytes(&p, "lines ")
	if err != nil {
		return binaryHeader6{}, err
	}

	uniqueLine, err := p.nextLine("unique ips")
	if err != nil {
		return binaryHeader6{}, err
	}
	if !headerLineHasPrefix(uniqueLine, "unique ips ") {
		return binaryHeader6{}, fmt.Errorf("expected 'unique ips' line, got %q", string(uniqueLine))
	}
	uniqueIPs, err := parseUint128Bytes(trimHeaderBytes(uniqueLine[len("unique ips "):]))
	if err != nil {
		return binaryHeader6{}, fmt.Errorf("parsing unique ips: %w", err)
	}

	if payloadBytes != records*32+4 {
		return binaryHeader6{}, fmt.Errorf("invalid payload size %d (expected %d)", payloadBytes, records*32+4)
	}

	return binaryHeader6{
		optimized:  optimized,
		recordSize: recordSize,
		records:    records,
		bytes:      payloadBytes,
		lines:      lines,
		uniqueIPs:  uniqueIPs,
		dataOffset: int64(p.offset),
	}, nil
}

func parseUint128Bytes(s []byte) (uint128, error) {
	if len(s) == 0 {
		return uint128Zero, fmt.Errorf("empty uint128 string")
	}
	var result uint128
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return uint128Zero, fmt.Errorf("non-digit %q in uint128", ch)
		}
		prev := result
		result = mulAdd10(result, uint64(ch-'0'))
		if result.LessThan(prev) {
			return uint128Zero, fmt.Errorf("uint128 overflow")
		}
	}
	return result, nil
}

func OpenFileSet6(path string) (FileSet6, error) {
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
	if fileSize == 0 {
		_ = f.Close()
		return &emptyFileSet6{}, nil
	}

	hdr, err := parseBinaryHeader6File(f)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if !hdr.optimized {
		_ = f.Close()
		return nil, fmt.Errorf("%s: %w", path, ErrNotOptimized)
	}

	expectedSize := hdr.dataOffset + 4 + int64(hdr.records)*32
	if fileSize != expectedSize {
		_ = f.Close()
		return nil, fmt.Errorf("%s: file size %d does not match expected %d", path, fileSize, expectedSize)
	}

	fs, err := openFileSet6Platform(f, path, fileSize, hdr)
	if err != nil {
		return nil, err
	}
	return fs, nil
}

type emptyFileSet6 struct {
	closed atomic.Bool
}

func (e *emptyFileSet6) Len() int { return 0 }
func (e *emptyFileSet6) Range(int) (Range6, error) {
	return Range6{}, fmt.Errorf("index 0 out of range [0, 0)")
}
func (e *emptyFileSet6) Contains(Uint128) bool               { return false }
func (e *emptyFileSet6) UniqueIPs() Uint128                  { return uint128Zero }
func (e *emptyFileSet6) Iter() func(yield func(Range6) bool) { return func(func(Range6) bool) {} }

func (e *emptyFileSet6) Err() error {
	if e.closed.Load() {
		return ErrFileSet6Closed
	}
	return nil
}

func (e *emptyFileSet6) Close() error {
	e.closed.Store(true)
	return nil
}

func fileSetContains6(ip Uint128, n int, readRange func(int) (Range6, error)) bool {
	ok, _ := fileSetContains6WithStats(ip, n, readRange)
	return ok
}

func fileSetContains6WithStats(ip Uint128, n int, readRange func(int) (Range6, error)) (bool, OperationStats) {
	stats := OperationStats{Lookups: 1, BinarySearches: 1}
	i := sort.Search(n, func(i int) bool {
		stats.Comparisons++
		r, err := readRange(i)
		if err != nil {
			return true
		}
		return !r.Hi.LessThan(ip)
	})
	if i >= n {
		return false, stats
	}
	r, err := readRange(i)
	if err != nil {
		return false, stats
	}
	stats.Comparisons++
	return !r.Lo.GreaterThan(ip), stats
}

func decodeRange6(buf []byte) Range6 {
	return Range6{
		Lo: uint128{
			Hi: nativeEndian.Uint64(buf[0:8]),
			Lo: nativeEndian.Uint64(buf[8:16]),
		},
		Hi: uint128{
			Hi: nativeEndian.Uint64(buf[16:24]),
			Lo: nativeEndian.Uint64(buf[24:32]),
		},
	}
}

func readRange6At(f *os.File, rangesOffset int64, i int) (Range6, error) {
	var buf [32]byte
	offset := rangesOffset + int64(i)*32
	n, err := f.ReadAt(buf[:], offset)
	if err != nil {
		return Range6{}, fmt.Errorf("reading range6 %d: %w", i, err)
	}
	if n != 32 {
		return Range6{}, fmt.Errorf("short read for range6 %d: got %d bytes", i, n)
	}
	return decodeRange6(buf[:]), nil
}
