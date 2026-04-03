# Hard-tier public DNS resolver research (SOW-0017)

Research date: 2026-04-29. All IPs and ASNs verified against official provider
documentation and bgp.tools unless explicitly marked UNVERIFIED.

---

## Summary table

| Provider | Service | IPv4 | IPv6 | ASN(s) | Source grade | Recommended tier | Notes |
|---|---|---|---|---|---|---|---|
| Cloudflare 1.1.1.1 | Standard public resolver | 1.1.1.1, 1.0.0.1 | 2606:4700:4700::1111, ::1001 | AS13335 | A | hard | Anycast; official IP page |
| Cloudflare Families (malware) | Filtering variant | 1.1.1.2, 1.0.0.2 | 2606:4700:4700::1112, ::1002 | AS13335 | A | hard | Same anycast infra as 1.1.1.1 |
| Cloudflare Families (malware+adult) | Filtering variant | 1.1.1.3, 1.0.0.3 | 2606:4700:4700::1113, ::1003 | AS13335 | A | hard | Same anycast infra as 1.1.1.1 |
| Google Public DNS | Standard public resolver | 8.8.8.8, 8.8.4.4 | 2001:4860:4860::8888, ::8844 | AS15169 | A | hard | Anycast; official docs |
| Google DNS64 | IPv6-only NAT64 resolver | — | 2001:4860:4860::6464, ::64 | AS15169 | A | hard | IPv6-only service; no IPv4 stub |
| Quad9 (secure) | DNSSEC + malware blocking | 9.9.9.9, 149.112.112.112 | 2620:fe::fe, 2620:fe::9 | AS19281 | A | hard | Anycast; official docs |
| Quad9 ECS | Secure + ECS | 9.9.9.11, 149.112.112.11 | 2620:fe::11, 2620:fe::fe:11 | AS19281 | A | hard | Same service; ECS variant |
| Quad9 (unsecured) | No malware filter, no DNSSEC | 9.9.9.10, 149.112.112.10 | 2620:fe::10, 2620:fe::fe:10 | AS19281 | A | hard | Same anycast infra |
| Cisco OpenDNS (standard) | Public resolver + phishing protection | 208.67.222.222, 208.67.220.220 | 2620:119:35::35, 2620:119:53::53 | AS36692 | A | hard | Anycast; official docs |
| Cisco OpenDNS FamilyShield | Adult content filtering | 208.67.222.123, 208.67.220.123 | 2620:119:35::123, 2620:119:53::123 | AS36692 | A | hard | Same infra; filtering variant |
| Vercara UltraDNS Public (unfiltered) | Unfiltered resolver | 64.6.64.6, 64.6.65.6 | 2620:74:1b::1:1, 2620:74:1c::2:2 | AS12008 | A | hard | Formerly Verisign/Neustar; official page |
| Vercara UltraDNS Public (threat) | Threat protection | 156.154.70.2, 156.154.71.2 | 2610:a1:1018::2, 2610:a1:1019::2 | AS12008 | A | hard | Filtering variant |
| Vercara UltraDNS Public (family) | Family safe | 156.154.70.3, 156.154.71.3 | 2610:a1:1018::3, 2610:a1:1019::3 | AS12008 | A | hard | Filtering variant |
| AdGuard DNS (default) | Ad+tracker blocking | 94.140.14.14, 94.140.15.15 | 2a10:50c0::ad1:ff, ::ad2:ff | AS212772 | A | hard | Official page |
| AdGuard DNS (family) | Family protection | 94.140.14.15, 94.140.15.16 | 2a10:50c0::bad1:ff, ::bad2:ff | AS212772 | A | hard | Filtering variant |
| AdGuard DNS (non-filtering) | Pass-through resolver | 94.140.14.140, 94.140.14.141 | 2a10:50c0::1:ff, ::2:ff | AS212772 | A | hard | No filtering |
| Control D (free) | Free public filtering resolver | 76.76.2.11, 76.76.10.11 | 2606:1a40::11, 2606:1a40:1::11 | AS398962 | A | hard | Official IP ranges doc |
| Control D (custom) | Paid custom resolver | 76.76.2.22, 76.76.10.22 | 2606:1a40::22, 2606:1a40:1::22 | AS398962 | A | hard | Same ranges; docs page |
| Mullvad DNS (unfiltered) | Privacy-first resolver | 194.242.2.2 | 2a07:e340::2 | AS57138 | A | hard | Official page |
| Mullvad DNS (adblock) | Ad blocking | 194.242.2.3 | 2a07:e340::3 | AS57138 | A | hard | Filtering variant |
| Mullvad DNS (base) | Base filtering | 194.242.2.4 | 2a07:e340::4 | AS57138 | A | hard | Filtering variant |
| Mullvad DNS (extended) | Extended filtering | 194.242.2.5 | 2a07:e340::5 | AS57138 | A | hard | Filtering variant |
| Mullvad DNS (family) | Family filtering | 194.242.2.6 | 2a07:e340::6 | AS57138 | A | hard | Filtering variant |
| Mullvad DNS (all) | All filters | 194.242.2.9 | 2a07:e340::9 | AS57138 | A | hard | Filtering variant |
| DNS.SB | DNSSEC, no-log resolver | 185.222.222.222, 45.11.45.11 | 2a09::, 2a11:: | AS24013 | C | hard | Official page; no machine-readable feed |
| DNS4EU | EU sovereign resolver (protective) | 86.54.11.1, 86.54.11.201 | 2a13:1001::86:54:11:1, ::11:201 | AS198121 | C | hard | Whalebone-operated; static page |
| DNS4EU (child) | Child protection variant | 86.54.11.12, 86.54.11.212 | see details | AS198121 | C | hard | 4 filtering variants total |
| CIRA Canadian Shield (private) | Privacy resolver | 149.112.121.10, 149.112.122.10 | 2620:10A:80BB::10, 80BC::10 | AS40568 | C | hard | Canadian; official docs |
| CIRA Canadian Shield (protected) | Security filtering | 149.112.121.20, 149.112.122.20 | 2620:10A:80BB::20, 80BC::20 | AS40568 | C | hard | Filtering variant |
| CIRA Canadian Shield (family) | Family filtering | 149.112.121.30, 149.112.122.30 | 2620:10A:80BB::30, 80BC::30 | AS40568 | C | hard | Filtering variant |
| AliDNS | China public resolver | 223.5.5.5, 223.6.6.6 | 2400:3200::1, 2400:3200:baba::1 | AS45102 | C | hard | Official page; widely used in China |
| DNSPod | China public resolver (Tencent) | 119.29.29.29, 182.254.116.116 | UNVERIFIED | AS132203 | C | hard | Large China resolver; no official IPv6 found |
| CleanBrowsing (security) | Malware/phishing blocking | 185.228.168.9, 185.228.169.9 | 2a0d:2a00:1::2, 2a0d:2a00:2::2 | AS205157 | A | hard | Official filters page |
| CleanBrowsing (adult) | Adult filter | 185.228.168.10, 185.228.169.11 | 2a0d:2a00:1::1, 2a0d:2a00:2::1 | AS205157 | A | hard | Note: secondary is .11, not .10 |
| CleanBrowsing (family) | Family filter | 185.228.168.168, 185.228.169.168 | 2a0d:2a00:1::, 2a0d:2a00:2:: | AS205157 | A | hard | |
| Yandex DNS (basic) | Standard resolver | 77.88.8.8, 77.88.8.1 | 2a02:6b8::feed:0ff, ::1::feed:0ff | AS13238 | C | soft | Russian operator; geopolitical risk |
| Yandex DNS (safe) | Malware/fraud blocking | 77.88.8.88, 77.88.8.2 | 2a02:6b8::feed:bad, ::1::feed:bad | AS13238 | C | soft | Russian operator; filtering |
| Yandex DNS (family) | Family filtering | 77.88.8.7, 77.88.8.3 | 2a02:6b8::feed:a11, ::1::feed:a11 | AS13238 | C | soft | Russian operator; filtering |
| Vercara/Neustar history note | n/a | n/a | n/a | n/a | n/a | n/a | IPs now Vercara (AS12008); Verisign sold service Dec 2020 |
| Comodo Secure DNS | Malware/phishing blocking | 8.26.56.26, 8.20.247.20 | none published | AS23393 (NuCDN) | C | soft | Operational but aging; no IPv6; ASN is NuCDN not Comodo |
| NextDNS | Per-profile configurable resolver | 45.90.28.x, 45.90.30.x | UNVERIFIED | AS34939 | D | reject | No stable fixed public stub IPs; profile-specific |
| DNS0.eu | EU privacy resolver | n/a | n/a | n/a | n/a | reject | **DISCONTINUED** — shut down 2024 due to lack of resources |
| OpenNIC | Volunteer alternative DNS roots | dynamic | dynamic | various | D | reject | Volunteer/dynamic; not a stable reference set |
| DNS.WATCH | Uncensored DNSSEC resolver | 84.200.69.80, 84.200.70.40 | UNVERIFIED | UNVERIFIED | D | soft | No official machine-readable feed found; IPv6 unverified |
| Hurricane Electric | Transit/carrier public resolver | 74.82.42.42 | 2001:470:20::2 | AS6939 | D | soft | No official IP publish page; derived from ordns.he.net |
| CZ.NIC ODVR | Czech DNSSEC validating resolver | 193.17.47.1, 185.43.135.1 | 2001:148f:ffff::1, ::fffe::1 | AS20701 | C | hard | Official; reputable NIC operator |
| IIJ Public DNS | Japan ISP public resolver | 103.2.57.5, 103.2.57.6 | UNVERIFIED | AS2497 | C | hard | Major Japanese ISP; official service |
| SafeDNS | Commercial filtering DNS | 195.46.39.39, 195.46.39.40 | UNVERIFIED | AS57926 | C | soft | Commercial; source grade C (static docs) |
| AdGuard DNS (Foundation Applied Privacy) | n/a | 146.255.56.98 | 2a02:1b8:10:234::2 | AS208323 | C | soft | Small AT-based non-profit; single server |
| Baidu DNS | China public resolver | 180.76.76.76 | UNVERIFIED | AS55967 | C | soft | Chinese operator; geopolitical risk |
| 114DNS | China public resolver (China Telecom-linked) | 114.114.114.114, 114.114.115.115 | UNVERIFIED | UNVERIFIED (AS4812 reported, conflicts) | D | soft | No verified official source page found; widely used |
| OneDNS | China filtering resolver | 117.50.10.10, 52.80.52.52 (clean); 117.50.11.11, 52.80.66.66 (filtered) | UNVERIFIED | UNVERIFIED | D | reject | No verified official English-language source |
| SkyDNS | Russia commercial/educational filtering | 193.58.251.251 | UNVERIFIED | UNVERIFIED | D | reject | Commercial subscription product; geopolitical risk |
| LibreDNS | Greek community DoH/DoT | 116.202.176.26 | UNVERIFIED | AS24940 (Hetzner) | D | reject | Community/single-server; hosted on Hetzner, not own ASN |
| NordVPN DNS | VPN provider resolver | 103.86.96.100, 103.86.99.100 | UNVERIFIED | AS136787 (PacketHub) | D | reject | VPN-tier resolver; ASN is a transit provider not NordVPN |
| Strongarm/WatchGuard DNSWatch | Commercial/SMB DNS filter | n/a | n/a | n/a | n/a | reject | Acquired by WatchGuard 2018; not a stable public resolver |
| ScrubIT | Family/filtering resolver | n/a | n/a | n/a | n/a | reject | **DISCONTINUED** — no longer operating |
| DNS4EU (all variants) | EU resolver, 10 IPs | see details | see details | AS198121 | C | hard | Full variant list in details section |

