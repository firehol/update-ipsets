package iprange

import (
	"fmt"
	"io"
)

type PrintFormat int

const (
	PrintCIDR PrintFormat = iota
	PrintRanges
	PrintSingleIPs
	PrintBinary
)

type PrintOptions struct {
	Format          PrintFormat
	PrefixesEnabled [33]bool
	PrintPrefixIPs  string
	PrintPrefixNets string
	PrintSuffixIPs  string
	PrintSuffixNets string
}

func DefaultPrintOptions() PrintOptions {
	var enabled [33]bool
	for i := range enabled {
		enabled[i] = true
	}
	return PrintOptions{Format: PrintCIDR, PrefixesEnabled: enabled}
}

func (s *IPSet) Write(w io.Writer, opts PrintOptions) error {
	s.Optimize()
	if opts.Format == PrintBinary {
		return WriteBinary(w, s)
	}

	switch opts.Format {
	case PrintCIDR:
		for _, r := range s.Ranges {
			if err := splitRange(0, 0, r.Lo, r.Hi, opts, func(addr uint32, prefix int) error {
				if prefix < 32 {
					_, err := fmt.Fprintf(w, "%s%s/%d%s\n", opts.PrintPrefixNets, Uint32ToIPv4(addr), prefix, opts.PrintSuffixNets)
					return err
				}
				_, err := fmt.Fprintf(w, "%s%s%s\n", opts.PrintPrefixIPs, Uint32ToIPv4(addr), opts.PrintSuffixIPs)
				return err
			}); err != nil {
				return err
			}
		}
	case PrintRanges:
		for _, r := range s.Ranges {
			prefix := opts.PrintPrefixNets
			suffix := opts.PrintSuffixNets
			if r.Lo == r.Hi {
				prefix = opts.PrintPrefixIPs
				suffix = opts.PrintSuffixIPs
			}
			if _, err := fmt.Fprintf(w, "%s%s-%s%s\n", prefix, Uint32ToIPv4(r.Lo), Uint32ToIPv4(r.Hi), suffix); err != nil {
				return err
			}
		}
	case PrintSingleIPs:
		for _, r := range s.Ranges {
			if r.Size() > 256*256*256 {
				return ErrSingleIPsRangeTooBig
			}
			for ip := uint64(r.Lo); ip <= uint64(r.Hi); ip++ {
				if _, err := fmt.Fprintf(w, "%s%s%s\n", opts.PrintPrefixIPs, Uint32ToIPv4(uint32(ip)), opts.PrintSuffixIPs); err != nil {
					return err
				}
			}
		}
	default:
		return fmt.Errorf("unsupported print format")
	}

	return nil
}

func splitRange(addr uint32, prefix int, lo uint32, hi uint32, opts PrintOptions, emit func(uint32, int) error) error {
	if prefix < 0 || prefix > 32 {
		return ErrInvalidPrefix
	}
	bc := Broadcast(addr, prefix)
	if lo < addr || hi > bc {
		return fmt.Errorf("range %s-%s outside %s/%d", Uint32ToIPv4(lo), Uint32ToIPv4(hi), Uint32ToIPv4(addr), prefix)
	}

	if lo == addr && hi == bc && opts.PrefixesEnabled[prefix] {
		return emit(addr, prefix)
	}
	if prefix == 32 {
		// A /32 cannot be split any further. When callers disable host prefixes
		// (for example for net-only output), drop the singleton instead of
		// recursing to an invalid /33.
		return nil
	}

	nextPrefix := prefix + 1
	lowerHalf := addr
	upperHalf := setBit(addr, nextPrefix, true)

	switch {
	case hi < upperHalf:
		return splitRange(lowerHalf, nextPrefix, lo, hi, opts, emit)
	case lo >= upperHalf:
		return splitRange(upperHalf, nextPrefix, lo, hi, opts, emit)
	default:
		if err := splitRange(lowerHalf, nextPrefix, lo, Broadcast(lowerHalf, nextPrefix), opts, emit); err != nil {
			return err
		}
		return splitRange(upperHalf, nextPrefix, upperHalf, hi, opts, emit)
	}
}

func setBit(addr uint32, bitNo int, value bool) uint32 {
	if bitNo < 1 || bitNo > 32 {
		return addr
	}
	mask := uint32(1) << (32 - bitNo)
	if value {
		return addr | mask
	}
	return addr &^ mask
}

func Reduce(set *IPSet, acceptableIncreasePercent int, minAcceptedEntries int, prefixes [33]bool) [33]bool {
	set.Optimize()
	counters := countPrefixes(set, prefixes)

	total := 0
	initial := 0
	for i := 0; i <= 32; i++ {
		if counters[i] > 0 {
			total += counters[i]
			initial++
		} else {
			prefixes[i] = false
		}
	}

	acceptable := total * acceptableIncreasePercent / 100
	if acceptable < minAcceptedEntries {
		acceptable = minAcceptedEntries
	}

	for total < acceptable {
		minIncrease := acceptable * 10
		from, to := -1, -1
		for i := 0; i <= 31; i++ {
			if counters[i] == 0 || !prefixes[i] {
				continue
			}
			multiplier := 2
			for j := i + 1; j <= 32; j++ {
				if counters[j] == 0 {
					multiplier *= 2
					continue
				}
				increase := counters[i] * (multiplier - 1)
				if increase < minIncrease {
					minIncrease = increase
					from, to = i, j
				}
				break
			}
		}
		if from == -1 || to == -1 || from == to || total+minIncrease > acceptable {
			break
		}
		counters[to] += minIncrease + counters[from]
		counters[from] = 0
		prefixes[from] = false
		total += minIncrease
	}

	return prefixes
}

func countPrefixes(set *IPSet, enabled [33]bool) [33]int {
	var counters [33]int
	opts := DefaultPrintOptions()
	opts.PrefixesEnabled = enabled
	for _, r := range set.Ranges {
		_ = splitRange(0, 0, r.Lo, r.Hi, opts, func(_ uint32, prefix int) error {
			counters[prefix]++
			return nil
		})
	}
	return counters
}
