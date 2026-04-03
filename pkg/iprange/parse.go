package iprange

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

type ParseOptions struct {
	DefaultPrefix  int
	UseCIDRNetwork bool
	DNSThreads     int
	Resolver       Resolver
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
	started := time.Now()
	ctx, span := iprangeStart(ctx, "iprange.load.text", attribute.String("ip.version", "4"), attribute.String("iprange.name", name))
	var opErr error
	textLoad := true
	defer func() {
		if textLoad {
			iprangeObserve(ctx, "iprange.load.text", 1, 0, time.Since(started), attribute.String("ip.version", "4"))
		}
		iprangeEnd(span, opErr)
	}()
	if opts.DefaultPrefix < 0 || opts.DefaultPrefix > 32 {
		opErr = ErrInvalidPrefix
		return nil, ErrInvalidPrefix
	}
	if opts.DNSThreads <= 0 {
		opts.DNSThreads = 1
	}
	if opts.Resolver == nil {
		opts.Resolver = DefaultResolver{}
	}

	// Use a buffered reader so we can peek at the first bytes to detect
	// the binary format without buffering the entire input.
	br := bufio.NewReaderSize(r, 64*1024)
	header, err := br.Peek(len(BinaryHeaderV10))
	if err == nil && string(header) == BinaryHeaderV10 {
		textLoad = false
		set, err := ReadBinary(name, br)
		opErr = err
		return set, err
	}
	header6, err := br.Peek(len(BinaryHeaderV20IPv6))
	if err == nil && string(header6) == BinaryHeaderV20IPv6 {
		opErr = fmt.Errorf("%s: IPv6 binary set loaded in IPv4 mode", name)
		return nil, opErr
	}

	set := New(name)
	var hostnames []hostnameRequest
	firstLine := true

	if err := forEachTextLine(br, func(line string) error {
		// Strip UTF-8 BOM from the first line.
		if firstLine {
			firstLine = false
			line = strings.TrimPrefix(line, "\xEF\xBB\xBF")
		}
		trimmed := stripInlineComment(line)
		if trimmed == "" {
			return nil
		}

		if strings.Contains(trimmed, "-") {
			left, right, ok := splitRangeLine(trimmed)
			if ok {
				lo, _, errLeft := parseRangeEndpoint(left, opts)
				if errLeft != nil {
					// Skip the malformed range line and keep going.
					return nil
				}
				_, hi, errRight := parseRangeEndpoint(right, opts)
				if errRight != nil {
					return nil
				}
				if err := set.Add(lo, hi); err != nil {
					return err
				}
				return nil
			}
		}

		lo, hi, err := parseRangeEndpoint(trimmed, opts)
		if err == nil {
			if err := set.Add(lo, hi); err != nil {
				return err
			}
			return nil
		}

		if looksLikeHostname(trimmed) {
			hostnames = append(hostnames, hostnameRequest{host: trimmed})
			return nil
		}

		// Unparseable line: silently skip (see ParseReader doc above
		// for the rationale). The engine will detect a fully-empty
		// result and surface it as `last_status: empty`.
		return nil
	}); err != nil {
		opErr = err
		return nil, err
	}

	if len(hostnames) > 0 {
		// DNS resolution failures are non-fatal: a single dead host
		// in a feed must not cause every other parsed IP to be
		// thrown away. ResolveHostnames returns whatever it managed
		// to resolve along with the error; we use the partial
		// result and discard the error.
		resolved, _ := ResolveHostnames(ctx, hostnamesToStrings(hostnames), opts.DNSThreads, opts.Resolver)
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
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return ParseReader(ctx, path, f, opts)
}

func stripInlineComment(line string) string {
	line = strings.ReplaceAll(line, "\r", "")
	if idx := strings.IndexAny(line, "#;"); idx >= 0 {
		line = line[:idx]
	}
	return strings.TrimSpace(line)
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

func hostnamesToStrings(in []hostnameRequest) []string {
	out := make([]string, 0, len(in))
	for _, entry := range in {
		out = append(out, entry.host)
	}
	return out
}
