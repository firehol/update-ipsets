package engine

import (
	"math"
	"sort"

	"github.com/firehol/update-ipsets/pkg/cache"
)

func (e *Engine) refreshRotationStatsFromLedger(name string, entry *cache.Entry) bool {
	return e.refreshRotationStatsFromLedgerWithSnapshot(e.operationSnapshot(), name, entry)
}

func (e *Engine) refreshRotationStatsFromLedgerWithRuntime(rt Runtime, name string, entry *cache.Entry) bool {
	return e.refreshRotationStatsFromLedgerWithSnapshot(operationSnapshot{runtime: rt}, name, entry)
}

func (e *Engine) refreshRotationStatsFromLedgerWithSnapshot(snap operationSnapshot, name string, entry *cache.Entry) bool {
	if e == nil || entry == nil {
		return false
	}
	sizes := e.readSizeSeriesWithSnapshot(snap, name)
	if len(sizes) == 0 {
		clearRotationStats(entry)
		return false
	}
	churn := e.readChurnSeriesWithSnapshot(snap, name, sizes)
	if len(churn) == 0 {
		clearRotationStats(entry)
		return false
	}
	turnoverRatios := make([]float64, 0, len(churn))
	changeRatios := make([]float64, 0, len(churn))
	for _, point := range churn {
		if point.Size == 0 {
			continue
		}
		totalChanged := float64(point.Added) + float64(point.Removed)
		turnoverRatios = append(turnoverRatios, (totalChanged/float64(point.Size))*100)
		union := float64(point.Kept) + totalChanged
		if union > 0 {
			changeRatios = append(changeRatios, (totalChanged/union)*100)
		}
	}
	if len(turnoverRatios) == 0 {
		clearRotationStats(entry)
		return false
	}
	sort.Float64s(turnoverRatios)
	stats := cache.RotationStatsSnapshot{
		RotationMedianPct: roundPct(percentile(turnoverRatios, 50)),
		RotationP75Pct:    roundPct(percentile(turnoverRatios, 75)),
		RotationSamples:   len(turnoverRatios),
	}
	if len(changeRatios) == 0 {
		entry.ApplyRotationStats(stats)
		return true
	}
	sort.Float64s(changeRatios)
	stats.ChangeRatioMedianPct = roundPct(percentile(changeRatios, 50))
	stats.ChangeRatioP75Pct = roundPct(percentile(changeRatios, 75))
	stats.ChangeRatioSamples = len(changeRatios)
	entry.ApplyRotationStats(stats)
	return true
}

func clearRotationStats(entry *cache.Entry) {
	entry.ClearRotationStats()
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	rank := (p / 100) * float64(len(sorted)-1)
	lo := int(math.Floor(rank))
	hi := int(math.Ceil(rank))
	if lo == hi {
		return sorted[lo]
	}
	weight := rank - float64(lo)
	return sorted[lo] + (sorted[hi]-sorted[lo])*weight
}

func roundPct(v float64) float64 {
	return math.Round(v*100) / 100
}
