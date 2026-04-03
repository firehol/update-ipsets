# Add new IP-to-ASN providers

## TL;DR

Add three new ASN providers to update-ipsets alongside the existing MaxMind GeoLite2-ASN:

1. **iptoasn.com** (public domain, hourly, ~519k IPv4 records) - new TSV decoder
2. **DB-IP Lite ASN** (CC BY 4.0, monthly, ~467k records) - reuse MMDB decoder, monthly URL via `{YYYY}-{MM}` template
3. **CAIDA prefix2as** (CAIDA AUA, daily, ~1.09M records) - new prefix2as text decoder, marked non-redistributable

## Verified facts (from research and live downloads)

- iptoasn v4 TSV (verified 2026-04-06): 519,165 lines, columns `start \t end \t asn \t country \t description`. AS=0 means "Not routed". Public domain.
- DB-IP asn-lite MMDB (verified 2026-04-06): URL `https://download.db-ip.com/free/dbip-asn-lite-2026-04.mmdb.gz`, 5,119,622 bytes, MMDB schema identical to MaxMind (`autonomous_system_number`, `autonomous_system_organization`).
- CAIDA prefix2as (verified 2026-04-06): URL `https://publicdata.caida.org/datasets/routing/routeviews-prefix2as/2026/04/routeviews-rv2-20260405-1200.pfx2as.gz`, 1,085,731 records, columns `prefix \t prefix_length \t asn(_asn_asn for MOAS)`. MOAS uses underscore separator (NOT comma as the task description claimed - verified live).
- CAIDA AUA (verified at https://www.caida.org/about/legal/aua/public_aua/): silent on raw data redistribution but explicitly permits derived publications with citation. So we will mark CAIDA non-redistributable for the raw download AND publish derived statistics with proper attribution.

## Design decisions made

### 1. Decoder abstraction in pkg/asnloc

Currently `asnloc.Database` wraps a `*maxminddb.Reader` directly. To support TSV (iptoasn) and prefix2as (CAIDA) which are text formats requiring an in-memory range table, I'll refactor `Database` to be backed by an interface:

```go
type backend interface {
    Lookup(ipv4 uint32) (Record, Network, error)
    Stats() (networks int, ipv4Covered uint64, err error)
    Close() error
}
```

Two implementations:
- `mmdbBackend` wraps `*maxminddb.Reader` + a `decoderFunc` (used by `maxmind_geolite2_asn_mmdb` and `dbip_asn_lite_mmdb`)
- `rangeTableBackend` wraps an in-memory sorted []range slice (used by `iptoasn_combined_tsv` and `caida_prefix2as`)

The `Open(providerType, path)` constructor dispatches by provider type. Public methods `Lookup`, `Stats`, `CountFeed`, `CountFeedWithBogons` are unchanged externally.

### 2. URL handling for monthly/daily providers

- **DB-IP**: use the existing `{YYYY}-{MM}` template variables (already supported by `expandTemplate`). Same pattern is already used for `dbip-country-lite`.
- **CAIDA**: too risky to template `{YYYY}{MM}{DD}` because the daily file isn't always ready. Add a per-feed type "URL resolver" step in `processASNFeeds` that, for `caida_prefix2as`, fetches the creation log first and substitutes the latest filename. Other types use the URL as-is.

### 3. Non-redistributable flag for ASN providers

The existing `Source.Redistributable` flag controls whether per-source files are git-ignored on the public repo. ASN providers also produce a downloaded file in `${LibDir}/asn/<name>/source` which is NOT in the public output dir, so it's already not pushed to the blocklist-ipsets repo. The derived `<feed>_asn_<provider>.json` files ARE in the public output dir.

For CAIDA we want:
- The raw download stays local only (already the case)
- The derived `<feed>_asn_caida.json` files MUST still be published (per AUA, derived publications are allowed with citation)
- Add a `redistributable` field to `ASNFeed` struct (default true), purely informational (used by methodology page generation and `ASNProvider` API output). The raw archive is already not published anywhere.

### 4. Attribution

Update `pkg/web/static/methodology/asn-attribution.md` to list all four providers with their attribution requirements:
- MaxMind GeoLite2 ASN: CC BY-SA 4.0 with attribution
- iptoasn.com: PDDL/Public Domain (no attribution required, but credit Frank Denis)
- DB-IP Lite ASN: CC BY 4.0 - "IP Geolocation by DB-IP" attribution
- CAIDA prefix2as: AUA - cite "The CAIDA UCSD Routeviews Prefix-to-AS mappings (pfx2as) - [date], https://catalog.caida.org/dataset/routeviews_prefix_to_as_mappings"

## Implementation plan

1. Refactor `pkg/asnloc/asnloc.go` to use a backend interface
2. Add `mmdbBackend` (wraps existing `decoderFunc` logic)
3. Add `rangeTableBackend` with sorted-range binary search lookup
4. Add `iptoasnTSVLoader` that reads gzipped TSV and builds a range table
5. Add `caidaPrefix2asLoader` that reads gzipped prefix2as text and builds a range table
6. Register provider types: `iptoasn_combined_tsv`, `dbip_asn_lite_mmdb`, `caida_prefix2as`
7. Update `pkg/engine/asn.go`:
   - Add per-type post-download "extract" step:
     - MMDB types: existing `extractMMDBFromArchive` for tar.gz, plus a new `decompressGzipMMDB` for `.mmdb.gz` (DB-IP)
     - TSV/prefix2as: gunzip the archive into a plain text file at a known path
   - Add per-type URL resolver step (only used by CAIDA today)
   - The `Open` call passes the resolved data path (mmdb file or text file)
8. Add `Redistributable bool` field to `ASNFeed` (default true via YAML omission)
9. Surface the redistributable flag in the `ASNProvider` API output
10. Update `configs/firehol.yaml` to add the three new providers
11. Add tests for the new parsers (TSV and prefix2as)
12. Update methodology page with provider attribution table
13. Build + test

## Tests to add

- `TestParseIPToASNTSV` with sample input covering: normal lines, AS=0 "Not routed" lines, comments, empty lines, malformed lines, IPv6 line that should be skipped (combined file has both)
- `TestParseCAIDAPrefix2AS` with sample input covering: normal /24, multi-origin (e.g. `1.0.4.0 22 56203_38803`), edge prefixes (/0, /32), invalid lines
- `TestRangeTableLookup` to verify the binary-search lookup returns correct (ASN, network) for hits and 0/single-IP for misses
- Keep the existing MMDB tests intact

## What NOT to do (constraints from task)

- Do NOT touch `pkg/engine/bogons.go` or anything in the bogon code path (commit 7498233 just landed)
- Do NOT touch the frontend (`pkg/web/static/*` except the methodology markdown)
- Do NOT reformat unrelated code
- No commit messages mentioning AI tools

## Status

- [x] TODO file written
- [x] Refactor asnloc backend interface
- [x] iptoasn TSV loader + tests
- [x] CAIDA prefix2as loader + tests
- [x] DB-IP MMDB support (gzip decompress, register type)
- [x] Engine: per-type extract + URL resolver
- [x] Config: ASNFeed Redistributable field, ASNProvider API
- [x] firehol.yaml: add three new providers
- [x] Methodology page: provider attribution
- [x] go build ./... passes
- [x] go test ./... passes
- [x] Functional verification with real iptoasn (387k merged networks, 3.11B IPs covered)
- [x] Functional verification with real CAIDA (278k merged networks, 3.10B IPs covered)
- [ ] Commit (pending Costa approval)

## Functional verification results

- **iptoasn v4 TSV** (519,165 raw lines → 387,137 merged ranges, 3.11B IPv4 covered)
  - 1.1.1.1 → AS13335 CLOUDFLARENET
  - 8.8.8.8 → AS15169 GOOGLE
  - 104.244.36.255 → miss; gap [104.244.33.0..104.244.39.255] (matches the AS=0 "Not routed" gap in the source)
- **CAIDA prefix2as** (1,085,731 raw lines → 278,544 merged ranges, 3.10B IPv4 covered)
  - 1.1.1.1 → AS13335 (origin AS)
  - 8.8.8.8 → AS3356 (Level 3 — the upstream that announces the prefix; Google is the actual operator). Different from iptoasn's AS15169 — exactly the kind of cross-validation difference Costa wants visible.
- **YAML round-trip**: all four providers load with correct redistributable flag (caida_prefix2as is the only one set to false)
