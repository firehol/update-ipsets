package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/pkg/feedhealth"
)

func (e *Engine) entityFeedSidecarReference(name, geoProvider, asnProvider string, geoPath string, geoTime time.Time, asnPath string, asnTime time.Time) (string, time.Time, error) {
	return e.entityFeedSidecarReferenceInOutputDir(name, e.outputDir(), geoProvider, asnProvider, geoPath, geoTime, asnPath, asnTime)
}

func (e *Engine) entityFeedSidecarReferenceInOutputDir(name, outDir, geoProvider, asnProvider string, geoPath string, geoTime time.Time, asnPath string, asnTime time.Time) (string, time.Time, error) {
	var ref entityDependencyRef
	if geoProvider != "" {
		path, when, err := e.entityFeedCountryPayloadReferenceInOutputDir(outDir, name, geoProvider)
		if err != nil {
			return "", time.Time{}, err
		}
		mergeEntityDependencyRef(&ref, path, when)
	}
	if asnProvider != "" {
		path, when, err := e.entityFeedASNPayloadReferenceInOutputDir(outDir, name, asnProvider)
		if err != nil {
			return "", time.Time{}, err
		}
		mergeEntityDependencyRef(&ref, path, when)
	}
	latestPath, latestTime, err := e.entityLatestSetReference(name)
	if err != nil {
		return "", time.Time{}, err
	}
	mergeEntityDependencyRef(&ref, latestPath, latestTime)
	mergeEntityDependencyRef(&ref, geoPath, geoTime)
	mergeEntityDependencyRef(&ref, asnPath, asnTime)
	return ref.path, ref.when, nil
}

func (e *Engine) feedEntitySidecarCoversReference(name string, sidecar *feedEntitySidecar, refPath string, refTime time.Time) bool {
	if sidecar == nil || sidecar.LastChangeTS <= 0 || refTime.IsZero() {
		return false
	}
	refPath = filepath.Clean(strings.TrimSpace(refPath))
	if refPath == "" {
		return false
	}
	candidates := []string{
		filepath.Join(e.runtime.LibDir, name, "latest"),
		filepath.Join(e.runtime.LibDir, name, "latest.set"),
		latestFeedBodyPath(e.feedBodyPath(name)),
	}
	var isLatestSetRef bool
	for _, candidate := range candidates {
		if refPath == filepath.Clean(candidate) {
			isLatestSetRef = true
			break
		}
	}
	if !isLatestSetRef {
		return false
	}
	sidecarSourceTime := time.Unix(sidecar.LastChangeTS, 0).UTC()
	return !sidecarSourceTime.Before(refTime.UTC().Truncate(time.Second))
}

func (e *Engine) feedEntityInputsHaveContributions(name, geoProvider, asnProvider string) (bool, error) {
	view := newEntityOutputView(e, "")
	if geoProvider != "" {
		payload, err := view.countryComparison(name, geoProvider)
		if err != nil {
			if !os.IsNotExist(err) {
				return false, err
			}
		} else {
			for _, country := range payload.Countries {
				if strings.TrimSpace(country.Code) != "" && country.Value > 0 {
					return true, nil
				}
			}
		}
	}
	if asnProvider != "" {
		data, err := readFirstExisting(asnCandidatePaths(e.outputDir(), name, asnProvider))
		if err != nil {
			if !os.IsNotExist(err) {
				return false, err
			}
			return false, nil
		}
		var payload struct {
			ByASN []struct {
				ASN   uint32 `json:"asn"`
				Count uint64 `json:"count"`
			} `json:"by_asn"`
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			return false, err
		}
		for _, row := range payload.ByASN {
			if row.ASN != 0 && row.Count > 0 {
				return true, nil
			}
		}
	}
	return false, nil
}

func (e *Engine) entityFeedCountryPayloadReferenceInOutputDir(outDir, name, provider string) (string, time.Time, error) {
	if provider == "" {
		return "", time.Time{}, nil
	}
	if strings.TrimSpace(outDir) == "" {
		outDir = e.outputDir()
	}
	path := filepath.Join(outDir, name+"_"+provider+".json")
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return path, time.Time{}, nil
		}
		return "", time.Time{}, err
	}
	return path, info.ModTime().UTC(), nil
}

