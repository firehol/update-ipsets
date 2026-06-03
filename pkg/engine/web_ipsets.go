package engine

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/pkg/output"
)

func (e *Engine) copyUpdatedIPSetsToWeb(updatedNames []string) ([]output.GeneratedFile, error) {
	if e.runtime.WebDirForIPSets == "" || !dirExists(e.runtime.WebDirForIPSets) {
		return nil, nil
	}
	names := dedupeStrings(updatedNames)
	slices.Sort(names)

	generated := make([]output.GeneratedFile, 0, len(names))
	for _, name := range names {
		if !e.isPublicFeedName(name) {
			continue
		}
		if !e.isRedistributable(name) {
			continue
		}
		entry := e.state.EntrySnapshot(name)
		if entry == nil || entry.File == "" {
			continue
		}
		if !rawFeedFileMatches(name, entry.File) {
			return nil, fmt.Errorf("set %q has unexpected materialized file %q", name, entry.File)
		}
		if _, ok := safeRuntimeFilePath(e.runtime.BaseDir, entry.File); !ok {
			return nil, fmt.Errorf("set %q has unsafe materialized file %q", name, entry.File)
		}
		dst := filepath.Join(e.runtime.WebDirForIPSets, entry.File)
		mod, err := copyFileViaNew(e.runtime.BaseDir, entry.File, dst, e.runtime.WebOwner)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		generated = append(generated, output.GeneratedFile{
			Path:            dst,
			Timestamp:       mod.UTC(),
			Redistributable: true,
		})
	}
	return generated, nil
}

func copyFileViaNew(srcRoot, srcRel, dst, owner string) (time.Time, error) {
	if err := os.MkdirAll(filepath.Dir(dst), generatedDirMode); err != nil {
		return time.Time{}, err
	}
	if err := chownPath(owner, filepath.Dir(dst)); err != nil {
		return time.Time{}, err
	}
	in, err := openFileInRoot(srcRoot, srcRel)
	if err != nil {
		return time.Time{}, err
	}
	defer func() { _ = in.Close() }()
	info, err := in.Stat()
	if err != nil {
		return time.Time{}, err
	}
	mod := info.ModTime()

	out, err := os.CreateTemp(filepath.Dir(dst), ".new-*")
	if err != nil {
		return time.Time{}, err
	}
	tmp := out.Name()
	defer func() { _ = os.Remove(tmp) }()
	if err := out.Chmod(generatedFileMode); err != nil {
		_ = out.Close()
		return time.Time{}, err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return time.Time{}, err
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		return time.Time{}, err
	}
	if err := out.Close(); err != nil {
		return time.Time{}, err
	}
	if !mod.IsZero() {
		if err := os.Chtimes(tmp, mod, mod); err != nil {
			return time.Time{}, err
		}
	}
	if err := chownPath(owner, tmp); err != nil {
		return time.Time{}, err
	}
	if err := os.Rename(tmp, dst); err != nil {
		return time.Time{}, err
	}
	return mod, nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
