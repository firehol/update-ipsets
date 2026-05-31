# update-ipsets

**A comparative observatory for public IP threat & blocking feeds.**

update-ipsets collects hundreds of public IP-based threat and blocking feeds, normalizes
them into one consistent format, preserves their history, compares them against each
other, and publishes the results — as a public website, a machine-readable API, and
downloadable set files.

> The product value is not any single feed. It is the **comparison**: tracking many
> feeds over time and showing — factually — how they overlap, how fresh they are, how
> they grow, shrink, and change, and where each one is unique.

🌐 **Live observatory:** [iplists.firehol.org](https://iplists.firehol.org)

| | |
|---|---|
| **Project** | [FireHOL](https://firehol.org) — community-maintained, open source |
| **License** | GNU GPL v2 |
| **Language** | Go 1.26 (this is a Go rewrite of the original FireHOL bash `update-ipsets`) |
| **Catalog** | 342 source feeds · 12 merged sets · 11 categories |
| **Status** | IPv4 fully implemented and verified · IPv6 is a stub |

This is open-source software — a gift to the community. There is no company behind it. It
is maintained by the FireHOL project.

---

## The problem it solves

Anyone evaluating IP threat feeds faces a hard information asymmetry. Every feed maintainer
claims broad coverage, low false positives, and fast updates — but there is no independent
way to verify those claims, and no neutral place to compare one feed against another.

In practice, picking feeds means answering questions no single maintainer can answer,
because each can only speak to their own list:

- **Overlap** — does this feed add *unique* coverage, or is it 80% contained in feeds I
  already have? (The top public feeds overlap heavily; the 10th feed usually adds far less
  than the 2nd.)
- **Freshness** — how often does it actually change, not just how often the file is
  regenerated?
- **Retention** — does it keep historical depth, or is it short-term only?
- **Trend** — is it growing, stable, or quietly abandoned?
- **Geography / ASN** — does it cover the regions and networks relevant to me?
- **Size vs. quality** — is it large because it's comprehensive, or bloated with stale
  entries?

update-ipsets answers these with measurements, not opinions. It is built for the people who
make feed decisions: SOC analysts, network and cloud security engineers, threat-intel and
incident-response teams, fraud and email-security teams, security architects, and red teams
checking their own infrastructure.

---

## What it does

- **Collects** live feeds plus supporting datasets (ASN, geolocation, bogon references).
- **Normalizes** every feed into a canonical IP or network set (`ipset` / `netset`).
- **Preserves** historical evidence so change over time is observable.
- **Compares** feeds pairwise, computes retention, and breaks down coverage by country
  and ASN.
- **Enriches** each feed with researched, structured metadata: official source, derivation,
  listing/unlisting policy, detection method, license & redistribution terms, current status.
- **Publishes** the results as a public website, a REST API, and downloadable set files.
- **Gives operators** full visibility into the download and processing pipeline, with
  integrity checks and manual controls.

### Design principles

- **Factual, not editorial.** It reports measurements. It does **not** rank feeds, declare a
  "best" feed, or present heuristics as truth. Users connect the dots.
- **Inclusive catalog.** A source is included if it is publicly reachable, about IP-based
  blocking/abuse/hygiene, alive, and changing — regardless of whether it is small, niche,
  academic, personal, or obscure. Uniqueness matters more than size.
- **Cache-first and cheap to serve.** Public requests read published artifacts; they never
  trigger upstream downloads or broad recomputation.
- **Bounded resources.** It is designed to process sets larger than available RAM without
  falling over.

### Catalog categories

`anonymizers` · `scanners` · `intrusion` · `malware_infrastructure` · `messaging_abuse` ·
`service_abuse` · `policy_risk` · `provider_infrastructure` · `special_use` ·
`asn` · `geolocation`

Plus curated **merged sets** (e.g. the FireHOL `level1`–`level4` blocklists, anonymizer and
proxy aggregates) that combine multiple feeds into ready-to-use lists.

---

## The public website

[iplists.firehol.org](https://iplists.firehol.org) is the observatory itself:

- a **feed explorer** across the whole catalog,
- a **per-feed deep dive** (size, history, retention, comparisons, country/ASN breakdown,
  researched provenance and policy),
- **pairwise comparison** between any feeds,
- **IP search** — which lists contain a given address,
- **historical timelines** showing how each feed evolves.

Every computed metric on the site links to a published **methodology page** explaining how it
is calculated. No magic numbers.

---

## REST API (daemon mode)

The daemon serves cache-first public endpoints. A separate, authenticated admin surface
exposes operations.

**Public (selected):**

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/sets` | List all sets with metadata |
| GET | `/api/v1/sets/{name}` | Detail for one feed |
| GET | `/api/v1/sets/{name}/data` | Raw IP list (`text/plain`) |
| GET | `/api/v1/sets/{name}/history` | History CSV (DateTime, Entries, UniqueIPs) |
| GET | `/api/v1/sets/{name}/compare` | Pairwise overlap comparison |
| GET | `/api/v1/sets/{name}/retention` | Retention analysis |
| GET | `/api/v1/sets/{name}/countries/{provider}` | Geolocation breakdown |
| GET | `/api/v1/sets/{name}/asn/{provider}` | ASN attribution |
| GET | `/api/v1/search?ip=...` | Which sets contain an IP (`&details=true` for matches) |
| GET | `/api/v1/compose?include=...&exclude=...&format=...` | Compose sets on the fly |
| GET | `/healthz` · `/robots.txt` · `/sitemap.xml` · `/llms.txt` | Service & crawler surfaces |

Plain text, CSV, and JSON — built to be fetched programmatically and dropped into firewalls,
SIEMs, TIPs, cloud security groups, and scripts without custom parsers. The public endpoint
reference is in [`docs/api/`](docs/api/); admin operation guidance is in [`docs/admin-ui/`](docs/admin-ui/).

---

## MCP server (for AI agents)

The service also speaks the **Model Context Protocol**, so AI agents, IDE integrations, and
MCP-compatible tools can discover and analyze the catalog directly.

🔌 **Endpoint:** `https://iplists.firehol.org/mcp` — Streamable HTTP transport, public,
with sessions managed automatically via the `Mcp-Session-Id` header.

| Tool | What it does |
|------|--------------|
| `find_feeds` | Discover feeds by metadata — full-text search plus combinable filters on category, maintainer, provenance, health, freshness, cadence, uniqueness, license, redistributability, critical-infrastructure tier, and size. |
| `fetch_analysis` | Retrieve a full per-feed analysis page (markdown). |

Point any MCP client at the URL above — no key required. See
[`docs/api/mcp-endpoint.md`](docs/api/mcp-endpoint.md) for tool parameters and client examples.

---

## Running it

### Build

```bash
make build        # build the binary
make test         # run tests
make race         # run tests with the race detector
make lint         # go vet
make bench        # run benchmarks
```

Cross-compile static Linux binaries:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o update-ipsets-amd64 ./cmd/update-ipsets
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o update-ipsets-arm64 ./cmd/update-ipsets
```

### Install

`./install.sh` is the authoritative install path. It builds the binary, deploys the catalog
to `/opt/update-ipsets/etc/config/`, and installs the systemd unit. See
[`docs/installation/`](docs/installation/).

### Run the daemon (local development)

```bash
update-ipsets daemon \
  --config configs/firehol \
  --listen :18888 \
  --admin-auth-mode=disabled \
  --allow-unauthenticated-admin
```

### Run the daemon (production, split admin port)

```bash
UPDATE_IPSETS_ADMIN_USER=admin \
UPDATE_IPSETS_ADMIN_PASSWORD=change-this-secret \
update-ipsets daemon \
  --config /opt/update-ipsets/etc/config \
  --listen :18888 \
  --admin-listen 127.0.0.1:18889 \
  --admin-auth-mode=required
```

In `required` mode, missing admin credentials fail closed. When `--admin-listen` is set, the
admin UI and admin API are removed from the public listener. `SIGHUP` reloads configuration
without a restart. On Linux as root, the daemon can apply sets natively via netlink.

---

## Command-line tools

update-ipsets is also a standalone CLI:

- **`update-ipsets iprange`** — a FireHOL `iprange`-compatible set tool: CIDR /
  range / single-IP / binary I/O, compare, diff, exclude, intersect, combine, hostname
  resolution, prefix reduction, count-unique, `@filelist` / `@directory` expansion,
  IPv4 by default, and IPv6 with `--ipv6`.
- **`update-ipsets query`** — ask which lists contain an IP, or compose sets with an
  expression like `--set "set1 + set2 - set3"`.
- **`update-ipsets enable`** — toggle which sources are collected; use `--disable` to remove enable markers.
- **`update-ipsets daemon`** — scheduler + web server + API + admin UI.
- **`update-ipsets version`** — print the version string.

---

## Architecture notes

**Out-of-core by design.** Instead of loading every set into the Go heap, the engine uses
file-backed binary snapshots (`mmap`/`pread` with on-file binary search), iterator-based set
operations (O(1) memory two-pointer sweeps), a streaming processor pipeline, and
spill-to-disk downloads. Catalog-wide comparisons stay within a bounded memory budget — set
`GOMEMLIMIT` (and systemd `MemoryHigh`/`MemoryMax`) to get graceful "degrade under pressure"
behavior rather than OOM kills.

**Two-loop pipeline.** A downloader loop decides what to fetch and when; a processing loop
consumes admitted work and produces published artifacts. Operators can always answer: what is
waiting to download, what is downloading, what is waiting to process, what is processing, what
failed, and at which stage.

**Observability.** The daemon can export traces, metrics, and logs over OTLP (HTTP or gRPC).
See the OpenTelemetry section in [`docs/`](docs/) and the daemon reference.

---

## Documentation

- [`docs/about-update-ipsets.md`](docs/about-update-ipsets.md) — what it is and its boundaries
- [`docs/quick-start.md`](docs/quick-start.md) — get running fast
- [`docs/installation/`](docs/installation/) — install & systemd setup
- [`docs/api/`](docs/api/) — full API reference
- [`docs/feeds/`](docs/feeds/) — feed model, families, and metadata
- [`docs/pipeline/`](docs/pipeline/) — download & processing lifecycle
- [`docs/admin-ui/`](docs/admin-ui/) — operator UI and actions
- [`docs/migration-from-bash.md`](docs/migration-from-bash.md) — migrating from the original bash implementation

---

## Scope & limits

**Well suited for** comparative analysis of IP-based feeds, publishing feed-local and
cross-feed facts, and operator-managed long-running collection pipelines.

**Not** a feed-ranking authority, a "which feed is best" policy engine, or a general-purpose
ETL platform. IPv6 is not yet implemented (IPv4 only); non-IP observables (domains, URLs,
hashes) are out of scope unless first transformed into the supported set model.

---

## License

GNU General Public License v2 — see [`COPYING`](COPYING).

Part of the [FireHOL](https://firehol.org) project. Free and open source, for the community.
```