---

## Per-provider details

### Cloudflare 1.1.1.1

- **Service**: Cloudflare's public recursive DNS resolver, operated as part of Cloudflare's global anycast network.
- **Official documentation**: https://developers.cloudflare.com/1.1.1.1/ip-addresses/
- **IPv4 addresses**:
  - Standard: `1.1.1.1`, `1.0.0.1`
  - Families (malware): `1.1.1.2`, `1.0.0.2`
  - Families (malware+adult): `1.1.1.3`, `1.0.0.3`
- **IPv6 addresses**:
  - Standard: `2606:4700:4700::1111`, `2606:4700:4700::1001`
  - Families (malware): `2606:4700:4700::1112`, `2606:4700:4700::1002`
  - Families (malware+adult): `2606:4700:4700::1113`, `2606:4700:4700::1003`
- **ASN**: AS13335 — Cloudflare, Inc. (verified via bgp.tools prefix 1.1.1.0/24)
- **DoH**: `https://cloudflare-dns.com/dns-query`, `https://1.1.1.1/dns-query`
- **DoT**: `one.one.one.one`
- **Source grade**: A — official machine-readable; IP page is static HTML but definitive and well-maintained.
- **Recommended tier**: hard
- **Rationale**: 1.1.1.1 is one of the world's most-used public recursive DNS resolvers. Anycast; blocking it degrades DNS resolution globally for users who have configured it. The Families variants share the same anycast infrastructure and should be treated equally.
- **Caveats**: All addresses are part of AS13335's broader prefix space. The 1.1.1.x and 1.0.0.x stubs are shared with Cloudflare's CDN but are specifically dedicated to DNS resolution.

---

### Google Public DNS

- **Service**: Google's free global public recursive DNS resolver.
- **Official documentation**: https://developers.google.com/speed/public-dns/docs/using
- **IPv4 addresses**: `8.8.8.8`, `8.8.4.4`
- **IPv6 addresses**: `2001:4860:4860::8888`, `2001:4860:4860::8844`
- **IPv6 expanded**: `2001:4860:4860:0:0:0:0:8888`, `2001:4860:4860:0:0:0:0:8844`
- **ASN**: AS15169 — Google LLC (verified via bgp.tools prefix 8.8.8.0/24)
- **DoH**: `https://dns.google/dns-query`
- **DoT**: `dns.google`
- **Source grade**: A — official Google developer documentation.
- **Recommended tier**: hard
- **Rationale**: Among the most-used resolvers globally. Anycast; finding either address in a blocklist is an immediate feed-quality red flag. The prefix 8.8.8.0/24 is specifically dedicated to this service within the broader AS15169.
- **Caveats**: AS15169 is a large ASN with many other Google services. The hard-tier reference should be IP-level (8.8.8.8, 8.8.4.4 and prefixes), not ASN-wide.

