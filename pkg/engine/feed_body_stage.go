package engine

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/downloader"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

func (e *Engine) prepareCanonicalFeedBody(ctx context.Context, name string, src *config.Source, inputPath string) ([]byte, *iprange.IPSet, error) {
	return e.prepareCanonicalFeedBodyWithSnapshot(ctx, e.operationSnapshot(), name, src, inputPath)
}

func (e *Engine) prepareCanonicalFeedBodyWithSnapshot(ctx context.Context, snap operationSnapshot, name string, src *config.Source, inputPath string) ([]byte, *iprange.IPSet, error) {
	return downloader.PrepareCanonicalFeedBody(ctx, name, src.Output, inputPath, processorSteps(src), snap.runtime.TmpDir, snap.runtime.ParallelDNSQueries)
}

func renderCanonicalFeedBody(set *iprange.IPSet) ([]byte, error) {
	return downloader.RenderCanonicalFeedBody(set, "netset")
}

func parseFeedBodyReader(ctx context.Context, name string, r io.Reader, dnsThreads int) (*iprange.IPSet, error) {
	opts := iprange.DefaultParseOptions()
	opts.DefaultPrefix = 32
	opts.DNSThreads = dnsThreads
	return downloader.ParseCanonicalFeedReader(ctx, name, r, opts)
}

func parseFeedBodyBytes(ctx context.Context, name string, body []byte, dnsThreads int) (*iprange.IPSet, error) {
	return parseFeedBodyReader(ctx, name, bytes.NewReader(body), dnsThreads)
}

func parseFeedBodyFile(ctx context.Context, name, path string, dnsThreads int) (*iprange.IPSet, error) {
	return downloader.ParseCanonicalFeedFile(ctx, name, path, dnsThreads)
}

func stageFeedBodyBytes(dst string, body []byte) error {
	tmpPath := pendingTempPath(dst)
	stagePath := stagedPath(dst)
	if err := os.MkdirAll(filepath.Dir(dst), generatedDirMode); err != nil {
		return err
	}
	_ = os.Remove(tmpPath)
	_ = os.Remove(stagePath)
	if err := writeFileAtomic(tmpPath, body, generatedFileMode); err != nil {
		return err
	}
	return os.Rename(tmpPath, stagePath)
}

func writeFeedBodyAtomic(path string, header []byte, bodyPath string, mod time.Time) error {
	if err := os.MkdirAll(filepath.Dir(path), generatedDirMode); err != nil {
		return err
	}
	body, err := openFilePathUnderRoot(filepath.Dir(bodyPath), bodyPath)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		_ = body.Close()
		return err
	}
	tmpName := tmp.Name()
	closed := false
	closeTmp := func() {
		if !closed {
			_ = tmp.Close()
			closed = true
		}
	}
	defer func() {
		closeTmp()
		_ = os.Remove(tmpName)
	}()

	if len(header) > 0 {
		if _, err := tmp.Write(header); err != nil {
			_ = body.Close()
			return err
		}
	}
	if err := copyNonCommentLines(tmp, body); err != nil {
		_ = body.Close()
		return err
	}
	if err := body.Close(); err != nil {
		return err
	}
	if err := tmp.Chmod(generatedFileMode); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	closed = true
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	if mod.IsZero() {
		return nil
	}
	return touchFileAt(path, mod.UTC())
}

func canonicalFeedBodySame(dst string, body []byte) (bool, error) {
	if !fileExists(dst) {
		return false, nil
	}
	existing, err := openFilePathUnderRoot(filepath.Dir(dst), dst)
	if err != nil {
		return false, err
	}
	defer func() { _ = existing.Close() }()
	return nonCommentReadersEqual(existing, bytes.NewReader(body))
}

func nonCommentReadersEqual(left io.Reader, right io.Reader) (bool, error) {
	leftReader := bufio.NewReader(left)
	rightReader := bufio.NewReader(right)
	for {
		leftLine, leftErr := nextNonCommentLine(leftReader)
		rightLine, rightErr := readTextLine(rightReader)
		switch {
		case leftErr == nil && rightErr == nil:
			if !bytes.Equal(leftLine, rightLine) {
				return false, nil
			}
		case errors.Is(leftErr, io.EOF) && errors.Is(rightErr, io.EOF):
			return true, nil
		case errors.Is(leftErr, io.EOF) || errors.Is(rightErr, io.EOF):
			return false, nil
		case leftErr != nil:
			return false, leftErr
		default:
			return false, rightErr
		}
	}
}

