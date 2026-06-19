package engine

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/firehol/update-ipsets/pkg/markdown"
	"github.com/firehol/update-ipsets/pkg/output"
)

const markdownTemplatesSubdir = "templates/markdown"

func (e *Engine) initMarkdownTemplates() {
	dir := filepath.Join(e.runtime.ConfigPath, markdownTemplatesSubdir)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		e.logger.Debug("markdown template directory not found", "dir", dir)
		return
	}
	store := markdown.NewTemplateStore(dir)
	if err := store.Load(); err != nil {
		e.logger.Error("failed to load markdown templates", "dir", dir, "error", err)
		return
	}
	e.markdownTemplates = store
}

func (e *Engine) writeMarkdownFilesForFeeds(ctx context.Context, feedNames []string, outDir string) ([]output.GeneratedFile, error) {
	ctx = nonNilContext(ctx)
	if e.markdownTemplates == nil {
		return nil, nil
	}
	if len(feedNames) == 0 {
		return nil, nil
	}

	reader := markdown.NewFeedArtifactReader(
		outDir,
		markdown.WithPreferredASNProvider(e.preferredASNProvider()),
		markdown.WithPreferredGEOProvider(e.preferredGeoProvider()),
	)
	var generated []output.GeneratedFile

	for _, name := range feedNames {
		if err := contextErr(ctx); err != nil {
			return nil, err
		}
		feedCtx, err := reader.BuildFeedContext(name)
		if err != nil {
			e.logger.Debug("markdown context build skipped", "feed", name, "error", err)
			continue
		}

		rel := e.publicFeedMarkdownRelPath(name)
		if err := e.markdownTemplates.WriteToDir("feed.md.tmpl", feedCtx, outDir, rel); err != nil {
			e.logger.Warn("markdown generation failed", "feed", name, "error", err)
			continue
		}

		generated = append(generated, output.GeneratedFile{
			Path:            filepath.Join(e.outputDir(), rel),
			Redistributable: true,
		})
		e.logger.Debug("markdown generated", "feed", name, "path", rel)
	}

	if len(generated) > 0 {
		e.logger.Info("markdown pages generated", "feeds", len(generated))
	}
	return generated, nil
}

func (e *Engine) publicFeedMarkdownRelPath(name string) string {
	return fmt.Sprintf("%s.md", name)
}

func (e *Engine) publicCountryMarkdownRelPath(code string) string {
	return fmt.Sprintf("countries/%s.md", code)
}

func (e *Engine) publicASNMarkdownRelPath(asn uint32) string {
	return fmt.Sprintf("asns/%d.md", asn)
}

func (e *Engine) publicMaintainerMarkdownRelPath(slug string) string {
	return fmt.Sprintf("maintainers/%s.md", slug)
}

func (e *Engine) ReloadMarkdownTemplates() {
	if e.markdownTemplates != nil {
		dir := e.markdownTemplates.Dir()
		if err := e.markdownTemplates.Load(); err != nil {
			slog.Error("failed to reload markdown templates", "dir", dir, "error", err)
		}
	} else {
		e.initMarkdownTemplates()
	}
}

func (e *Engine) writeMaintainerMarkdownFiles(stageDir string) ([]output.GeneratedFile, error) {
	if e.markdownTemplates == nil {
		return nil, nil
	}

	seen := map[string]struct{}{}
	for _, entry := range e.EntriesSnapshot() {
		src := e.lookupSource(entry.Name)
		if !homeSummaryEligible(e.cfg, src, nil) {
			continue
		}
		maintainerName := strings.TrimSpace(entry.Maintainer)
		if maintainerName == "" {
			continue
		}
		slug := maintainerSlugify(maintainerName)
		if _, ok := seen[slug]; ok {
			continue
		}
		seen[slug] = struct{}{}
	}

	var generated []output.GeneratedFile
	for slug := range seen {
		payload, err := e.MaintainerDetail(slug)
		if err != nil {
			e.logger.Debug("maintainer markdown skipped", "slug", slug, "error", err)
			continue
		}
		mdFile, _ := e.stageMaintainerMarkdown(slug, payload, stageDir)
		if mdFile.Path != "" {
			generated = append(generated, mdFile)
		}
	}

	if len(generated) > 0 {
		e.logger.Info("maintainer markdown pages generated", "count", len(generated))
	}
	return generated, nil
}
