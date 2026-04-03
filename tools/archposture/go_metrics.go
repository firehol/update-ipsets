package main

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"strings"
)

func collectGoFile(root, path string, lines int, packages map[string]*PackageMetric, posture *Posture) error {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return err
	}
	rel := relPath(root, path)
	pkgPath := filepath.Dir(rel)
	pkg := packages[pkgPath]
	if pkg == nil {
		pkg = &PackageMetric{Package: file.Name.Name, Path: pkgPath}
		packages[pkgPath] = pkg
	}
	pkg.Files++
	pkg.Lines += lines
	imports := map[string]struct{}{}
	projectImports := map[string]struct{}{}
	for _, spec := range file.Imports {
		var importPath string
		if err := json.Unmarshal([]byte(spec.Path.Value), &importPath); err != nil {
			continue
		}
		imports[importPath] = struct{}{}
		if strings.HasPrefix(importPath, modulePath+"/") {
			projectImports[importPath] = struct{}{}
		}
	}
	if len(imports) > pkg.DirectImports {
		pkg.DirectImports = len(imports)
	}
	if len(projectImports) > pkg.ProjectImports {
		pkg.ProjectImports = len(projectImports)
	}
	collectFunctions(fset, file, rel, posture)
	collectCacheAccess(file, rel, posture)
	collectWebRoutes(file, posture)
	return nil
}

func collectFunctions(fset *token.FileSet, file *ast.File, rel string, posture *Posture) {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		start := fset.Position(fn.Pos()).Line
		end := fset.Position(fn.End()).Line
		lines := end - start + 1
		if lines < posture.Thresholds.ReviewFunctionLines {
			continue
		}
		posture.LargeFunctions = append(posture.LargeFunctions, FunctionMetric{
			Path:       rel,
			Name:       functionName(fn),
			StartLine:  start,
			Lines:      lines,
			Complexity: complexity(fn.Body),
		})
	}
}

func functionName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	return receiverName(fn.Recv.List[0].Type) + "." + fn.Name.Name
}

func receiverName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return "(*" + receiverName(t.X) + ")"
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return receiverName(t.X) + "." + t.Sel.Name
	default:
		return "receiver"
	}
}

func complexity(body *ast.BlockStmt) int {
	score := 1
	ast.Inspect(body, func(n ast.Node) bool {
		switch expr := n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			score++
		case *ast.CaseClause:
			if len(expr.List) > 0 {
				score++
			}
		case *ast.CommClause:
			score++
		case *ast.BinaryExpr:
			if expr.Op == token.LAND || expr.Op == token.LOR {
				score++
			}
		}
		return true
	})
	return score
}

func mergeGoList(root string, packages map[string]*PackageMetric) error {
	cmd := exec.Command("go", "list", "-json", "./...")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var item struct {
			ImportPath string
			Dir        string
			Imports    []string
			Deps       []string
		}
		if err := dec.Decode(&item); err != nil {
			return nil
		}
		rel := relPath(root, item.Dir)
		pkg := packages[rel]
		if pkg == nil {
			continue
		}
		pkg.DirectImports = len(item.Imports)
		pkg.TransitiveDeps = len(item.Deps)
	}
	return nil
}
