package engine

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/downloader"
	dronebl "github.com/firehol/update-ipsets/tools/dronebl2ipsets"
)

func (e *Engine) fetchAndStageArtifact(ctx context.Context, name string, force, enableAll bool) (DownloadDecision, error) {
	return e.fetchAndStageArtifactWithSnapshot(ctx, e.operationSnapshot(), name, force, enableAll)
}

func (e *Engine) fetchAndStageArtifactWithSnapshot(ctx context.Context, snap operationSnapshot, name string, force, enableAll bool) (DownloadDecision, error) {
	artifact := snap.cfg.ArtifactByName(name)
	if artifact == nil {
		return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: "unknown artifact"}, fmt.Errorf("unknown artifact %q", name)
	}
	switch artifact.Type {
	case config.ArtifactTypeDroneBLBuildzone:
		return e.fetchAndStageDroneBLArtifactWithSnapshot(ctx, snap, artifact, force, enableAll)
	default:
		return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: "unsupported artifact type"}, fmt.Errorf("unsupported artifact type %q", artifact.Type)
	}
}

func artifactDownloadMaxSizeForRuntime(rt Runtime, artifact *config.Artifact) int64 {
	if artifact != nil && artifact.MaxDownloadSize != 0 {
		return artifact.MaxDownloadSize
	}
	return rt.MaxDownloadSize
}

func (e *Engine) fetchAndStageDroneBLArtifact(ctx context.Context, artifact *config.Artifact, force, enableAll bool) (DownloadDecision, error) {
	return e.fetchAndStageDroneBLArtifactWithSnapshot(ctx, e.operationSnapshot(), artifact, force, enableAll)
}

func (e *Engine) fetchAndStageDroneBLArtifactWithSnapshot(ctx context.Context, snap operationSnapshot, artifact *config.Artifact, force, enableAll bool) (DownloadDecision, error) {
	name := artifact.Name
	entry := e.state.Entry(name)
	e.seedEntryFromArtifactConfig(entry, name, artifact)
	entry.MarkDownloadStarted(e.now().UTC().Unix())

	fetchDir := filepath.Join(artifactRootDirForRuntime(snap.runtime, name), "fetch")
	buildzonePath := filepath.Join(fetchDir, "buildzone")
	rsyncURL := artifact.RSyncURL
	if rsyncURL == "" {
		rsyncURL = dronebl.DefaultRsyncURL
	}
	if err := dronebl.FetchBuildzone(ctx, dronebl.FetchOptions{
		RsyncURL: rsyncURL,
		DataDir:  fetchDir,
		Timeout:  snap.runtime.MaxDownloadTime,
	}); err != nil {
		e.incrementFailure(entry)
		entry.MarkDownloadFetchFailed(err.Error())
		return DownloadDecision{Name: name, Status: DownloadStatusDownloadFailed, Message: err.Error()}, err
	}

	finalPath := artifactSourcePathForRuntime(snap.runtime, name)
	result, err := snap.downloads.Fetch(ctx, downloader.Request{
		Name:            name,
		URL:             fileURL(buildzonePath),
		ReferencePath:   finalPath,
		MaxDownloadSize: artifactDownloadMaxSizeForRuntime(snap.runtime, artifact),
		TmpDir:          snap.runtime.TmpDir,
	})
	if err != nil {
		e.incrementFailure(entry)
		entry.MarkDownloadFetchFailed(err.Error())
		return DownloadDecision{Name: name, Status: DownloadStatusDownloadFailed, Message: err.Error()}, err
	}
	return e.applyArtifactFetchResultWithSnapshot(ctx, snap, artifact, entry, buildzonePath, result, force, enableAll)
}

func (e *Engine) applyArtifactFetchResult(ctx context.Context, artifact *config.Artifact, entry *cache.Entry, fetchedPath string, result *downloader.Result, force, enableAll bool) (DownloadDecision, error) {
	return e.applyArtifactFetchResultWithSnapshot(ctx, e.operationSnapshot(), artifact, entry, fetchedPath, result, force, enableAll)
}

