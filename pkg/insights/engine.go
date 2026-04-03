package insights

// catalog is the package-level registry of rules. Each rules_*.go file
// appends to it from an init() function. Order in the catalog only
// affects the order rules are evaluated in; rules are independent and
// never read each other's output.
var catalog []Rule

// Engine evaluates the catalog against a SignalSnapshot. The type is a
// trivial wrapper kept for forward compatibility: future versions may
// allow per-Engine rule selection, threshold overrides, or injected
// time sources. The zero value is not valid — always use NewEngine.
type Engine struct {
	rules []Rule
}

// NewEngine returns a new Engine bound to the package-level catalog.
// The catalog is captured by reference only at construction time, so
// adding rules in test helpers after NewEngine will not affect an
// already-constructed engine.
func NewEngine() *Engine {
	cp := make([]Rule, len(catalog))
	copy(cp, catalog)
	return &Engine{rules: cp}
}

// Derive runs every rule in the catalog against snap and returns the
// insights that fired. Rules that fail their MinSamples guard or that
// decide not to fire are silently skipped — the returned slice only
// contains insights the engine considers factual and worth publishing.
func (e *Engine) Derive(snap SignalSnapshot) []Insight {
	out := make([]Insight, 0, len(e.rules))
	for _, r := range e.rules {
		if r.MinSamples != nil && !r.MinSamples(snap) {
			continue
		}
		if r.Compute == nil {
			continue
		}
		ins, ok := r.Compute(snap)
		if !ok {
			continue
		}
		ins.Code = r.Code
		ins.Section = r.Section
		ins.Methodology = r.Methodology
		out = append(out, ins)
	}
	return out
}

// Rules returns a copy of the active rule catalog. Intended for tests
// and documentation tooling; callers must not mutate the returned
// slice's contents.
func (e *Engine) Rules() []Rule {
	cp := make([]Rule, len(e.rules))
	copy(cp, e.rules)
	return cp
}
