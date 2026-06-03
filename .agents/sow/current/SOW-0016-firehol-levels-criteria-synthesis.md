# SOW-0016 FireHOL Level Criteria Synthesis

## Surface

- Surface: SOW working artifact.
- Audience: future agents and maintainers.
- Job: summarize the four level-only evaluator reports for
  `firehol_level1` through `firehol_level4`.
- Success criteria: the decision trail explains candidate counts, drop reasons,
  current-component risks, and level-specific candidates before any
  implementation.
- Forbidden content: raw IP lists, secrets, customer data, public tutorial copy,
  or unapproved implementation decisions.

## Status

This artifact supersedes the earlier broad `firehol_*` synthesis for
`firehol_level1` through `firehol_level4` decisions.

No YAML, code, docs, specs, generated artifacts, or public copy were changed as
part of this evaluation.

## Evaluation Scope

Candidate universe:

- active primary feeds only for new additions;
- included categories: `intrusion`, `malware_infrastructure`,
  `service_abuse`, `messaging_abuse`, `scanners`;
- excluded new-addition candidates: `health.class` of `archived`, `empty`,
  `unavailable`, plus `provenance: secondary_retention` and
  `provenance: secondary_merge`;
- current `firehol_level1..4` components were reviewed as exceptions even when
  outside the new-addition filter.

Counts:

| Metric | Count |
|---|---:|
| Active feeds in published index | 349 |
| Active feeds in candidate categories | 212 |
| Active primary new-addition candidates after excluding retention and merges | 171 |
| Current `firehol_level1..4` components | 18 |
| Total review universe after current-component exceptions and dedupe | 179 |
| Missing markdown files | 0 |

Agent slices:

| Slice | Count | First feed | Last feed | Agent |
|---|---:|---|---|---|
| 1 | 44 | `apnic_ssh_bruteforce` | `drb_ra_c2intel` | Bernoulli |
| 2 | 45 | `drb_ra_c2intel_30d` | `ipsum_6` | Poincare |
| 3 | 45 | `ipsum_7` | `palinkas_scanners` | Mendel |
| 4 | 45 | `php_bad` | `vxvault_url_list` | Franklin |

## Aggregate Funnel

Raw agent accounting adds up as follows. Some current components are also
new-candidate-category feeds, so "current exception" and "new candidate" are
not mutually exclusive in every slice.

| Metric | Total |
|---|---:|
| Assigned rows | 179 |
| Current-component rows or exceptions reported by agents | 18 |
| Recommended `level1` rows | 2 |
| Recommended `level2` rows | 16 |
| Recommended `level3` rows | 18 |
| Recommended `level4` rows | 22 |
| Dropped / not recommended rows | 118 |
| Human/legal-review rows | 15 |

Drop reasons reported by agents:

| Drop reason | Count |
|---|---:|
| `context-or-provider-only` | 34 |
| `license-fail` | 30 |
| `duplicate-or-derived` | 18 |
| `methodology-weak` | 11 |
| `fp-risk-too-high` | 7 |
| `maintenance-weak` | 5 |
| `aggregate-too-opaque` | 4 |
| `scanner-too-noisy` | 4 |
| `inactive` | 2 |
| `needs-human-review` | 6 |
| `critical-infra-risk` | 1 |
| `freshness-wrong` | 1 |
| `not-level-family` | 1 |
| `policy-only` | 1 |
| `special-use-only` | 1 |

Interpretation:

- The pre-filter was effective: the dominant drop class was
  `context-or-provider-only`, mostly MISP scanner/provider warninglists that
  passed the broad category filter but are explicitly context/known-good
  references.
- Legal and redistribution issues are a first-order blocker, not an edge case:
  DataPlane, iBlocklist, Project Honey Pot, GreenSnow, Threatview, and some
  non-commercial/share-alike sources repeatedly failed or require legal review.
- Retention/window derivatives should be chosen intentionally from a qualifying
  parent; agents consistently warned against stacking adjacent variants.

## Current Component Matrix

