package engine

import (
	"bytes"
	"context"
	"io"
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

const webPublishStagePattern = ".update-ipsets-web-*"
const webPublishStagePrefix = ".update-ipsets-web-"

func newStagedPublishBatch(liveDir, owner, pattern string) (*stagedPublishBatch, error) {
	if err := os.MkdirAll(liveDir, generatedDirMode); err != nil {
		return nil, err
	}
	stageDir, err := os.MkdirTemp(liveDir, pattern)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(stageDir, generatedDirMode); err != nil {
		_ = os.RemoveAll(stageDir)
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
	batch, err := newStagedPublishBatch(e.outputDir(), e.runtime.WebOwner, webPublishStagePattern)
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
	return b.applyGeneratedFileTimestampsContext(context.Background(), files)
}

func (b *stagedPublishBatch) applyGeneratedFileTimestampsContext(ctx context.Context, files []output.GeneratedFile) error {
	ctx = nonNilContext(ctx)
	if b == nil || len(files) == 0 {
		return nil
	}
	for _, file := range files {
		if err := contextErr(ctx); err != nil {
			return err
		}
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
	return b.publishContext(context.Background())
}

func (b *stagedPublishBatch) publishContext(ctx context.Context) ([]string, error) {
	ctx = nonNilContext(ctx)
	if b == nil {
		return nil, nil
	}
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	published := make([]string, 0, 32)
	if err := filepath.WalkDir(b.stageDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := contextErr(ctx); err != nil {
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
			if err := os.MkdirAll(dst, generatedDirMode); err != nil {
				return err
			}
			if err := chownPath(b.owner, dst); err != nil {
				return err
			}
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(dst), generatedDirMode); err != nil {
			return err
		}
		if err := os.Chmod(path, generatedFileMode); err != nil {
			return err
		}
		if same, _ := sameRegularFileContentContext(ctx, path, dst); same {
			info, err := os.Stat(path)
			if err != nil {
				return err
			}
			if err := os.Chmod(dst, generatedFileMode); err != nil {
				return err
			}
			mod := info.ModTime()
			if err := os.Chtimes(dst, mod, mod); err != nil {
				return err
			}
			if err := chownPath(b.owner, dst); err != nil {
				return err
			}
			if err := os.Remove(path); err != nil {
				return err
			}
			published = append(published, dst)
			return nil
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
			if err := contextErr(ctx); err != nil {
				return nil, err
			}
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

func sameRegularFileContent(left, right string) (bool, error) {
	return sameRegularFileContentContext(context.Background(), left, right)
}

func sameRegularFileContentContext(ctx context.Context, left, right string) (bool, error) {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return false, err
	}
	leftInfo, err := os.Lstat(left)
	if err != nil {
		return false, err
	}
	rightInfo, err := os.Lstat(right)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if !leftInfo.Mode().IsRegular() || !rightInfo.Mode().IsRegular() {
		return false, nil
	}
	if leftInfo.Size() != rightInfo.Size() {
		return false, nil
	}
	leftFile, err := os.Open(left)
	if err != nil {
		return false, err
	}
	defer func() { _ = leftFile.Close() }()
	rightFile, err := os.Open(right)
	if err != nil {
		return false, err
	}
	defer func() { _ = rightFile.Close() }()
	leftBuf := make([]byte, 32*1024)
	rightBuf := make([]byte, 32*1024)
	remaining := leftInfo.Size()
	for remaining > 0 {
		if err := contextErr(ctx); err != nil {
			return false, err
		}
		chunkSize := int64(len(leftBuf))
		if remaining < chunkSize {
			chunkSize = remaining
		}
		leftN, leftErr := io.ReadFull(leftFile, leftBuf[:chunkSize])
		rightN, rightErr := io.ReadFull(rightFile, rightBuf[:chunkSize])
		if leftErr != nil {
			return false, leftErr
		}
		if rightErr != nil {
			return false, rightErr
		}
		if leftN != rightN || !bytes.Equal(leftBuf[:leftN], rightBuf[:rightN]) {
			return false, nil
		}
		remaining -= int64(leftN)
	}
	leftEOF, err := regularFileAtEOF(leftFile)
	if err != nil || !leftEOF {
		return false, err
	}
	rightEOF, err := regularFileAtEOF(rightFile)
	if err != nil || !rightEOF {
		return false, err
	}
	return true, nil
}

func regularFileAtEOF(file *os.File) (bool, error) {
	var extra [1]byte
	n, err := file.Read(extra[:])
	if err == io.EOF {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return n == 0, nil
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
