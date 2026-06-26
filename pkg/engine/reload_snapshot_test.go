package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

func TestRunOnceUsesStartGenerationWhenReloadOverlapsAfterAdmission(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	oldBase := filepath.Join(root, "old-base")
	newBase := filepath.Join(root, "new-base")
	writeReloadSnapshotConfig(t, cfgPath, root, "old", oldBase)

	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(oldBase, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldBase, "old.ipset.new"), []byte("1.2.3.4\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	reloadRan := false
	restoreHook := setRunOnceAfterStartHookForTest(func() {
		reloadRan = true
		writeReloadSnapshotConfig(t, cfgPath, root, "new", newBase)
		if err := eng.ReloadContext(t.Context()); err != nil {
			t.Fatalf("reload during admitted run: %v", err)
		}
	})
	defer restoreHook()

	report, err := eng.RunOnce(t.Context(), RunOptions{
		Selected:   []string{"old"},
		EnableAll:  true,
		Manual:     true,
		CleanupOld: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reloadRan {
		t.Fatal("test hook did not trigger reload")
	}
	if !slices.Equal(report.Updated, []string{"old"}) {
		t.Fatalf("updated feeds = %v, want [old]; report=%#v", report.Updated, report)
	}
	if len(report.Failed) != 0 {
		t.Fatalf("failed feeds = %v, want none; report=%#v", report.Failed, report)
	}
	if _, err := os.Stat(filepath.Join(oldBase, "old.ipset")); err != nil {
		t.Fatalf("admitted run did not publish old-generation source: %v", err)
	}
}

func TestRetentionCohortsCacheDoesNotAdvanceWhenDurableWriteFails(t *testing.T) {
	root := t.TempDir()
	libDir := filepath.Join(root, "lib")
	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.LibDir = libDir
	}))

	oldAt := time.Unix(1_700_000_000, 0).UTC()
	updatedAt := oldAt.Add(time.Hour)
	writeRetentionCohortForTest(t, libDir, "sample", oldAt, "0.0.0.10", "0.0.0.11")
	resetRetentionCohortCacheForTest(eng, "sample")
	if got := eng.retentionCohortsFromRuntime(t.Context(), "sample"); !reflect.DeepEqual(got, map[int64]uint64{oldAt.Unix(): 2}) {
		t.Fatalf("initial retention cohorts = %#v, want old cohort count 2", got)
	}
	cohortIndexPath := filepath.Join(libDir, "sample", "retention_cohorts.csv")
	if err := os.Remove(cohortIndexPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(cohortIndexPath, 0o700); err != nil {
		t.Fatal(err)
	}

	current := iprange.New("sample")
	if err := current.Add(10, 10); err != nil {
		t.Fatal(err)
	}
	current.Optimize()

	diff := retentionUpdateDiff{removed: 1}
	err := eng.updateRetentionWithDiff(t.Context(), "sample", retentionUpdatePaths{
		dir:    filepath.Join(libDir, "sample"),
		newDir: filepath.Join(libDir, "sample", "new"),
	}, diff, current, updatedAt, updatedAt.Unix())
	if err == nil {
		t.Fatal("updateRetentionWithDiff succeeded after retention cohort index was made unwritable")
	}

	got := eng.retentionCohortsFromRuntime(t.Context(), "sample")
	want := map[int64]uint64{oldAt.Unix(): 2}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("retention cohorts advanced after failed durable write: got %#v want %#v", got, want)
	}
}

