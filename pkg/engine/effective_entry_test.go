package engine

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/feedhealth"
)

func TestEntrySnapshotUsesParentLastChangeForRetentionDerivative(t *testing.T) {
	now := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	cfg := config.New()
	cfg.Runtime.FeedHealthSingleObservationGraceMins = 24 * 60
	cfg.Runtime.FeedHealthDefaultHealthyCadenceMins = 24 * 60
	cfg.Runtime.FeedHealthDefaultRiskyCadenceMins = 7 * 24 * 60
	cfg.Sources["parent"] = &config.Source{
		Name:     "parent",
		URL:      "https://example.test/parent.txt",
		Category: "attacks",
	}
	cfg.Sources["parent_1d"] = &config.Source{
		Name:        "parent_1d",
		URL:         config.InternalRetentionWindowScheme + "?parent=parent&minutes=1440",
		Category:    "attacks",
		DerivedFrom: []string{"parent"},
		Provenance:  config.ProvenanceSecondaryRetention,
	}

	eng := newEngineFixture(t, withConfig(cfg), withNow(func() time.Time { return now }))

	parentTS := now.Add(-20 * 24 * time.Hour).Unix()
	childTS := now.Unix()

	parent := eng.state.Entry("parent")
	parent.SourceDate = parentTS
	parent.ProcessedDate = parentTS
	parent.Version = 4
	parent.Entries = 10
	parent.AverageUpdateMins = 60

	child := eng.state.Entry("parent_1d")
	child.SourceDate = childTS
	child.ProcessedDate = childTS
	child.Version = 4
	child.Entries = 10
	child.AverageUpdateMins = 60

	view := eng.EntrySnapshot("parent_1d")
	if view == nil {
		t.Fatal("expected derivative entry snapshot")
	}
	if got, want := view.SourceDate, parentTS; got != want {
		t.Fatalf("effective source_date = %d, want %d", got, want)
	}

	health := eng.healthSnapshotFromFreshStateSnapshot("parent_1d", child)
	if health.Class != feedhealth.ClassUnmaintained {
		t.Fatalf("health class = %q, want %q", health.Class, feedhealth.ClassUnmaintained)
	}
}

func TestEntrySnapshotUsesParentFailureStateForRetentionDerivative(t *testing.T) {
	now := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	cfg := config.New()
	cfg.Runtime.FeedHealthSingleObservationGraceMins = 60
	cfg.Runtime.FeedHealthDefaultHealthyCadenceMins = 60
	cfg.Runtime.FeedHealthDefaultRiskyCadenceMins = 60
	cfg.Runtime.FeedHealthArchivalThresholdMins = 60
	cfg.Sources["parent"] = &config.Source{
		Name: "parent",
		URL:  "https://example.test/parent.txt",
	}
	cfg.Sources["parent_1d"] = &config.Source{
		Name:        "parent_1d",
		URL:         config.InternalRetentionWindowScheme + "?parent=parent&minutes=1440",
		DerivedFrom: []string{"parent"},
		Provenance:  config.ProvenanceSecondaryRetention,
	}

	eng := newEngineFixture(t, withConfig(cfg), withNow(func() time.Time { return now }))

	parent := eng.state.Entry("parent")
	parent.SourceDate = now.Add(-200 * time.Minute).Unix()
	parent.ProcessedDate = now.Add(-200 * time.Minute).Unix()
	parent.CheckedDate = now.Unix()
	parent.DownloadFailures = 5
	parent.FailureStartedDate = now.Add(-200 * time.Minute).Unix()
	parent.LastStatus = "download_failed"
	parent.Version = 3

	child := eng.state.Entry("parent_1d")
	child.SourceDate = now.Unix()
	child.ProcessedDate = now.Unix()
	child.Version = 3

	view := eng.EntrySnapshot("parent_1d")
	if view == nil {
		t.Fatal("expected derivative entry snapshot")
	}
	if got, want := view.DownloadFailures, parent.DownloadFailures; got != want {
		t.Fatalf("effective download_failures = %d, want %d", got, want)
	}
	if got, want := view.LastStatus, parent.LastStatus; got != want {
		t.Fatalf("effective last_status = %q, want %q", got, want)
	}

	health := eng.healthSnapshotFromFreshStateSnapshot("parent_1d", child)
	if health.Class != feedhealth.ClassArchived {
		t.Fatalf("health class = %q, want %q", health.Class, feedhealth.ClassArchived)
	}
}

