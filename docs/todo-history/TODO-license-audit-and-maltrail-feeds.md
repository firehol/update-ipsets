# TODO: License pass-through audit + new Maltrail feeds

## TL;DR

Three interlocking tasks following Costa's 2026-04-09 guidance:

1. **Codify the pass-through license policy** in `AGENTS.md` as an authoritative rule.
2. **Audit every `redistributable: false`** entry in `configs/firehol.yaml` (64 entries today) — many are wrongly marked because a NonCommercial license was conflated with "not redistributable". Pass-through redistribution under the original license is allowed.
3. **Add 5 new Maltrail-surfaced IP feeds** with correct license + redistributable + category fields, informed by the audit.

## The rule (Costa's words)

> "non-commercial does not mean non-redistributable. The downloader must comply to the original license. iplists.firehol.org is pass-through. It redistributes it under the original license."

**Implication**: `iplists.firehol.org` and the `blocklist-ipsets` git repo are **pass-through** distributors. Downstream consumers (not FireHOL) must comply with each source's license. Our obligation is:
- Record the original license accurately in `configs/firehol.yaml`'s `license:` field
- Preserve attribution where required (`attribution:` field)
- Set `redistributable: false` **only** when the source explicitly forbids redistribution — not when it restricts downstream use

## Also (second Costa clarification)

> "We also have 'organizations' category - this is not to be blocked. It is just a list of IPs of something not necessarily bad."

**Implication**: `category: organizations` is the informational bucket (ISPs, CDNs, corp IPs, benign research scanners). It is NOT a blocklist category. The existing `maltrail_scanners` (configs/firehol.yaml:1962) currently sits in `attacks` but contains Rapid7 / Cortex Xpanse / ltx71 / Censys research scanners — this is exactly the `organizations` pattern. Flag for recategorization (not in this PR unless Costa approves).

---

## Analysis: current state of `redistributable` field

### Codebase facts
- **Field**: `Redistributable *bool` in `pkg/config/config.go:158` — tri-state; `nil == true` (default is redistribute).
- **Accessor**: `Source.IsRedistributable()` at `pkg/config/config.go:214` — returns `true` when pointer is nil.
- **Legacy loader**: `pkg/config/extract.go:166` maps the bash `dont_redistribute` attribute to `Redistributable: boolPtr(false)`.
- **Enforcement test**: `pkg/config/catalog_verify_test.go:755-826` — hardcodes the `shouldNotRedistribute` and `shouldRedistribute` lists. **This test must be updated in lockstep with every YAML change** or it fails.

### Count
- 64 `redistributable: false` entries in `configs/firehol.yaml` today.
- The remaining ~113 sources default to redistributable (no field or `redistributable: true`).

### Inconsistency observed
iBlocklist entries share the **same license string** ("iBlocklist Terms of Service — personal / non-commercial only") but are inconsistently marked:
- **Marked `redistributable: false`** (20 entries): ads, bogons, dshield, edu, exclusions, fornonlancomputers, forumspam, hijacked, iana_multicast, iana_private, iana_reserved, level1, level2, level3, org_microsoft, proxies, rangetest, spider, spyware, webexploit
- **NOT marked** (defaulting to redistributable) (~40 entries): abuse_palevo, abuse_spyeye, ciarmy_malicious, cidr_report_bogons, cruzit_web_attacks, isp_* (13), malc0de, onion_router, org_* (21 except microsoft), pedophiles, spamhaus_drop, yoyo_adservers

Same license, different treatment — the test list at `catalog_verify_test.go:773-781` hardcodes only the first group. This is a bug.

---

## Audit: every `redistributable: false` entry

Grouped by verdict. Evidence is the `license:` string as recorded today.

### GROUP A — Stay `redistributable: false` (license explicitly forbids redistribution)

