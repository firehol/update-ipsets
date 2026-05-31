# Bitwire Feed Audit TODO

## TL;DR

user asked to:

- Fix existing Bitwire catalog entries:
  - `license: CC BY-NC-SA 4.0`
  - change output from `ipset` to `netset` if the feeds contain CIDR ranges.
- Check all upstream feeds used by `https://github.com/bitwire-it/ipblocklist` for validity and freshness.
- Add every maintained feed that is missing from this catalog.
- Audit all configured IP feeds for the same `ipset` / `netset` mistake and fix any wrong entries.

## Purpose

Improve `iplists.firehol.org` as a factual comparison site for public IP feeds. The consumer impact is:

- Users see maintained Bitwire upstream feeds individually, not only through Bitwire's aggregate.
- CIDR feeds are published as `netset`, preserving network blocks instead of expanding them into individual IPs.
- License metadata shown to downstream consumers is accurate.

## Analysis

Facts verified before implementation:

- The worktree was clean before this task (`git status --short` returned no output).
- Existing Bitwire entries are in `configs/firehol.yaml`:
  - `bitwire_inbound` around line 2393.
  - `bitwire_outbound` around line 2407.
- `pkg/config/catalog_verify_test.go` includes both Bitwire sources in the expected source list around line 548.
- Bitwire's README states:
  - The aggregate lists are updated every 2 hours.
  - Code is MIT.
  - Aggregated data is licensed as CC BY-NC-SA 4.0.
- Bitwire's workflow `.github/workflows/update.yml` uses `cron: '0 */2 * * *'`.
- Bitwire's latest stats were current during inspection:
  - Timestamp: `2026-04-10T01:13:19.843082+00:00`.
  - Inbound: `3,413,819` optimized entries.
  - Outbound: `193,900` optimized entries.
- Bitwire's raw `inbound.txt` and `outbound.txt` contain CIDR ranges (`/31`, `/30`, `/29`, `/20`, `/16`, etc.).
- The engine renders output types in `pkg/engine/finalize.go`:
  - `ipset` -> `iprange.PrintSingleIPs`.
  - `netset` -> `iprange.PrintCIDR`.
- `pkg/iprange/print.go` expands `PrintSingleIPs` into individual addresses, so using `ipset` for CIDR-heavy feeds can produce unnecessarily large outputs.
- Bitwire has 86 unique upstream URLs across its inbound and outbound source tables.
- Some Bitwire URLs are alternate mirrors of feeds already tracked directly, especially `borestad/firehol-mirror` URLs. These are not automatically new feeds.
- Maintained missing Bitwire upstream feeds selected for addition:
  - `abuseipdb_3d`
  - `abuseipdb_7d`
  - `cinsarmy`
  - `bitwire_ipsum_clean`
  - `bitwire_iplistfetch_blacklist`
  - `bitwire_iplistfetch_blacklist2`
  - `opendbl_bruteforce`
  - `shadowwhisperer_scanners`
  - `shadowwhisperer_tunnel`
  - `shadowwhisperer_hackers`
  - `shadowwhisperer_hosting`
  - `shadowwhisperer_bruteforce_medium`
  - `shadowwhisperer_bruteforce_high`
  - `shadowwhisperer_bruteforce_extreme`
  - `romainmarcoux_malicious_aa`
  - `romainmarcoux_malicious_ab`
  - `romainmarcoux_malicious_ac`
  - `romainmarcoux_malicious_ad`
  - `romainmarcoux_outgoing_aa`
  - `romainmarcoux_outgoing_ab`
  - `threatfox_ips`
  - `criticalpath_log4j`
  - `criticalpath_cobaltstrike`
  - `criticalpath_sip`
  - `blackmirror_ipv4`
  - `malwarefilter_botnet`
  - `ustc_blackip`
  - `nginx_bad_bot_blocker`
  - `vxvault_url_list`
