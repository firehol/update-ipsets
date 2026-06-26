package engine

import (
	"context"
	"os"

	"github.com/firehol/update-ipsets/pkg/config"
)

type RecheckAdmission struct {
	Name         string
	Target       string
	Downloadable bool
}

type ReprocessAdmission struct {
	Name             string
	HasLocalState    bool
	ProviderDatabase bool
	ProcessingNames  []string
	PromoteNames     []string
}

type DownloadAdmission struct {
	Name         string
	Downloadable bool
}

func (e *Engine) IsDownloadable(name string) bool {
	return e.operationSnapshot().isDownloadable(name)
}

func (e *Engine) IsProviderDatabase(name string) bool {
	snap := e.operationSnapshot()
	if snap.cfg == nil {
		return false
	}
	src := snap.cfg.Sources[name]
	return src != nil && (src.HasUse(config.UseASN) || src.HasUse(config.UseGeoIP))
}

func (e *Engine) IsMerge(name string) bool {
	return e.operationSnapshot().isMerge(name)
}

func (e *Engine) IsHistoryDerivative(name string) bool {
	return e.operationSnapshot().isHistoryDerivative(name)
}

func (e *Engine) EffectiveScheduleMinutes(name string) int {
	snap := e.operationSnapshot()
	if snap.cfg == nil {
		return 0
	}
	src := snap.cfg.Sources[name]
	if src == nil {
		return 0
	}
	return src.Frequency
}

func (e *Engine) RecoverStagedSources() ([]string, error) {
	snap := e.operationSnapshot()
	names := make([]string, 0)
	for _, name := range config.SortedSourceNames(snap.cfg) {
		src := snap.cfg.Sources[name]
		if src == nil || !snap.isDownloadable(name) {
			continue
		}
		if src.HasUse(config.UseASN) || src.HasUse(config.UseGeoIP) {
			finalPath := providerArchivePathForRuntime(snap.runtime, name, src)
			_ = os.Remove(pendingTempPath(finalPath))
			if fileExists(stagedPath(finalPath)) {
				names = append(names, name)
			}
			continue
		}
		bodyPath := snap.feedBodyPath(name)
		_ = os.Remove(pendingTempPath(bodyPath))
		_ = os.Remove(pendingTempPath(snap.sourcePath(name)))
		if fileExists(stagedPath(bodyPath)) {
			if _, err := claimProcessingFeedBody(bodyPath); err != nil {
				return nil, err
			}
			names = append(names, name)
			continue
		}
		if fileExists(processingPath(bodyPath)) {
			names = append(names, name)
		}
	}
	return names, nil
}

func (e *Engine) PromoteCommittedDownloads(names []string) error {
	snap := e.operationSnapshot()
	promoted := false
	for _, name := range names {
		if !snap.isDownloadable(name) {
			continue
		}
		finalPath := ""
		if snap.cfg != nil && snap.cfg.ArtifactByName(name) != nil {
			finalPath = artifactSourcePathForRuntime(snap.runtime, name)
		} else {
			src := snap.cfg.Sources[name]
			if src == nil {
				continue
			}
			if src.HasUse(config.UseASN) || src.HasUse(config.UseGeoIP) {
				finalPath = providerArchivePathForRuntime(snap.runtime, name, src)
			} else {
				continue
			}
		}
		if err := promoteStagedFile(finalPath); err != nil {
			return err
		}
		promoted = true
	}
	if promoted {
		e.MarkIntegrityCachesStale()
	}
	return nil
}

func (e *Engine) HasStagedDownload(name string) bool {
	snap := e.operationSnapshot()
	return e.hasStagedDownloadWithSnapshot(snap, name)
}

func (e *Engine) hasStagedDownloadWithSnapshot(snap operationSnapshot, name string) bool {
	if !snap.isDownloadable(name) {
		return false
	}
	finalPath := ""
	if snap.cfg != nil && snap.cfg.ArtifactByName(name) != nil {
		finalPath = artifactSourcePathForRuntime(snap.runtime, name)
	} else {
		src := snap.cfg.Sources[name]
		if src == nil {
			return false
		}
		if src.HasUse(config.UseASN) || src.HasUse(config.UseGeoIP) {
			finalPath = providerArchivePathForRuntime(snap.runtime, name, src)
		} else {
			finalPath = snap.feedBodyPath(name)
		}
	}
	return fileExists(stagedPath(finalPath))
}

func (e *Engine) HasLocalFeedBody(name string) bool {
	snap := e.operationSnapshot()
	if e == nil || snap.cfg == nil || snap.cfg.Sources[name] == nil {
		return false
	}
	return fileExists(latestFeedBodyPath(snap.feedBodyPath(name)))
}

func (e *Engine) HasLocalReprocessState(name string) bool {
	snap := e.operationSnapshot()
	return e.hasLocalReprocessStateWithSnapshot(snap, name)
}

func (e *Engine) hasLocalReprocessStateWithSnapshot(snap operationSnapshot, name string) bool {
	if e == nil || snap.cfg == nil {
		return false
	}
	if snap.cfg.ArtifactByName(name) != nil {
		return fileExists(preferStagedPath(artifactSourcePathForRuntime(snap.runtime, name)))
	}
	src := snap.cfg.Sources[name]
	if src == nil {
		return false
	}
	if src.HasUse(config.UseASN) || src.HasUse(config.UseGeoIP) {
		return fileExists(preferStagedPath(providerArchivePathForRuntime(snap.runtime, name, src)))
	}
	return fileExists(latestFeedBodyPath(snap.feedBodyPath(name)))
}

