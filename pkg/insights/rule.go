package insights

// Rule is one deterministic observation. MinSamples is the sample-size
// guard evaluated BEFORE Compute runs; it must return true for Compute
// to be invoked. Compute returns an Insight and a boolean that tells
// the engine whether the rule fired. The engine fills in Code, Section,
// and Methodology on the returned Insight from the Rule's own fields so
// individual rules only have to build Headline + Evidence.
type Rule struct {
	Code        string
	Name        string
	Section     Section
	MinSamples  func(snap SignalSnapshot) bool
	Compute     func(snap SignalSnapshot) (Insight, bool)
	Methodology string
}
