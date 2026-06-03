# SOW-0016 | 2026-04-26 | review-firehol-merges

## Status

Status: in-progress

Sub-state: re-scoped by user on 2026-06-02; prompt-and-agent evaluation phase
started before any merge composition implementation.

## Requirements

### Purpose

Re-evaluate the `firehol_*` merge family so the popular FireHOL-owned aggregate
feeds remain trustworthy, explainable, conservative where required, and safe for
users who rely on their historical semantics. The work must decide what each
merge is for, which active feeds deserve inclusion, why they do or do not
belong, and how critical infrastructure should be excluded from all merge
outputs without hardcoding policy data.

### User Direction - 2026-06-02

The user clarified the intended SOW-0016 execution model:

1. Create a prompt that explains the scope of the `firehol_*` merges.
2. Spawn four agents, each responsible for one quarter of active feed markdown
   files. Active means all feeds except those classified as `archived`, `empty`,
   or `unavailable`.
3. Each agent must decide whether each assigned feed deserves inclusion in any
   `firehol_*` merge and why.
4. Reports must be structured carefully because FireHOL merges are popular and
   users trust them.
5. `firehol_level1` requires an especially conservative bar.
6. All FireHOL-owned merges should exclude critical infrastructure such as core
   DNS and router infrastructure.

Given the `firehol_*` merge feeds are project-owned aggregates, when this SOW is complete, then their continued existence, dependencies, descriptions, categories, and public presentation must be reviewed and corrected.

Given merge composition should be informed by feed quality, when deciding which feeds to include in merges, then AI-assisted feed methodology collection and grading (from SOW-0014) should inform the decision.

Given merge changes can affect compatibility and user expectations, when any merge is changed, then compatibility, docs, redirects/renames, and migration impact must be explicit.

Given all FireHOL-owned merges should avoid blocking critical infrastructure,
when merge composition is implemented, then each merge must exclude configured
critical-infrastructure reference feeds through configuration/typed metadata,
not hardcoded names or embedded IP/CIDR lists.

## Analysis

### User Context (2026-05-02)

- The project synthesizes a few feeds for general consumption (the `firehol_*` merges).
- Deciding which feeds should be part of these merges requires understanding each source feed's methodology and quality.
- User wants to use AI (SOW-0014) to collect the mechanics/methodology of each feed, grade it based on feed analysis, and then decide merge composition.
- This SOW is effectively blocked until SOW-0014 provides feed quality grading.

Initial sources to consult:

- `configs/firehol/merges/firehol_*.yaml`
- Existing public feed descriptions for `firehol_*`.
- Legacy FireHOL merge semantics.
- Public homepage/feed explorer presentation.
- `.agents/sow/specs/compatibility.md`, `.agents/sow/specs/feeds.md`, and `.agents/sow/specs/config.md`.

## Implications and decisions

- This SOW is no longer blocked on SOW-0014 for the evaluation phase: the
  existing generated active feed markdown under `/opt/update-ipsets/web/` is
  sufficient for evaluator reports.
- Removing merges can break users who rely on historical FireHOL names.
- Dependency updates can change feed contents and trust semantics.
- Adding weak, stale, poorly scoped, or policy-heavy feeds to
  `firehol_level1` can damage user trust because that merge is historically the
  conservative low-false-positive set.
- Excluding critical infrastructure must use configured roles/metadata such as
  `use: [critical_infrastructure]` and published critical artifacts. It must not
  rely on feed-name substrings or hardcoded IP/CIDR policy data.

## Plan

1. `merge-inventory` — enumerate all `firehol_*` merges and their source feeds. Can start now (no dependency on SOW-0014).
2. `evaluation-prompt` — create a reusable evaluator prompt defining merge
   scope, inclusion criteria, evidence rules, and report schema.
3. `active-feed-split` — derive the active feed list from the published index,
   excluding feeds whose `health.class` is `archived`, `empty`, or
   `unavailable`, and split the sorted list into four non-overlapping quarters.
4. `parallel-feed-evaluation` — spawn four agents to review one quarter each
   of active generated feed markdown files under `/opt/update-ipsets/web/`.
