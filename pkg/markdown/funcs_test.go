package markdown_test

import (
	"strings"
	"testing"

	"github.com/firehol/update-ipsets/pkg/markdown"
)

func TestRenderTable(t *testing.T) {
	t.Parallel()

	t.Run("basic table", func(t *testing.T) {
		t.Parallel()

		cols := []markdown.TableColumn{
			{Header: "Name"},
			{Header: "Count", Right: true},
		}
		rows := [][]string{
			{"foo", "100"},
			{"bar", "2,000"},
		}

		got := markdown.RenderTable(cols, rows)
		if !strings.Contains(got, "| Name") {
			t.Fatalf("missing header: %q", got)
		}
		if !strings.Contains(got, "foo") {
			t.Fatalf("missing row: %q", got)
		}
		if !strings.Contains(got, "2,000") {
			t.Fatalf("missing value: %q", got)
		}
	})

	t.Run("empty columns returns empty", func(t *testing.T) {
		t.Parallel()
		got := markdown.RenderTable(nil, nil)
		if got != "" {
			t.Fatalf("expected empty, got %q", got)
		}
	})

	t.Run("rows shorter than columns padded", func(t *testing.T) {
		t.Parallel()

		cols := []markdown.TableColumn{
			{Header: "A"},
			{Header: "B"},
		}
		rows := [][]string{
			{"only-one"},
		}

		got := markdown.RenderTable(cols, rows)
		lines := strings.Split(strings.TrimSpace(got), "\n")
		if len(lines) < 3 {
			t.Fatalf("expected >=3 lines, got %d", len(lines))
		}
	})
}

func TestFuncs(t *testing.T) {
	t.Parallel()

	t.Run("commaUint", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			in   uint64
			want string
		}{
			{0, "0"},
			{999, "999"},
			{1000, "1,000"},
			{1234567, "1,234,567"},
			{1000000000, "1,000,000,000"},
		}
		for _, tc := range cases {
			got, err := markdown.ExecuteInline("{{comma .}}", tc.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("comma(%d)=%q; want %q", tc.in, got, tc.want)
			}
		}
	})

	t.Run("pct", func(t *testing.T) {
		t.Parallel()
		got, err := markdown.ExecuteInline("{{pct .}}", 45.67)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "45.7%" {
			t.Fatalf("pct(45.67)=%q; want '45.7%%'", got)
		}
	})

	t.Run("date formats unix ms", func(t *testing.T) {
		t.Parallel()
		got, err := markdown.ExecuteInline("{{date .}}", int64(1714646400000))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(got, "2024") {
			t.Fatalf("date=%q; want contains 2024", got)
		}
	})

	t.Run("date zero returns empty", func(t *testing.T) {
		t.Parallel()
		got, err := markdown.ExecuteInline("{{date .}}", int64(0))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Fatalf("date(0)=%q; want empty", got)
		}
	})

	t.Run("minsToDuration", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			mins int
			want string
		}{
			{30, "30m"},
			{60, "1h"},
			{90, "1.5h"},
			{1440, "1d"},
			{0, ""},
		}
		for _, tc := range cases {
			got, err := markdown.ExecuteInline("{{mins .}}", tc.mins)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("mins(%d)=%q; want %q", tc.mins, got, tc.want)
			}
		}
	})
}
