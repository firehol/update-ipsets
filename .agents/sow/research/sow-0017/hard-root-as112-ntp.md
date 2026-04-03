# Hard-tier DNS root, AS112, NTP/time research (SOW-0017)

Research date: 2026-04-29
Purpose: verify exact IPs, ASNs, service prefixes, and source quality for hard-tier
critical infrastructure candidates in the DNS root, AS112, and public NTP/time categories.

---

## DNS Root servers

### Summary table

| Letter | Hostname | IPv4 | IPv6 | Operator | ASN | Anycast instances |
|--------|----------|------|------|----------|-----|-------------------|
| A | a.root-servers.net | 198.41.0.4 | 2001:503:ba3e::2:30 | Verisign, Inc. | AS7342 | 59 |
| B | b.root-servers.net | 170.247.170.2 | 2801:1b8:10::b | University of Southern California, ISI | AS394353 | 6 |
| C | c.root-servers.net | 192.33.4.12 | 2001:500:2::c | Cogent Communications | AS2149 | 13 |
| D | d.root-servers.net | 199.7.91.13 | 2001:500:2d::d | University of Maryland | AS10886 | 231 |
| E | e.root-servers.net | 192.203.230.10 | 2001:500:a8::e | NASA Office of the CIO | AS21556 | 328 |
| F | f.root-servers.net | 192.5.5.241 | 2001:500:2f::f | Internet Systems Consortium (ISC) | AS3557 | 366 |
| G | g.root-servers.net | 192.112.36.4 | 2001:500:12::d0d | Defense Information Systems Agency (DISA) | AS5927 | 6 |
| H | h.root-servers.net | 198.97.190.53 | 2001:500:1::53 | U.S. Army DEVCOM Army Research Lab | AS1508 | 12 |
| I | i.root-servers.net | 192.36.148.17 | 2001:7fe::53 | Netnod | AS29216 | 90 |
| J | j.root-servers.net | 192.58.128.30 | 2001:503:c27::2:30 | Verisign, Inc. | AS26415 | 150 |
| K | k.root-servers.net | 193.0.14.129 | 2001:7fd::1 | RIPE NCC | AS25152 | 152 |
| L | l.root-servers.net | 199.7.83.42 | 2001:500:9f::42 | ICANN | AS20144 | 141 |
| M | m.root-servers.net | 202.12.27.33 | 2001:dc3::35 | WIDE Project | AS7500 | 29 |

**Total instances** (as of April 2026): 2,019 instances operated by 12 independent
organizations (Verisign runs both A and J).

Sources:
- IANA: https://www.iana.org/domains/root/servers
- InterNIC named.root: https://www.internic.net/domain/named.root
- Root Server Technical Operations Association: https://root-servers.org/

### Per-letter details

**A — Verisign (AS7342)**
- IPv4: 198.41.0.4 (announced from /32 inside 198.41.0.0/24)
- IPv6: 2001:503:ba3e::2:30
- 59 anycast instances globally
- ASN AS7342 = VeriSign Infrastructure & Operations
- Operated since 1983 (predates formal root server system)

**B — USC/ISI (AS394353)**
- IPv4: 170.247.170.2 (changed from 192.228.79.201 in 2017)
- IPv6: 2801:1b8:10::b
- 6 anycast instances; smallest cloud among root operators
- ASN AS394353 = B.Root-Server-OPS (created 2017 specifically for B-root)
- Prior to 2017-03-01 origin was a different USC ASN; current ASN is dedicated

**C — Cogent Communications (AS2149)**
- IPv4: 192.33.4.12
- IPv6: 2001:500:2::c
- 13 instances
- ASN AS2149 = Cogent Communications

**D — University of Maryland (AS10886)**
- IPv4: 199.7.91.13
- IPv6: 2001:500:2d::d
- 231 instances (very aggressive anycast deployment)
- ASN AS10886 = University of Maryland

**E — NASA (AS21556)**
- IPv4: 192.203.230.10
- IPv6: 2001:500:a8::e
- 328 instances
- ASN AS21556 = NASA Office of the CIO

**F — ISC (AS3557)**
- IPv4: 192.5.5.241
- IPv6: 2001:500:2f::f
- 366 instances (largest cloud by instance count)
- ASN AS3557 = Internet Systems Consortium
- F-Root operated by ISC since 1994 on behalf of IANA