5. `synthesis` — merge evaluator reports into candidate inclusion/exclusion
   recommendations per `firehol_*` merge, with explicit uncertainty and
   evidence.
6. `user-decisions` — present concrete merge-composition decisions before
   changing YAML, enrichment, specs, docs, or tests.
7. `implement-approved-actions` — apply user-approved composition changes and
   critical-infrastructure exclusions. High risk.
8. `compatibility-docs-tests` — verify no user breakage. High risk.

## Pre-Implementation Gate

Status: evaluation-ready; implementation blocked until user approves concrete
merge changes after evaluator synthesis.

Problem / root-cause model:

- The current `firehol_*` merges preserve legacy composition in several places,
  but SOW-0016 has not yet revalidated whether each component is still fit for
  the intended merge purpose.
- FireHOL merges are high-trust public outputs; weak inclusion criteria can
  produce false positives at scale.
- Critical infrastructure overlap is unsafe for blocking-oriented outputs and
  should be subtracted from every FireHOL-owned merge through configured
  critical-infrastructure reference data.

Evidence reviewed:

- Current merge definitions live under `configs/firehol/merges/firehol_*.yaml`.
- Generated feed markdown lives under `/opt/update-ipsets/web/{feed}.md`.
- Published feed index lives at `/opt/update-ipsets/web/index.json`.
- The published index currently contains `health.class`; active-feed evaluation
  will exclude `archived`, `empty`, and `unavailable`.
- `docs/feeds/feed-visibility-lifecycle.md` defines the relevant health states
  and notes that archived additive merge inputs are skipped automatically.
- `pkg/engine/markdown.go` writes feed markdown as `{feed}.md` in the published
  web artifact tree.
- Current project rules require semantic feed roles/typed metadata instead of
  feed-name pattern matching or hardcoded policy IP/CIDR lists.

Affected contracts and surfaces:

- Merge YAML definitions under `configs/firehol/merges/`.
- Critical-infrastructure exclusion behavior for merge composition.
- Static FireHOL enrichment generated by `tools/build-firehol-static-enrichment.py`.
- Public feed markdown/API/UI interpretation of FireHOL merges.
- Specs under `.agents/sow/specs/` if composition or critical-exclusion
  semantics change.
- Operator docs if compatibility, migration, or published behavior changes.

Existing patterns to reuse:

- Merge composition is YAML-owned and expanded into sources by config loading.
- Critical infrastructure uses configuration roles/metadata, especially
  `use: [critical_infrastructure]`.
- Public markdown is generated from published artifacts and enrichment, not
  hand-authored per feed.
- Static enrichment already handles FireHOL-maintained merges deterministically.

Risk and blast radius:

- `firehol_level1` has the highest trust and should admit only feeds with a
  clearly low false-positive risk and strong current methodology evidence.
- `firehol_level2` through `firehol_level4` may tolerate broader or noisier
  feeds only when the merge purpose and warning language remain explicit.
- Abuser, proxy, anonymous, webclient, and webserver merges have different
  operational semantics; a feed suitable for one may be unsafe for another.
- Subtracting critical infrastructure from every merge can reduce false
  positives but may surprise users comparing historical raw merge sizes.
- Removing or adding component feeds can break user expectations and should be
  documented and validated.

Sensitive data handling plan:

- Evaluator reports, SOWs, specs, docs, skills, prompts, and comments must not
  include secrets, customer data, private endpoints, or proprietary incidents.
- Reports may cite public feed names, public markdown paths, public URLs already
  present in catalog metadata, health classes, categories, and summarized
  evidence.
- Do not copy raw IP lists into SOW artifacts; use feed names and high-level
  metrics instead.

Implementation plan:

1. Create the evaluator prompt as a SOW working artifact.
2. Spawn four read-only evaluator agents, one per sorted active-feed quarter.
3. Collect and normalize their reports.
4. Produce a synthesis with per-merge recommendations, conflicts, and unknowns.
5. Ask the user to approve concrete composition and exclusion decisions.
6. Only after approval, update YAML/config, static enrichment, specs, docs, and
   tests.

