package engine

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (s *entityIntegrityScanner) scanCountryDetails() error {
	for code, ref := range s.countryRefs {
		if err := s.checkCountryDetail(code, ref); err != nil {
			return err
		}
	}
	return nil
}

func (s *entityIntegrityScanner) checkCountryDetail(code string, ref entityDependencyRef) error {
	sidecarPath := filepath.Join(s.e.entityCountriesDir(), code+".json")
	sidecarInfo, err := os.Stat(sidecarPath)
	if err != nil {
		return s.handleMissingCountrySidecar(err, code, sidecarPath, ref)
	}
	sidecarMTime := sidecarInfo.ModTime().UTC()
	if _, err := loadCountryDetailSidecar(sidecarPath); err != nil {
		s.findings = append(s.findings, EntityIntegrityFinding{
			Scope:        "country",
			Kind:         "detail_sidecar_malformed",
			Subject:      code,
			Country:      code,
			Path:         sidecarPath,
			PathMTime:    sidecarMTime,
			RepairAction: "refresh_entity",
			Reason:       "country detail sidecar is unreadable",
		})
		s.plan.addCountry(code)
		return nil
	}
	return s.checkCountryPublicDetail(code, sidecarPath, sidecarMTime)
}

func (s *entityIntegrityScanner) handleMissingCountrySidecar(err error, code, sidecarPath string, ref entityDependencyRef) error {
	if !os.IsNotExist(err) {
		return err
	}
	s.findings = append(s.findings, EntityIntegrityFinding{
		Scope:          "country",
		Kind:           "detail_sidecar_missing",
		Subject:        code,
		Country:        code,
		Path:           sidecarPath,
		ReferencePath:  ref.path,
		ReferenceMTime: ref.when,
		RepairAction:   "refresh_entity",
		Reason:         "country detail sidecar is missing",
	})
	s.plan.addCountry(code)
	return nil
}

func (s *entityIntegrityScanner) checkCountryPublicDetail(code, sidecarPath string, sidecarMTime time.Time) error {
	publicPath := s.e.PublicCountryDetailPath(code)
	publicInfo, err := os.Stat(publicPath)
	if err != nil {
		if os.IsNotExist(err) {
			s.findings = append(s.findings, EntityIntegrityFinding{
				Scope:          "country",
				Kind:           "detail_public_missing",
				Subject:        code,
				Country:        code,
				Path:           publicPath,
				ReferencePath:  sidecarPath,
				ReferenceMTime: sidecarMTime,
				RepairAction:   "refresh_entity",
				Reason:         "country public JSON is missing",
			})
			s.plan.addCountry(code)
			return nil
		}
		return err
	}
	publicMTime := publicInfo.ModTime().UTC()
	healths, err := loadCountryDetailPublicHealth(s.e.outputDir(), s.e.publicCountryDetailRelPath(code))
	if err != nil {
		s.findings = append(s.findings, EntityIntegrityFinding{
			Scope:        "country",
			Kind:         "detail_public_malformed",
			Subject:      code,
			Country:      code,
			Path:         publicPath,
			PathMTime:    publicMTime,
			RepairAction: "refresh_entity",
			Reason:       "country public JSON is unreadable",
		})
		s.plan.addCountry(code)
		return nil
	}
	s.countryPublicHealth[code] = healths
	if publicMTime.Before(sidecarMTime) {
		s.findings = append(s.findings, EntityIntegrityFinding{
			Scope:          "country",
			Kind:           "detail_public_stale",
			Subject:        code,
			Country:        code,
			Path:           publicPath,
			PathMTime:      publicMTime,
			ReferencePath:  sidecarPath,
			ReferenceMTime: sidecarMTime,
			RepairAction:   "refresh_entity",
			Reason:         "country public JSON is older than its private sidecar",
		})
		s.plan.addCountry(code)
	}
	return nil
}

func (s *entityIntegrityScanner) scanASNDetails() error {
	for asn, ref := range s.asnRefs {
		if err := s.checkASNDetail(asn, ref); err != nil {
			return err
		}
	}
	return nil
}

func (s *entityIntegrityScanner) checkASNDetail(asn uint32, ref entityDependencyRef) error {
	subject := strconv.FormatUint(uint64(asn), 10)
	sidecarPath := filepath.Join(s.e.entityASNsDir(), subject+".json")
	sidecarInfo, err := os.Stat(sidecarPath)
	if err != nil {
		return s.handleMissingASNSidecar(err, asn, subject, sidecarPath, ref)
	}
	sidecarMTime := sidecarInfo.ModTime().UTC()
	if _, err := loadASNDetailSidecar(sidecarPath); err != nil {
		s.findings = append(s.findings, EntityIntegrityFinding{
			Scope:        "asn",
			Kind:         "detail_sidecar_malformed",
			Subject:      subject,
			ASN:          asn,
			Path:         sidecarPath,
			PathMTime:    sidecarMTime,
			RepairAction: "refresh_entity",
			Reason:       "ASN detail sidecar is unreadable",
		})
		s.plan.addASN(asn)
		return nil
	}
	return s.checkASNPublicDetail(asn, subject, sidecarPath, sidecarMTime)
}

