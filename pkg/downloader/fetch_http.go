package downloader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/internal/observability"
	"go.opentelemetry.io/otel/attribute"
)

func (c *Client) Fetch(ctx context.Context, req Request) (result *Result, err error) {
	started := time.Now()
	ctx, span := observability.Start(ctx, "download.fetch",
		attribute.String("feed.name", req.Name),
		attribute.String("download.downloader", req.Downloader),
	)
	defer func() {
		attrs := []attribute.KeyValue{attribute.String("download.downloader", req.Downloader)}
		status := "error"
		var bytes int64
		if result != nil {
			status = string(result.Status)
			bytes = result.BodySize
		}
		attrs = append(attrs, attribute.String("download.status", status))
		observability.TryCount("download.fetches", 1, attrs...)
		observability.TryBytes("download.fetch", bytes, attrs...)
		observability.TryDuration("download.fetch", time.Since(started), attrs...)
		if err != nil || status == string(StatusFailed) {
			observability.TryCount("download.errors", 1, attrs...)
		}
		observability.End(span, err)
	}()
	now := time.Now().UTC()
	return c.fetch(ctx, req, now)
}

func (c *Client) fetch(ctx context.Context, req Request, now time.Time) (*Result, error) {
	if req.Downloader == "copyfile" {
		return fetchLocalCopy(req, now)
	}
	if req.URL == "" {
		return nil, fmt.Errorf("empty download url")
	}
	if IsInternalURL(req.URL) {
		return fetchInternal(req, now)
	}
	if result, handled, err := fetchLocalURLIfNeeded(req, now); handled || err != nil {
		return result, err
	}
	return c.fetchHTTP(ctx, req, now)
}

func fetchLocalURLIfNeeded(req Request, now time.Time) (*Result, bool, error) {
	parsed, err := url.Parse(req.URL)
	if err != nil {
		return nil, false, nil
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme == "file" {
		if parsed.Host != "" {
			return nil, true, fmt.Errorf("file url host component is not allowed: %q", req.URL)
		}
		result, err := fetchLocalPath(req, now, parsed.Path)
		return result, true, err
	}
	if scheme != "" && scheme != "http" && scheme != "https" {
		return nil, true, fmt.Errorf("disallowed url scheme %q (only http, https, and file are permitted)", parsed.Scheme)
	}
	return nil, false, nil
}

func (c *Client) fetchHTTP(ctx context.Context, req Request, now time.Time) (*Result, error) {
	req = withHTTPDefaults(req)
	opts := parseCurlLikeOptions(req.DownloaderOptions)
	httpReq, err := buildHTTPRequest(ctx, req, opts)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return &Result{
			Status:    StatusFailed,
			Message:   err.Error(),
			CheckedAt: now,
		}, nil
	}
	defer func() { _ = resp.Body.Close() }()
	return handleHTTPResponse(resp, req, now)
}

func withHTTPDefaults(req Request) Request {
	if req.UserAgent == "" {
		req.UserAgent = "update-ipsets"
	}
	if req.Referer == "" {
		req.Referer = "https://iplists.firehol.org/"
	}
	return req
}

func buildHTTPRequest(ctx context.Context, req Request, opts curlLikeOptions) (*http.Request, error) {
	method, body := requestMethodAndBody(opts)
	httpReq, err := http.NewRequestWithContext(ctx, method, req.URL, body)
	if err != nil {
		return nil, err
	}
	applyHTTPRequestHeaders(httpReq, req, opts)
	applyConditionalHeaders(httpReq, req)
	return httpReq, nil
}

func requestMethodAndBody(opts curlLikeOptions) (string, io.Reader) {
	method := http.MethodGet
	var body io.Reader
	if opts.Data != "" {
		method = http.MethodPost
		body = strings.NewReader(opts.Data)
	}
	if opts.Method != "" {
		method = strings.ToUpper(opts.Method)
	}
	return method, body
}

