# Soft-tier CDN edges and edge computing research (SOW-0017)

Research date: 2026-04-29
Scope: every provider in the task brief, plus additional candidates surfaced during research.
Method: WebFetch against each provider's official IP-range page/API; BGP data from bgp.he.net; bash
validation against live AWS and Azure JSON endpoints.

Source quality rubric (from SOW):
- A = official, machine-readable, current feed/API
- B = official machine-readable but partial, geofeed, DNS-derived, or requiring derivation
- C = official docs/static page (HTML), not a clean dynamic feed
- D = no official public source / third-party only / stale / unsuitable

---

## Summary table

| Provider | Category | Source URL | Format | Grade | ASN(s) | Recommended tier | Notes |
|---|---|---|---|---|---|---|---|
| Cloudflare CDN | CDN edge | https://www.cloudflare.com/ips-v4 + ips-v6 ; https://api.cloudflare.com/client/v4/ips | text + JSON | A | AS13335 | soft | 15 IPv4 + 7 IPv6 ranges; entire published list covers all CF products |
| Fastly | CDN edge | https://api.fastly.com/public-ip-list | JSON | A | AS54113 | soft | 19 IPv4 + 2 IPv6 ranges; clear JSON structure |
| AWS CloudFront | CDN edge | https://ip-ranges.amazonaws.com/ip-ranges.json (service=CLOUDFRONT) | JSON | A | AS16509 | soft | 203 IPv4 prefixes; separate dedicated endpoint also exists |
| AWS CloudFront (dedicated) | CDN edge | https://d7uri8nf7uskq.cloudfront.net/tools/list-cloudfront-ips | JSON | A | AS16509 | soft | 130 global + 84 regional = 214 ranges; two-key JSON |
| AWS GlobalAccelerator | Accelerator edge | https://ip-ranges.amazonaws.com/ip-ranges.json (service=GLOBALACCELERATOR) | JSON | A | AS16509 | soft | 113 IPv4 prefixes |
| Azure Front Door (Frontend) | CDN/WAF edge | Azure service tags JSON — tag AzureFrontDoor.Frontend | JSON (weekly) | A | AS8075 | soft | 208 prefixes; covers user-facing edge |
| Azure Front Door (Backend) | CDN origin-pull | Azure service tags JSON — tag AzureFrontDoor.Backend | JSON (weekly) | A | AS8075 | soft | 213 prefixes; origin-facing only |
| Azure Front Door (FirstParty) | Microsoft-internal | Azure service tags JSON — tag AzureFrontDoor.FirstParty | JSON (weekly) | A | AS8075 | soft | 69 prefixes; Microsoft-internal services, lower user blast radius |
| Azure Front Door (MicrosoftSecurity) | Security services | Azure service tags JSON — tag AzureFrontDoor.MicrosoftSecurity | JSON (weekly) | A | AS8075 | soft | 7 prefixes; Defender/security product range |
| Google service edge (GFE) | Google service edge | Derive: goog.json minus cloud.json | JSON (derived) | B | AS15169 / AS396982 | soft | 111 total (91 IPv4 + 20 IPv6) in goog.json; cloud.json has 1177 prefixes; subtraction yields Google-operated non-customer ranges |
| Akamai CDN | CDN edge | No official public unauthenticated bulk feed | BGP-derived | D→B (secondary) | AS20940 (4778 prefixes) / AS16625 (2795 prefixes) | soft (secondary only) | Authenticated API exists for customers; recommend BGP enumeration of AS20940+AS16625 marked generated_bgp |
| Imperva / Incapsula | CDN + WAF + DDoS | https://my.imperva.com/api/integration/v1/ips | JSON | A | AS19551 | soft | 11 IPv4 + 1 IPv6 range; requires no authentication in test; covers WAF/DDoS/CDN scrubbing |
| Bunny.net / BunnyCDN | CDN edge | https://api.bunny.net/system/edgeserverlist | JSON | B | multiple (global ISP peering) | soft (later) | Returns 572 individual /32 IPs, not CIDR ranges; no subnet aggregation |
| G-Core Labs / Gcore | CDN edge | https://api.gcore.com/cdn/public-ip-list | JSON | B | multiple | soft (later) | Returns /32 and /128 individual IPs, not aggregated ranges; ~1000+ entries |
| Sucuri WAF | WAF edge | API requires authentication key (waf.sucuri.net/api?v2&a=firewall_ip_list) | JSON (auth) | D | unknown | reject | No unauthenticated public source; per-site assignment model |
| Vercel edge | App platform | No official public edge IP feed | N/A | D | unknown | reject (no source) | Trusted IPs is an inbound customer allowlist feature, not a published edge IP list |
| Netlify edge | App platform | No official public edge IP feed | N/A | D | unknown | reject (no source) | No edge IP publication found in docs |
| CDN77 | CDN edge | No official public feed found | N/A | D | unknown | reject | Support docs 404 or behind auth |
| KeyCDN | CDN edge | No official public feed found | N/A | D | unknown | reject | Support URL ECONNREFUSED; no known public IP list |
| Edgio (Limelight) | CDN edge | AS22822 not visible in routing table since 2026-04-16 | N/A | D | AS22822 (withdrawn) | reject | Company appears defunct in BGP; docs redirect to unrelated uplynk.com |
| StackPath (MaxCDN) | CDN edge | No official public feed found | N/A | D | unknown | reject | Website 404; likely merged/retired |
| Fly.io | App edge | No official public edge IP feed | N/A | D | unknown | reject | Docs explicitly say outbound IPs change without notice; anycast announced via BGP but not published |
| Render edge | App platform | No official edge/CDN IP feed | N/A | D | unknown | reject | Docs only cover per-service outbound IPs for hosted services |
| Deno Deploy | App platform | No official public edge IP feed | N/A | D | unknown | reject | Support docs ECONNREFUSED |
| Section.io | CDN/edge | No official public feed found | N/A | D | unknown | reject | Docs 403 |
| OVHcloud CDN | CDN edge | No machine-readable feed found | C/D | AS16276 | later | Official docs exist but are static/manual and note range limitations |
| Medianova | CDN edge | No official public feed found | N/A | D | unknown | reject | Website 404 |
| BelugaCDN | CDN edge | No official public feed found | N/A | D | unknown | reject | Website 404 |
| Leaseweb CDN | CDN edge | No usable content found | D | unknown | reject | Page returned empty content |
| Cachefly | CDN edge | No usable content found | D | unknown | reject | Docs ECONNREFUSED |
| jsDelivr | CDN (proxy) | No own IPs; rides Fastly + Cloudflare + Bunny | N/A | D (self) | N/A | reject (covered by Fastly/CF) | jsDelivr is a logical CDN using third-party physical CDNs; their IPs are already in Fastly/Cloudflare sets |
| Tencent CDN | China CDN | No official international public feed found | D | AS132203 | reject (for now) | Docs 404; China-regional feed only |
| Alibaba CDN | China CDN | No official global CDN IP feed found | D | AS37963 | reject (for now) | Docs reference internal cloud service ranges only |
| Wangsu / ChinaNetCenter | China CDN | No official public feed found | D | unknown | reject (for now) | Connection timeout |
| ByteDance / Volcengine CDN | China CDN | No official public feed found | D | unknown | reject (for now) | Docs content not extractable |
| Huawei Cloud CDN | China CDN | No official public feed found | D | AS55990 | reject (for now) | Docs 404 |
| Cloudflare Workers / R2 | App edge | Same ranges as CDN (cloudflare.com/ips-v4) | text/JSON | A | AS13335 | soft (covered by CF entry) | CF docs confirm shared range; no separate Workers/R2 range published |
| Cloudflare Magic Transit | Network edge | Same published ranges per CF docs | text/JSON | A | AS13335 | soft (covered by CF entry) | Magic Transit uses CF-published IP list; BYOIP customers use their own addresses |
| F5 Distributed Cloud (Volterra) | WAF/edge | No public unauthenticated feed found | D | unknown | reject | Support docs 404/403; authentication required for any IP list |
| Arbor Cloud / NETSCOUT | DDoS scrubbing | No official public feed found | D | unknown | reject | Redirects to marketing page; no IP publication |
| Radware DefensePro | DDoS scrubbing | No official public feed found | D | unknown | reject | Browser verification challenge; no usable content |
| Nexusguard | DDoS scrubbing | No official public feed found | D | unknown | reject (redirects loop) | |
| Voxility | DDoS scrubbing | No official public feed found | D | unknown | reject | |
| GitHub (Meta API) | Developer platform | https://api.github.com/meta | JSON | A | AS36459 (26 prefixes) | soft | Covers hooks, web, api, git, packages, pages, actions; actions category is large (1000+ ranges) |
| Atlassian Cloud | Dev platform/SaaS | https://ip-ranges.atlassian.com/ | JSON | A | multiple | soft | Hundreds of ranges with product/region/direction metadata; covers Jira, Confluence, Bitbucket, Forge |
| Terraform Cloud | Dev platform | https://app.terraform.io/api/meta/ip-ranges | JSON | A | multiple | soft | 4 categories: API (2), Notifications (14), Sentinel (14), VCS (14); small set of /32 addresses |

