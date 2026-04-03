# SOW-0037 | 2026-05-01 | reference-provider-health-semantics

## Status

completed

## Requirements

### Purpose

Keep feed health meaningful by separating threat-feed freshness semantics from
reference/provider data semantics. Critical-infrastructure reference feeds and
provider databases must not be labelled `unmaintained`, `risky`, `delayed`, or
`empty` in ways that mislead operators or suppress useful derived artifacts.

### User request quoted verbatim

> some critical feeds and ipip_country are considered unmaintained

Follow-up:

> or empty

### Assistant understanding

- The installed admin feed inventory currently reports some critical reference
  feeds as age-stale, and `ipip_country` as `unmaintained`.
- At least one critical feed is `empty`.
- The likely problem is not a UI wording issue. The health classifier is using
  the same age/emptiness model for ordinary threat feeds, critical
  reference feeds, and provider databases.
- The fix must be data/config/role driven, not based on feed-name patterns.

### Acceptance criteria

- Critical-infrastructure reference feeds do not become delayed/risky/
  unmaintained only because their authoritative reference data is stable.
- Provider-database feeds such as `ipip_country` do not become age-unmaintained
  only because provider data is stable.
- Truly empty critical/provider sources are investigated and fixed or
  classified with explicit, role-aware semantics.
- Implementation uses config fields, `use:` roles, or typed metadata; no
  substring matching on feed names.
- Config/spec/tests are updated so this class does not recur.
- Installed service validates the affected feeds no longer show misleading
  health classes.

## Analysis

Installed runtime evidence on 2026-05-01:

- `critical_soft_auth0` is `unmaintained`; `time_since_last_change_mins`
  exceeds `unmaintained_threshold_mins`.
- `critical_soft_braintree` is `unmaintained`.
- `critical_soft_terraform_cloud` is `unmaintained`.
- `critical_soft_mollie` is `risky`.
- `critical_soft_zoom` is `delayed`.
- `critical_soft_salesforce_hyperforce` is `empty`.
- `ipip_country` is `unmaintained`.

Code/config facts:

- `feedhealth.Classify` already supports `Source.ExcludeFromUnmaintained`.
- The field suppresses age-based states, but not empty/unavailable.
- Special-use bogon/reference feeds already use `exclude_from_unmaintained`.
- Critical reference feeds carry `use: [critical_infrastructure]` plus typed
  `critical:` metadata.
- `ipip_country` carries `use: [geoip]` and is hidden provider data.

Working theory:

- Age-based unmaintained status is appropriate for threat feeds whose value
  decays when content stops changing.
- It is not appropriate for authoritative reference feeds whose correct content
  may remain unchanged for months.
- Empty status needs separate handling: either the source/parser is broken, or
  the source legitimately has no current IPv4 entries and should not be
  presented as an ordinary feed-health failure without role context.

## Implications and decisions

No user decision is needed before implementation. The user already gave the
product direction: these classifications are wrong for the affected feeds.

Implementation constraints:

- Prefer existing `exclude_from_unmaintained` when role-specific config needs
  to opt out of age classification.
- If a role-wide rule is needed, derive it from explicit `use:` tags, not names.
- Do not suppress true upstream/parser failures under a broad "critical means
  healthy" rule.

## Plan

1. Reproduce and locate the affected health inputs.
2. Fix role/config-driven age classification for critical reference feeds and
   provider data.
3. Investigate the empty critical Salesforce feed and fix parser/config/source
   behavior if the upstream contains usable ranges.
4. Add regression tests for both age-stability and empty/source behavior.
5. Update specs/skills if a durable rule changes.
6. Install and smoke-check the affected feeds.

## Execution log

- Opened SOW from installed-service regression report.
- Confirmed installed runtime symptoms through `/api/v1/admin/feeds`:
  critical reference feeds were `delayed`, `risky`, or `unmaintained` from
  timestamp age alone; `ipip_country` was `unmaintained`; Salesforce Hyperforce
  was `empty`.
- Fixed `feedhealth.Classify` to suppress age-only health states for configured
  reference/provider roles: `critical_infrastructure`, `provider_context`,
  `asn`, and `geoip`.
- Preserved real failure semantics by keeping `unavailable`, `archived`, and
  `empty` checks before age suppression.
- Fixed Salesforce Hyperforce extraction because the upstream JSON currently
  publishes `prefixes[].ip_prefix` as an array of CIDRs, not a scalar string.
- Updated feed-health methodology to explain user-facing interpretation rather
  than code paths/config internals.
- Updated specs and project skills with the durable role-driven health rule.

## Validation

- `go test ./pkg/feedhealth ./pkg/processor ./pkg/config ./pkg/engine ./pkg/web ./pkg/scheduler` passed.
- `make test-strict` passed.
- `make test` passed.
- `git diff --check` passed.
- `./install.sh` completed and restarted `update-ipsets.service`.
- Runtime smoke:
  - `GET /healthz` returned `ok`.
  - `GET /api/v1/sets` returned 821010 bytes.
- Forced recheck of `critical_soft_salesforce_hyperforce` after the parser fix:
  it moved from `empty`/0 entries to `healthy`/18 entries/12544 unique IPs.
- Affected installed rows after fix:
  - `critical_soft_auth0`: `healthy`, excluded from age-only health, 103 entries.
  - `critical_soft_braintree`: `healthy`, excluded from age-only health, 148 entries.
  - `critical_soft_mollie`: `healthy`, excluded from age-only health, 15 entries.
  - `critical_soft_salesforce_hyperforce`: `healthy`, excluded from age-only health, 18 entries.
  - `critical_soft_terraform_cloud`: `healthy`, excluded from age-only health, 16 entries.
  - `critical_soft_zoom`: `healthy`, excluded from age-only health, 47 entries.
  - `ip2location_country`: `healthy`, excluded from age-only health, 282384 entries.
  - `ipip_country`: `healthy`, excluded from age-only health, 528621 entries.
- No installed `critical_*` feed remained non-healthy after the Salesforce
  recheck.
- `GET /api/v1/admin/integrity` settled as `clean`, count 0.
- `GET /api/v1/admin/integrity/entities` settled as `clean`, count 0.

## Outcome

Shipped role-driven feed-health semantics for stable reference/provider data
and fixed the Salesforce Hyperforce parser drift. Critical infrastructure
reference feeds, broad provider context, ASN providers, and geolocation
providers no longer become delayed/risky/unmaintained from age alone, but real
empty/unavailable/archived states still surface.

## Lessons extracted

- Feed health is not one semantic for all feeds. Threat feeds use freshness as
  a value-decay signal; reference/provider data can be stable and still correct.
  This belongs in specs and project-coding rules.
- Parser drift on public reference feeds should be treated as a real `empty`
  finding, not suppressed by criticality. Tests now cover the Salesforce array
  shape and the feed-health empty-preservation rule.
- Public methodology pages must explain interpretation and limitations. Code
  paths, config key names, and implementation provenance belong in specs/docs,
  not in public methodology.
