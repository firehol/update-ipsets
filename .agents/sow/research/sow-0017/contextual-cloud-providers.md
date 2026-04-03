# Contextual-tier cloud and hosting providers research (SOW-0017)

Research date: 2026-04-29

## Summary table

| Provider | Region | Source URL | Format | Source grade | ASN(s) | Recommended tier | Notes |
|---|---|---|---|---|---|---|---|
| **AWS** | Global | https://ip-ranges.amazonaws.com/ip-ranges.json | JSON (native) | A | AS16509, AS14618 | contextual | Includes customer space. Updated continuously (latest: 2026-04-28). |
| **Azure Public** | Global | https://www.microsoft.com/download/details.aspx?id=56519 | JSON service tags | A | AS8075, AS8068 | contextual | Weekly cadence. Indirect download (no stable direct URL). |
| **Azure Government** | US Gov | https://www.microsoft.com/download/details.aspx?id=57063 | JSON service tags | A | AS8075 | contextual | Separate file, weekly cadence. |
| **Azure China** | China | https://www.microsoft.com/download/details.aspx?id=57062 | JSON service tags | A | AS8075 | contextual | Separate file, weekly cadence. |
| **Google Cloud (GCP)** | Global | https://www.gstatic.com/ipranges/cloud.json | JSON (native) | A | AS396982, AS15169 | contextual | 800+ IPv4 prefixes (2026-04-28). Also `goog.json` for all Google. |
| **Oracle Cloud (OCI)** | Global (24+ regions) | https://docs.oracle.com/en-us/iaas/tools/public_ip_ranges.json | JSON (native) | A | AS31898, AS7160 | contextual | Region+service tagged. Updated 2026-04-28. |
| **DigitalOcean** | Global | https://www.digitalocean.com/geo/google.csv | RFC 8805 geofeed | B | AS14061 | contextual | Geolocation only; ~1,500+ entries. Not a service-role feed. |
| **Linode / Akamai Cloud** | Global | https://geoip.linode.com/ | RFC 8805 geofeed | B | AS63949, AS6364 | contextual | ~2,600+ entries. Geolocation only. |
| **Vultr / Constant** | Global | https://geofeed.constant.com/ | RFC 8805 geofeed | B | AS20473 | contextual | ~450 entries (AS20473 = The Constant Company, parent of Vultr). Geolocation only. |
| **Scaleway** | Europe | https://www.scaleway.com/en/docs/account/reference-content/scaleway-network-information/ | HTML docs (static) | C | AS12876, AS50969 | contextual | Static HTML page, no machine-readable bulk feed. ~18 IPv4 networks documented. |
| **IBM Cloud Classic** | Global | https://cloud.ibm.com/docs/cloud-infrastructure?topic=cloud-infrastructure-ibm-cloud-ip-ranges | HTML docs (static) | C | AS36351, AS13884 | contextual | Static docs only; no public JSON/CSV feed verified. Page load timed out during research. |
| **Yandex Cloud** | Russia/Global | https://yandex.cloud/en/docs/overview/concepts/public-ips | HTML docs (static) | C | AS200350, AS215013 | contextual | Static HTML. 29 IPv4 CIDRs for VPC, 4 for BareMetal documented explicitly. CDN has separate JSON: `https://tech.cdn.yandex.net/prefixes/yc.json`. |
| **Equinix Metal** | Global | https://geofeed.equinixmetal.com/ | RFC 8805 geofeed | B | AS54825, AS29884 | contextual (sunsetting) | ~180 entries. Service being retired 2026-06-30. |
| **T-Systems Open Telekom Cloud** | Europe (Germany) | https://imagefactory.otc.t-systems.com/home/public-services-in-otc-new-ip-addresses | HTML blog post | C | AS8893, AS33920 | contextual | Two static blocks: `100.64.0.0/10` and `198.19.0.0/16`. No machine-readable feed. |
| **Render** | US/EU | https://render.com/docs/outbound-ip-addresses | Dashboard only | D | AS54913 | contextual (later) | Ranges only accessible via dashboard UI, not a public endpoint. |
| **Fly.io** | Global | N/A | None | D | AS55256 | reject | Explicitly no static IP ranges published. Dynamic by design. |
| **Heroku** | US/EU | N/A | None | D | (AWS sub-customer) | reject | No official range feed. Dynos use dynamic IPs; Private Spaces static IPs are per-customer. |
| **Hetzner** | Europe/US | N/A (BGP only) | BGP/RDAP-derived | D | AS24940, AS210083 | contextual (BGP only) | No official published feed. BGP-derived community sources (e.g. github.com/disposable/cloud-ip-ranges) exist but are third-party. |
| **OVHcloud** | Global | N/A (BGP only) | BGP/RDAP-derived | D | AS16276 | contextual (BGP only) | 638 IPv4 prefixes announced from AS16276. No official public JSON/geofeed found. Broad hosting+dedicated+CDN mixed. |
| **Alibaba Cloud** | Global/China | N/A | BGP/RDAP-derived | D | AS45102, AS134963, AS37963 | contextual (BGP only) | No official global public IP feed found. Multiple regional ASNs. Community tools only. |
| **Tencent Cloud** | Global/China | N/A | HTML (private only) | D | AS45090, AS132203 | contextual (BGP only) | Tencent docs only describe RFC 1918 VPC ranges. No public global feed. |
| **Huawei Cloud** | Global/China | N/A | None | D | AS55990, AS136907 | contextual (BGP only) | No official global published feed. AS55990 (China DC), AS136907 (international). |
| **VK Cloud (Mail.ru)** | Russia | N/A | None | D | AS47764, AS60863 | reject | No official published feed. Russia-only cloud. |
| **Selectel** | Russia | N/A | None | D | AS49505, AS50340 | reject | No official published feed. Russia-only cloud. |
| **SberCloud / Cloud.ru** | Russia | N/A | None | D | AS35237 (Sberbank parent) | reject | No official published feed. Russia-only cloud. |
| **Rackspace** | US/UK | N/A | None | D | AS27357, AS33070, AS19994 | reject | No official feed. Legacy cloud (declining), no formal IP range publication. |
| **IONOS / 1&1** | Europe/US | N/A | None | D | AS8560, AS54548 | contextual (BGP only) | No official published feed. 1,391+ IPv4 networks but BGP-derived only. |
| **UpCloud** | Global | N/A | None | D | AS202053, AS25697 | contextual (BGP only) | No official published feed. BGP-derived via RIPEstat. |
| **Zenlayer** | Global | N/A | None | D | AS21859, AS4229 | contextual (BGP only) | No official published feed. 652+ IPv4 networks, edge-heavy. |
| **RunPod** | US/EU | N/A | None | D | AS400441 | reject | GPU cloud, no official published IP range feed. |
| **Lambda Labs** | US | N/A | None | D | AS397441 | reject | GPU cloud, no official published IP range feed. |
| **CoreWeave** | US | N/A | None | D | AS396507 | reject | GPU cloud, no official published IP range feed. |
| **Naver Cloud** | Korea/Global | N/A | None | D | AS23576 | reject | No official published IP range feed. Korea-primary cloud. |
| **NHN Cloud** | Korea | N/A | None | D | AS45974 | reject | No official published IP range feed. Korea-primary cloud. |
| **KT Cloud** | Korea | N/A | None | D | AS4766 (KT parent) | reject | No official published IP range feed. Korea-primary cloud. |
| **Kakao Cloud** | Korea | N/A | None | D | AS9764, AS7625 | reject | No official published IP range feed. Korea-primary cloud. |
| **Baidu AI Cloud** | China | N/A | None | D | (Baidu parent ASNs) | reject | No global official published feed. China-only operation. |
| **JD Cloud** | China | N/A | None | D | AS57202 | reject | No global official published feed. China-only operation. |
| **Lumen/CenturyLink Cloud** | US | N/A | None | D | AS3356, AS209 | reject | "CenturyLink Cloud" as a product is retired; network carrier still active. No feed. |
| **Verizon Cloud** | US | N/A | None | D | AS6167 | reject | Cloud product retired. Carrier ASN still active. |
| **AT&T Business Cloud** | US | N/A | None | D | (AT&T carrier ASNs) | reject | Cloud product retired/merged. No published feed. |
| **VMware Cloud / VMC on AWS** | Global | N/A | None | D | (sub-customer of AWS AS16509) | reject | No separate ASN; rides AWS IP space. Covered by AWS `ip-ranges.json`. |
| **TATA Communications Cloud** | Global/India | N/A | None | D | AS6453, AS4755 | reject | ISP/transit carrier, not a cloud provider with public IP range feed. |
| **NTT Communications Cloud** | Japan/Global | N/A | None | D | AS2914, AS27435 | reject | Transit/ISP carrier. AS27435 (NTT Cloud Infrastructure) has minimal IP allocation. No feed. |
| **Locaweb** | Brazil | N/A | None | D | AS27715, AS53244 | reject | Brazil-local hosting, no official published IP range feed. |
| **Embratel** | Brazil | N/A | None | D | AS4230 | reject | Telecom/hosting, Brazil-local. No official feed. |
| **BSNL Cloud** | India | N/A | None | D | AS9829 | reject | Government ISP, not a cloud provider with published IP feed. |
| **Reliance Jio Cloud** | India | N/A | None | D | AS55836 | reject | Telecom/consumer, no published cloud IP range feed. |
| **Paperspace / DigitalOcean** | US | N/A | None | D | (acquired by DigitalOcean) | reject | Absorbed into DigitalOcean. No separate feed. |
| **Vast.ai** | Global | N/A | None | D | (customer IPs) | reject | Marketplace model: IPs belong to individual GPU host providers. No single feed. |
| **Modal** | US | N/A | None | D | AS396982 (GCP-based) | reject | Runs on GCP. No separate published IP range feed. |
| **Anyscale** | US | N/A | None | D | (AWS-based) | reject | Runs on AWS. No separate published IP range feed. |
| **Railway** | US/EU | N/A | None | D | (GCP-based) | reject | Runs on GCP. No separate published IP range feed. |

