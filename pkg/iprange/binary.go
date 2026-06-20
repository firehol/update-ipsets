package iprange

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const binaryEndiannessMarker uint32 = 0x1A2B3C4D

func WriteBinary(w io.Writer, set *IPSet) error {
	_, err := WriteBinaryWithStats(w, set)
	return err
}

func WriteBinaryWithStats(w io.Writer, set *IPSet) (OperationStats, error) {
	var stats OperationStats
	set.Optimize()
	if len(set.Ranges) == 0 {
		return stats, nil
	}

	cw := &countingWriter{w: w}
	if _, err := fmt.Fprint(cw, BinaryHeaderV10); err != nil {
		return stats, err
	}
	if set.Optimized {
		if _, err := fmt.Fprintln(cw, "optimized"); err != nil {
			return stats, err
		}
	} else if _, err := fmt.Fprintln(cw, "non-optimized"); err != nil {
		return stats, err
	}
	if _, err := fmt.Fprintf(cw, "record size %d\n", 8); err != nil {
		return stats, err
	}
	if _, err := fmt.Fprintf(cw, "records %d\n", len(set.Ranges)); err != nil {
		return stats, err
	}
	if _, err := fmt.Fprintf(cw, "bytes %d\n", len(set.Ranges)*8+4); err != nil {
		return stats, err
	}
	if _, err := fmt.Fprintf(cw, "lines %d\n", set.Lines); err != nil {
		return stats, err
	}
	if _, err := fmt.Fprintf(cw, "unique ips %d\n", set.UniqueIPs); err != nil {
		return stats, err
	}
	var marker [4]byte
	nativeEndian.PutUint32(marker[:], binaryEndiannessMarker)
	if _, err := cw.Write(marker[:]); err != nil {
		return stats, err
	}

	var payload [4096]byte
	const recordsPerChunk = len(payload) / 8
	for start := 0; start < len(set.Ranges); {
		n := len(set.Ranges) - start
		if n > recordsPerChunk {
			n = recordsPerChunk
		}
		off := 0
		for _, r := range set.Ranges[start : start+n] {
			nativeEndian.PutUint32(payload[off:off+4], r.Lo)
			nativeEndian.PutUint32(payload[off+4:off+8], r.Hi)
			off += 8
		}
		if _, err := cw.Write(payload[:off]); err != nil {
			return stats, err
		}
		stats.RangesWritten += int64(n)
		start += n
	}
	stats.BytesWritten = cw.n
	return stats, nil
}

func ReadBinary(name string, r io.Reader) (*IPSet, error) {
	set, _, err := ReadBinaryWithStats(name, r)
	return set, err
}

