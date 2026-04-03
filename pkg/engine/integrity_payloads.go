package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/firehol/update-ipsets/pkg/config"
)

type secondaryArtifactKind string

const (
	secondaryArtifactMetadata          secondaryArtifactKind = "metadata"
	secondaryArtifactHistory           secondaryArtifactKind = "history"
	secondaryArtifactChangesets        secondaryArtifactKind = "changesets"
	secondaryArtifactRetention         secondaryArtifactKind = "retention"
	secondaryArtifactComparison        secondaryArtifactKind = "comparison"
	secondaryArtifactInsights          secondaryArtifactKind = "insights"
	secondaryArtifactGeo               secondaryArtifactKind = "geo"
	secondaryArtifactASN               secondaryArtifactKind = "asn"
	secondaryArtifactBogons            secondaryArtifactKind = "bogons"
	secondaryArtifactCriticalAggregate secondaryArtifactKind = "critical_aggregate"
	secondaryArtifactCriticalProvider  secondaryArtifactKind = "critical_provider"
)

type secondaryArtifactDescriptor struct {
	RelPath  string
	Kind     secondaryArtifactKind
	Feed     string
	Provider string
}

func validateStructuredSecondary(relPath, path string, geoFiles map[string]struct{}, eng *Engine) error {
	if desc, ok := describeSecondaryArtifact(relPath, eng); ok {
		return validateStructuredSecondaryArtifact(desc, path, eng)
	}
	if _, ok := geoFiles[relPath]; ok {
		return validateStructuredSecondaryArtifact(secondaryArtifactDescriptor{RelPath: relPath, Kind: secondaryArtifactGeo}, path, eng)
	}
	return validateStructuredSecondaryArtifact(secondaryArtifactDescriptor{RelPath: relPath, Kind: secondaryArtifactKindFromRelPath(relPath)}, path, eng)
}

func validateStructuredSecondaryArtifact(desc secondaryArtifactDescriptor, path string, eng *Engine) error {
	switch desc.Kind {
	case secondaryArtifactGeo:
		_, err := loadCountryComparisonPayload(path)
		return err
	case secondaryArtifactComparison:
		return validateComparisonPayload(path)
	case secondaryArtifactRetention:
		return validateJSONFile[RetentionData](path)
	case secondaryArtifactInsights:
		return validateJSONFile[insightsPayload](path)
	case secondaryArtifactASN:
		return validateJSONFile[asnFeedJSON](path)
	case secondaryArtifactBogons:
		return validateJSONFile[bogonFeedJSON](path)
	case secondaryArtifactCriticalAggregate:
		return validateCriticalAggregateArtifact(path, eng)
	case secondaryArtifactCriticalProvider:
		return validateCriticalProviderArtifact(path, eng)
	case secondaryArtifactMetadata:
		return validateJSONFile[setMetadata](path)
	default:
		return nil
	}
}

func secondaryArtifactKindFromRelPath(relPath string) secondaryArtifactKind {
	switch {
	case strings.HasSuffix(relPath, "_comparison.json"):
		return secondaryArtifactComparison
	case strings.HasSuffix(relPath, "_retention.json"):
		return secondaryArtifactRetention
	case strings.HasSuffix(relPath, "_insights.json"):
		return secondaryArtifactInsights
	case strings.HasSuffix(relPath, ".json"):
		return secondaryArtifactMetadata
	default:
		return ""
	}
}

