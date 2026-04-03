---
name: project-reviewing
description: "Code review checklist for update-ipsets. MUST be followed when reviewing code in this repo."
---

## Review priorities

- Correctness of feed processing, generated artifacts, public APIs, and operator actions (evidence: product specs in `.agents/sow/specs/`).
- Performance and bounded resource use: avoid unnecessary CPU, memory, network, and I/O work.
- Security and release hygiene: no secrets, unsafe admin exposure, or unbounded public compute paths.
- Spec consistency: implementation changes must update the canonical `.agents/sow/specs/*.md` owner.
- Surface/audience fit for every non-code artifact: SOWs, specs, docs, public methodology, public UI copy, and admin UI copy each have different jobs.
- Tests and real-use validation appropriate to the touched surface.

## Standard checklist

- Does the change preserve cache-first public serving and avoid request-time broad recomputation?
- Are download, engine, scheduler, background, and public API paths bounded and observable?
- Are all material iprange/file/network/JSON operations represented in telemetry when relevant?
- Does any new background work appear in admin status/UI?
- Are generated files avoided unless produced by the normal build/install flow?
- Are config/catalog changes data-driven instead of hardcoding feed names?
- Are context cancellation, child-goroutine ownership, error wrapping, and
  structured logging preserved?
- Are UI query keys, invalidations, and URL state consistent with nearby code?
- Are docs/specs updated in the canonical owner file?
- Does each non-code artifact match its surface contract from `project-content-surfaces`?
- If a SOW uses words such as deferred, later, follow-up, future, TODO, or
  pending, is every valid remaining item either implemented now, explicitly
  rejected/non-goal with evidence, or linked to a concrete pending SOW path?
- For Go changes, did the review account for current blocking CI gates:
  `govulncheck`, `staticcheck`, and `golangci-lint`? These gates cover both
  the root module and `tools/dronebl2ipsets`, so cleanup in nested modules is
  part of the normal validation surface (from SOW-0045).
- Are public methodology pages free of internal implementation/config/code/SOW details unless a brief link is explicitly necessary?
- Are operator/config/API details in `docs/` or README, not promoted into public methodology pages?
- Are semantic classifications driven by config fields, `use:` roles, typed
  metadata, or exact configured-name identity, without substring/suffix guesses
  over feed/provider/artifact names?
- Are app-created artifacts that integrity checks depend on stamped with an
  explicit logical mtime, not incidental wall-clock write/rename time?
- If the change touches `pkg/engine`, `pkg/web`, `pkg/scheduler`, `pkg/cache`,
  route registration, or large UI components, did `go test ./tools/archposture`
  pass and did the SOW record any accepted posture-regression baseline update?
- Do touched Go tests avoid wall-clock sleeps as synchronization, raw
  `os.Setenv`/`os.Unsetenv`, unstructured log/body substring assertions, and
  legacy benchmark `b.N` loops where `b.Loop()` is available?
- If the change touches a nested Go module, is there an explicit Makefile/CI
  gate for that module instead of relying on root `go test ./...`?
- Do tests that schedule background/admin work wait for the observable output
  or state transition and for the relevant background work to drain, rather
  than only asserting that work was scheduled?
- Do scheduler cancellation tests wait for `Runner.Run` to return after
  cancellation, so cache/staging cleanup is not racing orphaned loop or
  download workers?
- For UI route changes, are top-level pages lazy-loaded behind the shared
  route-level `Suspense` and error boundary instead of inflating the homepage
  entry bundle (from SOW-0033)?
- For UI query/API boundary changes, do emitted chunks prove the public shell
  avoided admin-only and feed-detail-section endpoints? Review the Vite build
  output and, when needed, grep built chunks for endpoint strings; source-level
  split modules are not enough if a shared layout imports a broad query/API
  module (from SOW-0050).
- For UI feature/component claims, is the component actually reachable from a
  route or mounted parent, not just present as dead source? If dead feature
  code is removed, were frontend-only API helpers, types, and direct package
  dependencies removed with it (from SOW-0040)?