---

## Per-provider details

### Cloudflare CDN / Workers / R2 / Spectrum / Magic Transit

**Brand context**: Cloudflare is one of the largest CDN and internet security providers, with the widest CDN market reach. Used by millions of websites globally. Products include CDN, DDoS protection, WAF, Workers (serverless), R2 (object storage), Magic Transit (BGP-level DDoS for enterprise).

**Official IP source URL (verified live)**:
- Text feed (IPv4): `https://www.cloudflare.com/ips-v4` — plain text, one CIDR per line
- Text feed (IPv6): `https://www.cloudflare.com/ips-v6` — plain text, one CIDR per line
- API (both families): `https://api.cloudflare.com/client/v4/ips` — JSON with `ipv4_cidrs`, `ipv6_cidrs`, and `etag` fields

**Current ranges (verified 2026-04-29)**:
- IPv4 (15 ranges): 173.245.48.0/20, 103.21.244.0/22, 103.22.200.0/22, 103.31.4.0/22, 141.101.64.0/18, 108.162.192.0/18, 190.93.240.0/20, 188.114.96.0/20, 197.234.240.0/22, 198.41.128.0/17, 162.158.0.0/15, 104.16.0.0/13, 104.24.0.0/14, 172.64.0.0/13, 131.0.72.0/22
- IPv6 (7 ranges): 2400:cb00::/32, 2606:4700::/32, 2803:f800::/32, 2405:b500::/32, 2405:8100::/32, 2a06:98c0::/29, 2c0f:f248::/32

**Coverage**: All Cloudflare products use the same published ranges for inbound traffic (CDN, Workers, R2, Spectrum TCP/UDP proxy). Magic Transit uses these ranges for Cloudflare-owned IP protection; BYOIP customers use their own address space (not in this list). No separate product-specific range is published.

**ASN**: AS13335 (5,702 prefixes originated, 11,075 announced including customer BYOIP)

**Source quality grade**: A — official, machine-readable, continuously maintained, `etag` enables delta detection.

**License / redistribution**: No license statement on the page. No redistribution restriction documented. Currently in `misp_cloudflare.yaml` (MISP secondary); primary direct source is superior.

