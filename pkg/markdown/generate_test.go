package markdown_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/firehol/update-ipsets/pkg/markdown"
)

func TestRenderTableOutput(t *testing.T) {
	t.Parallel()

	cols := []markdown.TableColumn{
		{Header: "Name"},
		{Header: "IPs", Right: true},
		{Header: "Pct", Right: true},
	}
	rows := [][]string{
		{"AS13335 Cloudflare", "12,345", "45.2%"},
		{"AS15169 Google", "8,901", "32.6%"},
		{"Other", "6,112", "22.2%"},
	}

	got := markdown.RenderTable(cols, rows)
	t.Log("rendered table:\n" + got)

	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines (header + separator + 3 rows), got %d", len(lines))
	}

	if !strings.HasPrefix(lines[0], "| Name") {
		t.Fatalf("header line should start with '| Name', got %q", lines[0])
	}

	if !strings.HasPrefix(lines[1], "|:") {
		t.Fatalf("separator should start with '|:', got %q", lines[1])
	}
	if !strings.Contains(lines[1], ":") {
		t.Fatalf("separator should contain alignment markers, got %q", lines[1])
	}

	for _, row := range lines[2:] {
		if !strings.HasPrefix(row, "| ") {
			t.Fatalf("row should start with '| ', got %q", row)
		}
	}
}

func TestTemplateStoreLoadMissingDir(t *testing.T) {
	t.Parallel()

	s := markdown.NewTemplateStore(t.TempDir() + "/nonexistent")
	if err := s.Load(); err != nil {
		t.Fatalf("Load on missing dir should not error: %v", err)
	}

	_, err := s.Execute("missing.tmpl", nil)
	if err == nil {
		t.Fatal("Execute on missing template should error")
	}
}

func TestTemplateStoreWithTemplates(t *testing.T) {
	dir := t.TempDir()

	templateContent := "# {{.Title}}\n\n{{comma .Count}} items.\n"
	if err := os.WriteFile(filepath.Join(dir, "test.tmpl"), []byte(templateContent), 0o600); err != nil {
		t.Fatal(err)
	}

	s := markdown.NewTemplateStore(dir)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	data := struct {
		Title string
		Count uint64
	}{Title: "Test Feed", Count: 1234567}

	got, err := s.Execute("test.tmpl", data)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(got, "1,234,567") {
		t.Fatalf("expected comma-formatted number in output, got %q", got)
	}
	if !strings.Contains(got, "Test Feed") {
		t.Fatalf("expected title in output, got %q", got)
	}
}

func TestWriteToDirRejectsTraversal(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "test.tmpl"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}

	s := markdown.NewTemplateStore(dir)
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}

	out := t.TempDir()

	traversalPaths := []string{
		"../etc/passwd",
		"../../secret",
		"foo/../../../bar",
	}
	for _, p := range traversalPaths {
		err := s.WriteToDir("test.tmpl", nil, out, p)
		if err == nil {
			t.Errorf("WriteToDir(%q) should reject path traversal", p)
		}
	}

	absPath := "/etc/passwd"
	err := s.WriteToDir("test.tmpl", nil, out, absPath)
	if err == nil {
		t.Errorf("WriteToDir(%q) should reject absolute path", absPath)
	}
}
