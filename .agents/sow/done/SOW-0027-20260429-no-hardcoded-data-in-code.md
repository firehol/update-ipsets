# SOW-0027 | 2026-04-29 | no-hardcoded-data-in-code

## Status

Status: completed

Sub-state: closed

Immediate SOW-0017 critical-reference hardcoding issue fixed by moving the
curated IP/CIDR bodies into config-backed YAML `static:` sources. The broader
production-code audit is now complete.

## Requirements

Given operators must be able to customize what the engine treats as "data"
(IPs, feed identities, feed categories), when this SOW is complete, then the
production code must contain no hardcoded IP addresses, feed names, or feed
categories, except a clearly documented static set of RFC-reserved/bogon
ranges that are protocol facts, not operator policy.

Given the project already declares "do not hardcode feed names" as a repo rule
in `CLAUDE.md`, when this SOW is complete, then that rule must hold across the
whole production code path, including engine, downloader, processor, web,
admin, and UI.

Given operators run their own deployments, when this SOW is complete, then any
IP, IP range, feed name, or feed category referenced by code must come from
configs (`configs/firehol/`), external files, or runtime arguments, so
operators can change them without patching the binary.

## Analysis

Initial sources to consult (do not pre-scan now; this is the audit list):

- `pkg/engine/`, `pkg/downloader/`, `pkg/processor/`, `pkg/web/`, `pkg/output/`,
  `cmd/`, `internal/`.
- `ui/src/` for hardcoded examples that leak into the public UI as facts rather
  than placeholders.
- `configs/firehol/` to confirm where each datum should live instead.
- `.agents/sow/specs/config.md`, `.agents/sow/specs/feeds.md`,
  `.agents/sow/specs/files-layout.md` for canonical ownership of feed data.
- `.agents/sow/specs/operating-principles.md` for the dependency-discipline
  rule.

Known context (already established, no scan needed to record):

- `pkg/engine/bogons_rfc.go` is RFC-reserved address space — protocol fact, not
  operator policy. This is the one allowed static dataset.
- SOW-0017 briefly introduced critical DNS/root/AS112 IP/CIDR data in Go code.
  user rejected that pattern. The immediate fix moved those curated bodies to
  `configs/firehol/sources/provider_infrastructure/critical_dns.yaml` using
  source-level `static:` config, and added generic static-source processing so
  operators can customize the data without patching the binary.
- A focused production-source grep after the immediate fix found no remaining
  SOW-0017 critical public DNS/root/AS112 literals in non-config, non-doc,
  non-test code. Remaining `1.1.1.1` occurrences are UI/API documentation
  examples, not operator-policy data.
- `CLAUDE.md` already forbids hardcoded feed names; this SOW extends the rule
  to IPs and categories and proves enforcement.

## Implications and decisions

- "Hardcoded" includes: literal IPv4/IPv6 addresses, literal CIDRs, literal
  feed names (string equality / switch on feed identity), literal category
  strings used as branching logic, and inline lists used as defaults that the
  operator cannot override.
- Placeholders in UI text (e.g. an example IP shown in an input box) and
  documentation example URLs are not the same as "hardcoded data" — they are
  human-facing examples. Treatment of these is a design decision for this SOW,
  not an automatic violation.
- Sentinel values used by parsers/filters (e.g. `0.0.0.0`, `/0`) are protocol
  semantics, not policy data. Treatment of these is a design decision for this
  SOW, not an automatic violation.
- Moving data out of code may shift startup behavior (config load order,
  failure modes when a config file is missing) and the public release artifact
  shape — both must be considered before any change.
- For each violation found, the fix may be: move to YAML config, move to a
  data file, expose as a CLI/runtime flag, or document and keep as a protocol
  fact. The SOW must record the decision per case, not blanket-rewrite.

## Plan

Chunked SOW - reasoning: audit, decision-making, and refactor are separate
risks; each violation may need its own fix path.

1. `audit` - high risk. Full scan of `pkg/`, `cmd/`, `internal/`, `ui/src/`
   for: literal IPs/CIDRs, literal feed names, literal feed categories.
   Output: evidence table (file:line, value, current usage, suggested fix).
2. `classify` - medium risk. For each finding, classify as: (a) RFC/protocol
   fact, keep static; (b) operator policy, move to config; (c) example/UX
   text, keep but document; (d) sentinel/parser semantics, keep but document.
   User approval required on the classification list before any refactor.
3. `critical-static-verification` - medium risk. Verify the SOW-0017 critical
   static fix remains config-owned and add a repeatable check if the broader
   audit decides that enforcement belongs in tests/CI.
4. `refactor` - high risk. Move classified-(b) entries into configs/data
   files; preserve current behavior; ensure missing-config failure modes are
   explicit and operator-visible.
5. `enforcement` - medium risk. Add a repeatable check (lint rule, test, or
   CI grep) that fails the build when new hardcoded data is introduced into
   production code paths. Scope of the check is a design decision.
