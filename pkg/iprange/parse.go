package iprange

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"
)

type ParseOptions struct {
	DefaultPrefix     int
	UseCIDRNetwork    bool
	DNSThreads        int
	Resolver          Resolver
	Progress          func(ParseProgress)
	Stats             *OperationStats
	RangeCapacityHint int
}

type ParseProgress struct {
	Stage              string
	BytesRead          int64
	LinesRead          int64
	RangesAccepted     int64
	HostnamesQueued    int64
	HostnamesCompleted int64
	HostnamesResolved  int64
}

type hostnameRequest struct {
	host string
}

func DefaultParseOptions() ParseOptions {
	return ParseOptions{
		DefaultPrefix:  32,
		UseCIDRNetwork: true,
		DNSThreads:     5,
		Resolver:       DefaultResolver{},
	}
}

const (
	maxParseRangeCapacityHint = 1 << 18
	parseIPv4BytesPerRange    = 16
	parseIPv6BytesPerRange    = 32
)

type remainingLenReader interface {
	Len() int
}

// EstimateRangeCapacityHint returns a bounded initial range-slice capacity
// estimate for text input of the given address family.
func EstimateRangeCapacityHint(inputBytes int64, family AddressFamily) int {
	if inputBytes <= 0 {
		return 0
	}
	bytesPerRange := parseIPv4BytesPerRange
	if family == FamilyIPv6 {
		bytesPerRange = parseIPv6BytesPerRange
	}
	hint := inputBytes / int64(bytesPerRange)
	if hint <= 0 {
		hint = 1
	}
	if hint > maxParseRangeCapacityHint {
		return maxParseRangeCapacityHint
	}
	return int(hint)
}

func parseRangeCapacityHint(explicit int, r io.Reader, family AddressFamily) int {
	if explicit > 0 {
		if explicit > maxParseRangeCapacityHint {
			return maxParseRangeCapacityHint
		}
		return explicit
	}
	if lr, ok := r.(remainingLenReader); ok {
		return EstimateRangeCapacityHint(int64(lr.Len()), family)
	}
	return 0
}

