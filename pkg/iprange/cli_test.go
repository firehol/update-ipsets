package iprange

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type cliResult struct {
	code   int
	stdout string
	stderr string
}

func runCLITest(t *testing.T, input string, args ...string) cliResult {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := RunCLI(t.Context(), &stdout, &stderr, strings.NewReader(input), args)
	return cliResult{
		code:   code,
		stdout: stdout.String(),
		stderr: stderr.String(),
	}
}

func requireCLI(t *testing.T, got cliResult, wantCode int, wantStdout, wantStderr string) {
	t.Helper()
	if got.code != wantCode {
		t.Fatalf("exit code = %d, want %d\nstdout=%q\nstderr=%q", got.code, wantCode, got.stdout, got.stderr)
	}
	if got.stdout != wantStdout {
		t.Fatalf("stdout = %q, want %q", got.stdout, wantStdout)
	}
	if got.stderr != wantStderr {
		t.Fatalf("stderr = %q, want %q", got.stderr, wantStderr)
	}
}

func writeCLIFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestRunCLIHasIPv6(t *testing.T) {
	got := runCLITest(t, "", "--has-ipv6")
	requireCLI(t, got, 0, "", "yes, IPv6 support is present.\n")
}

func TestRunCLIV6CountUnique(t *testing.T) {
	got := runCLITest(t, "2001:db8::1\n2001:db8::2\n", "-6", "--count-unique", "--header")
	requireCLI(t, got, 0, "entries,unique_ips\n1,2\n", "")
}

func TestRunCLIRejectsMixedFamilyFlags(t *testing.T) {
	got := runCLITest(t, "", "-4", "-6")
	requireCLI(t, got, 1, "", "iprange: cannot combine IPv4 and IPv6 flags in one invocation\n")
}

func TestRunCLIV4DocumentedModes(t *testing.T) {
	dir := t.TempDir()
	left := writeCLIFile(t, dir, "left.ipset", "192.0.2.1\n192.0.2.2\n")
	right := writeCLIFile(t, dir, "right.ipset", "192.0.2.2\n192.0.2.3\n")

	cases := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
	}{
		{
			name:       "combine stdin with custom IP and network affixes",
			args:       []string{"--print-prefix-ips", "ip:", "--print-prefix-nets", "net:", "--print-suffix-ips", ";", "--print-suffix-nets", "!"},
			wantStdout: "ip:192.0.2.1;\nnet:198.51.100.0/24!\n",
		},
		{
			name:       "common",
			args:       []string{"--common", left, right, "--print-ranges"},
			wantStdout: "192.0.2.2-192.0.2.2\n",
		},
		{
			name:       "exclude next",
			args:       []string{left, "--exclude-next", right, "--print-ranges"},
			wantStdout: "192.0.2.1-192.0.2.1\n",
		},
		{
			name:     "quiet diff exits one on difference",
			args:     []string{left, "--diff", right, "--quiet"},
			wantCode: 1,
		},
		{
			name:       "compare aliases",
			args:       []string{left, "as", "left", right, "as", "right", "--compare", "--header"},
			wantStdout: "name1,name2,entries1,entries2,ips1,ips2,combined_ips,common_ips\nleft,right,1,1,2,2,3,1\n",
		},
		{
			name:       "compare first",
			args:       []string{left, "as", "base", right, "as", "candidate", "--compare-first", "--header"},
			wantStdout: "name,entries,unique_ips,common_ips\ncandidate,1,2,1\n",
		},
		{
			name:       "compare next",
			args:       []string{left, "as", "left", "--compare-next", right, "as", "right", "--header"},
			wantStdout: "name1,name2,entries1,entries2,ips1,ips2,combined_ips,common_ips\nleft,right,1,1,2,2,3,1\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := "192.0.2.1\n198.51.100.0/24\n"
			got := runCLITest(t, input, tc.args...)
			requireCLI(t, got, tc.wantCode, tc.wantStdout, "")
		})
	}
}

