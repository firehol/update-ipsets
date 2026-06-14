package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
)

func TestDroneBLArtifactSpecsComeFromCatalog(t *testing.T) {
	cfg := config.New()
	cfg.Artifacts["dronebl"] = &config.Artifact{
		Name:      "dronebl",
		Type:      config.ArtifactTypeDroneBLBuildzone,
		Frequency: 1,
	}
	cfg.Sources["alpha"] = &config.Source{
		Name:           "alpha",
		URL:            "artifact://dronebl?parts=class_a,class_b",
		ArtifactParent: "dronebl",
		Frequency:      0,
		IPV:            "ipv4",
		Output:         "netset",
	}
	cfg.Sources["beta"] = &config.Source{
		Name:           "beta",
		URL:            "artifact://dronebl?parts=class_c",
		ArtifactParent: "dronebl",
		Frequency:      0,
		IPV:            "ipv4",
		Output:         "netset",
	}

	root := t.TempDir()
	eng := newEngineFixture(t, withConfig(cfg), withRuntime(func(rt *Runtime) {
		rt.BaseDir = root
		rt.LibDir = root
	}), withNow(func() time.Time { return time.Unix(1, 0).UTC() }))
	if err := touchFileAt(eng.artifactEnablePath("dronebl"), eng.now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := touchFileAt(eng.sourceEnablePath("alpha"), eng.now().UTC()); err != nil {
		t.Fatal(err)
	}

	specs := eng.droneBLArtifactSpecs("dronebl", false)
	if len(specs) != 1 {
		t.Fatalf("got %d specs, want 1: %#v", len(specs), specs)
	}
	if specs[0].Name != "alpha" {
		t.Fatalf("spec name = %q, want alpha", specs[0].Name)
	}
	if got := len(specs[0].Lists); got != 2 {
		t.Fatalf("list count = %d, want 2", got)
	}
	if specs[0].Lists[0] != "class_a" || specs[0].Lists[1] != "class_b" {
		t.Fatalf("lists = %#v, want class_a,class_b", specs[0].Lists)
	}
}

func TestArtifactDownloadMaxSizeUsesArtifactOverride(t *testing.T) {
	cfg := config.New()
	cfg.Artifacts["dronebl"] = &config.Artifact{
		Name:            "dronebl",
		Type:            config.ArtifactTypeDroneBLBuildzone,
		Frequency:       1,
		MaxDownloadSize: 200,
	}

	eng := newEngineFixture(t, withConfig(cfg), withRuntime(func(rt *Runtime) {
		rt.MaxDownloadSize = 100
	}))

	if got := eng.artifactDownloadMaxSize(cfg.Artifacts["dronebl"]); got != 200 {
		t.Fatalf("artifactDownloadMaxSize() = %d, want 200", got)
	}
}

func TestArtifactDownloadMaxSizeFallsBackToRuntimeDefault(t *testing.T) {
	cfg := config.New()
	cfg.Artifacts["dronebl"] = &config.Artifact{
		Name:      "dronebl",
		Type:      config.ArtifactTypeDroneBLBuildzone,
		Frequency: 1,
	}

	eng := newEngineFixture(t, withConfig(cfg), withRuntime(func(rt *Runtime) {
		rt.MaxDownloadSize = 100
	}))

	if got := eng.artifactDownloadMaxSize(cfg.Artifacts["dronebl"]); got != 100 {
		t.Fatalf("artifactDownloadMaxSize() = %d, want 100", got)
	}
}

func TestEntriesSnapshotWithArtifactsIncludesArtifactParents(t *testing.T) {
	cfg := config.New()
	cfg.Artifacts["dronebl"] = &config.Artifact{
		Name:      "dronebl",
		Type:      config.ArtifactTypeDroneBLBuildzone,
		Frequency: 60,
	}

	eng := newEngineFixture(t, withConfig(cfg))
	entry := eng.state.Entry("dronebl")
	entry.CheckedDate = 1700000000
	entry.DownloadFailures = 3

	if got := eng.EntriesSnapshot(); len(got) != 0 {
		t.Fatalf("EntriesSnapshot() returned %d entries, want 0 for artifact-only state", len(got))
	}

	got := eng.EntriesSnapshotWithArtifacts()
	if len(got) != 1 {
		t.Fatalf("EntriesSnapshotWithArtifacts() returned %d entries, want 1", len(got))
	}
	if got[0].Name != "dronebl" {
		t.Fatalf("artifact entry name = %q, want dronebl", got[0].Name)
	}
	if got[0].CheckedDate != 1700000000 {
		t.Fatalf("artifact checked_date = %d, want 1700000000", got[0].CheckedDate)
	}
	if got[0].DownloadFailures != 3 {
		t.Fatalf("artifact download_failures = %d, want 3", got[0].DownloadFailures)
	}
}

func TestCleanupDroneBLExtractDirRemovesOnlyOutputScratchDirs(t *testing.T) {
	extractDir := t.TempDir()
	for _, name := range []string{"outputs-old", "outputs-interrupted"} {
		path := filepath.Join(extractDir, name)
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(path, "child.source"), []byte("scratch\n"), 0o600); err != nil {
			t.Fatalf("write scratch file in %s: %v", name, err)
		}
	}
	keptDir := filepath.Join(extractDir, "manual")
	if err := os.Mkdir(keptDir, 0o700); err != nil {
		t.Fatalf("create kept dir: %v", err)
	}
	keptFile := filepath.Join(extractDir, "outputs-note")
	if err := os.WriteFile(keptFile, []byte("not a directory\n"), 0o600); err != nil {
		t.Fatalf("write kept file: %v", err)
	}

	if err := cleanupDroneBLExtractDir(extractDir); err != nil {
		t.Fatalf("cleanupDroneBLExtractDir: %v", err)
	}

	for _, name := range []string{"outputs-old", "outputs-interrupted"} {
		if _, err := os.Stat(filepath.Join(extractDir, name)); !os.IsNotExist(err) {
			t.Fatalf("scratch dir %s still exists or stat failed with %v", name, err)
		}
	}
	if _, err := os.Stat(keptDir); err != nil {
		t.Fatalf("kept dir was removed: %v", err)
	}
	if _, err := os.Stat(keptFile); err != nil {
		t.Fatalf("kept file was removed: %v", err)
	}
}
