package engine

import (
	"github.com/firehol/update-ipsets/pkg/config"
)

// leafAncestors returns the set of positive leaf (primary, non-derivative)
// source names that a given feed ultimately derives from. It walks positive
// ancestry until it hits sources with no positive parents and collects their
// names. For signed merges, only additive parents are positive ancestry;
// subtractive parents remain dependencies but do not make comparison rows
// related.
//
// For a plain HTTP source X:               leafAncestors(X) = {X}
// For a retention variant X_1d of X:       leafAncestors(X_1d) = {X}
// For a merge M with sources [A, B] - [C]: leafAncestors(M) = {A, B}
//
//	(or the leaves of A and B if either is itself a
//	derivative — merges-of-merges are legal in the DAG model
//	and this walks through transitively)
//
// Used by writeComparisonFiles to mark "related" pairs: two feeds whose
// positive leaf-ancestor sets intersect share at least one primary ancestor,
// which means any overlap between them is trivially explained by shared positive
// ancestry rather than by genuine agreement between independent sources. The
// public UI's uniqueness / inclusion tiles skip these rows so the numbers
// reflect real cross-feed signal instead of derivative echo.
func leafAncestors(cfg *config.Config, name string) map[string]bool {
	result := map[string]bool{}
	if cfg == nil {
		return result
	}
	visited := map[string]bool{}
	var walk func(n string)
	walk = func(n string) {
		if visited[n] {
			return
		}
		visited[n] = true
		src := cfg.Sources[n]
		if src == nil {
			return
		}
		parents := positiveLineageParents(src)
		if len(parents) == 0 {
			result[n] = true
			return
		}
		for _, parent := range parents {
			walk(parent)
		}
	}
	walk(name)
	return result
}

func positiveLineageParents(src *config.Source) []string {
	if src == nil {
		return nil
	}
	if src.Provenance == config.ProvenanceSecondaryMerge || len(src.MergeSources) > 0 || len(src.MergeExclude) > 0 {
		return mergeSourceNames(src)
	}
	return src.DerivedFrom
}

// mergeCompareRows merges fresh comparison rows into the existing rows
// loaded from disk. Fresh positive-overlap rows replace any existing entry with
// the same counterpart Name; rows that only exist on disk are preserved. Fresh
// zero-overlap rows delete matching existing rows because absence is the public
// representation of "no overlap".
func mergeCompareRows(existing, fresh []CompareRow) []CompareRow {
	byName := make(map[string]CompareRow, len(existing)+len(fresh))
	for _, r := range existing {
		if r.Common == 0 {
			continue
		}
		byName[r.Name] = r
	}
	for _, r := range fresh {
		if r.Common == 0 {
			delete(byName, r.Name)
			continue
		}
		byName[r.Name] = r
	}
	out := make([]CompareRow, 0, len(byName))
	for _, r := range byName {
		out = append(out, r)
	}
	return out
}
