# Size variation

How we report the range of a feed's size across its recent history.

## How we calculate this

Every successful content-changing update records the feed's unique IP count.
The insight reads the last 500 recorded updates and computes the minimum,
maximum, and median unique-IP count.

The headline reports the `min` and `max` values directly. The median is used only to suppress the rule when min equals max — i.e. when there is no range to report.

## Threshold

The rule fires whenever `min != max` and the median is greater than zero. There is no ratio threshold because a stable size (e.g. ±3%) is itself a factual observation worth displaying.

## Sample size

Requires at least 50 recorded updates. Below this, the series is too short to characterise the feed.

## When this rule would be wrong

- A feed that was renamed mid-history can carry over observations from both
  identities. We accept this because renames are rare and the headline remains
  tied to observed public data.
- A feed whose maintainer recently changed retention policy will show a step in the series that widens the reported range. That is not wrong — it is precisely the information the headline is designed to surface.
- When history sync from the production server is incomplete the range may reflect only a partial window. We mitigate this by using `len(SizeSeries)` in the headline so the user always knows how many updates the range covers.
