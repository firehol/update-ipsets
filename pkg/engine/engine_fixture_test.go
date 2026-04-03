package engine

import (
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/downloader"
)

type engineFixtureOption func(*Engine)

func newEngineFixture(t *testing.T, opts ...engineFixtureOption) *Engine {
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
	}
	eng := &Engine{
		cfg:               config.New(),
		runtime:           rt,
		cachePath:         filepath.Join(rt.BaseDir, ".cache.json"),
		state:             cache.New(),
		downloads:         downloader.New(rt.MaxConnectTime, rt.MaxDownloadTime),
		logger:            slog.New(slog.DiscardHandler),
		now:               time.Now,
		backgroundLimiter: newBackgroundLimiter(rt.BackgroundWorkers()),
		geoProviders:      newGeoProviderCache(),
		asnLookupCache:    newASNDatabaseCache(),
		ledgerCache:       newRuntimeLedgerCache(),
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
	if eng.backgroundLimiter == nil {
		eng.backgroundLimiter = newBackgroundLimiter(eng.runtime.BackgroundWorkers())
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
