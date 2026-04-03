# Evolution

How the public product tracks a feed's size over time.

## What it shows

On the feed-detail page, the Behaviour section includes:

- **IP count evolution**

It plots the feed's unique-IP count across the most recent published history
window.

The related overview insight is documented separately at
[Size variation](/methodology/size-variation).

## Data source

Every successful content-changing update records the feed's observed timestamp,
entry count, and expanded unique IPv4 count.

## Windowing

The public chart does **not** use the entire lifetime ledger.

The public chart uses the most recent **500** observed updates. So the chart
answers:

> How has this feed's size moved over the last 500 observed updates?

not:

> How has this feed moved across all history since the project began?

## Current product usage

- The feed-detail Behaviour section renders the size chart from the bounded
  public history window.
- The same bounded history also feeds the cadence chart in the Behaviour
  section.
- The `size_variation` insight derives its min/max range from the same bounded
  series, with its own sample-size guard.

## Edge cases

- If fewer than two history points exist, the public chart has little to show.
- Once a feed exceeds the public cap, older lifetime history remains in the
  internal ledger but drops out of the published chart window.
- The chart reports unique IP count, not entry count. A feed can keep a similar
  number of lines while its expanded address space changes significantly.