| # | Source(s) | License | Evidence |
|---|-----------|---------|----------|
| A1 | `bogons` (L240), `fullbogons` (L834) | Team Cymru — no redistribution per Team Cymru policy | **Explicit** — "no redistribution" |
| A2 | `dataplane_*` (9 feeds, L424–544) | Dataplane.org — non-commercial use only, **redistribution prohibited** | **Explicit** — "redistribution prohibited" |
| A3 | `greensnow` (L905) | Reproduction or republication prohibited | **Explicit** — "republication prohibited" |
| A4 | `php_bad`, `php_commenters`, `php_dictionary`, `php_harvesters`, `php_spammers` (L2003–2079) | Project Honey Pot Terms of Use — **All Rights Reserved** | **Explicit** — All Rights Reserved grants no redistribution right |
| A5 | `caida_prefix2as` (L361) | CAIDA Acceptable Use Agreement | CAIDA AUP typically requires prior permission for redistribution — **KEEP false, verify AUP terms** |
| A6 | `abuseipdb_1d`, `abuseipdb_30d` (L39, L54) | AbuseIPDB Terms of Service | AbuseIPDB's ToS restricts bulk redistribution; borestad's public mirror is a gray zone — **KEEP false pending clarification** |

### GROUP B — Flip to `redistributable: true` (license is NC-only or use-restricted, not redistribution-restricted)

| # | Source(s) | License | Verdict |
|---|-----------|---------|---------|
| B1 | `bds_atif` (L68) | Binary Defense ATIF — **non-commercial use only** | NC ≠ no redistribution. Pass-through OK. **FLIP** |
| B2 | `botscout` (L254) | BotScout Terms of Service — personal/non-commercial only | NC ≠ no redistribution. **FLIP** |
| B3 | `botvrij_dst`, `botvrij_src` (L273, L289) | Botvrij.eu — use at own risk, **no resale** | "No resale" ≠ "no redistribution". **FLIP** |
| B4 | `dronebl_*` (9 feeds, L595–701) | DroneBL community data — **software BSD-style** | BSD = permissive redistribution. **FLIP** |
| B5 | `dshield` (L712) | CC BY-NC-SA 4.0 | NC with attribution + share-alike is explicitly redistributable. **FLIP** |
| B6 | `griffinguard` (L919) | GriffinGuard — security research / monitoring only | USE restriction, not distribution restriction. **FLIP** |
| B7 | `iblocklist_*` (20 feeds marked false + 40 feeds unmarked) | iBlocklist ToS — personal / non-commercial only | NC ≠ no redistribution. **NORMALIZE all 60 to `true`** |
| B8 | `ip2proxy_px1lite` (L1811) | IP2Proxy LITE — **attribution required** | "Attribution required" ≠ "no redistribution". **FLIP** + ensure `attribution:` field is present |
| B9 | `stopforumspam_*` (8 feeds, L2179–2285) | CC BY-NC-ND 3.0 (modified) | CC BY-NC-ND permits verbatim redistribution (the ND restricts derivatives, not distribution). Pass-through unmodified IP lists are fine. **FLIP** |

### Totals
- **Stay false**: 6 groups covering ~18 entries (bogons×2, dataplane×9, greensnow, php×5, caida, abuseipdb×2)
- **Flip to true**: 9 groups covering ~46 entries (bds_atif, botscout, botvrij×2, dronebl×9, dshield, griffinguard, iblocklist×20 + normalize 40 more, ip2proxy_px1lite, stopforumspam×8)

---

## 5 new Maltrail-surfaced feeds to add

See separate research in conversation. Decisions from Costa: **add all 5**.

