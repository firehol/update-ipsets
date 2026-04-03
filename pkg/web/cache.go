package web

import (
	"bytes"
	"container/list"
	"crypto/sha1"
	"encoding/hex"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	defaultWebArtifactCacheMaxEntries   = 2048
	defaultWebArtifactCacheMaxBytes     = 64 << 20
	defaultWebArtifactCacheMaxFileBytes = 8 << 20
)

type cachedFile struct {
	modTime     time.Time
	size        int64
	contentType string
	etag        string
	data        []byte
	elem        *list.Element
}

type fileCache struct {
	mu           sync.Mutex
	files        map[string]*cachedFile
	order        *list.List
	maxEntries   int
	maxBytes     int64
	maxFileBytes int64
	bytes        int64
}

type fileCacheLimits struct {
	MaxEntries   int
	MaxBytes     int64
	MaxFileBytes int64
}

func newFileCache() *fileCache {
	return newFileCacheWithLimits(fileCacheLimits{})
}

func newFileCacheWithLimits(limits fileCacheLimits) *fileCache {
	limits = normalizeFileCacheLimits(limits)
	return &fileCache{
		files:        map[string]*cachedFile{},
		order:        list.New(),
		maxEntries:   limits.MaxEntries,
		maxBytes:     limits.MaxBytes,
		maxFileBytes: limits.MaxFileBytes,
	}
}

func normalizeFileCacheLimits(limits fileCacheLimits) fileCacheLimits {
	if limits.MaxEntries <= 0 {
		limits.MaxEntries = defaultWebArtifactCacheMaxEntries
	}
	if limits.MaxBytes <= 0 {
		limits.MaxBytes = defaultWebArtifactCacheMaxBytes
	}
	if limits.MaxFileBytes <= 0 {
		limits.MaxFileBytes = defaultWebArtifactCacheMaxFileBytes
	}
	if limits.MaxFileBytes > limits.MaxBytes {
		limits.MaxFileBytes = limits.MaxBytes
	}
	return limits
}

func (c *fileCache) ServeFile(w http.ResponseWriter, r *http.Request, path string, contentType string) bool {
	entry, ok, err := c.load(path, contentType)
	if err != nil {
		return false
	}
	if !ok {
		return serveUncachedFile(w, r, path, contentType)
	}
	serveCachedFile(w, r, path, entry)
	return true
}

func (c *fileCache) ServeRootedFile(w http.ResponseWriter, r *http.Request, rootDir, rel, contentType string) bool {
	entry, ok, key, err := c.loadRooted(rootDir, rel, contentType)
	if err != nil {
		return false
	}
	if !ok {
		return serveUncachedRootedFile(w, r, rootDir, rel, contentType)
	}
	serveCachedFile(w, r, key, entry)
	return true
}

func serveCachedFile(w http.ResponseWriter, r *http.Request, path string, entry cachedFile) {
	if match := r.Header.Get("If-None-Match"); match != "" && match == entry.etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if modified := r.Header.Get("If-Modified-Since"); modified != "" {
		if t, err := http.ParseTime(modified); err == nil && !entry.modTime.After(t) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	if entry.contentType != "" {
		w.Header().Set("Content-Type", entry.contentType)
	}
	w.Header().Set("ETag", entry.etag)
	w.Header().Set("Last-Modified", entry.modTime.UTC().Format(http.TimeFormat))
	w.Header().Set("Cache-Control", "public, max-age=300")
	http.ServeContent(w, r, filepath.Base(path), entry.modTime, bytes.NewReader(entry.data))
}

func (c *fileCache) load(path string, contentType string) (cachedFile, bool, error) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return cachedFile{}, false, err
	}
	if info.Size() > c.maxFileBytes {
		c.remove(path)
		return cachedFile{}, false, nil
	}

	c.mu.Lock()
	entry, ok := c.files[path]
	if ok && entry.modTime.Equal(info.ModTime()) && entry.size == info.Size() {
		c.order.MoveToFront(entry.elem)
		out := *entry
		c.mu.Unlock()
		return out, true, nil
	}
	if ok {
		c.removeLocked(path, entry)
	}
	c.mu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return cachedFile{}, false, err
	}
	if int64(len(data)) > c.maxFileBytes {
		return cachedFile{}, false, nil
	}
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(path))
	}
	sum := sha1.Sum(data)
	newEntry := &cachedFile{
		modTime:     info.ModTime(),
		size:        int64(len(data)),
		contentType: contentType,
		etag:        `"` + hex.EncodeToString(sum[:]) + `"`,
		data:        data,
	}
	c.mu.Lock()
	newEntry.elem = c.order.PushFront(path)
	c.files[path] = newEntry
	c.bytes += newEntry.size
	c.evictLocked()
	out := *newEntry
	c.mu.Unlock()
	return out, true, nil
}