---

## Per-provider details

### Tier A (official machine-readable native feeds)

#### AWS (Amazon Web Services)

- **Role**: Global hyperscaler; customer cloud + managed services + infrastructure.
- **URL**: https://ip-ranges.amazonaws.com/ip-ranges.json
- **Format**: JSON. Fields: `ip_prefix`, `region`, `service`, `network_border_group`.
- **Full IP feed**: Yes (all AWS-operated prefixes, not a geofeed).
- **ASNs**: AS16509 (primary), AS14618 (EC2/legacy).
- **Source grade**: A.
- **License**: Public, no explicit license stated. AWS documentation says it is provided for customers to configure firewall rules. Redistributable in context.
- **Update cadence**: Continuous. Latest observed: 2026-04-28T19:07:06. `syncToken` changes on update.
- **Recommended tier**: contextual.
- **Caveats**:
  - AWS explicitly states EC2 address space can back non-EC2 services, so the service tag is approximate.
  - Customer-hosted workloads sit alongside AWS-managed services; ASN-level blocking hits both.
  - GovCloud and China are separate regions in the same file (filter by `region` starting with `us-gov-*` and `cn-*`).
  - No stable direct JSON URL for region-specific subsets; must filter the global file.
  - Co-tenancy is the defining risk: any AWS IP could be hosting legitimate or abusive tenant traffic.

