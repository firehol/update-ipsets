package markdown

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

func commaAny(v any) string {
	switch n := v.(type) {
	case uint64:
		return commaUint(n)
	case int:
		return commaInt(int64(n))
	case int64:
		return commaInt(n)
	case float64:
		return commaInt(int64(n))
	default:
		return fmt.Sprintf("%v", v)
	}
}

func commaInt(v int64) string {
	if v < 0 {
		return "-" + commaInt(-v)
	}
	s := fmt.Sprintf("%d", v)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	rem := len(s) % 3
	if rem > 0 {
		b.WriteString(s[:rem])
	}
	for i := rem; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

func commaUint(v uint64) string {
	return commaInt(int64(v))
}

func pct(v float64) string {
	return fmt.Sprintf("%.1f%%", v)
}

func dateStr(unixMs int64) string {
	if unixMs == 0 {
		return ""
	}
	return time.UnixMilli(unixMs).UTC().Format("2006-01-02 15:04 MST")
}

func relTime(unixMs int64) string {
	if unixMs == 0 {
		return ""
	}
	t := time.UnixMilli(unixMs)
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo ago", int(d.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%dy ago", int(d.Hours()/(24*365)))
	}
}

func barFill(pctVal float64, width int) string {
	if width < 1 {
		width = 10
	}
	if pctVal < 0 {
		pctVal = 0
	}
	if pctVal > 100 {
		pctVal = 100
	}
	filled := int(pctVal / 100 * float64(width))
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

func minsToDuration(mins int) string {
	if mins <= 0 {
		return ""
	}
	d := time.Duration(mins) * time.Minute
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", mins)
	case d < 24*time.Hour:
		h := d.Hours()
		if h == float64(int(h)) {
			return fmt.Sprintf("%dh", int(h))
		}
		return fmt.Sprintf("%.1fh", h)
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 4 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func sortedKeys(m map[string][]FeedInEntity) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