func TestRunCLIV4FileListAndDirectoryInputs(t *testing.T) {
	dir := t.TempDir()
	first := writeCLIFile(t, dir, "a.ipset", "192.0.2.1\n192.0.2.2\n")
	second := writeCLIFile(t, dir, "b.ipset", "198.51.100.1\n")
	list := writeCLIFile(t, dir, "inputs.txt", "\n# ignored\n"+first+" # first\n"+second+"\n")

	got := runCLITest(t, "", "--count-unique-all", "--header", "@"+list)
	want := "name,entries,unique_ips\n" +
		first + ",1,2\n" +
		second + ",1,1\n"
	requireCLI(t, got, 0, want, "")

	got = runCLITest(t, "", "--count-unique-all", "--header", "@"+dir)
	want = "name,entries,unique_ips\n" +
		first + ",1,2\n" +
		second + ",1,1\n" +
		list + ",0,0\n"
	requireCLI(t, got, 0, want, "")
}

func TestRunCLIV4PrefixAndReduceOptions(t *testing.T) {
	got := runCLITest(t, "192.0.2.1\n", "--print-prefix", "<", "--print-suffix", ">")
	requireCLI(t, got, 0, "<192.0.2.1>\n", "")

	got = runCLITest(t, "192.0.2.0\n", "--default-prefix", "24")
	requireCLI(t, got, 0, "192.0.2.0/24\n", "")

	got = runCLITest(t, "192.0.2.0/24\n", "--prefixes", "24,32")
	requireCLI(t, got, 0, "192.0.2.0/24\n", "")

	got = runCLITest(t, "192.0.2.0/24\n", "--min-prefix", "24")
	requireCLI(t, got, 0, "192.0.2.0/24\n", "")

	got = runCLITest(t, "192.0.2.0/24\n", "--reduce-factor", "0", "--reduce-entries", "1")
	requireCLI(t, got, 0, "192.0.2.0/24\n", "")

	got = runCLITest(t, "192.0.2.1\n192.0.2.2\n", "--count-unique", "--header")
	requireCLI(t, got, 0, "entries,unique_ips\n1,2\n", "")
}

func TestRunCLIV6DocumentedModes(t *testing.T) {
	dir := t.TempDir()
	left := writeCLIFile(t, dir, "left6.ipset", "2001:db8::1\n2001:db8::2\n")
	right := writeCLIFile(t, dir, "right6.ipset", "2001:db8::2\n2001:db8::3\n")

	cases := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
	}{
		{
			name:       "common",
			args:       []string{"-6", "--common", left, right, "--print-ranges"},
			wantStdout: "2001:db8::2\n",
		},
		{
			name:       "exclude next",
			args:       []string{"-6", left, "--exclude-next", right, "--print-ranges"},
			wantStdout: "2001:db8::1\n",
		},
		{
			name:     "quiet diff exits one on difference",
			args:     []string{"-6", left, "--diff", right, "--quiet"},
			wantCode: 1,
		},
		{
			name:       "compare aliases",
			args:       []string{"-6", left, "as", "left", right, "as", "right", "--compare", "--header"},
			wantStdout: "name1,name2,entries1,entries2,ips1,ips2,combined_ips,common_ips\nleft,right,1,1,2,2,3,1\n",
		},
		{
			name:       "compare first",
			args:       []string{"-6", left, "as", "base", right, "as", "candidate", "--compare-first", "--header"},
			wantStdout: "name,entries,unique_ips,common_ips\ncandidate,1,2,1\n",
		},
		{
			name:       "compare next",
			args:       []string{"-6", left, "as", "left", "--compare-next", right, "as", "right", "--header"},
			wantStdout: "name1,name2,entries1,entries2,ips1,ips2,combined_ips,common_ips\nleft,right,1,1,2,2,3,1\n",
		},
		{
			name:       "count all",
			args:       []string{"-6", left, "as", "left", right, "as", "right", "--count-unique-all", "--header"},
			wantStdout: "name,entries,unique_ips\nleft,1,2\nright,1,2\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runCLITest(t, "", tc.args...)
			requireCLI(t, got, tc.wantCode, tc.wantStdout, "")
		})
	}
}

