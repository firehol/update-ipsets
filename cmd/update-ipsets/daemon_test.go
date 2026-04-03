package main

import (
	"slices"
	"testing"
)

func TestCompactCLIArgsDropsEmptyEntries(t *testing.T) {
	input := []string{
		"--config", "/opt/update-ipsets/etc/config",
		"--listen", ":18888",
		"",
		"--admin-auth-mode=disabled",
		"--allow-unauthenticated-admin",
	}

	got := compactCLIArgs(append([]string(nil), input...))
	want := []string{
		"--config", "/opt/update-ipsets/etc/config",
		"--listen", ":18888",
		"--admin-auth-mode=disabled",
		"--allow-unauthenticated-admin",
	}

	if !slices.Equal(got, want) {
		t.Fatalf("compactCLIArgs() = %#v, want %#v", got, want)
	}
}
