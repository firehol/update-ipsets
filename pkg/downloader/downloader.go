package downloader

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/internal/fileutil"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
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
	tmpFile, err := createGeneratedTemp(tmpDir, "dl-copy-*.tmp")
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

func createGeneratedTemp(dir, pattern string) (*os.File, error) {
	tmp, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, err
	}
	if err := tmp.Chmod(fileutil.GeneratedFileMode); err != nil {
		tmpName := tmp.Name()
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return nil, fmt.Errorf("chmod temp file: %w", err)
	}
	return tmp, nil
}
