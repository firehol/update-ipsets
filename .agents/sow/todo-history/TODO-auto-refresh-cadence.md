Purpose
- Design an automatic cadence-discovery system that learns when each feed actually updates and schedules refreshes at the right frequency and wall-clock time, while allowing operators to opt out for sensitive or fixed-time feeds.

TL;DR
- Today the user configures each feed's refresh cadence explicitly.
- Goal: the system should learn cadence automatically over time, including likely publication time, with an opt-out for feeds that must remain manually scheduled.

Analysis
- Current feed configuration includes an explicit per-feed `frequency` / `frequency_minutes` schedule contract.
- Scheduler behavior today is frequency-driven, with failure backoff and small margin logic.
- The engine already tracks observed source update history and derives:
  - average update cadence
  - minimum update cadence
  - maximum update cadence
- Admin and public UI already expose observed cadence as `avg_update_mins` and related fields.
- This means the project already has the raw observations needed for a learning scheduler, but the scheduler still uses user-configured cadence as the primary trigger source.

Decisions
- Pending: whether automatic cadence discovery replaces manual cadence by default or is opt-in first.
- Pending: the exact operator-facing configuration flag to disable/lock cadence discovery for selected feeds.
- Pending: whether the learned scheduler should target:
  - only frequency
  - frequency plus likely wall-clock release time
  - frequency, wall-clock time, and confidence window

Plan
- Inspect current scheduler and history-stat code paths.
- Research prior-art algorithms/patterns for adaptive polling / publication-time prediction.
- Compare candidate algorithms against update-ipsets constraints:
  - many feeds
  - cheap no-change downloads but still not free
  - some feeds are sensitive to excessive checks
  - feeds often follow cron-like publication times
  - operators need explicit override / opt-out
- Return a design recommendation before any implementation.

Implied decisions
- Learned cadence must be stable and bounded; it cannot oscillate wildly on noisy data.
- Sensitive or operator-pinned feeds need a hard bypass.
- The system should converge gradually and remain explainable in the admin UI.

Testing requirements
- Build synthetic feed histories for hourly/daily/weekly cron-like feeds.
- Cover feeds that shift publication time seasonally or drift slowly.
- Cover feeds with bursty/manual publication.
- Cover failure periods and no-change periods.

Documentation updates required
- `specs/config.md`
- `specs/pipeline.md`
- `specs/admin-ui.md`
