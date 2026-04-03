package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestArchitecturePostureDoesNotRegress(t *testing.T) {
	root := repoRoot(t)
	baseline := loadBaseline(t)
	current, err := Collect(root)
	if err != nil {
		t.Fatal(err)
	}

	assertNoNewLargeFiles(t, baseline, current)
	assertNoNewLargeFunctions(t, baseline, current)
	assertNotIncreased(t, "production cache Entry() calls", baseline.Cache.ProductionEntryCalls, current.Cache.ProductionEntryCalls)
	assertNotIncreased(t, "production cache field writes", baseline.Cache.ProductionFieldWrites, current.Cache.ProductionFieldWrites)
	assertNotIncreased(t, "production cache entry replacements", baseline.Cache.ProductionEntryReplacements, current.Cache.ProductionEntryReplacements)
	assertNotIncreased(t, "semantic shortcut matches", len(baseline.SemanticShortcutMatches), len(current.SemanticShortcutMatches))
	assertIPRangeStandalone(t, current)
}

func assertNoNewLargeFiles(t *testing.T, baseline, current Posture) {
	t.Helper()
	known := make(map[string]int, len(baseline.LargeFiles))
	for _, file := range baseline.LargeFiles {
		known[file.Path] = file.Lines
	}
	for _, file := range current.LargeFiles {
		base, ok := known[file.Path]
		if !ok && file.Lines >= current.Thresholds.NewLargeFileLines {
			t.Fatalf("new large file %s has %d lines; update the architecture baseline only with SOW approval", file.Path, file.Lines)
		}
		if ok && file.Lines > base+25 {
			t.Fatalf("large file %s grew from %d to %d lines; update the architecture baseline only with SOW approval", file.Path, base, file.Lines)
		}
	}
}

func assertNoNewLargeFunctions(t *testing.T, baseline, current Posture) {
	t.Helper()
	known := make(map[string]int, len(baseline.LargeFunctions))
	for _, fn := range baseline.LargeFunctions {
		known[fn.Path+"::"+fn.Name] = fn.Lines
	}
	for _, fn := range current.LargeFunctions {
		key := fn.Path + "::" + fn.Name
		base, ok := known[key]
		if !ok && fn.Lines >= current.Thresholds.NewLargeFunctionLines {
			t.Fatalf("new large function %s %s has %d lines; update the architecture baseline only with SOW approval", fn.Path, fn.Name, fn.Lines)
		}
		if ok && fn.Lines > base+25 {
			t.Fatalf("large function %s %s grew from %d to %d lines; update the architecture baseline only with SOW approval", fn.Path, fn.Name, base, fn.Lines)
		}
	}
}

func assertNotIncreased(t *testing.T, label string, baseline, current int) {
	t.Helper()
	if current > baseline {
		t.Fatalf("%s increased from %d to %d; update the architecture baseline only with SOW approval", label, baseline, current)
	}
}

func assertIPRangeStandalone(t *testing.T, posture Posture) {
	t.Helper()
	for _, pkg := range posture.Packages {
		if pkg.Path == "pkg/iprange" {
			if pkg.ProjectImports != 0 {
				t.Fatalf("pkg/iprange imports %d project package(s); it must stay standalone", pkg.ProjectImports)
			}
			return
		}
	}
	t.Fatal("pkg/iprange package not found in architecture posture")
}

func loadBaseline(t *testing.T) Posture {
	t.Helper()
	path := filepath.Join("testdata", "posture_baseline.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	var posture Posture
	if err := json.Unmarshal(data, &posture); err != nil {
		t.Fatalf("parse baseline: %v", err)
	}
	return posture
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, ".agents", "sow", "specs")); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repository root")
		}
		dir = parent
	}
}
