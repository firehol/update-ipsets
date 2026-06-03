# SOW-0016 FireHOL Level Criteria Evaluation Prompt

## Task

Review your assigned slice of the `firehol_level1` through `firehol_level4`
candidate universe. Classify every assigned feed against the criteria below.

This is a read-only evaluation task. Do not modify files. Do not change YAML,
code, docs, specs, generated artifacts, or SOW files.

## Goal

The goal is not to say "none fits". The goal is to produce a structured
evaluation trail:

- how many feeds entered the candidate universe,
- how many feeds were current-component exceptions,
- which feeds were candidate-level material,
- which criteria each feed passed or failed,
- why each rejected feed was rejected,
- which current components should be kept, removed, replaced, or reviewed.

## Evidence Sources

Use local published artifacts and catalog files:

- Published index: `/opt/update-ipsets/web/index.json`
- Published markdown: `/opt/update-ipsets/web/{feed}.md`
- Current level merge YAML:
  - `configs/firehol/merges/firehol_level1.yaml`
  - `configs/firehol/merges/firehol_level2.yaml`
  - `configs/firehol/merges/firehol_level3.yaml`
  - `configs/firehol/merges/firehol_level4.yaml`
- Category definitions: `configs/firehol/categories.yaml`

Do not copy raw IP lists into the report. Summarize critical-infrastructure
overlap from the markdown page if relevant.

## Candidate Universe

New-addition candidates are active primary feeds in these categories only:

- `intrusion`
- `malware_infrastructure`
- `service_abuse`
- `messaging_abuse`
- `scanners`

Exclude from new-addition candidates:

- feeds whose `health.class` is `archived`, `empty`, or `unavailable`;
- `provenance: secondary_retention`;
- `provenance: secondary_merge`.

Current `firehol_level1..4` components are exceptions. They must be reviewed
even if they are inactive, retention derivatives, merges, non-candidate
categories, or otherwise outside the new-addition filter.

Use this command to generate your assigned sorted slice. Replace `SLICE=1` with
your assigned slice number from 1 to 4:

```bash
SLICE=1 ruby -rjson -e '
idx = JSON.parse(File.read("/opt/update-ipsets/web/index.json"))
excluded_health = %w[archived empty unavailable]
candidate_categories = %w[
  intrusion
  malware_infrastructure
  service_abuse
  messaging_abuse
  scanners
]
current_components = %w[
  dshield
  feodo
  fullbogons
  spamhaus_drop
  blocklist_de
  dshield_1d
  greensnow
  bruteforceblocker
  ciarmy
  dshield_30d
  myip
  vxvault
  blocklist_net_ua
  botscout_30d
  cybercrime
  iblocklist_hijacked
  iblocklist_spyware
  iblocklist_webexploit
]
candidate = idx.select do |x|
  candidate_categories.include?(x["category"]) &&
    !excluded_health.include?(x.dig("health", "class").to_s) &&
    !%w[secondary_retention secondary_merge].include?(x["provenance"].to_s)
end.map { |x| x["name"] }

universe = (candidate + current_components).uniq.sort
i = Integer(ENV.fetch("SLICE"))
start = (universe.size * (i - 1)) / 4
stop = (universe.size * i) / 4
universe[start...stop].each { |name| puts name }
'
```

At prompt creation time this universe contained:

- 349 active feeds in the published index.
- 212 active feeds in candidate categories.
- 171 active primary new-addition candidates after excluding retention and
  merge derivatives.
- 18 current level components.
- 179 total review items after adding current-component exceptions and
  deduplicating.

## FireHOL Level Criteria

### `firehol_level1`

Purpose: maximum protection with minimum false positives. Safe enough for blind
perimeter blocking on internet-facing servers, routers, and firewalls.

Inclusion criteria:

- near-zero false positives by design, not merely low observed overlap;
- clearly malicious infrastructure, clearly hostile traffic source, or
  historical edge-safety reference with explicit justification;
- strong maintainer/source process;
- current and maintained;
- clear expiration/removal behavior or a listing method that makes manual
  removal unnecessary;
- no broad reputation scoring;
- no policy-only feed;
- no scanner-only feed unless the scanner behavior is severe enough to be a
  proven attack source and FP risk is near-zero;
- no shared-hosting/cloud-heavy feed unless confidence and expiry are
  exceptional;
- no aggregate-of-aggregates unless every input would independently satisfy
  level1.

### `firehol_level2`

Purpose: recent attack-source blocking, roughly 24-48 hours, used on top of
level1 by operators who can tolerate modest false-positive risk.

Inclusion criteria:

- recent observed attack or abuse source;
- attack-source oriented: brute force, exposed-service attack, exploit attempt,
  hostile web/service automation, abuse report with clear behavior;
- freshness/expiry is suitable for about 24-48 hours;
- methodology is clear enough for blocking with modest FP tolerance;
- no broad long-memory reputation feed;
- no policy/context-only feed.

Retention handling:

