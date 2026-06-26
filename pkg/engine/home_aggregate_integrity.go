package engine

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (e *Engine) checkHomeAggregatesIntegrity(findings *[]EntityIntegrityFinding, plan *entityIntegrityPlan, health *feedHealthClassifier) error {
	return e.checkHomeAggregatesIntegrityWithSnapshot(e.operationSnapshot(), findings, plan, health)
}

func (e *Engine) checkHomeAggregatesIntegrityWithSnapshot(snap operationSnapshot, findings *[]EntityIntegrityFinding, plan *entityIntegrityPlan, health *feedHealthClassifier) error {
	if e == nil || snap.cfg == nil || findings == nil || plan == nil {
		return nil
	}
	refPath, refTime, expected := e.homeAggregatesReferenceWithSnapshotAndHealth(snap, health)
	if !expected {
		return nil
	}
	path := filepath.Join(outputDirForRuntime(snap.runtime), e.publicHomeAggregatesRelPath())
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			*findings = append(*findings, EntityIntegrityFinding{
				Scope:          "homepage",
				Kind:           "home_aggregates_missing",
				Subject:        "home",
				Path:           path,
				ReferencePath:  refPath,
				ReferenceMTime: refTime,
				RepairAction:   "refresh_home_aggregates",
				Reason:         "homepage aggregate JSON is missing",
			})
			plan.addHomeAggregate()
			return nil
		}
		return err
	}
	pathMTime := info.ModTime().UTC()
	if _, _, err := readHomeAggregatesFile(outputDirForRuntime(snap.runtime), e.publicHomeAggregatesRelPath()); err != nil {
		*findings = append(*findings, EntityIntegrityFinding{
			Scope:        "homepage",
			Kind:         "home_aggregates_malformed",
			Subject:      "home",
			Path:         path,
			PathMTime:    pathMTime,
			RepairAction: "refresh_home_aggregates",
			Reason:       "homepage aggregate JSON is unreadable or unsupported",
		})
		plan.addHomeAggregate()
		return nil
	}
	if !refTime.IsZero() && pathMTime.Before(refTime) {
		*findings = append(*findings, EntityIntegrityFinding{
			Scope:          "homepage",
			Kind:           "home_aggregates_stale",
			Subject:        "home",
			Path:           path,
			PathMTime:      pathMTime,
			ReferencePath:  refPath,
			ReferenceMTime: refTime,
			RepairAction:   "refresh_home_aggregates",
			Reason:         "homepage aggregate JSON is older than the feed state it summarizes",
		})
		plan.addHomeAggregate()
	}
	return nil
}

func (e *Engine) homeAggregatesReference() (string, time.Time, bool) {
	return e.homeAggregatesReferenceWithHealth(nil)
}

func (e *Engine) homeAggregatesReferenceWithHealth(health *feedHealthClassifier) (string, time.Time, bool) {
	return e.homeAggregatesReferenceWithSnapshotAndHealth(e.operationSnapshot(), health)
}

func (e *Engine) homeAggregatesReferenceWithSnapshot(snap operationSnapshot) (string, time.Time, bool) {
	return e.homeAggregatesReferenceWithSnapshotAndHealth(snap, nil)
}

func (e *Engine) homeAggregatesReferenceWithSnapshotAndHealth(snap operationSnapshot, health *feedHealthClassifier) (string, time.Time, bool) {
	if e == nil || snap.cfg == nil {
		return "", time.Time{}, false
	}
	if health == nil {
		now := time.Now().UTC()
		if e.now != nil {
			now = e.now().UTC()
		}
		health = e.newFeedHealthClassifierForConfigPolicy(snap.cfg, snap.feedHealthPolicy, e.state.SnapshotEntries(), now)
	}
	var ref entityDependencyRef
	expected := false
	for _, entry := range e.entriesSnapshot(snap.cfg, configuredNamesForConfig(snap.cfg)) {
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			continue
		}
		src := lookupSourceForConfig(snap.cfg, name)
		if !homeSummaryEligible(snap.cfg, src, nil) {
			continue
		}
		expected = true
		refPath := filepath.Join(outputDirForRuntime(snap.runtime), name+".json")
		if entry.ProcessedDate > 0 {
			mergeEntityDependencyRef(&ref, refPath, time.Unix(entry.ProcessedDate, 0).UTC())
		}
		if transitionAt := e.entityFeedHealthTransitionTimeWithSnapshot(snap, name, health); !transitionAt.IsZero() {
			mergeEntityDependencyRef(&ref, refPath, transitionAt)
		}
	}
	return ref.path, ref.when, expected
}
