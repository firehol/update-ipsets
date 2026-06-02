package output

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteREADME(t *testing.T) {
	dir := t.TempDir()
	makeGitDir(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "README-EDIT.md"), []byte("curated intro\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteREADME(dir, map[string]string{"b": "[b](x)|B|ipv4 hash:ip|1|src", "a": "[a](x)|A|ipv4 hash:ip|1|src"}); err != nil {
		t.Fatalf("WriteREADME returned error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"curated intro",
		"The following list was automatically generated on ",
		"The update frequency is the maximum allowed by internal configuration.",
		"name|info|type|entries|update|",
		":--:|:--:|:--:|:-----:|:----:|",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("README content missing %q: %s", want, text)
		}
	}
	if strings.Index(text, "[a]") > strings.Index(text, "[b]") {
		t.Fatalf("README is not sorted: %s", text)
	}
}

func TestWriteGitIgnore(t *testing.T) {
	dir := t.TempDir()
	makeGitDir(t, dir)
	files := []GeneratedFile{
		{Path: filepath.Join(dir, "public.ipset"), Redistributable: true},
		{Path: filepath.Join(dir, "secret.ipset"), Redistributable: false},
		{Path: filepath.Join(dir, "secret.json"), Redistributable: false},
		{Path: filepath.Join(dir, "nested", "private.netset"), Redistributable: false},
	}
	if err := WriteGitIgnore(dir, files); err != nil {
		t.Fatalf("WriteGitIgnore returned error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	got := string(data)
	for _, want := range []string{"*.setinfo", "*.source", "secret.ipset", "nested/private.netset"} {
		if !strings.Contains(got, want) {
			t.Fatalf(".gitignore missing %q: %s", want, got)
		}
	}
	for _, unwanted := range []string{"public.ipset", "secret.json"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf(".gitignore unexpectedly contains %q: %s", unwanted, got)
		}
	}
}

func TestGitSupportFilesNoopOutsideGitDir(t *testing.T) {
	dir := t.TempDir()
	if err := WriteREADME(dir, map[string]string{"a": "a|b|c|d|e"}); err != nil {
		t.Fatalf("WriteREADME returned error: %v", err)
	}
	if err := WriteGitIgnore(dir, []GeneratedFile{{Path: filepath.Join(dir, "secret.ipset"), Redistributable: false}}); err != nil {
		t.Fatalf("WriteGitIgnore returned error: %v", err)
	}
	if err := WriteTimestampScript(dir, []GeneratedFile{{Path: filepath.Join(dir, "sample.ipset"), Timestamp: time.Unix(1, 0)}}); err != nil {
		t.Fatalf("WriteTimestampScript returned error: %v", err)
	}
	for _, name := range []string{"README.md", ".gitignore", "set_file_timestamps.sh"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("expected no %s outside git dir, got err=%v", name, err)
		}
	}
}

