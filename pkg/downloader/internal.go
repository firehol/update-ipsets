package downloader

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// ErrInternalNotModified is a sentinel error that internal providers
// may return in place of regenerating output when they can cheaply
// determine their inputs are unchanged since the last generation
// (typically by comparing input file mtimes against the mtime of the
// reference file passed in via InternalProvider). fetchInternal
// translates this into StatusSame without writing any temp file —
// the expensive union / merge / synthesis work is skipped entirely.
//
// Providers that always regenerate (e.g. rfc_reserved which returns
// a fixed byte array) should never return this error; the downloader's
// hash-based same-body detection catches the unchanged case for them.
var ErrInternalNotModified = errors.New("internal provider: not modified")

// InternalScheme is the URL scheme reserved for synthetic sources whose
// data is built into the binary. A URL of the form "internal://<name>"
// is resolved through a process-wide registry; the Fetch call returns
// the registered bytes as if they had been downloaded from the network.
//
// The canonical use is the rfc_reserved source: the 15 hardcoded RFC
// reserved IPv4 ranges live in pkg/engine/bogons_rfc.go and are
// registered here at engine startup so they flow through the same
// download/cache/process pipeline as any real source.
const InternalScheme = "internal"

// InternalSentinelTime is kept for backwards compatibility with
// tests that assert StatusSame responses carry the epoch timestamp.
// New code should NOT use this for fresh bodies; fetchInternal
// stamps generated bodies with time.Now().UTC() so downstream
// consumers (integrity checks, staleness comparisons, file mtime
// propagation through finalize → touchFileAt) see the real time
// the body was produced. The sentinel survives only as a signal
// on the StatusSame path where no body was generated.
var InternalSentinelTime = time.Unix(0, 0).UTC()

// InternalProvider returns the bytes for a synthetic source.
//
// referencePath is the path to the on-disk reference file the
// downloader would compare the freshly-produced body against (the
// "current output" from the previous successful run). Providers that
// can cheaply determine their inputs are unchanged since the
// reference file's mtime may stat the file and return
// ErrInternalNotModified to skip generation entirely.
//
// Providers that always regenerate may ignore referencePath; the
// downloader's existing SHA-256 same-body detection handles the
// unchanged case by returning StatusSame after the bytes are
// produced (so generation work is done, but no temp file is written).
//
// The function is invoked fresh on every Fetch call; implementations
// may cache internally if generation is expensive.
type InternalProvider func(referencePath string) ([]byte, error)

var (
	internalRegistryMu sync.RWMutex
	internalRegistry   = map[string]InternalProvider{}
)

// RegisterInternal binds name to a bytes provider. Subsequent calls
// overwrite the previous registration; this makes test setup easier
// and lets the engine re-register on each startup without leaking.
func RegisterInternal(name string, provider InternalProvider) {
	internalRegistryMu.Lock()
	defer internalRegistryMu.Unlock()
	internalRegistry[name] = provider
}

// UnregisterInternal removes a name from the registry. Mainly useful
// in tests that want to verify "missing provider" error paths.
func UnregisterInternal(name string) {
	internalRegistryMu.Lock()
	defer internalRegistryMu.Unlock()
	delete(internalRegistry, name)
}

// LookupInternal returns the registered provider for name. The boolean
// is false when no provider is registered. Used by the validator to
// reject internal:// URLs that reference unknown names.
func LookupInternal(name string) (InternalProvider, bool) {
	internalRegistryMu.RLock()
	defer internalRegistryMu.RUnlock()
	p, ok := internalRegistry[name]
	return p, ok
}

// IsInternalURL reports whether the URL uses the internal:// scheme.
// Accepts both "internal://name" and the rare "internal:name" form.
func IsInternalURL(url string) bool {
	return strings.HasPrefix(url, InternalScheme+"://") || strings.HasPrefix(url, InternalScheme+":")
}

// InternalName extracts the provider name from an internal:// URL.
// Returns the empty string if the URL is not an internal URL.
func InternalName(url string) string {
	if strings.HasPrefix(url, InternalScheme+"://") {
		return strings.TrimPrefix(url, InternalScheme+"://")
	}
	if strings.HasPrefix(url, InternalScheme+":") {
		return strings.TrimPrefix(url, InternalScheme+":")
	}
	return ""
}

