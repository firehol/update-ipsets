package dronebl

import "strings"

type OutputSpec struct {
	Name  string
	Lists []string
}

func ParseListNames(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	lists := parts[:0]
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			lists = append(lists, part)
		}
	}
	return lists
}

func BuildOutputs(parsed *ParsedBuildzone, specs []OutputSpec) map[string]*RangeSet {
	outputs := make(map[string]*RangeSet, len(specs))
	for _, spec := range specs {
		include := mergedLists(parsed, spec.Lists, true)
		exclude := mergedLists(parsed, spec.Lists, false)
		outputs[spec.Name] = Exclude(include, exclude)
	}
	return outputs
}

func mergedLists(parsed *ParsedBuildzone, lists []string, include bool) *RangeSet {
	out := NewRangeSet()
	for _, list := range append([]string{"global"}, lists...) {
		data := parsed.Lists[list]
		if data == nil {
			continue
		}
		var set *RangeSet
		if include {
			set = data.Include
		} else {
			set = data.Exclude
		}
		_ = out.Merge(set)
	}
	out.Optimize()
	return out
}
