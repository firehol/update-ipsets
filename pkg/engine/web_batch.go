package engine

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/firehol/update-ipsets/pkg/output"
)

type stagedPublishBatch struct {
	liveDir  string
	stageDir string
	owner    string
	deletes  map[string]struct{}
}

type webPublishBatch struct {
	*stagedPublishBatch
}

func newStagedPublishBatch(liveDir, owner, pattern string) (*stagedPublishBatch, error) {
	if err := os.MkdirAll(liveDir, 0o755); err != nil {
		return nil, err
	}
	stageDir, err := os.MkdirTemp(liveDir, pattern)
	if err != nil {
		return nil, err
	}
	return &stagedPublishBatch{
		liveDir:  liveDir,
		stageDir: stageDir,
		owner:    owner,
		deletes:  map[string]struct{}{},
	}, nil
}

func (e *Engine) newWebPublishBatch() (*webPublishBatch, error) {
	batch, err := newStagedPublishBatch(e.outputDir(), e.runtime.WebOwner, ".update-ipsets-web-*")
	if err != nil {
		return nil, err
	}
	return &webPublishBatch{stagedPublishBatch: batch}, nil
}

func (b *stagedPublishBatch) cleanup() {
	if b == nil || b.stageDir == "" {
		return
	}
	_ = os.RemoveAll(b.stageDir)
}

func (b *stagedPublishBatch) markDelete(rel string) {
	if b == nil {
		return
	}
	if clean, ok := cleanPublishRel(rel); ok {
		b.deletes[clean] = struct{}{}
	}
}

func (b *stagedPublishBatch) applyGeneratedFileTimestamps(files []output.GeneratedFile) error {
	if b == nil || len(files) == 0 {
		return nil
	}
	for _, file := range files {
		if file.Timestamp.IsZero() {
			continue
		}
		rel, err := filepath.Rel(b.liveDir, file.Path)
		if err != nil {
			return err
		}
		rel, ok := cleanPublishRel(rel)
		if !ok {
			continue
		}
		stagePath := filepath.Join(b.stageDir, rel)
		info, err := os.Stat(stagePath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if info.IsDir() {
			continue
		}
		ts := file.Timestamp.UTC()
		if err := os.Chtimes(stagePath, ts, ts); err != nil {
			return err
		}
	}
	return nil
}

func (b *stagedPublishBatch) publish() ([]string, error) {
	if b == nil {
		return nil, nil
	}
	published := make([]string, 0, 32)
	if err := filepath.WalkDir(b.stageDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(b.stageDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel, ok := cleanPublishRel(rel)
		if !ok {
			return nil
		}
		dst := filepath.Join(b.liveDir, rel)
		if d.IsDir() {
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return err
			}
			if err := chownPath(b.owner, dst); err != nil {
				return err
			}
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.Chmod(path, 0o644); err != nil {
			return err
		}
		if err := os.Rename(path, dst); err != nil {
			return err
		}
		if err := chownPath(b.owner, dst); err != nil {
			return err
		}
		published = append(published, dst)
		return nil
	}); err != nil {
		return nil, err
	}
	if len(b.deletes) > 0 {
		rels := make([]string, 0, len(b.deletes))
		for rel := range b.deletes {
			rels = append(rels, rel)
		}
		slices.Sort(rels)
		for _, rel := range rels {
			dst := filepath.Join(b.liveDir, rel)
			if err := os.RemoveAll(dst); err != nil {
				return nil, err
			}
			published = append(published, dst)
			pruneEmptyPublishParents(filepath.Dir(dst), b.liveDir)
		}
	}
	return published, os.RemoveAll(b.stageDir)
}

func cleanPublishRel(rel string) (string, bool) {
	rel = filepath.Clean(strings.TrimSpace(rel))
	if rel == "." || rel == "" || filepath.IsAbs(rel) {
		return "", false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

func pruneEmptyPublishParents(path, stop string) {
	stop = filepath.Clean(stop)
	for {
		path = filepath.Clean(path)
		if path == "." || path == stop || path == string(filepath.Separator) {
			return
		}
		if err := os.Remove(path); err != nil {
			return
		}
		path = filepath.Dir(path)
	}
}