func TestRetentionCohortsCacheDoesNotAdvanceWhenAddedCohortOutputFails(t *testing.T) {
	root := t.TempDir()
	libDir := filepath.Join(root, "lib")
	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.LibDir = libDir
	}))

	updatedAt := time.Unix(1_700_003_600, 0).UTC()
	feedDir := filepath.Join(libDir, "sample")
	newDir := filepath.Join(feedDir, "new")
	if err := os.MkdirAll(newDir, 0o700); err != nil {
		t.Fatal(err)
	}
	resetRetentionCohortCacheForTest(eng, "sample")
	if got := eng.retentionCohortsFromRuntime(t.Context(), "sample"); !reflect.DeepEqual(got, map[int64]uint64{}) {
		t.Fatalf("initial retention cohorts = %#v, want empty", got)
	}
	cohortIndexPath := filepath.Join(feedDir, "retention_cohorts.csv")
	if err := os.Mkdir(cohortIndexPath, 0o700); err != nil {
		t.Fatal(err)
	}

	current := iprange.New("sample")
	if err := current.Add(10, 10); err != nil {
		t.Fatal(err)
	}
	current.Optimize()

	diff := retentionUpdateDiff{newSet: current, added: 1}
	err := eng.updateRetentionWithDiff(t.Context(), "sample", retentionUpdatePaths{
		dir:    feedDir,
		newDir: newDir,
	}, diff, current, updatedAt, updatedAt.Unix())
	if err == nil {
		t.Fatal("updateRetentionWithDiff succeeded after retention cohort index was made unwritable")
	}

	got := eng.retentionCohortsFromRuntime(t.Context(), "sample")
	if !reflect.DeepEqual(got, map[int64]uint64{}) {
		t.Fatalf("retention cohorts advanced after failed added-cohort durable write: got %#v want empty", got)
	}
}

func TestFinalizeDoesNotAdvanceHistoryCacheWhenHistoryAppendFails(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	libDir := filepath.Join(root, "lib")
	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.BaseDir = baseDir
		rt.LibDir = libDir
	}))

	oldAt := time.Unix(1_700_000_000, 0).UTC()
	newAt := oldAt.Add(time.Hour)
	historyDir := filepath.Join(libDir, "sample")
	if err := os.MkdirAll(historyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	historyPath := filepath.Join(historyDir, "history.csv")
	if err := os.WriteFile(historyPath, []byte(fmt.Sprintf("DateTime,Entries,UniqueIPs\n%d,2,2\n", oldAt.Unix())), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := eng.historyTailFromRuntime("sample"); !reflect.DeepEqual(got, []HistoryPoint{{Timestamp: oldAt.Unix(), Name: "sample", Entries: 2, UniqueIPs: 2}}) {
		t.Fatalf("initial history tail = %#v, want old point", got)
	}
	if err := os.Remove(historyPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(historyPath, 0o700); err != nil {
		t.Fatal(err)
	}

	bodyPath := filepath.Join(root, "sample.body")
	if err := os.WriteFile(bodyPath, []byte("0.0.0.10\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	finalSet := iprange.New("sample")
	if err := finalSet.Add(10, 10); err != nil {
		t.Fatal(err)
	}
	finalSet.Optimize()
	src := &config.Source{Name: "sample", Frequency: 60, IPV: "ipv4", Output: "ipset"}

	err := eng.finalizeWithSnapshot(t.Context(), eng.operationSnapshot(), "sample", src, "ipset", bodyPath, finalSet, newAt, newAt)
	if err == nil {
		t.Fatal("finalize succeeded after history.csv was made unwritable")
	}
	got := eng.historyTailFromRuntime("sample")
	want := []HistoryPoint{{Timestamp: oldAt.Unix(), Name: "sample", Entries: 2, UniqueIPs: 2}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("history tail advanced after failed durable append: got %#v want %#v", got, want)
	}
}

func writeReloadSnapshotConfig(t *testing.T, path, root, sourceName, baseDir string) {
	t.Helper()
	body := fmt.Sprintf(`
runtime:
  base_dir: %q
  history_dir: %q
  lib_dir: %q
  errors_dir: %q
  web_dir: %q
  cache_dir: %q
  tmp_dir: %q
  ipsets_apply: false
sources:
  %s:
    url: https://example.test/%s.txt
    frequency: 60
    ipv: ipv4
    output: ipset
    processor:
      - passthrough
    category: attacks
    info: %s feed
    maintainer: test
    maintainer_url: https://example.test
`, baseDir, filepath.Join(root, sourceName+"-history"), filepath.Join(root, sourceName+"-lib"), filepath.Join(root, sourceName+"-errors"), filepath.Join(root, sourceName+"-web"), filepath.Join(root, sourceName+"-cache"), filepath.Join(root, sourceName+"-tmp"), sourceName, sourceName, sourceName)
	if err := os.WriteFile(path, []byte(strings.TrimSpace(body)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}