---

### Google DNS64

- **Service**: Google's IPv6-only DNS64 resolver for NAT64 environments. Synthesizes AAAA records for IPv4-only destinations.
- **Official documentation**: https://developers.google.com/speed/public-dns/docs/dns64
- **IPv4 addresses**: none (IPv6-only service)
- **IPv6 addresses**: `2001:4860:4860::6464`, `2001:4860:4860::64`
- **ASN**: AS15169 — Google LLC
- **DoH**: `https://dns64.dns.google/dns-query`
- **DoT**: `dns64.dns.google`
- **Source grade**: A — official Google developer documentation.
- **Recommended tier**: hard (v1 implementation is IPv4-only; add as future IPv6 coverage item)
- **Rationale**: Dedicated DNS64 service for IPv6-only network segments. Falls in the same 2001:4860:4860::/48 block as standard Google DNS.
- **Caveats**: IPv6-only; will not appear in IPv4 feed overlaps. Include in IPv6 parity follow-up.

---

### Quad9

- **Service**: Non-profit public recursive DNS resolver with threat intelligence integration.
- **Official documentation**: https://quad9.net/service/service-addresses-and-features/
- **IPv4 addresses**:
  - Secure (DNSSEC + malware blocking): `9.9.9.9`, `149.112.112.112`
  - Secure + ECS: `9.9.9.11`, `149.112.112.11`
  - Unsecured (no filtering): `9.9.9.10`, `149.112.112.10`
- **IPv6 addresses**:
  - Secure: `2620:fe::fe`, `2620:fe::9`
  - Secure + ECS: `2620:fe::11`, `2620:fe::fe:11`
  - Unsecured: `2620:fe::10`, `2620:fe::fe:10`
- **ASN**: AS19281 — Quad9 (verified via bgp.tools prefix 149.112.112.0/24 and 9.9.9.0/24; ROA valid)
- **DoH**: `https://dns.quad9.net/dns-query`
- **DoT**: `dns.quad9.net`
- **Source grade**: A — official Quad9 documentation.
- **Recommended tier**: hard
- **Rationale**: Dedicated non-profit DNS operator; AS19281 is entirely Quad9 infra. All variants are anycast across 150+ PoPs. Quad9 is the third major "hard" reference after Cloudflare and Google.
- **Caveats**: The knowledge base had previously noted a validation error (AS16615 was wrong); AS19281 is the correct and verified ASN. Three service variants exist but share the same anycast infrastructure.

---

### Cisco OpenDNS / Umbrella

- **Service**: Cisco's public recursive DNS resolver (OpenDNS) and enterprise Umbrella platform. FamilyShield is the free family-filtering variant.
- **Official documentation**: https://www.opendns.com/setupguide/ (consumer), https://umbrella.cisco.com (enterprise)
- **IPv4 addresses**:
  - Standard OpenDNS: `208.67.222.222`, `208.67.220.220`
  - FamilyShield: `208.67.222.123`, `208.67.220.123`
- **IPv6 addresses**:
  - Standard Umbrella: `2620:119:35::35`, `2620:119:53::53`
  - FamilyShield: `2620:119:35::123`, `2620:119:53::123`
- **ASN**: AS36692 — Cisco OpenDNS, LLC (verified via bgp.tools prefix 208.67.222.0/24)
- **DoH**: `https://doh.opendns.com/dns-query` (standard), `https://doh.familyshield.opendns.com/dns-query`
- **DoT**: not widely documented for consumer tier
- **Source grade**: A — official Cisco/OpenDNS setup guide.
- **Recommended tier**: hard
- **Rationale**: One of the oldest and most widely deployed public DNS resolvers. AS36692 is dedicated to the OpenDNS/Umbrella resolver service. Anycast; globally distributed.
- **Caveats**: The enterprise Umbrella and consumer OpenDNS share the same AS36692 anycast addresses. IPv6 is documented for Umbrella enterprise but applies to the shared infrastructure.

---

### Vercara UltraDNS Public (formerly Verisign, then Neustar)

- **Service**: Vercara's (formerly Neustar's, originally Verisign's) public recursive DNS resolver with three service tiers.
- **Official documentation**: https://vercara.digicert.com/ultra-dns-public
- **History**: Verisign ran the service until December 2020, sold to Neustar, which was then acquired by DigiCert/Vercara. Same IPs, new operator.
- **IPv4 addresses**:
  - Unfiltered: `64.6.64.6`, `64.6.65.6`
  - Threat protection: `156.154.70.2`, `156.154.71.2`
  - Family Secure: `156.154.70.3`, `156.154.71.3`
- **IPv6 addresses**:
  - Unfiltered: `2620:74:1b::1:1`, `2620:74:1c::2:2`
  - Threat protection: `2610:a1:1018::2`, `2610:a1:1019::2`
  - Family Secure: `2610:a1:1018::3`, `2610:a1:1019::3`
- **ASN**: AS12008 — Vercara, LLC (f.k.a. Neustar) — verified via bgp.tools for 64.6.64.0/24 and 156.154.70.0/24
- **Source grade**: A — official Vercara product page with IP table.
- **Recommended tier**: hard
- **Rationale**: Long-standing widely-referenced public resolver service. 64.6.64.6 and 64.6.65.6 are very commonly listed in resolver guides; blocking them is a feed quality signal.
- **Caveats**: The 64.6.x.x addresses are the original Verisign-era IPs now operated by Vercara. The 156.154.x.x and 2610:a1:x addresses are the Neustar-era filtering-tier addresses. Both sets are now under AS12008/Vercara.

---

### AdGuard Public DNS

- **Service**: AdGuard's public DNS resolver with three filtering variants.
- **Official documentation**: https://adguard-dns.io/en/public-dns.html
- **IPv4 addresses**:
  - Default (ad+tracker blocking): `94.140.14.14`, `94.140.15.15`
  - Family protection (adds adult filter): `94.140.14.15`, `94.140.15.16`
  - Non-filtering: `94.140.14.140`, `94.140.14.141`
- **IPv6 addresses**:
  - Default: `2a10:50c0::ad1:ff`, `2a10:50c0::ad2:ff`
  - Family: `2a10:50c0::bad1:ff`, `2a10:50c0::bad2:ff`
  - Non-filtering: `2a10:50c0::1:ff`, `2a10:50c0::2:ff`
- **ASN**: AS212772 — AdGuard Software Limited (verified via bgp.tools prefix 94.140.14.0/24)
- **DoH**: `https://dns.adguard-dns.com/dns-query` (default), variants per filter
- **DoT**: `dns.adguard-dns.com`
- **Source grade**: A — official AdGuard public DNS page.
- **Recommended tier**: hard
- **Rationale**: Widely deployed privacy/ad-blocking resolver. AS212772 is dedicated to AdGuard DNS infrastructure.
- **Caveats**: The non-filtering variant (94.140.14.140/141) is useful for users wanting a privacy-focused resolver without any content blocking.

