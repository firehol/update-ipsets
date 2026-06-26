package web

import (
	_ "embed"
	"net/http"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/feedhealth"
	"github.com/firehol/update-ipsets/pkg/runreason"
	"github.com/firehol/update-ipsets/pkg/scheduler"
)

const adminDerivedScheduleLabel = "triggered by inputs"

// adminStatus is the response for GET /api/v1/admin/status.
type adminStatus struct {
	PublicBaseURL string                     `json:"public_base_url,omitempty"`
	System        adminSystemInfo            `json:"system"`
	Engine        engine.StatusSnapshot      `json:"engine"`
	Scheduler     scheduler.Snapshot         `json:"scheduler"`
	Queues        scheduler.ActivitySnapshot `json:"queues"`
	Metrics       scheduler.MetricsSnapshot  `json:"metrics"`
	Feeds         adminFeedsSummary          `json:"feeds"`
	Artifacts     []adminArtifact            `json:"artifacts,omitempty"`
}

type adminSystemInfo struct {
	Uptime             string  `json:"uptime"`
	GoVersion          string  `json:"go_version"`
	GOOS               string  `json:"goos"`
	GOARCH             string  `json:"goarch"`
	Goroutines         int     `json:"goroutines"`
	HeapAlloc          uint64  `json:"heap_alloc"`
	HeapSys            uint64  `json:"heap_sys"`
	HeapInuse          uint64  `json:"heap_inuse"`
	StackInuse         uint64  `json:"stack_inuse"`
	Sys                uint64  `json:"sys"`
	NumGC              uint32  `json:"num_gc"`
	LastGC             int64   `json:"last_gc_unix"`
	GCPauseTotal       uint64  `json:"gc_pause_total_ns"`
	DiskFree           string  `json:"disk_free"`
	RSSKB              uint64  `json:"rss_kb,omitempty"`
	VMSKB              uint64  `json:"vms_kb,omitempty"`
	DataKB             uint64  `json:"data_kb,omitempty"`
	CPUUserSeconds     float64 `json:"cpu_user_seconds,omitempty"`
	CPUSystemSeconds   float64 `json:"cpu_system_seconds,omitempty"`
	CPUTotalSeconds    float64 `json:"cpu_total_seconds,omitempty"`
	ProcReadBytes      uint64  `json:"proc_read_bytes,omitempty"`
	ProcWriteBytes     uint64  `json:"proc_write_bytes,omitempty"`
	ProcCancelledWrite uint64  `json:"proc_cancelled_write_bytes,omitempty"`
	ProcReadSyscalls   uint64  `json:"proc_read_syscalls,omitempty"`
	ProcWriteSyscalls  uint64  `json:"proc_write_syscalls,omitempty"`
	OpenFDs            int     `json:"open_fds,omitempty"`
}

// adminFeedsSummary is the aggregate feed health rollup the
// dashboard shows in its heartbeat strip. Each breakdown count
// lets the operator click to filter the feeds table to exactly
// that subset.
//
// Health counts come from the shared backend feed-health classifier.
// Hidden is a visibility flag, not an operational exclusion, so hidden
// feeds still contribute to enabled / health counters.
// Running / disabled / hidden remain operational counters.
type adminFeedsSummary struct {
	TotalConfigured int    `json:"total_configured"`
	TotalEnabled    int    `json:"total_enabled"`
	TotalEntries    int    `json:"total_entries"`
	TotalUniqueIPs  uint64 `json:"total_unique_ips"`
	Delayed         int    `json:"delayed"`
	Risky           int    `json:"risky"`
	Unavailable     int    `json:"unavailable"`
	Archived        int    `json:"archived"`
	Empty           int    `json:"empty"`
	Unmaintained    int    `json:"unmaintained"`
	Healthy         int    `json:"healthy"`
	Stale           int    `json:"stale"`
	Errors          int    `json:"errors"`
	Running         int    `json:"running"`
	NeverRun        int    `json:"never_run"`
	Disabled        int    `json:"disabled"`
	// Hidden feeds are counted separately, but they still contribute to
	// the operational totals above because hidden only affects public
	// visibility, not scheduling or processing.
	Hidden int `json:"hidden"`
}

