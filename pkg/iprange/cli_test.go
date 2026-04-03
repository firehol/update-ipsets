package iprange

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunCLIHasIPv6(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunCLI(t.Context(), &stdout, &stderr, strings.NewReader(""), []string{"--has-ipv6"})
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stderr.String(), "IPv6 support is present") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunCLIV6CountUnique(t *testing.T) {
	var stdout, stderr bytes.Buffer
	input := strings.NewReader("2001:db8::1\n2001:db8::2\n")
	code := RunCLI(t.Context(), &stdout, &stderr, input, []string{"-6", "--count-unique", "--header"})
	if code != 0 {
		t.Fatalf("exit code = %d stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); got != "entries,unique_ips\n1,2\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestRunCLIRejectsMixedFamilyFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunCLI(t.Context(), &stdout, &stderr, strings.NewReader(""), []string{"-4", "-6"})
	if code == 0 {
		t.Fatal("expected failure for mixed family flags")
	}
	if !strings.Contains(stderr.String(), "cannot combine IPv4 and IPv6 flags") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