**Update cadence**: No published SLA; Cloudflare updates when ranges change. The `etag` field on the API response enables efficient polling.

**Recommended tier**: soft / cdn_edge

**Caveats**:
- AS13335 also originates 1.1.1.0/24 and 1.0.0.0/24 (public DNS resolver addresses). These are hard-tier DNS entries already in the hard candidate list. The soft CDN range feed will include these prefixes — the engine must handle multi-tier overlap correctly and not double-count.
- The published list covers user-facing edge IPs. Cloudflare's internal network, peering links, and some BYOIP customer blocks are not in this list.
- 104.16.0.0/13 and adjacent large blocks contain both CDN edge and customer reverse-proxy IPs. There is no way to separate "CF origin" from "CF edge" at the public IP level.

---

### Fastly

**Brand context**: Major CDN provider, backbone for many large media sites (GitHub Pages, Reddit, Twitter at times, npm, etc.) and developer platforms. Also provides edge compute (Compute@Edge).

**Official IP source URL (verified live)**:
- `https://api.fastly.com/public-ip-list` — JSON with `addresses` (IPv4) and `ipv6_addresses` keys

**Current ranges (verified 2026-04-29)**:
- IPv4 (19 ranges): 23.235.32.0/20, 43.249.72.0/22, 103.244.50.0/24, 103.245.222.0/23, 103.245.224.0/24, 104.156.80.0/20, 140.248.64.0/18, 140.248.128.0/17, 146.75.0.0/17, 151.101.0.0/16, 157.52.64.0/18, 167.82.0.0/17, 167.82.128.0/20, 167.82.160.0/20, 167.82.224.0/20, 172.111.64.0/18, 185.31.16.0/22, 199.27.72.0/21, 199.232.0.0/16
- IPv6 (2 ranges): 2a04:4e40::/32, 2a04:4e42::/32

**Coverage**: All Fastly edge PoP servers. Origin-pull IPs are included (traffic from Fastly to customer origin servers uses these same ranges).

**ASN**: AS54113 (505 IPv4 prefixes originated, 1,661 total with IPv6)

**Source quality grade**: A — official machine-readable JSON endpoint with no authentication required.

**License / redistribution**: No license statement visible. Currently in `misp_fastly.yaml` (MISP secondary); primary direct source is superior.

**Update cadence**: No published SLA; polling recommended.

**Recommended tier**: soft / cdn_edge

**Caveats**:
- jsDelivr uses Fastly (among others) as its CDN backbone — jsDelivr hits will appear as Fastly hits. No separate jsDelivr IP feed is needed.
- 151.101.0.0/16 is a large /16 (65,536 IPs) that Fastly announces as one block. Blocking it would affect all Fastly-served content.

---

### AWS CloudFront

**Brand context**: Amazon CloudFront is AWS's CDN. Extremely widely used by AWS customers for static asset delivery, API acceleration, and video streaming. Origin-facing and user-facing ranges are mixed in the published list.

**Official IP source URLs (both verified live)**:
1. Primary JSON: `https://ip-ranges.amazonaws.com/ip-ranges.json` — filter `service=CLOUDFRONT`
2. Dedicated CloudFront endpoint: `https://d7uri8nf7uskq.cloudfront.net/tools/list-cloudfront-ips` — JSON with `CLOUDFRONT_GLOBAL_IP_LIST` and `CLOUDFRONT_REGIONAL_EDGE_IP_LIST` keys

**Current ranges (verified 2026-04-29)**:
- 203 IPv4 prefixes in ip-ranges.json (service=CLOUDFRONT)
- Dedicated endpoint: 130 global + 84 regional = 214 ranges (slight difference — different documentation path)

**Coverage**: CloudFront edge PoP servers for user traffic and regional edge caches. Does NOT include origin-facing ranges for custom origins (use AMAZON tag for those, much larger).

**ASN**: AS16509

**Source quality grade**: A — official AWS endpoint, machine-readable JSON.

**License / redistribution**: No license restriction stated. AWS documentation says "we publish" these ranges for customer use.

**Update cadence**: AWS publishes via `syncToken` timestamp; recommend polling ip-ranges.json and checking `syncToken` before processing.

**Recommended tier**: soft / cdn_edge

**Caveats**:
- CLOUDFRONT ranges are a subset of the full AWS space; other AWS services use the same ASN. Filtering to `service=CLOUDFRONT` is essential.
- The dedicated CloudFront endpoint (`d7uri8nf7uskq.cloudfront.net`) is itself served by CloudFront — if CloudFront is blocked, you cannot fetch the CloudFront IP list. The ip-ranges.json endpoint is more resilient.
- "regional edge" ranges include intermediate CloudFront cache nodes; "global" ranges include leaf PoPs.

---

### AWS Global Accelerator

**Brand context**: AWS Global Accelerator provides static anycast IP-based acceleration for AWS-hosted applications. Uses dedicated anycast ranges, not shared with CloudFront.

**Official IP source URL (verified live)**:
- `https://ip-ranges.amazonaws.com/ip-ranges.json` — filter `service=GLOBALACCELERATOR`

**Current ranges (verified 2026-04-29)**:
- 113 IPv4 prefixes (examples: 3.2.58.0/24, 13.248.117.0/24, 15.197.34.0/23, 15.197.36.0/22)
- No IPv6 in current data

**Coverage**: Static anycast IPs allocated to accelerators. Traffic to Global Accelerator enters the AWS backbone at the nearest PoP.

**ASN**: AS16509

**Source quality grade**: A

**License / redistribution**: Same as other AWS ranges.

**Update cadence**: Same syncToken mechanism as ip-ranges.json.

**Recommended tier**: soft / cdn_edge (or a separate accelerator role)

