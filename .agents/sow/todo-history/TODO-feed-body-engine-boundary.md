## Purpose

Define the downloader/engine boundary so the pipeline is fit for purpose:
simple, restart-safe, bash-compatible, downloader-only parsing, and parallel
downloader/engine operation without shared ownership ambiguity.

## TL;DR

user clarified the intended architecture:

1. Downloader downloads raw upstream source to `.source`.
2. `.source` is retained only for debugging / inspection and is not consumed by
   the rest of the application.
3. Downloader parses, cleans, normalizes, and renders the canonical text feed
   body to `.{ip,net}set.new`.
4. `.{ip,net}set.new` is the canonical staged source of truth for the feed.
5. Engine claims staged work by moving `.{ip,net}set.new` to
   `.{ip,net}set.processing` when processing starts.
6. Engine processes `.{ip,net}set.processing` to generate website artifacts,
   comparisons, insights, and related downstream outputs.
7. On successful completion, engine promotes `.{ip,net}set.processing` to the
   committed `.{ip,net}set`.
8. On restart:
   - any `.{ip,net}set.new` MUST be moved to `.{ip,net}set.processing`
   - any existing `.{ip,net}set.processing` MUST be automatically queued to the
     engine
9. Any source-format parsing that currently exists in the engine MUST move to
   the downloader.
10. The downloader must safely accept arbitrary source files, including very
    long lines, because upstream sources can be HTML, JSON, compressed content,
    or other non-line-friendly formats before normalization.
11. Downloader parsing must happen once per feed run; the downloader must
    determine its result enum/status by comparing the canonical parsed feed body
    with the last committed version, not by leaving that comparison to the
    engine.

## Analysis

Current specs contradict this model in multiple places:

- `specs/downloader.md` currently says downloader produces canonical "feed
  bodies" to `.source`-family files and the engine consumes them.
- `specs/processing-engine.md` currently says the engine renders the canonical
  committed set output (`.ipset` / `.netset`).
- `specs/files-layout.md` currently assigns `.source` to downloader-owned feed
  bodies and `.ipset` / `.netset` to engine-owned committed canonical outputs.
- `specs/pipeline.md` currently defines "feed body" as downloader output to
  engine input, but the surrounding wording still assumes the engine renders
  canonical set outputs from those feed bodies.

Historical / live behavior reviewed earlier does not match the current specs
cleanly either, so the spec set needs an explicit rewrite around this decision.

Current implementation also contradicts the intended boundary:

- canonical parse/render work still exists inside the engine path
- the engine currently scans downloader-side source-derived input via
  `bufio.Scanner`
- `bufio.Scanner` line-token limits are causing failures such as
  `bufio.Scanner: token too long` for raw upstream-shaped content
- plain-feed downloader flow currently normalizes/parses during downloader
  staging and then the engine parses the staged output again during processing,
  so feed parsing work is duplicated

## Decisions

Made by user on 2026-04-22:

- `.source` is raw downloaded source, retained for debugging only.
- All parsing / extraction / cleanup of upstream source happens only in the
  downloader.
- The downloader emits canonical plain-text feed data as `.{ip,net}set.new`.
- `.{ip,net}set.new` is the staged canonical source of truth for the feed.
- Engine input is `.{ip,net}set.processing`, obtained by renaming from `.new`
  when processing starts.
- The engine does not parse upstream source formats.
- The engine does not define or render the canonical feed body format.
- The engine processes canonical staged feed files to produce downstream
  artifacts and, on success, promotes `.processing` to committed
  `.{ip,net}set`.
- Restart recovery must adopt both `.new` and `.processing` as resumable
  engine work.
- The downloader and engine must be able to operate in parallel without
  conflicting over the same committed/staged file.
- The engine-side parser must be removed from any responsibility for raw-source
  normalization.
- Downloader-side source handling must support arbitrary long lines / large
  tokens without failing on `bufio.Scanner` defaults.
- Downloader result status must be based on canonical parsed content, so
  semantic `same` / `downloaded` / `empty` decisions do not require engine-side
  reparsing.

## Plan

1. Identify every spec section that currently assigns canonical feed-body
   ownership to `.source` or to the engine.
2. Rewrite the relevant specs so terminology is consistent:
   `.source` = raw debug artifact, `.{ip,net}set.new` = staged canonical feed
   body, `.{ip,net}set.processing` = engine-claimed in-flight canonical feed
   body, `.{ip,net}set` = committed canonical feed body.
3. Audit implementation against the corrected contract.
4. Move raw-source parsing/canonicalization fully into the downloader and make
   downloader status decisions from that canonical parse.
5. Remove duplicate engine-side parse/normalize work for canonical feed-body
   admission.
6. Implement the file-state machine and restart recovery semantics.
7. Add tests for downloader/engine parallelism, restart recovery, and very
   large-line source handling.

## Implied decisions

- Canonical feed bodies remain text, not binary, for bash compatibility.
- `iprange` and other downstream consumers should ingest the canonical
  `.{ip,net}set*` text files directly.
- The engine promotion boundary is success of downstream artifact generation,
  not downloader completion.
- Feed enable/disable state for normal/public source feeds must not piggy-back
  on `.source`, because `.source` is downloader-only debug retention. Use a
  dedicated source enable marker file instead.
- The chosen normal-source enable marker is `data/{feed}.enabled`.
- Artifact parents keep their existing dedicated enable marker under
  `lib/artifacts/{artifact}/enabled`.

## Testing requirements

- Downloader writes `.source` and `.{ip,net}set.new` correctly.
- Engine claims `.new` by atomic rename to `.processing`.
- Engine promotes `.processing` to committed `.{ip,net}set` only on success.
- Restart recovery handles leftover `.new` and `.processing`.
- Downloader can produce a fresh `.new` while engine is processing a previous
  `.processing`.
- Crash/restart scenarios preserve correctness and queue recovery.
- Downloader handles sources with extremely long lines without scanner-token
  failures.

## Documentation updates required

- `specs/downloader.md`
- `specs/processing-engine.md`
- `specs/files-layout.md`
- `specs/pipeline.md`
- `specs/compatibility.md`
- `docs/migration-from-bash.md` if migration semantics change
