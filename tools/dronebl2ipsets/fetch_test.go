package dronebl

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestFetchBuildzonePromotesOnlyBuildzone(t *testing.T) {
	dataDir := t.TempDir()
	recordPath := filepath.Join(t.TempDir(), "rsync-record.txt")
	rsyncPath := fakeRsync(t, `#!/bin/sh
set -eu
printf 'RSYNC_PASSWORD=%s\n' "$RSYNC_PASSWORD" > "$RSYNC_RECORD"
for arg do
	printf 'ARG=%s\n' "$arg" >> "$RSYNC_RECORD"
done
dest=""
for arg do
	dest="$arg"
done
mkdir -p "$dest"
printf 'fresh-buildzone\n' > "$dest/buildzone"
printf 'unused\n' > "$dest/buildzone.new"
printf 'unused\n' > "$dest/buildzone6"
`)

	for _, name := range []string{"buildzone.new", "buildzone.combined", "buildzone6", ".buildzone.123.tmp", "unconsumed.extra"} {
		if err := os.WriteFile(filepath.Join(dataDir, name), []byte("stale\n"), 0o600); err != nil {
			t.Fatalf("write stale %s: %v", name, err)
		}
	}
	if err := os.Mkdir(filepath.Join(dataDir, ".fetch-stale"), 0o700); err != nil {
		t.Fatalf("create stale fetch dir: %v", err)
	}

	t.Setenv("DRONEBL_RSYNC_PASSWORD", "secret")
	t.Setenv("RSYNC_PASSWORD", "old-secret")
	t.Setenv("RSYNC_RECORD", recordPath)

	if err := FetchBuildzone(t.Context(), FetchOptions{
		RsyncURL:    "rsync://example.test/dronebl/",
		DataDir:     dataDir,
		RsyncBinary: rsyncPath,
		Timeout:     17 * time.Second,
	}); err != nil {
		t.Fatalf("FetchBuildzone: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(dataDir, "buildzone"))
	if err != nil {
		t.Fatalf("read promoted buildzone: %v", err)
	}
	if got, want := string(body), "fresh-buildzone\n"; got != want {
		t.Fatalf("buildzone body = %q, want %q", got, want)
	}

	for _, name := range []string{"buildzone.new", "buildzone.combined", "buildzone6", ".buildzone.123.tmp", "unconsumed.extra", ".fetch-stale"} {
		if _, err := os.Stat(filepath.Join(dataDir, name)); !os.IsNotExist(err) {
			t.Fatalf("stale path %s still exists or stat failed with %v", name, err)
		}
	}
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		t.Fatalf("read data dir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".fetch-") {
			t.Fatalf("temporary fetch directory was not cleaned: %s", entry.Name())
		}
	}

	record, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatalf("read rsync record: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(record)), "\n")
	if !slices.Contains(lines, "RSYNC_PASSWORD=secret") {
		t.Fatalf("rsync did not receive DroneBL password, record:\n%s", record)
	}
	if slices.Contains(lines, "ARG=-HaSPvz") {
		t.Fatalf("rsync used partial/progress flags, record:\n%s", record)
	}
	if !slices.Contains(lines, "ARG=-HaSz") {
		t.Fatalf("rsync did not preserve archive/hardlink/sparse/compress flags, record:\n%s", record)
	}
	if !slices.Contains(lines, "ARG=--timeout=17") {
		t.Fatalf("rsync timeout not passed, record:\n%s", record)
	}
	if !slices.Contains(lines, "ARG=rsync://example.test/dronebl/buildzone") {
		t.Fatalf("rsync did not target buildzone only, record:\n%s", record)
	}
}

func TestFetchBuildzoneKeepsExistingBuildzoneOnRsyncFailure(t *testing.T) {
	dataDir := t.TempDir()
	rsyncPath := fakeRsync(t, `#!/bin/sh
set -eu
dest=""
for arg do
	dest="$arg"
done
mkdir -p "$dest"
printf 'partial-buildzone\n' > "$dest/buildzone"
exit 23
`)
	buildzonePath := filepath.Join(dataDir, "buildzone")
	if err := os.WriteFile(buildzonePath, []byte("old-buildzone\n"), 0o600); err != nil {
		t.Fatalf("write old buildzone: %v", err)
	}

	t.Setenv("DRONEBL_RSYNC_PASSWORD", "secret")
	err := FetchBuildzone(t.Context(), FetchOptions{
		RsyncURL:    "rsync://example.test/dronebl/",
		DataDir:     dataDir,
		RsyncBinary: rsyncPath,
	})
	if err == nil {
		t.Fatalf("FetchBuildzone succeeded, want rsync failure")
	}

	body, readErr := os.ReadFile(buildzonePath)
	if readErr != nil {
		t.Fatalf("read old buildzone after failure: %v", readErr)
	}
	if got, want := string(body), "old-buildzone\n"; got != want {
		t.Fatalf("buildzone body after failure = %q, want %q", got, want)
	}
	entries, readDirErr := os.ReadDir(dataDir)
	if readDirErr != nil {
		t.Fatalf("read data dir: %v", readDirErr)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".fetch-") {
			t.Fatalf("temporary fetch directory was not cleaned after failure: %s", entry.Name())
		}
	}
}

func TestFetchBuildzoneAcceptsDirectBuildzoneURL(t *testing.T) {
	dataDir := t.TempDir()
	recordPath := filepath.Join(t.TempDir(), "rsync-record.txt")
	rsyncPath := fakeRsync(t, `#!/bin/sh
set -eu
for arg do
	printf 'ARG=%s\n' "$arg" >> "$RSYNC_RECORD"
done
dest=""
for arg do
	dest="$arg"
done
mkdir -p "$dest"
printf 'fresh-buildzone\n' > "$dest/buildzone"
`)

	t.Setenv("DRONEBL_RSYNC_PASSWORD", "secret")
	t.Setenv("RSYNC_RECORD", recordPath)

	if err := FetchBuildzone(t.Context(), FetchOptions{
		RsyncURL:    "rsync://example.test/dronebl/buildzone",
		DataDir:     dataDir,
		RsyncBinary: rsyncPath,
	}); err != nil {
		t.Fatalf("FetchBuildzone: %v", err)
	}

	record, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatalf("read rsync record: %v", err)
	}
	if got := strings.Count(string(record), "ARG=rsync://example.test/dronebl/buildzone\n"); got != 1 {
		t.Fatalf("direct buildzone URL was not used exactly once, count=%d record:\n%s", got, record)
	}
}

