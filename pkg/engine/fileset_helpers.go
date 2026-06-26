package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/firehol/update-ipsets/pkg/iprange"
)

// closableSource wraps an iprange.RangeSource with a Close method,
// allowing uniform cleanup whether the source is a FileSet (needs
// Close to release mmap/fd) or an in-memory IPSet (Close is a no-op).
type closableSource struct {
	iprange.RangeSource
	close func() error
}

func (c *closableSource) Close() error {
	if c.close != nil {
		return c.close()
	}
	return nil
}

func closeClosableSources(srcs []*closableSource) error {
	var errs []error
	for _, src := range srcs {
		if err := src.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// UniqueIPs returns the unique IP count. For FileSet sources this
// reads the count from the binary header; for in-memory IPSets it
// calls UniqueCount().
func (c *closableSource) UniqueIPs() uint64 {
	if fs, ok := c.RangeSource.(iprange.FileSet); ok {
		return fs.UniqueIPs()
	}
	if set, ok := c.RangeSource.(*iprange.IPSet); ok {
		return set.UniqueCount()
	}
	return iprange.CountUniqueIter(c.RangeSource)
}

// Contains checks whether ip is in the set. For FileSet sources this
// uses binary search on file-backed data; for in-memory sets it uses
// the IPSet's Contains.
func (c *closableSource) Contains(ip uint32) bool {
	if fs, ok := c.RangeSource.(iprange.FileSet); ok {
		return fs.Contains(ip)
	}
	if set, ok := c.RangeSource.(*iprange.IPSet); ok {
		return set.Contains(ip)
	}
	return false
}

var (
	openLatestSetHookMu sync.Mutex
	openLatestSetHook   func(string)
)

func setOpenLatestSetHookForTest(fn func(string)) func() {
	openLatestSetHookMu.Lock()
	previous := openLatestSetHook
	openLatestSetHook = fn
	openLatestSetHookMu.Unlock()

	return func() {
		openLatestSetHookMu.Lock()
		openLatestSetHook = previous
		openLatestSetHookMu.Unlock()
	}
}

func openLatestSetHookForTest() func(string) {
	openLatestSetHookMu.Lock()
	defer openLatestSetHookMu.Unlock()
	return openLatestSetHook
}

// openLatestSet opens the binary latest file for name from the lib
// directory. If the binary file doesn't exist or can't be opened, it
// falls back to parsing the text .ipset/.netset file from BaseDir.
// The returned closableSource must be closed after use.
func (e *Engine) openLatestSet(ctx context.Context, name string) (*closableSource, error) {
	if e == nil {
		return nil, fmt.Errorf("unknown set %q", name)
	}
	cfg, rt := e.configRuntimeSnapshot()
	if cfg == nil || !configuredNamesForConfig(cfg)[name] {
		return nil, fmt.Errorf("unknown set %q", name)
	}
	if hook := openLatestSetHookForTest(); hook != nil {
		hook(name)
	}
	for _, filename := range []string{"latest", "latest.set"} {
		binaryPath := filepath.Join(rt.LibDir, name, filename)
		start := time.Now()
		fs, err := iprange.OpenFileSet(binaryPath)
		if err == nil {
			var bytes int64
			if info, statErr := os.Stat(binaryPath); statErr == nil {
				bytes = info.Size()
			}
			e.observeRunCounter("engine.latest_set.binary_open", 1, bytes)
			e.observeRunOperation("engine.latest_set.binary_open", time.Since(start))
			return &closableSource{RangeSource: fs, close: fs.Close}, nil
		}
	}
	return e.loadTextSetWithRuntime(ctx, name, rt)
}

func (e *Engine) hasBinaryLatestSet(name string) bool {
	if e == nil {
		return false
	}
	return hasBinaryLatestSetForRuntime(e.Runtime(), name)
}

func hasBinaryLatestSetForRuntime(rt Runtime, name string) bool {
	for _, filename := range []string{"latest", "latest.set"} {
		if fileExists(filepath.Join(rt.LibDir, name, filename)) {
			return true
		}
	}
	return false
}

func (e *Engine) hasUsableSet(name string) bool {
	if e == nil {
		return false
	}
	cfg, rt := e.configRuntimeSnapshot()
	if hasBinaryLatestSetForRuntime(rt, name) {
		return true
	}
	if entry := e.state.EntrySnapshot(name); entry != nil && entry.File != "" {
		return true
	}
	src := lookupSourceForConfig(cfg, name)
	if src == nil {
		return false
	}
	return fileExists(finalPathForRuntime(rt, name, src.Output))
}

func (e *Engine) loadTextSetWithRuntime(ctx context.Context, name string, rt Runtime) (*closableSource, error) {
	entry := e.state.EntrySnapshot(name)
	if entry == nil || entry.File == "" {
		return nil, fmt.Errorf("set %q has no materialized file", name)
	}
	if !rawFeedFileMatches(name, entry.File) {
		return nil, fmt.Errorf("set %q has unexpected materialized file %q", name, entry.File)
	}
	path, ok := safeRuntimeFilePath(rt.BaseDir, entry.File)
	if !ok {
		return nil, fmt.Errorf("set %q has unsafe materialized file %q", name, entry.File)
	}
	start := time.Now()
	set, err := iprange.LoadPath(ctx, path, iprange.DefaultParseOptions())
	if err != nil {
		return nil, err
	}
	var bytes int64
	if info, statErr := os.Stat(path); statErr == nil {
		bytes = info.Size()
	}
	e.observeRunCounter("engine.latest_set.text_parse", 1, bytes)
	e.observeRunOperation("engine.latest_set.text_parse", time.Since(start))
	set.Name = name
	return &closableSource{RangeSource: set, close: nil}, nil
}

// checkFileSetErr checks whether a RangeSource (which may be a FileSet)
// encountered an I/O error during iteration. Returns the error if any.
func checkFileSetErr(src iprange.RangeSource, name string, logger *slog.Logger) error {
	if err := iprange.RangeSourceErr(src); err != nil {
		logger.Warn("I/O error during FileSet operation", "set", name, "err", err)
		return fmt.Errorf("I/O error reading set %s: %w", name, err)
	}
	return nil
}
