package config

import (
	"strings"
	"testing"
)

// TestDependentsEmpty confirms Dependents() returns an empty map on
// a nil receiver and on a config with no DerivedFrom entries.
func TestDependentsEmpty(t *testing.T) {
	var nilCfg *Config
	if got := nilCfg.Dependents(); len(got) != 0 {
		t.Errorf("nil config Dependents() = %v, want empty", got)
	}

	cfg := &Config{Sources: map[string]*Source{
		"plain": {Name: "plain", URL: "https://example.com/a.txt"},
	}}
	if got := cfg.Dependents(); len(got) != 0 {
		t.Errorf("config without DerivedFrom = %v, want empty", got)
	}
}

// TestDependentsReverseIndex verifies a parent with multiple
// derivatives and a grandchild produces the expected reverse index,
// including alphabetical ordering of the dependent lists.
func TestDependentsReverseIndex(t *testing.T) {
	cfg := &Config{Sources: map[string]*Source{
		"viriback": {
			Name: "viriback",
			URL:  "https://example.com/viriback.txt",
		},
		"viriback_1d": {
			Name:        "viriback_1d",
			URL:         "internal://retention_window?parent=viriback&minutes=1440",
			DerivedFrom: []string{"viriback"},
		},
		"viriback_7d": {
			Name:        "viriback_7d",
			URL:         "internal://retention_window?parent=viriback&minutes=10080",
			DerivedFrom: []string{"viriback"},
		},
		"viriback_30d": {
			Name:        "viriback_30d",
			URL:         "internal://retention_window?parent=viriback&minutes=43200",
			DerivedFrom: []string{"viriback"},
		},
		"merge_abc": {
			Name:        "merge_abc",
			URL:         "internal://merge?exclude=viriback_30d&inputs=viriback,viriback_1d",
			DerivedFrom: []string{"viriback", "viriback_1d", "viriback_30d"},
			MergeSources: []string{
				"viriback",
				"viriback_1d",
			},
			MergeExclude: []string{"viriback_30d"},
		},
	}}

	dep := cfg.Dependents()
	viribackDeps := dep["viriback"]
	want := []string{"merge_abc", "viriback_1d", "viriback_30d", "viriback_7d"}
	if len(viribackDeps) != len(want) {
		t.Fatalf("viriback dependents = %v, want %v", viribackDeps, want)
	}
	for i, name := range want {
		if viribackDeps[i] != name {
			t.Errorf("viriback dependents[%d] = %q, want %q", i, viribackDeps[i], name)
		}
	}

	viriback1dDeps := dep["viriback_1d"]
	if len(viriback1dDeps) != 1 || viriback1dDeps[0] != "merge_abc" {
		t.Errorf("viriback_1d dependents = %v, want [merge_abc]", viriback1dDeps)
	}
	viriback30dDeps := dep["viriback_30d"]
	if len(viriback30dDeps) != 1 || viriback30dDeps[0] != "merge_abc" {
		t.Errorf("viriback_30d dependents = %v, want [merge_abc]", viriback30dDeps)
	}

	// Leaf nodes (merge_abc) have no dependents.
	if _, ok := dep["merge_abc"]; ok {
		t.Errorf("merge_abc should have no entry in the reverse index, got %v", dep["merge_abc"])
	}
}

// TestDetectCyclesAcyclic confirms a valid graph produces no error.
func TestDetectCyclesAcyclic(t *testing.T) {
	cfg := &Config{Sources: map[string]*Source{
		"a":   {Name: "a", URL: "https://example.com/a.txt"},
		"b":   {Name: "b", URL: "https://example.com/b.txt"},
		"a_1": {Name: "a_1", URL: "internal://retention_window?parent=a", DerivedFrom: []string{"a"}},
		"ab":  {Name: "ab", URL: "internal://merge?inputs=a,b", DerivedFrom: []string{"a", "b"}},
	}}
	if err := cfg.DetectCycles(); err != nil {
		t.Errorf("acyclic graph returned error: %v", err)
	}
}

// TestDetectCyclesDirect catches a direct 2-node cycle.
func TestDetectCyclesDirect(t *testing.T) {
	cfg := &Config{Sources: map[string]*Source{
		"a": {Name: "a", URL: "internal://merge?inputs=b", DerivedFrom: []string{"b"}},
		"b": {Name: "b", URL: "internal://merge?inputs=a", DerivedFrom: []string{"a"}},
	}}
	err := cfg.DetectCycles()
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "a") || !strings.Contains(msg, "b") {
		t.Errorf("cycle error %q should mention both a and b", msg)
	}
}

// TestDetectCyclesIndirect catches a longer cycle through a chain.
func TestDetectCyclesIndirect(t *testing.T) {
	cfg := &Config{Sources: map[string]*Source{
		"x": {Name: "x", URL: "internal://merge?inputs=z", DerivedFrom: []string{"z"}},
		"y": {Name: "y", URL: "internal://merge?inputs=x", DerivedFrom: []string{"x"}},
		"z": {Name: "z", URL: "internal://merge?inputs=y", DerivedFrom: []string{"y"}},
	}}
	if err := cfg.DetectCycles(); err == nil {
		t.Fatal("expected cycle error for 3-node cycle, got nil")
	}
}

// TestDetectCyclesSelfLoop catches a source that lists itself.
func TestDetectCyclesSelfLoop(t *testing.T) {
	cfg := &Config{Sources: map[string]*Source{
		"narcissus": {
			Name:        "narcissus",
			URL:         "internal://retention_window?parent=narcissus",
			DerivedFrom: []string{"narcissus"},
		},
	}}
	if err := cfg.DetectCycles(); err == nil {
		t.Fatal("expected cycle error for self-loop, got nil")
	}
}
