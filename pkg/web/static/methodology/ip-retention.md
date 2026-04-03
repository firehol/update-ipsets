# IP retention

How we measure how long removed IPs stayed in a feed before the maintainer
dropped them.

## What it shows

On the public feed-detail page this is the right-hand retention panel:

- **Retention — age at removal**

It answers:

> Once an IP enters this feed, how long does it usually stay before removal?

This is a fact about the maintainer's retention policy, not a claim about the
IP's real-world maliciousness.

## Data source

The same observation window that powers data freshness also records removals.

Whenever an observed update removes IPs that were present in an earlier
cohort, the product records how long those IPs stayed listed and aggregates the
result into the removal-age histogram.

## How the current UI uses it

The current UI renders the removal-age histogram directly and overlays the same
percentile markers used elsewhere in the retention section:

- **p75**
- **p90**
- **p100**

The panel also surfaces state explicitly:

- if there are no observed removals yet, it shows an explanatory notice instead
  of an empty chart
- if the feed's current state predates the observation window
  it warns that the removal-age picture is based on observed removals only and
  is therefore partial

The methodology pages for the retention insights are separate because the
insight rules are not the same as the chart itself:

- [Removed IP duration (p75)](/methodology/removed-age-p75)
- [Multiple retention policies](/methodology/multiple-retention-policies)
- [Permanent bans](/methodology/permanent-bans)

## What the metric means

Retention is based on the local observation window and on actual observed
removals.

- current entries that have not yet been removed do not appear in the removal
  histogram
- if a feed almost never removes IPs, the histogram stays sparse
- if a maintainer changes policy, the histogram becomes multi-modal instead of
  "averaging away" the change

## Edge cases

- A feed with no observed removals has no meaningful `past` histogram yet.
- A newly tracked feed may need a long time before meaningful removal history
  accumulates.
- If many currently-listed IPs predate the observation window, the removal-age
  histogram still reports only the removals we have actually seen. It does not
  claim complete lifetime coverage for the older population.
- Long-lived historical policy changes remain visible in the histogram. That is
  intentional: the chart reports what we observed, not only the maintainer's
  most recent behavior.