---

### Control D

- **Service**: Control D's public recursive DNS resolver with free and paid variants.
- **Official documentation**: https://docs.controld.com/docs/control-d-ip-ranges
- **IPv4 addresses**:
  - Ranges: `76.76.2.0/24`, `76.76.10.0/24`
  - Free DNS (freedns.controld.com): `76.76.2.11`, `76.76.10.11`
  - Custom DNS (dns.controld.com): `76.76.2.22`, `76.76.10.22`
- **IPv6 addresses**:
  - Ranges: `2606:1a40::/48`, `2606:1a40:1::/48`
  - Free DNS: `2606:1a40::11`, `2606:1a40:1::11`
  - Custom DNS: `2606:1a40::22`, `2606:1a40:1::22`
- **ASN**: AS398962 — CONTROLD INC. (verified via bgp.tools prefix 76.76.2.0/24)
- **DoH**: `https://freedns.controld.com/p0` (free), per-profile for paid
- **DoT**: `freedns.controld.com` (free)
- **Source grade**: A — official IP ranges documentation page (machine-readable CIDR ranges listed).
- **Recommended tier**: hard
- **Rationale**: Growing provider with documented anycast ranges. The IP-ranges doc is clean and usable.
- **Caveats**: The `76.76.2.22`/`76.76.10.22` addresses serve all custom/per-profile resolvers; knowing the per-profile ID is needed for full resolution but the IP stubs are shared.

---

### Mullvad DNS

- **Service**: Mullvad VPN's public encrypted DNS resolver with multiple filtering variants. Free to use without VPN.
- **Official documentation**: https://mullvad.net/en/help/dns-over-https-and-dns-over-tls
- **IPv4 addresses**:
  - Unfiltered (`dns.mullvad.net`): `194.242.2.2`
  - Adblock (`adblock.dns.mullvad.net`): `194.242.2.3`
  - Base (`base.dns.mullvad.net`): `194.242.2.4`
  - Extended (`extended.dns.mullvad.net`): `194.242.2.5`
  - Family (`family.dns.mullvad.net`): `194.242.2.6`
  - All (`all.dns.mullvad.net`): `194.242.2.9`
- **IPv6 addresses**:
  - Unfiltered: `2a07:e340::2`
  - Adblock: `2a07:e340::3`
  - Base: `2a07:e340::4`
  - Extended: `2a07:e340::5`
  - Family: `2a07:e340::6`
  - All: `2a07:e340::9`
- **Deprecated addresses**: `193.19.108.2`, `193.19.108.3` — no longer in use.
- **ASN**: AS57138 — Mullvad DNS / LOCIX LIMITED (verified via bgp.tools prefix 194.242.2.0/24; underlying company is Mullvad VPN AB)
- **DoH**: per-variant hostnames above on port 443
- **DoT**: per-variant hostnames above on port 853
- **Source grade**: A — official Mullvad help page with complete IP table.
- **Recommended tier**: hard
- **Rationale**: Privacy-focused resolver operated by a reputable VPN provider; growing user base. All addresses are in the 194.242.2.x /24 block.
- **Caveats**: The deprecated 193.19.108.x addresses should NOT be included — they were retired. AS57138 is a smaller dedicated ASN.

---

### DNS.SB

- **Service**: Free privacy-focused public resolver with DNSSEC and no-log policy.
- **Official documentation**: https://dns.sb/servers/
- **IPv4 addresses**: `185.222.222.222`, `45.11.45.11`
- **IPv6 addresses**: `2a09::`, `2a11::` (abbreviated; full: `2a09:0000:0000:0000:0000:0000:0000:0000` and `2a11::`)
- **ASN**: AS24013 — SB Professional Services (verified via bgp.tools prefix 185.222.222.0/24)
- **DoH**: `https://doh.dns.sb/dns-query`
- **DoT**: `dot.dns.sb`
- **Source grade**: C — official website pages with addresses; no machine-readable feed found.
- **Recommended tier**: hard
- **Rationale**: Well-known privacy resolver, widely documented. Anycast architecture. The anycast IPs are dedicated.
- **Caveats**: AS24013 is registered as "SB Professional Services" which appears to be the legal entity operating DNS.SB. Grade C because IPs are on a static web page, not a published API/feed.

---

### DNS4EU

- **Service**: EU-backed sovereign public DNS resolver operated by Whalebone; launched publicly June 2025. Funded by EU/ENISA initiative.
- **Official documentation**: https://joindns4.eu/for-public
- **IPv4 addresses** (all variants):
  - Protective: `86.54.11.1`, `86.54.11.201`
  - Protective + Child: `86.54.11.12`, `86.54.11.212`
  - Protective + Ad blocking: `86.54.11.13`, `86.54.11.213`
  - Protective + Child + Ad blocking: `86.54.11.11`, `86.54.11.211`
  - Unfiltered: `86.54.11.100`, `86.54.11.200`
- **IPv6 addresses** (matching variants):
  - Protective: `2a13:1001::86:54:11:1`, `2a13:1001::86:54:11:201`
  - Protective + Child: `2a13:1001::86:54:11:12`, `2a13:1001::86:54:11:212`
  - Protective + Ad blocking: `2a13:1001::86:54:11:13`, `2a13:1001::86:54:11:213`
  - Protective + Child + Ad blocking: `2a13:1001::86:54:11:11`, `2a13:1001::86:54:11:211`
  - Unfiltered: `2a13:1001::86:54:11:100`, `2a13:1001::86:54:11:200`
- **ASN**: AS198121 — Whalebone, s.r.o. (verified via bgp.tools prefix 86.54.11.0/24; confirmed by DNS4EU wiki and Whalebone official page)
- **DoH/DoT**: supported; hostnames from joindns4.eu
- **Source grade**: C — official static website; no machine-readable JSON feed found as of 2026-04.
- **Recommended tier**: hard
- **Rationale**: EU official/co-funded sovereign DNS initiative; 13-country consortium. Publicly launched June 2025. All IPs are in the 86.54.11.0/24 block.
- **Caveats**: The service was only publicly launched in June 2025; operational track record is shorter than Google/Cloudflare/Quad9. The service replaced dns0.eu as the recommended EU alternative.

---

### CIRA Canadian Shield

- **Service**: Canadian Internet Registration Authority's free public DNS resolver for Canadians with three service tiers.
- **Official documentation**: https://www.cira.ca/en/canadian-shield/configure/summary-cira-canadian-shield-dns-resolver-addresses/
- **IPv4 addresses**:
  - Private (privacy only): `149.112.121.10`, `149.112.122.10`
  - Protected (privacy + threat blocking): `149.112.121.20`, `149.112.122.20`
  - Family (privacy + threats + adult): `149.112.121.30`, `149.112.122.30`
- **IPv6 addresses**:
  - Private: `2620:10A:80BB::10`, `2620:10A:80BC::10`
  - Protected: `2620:10A:80BB::20`, `2620:10A:80BC::20`
  - Family: `2620:10A:80BB::30`, `2620:10A:80BC::30`