Validation plan:

- Verify the active feed split is complete, non-overlapping, and excludes only
  `health.class` values `archived`, `empty`, and `unavailable`.
- Require each evaluator row to cite the markdown file path and concrete
  evidence from that feed's generated page.
- Cross-check evaluator recommendations against current merge composition and
  legacy FireHOL composition.
- Before any implementation, present user decisions with evidence and risks.
- After implementation, run relevant config/catalog tests and static enrichment
  validation before broader build/test commands.

Artifact impact plan:

- AGENTS.md: no update expected unless this SOW exposes a durable workflow rule.
- Runtime project skills: possible update only if the evaluation pattern should
  become a reusable agent workflow.
- Specs: update feed/compatibility/pipeline/public-website specs if merge
  composition or critical-infrastructure exclusion semantics change.
- End-user/operator docs: update only after approved behavior changes.
- End-user/operator skills: no update expected.
- SOW lifecycle: this SOW is now in `.agents/sow/current/`; SOW-0099 was parked
  back in `.agents/sow/pending/` because the user redirected active work here.

Open decisions:

- No merge composition decision is approved yet.
- No YAML/code/docs/spec implementation should start until evaluator synthesis
  is complete and the user approves concrete options.

## Execution log

### 2026-06-02 read-only merge inventory

The user asked to start SOW-0016 by finding all FireHOL merges. A separate SOW
was already present in `.agents/sow/current/` with `Status: in-progress`, so
this pass recorded a read-only inventory without moving this SOW to `current`.

Current config evidence:

- `configs/firehol/merges/` contains 12 YAML merge definitions.
- Searching all `configs/**/*.yaml` for top-level `merges:` definitions finds
  13 merge definitions total: the 12 files under `configs/firehol/merges/` plus
  one provider-infrastructure merge,
  `critical_soft_akamai_edge_secondary`.
- The exact `firehol_*` merge definitions in current config are 10:
  `firehol_abusers_1d`, `firehol_abusers_30d`, `firehol_anonymous`,
  `firehol_level1`, `firehol_level2`, `firehol_level3`, `firehol_level4`,
  `firehol_proxies`, `firehol_webclient`, and `firehol_webserver`.
- FireHOL-maintained non-`firehol_*` current merge definitions are `cleantalk`,
  `cymru_unassigned`, and `critical_soft_akamai_edge_secondary`.

Current source composition:

| Merge | Sources | Exclude | Redistributable |
|---|---|---|---|
| `firehol_level1` | `dshield`, `feodo`, `fullbogons`, `spamhaus_drop` | none | true |
| `firehol_level2` | `blocklist_de`, `dshield_1d`, `greensnow` | none | false |
| `firehol_level3` | `bruteforceblocker`, `ciarmy`, `dshield_30d`, `myip`, `vxvault` | none | true |
| `firehol_level4` | `blocklist_net_ua`, `botscout_30d`, `cybercrime`, `iblocklist_hijacked`, `iblocklist_spyware`, `iblocklist_webexploit` | none | false |
| `firehol_abusers_1d` | `botscout_1d`, `php_commenters_1d`, `php_dictionary_1d`, `php_harvesters_1d`, `php_spammers_1d`, `stopforumspam_1d` | none | true |
| `firehol_abusers_30d` | `php_commenters_30d`, `php_dictionary_30d`, `php_harvesters_30d`, `php_spammers_30d`, `stopforumspam`, `sblam` | none | true |
| `firehol_anonymous` | `anonymous`, `dm_tor`, `firehol_proxies`, `tor_exits` | none | false |
| `firehol_proxies` | `iblocklist_proxies`, `ip2proxy_px1lite`, `socks_proxy_30d`, `sslproxies_30d` | none | false |
| `firehol_webclient` | `cybercrime` | none | true |
| `firehol_webserver` | `myip`, `stopforumspam_toxic` | none | true |
| `cleantalk` | `cleantalk_new`, `cleantalk_updated` | none | false |
| `cymru_unassigned` | `fullbogons` | `bogons` | true |
| `critical_soft_akamai_edge_secondary` | `misp_akamai` | none | true |

