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
	"path/filepath"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/internal/observability"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
)

type Status string

const (
	StatusOK          Status = "ok"
	StatusNotModified Status = "not_modified"
	StatusSame        Status = "same"
	StatusSkipped     Status = "skipped"
	StatusFailed      Status = "failed"
)

// DefaultMaxDownloadSize is the default cap on response body size (100 MB).
const DefaultMaxDownloadSize int64 = 100 * 1024 * 1024

type Request struct {
	Name              string
	URL               string
	ReferencePath     string
	UserAgent         string
	MaxConnectTime    time.Duration
	MaxDownloadTime   time.Duration
	NoIfModifiedSince bool
	Downloader        string
	DownloaderOptions string
	Referer           string
	AcceptEmpty       bool
	// MaxDownloadSize caps the response body. Zero means DefaultMaxDownloadSize.
	// Set to -1 to disable the limit.
	MaxDownloadSize int64
	// TmpDir is the directory for temporary download files.
	// If empty, os.TempDir() is used.
	TmpDir string
}

// Result describes the outcome of a download. On StatusOK or StatusSame the
// downloaded body is stored in a temporary file whose path is BodyPath. The
// caller owns the file and must either rename or remove it.
type Result struct {
	Status       Status
	Message      string
	HTTPCode     int
	BodyPath     string // path to temp file with downloaded body (empty when no body)
	BodySize     int64
	BodyHash     string // hex-encoded SHA-256 of the body
	ModifiedTime time.Time
	CheckedAt    time.Time
}

// CleanUp removes the temporary body file if it exists. Safe to call on nil
// or on results that have no body file.
func (r *Result) CleanUp() {
	if r != nil && r.BodyPath != "" {
		_ = os.Remove(r.BodyPath)
		r.BodyPath = ""
	}
}

type Client struct {
	client *http.Client
}

func New(maxConnectTime, maxDownloadTime time.Duration) *Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = maxConnectTime
	return &Client{
		client: &http.Client{
			Timeout:   maxDownloadTime,
			Transport: otelhttp.NewTransport(transport),
			// Restrict redirects to HTTP(S) only and limit the chain length.
			// This prevents SSRF via redirect to file://, gopher://, etc.
			CheckRedirect: safeRedirectPolicy,
		},
	}
}

// maxRedirects is the maximum number of redirects allowed before aborting.
const maxRedirects = 10

// safeRedirectPolicy rejects redirects to non-HTTP(S) schemes and limits
// the redirect chain to prevent abuse.
func safeRedirectPolicy(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("stopped after %d redirects", maxRedirects)
	}
	scheme := strings.ToLower(req.URL.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("redirect to disallowed scheme %q", req.URL.Scheme)
	}
	return nil
}

