package engine

import "testing"

func TestFeedEntitySidecarResultBufferSizeBoundedByWorkers(t *testing.T) {
	cases := []struct {
		name    string
		feeds   int
		workers int
		want    int
	}{
		{name: "no feeds", feeds: 0, workers: 4, want: 0},
		{name: "invalid workers", feeds: 10, workers: 0, want: 1},
		{name: "workers below feeds", feeds: 1000, workers: 4, want: 4},
		{name: "workers above feeds", feeds: 3, workers: 10, want: 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := feedEntitySidecarResultBufferSize(tc.feeds, tc.workers); got != tc.want {
				t.Fatalf("feedEntitySidecarResultBufferSize(%d, %d) = %d, want %d", tc.feeds, tc.workers, got, tc.want)
			}
		})
	}
}
