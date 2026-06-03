# SOW-0016 FireHOL Merge Evaluation Synthesis

## Supersession Note

This broad `firehol_*` synthesis is preserved as historical evidence only. On
2026-06-02 the user corrected the evaluation direction: implementation decisions
must focus first on `firehol_level1` through `firehol_level4` and must use a
criterion-by-criterion classification with explicit candidate/drop accounting.

Use
`.agents/sow/current/SOW-0016-firehol-levels-criteria-evaluation-prompt.md` and
the resulting level-only synthesis as the implementation basis.

## Surface

- Surface: SOW working artifact.
- Audience: future agents and maintainers.
- Job: preserve the evaluator scope, evidence, merge-specific findings, and
  decision points before any implementation.
- Success criteria: a future maintainer can understand why a feed is a keep,
  remove, possible addition, or human-review item without reading the full
  session transcript.
- Forbidden content: public tutorial copy, raw IP lists, secrets, customer data,
  private endpoints, or implementation claims that have not been validated.

## Input Evidence

Verified project facts:

- Current `firehol_*` merge source lists are defined in
  `configs/firehol/merges/firehol_*.yaml`.
- The evaluator prompt is
  `.agents/sow/current/SOW-0016-firehol-merge-evaluation-prompt.md`.
- The published feed index used for the active split was
  `/opt/update-ipsets/web/index.json`.
- The generated feed markdown reviewed by the agents was
  `/opt/update-ipsets/web/{feed}.md`.
- The published index is a top-level JSON array, not an object with a `feeds`
  property.
- `use: [critical_infrastructure]` is defined in catalog YAML, not surfaced as a
  top-level field in the published index.
- Active critical-infrastructure sources are currently published as healthy
  `critical_*` provider-infrastructure feeds.

Evaluator judgment:

- Four read-only agents reviewed the generated markdown for one sorted quarter
  each of the 349 active feeds.
- The agents were asked to recommend inclusion only in exact `firehol_*` merges
  and to separate facts from judgment.
- The tables below synthesize their findings; they are not approved
  implementation decisions.

## Active Feed Scope

- Published feed entries: 403.
- Active feed entries: 349.
- Excluded health classes: `archived`, `empty`, `unavailable`.
- Excluded counts: `archived=27`, `empty=9`, `unavailable=18`.
- Active non-excluded counts: `healthy=337`, `delayed=10`,
  `unmaintained=2`.
- Every active feed had a matching markdown file.

Agent split:

| Slice | Agent | Count | First feed | Last feed |
|---|---|---:|---|---|
| 1 | Bohr | 87 | `abuseipdb_1d` | `dronebl_irc_drones` |
| 2 | Leibniz | 87 | `dronebl_open_dns_resolvers` | `iblocklist_spamhaus_drop` |
| 3 | Locke | 87 | `iblocklist_spider` | `php_commenters` |
| 4 | Sartre | 88 | `php_commenters_1d` | `yoyo_adservers` |

## Current Merge Evidence

Current source-list evidence:

- `firehol_level1`: `configs/firehol/merges/firehol_level1.yaml:14`
  includes `dshield`, `feodo`, `fullbogons`, `spamhaus_drop`.
- `firehol_level2`: `configs/firehol/merges/firehol_level2.yaml:13`
  includes `blocklist_de`, `dshield_1d`, `greensnow`.
- `firehol_level3`: `configs/firehol/merges/firehol_level3.yaml:13`
  includes `bruteforceblocker`, `ciarmy`, `dshield_30d`, `myip`,
  `vxvault`.
- `firehol_level4`: `configs/firehol/merges/firehol_level4.yaml:13`
  includes `blocklist_net_ua`, `botscout_30d`, `cybercrime`,
  `iblocklist_hijacked`, `iblocklist_spyware`, `iblocklist_webexploit`.
- `firehol_abusers_1d`:
  `configs/firehol/merges/firehol_abusers_1d.yaml:12` includes
  `botscout_1d`, `php_commenters_1d`, `php_dictionary_1d`,
  `php_harvesters_1d`, `php_spammers_1d`, `stopforumspam_1d`.
- `firehol_abusers_30d`:
  `configs/firehol/merges/firehol_abusers_30d.yaml:12` includes
  `php_commenters_30d`, `php_dictionary_30d`, `php_harvesters_30d`,
  `php_spammers_30d`, `stopforumspam`, `sblam`.
