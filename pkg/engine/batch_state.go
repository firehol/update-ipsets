package engine

func (e *Engine) setRunPhase(phase RunPhase) {
	if e == nil {
		return
	}
	e.mu.Lock()
	e.currentPhase = phase
	current := e.currentMetrics
	e.mu.Unlock()
	if current != nil {
		current.setPhase(phase)
	}
}