func (e *Engine) applyArtifactFetchResultWithSnapshot(ctx context.Context, snap operationSnapshot, artifact *config.Artifact, entry *cache.Entry, fetchedPath string, result *downloader.Result, force, enableAll bool) (DownloadDecision, error) {
	name := artifact.Name
	finalPath := artifactSourcePathForRuntime(snap.runtime, name)
	if result == nil {
		return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: "empty downloader result"}, fmt.Errorf("empty downloader result for artifact %s", name)
	}
	entry.RecordDownloadSourceDate(result.ModifiedTime)
	switch result.Status {
	case downloader.StatusFailed:
		result.CleanUp()
		e.incrementFailure(entry)
		entry.MarkDownloadFetchFailed(result.Message)
		return DownloadDecision{Name: name, Status: DownloadStatusDownloadFailed, Message: result.Message}, errors.New(result.Message)
	case downloader.StatusSame:
		result.CleanUp()
		clearFailure(entry)
		if err := touchFileAt(finalPath, result.ModifiedTime); err != nil {
			entry.MarkDownloadOperationFailed(err.Error())
			return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: err.Error()}, err
		}
		entry.MarkDownloadSame()
		if !force {
			return DownloadDecision{Name: name, Status: DownloadStatusSame, Message: result.Message}, nil
		}
		decision, err := e.materializeArtifactChildrenWithSnapshot(ctx, snap, name, fetchedPath, enableAll, force)
		decision.Name = name
		decision.Status = DownloadStatusSame
		decision.Message = result.Message
		return decision, err
	case downloader.StatusOK:
		clearFailure(entry)
		if err := stageDownloadedBody(result, finalPath); err != nil {
			result.CleanUp()
			entry.MarkDownloadOperationFailed(err.Error())
			return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: err.Error()}, err
		}
		if err := touchFileAt(stagedPath(finalPath), result.ModifiedTime); err != nil {
			entry.MarkDownloadOperationFailed(err.Error())
			return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: err.Error()}, err
		}
		entry.MarkDownloadDownloaded()
		decision, err := e.materializeArtifactChildrenWithSnapshot(ctx, snap, name, stagedPath(finalPath), enableAll, force)
		if err != nil {
			return DownloadDecision{Name: name, Status: DownloadStatusDownloaded, Message: err.Error()}, err
		}
		decision.Name = name
		decision.Status = DownloadStatusDownloaded
		decision.Message = result.Message
		if len(decision.ProcessingNames) == 0 {
			if err := promoteStagedFile(finalPath); err != nil {
				return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: err.Error()}, err
			}
			return decision, nil
		}
		decision.PromoteNames = appendUniqueName(decision.PromoteNames, name)
		return decision, nil
	default:
		result.CleanUp()
		entry.MarkDownloadOperationFailed(result.Message)
		return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: result.Message}, errors.New(result.Message)
	}
}

func (e *Engine) materializeArtifactChildren(ctx context.Context, artifactName, rawPath string, enableAll, force bool) (DownloadDecision, error) {
	return e.materializeArtifactChildrenWithSnapshot(ctx, e.operationSnapshot(), artifactName, rawPath, enableAll, force)
}

func (e *Engine) materializeArtifactChildrenWithSnapshot(ctx context.Context, snap operationSnapshot, artifactName, rawPath string, enableAll, force bool) (DownloadDecision, error) {
	artifact := snap.cfg.ArtifactByName(artifactName)
	if artifact == nil {
		return DownloadDecision{Name: artifactName}, fmt.Errorf("unknown artifact %q", artifactName)
	}
	switch artifact.Type {
	case config.ArtifactTypeDroneBLBuildzone:
		return e.materializeDroneBLChildrenWithSnapshot(ctx, snap, artifact, rawPath, enableAll, force)
	default:
		return DownloadDecision{Name: artifactName}, fmt.Errorf("unsupported artifact type %q", artifact.Type)
	}
}

func (e *Engine) materializeDroneBLChildren(ctx context.Context, artifact *config.Artifact, rawPath string, enableAll, force bool) (DownloadDecision, error) {
	return e.materializeDroneBLChildrenWithSnapshot(ctx, e.operationSnapshot(), artifact, rawPath, enableAll, force)
}