#### Google Cloud Platform (GCP)

- **Role**: Global hyperscaler; customer cloud + Google-managed services.
- **URL**: https://www.gstatic.com/ipranges/cloud.json
- **Format**: JSON. Fields: `ipv4Prefix`/`ipv6Prefix`, `service`, `scope`.
- **Full IP feed**: Yes.
- **Companion**: https://www.gstatic.com/ipranges/goog.json — all Google-operated IPs (broader, includes non-Cloud).
- **ASNs**: AS396982 (GCP), AS15169 (Google including non-GCP).
- **Source grade**: A.
- **License**: Public, no explicit license. Used widely for firewall configuration. Redistributable in context.
- **Update cadence**: Continuous. Latest observed: 2026-04-28T13:09:45.
- **Recommended tier**: contextual.
- **Caveats**:
  - `cloud.json` covers GCP customer space.
  - `goog.json - cloud.json` (difference) gives Google-operated non-customer edge ranges (useful for soft tier: Google service edge).
  - 800+ IPv4 prefixes; region-tagged, so can be filtered for specific use cases.
  - Same co-tenancy issue as AWS.

#### Oracle Cloud Infrastructure (OCI)

- **Role**: Global hyperscaler; customer IaaS.
- **URL**: https://docs.oracle.com/en-us/iaas/tools/public_ip_ranges.json
- **Format**: JSON. Fields: `region`, `cidrs[]` (each with `cidr` and `tags[]`).
- **Service tags**: `OCI`, `OSN` (Oracle Service Network), `OBJECT_STORAGE`.
- **Full IP feed**: Yes.
- **ASNs**: AS31898, AS7160.
- **Source grade**: A.
- **License**: Public Oracle documentation. No explicit redistribution restriction stated. Redistributable in context.
- **Update cadence**: Regular. Latest observed: 2026-04-28T07:16:40.
- **Recommended tier**: contextual.
- **Caveats**:
  - 24+ regions covered.
  - Tags allow filtering by service (e.g., Object Storage specifically if needed).
  - Smaller overall IP space than AWS/Azure/GCP; less co-tenancy noise, but same principle applies.

---

### Tier B (official machine-readable but geofeed or partial)

#### DigitalOcean

- **Role**: Mid-tier VPS cloud; customer-facing developer cloud.
- **URL**: https://www.digitalocean.com/geo/google.csv
- **Format**: RFC 8805 geofeed (pipe-delimited CSV). Fields: IP CIDR, country, region, city, postal code.
- **Full IP feed**: Partial — geolocation only, not service-role data.
- **ASNs**: AS14061.
- **Source grade**: B (official RFC 8805 geofeed, but covers location, not service role).
- **License**: Publicly accessible. No explicit license. Redistributable in context.
- **Update cadence**: Active (last observed 2026-04-28 contents). No stated cadence.
- **Recommended tier**: contextual.
- **Caveats**:
  - ~1,500+ entries covering both IPv4 and IPv6.
  - A geofeed proves DigitalOcean operates these ranges, but does not distinguish customer VPS from DigitalOcean-managed infrastructure.
  - For a full contextual-tier feed, this is the best available official source. Treat as "DigitalOcean-operated space" not "DigitalOcean customer only".

#### Linode / Akamai Connected Cloud

- **Role**: Mid-tier VPS cloud; now part of Akamai.
- **URL**: https://geoip.linode.com/
- **Format**: RFC 8805 geofeed (header: "This file contains a self-published geofeed as defined in RFC 8805").
- **Full IP feed**: Partial — geolocation only.
- **ASNs**: AS63949, AS6364 (Akamai parent).
- **Source grade**: B.
- **License**: Publicly accessible. No explicit license. Redistributable in context.
- **Update cadence**: Active. ~2,600+ entries. No stated cadence.
- **Recommended tier**: contextual.
- **Caveats**:
  - Post-Akamai acquisition, ranges may overlap with Akamai CDN (AS20940) space. The geofeed covers Linode/Akamai Cloud; Akamai CDN edge is a separate service.
  - Same co-tenancy issue as other VPS clouds.

