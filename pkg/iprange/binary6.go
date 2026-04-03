package iprange

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

const BinaryHeaderV20IPv6 = "iprange binary format v2.0\n"

func WriteBinary6(w io.Writer, set *IPSet6) (err error) {
	started := time.Now()
	_, span := iprangeStart(iprangeBackground(), "iprange.save.binary", attribute.String("ip.version", "6"))
	defer func() {
		iprangeEnd(span, err)
		bytes := int64(0)
		if set != nil {
			bytes = int64(len(set.Ranges)*32 + 4)
		}
		iprangeObserve(iprangeBackground(), "iprange.save.binary", 1, bytes, time.Since(started), attribute.String("ip.version", "6"))
	}()
	set.Optimize()
	if len(set.Ranges) == 0 {
		return nil
	}

	bw := bufio.NewWriterSize(w, 64*1024)
	if _, err := fmt.Fprint(bw, BinaryHeaderV20IPv6); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(bw, "ipv6"); err != nil {
		return err
	}
	if set.Optimized {
		if _, err := fmt.Fprintln(bw, "optimized"); err != nil {
			return err
		}
	} else if _, err := fmt.Fprintln(bw, "non-optimized"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(bw, "record size %d\n", 32); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(bw, "records %d\n", len(set.Ranges)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(bw, "bytes %d\n", len(set.Ranges)*32+4); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(bw, "lines %d\n", set.Lines); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(bw, "unique ips %s\n", set.UniqueIPs.String()); err != nil {
		return err
	}
	if err := binary.Write(bw, nativeEndian, binaryEndiannessMarker); err != nil {
		return err
	}

	payload := make([]byte, len(set.Ranges)*32)
	off := 0
	for _, r := range set.Ranges {
		nativeEndian.PutUint64(payload[off:off+8], r.Lo.Hi)
		nativeEndian.PutUint64(payload[off+8:off+16], r.Lo.Lo)
		nativeEndian.PutUint64(payload[off+16:off+24], r.Hi.Hi)
		nativeEndian.PutUint64(payload[off+24:off+32], r.Hi.Lo)
		off += 32
	}
	if _, err := bw.Write(payload); err != nil {
		return err
	}
	return bw.Flush()
}

func ReadBinary6(name string, r io.Reader) (set *IPSet6, err error) {
	started := time.Now()
	_, span := iprangeStart(iprangeBackground(), "iprange.load.binary", attribute.String("ip.version", "6"))
	var bytes int64
	defer func() {
		iprangeEnd(span, err)
		iprangeObserve(iprangeBackground(), "iprange.load.binary", 1, bytes, time.Since(started), attribute.String("ip.version", "6"))
	}()
	br := bufio.NewReader(r)

	line, err := br.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if line != BinaryHeaderV20IPv6 {
		return nil, fmt.Errorf("%s: expecting binary v2 header but found %q", name, strings.TrimSpace(line))
	}

	family, err := br.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(family) != "ipv6" {
		return nil, fmt.Errorf("%s: expected family 'ipv6' but found %q", name, strings.TrimSpace(family))
	}

	mode, err := br.ReadString('\n')
	if err != nil {
		return nil, err
	}
	mode = strings.TrimSpace(mode)
	if mode != "optimized" && mode != "non-optimized" {
		return nil, fmt.Errorf("%s: invalid optimization marker %q", name, mode)
	}

	recordSize, err := readBinaryIntLine(br, "record size ")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if recordSize != 32 {
		return nil, fmt.Errorf("%s: invalid record size %d (expected 32)", name, recordSize)
	}
	records, err := readBinaryIntLine(br, "records ")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	payloadBytes, err := readBinaryIntLine(br, "bytes ")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	bytes = int64(payloadBytes)
	lines, err := readBinaryIntLine(br, "lines ")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	uniqueLine, err := br.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(uniqueLine, "unique ips ") {
		return nil, fmt.Errorf("%s: expected 'unique ips' line", name)
	}
	uniqueIPs, err := parseUint128(strings.TrimSpace(strings.TrimPrefix(uniqueLine, "unique ips ")))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	if payloadBytes != records*32+4 {
		return nil, fmt.Errorf("%s: invalid payload size %d", name, payloadBytes)
	}

	var marker uint32
	if err := binary.Read(br, nativeEndian, &marker); err != nil {
		return nil, err
	}
	if marker != binaryEndiannessMarker {
		return nil, fmt.Errorf("%s: incompatible endianness", name)
	}

	set = New6(name)
	set.Ranges = make([]Range6, records)

	var buf [32]byte
	for i := 0; i < records; i++ {
		if _, err := io.ReadFull(br, buf[:]); err != nil {
			return nil, fmt.Errorf("%s: reading range6 %d: %w", name, i+1, err)
		}
		set.Ranges[i] = decodeRange6(buf[:])
		if !set.Ranges[i].Valid() {
			return nil, fmt.Errorf("%s: invalid binary range6 %d", name, i+1)
		}
	}

	if _, err := br.Peek(1); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("%s: trailing binary data found", name)
		}
		return nil, err
	}

	set.Lines = lines
	set.Optimized = false
	if mode == "optimized" {
		set.Optimize()
		if !set.UniqueIPs.Equals(uniqueIPs) {
			return nil, fmt.Errorf("%s: unique IPs do not match payload", name)
		}
	} else {
		set.UniqueIPs = uniqueIPs
	}
	return set, nil
}
