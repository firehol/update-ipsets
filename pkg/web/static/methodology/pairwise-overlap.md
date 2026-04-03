# Pairwise overlap

How we compute the overlap between one feed and every other public feed.

## What it shows

The feed-detail Overlap section compares the selected feed with every other
public feed.

For each other feed with at least one shared IP it publishes:

- **Overlap** — how many IPs the two feeds have in common
- **% of this** — `common / this_feed_ips`
- **% of other** — `common / other_feed_ips`
- **Health** — the peer feed's current public health state, joined from the
  live public feed catalog so the overlap surface can warn about stale peers

These percentages are directional. A small specialist feed can be fully
contained in a large broad feed while contributing only a tiny fraction back.

## Data source

Each comparison row contains:

- `name` — the other feed
- `category` — the other feed's category
- `ips` — total unique IPs in the other feed
- `common` — IPs present in both feeds
- `related` — whether the other feed belongs to the same positive derivative
  family

The public UI joins comparison rows with the current public feed catalog to show
each peer's current health. Health is time-derived and can change without the
pairwise overlap counts changing.

## How it is computed

The product compares both feeds as normalized IPv4 ranges and computes their
IP-space intersection.

This is done on the **expanded unique IP space**, not on raw lines:

- a `/24` and one IP inside that `/24` overlap by **1 IP**
- two different CIDR spellings that cover the same address space overlap fully

## Current UI behavior

The Overlap section has two layers:

1. overlap rows
   - the table view uses the non-zero comparison rows as published
   - each overlap row also shows the peer feed's current health from the live
     public catalog
   - the sankey view keeps the 14 strongest overlaps for readability and labels
     that truncation locally
   - the network view keeps the 24 strongest overlaps for readability and
     labels that truncation locally
2. summary tiles
   - `Included in`
   - `Includes`
   - `>=50% overlap`
   - `Unique`

Those summary tiles intentionally **exclude rows where `related == true`**.
Related rows are tautological overlaps such as:

- a retention derivative vs its parent
- a merge vs one of its additive inputs
- feeds in the same positive leaf-ancestor family

For signed merges, subtractive inputs are dependencies but not positive
ancestors. A merge that subtracts a feed is not marked related to that feed just
because the feed was used for exclusion.

The full overlap table still shows those related rows when their overlap is
non-zero.

When a feed that is itself neither `archived` nor `unmaintained` has
structural overlap with peers that are currently `archived` or
`unmaintained`, the overlap section shows a local warning. The purpose is to
stop users from reading a strong relationship as fresh corroboration when part
of that relationship is stale upstream composition.

The feed-detail `Unique` tile is a local proxy derived from the strongest
independent overlap. It is related to, but distinct from, the catalog-level
unique-share metric described in [Unique share](/methodology/unique-share).

## Freshness rule

Comparison facts are refreshed whenever a feed changes in a way that can affect
either side of the relationship. The goal is that the overlap shown on feed A's
page stays current even when only feed B changed recently.

## Edge cases

- A feed with zero IPs publishes an empty comparison list; zero-overlap rows are
  omitted because absence is the public representation of no overlap.
- Very broad reference feeds can overlap many feeds by design; high overlap is a
  fact, not automatically a sign of duplication.
- The comparison rows are pairwise overlaps only. Any "unique" summary derived
  from them is necessarily a bounded proxy, not a full N-way set subtraction.