#### Vultr / Constant

- **Role**: Mid-tier VPS cloud; "The Constant Company" is the parent of Vultr.
- **URL**: https://geofeed.constant.com/
- **Format**: RFC 8805 geofeed. Updated 2026-04-28.
- **Full IP feed**: Partial — geolocation only.
- **ASNs**: AS20473 (The Constant Company, LLC) — 1,553 IPv4, 389 IPv6 prefixes.
- **Source grade**: B.
- **License**: Publicly accessible. No explicit license. Redistributable in context.
- **Update cadence**: Active. ~450 entries. No stated cadence.
- **Recommended tier**: contextual.
- **Caveats**:
  - Geofeed URL is `geofeed.constant.com`, not `vultr.com`. The parent company is Constant; brand is Vultr.
  - Vultr bare metal and cloud share the same AS20473 IP space.

#### Azure (Public, Government, China)

- **Role**: Global hyperscaler; customer cloud + Microsoft-managed services.
- **URLs**:
  - Public: https://www.microsoft.com/download/details.aspx?id=56519 → `ServiceTags_Public_<YYYYMMDD>.json`
  - Government: https://www.microsoft.com/download/details.aspx?id=57063 → `ServiceTags_AzureGovernment_<YYYYMMDD>.json`
  - China: https://www.microsoft.com/download/details.aspx?id=57062 → `ServiceTags_China_<YYYYMMDD>.json`
- **Format**: JSON service tags. Fields: `name`, `id`, `properties.region`, `properties.addressPrefixes[]`.
- **Full IP feed**: Yes per file. No single unified URL; must download from the Download Center landing page.
- **Issue**: The actual JSON download URL changes weekly (date-stamped). There is no stable permanent direct download URL.
- **Alternative stable discovery**: https://www.microsoft.com/en-us/download/confirmation.aspx?id=56519 redirects to the latest file, but the redirect target changes. The recommended programmatic discovery is:
  - `https://download.microsoft.com/download/7/1/D/71D86715-5596-4529-9B13-DA13A5DE5B63/ServiceTags_Public_<date>.json`
  - Or use the Azure REST API endpoint: `https://management.azure.com/subscriptions/{subscriptionId}/providers/Microsoft.Network/locations/{location}/serviceTags?api-version=2021-02-01` (requires auth).
- **ASNs**: AS8075 (Microsoft primary), AS8068.
- **Source grade**: A (data quality and freshness) / B from a feed-integration standpoint (no stable direct URL).
  - **Rationale for A**: The data is officially published weekly by Microsoft, is machine-readable JSON, and is used widely for firewall configuration.
  - **The challenge**: Stable URL discovery requires scraping the download page or using a third-party mirror. The MISP warninglist for Azure is a good secondary source.
- **License**: Microsoft Download Center terms. Publicly accessible. Redistributable for security/network purposes.
- **Update cadence**: Weekly. Latest observed: 2026.04.20.
- **Recommended tier**: contextual.
- **Caveats**:
  - `ServiceTags_Public` is IPv4-only. IPv6 support announced as "future" in the docs — verify before relying on IPv4-completeness assumption.
  - Service tags allow very granular filtering (e.g., `AzureActiveDirectory`, `Storage.WestEurope`, `AzureLoadBalancer`). For contextual tier, `AzureCloud` gives the full cloud range; specific service tags enable more precise soft-tier entries.
  - Government and China are geopolitically separate; each should be tagged distinctly in the config.
  - Customer workloads and Microsoft-managed services coexist in the same prefixes.

#### Yandex Cloud

- **Role**: Russian hyperscaler with limited global presence.
- **URL**: https://yandex.cloud/en/docs/overview/concepts/public-ips
- **Format**: Static HTML page listing CIDRs explicitly.
- **CDN JSON subset**: https://tech.cdn.yandex.net/prefixes/yc.json (CDN ranges only; not full cloud).
- **Full IP feed**: No — no bulk machine-readable download for all services.
- **ASNs**: AS200350, AS215013.
- **Source grade**: C (static HTML docs listing explicit prefixes; plus partial JSON for CDN).
- **IP ranges documented (2026-03-31)**:
  - VPC: 31.44.8.0/21, 37.230.168.0/23, 37.230.172.0/22, 37.230.188.0/22, 45.133.96.0/22, 46.21.244.0/22, 46.243.210.0/23, 51.250.0.0/17, 62.84.112.0/20, 81.26.176.0/20, 84.201.128.0/18, 84.252.128.0/20, 89.169.128.0/18, 89.232.188.0/22, 92.255.1.0/24, 92.255.3.0/24, 93.77.160.0/19, 103.76.52.0/22, 111.88.144.0/20, 111.88.240.0/20, 130.193.32.0/19, 158.160.0.0/16, 178.154.192.0/18, 178.170.222.0/24, 185.206.164.0/22, 193.32.216.0/22, 213.165.192.0/19, 217.28.224.0/20, 217.198.168.0/21
  - BareMetal: 94.126.204.0/22, 94.139.248.0/22, 92.255.58.0/23, 89.223.20.0/24
  - Smart Web Security: 46.243.212.0/24, 194.247.51.0/24
