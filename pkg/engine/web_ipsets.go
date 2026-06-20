package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/pkg/output"
)

func (e *Engine) copyUpdatedIPSetsToWebContext(ctx context.Context, updatedNames []string) ([]output.GeneratedFile, error) {
	ctx = nonNilContext(ctx)
	if e.runtime.WebDirForIPSets == "" || !dirExists(e.runtime.WebDirForIPSets) {
		return nil, nil
	}
	names := dedupeStrings(updatedNames)
	slices.Sort(names)
	progress := e.beginActiveOperation("publish.copy_raw_ipsets", "", "copy", "feeds", int64(len(names)))
	defer progress.Finish()

	generated := make([]output.GeneratedFile, 0, len(names))
	for _, name := range names {
		if err := contextErr(ctx); err != nil {
			return nil, err
		}
		if !e.isPublicFeedName(name) {
			progress.Add(1, int64(len(names)), nil)
			continue
		}
		if !e.isRedistributable(name) {
			progress.Add(1, int64(len(names)), nil)
			continue
		}
		entry := e.state.EntrySnapshot(name)
		if entry == nil || entry.File == "" {
			progress.Add(1, int64(len(names)), nil)
			continue
		}
		if !rawFeedFileMatches(name, entry.File) {
			progress.Add(1, int64(len(names)), nil)
			return nil, fmt.Errorf("set %q has unexpected materialized file %q", name, entry.File)
		}
		if _, ok := safeRuntimeFilePath(e.runtime.BaseDir, entry.File); !ok {
			progress.Add(1, int64(len(names)), nil)
			return nil, fmt.Errorf("set %q has unsafe materialized file %q", name, entry.File)
		}
		dst := filepath.Join(e.runtime.WebDirForIPSets, entry.File)
		mod, err := copyFileViaNewContext(ctx, e.runtime.BaseDir, entry.File, dst, e.runtime.WebOwner)
		if err != nil {
			if os.IsNotExist(err) {
				progress.Add(1, int64(len(names)), nil)
				continue
			}
			progress.Add(1, int64(len(names)), nil)
			return nil, err
		}
		generated = append(generated, output.GeneratedFile{
			Path:            dst,
			Timestamp:       mod.UTC(),
			Redistributable: true,
		})
		progress.Add(1, int64(len(names)), nil)
	}
	return generated, nil
}

func copyFileViaNew(srcRoot, srcRel, dst, owner string) (time.Time, error) {
	return copyFileViaNewContext(context.Background(), srcRoot, srcRel, dst, owner)
}

func copyFileViaNewContext(ctx context.Context, srcRoot, srcRel, dst, owner string) (time.Time, error) {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return time.Time{}, err
	}
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
	if srcPath, ok := safeRuntimeFilePath(srcRoot, srcRel); ok {
		if same, _ := sameRegularFileContentContext(ctx, srcPath, dst); same {
			if err := os.Chmod(dst, generatedFileMode); err != nil {
				return time.Time{}, err
			}
			if !mod.IsZero() {
				if err := os.Chtimes(dst, mod, mod); err != nil {
					return time.Time{}, err
				}
			}
			if err := chownPath(owner, dst); err != nil {
				return time.Time{}, err
			}
			return mod, nil
		}
	}

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
	if _, err := copyWithContext(ctx, out, in); err != nil {
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

func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	ctx = nonNilContext(ctx)
	buf := make([]byte, 32*1024)
	var written int64
	for {
		if err := contextErr(ctx); err != nil {
			return written, err
		}
		nr, er := src.Read(buf)
		if nr > 0 {
			if err := contextErr(ctx); err != nil {
				return written, err
			}
			nw, ew := dst.Write(buf[:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				return written, ew
			}
			if nw != nr {
				return written, io.ErrShortWrite
			}
		}
		if er != nil {
			if er == io.EOF {
				return written, nil
			}
			return written, er
		}
	}
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
