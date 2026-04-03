package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

const modulePath = "github.com/firehol/update-ipsets"

type Posture struct {
	Version                 int                  `json:"version"`
	Scope                   []string             `json:"scope"`
	ExcludedPaths           []string             `json:"excluded_paths"`
	NestedModulesExcluded   []string             `json:"nested_modules_excluded"`
	Thresholds              Thresholds           `json:"thresholds"`
	Source                  SourceSummary        `json:"source"`
	Packages                []PackageMetric      `json:"packages"`
	LargeFiles              []FileMetric         `json:"large_files"`
	LargeFunctions          []FunctionMetric     `json:"large_functions"`
	Cache                   CacheMetrics         `json:"cache"`
	SemanticShortcutMatches []LineMatch          `json:"semantic_shortcut_matches"`
	WebRoutes               WebRouteMetrics      `json:"web_routes"`
	Generation              BaselineInstructions `json:"generation"`
}

type BaselineInstructions struct {
	Command string `json:"command"`
	Note    string `json:"note"`
}

type Thresholds struct {
	ReviewFileLines       int `json:"review_file_lines"`
	NewLargeFileLines     int `json:"new_large_file_lines"`
	ReviewFunctionLines   int `json:"review_function_lines"`
	NewLargeFunctionLines int `json:"new_large_function_lines"`
}

type SourceSummary struct {
	Files   int        `json:"files"`
	Lines   int        `json:"lines"`
	MaxFile FileMetric `json:"max_file"`
}

type FileMetric struct {
	Path  string `json:"path"`
	Lines int    `json:"lines"`
}

type FunctionMetric struct {
	Path       string `json:"path"`
	Name       string `json:"name"`
	StartLine  int    `json:"start_line"`
	Lines      int    `json:"lines"`
	Complexity int    `json:"complexity"`
}

type PackageMetric struct {
	Package        string `json:"package"`
	Path           string `json:"path"`
	Files          int    `json:"files"`
	Lines          int    `json:"lines"`
	DirectImports  int    `json:"direct_imports"`
	ProjectImports int    `json:"project_imports"`
	TransitiveDeps int    `json:"transitive_deps"`
}

type CacheMetrics struct {
	ProductionEntryCalls        int         `json:"production_entry_calls"`
	TestEntryCalls              int         `json:"test_entry_calls"`
	ProductionFieldWrites       int         `json:"production_field_writes"`
	TestFieldWrites             int         `json:"test_field_writes"`
	ProductionEntryReplacements int         `json:"production_entry_replacements"`
	TestEntryReplacements       int         `json:"test_entry_replacements"`
	ProductionMutationFiles     []FileCount `json:"production_mutation_files"`
	TestMutationFiles           []FileCount `json:"test_mutation_files"`
}

type FileCount struct {
	Path  string `json:"path"`
	Count int    `json:"count"`
}

type LineMatch struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

type WebRouteMetrics struct {
	MuxHandleFuncCalls int `json:"mux_handle_func_calls"`
	MuxHandleCalls     int `json:"mux_handle_calls"`
}

func defaultThresholds() Thresholds {
	return Thresholds{
		ReviewFileLines:       500,
		NewLargeFileLines:     800,
		ReviewFunctionLines:   120,
		NewLargeFunctionLines: 200,
	}
}