// adminFeed is a single feed in the feeds list response.
//
// The shape is deliberately broad so the admin UI can render the
// whole operator lifecycle in the list view without a second
// fetch to /feeds/{name} for every row. Four distinct timestamps
// exist and mean different things — get them right or the
// operator UX lies:
//
//	LastCheck (= cache.Entry.CheckedDate)     → the last time we
//	    asked upstream "has this changed?" Always moves forward on
//	    every tick, even on 304 / StatusSame / not-modified.
//	LastUpdate (= cache.Entry.SourceDate)     → the upstream's own
//	    "this is when the content last changed" timestamp (HTTP
//	    Last-Modified header). Can be in the future if the upstream
//	    clock is skewed or forward-stamps publications. Never trust
//	    it for scheduling decisions.
//	ProcessedDate (= cache.Entry.ProcessedDate) → the wall clock at
//	    which finalize() last ran for this feed. This is the
//	    authoritative "when did we actually produce new output"
//	    timestamp, and the only one the integrity check trusts. If
//	    this and LastUpdate diverge, the upstream is lying about
//	    Last-Modified but we still caught fresh content.
//	NextCheck (= scheduler.Item.NextDue)      → the scheduler's own
//	    "when should I look again" time. Can be in the past, meaning
//	    "overdue, will run on next tick".
//
// Version is the processing iteration count — it increments every
// time finalize() writes a new output. Two feeds with the same
// ProcessedDate but different Version values tell you one kept
// getting fresh content and the other is stuck on a stale set that
// happens to still get revalidated.
type adminFeed struct {
	Name             string              `json:"name"`
	Kind             string              `json:"kind"`
	Uses             []string            `json:"uses,omitempty"`
	Category         string              `json:"category"`
	Hidden           bool                `json:"hidden,omitempty"`
	Enabled          bool                `json:"enabled"`
	Status           string              `json:"status"`
	Health           feedhealth.Snapshot `json:"health"`
	LastStatus       string              `json:"last_status"`
	LastStatusLabel  string              `json:"last_status_label,omitempty"`
	LastRunReason    string              `json:"last_run_reason,omitempty"`
	LastProcessingMS int64               `json:"last_processing_ms,omitempty"`
	LastError        string              `json:"last_error,omitempty"`
	LastProblemClass adminProblemClass   `json:"last_problem_class,omitempty"`
	LastCheck        int64               `json:"last_check"`
	LastUpdate       int64               `json:"last_update"`
	ProcessedDate    int64               `json:"processed_date"`
	StartedDate      int64               `json:"started_date"`
	NextCheck        int64               `json:"next_check"`
	ClockSkewSeconds int64               `json:"clock_skew_seconds,omitempty"`
	Entries          int                 `json:"entries"`
	EntriesMin       int                 `json:"entries_min,omitempty"`
	EntriesMax       int                 `json:"entries_max,omitempty"`
	UniqueIPs        uint64              `json:"unique_ips"`
	IPsMin           uint64              `json:"ips_min,omitempty"`
	IPsMax           uint64              `json:"ips_max,omitempty"`
	Version          int                 `json:"version,omitempty"`
	AvgUpdateMins    int                 `json:"avg_update_mins,omitempty"`
	MinUpdateMins    int                 `json:"min_update_mins,omitempty"`
	MaxUpdateMins    int                 `json:"max_update_mins,omitempty"`
	DownloadFailures int                 `json:"download_failures"`
	FrequencyMinutes int                 `json:"frequency_minutes"`
	// SchedulerDetail is the scheduler's own human-readable reason
	// for why the feed is due / not due right now. Examples:
	//   "never checked"
	//   "12/30 mins passed, next check in 18 mins (base 30 mins)"
	//   "next check in 150 mins (base 30 mins)"  ← backoff kicked in
	//   "due now"
	// Captures the current retry / backoff state of a failing feed
	// without needing to recompute it in the UI.
	SchedulerDetail   string `json:"scheduler_detail,omitempty"`
	URL               string `json:"url,omitempty"`
	PublicURL         string `json:"public_url,omitempty"`
	IPV               string `json:"ipv,omitempty"`
	Hash              string `json:"hash,omitempty"`
	Output            string `json:"output,omitempty"`
	ProcessorRaw      string `json:"processor_raw,omitempty"`
	Downloader        string `json:"downloader,omitempty"`
	DownloaderOptions string `json:"downloader_options,omitempty"`
	HistoryMinutes    []int  `json:"history_minutes,omitempty"`
	AcceptEmpty       bool   `json:"accept_empty,omitempty"`
	Maintainer        string `json:"maintainer,omitempty"`
	MaintainerURL     string `json:"maintainer_url,omitempty"`
	Info              string `json:"info,omitempty"`
	License           string `json:"license,omitempty"`
	Attribution       string `json:"attribution,omitempty"`
	Redistributable   bool   `json:"redistributable"`
	// Files that the cache currently thinks are on disk. Not the
	// full manifest — that comes from the /manifest endpoint.
	File   string `json:"file,omitempty"`
	Source string `json:"source,omitempty"`
	// DerivedFrom lists the parent sources for derivatives
	// (merges, retention windows). Empty for plain HTTP sources.
	DerivedFrom     []string                 `json:"derived_from,omitempty"`
	MergeIncluded   []engine.MergeInputState `json:"merge_included,omitempty"`
	MergeSubtracted []engine.MergeInputState `json:"merge_subtracted,omitempty"`
	MergeExcluded   []engine.MergeInputState `json:"merge_excluded,omitempty"`
}

// adminFeedDetail extends adminFeed with extra metadata for the detail view.
type adminFeedDetail struct {
	adminFeed
	File          string   `json:"file,omitempty"`
	Hash          string   `json:"hash,omitempty"`
	History       []int    `json:"history_minutes,omitempty"`
	EntriesMin    int      `json:"entries_min"`
	EntriesMax    int      `json:"entries_max"`
	IPsMin        uint64   `json:"ips_min"`
	IPsMax        uint64   `json:"ips_max"`
	AvgUpdateMins int      `json:"average_update_mins"`
	MinUpdateMins int      `json:"min_update_mins"`
	MaxUpdateMins int      `json:"max_update_mins"`
	MergeSources  []string `json:"merge_sources,omitempty"`
}

