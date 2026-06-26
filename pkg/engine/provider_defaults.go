package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/firehol/update-ipsets/pkg/config"
)

type providerDefaultIdentity struct {
	Role          string
	Name          string
	Label         string
	Format        string
	Info          string
	License       string
	Attribution   string
	Maintainer    string
	MaintainerURL string
}

func providerDefaultsSetIDForConfig(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	items := []providerDefaultIdentity{
		providerDefaultIdentityForRole(cfg, config.UseASN),
		providerDefaultIdentityForRole(cfg, config.UseGeoIP),
	}
	if items[0].Name == "" && items[1].Name == "" {
		return ""
	}
	h := sha256.New()
	for _, item := range items {
		_, _ = fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00",
			item.Role, item.Name, item.Label, item.Format, item.Info, item.License,
			item.Attribution, item.Maintainer, item.MaintainerURL)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func providerDefaultIdentityForRole(cfg *config.Config, role string) providerDefaultIdentity {
	name := cfg.DefaultProviderForRole(role)
	if name == "" {
		for _, src := range cfg.SourcesWithUse(role) {
			if src != nil {
				name = src.Name
				break
			}
		}
	}
	src := cfg.SourceByName(name)
	if src == nil {
		return providerDefaultIdentity{Role: role, Name: name}
	}
	return providerDefaultIdentity{
		Role:          role,
		Name:          src.Name,
		Label:         src.Label,
		Format:        src.Format,
		Info:          src.Info,
		License:       src.License,
		Attribution:   src.Attribution,
		Maintainer:    src.Maintainer,
		MaintainerURL: src.MaintainerURL,
	}
}

func ProviderDefaultsSetMarkerPath(rt Runtime) string {
	if rt.LibDir == "" {
		return ""
	}
	return filepath.Join(rt.LibDir, "provider_defaults", "provider_set_id")
}

func ProviderDefaultsChangedForConfig(cfg *config.Config, rt Runtime) bool {
	current := providerDefaultsSetIDForConfig(cfg)
	path := ProviderDefaultsSetMarkerPath(rt)
	if path == "" {
		return false
	}
	return readProviderDefaultsMarker(rt) != current
}

func (e *Engine) ProviderDefaultsChanged() bool {
	if e == nil {
		return false
	}
	cfg, rt := e.configRuntimeSnapshot()
	return ProviderDefaultsChangedForConfig(cfg, rt)
}

func (e *Engine) providerDefaultsChanged() bool {
	return e.ProviderDefaultsChanged()
}

func (e *Engine) writeProviderDefaultsMarker() error {
	if e == nil {
		return nil
	}
	cfg, rt := e.configRuntimeSnapshot()
	return writeProviderDefaultsMarkerForConfigRuntime(cfg, rt)
}

func writeProviderDefaultsMarkerForConfigRuntime(cfg *config.Config, rt Runtime) error {
	path := ProviderDefaultsSetMarkerPath(rt)
	if path == "" {
		return nil
	}
	current := providerDefaultsSetIDForConfig(cfg)
	if current == "" {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return writeFileAtomic(path, []byte(current+"\n"), generatedFileMode)
}

func readProviderDefaultsMarker(rt Runtime) string {
	data, err := readFileInRoot(rt.LibDir, filepath.Join("provider_defaults", "provider_set_id"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