Legacy comparison evidence from `/home/costa/src/firehol/firehol/sbin/update-ipsets`:

- Legacy has 10 `firehol_*` merge statements.
- Legacy has explicit `cleantalk`, `cleantalk_1d`, `cleantalk_7d`, and
  `cleantalk_30d` merge statements. Current config keeps `cleantalk` as the
  base merge and generates retention derivatives through `history`.
- Current and legacy source lists match for `firehol_level1`,
  `firehol_level2`, `firehol_level3`, `firehol_level4`,
  `firehol_webclient`, and `firehol_webserver`.
- Current `firehol_abusers_1d` omits legacy `cleantalk_new_1d` and
  `cleantalk_updated_1d`.
- Current `firehol_abusers_30d` omits legacy `cleantalk_new_30d` and
  `cleantalk_updated_30d`.
- Current `firehol_proxies` omits legacy `proxyrss_30d`,
  `ri_connect_proxies_30d`, and `ri_web_proxies_30d`; the base feeds
  `proxyrss`, `ri_connect_proxies`, and `ri_web_proxies` are listed in
  `configs/firehol/deleted.yaml`.
- Current `firehol_anonymous` omits legacy `bm_tor`; `bm_tor` is listed in
  `configs/firehol/deleted.yaml`.

Immediate implication:

- The SOW text that says FireHOL static enrichment covers "15 entries in
  `configs/firehol/merges/`" is stale for the current tree. Current evidence is
  12 files in `configs/firehol/merges/` and 13 total config-level `merges:`
  definitions across `configs/**/*.yaml`.

### 2026-06-02 SOW re-scope and evaluator launch

The user re-scoped this SOW to re-evaluate the `firehol_*` merge family from
active generated feed markdown. Active means feeds whose published
`health.class` is not `archived`, `empty`, or `unavailable`.

Lifecycle:

- SOW-0099 was parked back in `.agents/sow/pending/` because it was blocked on
  user scanner/enforcement decisions and the user redirected active work here.
- This SOW was moved to `.agents/sow/current/`.

Evaluator prompt:

- `.agents/sow/current/SOW-0016-firehol-merge-evaluation-prompt.md`

Published artifact evidence:

- `/opt/update-ipsets/web/index.json` contained 403 feed entries.
- Active-feed filter produced 349 active feeds.
- Excluded feed counts: `archived=27`, `empty=9`, `unavailable=18`.
- Active non-excluded health counts: `healthy=337`, `delayed=10`,
  `unmaintained=2`.
- Every active feed had a matching `/opt/update-ipsets/web/{feed}.md` file.

Agent split:

| Slice | Agent | Count | First feed | Last feed |
|---|---|---:|---|---|
| 1 | Bohr (`019e86d6-162d-7390-a1e7-8efd38aa9311`) | 87 | `abuseipdb_1d` | `dronebl_irc_drones` |
| 2 | Leibniz (`019e86d6-1676-7582-ab75-a1ed38827582`) | 87 | `dronebl_open_dns_resolvers` | `iblocklist_spamhaus_drop` |
| 3 | Locke (`019e86d6-16ed-7be2-a23e-8267cebf3c90`) | 87 | `iblocklist_spider` | `php_commenters` |
| 4 | Sartre (`019e86d6-178a-79c1-8936-91997a9aeff1`) | 88 | `php_commenters_1d` | `yoyo_adservers` |

All four agents were instructed to run read-only, avoid external AI
assistants, avoid raw IP lists, and report facts separately from judgment.

Current-component health check:

- `firehol_level1` currently includes `feodo`; published `health.class` is
  `archived`.
- `firehol_level3` currently includes `ciarmy`; published `health.class` is
  `empty`.
- `firehol_anonymous` currently includes `anonymous`; no matching entry was
  found in `/opt/update-ipsets/web/index.json`.
- Current critical-infrastructure reference feeds with
  `use: [critical_infrastructure]` are active/healthy and can serve as the
  configured subtractive source family for critical-infrastructure exclusions.

