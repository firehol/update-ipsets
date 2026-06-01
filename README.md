# update-ipsets

**See what's actually inside a public IP blocklist — before you trust it to block, alert, or score.**

Every feed maintainer claims their list is fresh, accurate, and comprehensive. There is no
neutral way to check. update-ipsets tracks **342 public IP threat, blocking, and reference
feeds**, refreshes them continuously, keeps years of history, and measures every public feed the
same way — so you can see the facts that actually decide whether a feed is worth using.

It reports measurements, never opinions. It will never tell you which feed is "best." It hands
you the evidence and you decide.

🌐 **Live:** [iplists.firehol.org](https://iplists.firehol.org)

| | |
|---|---|
| **Project** | [FireHOL](https://firehol.org) — community-maintained, open source |
| **License** | GNU GPL v2 |
| **Language** | Go (a rewrite of the original FireHOL bash `update-ipsets`) |
| **Catalog** | 342 public feeds: 329 source feeds + 13 curated merges · 11 categories · tracked since 2015 |
| **Status** | IPv4 feed pipeline fully implemented · `iprange` CLI supports IPv6 |

---

## What you learn about any feed

### Will it block legitimate traffic? *(the costliest mistake)*

Blocking something real is an outage. update-ipsets does **not** promise a false-positive rate —
nobody honestly can without ground truth — but it shows you the risk directly:

- **overlap with critical internet infrastructure** (public DNS, major clouds and CDNs) you'd
  likely regret blocking — *e.g. a small bot feed where ~30% of entries fall in Cloudflare ranges*
- **how much of the feed is bogon / reserved / unroutable space** that should never be in a
  blocklist — *some "large" feeds are mostly empty address space*
- **how long IPs stay listed** — stale entries are a top source of false positives
- **how strongly the feed agrees with others** — an IP confirmed by many independent feeds is
  safer to act on than one listed alone

### How fresh is it, really?

Measured from the feed's **actual content changes**, not the maintainer's claim. You see the real
update cadence and how violently the list churns between updates *(some feeds replace most of
their entries every single update)*.

### How long are IPs listed? *(retention)*

Age distributions for IPs still listed **and** for IPs already removed — so you can tell whether a
feed expires entries or holds them forever.

### What does it add that others don't? *(comparison)*

Pairwise overlap against every other feed, and how unique its IPs are. Tells you whether a feed is
worth adding or just duplicates coverage you already have.

### Where does it cover?

Per-country and per-ASN breakdown of the feed's address space.

### Can I legally use it?

Each feed's license, and whether it may be redistributed or used for automated blocking.

### Is it even alive?

A health signal — healthy, delayed, archived, unmaintained — plus explicit **discontinued**
detection when a source has quietly frozen and stopped changing.

### Who runs it, and how do I get off it?

Researched provenance for every feed: the maintainer, how the feed is built (honeypot, sandbox,
community reports, manual curation), what gets an IP **listed**, what gets it **unlisted**, and the
exact removal channel.

---

## What it deliberately does **not** do

- **It does not rank feeds.** No "best feed," no score of feed against feed. Evidence only.
- **It is not a per-IP threat database.** It tells you which lists contain an IP and how good
  those lists are. It does **not** attribute malware families, campaigns, or actors to individual
  IPs.
- **It does not invent a false-positive number.** It surfaces the risk signals; the judgment is
  yours.
- **The shipped feed pipeline is IPv4-oriented.** Public feed lookup, enrichment, and
  critical-infrastructure overlap are IPv4-only in this release. The bundled `iprange` CLI
  supports IPv6 set operations.

This restraint is the point. The data is trustworthy precisely because the software refuses to
editorialize.

---

## How to get the data

- **Website** — [iplists.firehol.org](https://iplists.firehol.org): explore the catalog, open a
  per-feed analysis, compare any two feeds, and search an IP across all lists. Every computed
  number links to a methodology page explaining how it is calculated.
- **REST API** — list and inspect feeds, history (CSV), comparison, retention, country/ASN
  breakdowns, IP search, and compose sets on the fly. Plain text, CSV, and JSON — built to drop
  straight into firewalls, SIEMs, threat-intel platforms, cloud security groups, and scripts. See
  [`docs/api/`](docs/api/).
- **MCP server** — `https://iplists.firehol.org/mcp`. Point an AI agent at it: `find_feeds`
  (filter by freshness, uniqueness, health, license, redistributability, critical-infrastructure
  tier, category, maintainer, size…) and `fetch_analysis` (the full per-feed page). See
  [`docs/api/mcp-endpoint.md`](docs/api/mcp-endpoint.md).
- **Downloadable sets** — every feed as a normalized `ipset`/`netset`, plus curated merges
  including the well-known FireHOL `level1`–`level4` blocklists.

Categories tracked: `anonymizers` · `scanners` · `intrusion` · `malware_infrastructure` ·
`messaging_abuse` · `service_abuse` · `policy_risk` · `provider_infrastructure` · `special_use` ·
`asn` · `geolocation`.

---

## Run your own instance

It is a single Go binary plus a YAML catalog. `./install.sh` builds it, installs the catalog, and
sets up the systemd service. The `daemon` serves the website, the REST API, the MCP endpoint, and
an admin UI in one process; it applies kernel ipsets natively when run as root on Linux.

It is built to run a full, long-lived collection pipeline on modest hardware — it handles IP sets
larger than available RAM by working file-backed and streaming instead of loading everything into
memory. Operators get full visibility through the admin UI: download and processing queues, feed
status, integrity checks, and manual recheck/reprocess controls.

Builds require Go and `pnpm`; the web UI is embedded into the binary.

```bash
make build        # build the binary
make test         # run tests
./install.sh      # build, install catalog, set up the service
```

**Also a command-line toolkit.** Beyond the daemon, the same binary is a standalone CLI:
`iprange` (a FireHOL `iprange`-compatible set tool — CIDR / range / IP math, compare, diff,
intersect, prefix reduction; IPv4 and IPv6), `query` (which lists contain an IP, or compose sets
with `set1 + set2 - set3`), `enable` (enable or disable sources), and `cache-merge` (migration
cache helper).

See [`docs/`](docs/) — [quick-start](docs/quick-start.md),
[installation](docs/installation/), [api](docs/api/), [feeds](docs/feeds/),
[pipeline](docs/pipeline/), [admin UI](docs/admin-ui/), and
[migrating from the bash version](docs/migration-from-bash.md).

---

## License

GNU General Public License v2 — see [`COPYING`](COPYING).

Part of the [FireHOL](https://firehol.org) project. Free and open source — a gift to the community.
There is no company behind it.