**Caveats**:
- Global Accelerator IPs are static per-accelerator assignments, not shared like CloudFront. Blocking a Global Accelerator IP affects only that specific accelerator endpoint — blast radius is lower than CloudFront. Still soft tier because the IPs are shared AWS infrastructure and not customer-owned.

---

### Azure Front Door

**Brand context**: Azure Front Door is Microsoft's globally distributed CDN, WAF, and application delivery platform. It replaced the older Azure CDN from Verizon (Edgio) and is in the process of absorbing "Azure CDN Standard from Microsoft (classic)", which retires 2027. Used by Microsoft's own services and Azure customers.

**Official IP source URL (verified live)**:
- Azure service tags weekly JSON: `https://download.microsoft.com/download/7/1/d/71d86715-5596-4529-9b13-da13a5de5b63/ServiceTags_Public_20260420.json`
- Stable discovery URL (redirects to current weekly file): `https://www.microsoft.com/en-us/download/details.aspx?id=56519`
- Machine-readable API alternative: `https://management.azure.com/subscriptions/.../providers/Microsoft.Network/locations/.../serviceTags` (requires Azure credentials)

**Service tags and prefix counts (verified 2026-04-29)**:
| Tag | Prefixes | Coverage |
|-----|---------|----------|
| `AzureFrontDoor.Frontend` | 208 | User-facing edge IPs (where clients connect) |
| `AzureFrontDoor.Backend` | 213 | Origin-pull IPs (where AFD connects to customer origins) |
| `AzureFrontDoor.FirstParty` | 69 | Microsoft-internal first-party service acceleration |
| `AzureFrontDoor.MicrosoftSecurity` | 7 | Microsoft Defender/security product ranges |

**Coverage**: All Azure Front Door PoP edge locations (192 edge locations across 109 metro cities per official docs).

**ASN**: AS8075 (primary Microsoft ASN)

**Source quality grade**: A — official Microsoft weekly JSON, stable download URL pattern.

**License / redistribution**: Microsoft's usage terms apply. No explicit redistribution restriction for these service tags in the documentation reviewed.

**Update cadence**: Weekly. The download URL date changes weekly; the discovery page at id=56519 always redirects to the current week's file.

**Recommended tier**: soft / cdn_edge

**Caveats**:
- `AzureFrontDoor.Backend` is origin-facing. A blocklist entry matching these IPs would block Azure Front Door from pulling content from customer origins — very high blast radius for Azure-hosted sites.
- `AzureFrontDoor.Frontend` is what users hit; this is the "edge" range for CDN purposes.
- Azure CDN Standard from Microsoft (classic) shares infrastructure with Front Door and uses the same service tags. The retirement path (AFD Standard/Premium) is the same infrastructure.
- `AzureFrontDoor.MicrosoftSecurity` (7 prefixes) covers Defender for Endpoint and similar — lower blast radius but still important.
- The weekly JSON file URL changes every week. The discovery page (id=56519) must be polled to find the current URL; alternatively use the Azure REST API.

---

### Google Service Edge / GFE (Google Front End)

**Brand context**: Google operates a globally distributed edge network (GFE = Google Front End) that handles traffic for Google Search, Gmail, YouTube, Google APIs, Google Cloud CDN, and most Google services. This is not the same as Google Cloud customer IP space.

**Official IP source approach (verified live)**:
- `https://www.gstatic.com/ipranges/goog.json` — all Google-used IP ranges (111 total: 91 IPv4 + 20 IPv6)
- `https://www.gstatic.com/ipranges/cloud.json` — Google Cloud customer-usable ranges (1,177 total with region/service metadata)
- Derivation: `goog.json MINUS cloud.json` = Google-operated non-customer ranges (GFE, services, infrastructure)

**Current goog.json ranges (verified 2026-04-29)**:
- 91 IPv4 + 20 IPv6 prefixes
- Includes 8.8.4.0/24 and 8.8.8.0/24 (Public DNS resolvers — hard tier, already handled separately)
- Includes 8.34.208.0/20, 8.35.192.0/20, large 34.x.x.x blocks

**Current cloud.json ranges (verified 2026-04-29)**:
- 1,177 prefixes (mixed IPv4/IPv6) with `service: "Google Cloud"` and regional `scope` fields

**Coverage**: `goog.json` = Google-served infrastructure; `cloud.json` = customer VM/GKE/Cloud Run IPs. The difference = Google-operated service IPs, including CDN, GFE, APIs, and internal tooling.

**ASN**: AS15169 (Google LLC, 1,209 IPv4 + 178 IPv6 prefixes originated) and AS396982 (Google LLC, 3,341 IPv4 + 198 IPv6 prefixes originated — includes GCP regional cloud)

**Source quality grade**: B — both files are official Google sources; the derivation step (set subtraction) is required and introduces implementation complexity. The result is not a directly published feed.

**License / redistribution**: No license restriction documented on either JSON file.

**Update cadence**: Both files have a `syncToken` (Unix timestamp in milliseconds) and `creationTime`. Recommend polling both and recomputing the difference only when either changes.

**Recommended tier**: soft / google_service_edge (dedicated role, not generic cdn_edge)

**Caveats**:
- The derivation includes ranges that are neither CDN nor accessible to end users (internal Google tooling, private APIs). There is no way to further filter these programmatically from public data.
- 8.8.8.0/24 and 8.8.4.0/24 will appear in goog.json but NOT in cloud.json — they will be in the derived set. These must be marked as hard-tier (public DNS) in the hard candidate list and must not be double-counted as soft CDN.
- The derived "GFE" set is large (~91 IPv4 entries before subtraction, many large blocks). It is not purely CDN edge; it includes APIs, login servers, etc.

---

### Akamai CDN

