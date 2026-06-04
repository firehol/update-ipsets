package markdown_test

import (
	"strings"
	"testing"
	"time"

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

func TestTemplateCommaFunction(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   any
		want string
	}{
		{name: "zero uint64", in: uint64(0), want: "0"},
		{name: "small uint64", in: uint64(999), want: "999"},
		{name: "grouped uint64", in: uint64(1234567), want: "1,234,567"},
		{name: "large uint64", in: uint64(1000000000), want: "1,000,000,000"},
		{name: "int", in: 1234567, want: "1,234,567"},
		{name: "int64", in: int64(-1234567), want: "-1,234,567"},
		{name: "float64", in: 1234.9, want: "1,234"},
		{name: "string fallback", in: "sample", want: "sample"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := markdown.ExecuteInline("{{comma .}}", tc.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("comma(%v)=%q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestTemplatePctAndDateFunctions(t *testing.T) {
	t.Parallel()

	got, err := markdown.ExecuteInline("{{pct .}}", 45.67)
	if err != nil {
		t.Fatalf("pct: %v", err)
	}
	if got != "45.7%" {
		t.Fatalf("pct(45.67)=%q; want '45.7%%'", got)
	}

	got, err = markdown.ExecuteInline("{{date .}}", int64(1714646400000))
	if err != nil {
		t.Fatalf("date: %v", err)
	}
	if !strings.Contains(got, "2024") {
		t.Fatalf("date=%q; want contains 2024", got)
	}

	got, err = markdown.ExecuteInline("{{date .}}", int64(0))
	if err != nil {
		t.Fatalf("date zero: %v", err)
	}
	if got != "" {
		t.Fatalf("date(0)=%q; want empty", got)
	}
}

func TestTemplateMinsFunction(t *testing.T) {
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
			t.Fatalf("mins(%d)=%q; want %q", tc.mins, got, tc.want)
		}
	}
}

func TestTemplateRelTimeFunction(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name string
		in   int64
		want string
	}{
		{name: "zero", in: 0, want: ""},
		{name: "current", in: now.UnixMilli(), want: "just now"},
		{name: "minutes", in: now.Add(-5 * time.Minute).UnixMilli(), want: "5m ago"},
		{name: "hours", in: now.Add(-2 * time.Hour).UnixMilli(), want: "2h ago"},
		{name: "days", in: now.Add(-3 * 24 * time.Hour).UnixMilli(), want: "3d ago"},
		{name: "months", in: now.Add(-60 * 24 * time.Hour).UnixMilli(), want: "2mo ago"},
		{name: "years", in: now.Add(-400 * 24 * time.Hour).UnixMilli(), want: "1y ago"},
	}
	for _, tc := range cases {
		got, err := markdown.ExecuteInline("{{relTime .}}", tc.in)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != tc.want {
			t.Fatalf("%s: relTime(%d)=%q; want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestTemplateBarFunction(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		pct   float64
		width int
		want  string
	}{
		{name: "fills width", pct: 40, width: 5, want: "██░░░"},
		{name: "negative clamps empty", pct: -20, width: 5, want: "░░░░░"},
		{name: "over hundred clamps full", pct: 120, width: 5, want: "█████"},
		{name: "default width", pct: 20, width: 0, want: "██░░░░░░░░"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := markdown.ExecuteInline("{{bar .Pct .Width}}", map[string]any{
				"Pct":   tc.pct,
				"Width": tc.width,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("bar(%v,%d)=%q; want %q", tc.pct, tc.width, got, tc.want)
			}
		})
	}
}

func TestTemplateTruncateFunction(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		text string
		max  int
		want string
	}{
		{name: "short unchanged", text: "sample", max: 10, want: "sample"},
		{name: "exact unchanged", text: "sample", max: 6, want: "sample"},
		{name: "with ellipsis", text: "sample text", max: 9, want: "sample..."},
		{name: "tiny max", text: "sample", max: 3, want: "sam"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := markdown.ExecuteInline("{{truncate .Text .Max}}", map[string]any{
				"Text": tc.text,
				"Max":  tc.max,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("truncate(%q,%d)=%q; want %q", tc.text, tc.max, got, tc.want)
			}
		})
	}
}

func TestTemplateStatusHelpers(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		status    string
		wantLabel string
		wantLead  string
	}{
		{
			name:      "archived",
			status:    "archived",
			wantLabel: "Archived",
			wantLead:  "Our health automation has archived this feed:",
		},
		{
			name:      "unmaintained",
			status:    "unmaintained",
			wantLabel: "Unmaintained",
			wantLead:  "Our health automation has flagged this feed as unmaintained:",
		},
		{
			name:      "empty",
			status:    "empty",
			wantLabel: "Empty",
			wantLead:  "This feed currently contains no entries:",
		},
		{
			name:      "discontinued",
			status:    "discontinued",
			wantLabel: "Discontinued",
			wantLead:  "The official status of this feed is discontinued:",
		},
		{
			name:      "merged",
			status:    "merged",
			wantLabel: "Merged",
			wantLead:  "The official status of this feed is merged:",
		},
		{
			name:      "forked",
			status:    "forked",
			wantLabel: "Forked",
			wantLead:  "The official status of this feed has been forked:",
		},
		{
			name:      "reformatted",
			status:    "reformatted",
			wantLabel: "Reformatted",
			wantLead:  "The official status of this feed is reformatted:",
		},
		{
			name:      "altered scope",
			status:    "altered_scope",
			wantLabel: "Altered scope",
			wantLead:  "The official status of this feed has been altered:",
		},
		{
			name:      "unknown",
			status:    "unknown",
			wantLabel: "Unknown",
			wantLead:  "The official status of this feed is unknown:",
		},
		{
			name:      "custom",
			status:    "custom_state",
			wantLabel: "custom state",
			wantLead:  "The status of this feed is custom state:",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			label, err := markdown.ExecuteInline("{{statusLabel .}}", tc.status)
			if err != nil {
				t.Fatalf("statusLabel: %v", err)
			}
			if label != tc.wantLabel {
				t.Fatalf("statusLabel(%q)=%q; want %q", tc.status, label, tc.wantLabel)
			}

			lead, err := markdown.ExecuteInline("{{statusLead .}}", tc.status)
			if err != nil {
				t.Fatalf("statusLead: %v", err)
			}
			if lead != tc.wantLead {
				t.Fatalf("statusLead(%q)=%q; want %q", tc.status, lead, tc.wantLead)
			}
		})
	}
}

func TestExecuteInlineReturnsParseErrors(t *testing.T) {
	t.Parallel()

	if _, err := markdown.ExecuteInline("{{", nil); err == nil {
		t.Fatal("ExecuteInline returned nil error for malformed template")
	}
}
