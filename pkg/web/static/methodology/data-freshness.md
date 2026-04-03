# Data freshness

How we measure how old the IPs currently in a feed are.

## What it shows

On the public feed-detail page this is the left-hand retention panel:

- **Freshness — currently listed**

It answers one question:

> How long have the IPs that are still on the list been there, according to our
> observation window?

The chart is a histogram of the currently-listed age distribution, with
percentile markers derived from that histogram.

## Data source

Every successful content-changing update compares the new feed membership with
the previous observed membership. Newly added IPs start a cohort; IPs that
remain listed keep aging with that cohort until they disappear.

For data freshness, only the currently-listed cohort histogram matters:

- each bucket is an age in hours since we first observed that cohort
- the bucket count is how many currently-listed IPs fall into that age

## How the current UI uses it

The feed-detail page does not build a bespoke headline sentence anymore.
Instead it renders the histogram directly and marks:

- **p75**
- **p90**
- **p100** (largest populated bucket)

Those percentile markers are calculated from the current age histogram when the
page loads.

The panel also surfaces three interpretation details directly in the UI:

- if the browser clock looks sane, the histogram is aged forward from
  the latest observed update to "now" so the chart answers "how old are the
  currently listed IPs right now?"
- if the browser clock appears to be behind the latest observed update, the
  panel stays anchored to that update and shows a local warning
- if some currently listed IPs predate the observation window, the panel warns
  that the oldest bucket is a lower bound rather than a complete lifetime fact

The related deterministic insight rules are documented separately:

- [Currently listed age (p75)](/methodology/currently-listed-age-p75)
- [Oldest currently listed IP](/methodology/currently-listed-age-p100)
- [Currently-listed IPs predate observation](/methodology/observation-wall)

## What the metric means

This is not "ground truth age of the IP in the maintainer's database". It is
the age we can prove from the local observation window.

If we started tracking a feed yesterday, an IP already present yesterday cannot
show as older than that window.

## Edge cases

- If no currently-listed cohorts exist yet, the Freshness panel shows an
  explicit "No currently listed IPs yet" notice instead of a misleading empty
  chart.
- If any current cohort predates our own observation start, the chart marks the
  age distribution as incomplete.
- When a large share of the current histogram sits at the observation boundary,
  the product suppresses the ordinary p75/p100 insights and shows the
  observation-wall insight instead.
- A browser clock that is materially behind the latest observed update disables
  the "aged to now" adjustment and is called out locally.