**Brand context**: Akamai is one of the oldest and largest CDNs, used by major enterprises, financial institutions, governments, and media companies. Also provides DDoS scrubbing (Prolexic) and WAF (Kona Site Defender).

**Official IP source**: NONE for unauthenticated public download.

**Authenticated API**: Akamai provides an authenticated `Edge DNS` and `IP Allow List` API for customers, but these require Akamai credentials and are not suitable as a public reference feed.

**BGP fallback (verified 2026-04-29)**:
- AS20940 (Akamai International B.V.): 3,987 IPv4 + 791 IPv6 = 4,778 prefixes originated
- AS16625 (Akamai Technologies, Inc.): ~2,787 IPv4 + 8 IPv6 prefixes originated
- Both ASNs are confirmed Akamai via bgp.he.net.
- MISP warninglists has an `akamai` entry (referenced in SOW knowledge), but it is BGP/RDAP-derived, not from an official Akamai feed.

**Coverage of BGP-derived set**: All publicly routed Akamai edge prefixes. Includes both CDN delivery and Prolexic/DDoS scrubbing infrastructure — no way to separate them without Akamai's internal routing policy.

**Source quality grade**: D (no official public source) → B (BGP enumeration of verified ASNs, marked `generated_bgp`)

**License / redistribution**: N/A for BGP-derived data (public routing data).

**Update cadence**: BGP changes daily; a BGP-derived set needs daily refresh from RIPE/ARIN/bgp.he.net or similar.

**Recommended tier**: soft (secondary only, `generated_bgp` source type)

**Caveats**:
- Existing `misp_akamai.yaml` in the catalog can be kept as a secondary MISP source.
- BGP-derived data proves routing ownership (Akamai originates these prefixes) but does NOT prove service role (CDN vs DDoS scrubbing vs other Akamai products).
- Akamai's customer-facing advisory is to use their authenticated network-lists API, not to rely on BGP data. BGP data should be documented as a best-effort approximation.
- AS20940 and AS16625 combined have approximately 6,000+ prefixes — this is a large set. Consider whether to include the full set or only well-known stable ranges.

---

### Imperva / Incapsula (WAF + DDoS + CDN)

**Brand context**: Imperva (formerly Incapsula) provides a combined WAF, DDoS protection, and CDN platform. Now part of Thales Group (Thales acquired Imperva). Used by enterprises and e-commerce. The `my.imperva.com` portal is the primary control plane.

**Official IP source URL (verified live)**:
- `https://my.imperva.com/api/integration/v1/ips` — no authentication required in test
- Returns JSON with `ipRanges` (IPv4) and `ipv6Ranges` (IPv6) keys, plus status fields

**Current ranges (verified 2026-04-29)**:
- IPv4 (11 ranges): 199.83.128.0/21, 198.143.32.0/19, 149.126.72.0/21, 103.28.248.0/22, 185.11.124.0/22, 192.230.64.0/18, 45.64.64.0/22, 107.154.0.0/16, 45.60.0.0/16, 45.223.0.0/16, 131.125.128.0/17
- IPv6 (1 range): 2a02:e980::/29

**Coverage**: All Imperva scrubbing center and WAF proxy IPs. Traffic proxied through Imperva's WAF/CDN comes from these ranges. The DDoS scrubbing center IPs are the same infrastructure.

**ASN**: AS19551 (Incapsula Inc. / Imperva) — 871 IPv4 + 707 IPv6 prefixes originated; 222,976 IPv4 IPs

**Source quality grade**: A — official unauthenticated machine-readable JSON endpoint, verified working.

**License / redistribution**: No license restriction documented.

**Update cadence**: Unknown; no `syncToken` or timestamp in response. Recommend polling weekly.

**Recommended tier**: soft / cdn_edge (also covers WAF and DDoS scrubbing roles)

**Caveats**:
- `107.154.0.0/16` is a large /16 that Imperva announces. Blocking it affects all Imperva-proxied sites.
- `45.60.0.0/16` and `45.223.0.0/16` are also large blocks.
- The Thales acquisition may affect the API URL in the future; the docs now redirect to `docs-cybersec.thalesgroup.com` but the API endpoint still resolves at `my.imperva.com`.

---

### Bunny.net / BunnyCDN

**Brand context**: Bunny.net is a growing CDN provider used by video streaming, SaaS products, and open-source projects (jsDelivr uses Bunny as one of its backends). Budget-friendly CDN with global PoPs.

**Official IP source URL (verified live)**:
- `https://api.bunny.net/system/edgeserverlist` — JSON array of individual IPv4 addresses

**Format**: JSON array of strings, each being a single IPv4 address (e.g., `"89.187.188.227"`). Not CIDR ranges.

**Current size**: 572 individual /32 IP addresses.

**A `?type=premium` variant** also exists per the task brief but returns the same individual-IP format.

**ASN**: Multiple (Bunny peers globally; individual IPs span diverse ASNs/providers). Not a single ASN.

**Source quality grade**: B — official machine-readable endpoint, but individual IPs rather than aggregated CIDR ranges. Usable as a reference set but requires careful handling (no CIDR aggregation possible without risk of over-broadening; serving 572 /32 entries directly is feasible).

**License / redistribution**: No license restriction documented.

**Update cadence**: Unknown; no timestamp in response. Recommend weekly refresh.

**Recommended tier**: soft (later) — source is usable but individual IPs make aggregate overlap calculations less efficient. Not a priority for v1.

**Caveats**:
- Individual IPs may change as Bunny adds/removes PoPs. The list must be refreshed.
- No CIDR aggregation available from the official source.
- jsDelivr uses Bunny among other CDNs. jsDelivr itself has no separate IP list and is fully covered by its constituent CDNs.

