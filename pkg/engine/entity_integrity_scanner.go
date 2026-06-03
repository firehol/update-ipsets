package engine

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

type entityIntegrityScanner struct {
	e                   *Engine
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

func newEntityIntegrityScanner(e *Engine) *entityIntegrityScanner {
	return &entityIntegrityScanner{
		e:                   e,
		findings:            make([]EntityIntegrityFinding, 0),
		countryRefs:         map[string]entityDependencyRef{},
		asnRefs:             map[uint32]entityDependencyRef{},
		countryPublicHealth: map[string]map[string]string{},
		asnPublicHealth:     map[uint32]map[string]string{},
		healthChecks:        make([]entityHealthCheck, 0, 32),
		health:              e.newFeedHealthClassifier(),
	}
}

func (s *entityIntegrityScanner) run() error {
	done, err := s.checkGlobalPrerequisites()
	if done || err != nil {
		return err
	}
	if err := s.loadProviderReferences(); err != nil {
		return err
	}
	if err := s.scanFeedSidecars(); err != nil {
		return err
	}
	if err := s.scanCountryDetails(); err != nil {
		return err
	}
	if err := s.scanASNDetails(); err != nil {
		return err
	}
	if err := s.checkEntityIndexes(); err != nil {
		return err
	}
	if err := s.e.checkHomeAggregatesIntegrity(&s.findings, &s.plan, s.health); err != nil {
		return err
	}
	s.checkHealthDrift()
	s.sortFindings()
	return nil
}

func (s *entityIntegrityScanner) checkGlobalPrerequisites() (bool, error) {
	versionPath := s.e.entityVersionPath()
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
	versionData, err := readFileInRoot(s.e.entitiesDir(), "version")
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

	configPath, configTime, err := s.e.latestEntityConfigInputTime()
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
	return false, nil
}

func (s *entityIntegrityScanner) loadProviderReferences() error {
	s.geoProvider = s.e.preferredGeoProvider()
	s.asnProvider = s.e.preferredASNProvider()

	var err error
	s.geoRefPath, s.geoRefTime, err = s.e.entityGeoProviderReference(s.geoProvider)
	if err != nil {
		return err
	}
	s.asnRefPath, s.asnRefTime, err = s.e.entityASNProviderReference(s.asnProvider)
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
