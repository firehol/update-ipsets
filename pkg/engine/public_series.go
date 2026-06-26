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
	"strings"
)

const (
	changesetLedgerHeader    = "DateTime,IPsAdded,IPsRemoved\n"
	oldChangesetLedgerHeader = "DateTime,AddedIPs,RemovedIPs\n"
)

func (e *Engine) webChartsEntries() int {
	return webChartsEntriesFromRuntime(e.Runtime())
}

func webChartsEntriesFromRuntime(rt Runtime) int {
	if rt.WebChartsEntries > 0 {
		return rt.WebChartsEntries
	}
	return 500
}

func trimHistoryWindow(points []HistoryPoint, limit int) []HistoryPoint {
	if limit <= 0 || len(points) <= limit {
		return points
	}
	return points[len(points)-limit:]
}

func (e *Engine) writePublicHistoryCSVContext(ctx context.Context, name, outDir string) error {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return err
	}
	points := e.historyTailFromRuntimeContext(ctx, name)
	if len(points) == 0 {
		points = e.publicHistorySeriesContext(ctx, name)
		points = trimHistoryWindow(points, e.webChartsEntries())
	}
	if err := contextErr(ctx); err != nil {
		return err
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Timestamp < points[j].Timestamp })

	var buf bytes.Buffer
	buf.WriteString("DateTime,Entries,UniqueIPs\n")
	for _, point := range points {
		if err := contextErr(ctx); err != nil {
			return err
		}
		fmt.Fprintf(&buf, "%d,%d,%d\n", point.Timestamp, point.Entries, point.UniqueIPs)
	}
	return writeFileAtomicAt(filepath.Join(outDir, name+"_history.csv"), buf.Bytes(), generatedFileMode, e.feedProcessingTimestamp(name))
}

func (e *Engine) writePublicChangesetsCSV(name, outDir string) error {
	return e.writePublicChangesetsCSVContext(context.Background(), name, outDir)
}

func (e *Engine) writePublicChangesetsCSVContext(ctx context.Context, name, outDir string) error {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return err
	}
	if err := normalizeChangesetLedgerHeader(e.runtime.LibDir, filepath.Join(name, "changesets.csv")); err != nil {
		return err
	}
	points := e.changesetTailFromRuntimeContext(ctx, name)
	if len(points) == 0 {
		var err error
		points, err = e.ChangesetSeriesContext(ctx, name)
		if err != nil {
			return err
		}
	}
	if err := contextErr(ctx); err != nil {
		return err
	}

	var buf bytes.Buffer
	buf.WriteString("DateTime,AddedIPs,RemovedIPs\n")
	for _, point := range points {
		if err := contextErr(ctx); err != nil {
			return err
		}
		fmt.Fprintf(&buf, "%d,%d,%d\n", point.Timestamp, point.Added, point.Removed)
	}
	return writeFileAtomicAt(filepath.Join(outDir, name+"_changesets.csv"), buf.Bytes(), generatedFileMode, e.feedProcessingTimestamp(name))
}

func (e *Engine) writePublicRetentionJSONContext(ctx context.Context, name, outDir string) error {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return err
	}
	data, err := readFileInRoot(e.runtime.LibDir, filepath.Join(name, "retention.json"))
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		retention, err := e.buildRetentionData(ctx, name, e.now().UTC().Unix())
		if err != nil {
			return err
		}
		data, err = jsonMarshalTabIndent(retention)
		if err != nil {
			return err
		}
		data = append(data, '\n')
	}
	return writeFileAtomicAt(filepath.Join(outDir, name+"_retention.json"), data, generatedFileMode, e.feedProcessingTimestamp(name))
}

func (e *Engine) publicHistorySeriesContext(ctx context.Context, name string) []HistoryPoint {
	points := e.historyFromLedgerCSVContext(ctx, name)
	if len(points) == 0 {
		points = e.historyFromWebCSVContext(ctx, name)
	}
	return points
}

func normalizeChangesetLedgerHeader(rootDir, rel string) error {
	file, err := openFileInRoot(rootDir, rel)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer func() { _ = file.Close() }()
	reader := bufio.NewReader(file)
	header, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(header), strings.TrimSpace(oldChangesetLedgerHeader)) {
		return nil
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	next := append([]byte(changesetLedgerHeader), data...)
	return writeFileAtomic(filepath.Join(rootDir, rel), next, generatedFileMode)
}
