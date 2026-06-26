package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/internal/observability"
	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/runreason"
	"github.com/firehol/update-ipsets/pkg/scheduler"

	"go.opentelemetry.io/otel/attribute"
)

const (
	integrityStatusClean      = "clean"
	integrityStatusIssues     = "issues"
	integrityStatusInProgress = "in_progress"
	integrityStatusScheduled  = "scheduled"
)

type integrityReport struct {
	IncludeArchived    bool                       `json:"include_archived,omitempty"`
	Status             string                     `json:"status"`
	Generation         uint64                     `json:"generation"`
	CacheState         engine.IntegrityCacheState `json:"cache_state,omitempty"`
	Running            bool                       `json:"running"`
	StartupScanRunning bool                       `json:"startup_scan_running,omitempty"`
	Queued             bool                       `json:"queued,omitempty"`
	Coalesced          bool                       `json:"coalesced,omitempty"`
	Ticket             *engine.LaneTicket         `json:"ticket,omitempty"`
	LastStarted        time.Time                  `json:"last_started,omitempty"`
	LastEnded          time.Time                  `json:"last_ended,omitempty"`
	CheckedAt          time.Time                  `json:"checked_at,omitempty"`
	LastError          string                     `json:"last_error,omitempty"`
	Count              int                        `json:"count"`
	Findings           []engine.IntegrityFinding  `json:"findings"`
}

type integrityReprocessResult struct {
	IncludeArchived    bool                       `json:"include_archived,omitempty"`
	Status             string                     `json:"status"`
	Generation         uint64                     `json:"generation"`
	CacheState         engine.IntegrityCacheState `json:"cache_state,omitempty"`
	Running            bool                       `json:"running,omitempty"`
	StartupScanRunning bool                       `json:"startup_scan_running,omitempty"`
	Queued             bool                       `json:"queued,omitempty"`
	Coalesced          bool                       `json:"coalesced,omitempty"`
	Ticket             *engine.LaneTicket         `json:"ticket,omitempty"`
	LastStarted        time.Time                  `json:"last_started,omitempty"`
	LastEnded          time.Time                  `json:"last_ended,omitempty"`
	CheckedAt          time.Time                  `json:"checked_at,omitempty"`
	LastError          string                     `json:"last_error,omitempty"`
	Count              int                        `json:"count"`
	Names              []string                   `json:"names,omitempty"`
	RecheckNames       []string                   `json:"recheck_names,omitempty"`
	ReprocessNames     []string                   `json:"reprocess_names,omitempty"`
	Findings           []engine.IntegrityFinding  `json:"findings"`
}

type entityIntegrityReport struct {
	Status             string                          `json:"status"`
	Generation         uint64                          `json:"generation"`
	CacheState         engine.IntegrityCacheState      `json:"cache_state,omitempty"`
	Running            bool                            `json:"running"`
	StartupScanRunning bool                            `json:"startup_scan_running,omitempty"`
	Queued             bool                            `json:"queued,omitempty"`
	Coalesced          bool                            `json:"coalesced,omitempty"`
	Ticket             *engine.LaneTicket              `json:"ticket,omitempty"`
	LastStarted        time.Time                       `json:"last_started,omitempty"`
	LastEnded          time.Time                       `json:"last_ended,omitempty"`
	CheckedAt          time.Time                       `json:"checked_at,omitempty"`
	LastError          string                          `json:"last_error,omitempty"`
	Count              int                             `json:"count"`
	Findings           []engine.EntityIntegrityFinding `json:"findings"`
}

type entityIntegrityActionResult struct {
	Status             string                     `json:"status"`
	Generation         uint64                     `json:"generation"`
	CacheState         engine.IntegrityCacheState `json:"cache_state,omitempty"`
	Running            bool                       `json:"running,omitempty"`
	StartupScanRunning bool                       `json:"startup_scan_running,omitempty"`
	Queued             bool                       `json:"queued,omitempty"`
	Coalesced          bool                       `json:"coalesced,omitempty"`
	Ticket             *engine.LaneTicket         `json:"ticket,omitempty"`
	LastStarted        time.Time                  `json:"last_started,omitempty"`
	LastEnded          time.Time                  `json:"last_ended,omitempty"`
	CheckedAt          time.Time                  `json:"checked_at,omitempty"`
	LastError          string                     `json:"last_error,omitempty"`
}

