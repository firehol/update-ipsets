package web

import (
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/feedhealth"
	"github.com/firehol/update-ipsets/pkg/runreason"
	"github.com/firehol/update-ipsets/pkg/scheduler"
)

func buildAdminFeedsFromCachedStatus(eng *engine.Engine, runner *scheduler.Runner) []adminFeed {
	cfg, rt, policy, ok := eng.TryConfigRuntimePolicySnapshot()
	if !ok {
		return buildAdminFeedsFromSchedulerSnapshot(runner)
	}
	if cfg == nil {
		return nil
	}
	now := time.Now().UTC()
	activity := runner.ActivitySnapshotLight()
	snap := cachedSchedulerSnapshot(runner)
	liveStates := liveFeedStates(activity)
	enableAll := runnerEnableAll(runner)
	entryIndex := cacheEntriesFromSchedulerItems(snap.Items)
	schedIndex := schedulerItemsByName(snap.Items)

	feeds := make([]adminFeed, 0, len(cfg.Sources))
	for _, name := range config.SortedSourceNames(cfg) {
		src := cfg.Sources[name]
		feed := buildSourceFeed(name, src, cfg, entryIndex, schedIndex, liveStates, policy, now, nil)
		if item, ok := schedIndex[name]; ok {
			populateAdminFeedFromSchedulerItem(&feed, *item)
		} else {
			feed.Enabled = engine.EffectiveSourceEnabled(cfg, rt, name, enableAll)
			feed.Status = deriveFeedStatus(&feed, liveStates[name])
			populateAdminFeedStatusMeta(&feed)
			feed.Health = feedhealth.Snapshot{Class: feedhealth.ClassUnavailable}
		}
		feeds = append(feeds, feed)
	}
	return feeds
}

func buildAdminArtifactsFromCachedStatus(eng *engine.Engine, runner *scheduler.Runner) []adminArtifact {
	cfg, rt, _, ok := eng.TryConfigRuntimePolicySnapshot()
	if !ok {
		return buildAdminArtifactsFromSchedulerSnapshot(runner)
	}
	if cfg == nil || len(cfg.Artifacts) == 0 {
		return nil
	}
	activity := runner.ActivitySnapshotLight()
	snap := cachedSchedulerSnapshot(runner)
	itemIndex := schedulerItemsByName(snap.ArtifactItems)
	enableAll := runnerEnableAll(runner)
	downloadWaiting := make(map[string]bool, len(activity.DownloadWaiting))
	for _, item := range activity.DownloadWaiting {
		downloadWaiting[item.Name] = true
	}
	downloadActive := make(map[string]bool, len(activity.DownloadActive))
	for _, item := range activity.DownloadActive {
		downloadActive[item.Name] = true
	}

	artifacts := make([]adminArtifact, 0, len(cfg.Artifacts))
	for _, name := range config.SortedArtifactNames(cfg) {
		artifact := cfg.ArtifactByName(name)
		if artifact == nil {
			continue
		}
		row := adminArtifact{
			Name:             name,
			Type:             artifact.Type,
			FrequencyMinutes: artifact.Frequency,
			Info:             artifact.Info,
			Maintainer:       artifact.Maintainer,
			MaintainerURL:    artifact.MaintainerURL,
		}
		for _, child := range cfg.ArtifactChildren(name) {
			if child != nil {
				row.ChildFeeds = append(row.ChildFeeds, child.Name)
			}
		}
		if item, ok := itemIndex[name]; ok {
			populateAdminArtifactFromSchedulerItem(&row, *item)
		} else {
			row.Enabled = engine.EffectiveArtifactEnabled(cfg, rt, name, enableAll)
		}
		populateAdminArtifactStatusMeta(&row)
		row.Status = deriveArtifactStatus(&row, downloadWaiting[name], downloadActive[name])
		artifacts = append(artifacts, row)
	}
	return artifacts
}

func buildAdminFeedsFromSchedulerSnapshot(runner *scheduler.Runner) []adminFeed {
	if runner == nil {
		return nil
	}
	activity := runner.ActivitySnapshotLight()
	liveStates := liveFeedStates(activity)
	snap := cachedSchedulerSnapshot(runner)
	feeds := make([]adminFeed, 0, len(snap.Items))
	for _, item := range snap.Items {
		feed := adminFeed{
			Name:   item.Name,
			Kind:   item.Kind,
			Hidden: item.Hidden,
		}
		populateAdminFeedFromSchedulerItem(&feed, item)
		feed.Status = deriveFeedStatus(&feed, liveStates[item.Name])
		populateAdminFeedStatusMeta(&feed)
		if feed.Health.Class == "" && item.HealthClass != "" {
			feed.Health.Class = feedhealth.Class(item.HealthClass)
		}
		feeds = append(feeds, feed)
	}
	return feeds
}