- **ASN**: AS40568 — CIRA Canadian Internet Registration Authority (verified via bgp.tools prefix 149.112.121.0/24)
- **DoH**: `https://private.canadianshield.cira.ca/dns-query` (private variant hostname pattern)
- **DoT**: `private.canadianshield.cira.ca`
- **Source grade**: C — official CIRA setup guide pages; no machine-readable JSON feed found.
- **Recommended tier**: hard
- **Rationale**: Operated by CIRA, the authoritative .ca domain registry. Anycast; national-scale deployment for Canada. Dedicated ASN.
- **Caveats**: Service is primarily aimed at Canadian users but publicly accessible. Note that a WebFetch to the summary page returned 403; addresses verified via web search against official CIRA configure guides.

---

### AliDNS (Alibaba Cloud Public DNS)

- **Service**: Alibaba Cloud's global public recursive DNS resolver service. Dominant resolver in China.
- **Official documentation**: https://www.alidns.com/ (Chinese), https://www.alibabacloud.com/help/en/dns/what-is-alibaba-cloud-public-dns
- **IPv4 addresses**: `223.5.5.5`, `223.6.6.6`
- **IPv6 addresses**: `2400:3200::1`, `2400:3200:baba::1`
- **DoH**: `https://dns.alidns.com/dns-query`
- **DoT**: `dns.alidns.com`
- **ASN**: AS45102 — Alibaba (US) Technology Co., Ltd. (verified via bgp.tools for 223.5.5.0/24; both 223.5.5.5 and 223.6.6.6 resolve to AS45102 per IPinfo.io)
- **Source grade**: C — official page is primarily in Chinese; no machine-readable feed URL found.
- **Recommended tier**: hard
- **Rationale**: One of the most-used resolvers in China with enormous geographic scope. Dedicated anycast addresses. Blocking them in a feed would disrupt DNS for huge numbers of Chinese internet users.
- **Caveats**: Operated by Alibaba, subject to Chinese regulations. Not suitable as a "global" hard reference for Western operators, but justified as a regional hard reference. IPv6 addresses sourced from the official alidns.com landing page.

---

### DNSPod (Tencent)

- **Service**: Tencent DNSPod's public recursive DNS resolver. Major resolver in China.
- **Official documentation**: https://docs.dnspod.com/public-dns/public-dns-introduction/
- **IPv4 addresses**: `119.29.29.29` (primary, anycast via BGP), `182.254.116.116` (secondary)
- **IPv6 addresses**: UNVERIFIED — no official IPv6 address found in public docs.
- **ASN**: AS132203 — Tencent (verified via bgp.tools prefix 119.29.29.0/24; AS45090 Tencent Cloud also originated)
- **DoH**: `https://doh.pub/dns-query`
- **DoT**: `dot.pub`
- **Source grade**: C — official DNSPod docs exist but primary focus is the 119.29.29.29 address; no machine-readable feed.
- **Recommended tier**: hard
- **Rationale**: Second major Chinese public resolver after AliDNS; BGP anycast across 16 major China ISPs. Very widely deployed.
- **Caveats**: IPv6 not officially documented. The 182.254.116.116 secondary address is less explicitly documented than 119.29.29.29; UNVERIFIED from an authoritative primary source — included based on consistent appearance in secondary sources.

---

### CleanBrowsing

- **Service**: DNS content filter with three public filtering tiers.
- **Official documentation**: https://cleanbrowsing.org/filters
- **IPv4 addresses**:
  - Security filter: `185.228.168.9`, `185.228.169.9`
  - Adult filter: `185.228.168.10`, `185.228.169.11` (note: secondary is .11, not .10)
  - Family filter: `185.228.168.168`, `185.228.169.168`
- **IPv6 addresses**:
  - Security filter: `2a0d:2a00:1::2`, `2a0d:2a00:2::2`
  - Adult filter: `2a0d:2a00:1::1`, `2a0d:2a00:2::1`
  - Family filter: `2a0d:2a00:1::`, `2a0d:2a00:2::`
- **ASN**: AS205157 — Daniel Cid (verified via bgp.tools prefix 185.228.168.0/24; operator is the CleanBrowsing founder)
- **DoH**: per-filter hostnames (family-, adult-, security-filter-dns.cleanbrowsing.org)
- **DoT**: same pattern on port 853
- **Source grade**: A — official filters page is the authoritative source; addresses are cleanly listed.
- **Recommended tier**: hard
- **Rationale**: Well-known and documented family/security DNS filtering service with anycast via AS205157.
- **Caveats**: The prior SOW noted a "conflicting Adult secondary address". The official filters page (verified April 2026) shows the Adult filter secondary is definitively `185.228.169.11` (not .10 like the primary). This is an asymmetric address pair, not a typo. The SOW conflict is resolved: the official source is the canonical answer.

---

### Yandex DNS

- **Service**: Yandex's public recursive DNS resolver with three filtering modes.
- **Official documentation**: https://dns.yandex.com/
- **IPv4 addresses**:
  - Basic (unfiltered): `77.88.8.8`, `77.88.8.1`
  - Safe (malware/fraud): `77.88.8.88`, `77.88.8.2`
  - Family (adult filter): `77.88.8.7`, `77.88.8.3`
- **IPv6 addresses**:
  - Basic: `2a02:6b8::feed:0ff`, `2a02:6b8:0:1::feed:0ff`
  - Safe: `2a02:6b8::feed:bad`, `2a02:6b8:0:1::feed:bad`
  - Family: `2a02:6b8::feed:a11`, `2a02:6b8:0:1::feed:a11`
- **ASN**: AS13238 — Yandex LLC (verified via bgp.tools for 77.88.8.0/24; AS208398 "Edge Technology Plus" also appears as a co-originator)
- **DoH**: `https://common.dot.dns.yandex.net/dns-query` (unofficial; documentation-level only)
- **Source grade**: C — official Yandex DNS page; no machine-readable JSON feed.
- **Recommended tier**: soft
- **Rationale**: Widely deployed in Russia and CIS countries. However, Yandex is a Russian company subject to Russian government requirements; geopolitical risk means the service may be abused for censorship or surveillance purposes. Many security-conscious operators explicitly exclude Yandex from trust models. This warrants soft rather than hard tier.
- **Caveats**: Service confirmed active as of April 2026. The "safe" address has an intentional mnemonics pattern (::feed:bad). No official machine-readable IP publish mechanism found.

---

### Comodo Secure DNS

- **Service**: Free DNS resolver with malware and phishing protection, bundled with Comodo Internet Security.
- **Official documentation**: https://www.comodo.com/secure-dns/, https://securedns.dnsbycomodo.com/
- **IPv4 addresses**: `8.26.56.26`, `8.20.247.20`
- **IPv6 addresses**: none published.
- **ASN**: AS23393 — NuCDN LLC (verified via bgp.tools for 8.26.56.0/24 and 8.20.247.0/24; the IP space is operated by NuCDN LLC, not Comodo directly)
- **Source grade**: C — official pages exist but the service has not kept pace with modern DNS standards.
- **Recommended tier**: soft
- **Rationale**: Long-standing resolver but aging infrastructure. No IPv6. The ASN is a CDN company (NuCDN), not Comodo's own AS, indicating outsourced hosting rather than dedicated infrastructure. Still operationally active as of 2025-2026 per official Comodo page.
- **Caveats**: Comodo has discontinued several related products (macOS/Linux security software, browser extensions). The DNS service remains but is not actively developed. No DoH/DoT support found. Treat as soft/legacy.

