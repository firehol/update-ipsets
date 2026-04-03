package iprange

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func loadInputArgument6(ctx context.Context, arg string, stdin io.Reader, opts ParseOptions) ([]*IPSet6, error) {
	if arg == "-" {
		set, err := ParseReader6(ctx, DefaultName, stdin, opts)
		if err != nil {
			return nil, err
		}
		return []*IPSet6{set}, nil
	}

	if strings.HasPrefix(arg, "@") {
		path := strings.TrimPrefix(arg, "@")
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			entries, err := os.ReadDir(path)
			if err != nil {
				return nil, err
			}
			paths := make([]string, 0, len(entries))
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				paths = append(paths, filepath.Join(path, entry.Name()))
			}
			slices.Sort(paths)
			if len(paths) == 0 {
				return nil, fmt.Errorf("no valid files found in directory: %s", path)
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

		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		lines := strings.Split(bytes.NewBuffer(data).String(), "\n")
		paths := make([]string, 0, len(lines))
		for _, line := range lines {
			trimmed := stripInlineComment(line)
			if trimmed == "" {
				continue
			}
			paths = append(paths, trimmed)
		}
		if len(paths) == 0 {
			return nil, fmt.Errorf("no valid files found in file list: %s", path)
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

	set, err := loadSinglePath6(ctx, arg, opts)
	if err != nil {
		return nil, err
	}
	return []*IPSet6{set}, nil
}

func loadSinglePath6(ctx context.Context, path string, opts ParseOptions) (*IPSet6, error) {
	file, err := os.Open(path)
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
