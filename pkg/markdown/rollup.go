package markdown

import (
	"fmt"
	"sort"
	"time"
)

type CadenceBin struct {
	Interval string
	Count    int
}

type RollupResult struct {
	Interval  string
	Rows      []ActivityRow
	Cadence   []CadenceBin
	TotalIPs  uint64
	LatestIPs uint64
}

type HistoryPoint struct {
	Timestamp int64
	Entries   int
	IPs       uint64
}

type ChangesetPoint struct {
	Timestamp int64
	Added     uint64
	Removed   uint64
}

func Rollup(history []HistoryPoint, changesets []ChangesetPoint) RollupResult {
	activity := BuildActivity(history, changesets, 0)
	return RollupResult{
		Interval:  activity.Resolution,
		Rows:      activity.Rows,
		Cadence:   computeCadenceFromHistory(history),
		LatestIPs: activity.LatestIPs,
	}
}

func BuildActivity(history []HistoryPoint, changesets []ChangesetPoint, frequencyMins int) ActivityContext {
	if len(history) == 0 {
		return ActivityContext{}
	}

	var latestIPs uint64
	if len(history) > 0 {
		latestIPs = history[len(history)-1].IPs
	}

	ctx := ActivityContext{
		LatestIPs:      latestIPs,
		RawChanges:     len(changesets),
		HistorySamples: len(history),
		WindowNote:     "Rows describe content-changing updates in the published changeset artifact. The artifact is bounded to the retained public window, so it is not a full lifetime ledger and must not be used to infer first publication. Use Tracked since for monitoring start.",
	}

	if len(changesets) == 0 {
		ctx.Resolution = "observed update"
		ctx.WindowNote = "No content-changing updates are available in the published changeset artifact. This can happen for young feeds, feeds that have not changed inside the retained window, or feeds whose older changes are outside the public artifact."
		return ctx
	}

	if len(changesets) <= 100 {
		ctx.Resolution = "observed update"
		ctx.Rows = buildRawActivityRows(history, changesets)
		return ctx
	}

	interval := chooseActivityInterval(history, changesets, frequencyMins, 100)
	ctx.Rollup = true
	ctx.Resolution = interval.name
	ctx.Rows = bucketActivityRows(history, changesets, interval.dur)
	ctx.RollupNote = fmt.Sprintf(
		"Rows are bucketed by %s because there are %d retained changes. The bucket is not smaller than the feed's expected or observed cadence. Added and removed are sums per bucket; IPs and entries are average observed sizes in that bucket.",
		interval.name,
		len(changesets),
	)
	return ctx
}

func buildRawActivityRows(history []HistoryPoint, changesets []ChangesetPoint) []ActivityRow {
	rows := make([]ActivityRow, 0, len(changesets))
	for _, c := range sortedChangesets(changesets) {
		ips, entries := sizeAtOrBefore(history, c.Timestamp)
		row := ActivityRow{
			Timestamp: time.Unix(c.Timestamp, 0).UTC(),
			IPs:       float64(ips),
			Entries:   float64(entries),
			Added:     float64(c.Added),
			Removed:   float64(c.Removed),
		}
		if ips > 0 {
			row.ChurnPct = float64(c.Added+c.Removed) / float64(ips) * 100
		}
		rows = append(rows, row)
	}
	return rows
}

type activityInterval struct {
	name string
	dur  time.Duration
}

func chooseActivityInterval(history []HistoryPoint, changesets []ChangesetPoint, frequencyMins, maxRows int) activityInterval {
	if maxRows < 1 {
		maxRows = 100
	}
	floor := cadenceFloor(history, frequencyMins)
	intervals := []activityInterval{
		{"1h", time.Hour},
		{"1d", 24 * time.Hour},
		{"1w", 7 * 24 * time.Hour},
		{"1mo", 30 * 24 * time.Hour},
		{"3mo", 90 * 24 * time.Hour},
		{"1y", 365 * 24 * time.Hour},
	}
	for _, iv := range intervals {
		if floor > 0 && iv.dur < floor {
			continue
		}
		if len(bucketActivityRows(history, changesets, iv.dur)) <= maxRows {
			return iv
		}
	}
	return intervals[len(intervals)-1]
}

func cadenceFloor(history []HistoryPoint, frequencyMins int) time.Duration {
	if frequencyMins > 0 {
		return time.Duration(frequencyMins) * time.Minute
	}
	gaps := historyGaps(history)
	if len(gaps) == 0 {
		return 0
	}
	sort.Slice(gaps, func(i, j int) bool { return gaps[i] < gaps[j] })
	return gaps[len(gaps)/2]
}