- **License**: Public docs, no explicit redistribution restriction.
- **Update cadence**: Documentation updated 2026-03-31. No stated machine-readable update cadence.
- **Recommended tier**: contextual.
- **Caveats**:
  - Russia-primary operation with some international regions. Geopolitical sanctions context may affect operator policy.
  - Static HTML list requires periodic manual re-checking for changes; no automated freshness check possible without scraping.
  - CDN JSON (tech.cdn.yandex.net) may have higher update cadence than the main docs page.
  - The `yandex.com/ips` page covers all Yandex-operated IPs (broader, includes search engine/consumer services), not just Yandex Cloud.

#### Equinix Metal

- **Role**: Bare-metal cloud (customer-provisioned, not shared VMs).
- **URL**: https://geofeed.equinixmetal.com/
- **Format**: RFC 8805 geofeed.
- **Full IP feed**: Partial — geolocation only.
- **ASNs**: AS54825, AS29884 (Equinix parent).
- **Source grade**: B.
- **License**: Publicly accessible. No explicit license.
- **Update cadence**: Active. ~180 entries.
- **Recommended tier**: contextual (with sunset caveat).
- **Caveats**:
  - **CRITICAL**: Equinix Metal is being retired. Commercial sales stopped. Service sunset date: June 30, 2026.
  - After June 2026 this feed becomes historical. Do not add as a long-lived reference source.
  - Bare metal means customer has dedicated hardware; less co-tenancy concern than shared VPS, but IP ranges still include multiple customers.

---

### Tier C (official static HTML/docs only)

#### Scaleway

- **Role**: European VPS/cloud; primarily France/Amsterdam/Warsaw.
- **URL**: https://www.scaleway.com/en/docs/account/reference-content/scaleway-network-information/
- **Format**: Static HTML documentation page.
- **Machine-readable feed**: None found.
- **ASNs**: AS12876 (Scaleway SAS), AS50969.
- **Source grade**: C.
- **Documented IP blocks (per third-party aggregation, 2026-04-13)**: ~18 IPv4 networks including 51.158.0.0/16, 78.232.0.0/16, 51.15.0.0/17 (Amsterdam).
- **License**: Public docs.
- **Update cadence**: Unknown; static page.
- **Recommended tier**: contextual.
- **Caveats**:
  - No official JSON/CSV/TXT machine-readable feed confirmed.
  - BGP-derived via RIPE/RIPEstat is the practical alternative.
  - Scaleway Online.net (sister brand) shares some ASN space.
  - Smaller scope than major hyperscalers; lower blast radius from co-tenancy.

#### IBM Cloud Classic

- **Role**: Hyperscaler-legacy; classic infrastructure (bare metal + virtual servers).
- **URL**: https://cloud.ibm.com/docs/cloud-infrastructure?topic=cloud-infrastructure-ibm-cloud-ip-ranges
- **Format**: Static HTML documentation.
- **Machine-readable feed**: None confirmed (docs page timed out during direct fetch; no bulk download URL found in search results).
- **ASNs**: AS36351 (Softlayer Technologies / IBM Cloud classic), AS13884.
- **Source grade**: C.
- **License**: IBM Cloud documentation terms. Publicly accessible.
- **Update cadence**: Unknown; static page.
- **Recommended tier**: contextual.
- **Caveats**:
  - IBM Cloud VPC is a separate product from Classic; they may have separate IP ranges.
  - IBM has been substantially reducing its cloud footprint; freshness of static docs is a concern.
  - No programmatic way to get current ranges without scraping or BGP fallback.

#### T-Systems Open Telekom Cloud (now T Cloud Public)

- **Role**: European cloud (Germany-centric, GDPR-focused); rebranded to T Cloud Public.
- **URL**: https://imagefactory.otc.t-systems.com/home/public-services-in-otc-new-ip-addresses (blog post)
- **Format**: Static blog post / HTML page.
- **Machine-readable feed**: None.
- **Documented blocks**: `100.64.0.0/10`, `198.19.0.0/16` (per official post, 2021, still cited as current).
- **ASNs**: AS8893 (T-Systems), AS33920.
- **Source grade**: C.
- **License**: Public.
- **Update cadence**: Low — last meaningful update 2021; rebranding to T Cloud Public happened without new IP publication.
- **Recommended tier**: contextual.
- **Caveats**:
  - `100.64.0.0/10` is RFC 6598 shared address space (carrier-grade NAT). It appears here as OTC internal service IP, but is not publicly routable.
  - `198.19.0.0/16` is benchmarking reserved space (RFC 2544). This is unusual to see as public cloud service IPs.
  - These IP ranges may be internal service endpoints only, not publicly announced prefixes.
  - Very limited global footprint. Germany/Europe only.

