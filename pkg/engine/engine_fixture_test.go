package engine

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/downloader"
)

type engineFixtureOption func(*Engine)

func newEngineFixture(t testing.TB, opts ...engineFixtureOption) *Engine {
	t.Helper()

	root := t.TempDir()
	rt := Runtime{
		BaseDir:              filepath.Join(root, "base"),
		HistoryDir:           filepath.Join(root, "history"),
		LibDir:               filepath.Join(root, "lib"),
		ErrorsDir:            filepath.Join(root, "errors"),
		CacheDir:             filepath.Join(root, "cache"),
		TmpDir:               filepath.Join(root, "tmp"),
		WebDir:               filepath.Join(root, "web"),
		WebDirForIPSets:      filepath.Join(root, "web", "files"),
		WebChartsEntries:     500,
		MaxProcessingWorkers: 1,
		MaxHeavyPhaseWorkers: 1,
		MaxBackgroundWorkers: 1,
		MaxEngineLaneWorkers: 1,
	}
	eng := &Engine{
		cfg:            config.New(),
		runtime:        rt,
		cachePath:      filepath.Join(rt.BaseDir, ".cache.json"),
		state:          cache.New(),
		downloads:      downloader.New(rt.MaxConnectTime, rt.MaxDownloadTime),
		logger:         slog.New(slog.DiscardHandler),
		now:            time.Now,
		engineLane:     NewWorkLane(rt.EngineLaneWorkers()),
		gitLane:        NewWorkLane(1),
		geoProviders:   newGeoProviderCache(),
		asnLookupCache: newASNDatabaseCache(),
		ledgerCache:    newRuntimeLedgerCache(),
	}
	for _, opt := range opts {
		opt(eng)
	}
	if eng.cfg == nil {
		eng.cfg = config.New()
	}
	if eng.state == nil {
		eng.state = cache.New()
	}
	if eng.logger == nil {
		eng.logger = slog.New(slog.DiscardHandler)
	}
	if eng.now == nil {
		eng.now = time.Now
	}
	if eng.engineLane == nil {
		eng.engineLane = NewWorkLane(eng.runtime.EngineLaneWorkers())
	} else {
		eng.engineLane.SetLimit(eng.runtime.EngineLaneWorkers())
	}
	if eng.gitLane == nil {
		eng.gitLane = NewWorkLane(1)
	}
	if eng.geoProviders == nil {
		eng.geoProviders = newGeoProviderCache()
	}
	if eng.asnLookupCache == nil {
		eng.asnLookupCache = newASNDatabaseCache()
	}
	if eng.ledgerCache == nil {
		eng.ledgerCache = newRuntimeLedgerCache()
	}
	if eng.downloads == nil {
		eng.downloads = downloader.New(eng.runtime.MaxConnectTime, eng.runtime.MaxDownloadTime)
	}
	if eng.cachePath == "" && eng.runtime.BaseDir != "" {
		eng.cachePath = filepath.Join(eng.runtime.BaseDir, ".cache.json")
	}
	if eng.querySetCache == nil {
		eng.querySetCache = newSharedLatestSetCache(eng)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = eng.StopCachePersistence(ctx)
		if eng.gitLane != nil {
			eng.gitLane.Shutdown(time.Second)
		}
	})
	return eng
}

func withConfig(cfg *config.Config) engineFixtureOption {
	return func(eng *Engine) {
		eng.cfg = cfg
	}
}

func withState(st *cache.State) engineFixtureOption {
	return func(eng *Engine) {
		eng.state = st
	}
}

func withRuntime(update func(*Runtime)) engineFixtureOption {
	return func(eng *Engine) {
		update(&eng.runtime)
	}
}

func withNow(now func() time.Time) engineFixtureOption {
	return func(eng *Engine) {
		eng.now = now
	}
}

func waitForEngineLaneIdle(t testing.TB, eng *Engine) {
	t.Helper()
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		lane := eng.StatusSnapshotLight().EngineLane
		if lane.ActiveCount == 0 && lane.WaitingCount == 0 {
			return
		}
		select {
		case <-deadline.C:
			t.Fatalf("engine lane did not settle: %#v", lane)
		case <-ticker.C:
		}
	}
}

func waitForCriticalCleanupRemoved(t testing.TB, eng *Engine, paths ...string) {
	t.Helper()
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		allGone := true
		for _, path := range paths {
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				allGone = false
				break
			}
		}
		lane := eng.StatusSnapshotLight().EngineLane
		if allGone && lane.ActiveCount == 0 && lane.WaitingCount == 0 {
			return
		}
		select {
		case <-deadline.C:
			for _, path := range paths {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("expected stale critical artifact or marker %q to be removed on reload, stat err = %v", path, err)
				}
			}
			t.Fatalf("critical cleanup lane did not settle: %#v", lane)
		case <-ticker.C:
		}
	}
}

func TestEngineTestsUseFixtureForDirectConstruction(t *testing.T) {
	files, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if file == "engine_fixture_test.go" {
			continue
		}
		fset := token.NewFileSet()
		tree, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		ast.Inspect(tree, func(node ast.Node) bool {
			lit, ok := node.(*ast.CompositeLit)
			if !ok {
				return true
			}
			ident, ok := lit.Type.(*ast.Ident)
			if !ok || ident.Name != "Engine" {
				return true
			}
			pos := fset.Position(lit.Pos())
			t.Fatalf("%s uses Engine literal directly; use newEngineFixture", pos)
			return false
		})
	}
}
