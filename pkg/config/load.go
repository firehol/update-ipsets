package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/internal/observability"

	"go.opentelemetry.io/otel/attribute"
	yaml "go.yaml.in/yaml/v3"
)

func LoadYAML(path string) (*Config, error) {
	started := time.Now()
	result := "ok"
	defer func() {
		observeConfigLoad(result, time.Since(started))
	}()
	cfg, err := loadYAMLDocument(path)
	if err != nil {
		result = "error"
		return nil, err
	}
	finalized, err := finalizeLoadedConfig(cfg)
	if err != nil {
		result = "error"
		return nil, err
	}
	return finalized, nil
}

func loadYAMLDocument(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Detect the deprecated top-level blocks before yaml.Unmarshal so
	// operators get a clear migration error instead of silently losing
	// their geolocation/asn/bogon configuration.
	if err := rejectLegacyTopLevelBlocks(data); err != nil {
		return nil, err
	}
	cfg := New()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func Load(path string) (*Config, error) {
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return LoadDirectory(path)
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return LoadYAML(path)
	case ".conf", "":
		return LoadLegacy(path)
	default:
		return nil, fmt.Errorf("unsupported config format %q", path)
	}
}

func LoadDirectory(dir string) (*Config, error) {
	started := time.Now()
	result := "ok"
	defer func() {
		observeConfigLoad(result, time.Since(started))
	}()
	cfg := New()
	if dir == "" {
		return cfg, nil
	}
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		result = "error"
		return nil, err
	}
	if !info.IsDir() {
		result = "error"
		return nil, fmt.Errorf("%s is not a directory", dir)
	}

	paths := make([]string, 0)
	if err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "." {
				return nil
			}
			if strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") {
			return nil
		}
		switch strings.ToLower(filepath.Ext(entry.Name())) {
		case ".yaml", ".yml", ".conf":
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		result = "error"
		return nil, err
	}
	slices.Sort(paths)

	for _, path := range paths {
		extra, err := loadConfigFragment(path)
		if err != nil {
			result = "error"
			return nil, err
		}
		cfg.Merge(extra)
	}
	finalized, err := finalizeLoadedConfig(cfg)
	if err != nil {
		result = "error"
		return nil, err
	}
	return finalized, nil
}

func loadConfigFragment(path string) (*Config, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return loadYAMLDocument(path)
	case ".conf":
		return ExtractLegacyScript(path, ExtractOptions{})
	default:
		return nil, fmt.Errorf("unsupported supplementary config format %q", path)
	}
}

func observeConfigLoad(result string, dur time.Duration) {
	if result == "" {
		result = "unknown"
	}
	attr := attribute.String("config.result", result)
	ctx := observability.BackgroundContext()
	observability.Count(ctx, "config.loads", 1, attr)
	observability.Duration(ctx, "config.load", dur, attr)
}
