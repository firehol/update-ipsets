package engine

import (
	"github.com/firehol/update-ipsets/internal/observability"

	"go.opentelemetry.io/otel/attribute"
)

func (e *Engine) setRunPhase(phase RunPhase) {
	if e == nil {
		return
	}
	e.mu.Lock()
	e.currentPhase = phase
	current := e.currentMetrics
	e.mu.Unlock()
	observeEnginePhaseCurrent(phase)
	if current != nil {
		current.setPhase(phase)
	}
}

func observeEnginePhaseCurrent(current RunPhase) {
	ctx := observability.BackgroundContext()
	for _, phase := range allRunPhases() {
		value := int64(0)
		if current == phase {
			value = 1
		}
		observability.Gauge(ctx, "engine.phase.current", value, attribute.String("engine.phase", string(phase)))
	}
}
