package iprange

import (
	"context"
	"fmt"
	"io"
	"os"
)

func loadInputArgument6(ctx context.Context, arg string, stdin io.Reader, opts ParseOptions) ([]*IPSet6, error) {
	paths, stdinInput, err := expandCLIInputPaths(arg)
	if err != nil {
		return nil, err
	}
	if stdinInput {
		set, err := ParseReader6(ctx, DefaultName, stdin, opts)
		if err != nil {
			return nil, err
		}
		return []*IPSet6{set}, nil
	}

	out := make([]*IPSet6, 0, len(paths))
	for _, candidate := range paths {
		set, err := loadSinglePath6(ctx, candidate, opts)
		if err != nil {
			return nil, err
		}
		out = append(out, set)
	}
	return out, nil
}

func loadSinglePath6(ctx context.Context, path string, opts ParseOptions) (*IPSet6, error) {
	file, err := os.Open(path) // nosemgrep: local CLI/parser input; no daemon or public request path reaches this API directly.
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	return ParseReader6(ctx, path, file, opts)
}

func mergeAll6(sets []*IPSet6) (*IPSet6, error) {
	if len(sets) == 0 {
		return nil, fmt.Errorf("no ipsets provided")
	}
	merged := sets[0].Clone()
	if len(sets) > 1 {
		merged.Name = "combined ipset"
	}
	for _, set := range sets[1:] {
		if err := merged.Merge(set); err != nil {
			return nil, err
		}
	}
	return merged, nil
}