func buildIntegrityReport(ctx context.Context, eng *engine.Engine, includeArchived, enableAll bool, webDir string) (integrityReport, error) {
	started := time.Now()
	opts := engine.IntegrityOptions{IncludeArchived: includeArchived, EnableAll: enableAll, WebDir: webDir}
	snap := eng.PipelineIntegrityCacheSnapshot(opts)

	findings := snap.Findings
	if findings == nil {
		findings = []engine.IntegrityFinding{}
	}
	findings = annotateIntegrityFindings(eng, findings)
	out := integrityReport{
		IncludeArchived:    includeArchived,
		Status:             integrityStatusForFindings(findings, snap),
		Generation:         snap.Generation,
		CacheState:         snap.CacheState,
		Running:            snap.Running || snap.Queued,
		StartupScanRunning: snap.StartupScanRunning,
		Queued:             snap.Queued,
		Coalesced:          snap.Coalesced,
		Ticket:             snap.Ticket,
		LastStarted:        snap.LastStarted,
		LastEnded:          snap.LastEnded,
		CheckedAt:          snap.CheckedAt,
		LastError:          snap.LastError,
		Count:              len(findings),
		Findings:           findings,
	}
	out = sanitizeIntegrityReport(out)
	observeIntegrityCheck("pipeline", out.Status, out.Count, time.Since(started))
	return out, nil
}

func handleAdminIntegrity(ctx context.Context, eng *engine.Engine, enableAll bool, webDir string) http.HandlerFunc {
	ctx = nonNilHandlerContext(ctx)
	return func(w http.ResponseWriter, r *http.Request) {
		if !isReadMethod(r.Method) {
			writeReadMethodNotAllowed(w)
			return
		}
		apiNoCache(w)
		report, err := buildIntegrityReport(ctx, eng, includeArchivedQuery(r), enableAll, webDir)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, report)
	}
}

func buildEntityIntegrityReport(ctx context.Context, eng *engine.Engine) (entityIntegrityReport, error) {
	started := time.Now()
	result := "error"
	findingCount := 0
	defer func() {
		eng.ObserveOperation("admin.entity_integrity_check", time.Since(started))
		eng.ObserveCounter("admin.entity_integrity_check", 1, 0)
		observeIntegrityCheck("entity", result, findingCount, time.Since(started))
	}()
	snap := eng.EntityIntegrityCacheSnapshot()

	findings := snap.Findings
	findingCount = len(findings)
	out := entityIntegrityReport{
		Status:             entityIntegrityStatusForFindings(findings, snap),
		Generation:         snap.Generation,
		CacheState:         snap.CacheState,
		Running:            snap.Running || snap.Queued,
		StartupScanRunning: snap.StartupScanRunning,
		Queued:             snap.Queued,
		Coalesced:          snap.Coalesced,
		Ticket:             snap.Ticket,
		LastStarted:        snap.LastStarted,
		LastEnded:          snap.LastEnded,
		CheckedAt:          snap.CheckedAt,
		LastError:          snap.LastError,
		Count:              len(findings),
		Findings:           findings,
	}
	result = out.Status
	return sanitizeEntityIntegrityReport(out), nil
}

func handleAdminEntityIntegrity(ctx context.Context, eng *engine.Engine) http.HandlerFunc {
	ctx = nonNilHandlerContext(ctx)
	return func(w http.ResponseWriter, r *http.Request) {
		if !isReadMethod(r.Method) {
			writeReadMethodNotAllowed(w)
			return
		}
		apiNoCache(w)
		report, err := buildEntityIntegrityReport(ctx, eng)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, report)
	}
}

