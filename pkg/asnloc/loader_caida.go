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

// loadCAIDAPrefix2AS reads a CAIDA RouteViews prefix2as file (optionally
// gzip compressed) from disk and builds an in-memory range-table backend.
//
// The file format is documented at
// https://www.caida.org/catalog/datasets/routeviews-prefix2as/ and has
// three tab-separated columns per line: prefix, prefix_length, ASN.
// Multi-origin prefixes (MOAS) report multiple ASNs separated by an
// underscore (e.g. "13335_38803"); we keep the first one as the primary
// origin and ignore the rest, matching CAIDA's documented ordering by
// BGP visibility.
//
// CAIDA prefix2as data has no organization names — only ASN numbers. The
// resulting range table reports the ASN with an empty name. Operators
// who want names can cross-reference the iptoasn or MaxMind providers.
func loadCAIDAPrefix2AS(path string) (*rangeTableBackend, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open caida prefix2as %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	var reader io.Reader = f
	header := make([]byte, 2)
	if _, peekErr := io.ReadFull(f, header); peekErr == nil && header[0] == 0x1f && header[1] == 0x8b {
		if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
			return nil, fmt.Errorf("seek caida prefix2as: %w", seekErr)
		}
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("gzip caida prefix2as: %w", err)
		}
		defer func() { _ = gz.Close() }()
		reader = gz
	} else {
		if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
			return nil, fmt.Errorf("seek caida prefix2as: %w", seekErr)
		}
	}

	ranges, err := parseCAIDAPrefix2ASStream(reader)
	if err != nil {
		return nil, fmt.Errorf("parse caida prefix2as %q: %w", path, err)
	}
	return newRangeTableBackend(ranges), nil
}

// parseCAIDAPrefix2ASStream is the format-only parser, reused by tests.
// It accepts both tab and whitespace separation because the published
// files have used both historically. Lines that fail to parse are
// rejected with an error so the loader fails loud.
func parseCAIDAPrefix2ASStream(r io.Reader) ([]asnRange, error) {
	out := make([]asnRange, 0, 1<<20)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// CAIDA uses tabs, but we accept any whitespace for resilience.
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return nil, fmt.Errorf("line %d: expected 3 columns, got %d", lineNo, len(fields))
		}
		prefix := fields[0]
		// Skip IPv6 prefixes if they ever appear in the IPv4 file.
		if strings.ContainsRune(prefix, ':') {
			continue
		}
		lengthRaw := fields[1]
		asnField := fields[2]

		ones, err := strconv.Atoi(lengthRaw)
		if err != nil || ones < 0 || ones > 32 {
			return nil, fmt.Errorf("line %d: invalid prefix length %q", lineNo, lengthRaw)
		}
		// Build a CIDR from the prefix and length so net.ParseCIDR
		// validates both halves consistently.
		_, ipnet, err := net.ParseCIDR(prefix + "/" + lengthRaw)
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid prefix %s/%s: %w", lineNo, prefix, lengthRaw, err)
		}
		lo, hi, ok := ipnetToBounds(ipnet)
		if !ok {
			// Non-IPv4 prefix — skip silently (we already filtered ':',
			// so this is a paranoid guard).
			continue
		}
		// MOAS handling: multiple origin ASNs separated by underscores.
		// CAIDA documents the order as BGP visibility, so the first one
		// is the primary origin and the one we want.
		first := asnField
		if idx := strings.Index(asnField, "_"); idx >= 0 {
			first = asnField[:idx]
		}
		// Some historical files have used commas — accept that too.
		if idx := strings.Index(first, ","); idx >= 0 {
			first = first[:idx]
		}
		asn, err := strconv.ParseUint(first, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid ASN %q: %w", lineNo, asnField, err)
		}
		if asn == 0 {
			continue
		}
		out = append(out, asnRange{lo: lo, hi: hi, asn: uint32(asn)})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}
	return out, nil
}
