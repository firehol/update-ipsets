package web

import (
	"context"
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
	IncludeArchived bool                      `json:"include_archived,omitempty"`
	Status          string                    `json:"status"`
	Running         bool                      `json:"running"`
	LastStarted     time.Time                 `json:"last_started,omitempty"`
	LastEnded       time.Time                 `json:"last_ended,omitempty"`
	Count           int                       `json:"count"`
	Findings        []engine.IntegrityFinding `json:"findings"`
}

type integrityReprocessResult struct {
	IncludeArchived bool                      `json:"include_archived,omitempty"`
	Status          string                    `json:"status"`
	Running         bool                      `json:"running,omitempty"`
	LastStarted     time.Time                 `json:"last_started,omitempty"`
	LastEnded       time.Time                 `json:"last_ended,omitempty"`
	Count           int                       `json:"count"`
	Names           []string                  `json:"names,omitempty"`
	RecheckNames    []string                  `json:"recheck_names,omitempty"`
	ReprocessNames  []string                  `json:"reprocess_names,omitempty"`
	Findings        []engine.IntegrityFinding `json:"findings"`
}

type entityIntegrityReport struct {
	Status      string                          `json:"status"`
	Running     bool                            `json:"running"`
	LastStarted time.Time                       `json:"last_started,omitempty"`
	LastEnded   time.Time                       `json:"last_ended,omitempty"`
	Count       int                             `json:"count"`
	Findings    []engine.EntityIntegrityFinding `json:"findings"`
}

type entityIntegrityActionResult struct {
	Status      string    `json:"status"`
	Running     bool      `json:"running,omitempty"`
	LastStarted time.Time `json:"last_started,omitempty"`
	LastEnded   time.Time `json:"last_ended,omitempty"`
}

func buildIntegrityReport(eng *engine.Engine, includeArchived, enableAll bool, webDir string) integrityReport {
	started := time.Now()
	status := eng.StatusSnapshotLight()
	if status.Running {
		out := sanitizeIntegrityReport(integrityReport{
			IncludeArchived: includeArchived,
			Status:          integrityStatusInProgress,
			Running:         true,
			LastStarted:     status.LastStarted,
			LastEnded:       status.LastEnded,
			Findings:        []engine.IntegrityFinding{},
		})
		observeIntegrityCheck("pipeline", out.Status, out.Count, time.Since(started))
		return out
	}

	findings := eng.CheckIntegrityWithOptions(engine.IntegrityOptions{IncludeArchived: includeArchived, EnableAll: enableAll, WebDir: webDir})
	if findings == nil {
		findings = []engine.IntegrityFinding{}
	}
	findings = annotateIntegrityFindings(eng, findings)
	out := integrityReport{
		IncludeArchived: includeArchived,
		Status:          integrityStatusClean,
		Running:         false,
		LastStarted:     status.LastStarted,
		LastEnded:       status.LastEnded,
		Count:           len(findings),
		Findings:        findings,
	}
	if len(findings) > 0 {
		out.Status = integrityStatusIssues
	}
	out = sanitizeIntegrityReport(out)
	observeIntegrityCheck("pipeline", out.Status, out.Count, time.Since(started))
	return out
}

func handleAdminIntegrity(eng *engine.Engine, enableAll bool, webDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isReadMethod(r.Method) {
			writeReadMethodNotAllowed(w)
			return
		}
		apiNoCache(w)
		writeJSON(w, http.StatusOK, buildIntegrityReport(eng, includeArchivedQuery(r), enableAll, webDir))
	}
}

func buildEntityIntegrityReport(eng *engine.Engine) (entityIntegrityReport, error) {
	started := time.Now()
	result := "error"
	findingCount := 0
	defer func() {
		eng.ObserveOperation("admin.entity_integrity_check", time.Since(started))
		eng.ObserveCounter("admin.entity_integrity_check", 1, 0)
		observeIntegrityCheck("entity", result, findingCount, time.Since(started))
	}()
	status := eng.StatusSnapshotLight()
	if entityIntegrityBusy(status) {
		result = integrityStatusInProgress
		return sanitizeEntityIntegrityReport(entityIntegrityReport{
			Status:      integrityStatusInProgress,
			Running:     true,
			LastStarted: status.LastStarted,
			LastEnded:   status.LastEnded,
			Findings:    []engine.EntityIntegrityFinding{},
		}), nil
	}

	findings, _, err := eng.CheckEntityArtifactsIntegrity()
	if err != nil {
		return entityIntegrityReport{}, err
	}
	findingCount = len(findings)
	out := entityIntegrityReport{
		Status:      integrityStatusClean,
		Running:     false,
		LastStarted: status.LastStarted,
		LastEnded:   status.LastEnded,
		Count:       len(findings),
		Findings:    findings,
	}
	if len(findings) > 0 {
		out.Status = integrityStatusIssues
	}
	result = out.Status
	return sanitizeEntityIntegrityReport(out), nil
}