func Collect(root string) (Posture, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Posture{}, err
	}
	files, err := sourceFiles(absRoot)
	if err != nil {
		return Posture{}, err
	}
	posture := Posture{
		Version: 1,
		Scope: []string{
			"cmd/**/*.go",
			"internal/**/*.go",
			"pkg/**/*.go",
			"tools/**/*.go except nested modules",
			"ui/src/**/*.{ts,tsx}",
		},
		ExcludedPaths: []string{
			".agents/",
			".git/",
			".playwright-mcp/",
			"pkg/web/static/",
			"ui/dist/",
			"ui/node_modules/",
		},
		NestedModulesExcluded: []string{"tools/dronebl2ipsets"},
		Thresholds:            defaultThresholds(),
		Generation: BaselineInstructions{
			Command: "go run ./tools/archposture -root . > tools/archposture/testdata/posture_baseline.json",
			Note:    "Update the baseline only when a SOW explicitly accepts the posture change.",
		},
	}

	packages := map[string]*PackageMetric{}
	for _, path := range files {
		rel := relPath(absRoot, path)
		lines, err := countLines(path)
		if err != nil {
			return Posture{}, err
		}
		posture.Source.Files++
		posture.Source.Lines += lines
		if lines > posture.Source.MaxFile.Lines {
			posture.Source.MaxFile = FileMetric{Path: rel, Lines: lines}
		}
		if lines >= posture.Thresholds.ReviewFileLines {
			posture.LargeFiles = append(posture.LargeFiles, FileMetric{Path: rel, Lines: lines})
		}
		if strings.HasSuffix(path, ".go") {
			if err := collectGoFile(absRoot, path, lines, packages, &posture); err != nil {
				return Posture{}, err
			}
		}
		matches, err := semanticShortcutMatches(path, rel)
		if err != nil {
			return Posture{}, err
		}
		posture.SemanticShortcutMatches = append(posture.SemanticShortcutMatches, matches...)
	}
	if err := mergeGoList(absRoot, packages); err != nil {
		return Posture{}, err
	}
	for _, pkg := range packages {
		posture.Packages = append(posture.Packages, *pkg)
	}
	sort.Slice(posture.Packages, func(i, j int) bool {
		return posture.Packages[i].Path < posture.Packages[j].Path
	})
	sort.Slice(posture.LargeFiles, func(i, j int) bool {
		if posture.LargeFiles[i].Lines == posture.LargeFiles[j].Lines {
			return posture.LargeFiles[i].Path < posture.LargeFiles[j].Path
		}
		return posture.LargeFiles[i].Lines > posture.LargeFiles[j].Lines
	})
	sort.Slice(posture.LargeFunctions, func(i, j int) bool {
		if posture.LargeFunctions[i].Lines == posture.LargeFunctions[j].Lines {
			return posture.LargeFunctions[i].Path < posture.LargeFunctions[j].Path
		}
		return posture.LargeFunctions[i].Lines > posture.LargeFunctions[j].Lines
	})
	sort.Slice(posture.SemanticShortcutMatches, func(i, j int) bool {
		if posture.SemanticShortcutMatches[i].Path == posture.SemanticShortcutMatches[j].Path {
			return posture.SemanticShortcutMatches[i].Line < posture.SemanticShortcutMatches[j].Line
		}
		return posture.SemanticShortcutMatches[i].Path < posture.SemanticShortcutMatches[j].Path
	})
	posture.Cache.ProductionMutationFiles = sortedFileCounts(posture.Cache.ProductionMutationFiles)
	posture.Cache.TestMutationFiles = sortedFileCounts(posture.Cache.TestMutationFiles)
	return posture, nil
}

func sourceFiles(root string) ([]string, error) {
	var files []string
	for _, dir := range []string{"cmd", "internal", "pkg", "tools", filepath.Join("ui", "src")} {
		start := filepath.Join(root, dir)
		if _, err := os.Stat(start); err != nil {
			continue
		}
		err := filepath.WalkDir(start, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel := relPath(root, path)
			if d.IsDir() {
				if skipDir(rel) {
					return filepath.SkipDir
				}
				return nil
			}
			if skipFile(rel) {
				return nil
			}
			switch filepath.Ext(path) {
			case ".go", ".ts", ".tsx":
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	slices.Sort(files)
	return files, nil
}

func skipDir(rel string) bool {
	rel = filepath.ToSlash(rel)
	switch rel {
	case "tools/dronebl2ipsets":
		return true
	case "pkg/web/static":
		return true
	case "ui/src/node_modules":
		return true
	default:
		return false
	}
}

func skipFile(rel string) bool {
	rel = filepath.ToSlash(rel)
	return strings.HasSuffix(rel, ".d.ts")
}

func countLines(path string) (int, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if len(body) == 0 {
		return 0, nil
	}
	lines := bytes.Count(body, []byte{'\n'})
	if body[len(body)-1] != '\n' {
		lines++
	}
	return lines, nil
}

func relPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}
