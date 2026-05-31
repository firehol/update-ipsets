# TODO — Find and record licenses for every IP feed

> **Mission for the agent picking this up**: research the legal terms
> under which each IP feed in `configs/firehol.yaml` is published, and
> add them to the YAML in two new fields per source: `license:` and
> `attribution:` (where required). The goal is that every feed page on
> the public site can show the user under what terms the data is
> distributed and what credit, if any, the maintainer requires.
>
> This is **research-heavy**, not code-heavy. The Go code already
> exposes both fields end-to-end (`pkg/cache/cache.go`,
> `pkg/engine/finalize.go`, `pkg/engine/output.go`,
> `ui/src/lib/api-types.ts`, `ui/src/components/feed-detail/section-specs.tsx`).
> All you need to do is fill in the YAML.

---

## TL;DR

- **177 sources** total in `configs/firehol.yaml`
- **4 sources** already have `license:` set (the ASN providers — caida, dbip, iptoasn, maxmind)
- **173 sources** are missing `license:` and need research
- **3 sources** have `attribution:` set; the rest are unknown
- For each one: read the maintainer's homepage / source URL / GitHub
  README and find the license. If you can't find one, mark it as
  "Unknown — see {url}" so a human can follow up later. **Never make
  one up.**

---

## Where the YAML lives

`configs/firehol.yaml` → top-level `sources:` block → one entry per
feed, keyed by feed name.

Example of a source with the fields you need to add:

```yaml
sources:
  abuseipdb_1d:
    url: https://raw.githubusercontent.com/borestad/blocklist-abuseipdb/main/abuseipdb-s100-1d.ipv4
    frequency: 1440
    ipv: ipv4
    output: ip
    processor:
      - remove_comments
    processor_raw: remove_comments
    category: abuse
    info: '[AbuseIPDB](https://www.abuseipdb.com/) aggregated blocklist of IPs with ~100% abuse confidence score, reported in the last 1 day. Aggregated by [borestad](https://github.com/borestad/blocklist-abuseipdb).'
    maintainer: AbuseIPDB / borestad
    maintainer_url: https://github.com/borestad/blocklist-abuseipdb
    enabled_by_all: true
    redistributable: true
    # ↓ ADD THESE TWO LINES:
    license: <the license you found, e.g. "MIT" or "CC BY 4.0">
    attribution: <required attribution string, OR omit if not required>
```

The four already-populated sources to use as **templates**:

```yaml
caida_prefix2as:
  license: CAIDA Acceptable Use Agreement
  attribution: The CAIDA UCSD Routeviews Prefix-to-AS mappings (pfx2as), https://catalog.caida.org/dataset/routeviews_prefix_to_as_mappings

dbip_asn_lite:
  license: CC BY 4.0
  attribution: IP Geolocation by DB-IP (https://db-ip.com)

iptoasn:
  license: PDDL v1.0 (Public Domain)

maxmind_geolite2_asn:
  license: GeoLite2 EULA + CC BY-SA 4.0
  attribution: This product includes GeoLite Data created by MaxMind, available from https://www.maxmind.com
```

---

## How to find the license for a feed

For each source, you have these starting points already in the YAML:

1. **`maintainer_url`** — usually the project homepage. Look for a
   LICENSE link, a "Terms" page, a footer, or a README section
   labelled "License" or "Terms".
2. **`url`** — the actual download URL of the IP list. If it's on
   GitHub, the LICENSE / LICENSE.md / COPYING file in the repo root
   is canonical.
3. **`info`** — the human-readable description. Sometimes mentions
   the license inline (e.g. "released under CC BY 4.0").
4. **`source`** / **`public_url`** — alternate URL forms, may point
   somewhere else worth checking.

**Order to check**, in priority:

1. `LICENSE`, `LICENSE.md`, `LICENSE.txt`, `COPYING` in the repo root
   (GitHub auto-detects and shows it in the sidebar — look for the
   "About" panel on the repo homepage).
2. The repo `README.md` — search for "license", "terms", "use",
   "copyright", "redistribut".
3. The maintainer's homepage — often a Terms / Legal / About page.
4. The list file itself — comment headers sometimes carry license
   text (e.g. `# License: CC0 …`).
5. Known reputable sources where the license is project-wide (e.g.
   anything from `firehol/blocklist-ipsets` follows that repo's
   license; anything from a known security org may have a published
   data policy).
6. **As a last resort**, check the Wayback Machine snapshot of the
   maintainer's homepage if the live site is down.

If after **all** of those the license is genuinely not stated, set:

```yaml
license: Unknown — no license stated at <maintainer_url>
```

This is a **research finding**, not a guess. A future curator will
follow up.

---

## What to write in the `license` field

Write a **short, recognisable** name. Examples:

