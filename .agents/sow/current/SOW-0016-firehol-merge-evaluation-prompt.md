# SOW-0016 FireHOL Merge Evaluation Prompt

## Task

Review your assigned quarter of active published feed markdown files and decide
whether each feed deserves inclusion in any exact `firehol_*` merge, and why.

This is a read-only evaluation task. Do not modify files. Do not change YAML,
code, docs, specs, generated artifacts, or SOW files.

## Context

The `firehol_*` merges are FireHOL-owned aggregate feeds. They are popular and
trusted by users, so inclusion must be conservative, evidence-backed, and
explicit about false-positive risk.

Current exact `firehol_*` merge family:

- `firehol_level1` — conservative low-false-positive edge blocking. Highest
  inclusion bar. Feed should be clearly maintained, precise, operationally
  safe, and suitable for internet-facing perimeter blocking where false
  positives are expensive.
- `firehol_level2` — medium-confidence recent-abuser / recent-attack blocking.
  Broader than level1, but still needs defensible current methodology and
  tolerable false-positive risk.
- `firehol_level3` — broader threat-intelligence and detection/hunting
  aggregate. Can include noisier or longer-window sources when analyst review is
  expected.
- `firehol_level4` — broadest high-coverage / high-false-positive-risk
  aggregate. Useful for research, retrospective analysis, and maximum coverage;
  not for blind inline blocking.
- `firehol_abusers_1d` — recent service-abuse sources in roughly a 24-hour
  window.
- `firehol_abusers_30d` — service-abuse sources in roughly a 30-day window.
- `firehol_proxies` — open proxy / proxy infrastructure sources.
- `firehol_anonymous` — anonymizing infrastructure. Existing semantics may
  include `firehol_proxies` as a component, but do not recommend arbitrary
  self-referential FireHOL merge nesting.
- `firehol_webclient` — destinations a web client should not contact, such as
  malware infrastructure.
- `firehol_webserver` — sources a web server should not accept traffic from,
  such as abusive users, malware hosts, or long-history service abusers.

All FireHOL-owned merges should exclude configured critical infrastructure
reference data, such as core DNS and router infrastructure. Treat this as a
global subtractive requirement implemented through configured roles/metadata,
not as a reason to manually hardcode IP ranges or feed-name patterns.

## Active Feed Scope

Use the published feed index:

- Index: `/opt/update-ipsets/web/index.json`
- Markdown pages: `/opt/update-ipsets/web/{feed}.md`
- Active means every feed except `health.class` in `archived`, `empty`, or
  `unavailable`.

At the time this prompt was created, the published index had:

- 403 total feeds
- 349 active feeds
- 54 excluded by the active filter
- health class counts: `healthy=337`, `delayed=10`, `unmaintained=2`,
  `archived=27`, `empty=9`, `unavailable=18`

Use this command to produce your assigned sorted quarter. Replace `SLICE=1`
with your assigned slice number from 1 to 4:

```bash
SLICE=1 ruby -rjson -e '
idx = JSON.parse(File.read("/opt/update-ipsets/web/index.json"))
excluded = %w[archived empty unavailable]
active = idx.select { |x| !excluded.include?(x.dig("health", "class").to_s) }
            .map { |x| x["name"] }
            .compact
            .sort
i = Integer(ENV.fetch("SLICE"))
start = (active.size * (i - 1)) / 4
stop = (active.size * i) / 4
active[start...stop].each { |name| puts name }
'
```

Expected slice sizes:

- Slice 1: 87 feeds, `abuseipdb_1d` through `dronebl_irc_drones`
- Slice 2: 87 feeds, `dronebl_open_dns_resolvers` through `iblocklist_spamhaus_drop`
- Slice 3: 87 feeds, `iblocklist_spider` through `php_commenters`
- Slice 4: 88 feeds, `php_commenters_1d` through `yoyo_adservers`

## Evidence Rules

- Base findings on the generated markdown page and, when useful, the matching
  object in `/opt/update-ipsets/web/index.json`.
- Cite concrete evidence: feed name, markdown path, health class, category,
  maintainer, update/frequency signals, stated methodology, intended use,
  licensing/redistribution risk, and false-positive warnings.
- Separate fact from judgment. Use `Facts:` and `Judgment:` when the difference
  matters.
- Do not infer quality from feed names alone.
- Do not recommend inclusion when evidence is thin. Use `needs-human-review`
  instead.
- Do not copy raw IP lists into the report.
- Do not include secrets, private endpoints, customer data, or non-public
  incidents.

## Recommendation Labels

Use exactly one primary recommendation per feed:

- `strong-include`: clear fit for at least one merge; evidence is strong.
- `possible-include`: plausible fit, but needs maintainer review or more
  validation before changing a trusted merge.
- `keep-if-current`: already belongs in a current merge and evidence supports
  keeping it, but do not newly expand use.
- `remove-if-current`: already belongs in a current merge and evidence suggests
  removal or downgrade.
- `do-not-include`: not fit for any `firehol_*` merge.
- `needs-human-review`: evidence is insufficient, contradictory, or too risky
  for an AI-only recommendation.

For each recommended target merge, also assign:

- `false_positive_risk`: `low`, `medium`, `high`, or `unknown`
- `operational_use`: `blocking`, `detection`, `research`, `context`, or
  `unknown`
- `confidence`: `high`, `medium`, or `low`

## Report Structure

Return a markdown report with these sections:

1. `# Slice N FireHOL Merge Evaluation`
2. `## Scope`
   - Slice number
   - Command used
   - Feed count
   - First and last feed
   - Any missing markdown files
3. `## Executive Findings`
   - 5-15 bullets covering strongest include candidates, strongest exclusions,
     risky current components, and uncertainty patterns.
4. `## Per-Merge Candidate Summary`
   - One subsection per exact `firehol_*` merge.
   - For each subsection, list `strong-include`, `possible-include`,
     `keep-if-current`, `remove-if-current`, and `needs-human-review` feed names
     from your slice.
5. `## Feed Decisions`
   - A table with one row per assigned feed.
   - Required columns:
     - `Feed`
     - `Health`
     - `Category`
     - `Maintainer`
     - `Current merge membership`
     - `Recommendation`
     - `Target merge(s)`
     - `False-positive risk`
     - `Operational use`
     - `Confidence`
     - `Evidence`
     - `Risks / caveats`
6. `## Critical Infrastructure Notes`
   - Note feeds that appear to represent infrastructure/context rather than
     malicious hosts.
   - Note any feed whose inclusion would require especially careful critical
     infrastructure subtraction.
7. `## Questions For Synthesis`
   - Concrete unresolved questions that the final synthesis must decide.

## Review Standards

For `firehol_level1`, recommend `strong-include` only when all are true:

- Low false-positive risk is supported by evidence.
- Feed scope is malicious/abusive hosts, not broad policy, geography, ISP,
  P2P, ad/tracker, scanner-only context, provider ranges, or infrastructure
  reference data.
- Feed has clear current methodology and active/healthy enough state.
- Feed is suitable for perimeter blocking without analyst review.
- Redistributability/licensing does not create obvious conflict with level1's
  public/trusted role.

For broader merges, be explicit about what gets worse:

- false-positive exposure
- stale listings
- policy-vs-threat ambiguity
- aggressive subnet escalation
- crowdsourced-report noise
- commercial/license restrictions
- missing unlisting process

The right answer may be "do not include" for most feeds.