### 2026-06-02 evaluator synthesis

Durable synthesis artifact:

- `.agents/sow/current/SOW-0016-firehol-merge-evaluation-synthesis.md`

Facts recorded by the synthesis:

- The four evaluator agents completed the active-feed markdown review for all
  349 active feeds.
- No evaluator found a new active feed that clearly deserves
  `firehol_level1`.
- `firehol_level1` has an unresolved product decision: either become a strict
  low-false-positive malicious/abusive-host tier, or preserve the historical
  edge-safety semantics that include `fullbogons`.
- Known current-component problems were identified before any implementation:
  `feodo` is archived, `ciarmy` is empty, and `anonymous` has no matching
  published index entry or source definition found by config search.
- The Project Honey Pot abuser components currently used by FireHOL abuser
  merges have published license evidence of `Project Honey Pot Terms of Use -
  All Rights Reserved`; this requires a user/legal decision before continuing
  to redistribute them through merges.
- Critical-infrastructure exclusion should be metadata-driven using
  `use: [critical_infrastructure]`, not hardcoded feed names or IP/CIDR lists.
- The strongest first-wave addition candidates, if additions are approved, are
  `blocklist_de_strongips` and `opendbl_bruteforce` for `firehol_level2`,
  `ipsum_7` and `ipsum_8` for `firehol_level3`, and `threatfox_ips` for
  `firehol_webclient`.

Implementation remains blocked until the user records decisions for the merge
purpose, first implementation scope, Project Honey Pot components,
critical-infrastructure exclusion shape, and whether `firehol_webclient` should
add `threatfox_ips` now.

### 2026-06-02 level-only criteria re-scope

The user corrected the evaluation approach: the next pass must focus only on
`firehol_level1` through `firehol_level4`, and it must classify feeds against
explicit criteria instead of summarizing that no feed fits.

The earlier broad synthesis remains preserved as history, but it is superseded
for implementation decisions until the level-only criterion-based report is
complete.

New durable evaluator prompt:

- `.agents/sow/current/SOW-0016-firehol-levels-criteria-evaluation-prompt.md`

Revised candidate funnel:

- Active feeds in the published index: 349.
- Active feeds in the candidate categories `intrusion`,
  `malware_infrastructure`, `service_abuse`, `messaging_abuse`, and
  `scanners`: 212.
- Active primary new-addition candidates after excluding
  `secondary_retention` and `secondary_merge`: 171.
- Current `firehol_level1..4` components: 18.
- Total review universe after adding current-component exceptions and
  deduplicating: 179.

Reasoning:

- Retention derivatives and merge derivatives are excluded as new-addition
  candidates so agents evaluate the underlying source methodology once.
- Current level components remain exceptions and must be reviewed even if they
  are inactive, retention-derived, outside the candidate categories, or
  otherwise disqualified for new additions. Examples include `spamhaus_drop`
  (`policy_risk`), `fullbogons` (`special_use`), and current DShield retention
  derivatives.
- Each assigned feed must receive criterion columns for category fit, source
  type, threat semantics, methodology quality, freshness, false-positive risk,
  critical-infrastructure overlap, maintenance, license fit, recommended level,
  recommended component, and primary drop reason.

Agent split:

| Slice | Count | First feed | Last feed |
|---|---:|---|---|
| 1 | 44 | `apnic_ssh_bruteforce` | `drb_ra_c2intel` |
| 2 | 45 | `drb_ra_c2intel_30d` | `ipsum_6` |
| 3 | 45 | `ipsum_7` | `palinkas_scanners` |
| 4 | 45 | `php_bad` | `vxvault_url_list` |

### 2026-06-02 level-only criteria synthesis

All four level-only evaluator agents completed read-only reviews with missing
markdown count `0`.

Durable synthesis artifact:

- `.agents/sow/current/SOW-0016-firehol-levels-criteria-synthesis.md`

Headline findings:

