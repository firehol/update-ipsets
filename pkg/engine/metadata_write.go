package engine

import (
	"context"
	"path/filepath"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/output"
)

func (e *Engine) writeMetadataFiles(ctx context.Context, skipComparisons bool, comparisonNames []string, perFeedNames []string, outDir string, setCache *latestSetCache, enableAll bool) ([]output.GeneratedFile, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	run := e.newMetadataWriteRun(ctx, outDir, perFeedNames, enableAll)
	if err := run.writeComparisonOutputs(skipComparisons, comparisonNames, setCache); err != nil {
		return nil, err
	}
	if err := run.writePublicMetadataList(); err != nil {
		return nil, err
	}
	if err := run.writePerFeedOutputs(); err != nil {
		return nil, err
	}
	if err := run.writeIndexOutputs(); err != nil {
		return nil, err
	}
	if run.baseGit {
		if err := run.writeGitArtifacts(); err != nil {
			return nil, err
		}
	}
	return run.generated, nil
}

type metadataWriteRun struct {
	e              *Engine
	ctx            context.Context
	outDir         string
	liveOutDir     string
	enableAll      bool
	outputNames    []string
	generated      []output.GeneratedFile
	viewResolver   *effectiveEntryResolver
	index          []setMetadata
	allIPSets      []allIPSetsItem
	setInfo        map[string]string
	timestampFiles []output.GeneratedFile
	perFeedSet     map[string]struct{}
	baseGit        bool
}

func (e *Engine) newMetadataWriteRun(ctx context.Context, outDir string, perFeedNames []string, enableAll bool) *metadataWriteRun {
	outputNames := e.publicOutputNames()
	return &metadataWriteRun{
		e:              e,
		ctx:            ctx,
		outDir:         outDir,
		liveOutDir:     e.outputDir(),
		enableAll:      enableAll,
		outputNames:    outputNames,
		generated:      make([]output.GeneratedFile, 0, len(outputNames)*8+3),
		viewResolver:   newEffectiveEntryResolver(e.cfg, e.state.SnapshotEntries()),
		index:          make([]setMetadata, 0, len(outputNames)),
		allIPSets:      make([]allIPSetsItem, 0, len(outputNames)),
		setInfo:        make(map[string]string, len(outputNames)),
		timestampFiles: make([]output.GeneratedFile, 0, len(outputNames)),
		perFeedSet:     metadataNameSet(perFeedNames),
		baseGit:        output.HasGitDir(e.runtime.BaseDir),
	}
}

func metadataNameSet(names []string) map[string]struct{} {
	out := make(map[string]struct{}, len(names))
	for _, name := range names {
		out[name] = struct{}{}
	}
	return out
}

func (r *metadataWriteRun) writeComparisonOutputs(skipComparisons bool, comparisonNames []string, setCache *latestSetCache) error {
	if skipComparisons {
		return nil
	}
	started := time.Now()
	if err := r.e.writeComparisonFiles(r.ctx, comparisonNames, r.outDir, setCache); err != nil {
		r.e.observeRunOperation("metadata.write_comparison_files", time.Since(started))
		return err
	}
	r.e.observeRunOperation("metadata.write_comparison_files", time.Since(started))

	started = time.Now()
	r.e.updateUniqueSharesContext(r.ctx, comparisonNames, r.outDir)
	r.e.observeRunOperation("metadata.update_unique_shares", time.Since(started))
	return nil
}

func (r *metadataWriteRun) writePublicMetadataList() error {
	started := time.Now()
	progress := r.e.beginActiveOperation("metadata.write_public_metadata", "", "write", "operations", 1)
	defer progress.Finish()
	metadataFiles, err := r.e.writePublicMetadataFiles(r.outDir, r.outputNames)
	if err != nil {
		r.e.observeRunOperation("metadata.write_public_metadata_files", time.Since(started))
		return err
	}
	progress.Update(1, 1, nil)
	r.e.observeRunOperation("metadata.write_public_metadata_files", time.Since(started))
	for _, name := range metadataFiles {
		r.addGenerated(filepath.Join(r.liveOutDir, name), r.e.now().UTC(), true)
	}
	return nil
}

func (r *metadataWriteRun) writePerFeedOutputs() error {
	started := time.Now()
	progress := r.e.beginActiveOperation("metadata.write_per_feed_outputs", "", "write", "feeds", int64(len(r.outputNames)))
	defer progress.Finish()
	for _, name := range r.outputNames {
		if err := contextErr(r.ctx); err != nil {
			return err
		}
		entry := r.e.state.Entry(name)
		viewEntry := r.viewResolver.entry(name, entry)
		if viewEntry == nil {
			progress.Add(1, int64(len(r.outputNames)), nil)
			continue
		}
		meta := r.e.buildSetMetadataFromEffectiveEntryInDirWithResolver(name, viewEntry, r.outDir, r.enableAll, r.viewResolver)
		redistributable := r.e.isRedistributable(name)
		r.addFeedIndexRows(name, viewEntry, meta, redistributable)
		if _, ok := r.perFeedSet[name]; ok {
			if err := r.writePerFeedArtifacts(name, viewEntry, meta, redistributable); err != nil {
				progress.Add(1, int64(len(r.outputNames)), nil)
				return err
			}
		}
		progress.Add(1, int64(len(r.outputNames)), nil)
	}
	r.e.observeRunOperation("metadata.write_per_feed_outputs", time.Since(started))
	return nil
}

