# Unique share

## What this measures

For every tracked public feed, unique share reports how many of the feed's IPs
are **not** already covered by its single most-overlapping independent peer.

The number is always between 0 and 100. Higher means the feed carries
more IPs that at least one specific peer does not — useful when ranking
feeds by how much new ground they add to an existing blocking policy that
already uses one similar feed.

## Exact definition

Let `F` be the feed. Let `P(F)` be the set of independent peer feeds of
`F`. A feed `G` is an **independent peer** of `F` when all of the
following hold:

- `G` is a public source (not a provider-role source such as an ASN or
  geolocation database).
- `G` is not hidden.
- `G`'s provenance is `primary` or `secondary_upstream`. Feeds derived
  by retention-window or merge expansion of `F` (or of any feed in
  `F`'s family) are excluded — they would overlap `F` trivially.
- `G`'s maintainer is different from `F`'s maintainer when both are
  known. Feeds published by the same maintainer often share collection
  infrastructure and their overlap says more about the maintainer than
  about the feed.
- `G` is not in the same positive derivative family as `F` (no shared positive
  leaf ancestor). For signed merges, subtractive inputs are dependencies, not
  positive ancestors.

For each `G` in `P(F)`, the product already knows `|F ∩ G|` from the pairwise
comparison pass. Let

    max_common = max over G in P(F) of |F ∩ G|

Then

    unique share = 100 × (|F| - max_common) / |F|

bounded to the interval `[0, 100]`.

The sample count is `|P(F)|` — the number of independent peers this computation
compared `F` against. A low sample count (e.g. 0 for a brand-new unique feed)
is a useful companion signal to the percentage.

## What this is not

This metric is a bounded proxy. It does **not** compute the true N-way
union of peer feeds, so:

- A feed that shares a different half of its IPs with each of two
  independent peers can still report 50% unique share, even though every one
  of its IPs is covered by some peer.
- A feed with 100% overlap against a single tiny peer and 0% overlap
  against all the rest will report low uniqueness against that one
  peer.

The proxy is chosen because it is honest, O(1) to compute from data we
already have, and rarely pessimistic in the direction operators care
about: a high unique-share value is a strong signal that the feed adds IPs not
already covered by any single peer. Low values deserve a second look.

## Important scope note

This methodology page describes the catalog-level unique-share value used by
the public catalog and homepage explorer.

The feed-detail Overlap section also renders a "Unique" tile, but that tile is
a local UI summary derived from the raw comparison rows. It is related, but it
is not the catalog-level unique-share value.

## When the metric updates

The value is refreshed when the feed's pairwise comparisons are refreshed. On
full rebuilds, every feed's value is refreshed. On incremental runs, only feeds
whose comparison rows may have changed are touched; all other feeds retain their
previous value.

## Where to see it

- Public catalog API: `/api/v1/sets` exposes the unique-share value and sample
  count on every feed row.
- Homepage explorer: the table view renders the value as a column, and
  explorer views can sort or filter by it.
