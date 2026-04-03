package markdown

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"

	"log/slog"
)

var builtins = map[string]any{
	"comma":             func(v any) string { return commaAny(v) },
	"commaI":            commaInt,
	"pct":               pct,
	"date":              dateStr,
	"relTime":           relTime,
	"bar":               barFill,
	"mins":              minsToDuration,
	"truncate":          truncate,
	"table":             RenderTable,
	"countryName":       countryName,
	"asnDisplayName":    asnDisplayName,
	"sortedKeys":        sortedKeys,
	"statusLabel":       statusLabel,
	"statusLead":        statusLead,
}

// statusLabel maps a status value (research lifecycle state or health
// class) to its display label. Kept here so the template, the UI, and
// the markdown stay in lockstep.
func statusLabel(value string) string {
	switch value {
	case "archived":
		return "Archived"
	case "unmaintained":
		return "Unmaintained"
	case "empty":
		return "Empty"
	case "discontinued":
		return "Discontinued"
	case "merged":
		return "Merged"
	case "forked":
		return "Forked"
	case "reformatted":
		return "Reformatted"
	case "altered_scope":
		return "Altered scope"
	case "unknown":
		return "Unknown"
	default:
		return strings.ReplaceAll(value, "_", " ")
	}
}

// statusLead supplies the sentence prefix that precedes the dynamic
// description for each status value. Mirrors the wording used by the
// UI's SectionStatus so the markdown and the website read identically.
func statusLead(value string) string {
	switch value {
	case "archived":
		return "Our health automation has archived this feed:"
	case "unmaintained":
		return "Our health automation has flagged this feed as unmaintained:"
	case "empty":
		return "This feed currently contains no entries:"
	case "discontinued":
		return "The official status of this feed is discontinued:"
	case "merged":
		return "The official status of this feed is merged:"
	case "forked":
		return "The official status of this feed has been forked:"
	case "reformatted":
		return "The official status of this feed is reformatted:"
	case "altered_scope":
		return "The official status of this feed has been altered:"
	case "unknown":
		return "The official status of this feed is unknown:"
	default:
		return "The status of this feed is " + strings.ReplaceAll(value, "_", " ") + ":"
	}
}

type TemplateStore struct {
	mu   sync.RWMutex
	tpls map[string]*template.Template
	dir  string
}

func NewTemplateStore(dir string) *TemplateStore {
	return &TemplateStore{
		dir:  dir,
		tpls: make(map[string]*template.Template),
	}
}

func (s *TemplateStore) Dir() string { return s.dir }

func (s *TemplateStore) Load() error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Warn("markdown template directory not found, markdown generation disabled", "dir", s.dir)
			return nil
		}
		return fmt.Errorf("read template dir %s: %w", s.dir, err)
	}

	loaded := make(map[string]*template.Template)
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".tmpl" {
			continue
		}
		name := e.Name()
		path := filepath.Join(s.dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read template %s: %w", name, err)
		}

		t, err := template.New(name).Funcs(builtins).Parse(string(data))
		if err != nil {
			return fmt.Errorf("parse template %s: %w", name, err)
		}
		loaded[name] = t
		slog.Debug("markdown template loaded", "name", name)
	}

	s.mu.Lock()
	s.tpls = loaded
	s.mu.Unlock()
	slog.Info("markdown templates loaded", "count", len(loaded), "dir", s.dir)
	return nil
}

func (s *TemplateStore) Execute(name string, data any) (string, error) {
	s.mu.RLock()
	t, ok := s.tpls[name]
	s.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("template %q not found", name)
	}

	var buf strings.Builder
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template %s: %w", name, err)
	}
	return buf.String(), nil
}

func (s *TemplateStore) WriteToDir(name string, data any, dir, relPath string) error {
	content, err := s.Execute(name, data)
	if err != nil {
		return err
	}

	if strings.Contains(relPath, "..") || filepath.IsAbs(relPath) {
		return fmt.Errorf("invalid markdown relPath %q: path traversal", relPath)
	}

	fullPath := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return fmt.Errorf("mkdir for %s: %w", relPath, err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", relPath, err)
	}
	return nil
}

func ExecuteInline(tmpl string, data any) (string, error) {
	t, err := template.New("inline").Funcs(builtins).Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
