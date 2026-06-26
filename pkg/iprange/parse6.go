package iprange

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/netip"
	"os"
	"unsafe"
)

func DefaultParseOptions6() ParseOptions {
	opts := DefaultParseOptions()
	opts.DefaultPrefix = 128
	return opts
}

func ParseReader6(ctx context.Context, name string, r io.Reader, opts ParseOptions) (*IPSet6, error) {
	if opts.DefaultPrefix < 0 || opts.DefaultPrefix > 128 {
		return nil, ErrInvalidPrefix
	}
	if opts.DNSThreads <= 0 {
		opts.DNSThreads = 1
	}
	if opts.Resolver == nil {
		opts.Resolver = DefaultResolver{}
	}
	capacityHint := parseRangeCapacityHint(opts.RangeCapacityHint, r, FamilyIPv6)

	br := bufio.NewReaderSize(r, 64*1024)
	header6, err := br.Peek(len(BinaryHeaderV20IPv6))
	if err == nil && string(header6) == BinaryHeaderV20IPv6 {
		set, stats, err := ReadBinary6WithStats(name, br)
		if opts.Stats != nil {
			opts.Stats.Add(stats)
		}
		return set, err
	}
	header4, err := br.Peek(len(BinaryHeaderV10))
	if err == nil && string(header4) == BinaryHeaderV10 {
		return nil, fmt.Errorf("%s: IPv4 binary set loaded in IPv6 mode", name)
	}

	set := New6(name)
	if capacityHint > 0 {
		set.Ranges = make([]Range6, 0, capacityHint)
	}
	var hostnames []hostnameRequest
	firstLine := true

	if err := forEachTextLineBytes(br, func(line []byte) error {
		if opts.Stats != nil {
			opts.Stats.LinesRead++
			opts.Stats.BytesRead += int64(len(line))
		}
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
				lo, _, errLeft := parseIPv6OrMappedEndpointBytes(left, opts)
				if errLeft != nil {
					return nil
				}
				_, hi, errRight := parseIPv6OrMappedEndpointBytes(right, opts)
				if errRight != nil {
					return nil
				}
				if err := set.Add6(lo, hi); err != nil {
					return err
				}
				if opts.Stats != nil {
					opts.Stats.RangesAccepted++
				}
				return nil
			}
		}

		lo, hi, err := parseIPv6OrMappedEndpointBytes(trimmed, opts)
		if err == nil {
			if err := set.Add6(lo, hi); err != nil {
				return err
			}
			if opts.Stats != nil {
				opts.Stats.RangesAccepted++
			}
			return nil
		}

		if looksLikeHostnameBytes(trimmed) {
			hostnames = append(hostnames, hostnameRequest{host: string(trimmed)})
			if opts.Stats != nil {
				opts.Stats.HostnamesQueued++
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if len(hostnames) > 0 {
		names := hostnamesToStrings(hostnames)

		resolved4, _ := ResolveHostnames(ctx, names, opts.DNSThreads, opts.Resolver)
		for _, ip := range resolved4 {
			mapped := IPv4ToMapped6(ip)
			if err := set.Add6(mapped, mapped); err != nil {
				return nil, err
			}
		}

		resolved6, _ := ResolveHostnames6(ctx, names, opts.DNSThreads)
		for _, ip := range resolved6 {
			if err := set.Add6(ip, ip); err != nil {
				return nil, err
			}
		}
		if opts.Stats != nil {
			opts.Stats.HostnamesCompleted = int64(len(hostnames))
			opts.Stats.HostnamesResolved = int64(len(resolved4) + len(resolved6))
		}
	}

	return set, nil
}

func LoadPath6(ctx context.Context, path string, opts ParseOptions) (*IPSet6, error) {
	if path == "" || path == "-" {
		return ParseReader6(ctx, DefaultName, os.Stdin, opts)
	}
	f, err := os.Open(path) // nosemgrep: exported local parser API; callers intentionally provide the file path.
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	if opts.RangeCapacityHint <= 0 {
		if info, statErr := f.Stat(); statErr == nil {
			opts.RangeCapacityHint = EstimateRangeCapacityHint(info.Size(), FamilyIPv6)
		}
	}
	return ParseReader6(ctx, path, f, opts)
}

func parseIPv6OrMappedEndpointBytes(token []byte, opts ParseOptions) (Uint128, Uint128, error) {
	trimmed := bytes.TrimSpace(token)
	if looksLikeIPv6Bytes(trimmed) {
		return parseIPv6EndpointBytes(trimmed, opts)
	}

	if idx := bytes.IndexByte(trimmed, '/'); idx >= 0 {
		addr, err := parseIPv4TokenBytes(trimmed[:idx])
		if err == nil {
			prefix, pErr := parsePrefixBytes(trimmed[idx+1:])
			if pErr == nil && prefix >= 0 && prefix <= 32 {
				lo := IPv4ToMapped6(Network(addr, prefix))
				hi := IPv4ToMapped6(Broadcast(addr, prefix))
				return lo, hi, nil
			}
		}
	}

	addr, err := parseIPv4TokenBytes(trimmed)
	if err == nil {
		mapped := IPv4ToMapped6(addr)
		if opts.DefaultPrefix < 0 || opts.DefaultPrefix > 128 {
			return uint128Zero, uint128Zero, ErrInvalidPrefix
		}
		if opts.DefaultPrefix >= 32 {
			return mapped, mapped, nil
		}
		lo := addr
		if opts.UseCIDRNetwork {
			lo = Network(addr, opts.DefaultPrefix)
		}
		return IPv4ToMapped6(lo), IPv4ToMapped6(Broadcast(lo, opts.DefaultPrefix)), nil
	}

	return parseIPv6EndpointBytes(trimmed, opts)
}

func parseIPv6EndpointBytes(token []byte, opts ParseOptions) (Uint128, Uint128, error) {
	if idx := bytes.IndexByte(token, '/'); idx >= 0 {
		addr, err := parseIPv6TokenBytes(token[:idx])
		if err != nil {
			return uint128Zero, uint128Zero, err
		}
		prefix, err := parsePrefix6Bytes(token[idx+1:])
		if err != nil {
			return uint128Zero, uint128Zero, err
		}
		lo := addr
		if opts.UseCIDRNetwork {
			lo = Network6(addr, prefix)
		}
		return lo, Broadcast6(lo, prefix), nil
	}

	addr, err := parseIPv6TokenBytes(token)
	if err != nil {
		return uint128Zero, uint128Zero, err
	}
	if opts.DefaultPrefix == 128 {
		return addr, addr, nil
	}
	lo := addr
	if opts.UseCIDRNetwork {
		lo = Network6(addr, opts.DefaultPrefix)
	}
	return lo, Broadcast6(lo, opts.DefaultPrefix), nil
}

func parseIPv6TokenBytes(token []byte) (Uint128, error) {
	trimmed := bytes.TrimSpace(token)
	if len(trimmed) == 0 {
		return uint128Zero, ErrInvalidIPv6
	}
	addr, err := netip.ParseAddr(bytesToUnsafeString(trimmed))
	if err != nil {
		return uint128Zero, fmt.Errorf("%w: %q", ErrInvalidIPv6, string(trimmed))
	}
	bytes := addr.As16()
	return u128FromBytes(bytes[:]), nil
}

func parsePrefix6Bytes(token []byte) (int, error) {
	trimmed := bytes.TrimSpace(token)
	if len(trimmed) == 0 {
		return 0, ErrInvalidPrefix
	}
	n := 0
	for _, ch := range trimmed {
		if ch < '0' || ch > '9' {
			return ParsePrefix6(string(trimmed))
		}
		n = n*10 + int(ch-'0')
		if n > 128 {
			return 0, ErrInvalidPrefix
		}
	}
	return n, nil
}

func looksLikeIPv6Bytes(token []byte) bool {
	return bytes.IndexByte(token, ':') >= 0
}

func bytesToUnsafeString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(unsafe.SliceData(b), len(b))
}
