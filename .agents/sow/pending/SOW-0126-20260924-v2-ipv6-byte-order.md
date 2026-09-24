# SOW-0126 - Correct the v2 IPv6 binary byte order

## Status

Status: open

Sub-state: recorded, not started. No code change is authorized until the
compatibility decision below is made.

## Requirements

### Purpose

Make a v2 IPv6 binary file written by this repository readable by the C
`iprange` tool and by both v4 engines, without silently corrupting addresses
and without breaking cache files this repository has already written.

### User Request

Create an update-ipsets SOW for the v2 IPv6 byte-order bug found by the
iprange review wave. Do not flip the reader blindly.

### Assistant Understanding

Facts:

- `pkg/iprange/binary6.go:69-72` writes each u128 as high 64 bits, then low
  64 bits.
- `pkg/iprange/fileset6.go:228-237` reads the same high-then-low order.
- C `iprange` writes the native `uint128_t` layout. On little-endian that is
  low 64 bits, then high 64 bits (`src/ipset6_binary.c:292`).
- Both v4 engines write the C layout. The Go writer says so at
  `v4/go/internal/cli/legacy/binary.go`.
- This repository is self-consistent: a file it writes, it can read back.
  A file written by C or v4 is read with the halves swapped. A file written
  here is rejected or misread by C and v4.
- v2 files are persisted. Callers include `WriteBinary6` from
  `pkg/iprange/print6.go:28`, and readers include `fileset6.go`,
  `range_source6_indexed.go`, and `fileset6_mmap.go`. Cache names include
  `feed-presence-v1.bin` and `comparison-pairs-v2.bin`.

Inferences:

- A one-line swap of the reader fixes new C/v4 files and breaks every v2
  cache this repository has already written.
- The safe fix is a version or a detection rule, plus a migration for old
  caches. That choice is not implied by the bug report.

Unknowns:

- Whether any published artifact, not only a local cache, is a v2 file
  written by this repository.
- Whether old caches can be deleted and rebuilt, or must be read in place.

### Acceptance Criteria

- A v2 file written by C `iprange` is read with the same addresses by this
  repository, on a little-endian host.
- A v2 file written by this repository after the fix is read with the same
  addresses by C `iprange` and by both v4 engines.
- An old v2 file written by this repository before the fix is either read
  correctly or rejected with an explicit error. It is never silently swapped.
- A committed fixture contains C-produced v2 bytes and the expected addresses.

## Analysis

Sources checked:

- `pkg/iprange/binary6.go:69-72`
- `pkg/iprange/fileset6.go:228-237`
- firehol/iprange `src/ipset6_binary.c:292`
- firehol/iprange `v4/go/internal/cli/legacy/binary.go`
- firehol/iprange `v4/rust/iprange-cli/src/legacy/binary.rs`

Current state:

- The reader and writer agree with each other and disagree with C and v4.
- No test in this repository compares v2 bytes with a C-produced file.

Risks:

- Swapping only the reader corrupts existing caches.
- Swapping reader and writer together makes old caches unreadable.
- Leaving the bug blocks the thin-wrapper migration: v4 will publish files
  this repository cannot read.

## Pre-Implementation Gate

Status: needs-user-decision

Problem / root-cause model:

- The Go u128 codec stored the high half at the lower address. C stores the
  native register order, which is the low half first on little-endian hosts.
  The two codecs were never compared against the same bytes.

Evidence reviewed:

- The files listed above. No external open-source repository was checked.
  The C oracle is firehol/iprange.

Affected contracts and surfaces:

- `pkg/iprange` v2 IPv6 read and write, persisted cache files, and any
  published v2 artifact.

Existing patterns to reuse:

- The C writer is the byte oracle. A fixture captured from `iprange -6`
  binary output is the test.

Risk and blast radius:

- Every persisted v2 cache. A wrong choice silently changes IP addresses.

Sensitive data handling plan:

- Fixtures use documentation addresses only. No customer addresses, cache
  contents, or host paths are copied into this SOW.

Implementation plan:

1. Decide the compatibility rule.
2. Add a C-produced v2 fixture and a test that fails on the current reader.
3. Change the codec and the cache migration together.

Validation plan:

- The new fixture test, a round trip through C `iprange`, and a read of one
  old cache file under the selected compatibility rule.

Artifact impact plan:

- AGENTS.md: unaffected unless the cache migration changes an operator step.
- Runtime project skills: unaffected.
- Specs: update the v2 binary description if this repository has one.
- End-user/operator docs: update only if operators must rebuild caches.
- End-user/operator skills: unaffected.
- SOW lifecycle: this file stays pending until the compatibility decision.

Open-source reference evidence:

- None beyond firehol/iprange, which is the C oracle for this bug.

Open decisions:

1. How to treat v2 files already written by this repository.

## Implications And Decisions

1. How to treat old v2 files.
   - A. Rebuild caches. New files use the C byte order. Old files are
     detected and rejected, not swapped.
     - Benefit: no silent corruption, and the migration is explicit.
     - Cost: the first run after the fix rebuilds IPv6 caches.
     - Risk: low, if every persisted v2 file is a cache.
   - B. Read both orders. A header version or a checksum selects the codec.
     - Benefit: old caches keep working.
     - Cost: two codecs forever, and a wrong selector still swaps addresses.
     - Risk: medium. Detection must be proven, not assumed.
   - C. Swap the reader only.
     - Benefit: none that survives the first old cache read.
     - Cost: existing caches return wrong addresses.
     - Risk: unacceptable.

Recommendation: option A, long-term-best, unless an inventory shows a
published v2 artifact that cannot be rebuilt. Option C is not acceptable.

No option is selected. Implementation is blocked.

## Plan

1. Inventory persisted v2 files and decide option A or B.
2. Add the C fixture test.
3. Change the codec and the cache rule in one commit.

## Execution Log

### 2026-09-24

- Recorded the bug and the compatibility choice. No code was changed.

## Validation

Pending. No implementation is authorized.

## Outcome

Pending.

## Lessons Extracted

Pending.

## Followup

None yet.

## Regression Log

None yet.