func bucketActivityRows(history []HistoryPoint, changesets []ChangesetPoint, interval time.Duration) []ActivityRow {
	if len(changesets) == 0 {
		return nil
	}

	type bucket struct {
		count   int
		sumIPs  float64
		sumEnts float64
		sumAdd  float64
		sumRem  float64
		start   time.Time
	}

	buckets := make(map[int64]*bucket)
	var keys []int64
	for _, c := range sortedChangesets(changesets) {
		ts := time.Unix(c.Timestamp, 0).UTC()
		key := ts.Truncate(interval).Unix()
		b, ok := buckets[key]
		if !ok {
			b = &bucket{start: ts.Truncate(interval)}
			buckets[key] = b
			keys = append(keys, key)
		}
		ips, entries := sizeAtOrBefore(history, c.Timestamp)
		b.count++
		b.sumIPs += float64(ips)
		b.sumEnts += float64(entries)
		b.sumAdd += float64(c.Added)
		b.sumRem += float64(c.Removed)
	}

	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	rows := make([]ActivityRow, 0, len(keys))
	for _, k := range keys {
		b := buckets[k]
		avgIPs := b.sumIPs / float64(b.count)
		avgEnts := b.sumEnts / float64(b.count)
		var churn float64
		if avgIPs > 0 {
			churn = (b.sumAdd + b.sumRem) / avgIPs * 100
		}
		rows = append(rows, ActivityRow{
			Timestamp: b.start,
			IPs:       avgIPs,
			Entries:   avgEnts,
			Added:     b.sumAdd,
			Removed:   b.sumRem,
			ChurnPct:  churn,
		})
	}
	return rows
}

func sizeAtOrBefore(history []HistoryPoint, ts int64) (uint64, int) {
	if len(history) == 0 {
		return 0, 0
	}
	sorted := append([]HistoryPoint(nil), history...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Timestamp < sorted[j].Timestamp })
	best := -1
	for i, h := range sorted {
		if h.Timestamp <= ts {
			best = i
			continue
		}
		break
	}
	if best < 0 {
		best = 0
	}
	return sorted[best].IPs, sorted[best].Entries
}

func sortedChangesets(changesets []ChangesetPoint) []ChangesetPoint {
	out := append([]ChangesetPoint(nil), changesets...)
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp < out[j].Timestamp })
	return out
}

func historyGaps(history []HistoryPoint) []time.Duration {
	if len(history) < 2 {
		return nil
	}
	sorted := append([]HistoryPoint(nil), history...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Timestamp < sorted[j].Timestamp })
	gaps := make([]time.Duration, 0, len(sorted)-1)
	var prev int64
	for _, h := range sorted {
		if prev > 0 {
			gap := time.Duration(h.Timestamp-prev) * time.Second
			if gap > 0 {
				gaps = append(gaps, gap)
			}
		}
		prev = h.Timestamp
	}
	return gaps
}

func computeCadenceFromHistory(history []HistoryPoint) []CadenceBin {
	gaps := historyGaps(history)
	if len(gaps) == 0 {
		return nil
	}

	return cadenceBins(gaps)
}

func cadenceBins(gaps []time.Duration) []CadenceBin {
	bins := []struct {
		label string
		min   time.Duration
		max   time.Duration
	}{
		{"<1h", 0, time.Hour},
		{"1-6h", time.Hour, 6 * time.Hour},
		{"6-12h", 6 * time.Hour, 12 * time.Hour},
		{"12-24h", 12 * time.Hour, 24 * time.Hour},
		{"1-3d", 24 * time.Hour, 3 * 24 * time.Hour},
		{"3-7d", 3 * 24 * time.Hour, 7 * 24 * time.Hour},
		{"1-4w", 7 * 24 * time.Hour, 4 * 7 * 24 * time.Hour},
		{">4w", 4 * 7 * 24 * time.Hour, time.Duration(1<<63 - 1)},
	}

	result := make([]CadenceBin, 0, len(bins))
	for _, b := range bins {
		count := 0
		for _, g := range gaps {
			if g >= b.min && g < b.max {
				count++
			}
		}
		if count > 0 {
			result = append(result, CadenceBin{Interval: b.label, Count: count})
		}
	}
	return result
}