func (c *fileCache) loadRooted(rootDir, rel string, contentType string) (cachedFile, bool, string, error) {
	cleanRel, ok := cleanRootedRel(rel)
	if !ok {
		return cachedFile{}, false, "", os.ErrInvalid
	}
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return cachedFile{}, false, "", err
	}
	defer func() { _ = root.Close() }()

	info, err := root.Stat(cleanRel)
	if err != nil || info.IsDir() {
		return cachedFile{}, false, "", err
	}
	key := filepath.Join(filepath.Clean(rootDir), cleanRel)
	if info.Size() > c.maxFileBytes {
		c.remove(key)
		return cachedFile{}, false, key, nil
	}

	c.mu.Lock()
	entry, ok := c.files[key]
	if ok && entry.modTime.Equal(info.ModTime()) && entry.size == info.Size() {
		c.order.MoveToFront(entry.elem)
		out := *entry
		c.mu.Unlock()
		return out, true, key, nil
	}
	if ok {
		c.removeLocked(key, entry)
	}
	c.mu.Unlock()

	data, err := root.ReadFile(cleanRel)
	if err != nil {
		return cachedFile{}, false, key, err
	}
	if int64(len(data)) > c.maxFileBytes {
		return cachedFile{}, false, key, nil
	}
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(cleanRel))
	}
	sum := sha1.Sum(data)
	newEntry := &cachedFile{
		modTime:     info.ModTime(),
		size:        int64(len(data)),
		contentType: contentType,
		etag:        `"` + hex.EncodeToString(sum[:]) + `"`,
		data:        data,
	}
	c.mu.Lock()
	newEntry.elem = c.order.PushFront(key)
	c.files[key] = newEntry
	c.bytes += newEntry.size
	c.evictLocked()
	out := *newEntry
	c.mu.Unlock()
	return out, true, key, nil
}

func (c *fileCache) remove(path string) {
	c.mu.Lock()
	if entry, ok := c.files[path]; ok {
		c.removeLocked(path, entry)
	}
	c.mu.Unlock()
}

func (c *fileCache) removeLocked(path string, entry *cachedFile) {
	delete(c.files, path)
	if entry != nil {
		c.bytes -= entry.size
		if entry.elem != nil {
			c.order.Remove(entry.elem)
		}
	}
	if c.bytes < 0 {
		c.bytes = 0
	}
}

func (c *fileCache) evictLocked() {
	for (len(c.files) > c.maxEntries || c.bytes > c.maxBytes) && c.order.Len() > 0 {
		back := c.order.Back()
		path, _ := back.Value.(string)
		entry := c.files[path]
		c.removeLocked(path, entry)
	}
}

func serveUncachedFile(w http.ResponseWriter, r *http.Request, path string, contentType string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		return false
	}
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(path))
	}
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Last-Modified", info.ModTime().UTC().Format(http.TimeFormat))
	w.Header().Set("Cache-Control", "public, max-age=300")
	http.ServeContent(w, r, filepath.Base(path), info.ModTime(), file)
	return true
}

func serveUncachedRootedFile(w http.ResponseWriter, r *http.Request, rootDir, rel, contentType string) bool {
	cleanRel, ok := cleanRootedRel(rel)
	if !ok {
		return false
	}
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return false
	}
	defer func() { _ = root.Close() }()
	file, err := root.Open(cleanRel)
	if err != nil {
		return false
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		return false
	}
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(cleanRel))
	}
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Last-Modified", info.ModTime().UTC().Format(http.TimeFormat))
	w.Header().Set("Cache-Control", "public, max-age=300")
	http.ServeContent(w, r, filepath.Base(cleanRel), info.ModTime(), file)
	return true
}
