package observability

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestProductionCodeUsesNonBlockingMetricHelpers(t *testing.T) {
	t.Parallel()

	root := findRepoRoot(t)
	for _, dir := range []string{"cmd", "internal", "pkg"} {
		scanProductionGoFiles(t, filepath.Join(root, dir), func(path string, file *ast.File) {
			if strings.Contains(filepath.ToSlash(path), "/internal/observability/") {
				return
			}
			observabilityNames := importNames(file, "github.com/firehol/update-ipsets/internal/observability", "observability")
			if len(observabilityNames) == 0 {
				return
			}
			forbidden := map[string]struct{}{
				"APIRecalculation": {},
				"Bytes":            {},
				"Count":            {},
				"Duration":         {},
				"Gauge":            {},
				"Observe":          {},
			}
			ast.Inspect(file, func(node ast.Node) bool {
				sel, ok := node.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				ident, ok := sel.X.(*ast.Ident)
				if !ok {
					return true
				}
				if _, ok := observabilityNames[ident.Name]; !ok {
					return true
				}
				if _, ok := forbidden[sel.Sel.Name]; ok {
					t.Fatalf("%s calls synchronous metric helper %s.%s; production metrics must use Try* helpers", path, ident.Name, sel.Sel.Name)
				}
				return true
			})
		})
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found while searching for repository root")
		}
		dir = parent
	}
}

func scanProductionGoFiles(t *testing.T, root string, fn func(string, *ast.File)) {
	t.Helper()

	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "node_modules", "dist":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		fn(path, file)
		return nil
	})
	if err != nil {
		t.Fatalf("scan %s: %v", root, err)
	}
}

func importNames(file *ast.File, importPath, defaultName string) map[string]struct{} {
	names := map[string]struct{}{}
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != importPath {
			continue
		}
		name := defaultName
		if imp.Name != nil {
			name = imp.Name.Name
		}
		names[name] = struct{}{}
	}
	return names
}
