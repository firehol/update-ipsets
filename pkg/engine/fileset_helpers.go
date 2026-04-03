package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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

// openLatestSet opens the binary latest file for name from the lib
// directory. If the binary file doesn't exist or can't be opened, it
// falls back to parsing the text .ipset/.netset file from BaseDir.
// The returned closableSource must be closed after use.
func (e *Engine) openLatestSet(ctx context.Context, name string) (*closableSource, error) {
	if e == nil || e.cfg == nil || !e.configuredNames()[name] {
		return nil, fmt.Errorf("unknown set %q", name)
	}
	for _, filename := range []string{"latest", "latest.set"} {
		binaryPath := filepath.Join(e.runtime.LibDir, name, filename)
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
	return e.loadTextSet(ctx, name)
}

func (e *Engine) hasBinaryLatestSet(name string) bool {
	if e == nil {
		return false
	}
	for _, filename := range []string{"latest", "latest.set"} {
		if fileExists(filepath.Join(e.runtime.LibDir, name, filename)) {
			return true
		}
	}
	return false
}

func (e *Engine) hasUsableSet(name string) bool {
	if e == nil {
		return false
	}
	if e.hasBinaryLatestSet(name) {
		return true
	}
	if entry := e.state.EntrySnapshot(name); entry != nil && entry.File != "" {
		return true
	}
	src := e.lookupSource(name)
	if src == nil {
		return false
	}
	return fileExists(e.finalPath(name, src.Output))
}

// loadTextSet loads a text .ipset or .netset file for name into memory
// and wraps it in a closableSource.
func (e *Engine) loadTextSet(ctx context.Context, name string) (*closableSource, error) {
	entry := e.state.Entry(name)
	if entry == nil || entry.File == "" {
		return nil, fmt.Errorf("set %q has no materialized file", name)
	}
	if !rawFeedFileMatches(name, entry.File) {
		return nil, fmt.Errorf("set %q has unexpected materialized file %q", name, entry.File)
	}
	path, ok := safeRuntimeFilePath(e.runtime.BaseDir, entry.File)
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

// collectIter materializes a range iterator into an in-memory IPSet.
// Used when the result must be written with IPSet.Write (print formats).
func collectIter(ctx context.Context, name string, iter func(yield func(iprange.Range) bool)) (*iprange.IPSet, error) {
	set := iprange.New(name)
	var count int
	for r := range iter {
		count++
		if count%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, fmt.Errorf("collectIter %s cancelled: %w", name, err)
			}
		}
		if err := set.AddRange(r); err != nil {
			slog.Warn("collectIter: failed to add range", "set", name, "lo", r.Lo, "hi", r.Hi, "error", err)
		}
	}
	set.Optimize()
	return set, nil
}

// checkFileSetErr checks whether a RangeSource (which may be a FileSet)
// encountered an I/O error during iteration. Returns the error if any.
func checkFileSetErr(src iprange.RangeSource, name string, logger *slog.Logger) error {
	type errChecker interface {
		Err() error
	}
	if ec, ok := src.(errChecker); ok {
		if err := ec.Err(); err != nil {
			logger.Warn("I/O error during FileSet operation", "set", name, "err", err)
			return fmt.Errorf("I/O error reading set %s: %w", name, err)
		}
	}
	return nil
}