| # | name | URL | License | redistributable | category | Notes |
|---|------|-----|---------|-----------------|----------|-------|
| N1 | `turris_greylist` | https://view.sentinel.turris.cz/greylist-data/greylist-latest.csv | CC BY-NC-SA 4.0 | **true** (pass-through) | `attacks` | Needs attribution; CSV parse — column 0 = IP, column 1 = tags |
| N2 | `viriback` | http://tracker.viriback.com/dump.php | Unknown (no published license) | **false** pending clarification | `malware` | CSV: Family,URL,IP,FirstSeen. Extract column 3, filter by date freshness |
| N3 | `maltrail_scanners_cidr` | https://raw.githubusercontent.com/stamparm/maltrail/master/trails/static/mass_scanner_cidr.txt | MIT | **true** | `organizations` | Sibling of existing `maltrail_scanners`; contains Rapid7/Cortex Xpanse/ltx71 /24 CIDRs — benign research scanners |
| N4 | `rutgers_drop` | https://report.cs.rutgers.edu/DROP/attackers | Unknown (no stated license) | **false** pending clarification | `attacks` | Small (~320 IPs) academic honeypot, refreshed frequently |
| N5 | `sekuripy_ipnoise` | https://www.sekuripy.hr/blacklist.txt | Unknown (no stated license) | **false** pending clarification | `attacks` | Small (~256 IPs) SSH brute-force offenders |

**Note on N3 category**: the existing `maltrail_scanners` (line 1962) is currently `category: attacks` but is semantically `organizations` per Costa's guidance. The new `maltrail_scanners_cidr` will go directly into `organizations`. The existing one's category change is flagged as a follow-up — not changed here to avoid scope creep.

---

## Decisions required from Costa

Please reply with numbers + Y/N/choice:

### D1 — AGENTS.md rule placement

Where should the pass-through rule live in `AGENTS.md`?
- **(a)** New top-level section "License & Redistribution Policy" near the top (authoritative, high visibility)
- **(b)** New Rule 9 under "Backend Guidelines (Go)" (fits with Rules 1–8)
- **(c)** Both: a short bullet under Backend Guidelines pointing to a dedicated section

**Recommendation: (c)** — the policy is domain-level, not just a Go coding rule, so it deserves its own section; Backend Guidelines can reference it.

### D2 — Audit decisions (Group A — should any be flipped?)

- **D2.1** — `caida_prefix2as` (CAIDA AUP): keep `false`, or research AUP and flip if it allows pass-through?
  - Recommendation: **research AUP; default keep false until confirmed**
- **D2.2** — `abuseipdb_1d/30d`: keep `false`, or flip given borestad publishes the data openly on GitHub?
  - Recommendation: **keep false; borestad's public mirror ≠ AbuseIPDB's blessing; ToS trumps mirror**

### D3 — Audit decisions (Group B — flip all, or any exceptions?)

- **D3.1** — Flip **all 9 groups** (B1–B9) at once? → **Recommendation: yes**
- **D3.2** — For `stopforumspam_*` (B9, CC BY-NC-ND) — ND permits verbatim redistribution but prohibits derivatives. Is our ipset processing a "derivative" (we strip headers, sort, aggregate)?
  - Recommendation: **flip — our processing is format conversion of the same facts, not a derivative work; downstream consumers inherit the ND obligation**
- **D3.3** — For `iblocklist_*` — normalize **all 60 entries** (add `redistributable: true` explicitly, or remove the field to use default)?
  - Recommendation: **remove the field (rely on default)** to reduce noise; keep `license:` everywhere

### D4 — `maltrail_scanners` recategorization

