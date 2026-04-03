package web

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
	"sync"
)

var gzipPool = sync.Pool{
	New: func() any { return gzip.NewWriter(nil) },
}

func gzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(strings.ToLower(r.Header.Get("Accept-Encoding")), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/"),
			strings.HasPrefix(r.URL.Path, "/static/"),
			strings.HasSuffix(r.URL.Path, ".json"),
			strings.HasSuffix(r.URL.Path, ".xml"),
			strings.HasSuffix(r.URL.Path, ".txt"),
			strings.HasSuffix(r.URL.Path, ".csv"),
			strings.HasSuffix(r.URL.Path, ".js"),
			strings.HasSuffix(r.URL.Path, ".css"),
			strings.HasSuffix(r.URL.Path, ".html"),
			r.URL.Path == "/":
		default:
			next.ServeHTTP(w, r)
			return
		}
		gz := gzipPool.Get().(*gzip.Writer)
		gz.Reset(w)
		defer func() {
			_ = gz.Close()
			gzipPool.Put(gz)
		}()
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Add("Vary", "Accept-Encoding")
		next.ServeHTTP(&gzipResponseWriter{ResponseWriter: w, Writer: gz}, r)
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/admin/") && !strings.HasPrefix(r.URL.Path, "/admin") {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			if strings.HasPrefix(r.URL.Path, "/mcp") {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Mcp-Session-Id, MCP-Protocol-Version, Last-Event-ID")
				w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id")
			} else {
				w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			}
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isReadMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead
}

func writeReadMethodNotAllowed(w http.ResponseWriter) {
	w.Header().Set("Allow", http.MethodGet+", "+http.MethodHead)
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET or HEAD"})
}

type gzipResponseWriter struct {
	http.ResponseWriter
	io.Writer
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	w.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(code)
}

func (w *gzipResponseWriter) Write(data []byte) (int, error) {
	return w.Writer.Write(data)
}

func (w *gzipResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		_ = w.Writer.(*gzip.Writer).Flush()
		f.Flush()
	}
}

func parseList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func writePlain(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func apiNoCache(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}

func serveEmbeddedIndex(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = io.WriteString(w, body)
}

func outputDirFromOptions(baseDir, webDir string) string {
	if webDir != "" {
		return webDir
	}
	return baseDir
}

func filesDir(baseDir, webDirForIPSets string) string {
	if webDirForIPSets != "" {
		return webDirForIPSets
	}
	return baseDir
}

func choose(preferred, fallback string) string {
	if preferred != "" {
		return preferred
	}
	return fallback
}

func jsonError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func plainError(w http.ResponseWriter, status int, msg string) {
	http.Error(w, msg, status)
}

func cacheSuffixRel(name, suffix string) string {
	return fmt.Sprintf("%s%s", name, suffix)
}

// validFeedName returns true if the name is a safe ipset/feed identifier.
// It rejects empty strings, path separators, path traversal sequences,
// commas, null bytes, and any non-printable or non-ASCII characters.
func validFeedName(name string) bool {
	if name == "" || len(name) > 256 {
		return false
	}
	for _, r := range name {
		switch {
		case r == '/' || r == '\\' || r == ',' || r == '\x00':
			return false
		case r < 0x20: // control characters
			return false
		case r > 0x7e: // non-ASCII
			return false
		}
	}
	// Reject path traversal components even without separators.
	if name == "." || name == ".." || strings.HasPrefix(name, "../") || strings.HasPrefix(name, "..\\") {
		return false
	}
	return true
}

func serveRawFeedRel(w http.ResponseWriter, r *http.Request, rel string, dirs ...string) bool {
	for _, dir := range dirs {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		// Raw ipset/netset bodies can be large. Stream them from a rooted
		// file descriptor instead of storing them in the long-lived
		// JSON/static artifact cache.
		if serveUncachedRootedFile(w, r, dir, rel, "text/plain; charset=utf-8") {
			return true
		}
	}
	return false
}

func rootedRegularFileStatus(rootDir, rel string) (exists bool, readable bool) {
	cleanRel, ok := cleanRootedRel(rel)
	if !ok {
		return true, false
	}
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return true, false
	}
	defer func() { _ = root.Close() }()
	info, err := root.Stat(cleanRel)
	if err != nil {
		if os.IsNotExist(err) {
			return false, false
		}
		return true, false
	}
	return !info.IsDir(), !info.IsDir()
}

// safePath joins dir and a filename, then verifies the result stays under dir.
// Returns the joined path and true if safe, or ("", false) if traversal detected.
func safePath(dir, name string) (string, bool) {
	joined := filepath.Join(dir, name)
	cleaned := filepath.Clean(joined)
	// Ensure the cleaned path is within dir.
	if !strings.HasPrefix(cleaned, filepath.Clean(dir)+string(filepath.Separator)) && cleaned != filepath.Clean(dir) {
		return "", false
	}
	return cleaned, true
}

func cleanRootedRel(name string) (string, bool) {
	if strings.TrimSpace(name) == "" || strings.Contains(name, "\x00") || strings.Contains(name, "\\") {
		return "", false
	}
	if strings.HasPrefix(name, "/") || filepath.IsAbs(name) {
		return "", false
	}
	clean := pathpkg.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	return filepath.FromSlash(clean), true
}
