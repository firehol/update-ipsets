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
	if e != nil && e.runtime.WebChartsEntries > 0 {
		return e.runtime.WebChartsEntries
	}
	return 500
}

func trimHistoryWindow(points []HistoryPoint, limit int) []HistoryPoint {
	if limit <= 0 || len(points) <= limit {
		return points
	}
	return points[len(points)-limit:]
}

func (e *Engine) writePublicHistoryCSV(name, outDir string) error {
	points := e.historyTailFromRuntime(name)
	if len(points) == 0 {
		points = e.publicHistorySeries(name)
		points = trimHistoryWindow(points, e.webChartsEntries())
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Timestamp < points[j].Timestamp })

	var buf bytes.Buffer
	buf.WriteString("DateTime,Entries,UniqueIPs\n")
	for _, point := range points {
		fmt.Fprintf(&buf, "%d,%d,%d\n", point.Timestamp, point.Entries, point.UniqueIPs)
	}
	return writeFileAtomicAt(filepath.Join(outDir, name+"_history.csv"), buf.Bytes(), 0o600, e.feedProcessingTimestamp(name))
}

func (e *Engine) writePublicChangesetsCSV(name, outDir string) error {
	if err := normalizeChangesetLedgerHeader(filepath.Join(e.runtime.LibDir, name, "changesets.csv")); err != nil {
		return err
	}
	points := e.changesetTailFromRuntime(name)
	if len(points) == 0 {
		var err error
		points, err = e.ChangesetSeries(name)
		if err != nil {
			return err
		}
	}

	var buf bytes.Buffer
	buf.WriteString("DateTime,AddedIPs,RemovedIPs\n")
	for _, point := range points {
		fmt.Fprintf(&buf, "%d,%d,%d\n", point.Timestamp, point.Added, point.Removed)
	}
	return writeFileAtomicAt(filepath.Join(outDir, name+"_changesets.csv"), buf.Bytes(), 0o600, e.feedProcessingTimestamp(name))
}

func (e *Engine) writePublicRetentionJSON(name, outDir string) error {
	path := filepath.Join(e.runtime.LibDir, name, "retention.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		retention, err := e.buildRetentionData(context.Background(), name, e.now().UTC().Unix())
		if err != nil {
			return err
		}
		data, err = jsonMarshalTabIndent(retention)
		if err != nil {
			return err
		}
		data = append(data, '\n')
	}
	return writeFileAtomicAt(filepath.Join(outDir, name+"_retention.json"), data, 0o600, e.feedProcessingTimestamp(name))
}

func (e *Engine) publicHistorySeries(name string) []HistoryPoint {
	points := e.historyFromLedgerCSV(name)
	if len(points) == 0 {
		points = e.historyFromWebCSV(name)
	}
	return points
}

func normalizeChangesetLedgerHeader(path string) error {
	file, err := os.Open(path)
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
	return writeFileAtomic(path, next, 0o600)
}
