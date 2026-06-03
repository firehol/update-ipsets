package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestRunDispatchValidationCommands(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr []string
	}{
		{
			name:     "no command",
			wantCode: 1,
			wantStderr: []string{
				"Usage: update-ipsets <command> [options]",
				"iprange   standalone iprange-compatible mode",
			},
		},
		{
			name:       "help",
			args:       []string{"help"},
			wantCode:   1,
			wantStdout: "Usage: update-ipsets <command> [options]\n\nCommands:\n  iprange   standalone iprange-compatible mode\n  query     query which lists contain an IP\n  enable    enable one or more sources\n  daemon    scheduler + web server + API + admin\n  version   print version\n",
		},
		{
			name:       "version",
			args:       []string{"version"},
			wantStdout: "update-ipsets dev\n",
		},
		{
			name:     "unknown",
			args:     []string{"missing"},
			wantCode: 1,
			wantStderr: []string{
				`update-ipsets: unknown subcommand "missing"`,
				"Usage: update-ipsets <command> [options]",
			},
		},
		{
			name: "iprange feature probe",
			args: []string{"iprange", "--has-ipv6"},
			wantStderr: []string{
				"yes, IPv6 support is present.",
			},
		},
		{
			name:     "query validation",
			args:     []string{"query"},
			wantCode: 2,
			wantStderr: []string{
				"update-ipsets query: missing IP argument",
			},
		},
		{
			name:     "enable validation",
			args:     []string{"enable"},
			wantCode: 2,
			wantStderr: []string{
				"update-ipsets enable: provide one or more names or use --all",
			},
		},
		{
			name:     "cache merge validation",
			args:     []string{"cache-merge"},
			wantCode: 2,
			wantStderr: []string{
				"cache-merge: --legacy, --local-only and --out are required",
			},
		},
		{
			name:     "daemon flag validation",
			args:     []string{"daemon", "--definitely-not-a-flag"},
			wantCode: 2,
			wantStderr: []string{
				"flag provided but not defined: -definitely-not-a-flag",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := captureCommandOutput(t, func() int {
				return run(tc.args)
			})
			if got.code != tc.wantCode {
				t.Fatalf("run(%v) code = %d, want %d\nstdout=%q\nstderr=%q", tc.args, got.code, tc.wantCode, got.stdout, got.stderr)
			}
			if got.stdout != tc.wantStdout {
				t.Fatalf("run(%v) stdout = %q, want %q", tc.args, got.stdout, tc.wantStdout)
			}
			for _, want := range tc.wantStderr {
				if !strings.Contains(got.stderr, want) {
					t.Fatalf("run(%v) stderr = %q, want substring %q", tc.args, got.stderr, want)
				}
			}
		})
	}
}

func TestParseSetExpression(t *testing.T) {
	include, exclude := parseSetExpression("level1 + level2 - bogons - local + scanners")
	if want := []string{"level1", "level2", "scanners"}; !slices.Equal(include, want) {
		t.Fatalf("include = %#v, want %#v", include, want)
	}
	if want := []string{"bogons", "local"}; !slices.Equal(exclude, want) {
		t.Fatalf("exclude = %#v, want %#v", exclude, want)
	}
}

func TestReadNameListIgnoresBlankAndCommentLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "names.txt")
	if err := os.WriteFile(path, []byte("\n# comment\nfirst\n  second  \n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := readNameList(path)
	if err != nil {
		t.Fatalf("readNameList() error = %v", err)
	}
	if want := []string{"first", "second"}; !slices.Equal(got, want) {
		t.Fatalf("readNameList() = %#v, want %#v", got, want)
	}
}

func TestNewLoggerLevelSelection(t *testing.T) {
	cases := []struct {
		name    string
		silent  bool
		verbose bool
		level   slog.Level
		want    bool
	}{
		{name: "default skips debug", level: slog.LevelDebug},
		{name: "default allows info", level: slog.LevelInfo, want: true},
		{name: "silent skips warn", silent: true, level: slog.LevelWarn},
		{name: "silent allows error", silent: true, level: slog.LevelError, want: true},
		{name: "verbose wins", silent: true, verbose: true, level: slog.LevelDebug, want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logger := newLogger(tc.silent, tc.verbose)
			if got := logger.Enabled(context.Background(), tc.level); got != tc.want {
				t.Fatalf("Enabled(%s) = %v, want %v", tc.level, got, tc.want)
			}
		})
	}
}

type commandResult struct {
	code   int
	stdout string
	stderr string
}

func captureCommandOutput(t *testing.T, fn func() int) commandResult {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		_ = outR.Close()
		_ = outW.Close()
		t.Fatal(err)
	}
	os.Stdout = outW
	os.Stderr = errW
	t.Cleanup(func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	})

	code := fn()
	if err := outW.Close(); err != nil {
		t.Fatalf("close stdout pipe: %v", err)
	}
	if err := errW.Close(); err != nil {
		t.Fatalf("close stderr pipe: %v", err)
	}
	stdout, err := io.ReadAll(outR)
	if err != nil {
		t.Fatalf("read stdout pipe: %v", err)
	}
	stderr, err := io.ReadAll(errR)
	if err != nil {
		t.Fatalf("read stderr pipe: %v", err)
	}
	if err := outR.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}
	if err := errR.Close(); err != nil {
		t.Fatalf("close stderr reader: %v", err)
	}

	return commandResult{
		code:   code,
		stdout: string(stdout),
		stderr: string(stderr),
	}
}
