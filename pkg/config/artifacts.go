package config

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
)

const ArtifactScheme = "artifact"

const ArtifactTypeDroneBLBuildzone = "dronebl_buildzone"

// Artifact is a downloadable parent input that does not itself produce a public
// feed. Child feeds reference it via artifact:// URLs.
type Artifact struct {
	Name string `yaml:"-"`

	Type            string `yaml:"type"`
	Frequency       int    `yaml:"frequency"`
	MaxDownloadSize int64  `yaml:"max_download_size,omitempty"`
	Info            string `yaml:"info,omitempty"`
	Maintainer      string `yaml:"maintainer,omitempty"`
	MaintainerURL   string `yaml:"maintainer_url,omitempty"`

	// Provider-specific fields. Keep these on the shared struct until we have
	// more than one artifact type and a real need to split them.
	RSyncURL string `yaml:"rsync_url,omitempty"`
}

// ArtifactRef is the parsed form of an artifact:// source URL.
type ArtifactRef struct {
	Artifact string
	Parts    []string
}

func (c *Config) ArtifactByName(name string) *Artifact {
	if c == nil || c.Artifacts == nil {
		return nil
	}
	return c.Artifacts[name]
}

func (c *Config) ArtifactChildren(name string) []*Source {
	if c == nil || name == "" || len(c.Sources) == 0 {
		return nil
	}
	names := c.orderedSourceNames()
	out := make([]*Source, 0)
	for _, sourceName := range names {
		src := c.Sources[sourceName]
		if src != nil && src.ArtifactParent == name {
			out = append(out, src)
		}
	}
	return out
}

func SortedArtifactNames(cfg *Config) []string {
	if cfg == nil || len(cfg.Artifacts) == 0 {
		return nil
	}
	names := make([]string, 0, len(cfg.Artifacts))
	for name := range cfg.Artifacts {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func IsArtifactURL(rawURL string) bool {
	return strings.HasPrefix(strings.ToLower(rawURL), ArtifactScheme+"://")
}

func ParseArtifactURL(rawURL string) (ArtifactRef, error) {
	if !IsArtifactURL(rawURL) {
		return ArtifactRef{}, fmt.Errorf("not an artifact URL: %q", rawURL)
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ArtifactRef{}, fmt.Errorf("parse artifact URL: %w", err)
	}
	artifactName := strings.TrimSpace(u.Host)
	if artifactName == "" {
		artifactName = strings.Trim(strings.TrimSpace(u.Path), "/")
	}
	if artifactName == "" {
		return ArtifactRef{}, fmt.Errorf("artifact URL missing artifact name: %q", rawURL)
	}
	rawParts := strings.TrimSpace(u.Query().Get("parts"))
	if rawParts == "" {
		return ArtifactRef{}, fmt.Errorf("artifact URL missing parts= query: %q", rawURL)
	}
	parts := make([]string, 0)
	for _, part := range strings.Split(rawParts, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return ArtifactRef{}, fmt.Errorf("artifact URL parts list is empty: %q", rawURL)
	}
	return ArtifactRef{
		Artifact: artifactName,
		Parts:    parts,
	}, nil
}

func normalizeArtifactBackedSources(cfg *Config) error {
	if cfg == nil {
		return nil
	}
	for name, src := range cfg.Sources {
		if src == nil {
			continue
		}
		src.Name = name
		src.ArtifactParent = ""
		if !IsArtifactURL(src.URL) {
			continue
		}
		ref, err := ParseArtifactURL(src.URL)
		if err != nil {
			return fmt.Errorf("source %q: %w", name, err)
		}
		src.ArtifactParent = ref.Artifact
	}
	for name, artifact := range cfg.Artifacts {
		if artifact == nil {
			continue
		}
		artifact.Name = name
	}
	return nil
}