- Bitwire upstreams excluded from direct addition after audit:
  - Empty / zero-entry during audit: `darklist.de/raw.php`, `multiproxy.org/txt_all/proxy.txt`, `romainmarcoux/malicious-ip/full-ae.txt`.
  - Stale during audit: `SecOps-Institute/Tor-IP-Addresses`, `secureupdates.checkpoint.com/IP-list/TOR.txt`, `reputation.alienvault.com/reputation.generic`.
  - Alternate mirrors or same-data variants already tracked: `borestad/firehol-mirror` feeds, CriticalPath duplicates of Emerging Threats / BinaryDefense / Rutgers / Tor data, `myip.ms` htaccess format variant.
- Output audit findings:
  - `bitwire_inbound` and `bitwire_outbound` preserve CIDR ranges and were changed from `ipset` to `netset`.
  - 19 MISP scanner warninglists use `extract_ipv4_cidr` but were still `ipset`; all were changed to `netset`.
  - `iblocklist_proxies` uses `p2p_gz_proxy` / `p2p_blocklist_proxy`, which emits IP ranges, and was changed from `ipset` to `netset`.
  - A static processor audit after the fixes reports `ipset_with_cidr_preserving_processor 0`.

## Decisions

User decisions already made:

1. Fix Bitwire license metadata to `CC BY-NC-SA 4.0`.
2. Fix Bitwire output to `netset` if the audit confirms CIDRs.
3. Add all maintained Bitwire upstream feeds missing from this catalog.
4. Audit all configured IP feeds for `ipset` / `netset` mistakes.

Pending decisions:

- None currently. If the audit finds a feed that is maintained but semantically ambiguous or unsafe to expose as a public source, pause and present evidence/options.

## Plan

1. Build a Bitwire upstream inventory from:
   - `tables/inbound/urltable_inbound`
   - `tables/outbound/urltable_outbound`
   - `stats/latest.json`
2. Classify every Bitwire upstream URL:
   - Already tracked exactly.
   - Already covered through a direct upstream equivalent.
   - Already covered through generated retention or FireHOL merge entries.
   - Missing and maintained.
   - Missing but dead, empty, stale, or not IPv4/network-blocking relevant.
3. Add missing maintained feeds to `configs/firehol.yaml`.
4. For split upstream feeds, use the least misleading catalog shape:
   - Prefer one public logical feed only when the source is clearly one feed split into transport chunks.
   - Avoid showing arbitrary chunks as independent feeds unless there is no better supported catalog shape.
5. Audit all existing source outputs:
   - If processed output preserves network prefixes, use `netset`.
   - If processed output emits only individual IPs, use `ipset`.
6. Update tests:
   - Source count in `TestCatalogExpectedCounts`.
   - Expected source names in `TestCatalogSourcesComplete`.
   - Redistributable consistency list only if a newly added feed explicitly forbids redistribution.
7. Run verification:
   - `go test ./pkg/config/...`
   - `go test ./pkg/processor/...`
   - Broader `go test ./...` if time/runtime is reasonable.

## Implied Decisions

- Do not add duplicate mirror URLs when a Bitwire upstream is only a mirror of an existing source.
- Do not mark non-commercial licenses as `redistributable: false`; AGENTS.md says non-commercial terms are not redistribution prohibitions for this community pass-through site.
- Prefer `netset` for CIDR-preserving feeds to avoid expanding CIDRs into individual IPs.
- Keep changes scoped to catalog/config/test updates unless the audit proves engine or processor behavior is wrong.

## Testing Requirements

- Catalog must load successfully through `config.LoadYAML`.
- Catalog source count and expected source list tests must be updated in lockstep with additions.
- Processor registry consistency must still pass for all new processor chains.
- Any changed `output` values must remain valid (`ipset` or `netset` only).

## Verification Status

- `go test ./pkg/config/...` passed.
- `go test ./pkg/processor/...` passed.
- `go test ./...` passed.
- `go build ./...` passed.
- `go vet ./...` passed.
- `go test -race ./...` passed.

## Documentation Updates Required

- Update `AGENTS.md` only if this task discovers a durable developer rule or workflow change.
- Feed catalog entries themselves are the user-facing documentation for each new source (`license`, `attribution`, `info`, `maintainer`, `maintainer_url`, `category`).
