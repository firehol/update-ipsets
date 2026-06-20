package iprange

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
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
	var hostnames []hostnameRequest
	firstLine := true

	if err := forEachTextLine(br, func(line string) error {
		if opts.Stats != nil {
			opts.Stats.LinesRead++
			opts.Stats.BytesRead += int64(len(line))
		}
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
				lo, _, errLeft := parseIPv6OrMappedEndpoint(left, opts)
				if errLeft != nil {
					return nil
				}
				_, hi, errRight := parseIPv6OrMappedEndpoint(right, opts)
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

		lo, hi, err := parseIPv6OrMappedEndpoint(trimmed, opts)
		if err == nil {
			if err := set.Add6(lo, hi); err != nil {
				return err
			}
			if opts.Stats != nil {
				opts.Stats.RangesAccepted++
			}
			return nil
		}

		if looksLikeHostname(trimmed) {
			hostnames = append(hostnames, hostnameRequest{host: trimmed})
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
	return ParseReader6(ctx, path, f, opts)
}

func parseIPv6OrMappedEndpoint(token string, opts ParseOptions) (Uint128, Uint128, error) {
	if looksLikeIPv6(token) {
		return parseIPv6Endpoint(token, opts)
	}

	if strings.Contains(token, "/") {
		parts := strings.SplitN(token, "/", 2)
		addr, err := ParseIPv4Token(parts[0])
		if err == nil {
			prefix, pErr := ParsePrefix(parts[1])
			if pErr == nil && prefix >= 0 && prefix <= 32 {
				lo := IPv4ToMapped6(Network(addr, prefix))
				hi := IPv4ToMapped6(Broadcast(addr, prefix))
				return lo, hi, nil
			}
		}
	}

	addr, err := ParseIPv4Token(token)
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

	return parseIPv6Endpoint(token, opts)
}