func (e *Engine) entityFeedASNPayloadReferenceInOutputDir(outDir, name, provider string) (string, time.Time, error) {
	if provider == "" {
		return "", time.Time{}, nil
	}
	if strings.TrimSpace(outDir) == "" {
		outDir = e.outputDir()
	}
	path := filepath.Join(outDir, name+"_asn_"+provider+".json")
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return path, time.Time{}, nil
		}
		return "", time.Time{}, err
	}
	return path, info.ModTime().UTC(), nil
}

func (e *Engine) entityGeoProviderReference(provider string) (string, time.Time, error) {
	if provider == "" {
		return "", time.Time{}, nil
	}
	path := filepath.Join(e.runtime.LibDir, "geolocation", provider+".source")
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return path, time.Time{}, nil
		}
		return "", time.Time{}, err
	}
	return path, info.ModTime().UTC(), nil
}

func (e *Engine) entityASNProviderReference(provider string) (string, time.Time, error) {
	if provider == "" {
		return "", time.Time{}, nil
	}
	src := e.lookupSource(provider)
	if src == nil {
		return "", time.Time{}, nil
	}
	spec, ok := lookupFormat(src.Format)
	if !ok {
		return "", time.Time{}, nil
	}
	path := filepath.Join(e.runtime.LibDir, "asn", provider, spec.dataFile)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return path, time.Time{}, nil
		}
		return "", time.Time{}, err
	}
	return path, info.ModTime().UTC(), nil
}

func (e *Engine) entityLatestSetReference(name string) (string, time.Time, error) {
	for _, filename := range []string{"latest", "latest.set"} {
		path := filepath.Join(e.runtime.LibDir, name, filename)
		info, err := os.Stat(path)
		if err == nil {
			return path, info.ModTime().UTC(), nil
		}
		if err != nil && !os.IsNotExist(err) {
			return "", time.Time{}, err
		}
	}
	path := latestFeedBodyPath(e.feedBodyPath(name))
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return path, time.Time{}, nil
		}
		return "", time.Time{}, err
	}
	return path, info.ModTime().UTC(), nil
}

func (e *Engine) latestEntityConfigInputTime() (string, time.Time, error) {
	var ref entityDependencyRef
	if e.runtime.ConfigPath != "" {
		info, err := os.Stat(e.runtime.ConfigPath)
		if err != nil {
			if !os.IsNotExist(err) {
				return "", time.Time{}, err
			}
		} else {
			mergeEntityDependencyRef(&ref, e.runtime.ConfigPath, info.ModTime().UTC())
		}
	}
	for _, dir := range []string{e.runtime.DistributionSuppliedIPSets, e.runtime.AdminSuppliedIPSets, e.runtime.UserSuppliedIPSets} {
		path, when, err := newestConfigFragmentTime(dir)
		if err != nil {
			return "", time.Time{}, err
		}
		mergeEntityDependencyRef(&ref, path, when)
	}
	return ref.path, ref.when, nil
}

func newestConfigFragmentTime(dir string) (string, time.Time, error) {
	if strings.TrimSpace(dir) == "" {
		return "", time.Time{}, nil
	}
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", time.Time{}, nil
		}
		return "", time.Time{}, err
	}
	if !info.IsDir() {
		return "", time.Time{}, fmt.Errorf("%s is not a directory", dir)
	}
	var ref entityDependencyRef
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", time.Time{}, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		switch strings.ToLower(filepath.Ext(entry.Name())) {
		case ".yaml", ".yml", ".conf":
		default:
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return "", time.Time{}, err
		}
		mergeEntityDependencyRef(&ref, path, info.ModTime().UTC())
	}
	return ref.path, ref.when, nil
}

func (e *Engine) entityFeedHealthTransitionTime(name string, health *feedHealthClassifier) time.Time {
	if e == nil || e.cfg == nil {
		return time.Time{}
	}
	snap, ok := health.snapshot(name)
	if !ok {
		return time.Time{}
	}
	now := e.now().UTC()
	switch snap.Class {
	case feedhealth.ClassDelayed:
		return transitionAfterLastChange(snap.LastChangeAt, snap.EffectiveHealthyGapMins, now)
	case feedhealth.ClassRisky:
		return transitionAfterLastChange(snap.LastChangeAt, snap.RiskyCadenceMins, now)
	case feedhealth.ClassUnmaintained:
		return transitionAfterLastChange(snap.LastChangeAt, snap.UnmaintainedThresholdMins, now)
	case feedhealth.ClassUnavailable:
		return entityUnavailableTransitionTime(snap, now)
	case feedhealth.ClassArchived:
		return entityArchivedTransitionTime(snap, now)
	default:
		return time.Time{}
	}
}

