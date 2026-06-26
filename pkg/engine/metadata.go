package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/iprange"
	"github.com/firehol/update-ipsets/pkg/output"
)

var syncGeneratedFilesHookMu sync.Mutex
var syncGeneratedFilesBeforeHook func()

func setSyncGeneratedFilesBeforeHookForTest(fn func()) func() {
	syncGeneratedFilesHookMu.Lock()
	old := syncGeneratedFilesBeforeHook
	syncGeneratedFilesBeforeHook = fn
	syncGeneratedFilesHookMu.Unlock()
	return func() {
		syncGeneratedFilesHookMu.Lock()
		syncGeneratedFilesBeforeHook = old
		syncGeneratedFilesHookMu.Unlock()
	}
}

func syncGeneratedFilesBeforeHookForTest() func() {
	syncGeneratedFilesHookMu.Lock()
	defer syncGeneratedFilesHookMu.Unlock()
	return syncGeneratedFilesBeforeHook
}

func (e *Engine) syncGeneratedFiles(ctx context.Context, generated []output.GeneratedFile, webPublished []string) error {
	if e == nil || e.gitLane == nil {
		return e.syncGeneratedFilesDirect(ctx, generated, webPublished)
	}
	return e.gitLane.Run(ctx, e.gitSyncWork("publish.sync_generated_files"), func(laneCtx context.Context) error {
		return e.syncGeneratedFilesDirect(laneCtx, generated, webPublished)
	})
}

func (e *Engine) gitSyncWork(name string) LaneWork {
	seq := uint64(0)
	if e != nil {
		seq = e.gitSyncSeq.Add(1)
	}
	if name == "" {
		name = "publish.sync_generated_files"
	}
	id := fmt.Sprintf("git-sync:%d", seq)
	return LaneWork{
		ID:            id,
		Kind:          LaneWorkGitSync,
		Component:     LaneComponentPublishStages,
		Name:          name,
		CoalescingKey: "git-sync:publish",
	}
}

func (e *Engine) syncGeneratedFilesDirect(ctx context.Context, generated []output.GeneratedFile, webPublished []string) error {
	if hook := syncGeneratedFilesBeforeHookForTest(); hook != nil {
		hook()
	}
	rt := e.Runtime()
	syncTargets := 0
	if rt.PushToGit {
		syncTargets++
	}
	outDir := outputDirForRuntime(rt)
	if rt.PushToGitWeb && outDir != rt.BaseDir {
		syncTargets++
	}
	op := e.beginActiveOperation("publish.sync_generated_files", "", "sync", "repositories", int64(syncTargets))
	defer op.Finish()
	if err := output.SyncGitContext(ctx, output.SyncOptions{
		BaseDir:       rt.BaseDir,
		PushToGit:     rt.PushToGit,
		PushMerged:    rt.PushToGitMerged,
		CommitOptions: strings.Fields(rt.PushToGitCommitOptions),
		PushOptions:   strings.Fields(rt.PushToGitPushOptions),
		Timeout:       rt.PushToGitTimeout,
	}, generated); err != nil {
		return err
	}
	if rt.PushToGit {
		op.Add(1, int64(syncTargets), nil)
	}
	if rt.PushToGitWeb && outDir != rt.BaseDir {
		if len(webPublished) > 0 {
			webGenerated := make([]output.GeneratedFile, 0, len(webPublished))
			for _, path := range webPublished {
				webGenerated = append(webGenerated, output.GeneratedFile{Path: path, Redistributable: true})
			}
			if err := output.SyncGitContext(ctx, output.SyncOptions{
				BaseDir:       outDir,
				PushToGit:     true,
				PushMerged:    rt.PushToGitMerged,
				CommitOptions: strings.Fields(rt.PushToGitCommitOptions),
				PushOptions:   strings.Fields(rt.PushToGitPushOptions),
				Timeout:       rt.PushToGitTimeout,
			}, webGenerated); err != nil {
				return err
			}
			op.Add(1, int64(syncTargets), nil)
			return nil
		}
		if err := output.SyncGitContext(ctx, output.SyncOptions{
			BaseDir:       outDir,
			PushToGit:     true,
			PushMerged:    rt.PushToGitMerged,
			CommitOptions: strings.Fields(rt.PushToGitCommitOptions),
			PushOptions:   strings.Fields(rt.PushToGitPushOptions),
			Timeout:       rt.PushToGitTimeout,
		}, filterGeneratedFiles(outDir, generated)); err != nil {
			return err
		}
		op.Add(1, int64(syncTargets), nil)
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