---

### G-Core Labs / Gcore CDN

**Brand context**: G-Core is a European CDN and cloud provider with global PoPs. Growing presence in gaming, media, and enterprise CDN markets.

**Official IP source URL (verified live)**:
- `https://api.gcore.com/cdn/public-ip-list` — JSON with `ipv4` and `ipv6` arrays of individual addresses

**Format**: Individual /32 (IPv4) and /128 (IPv6) addresses. Not CIDR ranges.

**Current size**: ~1,000+ IPv4 and ~700+ IPv6 individual addresses.

**ASN**: Multiple (global peering).

**Source quality grade**: B — official machine-readable JSON, but individual IPs rather than CIDR ranges.

**License / redistribution**: No license restriction documented.

**Update cadence**: Unknown.

**Recommended tier**: soft (later) — same individual-IP limitations as Bunny.net; lower priority for v1.

**Caveats**: Same as Bunny.net — individual IPs, no aggregation, requires frequent refresh.

---

### GitHub (Meta API)

**Brand context**: GitHub is the dominant developer platform. Blocklisting GitHub IPs would break CI/CD pipelines, package managers (npm, PyPI, Maven via GHCR), webhook deliveries, and development workflows globally.

**Official IP source URL (verified live)**:
- `https://api.github.com/meta` — JSON with multiple service categories

**Categories and range counts (verified 2026-04-29)**:
| Category | IPv4 ranges | IPv6 ranges | Total | Notes |
|----------|------------|------------|-------|-------|
| hooks | 6 | 2 | 8 | Webhook source IPs |
| web | 22 | 4 | 26 | Web browsing |
| api | 22 | 4 | 26 | API access |
| git | 32 | 4 | 36 | Git clone/push |
| github_enterprise_importer | 8 | 2 | 10 | Migration tool |
| packages | 32 | 0 | 32 | GitHub Packages / GHCR |
| pages | 8 | 4 | 12 | GitHub Pages |
| importer | 5 | 0 | 5 | Legacy importer |
| actions | 1000+ | 300+ | 1300+ | GitHub Actions runners |

**Coverage**: Multiple GitHub service categories. The `actions` category is large because GitHub Actions uses dynamically allocated VMs across Azure regions.

**ASN**: AS36459 (GitHub Inc.) — 24 IPv4 + 2 IPv6 prefixes; 26 total originated prefixes.

**Note**: The `actions` ranges are not in AS36459 — they are Azure VM ranges in AS8075. The GitHub Meta API is the authoritative source, not BGP enumeration.

**Source quality grade**: A — official GitHub Meta API, no authentication required.

**License / redistribution**: GitHub's documentation warns the list is not exhaustive. No explicit redistribution restriction.

**Update cadence**: Changes without notice; `syncToken`-style versioning not present. Recommend daily polling.

**Recommended tier**: soft / developer_platform

**Caveats**:
- The `actions` category contains 1300+ ranges that overlap significantly with Azure (AS8075). These are shared Azure VM ranges used by GitHub Actions — not GitHub-owned ranges. This is expected and documented.
- GitHub itself warns the list is "not exhaustive" — some GitHub traffic may not originate from these ranges.
- For critical-infrastructure overlap purposes, the most important categories are `web`, `api`, `git`, `hooks`, and `packages`. The `actions` category has lower criticality since it is shared cloud compute.

---

### Atlassian Cloud (Jira, Confluence, Bitbucket)

**Brand context**: Atlassian hosts the dominant enterprise project management and code collaboration tools (Jira, Confluence, Bitbucket, Forge). Webhooks and CI/CD integrations depend on specific Atlassian egress IPs.

**Official IP source URL (verified live)**:
- `https://ip-ranges.atlassian.com/` — JSON array with rich metadata per entry

**Format**: JSON array; each entry has:
- Network address and mask length
- Region assignments (ap-northeast-1, us-east-1, eu-west-1, global, etc.)
- Product (Jira, Confluence, Bitbucket, Forge, etc.)
- Direction (egress, ingress, or both)
- Perimeter (commercial, fedramp-moderate)

**Current size**: 400+ ranges (mix of /32 and larger CIDR blocks).

**ASN**: Multiple (AWS-hosted infrastructure across multiple AWS regions).

**Source quality grade**: A — official, machine-readable, well-structured JSON with product/direction metadata.

**License / redistribution**: No explicit redistribution restriction.

**Update cadence**: Unknown; no timestamp in response.

**Recommended tier**: soft / developer_platform

**Caveats**:
- Atlassian ranges are hosted on AWS; they overlap with the AWS/EC2 space. This is expected.
- The `direction: egress` ranges are the outbound IPs (webhooks, integrations), which are most relevant for inbound firewalls. The `direction: ingress` ranges are the IPs users connect to.

---

### Terraform Cloud (HashiCorp / IBM)

**Brand context**: Terraform Cloud is the managed CI/CD and state management platform for Terraform IaC. Many enterprises depend on it for infrastructure automation webhooks and VCS integrations.

**Official IP source URL (verified live)**:
- `https://app.terraform.io/api/meta/ip-ranges` — JSON with service categories

**Categories (verified 2026-04-29)**:
- API: 2 addresses
- Notifications: 14 addresses
- Sentinel: 14 addresses
- VCS: 14 addresses

**Format**: JSON with service keys mapping to arrays of /32 CIDR addresses.

**Source quality grade**: A — official machine-readable endpoint.

**License / redistribution**: No explicit restriction.

**Update cadence**: Unknown.

**Recommended tier**: soft / developer_platform (lower priority than GitHub/Atlassian due to smaller blast radius; mainly relevant for VCS webhook flows)

