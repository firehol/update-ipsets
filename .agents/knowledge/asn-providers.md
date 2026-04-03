# IP-to-ASN Provider Research

## TL;DR

The update-ipsets Go project currently integrates one ASN provider (MaxMind GeoLite2-ASN).
The two best additions that add real cross-validation value are **iptoasn.com** (public domain,
hourly, largest coverage at ~694k records) and **RIPE RIS whois dump** (authoritative BGP
routing data, ~1.2M records, daily, appears freely usable). DB-IP Lite ASN (CC BY 4.0, 467k
records, MMDB format, already known to Netdata) is a third option with the easiest Go
integration but smallest coverage and monthly updates. CAIDA prefix2as and bgp.tools table
exports are viable but have licensing ambiguity for redistribution.

---

## Providers Netdata already uses

Found in:
`~/src/PRs/topology-combined/src/go/tools/topology-ip-intel-downloader/`

### 1. iptoasn.com (Netdata: `iptoasn:combined`)

- **File:** `config.go` lines 19, 175–188 — defines `providerIPToASN = "iptoasn"`, `artifactIPToASNCombined = "combined"`, and the built-in spec with `directURL: "https://iptoasn.com/data/ip2asn-combined.tsv.gz"`
- **Parser:** `parse.go` lines 168–238 — `parseIPToASNCombinedTSVAsn` / `parseIPToASNCombinedTSVGeo` read tab-separated fields: `range_start \t range_end \t ASN \t country_code \t AS_description`
- **Format:** TSV (gzip). Can also be used for geo (country code in column 4).
- **Default config:** Not in the default source list — it is an available option but Netdata defaults to DB-IP.

### 2. DB-IP Lite ASN (Netdata: `dbip:asn-lite`)

- **File:** `config.go` lines 21, 185–199 — defines `providerDBIP = "dbip"`, `artifactDBIPASNLite = "asn-lite"`, page URL `"https://db-ip.com/db/download/ip-to-asn-lite"`. The downloader scrapes the page to find the monthly URL matching `dbip-asn-lite-YYYY-MM.mmdb.gz`.
- **Parser:** `parse.go` lines 240–436 — `parseDBIPAsnMMDB` and `parseDBIPAsnCSV`. MMDB fields: `autonomous_system_number` (uint32) + `autonomous_system_organization` (string).
- **Format:** MMDB (preferred) or CSV (gzip). Both include IPv6.
- **Default config:** This IS the default ASN source in Netdata's `defaultConfig()` (`config.go` lines 131–134).

### 3. MaxMind GeoLite2-ASN (update-ipsets: `maxmind_geolite2`)

- **File:** `~/src/firehol/update-ipsets/pkg/asnloc/asnloc.go` — only decoder registered is `"maxmind_geolite2_asn_mmdb"` (line 226). MMDB fields: `autonomous_system_number`, `autonomous_system_organization`.
- **Config:** `~/src/firehol/update-ipsets/configs/firehol/sources/asn/maxmind_geolite2_asn.yaml` — type `maxmind_geolite2_asn_mmdb`, URL template using `${MAXMIND_LICENSE_KEY}`.
- **Integration:** Already integrated and working. This is the only ASN provider currently in update-ipsets.

---

## All providers found

### Provider 1: MaxMind GeoLite2-ASN *(already integrated in update-ipsets)*