type adminArtifact struct {
	Name             string            `json:"name"`
	Type             string            `json:"type"`
	Enabled          bool              `json:"enabled"`
	Status           string            `json:"status"`
	LastStatus       string            `json:"last_status"`
	LastStatusLabel  string            `json:"last_status_label,omitempty"`
	LastError        string            `json:"last_error,omitempty"`
	LastProblemClass adminProblemClass `json:"last_problem_class,omitempty"`
	LastCheck        int64             `json:"last_check"`
	LastUpdate       int64             `json:"last_update"`
	NextCheck        int64             `json:"next_check"`
	FrequencyMinutes int               `json:"frequency_minutes"`
	DownloadFailures int               `json:"download_failures"`
	SchedulerDetail  string            `json:"scheduler_detail,omitempty"`
	Info             string            `json:"info,omitempty"`
	Maintainer       string            `json:"maintainer,omitempty"`
	MaintainerURL    string            `json:"maintainer_url,omitempty"`
	ChildFeeds       []string          `json:"child_feeds,omitempty"`
}

// adminScheduleEntry represents one item in the schedule timeline.
type adminScheduleEntry struct {
	Name             string `json:"name"`
	Kind             string `json:"kind"`
	Enabled          bool   `json:"enabled"`
	NextDue          int64  `json:"next_due"`
	LastCheck        int64  `json:"last_check"`
	FrequencyMinutes int    `json:"frequency_minutes"`
	AvgUpdateMins    int    `json:"avg_update_mins"`
	MinUpdateMins    int    `json:"min_update_mins"`
	MaxUpdateMins    int    `json:"max_update_mins"`
	Failures         int    `json:"failures"`
	LastError        string `json:"last_error,omitempty"`
	Detail           string `json:"detail"`
}

// handleAdminPage serves the React SPA shell for /admin and /admin/*.
// The actual admin UI is the same single-page application as the
// public site — the React router decides which view to render based
// on the URL. The handler itself is wrapped by basicAuth when the
// admin surface is registered, so the operator shell and the operator
// APIs follow the same access-control boundary.
func handleAdminPage(w http.ResponseWriter, _ *http.Request) {
	serveEmbeddedIndexNoStore(w, embeddedIndex)
}

func handleAdminStatus(eng *engine.Engine, runner *scheduler.Runner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		apiNoCache(w)
		buildStarted := time.Now()
		var status any
		if r.URL.Query().Get("mode") == "full" {
			status = buildAdminStatus(eng, runner)
		} else {
			status = buildAdminStatusLight(eng, runner)
		}
		eng.ObserveOperation("http.admin_status.build", time.Since(buildStarted))
		writeStarted := time.Now()
		bytes := writeJSON(w, http.StatusOK, status)
		eng.ObserveOperation("http.admin_status.write_json", time.Since(writeStarted))
		eng.ObserveOperation("http.admin_status.total", time.Since(started))
		eng.ObserveCounter("http.admin_status", 1, int64(bytes))
	}
}

func handleAdminFeeds(eng *engine.Engine, runner *scheduler.Runner) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		started := time.Now()
		apiNoCache(w)
		buildStarted := time.Now()
		feeds := buildAdminFeeds(eng, runner)
		eng.ObserveOperation("http.admin_feeds.build", time.Since(buildStarted))
		writeStarted := time.Now()
		bytes := writeJSON(w, http.StatusOK, feeds)
		eng.ObserveOperation("http.admin_feeds.write_json", time.Since(writeStarted))
		eng.ObserveOperation("http.admin_feeds.total", time.Since(started))
		eng.ObserveCounter("http.admin_feeds", 1, int64(bytes))
	}
}

func handleAdminArtifacts(eng *engine.Engine, runner *scheduler.Runner) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		apiNoCache(w)
		writeJSON(w, http.StatusOK, buildAdminArtifacts(eng, runner))
	}
}