**Caveats**: Very small set (44 total IP addresses). IBM acquired HashiCorp; the `app.terraform.io` endpoint may change branding.

---

## Reject (no usable source)

### Vercel edge
No official public edge IP feed. The "Trusted IPs" feature is an inbound allowlist for *customer* deployments, not a published list of Vercel's own edge nodes. Vercel's edge network is shared with Next.js deployments globally, but Vercel has never published its edge server IP ranges. There is no ASN isolation — Vercel uses AWS, Cloudflare, and other providers underneath.
**UNVERIFIED**: Some community sources claim Vercel uses 76.76.21.0/24 — not verified from official source.

### Netlify edge
No official public edge IP feed found in any Netlify documentation page. Netlify's CDN uses third-party providers. No dedicated Netlify ASN for edge delivery was identified.

### CDN77
Support docs return 404 or connection errors. No machine-readable IP list found. Reject until an official source is identified.

### KeyCDN
Support doc URL for "IP Ranges" returns ECONNREFUSED. No machine-readable IP list found. Reject until an official source is identified.

### Edgio (Limelight Networks)
AS22822 has not been visible in the global routing table since 2026-04-16. Company appears to have ceased BGP operations. Docs redirect to uplynk.com (unrelated video streaming platform). Reject.

### StackPath (MaxCDN)
Website returns 404. Company appears defunct or merged. Reject.

### Fly.io edge
Docs explicitly state outbound IPs change without notice. Anycast announced via BGP but not published. No machine-readable feed exists. Reject.

### Render edge
Render does not have a CDN edge network in the traditional sense. Its outbound IPs are per-service and region-specific (not a shared CDN edge).

### Deno Deploy
Docs site ECONNREFUSED. No public edge IP range found.

### Section.io
Docs return 403. No public IP feed found. Reject until official source is identified.

### OVHcloud CDN
No machine-readable feed. Official docs note range limitations and state the service is not a true global CDN. OVHcloud's hosting ASN (AS16276) covers all OVHcloud products including VPS, dedicated servers, and CDN — too broad for a CDN-specific soft-tier entry. Reject as CDN edge; could be contextual as cloud provider.

### Sucuri WAF
The `waf.sucuri.net` API requires authentication (returns `"Missing API key"`). No unauthenticated public endpoint. Per-customer WAF assignment model. Reject.

### Barracuda WAF
Docs redirect to generic campus.barracuda.com. No machine-readable IP feed found. Reject.

### F5 Distributed Cloud (Volterra) / Silverline
Support docs 404 or 403. No public machine-readable IP feed. Authenticated portal only. Reject.

### Arbor Cloud / NETSCOUT
No IP publication found. Redirects to marketing page. Reject.

### Radware DefensePro / DefenseFlow
Browser verification challenge; no usable content. No public IP publication found. Reject.

### Nexusguard
Redirect loop. No public IP publication found. Reject.

### Voxility
Website 404. No public IP publication found. Reject.

### jsDelivr
jsDelivr is a logical overlay CDN that uses Fastly, Cloudflare, and Bunny as physical CDN backends. It does not announce its own IP ranges. Coverage through the constituent CDN feeds (Cloudflare, Fastly, Bunny) is already complete.

### Medianova
Website 404. No public IP publication found. Reject.

### BelugaCDN
Website 404. No public IP publication found. Reject.

### Leaseweb CDN
No usable content found on IP ranges page. Reject.

### Cachefly
Docs ECONNREFUSED. Reject.

### Tencent CDN
Docs 404. China-regional deployment; no international public IP feed verified. Reject for now. Relevant only for global feeds with significant China CDN overlap, which is an advanced use case.

### Alibaba Cloud CDN
No dedicated CDN IP range found. The docs reference internal cloud service ranges (100.64.0.0/10) not CDN edge IPs. Reject for now.

### Wangsu / ChinaNetCenter
Connection timeout. No public IP feed. Reject for now.

### ByteDance / Volcengine CDN
Docs content not extractable. Reject for now.

### Huawei Cloud CDN
Docs 404. Reject for now.

### Cloudflare Workers / R2 / Spectrum
Covered by the main Cloudflare CDN entry (same ranges). No separate range published. Not a separate entry needed.

