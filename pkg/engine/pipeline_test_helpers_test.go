package engine

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

func runSchedulerStyleOnce(t *testing.T, eng *Engine, opts RunOptions) (*Report, error) {
	t.Helper()

	requested := opts.Selected
	if len(requested) == 0 {
		requested = config.SortedSourceNames(eng.Config())
	}

	force := opts.Recheck || opts.Reprocess
	processSet := make(map[string]struct{}, len(requested))
	promoteSet := make(map[string]struct{}, len(requested))

	for _, name := range requested {
		if name == "" {
			continue
		}
		if !eng.IsDownloadable(name) {
			processSet[name] = struct{}{}
			continue
		}
		decision, err := eng.FetchAndStage(t.Context(), name, force, opts.EnableAll)
		if err != nil {
			continue
		}
		for _, processName := range decision.ProcessingNames {
			if processName != "" {
				processSet[processName] = struct{}{}
			}
		}
		for _, promoteName := range decision.PromoteNames {
			if promoteName != "" {
				promoteSet[promoteName] = struct{}{}
			}
		}
	}

	if len(processSet) == 0 {
		return &Report{
			Messages: map[string]string{},
			Statuses: map[string]string{},
		}, nil
	}

	processNames := make([]string, 0, len(processSet))
	for _, name := range config.SortedSourceNames(eng.Config()) {
		if _, ok := processSet[name]; ok {
			processNames = append(processNames, name)
		}
	}
	slices.Sort(processNames)

	runOpts := opts
	runOpts.Selected = processNames
	if len(promoteSet) > 0 {
		promoteNames := make([]string, 0, len(promoteSet))
		for name := range promoteSet {
			promoteNames = append(promoteNames, name)
		}
		slices.Sort(promoteNames)
		runOpts.BeforePublish = func(report *Report) error {
			return eng.PromoteCommittedDownloads(promoteNames)
		}
	}

	report, err := eng.RunOnce(t.Context(), runOpts)
	if err != nil {
		return report, err
	}

	return report, nil
}

func writeSnapshotForTest(t *testing.T, historyDir, parent string, ts time.Time, cidrs ...string) string {
	t.Helper()

	dir := filepath.Join(historyDir, parent)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	text := strings.Join(cidrs, "\n") + "\n"
	set, err := iprange.ParseReader(t.Context(), parent, strings.NewReader(text), iprange.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse snapshot cidrs: %v", err)
	}
	set.Optimize()
	var buf bytes.Buffer
	if err := iprange.WriteBinary(&buf, set); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, strconv.FormatInt(ts.Unix(), 10)+".set")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, ts, ts); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeRetentionCohortForTest(t *testing.T, libDir, feed string, ts time.Time, cidrs ...string) string {
	t.Helper()

	dir := filepath.Join(libDir, feed, "new")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	text := strings.Join(cidrs, "\n") + "\n"
	set, err := iprange.ParseReader(t.Context(), feed, strings.NewReader(text), iprange.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse retention cohort cidrs: %v", err)
	}
	set.Optimize()
	path := filepath.Join(dir, fmt.Sprintf("%d", ts.Unix()))
	if err := writeBinaryPath(path, set, ts); err != nil {
		t.Fatal(err)
	}

	if err := rewriteRetentionCohortIndexForTest(filepath.Join(libDir, feed)); err != nil {
		t.Fatalf("write retention cohort index: %v", err)
	}
	return path
}

func rewriteRetentionCohortIndexForTest(feedDir string) error {
	newDir := filepath.Join(feedDir, "new")
	entries, err := os.ReadDir(newDir)
	if err != nil {
		return err
	}
	cohorts := make(map[int64]uint64, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || isIgnoredRetentionSnapshotName(entry.Name()) {
			continue
		}
		addedAt, err := strconv.ParseInt(strings.TrimSuffix(entry.Name(), ".set"), 10, 64)
		if err != nil || addedAt <= 0 {
			continue
		}
		path := filepath.Join(newDir, entry.Name())
		fs, err := iprange.OpenFileSet(path)
		if err != nil {
			return err
		}
		cohorts[addedAt] = fs.UniqueIPs()
		_ = fs.Close()
	}
	return writeRetentionCohortIndex(filepath.Join(feedDir, "retention_cohorts.csv"), cohorts)
}

func resetRetentionCohortCacheForTest(eng *Engine, name string) {
	if eng == nil || eng.ledgerCache == nil {
		return
	}
	st := eng.ledgerCache.feed(name)
	if st == nil {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	st.cohortsLoaded = false
	st.cohorts = nil
}
