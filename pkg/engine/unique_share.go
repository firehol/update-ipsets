package engine

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
)

// updateUniqueShares recomputes the unique_share_pct and
// unique_share_samples fields for every feed whose comparison rows
// may have changed during the current run. When names is nil, every
// feed with an on-disk _comparison.json is refreshed (initial run or
// full rebuild). When names is empty (but not nil), nothing is done.
//
// The metric is a bounded proxy for "how independent is this feed
// from its closest peer": given the feed's pairwise Common counts
// against every independent peer, the share of the feed's IPs that
// are NOT in the peer with the largest overlap. It never computes
// the true N-way union, so a feed that shares a different half of
// its IPs with each of two peers will still register as
// "60% unique" if its strongest pairwise overlap is 40%. The
// methodology page at /api/v1/methodology/unique-share documents the
// exact definition.
//
// Errors loading an individual comparison file are logged and skipped;
// a missing or malformed file leaves the cached fields untouched.
func (e *Engine) updateUniqueShares(names []string, outDir string) {
	e.updateUniqueSharesContext(context.Background(), names, outDir)
}

func (e *Engine) updateUniqueSharesContext(ctx context.Context, names []string, outDir string) {
	ctx = nonNilContext(ctx)
	if e == nil || e.cfg == nil {
		return
	}
	if names != nil && len(names) == 0 {
		return
	}

	// Which feeds to recompute: either the provided names, or every
	// output feed on a full-rebuild run (names == nil).
	targets := names
	if targets == nil {
		targets = e.publicOutputNames()
	}
	progress := e.beginActiveOperation("metadata.update_unique_shares", "", "update", "feeds", int64(len(targets)))
	defer progress.Finish()

	liveOutDir := e.outputDir()

	// Build the set of peers that count as "independent" for a given
	// feed. We reuse the same relatedness predicate writeComparisonFiles
	// uses (leaf-ancestor overlap) so our definition of "independent"
	// matches the Related flag already on each CompareRow. Provenance
	// and maintainer filters are applied on top of that.
	familyCache := make(map[string]map[string]bool, len(e.cfg.Sources))
	familyFor := func(name string) map[string]bool {
		if cached, ok := familyCache[name]; ok {
			return cached
		}
		family := leafAncestors(e.cfg, name)
		familyCache[name] = family
		return family
	}
	isRelated := func(a, b string) bool {
		fa := familyFor(a)
		fb := familyFor(b)
		smaller, larger := fa, fb
		if len(fb) < len(fa) {
			smaller, larger = fb, fa
		}
		for k := range smaller {
			if larger[k] {
				return true
			}
		}
		return false
	}

	for _, name := range targets {
		if err := contextErr(ctx); err != nil {
			return
		}
		entry := e.state.Entry(name)
		if entry == nil {
			progress.Add(1, int64(len(targets)), nil)
			continue
		}
		selfSrc := e.lookupSource(name)
		if selfSrc == nil {
			progress.Add(1, int64(len(targets)), nil)
			continue
		}
		selfMaintainer := strings.TrimSpace(strings.ToLower(entry.Maintainer))

		rel := name + "_comparison.json"
		data, err := readFirstExisting(singleCandidatePath(outDir, rel), singleCandidatePath(liveOutDir, rel))
		if err != nil {
			progress.Add(1, int64(len(targets)), nil)
			continue
		}
		var rows []CompareRow
		if err := json.Unmarshal(data, &rows); err != nil {
			progress.Add(1, int64(len(targets)), nil)
			continue
		}
		if entry.UniqueIPs == 0 {
			applyUniqueShareResult(e.state, name, 0, 0)
			progress.Add(1, int64(len(targets)), nil)
			continue
		}

		var maxCommon uint64
		samples := 0
		for _, row := range rows {
			if row.Related {
				continue
			}
			peer := e.lookupSource(row.Name)
			if !isIndependentPeer(peer, selfSrc) {
				continue
			}
			if isRelated(name, row.Name) {
				continue
			}
			peerEntry := e.state.EntrySnapshot(row.Name)
			if peerEntry == nil {
				continue
			}
			peerMaintainer := strings.TrimSpace(strings.ToLower(peerEntry.Maintainer))
			if peerMaintainer != "" && peerMaintainer == selfMaintainer {
				continue
			}
			samples++
			if row.Common > maxCommon {
				maxCommon = row.Common
			}
		}

		var unique float64
		if maxCommon < entry.UniqueIPs {
			unique = 100.0 * float64(entry.UniqueIPs-maxCommon) / float64(entry.UniqueIPs)
		}
		if samples == 0 {
			// No independent peers yet — every IP is trivially unique
			// among the peers we have. Document this state in the
			// sample count rather than hiding it.
			unique = 100.0
		}
		applyUniqueShareResult(e.state, name, unique, samples)
		progress.Add(1, int64(len(targets)), nil)
	}
}

// applyUniqueShareResult writes the unique_share fields back to the
// cache.Entry for the given feed. The state mutation is isolated so
// future adjustments (e.g. recording a timestamp) have a single
// writer.
func applyUniqueShareResult(state *cache.State, name string, pct float64, samples int) {
	if state == nil {
		return
	}
	entry := state.Entry(name)
	if entry == nil {
		return
	}
	entry.SetUniqueShare(pct, samples)
}

// isIndependentPeer returns true when the peer source counts as an
// "independent" peer for the uniqueness metric: it must be a real
// public source (not a provider role), its provenance must be
// primary or upstream, and it must not be the self-reference.
func isIndependentPeer(peer *config.Source, self *config.Source) bool {
	if peer == nil || self == nil {
		return false
	}
	if peer == self {
		return false
	}
	if peer.Hidden {
		return false
	}
	if peer.HasUse(config.UseGeoIP) || peer.HasUse(config.UseASN) {
		return false
	}
	prov := publicProvenance(peer)
	if prov != config.ProvenancePrimary && prov != config.ProvenanceSecondaryUpstream {
		return false
	}
	return true
}