The existing `maltrail_scanners` (configs/firehol.yaml:1962) is `category: attacks` but contains benign research scanners (Rapid7, Cortex Xpanse, ltx71). Change to `category: organizations`?
- **(a)** Yes — change in this PR alongside the new `maltrail_scanners_cidr`
- **(b)** No — separate follow-up PR
- **(c)** No — keep as `attacks` (argue they're attack-adjacent regardless)

**Recommendation: (a)** — the new CIDR sibling will be in `organizations` anyway; consistency is better in one change.

### D5 — New feeds: categories and redistributable for unknown-license sources

- **D5.1** — For `viriback` (N2), `rutgers_drop` (N4), `sekuripy_ipnoise` (N5): add as `redistributable: false` (safe default for unknown license)?
  - Recommendation: **yes** — default safe when license isn't stated; flip after contacting maintainers
- **D5.2** — Should I email maintainers (Turris, ViriBack, Rutgers, sekuripy) to request license clarification?
  - Recommendation: **yes for ViriBack** (strong candidate, actively maintained); **optional for Turris** (license already clear — CC BY-NC-SA); **skip Rutgers/sekuripy** until we know they're valuable

---

## Plan (once decisions are made)

### Phase 1 — Policy rule (AGENTS.md)
1. Add the pass-through rule per D1 placement choice.
2. Include: (a) the rule itself, (b) the `redistributable:` decision tree, (c) pointers to `pkg/config/config.go:158` and `catalog_verify_test.go:755`.

### Phase 2 — Audit fixes (`configs/firehol.yaml`)
1. Apply Group A keep-false decisions (verify comments).
2. Apply Group B flip decisions (per D3.1–D3.3).
3. Update `license:` strings where they misrepresented the terms.

### Phase 3 — Test updates (`pkg/config/catalog_verify_test.go`)
1. Update `shouldNotRedistribute` list to match Group A survivors.
2. Update `shouldRedistribute` list to include Group B flipped entries plus iblocklist normalization.
3. Run `go test ./pkg/config/...` — must pass.

### Phase 4 — Add 5 new Maltrail feeds
1. Insert YAML entries for `turris_greylist`, `viriback`, `maltrail_scanners_cidr`, `rutgers_drop`, `sekuripy_ipnoise`.
2. For each: url, frequency, ipv, output, processor chain, category, info (with markdown link), maintainer, maintainer_url, license, redistributable, attribution (where required).
3. For `viriback`: custom processor for CSV column-3 extraction with date freshness filter (see `pkg/engine/` for existing custom parsers).
4. Apply `maltrail_scanners` recategorization per D4.

### Phase 5 — Verify
1. `go build ./...` — must succeed.
2. `go test ./...` — must pass including `TestCatalogRedistributableConsistency`.
3. `go test -race ./...` — must pass.
4. `go vet ./...` — clean.
5. Install locally: `./install.sh` → restart service.
6. Verify each new feed downloads and parses:
   ```bash
   curl -u "$ADMIN" -X POST http://localhost:18888/api/v1/admin/feeds/turris_greylist/recheck
   curl http://localhost:18888/api/v1/sets/turris_greylist | jq '.unique_ips'
   ```
7. Repeat for each of the 5 new feeds.

### Phase 6 — Deploy to d1
Following the standard `~/src/firehol/CLAUDE.md` workflow:
1. Branch in `~/src/firehol/firehol/` (upstream bash clone) — **NOTE: this TODO is for the Go repo, not the bash one**. The YAML changes are in the Go repo; only bash-side changes (new `.html` description pages, if any) go through the bash PR process.
2. If the Go `configs/firehol.yaml` change is NOT meant to sync back to the bash `update-ipsets` script, deployment is simply `./install.sh` locally (and later to the target box).

**Open question for Costa**: does the Go repo's `configs/firehol.yaml` have an authoritative upstream separate from the bash repo? **Recommendation: confirm before Phase 6.**

## Testing requirements

- `go test ./pkg/config/...` passes (catalog consistency)
- `go test ./...` passes  
- `go build ./...` succeeds
- Each new feed downloads successfully once enabled
- Each new feed produces a non-empty `.ipset`/`.netset`
- Each new feed's JSON metadata renders on the admin UI without error

## Documentation updates

- **`AGENTS.md`** — add the pass-through rule (Phase 1)
- **`MEMORY.md`** — already added two feedback memories (license pass-through, organizations category)
- **Per-feed `.html` descriptions** — not required for the new Go repo (it renders `info` markdown directly from YAML)

## Implied decisions (not asked, but reasonable)

- Keep `attribution:` fields up-to-date for CC BY-* feeds (MaxMind already has this pattern at L1983)
- The field `redistributable: true` should generally NOT be written explicitly — rely on the `nil == true` default to keep YAML tidy. Only write `false` when needed.