func ReadBinaryWithStats(name string, r io.Reader) (*IPSet, OperationStats, error) {
	var stats OperationStats
	var records int
	cr := &countingReader{r: r}
	br := bufio.NewReader(cr)

	line, err := br.ReadString('\n')
	if err != nil {
		stats.BytesRead = cr.n
		return nil, stats, err
	}
	if line != BinaryHeaderV10 {
		stats.BytesRead = cr.n
		return nil, stats, fmt.Errorf("%s: expecting binary header but found %q", name, strings.TrimSpace(line))
	}

	mode, err := br.ReadString('\n')
	if err != nil {
		stats.BytesRead = cr.n
		return nil, stats, err
	}
	mode = strings.TrimSpace(mode)
	if mode != "optimized" && mode != "non-optimized" {
		stats.BytesRead = cr.n
		return nil, stats, fmt.Errorf("%s: invalid optimization marker %q", name, mode)
	}

	recordSize, err := readBinaryIntLine(br, "record size ")
	if err != nil {
		stats.BytesRead = cr.n
		return nil, stats, fmt.Errorf("%s: %w", name, err)
	}
	if recordSize != 8 {
		stats.BytesRead = cr.n
		return nil, stats, fmt.Errorf("%s: invalid record size %d", name, recordSize)
	}
	records, err = readBinaryIntLine(br, "records ")
	if err != nil {
		stats.BytesRead = cr.n
		return nil, stats, fmt.Errorf("%s: %w", name, err)
	}
	payloadBytes, err := readBinaryIntLine(br, "bytes ")
	if err != nil {
		stats.BytesRead = cr.n
		return nil, stats, fmt.Errorf("%s: %w", name, err)
	}
	lines, err := readBinaryIntLine(br, "lines ")
	if err != nil {
		stats.BytesRead = cr.n
		return nil, stats, fmt.Errorf("%s: %w", name, err)
	}
	uniqueIPs, err := readBinaryUint64Line(br, "unique ips ")
	if err != nil {
		stats.BytesRead = cr.n
		return nil, stats, fmt.Errorf("%s: %w", name, err)
	}
	if payloadBytes != records*8+4 {
		stats.BytesRead = cr.n
		return nil, stats, fmt.Errorf("%s: invalid payload size %d", name, payloadBytes)
	}
	if uniqueIPs < uint64(records) || lines < records {
		stats.BytesRead = cr.n
		return nil, stats, fmt.Errorf("%s: inconsistent binary counters", name)
	}

	var markerBuf [4]byte
	if _, err := io.ReadFull(br, markerBuf[:]); err != nil {
		stats.BytesRead = cr.n
		return nil, stats, err
	}
	marker := nativeEndian.Uint32(markerBuf[:])
	if marker != binaryEndiannessMarker {
		stats.BytesRead = cr.n
		return nil, stats, fmt.Errorf("%s: incompatible endianness", name)
	}

	set := New(name)
	set.Ranges = make([]Range, records)
	var rangeBuf [8]byte
	for i := 0; i < records; i++ {
		if _, err := io.ReadFull(br, rangeBuf[:]); err != nil {
			stats.BytesRead = cr.n
			return nil, stats, fmt.Errorf("%s: reading range %d: %w", name, i+1, err)
		}
		set.Ranges[i] = decodeRange(rangeBuf[:])
		if !set.Ranges[i].Valid() {
			stats.BytesRead = cr.n
			return nil, stats, fmt.Errorf("%s: invalid binary range %d", name, i+1)
		}
		stats.RangesRead++
	}
	if _, err := br.Peek(1); !errors.Is(err, io.EOF) {
		if err == nil {
			stats.BytesRead = cr.n
			return nil, stats, fmt.Errorf("%s: trailing binary data found", name)
		}
		stats.BytesRead = cr.n
		return nil, stats, err
	}

	set.Lines = lines
	set.Optimized = false
	if mode == "optimized" {
		set.Optimize()
		if set.UniqueIPs != uniqueIPs {
			stats.BytesRead = cr.n
			return nil, stats, fmt.Errorf("%s: unique IPs do not match payload", name)
		}
	} else {
		set.UniqueIPs = uniqueIPs
	}
	stats.BytesRead = cr.n
	return set, stats, nil
}

type countingWriter struct {
	w io.Writer
	n int64
}

func (w *countingWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	w.n += int64(n)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	return n, err
}

type countingReader struct {
	r io.Reader
	n int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	r.n += int64(n)
	return n, err
}

func readBinaryIntLine(br *bufio.Reader, prefix string) (int, error) {
	line, err := br.ReadString('\n')
	if err != nil {
		return 0, err
	}
	if !strings.HasPrefix(line, prefix) {
		return 0, fmt.Errorf("expected %q line", prefix)
	}
	n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, prefix)))
	if err != nil {
		return 0, err
	}
	return n, nil
}

func readBinaryUint64Line(br *bufio.Reader, prefix string) (uint64, error) {
	line, err := br.ReadString('\n')
	if err != nil {
		return 0, err
	}
	if !strings.HasPrefix(line, prefix) {
		return 0, fmt.Errorf("expected %q line", prefix)
	}
	n, err := strconv.ParseUint(strings.TrimSpace(strings.TrimPrefix(line, prefix)), 10, 64)
	if err != nil {
		return 0, err
	}
	return n, nil
}
