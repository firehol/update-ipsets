package engine

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s *entityIntegrityScanner) scanFeedSidecars() error {
	for _, name := range s.e.publicOutputNames() {
		if strings.TrimSpace(name) == "" {
			continue
		}
		if err := s.checkFeedSidecar(name); err != nil {
			return err
		}
	}
	return nil
}

func (s *entityIntegrityScanner) checkFeedSidecar(name string) error {
	sidecarPath := filepath.Join(s.e.entityFeedsDir(), name+".json")
	sidecarRefPath, sidecarRefTime, err := s.e.entityFeedSidecarReference(
		name,
		s.geoProvider,
		s.asnProvider,
		s.geoRefPath,
		s.geoRefTime,
		s.asnRefPath,
		s.asnRefTime,
	)
	if err != nil {
		return err
	}
	if sidecarRefTime.IsZero() {
		return nil
	}

	sidecarInfo, err := os.Stat(sidecarPath)
	if err != nil {
		return s.handleMissingFeedSidecar(err, name, sidecarPath, sidecarRefPath, sidecarRefTime)
	}
	sidecarMTime := sidecarInfo.ModTime().UTC()
	sidecar, err := s.e.loadFeedEntitySidecar(sidecarPath)
	if err != nil {
		s.findings = append(s.findings, EntityIntegrityFinding{
			Scope:        "feed",
			Kind:         "feed_sidecar_malformed",
			Subject:      name,
			Feed:         name,
			Path:         sidecarPath,
			PathMTime:    sidecarMTime,
			RepairAction: "refresh_feed",
			Reason:       "feed entity sidecar is unreadable",
		})
		s.plan.addFeed(name)
		return nil
	}
	if sidecarMTime.Before(sidecarRefTime) && !s.e.feedEntitySidecarCoversReference(name, sidecar, sidecarRefPath, sidecarRefTime) {
		s.findings = append(s.findings, EntityIntegrityFinding{
			Scope:          "feed",
			Kind:           "feed_sidecar_stale",
			Subject:        name,
			Feed:           name,
			Path:           sidecarPath,
			PathMTime:      sidecarMTime,
			ReferencePath:  sidecarRefPath,
			ReferenceMTime: sidecarRefTime,
			RepairAction:   "refresh_feed",
			Reason:         "feed entity sidecar is older than its local inputs",
		})
		s.plan.addFeed(name)
		return nil
	}

	s.recordEntityRefs(sidecarPath, sidecarMTime, sidecar)
	s.recordHealthCheck(name, sidecar)
	return nil
}

func (s *entityIntegrityScanner) handleMissingFeedSidecar(err error, name, sidecarPath, sidecarRefPath string, sidecarRefTime time.Time) error {
	if !os.IsNotExist(err) {
		return err
	}
	if !s.feedSidecarExpected(name) {
		return nil
	}
	s.findings = append(s.findings, EntityIntegrityFinding{
		Scope:          "feed",
		Kind:           "feed_sidecar_missing",
		Subject:        name,
		Feed:           name,
		Path:           sidecarPath,
		ReferencePath:  sidecarRefPath,
		ReferenceMTime: sidecarRefTime,
		RepairAction:   "refresh_feed",
		Reason:         "feed entity sidecar is missing",
	})
	s.plan.addFeed(name)
	return nil
}

func (s *entityIntegrityScanner) feedSidecarExpected(name string) bool {
	if s == nil || s.e == nil {
		return false
	}
	var resolver *effectiveEntryResolver
	if s.health != nil {
		resolver = s.health.resolver
	}
	return s.e.feedEntitySidecarExpected(name, s.geoProvider, s.asnProvider, resolver)
}

func (s *entityIntegrityScanner) recordEntityRefs(sidecarPath string, sidecarMTime time.Time, sidecar *feedEntitySidecar) {
	for _, code := range sidecar.countryCodes() {
		normalized := strings.ToUpper(strings.TrimSpace(code))
		ref := s.countryRefs[normalized]
		mergeEntityDependencyRef(&ref, sidecarPath, sidecarMTime)
		s.countryRefs[normalized] = ref
	}
	for _, asn := range sidecar.asnNumbers() {
		ref := s.asnRefs[asn]
		mergeEntityDependencyRef(&ref, sidecarPath, sidecarMTime)
		s.asnRefs[asn] = ref
	}
}

func (s *entityIntegrityScanner) recordHealthCheck(name string, sidecar *feedEntitySidecar) {
	transitionAt := s.e.entityFeedHealthTransitionTime(name, s.health)
	if transitionAt.IsZero() {
		return
	}
	healthClass := s.health.class(name)
	if strings.TrimSpace(healthClass) == "" {
		return
	}
	s.healthChecks = append(s.healthChecks, entityHealthCheck{
		feed:         name,
		healthClass:  healthClass,
		transitionAt: transitionAt,
		countries:    append([]string(nil), sidecar.countryCodes()...),
		asns:         append([]uint32(nil), sidecar.asnNumbers()...),
	})
}
