package engine

import "github.com/firehol/update-ipsets/pkg/config"

type PublicCategory struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Color       string `json:"color,omitempty"`
	SortOrder   int    `json:"sort_order,omitempty"`
}

func (e *Engine) PublicCategories() []PublicCategory {
	if e == nil {
		return nil
	}
	return publicCategoriesForConfig(e.Config())
}

func publicCategoriesForConfig(cfg *config.Config) []PublicCategory {
	if cfg == nil {
		return nil
	}
	ordered := cfg.PublicCategoriesOrdered()
	out := make([]PublicCategory, 0, len(ordered))
	for _, category := range ordered {
		out = append(out, PublicCategory{
			Name:        category.Name,
			Label:       category.Label,
			Description: category.Description,
			Color:       category.Color,
			SortOrder:   category.SortOrder,
		})
	}
	return out
}

func publicProvenance(src *config.Source) config.Provenance {
	if src == nil || src.Provenance == "" {
		return config.ProvenancePrimary
	}
	return src.Provenance
}