func handleAdminEntityIntegrity(eng *engine.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isReadMethod(r.Method) {
			writeReadMethodNotAllowed(w)
			return
		}
		apiNoCache(w)
		report, err := buildEntityIntegrityReport(eng)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, report)
	}
}

func handleAdminEntityIntegrityRebuild(eng *engine.Engine) http.HandlerFunc {
	return handleAdminEntityIntegrityRebuildWithContext(context.Background(), eng)
}

func handleAdminEntityIntegrityRebuildWithContext(ctx context.Context, eng *engine.Engine) http.HandlerFunc {
	if ctx == nil {
		ctx = context.Background()
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
			return
		}
		apiNoCache(w)
		if !eng.QueueEntityArtifactsRebuild(ctx, "operator_rebuild") {
			report, err := buildEntityIntegrityReport(eng)
			if err != nil {
				observeAPIRecalculation(r, "admin", "entity_rebuild", "error", 0)
				jsonError(w, http.StatusInternalServerError, err)
				return
			}
			observeAPIRecalculation(r, "admin", "entity_rebuild", "in_progress", 0)
			writeJSON(w, http.StatusOK, entityIntegrityActionResult{
				Status:      integrityStatusInProgress,
				Running:     true,
				LastStarted: report.LastStarted,
				LastEnded:   report.LastEnded,
			})
			return
		}
		observeAPIRecalculation(r, "admin", "entity_rebuild", "scheduled", 1)
		observeIntegrityRecoveryTargets("entity", "rebuild", 1)
		writeJSON(w, http.StatusAccepted, entityIntegrityActionResult{
			Status: integrityStatusScheduled,
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

func entityBackgroundTaskRunning(status engine.StatusSnapshot) bool {
	for _, task := range status.BackgroundTasks {
		if strings.HasPrefix(task.Name, "Entity artifacts ") {
			return true
		}
	}
	return false
}

func entityIntegrityBusy(status engine.StatusSnapshot) bool {
	return status.Running || entityBackgroundTaskRunning(status)
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

func handleAdminIntegrityReprocess(eng *engine.Engine, runner *scheduler.Runner, webDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
			return
		}
		apiNoCache(w)

		includeArchived := includeArchivedQuery(r)
		report := buildIntegrityReport(eng, includeArchived, runner.EnableAll(), webDir)
		if report.Running {
			observeAPIRecalculation(r, "admin", "integrity_reprocess", "in_progress", 0)
			writeJSON(w, http.StatusOK, integrityReprocessResult{
				IncludeArchived: includeArchived,
				Status:          integrityStatusInProgress,
				Running:         true,
				LastStarted:     report.LastStarted,
				LastEnded:       report.LastEnded,
				Count:           0,
				Findings:        []engine.IntegrityFinding{},
			})
			return
		}
		if report.Count == 0 {
			observeAPIRecalculation(r, "admin", "integrity_reprocess", "clean", 0)
			writeJSON(w, http.StatusOK, integrityReprocessResult{
				IncludeArchived: includeArchived,
				Status:          integrityStatusClean,
				LastStarted:     report.LastStarted,
				LastEnded:       report.LastEnded,
				Count:           0,
				Findings:        []engine.IntegrityFinding{},
			})
			return
		}

		recheckNames, reprocessNames := eng.IntegrityRecoveryPlan(report.Findings)
		if len(recheckNames) > 0 {
			observeIntegrityRecoveryTargets("pipeline", "recheck", len(recheckNames))
			runner.TriggerSources(scheduler.PendingAction{
				Names:   recheckNames,
				Recheck: true,
				Reason:  runreason.ReasonIntegrityReprocess,
			})
		}
		if len(reprocessNames) > 0 {
			observeIntegrityRecoveryTargets("pipeline", "reprocess", len(reprocessNames))
			runner.TriggerSources(scheduler.PendingAction{
				Names:     reprocessNames,
				Reprocess: true,
				Reason:    runreason.ReasonIntegrityReprocess,
			})
		}
		names := append(append([]string(nil), recheckNames...), reprocessNames...)
		observeAPIRecalculation(r, "admin", "integrity_reprocess", "scheduled", len(names))
		writeJSON(w, http.StatusAccepted, integrityReprocessResult{
			IncludeArchived: includeArchived,
			Status:          integrityStatusScheduled,
			LastStarted:     report.LastStarted,
			LastEnded:       report.LastEnded,
			Count:           len(names),
			Names:           names,
			RecheckNames:    recheckNames,
			ReprocessNames:  reprocessNames,
			Findings:        report.Findings,
		})
	}
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
