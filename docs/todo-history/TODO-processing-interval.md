# TODO - Processing Interval 5 Minutes

## Purpose

Reduce the automatic processing-loop cadence from 10 minutes to 5 minutes,
while preserving the current queue semantics and documenting exactly what
happens when a processing run takes longer than the configured interval.

## TL;DR

- Costa asked to set the processing cadence to 5 minutes.
- The current live install does not override `processing_interval_minutes`, so
  it still uses the code default of 10 minutes.
- The scheduler uses a single timer-based processing loop plus immediate wake
  events for manual actions and startup recovery.

## Analysis

- The runtime default is currently `ProcessingIntervalMinutes: 10` in
  `pkg/config/config.go`.
- The installed config at `/opt/update-ipsets/etc/config.yaml` does not define
  `processing_interval_minutes`, so the live daemon uses the code default.
- The processing loop is in `pkg/scheduler/scheduler.go:294`.
- The automatic timer path behaves like this:
  - wait for `interval`
  - run `runQueuedProcessing(ctx)`
  - reset the timer after that run returns
- This means an automatic run that lasts longer than the interval does not
  cause overlapping processing runs.
- For automatic timer-triggered runs, the next automatic timer starts counting
  after the long run finishes.
- Immediate wake signals (`processWake`) are handled on a separate select case.
  If one is queued while a long run is in progress, it is processed after the
  current run returns.

## Decisions

### Made by Costa

- Set the processing cadence to 5 minutes.

## Plan

1. Update the code default from 10 minutes to 5 minutes.
2. Add the explicit runtime setting to the shipped config template for clarity.
3. Update the live installed config to 5 minutes.
4. Restart the daemon and verify it is running cleanly.
5. Explain the exact overrun behavior for a 6-minute run.

## Implied decisions

- Keep the current single-run, no-overlap scheduler behavior unchanged.
- Do not change fetch cadence or per-feed frequency semantics.

## Testing requirements

- Verify the service restarts successfully.
- Verify the daemon still reports healthy after the config change.

## Documentation updates required

- None required beyond the TODO for this small runtime change.
