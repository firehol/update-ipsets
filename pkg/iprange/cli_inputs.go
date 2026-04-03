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

func printCLIUsage(w io.Writer) int {
	_, _ = fmt.Fprintln(w, "Usage: update-ipsets iprange [options] file1 file2 ...")
	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, "Address family:")
	_, _ = fmt.Fprintln(w, "  -4 | --ipv4    IPv4 mode (default)")
	_, _ = fmt.Fprintln(w, "  -6 | --ipv6    IPv6 mode")
	_, _ = fmt.Fprintln(w, "  --has-ipv6     feature detection for scripts")
	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, "Supported modes:")
	_, _ = fmt.Fprintln(w, "  --union | --combine | --merge | --optimize")
	_, _ = fmt.Fprintln(w, "  --common | --intersect")
	_, _ = fmt.Fprintln(w, "  --exclude-next | --except")
	_, _ = fmt.Fprintln(w, "  --diff | --diff-next")
	_, _ = fmt.Fprintln(w, "  --compare | --compare-first | --compare-next")
	_, _ = fmt.Fprintln(w, "  --count-unique | --count-unique-all")
	_, _ = fmt.Fprintln(w, "  --ipset-reduce | --reduce-factor")
	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, "Input expansion:")
	_, _ = fmt.Fprintln(w, "  @filelist    load one file path per line")
	_, _ = fmt.Fprintln(w, "  @directory   load all regular files sorted by name")
	return 0
}

func loadInputArgument(ctx context.Context, arg string, stdin io.Reader, opts ParseOptions) ([]*IPSet, error) {
	if arg == "-" {
		set, err := ParseReader(ctx, DefaultName, stdin, opts)
		if err != nil {
			return nil, err
		}
		return []*IPSet{set}, nil
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
			out := make([]*IPSet, 0, len(paths))
			for _, candidate := range paths {
				set, err := loadSinglePath(ctx, candidate, opts)
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
		scanner := bytes.NewBuffer(data)
		lines := strings.Split(scanner.String(), "\n")
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
		out := make([]*IPSet, 0, len(paths))
		for _, candidate := range paths {
			set, err := loadSinglePath(ctx, candidate, opts)
			if err != nil {
				return nil, err
			}
			out = append(out, set)
		}
		return out, nil
	}

	set, err := loadSinglePath(ctx, arg, opts)
	if err != nil {
		return nil, err
	}
	return []*IPSet{set}, nil
}

func loadSinglePath(ctx context.Context, path string, opts ParseOptions) (*IPSet, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	return ParseReader(ctx, path, file, opts)
}

func mergeAll(sets []*IPSet) (*IPSet, error) {
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
