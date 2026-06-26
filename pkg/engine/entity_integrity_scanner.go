package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
)

type entityIntegrityScanner struct {
	ctx                 context.Context
	e                   *Engine
	snapshot            operationSnapshot
	findings            []EntityIntegrityFinding
	plan                entityIntegrityPlan
	geoProvider         string
	asnProvider         string
	geoRefPath          string
	geoRefTime          time.Time
	asnRefPath          string
	asnRefTime          time.Time
	countryRefs         map[string]entityDependencyRef
	asnRefs             map[uint32]entityDependencyRef
	countryPublicHealth map[string]map[string]string
	asnPublicHealth     map[uint32]map[string]string
	healthChecks        []entityHealthCheck
	health              *feedHealthClassifier
}

func newEntityIntegrityScanner(ctx context.Context, e *Engine) *entityIntegrityScanner {
	snap := e.operationSnapshot()
	var entries map[string]cache.Entry
	if e != nil && e.state != nil {
		entries = e.state.SnapshotEntries()
	}
	now := time.Now().UTC()
	if e != nil && e.now != nil {
		now = e.now().UTC()
	}
	return &entityIntegrityScanner{
		ctx:                 nonNilContext(ctx),
		e:                   e,
		snapshot:            snap,
		findings:            make([]EntityIntegrityFinding, 0),
		countryRefs:         map[string]entityDependencyRef{},
		asnRefs:             map[uint32]entityDependencyRef{},
		countryPublicHealth: map[string]map[string]string{},
		asnPublicHealth:     map[uint32]map[string]string{},
		healthChecks:        make([]entityHealthCheck, 0, 32),
		health:              e.newFeedHealthClassifierForConfigPolicy(snap.cfg, snap.feedHealthPolicy, entries, now),
	}
}

func (s *entityIntegrityScanner) run() error {
	if err := s.checkContext(); err != nil {
		return err
	}
	done, err := s.checkGlobalPrerequisites()
	if done || err != nil {
		return err
	}
	if err := s.checkContext(); err != nil {
		return err
	}
	if err := s.loadProviderReferences(); err != nil {
		return err
	}
	if err := s.checkContext(); err != nil {
		return err
	}
	if err := s.scanFeedSidecars(); err != nil {
		return err
	}
	if err := s.checkContext(); err != nil {
		return err
	}
	if err := s.scanCountryDetails(); err != nil {
		return err
	}
	if err := s.checkContext(); err != nil {
		return err
	}
	if err := s.scanASNDetails(); err != nil {
		return err
	}
	if err := s.checkContext(); err != nil {
		return err
	}
	if err := s.checkEntityIndexes(); err != nil {
		return err
	}
	if err := s.e.checkHomeAggregatesIntegrityWithSnapshot(s.snapshot, &s.findings, &s.plan, s.health); err != nil {
		return err
	}
	if err := s.checkHealthDrift(); err != nil {
		return err
	}
	s.sortFindings()
	return nil
}

func (s *entityIntegrityScanner) checkContext() error {
	if s == nil {
		return nil
	}
	return contextErr(s.ctx)
}

func (s *entityIntegrityScanner) checkGlobalPrerequisites() (bool, error) {
	versionPath := entityVersionPathForRuntime(s.snapshot.runtime)
	versionInfo, err := os.Stat(versionPath)
	if err != nil {
		if os.IsNotExist(err) {
			s.plan.markFull()
			s.findings = append(s.findings, EntityIntegrityFinding{
				Scope:        "global",
				Kind:         "version_missing",
				Subject:      "entity_artifacts",
				Path:         versionPath,
				RepairAction: "full_rebuild",
				Reason:       "entity artifact version marker is missing",
			})
			return true, nil
		}
		return false, err
	}
	versionData, err := readFileInRoot(entitiesDirForRuntime(s.snapshot.runtime), "version")
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(string(versionData)) != entityArtifactsVersion {
		s.plan.markFull()
		s.findings = append(s.findings, EntityIntegrityFinding{
			Scope:        "global",
			Kind:         "version_mismatch",
			Subject:      "entity_artifacts",
			Path:         versionPath,
			PathMTime:    versionInfo.ModTime().UTC(),
			RepairAction: "full_rebuild",
			Reason:       fmt.Sprintf("entity artifact version is stale; want %s", entityArtifactsVersion),
		})
		return true, nil
	}

	configPath, configTime, err := s.e.latestEntityConfigInputTimeWithSnapshot(s.snapshot)
	if err != nil {
		return false, err
	}
	if !configTime.IsZero() && versionInfo.ModTime().UTC().Before(configTime) {
		s.plan.markFull()
		s.findings = append(s.findings, EntityIntegrityFinding{
			Scope:          "global",
			Kind:           "config_newer",
			Subject:        "entity_artifacts",
			Path:           versionPath,
			PathMTime:      versionInfo.ModTime().UTC(),
			ReferencePath:  configPath,
			ReferenceMTime: configTime,
			RepairAction:   "full_rebuild",
			Reason:         "entity artifacts are older than the loaded catalog inputs",
		})
		return true, nil
	}
	if _, _, err := loadEntityFeedPresenceIndexForRuntime(s.snapshot.runtime); err != nil {
		s.plan.markFull()
		kind := "feed_presence_index_malformed"
		reason := "entity feed presence index is unreadable"
		if errors.Is(err, errEntityFeedPresenceIndexMiss) {
			kind = "feed_presence_index_missing"
			reason = "entity feed presence index is missing"
		}
		s.findings = append(s.findings, EntityIntegrityFinding{
			Scope:        "global",
			Kind:         kind,
			Subject:      "entity_artifacts",
			Path:         entityFeedPresenceIndexPathForRuntime(s.snapshot.runtime),
			RepairAction: "full_rebuild",
			Reason:       reason,
		})
		return true, nil
	}
	return false, nil
}

func (s *entityIntegrityScanner) loadProviderReferences() error {
	s.geoProvider = preferredGeoProviderForConfig(s.snapshot.cfg)
	s.asnProvider = preferredASNProviderForConfig(s.snapshot.cfg)

	var err error
	s.geoRefPath, s.geoRefTime, err = s.e.entityGeoProviderReferenceWithSnapshot(s.snapshot, s.geoProvider)
	if err != nil {
		return err
	}
	s.asnRefPath, s.asnRefTime, err = s.e.entityASNProviderReferenceWithSnapshot(s.snapshot, s.asnProvider)
	return err
}

func (s *entityIntegrityScanner) sortFindings() {
	sort.Slice(s.findings, func(i, j int) bool {
		if s.findings[i].Scope != s.findings[j].Scope {
			return s.findings[i].Scope < s.findings[j].Scope
		}
		if s.findings[i].Subject != s.findings[j].Subject {
			return s.findings[i].Subject < s.findings[j].Subject
		}
		return s.findings[i].Kind < s.findings[j].Kind
	})
}