// ParseReader parses an IPv4 list from r and returns the resulting set.
//
// Parsing is INTENTIONALLY LENIENT. Lines that do not parse as an IPv4
// address, range, or CIDR are SKIPPED, not raised as errors. This
// matches the historical behaviour of the C `iprange` tool that the
// bash version of update-ipsets has used in production for years —
// see `firehol/iprange/src/ipset_load.c` lines 290-330. Skipping is
// the right default because:
//
//   - IPv4 feeds occasionally pick up IPv6 entries when a maintainer
//     publishes both families to the same URL. The C tool silently
//     drops them with a counter; we do the same.
//   - Some sources interleave section headings, blank prose, or
//     stray tokens that the upstream `remove_comments` filter does
//     not catch. A single bad line should not destroy a 1M-entry
//     feed.
//   - Genuinely broken sources (HTML/XML returned in place of an
//     IP list) parse to ZERO addresses, which the engine surfaces
//     downstream as `last_status: empty` — distinguishable from a
//     real `parse_failed` for actual systemic issues.
//
// The only errors `ParseReader` can return are now:
//   - invalid options (`DefaultPrefix` out of range)
//   - underlying I/O errors from `r`
//   - errors from the binary loader if the input is a binary set
//   - errors from `set.Add` (impossible for valid CIDRs)
//   - DNS resolution errors when the file contains hostnames
func ParseReader(ctx context.Context, name string, r io.Reader, opts ParseOptions) (*IPSet, error) {
	if opts.DefaultPrefix < 0 || opts.DefaultPrefix > 32 {
		return nil, ErrInvalidPrefix
	}
	if opts.DNSThreads <= 0 {
		opts.DNSThreads = 1
	}
	if opts.Resolver == nil {
		opts.Resolver = DefaultResolver{}
	}
	capacityHint := parseRangeCapacityHint(opts.RangeCapacityHint, r, FamilyIPv4)

	// Use a buffered reader so we can peek at the first bytes to detect
	// the binary format without buffering the entire input.
	br := bufio.NewReaderSize(r, 64*1024)
	header, err := br.Peek(len(BinaryHeaderV10))
	if err == nil && string(header) == BinaryHeaderV10 {
		set, stats, err := ReadBinaryWithStats(name, br)
		if opts.Stats != nil {
			opts.Stats.Add(stats)
		}
		return set, err
	}
	header6, err := br.Peek(len(BinaryHeaderV20IPv6))
	if err == nil && string(header6) == BinaryHeaderV20IPv6 {
		return nil, fmt.Errorf("%s: IPv6 binary set loaded in IPv4 mode", name)
	}

	set := New(name)
	if capacityHint > 0 {
		set.Ranges = make([]Range, 0, capacityHint)
	}
	var hostnames []hostnameRequest
	firstLine := true
	var progress ParseProgress
	lastProgressAt := time.Now()
	lastProgressBytes := int64(0)
	notifyProgress := func(stage string, force bool) {
		if opts.Progress == nil {
			return
		}
		now := time.Now()
		if !force && progress.BytesRead-lastProgressBytes < 1024*1024 && now.Sub(lastProgressAt) < time.Second {
			return
		}
		progress.Stage = stage
		opts.Progress(progress)
		lastProgressAt = now
		lastProgressBytes = progress.BytesRead
	}

	if err := forEachTextLineBytes(br, func(line []byte) error {
		progress.LinesRead++
		progress.BytesRead += int64(len(line))
		if opts.Stats != nil {
			opts.Stats.LinesRead++
			opts.Stats.BytesRead += int64(len(line))
		}
		defer notifyProgress("read", false)
		// Strip UTF-8 BOM from the first line.
		if firstLine {
			firstLine = false
			line = trimBOMBytes(line)
		}
		trimmed := stripInlineCommentBytes(line)
		if len(trimmed) == 0 {
			return nil
		}

		if bytes.IndexByte(trimmed, '-') >= 0 {
			left, right, ok := splitRangeLineBytes(trimmed)
			if ok {
				lo, _, errLeft := parseRangeEndpointBytes(left, opts)
				if errLeft != nil {
					// Skip the malformed range line and keep going.
					return nil
				}
				_, hi, errRight := parseRangeEndpointBytes(right, opts)
				if errRight != nil {
					return nil
				}
				if err := set.Add(lo, hi); err != nil {
					return err
				}
				progress.RangesAccepted++
				if opts.Stats != nil {
					opts.Stats.RangesAccepted++
				}
				notifyProgress("read", false)
				return nil
			}
		}

		lo, hi, err := parseRangeEndpointBytes(trimmed, opts)
		if err == nil {
			if err := set.Add(lo, hi); err != nil {
				return err
			}
			progress.RangesAccepted++
			if opts.Stats != nil {
				opts.Stats.RangesAccepted++
			}
			notifyProgress("read", false)
			return nil
		}

		if looksLikeHostnameBytes(trimmed) {
			hostnames = append(hostnames, hostnameRequest{host: string(trimmed)})
			progress.HostnamesQueued++
			if opts.Stats != nil {
				opts.Stats.HostnamesQueued++
			}
			notifyProgress("read", false)
			return nil
		}

		// Unparseable line: silently skip (see ParseReader doc above
		// for the rationale). The engine will detect a fully-empty
		// result and surface it as `last_status: empty`.
		notifyProgress("read", false)
		return nil
	}); err != nil {
		return nil, err
	}
	notifyProgress("read", true)

	if len(hostnames) > 0 {
		// DNS resolution failures are non-fatal: a single dead host
		// in a feed must not cause every other parsed IP to be
		// thrown away. ResolveHostnames returns whatever it managed
		// to resolve along with the error; we use the partial
		// result and discard the error.
		notifyProgress("resolve", true)
		resolved, _ := ResolveHostnamesWithProgress(ctx, hostnamesToStrings(hostnames), opts.DNSThreads, opts.Resolver, func(done, resolved int64) {
			progress.HostnamesCompleted = done
			progress.HostnamesResolved = resolved
			progress.HostnamesQueued = int64(len(hostnames))
			if opts.Stats != nil {
				opts.Stats.HostnamesCompleted = done
				opts.Stats.HostnamesResolved = resolved
				opts.Stats.HostnamesQueued = int64(len(hostnames))
			}
			notifyProgress("resolve", true)
		})
		progress.HostnamesCompleted = int64(len(hostnames))
		progress.HostnamesResolved = int64(len(resolved))
		if opts.Stats != nil {
			opts.Stats.HostnamesCompleted = int64(len(hostnames))
			opts.Stats.HostnamesResolved = int64(len(resolved))
			opts.Stats.HostnamesQueued = int64(len(hostnames))
		}
		notifyProgress("resolve", true)
		for _, ip := range resolved {
			if err := set.Add(ip, ip); err != nil {
				return nil, err
			}
		}
	}

	return set, nil
}

