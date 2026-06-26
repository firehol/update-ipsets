package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/firehol/update-ipsets/internal/observability"
	"go.opentelemetry.io/otel/attribute"
)

const (
	engineLaneDiagnosticsInterval = 30 * time.Second
	engineLaneLongHoldThreshold   = 30 * time.Second
)

func (e *Engine) startEngineLaneDiagnostics(ctx context.Context) {
	if e == nil || e.engineLane == nil || e.logger == nil {
		return
	}
	ctx = nonNilContext(ctx)
	e.engineLaneDiagnosticsOnce.Do(func() {
		go e.runEngineLaneDiagnostics(ctx)
	})
}

func (e *Engine) runEngineLaneDiagnostics(ctx context.Context) {
	ticker := time.NewTicker(engineLaneDiagnosticsInterval)
	defer ticker.Stop()
	lastLogged := map[string]time.Time{}
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			e.logLongRunningEngineLaneWorkSafely(ctx, now.UTC(), lastLogged)
		}
	}
}

func (e *Engine) logLongRunningEngineLaneWorkSafely(ctx context.Context, now time.Time, lastLogged map[string]time.Time) {
	defer func() {
		if recovered := recover(); recovered != nil {
			observability.Count(context.Background(), "engine.lane.diagnostics.panics", 1)
			if e != nil && e.logger != nil {
				func() {
					defer func() {
						_ = recover()
					}()
					e.logger.Error("engine lane diagnostics panic recovered", "error", fmt.Sprint(recovered))
				}()
			}
		}
	}()
	e.logLongRunningEngineLaneWork(ctx, now, lastLogged)
}

func (e *Engine) logLongRunningEngineLaneWork(ctx context.Context, now time.Time, lastLogged map[string]time.Time) {
	if e == nil || e.engineLane == nil || e.logger == nil {
		return
	}
	if lastLogged == nil {
		lastLogged = map[string]time.Time{}
	}
	snap := e.engineLane.snapshotAt(now)
	threshold := engineLaneLongHoldThreshold
	for _, work := range snap.Active {
		elapsed := time.Duration(work.ElapsedMS) * time.Millisecond
		if elapsed < threshold {
			continue
		}
		key := work.ID
		if key == "" {
			key = string(work.Kind) + ":" + work.Name
		}
		if last := lastLogged[key]; !last.IsZero() && now.Sub(last) < threshold {
			continue
		}
		lastLogged[key] = now
		e.recordEngineLaneLongHoldWarning(work, now, threshold)
		observability.Count(
			ctx,
			"background.worker.long_running",
			1,
			attribute.String("background.component", string(work.Component)),
			attribute.String("engine.work.kind", string(work.Kind)),
		)
		e.logger.Warn("engine lane work running longer than expected",
			"id", work.ID,
			"kind", work.Kind,
			"component", work.Component,
			"name", work.Name,
			"trigger", work.Trigger,
			"phase", work.Phase,
			"stage", work.Stage,
			"detail", work.Detail,
			"elapsed_ms", work.ElapsedMS,
			"threshold_ms", threshold.Milliseconds(),
			"limit", snap.Limit,
			"active", snap.ActiveCount,
			"waiting", snap.WaitingCount,
		)
	}
}
