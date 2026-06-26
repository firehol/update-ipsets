package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
)

var ErrRecoveredArtifactCorrupt = errors.New("recovered artifact is corrupt")

type recoveredArtifactCorruptionSidecar struct {
	Name            string    `json:"name"`
	Artifact        string    `json:"artifact"`
	CorruptionClass string    `json:"corruption_class"`
	Timestamp       time.Time `json:"timestamp"`
}

func (e *Engine) RecoverStagedArtifacts(ctx context.Context) ([]string, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	snap := e.operationSnapshot()
	var out []string
	for _, name := range config.SortedArtifactNames(snap.cfg) {
		if err := contextErr(ctx); err != nil {
			return nil, err
		}
		if snap.cfg == nil || snap.cfg.ArtifactByName(name) == nil {
			continue
		}
		finalPath := artifactSourcePathForRuntime(snap.runtime, name)
		if err := os.Remove(pendingTempPath(finalPath)); err != nil && !os.IsNotExist(err) && e.logger != nil {
			e.logger.Warn("failed to remove pending artifact temp file during recovery scan", "artifact", name, "error", err)
		}
		stagePath := stagedPath(finalPath)
		if !fileExists(stagePath) {
			continue
		}
		out = append(out, name)
	}
	return out, nil
}

func (e *Engine) RecoverStagedArtifact(ctx context.Context, name string, enableAll bool) (DownloadDecision, error) {
	if err := contextErr(ctx); err != nil {
		return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: err.Error()}, err
	}
	snap := e.operationSnapshot()
	if snap.cfg == nil || snap.cfg.ArtifactByName(name) == nil {
		return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: "unknown artifact"}, fmt.Errorf("unknown artifact %q", name)
	}
	finalPath := artifactSourcePathForRuntime(snap.runtime, name)
	stagePath := stagedPath(finalPath)
	if !fileExists(stagePath) {
		return DownloadDecision{Name: name, Status: DownloadStatusSkipped, Message: "no staged artifact source"}, nil
	}
	decision, err := e.materializeArtifactChildrenWithSnapshot(ctx, snap, name, stagePath, enableAll, true)
	if err != nil {
		if isRecoveredArtifactCorruption(err) {
			corruptPath := stagePath + ".corrupt"
			if removeErr := os.Remove(corruptPath); removeErr != nil && !os.IsNotExist(removeErr) && e.logger != nil {
				e.logger.Warn("failed to remove previous corrupt recovered artifact", "artifact", name, "error", removeErr)
			}
			if removeErr := os.Remove(corruptPath + ".json"); removeErr != nil && !os.IsNotExist(removeErr) && e.logger != nil {
				e.logger.Warn("failed to remove previous corrupt recovered artifact sidecar", "artifact", name, "error", removeErr)
			}
			if renameErr := os.Rename(stagePath, corruptPath); renameErr != nil {
				return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: renameErr.Error()}, errors.Join(err, renameErr)
			}
			sidecarErr := e.writeRecoveredArtifactCorruptionSidecar(name, corruptPath, err)
			recoveryErr := fmt.Errorf("%w: %v", ErrRecoveredArtifactCorrupt, err)
			if sidecarErr != nil {
				return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: err.Error()}, errors.Join(recoveryErr, sidecarErr)
			}
			return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: err.Error()}, fmt.Errorf("%w: %v", ErrRecoveredArtifactCorrupt, err)
		}
		return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: err.Error()}, err
	}
	decision.Name = name
	decision.Status = DownloadStatusDownloaded
	decision.Message = "recovered staged artifact"
	if len(decision.ProcessingNames) == 0 {
		if err := promoteStagedFile(finalPath); err != nil {
			return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: err.Error()}, err
		}
		return decision, nil
	}
	decision.PromoteNames = appendUniqueName(decision.PromoteNames, name)
	return decision, nil
}

func (e *Engine) writeRecoveredArtifactCorruptionSidecar(name, corruptPath string, corruptionErr error) error {
	now := time.Now().UTC()
	if e != nil && e.now != nil {
		now = e.now().UTC()
	}
	sidecar := recoveredArtifactCorruptionSidecar{
		Name:            name,
		Artifact:        filepath.Base(corruptPath),
		CorruptionClass: recoveredArtifactCorruptionClass(corruptionErr),
		Timestamp:       now,
	}
	data, err := json.MarshalIndent(sidecar, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal corrupt artifact sidecar: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(corruptPath+".json", data, 0o600); err != nil {
		return fmt.Errorf("write corrupt artifact sidecar: %w", err)
	}
	return nil
}

func isRecoveredArtifactCorruption(err error) bool {
	return recoveredArtifactCorruptionClass(err) != ""
}

func recoveredArtifactCorruptionClass(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return ""
	}
	msg := strings.ToLower(err.Error())
	for _, marker := range []string{
		"token too long",
		"malformed",
		"invalid buildzone",
		"unsupported artifact type",
	} {
		if strings.Contains(msg, marker) {
			return strings.ReplaceAll(marker, " ", "_")
		}
	}
	return ""
}