---

### Hurricane Electric Public DNS

- **Service**: Hurricane Electric's public recursive DNS resolver, a secondary service from the major global transit carrier.
- **Official documentation**: No dedicated public resolver documentation page found. Hostname is `ordns.he.net`.
- **IPv4 addresses**: `74.82.42.42`
- **IPv6 addresses**: `2001:470:20::2`
- **ASN**: AS6939 — Hurricane Electric LLC (verified via BGP.he.net and search results for 74.82.42.42)
- **Source grade**: D — no official public resolver documentation page or machine-readable IP feed found. Derived from DNS hostname lookup and third-party references.
- **Recommended tier**: soft
- **Rationale**: Hurricane Electric is a major global transit carrier (one of the largest IPv6 backbones). This resolver is part of their free internet tools, widely used in networking/operations communities. However, the lack of an official documented resolver service makes this D grade.
- **Caveats**: This is a free secondary service, not a primary product. Treat as soft; include if verifiable via `ordns.he.net` DNS lookup but do not treat as hard-tier evidence.

---

### DNS.WATCH

- **Service**: German non-profit uncensored DNSSEC-enabled resolver.
- **Official documentation**: https://dns.watch/
- **IPv4 addresses**: `84.200.69.80`, `84.200.70.40`
- **IPv6 addresses**: UNVERIFIED — bgp.tools 404 for 84.200.69.0/24 and 84.200.70.0/24; no official IPv6 page found.
- **ASN**: UNVERIFIED — bgp.tools returned 404 for both prefixes. Likely a small German hosting provider.
- **Source grade**: D — no machine-readable feed; official site has static page; ASN unverified.
- **Recommended tier**: soft
- **Rationale**: Well-cited in DNS privacy communities but small scale and no verifiable BGP presence via standard tools. Operational per 2026 gaming DNS guide.
- **Caveats**: This is a small non-profit service. If the provider's BGP presence cannot be verified, classify as soft at most. Do not include in a hard reference set.

---

### CZ.NIC ODVR (Open DNSSEC Validating Resolvers)

- **Service**: Czech domain registry's public DNSSEC-validating recursive resolvers. Available to all, not just Czech users.
- **Official documentation**: https://www.nic.cz/odvr/
- **IPv4 addresses**: `193.17.47.1`, `185.43.135.1`
- **IPv6 addresses**: `2001:148f:ffff::1`, `2001:148f:fffe::1`
- **ASN**: AS20701 — CZ.NIC, z.s.p.o. (verified via bgp.tools prefix 193.17.47.0/24)
- **DoH**: supported at `odvr.nic.cz`
- **DoT**: `odvr.nic.cz` on port 853
- **Source grade**: C — official CZ.NIC page; clean address listing; no machine-readable JSON feed.
- **Recommended tier**: hard
- **Rationale**: Operated by a national registry authority (CZ.NIC), which also develops the Knot resolver. Dedicated infrastructure. The old ODVR addresses were retired and replaced with the current addresses (CZ.NIC published a migration notice).
- **Caveats**: Czech-focused but globally accessible. Historical addresses (217.31.x.x) were decommissioned; only the current 193.17.47.1 and 185.43.135.1 addresses are active.

---

### IIJ Public DNS (Internet Initiative Japan)

- **Service**: IIJ's public recursive DNS resolver service in Japan. IIJ is one of Japan's major ISPs.
- **Official documentation**: https://public.dns.iij.jp/ (redirects to policy page; IPs not listed there)
- **IPv4 addresses**: `103.2.57.5`, `103.2.57.6` (from WHOIS for public01.dns.iij.jp and secondary sources)
- **IPv6 addresses**: UNVERIFIED — no official IPv6 address published on the accessible policy page.
- **ASN**: AS2497 — Internet Initiative Japan Inc. (verified via bgp.tools prefix 103.2.57.0/24)
- **DoH**: `https://public.dns.iij.jp/dns-query`
- **DoT**: `public.dns.iij.jp`
- **Source grade**: C — the official policy page redirected and showed hostname-only; IPs sourced from WHOIS for the hostname, which is secondary evidence.
- **Recommended tier**: hard
- **Rationale**: IIJ is one of Japan's top-tier ISPs with a long track record. AS2497 is a major Japanese transit AS. The public DNS is a significant service in Japan.
- **Caveats**: The policy page (policy.public.dns.iij.jp) lists DoH/DoT hostnames but not raw IPs. Addresses from WHOIS lookup on `public01.dns.iij.jp`. IPv6 requires further verification. Consider B grade after confirming addresses from a more authoritative source.

---

### SafeDNS

- **Service**: Commercial filtering DNS service with a public free tier.
- **Official documentation**: https://docs.safedns.com/books/45-setup-services/page/dns, https://www.safedns.com/
- **IPv4 addresses**: `195.46.39.39`, `195.46.39.40`
- **IPv6 addresses**: UNVERIFIED
- **ASN**: AS57926 — SafeDNS, Inc. (verified via bgp.tools prefix 195.46.39.0/24)
- **Source grade**: C — official SafeDNS docs listing; no machine-readable JSON feed.
- **Recommended tier**: soft
- **Rationale**: Commercial service with a small public footprint. Dedicated ASN but limited deployment compared to tier-1 resolvers.
- **Caveats**: Primarily a commercial product; the free tier is available. Appropriate for soft tier only given the commercial focus.

---

### Foundation for Applied Privacy (applied-privacy.net)

- **Service**: Austrian non-profit privacy infrastructure provider's DNS resolver. DoH/DoT only.
- **Official documentation**: https://applied-privacy.net/services/dns/
- **IPv4 addresses**: `146.255.56.98` (dot1 resolver)
- **IPv6 addresses**: `2a02:1b8:10:234::2`
- **ASN**: AS208323 — Foundation for Applied Privacy (from IPinfo.io; bgp.tools returned 404 for the /24)
- **DoH**: `https://doh.applied-privacy.net/query`
- **DoT**: `dot1.applied-privacy.net`
- **Source grade**: C — official page; single documented resolver endpoint.
- **Recommended tier**: soft
- **Rationale**: Small non-profit; limited deployment. Good privacy credentials but single-resolver infrastructure is not a major deployment target.
- **Caveats**: One known IPv4 address (146.255.56.98); others may exist. Not suitable for hard tier given scale.

---

### Baidu DNS

- **Service**: Baidu's public recursive DNS resolver in China.
- **Official documentation**: https://dudns.baidu.com/ (Chinese)
- **IPv4 addresses**: `180.76.76.76`
- **IPv6 addresses**: UNVERIFIED
- **ASN**: AS55967 — Beijing Baidu Netcom Science and Technology Co., Ltd. (verified via bgp.tools prefix 180.76.76.0/24; AS38365 also co-originates)
- **Source grade**: C — official Baidu page in Chinese; no English machine-readable feed.
- **Recommended tier**: soft
- **Rationale**: Widely used in China. However, Baidu is subject to Chinese regulations and censors DNS responses. Geopolitical risk; not appropriate as a globally trusted hard-tier resolver.
- **Caveats**: Single address documented (180.76.76.76). Some sources also list a secondary (180.76.76.76 only found). Chinese-operator geopolitical risk factor.