// fetchInternal materializes a registered internal provider as a
// Result with a body file on disk, matching the real HTTP Fetch path so
// downstream code does not have to special-case synthetic sources.
func fetchInternal(req Request, now time.Time) (*Result, error) {
	name := InternalName(req.URL)
	if name == "" {
		return internalFailedResult(now, "malformed internal URL %q", req.URL), nil
	}
	provider, ok := LookupInternal(name)
	if !ok {
		return internalFailedResult(now, "no internal provider registered for %q", name), nil
	}
	body, err := provider(req.ReferencePath)
	if errors.Is(err, ErrInternalNotModified) {
		return internalNotModifiedResult(req, now, name), nil
	}
	if err != nil {
		return internalFailedResult(now, "internal provider %q: %v", name, err), nil
	}

	return internalBodyResult(req, now, name, body)
}

func internalFailedResult(now time.Time, format string, args ...any) *Result {
	return &Result{
		Status:    StatusFailed,
		Message:   fmt.Sprintf(format, args...),
		CheckedAt: now,
	}
}

func internalNotModifiedResult(req Request, now time.Time, name string) *Result {
	modTime := InternalSentinelTime
	if req.ReferencePath != "" {
		if info, statErr := os.Stat(req.ReferencePath); statErr == nil {
			modTime = info.ModTime().UTC()
		}
	}
	return &Result{
		Status:       StatusSame,
		Message:      fmt.Sprintf("internal provider %q: not modified", name),
		ModifiedTime: modTime,
		CheckedAt:    now,
	}
}

func internalBodyResult(req Request, now time.Time, name string, body []byte) (*Result, error) {
	if len(body) == 0 && !req.AcceptEmpty {
		return internalFailedResult(now, "internal provider %q returned empty body", name), nil
	}

	bodyHash := hashBytes(body)
	if req.ReferencePath != "" {
		if same, _ := fileHashEquals(req.ReferencePath, bodyHash); same {
			return internalSameReferenceResult(now, body, bodyHash), nil
		}
	}
	return materializeInternalBody(req, now, name, body, bodyHash)
}

func internalSameReferenceResult(now time.Time, body []byte, bodyHash string) *Result {
	return &Result{
		Status:       StatusSame,
		Message:      "internal source unchanged",
		BodySize:     int64(len(body)),
		BodyHash:     bodyHash,
		ModifiedTime: now,
		CheckedAt:    now,
	}
}

func materializeInternalBody(req Request, now time.Time, name string, body []byte, bodyHash string) (*Result, error) {
	tmpPath, err := writeInternalTempBody(req.TmpDir, body)
	if err != nil {
		return nil, err
	}
	stampInternalBodyTime(tmpPath, now)

	return &Result{
		Status:       StatusOK,
		Message:      fmt.Sprintf("internal source %q", name),
		BodyPath:     tmpPath,
		BodySize:     int64(len(body)),
		BodyHash:     bodyHash,
		ModifiedTime: now,
		CheckedAt:    now,
	}, nil
}

func writeInternalTempBody(tmpDir string, body []byte) (string, error) {
	if tmpDir == "" {
		tmpDir = os.TempDir()
	}
	tmp, err := createGeneratedTemp(tmpDir, "dl-internal-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("write internal body: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("close internal temp: %w", err)
	}
	return tmpPath, nil
}

func stampInternalBodyTime(tmpPath string, now time.Time) {
	// Stamp the body file with the CURRENT time so downstream
	// consumers — finalize's touchFileAt, the integrity check's
	// source-vs-secondary mtime comparison, file listings in the
	// admin UI — see the real time this body was produced. The
	// legacy sentinel-time behaviour here caused the integrity
	// check to silently treat every internal source as "always
	// stale" because its output mtime was stuck at 1970.
	//
	// rfc_reserved (the only static internal source today) still
	// hits the same-body hash shortcut above before reaching
	// this branch on every tick after the first, so its output
	// file keeps whatever mtime the first-ever generation set
	// and does not churn on subsequent ticks.
	if err := os.Chtimes(tmpPath, now, now); err != nil {
		// Best-effort: a failed chtimes only means the final
		// file inherits the temp file's natural create time,
		// which is still a real timestamp rather than 1970.
		// Not worth failing the fetch for.
		_ = err
	}
}

// hashBytes wraps the SHA-256 helper already used by Fetch so the
// internal path produces identical hashes.
func hashBytes(body []byte) string {
	return sha256Hex(body)
}