func describeSecondaryArtifact(relPath string, eng *Engine) (secondaryArtifactDescriptor, bool) {
	if eng == nil || eng.cfg == nil {
		return secondaryArtifactDescriptor{}, false
	}
	relPath = strings.TrimSpace(relPath)
	base, isJSON := strings.CutSuffix(relPath, ".json")
	if isJSON && eng.IsPublicFeedName(base) {
		return secondaryArtifactDescriptor{RelPath: relPath, Kind: secondaryArtifactMetadata, Feed: base}, true
	}
	if feed, ok := strings.CutSuffix(base, "_comparison"); isJSON && ok && eng.IsPublicFeedName(feed) {
		return secondaryArtifactDescriptor{RelPath: relPath, Kind: secondaryArtifactComparison, Feed: feed}, true
	}
	if feed, ok := strings.CutSuffix(base, "_retention"); isJSON && ok && eng.IsPublicFeedName(feed) {
		return secondaryArtifactDescriptor{RelPath: relPath, Kind: secondaryArtifactRetention, Feed: feed}, true
	}
	if feed, ok := strings.CutSuffix(base, "_insights"); isJSON && ok && eng.IsPublicFeedName(feed) {
		return secondaryArtifactDescriptor{RelPath: relPath, Kind: secondaryArtifactInsights, Feed: feed}, true
	}
	if strings.HasSuffix(relPath, "_history.csv") {
		if feed, ok := strings.CutSuffix(relPath, "_history.csv"); ok && eng.IsPublicFeedName(feed) {
			return secondaryArtifactDescriptor{RelPath: relPath, Kind: secondaryArtifactHistory, Feed: feed}, true
		}
	}
	if strings.HasSuffix(relPath, "_changesets.csv") {
		if feed, ok := strings.CutSuffix(relPath, "_changesets.csv"); ok && eng.IsPublicFeedName(feed) {
			return secondaryArtifactDescriptor{RelPath: relPath, Kind: secondaryArtifactChangesets, Feed: feed}, true
		}
	}
	if !isJSON {
		return secondaryArtifactDescriptor{}, false
	}
	for _, provider := range eng.cfg.SourcesWithUse(config.UseGeoIP) {
		if provider == nil {
			continue
		}
		suffix := "_" + provider.Name
		if feed, ok := strings.CutSuffix(base, suffix); ok && eng.IsPublicFeedName(feed) {
			return secondaryArtifactDescriptor{RelPath: relPath, Kind: secondaryArtifactGeo, Feed: feed, Provider: provider.Name}, true
		}
	}
	for _, provider := range eng.cfg.SourcesWithUse(config.UseASN) {
		if provider == nil {
			continue
		}
		suffix := "_asn_" + provider.Name
		if feed, ok := strings.CutSuffix(base, suffix); ok && eng.IsPublicFeedName(feed) {
			return secondaryArtifactDescriptor{RelPath: relPath, Kind: secondaryArtifactASN, Feed: feed, Provider: provider.Name}, true
		}
	}
	for _, provider := range eng.cfg.SourcesWithUse(config.UseBogons) {
		if provider == nil {
			continue
		}
		suffix := "_bogons_" + provider.Name
		if feed, ok := strings.CutSuffix(base, suffix); ok && eng.IsPublicFeedName(feed) {
			return secondaryArtifactDescriptor{RelPath: relPath, Kind: secondaryArtifactBogons, Feed: feed, Provider: provider.Name}, true
		}
	}
	if feed, ok := strings.CutSuffix(base, "_critical_infrastructure"); ok && eng.IsPublicFeedName(feed) && !eng.IsPublicFeedName(base) {
		return secondaryArtifactDescriptor{RelPath: relPath, Kind: secondaryArtifactCriticalAggregate, Feed: feed}, true
	}
	for _, provider := range eng.cfg.SourcesWithUse(config.UseCriticalInfrastructure) {
		if provider == nil {
			continue
		}
		suffix := "_critical_" + provider.Name
		if feed, ok := strings.CutSuffix(base, suffix); ok && eng.IsPublicFeedName(feed) && !eng.IsPublicFeedName(base) {
			return secondaryArtifactDescriptor{RelPath: relPath, Kind: secondaryArtifactCriticalProvider, Feed: feed, Provider: provider.Name}, true
		}
	}
	return secondaryArtifactDescriptor{}, false
}

func validateCriticalAggregateArtifact(path string, eng *Engine) error {
	if err := validateJSONFile[criticalAggregateJSON](path); err != nil {
		return err
	}
	if eng == nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var payload struct {
		ProviderSetID string `json:"provider_set_id"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	if payload.ProviderSetID == "" || payload.ProviderSetID != eng.CriticalInfrastructureProviderSetID() {
		return fmt.Errorf("critical aggregate provider_set_id is stale")
	}
	return nil
}

func validateCriticalProviderArtifact(path string, eng *Engine) error {
	if err := validateJSONFile[criticalProviderOverlapJSON](path); err != nil {
		return err
	}
	if eng == nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var payload struct {
		ProviderSetID string `json:"provider_set_id"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	if payload.ProviderSetID == "" || payload.ProviderSetID != eng.CriticalInfrastructureProviderSetID() {
		return fmt.Errorf("critical provider provider_set_id is stale")
	}
	return nil
}

func validateComparisonPayload(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var rows []CompareRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return err
	}
	for _, row := range rows {
		if row.Common == 0 {
			return fmt.Errorf("comparison row %q has zero overlap", row.Name)
		}
	}
	return nil
}

func validateJSONFile[T any](path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var payload T
	return json.Unmarshal(data, &payload)
}

func validatePublicMetadataArtifactPolicy(path string, rawAllowed bool) error {
	if rawAllowed {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var payload setMetadata
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	if payload.Source != "" || payload.File != "" || payload.FileLocal != "" || payload.CommitHistory != "" {
		return fmt.Errorf("metadata exposes raw/source fields for non-raw feed")
	}
	return nil
}
