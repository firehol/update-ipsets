package asnloc

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
)

// loadIPToASNTSV reads an iptoasn.com combined TSV file (optionally gzip
// compressed) from disk and builds an in-memory range-table backend. The
// file format documented at https://iptoasn.com/ has tab-separated
// columns: range_start, range_end, AS_number, country_code,
// AS_description.
//
// Lines with ASN=0 (which iptoasn marks as "Not routed") are skipped so
// the resulting backend reports them as misses (the range walker treats
// misses as "unknown") rather than as a real attribution to AS0.
//
// IPv6 lines (which appear in the combined file but not in the v4-only
// file) are silently skipped because the rest of the engine is IPv4
// only.
//
// Malformed lines return an error so the loader fails loud rather than
// silently producing a half-loaded database.
func loadIPToASNTSV(path string) (*rangeTableBackend, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open iptoasn tsv %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	var reader io.Reader = f
	// Auto-detect gzip — the engine usually decompresses the archive
	// before calling Open, but staying tolerant lets the parser be
	// reused from tests with raw .tsv.gz fixtures.
	header := make([]byte, 2)
	if _, peekErr := io.ReadFull(f, header); peekErr == nil && header[0] == 0x1f && header[1] == 0x8b {
		if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
			return nil, fmt.Errorf("seek iptoasn tsv: %w", seekErr)
		}
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("gzip iptoasn tsv: %w", err)
		}
		defer func() { _ = gz.Close() }()
		reader = gz
	} else {
		if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
			return nil, fmt.Errorf("seek iptoasn tsv: %w", seekErr)
		}
	}

	ranges, err := parseIPToASNTSVStream(reader)
	if err != nil {
		return nil, fmt.Errorf("parse iptoasn tsv %q: %w", path, err)
	}
	return newRangeTableBackend(ranges), nil
}

// parseIPToASNTSVStream is the format-only parser, reused by tests so
// they do not need a temp file. It enforces strict column count and
// numeric ASN/IP validation but skips lines that are clearly informational
// noise (blank lines, comments, AS0 "Not routed" gaps, IPv6 entries).
func parseIPToASNTSVStream(r io.Reader) ([]asnRange, error) {
	out := make([]asnRange, 0, 1<<19)
	scanner := bufio.NewScanner(r)
	// iptoasn lines are short, but be generous to survive long org names.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 5 {
			return nil, fmt.Errorf("line %d: expected at least 5 tab-separated columns, got %d", lineNo, len(parts))
		}
		startRaw := strings.TrimSpace(parts[0])
		endRaw := strings.TrimSpace(parts[1])
		// Skip IPv6 lines that may appear in the combined file. We
		// detect them by looking for ':' which only occurs in IPv6
		// addresses, never in IPv4.
		if strings.ContainsRune(startRaw, ':') || strings.ContainsRune(endRaw, ':') {
			continue
		}
		lo, ok := parseIPv4Decimal(startRaw)
		if !ok {
			return nil, fmt.Errorf("line %d: invalid start address %q", lineNo, startRaw)
		}
		hi, ok := parseIPv4Decimal(endRaw)
		if !ok {
			return nil, fmt.Errorf("line %d: invalid end address %q", lineNo, endRaw)
		}
		if hi < lo {
			return nil, fmt.Errorf("line %d: end %s before start %s", lineNo, endRaw, startRaw)
		}
		asn, err := strconv.ParseUint(strings.TrimSpace(parts[2]), 10, 32)
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid ASN %q: %w", lineNo, parts[2], err)
		}
		if asn == 0 {
			// "Not routed" gap — skip so it surfaces as unknown rather
			// than as a fake AS0 attribution.
			continue
		}
		name := strings.TrimSpace(parts[4])
		out = append(out, asnRange{lo: lo, hi: hi, asn: uint32(asn), name: name})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}
	return out, nil
}

// parseIPv4Decimal parses a dotted-quad IPv4 string into a host-order
// uint32. Returns false for any malformed input. Anything containing a
// colon is rejected (the caller is expected to filter IPv6 lines first,
// but the guard is cheap).
func parseIPv4Decimal(s string) (uint32, bool) {
	ip := net.ParseIP(s)
	if ip == nil {
		return 0, false
	}
	v4 := ip.To4()
	if v4 == nil {
		return 0, false
	}
	return uint32(v4[0])<<24 | uint32(v4[1])<<16 | uint32(v4[2])<<8 | uint32(v4[3]), true
}
