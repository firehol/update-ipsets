# Currently listed age (p75)

How we measure how long the currently-listed IPs have been in the feed.

## How we calculate this

Every successful content-changing update identifies new IPs added since the
previous observation. The product then bucketises every still-listed IP by how
many hours it has been observed in the feed.

The rule reads the currently-listed age histogram and computes the 75th
percentile: the lowest bucket `H` at which the cumulative count of IPs at or
below `H` reaches 75% of the total.

## Threshold

The rule always fires once the sample-size guard is met **unless** the
observation-wall rule is already firing.

That suppression is intentional: if most of the current list sits at the edge
of our observation window, "75% are at most X old" does not add useful
information.

## Sample size

Requires at least 100 currently-listed IPs. Below this the percentile can jump between disparate buckets on a single addition/removal.

## When this rule would be wrong

- A feed we have only just started tracking will report ages relative to our
  first observation, not the feed's own history. The retention chart marks this
  case visually when the age distribution is incomplete.
- A maintainer who temporarily stops updating the feed will shift the distribution upward, making all IPs look older than they actually are.
- Hour buckets are computed against the most recent processed timestamp; a large clock skew on the source server could round the bucket choice by up to an hour.
