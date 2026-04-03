# Feed health

How update-ipsets classifies each feed as **healthy**, **delayed**,
**risky**, **unmaintained**, **empty**, **unavailable**, or **archived**.

## Why this exists

Different feeds have different expected cadences.

- a fast-moving attack feed going quiet can be a real problem
- a slow baseline or reference feed can be perfectly normal after weeks
- some static baselines are intentionally excluded from age-based degradation

So health combines observed behavior, current failure state, and the expected
cadence for that kind of feed.

## What is considered

Feed health is based on:

- how often the feed has changed in the past
- how long it has been since the last useful local publication
- whether the latest local publication produced any entries
- whether downloads or processing are currently failing
- whether the feed is ordinary threat intelligence or stable reference data
- the expected cadence policy for the feed category

The public site and the admin UI show the same health class. The UI does not
make a separate judgement from raw timestamps.

## What users should look at

The health class is the first signal. The supporting timestamps explain why
the class was assigned.

- **Last change** explains whether the feed content is stale.
- **Last check** explains whether the source has been contacted recently.
- **Failure time** explains how long the current failure has lasted.
- **Observed cadence** explains whether the feed usually changes quickly or
  slowly.

## Age ladder

For feeds that are not empty, not currently unavailable, not archived, and not
excluded from age-based degradation, the product compares time since last
change with the feed's observed cadence and category policy.

Then:

- **healthy**: still inside the expected healthy gap
- **delayed**: outside the healthy gap, but not yet at the risky threshold
- **risky**: old enough that users should review the feed before relying on it
- **unmaintained**: old enough to be treated as no longer actively maintained

## Single-observation grace

Feeds with zero or one observed update do not yet have trustworthy cadence
history.

While the feed has at most one observed update and is still inside the
single-observation grace window, it remains **healthy** on the age ladder.

Once the grace window expires, the normal category thresholds take over.

## Empty

If the latest successful local publication contains zero entries, the class is
**empty** regardless of cadence history.

`empty` is a successful local result, not a failure.

## Unavailable

`unavailable` is separate from the age ladder.

A feed becomes **unavailable** when it is currently failing and the feed has
crossed the ordinary unavailable threshold.

The classifier intentionally considers two lower bounds:

- how long the current failure streak has lasted
- how long it has been since the last usable local refresh beyond the ordinary
  unavailable threshold

This means a currently failing feed can become unavailable immediately if its
last successful local refresh is already old enough.

## Archived

`archived` replaces `unavailable` after the feed has remained unavailable past
the archival window.

Important details:

- `archived` is a derived health state
- it replaces `unavailable` on the health axis
- archived feeds are not retried automatically by ordinary scheduling
- an explicit operator recheck can still succeed and move the feed back into a
  normal health state

## Exclusions and derived feeds

Some sources are reference or provider data rather than ordinary threat feeds.
For those sources, stable content can be correct: a public DNS resolver list,
an ASN database, or a geolocation database may remain useful even when the
published ranges do not change for a long time.

For reference/provider sources:

- `empty` still applies
- `unavailable` still applies
- `archived` still applies
- age-based states (`delayed`, `risky`, `unmaintained`) are suppressed

For one-parent derivatives, the operator-facing health view follows the parent
feed rather than the derivative's own rebuild timestamp.

## Edge cases

- Before the first successful local publication, an enabled feed is
  **unavailable**.
- A successful empty publication is **empty**, not **unavailable**.
- A feed can move straight from currently failing to **archived** if the last
  usable local refresh is already older than the ordinary unavailable threshold
  plus the archival threshold.
