package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/engine"
)

// ManifestFile describes one file the pipeline should be
// maintaining for a feed, together with its actual on-disk
// state. The admin console renders a table of these so the
// operator can verify, at a glance, that every expected output
// exists, is the right size, and is fresher than the feed's
// last ProcessedDate.
//
// Required marks files that MUST exist after a settled pipeline run; missing
// required files are repair signals. Optional files (source enable markers,
// raw retained inputs, setinfo, and downloader history snapshots) are reported
// when present but do not count as "missing" when absent.
type ManifestFile struct {
	// Rel is the file's path relative to the daemon root so the
	// UI can render it without leaking absolute paths.
	Rel string `json:"rel"`
	// Absolute path for operators who want to grep / ls the
	// file from a shell.
	Path string `json:"path"`
	// Kind classifies the file's role in the pipeline.
	//   enabled  — per-source enable marker
	//   raw_source — retained downloader/materialized raw local input (.source)
	//   canonical — committed canonical feed body (.ipset / .netset)
	//   provider_source — committed provider archive/body under lib/
	//   setinfo  — one-line metadata sidecar
	//   metadata — per-feed JSON served by the website
	//   history  — public last-N CSV of sizes over time
	//   changesets — public last-N CSV of added/removed IPs
	//   retention — public retention histogram JSON
	//   comparison — pairwise overlap with other feeds
	//   insights — computed insight rules output
	//   geo      — per-geo-provider country distribution JSON
	//   asn      — per-asn-provider ASN distribution JSON
	//   bogons   — per-bogon-provider overlap JSON
	//   binary   — mmap-backed latest
	//   history_snapshot — downloader-owned timestamped history snapshot for history derivatives
	Kind     string `json:"kind"`
	Provider string `json:"provider,omitempty"`
	Required bool   `json:"required"`
	Exists   bool   `json:"exists"`
	Size     int64  `json:"size,omitempty"`
	MTime    int64  `json:"mtime,omitempty"`
	// Stale is true when the file exists but its mtime is
	// strictly before the feed's ProcessedDate. Only meaningful
	// for files the fan-out writers stamp; raw/canonical/provider/binary files
	// often carry upstream Last-Modified mtimes and are never
	// marked stale here.
	Stale bool `json:"stale,omitempty"`
}

// ManifestSummary rolls up the per-file results into counts the
// UI can display without re-iterating the array.
type ManifestSummary struct {
	Total    int `json:"total"`
	Present  int `json:"present"`
	Missing  int `json:"missing"`
	Stale    int `json:"stale"`
	Required int `json:"required"`
}

// ManifestResponse is the envelope returned by
// GET /api/v1/admin/feeds/{name}/manifest.
type ManifestResponse struct {
	Feed          string          `json:"feed"`
	ProcessedDate int64           `json:"processed_date"`
	Files         []ManifestFile  `json:"files"`
	Summary       ManifestSummary `json:"summary"`
}

// handleAdminFeedManifest serves the file manifest for a single
// feed. It enumerates every file the pipeline should be
// maintaining for that feed based on the catalog configuration
// (source kind, enabled geo/asn/bogon providers, configured
// retention windows) and stat()s each one to report its actual
// state.
//
// This is the evidence the operator uses to verify that the
// pipeline actually produced what it claims. A visible
// "20/20 present" tells them everything is fine; a "17/20 with
// 3 missing" tells them exactly what to reprocess.
func handleAdminFeedManifest(eng *engine.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiNoCache(w)
		if !isReadMethod(r.Method) {
			writeReadMethodNotAllowed(w)
			return
		}
		// Path is /api/v1/admin/feeds/{name}/manifest — strip the
		// prefix and the trailing /manifest.
		rest := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/feeds/")
		name := strings.TrimSuffix(rest, "/manifest")
		if name == "" || strings.Contains(name, "/") {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing feed name"})
			return
		}
		if !validFeedName(name) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid feed name"})
			return
		}

		cfg := eng.Config()
		src, ok := cfg.Sources[name]
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown feed: " + name})
			return
		}
		if src.Name == "" {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "feed configuration is missing a canonical name"})
			return
		}
		rt := eng.Runtime()
		resp := buildFeedManifest(src.Name, src, cfg, rt, eng)
		writeJSON(w, http.StatusOK, resp)
	}
}

// buildFeedManifest enumerates every expected file for a feed
// and stat()s it to determine present / missing / stale status.
// The enumeration is derived from the catalog configuration, so
// it stays in sync automatically when providers are added,
// removed, or renamed.
func buildFeedManifest(name string, src *config.Source, cfg *config.Config, rt engine.Runtime, eng *engine.Engine) ManifestResponse {
	return newFeedManifestBuilder(name, src, cfg, rt, eng).build()
}

// statManifestFile fills in the runtime state of a manifest
// entry: whether the file exists, its size, its mtime, and
// whether that mtime is strictly before the reference
// ProcessedDate (which marks it as stale relative to the last
// successful run of the heavy fan-out block).
func statManifestFile(mf ManifestFile, processedDate int64) ManifestFile {
	info, err := os.Stat(mf.Path)
	if err != nil {
		mf.Exists = false
		return mf
	}
	mf.Exists = true
	mf.Size = info.Size()
	mf.MTime = info.ModTime().Unix()
	// Only secondary files generated by the fan-out writers are
	// checked for staleness against ProcessedDate. Feed bodies,
	// provider archives, binaries, enable markers, and history snapshots
	// carry acquisition or batch mtimes and are never marked stale
	// by this check.
	switch mf.Kind {
	case "metadata", "history", "changesets", "retention", "comparison", "insights", "geo", "asn", "bogons":
		if processedDate > 0 && mf.MTime < processedDate {
			mf.Stale = true
		}
	}
	return mf
}

// daemonRoot returns a best-effort guess at the daemon's root
// directory so we can render paths as e.g. "data/dshield.netset"
// instead of "/opt/update-ipsets/data/dshield.netset" in the
// response payload. The fallback is the baseDir itself, which
// makes the rel path identical to path for the file types that
// land in baseDir.
func daemonRoot(baseDir string) string {
	// Strip a trailing /data if present.
	root := filepath.Dir(baseDir)
	if filepath.Base(baseDir) != "data" {
		root = baseDir
	}
	return root
}

// relOrPath computes a relative path from root to target, or
// returns target unchanged on error.
func relOrPath(root, target string) string {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return target
	}
	return rel
}