- For UI query changes, does every TanStack Query `queryFn` pass its
  `AbortSignal` through to the typed API helper or direct static fetch (from
  SOW-0033)?
- For UI theme changes, is `next-themes` still the single theme owner and are
  Sonner/toast theme reads backed by the same provider (from SOW-0033)?
- For Three/WebGL changes, is unmount cleanup explicit, are data-derived HTML
  labels escaped/sanitized, and was a nonblank canvas/screenshot check run
  when the scene is reachable (from SOW-0033)?
- For clickable UI rows or cards, is keyboard activation present and is there
  an accessible name, or is the interaction implemented as a real link/button
  (from SOW-0033)?
- For UI tests, are assertions black-box behavioral: real components, MSW at
  the network boundary, `userEvent` for interactions, role/text/accessibility
  queries, and no mocked hooks/children/render counts/internal state (from
  SOW-0034)?
- If UI tests are changed, did `make ui-test` and `pnpm --dir ui lint` pass,
  and are new tests colocated as `*.test.tsx` next to the surface under test
  with shared fixtures under `ui/src/test/` (from SOW-0034)?

## Project-specific concerns

- `.agents/sow/specs/*.md` are the canonical product contracts; do not accept a second product-spec source elsewhere (from SOW-0009).
- `pkg/iprange` must stay standalone and should remain suitable for CLI/library use.
- Startup availability matters; guarded repair/background work is preferred over unconditional broad startup rebuilds.
- Merge recomposition is intentionally time-based unless the user explicitly
  approves a scheduling model change (from SOW-0006).
- Entity country/ASN work must stay incremental in normal feed-update paths; full rebuild is a fallback/operator tool (from SOW-0012 regression validation).
- Feed-catalog and parser changes need installed-service or end-to-end validation when the risk is publication correctness; URL/parser smoke checks alone are not enough (from SOW-0008).
- Timing instrumentation must be reviewed for deferred-argument bugs; direct deferred calls evaluate arguments immediately and can make duration metrics wrong (from SOW-0022).
- Review loop/batch code for hidden full-cache snapshots. Calls to `EntrySnapshot`, fresh-snapshot helpers, or health/effective-entry helpers inside loops require explicit justification or conversion to a batch-scoped resolver/classifier (from SOW-0024).
- Raw feed-body surfaces must enforce public-feed eligibility, archival health, and redistributability. Check `/api/v1/sets/{name}/data`, `/api/v1/compose`, `/files/{feed}.ipset|netset`, and direct `/{feed}.ipset|netset` routes together whenever raw serving policy changes (from SOW-0025 follow-up).
- Public artifact readers must not silently fall back to live builders when the configured published artifact tree is missing data. Review global country/ASN API routes together with feed-scoped artifact routes and `Options.WebDir` handling (from SOW-0025 follow-up).
- Public artifacts must not carry valueless explicit empty facts. Review new
  public JSON surfaces for avoidable zero/empty rows, and for comparison
  artifacts specifically require `common > 0` rows only plus stale-row cleanup
  on incremental updates (from SOW-0026).
- Raw body review must include cache-entry file trust boundaries: reject unexpected materialized filenames, use safe path joining, and keep the curated mirror/base-dir fallback order consistent across raw routes (from SOW-0025 follow-up).
- Raw body review must check memory shape as well as authorization: large `.ipset`/`.netset` responses should stream from disk or another bounded reader, not populate the in-memory static artifact cache, and every failure path must write an error status rather than falling through to empty `200 OK` (from SOW-0025 regression).
- Runtime performance review must verify that a suspected hot path has real
  production callers before optimizing it. If the finding is actually dead
  private code, prefer deletion over preserving the helper and adding tests
  around unused behavior (from SOW-0031).
- HTTP middleware review must check panic recovery placement relative to gzip
  and logging. Recovery should happen inside compression so negotiated panic
  responses are correctly encoded, with outer logging still seeing the 500
  status (from SOW-0031).
