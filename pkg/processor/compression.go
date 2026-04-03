package processor

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
)

// maxDecompressedSize caps how much data gunzip/gzip decompression will
// produce. This guards against gzip bombs — tiny compressed payloads that
// expand to gigabytes.  500 MB is generous for any legitimate IP feed.
const maxDecompressedSize int64 = 500 * 1024 * 1024

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if r.ctx == nil {
		return r.r.Read(p)
	}
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.r.Read(p)
	if err != nil {
		return n, err
	}
	if ctxErr := r.ctx.Err(); ctxErr != nil {
		return n, ctxErr
	}
	return n, nil
}

func gunzipFile(ctx context.Context, input []byte, _ map[string]string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	reader, err := gzip.NewReader(bytes.NewReader(input))
	if err != nil {
		return nil, err
	}
	defer func() { _ = reader.Close() }()
	limited := io.LimitReader(contextReader{ctx: ctx, r: reader}, maxDecompressedSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxDecompressedSize {
		return nil, fmt.Errorf("decompressed data exceeds %d bytes limit", maxDecompressedSize)
	}
	return data, nil
}
