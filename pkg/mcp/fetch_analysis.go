package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// Markdown files live next to each entity's other published artifacts:
//
//	feed       -> web/{feed}.md          (sibling of web/{feed}.json)
//	country    -> web/countries/{CODE}.md (sibling of web/countries/{CODE}.json)
//	asn        -> web/asns/{ASN}.md      (sibling of web/asns/{ASN}.json)
//	maintainer -> web/maintainers/{slug}.md (no JSON sibling today)
var validEntityTypes = map[string]struct{}{
	"feed":       {},
	"country":    {},
	"asn":        {},
	"maintainer": {},
}

func markdownRelPath(entityType, name string) (string, bool) {
	switch entityType {
	case "feed":
		return name + ".md", true
	case "country":
		return filepath.Join("countries", name+".md"), true
	case "asn":
		return filepath.Join("asns", name+".md"), true
	case "maintainer":
		return filepath.Join("maintainers", name+".md"), true
	}
	return "", false
}

type FileMarkdownStore struct {
	webDir string
}

func NewFileMarkdownStore(webDir string) *FileMarkdownStore {
	return &FileMarkdownStore{webDir: webDir}
}

func (s *FileMarkdownStore) ReadMarkdown(entityType, name string) ([]byte, error) {
	if name == "" {
		return nil, fmt.Errorf("empty entity name")
	}
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return nil, fmt.Errorf("invalid entity name %q", name)
	}
	rel, ok := markdownRelPath(entityType, name)
	if !ok {
		return nil, fmt.Errorf("unknown entity type %q", entityType)
	}

	path := filepath.Join(s.webDir, rel)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s %q not found", entityType, name)
		}
		return nil, fmt.Errorf("read %s %q: %w", entityType, name, err)
	}
	return data, nil
}

func (s *Server) handleFetchAnalysis(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	entityType, err := req.RequireString("type")
	if err != nil {
		return mcpgo.NewToolResultError("missing required parameter \"type\""), nil
	}
	name, err := req.RequireString("name")
	if err != nil {
		return mcpgo.NewToolResultError("missing required parameter \"name\""), nil
	}

	entityType = strings.ToLower(entityType)
	if _, ok := validEntityTypes[entityType]; !ok {
		valid := make([]string, 0, len(validEntityTypes))
		for k := range validEntityTypes {
			valid = append(valid, k)
		}
		return mcpgo.NewToolResultError(fmt.Sprintf("invalid type %q; valid types: %s", entityType, strings.Join(valid, ", "))), nil
	}

	data, err := s.markdown.ReadMarkdown(entityType, name)
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}

	return mcpgo.NewToolResultText(string(data)), nil
}
