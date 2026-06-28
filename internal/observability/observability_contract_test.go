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

func TestNoOpenTelemetryImportsOutsideIsolatedExporter(t *testing.T) {
	t.Parallel()

	root := findRepoRoot(t)
	for _, dir := range []string{"cmd", "internal", "pkg", "tools"} {
		scanGoFiles(t, filepath.Join(root, dir), func(path string, file *ast.File) {
			slash := filepath.ToSlash(path)
			if strings.Contains(slash, "/internal/observability/otelexporter/") {
				return
			}
			for _, imp := range file.Imports {
				importPath, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					t.Fatalf("%s has invalid import path %s: %v", path, imp.Path.Value, err)
				}
				if strings.HasPrefix(importPath, "go.opentelemetry.io/") {
					t.Fatalf("%s imports %s; OTel is allowed only in internal/observability/otelexporter", path, importPath)
				}
			}
		})
	}
}

func TestProductionMetricCallsResolveToCompileTimeSchema(t *testing.T) {
	t.Parallel()

	root := findRepoRoot(t)
	metricFuncs := map[string]struct{}{
		"Count":       {},
		"Bytes":       {},
		"Duration":    {},
		"Gauge":       {},
		"Observe":     {},
		"TryCount":    {},
		"TryBytes":    {},
		"TryDuration": {},
		"TryGauge":    {},
		"TryObserve":  {},
	}
	for _, dir := range []string{"cmd", "internal", "pkg", "tools"} {
		scanGoFiles(t, filepath.Join(root, dir), func(path string, file *ast.File) {
			slash := filepath.ToSlash(path)
			if strings.Contains(slash, "/internal/observability/") || strings.HasSuffix(slash, "_test.go") {
				return
			}
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if _, ok := metricFuncs[selector.Sel.Name]; !ok {
					return true
				}
				pkg, ok := selector.X.(*ast.Ident)
				if !ok || pkg.Name != "observability" {
					return true
				}
				metricArg := 0
				switch selector.Sel.Name {
				case "Count", "Bytes", "Duration", "Gauge", "Observe":
					metricArg = 1
				}
				if len(call.Args) <= metricArg {
					t.Fatalf("%s calls observability.%s without metric name argument", path, selector.Sel.Name)
				}
				lit, ok := call.Args[metricArg].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					t.Fatalf("%s calls observability.%s with non-literal metric name; metric cardinality must be compile-time finite", path, selector.Sel.Name)
				}
				base, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatalf("%s has invalid metric literal %s: %v", path, lit.Value, err)
				}
				for _, name := range metricNamesForCall(selector.Sel.Name, base) {
					if _, ok := defaultRegistry.byName[name]; !ok {
						t.Fatalf("%s calls observability.%s(%q), which resolves to undeclared metric %q", path, selector.Sel.Name, base, name)
					}
				}
				return true
			})
		})
	}
}

func metricNamesForCall(fn, base string) []string {
	switch fn {
	case "Bytes", "TryBytes":
		return []string{base + ".bytes"}
	case "Duration", "TryDuration":
		return []string{base + ".duration_ms"}
	case "Observe", "TryObserve":
		return []string{base, base + ".bytes", base + ".duration_ms"}
	default:
		return []string{base}
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

func scanGoFiles(t *testing.T, root string, fn func(string, *ast.File)) {
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
		if !strings.HasSuffix(path, ".go") {
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
