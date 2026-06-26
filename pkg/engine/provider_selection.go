package engine

import "github.com/firehol/update-ipsets/pkg/config"

// preferredGeoProvider returns the configured geolocation provider used for
// canonical summaries, insights, IP context, and entity pages. It falls back to
// catalog order for programmatic/test configs that do not set defaults.
func (e *Engine) preferredGeoProvider() string {
	if e == nil {
		return ""
	}
	return preferredGeoProviderForConfig(e.Config())
}

func preferredGeoProviderForConfig(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	if provider := cfg.DefaultProviderForRole(config.UseGeoIP); provider != "" {
		return provider
	}
	for _, src := range cfg.SourcesWithUse(config.UseGeoIP) {
		return src.Name
	}
	return ""
}

// preferredASNProvider returns the configured ASN provider used for canonical
// summaries, insights, IP context, and entity pages. It falls back to catalog
// order for programmatic/test configs that do not set defaults.
func (e *Engine) preferredASNProvider() string {
	if e == nil {
		return ""
	}
	return preferredASNProviderForConfig(e.Config())
}

func preferredASNProviderForConfig(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	if provider := cfg.DefaultProviderForRole(config.UseASN); provider != "" {
		return provider
	}
	for _, src := range cfg.SourcesWithUse(config.UseASN) {
		return src.Name
	}
	return ""
}
