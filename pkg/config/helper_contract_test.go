package config

import (
	"reflect"
	"testing"
)

func TestCategoryRegistryHelpers(t *testing.T) {
	private := false
	public := true
	cfg := New()
	cfg.Categories = map[string]CategoryDefinition{
		"hidden": {
			Label:       "Hidden",
			Description: "private category",
			SortOrder:   1,
			Public:      &private,
		},
		"alpha_a": {
			Label:       "Alpha",
			Description: "first alpha category",
			SortOrder:   1,
			Public:      &public,
		},
		"alpha_b": {
			Label:       "Alpha",
			Description: "second alpha category",
			SortOrder:   1,
		},
		"beta": {
			Label:       "Beta",
			Description: "beta category",
			SortOrder:   2,
		},
	}

	def, ok := cfg.CategoryByName("hidden")
	if !ok || def.Label != "Hidden" {
		t.Fatalf("CategoryByName(hidden) = %+v, %v", def, ok)
	}
	if _, ok := cfg.CategoryByName("missing"); ok {
		t.Fatal("CategoryByName(missing) ok = true, want false")
	}
	if (&Config{}).CategoryIsPublic("") {
		t.Fatal("blank category should not be public")
	}
	if !cfg.CategoryIsPublic("unknown") {
		t.Fatal("unknown categories should default to public")
	}
	if cfg.CategoryIsPublic("hidden") {
		t.Fatal("explicit private category should not be public")
	}
	if !cfg.CategoryIsPublic("alpha_b") {
		t.Fatal("category with nil Public should be public")
	}

	if got, want := categoryNames(cfg.CategoriesOrdered()), []string{"alpha_a", "alpha_b", "hidden", "beta"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("CategoriesOrdered names = %v, want %v", got, want)
	}
	if got, want := categoryNames(cfg.PublicCategoriesOrdered()), []string{"alpha_a", "alpha_b", "beta"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("PublicCategoriesOrdered names = %v, want %v", got, want)
	}
}

func TestCategoryRegistryNilConfig(t *testing.T) {
	var cfg *Config
	if _, ok := cfg.CategoryByName("anything"); ok {
		t.Fatal("nil CategoryByName ok = true, want false")
	}
	if cfg.CategoriesOrdered() != nil {
		t.Fatal("nil CategoriesOrdered should return nil")
	}
	if cfg.PublicCategoriesOrdered() != nil {
		t.Fatal("nil PublicCategoriesOrdered should return nil")
	}
	if !cfg.CategoryIsPublic("anything") {
		t.Fatal("nil config should treat unknown named category as public")
	}
	if (CategoryDefinition{}).IsPublic() != true {
		t.Fatal("zero CategoryDefinition should default to public")
	}
}

func TestArtifactHelpers(t *testing.T) {
	cfg := New()
	cfg.Artifacts = map[string]*Artifact{
		"zeta":  {Name: "zeta"},
		"alpha": {Name: "alpha"},
		"mid":   {Name: "mid"},
	}
	cfg.Sources = map[string]*Source{
		"first":  {Name: "first", ArtifactParent: "alpha"},
		"second": {Name: "second", ArtifactParent: "alpha"},
		"other":  {Name: "other", ArtifactParent: "zeta"},
		"plain":  {Name: "plain"},
	}
	cfg.SourceOrder = []string{"second", "plain", "first", "other"}

	if got, want := SortedArtifactNames(cfg), []string{"alpha", "mid", "zeta"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("SortedArtifactNames = %v, want %v", got, want)
	}
	if SortedArtifactNames(nil) != nil {
		t.Fatal("SortedArtifactNames(nil) should return nil")
	}
	children := cfg.ArtifactChildren("alpha")
	if got, want := sourceNames(children), []string{"second", "first"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ArtifactChildren(alpha) = %v, want %v", got, want)
	}
	if got := cfg.ArtifactChildren("missing"); len(got) != 0 {
		t.Fatalf("ArtifactChildren(missing) len = %d, want 0", len(got))
	}
}

func TestParseRetentionWindowURL(t *testing.T) {
	parent, minutes, err := ParseRetentionWindowURL(InternalRetentionWindowScheme + "?parent=base&minutes=1440")
	if err != nil {
		t.Fatalf("ParseRetentionWindowURL(valid): %v", err)
	}
	if parent != "base" || minutes != 1440 {
		t.Fatalf("ParseRetentionWindowURL(valid) = %q, %d", parent, minutes)
	}

	tests := []string{
		"internal://other?parent=base&minutes=1440",
		InternalRetentionWindowScheme + "?minutes=1440",
		InternalRetentionWindowScheme + "?parent=base",
		InternalRetentionWindowScheme + "?parent=base&minutes=not-an-int",
		InternalRetentionWindowScheme + "?parent=base&minutes=0",
	}
	for _, rawURL := range tests {
		t.Run(rawURL, func(t *testing.T) {
			if _, _, err := ParseRetentionWindowURL(rawURL); err == nil {
				t.Fatal("ParseRetentionWindowURL error = nil, want error")
			}
		})
	}
}

func TestParseMergeURLCompatibility(t *testing.T) {
	inputs, err := ParseMergeURL(InternalMergeScheme + "?inputs=alpha,,beta&exclude=private")
	if err != nil {
		t.Fatalf("ParseMergeURL(valid): %v", err)
	}
	if got, want := inputs, []string{"alpha", "beta"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseMergeURL inputs = %v, want %v", got, want)
	}
	if _, err := ParseMergeURL("internal://other?inputs=alpha"); err == nil {
		t.Fatal("ParseMergeURL(wrong scheme) error = nil, want error")
	}
	if _, err := ParseMergeURL(InternalMergeScheme + "?exclude=private"); err == nil {
		t.Fatal("ParseMergeURL(missing inputs) error = nil, want error")
	}
}

func categoryNames(categories []NamedCategory) []string {
	names := make([]string, 0, len(categories))
	for _, category := range categories {
		names = append(names, category.Name)
	}
	return names
}

func sourceNames(sources []*Source) []string {
	names := make([]string, 0, len(sources))
	for _, source := range sources {
		names = append(names, source.Name)
	}
	return names
}