func (e *Engine) ResolveRecheckTarget(ctx context.Context, name string) string {
	ctx = nonNilContext(ctx)
	snap := e.operationSnapshot()
	return e.resolveRecheckTargetWithSnapshot(ctx, snap, name)
}

func (e *Engine) ResolveRecheckAdmission(ctx context.Context, name string) RecheckAdmission {
	ctx = nonNilContext(ctx)
	snap := e.operationSnapshot()
	return e.resolveRecheckAdmissionWithSnapshot(ctx, snap, name)
}

func (e *Engine) ResolveRecheckAdmissions(ctx context.Context, names []string) []RecheckAdmission {
	ctx = nonNilContext(ctx)
	snap := e.operationSnapshot()
	admissions := make([]RecheckAdmission, 0, len(names))
	for _, name := range names {
		admissions = append(admissions, e.resolveRecheckAdmissionWithSnapshot(ctx, snap, name))
	}
	return admissions
}

func (e *Engine) resolveRecheckAdmissionWithSnapshot(ctx context.Context, snap operationSnapshot, name string) RecheckAdmission {
	if !snap.isDownloadable(name) {
		return RecheckAdmission{Name: name, Target: name}
	}
	return RecheckAdmission{
		Name:         name,
		Target:       e.resolveRecheckTargetWithSnapshot(ctx, snap, name),
		Downloadable: true,
	}
}

func (e *Engine) resolveRecheckTargetWithSnapshot(ctx context.Context, snap operationSnapshot, name string) string {
	if e == nil || snap.cfg == nil {
		return name
	}
	src := snap.cfg.Sources[name]
	if src == nil {
		return name
	}
	if snap.isHistoryDerivative(name) {
		if _, _, err := e.composeHistoryDerivativeBodyWithSnapshot(ctx, snap, src); err == nil {
			return name
		}
		if len(src.DerivedFrom) > 0 {
			return e.resolveRecheckTargetWithSnapshot(ctx, snap, src.DerivedFrom[0])
		}
		return name
	}
	if src.ArtifactParent == "" {
		return name
	}
	if fileExists(preferStagedPath(snap.sourcePath(name))) || fileExists(latestFeedBodyPath(snap.feedBodyPath(name))) {
		return name
	}
	if snap.cfg.ArtifactByName(src.ArtifactParent) != nil {
		return src.ArtifactParent
	}
	return name
}

func (e *Engine) ReprocessAdmission(name string, enableAll bool) ReprocessAdmission {
	snap := e.operationSnapshot()
	return e.reprocessAdmissionWithSnapshot(snap, name, enableAll)
}

func (e *Engine) ReprocessAdmissions(names []string, enableAll bool) []ReprocessAdmission {
	snap := e.operationSnapshot()
	if len(names) == 0 {
		names = config.SortedSourceNames(snap.cfg)
	}
	admissions := make([]ReprocessAdmission, 0, len(names))
	for _, name := range names {
		admissions = append(admissions, e.reprocessAdmissionWithSnapshot(snap, name, enableAll))
	}
	return admissions
}

func (e *Engine) DownloadAdmissions(names []string) []DownloadAdmission {
	snap := e.operationSnapshot()
	if len(names) == 0 {
		names = config.SortedSourceNames(snap.cfg)
	}
	admissions := make([]DownloadAdmission, 0, len(names))
	for _, name := range names {
		admissions = append(admissions, DownloadAdmission{
			Name:         name,
			Downloadable: snap.isDownloadable(name),
		})
	}
	return admissions
}

func (e *Engine) ProviderReprocessTargetsForSource(name string, enableAll bool) ([]string, bool) {
	snap := e.operationSnapshot()
	if e == nil || snap.cfg == nil {
		return nil, false
	}
	src := snap.cfg.Sources[name]
	if src == nil || !(src.HasUse(config.UseASN) || src.HasUse(config.UseGeoIP)) {
		return nil, false
	}
	return e.fullFeedReprocessTargetsWithSnapshot(snap, enableAll), true
}

func (e *Engine) reprocessAdmissionWithSnapshot(snap operationSnapshot, name string, enableAll bool) ReprocessAdmission {
	if e == nil || snap.cfg == nil {
		return ReprocessAdmission{}
	}
	if !e.hasLocalReprocessStateWithSnapshot(snap, name) {
		return ReprocessAdmission{Name: name}
	}
	src := snap.cfg.Sources[name]
	if src == nil {
		return ReprocessAdmission{Name: name}
	}
	if src.HasUse(config.UseASN) || src.HasUse(config.UseGeoIP) {
		admission := ReprocessAdmission{
			Name:             name,
			HasLocalState:    true,
			ProviderDatabase: true,
			ProcessingNames:  e.fullFeedReprocessTargetsWithSnapshot(snap, enableAll),
		}
		if e.hasStagedDownloadWithSnapshot(snap, name) {
			admission.PromoteNames = []string{name}
		}
		return admission
	}
	return ReprocessAdmission{
		Name:            name,
		HasLocalState:   true,
		ProcessingNames: []string{name},
	}
}
