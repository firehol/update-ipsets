package insights

import (
	"fmt"
	"strings"
)

// formatDuration renders an hour count as a short human-readable
// duration using up to two units. Examples:
//
//	formatDuration(3)     == "3 h"
//	formatDuration(79)    == "3 d 7 h"
//	formatDuration(750)   == "1 mo 1 d"
//	formatDuration(72456) == "8 yr 3 mo"
//
// The function favors readability over precision: it picks the two
// largest populated units and discards the rest. For insights headlines
// this is the right tradeoff because the numbers are already bucketed
// into hours.
func formatDuration(hours int) string {
	if hours <= 0 {
		return "0 h"
	}
	const (
		hoursPerDay   = 24
		hoursPerMonth = 24 * 30 // calendar-agnostic approximation
		hoursPerYear  = 24 * 365
	)

	years := hours / hoursPerYear
	rem := hours - years*hoursPerYear
	months := rem / hoursPerMonth
	rem -= months * hoursPerMonth
	days := rem / hoursPerDay
	remHours := rem - days*hoursPerDay

	parts := make([]string, 0, 2)
	add := func(value int, unit string) {
		if value == 0 {
			return
		}
		if len(parts) >= 2 {
			return
		}
		parts = append(parts, fmt.Sprintf("%d %s", value, unit))
	}
	add(years, "yr")
	add(months, "mo")
	add(days, "d")
	add(remHours, "h")
	if len(parts) == 0 {
		return "0 h"
	}
	return strings.Join(parts, " ")
}

// formatCount renders a large IP count using short SI-style suffixes
// (1.2M, 12.4M, 3.1K). Used by headlines that show min/max ranges so
// they do not turn into walls of digits.
func formatCount(n uint64) string {
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.1fB", float64(n)/1_000_000_000)
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 10_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// formatPercent renders a 0..1 fraction as a percentage with adaptive
// precision. Very small shares keep their non-zero digits so headlines
// never show "0%" when the underlying value is 0.0001.
func formatPercent(frac float64) string {
	pct := frac * 100
	switch {
	case pct >= 10:
		return fmt.Sprintf("%.0f%%", pct)
	case pct >= 1:
		return fmt.Sprintf("%.1f%%", pct)
	case pct >= 0.01:
		return fmt.Sprintf("%.2f%%", pct)
	case pct > 0:
		return fmt.Sprintf("%.4f%%", pct)
	default:
		return "0%"
	}
}