// handleAdminFeedsRouter dispatches /api/v1/admin/feeds/{name} and
// /api/v1/admin/feeds/{name}/{action} requests.
func handleAdminFeedsRouter(eng *engine.Engine, runner *scheduler.Runner, _ Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiNoCache(w)
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/feeds/")
		if path == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing feed name"})
			return
		}

		name, action, _ := strings.Cut(path, "/")
		if name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing feed name"})
			return
		}
		if !validFeedName(name) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid feed name"})
			return
		}

		switch action {
		case "":
			// GET /api/v1/admin/feeds/{name} — feed detail.
			if !isReadMethod(r.Method) {
				writeReadMethodNotAllowed(w)
				return
			}
			detail, err := buildAdminFeedDetail(eng, runner, name)
			if err != nil {
				jsonError(w, http.StatusNotFound, err)
				return
			}
			writeJSON(w, http.StatusOK, detail)

		case "manifest":
			// GET /api/v1/admin/feeds/{name}/manifest —
			// enumerate every expected file on disk and
			// report its actual state. Expensive: it
			// stat()s every secondary file, but only
			// runs when the operator opens a feed modal.
			handleAdminFeedManifest(eng)(w, r)

		case "recheck":
			requirePOST(w, r, func() {
				target := eng.ResolveRecheckTarget(r.Context(), name)
				if !runner.TryTriggerSources(scheduler.PendingAction{
					Names:   []string{target},
					Recheck: true,
					Reason:  runreason.ReasonManualRecheck,
				}) {
					observeAPIRecalculation(r, "admin", "feed_recheck", "conflict", 0)
					writeJSON(w, http.StatusConflict, map[string]string{"error": "scheduler action queue is full"})
					return
				}
				observeAPIRecalculation(r, "admin", "feed_recheck", "scheduled", 1)
				writeJSON(w, http.StatusAccepted, map[string]string{"status": "scheduled", "name": target, "action": "recheck"})
			})

		case "reprocess":
			requirePOST(w, r, func() {
				if !eng.HasLocalReprocessState(name) {
					observeAPIRecalculation(r, "admin", "feed_reprocess", "conflict", 0)
					writeJSON(w, http.StatusConflict, map[string]string{"error": "no staged or committed local input exists for reprocess"})
					return
				}
				if !runner.TryTriggerSources(scheduler.PendingAction{
					Names:     []string{name},
					Reprocess: true,
					Reason:    runreason.ReasonManualReprocess,
				}) {
					observeAPIRecalculation(r, "admin", "feed_reprocess", "conflict", 0)
					writeJSON(w, http.StatusConflict, map[string]string{"error": "scheduler action queue is full"})
					return
				}
				observeAPIRecalculation(r, "admin", "feed_reprocess", "scheduled", 1)
				writeJSON(w, http.StatusAccepted, map[string]string{"status": "scheduled", "name": name, "action": "reprocess"})
			})

		case "enable":
			requirePOST(w, r, func() {
				if err := eng.Enable([]string{name}, false); err != nil {
					jsonError(w, http.StatusInternalServerError, err)
					return
				}
				writeJSON(w, http.StatusOK, map[string]string{"status": "enabled", "name": name})
			})

		case "disable":
			requirePOST(w, r, func() {
				if err := eng.Disable([]string{name}, false); err != nil {
					jsonError(w, http.StatusInternalServerError, err)
					return
				}
				writeJSON(w, http.StatusOK, map[string]string{"status": "disabled", "name": name})
			})

		default:
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown action"})
		}
	}
}

func handleAdminArtifactsRouter(eng *engine.Engine, runner *scheduler.Runner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiNoCache(w)
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/artifacts/")
		if path == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing artifact name"})
			return
		}

		name, action, _ := strings.Cut(path, "/")
		if name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing artifact name"})
			return
		}
		if !validFeedName(name) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid artifact name"})
			return
		}
		if eng.Config().ArtifactByName(name) == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown artifact"})
			return
		}

		switch action {
		case "":
			if !isReadMethod(r.Method) {
				writeReadMethodNotAllowed(w)
				return
			}
			for _, artifact := range buildAdminArtifacts(eng, runner) {
				if artifact.Name == name {
					writeJSON(w, http.StatusOK, artifact)
					return
				}
			}
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown artifact"})

		case "recheck":
			requirePOST(w, r, func() {
				if !runner.TryTriggerSources(scheduler.PendingAction{
					Names:   []string{name},
					Recheck: true,
					Reason:  runreason.ReasonManualRecheck,
				}) {
					observeAPIRecalculation(r, "admin", "artifact_recheck", "conflict", 0)
					writeJSON(w, http.StatusConflict, map[string]string{"error": "scheduler action queue is full"})
					return
				}
				observeAPIRecalculation(r, "admin", "artifact_recheck", "scheduled", 1)
				writeJSON(w, http.StatusAccepted, map[string]string{"status": "scheduled", "name": name, "action": "recheck"})
			})

		case "enable":
			requirePOST(w, r, func() {
				if err := eng.EnableArtifacts([]string{name}, false); err != nil {
					jsonError(w, http.StatusInternalServerError, err)
					return
				}
				writeJSON(w, http.StatusOK, map[string]string{"status": "enabled", "name": name})
			})

		case "disable":
			requirePOST(w, r, func() {
				if err := eng.DisableArtifacts([]string{name}, false); err != nil {
					jsonError(w, http.StatusInternalServerError, err)
					return
				}
				writeJSON(w, http.StatusOK, map[string]string{"status": "disabled", "name": name})
			})

		default:
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown action"})
		}
	}
}

// requirePOST rejects non-POST methods and calls fn for POST requests.
func requirePOST(w http.ResponseWriter, r *http.Request, fn func()) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}
	fn()
}

