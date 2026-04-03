package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var allowedConfiguredSourceNameLiterals = map[string]bool{
	// RFC-reserved ranges are protocol facts owned by the synthetic source.
	"rfc_reserved": true,
	// "bogons" is both a legacy source name and a first-class config use role.
	"bogons": true,
}

var allowedConfiguredSourceNameLiteralSites = map[string]map[string]bool{
	// CAIDA's configured source name is also the parser format identifier.
	"caida_prefix2as": {
		"pkg/asnloc/asnloc.go":           true,
		"pkg/engine/asn_url_resolver.go": true,
		"pkg/engine/format_handlers.go":  true,
	},
}

func TestProductionGoAvoidsConfiguredSourceNameLiterals(t *testing.T) {
	root := repoRoot(t)
	sourceNames := loadConfiguredSourceNames(t, root)
	for name := range allowedConfiguredSourceNameLiterals {
		delete(sourceNames, name)
	}

	fset := token.NewFileSet()
	for _, dir := range []string{"cmd", "internal", "pkg"} {
		rootDir := filepath.Join(root, dir)
		if err := filepath.WalkDir(rootDir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if path == filepath.Join(root, "pkg", "web", "static") {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			return assertGoFileHasNoConfiguredSourceNameLiterals(t, fset, root, path, sourceNames)
		}); err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
	}
}

func loadConfiguredSourceNames(t *testing.T, root string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	namePattern := regexp.MustCompile(`^  ([a-z0-9][a-z0-9_]*):\s*$`)
	configRoot := filepath.Join(root, "configs", "firehol")
	if err := filepath.WalkDir(configRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		inSources := false
		for _, line := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(line) == "sources:" {
				inSources = true
				continue
			}
			if inSources && line != "" && line[0] != ' ' {
				inSources = false
			}
			if !inSources {
				continue
			}
			if match := namePattern.FindStringSubmatch(line); match != nil {
				out[match[1]] = true
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("load configured source names: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("no configured source names found")
	}
	return out
}

func assertGoFileHasNoConfiguredSourceNameLiterals(t *testing.T, fset *token.FileSet, root, path string, sourceNames map[string]bool) error {
	t.Helper()
	tree, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return err
	}
	var violation string
	ast.Inspect(tree, func(node ast.Node) bool {
		lit, ok := node.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		if !sourceNames[value] {
			return true
		}
		pos := fset.Position(lit.Pos())
		rel, relErr := filepath.Rel(root, pos.Filename)
		if relErr != nil {
			rel = pos.Filename
		}
		if allowedConfiguredSourceNameLiteralSites[value][rel] {
			return true
		}
		violation = rel + ":" + strconv.Itoa(pos.Line) + " hardcodes configured source name " + strconv.Quote(value)
		return false
	})
	if violation != "" {
		t.Fatal(violation)
	}
	return nil
}