- `firehol_anonymous`:
  `configs/firehol/merges/firehol_anonymous.yaml:12` includes `anonymous`,
  `dm_tor`, `firehol_proxies`, `tor_exits`.
- `firehol_proxies`: `configs/firehol/merges/firehol_proxies.yaml:13`
  includes `iblocklist_proxies`, `ip2proxy_px1lite`, `socks_proxy_30d`,
  `sslproxies_30d`.
- `firehol_webclient`: `configs/firehol/merges/firehol_webclient.yaml:13`
  includes `cybercrime`.
- `firehol_webserver`: `configs/firehol/merges/firehol_webserver.yaml:16`
  includes `myip`, `stopforumspam_toxic`.

Published index status evidence for notable current components:

| Feed | Published health | Category | License evidence |
|---|---|---|---|
| `dshield` | healthy | intrusion | CC BY-NC-SA 2.5 |
| `feodo` | archived | malware_infrastructure | CC0 1.0 |
| `fullbogons` | healthy | special_use | Team Cymru free/no stated restrictions |
| `spamhaus_drop` | healthy | policy_risk | Spamhaus DROP Fair Use Policy |
| `ciarmy` | empty | intrusion | public feed |
| `iblocklist_hijacked` | healthy | policy_risk | iBlocklist personal/non-commercial terms |
| `iblocklist_webexploit` | healthy | intrusion | iBlocklist personal/non-commercial terms |
| `php_*_1d` / `php_*_30d` | healthy | messaging/service abuse | Project Honey Pot Terms of Use - All Rights Reserved |
| `sblam` | healthy | messaging_abuse | public feed |

No published index entry was found for the current `firehol_anonymous` source
`anonymous`. A config search found it only as a component name inside
`firehol_anonymous`, not as a source definition.

## Cross-Agent Conclusions

### Level 1

Consensus:

- No agent found a new active feed that clearly deserves `firehol_level1`.
- `firehol_level1` should remain the most conservative merge.
- `dshield` and `spamhaus_drop` are defensible current keeps.
- `feodo` should not remain as-is because its published health is `archived`.
- `fullbogons` is valuable network-operations reference data, but it is not a
  malicious-host feed. Its generated page describes it as registry-policy and
  border-filtering data, not threat detection.

Decision implication:

- The project must choose whether `firehol_level1` means strictly
  low-false-positive malicious or abusive hosts, or whether it preserves the
  historical "edge safety" meaning that includes bogon/border-filter data.

Recommendation:

- Keep `firehol_level1` extremely conservative.
- Remove `feodo` unless a current successor is identified.
- Split `fullbogons` out of `firehol_level1` or document a deliberate
  compatibility exception if it stays.
- Do not add any new level1 feed in this SOW.

### Level 2

Current keeps:

- `blocklist_de`, `dshield_1d`, and `greensnow` remain plausible level2
  components, with the normal level2 false-positive warning.

Strong additions to consider:

- `blocklist_de_strongips`: healthy, intrusion category, public feed; strongest
  slice-1 level2 candidate.
- `opendbl_bruteforce`: healthy, intrusion category, public feed; strongest
  slice-3 level2 candidate.

Human-review additions:

- `hfish_honeypot`, `abuseipdb_1d`, `abuseipdb_3d`, `abuseipdb_7d`,
  `apnic_*_bruteforce`, `sekuripy_ipnoise_1d`,
  `shadowwhisperer_bruteforce_high`, `shadowwhisperer_bruteforce_extreme`,
  `rutgers_drop_1d`, and selected DroneBL abuse classes.

Recommendation:

- First implementation should either avoid level2 additions or add only the
  strongest two candidates after critical-infrastructure subtraction is in
  place.

### Level 3

Current keeps:

- `bruteforceblocker`, `dshield_30d`, `myip`, and `vxvault` remain plausible
  for broader detection and hunting.

Current removal:

- `ciarmy` should not remain as-is because its published health is `empty`.

Strong additions to consider:

- `ipsum_7` and `ipsum_8`: healthy, intrusion category, Unlicense;
  high-threshold IPsum variants are the strongest slice-3 level3 candidates.

Human-review additions:

