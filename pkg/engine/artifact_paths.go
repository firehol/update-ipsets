package engine

import (
	"path/filepath"
)

func sourcePathForRuntime(rt Runtime, name string) string {
	return filepath.Join(rt.BaseDir, name+".source")
}

func sourceEnablePathForRuntime(rt Runtime, name string) string {
	return filepath.Join(rt.BaseDir, name+".enabled")
}

func artifactRootDirForRuntime(rt Runtime, name string) string {
	return filepath.Join(rt.LibDir, "artifacts", name)
}

func artifactEnablePathForRuntime(rt Runtime, name string) string {
	return filepath.Join(artifactRootDirForRuntime(rt, name), "enabled")
}

func artifactSourcePathForRuntime(rt Runtime, name string) string {
	return filepath.Join(artifactRootDirForRuntime(rt, name), "source")
}

func artifactExtractDirForRuntime(rt Runtime, name string) string {
	return filepath.Join(artifactRootDirForRuntime(rt, name), "extract")
}

func (e *Engine) isArtifact(name string) bool {
	cfg := e.Config()
	return cfg != nil && cfg.ArtifactByName(name) != nil
}

func (e *Engine) artifactRootDir(name string) string {
	return artifactRootDirForRuntime(e.Runtime(), name)
}

func (e *Engine) sourceEnablePath(name string) string {
	return sourceEnablePathForRuntime(e.Runtime(), name)
}

func (e *Engine) artifactEnablePath(name string) string {
	return artifactEnablePathForRuntime(e.Runtime(), name)
}

func (e *Engine) artifactSourcePath(name string) string {
	return artifactSourcePathForRuntime(e.Runtime(), name)
}

func (e *Engine) artifactExtractDir(name string) string {
	return artifactExtractDirForRuntime(e.Runtime(), name)
}
