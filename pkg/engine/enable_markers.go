package engine

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
)

func (e *Engine) Enable(names []string, all bool) error {
	if e == nil {
		return nil
	}
	cfg, rt := e.configRuntimeSnapshot()
	return enableSourceMarkers(cfg, rt, names, all)
}

func (e *Engine) TryEnable(names []string, all bool) (bool, error) {
	if e == nil {
		return true, nil
	}
	cfg, rt, ok := e.TryConfigRuntimeSnapshot()
	if !ok {
		return false, nil
	}
	return true, enableSourceMarkers(cfg, rt, names, all)
}

func enableSourceMarkers(cfg *config.Config, rt Runtime, names []string, all bool) error {
	if all {
		// After config expansion, derivatives are regular sources.
		names = config.SortedSourceNames(cfg)
	}
	for _, name := range names {
		if name == "" {
			continue
		}
		path := sourceEnablePathForRuntime(rt, name)
		if err := os.MkdirAll(filepath.Dir(path), generatedDirMode); err != nil {
			return err
		}
		if err := touchFileAt(path, time.Unix(0, 0)); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) EnableArtifacts(names []string, all bool) error {
	if e == nil {
		return nil
	}
	cfg, rt := e.configRuntimeSnapshot()
	return enableArtifactMarkers(cfg, rt, names, all)
}

func (e *Engine) TryEnableArtifacts(names []string, all bool) (bool, error) {
	if e == nil {
		return true, nil
	}
	cfg, rt, ok := e.TryConfigRuntimeSnapshot()
	if !ok {
		return false, nil
	}
	return true, enableArtifactMarkers(cfg, rt, names, all)
}

func enableArtifactMarkers(cfg *config.Config, rt Runtime, names []string, all bool) error {
	if all {
		names = config.SortedArtifactNames(cfg)
	}
	for _, name := range names {
		if name == "" || cfg == nil || cfg.ArtifactByName(name) == nil {
			continue
		}
		path := artifactEnablePathForRuntime(rt, name)
		if err := os.MkdirAll(filepath.Dir(path), generatedDirMode); err != nil {
			return err
		}
		if err := touchFileAt(path, time.Unix(0, 0)); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) Disable(names []string, all bool) error {
	if e == nil {
		return nil
	}
	cfg, rt := e.configRuntimeSnapshot()
	return disableSourceMarkers(cfg, rt, names, all)
}

func (e *Engine) TryDisable(names []string, all bool) (bool, error) {
	if e == nil {
		return true, nil
	}
	cfg, rt, ok := e.TryConfigRuntimeSnapshot()
	if !ok {
		return false, nil
	}
	return true, disableSourceMarkers(cfg, rt, names, all)
}

func disableSourceMarkers(cfg *config.Config, rt Runtime, names []string, all bool) error {
	if all {
		// After config expansion, derivatives are regular sources.
		names = config.SortedSourceNames(cfg)
	}
	for _, name := range names {
		if name == "" {
			continue
		}
		if err := os.Remove(sourceEnablePathForRuntime(rt, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (e *Engine) DisableArtifacts(names []string, all bool) error {
	if e == nil {
		return nil
	}
	cfg, rt := e.configRuntimeSnapshot()
	return disableArtifactMarkers(cfg, rt, names, all)
}

func (e *Engine) TryDisableArtifacts(names []string, all bool) (bool, error) {
	if e == nil {
		return true, nil
	}
	cfg, rt, ok := e.TryConfigRuntimeSnapshot()
	if !ok {
		return false, nil
	}
	return true, disableArtifactMarkers(cfg, rt, names, all)
}

func disableArtifactMarkers(cfg *config.Config, rt Runtime, names []string, all bool) error {
	if all {
		names = config.SortedArtifactNames(cfg)
	}
	for _, name := range names {
		if name == "" || cfg == nil || cfg.ArtifactByName(name) == nil {
			continue
		}
		if err := os.Remove(artifactEnablePathForRuntime(rt, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}