### Cloudflare Magic Transit (Cloudflare-owned IPs)
Covered by the main Cloudflare CDN entry. BYOIP customers use their own address space (not Cloudflare's published list). No separate range.

---

## Open questions / unverified

1. **Vercel community claim (76.76.21.0/24)**: Community sources mention Vercel uses this range. Not confirmed from an official Vercel source. Needs verification against Vercel's public API or official statement before inclusion.

2. **Azure CDN Standard from Microsoft (classic) vs Azure Front Door**: Azure CDN from Microsoft (classic) uses the same `AzureFrontDoor.*` service tags per the retirement migration documentation. This needs confirmation — it is possible some classic CDN ranges use a different tag or separate infrastructure during the transition period through 2027.

3. **Akamai Prolexic vs Akamai CDN**: Both use the same two Akamai ASNs (AS20940, AS16625). There is no BGP or public-source separation between CDN edge and DDoS scrubbing ranges. A BGP-derived set covers both products indiscriminately.

4. **AWS CloudFront vs CloudFront Origin Facing**: `ip-ranges.json` has both `CLOUDFRONT` (203 prefixes) and `CLOUDFRONT_ORIGIN_FACING` (45 prefixes) service tags. The `ORIGIN_FACING` tag covers IPs that connect back to customer origins. Whether to include `ORIGIN_FACING` in the CDN edge soft-tier feed is an implementation decision. For blast-radius purposes, origin-facing IPs blocking means CloudFront cannot pull content from customer servers — this is high blast radius and should be included.

5. **Azure Front Door weekly URL**: The service tags JSON download URL changes weekly (file named with date). A stable mechanism for finding the current URL is needed. Options: (a) poll the discovery page (id=56519) and extract the redirect URL; (b) use the Azure REST API with service principal auth; (c) use the MISP warninglists secondary source which tracks the current URL.

6. **GitHub Actions ASN overlap**: GitHub Actions ranges are Azure VM ranges (AS8075). These already appear in Azure service tags. Whether to deduplicate at source ingestion or let the engine handle deduplication across soft-tier providers is an implementation question.

7. **Bunny.net and Gcore aggregation**: Both return individual /32 IPs. For overlap computation, individual /32 entries work correctly. For display and methodology, noting "individual IPs" vs "CIDR ranges" matters. No implementation blocker, just a documentation point.

8. **OVHcloud CDN specifics**: OVHcloud has a CDN product but it uses the same ASN (AS16276) as all OVHcloud infrastructure. The dedicated CDN IP range, if it exists, is not identified in a machine-readable form. A future pass could check OVHcloud's developer API for CDN-specific ranges.

---

## Sources consulted

### Live endpoints verified (direct WebFetch):
- `https://www.cloudflare.com/ips-v4` — OK, 15 IPv4 ranges
- `https://www.cloudflare.com/ips-v6` — OK, 7 IPv6 ranges
- `https://api.cloudflare.com/client/v4/ips` — OK, JSON structure confirmed
- `https://api.fastly.com/public-ip-list` — OK, 19 IPv4 + 2 IPv6 ranges
- `https://ip-ranges.amazonaws.com/ip-ranges.json` — OK, validated with bash/Python; all 26 service tags enumerated
- `https://d7uri8nf7uskq.cloudfront.net/tools/list-cloudfront-ips` — OK, 214 CloudFront ranges
- `https://download.microsoft.com/download/7/1/d/71d86715-5596-4529-9b13-da13a5de5b63/ServiceTags_Public_20260420.json` — OK, AzureFrontDoor tags enumerated
- `https://www.gstatic.com/ipranges/goog.json` — OK, 111 total prefixes
- `https://www.gstatic.com/ipranges/cloud.json` — OK, 1,177 prefixes with region/service metadata
- `https://my.imperva.com/api/integration/v1/ips` — OK, 11 IPv4 + 1 IPv6 range
- `https://api.bunny.net/system/edgeserverlist` — OK, 572 individual IPv4 IPs
- `https://api.gcore.com/cdn/public-ip-list` — OK, 1000+ individual /32 and /128 addresses
- `https://api.github.com/meta` — OK, all category counts documented
- `https://ip-ranges.atlassian.com/` — OK, 400+ entries with metadata
- `https://app.terraform.io/api/meta/ip-ranges` — OK, 4 categories documented

### Live endpoints that failed or returned no usable IP data:
- Vercel (docs), Netlify (docs), CDN77 (404/ECONNREFUSED), KeyCDN (ECONNREFUSED), Edgio (redirect to unrelated site), StackPath (404), Fly.io (no IP list), Render (no CDN IPs), Deno Deploy (ECONNREFUSED), Section.io (403), Medianova (404), BelugaCDN (404), Leaseweb CDN (empty), Cachefly (ECONNREFUSED), Sucuri (requires auth), Barracuda (redirect), F5 Distributed Cloud (404/403), Arbor/NETSCOUT (redirect), Radware (bot challenge), Nexusguard (redirect loop), Voxility (404), Tencent CDN (404), Alibaba CDN (internal ranges only), Wangsu (timeout), Volcengine CDN (empty), Huawei Cloud CDN (404), OVHcloud CDN (404)

### BGP data (bgp.he.net):
- AS13335 (Cloudflare): 2,533 IPv4 + 3,169 IPv6 originated prefixes
- AS54113 (Fastly): 505 IPv4 + 1,156 IPv6 originated prefixes
- AS36459 (GitHub): 24 IPv4 + 2 IPv6 originated prefixes
- AS19551 (Incapsula/Imperva): 871 IPv4 + 707 IPv6 originated prefixes
- AS20940 (Akamai International): 3,987 IPv4 + 791 IPv6 originated prefixes
- AS16625 (Akamai Technologies): ~2,787 IPv4 + 8 IPv6 originated prefixes
- AS22822 (Limelight/Edgio): not visible in routing table since 2026-04-16
- AS15169 (Google LLC): 1,209 IPv4 + 178 IPv6 originated prefixes
- AS396982 (Google LLC / GCP): 3,341 IPv4 + 198 IPv6 originated prefixes
- AS8075 (Microsoft): primary Azure/O365/AFD ASN

### Documentation pages consulted:
- Cloudflare fundamentals (IP concepts, Magic Transit, Spectrum, R2)
- Azure CDN and Front Door overview (retirement timeline confirmed)
- AWS CloudFront edge locations doc (CLOUDFRONT service tag and dedicated endpoint)
- AWS Global Accelerator documentation (GLOBALACCELERATOR service tag)
- Azure service tags download page (id=56519, weekly cadence confirmed)
- Google Cloud Compute FAQ (cloud.json documentation)
- GitHub IP addresses documentation (meta API and "not exhaustive" caveat)
- Atlassian IP ranges (ip-ranges.atlassian.com — structure confirmed)
- Fly.io networking docs (dynamic IP caveat confirmed)
- Render outbound IPs (not CDN edge)
- Vercel Trusted IPs (inbound customer allowlist, not published edge IPs)
- Bunny.net edgeserverlist reference (individual IPs confirmed)
- Gcore CDN public IP list (individual IPs confirmed)
- Sucuri WAF API (authentication required confirmed)
- Edgio/Limelight redirect analysis (appears defunct)