func (s *entityIntegrityScanner) handleMissingASNSidecar(err error, asn uint32, subject, sidecarPath string, ref entityDependencyRef) error {
	if !os.IsNotExist(err) {
		return err
	}
	s.findings = append(s.findings, EntityIntegrityFinding{
		Scope:          "asn",
		Kind:           "detail_sidecar_missing",
		Subject:        subject,
		ASN:            asn,
		Path:           sidecarPath,
		ReferencePath:  ref.path,
		ReferenceMTime: ref.when,
		RepairAction:   "refresh_entity",
		Reason:         "ASN detail sidecar is missing",
	})
	s.plan.addASN(asn)
	return nil
}

func (s *entityIntegrityScanner) checkASNPublicDetail(asn uint32, subject, sidecarPath string, sidecarMTime time.Time) error {
	publicPath := s.e.PublicASNDetailPath(asn)
	publicInfo, err := os.Stat(publicPath)
	if err != nil {
		if os.IsNotExist(err) {
			s.findings = append(s.findings, EntityIntegrityFinding{
				Scope:          "asn",
				Kind:           "detail_public_missing",
				Subject:        subject,
				ASN:            asn,
				Path:           publicPath,
				ReferencePath:  sidecarPath,
				ReferenceMTime: sidecarMTime,
				RepairAction:   "refresh_entity",
				Reason:         "ASN public JSON is missing",
			})
			s.plan.addASN(asn)
			return nil
		}
		return err
	}
	publicMTime := publicInfo.ModTime().UTC()
	healths, err := loadASNDetailPublicHealth(s.e.outputDir(), s.e.publicASNDetailRelPath(asn))
	if err != nil {
		s.findings = append(s.findings, EntityIntegrityFinding{
			Scope:        "asn",
			Kind:         "detail_public_malformed",
			Subject:      subject,
			ASN:          asn,
			Path:         publicPath,
			PathMTime:    publicMTime,
			RepairAction: "refresh_entity",
			Reason:       "ASN public JSON is unreadable",
		})
		s.plan.addASN(asn)
		return nil
	}
	s.asnPublicHealth[asn] = healths
	if publicMTime.Before(sidecarMTime) {
		s.findings = append(s.findings, EntityIntegrityFinding{
			Scope:          "asn",
			Kind:           "detail_public_stale",
			Subject:        subject,
			ASN:            asn,
			Path:           publicPath,
			PathMTime:      publicMTime,
			ReferencePath:  sidecarPath,
			ReferenceMTime: sidecarMTime,
			RepairAction:   "refresh_entity",
			Reason:         "ASN public JSON is older than its private sidecar",
		})
		s.plan.addASN(asn)
	}
	return nil
}

func (s *entityIntegrityScanner) checkEntityIndexes() error {
	if err := s.checkCountryIndex(); err != nil {
		return err
	}
	return s.checkASNIndex()
}

func (s *entityIntegrityScanner) checkCountryIndex() error {
	countryIndexPath := s.e.PublicCountryIndexPath()
	if _, err := os.Stat(countryIndexPath); err != nil {
		if os.IsNotExist(err) {
			s.findings = append(s.findings, EntityIntegrityFinding{
				Scope:        "index",
				Kind:         "country_index_missing",
				Subject:      "countries",
				Path:         countryIndexPath,
				RepairAction: "refresh_index",
				Reason:       "country index JSON is missing",
			})
			s.plan.rebuildCountryIndex = true
			return nil
		}
		return err
	}
	return nil
}

func (s *entityIntegrityScanner) checkASNIndex() error {
	asnIndexPath := s.e.PublicASNIndexPath()
	if _, err := os.Stat(asnIndexPath); err != nil {
		if os.IsNotExist(err) {
			s.findings = append(s.findings, EntityIntegrityFinding{
				Scope:        "index",
				Kind:         "asn_index_missing",
				Subject:      "asns",
				Path:         asnIndexPath,
				RepairAction: "refresh_index",
				Reason:       "ASN index JSON is missing",
			})
			s.plan.rebuildASNIndex = true
			return nil
		}
		return err
	}
	return nil
}

func (s *entityIntegrityScanner) checkHealthDrift() {
	for _, check := range s.healthChecks {
		staleCountries, staleASNs := s.countStaleHealthTargets(check)
		if staleCountries == 0 && staleASNs == 0 {
			continue
		}
		s.findings = append(s.findings, EntityIntegrityFinding{
			Scope:             "feed",
			Kind:              "detail_health_stale",
			Subject:           check.feed,
			Feed:              check.feed,
			ReferenceMTime:    check.transitionAt,
			RepairAction:      "refresh_health",
			Reason:            "entity public payloads are older than the feed's current health transition",
			AffectedCountries: staleCountries,
			AffectedASNs:      staleASNs,
		})
		s.plan.addHealthFeed(check.feed)
	}
}

func (s *entityIntegrityScanner) countStaleHealthTargets(check entityHealthCheck) (int, int) {
	staleCountries := 0
	for _, code := range check.countries {
		normalized := strings.ToUpper(strings.TrimSpace(code))
		if _, pending := s.plan.countryCodes[normalized]; pending {
			continue
		}
		healths, ok := s.countryPublicHealth[normalized]
		if ok && healths[check.feed] != check.healthClass {
			staleCountries++
		}
	}

	staleASNs := 0
	for _, asn := range check.asns {
		if _, pending := s.plan.asns[asn]; pending {
			continue
		}
		healths, ok := s.asnPublicHealth[asn]
		if ok && healths[check.feed] != check.healthClass {
			staleASNs++
		}
	}
	return staleCountries, staleASNs
}
