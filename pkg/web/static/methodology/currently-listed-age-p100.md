# Oldest currently listed IP

How we report the longest-running IP in a feed.

## How we calculate this

The currently-listed age distribution is a histogram keyed by
hours-since-first-seen. The rule takes the largest populated bucket — the
methodology's `p100`.

## Threshold

The rule fires once the sample-size guard is met **unless** either of these is
true:

1. the observation-wall rule already explains the result
2. `p100 == p75`, so the p75 rule already conveys the same number

## Sample size

Requires at least 100 currently-listed IPs. Below this a single long-tenured IP would produce a misleading "oldest is X years" headline.

## When this rule would be wrong

- On a feed we have only just begun tracking, the oldest IP we have a record
  for may be much younger than the IP's actual first appearance. The retention
  chart marks this case when the age distribution is incomplete.
- Hour bucket resolution rounds ages down to the nearest hour, so a very recently added IP will report 0 h rather than "a few minutes".