| Type | Write this |
|---|---|
| MIT | `MIT` |
| Apache 2.0 | `Apache-2.0` |
| GPL v3 | `GPL-3.0` |
| Creative Commons Attribution | `CC BY 4.0` |
| Creative Commons Attribution-ShareAlike | `CC BY-SA 4.0` |
| Creative Commons Zero / Public Domain Dedication | `CC0 1.0` |
| Open Data Commons Public Domain Dedication | `PDDL v1.0 (Public Domain)` |
| Open Data Commons Attribution | `ODC-BY 1.0` |
| BSD 2-clause | `BSD-2-Clause` |
| BSD 3-clause | `BSD-3-Clause` |
| Custom EULA | `<Project Name> EULA — see <url>` |
| Public domain (US gov, etc) | `Public Domain` |
| Unspecified | `Unknown — no license stated at <maintainer_url>` |

If the license has **multiple parts** (e.g. data under one license,
metadata under another), write both joined with `+`:

```yaml
license: GeoLite2 EULA + CC BY-SA 4.0
```

---

## When to add `attribution`

Add an `attribution:` field **only** when the license **requires**
attribution and the maintainer specifies the exact wording. Examples:

- CC BY / CC BY-SA require attribution → add it
- MIT / Apache require notice preservation → not the same thing,
  generally **don't** add an attribution string
- GPL → not attribution per se, **don't** add
- Public domain / CC0 → no attribution required, **don't** add
- Custom EULAs that demand a specific credit line → add the exact
  line they require

The string should be **the exact text the maintainer asks you to
display**. Don't paraphrase. If the maintainer says "this product
includes data from X" verbatim, write that.

---

## When to flip `redistributable`

If the license **forbids** raw redistribution of the data (it allows
derived analysis but not republishing the IP list), also set:

```yaml
redistributable: false
```

This causes the build pipeline to **not** publish the raw `.ipset` /
`.netset` file in the public mirror. The feed page on the public
site still shows analysis, comparisons, retention, etc. — those are
derived works, which most "no redistribution" licenses allow.

Currently only `caida_prefix2as` has `redistributable: false` set,
because CAIDA explicitly forbids redistributing the raw routing
tables but allows publishing derived statistics with attribution.
Apply the same pattern wherever you find similar terms.

---

## Working order

The 173 sources fall into a few natural buckets. Processing them in
this order will be the most efficient:

### Bucket 1: same maintainer, multiple feeds (do them together)

These maintainers publish several variants of the same list. Find
the license **once**, apply it to all variants in one pass:

- **abuseipdb_*** (4 feeds: `abuseipdb_1d`, `abuseipdb_30d`, etc) —
  one maintainer (`borestad`)
- **php_bad_*** (4 feeds) — same maintainer
- **bi_*** (many feeds — bi_apache-phpmyadmin_*, bi_unknown_*, …) —
  Binary Defense / borestad
- **bm_*** (many feeds) — botscout
- **bl_*** (many feeds) — blocklist.de
- **cleantalk_*** (multiple variants)
- **firehol_level1** through **firehol_level4** — these are merge
  feeds, see "merges" block separately
- **ipsum_2** through **ipsum_8** — single project
- **ri_connect_*** (many feeds) — same project
- **bambenek_*** — bambenek consulting
- **et_*** (emerging threats) — proofpoint
- **sblam_*** — sblam.com
- **stopforumspam_*** — stopforumspam
- **xroxy_*** — xroxy

### Bucket 2: well-known reputable sources (license is usually obvious)

These have published data policies you can find quickly:

- **spamhaus_drop**, **spamhaus_edrop** — Spamhaus DROP terms
- **dshield**, **dshield_top_*** — SANS ISC
- **maxmind_geolite2** (already done — use as template)
- **google_***, **cloudflare_*** — corporate published lists
- **bitcoin_nodes** — getaddr.bitnodes.io
- **tor_exits**, **dm_tor**, **bm_tor** — Tor project
- **firehol_*** merges — derived from sources, see "Special cases"

### Bucket 3: GitHub-hosted lists

The maintainer URL points at a GitHub repo. Easiest research path:

1. Visit the `maintainer_url`
2. Look at the repo's "About" sidebar — GitHub auto-detects the
   LICENSE file and shows the license name
3. If shown, that's the answer
4. If not shown, open the LICENSE file directly
5. If no LICENSE file, search the README

### Bucket 4: One-off lists

Everything else. Process individually using the search order above.

---

## Special cases

### `merge:` feeds (not in `sources:` but in the `merges:` block)

These are composite lists assembled from other feeds (firehol_level1
through firehol_level4, plus a few others). Their license is the
**most restrictive** of their inputs. Don't research them
independently — once all the input sources have licenses, derive the
merge license by hand:

- If any input is "no redistribution" → the merge is too
- If any input requires attribution → the merge needs it (combined)
- If all inputs are public-domain compatible → the merge is too

Note this in the merge entry's `license:` field with a brief
explanation, e.g.

```yaml
merges:
  firehol_level1:
    license: Mixed (most restrictive of: spamhaus_drop, dshield, ...)
    attribution: Composed from multiple feeds — see component pages for individual attributions.
```

### Sources marked `hidden: true`

