# Update cadence

How we measure how often a feed actually changes.

## What it shows

For every feed we publish three cadence facts:

- **Average update interval**
- **Shortest observed interval**
- **Longest observed interval**

These are about **observed content changes**, not about how often the daemon
checks the source URL.

## Data source

The downloader decides whether a feed produced changed content. When it does,
the product records a new history point for that feed.

Cadence is then derived from the sequence of recorded timestamps in that
history ledger.

The public site and admin UI expose the same cadence values so users and
operators see one consistent interpretation.

## How it is calculated

For a feed with **two or more** strictly increasing history points:

- every gap between consecutive timestamps is measured in seconds
- the average interval is the **arithmetic mean** of those gaps, rounded to
  minutes
- the shortest interval is the smallest observed gap
- the longest interval is the largest observed gap

For a feed with **zero or one** observed update:

- the average/min/max values are seeded from the expected check frequency
- this avoids reporting zero cadence before enough history exists

Timestamps that do not move forward are ignored. Duplicate or backward points do
not affect the cadence window.

## Important nuances

- The timestamp used for a history point is the feed body's observed change
  time, not necessarily wall-clock processing time.
- For remote feeds, the downloader prefers upstream `Last-Modified` when it is
  usable; otherwise it falls back to the current time.
- For synthetic feeds such as merges or retention derivatives, the timestamp is
  whatever timestamp the downloader assigned to the canonical body it produced.
- Cadence is therefore a fact about the published canonical feed as this
  product observes it.

## Current product usage

- The admin feed table and feed modal use the same cadence values.
- Public feed metadata and homepage explorer filters use those same values.
- The feed-detail Behaviour section also visualizes the same bounded history as
  a cadence distribution chart.

## Edge cases

- A brand-new feed reports the expected frequency until at least two observed
  updates exist.
- A maintainer that suddenly changes cadence will widen the min/max window
  immediately, while the average moves as more observations arrive.
- Rarely-changing feeds naturally report long cadence values. That is a fact
  about the feed, not an error in the metric.
