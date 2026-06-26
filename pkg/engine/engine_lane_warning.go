package engine

import "time"

type LaneLongHoldWarning struct {
	LaneWorkSnapshot
	WarningAt   time.Time `json:"warning_at"`
	ThresholdMS int64     `json:"threshold_ms"`
}

func (e *Engine) recordEngineLaneLongHoldWarning(work LaneWorkSnapshot, at time.Time, threshold time.Duration) {
	if e == nil {
		return
	}
	warning := &LaneLongHoldWarning{
		LaneWorkSnapshot: work,
		WarningAt:        at.UTC(),
		ThresholdMS:      threshold.Milliseconds(),
	}
	e.engineLaneLongHoldWarningMu.Lock()
	e.engineLaneLongHoldWarning = warning
	e.engineLaneLongHoldWarningMu.Unlock()
}

func (e *Engine) engineLaneLongHoldWarningSnapshot() *LaneLongHoldWarning {
	if e == nil {
		return nil
	}
	e.engineLaneLongHoldWarningMu.RLock()
	defer e.engineLaneLongHoldWarningMu.RUnlock()
	if e.engineLaneLongHoldWarning == nil {
		return nil
	}
	warning := *e.engineLaneLongHoldWarning
	return &warning
}

func (e *Engine) attachEngineLaneWarning(snapshot LaneSnapshot) LaneSnapshot {
	snapshot.LongHoldWarning = e.engineLaneLongHoldWarningSnapshot()
	return snapshot
}
