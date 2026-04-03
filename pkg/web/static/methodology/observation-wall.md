# Currently-listed IPs predate observation

How we detect the case where the currently-listed age histogram has reached the
edge of our observation window and ordinary age percentiles stop being useful.

## Why this rule exists

Suppose we started tracking a feed 8 days ago and almost every IP currently in
the feed was already present on day one.

In that case:

- p75 of currently-listed age is about 8 days
- p100 of currently-listed age is also about 8 days

Those numbers are true, but they do **not** mean the maintainer keeps IPs for 8
days. They mean we have only been watching for 8 days.

This rule replaces the misleading percentile headlines with one honest
statement about the observation boundary.

## How we calculate it

The rule uses the currently-listed age histogram for the feed.

It then computes:

- `observation_hours` = how long we have been tracking the feed
- a wall threshold at `0.9 × observation_hours`
- `walled_share` = the fraction of currently-listed IPs whose age bucket is at
  or beyond that wall threshold

If:

- at least 100 currently-listed IPs exist
- `observation_hours > 0`
- `walled_share >= 0.5`

the rule fires.

The 0.9 factor is deliberate. Bucketed age data is coarse, so we treat ages
near the edge of the observation window as effectively "at the wall".

## What it suppresses

When this rule fires, the product suppresses:

- [Currently listed age (p75)](/methodology/currently-listed-age-p75)
- [Oldest currently listed IP](/methodology/currently-listed-age-p100)

That prevents three near-duplicate headlines from describing the same
observation-window limit.

## What the headline means

The headline is not saying the IPs are young or old in any global sense.

It is saying:

> Most of the currently-listed IPs were already present when we started
> observing, so we cannot measure their true pre-observation age.

## Edge cases

- A feed can legitimately have a strong observation wall because it is stable
  and we started tracking it recently.
- Once the feed has been observed for longer and enough newer cohorts appear,
  the wall share drops and the ordinary p75/p100 rules can become useful again.
