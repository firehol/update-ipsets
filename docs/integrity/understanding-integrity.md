# Understanding Integrity

You will learn what integrity checks, how it differs from health, and when it runs.

## What integrity answers

Integrity answers one question: does the local on-disk state still match the last successful publication?

It checks local correctness. It does not check whether upstream feeds are alive, reachable, or current. A feed can be completely healthy upstream and still have a local integrity problem if an output file was lost or corrupted after publication.

## Integrity vs health

These are separate concerns.

**Health** asks: is this feed behaving like a healthy upstream source?

**Integrity** asks: does the local product state still match the last successful local publication?

Do not confuse:

- a dead upstream feed (health problem)
- an empty but successful feed (fine — just empty)
- missing local outputs after a claimed successful publication (integrity problem)

## When integrity runs

Integrity runs at three points:

1. **Startup** — the daemon inspects settled local state and queues repair work without blocking service availability.
2. **After processing settles** — once the scheduler finishes a run, integrity re-evaluates to catch local drift.
3. **On operator request** — the admin UI integrity panel lets you trigger a fresh evaluation at any time.

## What integrity skips

Integrity does not check in-flight work. When a processing run is active, integrity reports `in progress` instead of emitting transient findings about files that are still being written.

Integrity also does not report on:

- feeds that are gone upstream and have no local rebuild path
- IPv6 feeds for IPv4-only features like critical-infrastructure overlap
- configured providers that have not been materialized yet

## What you see

When integrity finds an issue, you know one of two things:

- the local state lost or corrupted outputs it claimed to have
- the local state needs a recheck or reprocess to restore consistency

When integrity reports no issue, the local published state is self-consistent with the last successful run.