---

### BGP/RDAP-only contextual candidates (Grade D, include with explicit labeling)

These providers have no official published IP range feed. They can only be covered via BGP prefix announcements from their ASNs. Such coverage is labeled `generated_bgp` in the proposed config model.

#### Hetzner

- **Role**: German budget hosting/cloud provider. Significant in European VPS/dedicated market.
- **ASNs**: AS24940 (Hetzner Online GmbH — primary), AS210083 (Hetzner Cloud).
- **Scope**: AS24940 announces 88 IPv4 ranges + 12 IPv6; AS210083 is cloud-specific.
- **Official feed**: None confirmed. Hetzner docs only cover individual customer IP management.
- **Community source**: `github.com/disposable/cloud-ip-ranges` maintains a regularly-updated Hetzner JSON derived from BGP data. Last updated 2026-04-21.
- **Source grade**: D.
- **Recommended tier**: contextual (BGP-derived, labeled `generated_bgp` or `community`).
- **Caveats**:
  - Hetzner is a high-volume VPS provider with known abuse issues; contextual warning is relevant.
  - No RPKI/geofeed link published in RIPE WHOIS.

#### OVHcloud

- **Role**: Global hosting/cloud/bare-metal. One of the world's largest hosting providers by IP space.
- **ASNs**: AS16276 (main OVH SAS) — 638 IPv4, 42 IPv6 prefixes (17,783 /24 equivalents). Plus sub-ASNs for Kimsufi/SoYouStart.
- **Official feed**: No official JSON/CSV/geofeed found. RIPE WHOIS geofeed attribute: not published as of research date.
- **Source grade**: D.
- **Recommended tier**: contextual (BGP-derived).
- **Caveats**:
  - OVHcloud is massive. Their IP space includes bare-metal, VPS, CDN, telecom, and internal infrastructure all under AS16276. A contextual warning that covers AS16276 entirely is very broad.
  - OVHcloud has historically high abuse volume from their VPS tier.
  - Kimsufi (AS16276 subset) and SoYouStart (AS16276 subset) are OVH budget brands with even higher abuse rates.
  - Consider splitting into OVH core hosting vs. OVH CDN if/when official tag-level data becomes available.

#### Alibaba Cloud

- **Role**: Chinese hyperscaler; significant international presence (Singapore, US, EU, etc.).
- **ASNs**: AS45102 (Alibaba US Technology), AS134963 (Singapore), AS37963 (China domestic Aliyun).
- **Official feed**: None. Alibaba Cloud international docs do not publish a machine-readable global IP range feed.
- **Source grade**: D.
- **Recommended tier**: contextual (BGP-derived).
- **Caveats**:
  - Multiple distinct ASNs for different regions/subsidiaries. No single ASN covers all Alibaba Cloud.
  - Chinese domestic traffic (AS37963) and international traffic (AS45102, AS134963) are distinct.
  - No redistribution restriction because there is no official feed to redistribute.

#### Tencent Cloud

- **Role**: Chinese hyperscaler; international operations (Singapore, Frankfurt, Tokyo, etc.).
- **ASNs**: AS45090 (Tencent Building), AS132203 (international).
- **Official feed**: The tencentcloud.com docs page for IP ranges only lists RFC 1918 private IP ranges for VPC configuration — not public ranges. No global public feed exists.
- **Source grade**: D.
- **Recommended tier**: contextual (BGP-derived).
- **Caveats**:
  - AS45090 announced 112 IPv4 networks (2026-03-16 per search data).
  - China-domestic vs. international distinction matters for geopolitical policy.
  - Tencent Cloud international platform at tencentcloud.com is separate from the China-domestic cloud.

#### Huawei Cloud

- **Role**: Chinese hyperscaler; significant international operations (Germany, Singapore, etc.).
- **ASNs**: AS55990 (Huawei Cloud Service Data Center — China), AS136907 (Huawei International PTE, 621 routes).
- **Official feed**: No official global published feed.
- **Source grade**: D.
- **Recommended tier**: contextual (BGP-derived).
- **Caveats**:
  - Huawei Cloud international (AS136907) is distinct from domestic (AS55990).
  - Open Telekom Cloud (T-Systems) is a white-labeled Huawei Cloud product — separate entity.
  - Geopolitical sensitivity in some jurisdictions (US, UK, Australia).

#### IONOS / 1&1

- **Role**: German hosting/cloud provider; significant European market.
- **ASNs**: AS8560 (IONOS SE — primary, 1,391+ networks), AS54548 (IONOS Cloud Inc US), AS15418, AS51862.
- **Official feed**: None found. Docs only cover customer-facing IP management for their products.
- **Source grade**: D.
- **Recommended tier**: contextual (BGP-derived).
- **Caveats**:
  - 1&1/IONOS historically high in spam-feed co-tenancy due to shared hosting.
  - IONOS Cloud (AS54548) is the newer cloud-focused brand; AS8560 covers all legacy products.

---

