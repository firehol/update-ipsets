package web

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

type parsedWebFile struct {
	name string
	file *ast.File
}

func TestWebServingDoesNotImportOtelHTTP(t *testing.T) {
	t.Parallel()

	for _, parsed := range parseWebSourceFiles(t) {
		for _, imp := range parsed.file.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatalf("unquote import path %s: %v", imp.Path.Value, err)
			}
			if path == "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp" {
				t.Fatalf("%s imports otelhttp; web serving telemetry must be non-blocking", parsed.name)
			}
		}
	}
}

func TestWebServingDoesNotCallSynchronousObservability(t *testing.T) {
	t.Parallel()

	for _, parsed := range parseWebSourceFiles(t) {
		observabilityNames := map[string]struct{}{}
		for _, imp := range parsed.file.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatalf("unquote import path %s: %v", imp.Path.Value, err)
			}
			if path != "github.com/firehol/update-ipsets/internal/observability" {
				continue
			}
			name := "observability"
			if imp.Name != nil {
				name = imp.Name.Name
			}
			observabilityNames[name] = struct{}{}
		}
		if len(observabilityNames) == 0 {
			continue
		}

		forbidden := map[string]struct{}{
			"APIRecalculation": {},
			"Bytes":            {},
			"Count":            {},
			"Duration":         {},
			"End":              {},
			"Gauge":            {},
			"Observe":          {},
			"Start":            {},
		}
		ast.Inspect(parsed.file, func(node ast.Node) bool {
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
				t.Fatalf("%s calls synchronous observability helper %s.%s", parsed.name, ident.Name, sel.Sel.Name)
			}
			return true
		})
	}
}

func TestWebServingDoesNotCallOpenTelemetryTracing(t *testing.T) {
	t.Parallel()

	for _, parsed := range parseWebSourceFiles(t) {
		observabilityNames := map[string]struct{}{}
		for _, imp := range parsed.file.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatalf("unquote import path %s: %v", imp.Path.Value, err)
			}
			if path != "github.com/firehol/update-ipsets/internal/observability" {
				continue
			}
			name := "observability"
			if imp.Name != nil {
				name = imp.Name.Name
			}
			observabilityNames[name] = struct{}{}
		}
		if len(observabilityNames) == 0 {
			continue
		}

		ast.Inspect(parsed.file, func(node ast.Node) bool {
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
			switch sel.Sel.Name {
			case "Start", "End":
				t.Fatalf("%s calls trace helper %s.%s; web serving must not add OpenTelemetry trace work to request/liveness paths", parsed.name, ident.Name, sel.Sel.Name)
			}
			return true
		})
	}
}

func TestWebServingDoesNotCallBlockingEngineTelemetry(t *testing.T) {
	t.Parallel()

	for _, parsed := range parseWebSourceFiles(t) {
		ast.Inspect(parsed.file, func(node ast.Node) bool {
			sel, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			switch sel.Sel.Name {
			case "ObserveOperation", "ObserveCounter":
				t.Fatalf("%s calls %s; web request telemetry must use best-effort TryObserve* helpers", parsed.name, sel.Sel.Name)
			}
			return true
		})
	}
}

func TestWebServingDoesNotRefreshRuntimeStatusSynchronously(t *testing.T) {
	t.Parallel()

	for _, parsed := range parseWebSourceFiles(t) {
		ast.Inspect(parsed.file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			ident, ok := call.Fun.(*ast.Ident)
			if !ok || ident.Name != "detailedStatus" {
				return true
			}
			t.Fatalf("%s calls detailedStatus; web serving must use sampled cached runtime stats", parsed.name)
			return true
		})
	}
}

func TestWebServingDoesNotCallBlockingEngineSnapshots(t *testing.T) {
	t.Parallel()

	allowedFunctions := map[string]struct{}{
		"queueStartupIntegrityRecovery": {},
	}
	for _, parsed := range parseWebSourceFiles(t) {
		for _, decl := range parsed.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if _, ok := allowedFunctions[fn.Name.Name]; ok {
				continue
			}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				switch sel.Sel.Name {
				case "Config", "Runtime", "ConfigRuntimeSnapshot", "ConfigRuntimePolicySnapshot", "StatusSnapshot", "StatusSnapshotLight",
					"PipelineIntegrityCacheSnapshot", "EntityIntegrityCacheSnapshot":
					t.Fatalf("%s calls blocking engine snapshot %s inside %s; web serving must use Try* snapshots or cached state",
						parsed.name, sel.Sel.Name, fn.Name.Name)
				}
				return true
			})
		}
	}
}

func TestWebServingDoesNotCallDirectEngineCleanup(t *testing.T) {
	t.Parallel()

	forbidden := map[string]struct{}{
		"CleanupPublishStagesBefore":                         {},
		"CleanupStaleCriticalInfrastructureArtifacts":        {},
		"CleanupStaleCriticalInfrastructureArtifactsContext": {},
		"CleanupStalePublishStages":                          {},
	}
	for _, parsed := range parseWebSourceFiles(t) {
		ast.Inspect(parsed.file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if _, ok := forbidden[sel.Sel.Name]; ok {
				t.Fatalf("%s calls direct cleanup helper %s; web serving cleanup must run after listeners through context-aware lane/background helpers", parsed.name, sel.Sel.Name)
			}
			return true
		})
	}
}

func TestWebServingDoesNotBuildFreshMergeCompositions(t *testing.T) {
	t.Parallel()

	for _, parsed := range parseWebSourceFiles(t) {
		ast.Inspect(parsed.file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if sel.Sel.Name == "MergeCompositionsForConfigRuntimePolicy" || sel.Sel.Name == "MergeCompositions" || sel.Sel.Name == "MergeComposition" {
				t.Fatalf("%s calls %s; admin serving must use cached scheduler rows for merge detail instead of fresh engine/cache snapshots", parsed.name, sel.Sel.Name)
			}
			return true
		})
	}
}