These are bogon contributors and other internal-use sources. They
still need licenses since the data flows through the pipeline, but
they don't render their own user-facing page. Process them, but
they're lower priority than the public feeds.

### Already-licensed sources (skip these — they're done)

```
caida_prefix2as
dbip_asn_lite
iptoasn
maxmind_geolite2_asn
```

---

## Output format expected

Two deliverables when the task is finished:

1. **A modified `configs/firehol.yaml`** with `license:` added to
   every source that's missing one, and `attribution:` added where
   required. Keep the YAML structure exactly as-is — only add the
   two new fields per source. Preserve the field ordering of the
   existing `caida_prefix2as` entry as a template.

2. **A summary report** at `TODO-ip-feeds-licenses-RESULTS.md`
   listing:
   - Total sources processed
   - Number with confirmed licenses (broken down by license type)
   - Number marked "Unknown — …"
   - Number that needed `redistributable: false` set
   - **Audit log**: for each source, one line with the source name,
     the license you assigned, and the URL where you found it. This
     lets a human spot-check any entry without re-doing the research.

   Example:
   ```
   abuseipdb_1d   | MIT      | https://github.com/borestad/blocklist-abuseipdb/blob/main/LICENSE
   bds_atif       | Custom — Binary Defense ToS | https://www.binarydefense.com/terms-of-service/
   spamhaus_drop  | Spamhaus DROP terms | https://www.spamhaus.org/drop/
   ...
   ```

---

## Hard rules

1. **NEVER guess a license.** If you can't find one, write
   `Unknown — no license stated at <url>`. A wrong license is worse
   than no license — it could mislead users into illegal use.
2. **NEVER mark a feed `redistributable: false` unless the maintainer
   explicitly forbids redistribution.** The default is `true` and
   that's correct for most open data.
3. **NEVER edit `info`, `maintainer`, or any other field**. Only add
   `license:` and (where applicable) `attribution:` and (where
   applicable) flip `redistributable:` to `false`.
4. **Quote license strings the maintainer's exact wording when
   custom**. Don't paraphrase "we ask that you cite us" into
   "Attribution required".
5. **Test your YAML edits** with `go test ./pkg/config/...` after
   each batch — there's a config validation test that will catch
   broken YAML.
6. **Commit in batches** of ~20-30 sources at a time so the diff
   stays reviewable. Use commit messages like
   `Add licenses for AbuseIPDB / Binary Defense / Blocklist.de feeds`.

---

## Verification when finished

After populating the YAML:

```bash
# 1. Config still parses
go test ./pkg/config/...

# 2. The full test suite still passes
go test ./...

# 3. Deploy and check a specific feed in the API
./install.sh
curl -s http://localhost:18888/api/v1/sets/abuseipdb_1d | python3 -c \
  "import sys, json; d = json.load(sys.stdin); print('license:', d.get('license')); print('attribution:', d.get('attribution'))"

# 4. Open a feed page in the browser, check the Identification
# group of the Specs section — license row should show the value
# you added, not "—".
```

---

## Reference: existing fields and their meaning

For context — these are the fields **already** in each source entry.
You should NOT touch them, but knowing what they are helps you find
licenses faster:

| Field | Purpose |
|---|---|
| `url` | Where to download the raw IP list |
| `frequency` | Update interval in minutes |
| `ipv` | `ipv4` / `ipv6` |
| `output` | `ip` / `net` / `both` — the kind of entries this feed produces |
| `processor` / `processor_raw` | Pipeline steps to parse the raw download |
| `category` | One of: attacks / abuse / malware / spam / reputation / anonymizers / organizations / unroutable / geolocation / asn |
| `info` | Markdown human description with embedded link to the maintainer |
| `maintainer` | Display name of the org / person |
| `maintainer_url` | Homepage of the maintainer |
| `enabled_by_all` | Whether `--enable-all` turns this source on |
| `redistributable` | `true` (default) — raw file is published in the mirror; `false` — only derived stats |
| `history` | Optional history retention windows (minutes) for time-windowed variants |
| `use` | Engine role (`asn`, `geoip`, `bogons`) — only for special-purpose sources |
| `format` | Parser type for non-text inputs (mmdb, tar.gz, …) |
| `hidden` | Don't render a public page for this source |
| `label` | Pretty display name (overrides the source key in tab labels) |

---

## Estimated effort

- **Bucket 1 (same-maintainer groups)**: ~10 maintainer groups × 5 min
  research each = ~1 hour. Covers ~80 sources.
- **Bucket 2 (well-known sources)**: ~15 sources × 3 min each = ~45 min.
- **Bucket 3 (GitHub-hosted)**: ~50 sources × 1 min each (GitHub's
  auto-detected license badge is fast) = ~50 min.
- **Bucket 4 (one-offs)**: ~30 sources × 5 min each = ~2.5 hours.
- **Merge feeds + summary report**: ~30 min.

**Total**: ~5-6 hours of focused web research for a thorough job.
Don't rush — the value is correctness, not throughput.
