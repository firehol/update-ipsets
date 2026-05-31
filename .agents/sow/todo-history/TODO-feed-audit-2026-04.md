# TODO: Feed Audit & URL Canonicalization — April 2026

## Purpose

Bring existing feed entries in `configs/firehol.yaml` into alignment with the
product contract and add a small, carefully vetted set of genuinely new primary
sources. Fit-for-purpose: keep the comparative observatory accurate (right
canonical URLs, right provenance, right license), without violating the
"keep pre-existing ipset names" rule.

## TL;DR

1. **Stale old stays** — feeds that went quiet upstream are left alone. The
   system's derived `archived` health state already surfaces this on the
   public site; removal would hide history.
2. **Stale new is rejected** — new feeds are added only if they are alive and
   changing (inclusion policy, `specs/design.md`).
3. **Wrong URLs get fixed** — URL is acquisition config, not feed identity.
   Changing a URL keeps the ipset name stable and preserves history.

## Principle — grounded in specs

- `specs/design.md` "Inclusion policy": sources MUST be alive and changing
  over time. Adding a dead feed violates this.
- `specs/feeds.md` "Feed health contract": `archived` is an auto-derived
  health class, not a curated flag. The site surfaces decay naturally.
- `specs/feeds.md` "Archived feeds": archived feeds remain first-class
  identities in public catalog/detail/analytical surfaces.
- `specs/config.md` "Redistribution rule": default is allowed unless upstream
  terms explicitly forbid. Attribution, NC, warranty disclaimers, unknown
  license are NOT sufficient to mark non-redistributable.

## Analysis — current state

Eight parallel research agents inspected candidate feeds against the live
upstream, cross-referenced with `configs/firehol.yaml`, and verified HTTP
reachability. Findings:

### Existing entries with non-canonical URLs

| ipset name | Current URL | Canonical URL | Reason |
|---|---|---|---|
| `data_shield` | GitHub mirror | GitLab | README labels GitLab as "main source" |
| `data_shield_critical` | GitHub mirror | GitLab | same |
| `shadowwhisperer_scanners` | `Other/Scanners` (legacy, frozen 2026-03-28) | `Lists/Scanners` | maintainer moved the live data |
| `shadowwhisperer_tunnel` | `Other/Tunnel` (frozen) | `Lists/Tunnels` | same — 22× larger now |
| `shadowwhisperer_hackers` | `Malware/Hackers` (frozen) | `Lists/Threats` | same |
| `threatfox_ips` | elliotwutingfeng GitHub mirror | abuse.ch canonical | mirror is one-hop; abuse.ch is source |

### Existing entries with wrong metadata (no URL change)

| ipset name | Field | Current | Correct |
|---|---|---|---|
| `data_shield` | license | `unknown` | `GPL-3.0` |
| `data_shield` | provenance | `secondary_upstream` | `primary` |
| `data_shield` | info | "daily refresh" | "every ~2 hours" |
| `data_shield_critical` | license | `unknown` | `GPL-3.0` |
| `data_shield_critical` | provenance | `secondary_upstream` | `primary` |
| `data_shield_critical` | info | "daily refresh" | "every ~2 hours" |
| `hagezi_tif` | info | "~52K entries" | "~44K entries" |

### Existing entries with no canonical successor — leave alone

- `shadowwhisperer_hosting` — upstream `Malware/Hosting` frozen; no 1:1
  successor in `Lists/`. Let the health state decay to archived naturally.
- `shadowwhisperer_bruteforce_medium`, `_high`, `_extreme` — upstream
  consolidated 3 feeds into `Lists/Probes`. Keep the three ipset names
  pointing at their frozen URLs; the new `shadowwhisperer_probes` entry
  (added below) carries the live data.
- `ustc_blackip`, `nginx_bad_bot_blocker` — working correctly.

### Genuinely new primary feeds to add