**G — DISA (AS5927)**
- IPv4: 192.112.36.4
- IPv6: 2001:500:12::d0d
- 6 instances; US federal government operated
- ASN AS5927 = Defense Information Systems Agency

**H — US Army ARL (AS1508)**
- IPv4: 198.97.190.53
- IPv6: 2001:500:1::53
- 12 instances
- ASN AS1508 = U.S. Army DEVCOM Army Research Laboratory

**I — Netnod (AS29216)**
- IPv4: 192.36.148.17
- IPv6: 2001:7fe::53
- 90 instances; Europe-heavy distribution
- ASN AS29216 = Netnod Internet Exchange AB
- Note: Netnod also operates the Swedish SDTS NTP service (separate ASN AS57021)

**J — Verisign (AS26415)**
- IPv4: 192.58.128.30
- IPv6: 2001:503:c27::2:30
- 150 instances
- ASN AS26415 = VeriSign Global Registry Services
- Distinct ASN from A-root (AS7342); both operated by Verisign

**K — RIPE NCC (AS25152)**
- IPv4: 193.0.14.129
- IPv6: 2001:7fd::1
- 152 instances
- ASN AS25152 = RIPE NCC

**L — ICANN (AS20144)**
- IPv4: 199.7.83.42
- IPv6: 2001:500:9f::42
- 141 instances
- ASN AS20144 = ICANN

**M — WIDE Project (AS7500)**
- IPv4: 202.12.27.33
- IPv6: 2001:dc3::35
- 29 instances; Asia-heavy distribution
- ASN AS7500 = WIDE Project / TISF (Japan)

### Source quality for root server feed