func transitionAfterLastChange(lastChangeAt int64, minutes int, now time.Time) time.Time {
	if lastChangeAt <= 0 || minutes <= 0 {
		return time.Time{}
	}
	transition := time.Unix(lastChangeAt, 0).UTC().Add(time.Duration(minutes) * time.Minute)
	if transition.After(now) {
		return time.Time{}
	}
	return transition
}

func entityUnavailableTransitionTime(snap feedhealth.Snapshot, now time.Time) time.Time {
	threshold := entityUnavailableThresholdMins(snap)
	if threshold <= 0 {
		return time.Time{}
	}
	candidates := make([]time.Time, 0, 2)
	if snap.FailureStartedAt > 0 {
		candidates = append(candidates, time.Unix(snap.FailureStartedAt, 0).UTC().Add(time.Duration(threshold)*time.Minute))
	}
	if snap.LastChangeAt > 0 {
		candidates = append(candidates, time.Unix(snap.LastChangeAt, 0).UTC().Add(time.Duration(threshold)*time.Minute))
	}
	return earliestPastTime(candidates, now)
}

func entityArchivedTransitionTime(snap feedhealth.Snapshot, now time.Time) time.Time {
	threshold := entityUnavailableThresholdMins(snap)
	if threshold <= 0 || snap.ArchivalThresholdMins <= 0 {
		return time.Time{}
	}
	candidates := make([]time.Time, 0, 2)
	if snap.FailureStartedAt > 0 {
		candidates = append(candidates, time.Unix(snap.FailureStartedAt, 0).UTC().Add(time.Duration(threshold+snap.ArchivalThresholdMins)*time.Minute))
	}
	if snap.LastChangeAt > 0 {
		candidates = append(candidates, time.Unix(snap.LastChangeAt, 0).UTC().Add(time.Duration(threshold+snap.ArchivalThresholdMins)*time.Minute))
	}
	return earliestPastTime(candidates, now)
}

func earliestPastTime(candidates []time.Time, now time.Time) time.Time {
	var earliest time.Time
	for _, candidate := range candidates {
		if candidate.IsZero() || candidate.After(now) {
			continue
		}
		if earliest.IsZero() || candidate.Before(earliest) {
			earliest = candidate
		}
	}
	return earliest
}

func entityUnavailableThresholdMins(snap feedhealth.Snapshot) int {
	threshold := snap.UnmaintainedThresholdMins
	if snap.ObservedUpdates <= 1 && snap.SingleObservationGraceMins > threshold {
		threshold = snap.SingleObservationGraceMins
	}
	return threshold
}

func mergeEntityDependencyRef(dst *entityDependencyRef, path string, when time.Time) {
	if dst == nil || when.IsZero() {
		return
	}
	when = when.UTC()
	if dst.when.IsZero() || when.After(dst.when) {
		dst.path = path
		dst.when = when
	}
}

func loadCountryDetailPublicHealth(rootDir, rel string) (map[string]string, error) {
	data, err := readFileInRoot(rootDir, rel)
	if err != nil {
		return nil, err
	}
	var payload CountryDetailPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	healths := make(map[string]string, len(payload.Feeds))
	for _, row := range payload.Feeds {
		if strings.TrimSpace(row.Name) == "" {
			continue
		}
		healths[row.Name] = row.HealthClass
	}
	return healths, nil
}

func loadASNDetailPublicHealth(rootDir, rel string) (map[string]string, error) {
	data, err := readFileInRoot(rootDir, rel)
	if err != nil {
		return nil, err
	}
	var payload ASNDetailPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	healths := make(map[string]string, len(payload.Feeds))
	for _, row := range payload.Feeds {
		if strings.TrimSpace(row.Name) == "" {
			continue
		}
		healths[row.Name] = row.HealthClass
	}
	return healths, nil
}
