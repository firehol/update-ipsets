package config

import (
	"fmt"
	"slices"
	"strings"
)

// Dependents returns the reverse-dependency index for the catalog:
// for every source name, the list of source names that declare it in
// their DerivedFrom list. The result is a fresh map the caller may
// mutate; it is recomputed from the current Source map on every call,
// so it reflects the latest Config state after loader expansion.
//
// The reverse index is the data structure the download queue uses for
// dynamic batch injection: when a parent source finishes updating in
// the current batch, the worker pool looks up Dependents()[parent]
// and pushes every derived source into the work queue so it runs in
// the same tick rather than waiting for the next scheduler pass.
//
// Ordering inside each list is alphabetical so iteration is stable
// across runs (tests, diffs, logs).
func (c *Config) Dependents() map[string][]string {
	if c == nil {
		return map[string][]string{}
	}
	out := map[string]map[string]struct{}{}
	for childName, src := range c.Sources {
		if src == nil {
			continue
		}
		for _, parentName := range src.DerivedFrom {
			if parentName == "" {
				continue
			}
			bucket, ok := out[parentName]
			if !ok {
				bucket = map[string]struct{}{}
				out[parentName] = bucket
			}
			bucket[childName] = struct{}{}
		}
	}
	final := make(map[string][]string, len(out))
	for parent, set := range out {
		names := make([]string, 0, len(set))
		for name := range set {
			names = append(names, name)
		}
		slices.Sort(names)
		final[parent] = names
	}
	return final
}

// DetectCycles walks the derived-from graph and returns an error
// listing every cycle it finds. A cycle exists when a source
// transitively lists itself in its DerivedFrom chain, which would
// cause the download queue's dynamic injection to spin forever.
//
// The error message enumerates the participating source names in
// the order they were discovered by the DFS, so curators can trace
// the cycle directly back to the YAML. Returns nil when the graph
// is acyclic.
//
// Detection runs in time linear in the number of source-to-parent
// edges (O(V+E)) via the classic white/grey/black DFS coloring.
// Called at config load time — failure aborts startup with a clear
// diagnostic rather than letting the engine spin on a broken graph.
func (c *Config) DetectCycles() error {
	if c == nil {
		return nil
	}
	const (
		white = 0
		grey  = 1
		black = 2
	)
	color := make(map[string]int, len(c.Sources))
	var cycles [][]string
	var stack []string
	var visit func(name string) bool
	visit = func(name string) bool {
		switch color[name] {
		case grey:
			// Found a back edge — extract the cycle from the stack.
			for i, n := range stack {
				if n == name {
					cycle := append([]string(nil), stack[i:]...)
					cycle = append(cycle, name)
					cycles = append(cycles, cycle)
					return true
				}
			}
			return true
		case black:
			return false
		}
		color[name] = grey
		stack = append(stack, name)
		src := c.Sources[name]
		if src != nil {
			for _, parent := range src.DerivedFrom {
				visit(parent)
			}
		}
		stack = stack[:len(stack)-1]
		color[name] = black
		return false
	}
	// Visit in sorted order for deterministic cycle enumeration.
	names := make([]string, 0, len(c.Sources))
	for name := range c.Sources {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if color[name] == white {
			visit(name)
		}
	}
	if len(cycles) == 0 {
		return nil
	}
	parts := make([]string, 0, len(cycles))
	for _, cycle := range cycles {
		parts = append(parts, strings.Join(cycle, " → "))
	}
	return fmt.Errorf("derived-from cycle(s) detected: %s", strings.Join(parts, "; "))
}