func TestRunCLIV6FileListInput(t *testing.T) {
	dir := t.TempDir()
	first := writeCLIFile(t, dir, "a6.ipset", "2001:db8::1\n2001:db8::2\n")
	second := writeCLIFile(t, dir, "b6.ipset", "2001:db8::3\n")
	list := writeCLIFile(t, dir, "inputs6.txt", first+"\n"+second+"\n")

	got := runCLITest(t, "", "-6", "--count-unique-all", "--header", "@"+list)
	want := "name,entries,unique_ips\n" +
		first + ",1,2\n" +
		second + ",1,1\n"
	requireCLI(t, got, 0, want, "")
}

func TestRunCLIReportsInputLoadErrors(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.ipset")
	got := runCLITest(t, "", missing)
	if got.code != 1 {
		t.Fatalf("exit code = %d, want 1", got.code)
	}
	if got.stdout != "" {
		t.Fatalf("stdout = %q, want empty", got.stdout)
	}
	wantPrefix := "iprange: open " + missing + ":"
	if !strings.HasPrefix(got.stderr, wantPrefix) {
		t.Fatalf("stderr = %q, want prefix %q", got.stderr, wantPrefix)
	}
}

func TestRunCLIOptionValidationAndFeatureFlags(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{
			name:       "help",
			args:       []string{"--help"},
			wantStdout: "Usage: update-ipsets iprange [options] file1 file2 ...\n\nAddress family:\n  -4 | --ipv4    IPv4 mode (default)\n  -6 | --ipv6    IPv6 mode\n  --has-ipv6     feature detection for scripts\n\nSupported modes:\n  --union | --combine | --merge | --optimize\n  --common | --intersect\n  --exclude-next | --except\n  --diff | --diff-next\n  --compare | --compare-first | --compare-next\n  --count-unique | --count-unique-all\n  --ipset-reduce | --reduce-factor\n\nInput expansion:\n  @filelist    load one file path per line\n  @directory   load all regular files sorted by name\n",
		},
		{
			name:       "version",
			args:       []string{"--version"},
			wantStdout: "iprange go-dev\n",
		},
		{
			name:       "compare feature flag",
			args:       []string{"--has-compare"},
			wantStderr: "yes, compare and reduce is present.\n",
		},
		{
			name:       "file list feature flag",
			args:       []string{"--has-directory-loading"},
			wantStderr: "yes, @filename and @directory support is present.\n",
		},
		{
			name:       "missing value",
			args:       []string{"--min-prefix"},
			wantCode:   1,
			wantStderr: "iprange: --min-prefix requires a value\n",
		},
		{
			name:       "bad dns threads",
			args:       []string{"--dns-threads", "0"},
			wantCode:   1,
			wantStderr: "iprange: invalid dns thread count \"0\"\n",
		},
		{
			name:       "alias without input",
			args:       []string{"as", "orphan"},
			wantCode:   1,
			wantStderr: "iprange: alias requires a previous input\n",
		},
		{
			name:       "diff without left input",
			args:       []string{"--diff"},
			wantCode:   1,
			wantStderr: "iprange: an ipset is needed before --diff\n",
		},
		{
			name:       "ipv6 bad default prefix",
			args:       []string{"-6", "--default-prefix", "bad"},
			wantCode:   1,
			wantStderr: "iprange: invalid --default-prefix value \"bad\"\n",
		},
		{
			name:       "ipv6 bad prefixes",
			args:       []string{"-6", "--prefixes", "129"},
			wantCode:   1,
			wantStderr: "iprange: invalid prefix \"129\"\n",
		},
		{
			name:       "bad reduce factor",
			args:       []string{"--reduce-factor", "-1"},
			wantCode:   1,
			wantStderr: "iprange: invalid reduce factor \"-1\"\n",
		},
		{
			name:       "bad reduce entries",
			args:       []string{"--reduce-entries", "-1"},
			wantCode:   1,
			wantStderr: "iprange: invalid reduce entries \"-1\"\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runCLITest(t, "", tc.args...)
			requireCLI(t, got, tc.wantCode, tc.wantStdout, tc.wantStderr)
		})
	}
}
