package iprange

import "testing"

func TestIprangeMetricOperation(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{name: "iprange.merge.ops", want: "merge"},
		{name: "iprange.count_unique.ops", want: "count_unique"},
		{name: "iprange.load.binary", want: "load_binary"},
		{name: "iprange.binary.searches", want: "binary"},
		{name: "", want: "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := iprangeMetricOperation(tt.name); got != tt.want {
				t.Fatalf("iprangeMetricOperation(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}
