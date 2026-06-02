package engine

import (
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
		src := filepath.Join(e.runtime.BaseDir, entry.File)
		info, err := os.Stat(src)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		dst := filepath.Join(e.runtime.WebDirForIPSets, entry.File)
		if err := copyFileViaNew(src, dst, e.runtime.WebOwner, info.ModTime()); err != nil {
			return nil, err
		}
		generated = append(generated, output.GeneratedFile{
			Path:            dst,
			Timestamp:       info.ModTime().UTC(),
			Redistributable: true,
		})
	}
	return generated, nil
}

func copyFileViaNew(src, dst, owner string, mod time.Time) error {
	if err := os.MkdirAll(filepath.Dir(dst), generatedDirMode); err != nil {
		return err
	}
	if err := chownPath(owner, filepath.Dir(dst)); err != nil {
		return err
	}
	tmp := dst + ".new"
	defer func() { _ = os.Remove(tmp) }()
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, generatedFileMode)
	if err != nil {
		return err
	}
	if err := out.Chmod(generatedFileMode); err != nil {
		_ = out.Close()
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if !mod.IsZero() {
		if err := os.Chtimes(tmp, mod, mod); err != nil {
			return err
		}
	}
	if err := chownPath(owner, tmp); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
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