func TestWriteGitIgnorePreservesExistingContent(t *testing.T) {
	dir := t.TempDir()
	makeGitDir(t, dir)
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("# keep me\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	files := []GeneratedFile{{Path: filepath.Join(dir, "secret.ipset"), Redistributable: false}}
	if err := WriteGitIgnore(dir, files); err != nil {
		t.Fatalf("WriteGitIgnore returned error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if got := string(data); !strings.Contains(got, "# keep me") || !strings.Contains(got, "secret.ipset") || strings.Contains(got, "*.setinfo") {
		t.Fatalf("unexpected .gitignore content: %s", got)
	}
}

func TestWriteTimestampScript(t *testing.T) {
	dir := t.TempDir()
	makeGitDir(t, dir)
	ts := time.Unix(1_700_000_000, 0).UTC()
	files := []GeneratedFile{
		{Path: filepath.Join(dir, "sample.ipset"), Timestamp: ts, Redistributable: true},
		{Path: filepath.Join(dir, "nested", "sample.netset"), Timestamp: ts.Add(time.Second), Redistributable: true},
		{Path: filepath.Join(dir, "sample.json"), Timestamp: ts, Redistributable: true},
	}
	if err := WriteTimestampScript(dir, files); err != nil {
		t.Fatalf("WriteTimestampScript returned error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "set_file_timestamps.sh"))
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	got := string(data)
	for _, want := range []string{
		"#!/bin/bash",
		"YES_I_AM_SURE_DO_IT_PLEASE",
		"[ -f 'sample.ipset' ] && touch --date=@1700000000 'sample.ipset'",
		"[ -f 'nested/sample.netset' ] && touch --date=@1700000001 'nested/sample.netset'",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("timestamp script missing %q: %s", want, got)
		}
	}
	if strings.Contains(got, "sample.json") {
		t.Fatalf("unexpected timestamp script content: %s", got)
	}
}

func makeGitDir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
}

func TestSyncGitCommitsAndPushes(t *testing.T) {
	baseDir := t.TempDir()
	remoteDir := t.TempDir()

	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	run(baseDir, "init", "-b", "main")
	run(baseDir, "config", "user.name", "Update Ipsets Test")
	run(baseDir, "config", "user.email", "update-ipsets@example.test")
	run(baseDir, "config", "commit.gpgsign", "false")
	run(baseDir, "config", "push.autoSetupRemote", "true")
	run(remoteDir, "init", "--bare", "-b", "main")
	run(baseDir, "remote", "add", "origin", remoteDir)

	if err := os.WriteFile(filepath.Join(baseDir, "sample.ipset"), []byte("1.2.3.4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	files := []GeneratedFile{
		{
			Path:            filepath.Join(baseDir, "sample.ipset"),
			Timestamp:       time.Unix(1_700_000_000, 0).UTC(),
			Redistributable: true,
		},
	}

	if err := SyncGit(SyncOptions{BaseDir: baseDir, PushToGit: true, PushMerged: true}, files); err != nil {
		t.Fatalf("SyncGit returned error: %v", err)
	}

	cmd := exec.Command("git", "log", "--format=%s", "-1")
	cmd.Dir = baseDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git log failed: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "update-ipsets: refresh generated data" {
		t.Fatalf("unexpected commit message %q", got)
	}

	cmd = exec.Command("git", "--git-dir", remoteDir, "show-ref", "--verify", "refs/heads/main")
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("remote show-ref failed: %v\n%s", err, out)
	}
}

func TestSyncGitIgnoresFilesOutsideRepository(t *testing.T) {
	baseDir := t.TempDir()
	remoteDir := t.TempDir()
	outsideDir := t.TempDir()

	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	run(baseDir, "init", "-b", "main")
	run(baseDir, "config", "user.name", "Update Ipsets Test")
	run(baseDir, "config", "user.email", "update-ipsets@example.test")
	run(baseDir, "config", "commit.gpgsign", "false")
	run(baseDir, "config", "push.autoSetupRemote", "true")
	run(remoteDir, "init", "--bare", "-b", "main")
	run(baseDir, "remote", "add", "origin", remoteDir)

	insidePath := filepath.Join(baseDir, "sample.ipset")
	outsidePath := filepath.Join(outsideDir, "sample.json")
	if err := os.WriteFile(insidePath, []byte("1.2.3.4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outsidePath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	files := []GeneratedFile{
		{Path: insidePath, Timestamp: time.Unix(1_700_000_000, 0).UTC(), Redistributable: true},
		{Path: outsidePath, Timestamp: time.Unix(1_700_000_000, 0).UTC(), Redistributable: true},
	}

	if err := SyncGit(SyncOptions{BaseDir: baseDir, PushToGit: true, PushMerged: true}, files); err != nil {
		t.Fatalf("SyncGit returned error: %v", err)
	}

	cmd := exec.Command("git", "show", "--name-only", "--format=", "HEAD")
	cmd.Dir = baseDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git show failed: %v\n%s", err, out)
	}
	got := strings.TrimSpace(string(out))
	if got != "sample.ipset" {
		t.Fatalf("unexpected committed files: %q", got)
	}
}