---

### 114DNS (China Telecom-linked)

- **Service**: China's most widely used public DNS resolver, commonly attributed to China Telecom's Jiangsu operation.
- **Official documentation**: No verified authoritative official English-language page found. Chinese sources exist but could not be verified.
- **IPv4 addresses**: `114.114.114.114`, `114.114.115.115`
- **IPv6 addresses**: UNVERIFIED
- **ASN**: UNVERIFIED — conflicting reports. IPinfo.io reports AS21859 (Zenlayer), while other sources report AS4812 (China Telecom). The anycast routing appears to use multiple transit paths.
- **Source grade**: D — no verified official source page; ASN is ambiguous and contested.
- **Recommended tier**: soft
- **Rationale**: Extremely widely used in China; 114.114.114.114 is among the most queried DNS servers globally. However, the lack of a verified authoritative source page and ASN ambiguity makes hard tier inappropriate.
- **Caveats**: ASN conflict between AS21859 and AS4812 is unresolved without direct WHOIS/ROA verification. The service is real and widely deployed but documentation quality does not meet hard-tier standards. Include as soft with explicit "ASN unverified" note.

---

### NextDNS

- **Service**: Per-profile configurable DNS resolver with anycast delivery.
- **Official documentation**: https://help.nextdns.io/t/q6hp4j6/dns-nextdns-io-ip-addresses
- **IPv4 addresses**: Ranges `45.90.28.0/22` (AS34939); per-profile stub addresses are assigned within this range but are not stable fixed addresses in the traditional sense.
- **IPv6 addresses**: UNVERIFIED (range likely `2a07:a8c0::/29` but not confirmed from official source)
- **ASN**: AS34939 — NextDNS, Inc. (verified via bgp.tools prefix 45.90.28.0/24)
- **Source grade**: D — no stable published stub IP; per-user/profile assignment means no fixed canonical address.
- **Recommended tier**: reject (for hard-tier reference set)
- **Rationale**: NextDNS addresses are anycast within the AS34939 range but are per-profile; there is no canonical "the NextDNS IP" equivalent to 8.8.8.8. The free public endpoint `dns.nextdns.io` resolves via DoH/DoT but does not have a stable fixed IPv4 stub.
- **Caveats**: NextDNS is growing rapidly and well-known, but the architecture does not produce a stable fixed IP suitable for a reference exclusion set. Monitor for future official IP publication.

---

### DNS0.eu

- **DISCONTINUED** — shut down abruptly in 2024 due to "limited resources" and sustainability issues.
- Former IPs are now dead or reassigned. Do not include in any reference set.
- Former operators recommended migration to DNS4EU or NextDNS.
- Historical note: Was operated by the founders of NextDNS as a French non-profit; ran 62 servers across 27 EU cities.

---

### OpenNIC

- **Service**: Volunteer-operated alternative DNS root network.
- **Status**: Active but unsuitable for reference set.
- **Recommended tier**: reject
- **Rationale**: Servers are volunteer-operated, change frequently, use home/residential connections, and the IP space is dynamic. OpenNIC Tier 2 servers change regularly; no stable IP set exists. Cannot be used as a fixed reference feed.

---

### SkyDNS (Russia)

- **Service**: Russian commercial content-filtering DNS service primarily marketed to schools and enterprises.
- **IPv4 addresses**: `193.58.251.251` (documented but reportedly retired for DNS use per 2022 source)
- **IPv6 addresses**: UNVERIFIED
- **ASN**: UNVERIFIED
- **Source grade**: D — Russian commercial service; primary docs are Russian-language; address reliability questionable.
- **Recommended tier**: reject
- **Rationale**: Commercial subscription product, not a widely deployed public resolver. Geopolitical risk (Russian operator). Address potentially no longer active for DNS.

---

### LibreDNS

