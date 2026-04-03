package engine

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (e *Engine) checkHomeAggregatesIntegrity(findings *[]EntityIntegrityFinding, plan *entityIntegrityPlan, health *feedHealthClassifier) error {
	if e == nil || e.cfg == nil || findings == nil || plan == nil {
		return nil
	}
	refPath, refTime, expected := e.homeAggregatesReferenceWithHealth(health)
	if !expected {
		return nil
	}
	path := e.PublicHomeAggregatesPath()
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
	if _, _, err := readHomeAggregatesFile(path); err != nil {
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
	if e == nil || e.cfg == nil {
		return "", time.Time{}, false
	}
	if health == nil {
		health = e.newFeedHealthClassifier()
	}
	var ref entityDependencyRef
	expected := false
	for _, entry := range e.EntriesSnapshot() {
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			continue
		}
		src := e.lookupSource(name)
		if !homeSummaryEligible(e.cfg, src, nil) {
			continue
		}
		expected = true
		refPath := filepath.Join(e.outputDir(), name+".json")
		if entry.ProcessedDate > 0 {
			mergeEntityDependencyRef(&ref, refPath, time.Unix(entry.ProcessedDate, 0).UTC())
		}
		if transitionAt := e.entityFeedHealthTransitionTime(name, health); !transitionAt.IsZero() {
			mergeEntityDependencyRef(&ref, refPath, transitionAt)
		}
	}
	return ref.path, ref.when, expected
}