func copyNonCommentLines(dst io.Writer, src io.Reader) error {
	reader := bufio.NewReader(src)
	for {
		line, err := nextNonCommentLine(reader)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if _, err := dst.Write(line); err != nil {
			return err
		}
	}
}

func nonCommentLineStrings(src io.Reader) ([]string, error) {
	reader := bufio.NewReader(src)
	var lines []string
	for {
		line, err := nextNonCommentLine(reader)
		if errors.Is(err, io.EOF) {
			return lines, nil
		}
		if err != nil {
			return nil, err
		}
		trimmed := strings.TrimSpace(string(line))
		if trimmed == "" {
			continue
		}
		lines = append(lines, trimmed)
	}
}

func nextNonCommentLine(reader *bufio.Reader) ([]byte, error) {
	for {
		line, err := readTextLine(reader)
		if err != nil {
			return nil, err
		}
		if isCommentLine(line) {
			continue
		}
		return line, nil
	}
}

func readTextLine(reader *bufio.Reader) ([]byte, error) {
	line, err := reader.ReadBytes('\n')
	if err == nil {
		return line, nil
	}
	if errors.Is(err, io.EOF) {
		if len(line) > 0 {
			return line, nil
		}
		return nil, io.EOF
	}
	return nil, err
}

func isCommentLine(line []byte) bool {
	line = bytes.TrimPrefix(line, []byte("\xef\xbb\xbf"))
	line = bytes.TrimLeft(line, " \t\r")
	return len(line) > 0 && line[0] == '#'
}

type historySnapshot struct {
	name string
	path string
	when time.Time
}

func parseHistorySnapshot(entryName, path string) (historySnapshot, bool) {
	if !strings.HasSuffix(entryName, ".set") {
		return historySnapshot{}, false
	}
	base := strings.TrimSuffix(entryName, ".set")
	if ts, err := strconv.ParseInt(base, 10, 64); err == nil && ts > 0 {
		return historySnapshot{
			name: entryName,
			path: path,
			when: time.Unix(ts, 0).UTC(),
		}, true
	}
	if day, err := time.Parse("2006-01-02", base); err == nil {
		return historySnapshot{
			name: entryName,
			path: path,
			when: day.UTC(),
		}, true
	}
	return historySnapshot{}, false
}

func readHistorySnapshots(dir string) ([]historySnapshot, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	snapshots := make([]historySnapshot, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if snapshot, ok := parseHistorySnapshot(entry.Name(), filepath.Join(dir, entry.Name())); ok {
			snapshots = append(snapshots, snapshot)
		}
	}
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].when.After(snapshots[j].when)
	})
	return snapshots, nil
}

func historySnapshotObservedTime(observedAt time.Time) time.Time {
	if observedAt.IsZero() {
		return time.Now().UTC().Truncate(time.Second)
	}
	return observedAt.UTC().Truncate(time.Second)
}

func historySnapshotPath(dir string, observedAt time.Time) (string, time.Time, error) {
	when := historySnapshotObservedTime(observedAt)
	if when.Unix() <= 0 {
		return "", time.Time{}, fmt.Errorf("history snapshot requires a positive timestamp")
	}
	return filepath.Join(dir, fmt.Sprintf("%d.set", when.Unix())), when, nil
}

func (e *Engine) appendHistorySnapshot(ctx context.Context, parent string, set *iprange.IPSet, observedAt time.Time) (bool, error) {
	return e.appendHistorySnapshotWithSnapshot(ctx, e.operationSnapshot(), parent, set, observedAt)
}

