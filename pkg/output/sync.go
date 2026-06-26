package output

import (
	"bytes"
	"context"
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
	Timeout       time.Duration
}

const (
	DefaultPushToGitTimeout = 10 * time.Minute
	gitCommandWaitDelay     = 5 * time.Second
)

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
	edit, err := fileutil.ReadFileUnderRoot(baseDir, "README-EDIT.md")
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := writeAtomic(editPath, nil, fileutil.GeneratedFileMode); err != nil {
			return err
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
	return writeAtomic(filepath.Join(baseDir, "README.md"), buf.Bytes(), fileutil.GeneratedFileMode)
}

func WriteGitIgnore(baseDir string, files []GeneratedFile) error {
	if !HasGitDir(baseDir) {
		return nil
	}
	path := filepath.Join(baseDir, ".gitignore")
	data, err := fileutil.ReadFileUnderRoot(baseDir, ".gitignore")
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
	return writeAtomic(path, buf.Bytes(), fileutil.GeneratedFileMode)
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
	return SyncGitContext(context.Background(), opts, files)
}

func SyncGitContext(ctx context.Context, opts SyncOptions, files []GeneratedFile) error {
	ctx = nonNilContext(ctx)
	if !opts.PushToGit || opts.BaseDir == "" {
		return nil
	}
	timeout, err := normalizeGitTimeout(opts.Timeout)
	if err != nil {
		return err
	}
	isRepo, err := isGitRepoContext(ctx, opts.BaseDir, timeout)
	if err != nil {
		return err
	}
	if !isRepo {
		return nil
	}
	defer runGitAutoGC(ctx, opts.BaseDir, timeout)

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
	if err := runGit(ctx, opts.BaseDir, timeout, addArgs...); err != nil {
		return err
	}
	clean, err := gitClean(ctx, opts.BaseDir, timeout)
	if err != nil || clean {
		return err
	}

	if opts.PushMerged || len(tracked) <= 1 {
		commitArgs := append([]string{"commit"}, opts.CommitOptions...)
		commitArgs = append(commitArgs, "-m", "update-ipsets: refresh generated data")
		if err := runGit(ctx, opts.BaseDir, timeout, commitArgs...); err != nil {
			return err
		}
	} else {
		for _, file := range tracked {
			args := append([]string{"commit"}, opts.CommitOptions...)
			args = append(args, "-m", "update-ipsets: refresh "+file, "--", file)
			if err := runGit(ctx, opts.BaseDir, timeout, args...); err != nil {
				return err
			}
		}
	}
	pushArgs := append([]string{"push"}, opts.PushOptions...)
	if err := runGit(ctx, opts.BaseDir, timeout, pushArgs...); err != nil {
		return err
	}
	slog.Info("git push completed", "dir", opts.BaseDir)
	return nil
}

func runGitAutoGC(ctx context.Context, dir string, timeout time.Duration) {
	output, err := runGitCommand(ctx, dir, timeout, "gc", "--auto")
	if err != nil {
		slog.Warn("git auto-gc failed", "dir", dir, "output", strings.TrimSpace(string(output)))
	}
}

func runGit(ctx context.Context, dir string, timeout time.Duration, args ...string) error {
	output, err := runGitCommand(ctx, dir, timeout, args...)
	if err != nil {
		slog.Error("git command failed", "args", strings.Join(args, " "), "dir", dir, "output", strings.TrimSpace(string(output)))
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return fmt.Errorf("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(string(output)))
	}
	slog.Debug("git command succeeded", "args", strings.Join(args, " "), "dir", dir)
	return nil
}

func gitClean(ctx context.Context, dir string, timeout time.Duration) (bool, error) {
	_, err := runGitCommand(ctx, dir, timeout, "diff", "--cached", "--quiet", "--")
	if err == nil {
		return true, nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false, err
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

func isGitRepoContext(ctx context.Context, dir string, timeout time.Duration) (bool, error) {
	_, err := runGitCommand(ctx, dir, timeout, "rev-parse", "--is-inside-work-tree")
	if err == nil {
		return true, nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false, err
	}
	return false, nil
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

func runGitCommand(ctx context.Context, dir string, timeout time.Duration, args ...string) ([]byte, error) {
	ctx = nonNilContext(ctx)
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	prepareGitCommand(cmd)
	cmd.Cancel = func() error {
		return killGitCommandTree(cmd)
	}
	cmd.WaitDelay = gitCommandWaitDelay
	output, err := cmd.CombinedOutput()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return output, fmt.Errorf("git %s canceled: %w", strings.Join(args, " "), ctxErr)
	}
	return output, err
}

func normalizeGitTimeout(timeout time.Duration) (time.Duration, error) {
	if timeout < 0 {
		return 0, fmt.Errorf("git timeout must be zero or positive, got %s", timeout)
	}
	if timeout == 0 {
		return DefaultPushToGitTimeout, nil
	}
	return timeout, nil
}

func nonNilContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