- No new feed qualified for `firehol_level1`.
- Current `firehol_level1` keep candidates are `dshield` and `spamhaus_drop`.
- Current `firehol_level1` removal/split candidates are `feodo` and
  `fullbogons`: `feodo` is archived; `fullbogons` is valuable special-use
  border-filter data but outside the revised level-family categories.
- Current `firehol_level2` keep candidates are `blocklist_de` and
  `dshield_1d`; `greensnow` requires removal or legal review because published
  evidence says reproduction/republication is prohibited.
- Current `firehol_level3` keep/review candidates are `dshield_30d`,
  `bruteforceblocker`, `myip`, and `vxvault`; `ciarmy` should be removed
  because it is empty.
- Current `firehol_level4` keep/review candidates are `blocklist_net_ua`,
  `botscout_30d`, and `cybercrime`; `iblocklist_hijacked`,
  `iblocklist_spyware`, and `iblocklist_webexploit` should be removed or
  legally rejected because of weak/discontinued method evidence and restrictive
  iBlocklist terms.
- Dominant drop reasons across agents were `context-or-provider-only`,
  `license-fail`, `duplicate-or-derived`, `methodology-weak`, and
  `fp-risk-too-high`.

Implementation remains blocked until the user approves the level1 semantic
contract, legal gate, level2 Blocklist.de family policy, aggregate/IPsum policy,
and first implementation scope.

## Validation

- [ ] Acceptance criteria evidence
- [ ] Real-use validation evidence
- [ ] Cross-model reviewer findings (logged + addressed)
- [ ] Lessons extracted (or "none, reasoning: ...")
- [ ] Same-failure-at-other-scales check

## Outcome

Pending.

## Lessons extracted

Pending.

## Cross-cutting dependency: FireHOL static enrichment (2026-05-19)

The AI-in-the-loop work (SOW-0014) produced a policy: FireHOL-maintained feeds — the 15 entries in `configs/firehol/merges/` plus `critical_dns` and `rfc_reserved` in sources — are NEVER enriched by the research agent (no-firehol-self-reference rule). Instead, `tools/build-firehol-static-enrichment.py` generates their enrichment deterministically from each YAML, using per-shape templates and hand-written editorial copy for the merge-tier semantics.

**Coupling**: any merge composition change made by this SOW (excluding strong critical infra, adding `_no_private` variations, removing archived-feed components, adjusting tier thresholds, renaming merges) requires regenerating the static enrichment so the public page reflects the new composition. Concretely:

1. **Component lists in the JSON** (`derivation.source_feeds[]`) must match the YAML's actual additive/subtractive composition.
2. **Tier-semantic editorial copy** in the generator (e.g., level1 = "low-FP edge blocking", level2 = "medium-confidence augment") must be revisited if the tier definitions change.
3. **`unlist_request` / `unlisting_policy`** for merges defer to component feeds — if a merge starts maintaining its own whitelisting or exclusion rules (e.g., a curated whitelist for `firehol_level1_no_critical_infra`), the generator's "defer to components" template stops being accurate.

**Long-term architectural direction (preferred)**: instead of carrying tier-semantic editorial copy in the static enrichment generator, derive the merge composition fully from YAML configuration at engine boot/refresh time. The engine already supports automated removal of archived/abandoned feeds from merges; extending that path so the public page renders composition dynamically from the live `additive:` / `subtractive:` / `excludes:` config would eliminate the enrichment-vs-config drift entirely. Under that model, the generator's role shrinks to producing only the immutable parts (license, redistribution, maintainer identity, neutral lifecycle); the dynamic composition and exclusion semantics come straight from the YAML on each render.

Whoever picks up this SOW should decide between (a) regenerate-on-config-change with the existing static-enrichment generator, or (b) move composition/exclusion semantics into engine-rendered dynamic blocks. Option (b) is cleaner; option (a) is faster to ship.

Affected paths:
- `tools/build-firehol-static-enrichment.py` (generator)
- `agents/run-enrichment.sh` (refuses FireHOL-maintained feeds via `maintainer: FireHOL` check)
- `.local/agents/feed-enrichment/<merge>/*/output.json` (generated outputs)
