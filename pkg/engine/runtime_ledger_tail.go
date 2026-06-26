package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

func readFileTailContext(ctx context.Context, path string, bytesWanted int64) ([]byte, bool, error) {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return nil, false, err
	}
	file, err := openFilePathUnderRoot(filepath.Dir(path), path)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = file.Close() }()

	if err := contextErr(ctx); err != nil {
		return nil, false, err
	}
	info, err := file.Stat()
	if err != nil {
		return nil, false, err
	}
	size := info.Size()
	if size <= 0 {
		return nil, true, nil
	}
	if bytesWanted <= 0 || bytesWanted > size {
		bytesWanted = size
	}
	if bytesWanted > int64(int(^uint(0)>>1)) {
		return nil, false, fmt.Errorf("tail window too large for %s", path)
	}
	if err := contextErr(ctx); err != nil {
		return nil, false, err
	}
	offset := size - bytesWanted
	buf := make([]byte, int(bytesWanted))
	if _, err := file.ReadAt(buf, offset); err != nil && !errors.Is(err, io.EOF) {
		return nil, false, err
	}
	return buf, offset == 0, nil
}

func parseChangesetTailData(data []byte, complete bool) []ChangesetPoint {
	if len(data) == 0 {
		return nil
	}
	text := string(data)
	if !complete {
		idx := strings.IndexByte(text, '\n')
		if idx < 0 {
			return nil
		}
		text = text[idx+1:]
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	out := make([]ChangesetPoint, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.EqualFold(line, strings.TrimSpace(changesetLedgerHeader)) || strings.EqualFold(line, strings.TrimSpace(oldChangesetLedgerHeader)) {
			continue
		}
		if point, ok := parseChangesetCSVLine(line); ok {
			out = append(out, point)
		}
	}
	return out
}

func trimChangesetTail(points []ChangesetPoint, complete bool, limit int) ([]ChangesetPoint, bool) {
	if len(points) > limit {
		return append([]ChangesetPoint(nil), points[len(points)-limit:]...), true
	}
	if complete {
		if len(points) <= 1 {
			return nil, true
		}
		return append([]ChangesetPoint(nil), points[1:]...), true
	}
	return nil, false
}

func loadChangesetTail(path string, limit int) ([]ChangesetPoint, error) {
	return loadChangesetTailContext(context.Background(), path, limit)
}

func loadChangesetTailContext(ctx context.Context, path string, limit int) ([]ChangesetPoint, error) {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	runRuntimeLedgerLoadHook("changesets", "")
	if limit < 1 {
		limit = 1
	}
	window := int64(64 * 1024)
	for {
		if err := contextErr(ctx); err != nil {
			return nil, err
		}
		data, complete, err := readFileTailContext(ctx, path, window)
		if err != nil {
			return nil, err
		}
		points := parseChangesetTailData(data, complete)
		if tail, ok := trimChangesetTail(points, complete, limit); ok {
			return tail, nil
		}
		if complete {
			return nil, nil
		}
		if window > int64(int(^uint(0)>>1))/2 {
			window = int64(int(^uint(0) >> 1))
		} else {
			window *= 2
		}
	}
}