| Feed | Current level | Evaluator result | Primary reason |
|---|---|---|---|
| `dshield` | level1 | keep | strong current intrusion source with FP filtering and removal path |
| `feodo` | level1 | remove | archived |
| `fullbogons` | level1 | remove or split | `special_use`; valuable border-filter data but outside level-family categories |
| `spamhaus_drop` | level1 | keep/review | strong historical edge-safety exception; category is `policy_risk` |
| `blocklist_de` | level2 | keep | active 48h attack window, expiry, delist, allowlist controls |
| `dshield_1d` | level2 | keep | valid retention derivative of qualifying `dshield` parent |
| `greensnow` | level2 | remove unless legal basis exists | semantics fit level2, but published license evidence says reproduction/republication prohibited |
| `bruteforceblocker` | level3 | review/conditional keep | SSH brute-force signal, but weak maintenance evidence |
| `ciarmy` | level3 | remove | empty |
| `dshield_30d` | level3 | keep | valid 30d retention derivative of qualifying `dshield` parent |
| `myip` | level3 | review | service-abuse fit, but license/FP/cloud overlap concerns |
| `vxvault` | level3 | keep/review | malware distribution fit, but high proportional critical overlap |
| `blocklist_net_ua` | level4 | keep | broad service-abuse source fits high-FP tier |
| `botscout_30d` | level4 | review | retention component with personal/non-commercial terms and critical overlap |
| `cybercrime` | level4 | keep | malware/C2 relevance, CC0, but no formal expiry and material critical overlap |
| `iblocklist_hijacked` | level4 | remove | policy-risk category, weak/discontinued method, restrictive iBlocklist terms |
| `iblocklist_spyware` | level4 | remove | weak/old Bluetack-derived method and restrictive iBlocklist terms |
| `iblocklist_webexploit` | level4 | remove | discontinued/no clear unlisting and restrictive iBlocklist terms |

## Level 1 Findings

Evaluator consensus:

- No new feed qualified for `firehol_level1`.
- Current keeps: `dshield`, `spamhaus_drop`.
- Current removals or splits: `feodo`, `fullbogons`.

Criterion detail:

- `dshield` passed because it is current, intrusion-focused, narrow, has
  false-positive filtering, and has a removal path.
- `spamhaus_drop` passed only as a historical edge-safety exception:
  investigator-driven rogue netblocks, strong operational purpose, but category
  is `policy_risk`.
- `feodo` failed because it is archived.
- `fullbogons` failed the revised level-family scope because it is
  `special_use`; it is excellent routing/bogon reference data, not intrusion,
  malware, service abuse, messaging abuse, or scanner evidence.

Open product decision:

- Whether `firehol_level1` remains "edge-safety plus malicious infrastructure"
  or becomes strictly "observed hostile/malicious internet sources".

## Level 2 Candidates

Current keeps:

- `blocklist_de`
- `dshield_1d`

Remove/legal-review current:

- `greensnow`: semantics fit level2, but published license evidence says
  reproduction/republication prohibited.

New/addition candidates reported:

| Candidate | Notes |
|---|---|
| `apnic_ssh_bruteforce` | SSH honeypot daily snapshot; material critical overlap; delayed health |
| `apnic_telnet_bruteforce` | Telnet honeypot daily snapshot; material critical overlap; delayed health |
| `blocklist_de_apache` | service-specific 48h Blocklist.de feed |
| `blocklist_de_bruteforce` | service-specific 48h Blocklist.de feed |
| `blocklist_de_ftp` | service-specific 48h Blocklist.de feed |
| `blocklist_de_imap` | service-specific 48h Blocklist.de feed |
| `blocklist_de_mail` | service-specific 48h Blocklist.de feed |
| `blocklist_de_sip` | service-specific 48h Blocklist.de feed |
| `blocklist_de_ssh` | service-specific 48h Blocklist.de feed |
| `blocklist_de_strongips` | persistent high-volume Blocklist.de attackers |
| `hfish_honeypot` | 24h honeypot attacker feed; high critical overlap |
| `opendbl_bruteforce` | SSH brute-force; 2h refresh; removal/expiry; 0 critical overlap |
| `rutgers_drop` | 300-observed-attack threshold; 5m rebuild; material critical overlap |
| `sekuripy_ipnoise` | SSH brute-force auth-log sensor; small feed; 0 critical overlap |