- Do not evaluate retention derivative pages as new sources.
- If a base feed qualifies for level2 only through a 1d derivative, recommend
  the derivative explicitly, e.g. `recommended_component: dshield_1d` and
  `parent_evaluated: dshield`.

### `firehol_level3`

Purpose: broader attack, malware, spyware, virus, and abuse coverage over
roughly 30 days. Suitable for detection, enrichment, hunting, or blocking where
the operator accepts higher FP risk.

Inclusion criteria:

- relevant to intrusion, malware infrastructure, service abuse, messaging
  abuse, or hostile scanners;
- acceptable methodology, even if broader/noisier than level2;
- time horizon roughly current to 30 days, or a clear maintained current feed
  whose content represents active threats;
- false-positive risk can be medium, but must be explainable;
- aggregate feeds may fit if inputs and maintenance are clear enough;
- no pure provider/context/reference feed.

Retention handling:

- If a base feed qualifies for level3 through a 30d derivative, recommend the
  derivative explicitly.

### `firehol_level4`

Purpose: broadest FireHOL level. Maximum coverage with high false-positive risk,
for research, retrospective analysis, and cautious detection. Not suitable for
blind inline blocking.

Inclusion criteria:

- security-relevant threat, abuse, malware, intrusion, or scanner signal;
- may be broad, older, noisy, or aggregate-heavy;
- high FP risk is acceptable only if it is clearly documented and not caused by
  pure context/provider data;
- still exclude feeds that are not threat/security feeds at all;
- still reject feeds with unacceptable license or maintenance risk unless the
  report marks them as `legal-review` or `human-review`, not include.

## Required Criterion Columns

For every assigned feed, produce one row with these fields:

| Field | Allowed values / guidance |
|---|---|
| `feed` | feed name |
| `review_type` | `new_candidate`, `current_component_exception`, or both |
| `current_level_membership` | `none`, `level1`, `level2`, `level3`, `level4` |
| `health` | published `health.class`, or `missing-index-entry` |
| `category` | published category |
| `provenance` | published provenance |
| `candidate_category` | `pass`, `fail-current-exception` |
| `primary_source` | `pass`, `retention-current-exception`, `merge-current-exception`, `secondary-review`, `fail` |
| `threat_semantics` | `intrusion`, `malware`, `service-abuse`, `messaging-abuse`, `scanner`, `policy-only`, `context-only`, `special-use`, `unclear` |
| `methodology_quality` | `strong`, `acceptable`, `weak`, `unknown` |
| `freshness_fit` | `level1`, `level2`, `level3`, `level4`, `none`, `unknown` |
| `false_positive_risk` | `very-low`, `low`, `medium`, `high`, `unacceptable`, `unknown` |
| `critical_infra_overlap` | `none`, `low`, `material`, `high`, `unknown` |
| `maintenance` | `strong`, `acceptable`, `weak`, `abandoned`, `unknown` |
| `license_fit` | `pass`, `review`, `fail`, `unknown` |
| `recommended_level` | `none`, `level1`, `level2`, `level3`, `level4`, or multiple only when justified |
| `recommended_component` | feed name or derivative feed name if recommending retention derivative |
| `drop_reason` | one primary reason if `recommended_level=none` |
| `notes` | short evidence-based explanation |

## Drop Reason Taxonomy

Use one primary drop reason:

- `inactive`
- `wrong-category`
- `context-or-provider-only`
- `policy-only`
- `special-use-only`
- `scanner-too-noisy`
- `methodology-weak`
- `freshness-wrong`
- `fp-risk-too-high`
- `critical-infra-risk`
- `license-fail`
- `maintenance-weak`
- `duplicate-or-derived`
- `aggregate-too-opaque`
- `not-level-family`
- `needs-human-review`

## Required Report Structure

Return the report in Markdown with these sections:

1. Scope
   - slice number
   - number of assigned feeds
   - first and last assigned feed
   - missing markdown count
2. Funnel Accounting
   - assigned total
   - current-component exceptions
   - new candidates
   - recommended level1/2/3/4 counts
   - dropped count by primary drop reason
   - human-review count
3. Current Component Review
   - one row for each assigned current component
   - keep/remove/replace/review recommendation
4. Candidate Classification Table
   - one row per assigned feed with all required criterion columns
5. Level Recommendations
   - candidate feeds per level
   - suggested retention derivatives, if any, with parent feed evidence
6. Questions / Uncertainty
   - only concrete questions backed by evidence

## Rules

- Facts must come from index fields, markdown text, current YAML, or category
  definitions.
- Separate facts from judgment.
- Do not use raw IP lists.
- Do not inspect excluded categories unless the feed is a current-component
  exception in your assigned slice.
- Do not recommend `firehol_*` merges as components of `firehol_level1..4`.
- Do not treat overlap with other threat feeds as enough evidence by itself.
- Do not recommend a feed for `level1` unless it clearly passes the level1
  false-positive and operational-safety bar.
- All final recommendations assume critical-infrastructure subtraction will be
  applied later through catalog metadata.