func LoadPath(ctx context.Context, path string, opts ParseOptions) (*IPSet, error) {
	if path == "" || path == "-" {
		return ParseReader(ctx, DefaultName, os.Stdin, opts)
	}
	f, err := os.Open(path) // nosemgrep: exported local parser API; callers intentionally provide the file path.
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	if opts.RangeCapacityHint <= 0 {
		if info, statErr := f.Stat(); statErr == nil {
			opts.RangeCapacityHint = EstimateRangeCapacityHint(info.Size(), FamilyIPv4)
		}
	}
	return ParseReader(ctx, path, f, opts)
}

func stripInlineComment(line string) string {
	line = strings.ReplaceAll(line, "\r", "")
	if idx := strings.IndexAny(line, "#;"); idx >= 0 {
		line = line[:idx]
	}
	return strings.TrimSpace(line)
}

func trimBOMBytes(line []byte) []byte {
	if len(line) >= 3 && line[0] == 0xEF && line[1] == 0xBB && line[2] == 0xBF {
		return line[3:]
	}
	return line
}

func stripInlineCommentBytes(line []byte) []byte {
	if idx := bytes.IndexAny(line, "#;"); idx >= 0 {
		line = line[:idx]
	}
	if idx := bytes.IndexByte(line, '\r'); idx >= 0 {
		write := idx
		for read := idx + 1; read < len(line); read++ {
			if line[read] == '\r' {
				continue
			}
			line[write] = line[read]
			write++
		}
		line = line[:write]
	}
	return bytes.TrimSpace(line)
}