Composition warning:

- Do not add all `blocklist_de_*` slices while keeping `blocklist_de`. Pick
  the aggregate or a deliberate subset; otherwise the same source family is
  overrepresented.

## Level 3 Candidates

Current keeps/reviews:

- `dshield_30d`: keep.
- `bruteforceblocker`: review/conditional keep.
- `myip`: review/conditional keep.
- `vxvault`: keep/review with critical subtraction.
- `ciarmy`: remove.

New/addition candidates reported:

| Candidate | Notes |
|---|---|
| `blocklist_de_bots` | too high critical overlap for level2; possible level3 if Blocklist.de family policy allows |
| `criticalpath_cobaltstrike` | C2 IPs; 0 critical overlap; no formal unlisting policy |
| `data_shield` | 15d HIDS/SIEM probe telemetry; choose one Data Shield variant |
| `drb_ra_c2intel` | 7d C2 fingerprints; CC BY-NC-SA review required |
| `drb_ra_c2intel_30d` | 30d C2 fingerprints; CC BY-NC-SA review required |
| `ipsum_6` | high-consensus IPsum threshold; aggregate, not level1 |
| `ipsum_7` | stricter IPsum threshold; choose intentionally versus `ipsum_6`/`ipsum_8` |
| `ipsum_8` | strictest reported IPsum threshold; narrowest coverage |
| `malwarefilter_botnet` | botnet C2; MIT; material critical overlap |
| `shadowwhisperer_bruteforce_extreme` | severe Cowrie brute-force threshold |
| `shadowwhisperer_bruteforce_high` | persistent brute-force threshold |
| `shadowwhisperer_hackers` | exploit/botnet/credential-stuffing honeypot behavior |
| `shadowwhisperer_hosting` | malware second-stage hosting |
| `stratosphere_aip_prioritize` | behavioral scoring and decay; high critical overlap |

Composition warnings:

- Choose one IPsum threshold; do not stack `ipsum_6`, `ipsum_7`, and
  `ipsum_8`.
- Choose one Data Shield variant; do not include both `data_shield` and
  `data_shield_critical`.
- C2/malware feeds with non-commercial/share-alike terms need explicit legal
  acceptance before any merge change.

## Level 4 Candidates

Current keeps/reviews/removals:

- Keep: `blocklist_net_ua`, `cybercrime`.
- Review: `botscout_30d`.
- Remove: `iblocklist_hijacked`, `iblocklist_spyware`,
  `iblocklist_webexploit`.

New/addition candidates reported:

| Candidate | Notes |
|---|---|
| `criticalpath_log4j` | historical/shared-hosting FP risk; level4 only |
| `drb_ra_c2intel_90d` | 90d C2 window; CC BY-NC-SA review required |
| `dronebl_autorooting_worms` | severe worm/SSH brute-force; manual/no fixed expiry |
| `dronebl_compromised` | compromised router/gateway signal; no automatic expiry |
| `dronebl_ddos_drones` | DDoS drone signal; manual expiry |
| `dronebl_dictionary_attacks` | credential attack signal; no 24-48h expiry |
| `dronebl_irc_drones` | IRC drone/messaging-abuse malware signal |
| `dronebl_worms_bots` | broad malware/worm/bot signal |
| `et_compromised` | private-source compromised-host signal; opaque method |
| `gpf_comics` | first-party web attack list; license/use review |
| `hagezi_tif` | broad malware/phishing/scam aggregate |
| `ipsum_3` | broad IPsum threshold |
| `netmountains_curated` | broad mixed aggregate with FP filter and unlisting |
| `romainmarcoux_malicious` | bounded top-40K aggregate |
| `romainmarcoux_outgoing_aa` | top-confidence outbound malware aggregate |
| `shadowwhisperer_bruteforce_medium` | lower brute-force threshold |
| `shadowwhisperer_probes` | reconnaissance/probe signal |
| `threatfox_ips` | ThreatFox malware IOC IPs, hourly, 6-month expiry |

