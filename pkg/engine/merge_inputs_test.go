package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
)

func TestMergeCompositionExcludesArchivedAndUnmaintainedInputs(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	baseDir := t.TempDir()
	cfg := config.New()
	cfg.Runtime.FeedHealthSingleObservationGraceMins = 60
	cfg.Runtime.FeedHealthDefaultHealthyCadenceMins = 60
	cfg.Runtime.FeedHealthDefaultRiskyCadenceMins = 60
	cfg.Runtime.FeedHealthArchivalThresholdMins = 60
	cfg.Sources["healthy"] = &config.Source{Name: "healthy", Frequency: 60, IPV: "ipv4", Output: "ipset"}
	cfg.Sources["archived"] = &config.Source{Name: "archived", Frequency: 60, IPV: "ipv4", Output: "ipset"}
	cfg.Sources["unmaintained"] = &config.Source{Name: "unmaintained", Frequency: 60, IPV: "ipv4", Output: "ipset"}
	cfg.Sources["merged"] = &config.Source{
		Name:        "merged",
		Frequency:   60,
		IPV:         "ipv4",
		Output:      "ipset",
		DerivedFrom: []string{"healthy", "archived", "unmaintained"},
		Provenance:  config.ProvenanceSecondaryMerge,
	}
	eng := newEngineFixture(t, withConfig(cfg), withRuntime(func(rt *Runtime) {
		rt.BaseDir = baseDir
	}), withNow(func() time.Time { return now }))

	for _, name := range []string{"healthy", "archived", "unmaintained", "merged"} {
		if err := os.WriteFile(sourceEnablePathForRuntime(eng.runtime, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(baseDir, "healthy.ipset"), []byte("1.2.3.4\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	healthy := eng.state.Entry("healthy")
	healthy.Name = "healthy"
	healthy.ProcessedDate = now.Add(-30 * time.Minute).Unix()
	healthy.SourceDate = now.Add(-30 * time.Minute).Unix()
	healthy.Entries = 1
	healthy.Version = 3

	archived := eng.state.Entry("archived")
	archived.Name = "archived"
	archived.ProcessedDate = now.Add(-200 * time.Minute).Unix()
	archived.SourceDate = now.Add(-200 * time.Minute).Unix()
	archived.CheckedDate = now.Unix()
	archived.DownloadFailures = 5
	archived.FailureStartedDate = now.Add(-200 * time.Minute).Unix()
	archived.LastStatus = "download_failed"
	archived.Version = 3

	unmaintained := eng.state.Entry("unmaintained")
	unmaintained.Name = "unmaintained"
	unmaintained.ProcessedDate = now.Add(-130 * time.Minute).Unix()
	unmaintained.SourceDate = now.Add(-130 * time.Minute).Unix()
	unmaintained.CheckedDate = now.Unix()
	unmaintained.Entries = 1
	unmaintained.Version = 3

	composition := eng.MergeComposition(cfg.Sources["merged"], false)
	if len(composition.Included) != 1 || composition.Included[0].Name != "healthy" {
		t.Fatalf("included = %+v, want only healthy", composition.Included)
	}
	if len(composition.Excluded) != 2 {
		t.Fatalf("excluded = %+v, want 2", composition.Excluded)
	}
	gotReasons := map[string]string{}
	for _, item := range composition.Excluded {
		gotReasons[item.Name] = item.Reason
	}
	if gotReasons["archived"] != mergeInputReasonArchived {
		t.Fatalf("archived exclusion reason = %q, want %q", gotReasons["archived"], mergeInputReasonArchived)
	}
	if gotReasons["unmaintained"] != mergeInputReasonUnmaintained {
		t.Fatalf("unmaintained exclusion reason = %q, want %q", gotReasons["unmaintained"], mergeInputReasonUnmaintained)
	}
}

func TestMergeEnablementIgnoresSubtractiveParents(t *testing.T) {
	baseDir := t.TempDir()
	cfg := config.New()
	cfg.Sources["included"] = &config.Source{Name: "included", Frequency: 60, IPV: "ipv4", Output: "ipset"}
	cfg.Sources["subtracted"] = &config.Source{Name: "subtracted", Frequency: 60, IPV: "ipv4", Output: "ipset"}
	cfg.Sources["merged"] = &config.Source{
		Name:         "merged",
		Frequency:    60,
		IPV:          "ipv4",
		Output:       "ipset",
		DerivedFrom:  []string{"included", "subtracted"},
		MergeSources: []string{"included"},
		MergeExclude: []string{"subtracted"},
		Provenance:   config.ProvenanceSecondaryMerge,
	}
	rt := Runtime{BaseDir: baseDir}
	for _, name := range []string{"merged", "subtracted"} {
		if err := os.WriteFile(sourceEnablePathForRuntime(rt, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if EffectiveSourceEnabled(cfg, rt, "merged", false) {
		t.Fatal("merge should not be enabled when only subtractive parents are enabled")
	}

	if err := os.WriteFile(sourceEnablePathForRuntime(rt, "included"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if !EffectiveSourceEnabled(cfg, rt, "merged", false) {
		t.Fatal("merge should be enabled when an additive parent is enabled")
	}
}

func TestPublicRawFeedAllowedIsFalseForArchivedFeed(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	cfg := config.New()
	cfg.Runtime.FeedHealthSingleObservationGraceMins = 60
	cfg.Runtime.FeedHealthDefaultHealthyCadenceMins = 60
	cfg.Runtime.FeedHealthDefaultRiskyCadenceMins = 60
	cfg.Runtime.FeedHealthArchivalThresholdMins = 60
	cfg.Sources["sample"] = &config.Source{
		Name:      "sample",
		URL:       "https://example.test/sample.txt",
		Frequency: 60,
		IPV:       "ipv4",
		Output:    "ipset",
	}
	eng := newEngineFixture(t, withConfig(cfg), withNow(func() time.Time { return now }))
	entry := eng.state.Entry("sample")
	entry.Name = "sample"
	entry.ProcessedDate = now.Add(-200 * time.Minute).Unix()
	entry.SourceDate = now.Add(-200 * time.Minute).Unix()
	entry.CheckedDate = now.Unix()
	entry.DownloadFailures = 5
	entry.FailureStartedDate = now.Add(-200 * time.Minute).Unix()
	entry.LastStatus = "download_failed"
	entry.Version = 3

	if eng.PublicRawFeedAllowed("sample") {
		t.Fatal("expected archived feed to disable public raw access")
	}
}
