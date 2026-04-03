package kernel

import "testing"

func TestEntryMode(t *testing.T) {
	tests := []struct {
		typeName string
		hash     string
		want     string
	}{
		{typeName: "hash:ip", hash: "net", want: "ip"},
		{typeName: "hash:net", hash: "ip", want: "net"},
		{typeName: "", hash: "net", want: "net"},
		{typeName: "", hash: "ip", want: "ip"},
	}
	for _, tt := range tests {
		if got := entryMode(tt.typeName, tt.hash); got != tt.want {
			t.Fatalf("entryMode(%q, %q) = %q, want %q", tt.typeName, tt.hash, got, tt.want)
		}
	}
}

func TestParseEntryHashIP(t *testing.T) {
	entry, err := parseEntry("ip", "1.2.3.4")
	if err != nil {
		t.Fatalf("parseEntry returned error: %v", err)
	}
	if entry.CIDR != 0 {
		t.Fatalf("expected CIDR 0 for hash:ip entry, got %d", entry.CIDR)
	}
	if got := entry.IP.String(); got != "1.2.3.4" {
		t.Fatalf("unexpected IP %q", got)
	}
}

func TestParseEntryHashNet(t *testing.T) {
	entry, err := parseEntry("net", "1.2.3.0/24")
	if err != nil {
		t.Fatalf("parseEntry returned error: %v", err)
	}
	if entry.CIDR != 24 {
		t.Fatalf("expected CIDR 24, got %d", entry.CIDR)
	}
	ipEntry, err := parseEntry("net", "1.2.3.4")
	if err != nil {
		t.Fatalf("parseEntry returned error: %v", err)
	}
	if ipEntry.CIDR != 32 {
		t.Fatalf("expected CIDR 32, got %d", ipEntry.CIDR)
	}
}

func TestTemporaryNameLength(t *testing.T) {
	got := temporaryName("firehol_level1")
	if len(got) > 31 {
		t.Fatalf("temporary name %q exceeds ipset limit", got)
	}
}