func TestFetchBuildzoneOmitsRsyncTimeoutWhenUnset(t *testing.T) {
	dataDir := t.TempDir()
	recordPath := filepath.Join(t.TempDir(), "rsync-record.txt")
	rsyncPath := fakeRsync(t, `#!/bin/sh
set -eu
for arg do
	printf 'ARG=%s\n' "$arg" >> "$RSYNC_RECORD"
done
dest=""
for arg do
	dest="$arg"
done
mkdir -p "$dest"
printf 'fresh-buildzone\n' > "$dest/buildzone"
`)

	t.Setenv("DRONEBL_RSYNC_PASSWORD", "secret")
	t.Setenv("RSYNC_RECORD", recordPath)

	if err := FetchBuildzone(t.Context(), FetchOptions{
		RsyncURL:    "rsync://example.test/dronebl/",
		DataDir:     dataDir,
		RsyncBinary: rsyncPath,
	}); err != nil {
		t.Fatalf("FetchBuildzone: %v", err)
	}

	record, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatalf("read rsync record: %v", err)
	}
	if strings.Contains(string(record), "ARG=--timeout=") {
		t.Fatalf("rsync timeout was passed when unset, record:\n%s", record)
	}
}

func TestBuildzoneRsyncURL(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "module root with trailing slash",
			raw:  "rsync://example.test/dronebl/",
			want: "rsync://example.test/dronebl/buildzone",
		},
		{
			name: "module root without trailing slash",
			raw:  "rsync://example.test/dronebl",
			want: "rsync://example.test/dronebl/buildzone",
		},
		{
			name: "direct buildzone URL",
			raw:  "rsync://example.test/dronebl/buildzone",
			want: "rsync://example.test/dronebl/buildzone",
		},
		{
			name: "direct buildzone URL with trailing slash",
			raw:  "rsync://example.test/dronebl/buildzone/",
			want: "rsync://example.test/dronebl/buildzone",
		},
		{
			name: "path-only buildzone",
			raw:  "buildzone",
			want: "buildzone",
		},
		{
			name: "absolute path-only buildzone",
			raw:  "/buildzone",
			want: "/buildzone",
		},
		{
			name: "empty",
			raw:  "",
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := buildzoneRsyncURL(tc.raw); got != tc.want {
				t.Fatalf("buildzoneRsyncURL(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func fakeRsync(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "rsync")
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatalf("write fake rsync: %v", err)
	}
	return path
}