- `abuseipdb_30d`, `criticalpath_cobaltstrike`, `criticalpath_sip`,
  `data_shield`, `drb_ra_c2intel_30d`, DroneBL botnet/compromised/DDOS/worm
  classes, `et_compromised`, `malwarefilter_botnet`,
  `romainmarcoux_malicious*`, `rutgers_drop*`, `sefinek_malicious`,
  `sekuripy_ipnoise*`, `shadowwhisperer_*`, and `threatview_ip`.

Recommendation:

- Remove the empty current component first.
- Treat new level3 additions as a separate decision because breadth can grow
  quickly and many candidates are aggregators or have provider overlap.

### Level 4

Current keeps:

- `blocklist_net_ua`, `botscout_30d`, `cybercrime`, and possibly
  `iblocklist_spyware` remain plausible for the broadest/highest-FP merge.

Current removals or review:

- `iblocklist_hijacked` is category `policy_risk` and was recommended for
  removal by its evaluator.
- `iblocklist_webexploit` was recommended for removal by its evaluator because
  it is stale/discontinued-style iBlocklist content with restrictive terms and
  no strong current methodology signal.

Human-review additions:

- `ipsum`, `ipsum_2`, `ipsum_3`, `netmountains_curated`,
  `romainmarcoux_malicious*`, `hagezi_tif`, `gazpitchy_blacklist`,
  `serpro_reputation`, `ustc_blackip`, and `turris_greylist*`.

Recommendation:

- Prune weak current components before expanding level4.
- Do not add aggregate-of-aggregates until the project decides its policy for
  duplicated source evidence and public warnings.

### Abusers 1d

Current keeps:

- `botscout_1d` and `stopforumspam_1d` remain plausible recent-abuser
  components.

Current removals or legal review:

- `php_commenters_1d`, `php_dictionary_1d`, `php_harvesters_1d`, and
  `php_spammers_1d` are Project Honey Pot feeds. Published index license
  evidence says `Project Honey Pot Terms of Use - All Rights Reserved`.

Human-review additions:

- `abuseipdb_1d`, `hfish_honeypot`, and `php_bad_1d`.

Recommendation:

- Do not add more abuser feeds until the Project Honey Pot legal/redistribution
  decision is made.

### Abusers 30d

Current keeps:

- `stopforumspam` remains plausible.

Current removals or legal review:

- `php_commenters_30d`, `php_dictionary_30d`, `php_harvesters_30d`, and
  `php_spammers_30d` have the same Project Honey Pot terms concern as the 1d
  variants.
- `sblam` was recommended for removal or downgrade by its evaluator because
  the upstream appears no longer actively supported and no clear removal path
  was found.

Human-review additions:

- `stopforumspam_30d`, `abuseipdb_30d`, and `php_bad_30d`.

Recommendation:

- Prefer a conservative prune/review pass before broadening this merge.
- Consider replacing `stopforumspam` with `stopforumspam_30d` if the merge
  contract is truly a 30-day window.

### Proxies

Current keeps:

- `ip2proxy_px1lite`, `socks_proxy_30d`, and `sslproxies_30d` remain plausible
  proxy components, subject to non-redistribution and critical-overlap caveats.

Current review:

- `iblocklist_proxies` is semantically on-topic but methodologically thin and
  license-restrictive.

Human-review additions:

- DroneBL anonymizer/compromised/open-proxy classes and shorter Didsoft proxy
  windows.

Recommendation:

- Keep the proxy merge narrow unless the user wants a broader policy/risk
  proxy detector.
- Do not add shorter proxy windows additively on top of 30-day windows; choose
  the freshness window intentionally.

### Anonymous

Current keeps:

- `dm_tor`, `tor_exits`, and nested `firehol_proxies` remain plausible.

Current removal or repair:

- `anonymous` has no matching published index entry and no source definition
  found in config. The merge should either remove it or replace it with a
  current defined source.

Human-review additions:

- `et_tor`, `shadowwhisperer_tunnel`, DroneBL anonymizer classes, and
  `misp_vpn`.

Recommendation:

- First repair the undefined `anonymous` component.
- Avoid adding broad VPN/provider lists unless the merge is explicitly a
  policy-risk anonymizer merge, not only Tor/open-proxy infrastructure.

### Webclient

Current keep:

- `cybercrime` remains plausible, but it is currently the only component.

Strong addition to consider:

