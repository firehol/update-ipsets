# Change rate

Change rate is the bounded, catalog-friendly version of feed movement.

## What it measures

For each recorded update, the product compares the previous and current feed
membership:

- `added`
- `removed`
- `kept`

It derives the bounded change ratio:

```text
(added + removed) / (kept + added + removed)
```

This is the share of the combined old-and-new membership that changed.

Because the denominator is the union of old and new membership, the ratio is
always between `0%` and `100%`.

## Why this exists

The feed-detail churn metric is intentionally unbounded:

```text
(added + removed) / size
```

That is analytically useful, but it can exceed `100%` on full refreshes and is
harder to compare quickly in catalog-style views.

Change rate keeps the same underlying movement signal in a bounded form.

## Public fields

The public catalog can expose:

- median change rate
- 75th percentile change rate
- number of samples behind the value

Homepage ranking and filtering can use those values directly.

## What the numbers mean

- `0%` means most updates barely change membership
- `100%` means most updates replace almost the entire effective membership

This is a fact about how the feed moves between observed versions. It is not a
quality score.

## Edge cases

- A cumulative feed naturally looks calmer than a sliding-window feed.
- Median hides spikes. A feed with one explosive refresh and many quiet updates
  can still show a modest median.
- If there are no correlated size/change points yet, the metric stays empty.
