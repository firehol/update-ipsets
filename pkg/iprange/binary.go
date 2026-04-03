package iprange

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

const binaryEndiannessMarker uint32 = 0x1A2B3C4D

func WriteBinary(w io.Writer, set *IPSet) error {
	started := time.Now()
	ctx, span := iprangeStart(iprangeBackground(), "iprange.save.binary", attribute.String("ip.version", "4"))
	var opErr error
	defer func() {
		var bytes int64
		if set != nil {
			bytes = int64(len(set.Ranges))*8 + 4
		}
		iprangeObserve(ctx, "iprange.save.binary", 1, bytes, time.Since(started), attribute.String("ip.version", "4"))
		iprangeEnd(span, opErr)
	}()
	set.Optimize()
	if len(set.Ranges) == 0 {
		return nil
	}

	bw := bufio.NewWriterSize(w, 64*1024)
	if _, err := fmt.Fprint(bw, BinaryHeaderV10); err != nil {
		opErr = err
		return err
	}
	if set.Optimized {
		if _, err := fmt.Fprintln(bw, "optimized"); err != nil {
			opErr = err
			return err
		}
	} else if _, err := fmt.Fprintln(bw, "non-optimized"); err != nil {
		opErr = err
		return err
	}
	if _, err := fmt.Fprintf(bw, "record size %d\n", 8); err != nil {
		opErr = err
		return err
	}
	if _, err := fmt.Fprintf(bw, "records %d\n", len(set.Ranges)); err != nil {
		opErr = err
		return err
	}
	if _, err := fmt.Fprintf(bw, "bytes %d\n", len(set.Ranges)*8+4); err != nil {
		opErr = err
		return err
	}
	if _, err := fmt.Fprintf(bw, "lines %d\n", set.Lines); err != nil {
		opErr = err
		return err
	}
	if _, err := fmt.Fprintf(bw, "unique ips %d\n", set.UniqueIPs); err != nil {
		opErr = err
		return err
	}
	if err := binary.Write(bw, nativeEndian, binaryEndiannessMarker); err != nil {
		opErr = err
		return err
	}

	payload := make([]byte, len(set.Ranges)*8)
	off := 0
	for _, r := range set.Ranges {
		nativeEndian.PutUint32(payload[off:off+4], r.Lo)
		nativeEndian.PutUint32(payload[off+4:off+8], r.Hi)
		off += 8
	}
	if _, err := bw.Write(payload); err != nil {
		opErr = err
		return err
	}
	opErr = bw.Flush()
	return opErr
}

func ReadBinary(name string, r io.Reader) (*IPSet, error) {
	started := time.Now()
	ctx, span := iprangeStart(iprangeBackground(), "iprange.load.binary", attribute.String("ip.version", "4"), attribute.String("iprange.name", name))
	var opErr error
	var records int
	defer func() {
		iprangeObserve(ctx, "iprange.load.binary", 1, int64(records)*8+4, time.Since(started), attribute.String("ip.version", "4"))
		iprangeEnd(span, opErr)
	}()
	br := bufio.NewReader(r)

	line, err := br.ReadString('\n')
	if err != nil {
		opErr = err
		return nil, err
	}
	if line != BinaryHeaderV10 {
		opErr = fmt.Errorf("%s: expecting binary header but found %q", name, strings.TrimSpace(line))
		return nil, opErr
	}

	mode, err := br.ReadString('\n')
	if err != nil {
		opErr = err
		return nil, err
	}
	mode = strings.TrimSpace(mode)
	if mode != "optimized" && mode != "non-optimized" {
		opErr = fmt.Errorf("%s: invalid optimization marker %q", name, mode)
		return nil, opErr
	}

	recordSize, err := readBinaryIntLine(br, "record size ")
	if err != nil {
		opErr = fmt.Errorf("%s: %w", name, err)
		return nil, opErr
	}
	if recordSize != 8 {
		opErr = fmt.Errorf("%s: invalid record size %d", name, recordSize)
		return nil, opErr
	}
	records, err = readBinaryIntLine(br, "records ")
	if err != nil {
		opErr = fmt.Errorf("%s: %w", name, err)
		return nil, opErr
	}
	payloadBytes, err := readBinaryIntLine(br, "bytes ")
	if err != nil {
		opErr = fmt.Errorf("%s: %w", name, err)
		return nil, opErr
	}
	lines, err := readBinaryIntLine(br, "lines ")
	if err != nil {
		opErr = fmt.Errorf("%s: %w", name, err)
		return nil, opErr
	}
	uniqueIPs, err := readBinaryUint64Line(br, "unique ips ")
	if err != nil {
		opErr = fmt.Errorf("%s: %w", name, err)
		return nil, opErr
	}
	if payloadBytes != records*8+4 {
		opErr = fmt.Errorf("%s: invalid payload size %d", name, payloadBytes)
		return nil, opErr
	}
	if uniqueIPs < uint64(records) || lines < records {
		opErr = fmt.Errorf("%s: inconsistent binary counters", name)
		return nil, opErr
	}

	var marker uint32
	if err := binary.Read(br, nativeEndian, &marker); err != nil {
		opErr = err
		return nil, err
	}
	if marker != binaryEndiannessMarker {
		opErr = fmt.Errorf("%s: incompatible endianness", name)
		return nil, opErr
	}

	set := New(name)
	set.Ranges = make([]Range, records)
	for i := 0; i < records; i++ {
		if err := binary.Read(br, nativeEndian, &set.Ranges[i].Lo); err != nil {
			opErr = err
			return nil, err
		}
		if err := binary.Read(br, nativeEndian, &set.Ranges[i].Hi); err != nil {
			opErr = err
			return nil, err
		}
		if !set.Ranges[i].Valid() {
			opErr = fmt.Errorf("%s: invalid binary range %d", name, i+1)
			return nil, opErr
		}
	}
	if _, err := br.Peek(1); !errors.Is(err, io.EOF) {
		if err == nil {
			opErr = fmt.Errorf("%s: trailing binary data found", name)
			return nil, opErr
		}
		opErr = err
		return nil, err
	}

	set.Lines = lines
	set.Optimized = false
	if mode == "optimized" {
		set.Optimize()
		if set.UniqueIPs != uniqueIPs {
			opErr = fmt.Errorf("%s: unique IPs do not match payload", name)
			return nil, opErr
		}
	} else {
		set.UniqueIPs = uniqueIPs
	}
	return set, nil
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
