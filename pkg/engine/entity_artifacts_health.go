package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/firehol/update-ipsets/pkg/output"
)

func (e *Engine) collectHealthTransitionAffectedEntities(ctx context.Context, feedNames []string, task *BackgroundTaskHandle, affectedCountries map[string]struct{}, affectedASNs map[uint32]struct{}) error {
	for i, name := range feedNames {
		if err := contextErr(ctx); err != nil {
			return err
		}
		if task != nil {
			task.Update("scanning memberships", backgroundEntityTaskDetail("health", len(feedNames)), i+1, len(feedNames))
		}
		sidecar, err := e.loadCommittedFeedEntitySidecar(name)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		for _, code := range sidecar.countryCodes() {
			affectedCountries[code] = struct{}{}
		}
		for _, asn := range sidecar.asnNumbers() {
			affectedASNs[asn] = struct{}{}
		}
	}
	return nil
}

func (e *Engine) publishHealthTransitionHomeAggregate(ctx context.Context) error {
	webBatch, err := e.newWebPublishBatch()
	if err != nil {
		return err
	}
	defer webBatch.cleanup()

	homeAggregate, err := e.stageHomeAggregates(ctx, webBatch.stageDir, "")
	if err != nil {
		return err
	}
	generated := []output.GeneratedFile{homeAggregate}
	return e.publishHealthTransitionGenerated(ctx, webBatch, generated)
}

func (e *Engine) publishHealthTransitionEntityPayloads(ctx context.Context, affectedCountries map[string]struct{}, affectedASNs map[uint32]struct{}, task *BackgroundTaskHandle) error {
	detail := healthTransitionRebuildDetail(affectedCountries, affectedASNs)
	total := len(affectedCountries) + len(affectedASNs)
	if task != nil {
		task.Update("rebuilding final payloads", detail, 0, total)
	}

	webBatch, err := e.newWebPublishBatch()
	if err != nil {
		return err
	}
	defer webBatch.cleanup()

	generated := make([]output.GeneratedFile, 0, total)
	health := e.newFeedHealthClassifier()
	progress := 0
	for _, code := range sortedStringSet(affectedCountries) {
		if err := contextErr(ctx); err != nil {
			return err
		}
		var err error
		generated, err = e.stageHealthTransitionCountryPayload(webBatch, generated, health, code)
		if err != nil {
			return err
		}
		progress++
		updateHealthTransitionRebuildProgress(task, detail, progress, total)
	}
	for _, asn := range sortedUint32Set(affectedASNs) {
		if err := contextErr(ctx); err != nil {
			return err
		}
		var err error
		generated, err = e.stageHealthTransitionASNPayload(webBatch, generated, health, asn)
		if err != nil {
			return err
		}
		progress++
		updateHealthTransitionRebuildProgress(task, detail, progress, total)
	}
	if task != nil {
		task.Update("publishing", "publishing refreshed entity artifacts", total, total)
	}
	homeAggregate, err := e.stageHomeAggregates(ctx, webBatch.stageDir, "")
	if err != nil {
		return err
	}
	generated = append(generated, homeAggregate)
	return e.publishHealthTransitionGenerated(ctx, webBatch, generated)
}

func (e *Engine) stageHealthTransitionCountryPayload(webBatch *webPublishBatch, generated []output.GeneratedFile, health *feedHealthClassifier, code string) ([]output.GeneratedFile, error) {
	sidecarPath := filepath.Join(e.entityCountriesDir(), code+".json")
	sidecar, err := loadCountryDetailSidecar(sidecarPath)
	if err != nil {
		return nil, err
	}
	sidecarInfo, err := os.Stat(sidecarPath)
	if err != nil {
		return nil, err
	}
	rel := e.publicCountryDetailRelPath(code)
	payload := e.materializeCountryDetailWithHealth(sidecar, health)
	if err := writeJSONFile(filepath.Join(webBatch.stageDir, rel), payload); err != nil {
		return nil, err
	}
	logicalTime := sidecarInfo.ModTime().UTC()
	generated = append(generated, output.GeneratedFile{Path: filepath.Join(e.outputDir(), rel), Timestamp: logicalTime, Redistributable: true})
	if mdFile, _ := e.stageCountryMarkdown(code, payload, webBatch.stageDir); mdFile.Path != "" {
		mdFile.Timestamp = logicalTime
		generated = append(generated, mdFile)
	}
	return generated, nil
}

func (e *Engine) stageHealthTransitionASNPayload(webBatch *webPublishBatch, generated []output.GeneratedFile, health *feedHealthClassifier, asn uint32) ([]output.GeneratedFile, error) {
	sidecarPath := filepath.Join(e.entityASNsDir(), strconv.FormatUint(uint64(asn), 10)+".json")
	sidecar, err := loadASNDetailSidecar(sidecarPath)
	if err != nil {
		return nil, err
	}
	sidecarInfo, err := os.Stat(sidecarPath)
	if err != nil {
		return nil, err
	}
	rel := e.publicASNDetailRelPath(asn)
	payload := e.materializeASNDetailWithHealth(sidecar, health)
	if err := writeJSONFile(filepath.Join(webBatch.stageDir, rel), payload); err != nil {
		return nil, err
	}
	logicalTime := sidecarInfo.ModTime().UTC()
	generated = append(generated, output.GeneratedFile{Path: filepath.Join(e.outputDir(), rel), Timestamp: logicalTime, Redistributable: true})
	if mdFile, _ := e.stageASNMarkdown(asn, payload, webBatch.stageDir); mdFile.Path != "" {
		mdFile.Timestamp = logicalTime
		generated = append(generated, mdFile)
	}
	return generated, nil
}

func (e *Engine) publishHealthTransitionGenerated(ctx context.Context, webBatch *webPublishBatch, generated []output.GeneratedFile) error {
	if err := webBatch.applyGeneratedFileTimestampsContext(ctx, generated); err != nil {
		return err
	}
	published, err := webBatch.publishContext(ctx)
	if err != nil {
		return err
	}
	return e.syncGeneratedFiles(generated, published)
}

func healthTransitionRebuildDetail(affectedCountries map[string]struct{}, affectedASNs map[uint32]struct{}) string {
	return fmt.Sprintf("rewriting %d countries and %d ASNs after health transitions", len(affectedCountries), len(affectedASNs))
}

func updateHealthTransitionRebuildProgress(task *BackgroundTaskHandle, detail string, progress, total int) {
	if task != nil {
		task.Update("rebuilding final payloads", detail, progress, total)
	}
}