func handleAdminSchedule(eng *engine.Engine, runner *scheduler.Runner) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		apiNoCache(w)
		snap := runner.Snapshot()

		// Index cache entries for detected update frequency and errors.
		cacheEntries := eng.EntriesSnapshot()
		cacheIdx := make(map[string]*cache.Entry, len(cacheEntries))
		for i := range cacheEntries {
			cacheIdx[cacheEntries[i].Name] = &cacheEntries[i]
		}

		entries := make([]adminScheduleEntry, 0, len(snap.Items))
		for _, item := range snap.Items {
			entry := adminScheduleEntry{
				Name:             item.Name,
				Kind:             item.Kind,
				Enabled:          item.Enabled,
				NextDue:          item.NextDue.Unix(),
				LastCheck:        item.CheckedAt.Unix(),
				FrequencyMinutes: item.FrequencyMinutes,
				Failures:         item.Failures,
				Detail:           item.Detail,
			}
			if ce, ok := cacheIdx[item.Name]; ok {
				entry.AvgUpdateMins = ce.AverageUpdateMins
				entry.MinUpdateMins = ce.MinUpdateMins
				entry.MaxUpdateMins = ce.MaxUpdateMins
				entry.LastError = ce.LastError
			}
			entries = append(entries, entry)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"generated_at": snap.GeneratedAt.Unix(),
			"items":        entries,
		})
	}
}

// buildAdminStatus constructs the full system status response.
//
// The feed health breakdown is computed here by walking the same adminFeed list the
// /admin/feeds endpoint returns, so the heartbeat strip tiles and
// the feeds-table filter chips share one derivation rule and cannot
// drift. If you change the classification logic, add a matching
// adjustment in the UI filter chip predicates or lose the invariant.
func buildAdminStatus(eng *engine.Engine, runner *scheduler.Runner) adminStatus {
	sys := detailedStatus()
	cfg := eng.Config()
	engineStatus := eng.StatusSnapshot()
	activity := runner.ActivitySnapshot()
	snapshot := runner.Snapshot()
	entriesWithArtifacts := eng.EntriesSnapshotWithArtifacts()
	feeds := buildAdminFeedsWithStatusEntries(eng, runner, activity, snapshot, entriesWithArtifacts)
	artifacts := buildAdminArtifactsWithQueuesEntries(eng, runner, activity, entriesWithArtifacts)

	summary := summarizeAdminFeeds(len(cfg.Sources), feeds)

	return adminStatus{
		PublicBaseURL: strings.TrimSpace(eng.Runtime().PublicBaseURL),
		System:        adminSystemFromDetailed(sys),
		Engine:        engineStatus,
		Scheduler:     sanitizeSchedulerSnapshot(snapshot),
		Queues:        activity,
		Metrics:       runner.MetricsSnapshot(),
		Feeds:         summary,
		Artifacts:     artifacts,
	}
}

func summarizeAdminFeeds(totalConfigured int, feeds []adminFeed) adminFeedsSummary {
	summary := adminFeedsSummary{
		// After derivative expansion every operator-visible feed family
		// lives in cfg.Sources already, so TotalConfigured is just the
		// size of that unified source map.
		TotalConfigured: totalConfigured,
	}
	for i := range feeds {
		f := &feeds[i]
		if f.Hidden {
			summary.Hidden++
		}
		summary.TotalEntries += f.Entries
		summary.TotalUniqueIPs += f.UniqueIPs
		if f.Enabled {
			summary.TotalEnabled++
		} else {
			summary.Disabled++
			continue
		}
		switch f.Health.Class {
		case feedhealth.ClassHealthy:
			summary.Healthy++
		case feedhealth.ClassDelayed:
			summary.Delayed++
		case feedhealth.ClassRisky:
			summary.Risky++
		case feedhealth.ClassUnavailable:
			summary.Unavailable++
			summary.Errors++
		case feedhealth.ClassArchived:
			summary.Archived++
		case feedhealth.ClassEmpty:
			summary.Empty++
		case feedhealth.ClassUnmaintained:
			summary.Unmaintained++
			summary.Stale++
		}
		switch {
		case f.Status == "running" || f.Status == "downloading" || f.Status == "processing":
			summary.Running++
		case f.LastCheck == 0 && f.LastUpdate == 0:
			summary.NeverRun++
		}
	}
	return summary
}

// buildAdminFeeds constructs the full feeds list for the admin
// dashboard. After source unification ASN, geoip, bogon and
// rfc_reserved are all regular sources, so we walk cfg.Sources once
// and let the kind/uses badges differentiate them in the UI.
func buildAdminFeeds(eng *engine.Engine, runner *scheduler.Runner) []adminFeed {
	activity := runner.ActivitySnapshot()
	return buildAdminFeedsWithStatus(eng, runner, activity, runner.Snapshot())
}

func buildAdminArtifacts(eng *engine.Engine, runner *scheduler.Runner) []adminArtifact {
	return buildAdminArtifactsWithQueues(eng, runner, runner.ActivitySnapshot())
}

func buildAdminArtifactsWithQueues(eng *engine.Engine, runner *scheduler.Runner, queues scheduler.ActivitySnapshot) []adminArtifact {
	return buildAdminArtifactsWithQueuesEntries(eng, runner, queues, eng.EntriesSnapshotWithArtifacts())
}

