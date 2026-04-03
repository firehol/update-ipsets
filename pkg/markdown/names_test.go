package markdown

import "testing"

func TestCountryNameLookup(t *testing.T) {
	tests := []struct {
		code, want string
	}{
		{"US", "United States"},
		{"DE", "Germany"},
		{"CN", "China"},
		{"XX", "XX"},
		{"", ""},
		{"COUNTRYLESS", "Countryless"},
	}
	for _, tc := range tests {
		got := countryName(tc.code)
		if got != tc.want {
			t.Errorf("countryName(%q) = %q, want %q", tc.code, got, tc.want)
		}
	}
}

func TestASNDisplayNameFunc(t *testing.T) {
	tests := []struct {
		asn  uint32
		name string
		want string
	}{
		{13335, "Cloudflare", "AS13335 (Cloudflare)"},
		{13335, "", "AS13335"},
		{0, "", "AS0"},
	}
	for _, tc := range tests {
		got := asnDisplayName(tc.asn, tc.name)
		if got != tc.want {
			t.Errorf("asnDisplayName(%d, %q) = %q, want %q", tc.asn, tc.name, got, tc.want)
		}
	}
}
