package output

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/internal/fileutil"
)

type GeneratedFile struct {
	Path            string
	Timestamp       time.Time
	Redistributable bool
}

type SyncOptions struct {
	BaseDir       string
	PushToGit     bool
	PushMerged    bool
	CommitOptions []string
	PushOptions   []string
}

func WriteREADME(baseDir string, setInfo map[string]string) error {
	if !HasGitDir(baseDir) {
		return nil
	}
	names := make([]string, 0, len(setInfo))
	for name := range setInfo {
		names = append(names, name)
	}
	slices.Sort(names)

	editPath := filepath.Join(baseDir, "README-EDIT.md")
	edit, err := os.ReadFile(editPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		file, createErr := os.OpenFile(editPath, os.O_CREATE|os.O_WRONLY, 0o600)
		if createErr != nil {
			return createErr
		}
		if closeErr := file.Close(); closeErr != nil {
			return closeErr
		}
	}

	var buf bytes.Buffer
	buf.Write(edit)
	buf.WriteByte('\n')
	buf.WriteString("The following list was automatically generated on ")
	buf.WriteString(posixDate(time.Now().UTC()))
	buf.WriteString(".\n\n")
	buf.WriteString("The update frequency is the maximum allowed by internal configuration. A list will never be downloaded sooner than the update frequency stated. A list may also not be downloaded, after this frequency expired, if it has not been modified on the server (as reported by HTTP `IF_MODIFIED_SINCE` method).\n\n")
	buf.WriteString("name|info|type|entries|update|\n")
	buf.WriteString(":--:|:--:|:--:|:-----:|:----:|\n")
	for _, name := range names {
		buf.WriteString(strings.TrimSpace(setInfo[name]))
		buf.WriteByte('\n')
	}
	return writeAtomic(filepath.Join(baseDir, "README.md"), buf.Bytes(), 0o600)
}

func WriteGitIgnore(baseDir string, files []GeneratedFile) error {
	if !HasGitDir(baseDir) {
		return nil
	}
	path := filepath.Join(baseDir, ".gitignore")
	data, err := os.ReadFile(path)
	var buf bytes.Buffer
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		buf.WriteString("*.setinfo\n*.source\n")
	} else {
		buf.Write(data)
	}

	existing := make(map[string]struct{})
	for _, line := range strings.Split(buf.String(), "\n") {
		existing[line] = struct{}{}
	}
	addLine := func(line string) {
		if line == "" {
			return
		}
		if _, ok := existing[line]; ok {
			return
		}
		if buf.Len() > 0 && !bytes.HasSuffix(buf.Bytes(), []byte("\n")) {
			buf.WriteByte('\n')
		}
		buf.WriteString(line)
		buf.WriteByte('\n')
		existing[line] = struct{}{}
	}

	for _, file := range files {
		if file.Redistributable {
			continue
		}
		rel, ok := repoRelativePath(baseDir, file.Path)
		if !ok {
			continue
		}
		if !isIPSetOutputFile(rel) {
			continue
		}
		addLine(filepath.ToSlash(rel))
	}
	if err == nil && bytes.Equal(data, buf.Bytes()) {
		return nil
	}
	return writeAtomic(path, buf.Bytes(), 0o600)
}

func WriteTimestampScript(baseDir string, files []GeneratedFile) error {
	if !HasGitDir(baseDir) {
		return nil
	}
	type timestampFile struct {
		rel string
		ts  time.Time
	}
	timestampFiles := make([]timestampFile, 0, len(files))
	seen := make(map[string]struct{})
	for _, file := range files {
		if file.Timestamp.IsZero() {
			continue
		}
		rel, ok := repoRelativePath(baseDir, file.Path)
		if !ok || !isIPSetOutputFile(rel) {
			continue
		}
		rel = filepath.ToSlash(rel)
		if _, ok := seen[rel]; ok {
			continue
		}
		seen[rel] = struct{}{}
		timestampFiles = append(timestampFiles, timestampFile{rel: rel, ts: file.Timestamp})
	}
	sort.Slice(timestampFiles, func(i, j int) bool {
		return timestampFiles[i].rel < timestampFiles[j].rel
	})
	var buf bytes.Buffer
	buf.WriteString("#!/bin/bash\n")
	buf.WriteString("[ ! \"$1\" = \"YES_I_AM_SURE_DO_IT_PLEASE\" ] && echo \"READ ME NOW\" && exit 1\n")
	for _, file := range timestampFiles {
		fmt.Fprintf(&buf, "[ -f %s ] && touch --date=@%d %s\n",
			shellSingleQuote(file.rel),
			file.ts.UTC().Unix(),
			shellSingleQuote(file.rel))
	}
	path := filepath.Join(baseDir, "set_file_timestamps.sh")
	return writeAtomic(path, buf.Bytes(), 0o755)
}