func buildAdminArtifactsWithQueuesEntries(eng *engine.Engine, runner *scheduler.Runner, queues scheduler.ActivitySnapshot, entries []cache.Entry) []adminArtifact {
	cfg := eng.Config()
	if cfg == nil || len(cfg.Artifacts) == 0 {
		return nil
	}
	entryIndex := make(map[string]*cache.Entry, len(entries))
	for i := range entries {
		entryIndex[entries[i].Name] = &entries[i]
	}
	itemIndex := make(map[string]scheduler.Item)
	for _, item := range scheduler.BuildArtifactItems(cfg, eng.Runtime(), entries, runner.EnableAll(), time.Now().UTC()) {
		itemIndex[item.Name] = item
	}
	downloadWaiting := make(map[string]bool, len(queues.DownloadWaiting))
	for _, item := range queues.DownloadWaiting {
		downloadWaiting[item.Name] = true
	}
	downloadActive := make(map[string]bool, len(queues.DownloadActive))
	for _, item := range queues.DownloadActive {
		downloadActive[item.Name] = true
	}

	artifacts := make([]adminArtifact, 0, len(cfg.Artifacts))
	for _, name := range config.SortedArtifactNames(cfg) {
		artifact := cfg.ArtifactByName(name)
		if artifact == nil {
			continue
		}
		row := adminArtifact{
			Name:             name,
			Type:             artifact.Type,
			FrequencyMinutes: artifact.Frequency,
			Info:             artifact.Info,
			Maintainer:       artifact.Maintainer,
			MaintainerURL:    artifact.MaintainerURL,
		}
		for _, child := range cfg.ArtifactChildren(name) {
			if child != nil {
				row.ChildFeeds = append(row.ChildFeeds, child.Name)
			}
		}
		if entry := entryIndex[name]; entry != nil {
			row.LastStatus = entry.LastStatus
			row.LastError = entry.LastError
			row.LastCheck = entry.CheckedDate
			row.LastUpdate = entry.SourceDate
			row.DownloadFailures = entry.DownloadFailures
		}
		if item, ok := itemIndex[name]; ok {
			row.Enabled = item.Enabled
			row.NextCheck = item.NextDue.Unix()
			row.SchedulerDetail = item.Detail
		}
		populateAdminArtifactStatusMeta(&row)
		row.Status = deriveArtifactStatus(&row, downloadWaiting[name], downloadActive[name])
		artifacts = append(artifacts, row)
	}
	return artifacts
}

func buildAdminFeedsWithStatus(eng *engine.Engine, runner *scheduler.Runner, activity scheduler.ActivitySnapshot, snap scheduler.Snapshot) []adminFeed {
	return buildAdminFeedsWithStatusEntries(eng, runner, activity, snap, eng.EntriesSnapshot())
}

func buildAdminFeedsWithStatusEntries(eng *engine.Engine, runner *scheduler.Runner, activity scheduler.ActivitySnapshot, snap scheduler.Snapshot, entries []cache.Entry) []adminFeed {
	cfg := eng.Config()
	policy := feedhealth.PolicyFromRuntime(cfg.Runtime)
	now := time.Now().UTC()
	liveStates := liveFeedStates(activity)
	enableAll := runnerEnableAll(runner)
	mergeCompositions := eng.MergeCompositions(enableAll)

	// Index cache entries and schedule items by name for O(1) lookup.
	entryIndex := make(map[string]*cache.Entry, len(entries))
	for i := range entries {
		entryIndex[entries[i].Name] = &entries[i]
	}
	schedIndex := make(map[string]*scheduler.Item, len(snap.Items))
	for i := range snap.Items {
		schedIndex[snap.Items[i].Name] = &snap.Items[i]
	}

	feeds := make([]adminFeed, 0, len(cfg.Sources))

	// After config expansion, merges and history derivatives are
	// first-class entries in cfg.Sources with populated DerivedFrom
	// lists. The legacy cfg.Merges walk is gone; buildSourceFeed copies
	// DerivedFrom into the admin feed so the UI can render lineage
	// without special casing.
	for _, name := range config.SortedSourceNames(cfg) {
		src := cfg.Sources[name]
		feeds = append(feeds, buildSourceFeed(eng, name, src, entryIndex, schedIndex, liveStates, policy, now, enableAll, mergeCompositions))
	}

	return feeds
}

// adminKindForSource picks a "kind" label that the admin UI uses for
// grouping. Kinds are the operator-facing feed families surfaced in
// the admin filter row: plain sources, derivatives, and supporting
// data providers (ASN / GeoIP / bogons). Other orthogonal attributes
// such as category, hidden, or disabled status are
// filtered separately and must not be folded into kind.
//
// Derivatives (merges, history windows) get their own kind so the admin UI can
// group them separately from plain upstream feeds without special-casing
// individual feed names.
func adminKindForSource(src *config.Source) string {
	if src == nil {
		return "source"
	}
	if src.Provenance == config.ProvenanceSecondaryMerge {
		return "merge"
	}
	if src.Provenance == config.ProvenanceSecondaryRetention {
		return "retention"
	}
	for _, role := range []string{config.UseASN, config.UseGeoIP, config.UseBogons} {
		if src.HasUse(role) {
			switch role {
			case config.UseASN:
				return "asn"
			case config.UseGeoIP:
				return "geolocation"
			case config.UseBogons:
				return "bogon"
			}
		}
	}
	return "source"
}

