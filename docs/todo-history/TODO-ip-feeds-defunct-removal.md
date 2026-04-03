# TODO — Verify and remove defunct IP feeds from configs/firehol.yaml

## TL;DR

Costa asked to verify all feeds the licenses-research flagged as defunct, then
delete the confirmed-dead ones from `configs/firehol.yaml`. This is a focused
cleanup task — no license fields are touched here.

## Task scope

Per the licenses research files (TODO-ip-feeds-licenses-RESEARCH-*.md), these
feeds were flagged as defunct or dead:

| Feed | Reason flagged | In config? | In merges? |
|---|---|---|---|
| `bm_tor` | torstatus.blutmagie.de status unknown | line 251 | `firehol_anonymous` |
| `bruteforceblocker` | conflicting reports (one said WORKING 503 IPs) | line 326 | `firehol_level3` |
| `cta_cryptowall` | URL returns 404 (Cyber Threat Alliance discontinued dashboard) | line 448 | no |
| `darklist_de` | site up but list is empty (only headers) | line 490 | no |
| `dronebl_*` (9 feeds) | DNSBL-only, **no `url:` field in config** — never downloaded | lines 659–757 | no |
| `graphiclineweb` | WordPress URL unreachable | line 930 | no |
| `proxyrss` | original site repurposed; `_30d` variant in `firehol_proxies` merge | line 2093 | `firehol_proxies` (via `proxyrss_30d`) |
| `ri_connect_proxies`, `ri_web_proxies` | RosInstrument URLs return HTTP 404 | lines 2127, 2145 | `firehol_proxies` (via `_30d` variants) |
| `sorbs_*` (9 feeds) | Proofpoint decommissioned SORBS late 2024; **no `url:` field in config** | lines 2195–2293 | no |

Stale `renames:` entries pointing to either non-existent sources or
about-to-be-deleted sources:
- `autoshun: shunlist` (line 2730) — `shunlist` doesn't exist as a source
- `clean_mx_viruses: cleanmx_viruses` (line 2732) — `cleanmx_viruses` doesn't exist
- `rosi_connect_proxies: ri_connect_proxies` (line 2799) — target will be deleted
- `rosi_web_proxies: ri_web_proxies` (line 2800) — target will be deleted
- `tor_servers: bm_tor` (line 2809) — target may be deleted

## Process

1. **Verification phase** (run in parallel via subagent):
   - For each candidate with a `url:` in the config, do a live HTTP fetch.
   - Report: HTTP status, content length, last-modified header, sample of body
     (first 30 lines), how many IPv4 addresses are present.
   - For `dronebl_*` and `sorbs_*`: no verification needed — config has no `url:`,
     so they cannot be downloaded by the engine. They are zombie entries.
   - Pay special attention to `bruteforceblocker` (research is contradictory)
     and `darklist_de` (list may be intermittently empty).

2. **Decision phase**:
   - Based on the verification report, decide which feeds are confirmed dead.
   - Anything that returns valid IP data stays.
   - Anything that returns 404, NXDOMAIN, empty list, or HTML error page is deleted.

3. **Deletion phase**:
   - Remove the source entry from `configs/firehol.yaml`.
   - Remove the source from any `merges:` entry that references it (or its
     `_30d` time-window variant).
   - Remove stale `renames:` entries that now point at nothing.
   - Add the source name (and any aliases via renames) to the `deleted:` block,
     keeping alphabetical order.
   - Do NOT touch processor functions in code (`pkg/processor/`) — those are
     code, not config, and harmless if unused.

4. **Verification**:
   - `go test ./pkg/config/...` — config still parses
   - `go test ./...` — full suite still passes
   - Show the exact diff to Costa before declaring done.

## Hard rules (per CLAUDE.md)

- Never `git add -A`. Add only the modified files.
- No commit until Costa approves the diff.
- If verification reveals a feed is actually alive, leave it in.
- If a feed is in a merge, remove it from the merge AT THE SAME TIME — never
  leave a merge referring to a deleted source.

## Status

- [x] Read all 39 license-research files
- [x] Confirm which defunct candidates exist in current YAML and where
- [ ] Run verification subagent (live HTTP tests)
- [ ] Decide deletion list
- [ ] Edit configs/firehol.yaml
- [ ] Run tests
- [ ] Present diff to Costa for approval