| ipset name | URL | Cadence | Entries |
|---|---|---|---|
| `drb_ra_c2intel` | `feeds/IPC2s.csv` | every ~2h | ~144 |
| `drb_ra_c2intel_30d` | `feeds/IPC2s-30day.csv` | every ~2h | ~240 |
| `drb_ra_c2intel_90d` | `feeds/IPC2s-90day.csv` | every ~2h | ~2,078 |
| `shadowwhisperer_probes` | `Lists/Probes` | hourly | ~20,171 |
| `shadowwhisperer_threats_unclassified` | `Lists/Threats_Unclassified` | hourly | ~9,168 |

All five are primary (Censys-based OSINT or the maintainer's own honeypots),
alive, and changing.

### Rejected additions

- **`multiproxy.org`** — dead since 2023; raw file 301s to a placeholder page;
  last live data dated 2009. Violates inclusion policy.

### Deferred (separate task)

- **`threatfox_ips` URL fix to abuse.ch canonical** — requires a new processor
  capable of ZIP + CSV + port-stripping. Keeping the mirror URL for now is
  not ideal but doesn't block the rest of the cleanup. Metadata fields for
  this entry are also flagged as wrong (license claim is inherited from the
  mirror README, not authoritative), and will be revisited in the same
  follow-up task.

## Decisions (user-approved)

- **Principle**: stale old stays, stale new is rejected, wrong URLs are fixed
  (user confirmed 2026-04-23).
- **Scope**: three-way split recommended earlier — URL+metadata fixes + new
  additions go together in this pass; ThreatFox canonical migration is a
  separate follow-up task (needs processor work).
- **User approved creating TODO + implementing** in the same pass.

## Plan

### Step 1 — YAML edits in `configs/firehol.yaml`

**URL changes:**

1. `data_shield` (line 3524): change `url:` to
   `https://gitlab.com/duggytuxy/Data-Shield-IPv4-Blocklist/-/raw/main/prod_data-shield_ipv4_blocklist.txt?ref_type=heads`
2. `data_shield_critical` (line 3539): change `url:` to
   `https://gitlab.com/duggytuxy/Data-Shield-IPv4-Blocklist/-/raw/main/prod_critical_data-shield_ipv4_blocklist.txt?ref_type=heads`
3. `shadowwhisperer_scanners` (line 3625): change `url:` to
   `https://raw.githubusercontent.com/ShadowWhisperer/IPs/master/Lists/Scanners`
4. `shadowwhisperer_tunnel` (line 3639): change `url:` to
   `https://raw.githubusercontent.com/ShadowWhisperer/IPs/master/Lists/Tunnels`
5. `shadowwhisperer_hackers` (line 3653): change `url:` to
   `https://raw.githubusercontent.com/ShadowWhisperer/IPs/master/Lists/Threats`

**Metadata changes:**

6. `data_shield`: `license: unknown` → `license: GPL-3.0`;
   `provenance: secondary_upstream` → `provenance: primary`;
   info string: "~77K individual IPs, daily refresh" → "~79K individual IPs, refreshed every ~2 hours"
7. `data_shield_critical`: same license + provenance fixes;
   info string: "daily refresh" → "refreshed every ~2 hours"
8. `hagezi_tif`: info string: "~52K entries" → "~44K entries"

**Info text alignment for URL-changed ShadowWhisperer entries:**

Current info text says "Bitwire consumes this legacy path; the file still
updates from the repository automation." — this is no longer accurate after
the URL change. Update to reflect that we now consume the active `Lists/*`
path. (Does not change identity.)

9. `shadowwhisperer_scanners` info: describe `Lists/Scanners` (hourly, ~32K)
10. `shadowwhisperer_tunnel` info: describe `Lists/Tunnels` (hourly, ~13K)
11. `shadowwhisperer_hackers` info: describe `Lists/Threats` (hourly, ~6.5K)

**New feed additions** (append to appropriate section, alphabetized where
possible; match existing style):

12. `drb_ra_c2intel`, `drb_ra_c2intel_30d`, `drb_ra_c2intel_90d` —
    category `malware_infrastructure`, processor `extract_ipv4_from_any_file`
    (CSV format: `#ip,ioc` header + `IP,label` rows; mixed content — use
    the regex-based extractor same as other CSV-shaped feeds in the config,
    e.g. `cybercrime_tracker`, `cybercure`).
    License `CC BY-NC-SA 4.0`. Provenance `primary`.
    Frequency 120 min.
13. `shadowwhisperer_probes` — category `intrusion`, processor
    `remove_comments`, frequency 60 min. License `Unlicense`.
    Provenance `primary`.
14. `shadowwhisperer_threats_unclassified` — category `intrusion`, processor
    `remove_comments`, frequency 60 min. License `Unlicense`.
    Provenance `primary`.

### Step 2 — Build & validate

- `make build` — ensure YAML still parses and config validation passes.
- `./install.sh --no-restart` — stage the new binary.

### Step 3 — Testing

- Local daemon: `systemctl --user restart update-ipsets` (or appropriate
  service command), then:
  - Check `curl http://localhost:18888/api/v1/sets` — new ipsets appear
  - Check `curl http://localhost:18888/api/v1/sets/drb_ra_c2intel` — returns
    metadata
  - Wait one downloader cycle, re-check — the new feeds have downloader
    statuses `downloaded` or `empty` (not `download_failed`)
- Admin UI: verify the updated URLs for existing feeds fetch successfully on
  next cadence.

### Step 4 — Git

- Work on a feature branch in `/home/user/src/firehol/update-ipsets/`:
  `git checkout -b feed-audit-2026-04`
- Commit scope: `configs/firehol.yaml` and this TODO file only.
- No PR target is ambiguous — this repo's `main` branch. User decides whether
  to PR or push directly.

## Implied decisions (call out before implementing)

- **Processor choice for drb-ra CSV files**: `extract_ipv4_from_any_file`
  (regex-extract IPs from mixed content). Confirmed by sampling the file
  (`#ip,ioc` header plus `IP,label` rows) and matching the existing pattern
  for CSV-shaped feeds already in the config (`cybercrime_tracker`, etc.).
- **Frequency for ShadowWhisperer `_probes` and `_threats_unclassified`**:
  60 minutes matches the upstream hourly cadence. Conservative (no upstream
  DoS risk) and aligned with other hourly feeds in the config.
- **Info text rewrites for URL-changed ShadowWhisperer entries**: the
  current text references "Bitwire consumes this legacy path" which is now
  false. Rewriting is mandatory for truthfulness (per `specs/design.md`
  truthfulness policy). Kept brief, factual, no editorial language.
- **Not touching `ustc_blackip` circular-dependency note**: optional
  improvement; skipped to keep this pass focused on URL/metadata correctness.

## Testing requirements

- YAML parses (`make build`).
- Config validation passes.
- New feeds reach `downloaded` state within one downloader cycle.
- URL-changed feeds produce non-empty bodies on next cadence (can verify
  with `curl` against the canonical URL directly).
- No existing ipset identities change.

## Documentation updates required

- No `specs/*.md` updates: this change touches only `configs/firehol.yaml`
  (catalog data, not product behavior).
- No website contract changes.
- This TODO file captures the audit trail and will be deleted after user
  verifies implementation.

## Out of scope (follow-up tasks)

- **ThreatFox canonical URL migration**: needs a ZIP+CSV+port-strip
  processor. Will get its own TODO file when prioritized.
- **SSLBL check**: the ThreatFox agent noted abuse.ch deprecated SSLBL on
  2025-01-03. If `sslbl_*` entries exist in the config, they would have
  moved to `archived` health state naturally. Worth a standalone check
  later.
- **URL audit of other feeds**: this pass covers only the 6 candidates
  surfaced by the research. Other entries in `firehol.yaml` may also have
  non-canonical URLs; a broader audit is possible in a follow-up.
