package config

import "testing"

func TestExpandDerivativesMergeRedistributabilityInheritance(t *testing.T) {
	cfg := New()
	cfg.Sources["allowed"] = &Source{Name: "allowed", Frequency: 60, IPV: "ipv4", Output: "ipset"}
	cfg.Sources["blocked"] = &Source{
		Name:            "blocked",
		Frequency:       60,
		IPV:             "ipv4",
		Output:          "ipset",
		Redistributable: boolPtr(false),
	}

	cfg.Merges["all_clear"] = &Merge{
		Name:    "all_clear",
		IPV:     "ipv4",
		Output:  "ipset",
		Sources: []string{"allowed"},
	}
	cfg.Merges["direct_add"] = &Merge{
		Name:            "direct_add",
		IPV:             "ipv4",
		Output:          "ipset",
		Sources:         []string{"blocked"},
		Redistributable: boolPtr(true),
	}
	cfg.Merges["direct_exclude"] = &Merge{
		Name:    "direct_exclude",
		IPV:     "ipv4",
		Output:  "ipset",
		Sources: []string{"allowed"},
		Exclude: []string{"blocked"},
	}
	cfg.Merges["explicit_false"] = &Merge{
		Name:            "explicit_false",
		IPV:             "ipv4",
		Output:          "ipset",
		Sources:         []string{"allowed"},
		Redistributable: boolPtr(false),
	}
	cfg.Merges["nested_child"] = &Merge{
		Name:    "nested_child",
		IPV:     "ipv4",
		Output:  "ipset",
		Sources: []string{"nested_parent"},
	}
	cfg.Merges["nested_parent"] = &Merge{
		Name:    "nested_parent",
		IPV:     "ipv4",
		Output:  "ipset",
		Sources: []string{"blocked"},
	}

	if err := ExpandDerivatives(cfg); err != nil {
		t.Fatalf("ExpandDerivatives() error = %v", err)
	}

	for _, name := range []string{"direct_add", "direct_exclude", "explicit_false", "nested_child", "nested_parent"} {
		if cfg.Sources[name].IsRedistributable() {
			t.Errorf("%s should inherit non-redistributable policy", name)
		}
	}
	if !cfg.Sources["all_clear"].IsRedistributable() {
		t.Error("all_clear should remain redistributable")
	}
}