func splitRangeLine(line string) (string, string, bool) {
	parts := strings.SplitN(line, "-", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	left := strings.TrimSpace(parts[0])
	right := strings.TrimSpace(parts[1])
	if left == "" || right == "" {
		return "", "", false
	}
	return left, right, true
}

func splitRangeLineBytes(line []byte) ([]byte, []byte, bool) {
	idx := bytes.IndexByte(line, '-')
	if idx < 0 {
		return nil, nil, false
	}
	left := bytes.TrimSpace(line[:idx])
	right := bytes.TrimSpace(line[idx+1:])
	if len(left) == 0 || len(right) == 0 {
		return nil, nil, false
	}
	return left, right, true
}

func parseRangeEndpoint(token string, opts ParseOptions) (uint32, uint32, error) {
	if strings.Contains(token, "/") {
		parts := strings.SplitN(token, "/", 2)
		addr, err := ParseIPv4Token(parts[0])
		if err != nil {
			return 0, 0, err
		}
		prefix, err := ParsePrefix(parts[1])
		if err != nil {
			return 0, 0, err
		}
		lo := addr
		if opts.UseCIDRNetwork {
			lo = Network(addr, prefix)
		}
		return lo, Broadcast(lo, prefix), nil
	}

	addr, err := ParseIPv4Token(token)
	if err != nil {
		return 0, 0, err
	}
	if opts.DefaultPrefix == 32 {
		return addr, addr, nil
	}
	lo := addr
	if opts.UseCIDRNetwork {
		lo = Network(addr, opts.DefaultPrefix)
	}
	return lo, Broadcast(lo, opts.DefaultPrefix), nil
}

func parseRangeEndpointBytes(token []byte, opts ParseOptions) (uint32, uint32, error) {
	if idx := bytes.IndexByte(token, '/'); idx >= 0 {
		addr, err := parseIPv4TokenBytes(token[:idx])
		if err != nil {
			return 0, 0, err
		}
		prefix, err := parsePrefixBytes(token[idx+1:])
		if err != nil {
			return 0, 0, err
		}
		lo := addr
		if opts.UseCIDRNetwork {
			lo = Network(addr, prefix)
		}
		return lo, Broadcast(lo, prefix), nil
	}

	addr, err := parseIPv4TokenBytes(token)
	if err != nil {
		return 0, 0, err
	}
	if opts.DefaultPrefix == 32 {
		return addr, addr, nil
	}
	lo := addr
	if opts.UseCIDRNetwork {
		lo = Network(addr, opts.DefaultPrefix)
	}
	return lo, Broadcast(lo, opts.DefaultPrefix), nil
}

func parseIPv4TokenBytes(token []byte) (uint32, error) {
	trimmed := bytes.TrimSpace(token)
	if len(trimmed) == 0 {
		return 0, ErrInvalidIPv4
	}

	var values [4]uint64
	parts := 0
	start := 0
	for i := 0; i <= len(trimmed); i++ {
		if i != len(trimmed) && trimmed[i] != '.' {
			continue
		}
		if i == start || parts == len(values) {
			return 0, ErrInvalidIPv4
		}
		v, ok := parseIPv4DecimalPartBytes(trimmed[start:i])
		if !ok {
			return ParseIPv4Token(string(trimmed))
		}
		values[parts] = v
		parts++
		start = i + 1
	}
	return ipv4FromParts(values, parts)
}

func parseIPv4DecimalPartBytes(part []byte) (uint64, bool) {
	if len(part) == 0 {
		return 0, false
	}
	if len(part) > 1 && part[0] == '0' {
		return 0, false
	}
	var v uint64
	for _, ch := range part {
		if ch < '0' || ch > '9' {
			return 0, false
		}
		v = v*10 + uint64(ch-'0')
		if v > math.MaxUint32 {
			return 0, false
		}
	}
	return v, true
}

func ipv4FromParts(values [4]uint64, parts int) (uint32, error) {
	switch parts {
	case 1:
		return uint32(values[0]), nil
	case 2:
		if values[0] > 0xff || values[1] > 0xffffff {
			return 0, ErrInvalidIPv4
		}
		return uint32(values[0]<<24 | values[1]), nil
	case 3:
		if values[0] > 0xff || values[1] > 0xff || values[2] > 0xffff {
			return 0, ErrInvalidIPv4
		}
		return uint32(values[0]<<24 | values[1]<<16 | values[2]), nil
	case 4:
		for _, v := range values {
			if v > 0xff {
				return 0, ErrInvalidIPv4
			}
		}
		return uint32(values[0]<<24 | values[1]<<16 | values[2]<<8 | values[3]), nil
	default:
		return 0, ErrInvalidIPv4
	}
}

func parsePrefixBytes(token []byte) (int, error) {
	trimmed := bytes.TrimSpace(token)
	if len(trimmed) == 0 {
		return 0, ErrInvalidPrefix
	}
	n := 0
	for _, ch := range trimmed {
		if ch < '0' || ch > '9' {
			return ParsePrefix(string(trimmed))
		}
		n = n*10 + int(ch-'0')
	}
	if n > 32 {
		return 0, ErrInvalidPrefix
	}
	return n, nil
}

// looksLikeHostname returns true when `line` is plausibly a DNS name
// the parser should resolve. It REQUIRES at least one dot — without
// the dot rule, single-word tokens that survive everything else
// (CSV header cells like "IP", "ENABLED", "total") get sent to DNS
// resolution and fail with `no such host`. The dot is the cheapest
// way to distinguish "real hostname" from "stray identifier".
func looksLikeHostname(line string) bool {
	if line == "" {
		return false
	}
	hasDot := false
	for _, r := range line {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-', r == '_':
		case r == '.':
			hasDot = true
		default:
			return false
		}
	}
	return hasDot
}

func looksLikeHostnameBytes(line []byte) bool {
	if len(line) == 0 {
		return false
	}
	hasDot := false
	for _, r := range line {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-', r == '_':
		case r == '.':
			hasDot = true
		default:
			return false
		}
	}
	return hasDot
}

func hostnamesToStrings(in []hostnameRequest) []string {
	out := make([]string, 0, len(in))
	for _, entry := range in {
		out = append(out, entry.host)
	}
	return out
}
