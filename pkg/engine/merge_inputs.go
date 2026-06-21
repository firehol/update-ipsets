package engine

import (
	"slices"
	"sort"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/feedhealth"
)

const (
	mergeInputReasonDisabled             = "disabled"
	mergeInputReasonArchived             = "archived"
	mergeInputReasonUnmaintained         = "unmaintained"
	mergeInputReasonMissingLocalFeedBody = "missing_local_feed_body"
	mergeInputRoleExclude                = "exclude"
)

type MergeInputState struct {
	Name        string           `json:"name"`
	Role        string           `json:"role,omitempty"`
	Reason      string           `json:"reason,omitempty"`
	HealthClass feedhealth.Class `json:"health_class,omitempty"`
	Enabled     bool             `json:"enabled"`
	HasFeedBody bool             `json:"has_feed_body"`
}

type MergeComposition struct {
	Included            []MergeInputState
	Subtracted          []MergeInputState
	Excluded            []MergeInputState
	eligibleSourceCount int
	// missingFeedBodies covers any required parent without a durable body,
	// including subtractive parents. Subtractive misses also appear in
	// unavailableSubtractive so composition fails with the stricter
	// "would broaden output" error while integrity recovery can still
	// recheck the missing body.
	missingFeedBodies           []string
	unavailableSubtractive      []string
	unavailableSubtractiveFeeds []string
}

func (e *Engine) MergeComposition(src *config.Source, enableAll bool) MergeComposition {
	return e.mergeCompositionWithResolver(src, enableAll, e.effectiveEntryResolverFromFreshStateSnapshot())
}

func (e *Engine) MergeCompositions(enableAll bool) map[string]MergeComposition {
	if e == nil || e.cfg == nil {
		return nil
	}
	return e.mergeCompositionsWithResolver(enableAll, e.effectiveEntryResolverFromFreshStateSnapshot())
}

func (e *Engine) mergeCompositionsWithResolver(enableAll bool, resolver *effectiveEntryResolver) map[string]MergeComposition {
	if e == nil || e.cfg == nil {
		return nil
	}
	out := make(map[string]MergeComposition)
	for _, name := range config.SortedSourceNames(e.cfg) {
		src := e.cfg.Sources[name]
		if src == nil || src.Provenance != config.ProvenanceSecondaryMerge {
			continue
		}
		out[name] = e.mergeCompositionWithResolver(src, enableAll, resolver)
	}
	return out
}

func (e *Engine) mergeCompositionWithResolver(src *config.Source, enableAll bool, resolver *effectiveEntryResolver) MergeComposition {
	if e == nil || e.cfg == nil || src == nil {
		return MergeComposition{}
	}
	if resolver == nil {
		resolver = e.effectiveEntryResolverFromFreshStateSnapshot()
	}

	includeNames := mergeSourceNames(src)
	excludeNames := mergeExcludeNames(src)
	state := MergeComposition{
		Included:   make([]MergeInputState, 0, len(includeNames)),
		Subtracted: make([]MergeInputState, 0, len(excludeNames)),
		Excluded:   make([]MergeInputState, 0, len(includeNames)+len(excludeNames)),
	}
	state.Included = e.mergeCompositionRows(includeNames, "", true, enableAll, resolver, &state)
	state.Subtracted = e.mergeCompositionRows(excludeNames, mergeInputRoleExclude, false, enableAll, resolver, &state)

	sort.Slice(state.Included, func(i, j int) bool { return state.Included[i].Name < state.Included[j].Name })
	sort.Slice(state.Subtracted, func(i, j int) bool { return state.Subtracted[i].Name < state.Subtracted[j].Name })
	sort.Slice(state.Excluded, func(i, j int) bool { return state.Excluded[i].Name < state.Excluded[j].Name })
	slices.Sort(state.missingFeedBodies)
	slices.Sort(state.unavailableSubtractive)
	slices.Sort(state.unavailableSubtractiveFeeds)
	return state
}

func (e *Engine) mergeCompositionRows(names []string, role string, additive bool, enableAll bool, resolver *effectiveEntryResolver, state *MergeComposition) []MergeInputState {
	rows := make([]MergeInputState, 0, len(names))
	for _, parent := range names {
		row := MergeInputState{Name: parent, Role: role}
		row.Enabled = EffectiveSourceEnabledForRun(e.cfg, e.runtime, parent, enableAll, false)
		if !row.Enabled {
			row.Reason = mergeInputReasonDisabled
			state.excludeMergeInput(row, additive)
			continue
		}

		row.HasFeedBody = fileExists(latestFeedBodyPath(e.feedBodyPath(parent)))
		health := e.classifyEffectiveEntryHealth(parent, resolver.entryFromSnapshot(parent))
		row.HealthClass = health.Class
		if additive {
			switch health.Class {
			case feedhealth.ClassArchived:
				row.Reason = mergeInputReasonArchived
				state.excludeMergeInput(row, additive)
				continue
			case feedhealth.ClassUnmaintained:
				row.Reason = mergeInputReasonUnmaintained
				state.excludeMergeInput(row, additive)
				continue
			}
			state.eligibleSourceCount++
		}

		if !row.HasFeedBody {
			row.Reason = mergeInputReasonMissingLocalFeedBody
			state.excludeMergeInput(row, additive)
			state.missingFeedBodies = append(state.missingFeedBodies, parent)
			continue
		}

		rows = append(rows, row)
	}
	return rows
}

func (state *MergeComposition) excludeMergeInput(row MergeInputState, additive bool) {
	state.Excluded = append(state.Excluded, row)
	if !additive {
		state.unavailableSubtractive = append(state.unavailableSubtractive, mergeInputFailureLabel(row))
		state.unavailableSubtractiveFeeds = append(state.unavailableSubtractiveFeeds, row.Name)
	}
}

func mergeInputFailureLabel(row MergeInputState) string {
	if row.Reason == "" {
		return row.Name
	}
	return row.Name + "(" + row.Reason + ")"
}

func mergeSourceNames(src *config.Source) []string {
	if src == nil {
		return nil
	}
	if len(src.MergeSources) > 0 || len(src.MergeExclude) > 0 {
		return src.MergeSources
	}
	// Legacy and programmatic tests created merge-like sources before
	// MergeSources existed. Loader-expanded merges always set MergeSources.
	return src.DerivedFrom
}

func mergeExcludeNames(src *config.Source) []string {
	if src == nil {
		return nil
	}
	return src.MergeExclude
}
