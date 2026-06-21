package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestPipelineIntegrityScenarioDeferredQueuedEntityRefreshSettlesMissingEmptySidecar(t *testing.T) {
	scenario := newPipelineIntegrityScenario(t)

	scenario.applyStep(pipelineIntegrityStep{
		Name:     "initial publish ordinary feeds and providers",
		Delta:    time.Minute,
		Selected: scenario.sourceNamesExcept("merged"),
	})
	scenario.applyStep(pipelineIntegrityStep{
		Name:   "sample becomes an explicit empty entity input",
		Delta:  time.Minute,
		Feed:   "sample",
		Remove: []string{"1.1.1.1", "10.0.0.1"},
		ExpectAbsent: map[string][]string{
			"sample": {"1.1.1.1", "10.0.0.1"},
		},
	})
	scenario.assertExplicitEmptyFeedSidecar("sample")

	scenario.removeCommittedFeedSidecar("sample")
	scenario.assertMissingFeedSidecarFinding("sample")

	task := scenario.eng.beginBackgroundTask(
		"Entity artifacts rebuild",
		"startup",
		"planning",
		"building full country and ASN entity artifacts",
		0,
		0,
	)
	defer task.Finish()

	scenario.now = scenario.now.Add(time.Minute)
	report, err := runSchedulerStyleOnce(t, scenario.eng, RunOptions{
		Selected:   []string{"sample"},
		EnableAll:  true,
		Manual:     true,
		Recheck:    true,
		CleanupOld: true,
	})
	if err != nil {
		t.Fatalf("run sample update while entity rebuild is active: %v", err)
	}
	if !slices.Contains(report.EntityRefreshTargets, "sample") {
		t.Fatalf("entity refresh targets = %v, want sample", report.EntityRefreshTargets)
	}
	scenario.assertNoPendingFeedSidecar("sample")
	scenario.assertMissingFeedSidecarFinding("sample")

	task.Finish()
	scenario.eng.QueueEntityArtifactsRefreshForFeedUpdates(t.Context(), report.EntityRefreshTargets, "pipeline_integrity_scenario")
	scenario.waitForQueuedEntityRefreshClean(5 * time.Second)
	scenario.assertExplicitEmptyFeedSidecar("sample")
	scenario.assertCleanIntegrity()
	scenario.assertGeneratedArtifactMTimeInvariant()
}

func (s *pipelineIntegrityScenario) removeCommittedFeedSidecar(feed string) {
	s.t.Helper()

	path := filepath.Join(s.eng.entityFeedsDir(), feed+".json")
	if err := os.Remove(path); err != nil {
		s.t.Fatalf("remove committed feed sidecar %s: %v", feed, err)
	}
}

func (s *pipelineIntegrityScenario) assertExplicitEmptyFeedSidecar(feed string) {
	s.t.Helper()

	path := filepath.Join(s.eng.entityFeedsDir(), feed+".json")
	var sidecar feedEntitySidecar
	loadJSONForTest(s.t, path, &sidecar)
	if sidecar.Feed != feed {
		s.t.Fatalf("feed sidecar %s feed = %q, want %q", path, sidecar.Feed, feed)
	}
	if len(sidecar.Countries) != 0 || len(sidecar.ASNs) != 0 {
		s.t.Fatalf("feed sidecar %s countries=%v asns=%v, want explicit empty contribution state", path, sidecar.Countries, sidecar.ASNs)
	}
}

func (s *pipelineIntegrityScenario) assertNoPendingFeedSidecar(feed string) {
	s.t.Helper()

	path := filepath.Join(s.eng.entityFeedPendingDir(), feed+".json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		s.t.Fatalf("pending feed sidecar %s should not be staged while full rebuild is active, stat err=%v", path, err)
	}
}

func (s *pipelineIntegrityScenario) assertMissingFeedSidecarFinding(feed string) {
	s.t.Helper()

	findings, _, err := s.eng.CheckEntityArtifactsIntegrity()
	if err != nil {
		s.t.Fatalf("check entity integrity: %v", err)
	}
	for _, finding := range findings {
		if finding.Kind == "feed_sidecar_missing" && finding.Feed == feed {
			if finding.RepairAction != "refresh_feed" {
				s.t.Fatalf("missing sidecar repair action = %q, want refresh_feed", finding.RepairAction)
			}
			return
		}
	}
	s.t.Fatalf("missing feed sidecar finding for %s not found in %+v", feed, findings)
}

func (s *pipelineIntegrityScenario) waitForQueuedEntityRefreshClean(timeout time.Duration) {
	s.t.Helper()

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	var last string
	for {
		if ok, summary := s.integrityCleanAndIdle(); ok {
			return
		} else {
			last = summary
		}

		select {
		case <-ticker.C:
		case <-deadline.C:
			s.t.Fatalf("queued entity refresh did not settle cleanly within %s: %s", timeout, last)
		}
	}
}

func (s *pipelineIntegrityScenario) integrityCleanAndIdle() (bool, string) {
	s.t.Helper()

	status := s.eng.StatusSnapshotLight()
	if len(status.BackgroundTasks) != 0 || status.EntityRefreshPending != 0 || status.EntityHealthPending != 0 || status.EntityRebuildPending {
		return false, fmt.Sprintf(
			"background_tasks=%d entity_refresh_pending=%d entity_health_pending=%d entity_rebuild_pending=%v",
			len(status.BackgroundTasks),
			status.EntityRefreshPending,
			status.EntityHealthPending,
			status.EntityRebuildPending,
		)
	}
	if findings := s.eng.CheckIntegrityWithOptions(IntegrityOptions{EnableAll: true}); len(findings) > 0 {
		return false, fmt.Sprintf("feed-output findings=%+v", findings)
	}
	entityFindings, plan, err := s.eng.CheckEntityArtifactsIntegrity()
	if err != nil {
		return false, fmt.Sprintf("entity integrity error=%v", err)
	}
	if len(entityFindings) > 0 || plan.hasWork() {
		return false, fmt.Sprintf("entity findings=%s plan=%+v", entityFindingSummary(entityFindings), plan)
	}
	return true, "clean"
}

func entityFindingSummary(findings []EntityIntegrityFinding) string {
	if len(findings) == 0 {
		return "[]"
	}
	rows := make([]string, 0, len(findings))
	for _, finding := range findings {
		rows = append(rows, fmt.Sprintf("%s/%s/%s", finding.Scope, finding.Kind, finding.Feed))
	}
	return "[" + strings.Join(rows, " ") + "]"
}
