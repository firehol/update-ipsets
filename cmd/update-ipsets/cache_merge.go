package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/firehol/update-ipsets/pkg/cache"
)

func runCacheMerge(args []string) int {
	fs := flag.NewFlagSet("cache-merge", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	legacyPath := fs.String("legacy", "", "bash legacy cache path")
	localJSONPath := fs.String("local-json", "", "existing Go JSON cache path")
	localOnlyPath := fs.String("local-only", "", "newline-delimited local-only feed names")
	outPath := fs.String("out", "", "output JSON cache path")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *legacyPath == "" || *localOnlyPath == "" || *outPath == "" {
		fmt.Fprintln(os.Stderr, "cache-merge: --legacy, --local-only and --out are required")
		return 2
	}

	merged, err := cache.LoadLegacy(*legacyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cache-merge: load legacy cache: %v\n", err)
		return 1
	}

	local := cache.New()
	if *localJSONPath != "" {
		local, err = cache.Load(*localJSONPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cache-merge: load local JSON cache: %v\n", err)
			return 1
		}
	}

	localOnlyNames, err := readNameList(*localOnlyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cache-merge: read local-only names: %v\n", err)
		return 1
	}

	preserved := 0
	for _, name := range localOnlyNames {
		entry := local.EntrySnapshot(name)
		if entry == nil {
			continue
		}
		clone := *entry
		clone.Name = name
		if merged.Entries == nil {
			merged.Entries = map[string]*cache.Entry{}
		}
		merged.Entries[name] = &clone
		preserved++
	}

	if err := cache.Save(*outPath, merged); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "cache-merge: write merged cache: %v\n", err)
		return 1
	}

	_, _ = fmt.Fprintf(os.Stdout, "merged %d total entries; preserved %d local-only entries\n", len(merged.SnapshotEntries()), preserved)
	return 0
}

func readNameList(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	names := make([]string, 0, len(lines))
	for _, line := range lines {
		name := strings.TrimSpace(line)
		if name == "" || strings.HasPrefix(name, "#") {
			continue
		}
		names = append(names, name)
	}
	return names, nil
}
