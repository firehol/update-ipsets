package main

import (
	"bufio"
	"go/ast"
	"os"
	"regexp"
	"strings"
)

var semanticShortcutPattern = regexp.MustCompile(`(Contains|HasPrefix|HasSuffix|TrimPrefix|TrimSuffix|includes|startsWith|endsWith)`)

var artifactTokens = []string{
	"_asn_",
	"_bogons_",
	"_comparison",
	"_countries",
	"_critical_",
	"_critical_infrastructure",
	"_history",
	"_insights",
	"_retention",
}

func semanticShortcutMatches(path, rel string) ([]LineMatch, error) {
	file, err := os.Open(path) // nosemgrep: repository architecture scanner intentionally reads tracked local source files.
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	var matches []LineMatch
	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		text := scanner.Text()
		if !semanticShortcutPattern.MatchString(text) {
			continue
		}
		for _, token := range artifactTokens {
			if strings.Contains(text, token) {
				matches = append(matches, LineMatch{Path: rel, Line: lineNo, Text: strings.TrimSpace(text)})
				break
			}
		}
	}
	return matches, scanner.Err()
}

func collectWebRoutes(file *ast.File, posture *Posture) {
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch sel.Sel.Name {
		case "HandleFunc":
			posture.WebRoutes.MuxHandleFuncCalls++
		case "Handle":
			posture.WebRoutes.MuxHandleCalls++
		}
		return true
	})
}