func handleAdminEntityIntegrityRebuildWithContext(ctx context.Context, eng *engine.Engine) http.HandlerFunc {
	ctx = nonNilHandlerContext(ctx)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
			return
		}
		apiNoCache(w)
		result, err := eng.QueueEntityArtifactsRebuild(ctx, "operator_rebuild")
		if err != nil {
			observeAPIRecalculation(r, "admin", "entity_rebuild", "error", 0)
			jsonQueueSubmissionError(w, err)
			return
		}
		if result.Coalesced {
			snap := eng.EntityIntegrityCacheSnapshot()
			observeAPIRecalculation(r, "admin", "entity_rebuild", "in_progress", 0)
			writeJSON(w, http.StatusOK, entityIntegrityActionResult{
				Status:             integrityStatusInProgress,
				Generation:         snap.Generation,
				CacheState:         snap.CacheState,
				Running:            true,
				StartupScanRunning: snap.StartupScanRunning,
				Queued:             result.Queued,
				Coalesced:          result.Coalesced,
				Ticket:             &result.Ticket,
			})
			return
		}
		snap := eng.EntityIntegrityCacheSnapshot()
		observeAPIRecalculation(r, "admin", "entity_rebuild", "scheduled", 1)
		observeIntegrityRecoveryTargets("entity", "rebuild", 1)
		writeJSON(w, http.StatusAccepted, entityIntegrityActionResult{
			Status:             integrityStatusScheduled,
			Generation:         snap.Generation,
			CacheState:         snap.CacheState,
			StartupScanRunning: snap.StartupScanRunning,
			Queued:             result.Queued,
			Ticket:             &result.Ticket,
		})
	}
}

func handleAdminIntegrityRefresh(ctx context.Context, eng *engine.Engine, enableAll bool, webDir string) http.HandlerFunc {
	ctx = nonNilHandlerContext(ctx)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
			return
		}
		apiNoCache(w)
		opts := engine.IntegrityOptions{IncludeArchived: includeArchivedQuery(r), EnableAll: enableAll, WebDir: webDir}
		snap, err := eng.QueuePipelineIntegrityRefresh(ctx, opts, "admin_refresh")
		if err != nil {
			observeAPIRecalculation(r, "admin", "integrity_refresh", "error", 0)
			jsonQueueSubmissionError(w, err)
			return
		}
		observeAPIRecalculation(r, "admin", "integrity_refresh", "scheduled", 1)
		writeJSON(w, http.StatusAccepted, integrityReprocessResult{
			IncludeArchived:    opts.IncludeArchived,
			Status:             integrityStatusScheduled,
			Generation:         snap.Generation,
			CacheState:         snap.CacheState,
			Running:            snap.Running,
			StartupScanRunning: snap.StartupScanRunning,
			Queued:             snap.Queued,
			Coalesced:          snap.Coalesced,
			Ticket:             snap.Ticket,
			LastStarted:        snap.LastStarted,
			LastEnded:          snap.LastEnded,
			CheckedAt:          snap.CheckedAt,
			LastError:          snap.LastError,
			Count:              len(snap.Findings),
			Findings:           annotateIntegrityFindings(eng, snap.Findings),
		})
	}
}

func handleAdminEntityIntegrityRefresh(ctx context.Context, eng *engine.Engine) http.HandlerFunc {
	ctx = nonNilHandlerContext(ctx)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
			return
		}
		apiNoCache(w)
		snap, err := eng.QueueEntityIntegrityRefresh(ctx, "admin_refresh")
		if err != nil {
			observeAPIRecalculation(r, "admin", "entity_integrity_refresh", "error", 0)
			jsonQueueSubmissionError(w, err)
			return
		}
		observeAPIRecalculation(r, "admin", "entity_integrity_refresh", "scheduled", 1)
		writeJSON(w, http.StatusAccepted, entityIntegrityActionResult{
			Status:             integrityStatusScheduled,
			Generation:         snap.Generation,
			CacheState:         snap.CacheState,
			Running:            snap.Running,
			StartupScanRunning: snap.StartupScanRunning,
			Queued:             snap.Queued,
			Coalesced:          snap.Coalesced,
			Ticket:             snap.Ticket,
			LastStarted:        snap.LastStarted,
			LastEnded:          snap.LastEnded,
			CheckedAt:          snap.CheckedAt,
			LastError:          snap.LastError,
		})
	}
}

