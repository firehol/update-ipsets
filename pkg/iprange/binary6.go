package iprange

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

const BinaryHeaderV20IPv6 = "iprange binary format v2.0\n"

func WriteBinary6(w io.Writer, set *IPSet6) (err error) {
	_, err = WriteBinary6WithStats(w, set)
	return err
}

func WriteBinary6WithStats(w io.Writer, set *IPSet6) (OperationStats, error) {
	var stats OperationStats
	set.Optimize()
	if len(set.Ranges) == 0 {
		return stats, nil
	}

	cw := &countingWriter{w: w}
	if _, err := fmt.Fprint(cw, BinaryHeaderV20IPv6); err != nil {
		return stats, err
	}
	if _, err := fmt.Fprintln(cw, "ipv6"); err != nil {
		return stats, err
	}
	if set.Optimized {
		if _, err := fmt.Fprintln(cw, "optimized"); err != nil {
			return stats, err
		}
	} else if _, err := fmt.Fprintln(cw, "non-optimized"); err != nil {
		return stats, err
	}
	if _, err := fmt.Fprintf(cw, "record size %d\n", 32); err != nil {
		return stats, err
	}
	if _, err := fmt.Fprintf(cw, "records %d\n", len(set.Ranges)); err != nil {
		return stats, err
	}
	if _, err := fmt.Fprintf(cw, "bytes %d\n", len(set.Ranges)*32+4); err != nil {
		return stats, err
	}
	if _, err := fmt.Fprintf(cw, "lines %d\n", set.Lines); err != nil {
		return stats, err
	}
	if _, err := fmt.Fprintf(cw, "unique ips %s\n", set.UniqueIPs.String()); err != nil {
		return stats, err
	}
	var marker [4]byte
	nativeEndian.PutUint32(marker[:], binaryEndiannessMarker)
	if _, err := cw.Write(marker[:]); err != nil {
		return stats, err
	}

	var payload [4096]byte
	const recordsPerChunk = len(payload) / 32
	for start := 0; start < len(set.Ranges); {
		n := len(set.Ranges) - start
		if n > recordsPerChunk {
			n = recordsPerChunk
		}
		off := 0
		for _, r := range set.Ranges[start : start+n] {
			nativeEndian.PutUint64(payload[off:off+8], r.Lo.Hi)
			nativeEndian.PutUint64(payload[off+8:off+16], r.Lo.Lo)
			nativeEndian.PutUint64(payload[off+16:off+24], r.Hi.Hi)
			nativeEndian.PutUint64(payload[off+24:off+32], r.Hi.Lo)
			off += 32
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

func ReadBinary6(name string, r io.Reader) (set *IPSet6, err error) {
	set, _, err = ReadBinary6WithStats(name, r)
	return set, err
}

func ReadBinary6WithStats(name string, r io.Reader) (*IPSet6, OperationStats, error) {
	var stats OperationStats
	cr := &countingReader{r: r}
	br := bufio.NewReader(cr)

	line, err := br.ReadString('\n')
	if err != nil {
		stats.BytesRead = cr.n
		return nil, stats, err
	}
	if line != BinaryHeaderV20IPv6 {
		stats.BytesRead = cr.n
		return nil, stats, fmt.Errorf("%s: expecting binary v2 header but found %q", name, strings.TrimSpace(line))
	}

	family, err := br.ReadString('\n')
	if err != nil {
		stats.BytesRead = cr.n
		return nil, stats, err
	}
	if strings.TrimSpace(family) != "ipv6" {
		stats.BytesRead = cr.n
		return nil, stats, fmt.Errorf("%s: expected family 'ipv6' but found %q", name, strings.TrimSpace(family))
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
	if recordSize != 32 {
		stats.BytesRead = cr.n
		return nil, stats, fmt.Errorf("%s: invalid record size %d (expected 32)", name, recordSize)
	}
	records, err := readBinaryIntLine(br, "records ")
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
	uniqueLine, err := br.ReadString('\n')
	if err != nil {
		stats.BytesRead = cr.n
		return nil, stats, err
	}
	if !strings.HasPrefix(uniqueLine, "unique ips ") {
		stats.BytesRead = cr.n
		return nil, stats, fmt.Errorf("%s: expected 'unique ips' line", name)
	}
	uniqueIPs, err := parseUint128(strings.TrimSpace(strings.TrimPrefix(uniqueLine, "unique ips ")))
	if err != nil {
		stats.BytesRead = cr.n
		return nil, stats, fmt.Errorf("%s: %w", name, err)
	}

	if payloadBytes != records*32+4 {
		stats.BytesRead = cr.n
		return nil, stats, fmt.Errorf("%s: invalid payload size %d", name, payloadBytes)
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

	set := New6(name)
	set.Ranges = make([]Range6, records)

	var buf [32]byte
	for i := 0; i < records; i++ {
		if _, err := io.ReadFull(br, buf[:]); err != nil {
			stats.BytesRead = cr.n
			return nil, stats, fmt.Errorf("%s: reading range6 %d: %w", name, i+1, err)
		}
		set.Ranges[i] = decodeRange6(buf[:])
		if !set.Ranges[i].Valid() {
			stats.BytesRead = cr.n
			return nil, stats, fmt.Errorf("%s: invalid binary range6 %d", name, i+1)
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
		if !set.UniqueIPs.Equals(uniqueIPs) {
			stats.BytesRead = cr.n
			return nil, stats, fmt.Errorf("%s: unique IPs do not match payload", name)
		}
	} else {
		set.UniqueIPs = uniqueIPs
	}
	stats.BytesRead = cr.n
	return set, stats, nil
}