func TestWebServingDoesNotCallEngineBackedPublicCatalog(t *testing.T) {
	t.Parallel()

	forbidden := map[string]struct{}{
		"ASNProviders":                    {},
		"BogonProviders":                  {},
		"CriticalInfrastructureProviders": {},
		"Entry":                           {},
		"GeoProviders":                    {},
		"IsCriticalInfrastructureTarget":  {},
		"IsPublicFeedName":                {},
		"NewEngineFeedCatalog":            {},
		"PublicCategories":                {},
		"PublicFeedSummaries":             {},
		"PublicRawFeedFile":               {},
	}
	for _, parsed := range parseWebSourceFiles(t) {
		ast.Inspect(parsed.file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if _, ok := forbidden[sel.Sel.Name]; ok {
				t.Fatalf("%s calls %s; public web serving must use cached publicServingState catalog data", parsed.name, sel.Sel.Name)
			}
			return true
		})
	}
}

func TestWebServingDoesNotComputeIntegrityRecoveryPlans(t *testing.T) {
	t.Parallel()

	allowedFunctions := map[string]struct{}{
		"queueStartupIntegrityRecovery": {},
	}
	for _, parsed := range parseWebSourceFiles(t) {
		for _, decl := range parsed.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if _, ok := allowedFunctions[fn.Name.Name]; ok {
				continue
			}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if sel.Sel.Name == "IntegrityRecoveryPlan" {
					t.Fatalf("%s calls IntegrityRecoveryPlan inside %s; web serving must read cached recovery hints or queue engine-lane work", parsed.name, fn.Name.Name)
				}
				return true
			})
		}
	}
}

func TestWebServingDoesNotCallEngineBackedActivitySnapshot(t *testing.T) {
	t.Parallel()

	for _, parsed := range parseWebSourceFiles(t) {
		ast.Inspect(parsed.file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if sel.Sel.Name == "ActivitySnapshot" {
				t.Fatalf("%s calls ActivitySnapshot; admin serving must use ActivitySnapshotLight to avoid engine/cache lookups", parsed.name)
			}
			if sel.Sel.Name == "CachedSnapshot" {
				t.Fatalf("%s calls CachedSnapshot; admin serving must use TryCachedSnapshot through cachedSchedulerSnapshot", parsed.name)
			}
			return true
		})
	}
}

func TestWebServingDoesNotCallBlockingAdminActionPreflights(t *testing.T) {
	t.Parallel()

	forbidden := map[string]struct{}{
		"ResolveRecheckTarget":   {},
		"HasLocalReprocessState": {},
		"Enable":                 {},
		"Disable":                {},
		"EnableArtifacts":        {},
		"DisableArtifacts":       {},
	}
	for _, parsed := range parseWebSourceFiles(t) {
		ast.Inspect(parsed.file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if _, ok := forbidden[sel.Sel.Name]; ok {
				t.Fatalf("%s calls blocking admin action helper %s; web serving must use Try* helpers", parsed.name, sel.Sel.Name)
			}
			return true
		})
	}
}

func TestWebServingMiddlewareDoesNotUseConfiguredLogger(t *testing.T) {
	t.Parallel()

	for _, parsed := range parseWebSourceFiles(t) {
		ast.Inspect(parsed.file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			ident, ok := call.Fun.(*ast.Ident)
			if !ok {
				return true
			}
			if ident.Name != "logMiddleware" && ident.Name != "recoverMiddleware" {
				return true
			}
			if len(call.Args) == 0 {
				t.Fatalf("%s calls %s without a logger argument", parsed.name, ident.Name)
			}
			if selector, ok := call.Args[0].(*ast.SelectorExpr); ok && selector.Sel.Name == "Logger" {
				t.Fatalf("%s passes configured logger directly to %s; request-path logs must not use telemetry-backed logging", parsed.name, ident.Name)
			}
			return true
		})
	}
}

func TestWebServingLifecycleDoesNotUseConfiguredLogger(t *testing.T) {
	t.Parallel()

	forbiddenFunctions := map[string]bool{
		"prepareEngineForRun":             false,
		"queueStartupIntegrityRecovery":   false,
		"startRunBackgroundWork":          false,
		"startStartupIntegrityRecovery":   false,
		"startStartupEntityArtifacts":     false,
		"startDelayedPublishStageCleanup": false,
		"startRunShutdownWatcher":         false,
		"startRunWatchdog":                false,
		"sendRunWatchdogTick":             false,
		"announceRunReady":                false,
		"notifyRunReady":                  false,
	}

	for _, parsed := range parseWebSourceFiles(t) {
		for _, decl := range parsed.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if _, ok := forbiddenFunctions[fn.Name.Name]; !ok {
				continue
			}
			forbiddenFunctions[fn.Name.Name] = true
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				selector, ok := node.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "Logger" {
					return true
				}
				ident, ok := selector.X.(*ast.Ident)
				if !ok || ident.Name != "opts" {
					return true
				}
				t.Fatalf("%s uses opts.Logger inside %s; web-serving lifecycle logs must use serving-safe logging", parsed.name, fn.Name.Name)
				return true
			})
		}
	}

	for name, found := range forbiddenFunctions {
		if !found {
			t.Fatalf("web-serving lifecycle logger guard did not find %s", name)
		}
	}
}

func parseWebSourceFiles(t *testing.T) []parsedWebFile {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read web package dir: %v", err)
	}
	fset := token.NewFileSet()
	files := make([]parsedWebFile, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Clean(name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files = append(files, parsedWebFile{name: name, file: file})
	}
	return files
}