| Field | Value |
|-------|-------|
| Homepage | https://dev.maxmind.com/geoip/geolite2-free-geolocation-data |
| License | GeoLite2 EULA + CC BY-SA 4.0. Attribution required: "This product includes GeoLite Data created by MaxMind, available from https://www.maxmind.com" |
| Redistribution | Requires prior written consent from MaxMind to share raw data with third parties. Derived/aggregated statistics (e.g. "X% of IPs are in AS12345") appear to be fine. Do NOT redistribute the raw MMDB. |
| Registration | Yes — free MaxMind account required (no credit card, just name/company/email/intended use). Generates a license key. |
| Bulk download URL | `https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-ASN&license_key=${KEY}&suffix=tar.gz` |
| Format | tar.gz containing a .mmdb file (8.5–8.6 MB compressed). Also available as CSV (~28 MB). |
| Update frequency | Weekly (Tuesdays typically) |
| Coverage | IPv4 + IPv6, global |
| Size | ~8.5 MB MMDB compressed; records: not enumerable without download but comparable to iptoasn |
| Fields | `autonomous_system_number` (uint32), `autonomous_system_organization` (string) |
| Go library | `github.com/oschwald/maxminddb-golang` — already used in update-ipsets |
| Integration difficulty | Already done. |
| Caveats | Redistribution of raw data forbidden without written consent. License key required. Must delete old database within 30 days of new release. |

---

### Provider 2: iptoasn.com

