package iprange

import (
	"fmt"
	"io"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

type PrintOptions6 struct {
	Format          PrintFormat
	PrefixesEnabled [129]bool
	PrintPrefixIPs  string
	PrintPrefixNets string
	PrintSuffixIPs  string
	PrintSuffixNets string
}

func DefaultPrintOptions6() PrintOptions6 {
	var enabled [129]bool
	for i := range enabled {
		enabled[i] = true
	}
	return PrintOptions6{Format: PrintCIDR, PrefixesEnabled: enabled}
}

func (s *IPSet6) Write6(w io.Writer, opts PrintOptions6) error {
	s.Optimize()
	if opts.Format == PrintBinary {
		return WriteBinary6(w, s)
	}
	started := time.Now()
	defer func() {
		iprangeObserve(iprangeBackground(), "iprange.save.text", 1, int64(s.Lines), time.Since(started), attribute.String("ip.version", "6"))
	}()

	switch opts.Format {
	case PrintCIDR:
		for _, r := range s.Ranges {
			if err := splitRange6(uint128Zero, 0, r.Lo, r.Hi, opts.PrefixesEnabled, func(addr Uint128, prefix int) error {
				if prefix < 128 {
					_, err := fmt.Fprintf(w, "%s%s/%d%s\n", opts.PrintPrefixNets, Uint128ToIPv6(addr), prefix, opts.PrintSuffixNets)
					return err
				}
				_, err := fmt.Fprintf(w, "%s%s%s\n", opts.PrintPrefixIPs, Uint128ToIPv6(addr), opts.PrintSuffixIPs)
				return err
			}); err != nil {
				return err
			}
		}
	case PrintRanges:
		for _, r := range s.Ranges {
			if r.Lo.Equals(r.Hi) {
				if _, err := fmt.Fprintf(w, "%s%s%s\n", opts.PrintPrefixIPs, Uint128ToIPv6(r.Lo), opts.PrintSuffixIPs); err != nil {
					return err
				}
				continue
			}
			if _, err := fmt.Fprintf(w, "%s%s-%s%s\n", opts.PrintPrefixNets, Uint128ToIPv6(r.Lo), Uint128ToIPv6(r.Hi), opts.PrintSuffixNets); err != nil {
				return err
			}
		}
	case PrintSingleIPs:
		limit := u128FromUint64(256 * 256 * 256)
		for _, r := range s.Ranges {
			if r.Size().GreaterThan(limit) {
				return ErrSingleIPsRangeTooBig
			}
			for ip := r.Lo; !ip.GreaterThan(r.Hi); ip = ip.Incr() {
				if _, err := fmt.Fprintf(w, "%s%s%s\n", opts.PrintPrefixIPs, Uint128ToIPv6(ip), opts.PrintSuffixIPs); err != nil {
					return err
				}
			}
		}
	default:
		return fmt.Errorf("unsupported print format")
	}

	return nil
}

func Reduce6(set *IPSet6, acceptableIncreasePercent int, minAcceptedEntries int, prefixes [129]bool) [129]bool {
	set.Optimize()
	counters := countPrefixes6(set, prefixes)

	total := 0
	for i := 0; i <= 128; i++ {
		if counters[i] > 0 {
			total += counters[i]
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
		for i := 0; i <= 127; i++ {
			if counters[i] == 0 || !prefixes[i] {
				continue
			}
			multiplier := 2
			for j := i + 1; j <= 128; j++ {
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

func countPrefixes6(set *IPSet6, enabled [129]bool) [129]int {
	var counters [129]int
	opts := DefaultPrintOptions6()
	opts.PrefixesEnabled = enabled
	for _, r := range set.Ranges {
		_ = splitRange6(uint128Zero, 0, r.Lo, r.Hi, opts.PrefixesEnabled, func(_ Uint128, prefix int) error {
			counters[prefix]++
			return nil
		})
	}
	return counters
}