func includeArchivedQuery(r *http.Request) bool {
	if r == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get("include_archived"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func sanitizeIntegrityReport(report integrityReport) integrityReport {
	report.LastStarted = sanitizeJSONTime(report.LastStarted)
	report.LastEnded = sanitizeJSONTime(report.LastEnded)
	if report.Findings == nil {
		report.Findings = []engine.IntegrityFinding{}
		return report
	}
	for i := range report.Findings {
		report.Findings[i] = sanitizeIntegrityFinding(report.Findings[i])
	}
	return report
}

func sanitizeIntegrityFinding(finding engine.IntegrityFinding) engine.IntegrityFinding {
	finding.SourceMTime = sanitizeJSONTime(finding.SourceMTime)
	finding.SourceFileMTime = sanitizeJSONTime(finding.SourceFileMTime)
	finding.ProcessedAt = sanitizeJSONTime(finding.ProcessedAt)
	return finding
}

func sanitizeEntityIntegrityReport(report entityIntegrityReport) entityIntegrityReport {
	report.LastStarted = sanitizeJSONTime(report.LastStarted)
	report.LastEnded = sanitizeJSONTime(report.LastEnded)
	if report.Findings == nil {
		report.Findings = []engine.EntityIntegrityFinding{}
		return report
	}
	for i := range report.Findings {
		report.Findings[i].PathMTime = sanitizeJSONTime(report.Findings[i].PathMTime)
		report.Findings[i].ReferenceMTime = sanitizeJSONTime(report.Findings[i].ReferenceMTime)
	}
	return report
}

func annotateIntegrityFindings(eng *engine.Engine, findings []engine.IntegrityFinding) []engine.IntegrityFinding {
	if len(findings) == 0 {
		return findings
	}
	out := make([]engine.IntegrityFinding, len(findings))
	copy(out, findings)
	for i := range out {
		recheckNames, reprocessNames := eng.IntegrityRecoveryPlan([]engine.IntegrityFinding{out[i]})
		switch {
		case len(recheckNames) > 0:
			out[i].RecoveryAction = engine.IntegrityRecoveryActionRecheck
			out[i].RecoveryTargets = recheckNames
		case len(reprocessNames) > 0:
			out[i].RecoveryAction = engine.IntegrityRecoveryActionReprocess
			out[i].RecoveryTargets = reprocessNames
		}
	}
	return out
}

func handleAdminIntegrityReprocess(ctx context.Context, eng *engine.Engine, runner *scheduler.Runner, webDir string) http.HandlerFunc {
	ctx = nonNilHandlerContext(ctx)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
			return
		}
		apiNoCache(w)

		includeArchived := includeArchivedQuery(r)
		opts := engine.IntegrityOptions{IncludeArchived: includeArchived, EnableAll: runner.EnableAll(), WebDir: webDir}
		snap := eng.PipelineIntegrityCacheSnapshot(opts)
		if snap.CacheState != engine.IntegrityCacheFresh {
			queued, err := eng.QueuePipelineIntegrityRefresh(ctx, opts, "admin_reprocess")
			if err != nil {
				observeAPIRecalculation(r, "admin", "integrity_reprocess", "error", 0)
				jsonQueueSubmissionError(w, err)
				return
			}
			observeAPIRecalculation(r, "admin", "integrity_reprocess", "in_progress", 0)
			writeJSON(w, http.StatusAccepted, integrityReprocessResult{
				IncludeArchived:    includeArchived,
				Status:             integrityStatusInProgress,
				Generation:         queued.Generation,
				CacheState:         queued.CacheState,
				Running:            queued.Running,
				StartupScanRunning: queued.StartupScanRunning,
				Queued:             queued.Queued,
				Coalesced:          queued.Coalesced,
				Ticket:             queued.Ticket,
				LastStarted:        queued.LastStarted,
				LastEnded:          queued.LastEnded,
				CheckedAt:          queued.CheckedAt,
				LastError:          queued.LastError,
				Count:              0,
				Findings:           []engine.IntegrityFinding{},
			})
			return
		}
		findings := annotateIntegrityFindings(eng, snap.Findings)
		report := integrityReport{
			IncludeArchived:    includeArchived,
			Status:             integrityStatusForFindings(findings, snap),
			Generation:         snap.Generation,
			CacheState:         snap.CacheState,
			Running:            false,
			StartupScanRunning: snap.StartupScanRunning,
			LastStarted:        snap.LastStarted,
			LastEnded:          snap.LastEnded,
			CheckedAt:          snap.CheckedAt,
			LastError:          snap.LastError,
			Count:              len(findings),
			Findings:           findings,
		}
		if report.Count == 0 {
			observeAPIRecalculation(r, "admin", "integrity_reprocess", "clean", 0)
			writeJSON(w, http.StatusOK, integrityReprocessResult{
				IncludeArchived:    includeArchived,
				Status:             integrityStatusClean,
				Generation:         report.Generation,
				CacheState:         report.CacheState,
				StartupScanRunning: report.StartupScanRunning,
				LastStarted:        report.LastStarted,
				LastEnded:          report.LastEnded,
				CheckedAt:          report.CheckedAt,
				LastError:          report.LastError,
				Count:              0,
				Findings:           []engine.IntegrityFinding{},
			})
			return
		}

		recheckNames, reprocessNames := eng.IntegrityRecoveryPlan(report.Findings)
		names := append(append([]string(nil), recheckNames...), reprocessNames...)
		ticket, err := eng.QueuePipelineIntegrityReprocess(ctx, opts, "admin_reprocess", func(laneCtx context.Context, findings []engine.IntegrityFinding) error {
			if err := laneCtx.Err(); err != nil {
				return err
			}
			recheckNames, reprocessNames := eng.IntegrityRecoveryPlan(findings)
			if len(recheckNames) > 0 {
				observeIntegrityRecoveryTargets("pipeline", "recheck", len(recheckNames))
				if err := runner.TriggerSourcesWithin(laneCtx, scheduler.LaneActionAdmissionTimeout, scheduler.PendingAction{
					Names:   recheckNames,
					Recheck: true,
					Reason:  runreason.ReasonIntegrityReprocess,
				}); err != nil {
					return fmt.Errorf("queue integrity recheck work: %w", err)
				}
			}
			if len(reprocessNames) > 0 {
				observeIntegrityRecoveryTargets("pipeline", "reprocess", len(reprocessNames))
				if err := runner.TriggerSourcesWithin(laneCtx, scheduler.LaneActionAdmissionTimeout, scheduler.PendingAction{
					Names:     reprocessNames,
					Reprocess: true,
					Reason:    runreason.ReasonIntegrityReprocess,
				}); err != nil {
					return fmt.Errorf("queue integrity reprocess work: %w", err)
				}
			}
			return laneCtx.Err()
		})
		if err != nil {
			if errors.Is(err, engine.ErrIntegrityCacheNotFresh) {
				queued, queueErr := eng.QueuePipelineIntegrityRefresh(ctx, opts, "admin_reprocess")
				if queueErr != nil {
					observeAPIRecalculation(r, "admin", "integrity_reprocess", "error", 0)
					jsonQueueSubmissionError(w, queueErr)
					return
				}
				observeAPIRecalculation(r, "admin", "integrity_reprocess", "in_progress", 0)
				writeJSON(w, http.StatusAccepted, integrityReprocessResult{
					IncludeArchived:    includeArchived,
					Status:             integrityStatusInProgress,
					Generation:         queued.Generation,
					CacheState:         queued.CacheState,
					Running:            queued.Running,
					StartupScanRunning: queued.StartupScanRunning,
					Queued:             queued.Queued,
					Coalesced:          queued.Coalesced,
					Ticket:             queued.Ticket,
					LastStarted:        queued.LastStarted,
					LastEnded:          queued.LastEnded,
					CheckedAt:          queued.CheckedAt,
					LastError:          queued.LastError,
					Count:              0,
					Findings:           []engine.IntegrityFinding{},
				})
				return
			}
			observeAPIRecalculation(r, "admin", "integrity_reprocess", "error", 0)
			jsonQueueSubmissionError(w, err)
			return
		}
		resultStatus := integrityStatusScheduled
		statusCode := http.StatusAccepted
		if ticket.Coalesced {
			resultStatus = integrityStatusInProgress
			statusCode = http.StatusOK
		}
		observeAPIRecalculation(r, "admin", "integrity_reprocess", resultStatus, len(names))
		writeJSON(w, statusCode, integrityReprocessResult{
			IncludeArchived:    includeArchived,
			Status:             resultStatus,
			Generation:         report.Generation,
			CacheState:         report.CacheState,
			Running:            ticket.State == engine.LaneWorkActive,
			StartupScanRunning: report.StartupScanRunning,
			Queued:             ticket.Queued,
			Coalesced:          ticket.Coalesced,
			Ticket:             &ticket,
			LastStarted:        report.LastStarted,
			LastEnded:          report.LastEnded,
			CheckedAt:          report.CheckedAt,
			LastError:          report.LastError,
			Count:              len(names),
			Names:              names,
			RecheckNames:       recheckNames,
			ReprocessNames:     reprocessNames,
			Findings:           report.Findings,
		})
	}
}

