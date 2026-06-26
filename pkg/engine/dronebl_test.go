package engine

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

	if got := artifactDownloadMaxSizeForRuntime(eng.Runtime(), cfg.Artifacts["dronebl"]); got != 200 {
		t.Fatalf("artifactDownloadMaxSizeForRuntime() = %d, want 200", got)
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

	if got := artifactDownloadMaxSizeForRuntime(eng.Runtime(), cfg.Artifacts["dronebl"]); got != 100 {
		t.Fatalf("artifactDownloadMaxSizeForRuntime() = %d, want 100", got)
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

func TestRecoverStagedDroneBLCorruptBuildzoneRenamesAside(t *testing.T) {
	eng := newDroneBLRecoveryFixture(t)
	stagePath := stagedPath(eng.artifactSourcePath("dronebl"))
	if err := os.MkdirAll(filepath.Dir(stagePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stagePath, []byte(strings.Repeat("1", 2*1024*1024+1)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := eng.RecoverStagedArtifact(t.Context(), "dronebl", true)
	if !errors.Is(err, ErrRecoveredArtifactCorrupt) {
		t.Fatalf("RecoverStagedArtifact error = %v, want ErrRecoveredArtifactCorrupt", err)
	}
	if _, err := os.Stat(stagePath); !os.IsNotExist(err) {
		t.Fatalf("corrupt recovered stage should be moved aside, stat err=%v", err)
	}
	if _, err := os.Stat(stagePath + ".corrupt"); err != nil {
		t.Fatalf("corrupt recovered stage was not preserved as .corrupt: %v", err)
	}
	sidecarPath := stagePath + ".corrupt.json"
	data, err := os.ReadFile(sidecarPath)
	if err != nil {
		t.Fatalf("corrupt recovered stage sidecar was not written: %v", err)
	}
	var sidecar recoveredArtifactCorruptionSidecar
	if err := json.Unmarshal(data, &sidecar); err != nil {
		t.Fatalf("decode corrupt recovered stage sidecar: %v", err)
	}
	if sidecar.Name != "dronebl" {
		t.Fatalf("sidecar name = %q, want dronebl", sidecar.Name)
	}
	if sidecar.Artifact != filepath.Base(stagePath+".corrupt") {
		t.Fatalf("sidecar artifact = %q, want %q", sidecar.Artifact, filepath.Base(stagePath+".corrupt"))
	}
	if sidecar.CorruptionClass != "token_too_long" {
		t.Fatalf("sidecar corruption_class = %q, want token_too_long", sidecar.CorruptionClass)
	}
	if sidecar.Timestamp.IsZero() {
		t.Fatal("sidecar timestamp is zero")
	}
}

func TestRecoverStagedDroneBLTransientMaterializationErrorKeepsStage(t *testing.T) {
	eng := newDroneBLRecoveryFixture(t)
	stagePath := stagedPath(eng.artifactSourcePath("dronebl"))
	if err := os.MkdirAll(filepath.Dir(stagePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stagePath, []byte("1.2.3.4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(eng.artifactExtractDir("dronebl"), []byte("not a directory\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := eng.RecoverStagedArtifact(t.Context(), "dronebl", true)
	if err == nil {
		t.Fatal("RecoverStagedArtifact returned nil error, want transient materialization error")
	}
	if errors.Is(err, ErrRecoveredArtifactCorrupt) {
		t.Fatalf("transient materialization error was classified as corrupt: %v", err)
	}
	if _, statErr := os.Stat(stagePath); statErr != nil {
		t.Fatalf("transient recovery failure should keep staged artifact, stat err=%v", statErr)
	}
	if _, statErr := os.Stat(stagePath + ".corrupt"); !os.IsNotExist(statErr) {
		t.Fatalf("transient recovery failure should not create .corrupt, stat err=%v", statErr)
	}
}

func newDroneBLRecoveryFixture(t *testing.T) *Engine {
	t.Helper()

	cfg := config.New()
	cfg.Artifacts["dronebl"] = &config.Artifact{
		Name:      "dronebl",
		Type:      config.ArtifactTypeDroneBLBuildzone,
		Frequency: 60,
	}
	cfg.Sources["child"] = &config.Source{
		Name:           "child",
		URL:            "artifact://dronebl?parts=auto_botnets",
		ArtifactParent: "dronebl",
		Frequency:      0,
		IPV:            "ipv4",
		Output:         "ipset",
	}
	eng := newEngineFixture(t, withConfig(cfg))
	for _, dir := range []string{
		eng.runtime.BaseDir,
		eng.runtime.HistoryDir,
		eng.runtime.LibDir,
		eng.runtime.ErrorsDir,
		eng.runtime.CacheDir,
		eng.runtime.TmpDir,
		eng.runtime.WebDir,
	} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("create runtime dir %s: %v", dir, err)
		}
	}
	return eng
}
