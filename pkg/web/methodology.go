package web

import (
	"io/fs"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	mdhtml "github.com/yuin/goldmark/renderer/html"
)

// methodologyPage is a single rendered methodology document. The Title is
// extracted from the first level-1 heading; the Summary is the first
// paragraph; the Body is the rendered HTML of the entire document.
type methodologyPage struct {
	Slug    string
	Title   string
	Summary string
	Body    string // rendered HTML, safe to write directly
}

type methodologyIndexItem struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Summary string `json:"summary,omitempty"`
}

type methodologyIndexPayload struct {
	Items []methodologyIndexItem `json:"items"`
}

type methodologyPagePayload struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Summary string `json:"summary,omitempty"`
	Body    string `json:"body"`
}

var loadMethodologyPages = sync.OnceValues(func() (map[string]*methodologyPage, []*methodologyPage) {
	pages := map[string]*methodologyPage{}
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Footnote),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(mdhtml.WithUnsafe()),
	)
	entries, err := fs.ReadDir(embeddedStatic, "static/methodology")
	if err != nil {
		return pages, nil
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".md") {
			continue
		}
		data, err := fs.ReadFile(embeddedStatic, "static/methodology/"+name)
		if err != nil {
			continue
		}
		slug := strings.TrimSuffix(name, ".md")
		title, summary := extractTitleAndSummary(data)
		var buf strings.Builder
		if err := md.Convert(data, &buf); err != nil {
			continue
		}
		pages[slug] = &methodologyPage{
			Slug:    slug,
			Title:   title,
			Summary: summary,
			Body:    buf.String(),
		}
	}
	ordered := make([]*methodologyPage, 0, len(pages))
	for _, page := range pages {
		ordered = append(ordered, page)
	}
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Title < ordered[j].Title
	})
	return pages, ordered
})

// extractTitleAndSummary scans the raw Markdown for the first level-1 ATX
// heading (used as the title) and the first non-empty paragraph after it
// (used as the summary). Both are returned as plain text.
func extractTitleAndSummary(data []byte) (title, summary string) {
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") && title == "" {
			title = strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
			// Look for the first paragraph that follows.
			for j := i + 1; j < len(lines); j++ {
				paragraph := strings.TrimSpace(lines[j])
				if paragraph == "" || strings.HasPrefix(paragraph, "#") {
					continue
				}
				summary = paragraph
				return
			}
		}
	}
	return
}

// handleMethodologyIndex serves the machine-readable methodology index.
func handleMethodologyIndex(w http.ResponseWriter, _ *http.Request) {
	_, ordered := loadMethodologyPages()
	items := make([]methodologyIndexItem, 0, len(ordered))
	for _, page := range ordered {
		items = append(items, methodologyIndexItem{
			Slug:    page.Slug,
			Title:   page.Title,
			Summary: page.Summary,
		})
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	writeJSON(w, http.StatusOK, methodologyIndexPayload{Items: items})
}

// handleMethodologyPage serves one machine-readable methodology page by slug.
func handleMethodologyPage(w http.ResponseWriter, r *http.Request) {
	pages, _ := loadMethodologyPages()
	slug := strings.TrimPrefix(r.URL.Path, "/api/v1/methodology/")
	slug = strings.TrimPrefix(slug, "/methodology/")
	slug = strings.TrimSuffix(slug, "/")
	if slug == "" {
		handleMethodologyIndex(w, r)
		return
	}
	page, ok := pages[slug]
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	writeJSON(w, http.StatusOK, methodologyPagePayload{
		Slug:    page.Slug,
		Title:   page.Title,
		Summary: page.Summary,
		Body:    page.Body,
	})
}
