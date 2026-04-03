package engine

import (
	"fmt"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
)

func BenchmarkEffectiveEntryResolverBatchView(b *testing.B) {
	cfg, entries, names := effectiveEntryBenchmarkFixture(250)

	b.ReportAllocs()
	for b.Loop() {
		resolver := newEffectiveEntryResolver(cfg, entries)
		var total int64
		for _, name := range names {
			view := resolver.entryFromSnapshot(name)
			if view == nil {
				b.Fatalf("missing effective entry for %s", name)
			}
			total += view.SourceDate
		}
		if total == 0 {
			b.Fatal("effective entry benchmark produced no timestamps")
		}
	}
}

func effectiveEntryBenchmarkFixture(parents int) (*config.Config, map[string]cache.Entry, []string) {
	cfg := config.New()
	entries := make(map[string]cache.Entry, parents*3)
	names := make([]string, 0, parents*3)
	base := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	for i := range parents {
		parent := fmt.Sprintf("parent_%04d", i)
		retention := fmt.Sprintf("%s_1d", parent)
		merge := fmt.Sprintf("merge_%04d", i)
		peer := fmt.Sprintf("peer_%04d", i)

		cfg.Sources[parent] = &config.Source{
			Name:     parent,
			Category: "attacks",
		}
		cfg.Sources[peer] = &config.Source{
			Name:     peer,
			Category: "attacks",
		}
		cfg.Sources[retention] = &config.Source{
			Name:        retention,
			Category:    "attacks",
			DerivedFrom: []string{parent},
			Provenance:  config.ProvenanceSecondaryRetention,
		}
		cfg.Sources[merge] = &config.Source{
			Name:        merge,
			Category:    "attacks",
			DerivedFrom: []string{parent, peer},
			Provenance:  config.ProvenanceSecondaryMerge,
		}

		parentTS := base.Add(time.Duration(i) * time.Minute).Unix()
		peerTS := base.Add(time.Duration(i+parents) * time.Minute).Unix()
		entries[parent] = cache.Entry{
			Name:             parent,
			Category:         "attacks",
			SourceDate:       parentTS,
			ProcessedDate:    parentTS,
			CheckedDate:      parentTS,
			FrequencyMinutes: 60,
		}
		entries[peer] = cache.Entry{
			Name:             peer,
			Category:         "attacks",
			SourceDate:       peerTS,
			ProcessedDate:    peerTS,
			CheckedDate:      peerTS,
			FrequencyMinutes: 60,
		}
		entries[retention] = cache.Entry{
			Name:             retention,
			Category:         "attacks",
			SourceDate:       base.Add(24 * time.Hour).Unix(),
			ProcessedDate:    base.Add(24 * time.Hour).Unix(),
			CheckedDate:      base.Add(24 * time.Hour).Unix(),
			FrequencyMinutes: 60,
		}
		entries[merge] = cache.Entry{
			Name:             merge,
			Category:         "attacks",
			SourceDate:       base.Add(48 * time.Hour).Unix(),
			ProcessedDate:    base.Add(48 * time.Hour).Unix(),
			CheckedDate:      base.Add(48 * time.Hour).Unix(),
			FrequencyMinutes: 60,
		}
		names = append(names, parent, peer, retention, merge)
	}
	return cfg, entries, names
}
