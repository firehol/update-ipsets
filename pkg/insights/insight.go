// Package insights derives deterministic, factual observations about a
// feed from the engine's existing per-feed data. It produces one-line
// headlines backed by raw numbers; rules either fire or stay silent and
// never publish hedged claims. See docs/todo-history/TODO-insights.md
// for the complete design and rule catalog.
package insights

import (
	"encoding/json"
	"fmt"
)

// Section identifies which part of the feed page an insight belongs to.
// The frontend groups insights by Section to render both the "What we
// noticed" top callout (sampled across sections for diversity) and the
// in-context callouts beside each chart.
type Section int

const (
	// SectionOverview is reserved for insights that are self-contained
	// enough to show in the top "What we noticed" card without any
	// surrounding chart context.
	SectionOverview Section = iota
	// SectionComposition covers the geographic, ASN, and bogon
	// attribution charts.
	SectionComposition
	// SectionRetention covers the currently-listed and removed-IP age
	// distributions.
	SectionRetention
	// SectionTrends covers the size and churn time-series.
	SectionTrends
	// SectionRelationships covers the pairwise overlap section.
	SectionRelationships
	// SectionFreshness covers operational health (update lag, clock
	// skew, download failures). Reserved for future rules.
	SectionFreshness
)

// String returns a stable lowercase identifier for serialization to JSON.
// The values are part of the public API shape consumed by the frontend.
func (s Section) String() string {
	switch s {
	case SectionOverview:
		return "overview"
	case SectionComposition:
		return "composition"
	case SectionRetention:
		return "retention"
	case SectionTrends:
		return "trends"
	case SectionRelationships:
		return "relationships"
	case SectionFreshness:
		return "freshness"
	}
	return "unknown"
}

// MarshalJSON renders Section as its stable string form so the JSON
// payload stays stable across Go iota renumbering.
func (s Section) MarshalJSON() ([]byte, error) {
	return []byte(`"` + s.String() + `"`), nil
}

// UnmarshalJSON accepts the stable string form produced by MarshalJSON so
// insight payloads can round-trip through JSON validation and tests.
func (s *Section) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	switch raw {
	case SectionOverview.String():
		*s = SectionOverview
	case SectionComposition.String():
		*s = SectionComposition
	case SectionRetention.String():
		*s = SectionRetention
	case SectionTrends.String():
		*s = SectionTrends
	case SectionRelationships.String():
		*s = SectionRelationships
	case SectionFreshness.String():
		*s = SectionFreshness
	default:
		return fmt.Errorf("unknown insight section %q", raw)
	}
	return nil
}

// Insight is one published observation about a feed. It carries a stable
// code for identification, a human-readable headline (the ONLY text the
// UI shows), an evidence map with the raw values that fired the rule,
// and a methodology link so users can inspect how the number was derived.
//
// There is deliberately no confidence field: a rule either fires or it
// stays silent. The threshold IS the publish/no-publish decision.
type Insight struct {
	Code        string         `json:"code"`
	Section     Section        `json:"section"`
	Headline    string         `json:"headline"`
	Evidence    map[string]any `json:"evidence,omitempty"`
	Methodology string         `json:"methodology,omitempty"`
}