Composition warnings:

- Level4 can tolerate high FP risk, but not pure context/provider data and not
  legally unusable data.
- Aggregate-of-aggregates are a policy decision. They improve coverage but
  weaken source accountability and per-entry explanation.

## Categorical Exclusions Supported By The Reports

Exclude from `firehol_level1..4` additive components:

- DataPlane feeds: semantically relevant in places, but direct terms prohibit
  redistribution and generated pages say the signals are not blocklists.
- iBlocklist feeds: repeated personal/non-commercial terms, old Bluetack
  lineage, discontinued sources, weak methods, or policy/provider scope.
- Project Honey Pot feeds: good methodology in several cases, but generated
  pages mark them non-redistributable / All Rights Reserved.
- MISP scanner warninglists: mostly known-good scanner infrastructure or
  false-positive suppression context, not malicious-host evidence.
- MISP provider and critical infrastructure warninglists: context or
  subtraction only.
- broad provider/context/ad/policy feeds, even when technically active.

## Required Decisions Before Implementation

### Decision 1 - `firehol_level1` semantic contract

Option A - strict level-family scope:

- Keep `dshield` and `spamhaus_drop`.
- Remove `feodo`.
- Split/remove `fullbogons` from `firehol_level1`.
- Treat `spamhaus_drop` as a documented historical exception.
- Pros: aligns level1 with intrusion/malware/abuse criteria.
- Risk: changes long-standing historical inclusion of `fullbogons`.

Option B - historical edge-safety scope:

- Keep `dshield`, `spamhaus_drop`, and `fullbogons`.
- Remove `feodo`.
- Pros: preserves historical border-filter semantics.
- Risk: level1 remains a mixed threat/special-use merge.

Recommendation: Option A if the new category criteria are authoritative.

### Decision 2 - legal gate

Option A - exclude `license-fail` and legal-review feeds for now.

- Pros: cleanest public redistribution posture.
- Risk: removes/blocks some historically used or useful feeds.

Option B - allow non-redistributable/non-commercial feeds only in merges marked
non-redistributable with explicit public warning.

- Pros: preserves more coverage.
- Risk: requires legal confidence; public users may still misuse outputs.

Recommendation: Option A until legal policy is explicit.

### Decision 3 - level2 Blocklist.de family

Option A - keep aggregate `blocklist_de`.

- Pros: stable current behavior, simple.
- Risk: less explainable service composition.

Option B - replace aggregate `blocklist_de` with selected service-specific
feeds and maybe `blocklist_de_strongips`.

- Pros: clearer composition.
- Risk: compatibility change and potential accidental coverage loss.

Recommendation: Option A for the first pass; evaluate replacement separately.

### Decision 4 - IPsum threshold policy

Option A - choose one threshold for level3, such as `ipsum_7`.

- Pros: avoids stacking the same source family.
- Risk: threshold choice is partly subjective.

Option B - do not add IPsum in this pass.

- Pros: avoids aggregate-of-aggregates until policy is decided.
- Risk: misses high-consensus low-overlap candidates.

Recommendation: Option B for first pruning pass; revisit after aggregate policy.

### Decision 5 - first implementation pass

Option A - prune/repair current components and add critical-infrastructure
subtraction only.

- Pros: reduces known problems without broadening list semantics.
- Risk: no new coverage.

Option B - prune plus add selected low-conflict candidates.

- Pros: improves coverage.
- Risk: harder to review and explain in one change.

Recommendation: Option A.

## Implementation Guardrails

- No public request path should generate merge artifacts on demand.
- Critical-infrastructure exclusion must be metadata-driven from configured
  roles such as `use: [critical_infrastructure]`, not hardcoded feed names or
  IP/CIDR lists.
- Static FireHOL enrichment must be regenerated or replaced by dynamic
  composition rendering after merge YAML changes.
- Specs and public documentation must be updated only after approved behavior
  changes.
