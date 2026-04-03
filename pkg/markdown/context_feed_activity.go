package markdown

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

func (r *FeedArtifactReader) populateBehavior(ctx *FeedPageContext, name string) {
	history := r.readHistoryCSV(name)
	publicChangesets := r.readChangesetsCSV(name)

	if len(history) == 0 {
		return
	}

	activity := BuildActivity(history, publicChangesets, ctx.Frequency)
	ctx.Activity = &activity
	ctx.Cadence = computeCadenceFromHistory(history)
}

func (r *FeedArtifactReader) readHistoryCSV(name string) []HistoryPoint {
	data, err := os.ReadFile(r.path(name + "_history.csv"))
	if err != nil {
		return nil
	}
	return parseHistoryCSV(data)
}

func (r *FeedArtifactReader) readChangesetsCSV(name string) []ChangesetPoint {
	data, err := os.ReadFile(r.path(name + "_changesets.csv"))
	if err != nil {
		return nil
	}
	return parseChangesetsCSV(data)
}

func (r *FeedArtifactReader) readRetention(name string) (*RetentionContext, error) {
	data, err := os.ReadFile(r.path(name + "_retention.json"))
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	ctx := &RetentionContext{}

	if current, ok := raw["current"].(map[string]any); ok {
		ctx.CurrentRetention = medianRetention(current)
		ctx.CurrentRetentionDays = ctx.CurrentRetention / 24
		ctx.CurrentSeries = parseRetentionSeries(current)
	}

	if past, ok := raw["past"].(map[string]any); ok {
		ctx.PastRetention = medianRetention(past)
		ctx.PastRetentionDays = ctx.PastRetention / 24
		ctx.PastSeries = parseRetentionSeries(past)
	}

	return ctx, nil
}

func (r *FeedArtifactReader) readComparison(name string, feedIPs uint64) ([]CompareRowContext, error) {
	data, err := os.ReadFile(r.path(name + "_comparison.json"))
	if err != nil {
		return nil, err
	}

	var raw []map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	var result []CompareRowContext
	for _, row := range raw {
		ips := uintVal(row["ips"])
		common := uintVal(row["common"])
		result = append(result, CompareRowContext{
			Name:         strVal(row["name"]),
			Category:     strVal(row["category"]),
			IPs:          ips,
			Common:       common,
			ThisPercent:  percentOf(common, feedIPs),
			TheirPercent: percentOf(common, ips),
			Related:      boolVal(row["related"]),
		})
	}

	return result, nil
}

func percentOf(part, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return 100 * float64(part) / float64(total)
}

func parseHistoryCSV(data []byte) []HistoryPoint {
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 {
		return nil
	}

	var result []HistoryPoint
	for _, line := range lines[1:] {
		fields := strings.Split(line, ",")
		if len(fields) < 3 {
			continue
		}
		ts, err := strconv.ParseInt(strings.TrimSpace(fields[0]), 10, 64)
		if err != nil {
			continue
		}
		entries, _ := strconv.Atoi(strings.TrimSpace(fields[1]))
		ips, _ := strconv.ParseUint(strings.TrimSpace(fields[2]), 10, 64)

		result = append(result, HistoryPoint{
			Timestamp: ts,
			Entries:   entries,
			IPs:       ips,
		})
	}
	return result
}

func parseChangesetsCSV(data []byte) []ChangesetPoint {
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 {
		return nil
	}

	var result []ChangesetPoint
	for _, line := range lines[1:] {
		fields := strings.Split(line, ",")
		if len(fields) < 3 {
			continue
		}
		ts, err := strconv.ParseInt(strings.TrimSpace(fields[0]), 10, 64)
		if err != nil {
			continue
		}
		added, _ := strconv.ParseUint(strings.TrimSpace(fields[1]), 10, 64)
		removed, _ := strconv.ParseUint(strings.TrimSpace(fields[2]), 10, 64)

		result = append(result, ChangesetPoint{
			Timestamp: ts,
			Added:     added,
			Removed:   removed,
		})
	}
	return result
}

func parseRetentionSeries(raw map[string]any) []RetentionBucket {
	hoursRaw, ok := raw["hours"].([]any)
	if !ok {
		return nil
	}
	ipsRaw, ok := raw["ips"].([]any)
	if !ok {
		return nil
	}

	n := min(len(hoursRaw), len(ipsRaw))
	if n == 0 {
		return nil
	}
	const retentionDayLimit = 365
	dayTotals := make([]uint64, retentionDayLimit)
	var older uint64

	for i := 0; i < n; i++ {
		h, ok1 := toFloat(hoursRaw[i])
		ip, ok2 := toFloat(ipsRaw[i])
		if !ok1 || !ok2 {
			continue
		}
		hours := int(h)
		ips := uint64(ip)
		day := hours/24 + 1
		if day <= 0 {
			continue
		}
		if day > retentionDayLimit {
			older += ips
			continue
		}
		dayTotals[day-1] += ips
	}

	result := make([]RetentionBucket, 0, retentionDayLimit+1)
	for i, ips := range dayTotals {
		if ips == 0 {
			continue
		}
		day := i + 1
		result = append(result, RetentionBucket{
			Day:   day,
			Label: strconv.Itoa(day),
			Hours: (day - 1) * 24,
			IPs:   ips,
		})
	}
	if older > 0 {
		result = append(result, RetentionBucket{
			Day:   retentionDayLimit + 1,
			Label: ">365 days",
			Hours: retentionDayLimit * 24,
			IPs:   older,
		})
	}
	return result
}

func medianRetention(raw map[string]any) float64 {
	hours := parseFloatSlice(raw["hours"])
	ips := parseFloatSlice(raw["ips"])
	if len(hours) == 0 || len(ips) == 0 {
		return 0
	}

	var total float64
	for _, ip := range ips {
		total += ip
	}
	if total == 0 {
		return 0
	}

	var cumulative float64
	for i, ip := range ips {
		cumulative += ip
		if cumulative >= total/2 {
			if i < len(hours) {
				return hours[i]
			}
			break
		}
	}
	return 0
}

func parseFloatSlice(v any) []float64 {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	result := make([]float64, 0, len(arr))
	for _, item := range arr {
		if f, ok := toFloat(item); ok {
			result = append(result, f)
		}
	}
	return result
}