## Reject (no usable source — defer or BGP-only with insufficient value)

### Providers rejected for inclusion as contextual reference feeds

The following were evaluated and rejected for inclusion as managed reference-feed entries. They may appear incidentally via BGP-derived union sets but should not be first-class reference entries.

| Provider | Reason |
|---|---|
| **Fly.io** | Explicitly publishes no IP ranges. Dynamic by design. Egress IPs unstable. |
| **Heroku** | No published ranges. Dynos use dynamic IP from AWS space. Private Spaces are per-customer. |
| **Railway** | Runs on GCP. No separate IP space. Covered by GCP `cloud.json`. |
| **Modal** | Runs on GCP. Same as above. |
| **Anyscale** | Runs on AWS. Covered by AWS `ip-ranges.json`. |
| **VMware Cloud on AWS** | Uses AWS IP space (AS16509). No separate ASN. |
| **Render** | Ranges only via dashboard; no public endpoint. CIDR format verified but retrieval requires auth. Consider scrape if operator access available. |
| **Paperspace** | Acquired by DigitalOcean 2023; covered by DigitalOcean geofeed. |
| **Vast.ai** | Marketplace of third-party GPU hosts. No single provider IP space. |
| **RunPod** | GPU rental marketplace. No official published IP range feed. |
| **Lambda Labs** | GPU cloud. No official published IP range feed. |
| **CoreWeave** | GPU cloud. No official published IP range feed. |
| **VK Cloud (Mail.ru)** | Russia-only; no published IP range feed. |
| **Selectel** | Russia-only; no published IP range feed. |
| **SberCloud / Cloud.ru** | Russia-only; no published IP range feed. |
| **Rackspace** | Declining cloud product, no published IP range feed. |
| **Lumen/CenturyLink Cloud** | Cloud product retired. Carrier network only. |
| **Verizon Cloud** | Product retired. Carrier ASN remains but no cloud product feed. |
| **AT&T Business Cloud** | Product retired/absorbed. No cloud IP range feed. |
| **Naver Cloud** | Korea-primary; no official published IP range feed. AS23576. |
| **NHN Cloud** | Korea-only; no official published IP range feed. AS45974. |
| **KT Cloud** | Korea-only; KT (AS4766) is primarily a telecom carrier. |
| **Kakao Cloud** | Korea-only; no official published IP range feed. |
| **Baidu AI Cloud** | China-only (international version is very limited); no global published feed. |
| **JD Cloud** | China-only; no official published feed. |
| **TATA Communications** | Transit/ISP carrier, not a cloud provider in the relevant sense. |
| **NTT Communications Cloud** | AS27435 has minimal IP allocation. NTT is primarily a transit/ISP carrier. |
| **Locaweb** | Brazil-local hosting. No official published feed. |
| **Embratel** | Brazilian telecom. No cloud IP range feed. |
| **BSNL Cloud** | Indian government telecom. Not a cloud provider with published IP range feed. |
| **Reliance Jio Cloud** | Indian telecom/consumer, not a cloud provider with published IP range feed. |

---

## Open questions / unverified

1. **Azure stable direct URL**: The current workflow requires scraping `https://www.microsoft.com/en-us/download/details.aspx?id=56519` to extract the latest download URL. Verify whether the Azure REST API endpoint (`/providers/Microsoft.Network/locations/{location}/serviceTags`) can be used without subscription auth for public service tags. MISP warninglist is a valid secondary source.

2. **IBM Cloud VPC vs Classic**: IBM Cloud VPC (newer product) has a different network architecture than Classic. The docs page for Classic was not loadable during research. Verify whether IBM Cloud VPC also has published ranges and whether VPC and Classic share ASN space.

3. **OVHcloud geofeed via RIPE**: RIPE now supports a `geofeed:` attribute on inetnum objects (RFC 9632, deployed 2026-03-04). OVHcloud may have published a geofeed via this mechanism post-research. Check `https://apps.db.ripe.net/db-web-ui/query?searchtext=AS16276` for the `geofeed:` attribute on their inet-num objects.

4. **Scaleway machine-readable**: Scaleway may publish a JSON or TXT file through their documentation system that was not surfaced by the research crawl. Verify by fetching the actual page content vs. the navigation stub that was returned.

5. **Alibaba Cloud international BGP completeness**: The research identified AS45102 (US), AS134963 (Singapore) as the primary international ASNs. Verify whether there are additional Alibaba Cloud international ASNs for EU/MENA operations before treating AS45102+AS134963 as complete BGP coverage.

6. **Tencent Cloud international ASN**: AS132203 appears in search results as Tencent international. Verify this via `bgp.tools` or RIPEstat before including in any BGP-derived set.

7. **Render**: The outbound IP documentation is login-gated by dashboard. Check if Render publishes a JSON endpoint (documented or undocumented) for their regional CIDR blocks. The example `216.24.60.0/24` format was confirmed.

8. **Equinix Metal sunset**: The feed URL `https://geofeed.equinixmetal.com/` will cease being updated after June 2026. If the feed is added, add a sunset note and a TODO to remove the entry after Q3 2026.