func buildSourceFeed(eng *engine.Engine, name string, src *config.Source, entryIdx map[string]*cache.Entry, schedIdx map[string]*scheduler.Item, liveStates map[string]adminLiveState, policy feedhealth.Policy, now time.Time, enableAll bool, mergeCompositions map[string]engine.MergeComposition) adminFeed {
	f := adminFeed{
		Name:             name,
		Kind:             adminKindForSource(src),
		Uses:             append([]string(nil), src.Use...),
		Category:         src.Category,
		Hidden:           src.Hidden,
		FrequencyMinutes: src.Frequency,
		URL:              src.URL,
		IPV:              src.IPV,
		Output:           src.Output,
		ProcessorRaw:     src.ProcessorRaw,
		HistoryMinutes:   append([]int(nil), src.History...),
		AcceptEmpty:      src.AcceptEmpty,
		Maintainer:       src.Maintainer,
		MaintainerURL:    src.MaintainerURL,
		Info:             src.Info,
		License:          src.License,
		Attribution:      src.Attribution,
		Redistributable:  sourceRedistributable(eng, name, src),
		DerivedFrom:      append([]string(nil), src.DerivedFrom...),
	}
	if eng != nil && src.Provenance == config.ProvenanceSecondaryMerge {
		composition, ok := mergeCompositions[name]
		if !ok {
			composition = eng.MergeComposition(src, enableAll)
		}
		f.MergeIncluded = append([]engine.MergeInputState(nil), composition.Included...)
		f.MergeSubtracted = append([]engine.MergeInputState(nil), composition.Subtracted...)
		f.MergeExcluded = append([]engine.MergeInputState(nil), composition.Excluded...)
	}
	if src.Attributes != nil {
		f.Downloader = src.Attributes["downloader"]
		f.DownloaderOptions = src.Attributes["downloader_options"]
	}
	populateFromCacheAndSchedule(&f, name, src, entryIdx, schedIdx, liveStates, policy, now)
	return f
}

func populateFromCacheAndSchedule(f *adminFeed, name string, src *config.Source, entryIdx map[string]*cache.Entry, schedIdx map[string]*scheduler.Item, liveStates map[string]adminLiveState, policy feedhealth.Policy, now time.Time) {
	entry := entryIdx[name]
	if entry != nil {
		f.Entries = entry.Entries
		f.EntriesMin = entry.EntriesMin
		f.EntriesMax = entry.EntriesMax
		f.UniqueIPs = entry.UniqueIPs
		f.IPsMin = entry.IPsMin
		f.IPsMax = entry.IPsMax
		f.Version = entry.Version
		f.AvgUpdateMins = entry.AverageUpdateMins
		f.MinUpdateMins = entry.MinUpdateMins
		f.MaxUpdateMins = entry.MaxUpdateMins
		f.DownloadFailures = entry.DownloadFailures
		f.LastCheck = entry.CheckedDate
		f.LastUpdate = entry.SourceDate
		f.ProcessedDate = entry.ProcessedDate
		f.StartedDate = entry.StartedDate
		f.ClockSkewSeconds = entry.ClockSkewSeconds
		f.LastStatus = entry.LastStatus
		f.LastRunReason = entry.LastRunReason.String()
		f.LastProcessingMS = entry.LastProcessingMS
		f.LastError = entry.LastError
		f.Hash = entry.Hash
		f.File = entry.File
		f.Source = entry.Source
		if f.PublicURL == "" {
			f.PublicURL = entry.PublicURL
		}
		// Populate license from the cache entry too when the
		// config did not set it directly (legacy feeds sometimes
		// carry the license only in the cache after migration).
		if f.License == "" {
			f.License = entry.License
		}
		if f.Attribution == "" {
			f.Attribution = entry.Attribution
		}
		if f.Downloader == "" {
			f.Downloader = entry.Downloader
		}
		if f.DownloaderOptions == "" {
			f.DownloaderOptions = entry.DownloaderOptions
		}
	}
	if item, ok := schedIdx[name]; ok {
		f.Enabled = item.Enabled
		if item.FrequencyMinutes > 0 {
			f.FrequencyMinutes = item.FrequencyMinutes
		}
		f.NextCheck = item.NextDue.Unix()
		f.SchedulerDetail = adminSchedulerDetail(f, item.Detail)
	}
	f.Status = deriveFeedStatus(f, liveStates[name])
	populateAdminFeedStatusMeta(f)
	f.Health = feedhealth.Classify(entry, src, policy, now)
}

func adminSchedulerDetail(f *adminFeed, detail string) string {
	if f == nil {
		return detail
	}
	if f.FrequencyMinutes == 0 && f.Kind == "retention" {
		return adminDerivedScheduleLabel
	}
	return detail
}

type adminLiveState struct {
	DownloadWaiting   bool
	DownloadActive    bool
	ProcessingWaiting bool
	ProcessingActive  bool
}

func liveFeedStates(activity scheduler.ActivitySnapshot) map[string]adminLiveState {
	states := make(map[string]adminLiveState, len(activity.DownloadWaiting)+len(activity.DownloadActive)+len(activity.ProcessingWaiting)+len(activity.ProcessingActive))
	for _, item := range activity.DownloadWaiting {
		state := states[item.Name]
		state.DownloadWaiting = true
		states[item.Name] = state
	}
	for _, item := range activity.DownloadActive {
		state := states[item.Name]
		state.DownloadActive = true
		states[item.Name] = state
	}
	for _, item := range activity.ProcessingWaiting {
		state := states[item.Name]
		state.ProcessingWaiting = true
		states[item.Name] = state
	}
	for _, item := range activity.ProcessingActive {
		state := states[item.Name]
		state.ProcessingActive = true
		states[item.Name] = state
	}
	return states
}

