package scheduler

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/runreason"
)

func TestActivitySnapshotLightUsesCopiedConfigFacts(t *testing.T) {
	eng := newActivitySnapshotEngine(t, true)
	runner := New(eng, true, nil)
	queuedAt := time.Now().UTC()
	runner.enqueueDownload(queuedWork{Name: "parent", Reason: runreason.ReasonScheduledDue, QueuedAt: queuedAt})
	runner.download.active["parent"] = ActiveQueueFeed{Name: "parent", StartedAt: queuedAt}
	runner.enqueueDownload(queuedWork{Name: "child", Reason: runreason.ReasonScheduledDue, QueuedAt: queuedAt.Add(time.Second)})
	runner.enqueueProcessing(queuedWork{Name: "sample", Reason: runreason.ReasonScheduledDue, QueuedAt: queuedAt})
	runner.enqueueProcessing(queuedWork{Name: "geolite2_country", Reason: runreason.ReasonScheduledDue, QueuedAt: queuedAt})

	activity := runner.ActivitySnapshotLight()

	if got := queueNames(activity.ProcessingWaiting); !reflect.DeepEqual(got, []string{"sample"}) {
		t.Fatalf("light processing waiting names = %v, want only sample", got)
	}
	var child QueueFeed
	for _, item := range activity.DownloadWaiting {
		if item.Name == "child" {
			child = item
			break
		}
	}
	if !child.Blocked {
		t.Fatalf("child download waiting entry not marked blocked: %+v", activity.DownloadWaiting)
	}
	if got, want := child.BlockedParents, []string{"parent"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("child blocked parents = %v, want %v", got, want)
	}
}

func TestActivitySnapshotLightConcurrentConfigReload(t *testing.T) {
	eng := newActivitySnapshotEngine(t, true)
	runner := New(eng, true, nil)
	runner.enqueueProcessing(queuedWork{Name: "geolite2_country", Reason: runreason.ReasonScheduledDue, QueuedAt: time.Now().UTC()})

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			_ = runner.ActivitySnapshotLight()
			runtime.Gosched()
		}
	}()
	for i := 0; i < 50; i++ {
		writeActivitySnapshotConfig(t, eng.Runtime().ConfigPath, activitySnapshotRoot(eng), i%2 == 0)
		if err := eng.ReloadContext(t.Context()); err != nil {
			t.Fatalf("reload config iteration %d: %v", i, err)
		}
		runtime.Gosched()
	}
	<-done
}

func queueNames(items []QueueFeed) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Name)
	}
	return out
}

func newActivitySnapshotEngine(t *testing.T, provider bool) *engine.Engine {
	t.Helper()
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	writeActivitySnapshotConfig(t, cfgPath, root, provider)
	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	return eng
}

func activitySnapshotRoot(eng *engine.Engine) string {
	rt := eng.Runtime()
	return filepath.Dir(rt.ConfigPath)
}

func writeActivitySnapshotConfig(t *testing.T, cfgPath, root string, provider bool) {
	t.Helper()
	useLine := ""
	formatLine := ""
	if provider {
		useLine = "    use: [geoip]\n"
		formatLine = "    format: maxmind_country_csv\n"
	}
	cfg := fmt.Sprintf(`
runtime:
  base_dir: %q
  history_dir: %q
  lib_dir: %q
  errors_dir: %q
  web_dir: %q
  cache_dir: %q
  ipsets_apply: false
sources:
  parent:
    url: https://example.test/parent.txt
    frequency: 60
    ipv: ipv4
    output: ipset
    processor: [passthrough]
    category: attacks
    info: parent feed
    maintainer: test
    maintainer_url: https://example.test
  sample:
    url: https://example.test/sample.txt
    frequency: 60
    ipv: ipv4
    output: ipset
    processor: [passthrough]
    category: attacks
    info: sample feed
    maintainer: test
    maintainer_url: https://example.test
  geolite2_country:
    url: https://example.test/geolite2-country.zip
    frequency: 1440
    ipv: ipv4
    output: ipset
    processor: [passthrough]
%s%s    info: geo provider
    maintainer: test
    maintainer_url: https://example.test
merges:
  child:
    sources: [parent]
    frequency: 60
    ipv: ipv4
    output: ipset
    category: attacks
    info: child feed
    maintainer: test
    maintainer_url: https://example.test
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"), useLine, formatLine)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
}