func (r *metadataWriteRun) addFeedIndexRows(name string, viewEntry *cache.Entry, meta setMetadata, redistributable bool) {
	r.index = append(r.index, meta)
	r.allIPSets = append(r.allIPSets, allIPSetsItem{
		IPSet:      name,
		Category:   viewEntry.Category,
		Maintainer: viewEntry.Maintainer,
		Started:    millis(viewEntry.StartedDate),
		Updated:    millis(viewEntry.SourceDate),
		Checked:    millis(max(viewEntry.CheckedDate, viewEntry.ProcessedDate)),
		ClockSkew:  millis(viewEntry.ClockSkewSeconds),
		IPs:        viewEntry.UniqueIPs,
		Errors:     viewEntry.DownloadFailures,
	})
	r.setInfo[name] = r.e.renderSetInfo(name, viewEntry)
	if viewEntry.File != "" {
		r.timestampFiles = append(r.timestampFiles, output.GeneratedFile{
			Path:            filepath.Join(r.e.runtime.BaseDir, viewEntry.File),
			Timestamp:       time.Unix(viewEntry.SourceDate, 0).UTC(),
			Redistributable: redistributable,
		})
	}
}

func (r *metadataWriteRun) writePerFeedArtifacts(name string, viewEntry *cache.Entry, meta setMetadata, redistributable bool) error {
	processedAt := r.e.feedProcessingTimestamp(name)
	data, err := jsonMarshalTabIndent(meta)
	if err != nil {
		return err
	}
	metaPath := filepath.Join(r.outDir, name+".json")
	if err := writeFileAtomic(metaPath, append(data, '\n'), generatedFileMode); err != nil {
		return err
	}
	r.addGenerated(filepath.Join(r.liveOutDir, name+".json"), time.Unix(viewEntry.ProcessedDate, 0).UTC(), redistributable)

	if r.baseGit {
		setInfoLine := r.e.renderSetInfo(name, viewEntry)
		setInfoPath := filepath.Join(r.e.runtime.BaseDir, name+".setinfo")
		if err := writeFileAtomic(setInfoPath, []byte(setInfoLine), generatedFileMode); err != nil {
			return err
		}
		r.addGenerated(setInfoPath, time.Unix(viewEntry.SourceDate, 0).UTC(), redistributable)
	}
	if viewEntry.File != "" {
		r.addGenerated(filepath.Join(r.e.runtime.BaseDir, viewEntry.File), time.Unix(viewEntry.SourceDate, 0).UTC(), redistributable)
	}
	if err := r.writePerFeedDerivativeArtifacts(name, processedAt, redistributable); err != nil {
		return err
	}
	return nil
}

func (r *metadataWriteRun) writePerFeedDerivativeArtifacts(name string, processedAt time.Time, redistributable bool) error {
	if err := r.e.writePublicHistoryCSV(name, r.outDir); err != nil {
		return err
	}
	r.addGenerated(filepath.Join(r.liveOutDir, name+"_history.csv"), processedAt, redistributable)

	if err := r.e.writePublicChangesetsCSV(name, r.outDir); err != nil {
		return err
	}
	r.addGenerated(filepath.Join(r.liveOutDir, name+"_changesets.csv"), processedAt, redistributable)

	if err := r.e.writePublicRetentionJSON(name, r.outDir); err != nil {
		return err
	}
	r.addGenerated(filepath.Join(r.liveOutDir, name+"_retention.json"), processedAt, redistributable)
	return nil
}

func (r *metadataWriteRun) writeIndexOutputs() error {
	started := time.Now()
	progress := r.e.beginActiveOperation("metadata.write_indexes", "", "write", "files", 2)
	defer progress.Finish()
	data, err := jsonMarshalTabIndent(r.index)
	if err != nil {
		return err
	}
	indexPath := filepath.Join(r.outDir, "index.json")
	if err := writeFileAtomic(indexPath, append(data, '\n'), generatedFileMode); err != nil {
		return err
	}
	r.addGenerated(filepath.Join(r.liveOutDir, "index.json"), r.e.now().UTC(), true)
	progress.Add(1, 2, nil)

	data, err = jsonMarshalTabIndent(r.allIPSets)
	if err != nil {
		return err
	}
	allPath := filepath.Join(r.outDir, "all-ipsets.json")
	if err := writeFileAtomic(allPath, append(data, '\n'), generatedFileMode); err != nil {
		return err
	}
	r.addGenerated(filepath.Join(r.liveOutDir, "all-ipsets.json"), r.e.now().UTC(), true)
	progress.Add(1, 2, nil)
	r.e.observeRunOperation("metadata.write_indexes", time.Since(started))
	return nil
}

func (r *metadataWriteRun) writeGitArtifacts() error {
	started := time.Now()
	progress := r.e.beginActiveOperation("metadata.write_git_artifacts", "", "write", "files", 3)
	defer progress.Finish()
	if err := output.WriteREADME(r.e.runtime.BaseDir, r.setInfo); err != nil {
		return err
	}
	progress.Add(1, 3, nil)
	if err := output.WriteGitIgnore(r.e.runtime.BaseDir, r.generated); err != nil {
		return err
	}
	progress.Add(1, 3, nil)
	if err := output.WriteTimestampScript(r.e.runtime.BaseDir, r.timestampFiles); err != nil {
		return err
	}
	progress.Add(1, 3, nil)
	r.e.observeRunOperation("metadata.write_git_artifacts", time.Since(started))
	return nil
}

func (r *metadataWriteRun) addGenerated(path string, timestamp time.Time, redistributable bool) {
	r.generated = append(r.generated, output.GeneratedFile{
		Path:            path,
		Timestamp:       timestamp,
		Redistributable: redistributable,
	})
}