func SyncGit(opts SyncOptions, files []GeneratedFile) error {
	if !opts.PushToGit || opts.BaseDir == "" || !isGitRepo(opts.BaseDir) {
		return nil
	}

	tracked := make([]string, 0, len(files)+3)
	for _, file := range files {
		if !file.Redistributable {
			continue
		}
		if rel, ok := repoRelativePath(opts.BaseDir, file.Path); ok {
			tracked = append(tracked, rel)
		}
	}
	for _, extra := range []string{"README.md", ".gitignore", "set_file_timestamps.sh"} {
		if _, err := os.Stat(filepath.Join(opts.BaseDir, extra)); err == nil {
			tracked = append(tracked, extra)
		}
	}
	tracked = dedupe(tracked)
	if len(tracked) == 0 {
		return nil
	}

	addArgs := append([]string{"add", "--"}, tracked...)
	if err := runGit(opts.BaseDir, addArgs...); err != nil {
		return err
	}
	clean, err := gitClean(opts.BaseDir)
	if err != nil || clean {
		return err
	}

	if opts.PushMerged || len(tracked) <= 1 {
		commitArgs := append([]string{"commit"}, opts.CommitOptions...)
		commitArgs = append(commitArgs, "-m", "update-ipsets: refresh generated data")
		if err := runGit(opts.BaseDir, commitArgs...); err != nil {
			return err
		}
	} else {
		for _, file := range tracked {
			args := append([]string{"commit"}, opts.CommitOptions...)
			args = append(args, "-m", "update-ipsets: refresh "+file, "--", file)
			if err := runGit(opts.BaseDir, args...); err != nil {
				return err
			}
		}
	}
	pushArgs := append([]string{"push"}, opts.PushOptions...)
	if err := runGit(opts.BaseDir, pushArgs...); err != nil {
		return err
	}
	slog.Info("git push completed", "dir", opts.BaseDir)
	return nil
}

func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("git command failed", "args", strings.Join(args, " "), "dir", dir, "output", strings.TrimSpace(string(output)))
		return fmt.Errorf("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(string(output)))
	}
	slog.Debug("git command succeeded", "args", strings.Join(args, " "), "dir", dir)
	return nil
}

func gitClean(dir string) (bool, error) {
	cmd := exec.Command("git", "diff", "--cached", "--quiet", "--")
	cmd.Dir = dir
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

func isGitRepo(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	return cmd.Run() == nil
}

func HasGitDir(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil && info.IsDir()
}

func posixDate(t time.Time) string {
	return fmt.Sprintf("%s %s %2d %s UTC %d",
		t.Format("Mon"),
		t.Format("Jan"),
		t.Day(),
		t.Format("15:04:05"),
		t.Year())
}

func isIPSetOutputFile(path string) bool {
	return strings.HasSuffix(path, ".ipset") || strings.HasSuffix(path, ".netset")
}

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	return fileutil.WriteAtomic(path, data, mode)
}

func dedupe(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	slices.Sort(values)
	out := values[:0]
	var prev string
	for i, value := range values {
		if i == 0 || value != prev {
			out = append(out, value)
			prev = value
		}
	}
	return out
}

func repoRelativePath(baseDir, path string) (string, bool) {
	rel, err := filepath.Rel(baseDir, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return rel, true
}