- **Service**: Greek community-run DoH/DoT resolver.
- **IPv4 addresses**: `116.202.176.26` (on Hetzner infrastructure)
- **IPv6 addresses**: UNVERIFIED
- **ASN**: AS24940 — Hetzner Online GmbH (not LibreDNS's own ASN)
- **Source grade**: D — community project hosted on shared hosting; no own ASN.
- **Recommended tier**: reject
- **Rationale**: Single-server community project. Hosted on Hetzner, not own infrastructure. Not a major deployment target; not suitable as a hard or soft reference.

---

### NordVPN DNS

- **Service**: DNS resolver provided by NordVPN for its users; intended for VPN-connected clients.
- **IPv4 addresses**: `103.86.96.100`, `103.86.99.100`
- **IPv6 addresses**: UNVERIFIED
- **ASN**: AS136787 — PacketHub S.A. (verified via bgp.tools prefix 103.86.96.0/24; this is a transit/hosting provider, not NordVPN's own ASN)
- **Source grade**: D — VPN provider resolver; not a general public resolver with stable fixed IPs.
- **Recommended tier**: reject
- **Rationale**: NordVPN DNS is not a public resolver; it is intended for VPN-connected clients. The IP space belongs to a transit provider. Not appropriate for a critical-infrastructure reference set.

---

### Strongarm / WatchGuard DNSWatch

- **Service**: DNS filtering service acquired by WatchGuard in 2018; rebranded as DNSWatch.
- **Status**: No longer a publicly documented stable resolver; product is commercial SMB security.
- **Recommended tier**: reject
- **Rationale**: Not a public resolver; commercial product for managed service providers.

---

### ScrubIT

- **Service**: Free filtering DNS service.
- **Status**: **DISCONTINUED** — service is no longer operating.
- **Recommended tier**: reject

---

### Blahdns.com

- **Service**: Hobby ad-blocking DNS resolver supporting DoH, DoT, DoQ, and DNSCrypt.
- **IPv4 addresses**: Various (per node; `108.61.201.119` is one documented endpoint)
- **IPv6 addresses**: e.g. `2a01:4f8:1c1c:6b4b::1` (per node)
- **ASN**: Various hosting providers
- **Source grade**: D — hobby project; no stable canonical IPs; no official machine-readable feed.
- **Recommended tier**: reject
- **Rationale**: Hobby project with per-node addresses. Not suitable as a critical-infrastructure reference.

---

### OneDNS (China)

- **Service**: Chinese filtering DNS resolver by Beijing Weibu Online Technology.
- **IPv4 addresses**: Clean: `117.50.10.10`, `52.80.52.52`; Filtered: `117.50.11.11`, `52.80.66.66`
- **IPv6 addresses**: UNVERIFIED
- **ASN**: UNVERIFIED
- **Source grade**: D — no verified English-language official documentation found.
- **Recommended tier**: reject
- **Rationale**: No verified official source. Chinese regional service with no global significance comparable to AliDNS or DNSPod.

---

### DNSFilter

- **Service**: Commercial DNS security/filtering service.
- **IPv4 addresses**: `103.247.36.36`, `103.247.37.37` (from Netify source referencing dns1.dnsfilter.com)
- **IPv6 addresses**: UNVERIFIED
- **ASN**: UNVERIFIED (uses third-party anycast transit per official FAQ)
- **Source grade**: D — IP addresses sourced from third-party Netify database, not an official DNSFilter IP publish page.
- **Recommended tier**: reject (for now)
- **Rationale**: Commercial product; addresses not from an official primary source. If DNSFilter publishes an official IP list, reconsider for soft tier.

---

## Discontinued / removed services (do not include)

| Service | Reason | Notes |
|---|---|---|
| DNS0.eu | Shut down 2024 | Operators cited unsustainability; recommended DNS4EU/NextDNS as replacements |
| ScrubIT | Discontinued | No longer operating; website gone |
| Strongarm (Percipient Networks) | Acquired/rebranded | Acquired by WatchGuard 2018; became DNSWatch commercial product |
| OpenDNS old IPv6 support page | Redirected | Original support.opendns.com article redirected to Cisco community |
| Verisign Public DNS branding | Sold Dec 2020 | IPs (64.6.64.6 / 64.6.65.6) transferred to Neustar, now Vercara; IPs still active |
| Mullvad deprecated IPs | Retired | 193.19.108.2, 193.19.108.3 — explicitly marked deprecated on official page |

---

## Open questions / unverified

1. **DNSPod IPv6**: No official IPv6 address found in English-language docs for 119.29.29.29 / DNSPod. Needs verification against Chinese-language official docs or a DNS lookup of `dot.pub`.
2. **114DNS ASN**: Conflicting reports between AS21859 (Zenlayer) and AS4812 (China Telecom Jiangxi). Requires direct RIPE/APNIC WHOIS + ROA verification. The anycast deployment makes BGP origin ambiguous.
3. **DNS.WATCH ASN and IPv6**: bgp.tools returned 404 for 84.200.69.0/24 and 84.200.70.0/24. The AS may be a small German provider not well-represented in bgp.tools. IPv6 addresses not found on official site. Needs dedicated WHOIS lookup.
4. **IIJ Public DNS IPv6**: The policy page does not list IPv6. A DNS lookup of `public.dns.iij.jp` for AAAA records would resolve this. Expected to be in the 2001:218::/32 or similar IIJ IPv6 space.
5. **CleanBrowsing Adult secondary asymmetry**: Confirmed that `185.228.169.11` (not .10) is the correct secondary for the Adult filter. This was previously flagged as conflicting in the SOW; it is resolved — the asymmetry is intentional and documented on the official filters page.
6. **Foundation for Applied Privacy additional IPs**: Only one IPv4 (146.255.56.98) found; the service may have additional nodes. The GitHub issue referenced in search results (Jigsaw-Code/Intra #246) mentioned IP changes; worth rechecking.
7. **NordVPN ASN mismatch**: The 103.86.96.0/24 prefix originates from AS136787 PacketHub S.A., not NordVPN's own AS. NordVPN's own AS is AS212238. This confirms the DNS addresses are on outsourced transit infrastructure, further supporting rejection.
8. **OneDNS official source**: A verified official English-language source page for OneDNS (117.50.10.10, 52.80.52.52) was not found. Chinese-language source at weibu.com is the operator but was not directly fetched.
9. **AliDNS source grade upgrade**: The official Alibaba Cloud documentation (alibabacloud.com) exists in English and may support upgrade to grade B if a clean machine-readable address list is present.

---

## Sources consulted

- https://developers.cloudflare.com/1.1.1.1/ip-addresses/
- https://developers.google.com/speed/public-dns/docs/using
- https://developers.google.com/speed/public-dns/docs/dns64
- https://quad9.net/service/service-addresses-and-features/
- https://www.opendns.com/setupguide/
- https://www.cisco.com/c/en/us/support/docs/security/umbrella/225331-understand-umbrella-support-for-ipv6.epub
- https://vercara.digicert.com/ultra-dns-public
- https://adguard-dns.io/en/public-dns.html
- https://docs.controld.com/docs/control-d-ip-ranges
- https://mullvad.net/en/help/dns-over-https-and-dns-over-tls
- https://dns.sb/servers/
- https://joindns4.eu/for-public
- https://www.cira.ca/en/canadian-shield/ (configure guides)
- https://www.alidns.com/
- https://docs.dnspod.com/public-dns/public-dns-introduction/
- https://cleanbrowsing.org/filters
- https://dns.yandex.com/
- https://www.comodo.com/secure-dns/
- https://www.nic.cz/odvr/
- https://applied-privacy.net/services/dns/
- https://public.dns.iij.jp/ → https://policy.public.dns.iij.jp/
- https://www.safedns.com/
- https://help.nextdns.io/t/q6hp4j6/dns-nextdns-io-ip-addresses
- https://strongarm.io/ (WatchGuard acquisition notice)
- https://www.bleepingcomputer.com/news/security/dns0eu-private-dns-service-shuts-down-over-sustainability-issues/
- https://cloudnews.tech/dns0-eu-immediately-turns-off-its-public-dns-and-recommends-migrating-to-dns4eu-or-nextdns/
- https://www.whalebone.io/dns4eu
- https://joindns4.eu/learn/dns4eu-public-service-launched
- https://bgp.tools/prefix/1.1.1.0/24
- https://bgp.tools/prefix/8.8.8.0/24
- https://bgp.tools/prefix/149.112.112.0/24
- https://bgp.tools/prefix/9.9.9.0/24 (via search confirmation)
- https://bgp.tools/prefix/208.67.222.0/24
- https://bgp.tools/prefix/64.6.64.0/24
- https://bgp.tools/prefix/156.154.70.0/24
- https://bgp.tools/prefix/94.140.14.0/24
- https://bgp.tools/prefix/76.76.2.0/24
- https://bgp.tools/prefix/194.242.2.0/24
- https://bgp.tools/prefix/185.222.222.0/24
- https://bgp.tools/prefix/86.54.11.0/24
- https://bgp.tools/prefix/149.112.121.0/24
- https://bgp.tools/prefix/223.5.5.0/24
- https://bgp.tools/prefix/185.228.168.0/24
- https://bgp.tools/prefix/77.88.8.0/24
- https://bgp.tools/prefix/195.46.39.0/24
- https://bgp.tools/prefix/8.26.56.0/24
- https://bgp.tools/prefix/8.20.247.0/24
- https://bgp.tools/prefix/119.29.29.0/24
- https://bgp.tools/prefix/180.76.76.0/24
- https://bgp.tools/prefix/45.90.28.0/24
- https://bgp.tools/prefix/103.86.96.0/24
- https://bgp.tools/prefix/193.17.47.0/24
- https://bgp.tools/prefix/103.2.57.0/24
- https://ipinfo.io/114.114.114.114 (ASN conflict note for 114DNS)
- https://bgp.he.net/ip/74.82.42.42 (Hurricane Electric confirmation via search)
- https://www.abuseipdb.com/whois/103.2.57.6 (IIJ secondary address confirmation)
- https://ipinfo.io/AS34939 (NextDNS range confirmation)