9. **Yandex Cloud CDN JSON**: Verify `https://tech.cdn.yandex.net/prefixes/yc.json` is still accessible and whether it covers more than just CDN ranges (could serve as a machine-readable partial source for Grade B treatment).

10. **SberCloud / Cloud.ru**: After the Sberbank/SberCloud rebrand to Cloud.ru, verify current ASN assignment. The search identified AS35237 as the Sberbank parent ASN, but the cloud product may use different routing.

---

## Sources consulted

Direct fetches:
- https://ip-ranges.amazonaws.com/ip-ranges.json
- https://www.gstatic.com/ipranges/cloud.json
- https://docs.oracle.com/en-us/iaas/tools/public_ip_ranges.json
- https://www.digitalocean.com/geo/google.csv
- https://geoip.linode.com/
- https://geofeed.constant.com/
- https://www.microsoft.com/en-us/download/details.aspx?id=56519
- https://www.microsoft.com/en-us/download/details.aspx?id=57063
- https://www.microsoft.com/en-us/download/details.aspx?id=57062
- https://yandex.cloud/en/docs/vpc/concepts/ips
- https://yandex.cloud/en/docs/overview/concepts/public-ips
- https://geofeed.equinixmetal.com/
- https://bgp.tools/as/16276
- https://bgp.tools/as/20473
- https://docs.hetzner.com/robot/dedicated-server/ip/ip-addresses/
- https://fly.io/docs/networking/services/
- https://render.com/docs/outbound-ip-addresses
- https://www.tencentcloud.com/document/product/215/35529

Web searches conducted:
- "Scaleway official IP ranges network addresses published feed 2026"
- "IBM Cloud IP ranges official public JSON 2026 site:cloud.ibm.com"
- "Alibaba Cloud official IP ranges JSON feed ASN international 2026"
- "Tencent Cloud official public IP ranges JSON feed international 2026"
- "Huawei Cloud official IP ranges published feed ASN international 2026"
- "Hetzner Cloud official IP ranges published download 2026"
- "OVHcloud official IP ranges published JSON download 2026"
- "OVHcloud RIPE whois geofeed attribute AS16276 2026"
- "Equinix Metal official IP ranges published feed 2026"
- "Yandex Cloud official IP ranges published feed yandex.cloud 2026"
- "UpCloud official IP ranges published feed ASN 2026"
- "IONOS cloud official IP ranges published feed ASN 2026"
- "Naver Cloud Platform official IP ranges published feed Korea 2026"
- "NHN Cloud KT Cloud Korea official IP ranges ASN published 2026"
- "Baidu AI Cloud JD Cloud official IP ranges published feed 2026"
- "VK Cloud Mail.ru Cloud official IP ranges published feed ASN 2026"
- "Selectel Russia cloud official IP ranges published feed 2026"
- "SberCloud Russia official IP ranges ASN published 2026"
- "RunPod Lambda Labs CoreWeave GPU cloud official IP ranges published 2026"
- "Fly.io Render Railway Heroku official IP ranges published 2026"
- "Contabo Time4VPS HostHatch LeaseWeb official IP ranges published feed 2026"
- "Zenlayer official IP ranges published feed ASN 2026"
- "Cloudflare Workers egress IPs published official ranges 2026"
- "Rackspace Cloud official IP ranges published feed ASN 2026"
- "PhoenixNAP Hivelocity ColoCrossing official IP ranges published feed 2026"
- "Azure China ServiceTags_China download Microsoft official IP ranges 2026"
- "Heroku official static IP ranges published 2026 outbound"
- "Lumen CenturyLink cloud IP ranges ASN retired 2026"
- "VMware Cloud AWS official IP ranges vmc published feed ASN 2026"
- "AT&T Business cloud Verizon Enterprise cloud retired IP ranges 2026"
- "T-Systems Open Telekom Cloud Telekom MMS official IP ranges published 2026"
- "Locaweb Embratel Brazil cloud official IP ranges ASN published 2026"
- "TATA Communications cloud official IP ranges published ASN 2026"
- "NTT Communications cloud official IP ranges published ASN 2026"
- "Kakao Cloud Korea official IP ranges ASN published 2026"
- "Render.com official outbound IP ranges published 2026"

---

## Important note on contextual tier semantics

Contextual ≠ whitelist. Every provider in this document can and does host abusive customer workloads alongside legitimate infrastructure. The contextual classification means:

- Overlap with these ranges is **policy-dependent context**, not evidence the feed is wrong.
- An operator enforcing a blocklist against an AWS IP may be entirely correct if the IP is a specific customer VM hosting malware.
- An operator blindly blocking all of AS16509 to suppress one abusive IP is almost certainly wrong.
- The feed page should surface "X% of this feed overlaps with AWS-operated space" as a factual collateral-risk signal, allowing operators to decide whether that overlap is acceptable for their deployment.

This is distinct from the **hard** tier (public DNS, root DNS: overlap is a feed-quality emergency) and the **soft** tier (CDN edges, developer platforms: overlap demands review, not immediate enforcement).