func (c *Client) Fetch(ctx context.Context, req Request) (result *Result, err error) {
	started := time.Now()
	ctx, span := observability.Start(ctx, "download.fetch",
		attribute.String("feed.name", req.Name),
		attribute.String("download.downloader", req.Downloader),
	)
	defer func() {
		attrs := []attribute.KeyValue{
			attribute.String("feed.name", req.Name),
			attribute.String("download.downloader", req.Downloader),
		}
		status := "error"
		var bytes int64
		if result != nil {
			status = string(result.Status)
			bytes = result.BodySize
			if result.HTTPCode > 0 {
				attrs = append(attrs, attribute.Int("http.response.status_code", result.HTTPCode))
				observability.Count(ctx, fmt.Sprintf("download.http_status.%d", result.HTTPCode), 1, attrs...)
			}
		}
		attrs = append(attrs, attribute.String("download.status", status))
		observability.Observe(ctx, "download.fetch", 1, bytes, time.Since(started), attrs...)
		observability.Count(ctx, "download."+status, 1, attrs...)
		observability.End(span, err)
	}()
	now := time.Now().UTC()
	if req.Downloader == "copyfile" {
		return fetchLocalCopy(req, now)
	}
	if req.URL == "" {
		return nil, fmt.Errorf("empty download url")
	}
	// Synthetic sources: resolve through the in-process registry and
	// return a Result that looks identical to a real HTTP download so
	// the engine can treat internal:// sources uniformly.
	if IsInternalURL(req.URL) {
		return fetchInternal(req, now)
	}
	// Validate the URL scheme before making any request to prevent SSRF.
	if parsed, err := url.Parse(req.URL); err == nil {
		scheme := strings.ToLower(parsed.Scheme)
		if scheme == "file" {
			if parsed.Host != "" {
				return nil, fmt.Errorf("file url host component is not allowed: %q", req.URL)
			}
			return fetchLocalPath(req, now, parsed.Path)
		}
		if scheme != "" && scheme != "http" && scheme != "https" {
			return nil, fmt.Errorf("disallowed url scheme %q (only http, https, and file are permitted)", parsed.Scheme)
		}
	}
	if req.UserAgent == "" {
		req.UserAgent = "update-ipsets"
	}
	if req.Referer == "" {
		req.Referer = "https://iplists.firehol.org/"
	}
	opts := parseCurlLikeOptions(req.DownloaderOptions)
	method := http.MethodGet
	var requestBody io.Reader
	if opts.Data != "" {
		method = http.MethodPost
		requestBody = strings.NewReader(opts.Data)
	}
	if opts.Method != "" {
		method = strings.ToUpper(opts.Method)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, req.URL, requestBody)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("User-Agent", req.UserAgent)
	referer := req.Referer
	if opts.Referer != "" {
		referer = opts.Referer
	}
	httpReq.Header.Set("Referer", referer)
	// Match curl's default: when --data is used, curl sends
	// Content-Type: application/x-www-form-urlencoded unless the
	// caller provided one explicitly. Go's net/http does NOT do
	// this automatically, so POSTs were arriving at servers with
	// no Content-Type header and form-based endpoints (gpf_comics
	// being the canonical example) silently returned their HTML
	// login page instead of the actual data. Only set the
	// default when the caller has not specified their own
	// content-type via -H.
	hasExplicitContentType := false
	for key := range opts.Headers {
		if strings.EqualFold(key, "Content-Type") {
			hasExplicitContentType = true
			break
		}
	}
	if opts.Data != "" && !hasExplicitContentType {
		httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for key, value := range opts.Headers {
		httpReq.Header.Set(key, value)
	}
	if opts.Username != "" || opts.Password != "" {
		httpReq.SetBasicAuth(opts.Username, opts.Password)
	}
	if !req.NoIfModifiedSince && req.ReferencePath != "" {
		if info, err := os.Stat(req.ReferencePath); err == nil {
			httpReq.Header.Set("If-Modified-Since", info.ModTime().UTC().Format(http.TimeFormat))
		}
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

	// Stream response body to a temp file while computing its SHA-256 hash.
	tmpDir := req.TmpDir
	if tmpDir == "" {
		tmpDir = os.TempDir()
	}
	tmpFile, err := os.CreateTemp(tmpDir, "dl-*.tmp")
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

	maxSize := req.MaxDownloadSize
	if maxSize == 0 {
		maxSize = DefaultMaxDownloadSize
	}

	hasher := sha256.New()
	var src io.Reader = resp.Body
	if maxSize > 0 {
		src = io.LimitReader(src, maxSize+1)
	}
	src = io.TeeReader(src, hasher)
	written, err := io.Copy(tmpFile, src)
	if err != nil {
		return &Result{
			Status:    StatusFailed,
			Message:   err.Error(),
			HTTPCode:  resp.StatusCode,
			CheckedAt: now,
		}, nil
	}
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

	bodyHash := hex.EncodeToString(hasher.Sum(nil))

	modified := now
	if header := resp.Header.Get("Last-Modified"); header != "" {
		if parsed, err := http.ParseTime(header); err == nil {
			modified = parsed.UTC()
		}
	}

	// Same-body detection: compare hash of downloaded body against reference file.
	if req.ReferencePath != "" {
		if same, _ := fileHashEquals(req.ReferencePath, bodyHash); same {
			// Body is identical to the reference — no need to keep the temp file.
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

	// New body — hand ownership of the temp file to the caller.
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

// sha256Hex returns the hex-encoded SHA-256 hash of the given bytes.
// Shared by the HTTP and internal:// fetch paths so the body hash is
// computed the same way regardless of how the bytes arrived.
func sha256Hex(body []byte) string {
	h := sha256.New()
	_, _ = h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

// fileHashEquals computes SHA-256 of the file at path and returns true if it
// matches the given hex hash. Returns false on any error.
func fileHashEquals(path, hexHash string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	return hex.EncodeToString(h.Sum(nil)) == hexHash, nil
}

func fetchLocalCopy(req Request, now time.Time) (*Result, error) {
	options := strings.Fields(req.DownloaderOptions)
	if len(options) == 0 {
		return &Result{
			Status:    StatusFailed,
			Message:   "copyfile downloader requires a source path",
			CheckedAt: now,
		}, nil
	}
	return fetchLocalPath(req, now, options[0])
}

func fetchLocalPath(req Request, now time.Time, srcPath string) (*Result, error) {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return &Result{
			Status:    StatusFailed,
			Message:   err.Error(),
			CheckedAt: now,
		}, nil
	}
	defer func() { _ = srcFile.Close() }()

	info, err := srcFile.Stat()
	if err != nil {
		return nil, err
	}

	// Stream the local file to a temp file while computing hash.
	tmpDir := req.TmpDir
	if tmpDir == "" {
		tmpDir = os.TempDir()
	}
	tmpFile, err := os.CreateTemp(tmpDir, "dl-copy-*.tmp")
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

	maxSize := req.MaxDownloadSize
	if maxSize == 0 {
		maxSize = DefaultMaxDownloadSize
	}

	hasher := sha256.New()
	var src io.Reader = srcFile
	if maxSize > 0 {
		src = io.LimitReader(srcFile, maxSize+1)
	}
	written, err := io.Copy(tmpFile, io.TeeReader(src, hasher))
	if err != nil {
		return &Result{
			Status:    StatusFailed,
			Message:   err.Error(),
			CheckedAt: now,
		}, nil
	}
	if maxSize > 0 && written > maxSize {
		return &Result{
			Status:    StatusFailed,
			Message:   fmt.Sprintf("local file exceeds max download size (%d bytes)", maxSize),
			CheckedAt: now,
		}, nil
	}
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("close temp file: %w", err)
	}

	if written == 0 && !req.AcceptEmpty {
		return &Result{
			Status:    StatusFailed,
			Message:   "copied file is empty",
			CheckedAt: now,
		}, nil
	}

	bodyHash := hex.EncodeToString(hasher.Sum(nil))

	if req.ReferencePath != "" {
		if same, _ := fileHashEquals(req.ReferencePath, bodyHash); same {
			return &Result{
				Status:       StatusSame,
				Message:      fmt.Sprintf("copied file %q", filepath.Base(srcPath)),
				BodySize:     written,
				BodyHash:     bodyHash,
				ModifiedTime: info.ModTime().UTC(),
				CheckedAt:    now,
			}, nil
		}
	}

	cleanup = false
	return &Result{
		Status:       StatusOK,
		Message:      fmt.Sprintf("copied file %q", filepath.Base(srcPath)),
		BodyPath:     tmpPath,
		BodySize:     written,
		BodyHash:     bodyHash,
		ModifiedTime: info.ModTime().UTC(),
		CheckedAt:    now,
	}, nil
}

type curlLikeOptions struct {
	Data     string
	Method   string
	Referer  string
	Username string
	Password string
	Headers  map[string]string
}

func parseCurlLikeOptions(raw string) curlLikeOptions {
	fields := splitShellWords(raw)
	out := curlLikeOptions{Headers: map[string]string{}}
	for i := 0; i < len(fields); i++ {
		switch fields[i] {
		case "--data", "-d", "--data-raw":
			if i+1 < len(fields) {
				out.Data = fields[i+1]
				i++
			}
		case "--request", "-X":
			if i+1 < len(fields) {
				out.Method = fields[i+1]
				i++
			}
		case "--referer":
			if i+1 < len(fields) {
				out.Referer = fields[i+1]
				i++
			}
		case "--user", "-u":
			if i+1 < len(fields) {
				out.Username, out.Password, _ = strings.Cut(fields[i+1], ":")
				i++
			}
		case "--header", "-H":
			if i+1 < len(fields) {
				key, value, ok := strings.Cut(fields[i+1], ":")
				if ok {
					key = strings.TrimSpace(key)
					if key != "" && isValidHeaderName(key) {
						out.Headers[key] = strings.TrimSpace(value)
					}
				}
				i++
			}
		default:
			if strings.HasPrefix(fields[i], "--data=") {
				out.Data = strings.TrimPrefix(fields[i], "--data=")
			}
			if strings.HasPrefix(fields[i], "--request=") {
				out.Method = strings.TrimPrefix(fields[i], "--request=")
			}
			if strings.HasPrefix(fields[i], "--referer=") {
				out.Referer = strings.TrimPrefix(fields[i], "--referer=")
			}
			if strings.HasPrefix(fields[i], "--user=") {
				out.Username, out.Password, _ = strings.Cut(strings.TrimPrefix(fields[i], "--user="), ":")
			}
		}
	}
	return out
}

// isValidHeaderName checks whether name is a valid HTTP header field name
// per RFC 7230 (token = 1*tchar).
func isValidHeaderName(name string) bool {
	for _, c := range name {
		if (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') && (c < '0' || c > '9') &&
			c != '!' && c != '#' && c != '$' && c != '%' && c != '&' && c != '\'' &&
			c != '*' && c != '+' && c != '-' && c != '.' && c != '^' && c != '_' &&
			c != '`' && c != '|' && c != '~' {
			return false
		}
	}
	return len(name) > 0
}

func splitShellWords(raw string) []string {
	var (
		out     []string
		current strings.Builder
		quote   rune
		escape  bool
	)
	flush := func() {
		if current.Len() == 0 {
			return
		}
		out = append(out, current.String())
		current.Reset()
	}
	for _, r := range raw {
		switch {
		case escape:
			current.WriteRune(r)
			escape = false
		case r == '\\' && quote != '\'':
			escape = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t' || r == '\n':
			flush()
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return out
}
