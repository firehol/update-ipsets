package scheduler

import (
	"context"
	"runtime/debug"
	"time"
)

func (r *Runner) runRecoverableLoop(ctx context.Context, component string, fn func()) {
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		if ctx.Err() != nil {
			return
		}
		panicked := false
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					panicked = true
					r.recordRecoveredPanic(component, recovered)
				}
			}()
			fn()
		}()
		if !panicked {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

func (r *Runner) handleActionRecovered(ctx context.Context, action PendingAction) {
	defer func() {
		if recovered := recover(); recovered != nil {
			r.recordRecoveredPanic("action", recovered)
		}
	}()
	r.handleAction(ctx, action)
}

func (r *Runner) recordRecoveredPanic(component string, recovered any) {
	if r == nil {
		return
	}
	if component == "" {
		component = "unknown"
	}
	r.metrics.recordRecoveredPanic(component, r.nowOrUTC())
	if r.logger != nil {
		r.logger.Error("scheduler panic recovered",
			"component", component,
			"panic", recovered,
			"stack", string(debug.Stack()))
	}
}

func (r *Runner) recordActionAdmissionFailure(err error) {
	if r == nil {
		return
	}
	r.metrics.recordActionAdmissionFailure(r.nowOrUTC())
	if r.logger != nil {
		r.logger.Error("scheduler action admission failed", "error", err)
	}
}

func (r *Runner) nowOrUTC() time.Time {
	if r != nil && r.now != nil {
		return r.now().UTC()
	}
	return time.Now().UTC()
}
