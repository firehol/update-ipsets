package engine

import "testing"

func TestRoundSecondsToMinutes(t *testing.T) {
	cases := []struct {
		seconds int64
		want    int
	}{
		{0, 0},
		{1, 1},
		{29, 1},
		{30, 1},
		{31, 1},
		{59, 1},
		{60, 1},
		{89, 1},
		{90, 2},
		{3599, 60},
		{3600, 60},
		{3660, 61},
	}

	for _, tc := range cases {
		if got := roundSecondsToMinutes(tc.seconds); got != tc.want {
			t.Fatalf("roundSecondsToMinutes(%d) = %d, want %d", tc.seconds, got, tc.want)
		}
	}
}