func buildAdminArtifactsFromSchedulerSnapshot(runner *scheduler.Runner) []adminArtifact {
	if runner == nil {
		return nil
	}
	activity := runner.ActivitySnapshotLight()
	snap := cachedSchedulerSnapshot(runner)
	downloadWaiting := make(map[string]bool, len(activity.DownloadWaiting))
	for _, item := range activity.DownloadWaiting {
		downloadWaiting[item.Name] = true
	}
	downloadActive := make(map[string]bool, len(activity.DownloadActive))
	for _, item := range activity.DownloadActive {
		downloadActive[item.Name] = true
	}
	artifacts := make([]adminArtifact, 0, len(snap.ArtifactItems))
	for _, item := range snap.ArtifactItems {
		artifact := adminArtifact{
			Name: item.Name,
			Type: item.Kind,
		}
		populateAdminArtifactFromSchedulerItem(&artifact, item)
		populateAdminArtifactStatusMeta(&artifact)
		artifact.Status = deriveArtifactStatus(&artifact, downloadWaiting[item.Name], downloadActive[item.Name])
		artifacts = append(artifacts, artifact)
	}
	return artifacts
}

func cachedSchedulerSnapshot(runner *scheduler.Runner) scheduler.Snapshot {
	snapshot, _ := runner.TryCachedSnapshot()
	return snapshot
}

func schedulerItemsByName(items []scheduler.Item) map[string]*scheduler.Item {
	index := make(map[string]*scheduler.Item, len(items))
	for i := range items {
		index[items[i].Name] = &items[i]
	}
	return index
}

func cacheEntriesFromSchedulerItems(items []scheduler.Item) map[string]*cache.Entry {
	index := make(map[string]*cache.Entry, len(items))
	for i := range items {
		item := items[i]
		entry := cache.Entry{
			Name:              item.Name,
			File:              item.File,
			Source:            item.Source,
			PublicURL:         item.PublicURL,
			Hash:              item.Hash,
			Entries:           item.Entries,
			EntriesMin:        item.EntriesMin,
			EntriesMax:        item.EntriesMax,
			UniqueIPs:         item.UniqueIPs,
			IPsMin:            item.IPsMin,
			IPsMax:            item.IPsMax,
			AverageUpdateMins: item.AvgUpdateMins,
			MinUpdateMins:     item.MinUpdateMins,
			MaxUpdateMins:     item.MaxUpdateMins,
			Version:           item.Version,
			DownloadFailures:  item.Failures,
			ClockSkewSeconds:  item.ClockSkewSeconds,
			LastStatus:        item.LastStatus,
			LastRunReason:     runreason.Reason(item.LastRunReason),
			LastError:         item.LastError,
			LastProcessingMS:  item.LastProcessingMS,
		}
		if !item.CheckedAt.IsZero() {
			entry.CheckedDate = item.CheckedAt.Unix()
		}
		if !item.UpdatedAt.IsZero() {
			entry.SourceDate = item.UpdatedAt.Unix()
		}
		if !item.ProcessedAt.IsZero() {
			entry.ProcessedDate = item.ProcessedAt.Unix()
		}
		if !item.StartedAt.IsZero() {
			entry.StartedDate = item.StartedAt.Unix()
		}
		index[item.Name] = &entry
	}
	return index
}

func populateAdminFeedFromSchedulerItem(feed *adminFeed, item scheduler.Item) {
	if feed == nil {
		return
	}
	feed.Enabled = item.Enabled
	if item.FrequencyMinutes > 0 {
		feed.FrequencyMinutes = item.FrequencyMinutes
	}
	feed.Entries = item.Entries
	feed.UniqueIPs = item.UniqueIPs
	feed.DownloadFailures = item.Failures
	feed.LastStatus = item.LastStatus
	feed.LastRunReason = item.LastRunReason
	feed.LastError = item.LastError
	if !item.CheckedAt.IsZero() {
		feed.LastCheck = item.CheckedAt.Unix()
	}
	if !item.UpdatedAt.IsZero() {
		feed.LastUpdate = item.UpdatedAt.Unix()
	}
	if !item.NextDue.IsZero() {
		feed.NextCheck = item.NextDue.Unix()
	}
	feed.SchedulerDetail = adminSchedulerDetail(feed, item.Detail)
	if item.HealthClass != "" {
		feed.Health.Class = feedhealth.Class(item.HealthClass)
	}
}

func populateAdminArtifactFromSchedulerItem(artifact *adminArtifact, item scheduler.Item) {
	if artifact == nil {
		return
	}
	artifact.Enabled = item.Enabled
	if item.FrequencyMinutes > 0 {
		artifact.FrequencyMinutes = item.FrequencyMinutes
	}
	artifact.DownloadFailures = item.Failures
	artifact.LastStatus = item.LastStatus
	artifact.LastError = item.LastError
	if !item.CheckedAt.IsZero() {
		artifact.LastCheck = item.CheckedAt.Unix()
	}
	if !item.UpdatedAt.IsZero() {
		artifact.LastUpdate = item.UpdatedAt.Unix()
	}
	if !item.NextDue.IsZero() {
		artifact.NextCheck = item.NextDue.Unix()
	}
	artifact.SchedulerDetail = item.Detail
}