func jsonQueueSubmissionError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, engine.ErrLaneShuttingDown) {
		status = http.StatusServiceUnavailable
	}
	jsonError(w, status, err)
}

func integrityStatusForFindings(findings []engine.IntegrityFinding, snap engine.PipelineIntegrityCacheSnapshot) string {
	if len(findings) > 0 {
		return integrityStatusIssues
	}
	if snap.CacheState == engine.IntegrityCacheRefreshQueued ||
		snap.CacheState == engine.IntegrityCacheRefreshRunning {
		return integrityStatusInProgress
	}
	return integrityStatusClean
}

func entityIntegrityStatusForFindings(findings []engine.EntityIntegrityFinding, snap engine.EntityIntegrityCacheSnapshot) string {
	if len(findings) > 0 {
		return integrityStatusIssues
	}
	if snap.CacheState == engine.IntegrityCacheRefreshQueued ||
		snap.CacheState == engine.IntegrityCacheRefreshRunning {
		return integrityStatusInProgress
	}
	return integrityStatusClean
}

func nonNilHandlerContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func observeIntegrityCheck(kind, result string, findings int, dur time.Duration) {
	if kind == "" {
		kind = "unknown"
	}
	if result == "" {
		result = "unknown"
	}
	attrs := []attribute.KeyValue{
		attribute.String("integrity.kind", kind),
		attribute.String("integrity.result", result),
	}
	ctx := observability.BackgroundContext()
	observability.Count(ctx, "integrity.checks", 1, attrs...)
	observability.Duration(ctx, "integrity.check", dur, attrs...)
	observability.Gauge(ctx, "integrity.findings", int64(findings), attribute.String("integrity.kind", kind))
}

func observeIntegrityRecoveryTargets(kind, action string, targets int) {
	if targets <= 0 {
		return
	}
	if kind == "" {
		kind = "unknown"
	}
	if action == "" {
		action = "unknown"
	}
	observability.Count(
		observability.BackgroundContext(),
		"integrity.recovery.targets",
		int64(targets),
		attribute.String("integrity.kind", kind),
		attribute.String("integrity.action", action),
	)
}