// deriveFeedStatus computes a status string from feed state.
func deriveFeedStatus(f *adminFeed, live adminLiveState) string {
	if !f.Enabled {
		return "disabled"
	}
	switch {
	case live.ProcessingActive:
		return "processing"
	case live.DownloadActive:
		return "downloading"
	case live.ProcessingWaiting:
		return "waiting_process"
	case live.DownloadWaiting:
		return "waiting_download"
	}
	if f.LastError != "" {
		return "error"
	}
	if f.LastStatus == "" && f.LastCheck == 0 {
		return "unavailable"
	}
	// Stale: last check was more than 3x the frequency ago.
	if f.FrequencyMinutes > 0 && f.LastCheck > 0 {
		staleThreshold := time.Duration(f.FrequencyMinutes*3) * time.Minute
		if time.Since(time.Unix(f.LastCheck, 0)) > staleThreshold {
			return "stale"
		}
	}
	return "healthy"
}

func deriveArtifactStatus(a *adminArtifact, queued, active bool) string {
	switch {
	case !a.Enabled:
		return "disabled"
	case active:
		return "downloading"
	case queued:
		return "queued"
	case a.LastError != "":
		return "error"
	case a.LastStatus == "" && a.LastCheck == 0:
		return "unavailable"
	}
	if a.FrequencyMinutes > 0 && a.LastCheck > 0 {
		staleThreshold := time.Duration(a.FrequencyMinutes*3) * time.Minute
		if time.Since(time.Unix(a.LastCheck, 0)) > staleThreshold {
			return "stale"
		}
	}
	return "healthy"
}

// buildAdminFeedDetail constructs the detailed view for a single feed.
func buildAdminFeedDetail(eng *engine.Engine, runner *scheduler.Runner, name string) (*adminFeedDetail, error) {
	cfg := eng.Config()
	policy := feedhealth.PolicyFromRuntime(cfg.Runtime)
	entries := eng.EntriesSnapshot()
	snap := runner.Snapshot()
	activity := runner.ActivitySnapshot()
	now := time.Now().UTC()
	liveStates := liveFeedStates(activity)
	enableAll := runnerEnableAll(runner)

	entryIndex := make(map[string]*cache.Entry, len(entries))
	for i := range entries {
		entryIndex[entries[i].Name] = &entries[i]
	}
	schedIndex := make(map[string]*scheduler.Item, len(snap.Items))
	for i := range snap.Items {
		schedIndex[snap.Items[i].Name] = &snap.Items[i]
	}

	// After config.ExpandDerivatives, everything is in cfg.Sources —
	// plain sources, merges, and retention variants alike. The old
	// fallback that looked up cfg.Merges[name] is gone because
	// cfg.Merges is always empty after expansion.
	src, ok := cfg.Sources[name]
	if !ok {
		return nil, &feedNotFoundError{name: name}
	}
	base := buildSourceFeed(eng, name, src, entryIndex, schedIndex, liveStates, policy, now, enableAll, nil)
	// MergeSources stays populated for backward compatibility with
	// the admin UI detail view. It lists only additive merge sources;
	// subtractive parents are exposed through merge_subtracted.
	var mergeSources []string
	if src.Provenance == config.ProvenanceSecondaryMerge {
		mergeSources = append([]string(nil), mergeSourcesForAdmin(src)...)
	}

	detail := &adminFeedDetail{
		adminFeed:    base,
		MergeSources: mergeSources,
		History:      nil, // History is consumed by ExpandDerivatives at load
	}

	// Populate extra fields from cache entry.
	if entry, ok := entryIndex[name]; ok {
		detail.File = entry.File
		detail.Hash = entry.Hash
		detail.EntriesMin = entry.EntriesMin
		detail.EntriesMax = entry.EntriesMax
		detail.IPsMin = entry.IPsMin
		detail.IPsMax = entry.IPsMax
		detail.AvgUpdateMins = entry.AverageUpdateMins
		detail.MinUpdateMins = entry.MinUpdateMins
		detail.MaxUpdateMins = entry.MaxUpdateMins
	}

	return detail, nil
}

func mergeSourcesForAdmin(src *config.Source) []string {
	if src == nil {
		return nil
	}
	if len(src.MergeSources) > 0 || len(src.MergeExclude) > 0 {
		return src.MergeSources
	}
	return src.DerivedFrom
}

func runnerEnableAll(runner *scheduler.Runner) bool {
	if runner == nil {
		return false
	}
	return runner.EnableAll()
}

func sourceRedistributable(eng *engine.Engine, name string, src *config.Source) bool {
	if eng != nil {
		return eng.IsRedistributable(name)
	}
	return src == nil || src.IsRedistributable()
}

type feedNotFoundError struct {
	name string
}

func (e *feedNotFoundError) Error() string {
	return "unknown feed \"" + e.name + "\""
}
