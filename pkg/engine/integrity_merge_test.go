package engine

import (
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
)

func TestIntegrityBlockedFeedsAllowsArchivedSubtractiveParentWithBody(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }
	eng.cfg.Runtime.FeedHealthSingleObservationGraceMins = 60
	eng.cfg.Runtime.FeedHealthDefaultHealthyCadenceMins = 60
	eng.cfg.Runtime.FeedHealthDefaultRiskyCadenceMins = 60
	eng.cfg.Runtime.FeedHealthArchivalThresholdMins = 600
	eng.cfg.Sources["bogons"] = &config.Source{
		Name:      "bogons",
		URL:       "https://example.test/bogons.txt",
		Frequency: 60,
		IPV:       "ipv4",
		Output:    "netset",
		Use:       []string{config.UseBogons},
	}
	eng.cfg.Sources["cymru_unassigned"] = &config.Source{
		Name:         "cymru_unassigned",
		URL:          "internal://merge?exclude=bogons&inputs=fullbogons",
		Frequency:    1440,
		IPV:          "ipv4",
		Output:       "netset",
		Use:          []string{config.UseBogons},
		DerivedFrom:  []string{"fullbogons", "bogons"},
		MergeSources: []string{"fullbogons"},
		MergeExclude: []string{"bogons"},
		Provenance:   config.ProvenanceSecondaryMerge,
	}

	writeFileForIntegrity(t, eng.feedBodyPath("fullbogons"), now.Add(-10*time.Minute))
	writeFileForIntegrity(t, eng.feedBodyPath("bogons"), now.Add(-24*time.Hour))
	fullbogons := eng.state.Entry("fullbogons")
	fullbogons.Name = "fullbogons"
	fullbogons.ProcessedDate = now.Add(-10 * time.Minute).Unix()
	fullbogons.SourceDate = now.Add(-10 * time.Minute).Unix()
	fullbogons.CheckedDate = now.Unix()
	fullbogons.Entries = 1
	fullbogons.Version = 1
	bogons := eng.state.Entry("bogons")
	bogons.Name = "bogons"
	bogons.ProcessedDate = now.Add(-24 * time.Hour).Unix()
	bogons.SourceDate = now.Add(-24 * time.Hour).Unix()
	bogons.CheckedDate = now.Unix()
	bogons.DownloadFailures = 5
	bogons.FailureStartedDate = now.Add(-24 * time.Hour).Unix()
	bogons.LastStatus = "download_failed"
	bogons.Entries = 1
	bogons.Version = 1

	resolver := newEffectiveEntryResolver(eng.cfg, eng.state.SnapshotEntries())
	blocked := eng.integrityBlockedFeeds("cymru_unassigned", eng.cfg.Sources["cymru_unassigned"], resolver, true)
	if len(blocked) != 0 {
		t.Fatalf("blocked feeds = %v, want none for archived subtractive parent with a materialized body", blocked)
	}
}