func TestEntrySnapshotUsesNewestParentForMergeDerivative(t *testing.T) {
	now := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	cfg := config.New()
	cfg.Sources["a"] = &config.Source{
		Name: "a",
		URL:  "https://example.test/a.txt",
	}
	cfg.Sources["b"] = &config.Source{
		Name: "b",
		URL:  "https://example.test/b.txt",
	}
	cfg.Sources["combo"] = &config.Source{
		Name:        "combo",
		URL:         config.InternalMergeScheme + "?inputs=a,b",
		DerivedFrom: []string{"a", "b"},
		Provenance:  config.ProvenanceSecondaryMerge,
	}

	eng := newEngineFixture(t, withConfig(cfg), withNow(func() time.Time { return now }))

	aTS := now.Add(-20 * 24 * time.Hour).Unix()
	bTS := now.Add(-3 * 24 * time.Hour).Unix()

	a := eng.state.Entry("a")
	a.SourceDate = aTS
	b := eng.state.Entry("b")
	b.SourceDate = bTS

	combo := eng.state.Entry("combo")
	combo.SourceDate = now.Unix()

	view := eng.EntrySnapshot("combo")
	if view == nil {
		t.Fatal("expected merge entry snapshot")
	}
	if got, want := view.SourceDate, bTS; got != want {
		t.Fatalf("effective source_date = %d, want %d", got, want)
	}
}

func TestEffectiveEntryHelpersExposeSnapshotCost(t *testing.T) {
	bannedCheapNames := map[string]string{
		"entryView":       "use entryViewFromFreshStateSnapshot for single-entry paths or effectiveEntryResolver for batch paths",
		"healthSnapshot":  "use classifyEffectiveEntryHealth after resolving the entry explicitly",
		"feedHealthClass": "use feedHealthClassifier with an explicit batch lifetime",
	}
	bannedLoopCalls := map[string]string{
		"EntrySnapshot":                        "use EntriesSnapshot or an explicit effectiveEntryResolver outside the loop",
		"entryViewFromFreshStateSnapshot":      "move the fresh full-cache snapshot outside the loop",
		"healthSnapshotFromFreshStateSnapshot": "move the fresh full-cache snapshot outside the loop",
	}

	fset := token.NewFileSet()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.FuncDecl:
				if reason, banned := bannedCheapNames[n.Name.Name]; banned {
					t.Fatalf("%s defines %s: %s", fset.Position(n.Pos()), n.Name.Name, reason)
				}
			case *ast.CallExpr:
				name := callName(n)
				if reason, banned := bannedCheapNames[name]; banned {
					t.Fatalf("%s calls %s: %s", fset.Position(n.Pos()), name, reason)
				}
			}
			return true
		})
		ast.Inspect(file, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.ForStmt:
				assertNoLoopSnapshotCalls(t, fset, n.Body, bannedLoopCalls)
			case *ast.RangeStmt:
				assertNoLoopSnapshotCalls(t, fset, n.Body, bannedLoopCalls)
			}
			return true
		})
	}
}

func assertNoLoopSnapshotCalls(t *testing.T, fset *token.FileSet, body *ast.BlockStmt, banned map[string]string) {
	t.Helper()
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "EntrySnapshot" && isStateSelector(selector.X) {
			return true
		}
		name := callName(call)
		if reason, found := banned[name]; found {
			t.Fatalf("%s calls %s inside a loop: %s", fset.Position(call.Pos()), name, reason)
		}
		return true
	})
}

func callName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	default:
		return ""
	}
}

func isStateSelector(expr ast.Expr) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == "state"
}
