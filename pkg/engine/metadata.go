package engine

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/iprange"
	"github.com/firehol/update-ipsets/pkg/output"
)

func (e *Engine) writeMetadataFiles(ctx context.Context, skipComparisons bool, comparisonNames []string, perFeedNames []string, outDir string, setCache *latestSetCache, enableAll bool) ([]output.GeneratedFile, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	liveOutDir := e.outputDir()
	outputNames := e.publicOutputNames()
	generated := make([]output.GeneratedFile, 0, len(outputNames)*8+3)
	if !skipComparisons {
		started := time.Now()
		if err := e.writeComparisonFiles(ctx, comparisonNames, outDir, setCache); err != nil {
			e.observeRunOperation("metadata.write_comparison_files", time.Since(started))
			return nil, err
		}
		e.observeRunOperation("metadata.write_comparison_files", time.Since(started))
		// Refresh the unique_share_pct proxy for every feed whose
		// comparison rows may have changed. Runs before the per-feed
		// metadata files are written so PublicFeedSummary picks up the
		// fresh values. Errors inside updateUniqueShares are swallowed
		// per-feed; uniqueness is a ranking signal, not a must-succeed
		// contract.
		started = time.Now()
		e.updateUniqueShares(comparisonNames, outDir)
		e.observeRunOperation("metadata.update_unique_shares", time.Since(started))
	}
	started := time.Now()
	metadataFiles, err := e.writePublicMetadataFiles(outDir, outputNames)
	if err != nil {
		e.observeRunOperation("metadata.write_public_metadata_files", time.Since(started))
		return nil, err
	}
	e.observeRunOperation("metadata.write_public_metadata_files", time.Since(started))
	for _, name := range metadataFiles {
		generated = append(generated, output.GeneratedFile{Path: filepath.Join(liveOutDir, name), Timestamp: e.now().UTC(), Redistributable: true})
	}
	viewResolver := newEffectiveEntryResolver(e.cfg, e.state.SnapshotEntries())
	index := make([]setMetadata, 0, len(outputNames))
	allIPSets := make([]allIPSetsItem, 0, len(outputNames))
	setInfo := make(map[string]string, len(outputNames))
	timestampFiles := make([]output.GeneratedFile, 0, len(outputNames))
	baseGit := output.HasGitDir(e.runtime.BaseDir)

	perFeedSet := make(map[string]struct{}, len(perFeedNames))
	for _, name := range perFeedNames {
		perFeedSet[name] = struct{}{}
	}
	perFeedStarted := time.Now()
	for _, name := range outputNames {
		if err := contextErr(ctx); err != nil {
			return nil, err
		}
		entry := e.state.Entry(name)
		viewEntry := viewResolver.entry(name, entry)
		if viewEntry == nil {
			continue
		}
		meta := e.buildSetMetadataFromEffectiveEntryInDirWithResolver(name, viewEntry, outDir, enableAll, viewResolver)
		redistributable := e.isRedistributable(name)
		index = append(index, meta)
		allIPSets = append(allIPSets, allIPSetsItem{
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

		setInfo[name] = e.renderSetInfo(name, viewEntry)
		if viewEntry.File != "" {
			timestampFiles = append(timestampFiles, output.GeneratedFile{Path: filepath.Join(e.runtime.BaseDir, viewEntry.File), Timestamp: time.Unix(viewEntry.SourceDate, 0).UTC(), Redistributable: redistributable})
		}
		if _, ok := perFeedSet[name]; ok {
			processedAt := e.feedProcessingTimestamp(name)
			data, err := jsonMarshalTabIndent(meta)
			if err != nil {
				return nil, err
			}
			metaPath := filepath.Join(outDir, name+".json")
			if err := writeFileAtomic(metaPath, append(data, '\n'), 0o644); err != nil {
				return nil, err
			}
			generated = append(generated, output.GeneratedFile{Path: filepath.Join(liveOutDir, name+".json"), Timestamp: time.Unix(viewEntry.ProcessedDate, 0).UTC(), Redistributable: redistributable})

			if baseGit {
				setInfoLine := e.renderSetInfo(name, viewEntry)
				setInfoPath := filepath.Join(e.runtime.BaseDir, name+".setinfo")
				if err := writeFileAtomic(setInfoPath, []byte(setInfoLine), 0o644); err != nil {
					return nil, err
				}
				generated = append(generated, output.GeneratedFile{Path: setInfoPath, Timestamp: time.Unix(viewEntry.SourceDate, 0).UTC(), Redistributable: redistributable})
			}
			if viewEntry.File != "" {
				generated = append(generated, output.GeneratedFile{Path: filepath.Join(e.runtime.BaseDir, viewEntry.File), Timestamp: time.Unix(viewEntry.SourceDate, 0).UTC(), Redistributable: redistributable})
			}

			if err := e.writePublicHistoryCSV(name, outDir); err != nil {
				return nil, err
			}
			generated = append(generated, output.GeneratedFile{Path: filepath.Join(liveOutDir, name+"_history.csv"), Timestamp: processedAt, Redistributable: redistributable})

			if err := e.writePublicChangesetsCSV(name, outDir); err != nil {
				return nil, err
			}
			generated = append(generated, output.GeneratedFile{Path: filepath.Join(liveOutDir, name+"_changesets.csv"), Timestamp: processedAt, Redistributable: redistributable})

			if err := e.writePublicRetentionJSON(name, outDir); err != nil {
				return nil, err
			}
			generated = append(generated, output.GeneratedFile{Path: filepath.Join(liveOutDir, name+"_retention.json"), Timestamp: processedAt, Redistributable: redistributable})
		}
	}
	e.observeRunOperation("metadata.write_per_feed_outputs", time.Since(perFeedStarted))
	indexStarted := time.Now()
	data, err := jsonMarshalTabIndent(index)
	if err != nil {
		return nil, err
	}
	indexPath := filepath.Join(outDir, "index.json")
	if err := writeFileAtomic(indexPath, append(data, '\n'), 0o644); err != nil {
		return nil, err
	}
	generated = append(generated, output.GeneratedFile{Path: filepath.Join(liveOutDir, "index.json"), Timestamp: e.now().UTC(), Redistributable: true})
	data, err = jsonMarshalTabIndent(allIPSets)
	if err != nil {
		return nil, err
	}
	allPath := filepath.Join(outDir, "all-ipsets.json")
	if err := writeFileAtomic(allPath, append(data, '\n'), 0o644); err != nil {
		return nil, err
	}
	generated = append(generated, output.GeneratedFile{Path: filepath.Join(liveOutDir, "all-ipsets.json"), Timestamp: e.now().UTC(), Redistributable: true})
	e.observeRunOperation("metadata.write_indexes", time.Since(indexStarted))
	if baseGit {
		gitStarted := time.Now()
		if err := output.WriteREADME(e.runtime.BaseDir, setInfo); err != nil {
			return nil, err
		}
		if err := output.WriteGitIgnore(e.runtime.BaseDir, generated); err != nil {
			return nil, err
		}
		if err := output.WriteTimestampScript(e.runtime.BaseDir, timestampFiles); err != nil {
			return nil, err
		}
		e.observeRunOperation("metadata.write_git_artifacts", time.Since(gitStarted))
	}
	return generated, nil
}

func (e *Engine) syncGeneratedFiles(generated []output.GeneratedFile, webPublished []string) error {
	if err := output.SyncGit(output.SyncOptions{
		BaseDir:       e.runtime.BaseDir,
		PushToGit:     e.runtime.PushToGit,
		PushMerged:    e.runtime.PushToGitMerged,
		CommitOptions: strings.Fields(e.runtime.PushToGitCommitOptions),
		PushOptions:   strings.Fields(e.runtime.PushToGitPushOptions),
	}, generated); err != nil {
		return err
	}
	outDir := e.outputDir()
	if e.runtime.PushToGitWeb && outDir != e.runtime.BaseDir {
		if len(webPublished) > 0 {
			webGenerated := make([]output.GeneratedFile, 0, len(webPublished))
			for _, path := range webPublished {
				webGenerated = append(webGenerated, output.GeneratedFile{Path: path, Redistributable: true})
			}
			return output.SyncGit(output.SyncOptions{
				BaseDir:       outDir,
				PushToGit:     true,
				PushMerged:    e.runtime.PushToGitMerged,
				CommitOptions: strings.Fields(e.runtime.PushToGitCommitOptions),
				PushOptions:   strings.Fields(e.runtime.PushToGitPushOptions),
			}, webGenerated)
		}
		return output.SyncGit(output.SyncOptions{
			BaseDir:       outDir,
			PushToGit:     true,
			PushMerged:    e.runtime.PushToGitMerged,
			CommitOptions: strings.Fields(e.runtime.PushToGitCommitOptions),
			PushOptions:   strings.Fields(e.runtime.PushToGitPushOptions),
		}, filterGeneratedFiles(outDir, generated))
	}
	return nil
}

func (e *Engine) renderHeader(name string, src *config.Source, hash string, finalSet *iprange.IPSet, sourceMTime time.Time) []byte {
	var buf strings.Builder
	quantity := fmt.Sprintf("%d unique IPs", finalSet.UniqueCount())
	if hash == "net" {
		quantity = fmt.Sprintf("%d subnets, %d unique IPs", finalSet.Entries(), finalSet.UniqueCount())
	}
	// Aggregation for the header: 0 for base ipsets, specific window
	// for history variants (e.g. 1440 for _1d). This matches the bash
	// which passes hmins=0 for the base finalize call.
	aggregation := aggregationMinutesFromName(name)
	fmt.Fprintf(&buf, "#\n# %s\n#\n", name)
	fmt.Fprintf(&buf, "# %s hash:%s ipset\n#\n", src.IPV, hash)
	for _, line := range wrapInfo(src.Info) {
		fmt.Fprintf(&buf, "# %s\n", line)
	}
	fmt.Fprintf(&buf, "#\n# Maintainer      : %s\n", src.Maintainer)
	fmt.Fprintf(&buf, "# Maintainer URL  : %s\n", src.MaintainerURL)
	fmt.Fprintf(&buf, "# List source URL : %s\n", publicURL(src))
	fmt.Fprintf(&buf, "# Source File Date: %s\n#\n", posixDate(sourceMTime.UTC()))
	fmt.Fprintf(&buf, "# Category        : %s\n", src.Category)
	fmt.Fprintf(&buf, "# Version         : %d\n#\n", e.state.Entry(name).Snapshot().Version+1)
	fmt.Fprintf(&buf, "# This File Date  : %s\n", posixDate(e.now().UTC()))
	fmt.Fprintf(&buf, "# Update Frequency: %s\n", minutesText(src.Frequency))
	fmt.Fprintf(&buf, "# Aggregation     : %s\n", minutesText(aggregation))
	fmt.Fprintf(&buf, "# Entries         : %s\n#\n", quantity)
	fmt.Fprintf(&buf, "# Full list analysis, including geolocation map, history,\n")
	fmt.Fprintf(&buf, "# retention policy, overlaps with other lists, etc.\n")
	fmt.Fprintf(&buf, "# available at:\n#\n#  %s%s\n#\n", e.runtime.WebURL, name)
	fmt.Fprintf(&buf, "# Generated by FireHOL's update-ipsets\n#\n")
	return []byte(buf.String())
}

func (e *Engine) renderSetInfo(name string, entry *cache.Entry) string {
	quantity := fmt.Sprintf("%d unique IPs", entry.UniqueIPs)
	if entry.Hash == "net" {
		quantity = fmt.Sprintf("%d subnets, %d unique IPs", entry.Entries, entry.UniqueIPs)
	}
	// Bash conditionally includes "from [this link]" only for redistributable sets.
	redistributable := e.isRedistributable(name)
	freq := minutesText(entry.FrequencyMinutes)
	if redistributable && entry.PublicURL != "" {
		return fmt.Sprintf("[%s](%s%s)|%s|%s hash:%s|%s|updated every %s from [this link](%s)\n",
			name, e.runtime.WebURL, name, entry.Info, entry.IPV, entry.Hash, quantity, freq, entry.PublicURL)
	}
	return fmt.Sprintf("[%s](%s%s)|%s|%s hash:%s|%s|updated every %s\n",
		name, e.runtime.WebURL, name, entry.Info, entry.IPV, entry.Hash, quantity, freq)
}

// posixDate formats a time in the POSIX locale format used by
// "date -u" in the bash script: "Mon Mar 30 07:44:35 UTC 2026".
// This matches the C locale default: "%a %b %e %H:%M:%S %Z %Y".
func posixDate(t time.Time) string {
	day := t.Day()
	// POSIX %e pads single-digit days with a leading space.
	dayStr := fmt.Sprintf("%2d", day)
	return fmt.Sprintf("%s %s %s %s UTC %d",
		t.Format("Mon"),
		t.Format("Jan"),
		dayStr,
		t.Format("15:04:05"),
		t.Year())
}
