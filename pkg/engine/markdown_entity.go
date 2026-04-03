package engine

import (
	"path/filepath"

	"github.com/firehol/update-ipsets/pkg/markdown"
	"github.com/firehol/update-ipsets/pkg/output"
)

func (e *Engine) stageCountryMarkdown(code string, payload *CountryDetailPayload, stageDir string) (output.GeneratedFile, error) {
	if e.markdownTemplates == nil || payload == nil {
		return output.GeneratedFile{}, nil
	}

	ctx := buildCountryMarkdownContext(payload)
	rel := e.publicCountryMarkdownRelPath(code)
	if err := e.markdownTemplates.WriteToDir("country.md.tmpl", ctx, stageDir, rel); err != nil {
		e.logger.Debug("country markdown skipped", "code", code, "error", err)
		return output.GeneratedFile{}, nil
	}

	return output.GeneratedFile{
		Path:            filepath.Join(e.outputDir(), rel),
		Redistributable: true,
	}, nil
}

func (e *Engine) stageASNMarkdown(asn uint32, payload *ASNDetailPayload, stageDir string) (output.GeneratedFile, error) {
	if e.markdownTemplates == nil || payload == nil {
		return output.GeneratedFile{}, nil
	}

	ctx := buildASNMarkdownContext(payload)
	rel := e.publicASNMarkdownRelPath(asn)
	if err := e.markdownTemplates.WriteToDir("asn.md.tmpl", ctx, stageDir, rel); err != nil {
		e.logger.Debug("asn markdown skipped", "asn", asn, "error", err)
		return output.GeneratedFile{}, nil
	}

	return output.GeneratedFile{
		Path:            filepath.Join(e.outputDir(), rel),
		Redistributable: true,
	}, nil
}

func (e *Engine) stageMaintainerMarkdown(slug string, payload *MaintainerDetailPayload, stageDir string) (output.GeneratedFile, error) {
	if e.markdownTemplates == nil || payload == nil {
		return output.GeneratedFile{}, nil
	}

	ctx := buildMaintainerMarkdownContext(payload)
	rel := e.publicMaintainerMarkdownRelPath(slug)
	if err := e.markdownTemplates.WriteToDir("maintainer.md.tmpl", ctx, stageDir, rel); err != nil {
		e.logger.Debug("maintainer markdown skipped", "slug", slug, "error", err)
		return output.GeneratedFile{}, nil
	}

	return output.GeneratedFile{
		Path:            filepath.Join(e.outputDir(), rel),
		Redistributable: true,
	}, nil
}

func buildCountryMarkdownContext(p *CountryDetailPayload) *markdown.CountryPageContext {
	ctx := &markdown.CountryPageContext{
		Code:     p.Code,
		Provider: providerLabel(p.Provider),
		Totals: markdown.CountryTotals{
			Feeds:       p.Totals.FeedsMatching,
			IPs:         p.Totals.AttributedIPsInFeed,
			Categories:  p.Totals.Categories,
			Maintainers: p.Totals.Maintainers,
			ASNs:        p.Totals.ASNs,
		},
	}

	for _, c := range p.TopCategories {
		ctx.TopCategories = append(ctx.TopCategories, markdown.CategorySummary{
			Category: c.Category,
			Feeds:    c.FeedCount,
			IPs:      c.AttributedIPs,
		})
	}

	for _, m := range p.TopMaintainers {
		ctx.TopMaintainers = append(ctx.TopMaintainers, markdown.MaintainerSummary{
			Slug:  m.Slug,
			Name:  m.Name,
			URL:   m.URL,
			Feeds: m.FeedCount,
			IPs:   m.AttributedIPs,
		})
	}

	for _, a := range p.TopASNs {
		ctx.TopASNs = append(ctx.TopASNs, markdown.CountryASN{
			ASN:   a.ASN,
			Name:  a.Name,
			Feeds: a.FeedCount,
			IPs:   a.AttributedIPs,
		})
	}

	ctx.FeedsByCategory = make(map[string][]markdown.FeedInEntity)
	for cat, feeds := range p.FeedsByCategory {
		for _, f := range feeds {
			ctx.FeedsByCategory[cat] = append(ctx.FeedsByCategory[cat], markdown.FeedInEntity{
				Name:     f.Name,
				Category: f.Category,
				IPs:      f.AttributedIPs,
				Health:   f.HealthClass,
			})
		}
	}

	return ctx
}

func buildASNMarkdownContext(p *ASNDetailPayload) *markdown.ASNPageContext {
	ctx := &markdown.ASNPageContext{
		ASN:         p.ASN,
		Name:        p.Name,
		Description: p.Description,
		Provider:    providerLabel(p.Provider),
		Totals: markdown.ASNTotals{
			Feeds:       p.Totals.FeedsMatching,
			IPs:         p.Totals.AttributedIPs,
			Categories:  p.Totals.Categories,
			Maintainers: p.Totals.Maintainers,
			Countries:   p.Totals.Countries,
		},
	}

	for _, c := range p.TopCountries {
		ctx.TopCountries = append(ctx.TopCountries, markdown.ASNCountry{
			Code:  c.Code,
			Feeds: c.FeedCount,
			IPs:   c.AttributedIPs,
		})
	}

	for _, c := range p.TopCategories {
		ctx.TopCategories = append(ctx.TopCategories, markdown.CategorySummary{
			Category: c.Category,
			Feeds:    c.FeedCount,
			IPs:      c.AttributedIPs,
		})
	}

	for _, m := range p.TopMaintainers {
		ctx.TopMaintainers = append(ctx.TopMaintainers, markdown.MaintainerSummary{
			Slug:  m.Slug,
			Name:  m.Name,
			URL:   m.URL,
			Feeds: m.FeedCount,
			IPs:   m.AttributedIPs,
		})
	}

	ctx.FeedsByCategory = make(map[string][]markdown.FeedInEntity)
	for cat, feeds := range p.FeedsByCategory {
		for _, f := range feeds {
			ctx.FeedsByCategory[cat] = append(ctx.FeedsByCategory[cat], markdown.FeedInEntity{
				Name:     f.Name,
				Category: f.Category,
				IPs:      f.AttributedIPs,
				Health:   f.HealthClass,
			})
		}
	}

	return ctx
}

func providerLabel(p HomeSummaryProvider) string {
	if p.Label != "" {
		return p.Label
	}
	return p.Name
}

func buildMaintainerMarkdownContext(p *MaintainerDetailPayload) *markdown.MaintainerPageContext {
	ctx := &markdown.MaintainerPageContext{
		Slug: p.Slug,
		Name: p.Name,
		URL:  p.URL,
		Totals: markdown.MaintainerTotals{
			Feeds:      p.Totals.Feeds,
			IPs:        p.Totals.UniqueIPs,
			Categories: p.Totals.Categories,
		},
	}

	ctx.FeedsByCategory = make(map[string][]markdown.FeedInEntity)
	for cat, feeds := range p.FeedsByCategory {
		for _, f := range feeds {
			ctx.FeedsByCategory[cat] = append(ctx.FeedsByCategory[cat], markdown.FeedInEntity{
				Name:     f.Name,
				Category: f.Category,
				IPs:      f.UniqueIPs,
				Health:   f.HealthClass,
			})
		}
	}

	return ctx
}