func applyHTTPRequestHeaders(httpReq *http.Request, req Request, opts curlLikeOptions) {
	httpReq.Header.Set("User-Agent", req.UserAgent)
	referer := req.Referer
	if opts.Referer != "" {
		referer = opts.Referer
	}
	httpReq.Header.Set("Referer", referer)
	if opts.Data != "" && !hasHeader(opts.Headers, "Content-Type") {
		httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for key, value := range opts.Headers {
		httpReq.Header.Set(key, value)
	}
	if opts.Username != "" || opts.Password != "" {
		httpReq.SetBasicAuth(opts.Username, opts.Password)
	}
}

func hasHeader(headers map[string]string, name string) bool {
	for key := range headers {
		if strings.EqualFold(key, name) {
			return true
		}
	}
	return false
}

func applyConditionalHeaders(httpReq *http.Request, req Request) {
	if req.NoIfModifiedSince || req.ReferencePath == "" {
		return
	}
	if info, err := os.Stat(req.ReferencePath); err == nil {
		httpReq.Header.Set("If-Modified-Since", info.ModTime().UTC().Format(http.TimeFormat))
	}
}

func handleHTTPResponse(resp *http.Response, req Request, now time.Time) (*Result, error) {
	if resp.StatusCode == http.StatusNotModified {
		return &Result{
			Status:    StatusNotModified,
			Message:   "HTTP/304 Not Modified",
			HTTPCode:  resp.StatusCode,
			CheckedAt: now,
		}, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &Result{
			Status:    StatusFailed,
			Message:   fmt.Sprintf("HTTP/%d %s", resp.StatusCode, http.StatusText(resp.StatusCode)),
			HTTPCode:  resp.StatusCode,
			CheckedAt: now,
		}, nil
	}
	return streamHTTPBody(resp, req, now)
}

func streamHTTPBody(resp *http.Response, req Request, now time.Time) (*Result, error) {
	tmpDir := req.TmpDir
	if tmpDir == "" {
		tmpDir = os.TempDir()
	}
	tmpFile, err := createGeneratedTemp(tmpDir, "dl-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = tmpFile.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	hash, written, err := copyHTTPBody(tmpFile, resp.Body, req.MaxDownloadSize)
	if err != nil {
		return &Result{
			Status:    StatusFailed,
			Message:   err.Error(),
			HTTPCode:  resp.StatusCode,
			CheckedAt: now,
		}, nil
	}
	maxSize := effectiveMaxDownloadSize(req.MaxDownloadSize)
	if maxSize > 0 && written > maxSize {
		return &Result{
			Status:    StatusFailed,
			Message:   fmt.Sprintf("response body exceeds max download size (%d bytes)", maxSize),
			HTTPCode:  resp.StatusCode,
			CheckedAt: now,
		}, nil
	}
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("close temp file: %w", err)
	}
	if written == 0 && !req.AcceptEmpty {
		return &Result{
			Status:    StatusFailed,
			Message:   "downloaded file is empty",
			HTTPCode:  resp.StatusCode,
			CheckedAt: now,
		}, nil
	}

	bodyHash := hex.EncodeToString(hash)
	modified := responseModifiedTime(resp, now)
	if req.ReferencePath != "" {
		if same, _ := fileHashEquals(req.ReferencePath, bodyHash); same {
			return &Result{
				Status:       StatusSame,
				Message:      fmt.Sprintf("HTTP/%d OK", resp.StatusCode),
				HTTPCode:     resp.StatusCode,
				BodySize:     written,
				BodyHash:     bodyHash,
				ModifiedTime: modified,
				CheckedAt:    now,
			}, nil
		}
	}

	cleanup = false
	return &Result{
		Status:       StatusOK,
		Message:      fmt.Sprintf("HTTP/%d OK", resp.StatusCode),
		HTTPCode:     resp.StatusCode,
		BodyPath:     tmpPath,
		BodySize:     written,
		BodyHash:     bodyHash,
		ModifiedTime: modified,
		CheckedAt:    now,
	}, nil
}

func copyHTTPBody(dst io.Writer, body io.Reader, configuredMaxSize int64) ([]byte, int64, error) {
	hasher := sha256.New()
	src := body
	maxSize := effectiveMaxDownloadSize(configuredMaxSize)
	if maxSize > 0 {
		src = io.LimitReader(src, maxSize+1)
	}
	written, err := io.Copy(dst, io.TeeReader(src, hasher))
	if err != nil {
		return nil, written, err
	}
	return hasher.Sum(nil), written, nil
}

func effectiveMaxDownloadSize(configured int64) int64 {
	if configured == 0 {
		return DefaultMaxDownloadSize
	}
	return configured
}

func responseModifiedTime(resp *http.Response, fallback time.Time) time.Time {
	if header := resp.Header.Get("Last-Modified"); header != "" {
		if parsed, err := http.ParseTime(header); err == nil {
			return parsed.UTC()
		}
	}
	return fallback
}