- **Grade: A** for IANA page + InterNIC named.root file.
- The InterNIC named.root file (https://www.internic.net/domain/named.root) is an
  official, machine-readable plaintext file listing all 13 IPv4 and IPv6 addresses.
  It is updated when addresses change and is the canonical source used by DNS software
  (BIND, Unbound, etc.) to bootstrap.
- Implementation note for this project: parse named.root at commit time to extract the
  13 IPv4 + 13 IPv6 addresses as a curated static set. Refresh when the file changes.
  Do NOT use root-operator ASNs as the warning unit — the anycast clouds include thousands
  of addresses that are routing infrastructure, not the service addresses.

### Alternative roots — do not include

The following alternative/supplementary root server projects must NOT be included as
critical infrastructure references:

- **ORSN (Open Root Server Network)**: Shut down permanently in May 2019. No current
  operations. Do not include.
- **OpenNIC**: Volunteer-operated, requires non-default resolver configuration,
  unreachable to most users. Operator-facing only; not a shared public dependency.
  Do not include.
- **Yeti DNS**: Research project for IPv6-only root. Not a public production service.
  Do not include.
- Any other alternative root not operated by IANA-designated root server operators:
  by definition, these are not part of the global public DNS resolution dependency
  chain. Do not include.

**Only the 13 IANA-designated root servers (A through M) qualify as hard-tier
critical infrastructure.**

---

## AS112

### Overview

AS112 is an anycast DNS sinkhole project that absorbs reverse-DNS queries for
private (RFC 1918), link-local, and other special-use addresses that leak onto the
public internet. Without AS112, these leaking queries would load the root DNS servers
and the reverse-delegation zones operated by ARIN/RIPE/APNIC. The project is
volunteer-operated and community-governed; it is not an IANA function.

### Service prefixes

| Prefix | Family | Origin AS | RFC | Service | Status | Notes |
|--------|--------|-----------|-----|---------|--------|-------|
| 192.175.48.0/24 | IPv4 | AS112 | RFC 7534 / RFC 6304 | Direct delegation sinkhole | Active | Original 1996 ARIN assignment; covers reverse zones for RFC 1918 space |
| 2620:4f:8000::/48 | IPv6 | AS112 | RFC 7534 | Direct delegation sinkhole | Active | IPv6 equivalent, ARIN assignment |
| 192.31.196.0/24 | IPv4 | AS112 | RFC 7535 | DNAME redirection (EMPTY.AS112.ARPA) | Active | IANA assignment for extended AS112 service |
| 2001:4:112::/48 | IPv6 | AS112 | RFC 7535 | DNAME redirection (EMPTY.AS112.ARPA) | Active | IANA assignment; service address is 2001:4:112::1 |

All four prefixes carry valid RPKI ROAs and are confirmed announced by AS112.
Source: bgp.tools/as/112 (verified April 2026).

### Service addresses within the prefixes

- **PRISONER.IANA.ORG / H.AS112.NET / B.IANA-SERVERS.NET**: nameservers at
  192.175.48.1 and 192.175.48.6 (within the 192.175.48.0/24 service prefix)
- **BLACKHOLE.AS112.ARPA**: nameserver at 192.31.196.1 (IPv4) and 2001:4:112::1
  (IPv6) for the DNAME/EMPTY.AS112.ARPA extended service

### ASN(s)

| ASN | Name | Role |
|-----|------|------|
| AS112 | AS112 Project | Sole origin ASN for all four service prefixes |
| AS5953 | IANA | Historical / administrative only; not currently the origin for service prefixes |

**AS112 is both the ASN number and the project name.** The number 112 was assigned by
ARIN specifically for this project. The ASN is community-operated — no single
organization controls it. Individual operators run AS112 nodes and peer with upstreams
who accept routes from AS112. The project has 42+ upstreams and 465+ peers
(bgp.tools, April 2026).

### Distinction: service prefixes vs node addresses

- **Service prefixes** (what we want in our feed): the four prefixes above. All AS112
  anycast nodes globally announce exactly these four prefixes. Blocking any of these
  prefixes prevents the sinkhole from absorbing the leaking queries, which means those
  queries reach the root servers and/or produce NXDOMAIN responses that can confuse
  poorly-coded DNS clients.
- **Node/operator addresses**: individual AS112 node operators use their own address
  space for management, peering, and administration. These are NOT part of the AS112
  service identity and must NOT be included in the AS112 reference set.

### Source quality for AS112 feed

- **Grade: C** — official source is RFC documents and the as112.net project page.
  The RFCs (RFC 7534, RFC 7535) explicitly specify the service prefixes but are static
  documents. The bgp.tools/as/112 lookup confirms current routing, but is a third-party
  BGP collector, not an official API.
- The four service prefixes are extremely stable (the 192.175.48.0/24 has been used
  since 1996). A manually curated static set is appropriate.
- RFC 8375 (2018) adds 'home.arpa' handling; does not add new service prefixes.

### RFC reference map

| RFC | Status | Summary |
|-----|--------|---------|
| RFC 6304 | Obsoleted by RFC 7534 | Original AS112 node operation guidance |
| RFC 6305 | Current | Explains AS112 to DNS operators encountering it |
| RFC 7534 | Current | Updated AS112 nameserver operations; specifies 192.175.48.0/24 + 2620:4f:8000::/48 |
| RFC 7535 | Current | AS112 redirection via DNAME; adds 192.31.196.0/24 + 2001:4:112::/48 |
| RFC 8375 | Current | Special-use domain 'home.arpa'; routed to AS112 |

---

## Public NTP / time services

### Summary table

| Service | Operator | IPv4 address(es) | IPv6 address(es) | ASN | Leap handling | Source grade | Tier | Notes |
|---------|----------|------------------|-----------------|-----|---------------|--------------|------|-------|
| Cloudflare Time | Cloudflare | 162.159.200.1, 162.159.200.123 | 2606:4700:f1::1, 2606:4700:f1::123 | AS13335 | Strict UTC step (NO smear) | A | Hard | Official docs publish exact IPs |
| Google Public NTP | Google | 216.239.35.0, 216.239.35.4, 216.239.35.8, 216.239.35.12 | 2001:4860:4806::, 2001:4860:4806:4::, 2001:4860:4806:8::, 2001:4860:4806:c:: | AS15169 | Leap smear (24h window) | B | Hard | IPs observed stable but not officially published as static; DNS-derived |
| NIST ITS | NIST/US Gov | 129.6.15.25–30, 132.163.96.1–6, 132.163.97.1–6 | None published | AS49 | Strict UTC step (NO smear) | A | Hard | 16 servers across 3 sites; official page lists exact IPs |
| Netnod SDTS | Netnod/PTS | 194.58.200.0/24–194.58.207.0/24 (range) | 2a01:3f7::/48 (range) | AS57021 | UNVERIFIED | B | Hard | Exact individual IPs not published; anycast via ntp.se; prefix range published |
| PTB Germany | PTB Braunschweig | 192.53.103.103, 192.53.103.104, 192.53.103.108 | 2001:638:610:be01::103/104/108 | AS680 | UNVERIFIED | B | Soft | Official docs now publish hostnames only, not IPs; IPs derived from DNS |
| NPL UK | NPL / UKRI | 139.143.5.30, 139.143.5.31 | UNVERIFIED | UNVERIFIED | UNVERIFIED | C | Soft | Official user guide published IPs; org on Janet/UKRI network |
| NICT Japan | NICT | 133.243.238.163, 133.243.238.164, 133.243.238.243, 133.243.238.244 | UNVERIFIED | AS9355 | UNVERIFIED | B | Soft | IPs known from historic operational status page; NICT advises using hostname |
| NTSC China | CAS/NTSC | 114.118.7.163 (current), 210.72.145.44 (historic) | UNVERIFIED | UNVERIFIED | UNVERIFIED | C | Soft | Limited bandwidth; not for large-scale use; IPs from third-party sources |
| USNO | US Navy | 192.5.41.40, 192.5.41.41, 192.5.41.209 | None published | UNVERIFIED | Strict UTC step (NO smear) | C | Soft | Access restricted to .mil/.gov and stratum-2 by arrangement; public access unclear |
| Meta / Facebook | Meta | Not published (hostname only: time.facebook.com) | Not published | AS32934 | UTC step preferred (Meta advocates removing leap seconds) | D | Reject | No static IPs published; privacy policy explicitly avoids IP fingerprinting |
| TimeNL | SIDN Labs | Not published (hostname only: ntp.time.nl) | Not published | UNVERIFIED | UNVERIFIED | D | Reject | Official docs explicitly say IPs can change; do not use IPs |
| NTP Pool (pool.ntp.org) | NTP Pool Project | Dynamic (changes every few minutes) | Dynamic | Multiple | Mixed (pool members vary) | D | Reject | By design not a stable IP set; operator FAQ explicitly says do not allowlist by IP |

### Per-provider details

---

#### Cloudflare Time Services

- **Official docs**: https://developers.cloudflare.com/time-services/ntp/usage/
- **IPv4**: 162.159.200.1, 162.159.200.123
- **IPv6**: 2606:4700:f1::1, 2606:4700:f1::123
- **Hostname**: time.cloudflare.com
- **ASN**: AS13335 (Cloudflare, Inc.)
- **Leap handling**: Strict UTC step. Cloudflare does NOT smear leap seconds.
  The NTP Leap Indicator field is set per spec; the kernel applies the step correction
  at the moment of the leap. This matches the behavior of pool.ntp.org servers.
- **Source grade: A** — official Cloudflare developer docs explicitly list exact IPs.
- **Tier recommendation: Hard** — dedicated anycast addresses; published as explicit
  static service IPs; low likelihood of being reassigned; blacklisting breaks NTP for
  any device pointing to these IPs.
- **Operational note**: mixing Cloudflare NTP (strict UTC step) with Google NTP
  (leap smear) in the same NTP client configuration can cause timing anomalies.

---

#### Google Public NTP

- **Official docs**: https://developers.google.com/time, https://developers.google.com/time/faq
- **IPv4**: 216.239.35.0, 216.239.35.4, 216.239.35.8, 216.239.35.12
  (time1.google.com through time4.google.com)
- **IPv6**: 2001:4860:4806::, 2001:4860:4806:4::, 2001:4860:4806:8::, 2001:4860:4806:c::
- **Hostname**: time.google.com (also time1–4.google.com)
- **ASN**: AS15169 (Google LLC) — confirmed via ipinfo.io
- **Leap handling**: Leap smear. Google distributes the leap second correction over
  a ±12h window centered on the event, so their clocks read UTC-derived "smeared time"
  rather than strict UTC during and after a leap event. Google explicitly warns: "mixing
  Google Public NTP with other non-smeared NTP services is not recommended." The LI
  (Leap Indicator) field is always set to 0 (no-warning) because the server does
  the smear and does not signal the raw leap event to clients.
- **Source grade: B** — Google's official pages recommend hostname use and do not
  explicitly state these are permanent static IPs. However, the 216.239.35.0/29 range
  is dedicated Google NTP infrastructure; the IPs have been stable since the service
  launched in 2016 and are widely confirmed in official Google Cloud NTP guidance.
- **Tier recommendation: Hard** — these are anycast addresses used by a massive number
  of devices globally. Blocking them constitutes a service disruption. They are not
  reassigned to arbitrary workloads.
- **Operational note**: leap smear means these servers are NOT interchangeable with
  strict-UTC NTP sources. Applications dependent on precise UTC (GPS timing, financial,
  NTP stratum-1 upstreams) must not mix with Google NTP.

---

#### NIST Internet Time Service

- **Official docs**: https://tf.nist.gov/tf-cgi/servers.cgi
- **IPv4** (all 16 servers, three sites):
  - Gaithersburg, Maryland (129.6.15.x):
    - 129.6.15.25 (time-f-g.nist.gov)
    - 129.6.15.26 (time-e-g.nist.gov)
    - 129.6.15.27 (time-d-g.nist.gov)
    - 129.6.15.28 (time-a-g.nist.gov)
    - 129.6.15.29 (time-b-g.nist.gov)
    - 129.6.15.30 (time-c-g.nist.gov)
  - Fort Collins, Colorado / WWV site (132.163.97.x):
    - 132.163.97.1 (time-a-wwv.nist.gov)
    - 132.163.97.2 (time-b-wwv.nist.gov)
    - 132.163.97.3 (time-c-wwv.nist.gov)
    - 132.163.97.4 (time-d-wwv.nist.gov)
    - 132.163.97.6 (time-e-wwv.nist.gov)
  - Boulder, Colorado (132.163.96.x):
    - 132.163.96.1 (time-a-b.nist.gov)
    - 132.163.96.2 (time-b-b.nist.gov)
    - 132.163.96.3 (time-c-b.nist.gov)
    - 132.163.96.4 (time-d-b.nist.gov)
    - 132.163.96.6 (time-e-b.nist.gov)
- **IPv6**: Not published on the official NIST ITS page.
- **Hostname**: time.nist.gov (round-robin across all 16)
- **ASN**: AS49 (National Institute of Standards and Technology) — confirmed via
  BGP prefix lookup; 129.6.0.0/16 and 132.163.96.0/24 originated by AS49.
- **Leap handling**: Strict UTC step. NIST applies the leap second as a
  discontinuity at the exact moment; no smearing. NIST is the US national time
  standard and must represent the correct UTC value.
- **Usage restriction**: NIST asks users not to query more than once per 4 seconds.
- **Source grade: A** — official NIST ITS page explicitly lists all 16 server
  IPs and hostnames; these are permanent government-operated addresses.
- **Tier recommendation: Hard** — official US federal time standard; widely
  embedded in OS defaults, devices, and applications; blocking disrupts time sync
  for a large population of US-centric deployments.

---

#### Netnod Swedish Distributed Time Service (SDTS)

- **Official docs**: https://www.netnod.se/swedish-distributed-time-service
- **IPv4**: Anycast prefix range 194.58.200.0/24 through 194.58.207.0/24 (8 subnets)
  within AS57021. Exact per-node IPs accessible via individual hostnames
  (gbg1.ntp.se, gbg2.ntp.se, mmo1.ntp.se, mmo2.ntp.se, sth1–4.ntp.se,
  svl1.ntp.se, svl2.ntp.se, lul1.ntp.se, lul2.ntp.se).
- **IPv6**: Anycast prefix range 2a01:3f7:0::/48 through 2a01:3f7:7::/48 (8 subnets)
- **Anycast hostname**: ntp.se (routes to nearest node)
- **ASN**: AS57021 (Netnod Internet Exchange i Sverige AB)
- **Leap handling**: UNVERIFIED — not stated on main docs page; likely strict UTC
  as Netnod is a national metrology-grade time service.
- **Service governance**: Owned by PTS (Swedish Post and Telecom Authority), operated
  by Netnod, monitored by RISE Research Institutes of Sweden. Stratum 1 via atomic
  clocks at each node. Custom FPGA NTP hardware at 10Gb/s per interface.
- **Source grade: B** — official Netnod docs publish the IPv4 and IPv6 prefix ranges
  but not explicit per-server IPs. The prefix ranges are official and stable. Exact
  individual IPs require DNS lookup per node.
- **Tier recommendation: Hard** — national metrology-grade stratum-1 service; widely
  used across Nordic/European infrastructure.

---

#### PTB Germany (Physikalisch-Technische Bundesanstalt)

- **Official docs**: https://www.ptb.de/cms/en/ptb/fachabteilungen/abt9/gruppe-95/ref-952/time-synchronization-of-computers-using-the-network-time-protocol-ntp.html
- **IPv4**:
  - ptbtime1.ptb.de: 192.53.103.108 (IPv6: 2001:638:610:be01::108)
  - ptbtime2.ptb.de: 192.53.103.104 (IPv6: 2001:638:610:be01::104)
  - ptbtime3.ptb.de: 192.53.103.103 (IPv6: 2001:638:610:be01::103)
  - ptbtime4.ptb.de: exists (exact IP UNVERIFIED from official source)
- **IPv6**: 2001:638:610:be01::/64 subnet
- **ASN**: AS680 (Verein zur Foerderung eines Deutschen Forschungsnetzes e.V. — DFN)
  — confirmed via BGP lookup. PTB's network is hosted within the German research
  network (DFN). AS680 is the German academic research network ASN.
- **Leap handling**: UNVERIFIED — likely strict UTC step as PTB is Germany's national
  metrology institute and the authoritative source of UTC(PTB).
- **Important caveat**: The current official PTB NTP docs page now lists only
  hostnames, not IPs. The IPs above (192.53.103.103/104/108) are derived from DNS
  and confirmed via multiple third-party sources. They have been stable for years but
  must be treated as DNS-derived, not officially published static IPs.
- **Source grade: B** — official docs; IPs are DNS-derived not explicitly printed
  as static. The 192.53.103.0/24 subnet is within the DFN/PTB allocated range.
- **Tier recommendation: Soft** — important national metrology service but not as
  widely embedded in global device defaults as NIST/Google/Cloudflare; primarily
  European/German deployment context. Downgrade from hard because IPs are not
  officially published as static and the official page now recommends hostnames.

---

#### NPL UK (National Physical Laboratory)

- **Official docs**: https://www.npl.co.uk/ (user guide PDF)
- **IPv4**:
  - ntp1.npl.co.uk: 139.143.5.30
  - ntp2.npl.co.uk: 139.143.5.31
- **IPv6**: UNVERIFIED — not found in available sources.
- **ASN**: UNVERIFIED — 139.143.x.x space is within the UKRI/Janet academic
  network. UNVERIFIED: likely AS786 (Janet/Jisc) or NPL-specific ASN.
- **Leap handling**: UNVERIFIED; NPL is the UK national metrology institute
  (UTC(NPL)); expected to be strict UTC.
- **Stratum**: Stratum 2 (NPL publishes these as stratum 2 — they are synchronized
  to atomic clocks via internal NPL LAN, but the public servers themselves are
  stratum 2, not stratum 1).
- **Source grade: C** — official NPL user guide PDF published the IPs, but this
  is a static PDF document, not a machine-readable feed. The IPs have been stable
  across multiple references but are not advertised via a live official endpoint.
- **Tier recommendation: Soft** — UK national lab stratum 2 servers; relatively
  stable but not widely embedded as default; primarily UK/European deployments.

---

#### NICT Japan (National Institute of Information and Communications Technology)

- **Official docs**: https://jjy.nict.go.jp/tsp/PubNtp/
- **IPv4** (operational as of 2025-2026, confirmed from NICT operational status):
  - System A: 133.243.238.243, 133.243.238.244
  - System B: 133.243.238.163, 133.243.238.164
- **IPv6**: UNVERIFIED — not confirmed from official sources.
- **Hostname**: ntp.nict.jp (recommended by NICT; specific IPs may change)
- **ASN**: AS9355 (confirmed via ipinfo.io for 133.243.238.243)
- **Leap handling**: UNVERIFIED — NICT is Japan's national time standard;
  expected to be strict UTC step.
- **Source grade: B** — IPs are confirmed from NICT's own operational status page
  (historic entries), but NICT themselves advise using the hostname ntp.nict.jp
  because individual IPs may change.
- **Tier recommendation: Soft** — Japan's national time standard, widely used in
  Japanese infrastructure; IPs have been stable but official guidance recommends
  hostname. Appropriate as soft rather than hard because static-IP suitability is
  explicitly disclaimed by the operator.

---

#### NTSC China (National Time Service Center, Chinese Academy of Sciences)

- **Official docs**: UNVERIFIED — no English-language official docs with static IP
  found in this research pass.
- **IPv4**: 114.118.7.163 (current, from 2019 system), 210.72.145.44 (older/historic)
- **IPv6**: UNVERIFIED
- **Hostname**: ntp.ntsc.ac.cn
- **ASN**: UNVERIFIED — 114.118.7.x space needs BGP lookup to confirm.
- **Leap handling**: UNVERIFIED
- **Source grade: C** — IPs sourced from Chinese-language technical blogs and
  community references, not from an official machine-readable English-language source.
  Service is real (backed by CAS/NTSC atomic clocks and BeiDou+GPS), but the
  official source has not been verified in this research pass.
- **Operator caveat**: NTSC explicitly states limited bandwidth and recommends
  against large-scale deployment.
- **Tier recommendation: Soft** — include as soft with a clear note that the source
  quality is C (not official machine-readable) and the service has bandwidth
  constraints. Primarily relevant for East Asian deployments.

---

#### USNO (U.S. Naval Observatory)

- **Official page**: https://www.cnmoc.usff.navy.mil/Our-Commands/United-States-Naval-Observatory/Precise-Time-Department/Network-Time-Protocol-NTP/
- **IPv4** (historic/documented):
  - tick.usno.navy.mil: 192.5.41.40
  - tock.usno.navy.mil: 192.5.41.41
  - ntp2.usno.navy.mil: 192.5.41.209
- **IPv6**: Not published.
- **ASN**: UNVERIFIED — 192.5.41.x is USNO/DoD space; likely a DoD ASN.
- **Leap handling**: Strict UTC step — USNO is the US DoD timescale (UTC(USNO))
  and the primary reference for GPS time. No smearing.
- **Access policy**: Restricted. USNO stratum-1 servers are documented as
  "open access for .mil, .gov, and other stratum-2 servers." Public general access
  is not offered. The official access restriction has been in place for many years.
- **Source grade: C** — official CNMOC/USNO pages document the servers, but the
  official page currently returns 403 from the NTP-specific URL. The IPs above come
  from documented historic official sources.
- **Tier recommendation: Soft** — historically important US DoD/government time
  source; not widely available to the public. Include as soft with a note about
  access restrictions. Blocking these IPs would affect DoD/government systems, which
  is still meaningful collateral risk. Mark as "restricted access" in the reference
  set metadata so the product can clarify the operator context.

---

### Services rejected for static hard-tier NTP feed

#### NTP Pool (pool.ntp.org)

- **Reason for rejection**: The NTP Pool Project explicitly documents that the set of
  IP addresses returned by pool.ntp.org changes dynamically every few minutes as
  servers rotate in and out. There is no stable IP set. Any static IP list built
  from pool.ntp.org is stale within hours. The project FAQ explicitly advises against
  using IP-based allowlists. The pool consists of volunteer servers across thousands
  of different ASNs and address ranges.
- **Verdict: Reject for hard-tier static feed.** The NTP Pool is a DNS-based
  routing system, not a fixed anycast service. It cannot be represented as a static
  reference IP set.

#### Meta / Facebook Public NTP (time.facebook.com)

- **Reason for rejection**: Meta's official engineering blog documents the service
  at time.facebook.com (and ntp[0-3].meta.com) but explicitly does not publish static
  IPs. Meta's privacy policy for the NTP service states that they avoid IP fingerprinting
  and do not provide fixed addresses. The service runs from Meta's PoPs across AS32934
  and AS9086, but the exact address set is internal.
- **Verdict: Reject for hard-tier static feed.** No official published static IP
  exists. DNS-only service. May be reconsidered if Meta publishes an IP feed.

#### TimeNL (ntp.time.nl)

- **Reason for rejection**: The official TimeNL documentation explicitly states:
  "In your settings, use only the name ('ntp.time.nl'), without the corresponding IP
  address, because they reserve the right to change the IP address." SIDN Labs
  deliberately does not publish or guarantee a static IP.
- **Verdict: Reject for hard-tier static feed.** Operator explicitly prohibits
  IP-based configuration. DNS-only service.

---

## Discontinued / not suitable for static feed

| Service | Status | Reason |
|---------|--------|--------|
| ORSN (Open Root Server Network) | Shut down May 2019 | Permanently discontinued; not suitable as any tier |
| OpenNIC roots | Active but niche | Not IANA-designated; requires non-default resolver config; not a shared public dependency |
| KRISS Korea (time.kriss.re.kr, time2.kriss.re.kr) | Discontinued 2026-02-01 | Service ended; ntp.kriss.re.kr may still exist but not verified |
| DNS.WATCH | Status unverified | No official current static IP source verified in this pass |
| Yeti DNS | Research/experimental | Not a production root server network |

---

## Open questions / unverified

1. **Google NTP official static-IP statement**: Google does not explicitly document the
   216.239.35.0/29 IPs as permanent static addresses in their public docs. They are
   DNS-stable and operationally confirmed as AS15169, but the official docs only mention
   hostnames. Grade is B not A for this reason.

2. **Netnod SDTS exact per-node IPs**: The 194.58.200.0/24 through 194.58.207.0/24
   prefix ranges are confirmed, but exact per-node IPs for each of the 12 location
   servers are not officially enumerated. The anycast ntp.se is sufficient for hostname
   lookup but not for a static IP feed.

3. **Netnod SDTS leap handling**: Not stated on the main docs page. Requires direct
   check against Netnod technical documentation or direct contact.

4. **NPL UK ASN**: 139.143.x.x resolves within the UK academic / Janet network, but
   the exact ASN and whether NPL has a dedicated ASN has not been confirmed.

5. **NICT Japan IPv6 addresses**: Not confirmed from official sources in this pass.

6. **PTB Germany ptbtime4.ptb.de IP**: Exists but the IP was not found in this research
   pass. PTB docs mention a fourth server but do not list its IP.

7. **USNO current service status**: The CNMOC NTP page returned 403 during research;
   the most current confirmed public access status for tick/tock is from documentation
   that may be several years old. Needs a live connectivity check or direct USNO contact.

8. **NTSC China official English source**: No English-language official static IP
   documentation was found. Chinese-language sources confirm the IPs but these are
   secondary.

9. **PTB and NPL IPv6 for static feed**: IPv6 addresses are derived from third-party
   sources, not explicitly published as a machine-readable official feed.

---

## Sources consulted

### DNS Root servers
- IANA Root Servers: https://www.iana.org/domains/root/servers
- InterNIC named.root file: https://www.internic.net/domain/named.root
- Root Server Technical Operations Association: https://root-servers.org/
- Verisign A-Root: https://a.root-servers.org/
- USC/ISI B-Root: https://b.root-servers.org/ (including 2017-03-01 ASN change notice)
- ISC F-Root: https://www.isc.org/f-root/
- IPinfo.io / bgp.tools for ASN confirmation

### AS112
- RFC 7534 (AS112 Nameserver Operations, current): https://www.rfc-editor.org/rfc/rfc7534
- RFC 7535 (AS112 Redirection via DNAME): https://www.rfc-editor.org/rfc/rfc7535
- RFC 6304 (obsoleted): https://www.rfc-editor.org/rfc/rfc6304
- RFC 6305 (informational): informational explanation for DNS operators
- RFC 8375 (home.arpa)
- AS112 project: https://www.as112.net/
- bgp.tools/as/112: https://bgp.tools/as/112 (prefix/route confirmation, April 2026)

### NTP / time services
- Cloudflare Time Services NTP docs: https://developers.cloudflare.com/time-services/ntp/usage/
- Cloudflare NTP overview: https://developers.cloudflare.com/time-services/ntp/
- Google Public NTP: https://developers.google.com/time
- Google Public NTP FAQ: https://developers.google.com/time/faq
- NIST Internet Time Service: https://tf.nist.gov/tf-cgi/servers.cgi
- Netnod Swedish Distributed Time Service: https://www.netnod.se/swedish-distributed-time-service
- PTB NTP docs: https://www.ptb.de/cms/en/ptb/fachabteilungen/abt9/gruppe-95/ref-952/time-synchronization-of-computers-using-the-network-time-protocol-ntp.html
- NPL Internet Time Service user guide: https://www.npl.co.uk/getattachment/7c097457-5a1f-436f-af7b-930634680c5d/its_user_guide.pdf
- NICT Public NTP status: https://jjy.nict.go.jp/tsp/PubNtp/status.html
- CNMOC/USNO NTP: https://www.cnmoc.usff.navy.mil/Our-Commands/United-States-Naval-Observatory/Precise-Time-Department/Network-Time-Protocol-NTP/
- TimeNL: https://time.nl/index_en.html
- Meta NTP engineering blog: https://engineering.fb.com/2020/03/18/production-engineering/ntp-service/
- BGP lookups: bgp.tools, ipinfo.io, bgp.he.net
- Alternative DNS root overview: https://en.wikipedia.org/wiki/Alternative_DNS_root
