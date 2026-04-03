package markdown_test

import (
	"testing"

	"github.com/firehol/update-ipsets/pkg/markdown"
)

func TestTopN(t *testing.T) {
	t.Parallel()

	t.Run("fewer entries than cap", func(t *testing.T) {
		t.Parallel()
		entries := []markdown.CappedEntry{
			{Name: "a", Value: 10},
			{Name: "b", Value: 5},
		}
		got := markdown.TopN(entries, 5)
		if got.Other != 0 {
			t.Fatalf("Other=%d; want 0", got.Other)
		}
		if len(got.Rows) != 2 {
			t.Fatalf("len(Rows)=%d; want 2", len(got.Rows))
		}
	})

	t.Run("more entries than cap", func(t *testing.T) {
		t.Parallel()
		entries := []markdown.CappedEntry{
			{Name: "a", Value: 100},
			{Name: "b", Value: 80},
			{Name: "c", Value: 60},
			{Name: "d", Value: 40},
			{Name: "e", Value: 20},
		}
		got := markdown.TopN(entries, 3)
		if len(got.Rows) != 3 {
			t.Fatalf("len(Rows)=%d; want 3", len(got.Rows))
		}
		if got.Rows[0].Name != "a" {
			t.Fatalf("Rows[0].Name=%q; want 'a'", got.Rows[0].Name)
		}
		if got.Other != 60 {
			t.Fatalf("Other=%d; want 60", got.Other)
		}
	})

	t.Run("equal values sorted by name", func(t *testing.T) {
		t.Parallel()
		entries := []markdown.CappedEntry{
			{Name: "c", Value: 50},
			{Name: "a", Value: 50},
			{Name: "b", Value: 50},
		}
		got := markdown.TopN(entries, 2)
		if got.Rows[0].Name != "a" {
			t.Fatalf("Rows[0].Name=%q; want 'a'", got.Rows[0].Name)
		}
		if got.Rows[1].Name != "b" {
			t.Fatalf("Rows[1].Name=%q; want 'b'", got.Rows[1].Name)
		}
		if got.Other != 50 {
			t.Fatalf("Other=%d; want 50", got.Other)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		t.Parallel()
		got := markdown.TopN(nil, 10)
		if len(got.Rows) != 0 {
			t.Fatalf("len(Rows)=%d; want 0", len(got.Rows))
		}
		if got.Other != 0 {
			t.Fatalf("Other=%d; want 0", got.Other)
		}
	})

	t.Run("exact cap size", func(t *testing.T) {
		t.Parallel()
		entries := []markdown.CappedEntry{
			{Name: "a", Value: 10},
			{Name: "b", Value: 5},
			{Name: "c", Value: 3},
		}
		got := markdown.TopN(entries, 3)
		if got.Other != 0 {
			t.Fatalf("Other=%d; want 0 when exact cap size", got.Other)
		}
		if len(got.Rows) != 3 {
			t.Fatalf("len(Rows)=%d; want 3", len(got.Rows))
		}
	})
}
