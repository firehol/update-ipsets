package web

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
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
// Required marks files that MUST exist — missing required files
// are a pipeline bug. Optional files (source enable markers, raw retained
// inputs, setinfo, downloader history snapshots, bogon/geo/asn per-provider files that depend on fan-out
// configuration) are reported when present but do not count as
// "missing" when absent.
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
	resp := ManifestResponse{Feed: name}

	// ProcessedDate is pulled from the cache and used as the
	// staleness reference for secondary files. Files older than
	// ProcessedDate were written by a previous pipeline run and
	// should have been regenerated during the most recent one.
	if entries := eng.EntriesSnapshot(); len(entries) > 0 {
		for i := range entries {
			if entries[i].Name == name {
				resp.ProcessedDate = entries[i].ProcessedDate
				break
			}
		}
	}

	baseDir := rt.BaseDir
	webDir := rt.WebDir
	if webDir == "" {
		webDir = baseDir
	}
	libDir := rt.LibDir

	isDatabase := src.HasUse(config.UseASN) || src.HasUse(config.UseGeoIP)
	enablePath := filepath.Join(baseDir, name+".enabled")
	rawSourcePath := filepath.Join(baseDir, name+".source")
	canonicalPath := eng.FeedBodyPath(name)
	providerPath := ""
	if isDatabase {
		if src.HasUse(config.UseASN) {
			providerPath = filepath.Join(libDir, "asn", name, "source")
		} else {
			providerPath = filepath.Join(libDir, "geolocation", name+".source")
		}
	}

	add := func(mf ManifestFile) {
		resp.Files = append(resp.Files, statManifestFile(mf, resp.ProcessedDate))
	}

	// --- Base files in baseDir ---

	// Stable source enable marker. This is separate from any raw or
	// canonical feed-body file so operators can disable a feed without
	// mutating downloader/debug state.
	add(ManifestFile{
		Rel:      relOrPath(daemonRoot(baseDir), enablePath),
		Path:     enablePath,
		Kind:     "enabled",
		Required: false,
	})

	if isDatabase {
		add(ManifestFile{
			Rel:      relOrPath(daemonRoot(baseDir), providerPath),
			Path:     providerPath,
			Kind:     "provider_source",
			Required: true,
		})
	} else {
		// Direct downloads and artifact-backed children retain a raw
		// local input; pure synthetic feeds do not.
		if src.URL != "" && src.Provenance != config.ProvenanceSecondaryRetention && src.Provenance != config.ProvenanceSecondaryMerge {
			add(ManifestFile{
				Rel:      relOrPath(daemonRoot(baseDir), rawSourcePath),
				Path:     rawSourcePath,
				Kind:     "raw_source",
				Required: false,
			})
		}
		add(ManifestFile{
			Rel:      relOrPath(daemonRoot(baseDir), canonicalPath),
			Path:     canonicalPath,
			Kind:     "canonical",
			Required: true,
		})
	}

	// One-line metadata sidecar.
	add(ManifestFile{
		Rel:      relOrPath(daemonRoot(baseDir), filepath.Join(baseDir, name+".setinfo")),
		Path:     filepath.Join(baseDir, name+".setinfo"),
		Kind:     "setinfo",
		Required: false,
	})

	// --- Web dir secondary files (only for non-database feeds) ---

	if !isDatabase {
		add(ManifestFile{
			Rel:      relOrPath(daemonRoot(baseDir), filepath.Join(webDir, name+".json")),
			Path:     filepath.Join(webDir, name+".json"),
			Kind:     "metadata",
			Required: true,
		})
		add(ManifestFile{
			Rel:      relOrPath(daemonRoot(baseDir), filepath.Join(webDir, name+"_history.csv")),
			Path:     filepath.Join(webDir, name+"_history.csv"),
			Kind:     "history",
			Required: true,
		})
		add(ManifestFile{
			Rel:      relOrPath(daemonRoot(baseDir), filepath.Join(webDir, name+"_changesets.csv")),
			Path:     filepath.Join(webDir, name+"_changesets.csv"),
			Kind:     "changesets",
			Required: true,
		})
		add(ManifestFile{
			Rel:      relOrPath(daemonRoot(baseDir), filepath.Join(webDir, name+"_retention.json")),
			Path:     filepath.Join(webDir, name+"_retention.json"),
			Kind:     "retention",
			Required: true,
		})
		add(ManifestFile{
			Rel:      relOrPath(daemonRoot(baseDir), filepath.Join(webDir, name+"_comparison.json")),
			Path:     filepath.Join(webDir, name+"_comparison.json"),
			Kind:     "comparison",
			Required: true,
		})
		add(ManifestFile{
			Rel:      relOrPath(daemonRoot(baseDir), filepath.Join(webDir, name+"_insights.json")),
			Path:     filepath.Join(webDir, name+"_insights.json"),
			Kind:     "insights",
			Required: true,
		})

		// Per-provider fan-out files. One entry per configured
		// provider — this keeps the manifest in sync with the
		// catalog automatically when providers are added or
		// removed.
		for _, p := range cfg.SourcesWithUse(config.UseGeoIP) {
			add(ManifestFile{
				Rel:      relOrPath(daemonRoot(baseDir), filepath.Join(webDir, name+"_"+p.Name+".json")),
				Path:     filepath.Join(webDir, name+"_"+p.Name+".json"),
				Kind:     "geo",
				Provider: p.Name,
				Required: true,
			})
		}
		for _, p := range cfg.SourcesWithUse(config.UseASN) {
			add(ManifestFile{
				Rel:      relOrPath(daemonRoot(baseDir), filepath.Join(webDir, name+"_asn_"+p.Name+".json")),
				Path:     filepath.Join(webDir, name+"_asn_"+p.Name+".json"),
				Kind:     "asn",
				Provider: p.Name,
				Required: true,
			})
		}
		for _, p := range cfg.SourcesWithUse(config.UseBogons) {
			add(ManifestFile{
				Rel:      relOrPath(daemonRoot(baseDir), filepath.Join(webDir, name+"_bogons_"+p.Name+".json")),
				Path:     filepath.Join(webDir, name+"_bogons_"+p.Name+".json"),
				Kind:     "bogons",
				Provider: p.Name,
				Required: true,
			})
		}
	}

	// --- Binary state in libDir ---

	if libDir != "" && !isDatabase {
		add(ManifestFile{
			Rel:      relOrPath(daemonRoot(baseDir), filepath.Join(libDir, name, "latest")),
			Path:     filepath.Join(libDir, name, "latest"),
			Kind:     "binary",
			Required: true,
		})

		// Downloader-owned history snapshots for history derivatives are
		// stored under runtime.HistoryDir. They are optional at the
		// manifest level because only feeds that participate in
		// history-derivative composition maintain them.
		rollupDir := filepath.Join(rt.HistoryDir, name)
		if ents, err := os.ReadDir(rollupDir); err == nil {
			names := make([]string, 0, len(ents))
			for _, ent := range ents {
				if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".set") {
					continue
				}
				names = append(names, ent.Name())
			}
			sort.Sort(sort.Reverse(sort.StringSlice(names)))
			const maxRollups = 20
			for i, ent := range names {
				if i >= maxRollups {
					break
				}
				add(ManifestFile{
					Rel:      relOrPath(daemonRoot(baseDir), filepath.Join(rollupDir, ent)),
					Path:     filepath.Join(rollupDir, ent),
					Kind:     "history_snapshot",
					Required: false,
				})
			}
		}
	}

	// --- Summary ---

	for i := range resp.Files {
		if resp.Files[i].Required {
			resp.Summary.Required++
		}
		if resp.Files[i].Exists {
			resp.Summary.Present++
		} else if resp.Files[i].Required {
			resp.Summary.Missing++
		}
		if resp.Files[i].Stale {
			resp.Summary.Stale++
		}
	}
	resp.Summary.Total = len(resp.Files)
	return resp
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
