package config

import (
	"fmt"
	"sort"
	"strings"
)

type CategoryDefinition struct {
	Label       string `yaml:"label,omitempty"`
	Description string `yaml:"description,omitempty"`
	Color       string `yaml:"color,omitempty"`
	SortOrder   int    `yaml:"sort_order,omitempty"`
	Public      *bool  `yaml:"public,omitempty"`
}

type NamedCategory struct {
	Name string
	CategoryDefinition
}

func (c *Config) CategoryByName(name string) (CategoryDefinition, bool) {
	if c == nil || c.Categories == nil {
		return CategoryDefinition{}, false
	}
	def, ok := c.Categories[name]
	return def, ok
}

func (c *Config) CategoriesOrdered() []NamedCategory {
	return c.categoriesOrdered(false)
}

func (c *Config) PublicCategoriesOrdered() []NamedCategory {
	return c.categoriesOrdered(true)
}

func (c *Config) CategoryIsPublic(name string) bool {
	if strings.TrimSpace(name) == "" {
		return false
	}
	def, ok := c.CategoryByName(name)
	if !ok {
		return true
	}
	return def.IsPublic()
}

func (d CategoryDefinition) IsPublic() bool {
	return d.Public == nil || *d.Public
}

func (c *Config) categoriesOrdered(publicOnly bool) []NamedCategory {
	if c == nil || len(c.Categories) == 0 {
		return nil
	}
	out := make([]NamedCategory, 0, len(c.Categories))
	for name, def := range c.Categories {
		if publicOnly && !def.IsPublic() {
			continue
		}
		out = append(out, NamedCategory{Name: name, CategoryDefinition: def})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder < out[j].SortOrder
		}
		if out[i].Label != out[j].Label {
			return strings.ToLower(out[i].Label) < strings.ToLower(out[j].Label)
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func validateCategoryRegistry(cfg *Config) error {
	if cfg == nil {
		return nil
	}
	if len(cfg.Categories) == 0 {
		return nil
	}
	for name, def := range cfg.Categories {
		if !validFeedName(name) {
			return fmt.Errorf("category %q contains invalid characters", name)
		}
		if strings.TrimSpace(def.Label) == "" {
			return fmt.Errorf("category %q has empty label", name)
		}
		if strings.TrimSpace(def.Description) == "" {
			return fmt.Errorf("category %q has empty description", name)
		}
		if def.Color != "" && !validHexColor(def.Color) {
			return fmt.Errorf("category %q has invalid color %q", name, def.Color)
		}
	}
	for name, src := range cfg.Sources {
		if src == nil || src.Hidden {
			continue
		}
		if src.Category == "" {
			continue
		}
		if _, ok := cfg.Categories[src.Category]; !ok {
			return fmt.Errorf("source %q references unknown category %q", name, src.Category)
		}
	}
	for name := range cfg.Runtime.FeedHealthCategoryThresholds {
		if _, ok := cfg.Categories[name]; ok {
			continue
		}
		return fmt.Errorf("runtime.feed_health_category_thresholds references unknown category %q", name)
	}
	return nil
}

func validHexColor(v string) bool {
	if len(v) != 7 || v[0] != '#' {
		return false
	}
	for _, r := range v[1:] {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}
