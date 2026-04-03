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
	"time"

	"go.opentelemetry.io/otel/attribute"
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

func OpenFileSet6(path string) (FileSet6, error) {
	started := time.Now()
	_, span := iprangeStart(iprangeBackground(), "iprange.load.binary", attribute.String("ip.version", "6"))
	var opErr error
	var bytes int64
	defer func() {
		iprangeEnd(span, opErr)
		iprangeObserve(iprangeBackground(), "iprange.load.binary", 1, bytes, time.Since(started), attribute.String("ip.version", "6"), attribute.String("iprange.source", "fileset"))
	}()
	f, err := os.Open(path)
	if err != nil {
		opErr = err
		return nil, err
	}

	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		opErr = err
		return nil, err
	}
	fileSize := fi.Size()
	bytes = fileSize
	if fileSize == 0 {
		_ = f.Close()
		return &emptyFileSet6{}, nil
	}

	hdr, err := parseBinaryHeader6(f)
	if err != nil {
		_ = f.Close()
		opErr = fmt.Errorf("%s: %w", path, err)
		return nil, opErr
	}
	if !hdr.optimized {
		_ = f.Close()
		opErr = fmt.Errorf("%s: %w", path, ErrNotOptimized)
		return nil, opErr
	}

	expectedSize := hdr.dataOffset + 4 + int64(hdr.records)*32
	if fileSize != expectedSize {
		_ = f.Close()
		opErr = fmt.Errorf("%s: file size %d does not match expected %d", path, fileSize, expectedSize)
		return nil, opErr
	}

	fs, err := openFileSet6Platform(f, path, fileSize, hdr)
	if err != nil {
		opErr = err
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
	iprangeCount(iprangeBackground(), "iprange.contains.ops", 1, attribute.String("ip.version", "6"), attribute.String("iprange.source", "fileset"))
	iprangeCount(iprangeBackground(), "iprange.binary.searches", 1, attribute.String("ip.version", "6"), attribute.String("iprange.source", "fileset"))
	i := sort.Search(n, func(i int) bool {
		r, err := readRange(i)
		if err != nil {
			return true
		}
		return !r.Hi.LessThan(ip)
	})
	if i >= n {
		return false
	}
	r, err := readRange(i)
	if err != nil {
		return false
	}
	return !r.Lo.GreaterThan(ip)
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
