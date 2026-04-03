package main

import (
	"go/ast"
	"sort"
	"strings"
)

func collectCacheAccess(file *ast.File, rel string, posture *Posture) {
	isTest := strings.HasSuffix(rel, "_test.go")
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isEntryCall(call) {
			return true
		}
		if isTest {
			posture.Cache.TestEntryCalls++
		} else {
			posture.Cache.ProductionEntryCalls++
		}
		return true
	})
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		collectCacheWritesInFunction(fn, rel, isTest, posture)
	}
}

func collectCacheWritesInFunction(fn *ast.FuncDecl, rel string, isTest bool, posture *Posture) {
	entryVars := map[string]bool{}
	if fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			if isCacheEntryType(field.Type) {
				for _, name := range field.Names {
					entryVars[name.Name] = true
				}
			}
		}
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, rhs := range assign.Rhs {
			if i >= len(assign.Lhs) || !exprIsEntryCall(rhs) {
				continue
			}
			if ident, ok := assign.Lhs[i].(*ast.Ident); ok {
				entryVars[ident.Name] = true
			}
		}
		for _, lhs := range assign.Lhs {
			switch {
			case selectorWritesEntry(lhs, entryVars):
				addCacheWrite(rel, isTest, posture)
			case starReplacesEntry(lhs):
				addCacheReplacement(rel, isTest, posture)
			}
		}
		return true
	})
}

func isCacheEntryType(expr ast.Expr) bool {
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	switch t := star.X.(type) {
	case *ast.Ident:
		return t.Name == "Entry"
	case *ast.SelectorExpr:
		return t.Sel.Name == "Entry"
	default:
		return false
	}
}

func exprIsEntryCall(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	return ok && isEntryCall(call)
}

func isEntryCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "Entry"
}

func selectorWritesEntry(expr ast.Expr, entryVars map[string]bool) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	switch x := sel.X.(type) {
	case *ast.Ident:
		return entryVars[x.Name]
	case *ast.CallExpr:
		return isEntryCall(x)
	default:
		return false
	}
}

func starReplacesEntry(expr ast.Expr) bool {
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	call, ok := star.X.(*ast.CallExpr)
	return ok && isEntryCall(call)
}

func addCacheWrite(rel string, isTest bool, posture *Posture) {
	if isTest {
		posture.Cache.TestFieldWrites++
		posture.Cache.TestMutationFiles = incrementFileCount(posture.Cache.TestMutationFiles, rel)
		return
	}
	posture.Cache.ProductionFieldWrites++
	posture.Cache.ProductionMutationFiles = incrementFileCount(posture.Cache.ProductionMutationFiles, rel)
}

func addCacheReplacement(rel string, isTest bool, posture *Posture) {
	if isTest {
		posture.Cache.TestEntryReplacements++
		posture.Cache.TestMutationFiles = incrementFileCount(posture.Cache.TestMutationFiles, rel)
		return
	}
	posture.Cache.ProductionEntryReplacements++
	posture.Cache.ProductionMutationFiles = incrementFileCount(posture.Cache.ProductionMutationFiles, rel)
}

func incrementFileCount(counts []FileCount, path string) []FileCount {
	for i := range counts {
		if counts[i].Path == path {
			counts[i].Count++
			return counts
		}
	}
	return append(counts, FileCount{Path: path, Count: 1})
}

func sortedFileCounts(counts []FileCount) []FileCount {
	sort.Slice(counts, func(i, j int) bool {
		if counts[i].Count == counts[j].Count {
			return counts[i].Path < counts[j].Path
		}
		return counts[i].Count > counts[j].Count
	})
	return counts
}