| Field | Value |
|-------|-------|
| Homepage | https://iptoasn.com/ |
| Maintainer | Frank Denis (`j[@]iptoasn.com`). GitHub: https://github.com/jedisct1/iptoasn-webservice |
| License | **Public Domain (PDDL v1.0)** — completely unrestricted. No attribution required. Can redistribute raw data. |
| Registration | None required |
| Bulk download URL | `https://iptoasn.com/data/ip2asn-combined.tsv.gz` (IPv4+IPv6 combined) |
| Alt URLs | `https://iptoasn.com/data/ip2asn-v4.tsv.gz` (IPv4 only), `https://iptoasn.com/data/ip2asn-v6.tsv.gz` (IPv6 only) |
| Format | Tab-separated values (.tsv), gzip compressed. Columns: `range_start \t range_end \t AS_number \t country_code \t AS_description` |
| Update frequency | **Hourly** |
| Coverage | IPv4 + IPv6, global. Includes unrouted gaps as `0 None Not routed`. |
| Size | Combined file: ~694k lines uncompressed (verified). Compressed ~a few MB. |
| Fields | ASN number, ASN description (org name), country code. IP range as start/end pair. |
| Go library | No special library needed — pure TSV parsing. Already parsed in Netdata's topology-ip-intel-downloader. |
| Integration difficulty | **Low.** The TSV parser already exists in Netdata (`parse.go`). A new `asnloc` decoder for the TSV format is needed in update-ipsets. |
| Data source | Built from BGP routing tables (Frank Denis's own BGP data collection). |
| Cross-validation value | **High.** Different methodology from MaxMind (BGP-derived vs. MaxMind's proprietary combination). Public domain means we can distribute raw data alongside update-ipsets output. |
| Caveats | The public per-IP API was shut down Dec 2020 — bulk download only. Includes many `ASN=0` "not routed" gaps. Country code is in column 4 (doubles as geo source). |

---

### Provider 3: DB-IP Lite ASN

| Field | Value |
|-------|-------|
| Homepage | https://db-ip.com/db/download/ip-to-asn-lite |
| License | **Creative Commons Attribution 4.0 (CC BY 4.0)**. Can redistribute, including in commercial products, if attribution is maintained. |
| Attribution required | Web apps: include `<a href='https://db-ip.com'>IP Geolocation by DB-IP</a>` on pages using results. For distributed software: attribution in documentation/about screen. |
| Registration | None required |
| Bulk download URL | `https://download.db-ip.com/free/dbip-asn-lite-YYYY-MM.mmdb.gz` (replace YYYY-MM with current month, e.g. `2026-04`) |
| Alt download URL | `https://download.db-ip.com/free/dbip-asn-lite-YYYY-MM.csv.gz` |
| Format | MMDB (9 MB) or CSV (28.3 MB), gzip compressed. Monthly filename — must be fetched by scraping download page or constructing URL from current date. |
| Update frequency | **Monthly** |
| Coverage | IPv4 + IPv6, global (verified: last lines are IPv6 African allocations) |
| Size | **466,840 records** (verified from live download). Much smaller than iptoasn or RIPE RIS. |
| Fields | CSV: `ip_start, ip_end, asn_number, asn_organization`. MMDB: `autonomous_system_number`, `autonomous_system_organization`. |
| Go library | `github.com/oschwald/maxminddb-golang` for MMDB. Already used in both Netdata and update-ipsets. |
| Integration difficulty | **Very low** — MMDB schema is identical to MaxMind GeoLite2. The existing `decodeMaxMindASN` function in update-ipsets `asnloc.go` works unchanged (same field names). |
| Cross-validation value | **Medium.** Different source from MaxMind but both are commercial databases with similar methodology. Less independent than iptoasn or RIPE RIS. Monthly cadence means stale data for 3–4 weeks. |
| Caveats | Only 467k records vs MaxMind's larger dataset. Monthly updates are a real limitation for a tool that runs every 4 minutes. |

---

### Provider 4: RIPE RIS Whois Dump

| Field | Value |
|-------|-------|
| Homepage | https://www.ripe.net/analyse/internet-measurements/routing-information-service-ris/ |
| Dumps URL | https://ris.ripe.net/dumps/ |
| License | No explicit license statement found. RIPE NCC data is generally open, but their formal terms were not accessible during this research. **Needs verification before redistribution of raw data.** |
| Registration | None required — publicly accessible |
| Bulk download URL | `https://ris.ripe.net/dumps/riswhoisdump.IPv4.gz` (IPv4), `https://ris.ripe.net/dumps/riswhoisdump.IPv6.gz` (IPv6) |
| Format | Gzip-compressed text. Columns: `origin_asn \t prefix/length \t num_ris_peers_seeing_it`. Comment lines start with `%`. |
| Example | `13335 \t 1.0.0.0/24 \t 373` means AS13335 (Cloudflare) originates 1.0.0.0/24, seen by 373 RIS peers. |
| Update frequency | **Daily** (files updated ~10:00 UTC based on observed timestamps) |
| Coverage | IPv4: ~1.2M lines (verified), IPv6: ~1.2M bytes compressed. Global BGP routing table perspective from RIPE's route collectors at major IXPs. |
| Size | IPv4 compressed: 5.2 MB. IPv6 compressed: 1.2 MB. Uncompressed ~1.2M lines for IPv4. |
| Fields | ASN number, CIDR prefix (requires converting to range for update-ipsets). No org name — must join with a name lookup file (e.g. RIPE `asn.txt` or bgp.tools `asns.csv`). |
| Go library | No special library needed — text parsing. But CIDR→range expansion is needed. |
| Integration difficulty | **Medium.** Format is different: it maps (ASN, prefix) not (range_start, range_end, ASN). Need to convert CIDR prefixes to IP ranges. Possible multiple ASNs per prefix (MOAS — multiple origin AS). No org name included — requires a second data source for names. |
| Cross-validation value | **Very high.** This is authoritative BGP routing data collected from actual BGP sessions, not a commercial database. Independently produced from MaxMind. |
| Caveats | No org names — need to augment with RIPE `asn.txt` or similar. One prefix can map to multiple ASNs (MOAS — pick by peer count). Private/reserved prefixes (RFC 1918 etc.) do appear. License formally unclear for redistribution (RIPE NCC's terms were not accessible during research). |

---

### Provider 5: CAIDA RouteViews Prefix-to-AS

| Field | Value |
|-------|-------|
| Homepage | https://www.caida.org/catalog/datasets/routeviews-prefix2as/ |
| Download base | `https://publicdata.caida.org/datasets/routing/routeviews-prefix2as/YYYY/MM/` |
| License | **CAIDA Acceptable Use Agreement (AUA).** The AUA grants a "limited, non-exclusive, non-transferable, non-assignable" license. It does **not** explicitly permit redistribution. The no-implied-licenses clause suggests redistribution is NOT permitted without additional authorization. **Do not redistribute raw data.** |
| Registration | None for download. But AUA must be agreed to. |
| Bulk download URL | `https://publicdata.caida.org/datasets/routing/routeviews-prefix2as/YYYY/MM/routeviews-rv2-YYYYMMDD-1200.pfx2as.gz` |
| Creation log | `https://publicdata.caida.org/datasets/routing/routeviews-prefix2as/pfx2as-creation.log` — check this to find latest file |
| Format | Gzip text. Tab-separated: `ip_prefix \t prefix_length \t AS`. Multi-origin ASNs shown as `AS1_AS2_AS3` (ordered by BGP visibility). |
| Example | `1.0.0.0 \t 24 \t 13335` (Cloudflare owns 1.0.0.0/24) |
| Update frequency | **Daily** |
| Coverage | IPv4 + IPv6 (separate file set for IPv6). **~1.09M prefix records** (verified from live download). |
| Size | Compressed: 3.6 MB per file (verified) |
| Fields | CIDR prefix (as network + length), ASN. No org name. |
| Go library | Pure text parsing. Need CIDR→range conversion. |
| Integration difficulty | **Medium-High.** Same parsing challenges as RIPE RIS (CIDR format, no org names, MOAS). Plus the AUA licensing restriction makes redistribution unclear. |
| Cross-validation value | **High** for internal use. Derived from RouteViews BGP data (different collector network from RIPE RIS). RouteViews uses CC BY 4.0 for the raw BGP data, but CAIDA's processing adds AUA restrictions. |
| Caveats | CAIDA AUA is restrictive for redistribution. MOAS prefixes need disambiguation. No org names. Typically 1 day behind. |

**Note on RouteViews direct access:** The raw BGP MRT dumps from RouteViews (`archive.routeviews.org`) are under CC BY 4.0, but extracting IP-to-ASN requires parsing binary MRT format (bzip2-compressed, 67–69 MB per dump) and is substantially more work than using CAIDA's pre-processed pfx2as files.

---

### Provider 6: bgp.tools Table Export

| Field | Value |
|-------|-------|
| Homepage | https://bgp.tools/ |
| Operator | Port 179 Ltd, England & Wales, Registration Number: 14127855 |
| License | **Not specified.** No explicit license or terms of service found for bulk data downloads. |
| Registration | None — HTTP with identifying User-Agent required |
| Bulk download URL | `https://bgp.tools/table.txt` (CIDR ASN pairs) or `https://bgp.tools/table.jsonl` (JSON with visibility) |
| ASN names | `https://bgp.tools/asns.csv` — columns: `asn, name, class, cc` |
| Format | `table.txt`: space-separated `CIDR ASN`. `table.jsonl`: one JSON per line `{"CIDR":"...", "ASN":12345, "Hits":509}`. `asns.csv`: CSV with ASN, org name, class, country. |
| Update frequency | **Every ~30 minutes** (tables); `asns.csv` is less frequent |
| Coverage | Global BGP routing table. Real-time. Very fresh data. |
| Size | Large (table.txt exceeds the 10MB WebFetch limit — file is substantial, likely 20+ MB) |
| Fields | CIDR + ASN. For org names, use `asns.csv` separately. |
| Go library | Pure text parsing. Need CIDR→range expansion. |
| Integration difficulty | **Medium.** CIDR-to-range conversion needed. Two-file join for names. No explicit license is a blocker for redistribution. |
| Cross-validation value | **High** for internal use — near real-time BGP data from a well-run looking glass. |
| Caveats | **No license statement** — cannot redistribute raw data without checking with admin@bgp.tools. "Robots scraping HTML pages will be banned." Bulk exports are fine with identifying User-Agent and reasonable poll intervals (min 30 min). No SLA or uptime guarantee. |

---

### Provider 7: RIPE NCC ASN Names File

| Field | Value |
|-------|-------|
| Homepage | https://ftp.ripe.net/ripe/asnames/ |
| License | Not explicitly stated in the file or adjacent directory. |
| Registration | None required |
| Download URL | `https://ftp.ripe.net/ripe/asnames/asn.txt` |
| Format | Plain text, space-delimited: `ASN_number \t ASN-name - Organization Name, CC` |
| Update frequency | Daily (file showed `06-Apr-2026 10:49`) |
| Coverage | All globally allocated ASNs. File size: 5 MB |
| Fields | ASN number, ASN handle, organization name, country code. **No IP ranges.** |
| Integration difficulty | **Low** — simple text parsing. But this is not an IP-to-ASN database — it is an ASN-to-name lookup table only. |
| Cross-validation value | **Supplementary.** Useful for enriching ASN names when the primary IP lookup returns an ASN number without a name (which RIPE RIS and CAIDA pfx2as do not provide). |
| Caveats | This is NOT an IP-to-ASN database. It maps ASN numbers to organization names. Use as a supplementary name lookup when the primary provider gives ASN number but not a name. |

---

### Provider 8: Team Cymru IP-to-ASN Mapping

| Field | Value |
|-------|-------|
| Homepage | https://team-cymru.com/community-services/ip-to-asn-mapping/ |
| License | Free for reasonable use (community service). No bulk download available. |
| Registration | None for per-query services |
| Bulk download URL | **None.** Per-IP or per-prefix queries only. |
| Methods | Whois (port 43 to `whois.cymru.com`), DNS (`origin.asn.cymru.com`), HTTP API |
| Format | Per-query responses |
| Update frequency | Near-real-time |
| Integration difficulty | **High** for bulk use — requires per-IP queries, not suitable for building a local database. |
| Cross-validation value | **Low for our use case** — no bulk file to cross-validate. Useful for spot-checking individual IPs but cannot build a full mapping. |
| Caveats | Per-query service only. Not suitable for building a local IP-to-ASN database. Rate limits apply for bulk queries. |

---

### Provider 9: IP2Location DB-ASN *(not free)*

| Field | Value |
|-------|-------|
| Homepage | https://www.ip2location.com/ |
| License | Commercial. Paid subscription required. |
| Free tier | No free standalone ASN database. ASN data only available in paid PX7+ products. "Free LITE Database" exists but does not include ASN. 7-day trial available. |
| Registration | Required, paid subscription |
| Verdict | **Skip.** Not free. |

---

### Provider 10: IPInfo

| Field | Value |
|-------|-------|
| Homepage | https://ipinfo.io/ |
| License | CC BY-SA 4.0 for "IPInfo Lite" |
| Registration | Required — free account |
| Free tier | "IPInfo Lite" — free API with country-level geo + ASN name. **Database downloads NOT included in self-serve free plans.** Contact sales required for bulk database access. |
| Verdict | **Skip.** Database downloads are not free — require contacting sales. The API has ASN but no bulk file for free tier. |

---

### Provider 11: sapics/ip-location-db (GitHub aggregator)

| Field | Value |
|-------|-------|
| Homepage | https://github.com/sapics/ip-location-db |
| Description | A GitHub project that re-packages iptoasn.com, DB-IP Lite, and MaxMind GeoLite2 ASN data into MMDB and CSV. Also publishes a "RouteViews + DB-IP" merged variant. |
| License | CC BY 4.0 (attribution to original sources required). Inherits restrictions of each upstream source. |
| Download URLs (CDN) | `https://cdn.jsdelivr.net/npm/@ip-location-db/asn/asn-ipv4.csv` (RouteViews+DB-IP, daily), `https://cdn.jsdelivr.net/npm/@ip-location-db/iptoasn-asn/iptoasn-asn-ipv4.csv` (iptoasn), and MMDB variants. |
| Format | CSV (`ip_start, ip_end, asn, org`) or MMDB |
| Update frequency | Daily for RouteViews-based; monthly for DB-IP |
| Integration difficulty | **Low** — CSV with clear range-start/range-end format. MMDB option also available. |
| Cross-validation value | **Low** — this is a repackaged version of sources we can consume directly. It adds no new data source. Useful only if we want CDN delivery without scraping upstream. |
| Caveats | Depends on upstream sources remaining available and properly licensed. CAIDA AUA restrictions likely apply to the RouteViews-derived variant — unclear if sapics's redistribution is proper. |

---

## Recommendations for update-ipsets

### Tier 1: Add immediately (clear licensing, high value)

**1. iptoasn.com (`iptoasn`)**
- Public domain — zero redistribution restrictions
- Hourly updates — freshest possible data
- ~694k records covering IPv4+IPv6 globally
- Parser already exists in Netdata's topology-ip-intel-downloader (TSV format)
- Cross-validation value: HIGH (BGP-derived, independent of MaxMind)
- **New decoder needed** in `asnloc.go`: parse the TSV file format (start, end, asn, country, name). The decoder can be TSV-based using bufio.Scanner, similar to Netdata's implementation.
- **Config addition:** new entry under `asn:` with type `iptoasn_combined_tsv`, URL `https://iptoasn.com/data/ip2asn-v4.tsv.gz` (IPv4 only for update-ipsets which is IPv4-focused)

**2. DB-IP Lite ASN (`dbip`)**
- CC BY 4.0 — redistributable with attribution (add attribution to website footer)
- MMDB format — the existing `decodeMaxMindASN` function works unchanged (same field names)
- 467k records, IPv4+IPv6
- Monthly updates (limitation, but still useful for cross-validation)
- **No new code needed** in `asnloc.go` — the MaxMind decoder already handles the same MMDB schema
- **Config addition:** new entry under `asn:` with type `maxmind_geolite2_asn_mmdb` (reuse same decoder), URL scraping the DB-IP download page or using the monthly URL pattern

### Tier 2: Add with caution (requires license clarification)

**3. RIPE RIS Whois Dump (`ripe_ris`)**
- No explicit license found — RIPE data is generally open but formal terms for redistribution unclear
- Very high cross-validation value (authoritative BGP from RIPE's route collectors at IXPs)
- ~1.2M IPv4 records + IPv6 — biggest coverage of any option
- Daily updates
- **New decoder needed:** TSV with columns (ASN, CIDR prefix, peer count). Requires:
  1. CIDR prefix → IP range conversion (one CIDR can expand to `start, end`)
  2. MOAS disambiguation (multiple ASNs for same prefix — pick the one seen by most peers)
  3. Join with RIPE `asn.txt` for org names (or use MaxMind/iptoasn for names and RIPE RIS for routing facts)
- **Action required:** Verify RIPE NCC's redistribution terms before publishing RIPE-derived data in the public blocklist-ipsets repo. The riswhoisdump data has no explicit CC license. Check `https://www.ripe.net/manage-ips-and-asns/db/`

**4. bgp.tools Table Export (`bgptools`)**
- No license statement found — contact admin@bgp.tools before redistributing
- Near-real-time (30-minute granularity) — freshest data
- Large dataset
- Requires org names from separate `asns.csv` file
- **Action required:** Contact bgp.tools for explicit license/redistribution permission

### Tier 3: Skip or low priority

**5. CAIDA RouteViews prefix2as** — AUA prohibits redistribution. Use only for internal analysis. Do NOT include as a publicly distributed provider.

**6. Team Cymru** — Per-IP queries only. No bulk file. Skip.

**7. IP2Location** — Paid only. Skip.

**8. IPInfo** — Free API but database downloads require sales contact. Skip.

**9. sapics/ip-location-db** — Re-packages sources we can consume directly. Adds no new data. Skip unless CDN delivery is specifically needed.

**10. RIPE ASN Names (`asn.txt`)** — Not an IP-to-ASN database (no IP ranges). Could be used as a supplementary name lookup for providers that give ASN numbers without names (RIPE RIS, CAIDA). Not a standalone provider.

---

## Integration architecture note

The current `asnloc.go` only handles MMDB files. To integrate iptoasn.com (TSV) and RIPE RIS (text), the `decoderFor` function needs extending, and the `Open` function needs an alternative code path for non-MMDB formats. One clean approach: add a `OpenTSV(providerType, path string)` constructor that returns the same `*Database` interface with a TSV-based decoder, so `processASNFeeds` in `engine/asn.go` works unchanged.

For RIPE RIS specifically: the file format maps (ASN → prefix), not (prefix → ASN start/end range). Update-ipsets needs the reverse direction: given an IP, find the ASN. This requires building an in-memory prefix trie or sorted slice during load, which is a more significant engineering task than the MMDB path.

---

## Sources consulted

- https://iptoasn.com/ — license, format, download URL (official)
- https://github.com/jedisct1/iptoasn-webservice — source repo, update mechanism (official)
- https://db-ip.com/db/download/ip-to-asn-lite — license, download URL, column names (official)
- https://www.routeviews.org/routeviews/index.php/about/ — CC BY 4.0 license confirmation (official)
- https://archive.routeviews.org/bgpdata/2026.04/RIBS/ — file format and size of MRT dumps (official)
- https://data.caida.org/datasets/routing/routeviews-prefix2as/ — dataset description, URL, format (official)
- https://data.caida.org/datasets/routing/routeviews-prefix2as/pfx2as-creation.log — live URL structure (official)
- https://www.caida.org/catalog/datasets/routeviews-prefix2as/ — CAIDA AUA license terms (official)
- https://www.caida.org/about/legal/aua/public_aua/ — full AUA text (official)
- https://team-cymru.com/community-services/ip-to-asn-mapping/ — 404, checked from knowledge (community services page no longer accessible)
- https://bgp.tools/kb/api — bulk download URLs, rate limits, format (official)
- https://bgp.tools/asns.csv — live download, verified column format (official)
- https://ris.ripe.net/dumps/ — directory listing, file availability (official)
- `curl https://ris.ripe.net/dumps/riswhoisdump.IPv4.gz` — format verification, line count (1.2M), live data inspection
- `curl https://ris.ripe.net/dumps/riswhoisdump.IPv6.gz` — size verification (1.2 MB compressed)
- https://ftp.ripe.net/ripe/asnames/ — file listing, size (5 MB), update date (official)
- https://ftp.ripe.net/ripe/asnames/asn.txt — format verification (official)
- https://ftp.ripe.net/ripe/stats/ — RIPE delegation files (official, not IP-to-ASN)
- https://www.maxmind.com/en/geolite2/signup — free account requirements, no credit card (official)
- https://dev.maxmind.com/geoip/docs/databases/asn — field names, MMDB/CSV formats (official)
- https://dev.maxmind.com/geoip/geolite2-free-geolocation-data — GeoLite2 overview (official)
- https://www.maxmind.com/en/geolite2/eula — redistribution restrictions (official)
- https://github.com/sapics/ip-location-db — aggregated provider, download URLs, licenses (community)
- https://ipinfo.io/pricing — free tier limitations, no database download (official)
- https://www.ip2location.com/ — no free ASN database confirmed (official)
- `curl https://download.db-ip.com/free/dbip-asn-lite-2026-04.csv.gz` — format verification, record count (466,840), IPv6 coverage (live)
- `curl https://iptoasn.com/data/ip2asn-combined.tsv.gz` — format verification, line count (693,958) (live)
- `curl https://publicdata.caida.org/datasets/routing/routeviews-prefix2as/2026/04/routeviews-rv2-20260405-1200.pfx2as.gz` — format, line count (1,085,731), file size (3.6 MB compressed) (live)
- ~/src/PRs/topology-combined/src/go/tools/topology-ip-intel-downloader/ — Netdata's existing Go implementation (local codebase)
- ~/src/firehol/update-ipsets/pkg/asnloc/asnloc.go — current update-ipsets ASN code (local codebase)
- ~/src/firehol/update-ipsets/configs/firehol/sources/asn/ — current ASN provider config fragments (local codebase)

---

## Methodology

### Official/Authoritative searches
- iptoasn.com homepage, download URLs, license
- DB-IP lite download page, license terms, format
- RouteViews about page (CC BY 4.0 license)
- CAIDA prefix2as catalog page, creation log, AUA
- MaxMind GeoLite2 signup, EULA, developer docs
- RIPE RIS dumps directory, whois dump format
- RIPE asnames FTP directory and asn.txt
- bgp.tools API documentation page

### Practical/Community searches
- Live data downloads to verify format and record counts (iptoasn, DB-IP, CAIDA, RIPE RIS)
- sapics/ip-location-db GitHub for aggregator options
- Netdata topology-ip-intel-downloader source code review
- update-ipsets Go project source code review (asnloc.go, engine/asn.go, configs/firehol/sources/asn/)

### Validation searches
- CC BY 4.0 terms (creativecommons.org)
- CAIDA AUA full text for redistribution terms
- DB-IP license terms page
- IPInfo pricing page for free tier limitations
- IP2Location ASN database page

### Searches that yielded nothing useful
- ARIN bulk whois data page (no ASN mapping)
- RIPE NCC formal terms pages (multiple 404s)
- Team Cymru community services page (404)
- IP2Location free ASN database (404, confirmed no free standalone ASN DB)
- IPInfo free database download URL (503/403)

---

## Limitations

1. **RIPE NCC redistribution terms**: The RIPE NCC terms and conditions pages returned 404 errors. The formal license for `riswhoisdump.IPv4.gz` redistribution is unverified. The data appears open (no registration, CC0-style access), but the actual license document was inaccessible during this research.

2. **bgp.tools license**: No license statement exists on the site. Redistribution status is unknown. Must contact admin@bgp.tools.

3. **MaxMind update frequency**: The exact day/schedule for GeoLite2-ASN updates was not confirmed — only "periodic" was stated. Community knowledge suggests weekly (Tuesdays).

4. **CAIDA IPv6 coverage**: The IPv6 counterpart of the prefix2as dataset (`routeviews6-prefix2as`) was not fully investigated. Likely similar format.

5. **DB-IP redistribution in software**: The CC BY 4.0 license permits redistribution, but DB-IP's own terms page for the Lite edition was a 404. The attribution requirement for shipped software (vs. web pages) needs verification for the specific use case of bundling the database in a distributed tool.

6. **iptoasn.com source**: Frank Denis does not publicly document which BGP table sources he uses to build the iptoasn database. The methodology is a black box beyond "BGP-derived."

7. **Size comparisons are not apples-to-apples**: Record counts differ because datasets use different granularities. iptoasn (694k lines) consolidates adjacent ranges with the same ASN; RIPE RIS (1.2M lines) keeps them separate as advertised prefixes. Coverage overlap statistics between providers were not computed.

---

## Self-Assessment Scores

- **Coverage Completeness**: 8/10 — All major known candidates investigated. Multiple RIPE NCC pages 404'd, so formal RIPE licensing is unverified. PeeringDB confirmed irrelevant. ARIN bulk whois checked (no IP-to-ASN mapping).
- **Source Quality**: 9/10 — Most data verified by live download and inspection of actual file contents, not just documentation. License terms verified from authoritative sources where accessible.
- **Contradiction Resolution**: 8/10 — CAIDA's own data is CC BY 4.0 (RouteViews is CC BY 4.0) but the AUA adds restrictions. This tension is noted but not fully resolved without a CAIDA lawyer answer.
- **Confidence in Conclusions**: 8/10 — Tier 1 recommendations (iptoasn + DB-IP) are solid. Tier 2 (RIPE RIS, bgp.tools) need license confirmation before redistribution but the technical feasibility is clear.

**Overall Reliability Score: 8.25/10**