- `threatfox_ips`: healthy, malware infrastructure, BSD-3-Clause, hourly
  update; generated markdown describes malware IOC scope and automatic
  expiration from the underlying ThreatFox export. It also has 2.0% critical
  infrastructure overlap, so subtraction should exist before inclusion.

Human-review additions:

- `criticalpath_cobaltstrike`, `drb_ra_c2intel`, `drb_ra_c2intel_30d`,
  `malwarefilter_botnet`, `romainmarcoux_outgoing_aa`,
  `romainmarcoux_outgoing_ab`, `shadowwhisperer_hosting`, `threatview_ip`,
  `threatview_c2`, `viriback`, and `vxvault_url_list`.

Recommendation:

- If the project adds one webclient feed in this SOW, `threatfox_ips` is the
  strongest candidate.
- Do not broaden webclient into a generic malware aggregate without updating
  its public purpose and warnings.

### Webserver

Current keeps:

- `myip` and `stopforumspam_toxic` remain plausible.

Human-review additions:

- `blocklist_de_apache`, `blocklist_de_bots`, `blocklist_de_bruteforce`,
  `data_shield`, `data_shield_critical`, `gpf_comics`, `hfish_honeypot`,
  `nginx_bad_bot_blocker`, `php_bad*`, and `php_commenters`.

Recommendation:

- Consider webserver additions only after deciding whether this merge is a
  WAF/HTTP-abuse merge or a general inbound-abuse merge.

## Feeds To Exclude From FireHOL Merges

Evaluator consensus exclusions:

- Active `critical_*` feeds are not additive threat feeds; they should be
  subtractive/contextual critical-infrastructure references.
- Provider infrastructure context feeds are not additive threat feeds.
- `datacenters` is not an additive malicious feed.
- Bogon/special-use feeds should not be treated as malicious-host evidence.
- DataPlane feeds should not be added to FireHOL merges; evaluators reported
  their generated pages say they are not blocklists and have redistribution
  restrictions.
- Most MISP warninglists/provider/context feeds are not additive threat feeds;
  use them as context or subtraction, not as FireHOL blocklist components.
- iBlocklist policy/ISP/org/P2P feeds should not be added to FireHOL merges.

## Critical-Infrastructure Exclusion

Verified facts:

- Catalog YAML defines active critical-infrastructure sources with
  `use: [critical_infrastructure]`, for example
  `configs/firehol/sources/provider_infrastructure/critical_dns_01_critical_public_dns_core.yaml:29`
  and
  `configs/firehol/sources/provider_infrastructure/critical_dns_02_critical_dns_root_servers.yaml:33`.
- The published index shows these `critical_*` feeds as healthy
  provider-infrastructure outputs.
- Current `firehol_*` merge definitions do not define an explicit exclude list.

Recommendation:

- Implement critical-infrastructure exclusion through catalog roles/metadata,
  not hardcoded feed names or IP/CIDR lists.
- Prefer a generated or config-expanded subtractive family that includes every
  feed with `use: [critical_infrastructure]`.
- Apply the subtraction to all FireHOL-owned merge outputs unless the user
  explicitly excludes a merge from this rule.
- Public copy should explain the operational meaning: FireHOL merges avoid
  known critical infrastructure to reduce collateral damage; overlap removal is
  a safety filter, not proof the upstream feed was wrong.

## Candidate Shortlist

Strongest first-wave additions if the user wants additions now:

| Candidate | Proposed merge | Why it is strongest | Risk |
|---|---|---|---|
| `blocklist_de_strongips` | `firehol_level2` | focused blocklist.de high-confidence/strong IP feed; healthy intrusion feed | possible duplication with `blocklist_de` |
| `opendbl_bruteforce` | `firehol_level2` | focused brute-force feed; healthy intrusion feed | needs critical subtraction and false-positive review |
| `ipsum_7`, `ipsum_8` | `firehol_level3` | high-threshold IPsum variants; healthier level3 fit than low-threshold variants | aggregate source, duplication risk |
| `threatfox_ips` | `firehol_webclient` | current malware IOC source, hourly update, BSD-3-Clause | 2.0% critical-infrastructure overlap before subtraction |

Strongest first-wave removals or repairs:

| Feed | Current merge | Reason |
|---|---|---|
| `feodo` | `firehol_level1` | published health is `archived` |
| `ciarmy` | `firehol_level3` | published health is `empty` |
| `anonymous` | `firehol_anonymous` | no published index entry or source definition found |
| `iblocklist_hijacked` | `firehol_level4` | policy-risk category and restrictive iBlocklist terms |
| `iblocklist_webexploit` | `firehol_level4` | restrictive iBlocklist terms and weak current methodology signal |
| `php_*_1d`, `php_*_30d` | abuser merges | Project Honey Pot terms say All Rights Reserved |
| `sblam` | `firehol_abusers_30d` | evaluator found upstream maintenance/removal concerns |

## Decisions Required Before Implementation

### Decision 1 - `firehol_level1` purpose

Option A - strict malicious/abusive low-FP tier:

- Remove `feodo`.
- Remove or split `fullbogons` from level1.
- Keep `dshield` and `spamhaus_drop`.
- Add no new level1 feeds.
- Pros: cleanest trust model; least surprising for a "malicious IP" tier.
- Risks: changes historical level1 semantics because fullbogons has long been
  part of the merge.

Option B - historical edge-safety tier:

- Remove `feodo`.
- Keep `fullbogons` with explicit compatibility explanation.
- Keep `dshield` and `spamhaus_drop`.
- Add no new level1 feeds.
- Pros: preserves historical border-filter behavior.
- Risks: level1 remains a mix of malicious-host and registry-policy data.

Recommendation: Option A, unless historical FireHOL compatibility is more
important than semantic clarity.

### Decision 2 - first implementation scope

Option A - prune and safety-filter only:

- Remove or repair stale/undefined/high-risk current components.
- Add critical-infrastructure subtraction to all FireHOL-owned merges.
- Add no new feeds.
- Pros: safest first change; reduces known problems before increasing coverage.
- Risks: users may see smaller lists and no new coverage.

Option B - prune, safety-filter, and add the strongest candidates:

- Do Option A.
- Add only `blocklist_de_strongips`, `opendbl_bruteforce`, `ipsum_7`,
  `ipsum_8`, and `threatfox_ips` where approved.
- Pros: improves coverage in a controlled way.
- Risks: still changes semantics and requires more validation.

Option C - broad expansion:

- Add many possible candidates from the evaluator reports.
- Pros: maximum coverage.
- Risks: highest false-positive and trust risk; not recommended for this SOW.

Recommendation: Option A for the first implementation pass.

### Decision 3 - Project Honey Pot components

Option A - remove now:

- Remove `php_commenters_*`, `php_dictionary_*`, `php_harvesters_*`, and
  `php_spammers_*` from FireHOL merges.
- Pros: avoids shipping aggregate outputs that depend on All Rights Reserved
  source terms.
- Risks: reduces abuser merge coverage.

Option B - keep pending legal review:

- Keep them temporarily and mark the legal risk in the SOW/spec/docs.
- Pros: preserves current behavior.
- Risks: knowingly continues a licensing concern.

Recommendation: Option A.

### Decision 4 - critical-infrastructure exclusion implementation

Option A - metadata-driven global subtractive family:

- Generate or expand an exclusion set from feeds with
  `use: [critical_infrastructure]`.
- Apply it to every FireHOL-owned merge.
- Pros: matches project rules; future critical feeds are included
  automatically.
- Risks: requires careful config/spec/test updates.

Option B - explicit per-merge exclude lists:

- Add named critical feeds manually to each merge.
- Pros: simpler to read locally.
- Risks: high drift risk; violates the spirit of typed metadata ownership.

Recommendation: Option A.

### Decision 5 - webclient expansion

Option A - no webclient addition in the first pass:

- Keep `cybercrime` only while pruning/safety filtering happens elsewhere.
- Pros: minimal blast radius.
- Risks: misses the strongest new candidate.

Option B - add `threatfox_ips` after critical subtraction exists:

- Pros: strongest current malware-infrastructure candidate for webclient.
- Risks: changes current one-feed merge and must be explained publicly.

Recommendation: Option B if Decision 2 is Option B; otherwise defer.

## Open Validation Needed After Approval

- Confirm the config loader supports the chosen subtractive critical-infra
  representation, or implement it with tests.
- Rebuild/regenerate FireHOL static enrichment so public component lists and
  merge descriptions match YAML.
- Verify published artifacts remove archived/empty/undefined inputs as
  expected.
- Run focused config/catalog tests, then broader build/test commands selected
  by the implementation blast radius.
- Update relevant specs and operator/public docs only after approved behavior
  changes are implemented.