6. `specs-and-docs` - low risk. Update `.agents/sow/specs/config.md`,
   `.agents/sow/specs/feeds.md`, and the project coding skill with the
   "no hardcoded data in code" rule and the agreed exception list.

## Execution log

2026-04-29:

- Immediate SOW-0017 hardcoded critical-IP/CIDR violation fixed before
  continuing implementation.
- Added generic YAML `static:` source support and moved the curated critical DNS,
  root-server, and AS112 bodies into
  `configs/firehol/sources/provider_infrastructure/critical_dns.yaml`.
- Updated `.agents/skills/project-coding/SKILL.md` to forbid hardcoded
  operator-policy IP/CIDR lists in Go/UI production code.
- Updated `.agents/skills/project-testing/SKILL.md` with validation expectations
  for config-backed static sources.

2026-05-01:

- User delegated implementation strategies, cleanups, testing, and audits that
  do not need product direction to the assistant. This SOW fits that class.
- Audited production Go, UI source, and config-backed static reference data for
  hardcoded IP/CIDR bodies and configured source-name coupling.
- Confirmed critical public DNS, DNS root, AS112, and critical service ranges
  remain in YAML `static:` config:
  - `configs/firehol/sources/provider_infrastructure/critical_dns.yaml:6`
  - `configs/firehol/sources/provider_infrastructure/critical_dns.yaml:39`
  - `configs/firehol/sources/provider_infrastructure/critical_dns.yaml:77`
  - `configs/firehol/sources/provider_infrastructure/critical_service_ranges.yaml:385`
- Confirmed remaining production IP/CIDR literals are classified as:
  - parser sentinels: `pkg/downloader/canonical.go:147`,
    `pkg/processor/stream_filters.go:54`, `pkg/processor/primitives.go:287`
  - comments/docs/examples: `pkg/processor/processor.go:327`,
    `pkg/engine/output.go:1323`, `ui/src/components/home/home-ip-lookup.tsx:45`
  - protocol facts: `pkg/engine/bogons_rfc.go:71`
    through `pkg/engine/bogons_rfc.go:85`
- Found and fixed a real configured-source-name smell in `pkg/geoloc`: parser
  internals returned hardcoded source names such as `dbip_country` and
  `geolite2_country`. The parser now records the format/provider type from the
  caller instead of baked-in source identities.
- Added `tools/archposture/hardcoded_data_test.go`, a low-noise regression
  guard that parses production Go string literals and fails if one exactly
  matches a configured source name. Allowed exceptions are explicit:
  `rfc_reserved`, `bogons` as a config role/source-name overlap, and
  `caida_prefix2as` only at parser-format sites.

## Validation

- [x] Broad audit acceptance criteria evidence:
  - IP/CIDR production-source scan completed and classified; no operator-policy
    IP/CIDR body remains outside config except the documented RFC-reserved
    protocol dataset.
  - Configured-source-name production Go enforcement added in
    `tools/archposture/hardcoded_data_test.go`.
- [x] Real-use validation evidence:
  - `go test ./pkg/geoloc` — passed.
  - `go test ./tools/archposture` — passed.
  - `make test` — passed.
  - `make lint` — passed.
  - `git diff --check` — passed.
  - `.agents/sow/audit.sh` — passed after moving this completed SOW to
    `.agents/sow/done/`.
- [x] Cross-model reviewer findings:
  - Not run. Reason: this was a focused local audit and low-risk cleanup; the
    repeatable archposture guard now enforces the main regression class.
- [x] Lessons extracted.
- [x] Same-failure-at-other-scales check:
  - Scan included production Go, `cmd/`, `internal/`, `pkg/`, and `ui/src/`
    excluding tests/generated public assets.
  - The regression guard scans production Go against all configured source
    names loaded from `configs/firehol/`.

Artifact maintenance gate:

- AGENTS.md: not updated. Reason: the runtime rule already exists there.
- Runtime project skills: not updated. Reason: `project-coding` and
  `project-testing` already contain the no-hardcoded-data guidance from
  SOW-0017.
- Specs: not updated. Reason: `.agents/sow/specs/config.md` and
  `.agents/sow/specs/feeds.md` already prohibit hardcoded feed/source identity
  and require config-owned static data.
- End-user/operator docs: not updated. Reason: no operator-facing behavior
  changed.
- End-user/operator skills: not updated. Reason: no portable operator workflow
  changed.
- SOW lifecycle: moved from pending to current to done.

## Outcome

Completed.

Shipped changes:

- Removed hardcoded geolocation catalog source identities from parser output.
- Added a geolocation parser regression assertion that `Dataset.Provider`
  reflects the caller's format/provider type.
- Added an archposture guard that blocks configured source-name literals in
  production Go code unless explicitly allowlisted.

## Lessons extracted

- A configured source name can also appear as a protocol/format/role token.
  Enforcement must be low-noise and exception-based, otherwise it trains
  maintainers to ignore the guard.
- Source-name hardcoding is easiest to catch at the string-literal layer in Go;
  comments and public examples should be audited but not treated as automatic
  violations.