- Signed-merge review must separate dependency graph from comparison lineage. `DerivedFrom` includes subtractive dependencies for scheduling/recovery, but pairwise relatedness and unique-share filtering must use additive/positive lineage only (from SOW-0025 follow-up).
- Critical-infrastructure review must confirm reference IP/CIDR data is config-owned (`static:`, `url:`, or merge), static critical entries are validated as IPv4 IPs/CIDRs, shipped reference feeds are not raw-redistributable unless explicitly reviewed, provider-set IDs include processing-shape config without volatile timestamps or path/output-cache noise, stale `provider_set_id` is rejected everywhere it is consumed using a cached current ID on public routes, generated artifact names cannot collide with public feed/provider names, exact feed names win over suffix parsing, public routes and cleanup reject artifacts for feeds that are no longer comparable critical-overlap targets, tier-aware insights distinguish hard from soft/contextual overlap, feed-page matched-reference tables surface tier criticality before overlap volume, integrity expects per-provider files only for materialized providers, and `critical_infrastructure` is not combined with `bogons` or provider-database roles (from SOW-0017).
- Critical-infrastructure public methodology must explain meaning, levels, rationale, strengths, weaknesses, missing/deferred coverage, and interpretation; implementation details belong in operator docs/specs, not methodology pages (from SOW-0017 regression).
- Integrity-review must trace the full timestamp path: source evidence,
  processing timestamp, generated-file ledger timestamp, staged-file `Chtimes`,
  publish rename, and admin integrity comparison. If a writer produces a public
  or entity artifact without a deliberate mtime owner, treat it as a correctness
  bug (from SOW-0017 regression).
- Entity-integrity review must compare the committed per-feed sidecar mtime,
  private country/ASN sidecar mtime, and public country/ASN payload mtime for
  unchanged and changed refresh paths. Private/public detail files touched in
  one repair cycle must receive one logical timestamp, not two wall-clock write
  times (from SOW-0017 regression).
- Admin entity-integrity review must verify the report is `in_progress` during
  the main engine run as well as during entity-specific background tasks, since
  feed processing can create legitimate temporary sidecar drift before the
  coalesced entity refresh publishes (from SOW-0017 regression).
- Admin integrity tables must remain bounded operator widgets. Review large
  finding lists for their own vertical scrollbars so hundreds of rows do not
  monopolize the admin page (from SOW-0017 regression).
- Architecture posture is guarded by `tools/archposture` and specified in
  `.agents/sow/specs/architecture-posture.md`. Do not accept new large files,
  large functions, production cache-entry mutations, artifact-token substring
  classification, or `pkg/iprange` project imports without an explicit SOW
  decision (from SOW-0030).
- Scheduler-review must verify the shared queue-lock invariant. Download and
  processing queue maps remain owned under `Runner.stateMu`; action handling,
  due policy, staged recovery, refetch/deferred admission, and processing
  batches may live in separate same-package files but must not introduce a
  second queue authority or hidden background path (from SOW-0030).
- Maintainer-owned quality SOWs must not use "defer" as a quiet dismissal.
  Deferral is valid only when the item has a concrete pending SOW, or when the
  SOW records an evidence-backed rejection/non-goal. This is especially strict
  before release hardening, where untracked valid work becomes product risk.
- Deferred valid work is immediate follow-up work, not backlog. After the
  current SOW closes, the next SOW should be the deferred focused item unless a
  newer user priority explicitly supersedes it; do not bury it behind unrelated
  quality SOWs.

## When to escalate

- Any public API accepting arbitrary user input that can trigger comparison, composition, upload, or parsing work.
- Any scheduler/background change that can increase CPU or memory operational profile.
- Any change touching config semantics, release publication, auth, rate limits, OTel, or admin actions.
- Any migration or deletion of TODO/spec/history content.
- Any medium/high-risk SOW validation gap; follow the project-local `AGENTS.md` rule for independent reviewer count and user-authorized gaps.
