package engine

import (
	"reflect"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
)

func TestSchedulerConfigSnapshotCopiesSchedulerFacts(t *testing.T) {
	cfg := config.New()
	cfg.Sources["geo"] = &config.Source{Name: "geo", Use: []string{config.UseGeoIP}}
	cfg.Sources["child"] = &config.Source{Name: "child", DerivedFrom: []string{"parent"}}
	cfg.Artifacts["artifact"] = &config.Artifact{Name: "artifact", Type: config.ArtifactTypeDroneBLBuildzone, Frequency: 60}
	eng := newEngineFixture(t, withConfig(cfg))

	snap := eng.SchedulerConfigSnapshot()
	cfg.Sources["geo"].Use = nil
	cfg.Sources["child"].DerivedFrom[0] = "mutated"
	delete(cfg.Artifacts, "artifact")

	if !snap.IsProviderDatabase("geo") {
		t.Fatal("scheduler config snapshot lost provider database role")
	}
	if got, want := snap.DerivedFrom("child"), []string{"parent"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshot derived parents = %v, want %v", got, want)
	}
	if !snap.IsArtifact("artifact") {
		t.Fatal("scheduler config snapshot lost artifact identity")
	}
}

func TestEngineLaneLongHoldThresholdDefault(t *testing.T) {
	if engineLaneLongHoldThreshold != 30*time.Second {
		t.Fatalf("engine lane long-hold threshold = %s, want 30s", engineLaneLongHoldThreshold)
	}
	if engineLaneDiagnosticsInterval != engineLaneLongHoldThreshold {
		t.Fatalf("engine lane diagnostics interval = %s, want threshold %s", engineLaneDiagnosticsInterval, engineLaneLongHoldThreshold)
	}
}