func (e *Engine) materializeDroneBLChildrenWithSnapshot(ctx context.Context, snap operationSnapshot, artifact *config.Artifact, rawPath string, enableAll, force bool) (DownloadDecision, error) {
	name := artifact.Name
	specs := droneBLArtifactSpecsForSnapshot(snap, name, enableAll)
	if len(specs) == 0 {
		return DownloadDecision{Name: name}, nil
	}

	extractDir := artifactExtractDirForRuntime(snap.runtime, name)
	if err := os.MkdirAll(extractDir, generatedDirMode); err != nil {
		return DownloadDecision{Name: name}, err
	}
	if err := cleanupDroneBLExtractDir(extractDir); err != nil {
		return DownloadDecision{Name: name}, err
	}
	outDir, err := os.MkdirTemp(extractDir, "outputs-")
	if err != nil {
		return DownloadDecision{Name: name}, err
	}
	if err := os.Chmod(outDir, generatedDirMode); err != nil {
		_ = os.RemoveAll(outDir)
		return DownloadDecision{Name: name}, err
	}
	defer func() { _ = os.RemoveAll(outDir) }()

	report, err := dronebl.Update(ctx, dronebl.Options{
		WorkDir:       artifactRootDirForRuntime(snap.runtime, name),
		OutputDir:     outDir,
		BuildzonePath: rawPath,
		Timeout:       snap.runtime.MaxDownloadTime,
		SkipFetch:     true,
		Specs:         specs,
	})
	if err != nil {
		return DownloadDecision{Name: name}, err
	}
	for i, warning := range report.Warnings {
		if i == 20 {
			e.logger.Warn("artifact parse warnings suppressed", "artifact", name, "suppressed", len(report.Warnings)-20)
			break
		}
		e.logger.Warn("artifact parse warning", "artifact", name, "warning", warning)
	}

	decision := DownloadDecision{Name: name}
	for _, spec := range specs {
		childName := spec.Name
		src := snap.cfg.Sources[childName]
		if src == nil {
			return DownloadDecision{Name: name}, fmt.Errorf("artifact child %q not found", childName)
		}
		entry := e.state.Entry(childName)
		e.seedEntryFromSourceConfig(entry, childName, src)
		entry.MarkArtifactChildMaterializing(e.now().UTC().Unix())

		childOutputPath := filepath.Join(outDir, childName+".source")
		result, err := snap.downloads.Fetch(ctx, downloader.Request{
			Name:            childName,
			URL:             fileURL(childOutputPath),
			ReferencePath:   snap.sourcePath(childName),
			AcceptEmpty:     true,
			MaxDownloadSize: artifactDownloadMaxSizeForRuntime(snap.runtime, artifact),
			TmpDir:          snap.runtime.TmpDir,
		})
		if err != nil {
			e.incrementFailure(entry)
			entry.MarkDownloadFetchFailed(err.Error())
			return DownloadDecision{Name: name}, err
		}
		childDecision, err := e.applyRawFeedDownloadResultWithSnapshot(ctx, snap, entry, src, result, snap.sourcePath(childName), snap.feedBodyPath(childName), force, enableAll)
		if err != nil {
			return DownloadDecision{Name: name}, err
		}
		decision.ProcessingNames = appendUnique(decision.ProcessingNames, childDecision.ProcessingNames...)
		decision.PromoteNames = appendUnique(decision.PromoteNames, childDecision.PromoteNames...)
	}
	return decision, nil
}

func (e *Engine) droneBLArtifactSpecs(artifactName string, enableAll bool) []dronebl.OutputSpec {
	return droneBLArtifactSpecsForSnapshot(e.operationSnapshot(), artifactName, enableAll)
}

func droneBLArtifactSpecsForSnapshot(snap operationSnapshot, artifactName string, enableAll bool) []dronebl.OutputSpec {
	children := snap.cfg.ArtifactChildren(artifactName)
	specs := make([]dronebl.OutputSpec, 0, len(children))
	for _, src := range children {
		if src == nil || !EffectiveSourceEnabled(snap.cfg, snap.runtime, src.Name, enableAll) {
			continue
		}
		ref, err := config.ParseArtifactURL(src.URL)
		if err != nil || ref.Artifact != artifactName {
			continue
		}
		specs = append(specs, dronebl.OutputSpec{
			Name:  src.Name,
			Lists: append([]string(nil), ref.Parts...),
		})
	}
	sort.Slice(specs, func(i, j int) bool {
		return specs[i].Name < specs[j].Name
	})
	return specs
}

func appendUnique(values []string, extras ...string) []string {
	for _, extra := range extras {
		values = appendUniqueName(values, extra)
	}
	return values
}

func appendUniqueName(values []string, extra string) []string {
	if extra == "" {
		return values
	}
	for _, existing := range values {
		if existing == extra {
			return values
		}
	}
	return append(values, extra)
}

func fileURL(path string) string {
	return (&url.URL{Scheme: "file", Path: path}).String()
}

func cleanupDroneBLExtractDir(extractDir string) error {
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return fmt.Errorf("read DroneBL extract directory: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "outputs-") {
			continue
		}
		path := filepath.Join(extractDir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("remove stale DroneBL extract directory %s: %w", entry.Name(), err)
		}
	}
	return nil
}