func (e *Engine) appendHistorySnapshotWithSnapshot(ctx context.Context, snap operationSnapshot, parent string, set *iprange.IPSet, observedAt time.Time) (bool, error) {
	if parent == "" || set == nil {
		return false, fmt.Errorf("history snapshot requires parent and set")
	}
	if e == nil || snap.retentionMaxWindow == nil {
		return false, nil
	}
	if window, ok := snap.retentionMaxWindow[parent]; !ok || window <= 0 {
		return false, nil
	}
	dir := filepath.Join(snap.runtime.HistoryDir, parent)
	if err := os.MkdirAll(dir, generatedDirMode); err != nil {
		return false, err
	}
	slot, snapshotTime, err := historySnapshotPath(dir, observedAt)
	if err != nil {
		return false, err
	}
	if fileExists(slot) {
		existing, err := iprange.OpenFileSet(slot)
		if err != nil {
			return false, fmt.Errorf("open history snapshot %s: %w", slot, err)
		}
		defer func() { _ = existing.Close() }()
		equal, err := iprange.RangeSourcesEqualContext(ctx, set, existing)
		if err != nil {
			return false, fmt.Errorf("compare history snapshot %s: %w", slot, err)
		}
		if equal {
			pruned, err := e.pruneHistorySnapshotsWithSnapshot(parent, snap, snapshotTime)
			if err != nil {
				return false, err
			}
			return pruned, nil
		}
	}
	if err := writeBinaryPath(slot, set, snapshotTime); err != nil {
		return false, err
	}
	_, err = e.pruneHistorySnapshotsWithSnapshot(parent, snap, snapshotTime)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (e *Engine) pruneHistorySnapshots(parent string, referenceTime time.Time) (bool, error) {
	return e.pruneHistorySnapshotsWithSnapshot(parent, e.operationSnapshot(), referenceTime)
}

func (e *Engine) pruneHistorySnapshotsWithSnapshot(parent string, snap operationSnapshot, referenceTime time.Time) (bool, error) {
	if e == nil || snap.retentionMaxWindow == nil {
		return false, nil
	}
	window, ok := snap.retentionMaxWindow[parent]
	if !ok || window <= 0 {
		return false, nil
	}
	dir := filepath.Join(snap.runtime.HistoryDir, parent)
	snapshots, err := readHistorySnapshots(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	referenceTime = historySnapshotObservedTime(referenceTime)
	cutoff := referenceTime.Add(-window)
	pruned := false
	for _, snapshot := range snapshots {
		if !snapshot.when.After(cutoff) {
			if err := os.Remove(snapshot.path); err != nil && !os.IsNotExist(err) {
				e.logger.Warn("history snapshot prune failed", "parent", parent, "file", snapshot.name, "error", err)
				continue
			}
			pruned = true
		}
	}
	return pruned, nil
}

func (e *Engine) historyDerivativeWindowDuration(src *config.Source) time.Duration {
	if src == nil {
		return 0
	}
	if strings.HasPrefix(src.URL, config.InternalRetentionWindowScheme) {
		_, minutes, err := config.ParseRetentionWindowURL(src.URL)
		if err == nil && minutes > 0 {
			return time.Duration(minutes) * time.Minute
		}
	}
	if src.HistoryWindowDays > 0 {
		return time.Duration(src.HistoryWindowDays) * 24 * time.Hour
	}
	return 0
}

func historyDerivativeReferenceTime(path string, fallback time.Time) time.Time {
	if info, err := os.Stat(path); err == nil && !info.ModTime().IsZero() {
		return historySnapshotObservedTime(info.ModTime())
	}
	return historySnapshotObservedTime(fallback)
}

func historySnapshotWithinWindow(snapshot historySnapshot, cutoff time.Time) bool {
	return snapshot.when.After(cutoff)
}

func (e *Engine) historyDerivativeSnapshots(parent string) ([]historySnapshot, error) {
	return historyDerivativeSnapshotsForRuntime(e.operationSnapshot().runtime, parent)
}

func historyDerivativeSnapshotsForRuntime(rt Runtime, parent string) ([]historySnapshot, error) {
	dir := filepath.Join(rt.HistoryDir, parent)
	snapshots, err := readHistorySnapshots(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("history derivative read snapshots: %w", err)
	}
	return snapshots, nil
}

func (e *Engine) composeHistoryDerivativeBody(ctx context.Context, src *config.Source) ([]byte, *iprange.IPSet, error) {
	return e.composeHistoryDerivativeBodyWithSnapshot(ctx, e.operationSnapshot(), src)
}

func (e *Engine) composeHistoryDerivativeBodyWithSnapshot(ctx context.Context, snap operationSnapshot, src *config.Source) ([]byte, *iprange.IPSet, error) {
	if src == nil || len(src.DerivedFrom) == 0 {
		return nil, nil, fmt.Errorf("history derivative missing parent")
	}
	parent := src.DerivedFrom[0]
	parentPath := latestFeedBodyPath(snap.feedBodyPath(parent))
	parentSet, err := parseFeedBodyFile(ctx, parent, parentPath, snap.runtime.ParallelDNSQueries)
	if err != nil {
		return nil, nil, fmt.Errorf("history derivative parent body %s: %w", parentPath, err)
	}

	window := e.historyDerivativeWindowDuration(src)
	if window <= 0 {
		return nil, nil, fmt.Errorf("history derivative %q has invalid window", src.Name)
	}
	referenceTime := historyDerivativeReferenceTime(parentPath, e.now().UTC())
	cutoff := referenceTime.Add(-window)
	snapshots, err := historyDerivativeSnapshotsForRuntime(snap.runtime, parent)
	if err != nil {
		return nil, nil, err
	}

	rangeSources := make([]iprange.RangeSource, 0, len(snapshots)+1)
	rangeSources = append(rangeSources, parentSet)
	fileSets := make([]iprange.FileSet, 0, len(snapshots))
	defer func() {
		for _, fs := range fileSets {
			_ = fs.Close()
		}
	}()
	for _, snapshot := range snapshots {
		if !historySnapshotWithinWindow(snapshot, cutoff) {
			continue
		}
		fs, err := iprange.OpenFileSet(snapshot.path)
		if err != nil {
			return nil, nil, fmt.Errorf("history derivative open snapshot %s: %w", snapshot.path, err)
		}
		fileSets = append(fileSets, fs)
		rangeSources = append(rangeSources, fs)
	}

	union, err := iprange.UnionSourcesContext(ctx, src.Name, rangeSources...)
	if err != nil {
		return nil, nil, fmt.Errorf("history derivative collect union: %w", err)
	}
	union.Optimize()
	body, err := downloader.RenderCanonicalFeedBody(union, src.Output)
	if err != nil {
		return nil, nil, err
	}
	return body, union, nil
}

func (e *Engine) composeMergeBody(ctx context.Context, src *config.Source, enableAll bool) ([]byte, *iprange.IPSet, string, error) {
	return e.composeMergeBodyWithSnapshot(ctx, e.operationSnapshot(), src, enableAll)
}

func (e *Engine) composeMergeBodyWithSnapshot(ctx context.Context, snap operationSnapshot, src *config.Source, enableAll bool) ([]byte, *iprange.IPSet, string, error) {
	if src == nil {
		return nil, nil, "", fmt.Errorf("nil merge source")
	}
	composition := e.mergeCompositionWithSnapshot(src, enableAll, snap)
	if composition.eligibleSourceCount == 0 {
		return nil, nil, "merge disabled: no currently eligible inputs", nil
	}
	if len(composition.unavailableSubtractive) > 0 {
		return nil, nil, "", fmt.Errorf("merge: unavailable subtractive inputs for %v", composition.unavailableSubtractive)
	}
	if len(composition.missingFeedBodies) > 0 {
		return nil, nil, "", fmt.Errorf("merge: missing committed feed bodies for %v", composition.missingFeedBodies)
	}
	readers, closeReaders, err := e.mergeInputReadersWithSnapshot(snap, composition.Included)
	if err != nil {
		return nil, nil, "", err
	}
	defer closeReaders()
	set, err := parseFeedBodyReader(ctx, src.Name, io.MultiReader(readers...), snap.runtime.ParallelDNSQueries)
	if err != nil {
		return nil, nil, "", err
	}
	if len(composition.Subtracted) > 0 {
		excludeReaders, closeExcludeReaders, err := e.mergeInputReadersWithSnapshot(snap, composition.Subtracted)
		if err != nil {
			return nil, nil, "", err
		}
		defer closeExcludeReaders()
		excludeSet, err := parseFeedBodyReader(ctx, src.Name+"_exclude", io.MultiReader(excludeReaders...), snap.runtime.ParallelDNSQueries)
		if err != nil {
			return nil, nil, "", err
		}
		set, err = iprange.ExcludeSourcesContext(ctx, src.Name, set, excludeSet)
		if err != nil {
			return nil, nil, "", fmt.Errorf("merge exclude collect: %w", err)
		}
	}
	body, err := downloader.RenderCanonicalFeedBody(set, src.Output)
	if err != nil {
		return nil, nil, "", err
	}
	return body, set, "", nil
}

func (e *Engine) mergeInputReaders(inputs []MergeInputState) ([]io.Reader, func(), error) {
	return e.mergeInputReadersWithSnapshot(e.operationSnapshot(), inputs)
}

func (e *Engine) mergeInputReadersWithSnapshot(snap operationSnapshot, inputs []MergeInputState) ([]io.Reader, func(), error) {
	readers := make([]io.Reader, 0, len(inputs)*2)
	files := make([]*os.File, 0, len(inputs))
	closeReaders := func() {
		for _, file := range files {
			_ = file.Close()
		}
	}
	for _, input := range inputs {
		path := latestFeedBodyPath(snap.feedBodyPath(input.Name))
		file, err := openFilePathUnderRoot(filepath.Dir(path), path)
		if err != nil {
			closeReaders()
			return nil, func() {}, err
		}
		files = append(files, file)
		readers = append(readers, file, strings.NewReader("\n"))
	}
	return readers, closeReaders, nil
}
