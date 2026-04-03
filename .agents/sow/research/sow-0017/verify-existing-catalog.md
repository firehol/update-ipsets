# Verify existing provider_infrastructure catalog (SOW-0017)

Audited: 2026-04-29  
MISP local clone: `/tmp/misp-warninglists-critical` at commit `9397afe`  
Primary source checks: performed live via HTTP and GitHub API  

---

## Summary table

| File | Tier (recommended) | Quality | Role | Drift? | Action |
|---|---|---|---|---|---|
| datacenters.yaml | reject | D | — | stale 2023 | Remove or keep as non-critical provider context only |
| misp_akamai.yaml | soft | D | cdn_edge | YES — BGP-only, stale 2024-04-22 | Keep as secondary/generated_bgp; no primary feed exists |
| misp_amazon_aws.yaml | contextual | A | cloud_customer_hosting | YES — MISP 3,629 vs primary 10,161 entries | Replace with primary `ip-ranges.amazonaws.com` |
| misp_apple.yaml | soft/contextual | B | software_update | No significant drift (4 entries) | Keep; label soft/contextual |
| misp_cloudflare.yaml | soft | A | cdn_edge | Minimal (MISP = primary) | Keep or replace with primary; MISP matches Cloudflare ips/ |
| misp_fastly.yaml | soft | A | cdn_edge | YES — 2 missing ranges, supernet vs /17 | Replace with primary `api.fastly.com/public-ip-list` |
| misp_github.yaml | soft | A | developer_platform | Overcount — 4,447 MISP vs ~90 core ranges | Scope review needed; primary is `api.github.com/meta` |
| misp_googlebot.yaml | reject | A | — | Current | Wrong category — crawler, not infra |
| misp_google_gcp.yaml | contextual | A | cloud_customer_hosting | YES — MISP 424 vs primary 862 IPv4 | Replace with primary `gstatic.com/ipranges/cloud.json` |
| misp_google_gmail_sending_ips.yaml | soft | A | email_delivery | Current (matches SPF) | Keep; upgrade to primary SPF-derived |
| misp_microsoft_azure.yaml | contextual | B | cloud_customer_hosting | YES — MISP 2,683 stale 2024-12-03; primary is download-portal | Keep MISP as secondary; note staleness |
| misp_microsoft_azure_china.yaml | contextual | B | cloud_customer_hosting | Stale 2024-12-03 | Same as azure; secondary only |
| misp_microsoft_azure_germany.yaml | contextual | B | cloud_customer_hosting | Stale 2024-12-03; Azure Germany is largely deprecated | Flag as possibly obsolete |
| misp_microsoft_azure_us_gov.yaml | contextual | B | cloud_customer_hosting | Stale 2024-12-03 | Secondary only |
| misp_microsoft_office365_cn.yaml | contextual | A | cloud_service_tag | Current (version 20260428) | Keep as contextual secondary |
| misp_microsoft_office365_ip.yaml | soft | A | cloud_service_tag | Current (version 20260428) | Keep; prefer primary `endpoints.office.com` |
| misp_openai_gptbot.yaml | reject | A | — | Current | Wrong category — crawler, not infra |
| misp_ovh_cluster.yaml | reject | C | — | Stale 2024-04-22; individual IPs not CIDR | Remove from critical catalog |
| misp_public_dns.yaml | reject (as hard) | D | — | Stale 2024-06-15; 62,745 entries — too broad | Do not use as hard reference |
| misp_smtp_receiving_ips.yaml | soft | B | email_delivery | Current (20260428); no source documentation | Keep as secondary email infra |
| misp_smtp_sending_ips.yaml | soft | B | email_delivery | Current (20260428); no source documentation | Keep as secondary email infra |
| misp_stackpath.yaml | reject | D | — | Stale 2024-04-22; StackPath sold CDN to Akamai Aug 2023 | Remove — provider no longer independent |
| misp_telegram.yaml | contextual | A | — | MISP matches primary; IPv6 included | Recategorize — messaging platform, not critical infra |
| misp_tenable_cloud.yaml | reject | A | — | Current (20260428) | Wrong category — scanner, not infra |
| misp_umbrella_blockpage.yaml | reject | A | — | Current (20260428); only 6 IPs | Wrong semantic — blockpage, not resolver |
| misp_zscaler.yaml | soft/contextual | A | cloud_proxy | Drift — MISP 78 vs primary 87 ranges | Keep as secondary; primary is `config.zscaler.com` |

---

## Per-file audit

### datacenters.yaml

1. **Internal name**: `datacenters`
2. **Current category**: `provider_infrastructure`
3. **Current `use:` roles**: none
4. **Current `dont_redistribute`**: not set
5. **Source URL**: `https://raw.githubusercontent.com/client9/ipcat/master/datacenters.csv`
6. **Upstream sanity check**:
   - URL returns HTTP 200 and file is accessible.
   - Upstream repo `client9/ipcat` last commit: **2023-02-02** (over 2 years ago, effectively unmaintained).
   - Last commit to `datacenters.csv` specifically: **2019-02-20**.
   - Data is CSV format: start IP, end IP, provider name, provider URL.
   - ~3,429 rows covering many hosting/datacenter providers including AWS (very broad).
   - The repo README itself says "no longer actively maintained".
7. **Semantic check**:
   - The feed is intentionally broad: "datacenters, co-location centers, shared and virtual webhosting providers" — i.e., any IP end-consumers should not be using.
   - This is the inverse of critical-infrastructure. It is a hosting-identification list, not a service-criticality list.
   - Includes large swaths of AWS, GCP, Azure, OVH, etc. alongside smaller hosters.
8. **Tier recommendation**: **reject** as a critical-infrastructure reference. This feed's semantics conflict with the critical-infrastructure model — it identifies "not-consumer" IPs, which includes both genuine critical infrastructure and abusive hosting equally.
9. **Source quality grade**: **D** — the primary source (`client9/ipcat`) has had no commits to the data file since 2019 (over 6 years). The project is unmaintained.
10. **License / redistribution**: GPLv3. May require downstream copyleft considerations.
11. **`critical_role` recommendation**: none — reject.
12. **Notes / drift / problems**:
   - The SOW's prior research already explicitly excluded this feed: "datacenters: too broad; provider context only."
   - The data is badly stale, missing hundreds of AWS, GCP, Azure ranges added since 2019.
   - If kept at all, it should be a non-critical provider reference with an explicit staleness warning and no `use: [critical_infrastructure]` annotation.

---

### misp_akamai.yaml

1. **Internal name**: `misp_akamai`
2. **Current category**: `provider_infrastructure`
3. **Current `use:` roles**: none
4. **Current `dont_redistribute`**: not set
5. **Source URL**: `https://raw.githubusercontent.com/MISP/misp-warninglists/main/lists/akamai/list.json`
6. **Upstream sanity check**:
   - URL returns HTTP 200.
   - MISP description: "Akamai IP ranges from BGP search". **No refs/citations** — source is BGP data, not an official Akamai publication.
   - MISP list version: **20240422** (April 2024). GitHub last commit: **2024-04-22T07:20:14Z** — over 1 year stale.
   - Live GitHub fetch shows **312 entries** (local clone shows 268; difference may indicate an interim update not cached locally).
   - Data: all CIDR ranges (e.g., `103.11.223.0/24`, `104.64.0.0/10`).
7. **Semantic check**:
   - "BGP search" means this was derived from BGP routing tables, not from Akamai's own published IP manifest.
   - Akamai does not publish a complete machine-readable public IP feed for its CDN edge. The SOW research confirmed: "no unauthenticated official complete public bulk feed found."
   - The MISP BGP-derived list represents Akamai's routed prefixes at a point in time. Akamai adds/withdraws ranges. The list is stale.
8. **Tier recommendation**: **soft** — Akamai is a major CDN with soft-whitelist semantics (high collateral risk if blocked broadly). However, source is BGP-derived.
9. **Source quality grade**: **D** — no official Akamai machine-readable feed exists. BGP derivation is the only option; it is inherently stale without continuous BGP refresh.
10. **License / redistribution**: CC0 1.0 (MISP). No redistribution concerns.
11. **`critical_role` recommendation**: `cdn_edge`
12. **Notes / drift / problems**:
   - Primary upstream does not exist; MISP is the best available secondary source.
   - The `provenance: secondary_upstream` field is already set correctly.
   - Must be labeled `source_type: generated_bgp` when tagged with critical metadata.
   - Staleness (1 year+) is a real concern for a CDN provider that changes edge ranges.

---

### misp_amazon_aws.yaml

1. **Internal name**: `misp_amazon_aws`
2. **Current category**: `provider_infrastructure`
3. **Current `use:` roles**: none
4. **Current `dont_redistribute`**: not set
5. **Source URL**: `https://raw.githubusercontent.com/MISP/misp-warninglists/main/lists/amazon-aws/list.json`
6. **Upstream sanity check**:
   - URL returns HTTP 200.
   - MISP description cites `https://ip-ranges.amazonaws.com/ip-ranges.json` (official AWS source). Version: **20260428** — current.
   - MISP count: **3,629 IPv4 entries**.
   - AWS primary source (`ip-ranges.amazonaws.com/ip-ranges.json`): **10,161 IPv4 prefixes** as of 2026-04-28, created "2026-04-28-19-07-06".
   - **Major drift**: MISP has 35% of what the primary publishes. This is expected — MISP may filter to certain service types or the flattening/deduplication differs.
7. **Semantic check**:
   - MISP sources from AWS `ip-ranges.json` but its 3,629 vs 10,161 gap suggests it publishes only a subset (possibly deduplicated unique prefixes vs all service-tagged overlapping entries).
   - AWS ip-ranges.json includes the same prefix for multiple services (e.g., EC2 + S3 + ROUTE53 all in `us-east-1`). The total 10,161 includes duplicates; unique CIDRs are fewer.
   - The semantic question is: what does AWS range overlap mean? It means "this IP belongs to an AWS customer or AWS service" — the SOW correctly categorizes this as **contextual**, not hard.
   - The feed name implies "all AWS" — it is broad cloud/customer hosting, not a specific critical service range.
8. **Tier recommendation**: **contextual** — AWS ranges include both critical AWS service infrastructure and millions of customer workloads, including abusive ones.
9. **Source quality grade**: **A** — primary source is machine-readable, official, and current. MISP re-publishes from it, but the entry count gap shows incomplete coverage.
10. **License / redistribution**: CC0 1.0 (MISP). AWS terms say IP ranges are for network administration purposes; no explicit redistribution restriction.
11. **`critical_role` recommendation**: `cloud_customer_hosting`
12. **Notes / drift / problems**:
   - **Significant drift**: MISP's 3,629 vs primary's 10,161. When implementing critical-infrastructure overlaps, prefer the primary source for accuracy.
   - Recommended action: replace MISP source with direct `ip-ranges.amazonaws.com/ip-ranges.json` for the contextual reference feed, or document MISP as incomplete secondary.
   - The SOW plan already lists `aws ip-ranges.json` as the preferred primary for the contextual tier.

---

### misp_apple.yaml

1. **Internal name**: `misp_apple`
2. **Current category**: `provider_infrastructure`
3. **Current `use:` roles**: none
4. **Current `dont_redistribute`**: not set
5. **Source URL**: `https://raw.githubusercontent.com/MISP/misp-warninglists/main/lists/apple/list.json`
6. **Upstream sanity check**:
   - URL returns HTTP 200.
   - MISP version: **20241023** (October 2024). GitHub last commit: **2024-10-23T06:37:54Z**.
   - MISP count: **4 entries only**: `17.0.0.0/8`, `192.12.74.0/24`, `192.42.249.0/24`, `204.79.190.0/24`.
   - Apple's AS714 and AS6185 originate primarily from `17.0.0.0/8` (historically assigned to Apple). The MISP list is minimal.
7. **Semantic check**:
   - `17.0.0.0/8` is Apple's historical Class A allocation (16.7M addresses). MISP includes this plus 3 smaller prefixes.
   - Apple's official enterprise network guidance (`https://support.apple.com/en-us/101555`) lists `17.0.0.0/8` for Apple services, but notes that many Apple services use CDN/CNAME chains with changing IPs.
   - The SOW research classified Apple as **soft/contextual**: `17.0.0.0/8` is official but extremely broad; Apple update traffic uses CDN dependencies not captured in this range.
8. **Tier recommendation**: **soft** with contextual caveats — `17.0.0.0/8` is the authoritative Apple range, but it is a /8 with 16.7M addresses. Apple devices depend on this for software updates. The SOW decision (Decision 6C) deferred Apple's category/tag migration to the typed `critical:` schema.
9. **Source quality grade**: **B** — Apple `17.0.0.0/8` is an officially registered RIR block, but no machine-readable Apple IP feed exists; MISP compiled it from RIR data. The CDN-dependency gap means this is incomplete for "all Apple traffic" purposes.
10. **License / redistribution**: CC0 1.0 (MISP). Apple RIR data is public.
11. **`critical_role` recommendation**: `software_update` (primary rationale) — blocking `17.0.0.0/8` breaks Apple software updates and device activation on any Apple ecosystem. 
12. **Notes / drift / problems**:
   - Small list (4 entries) — minimal maintenance burden.
   - The 4-entry list represents registered prefixes, not Apple's dynamic CDN/service delivery range which is much larger and not statically enumerable.

---

### misp_cloudflare.yaml

1. **Internal name**: `misp_cloudflare`
2. **Current category**: `provider_infrastructure`
3. **Current `use:` roles**: none
4. **Current `dont_redistribute`**: not set
5. **Source URL**: `https://raw.githubusercontent.com/MISP/misp-warninglists/main/lists/cloudflare/list.json`
6. **Upstream sanity check**:
   - URL returns HTTP 200.
   - MISP description cites `https://www.cloudflare.com/ips/`. Version: **20260428** — current.
   - MISP count: **22 entries** (15 IPv4 + 7 IPv6).
   - Primary `https://www.cloudflare.com/ips-v4` returns 15 IPv4 ranges — **exact match** to MISP IPv4 entries.
   - MISP = primary, no drift.
7. **Semantic check**:
   - Cloudflare publishes `ips-v4` and `ips-v6` as the complete list of IP ranges used for Cloudflare edge infrastructure (CDN, proxy, DDoS protection). These are authoritative.
   - Blocking these ranges disrupts any customer behind Cloudflare — very high collateral blast radius.
   - Important distinction: Cloudflare's CDN edge ranges (`104.16.0.0/13`, etc.) are different from Cloudflare's public DNS resolver anycast addresses (`1.1.1.1` in `AS13335`). Both are within Cloudflare ranges, but the DNS resolver addresses need a separate **hard** reference feed for the specific resolver IPs.
8. **Tier recommendation**: **soft** — Cloudflare CDN edge ranges have high collateral risk. The specific DNS resolver IPs (`1.1.1.1`, `1.0.0.1`) need a separate hard-tier entry.
9. **Source quality grade**: **A** — primary source is official, machine-readable, current.
10. **License / redistribution**: CC0 1.0 (MISP). Cloudflare ips pages are public with no redistribution restriction.
11. **`critical_role` recommendation**: `cdn_edge`
12. **Notes / drift / problems**:
   - MISP exactly matches the primary source. Either source is acceptable.
   - Recommended: replace MISP source URL with the primary `https://www.cloudflare.com/ips-v4` (and a separate IPv6 feed or the JSON API at `api.cloudflare.com/client/v4/ips`) for independence from MISP.
   - A separate `hard` feed is needed for the specific Cloudflare public DNS resolver IPs (`1.1.1.1`, `1.0.0.1`, `2606:4700:4700::1111`, `2606:4700:4700::1001`) — these are within the broader CDN ranges but deserve distinct hard classification.

---

### misp_fastly.yaml

1. **Internal name**: `misp_fastly`
2. **Current category**: `provider_infrastructure`
3. **Current `use:` roles**: none
4. **Current `dont_redistribute`**: not set
5. **Source URL**: `https://raw.githubusercontent.com/MISP/misp-warninglists/main/lists/fastly/list.json`
6. **Upstream sanity check**:
   - URL returns HTTP 200.
   - MISP description cites `https://api.fastly.com/public-ip-list`. Version: **20240422** (April 2024). GitHub last commit: **2024-04-22T07:20:14Z** — over 1 year stale.
   - MISP count: **19 entries**.
   - Primary `api.fastly.com/public-ip-list`: **21 entries** (19 IPv4 + 2 IPv6).
   - **Drift confirmed**:
     - MISP has `146.75.0.0/16` (supernet) — primary now has `146.75.0.0/17` (smaller, split).
     - Primary has `140.248.64.0/18` and `140.248.128.0/17` — **missing from MISP**.
     - 18 entries in common; 1 outdated supernet in MISP; 2 new ranges in primary not in MISP.
7. **Semantic check**:
   - Fastly publishes `api.fastly.com/public-ip-list` as its authoritative CDN edge IP list. This is official and machine-readable.
   - Blocking these ranges disrupts any site behind Fastly's CDN (GitHub Pages, npm registry, fastly customers).
8. **Tier recommendation**: **soft** — Fastly CDN edge with high blast radius.
9. **Source quality grade**: **A** for the primary source; the MISP copy is stale (effectively **B** due to staleness).
10. **License / redistribution**: CC0 1.0 (MISP). Fastly's public IP list has no stated redistribution restriction.
11. **`critical_role` recommendation**: `cdn_edge`
12. **Notes / drift / problems**:
   - **Drift is confirmed and significant** — the supernet split means the MISP version incorrectly covers a range Fastly no longer uses in full.
   - Recommended action: replace MISP source URL with `https://api.fastly.com/public-ip-list` directly. The primary is machine-readable JSON and returns the same format MISP re-publishes.
   - This is one of the SOW's explicit primary-over-MISP preferences.

---

### misp_github.yaml

1. **Internal name**: `misp_github`
2. **Current category**: `provider_infrastructure`
3. **Current `use:` roles**: none
4. **Current `dont_redistribute`**: not set
5. **Source URL**: `https://raw.githubusercontent.com/MISP/misp-warninglists/main/lists/github/list.json`
6. **Upstream sanity check**:
   - URL returns HTTP 200.
   - MISP description cites `https://api.github.com/meta`. Version: **20260428** — current.
   - MISP count: **4,447 entries**.
   - Primary `api.github.com/meta`: keys include `hooks`, `web`, `api`, `git`, `github_enterprise_importer`, `packages`, `pages`, `importer`, `actions`, `actions_macos`, `codespaces`, `copilot`.
   - **`actions` alone has 6,237 entries** — GitHub Actions hosted runners span large Azure address blocks.
   - Total primary: ~6,589 ranges across all keys.
   - MISP's 4,447 is a subset. GitHub warns the meta list is "not exhaustive."
7. **Semantic check**:
   - The semantics differ materially by key: `hooks` (6 ranges, webhook source IPs) and `web`/`api` (22 each, core GitHub.com service IPs) are high-criticality; `actions` (6,237 Azure ranges) are customer workload IPs used by CI runners.
   - Blocking all 4,447 MISP entries would block GitHub.com services AND GitHub Actions runner IPs — these have very different criticality levels.
   - The SOW correctly classifies GitHub as **soft** (developer platform), but the "actions" ranges are broad Azure customer hosting, not GitHub core infrastructure.
8. **Tier recommendation**: **soft** for `hooks`, `web`, `api`, `git`, `packages`, `pages` keys. The `actions` ranges are contextual/broad Azure space — different tier.
9. **Source quality grade**: **A** for primary `api.github.com/meta`. The MISP re-publication is current (20260428) but flattens all categories.
10. **License / redistribution**: CC0 1.0 (MISP). GitHub meta API data is public.
11. **`critical_role` recommendation**: `developer_platform`
12. **Notes / drift / problems**:
   - **Semantic problem**: all 4,447 MISP entries treated equivalently is misleading. GitHub core service ranges (~100 entries from `hooks/web/api/git/packages/pages`) should have soft status; `actions` ranges are customer compute that happens to run GitHub CI.
   - Recommended action: use primary `api.github.com/meta` and scope to `hooks`, `web`, `api`, `git`, `packages`, `pages` (exclude `actions` from soft critical classification, or add `actions` as contextual with explicit labeling).
   - GitHub explicitly says its IP list is not exhaustive — document this limitation.

---

### misp_googlebot.yaml

1. **Internal name**: `misp_googlebot`
2. **Current category**: `provider_infrastructure`
3. **Current `use:` roles**: none
4. **Current `dont_redistribute`**: not set
5. **Source URL**: `https://raw.githubusercontent.com/MISP/misp-warninglists/main/lists/googlebot/list.json`
6. **Upstream sanity check**:
   - URL returns HTTP 200.
   - MISP cites `https://developers.google.com/search/apis/ipranges/googlebot.json`. Version: **20260428** — current.
   - Count: **76 entries** (mix of IPv4 and IPv6 — 76 total).
   - Primary `developers.google.com/search/apis/ipranges/googlebot.json` is an official Google JSON feed of Googlebot crawler ranges.
7. **Semantic check**:
   - Googlebot is a web crawler, not critical service infrastructure. Blocking Googlebot IPs prevents Google from indexing your site — a business impact, not an operational failure.
   - This is explicitly listed in the SOW's exclusion list: "crawlers such as Googlebot/OpenAI GPTBot: useful references, not critical infrastructure."
   - The YAML `info` field already says "Search crawler infrastructure reference, not a threat indicator" — correctly labeled, but the category assignment is wrong for critical-infra purposes.
8. **Tier recommendation**: **reject** from critical-infrastructure classification. This is a crawler, not critical network infrastructure.
9. **Source quality grade**: **A** — official Google JSON feed, current.
10. **License / redistribution**: CC0 1.0 (MISP).
11. **`critical_role` recommendation**: none — reject.
12. **Notes / drift / problems**:
   - SOW research already excluded this. No action needed beyond not tagging with `use: [critical_infrastructure]`.
   - Remains useful as an `organizations` category reference (benign scanner/crawler). Memory note from previous session confirms `organizations` as the right category for benign scanners.
   - The current category `provider_infrastructure` is semantically off — Googlebot is a service, not infrastructure that other services depend on.

---

### misp_google_gcp.yaml

1. **Internal name**: `misp_google_gcp`
2. **Current category**: `provider_infrastructure`
3. **Current `use:` roles**: none
4. **Current `dont_redistribute`**: not set
5. **Source URL**: `https://raw.githubusercontent.com/MISP/misp-warninglists/main/lists/google-gcp/list.json`
6. **Upstream sanity check**:
   - URL returns HTTP 200.
   - MISP description cites `https://www.gstatic.com/ipranges/cloud.json`. Version: **20260428** — current.
   - MISP count: **424 IPv4 entries**.
   - Primary `gstatic.com/ipranges/cloud.json`: **862 IPv4 prefixes** as of syncToken `1777406985447` (2026-04-28).
   - **Major drift**: MISP publishes 49% of the primary's entries.
7. **Semantic check**:
   - GCP `cloud.json` covers Google Cloud Platform customer-facing compute ranges. Like AWS, this is shared multi-tenant hosting — contextual, not hard.
   - The remaining 438 prefixes in the primary but not in MISP represent recent GCP expansions that the MISP list has not captured, despite its 20260428 version.
   - UNVERIFIED: The gap may be because MISP's GCP list filters to unique CIDRs while the primary may repeat prefixes for different regions/services.
8. **Tier recommendation**: **contextual** — GCP customer hosting ranges, not Google's service infrastructure (for which use `goog.json - cloud.json` derivation as the SOW proposed).
9. **Source quality grade**: **A** for primary `gstatic.com/ipranges/cloud.json`; MISP is effectively incomplete (**B**).
10. **License / redistribution**: CC0 1.0 (MISP). GCP IP ranges are publicly accessible.
11. **`critical_role` recommendation**: `cloud_customer_hosting`
12. **Notes / drift / problems**:
   - **Significant drift** (49% coverage). Replace with primary source.
   - The SOW also proposes deriving Google service edge from `goog.json - cloud.json` (i.e., Google's non-cloud service ranges) — this is a separate feed idea and not currently present in the catalog.

---

### misp_google_gmail_sending_ips.yaml

1. **Internal name**: `misp_google_gmail_sending_ips`
2. **Current category**: `provider_infrastructure`
3. **Current `use:` roles**: none
4. **Current `dont_redistribute`**: not set
5. **Source URL**: `https://raw.githubusercontent.com/MISP/misp-warninglists/main/lists/google-gmail-sending-ips/list.json`
6. **Upstream sanity check**:
   - URL returns HTTP 200.
   - MISP description cites `https://support.google.com/a/answer/27642?hl=en`. Version: **20260428** — current.
   - MISP count: **8 entries** (2 IPv4 CIDRs + 6 IPv6 prefixes).
   - Live DNS SPF check of `_spf.google.com` returns exactly: `ip4:74.125.0.0/16 ip4:209.85.128.0/17 ip6:2001:4860:4864::/56 ip6:2404:6800:4864::/56 ip6:2607:f8b0:4864::/56 ip6:2800:3f0:4864::/56 ip6:2a00:1450:4864::/56 ip6:2c0f:fb50:4864::/56`.
   - **Exact match** with MISP. MISP mirrors the SPF record accurately.
7. **Semantic check**:
   - These are IP ranges from which Gmail sends outbound email. Blocking them would cause Gmail outbound mail to be dropped/rejected at destination MX servers. This is significant for email deliverability.
   - Semantics: **email sending infrastructure** — the source IPs of legitimate outbound Gmail. Used for SPF allowlisting by mail server operators.
   - Relevant for: operators who need to allow Gmail delivery; feed users checking if a feed incorrectly flags Gmail sending IPs.
8. **Tier recommendation**: **soft** — email delivery infrastructure. Blocking Gmail sending IPs causes email delivery failures for anyone receiving email from Gmail.
9. **Source quality grade**: **A** — primary is the Google SPF TXT record, DNS-derived but official and stable. MISP matches exactly.
10. **License / redistribution**: CC0 1.0 (MISP). Google SPF data is public.
11. **`critical_role` recommendation**: `email_delivery`
12. **Notes / drift / problems**:
   - Current and accurate — good fit for critical-infra reference.
   - Alternative primary: directly resolve `_spf.google.com` TXT via DNS on each update rather than depending on MISP. This gives true freshness.
   - The MISP list is DNS-derived, so `source_type: dns_derived` is appropriate.

---

### misp_microsoft_azure.yaml

1. **Internal name**: `misp_microsoft_azure`
2. **Current category**: `provider_infrastructure`
3. **Current `use:` roles**: none
4. **Current `dont_redistribute`**: not set
5. **Source URL**: `https://raw.githubusercontent.com/MISP/misp-warninglists/main/lists/microsoft-azure/list.json`
6. **Upstream sanity check**:
   - URL returns HTTP 200.
   - MISP version: **20241203** (December 2024). GitHub last commit: **2024-12-03T08:44:20Z** — approximately 5 months stale.
   - MISP count: **2,683 entries**.
   - Primary source: Microsoft publishes Azure service tags as a JSON download from `microsoft.com/en-us/download/details.aspx?id=56519`, but this page returns HTTP 403. The actual JSON download links are generated dynamically and change weekly.
   - Microsoft also provides the service tag API: `management.azure.com/subscriptions/.../providers/Microsoft.Network/locations/.../serviceTags?api-version=...` (requires Azure auth).
   - **No unauthenticated stable public JSON URL** for complete Azure IP ranges — only the weekly download via an HTML page.
   - MISP's 2,683 vs the ~4,000+ prefixes in recent Azure service tag downloads (varies by version).
7. **Semantic check**:
   - "Microsoft Azure Datacenter IP Ranges" — this is the broad multi-tenant cloud hosting range. Same contextual semantics as AWS/GCP.
   - MISP description says "Datacenter IP Ranges" but does not cite an explicit source URL in the list.json file.
8. **Tier recommendation**: **contextual** — broad Azure customer hosting.
9. **Source quality grade**: **B** — official source exists (Azure service tags) but requires HTML scraping or portal download; no stable unauthenticated JSON endpoint. MISP is a convenient secondary.
10. **License / redistribution**: CC0 1.0 (MISP). Azure IP data is public.
11. **`critical_role` recommendation**: `cloud_customer_hosting`
12. **Notes / drift / problems**:
   - MISP is 5 months stale and the primary is not easily machine-readable without Azure credentials.
   - Recommend keeping MISP as secondary for now, labeled `source_type: secondary` and `source_quality: B`.
   - Azure service-specific tags (AzureCloud, AzureFrontDoor, etc.) are available via Azure CLI/SDK — can scope CDN edge vs customer hosting if a processor is implemented.
   - The separate `misp_microsoft_office365_ip.yaml` feed is more specific for Office 365 service ranges.

---

### misp_microsoft_azure_china.yaml

1. **Internal name**: `misp_microsoft_azure_china`
2. **Current category**: `provider_infrastructure`
3. **Current `use:` roles**: none
4. **Current `dont_redistribute`**: not set
5. **Source URL**: `https://raw.githubusercontent.com/MISP/misp-warninglists/main/lists/microsoft-azure-china/list.json`
6. **Upstream sanity check**:
   - URL returns HTTP 200. MISP version: **20241203** — 5 months stale. Count: 211 entries.
   - Primary: `microsoft.com/en-us/download/details.aspx?id=57062` (Azure China) — returns HTTP 403 per prior checks.
7. **Semantic check**:
   - Azure China (operated by 21Vianet) is a separate sovereign cloud for China-region services. From a threat-intelligence perspective, China-region cloud ranges may be contextually more likely to appear in threat feeds (and legitimately so).
   - Contextual — definitely not hard or soft critical infrastructure for a global blocklist aggregator.
8. **Tier recommendation**: **contextual** — sovereign cloud ranges; high customer-abuse potential coexists with legitimate use.
9. **Source quality grade**: **B** (same issues as `misp_microsoft_azure`).
10. **License / redistribution**: CC0 1.0 (MISP).
11. **`critical_role` recommendation**: `cloud_customer_hosting`
12. **Notes / drift / problems**: 5 months stale. Same freshness caveat as `misp_microsoft_azure`.

---

### misp_microsoft_azure_germany.yaml

1. **Internal name**: `misp_microsoft_azure_germany`
2. **Current category**: `provider_infrastructure`
3. **Current `use:` roles**: none
4. **Current `dont_redistribute`**: not set
5. **Source URL**: `https://raw.githubusercontent.com/MISP/misp-warninglists/main/lists/microsoft-azure-germany/list.json`
6. **Upstream sanity check**:
   - URL returns HTTP 200. MISP version: **20241203** — 5 months stale. Count: **36 entries** — very small.
   - Primary: `microsoft.com/en-us/download/details.aspx?id=57063` (Azure Germany).
7. **Semantic check**:
   - Azure Germany was the sovereign German cloud (operated by T-Systems as data trustee). Microsoft announced the closure of the Germany sovereign cloud regions in 2018; customers were migrated to the new Germany West Central/North region by late 2021.
   - **IMPORTANT**: Azure Germany (the original T-Systems trustee model) is effectively deprecated/closed. Remaining IPs may be in wind-down state.
   - The 36 entries likely represent legacy ranges that are now part of standard Azure European regions.
8. **Tier recommendation**: **contextual** — but possibly **obsolete**. Verify if the Azure Germany sovereign cloud is fully decommissioned before including.
9. **Source quality grade**: **C** — the underlying cloud is deprecated; official source may no longer be updated.
10. **License / redistribution**: CC0 1.0 (MISP).
11. **`critical_role` recommendation**: none, or `cloud_customer_hosting` with obsolescence note.
12. **Notes / drift / problems**: Azure Germany (T-Systems trustee model) was retired by 2021. This feed may be tracking dead ranges. **Flag for removal or manual verification**.

---

### misp_microsoft_azure_us_gov.yaml

1. **Internal name**: `misp_microsoft_azure_us_gov`
2. **Current category**: `provider_infrastructure`
3. **Current `use:` roles**: none
4. **Current `dont_redistribute`**: not set
5. **Source URL**: `https://raw.githubusercontent.com/MISP/misp-warninglists/main/lists/microsoft-azure-us-gov/list.json`
6. **Upstream sanity check**:
   - URL returns HTTP 200. MISP version: **20241203** — 5 months stale. Count: 201 entries.
   - Primary: `microsoft.com/en-us/download/details.aspx?id=57063` (Azure Gov) — requires HTML scraping.
7. **Semantic check**:
   - Azure US Government is a FedRAMP-authorized cloud for US government agencies. These are dedicated US government cloud ranges. From a global blocklist perspective, these are cloud customer hosting ranges — not hard critical infrastructure for the public internet.
   - May have compliance sensitivity: blocking US Gov cloud could affect government services.
8. **Tier recommendation**: **contextual** — government-dedicated cloud with the same multi-tenant hosting semantics as commercial Azure.
9. **Source quality grade**: **B** (same download-portal limitation).
10. **License / redistribution**: CC0 1.0 (MISP).
11. **`critical_role` recommendation**: `cloud_customer_hosting`
12. **Notes / drift / problems**: 5 months stale. Primary is not machine-readable without scraping.

---

### misp_microsoft_office365_ip.yaml

1. **Internal name**: `misp_microsoft_office365_ip`
2. **Current category**: `provider_infrastructure`
3. **Current `use:` roles**: none
4. **Current `dont_redistribute`**: not set
5. **Source URL**: `https://raw.githubusercontent.com/MISP/misp-warninglists/main/lists/microsoft-office365-ip/list.json`
6. **Upstream sanity check**:
   - URL returns HTTP 200.
   - MISP version: **20260428** — current. Count: **82 entries**.
   - Primary `endpoints.office.com/endpoints/worldwide?clientrequestid=<GUID>` is a JSON API returning Microsoft 365 endpoint data including IPs. The version API (`endpoints.office.com/version`) confirmed instance versions (e.g., `2026033100`).
   - The primary endpoint API returns structured data with service areas; MISP flattens to a CIDR list.
7. **Semantic check**:
   - Office 365 service IPs are the ranges that Microsoft 365/Exchange/Teams/SharePoint traffic originates from or is delivered to. Blocking these disrupts Microsoft's productivity suite for thousands of enterprise customers.
   - This is **soft** critical infrastructure — email delivery, collaboration services, authentication services for major enterprises.
   - Distinct from generic Azure hosting — these are specific Microsoft-operated service IPs, not customer compute.
8. **Tier recommendation**: **soft** — Microsoft 365 service infrastructure. Blocking disrupts enterprise email, calendar, Teams, and SharePoint for large numbers of users.
9. **Source quality grade**: **A** — primary `endpoints.office.com` is official, machine-readable, and actively maintained with version tracking.
10. **License / redistribution**: CC0 1.0 (MISP).
11. **`critical_role` recommendation**: `cloud_service_tag` (Microsoft 365 specific services)
12. **Notes / drift / problems**:
   - MISP is current. Primary endpoint API is preferred for completeness and service-area scoping.
   - Recommended: upgrade to primary `endpoints.office.com` endpoint for the reference feed, with MISP as secondary validation.

---

### misp_microsoft_office365_cn.yaml

1. **Internal name**: `misp_microsoft_office365_cn`
2. **Current category**: `provider_infrastructure`
3. **Current `use:` roles**: none
4. **Current `dont_redistribute`**: not set
5. **Source URL**: `https://raw.githubusercontent.com/MISP/misp-warninglists/main/lists/microsoft-office365-cn/list.json`
6. **Upstream sanity check**:
   - URL returns HTTP 200. MISP version: **20260428** — current. Count: 67 entries.
   - China-region Office 365 endpoint data.
7. **Semantic check**:
   - Office 365 China is operated by 21Vianet (same as Azure China). These are China-specific Microsoft 365 service IPs.
   - Same contextual reasoning as Azure China: sovereign cloud, higher likelihood of appearing in threat feeds for unrelated reasons.
8. **Tier recommendation**: **contextual** — China sovereign cloud service ranges.
9. **Source quality grade**: **A** — current.
10. **License / redistribution**: CC0 1.0.
11. **`critical_role` recommendation**: `cloud_service_tag`
12. **Notes / drift / problems**: Current. Note that China-region ranges are more likely to appear in threat intelligence feeds than global ranges.

---

### misp_openai_gptbot.yaml

1. **Internal name**: `misp_openai_gptbot`
2. **Current category**: `provider_infrastructure`
3. **Current `use:` roles**: none
4. **Current `dont_redistribute`**: not set
5. **Source URL**: `https://raw.githubusercontent.com/MISP/misp-warninglists/main/lists/openai-gptbot/list.json`
6. **Upstream sanity check**:
   - URL returns HTTP 200.
   - MISP version: **20240426** (April 2024). GitHub check: this is over 1 year stale.
   - Count: **2 entries only**. Primary cites `https://openai.com/gptbot-ranges.txt`.
   - The two entries are likely `20.171.207.128/27` and `40.83.2.64/26` (or similar Azure ranges used by GPTBot).
7. **Semantic check**:
   - GPTBot is OpenAI's web crawler. Blocking it prevents OpenAI from indexing your content for training data. This is a content/business decision, not critical infrastructure.
   - The SOW explicitly excludes this: "crawlers such as Googlebot/OpenAI GPTBot: useful references, not critical infrastructure."
8. **Tier recommendation**: **reject** from critical-infrastructure classification. Web crawler.
9. **Source quality grade**: **A** — official OpenAI feed, but MISP copy is stale (1+ year).
10. **License / redistribution**: CC0 1.0 (MISP).
11. **`critical_role` recommendation**: none — reject.
12. **Notes / drift / problems**:
   - 1+ year stale. OpenAI has expanded its crawler IPs significantly since 2024.
   - Same recategorization recommendation as Googlebot: `organizations` category (benign service), not `provider_infrastructure` critical-infra candidate.

---

### misp_ovh_cluster.yaml

1. **Internal name**: `misp_ovh_cluster`
2. **Current category**: `provider_infrastructure`
3. **Current `use:` roles**: none
4. **Current `dont_redistribute`**: not set
5. **Source URL**: `https://raw.githubusercontent.com/MISP/misp-warninglists/main/lists/ovh-cluster/list.json`
6. **Upstream sanity check**:
   - URL returns HTTP 200.
   - MISP description cites an OVH French documentation page for cluster IPs. Version: **20240422** — over 1 year stale.
   - Count: **431 individual IP addresses** (no CIDR ranges — all /32 equivalents).
   - Source docs are a French-language OVH hosting cluster page listing cluster-specific IPs for shared hosting infrastructure.
7. **Semantic check**:
   - OVH cluster IPs are the server-side IPs of OVH's shared web hosting clusters. These are individual hosting server IPs, not OVH's backbone infrastructure, CDN edges, or public internet service IPs.
   - "Cluster" here means OVH's internal shared-hosting clusters — essentially the IP space of OVH's Apache/Nginx servers hosting customer websites.
   - This is **not** global critical infrastructure. OVH is a large European hoster, but its hosting cluster IPs are shared customer hosting space. Blocking them would disrupt OVH-hosted websites.
   - The SOW explicitly excludes this: "OVH cluster: hosting cluster, not global critical infrastructure."
8. **Tier recommendation**: **reject** from critical-infrastructure classification.
9. **Source quality grade**: **C** — official OVH docs (French static page), not a machine-readable API. Over 1 year stale.
10. **License / redistribution**: CC0 1.0 (MISP).
11. **`critical_role` recommendation**: none — reject.
12. **Notes / drift / problems**:
   - The individual IP format (431 /32s) is unusual for critical infra reference — most sources publish CIDR ranges.
   - Even if kept, this should not be `use: [critical_infrastructure]`.
   - OVH's broader cloud infrastructure (not just shared hosting cluster IPs) might be considered contextual in the future if an official OVH cloud IP range feed exists, but this specific feed does not represent that.

---

### misp_public_dns.yaml

1. **Internal name**: `misp_public_dns`
2. **Current category**: `provider_infrastructure`
3. **Current `use:` roles**: none
4. **Current `dont_redistribute`**: not set
5. **Source URL**: `https://raw.githubusercontent.com/MISP/misp-warninglists/main/lists/public-dns-v4/list.json`
6. **Upstream sanity check**:
   - URL returns HTTP 200.
   - MISP description: "Event contains one or more public IPv4 DNS resolvers as attribute with an IDS flag set." Version: **20240615** (June 2024). GitHub last commit: **2024-06-15T01:44:47Z** — over 10 months stale.
   - Count: **62,745 individual IPs** (not CIDR ranges). These are individual IPv4 addresses of public DNS resolvers.
7. **Semantic check**:
   - This is a broad enumeration of any public IPv4 DNS resolver — including small/obscure resolvers, open resolvers run by random parties, ISP resolvers, etc.
   - The SOW research explicitly rejected this for hard-tier use: "MISP `public-dns-v4`: too broad; local clone has 62,745 entries."
   - 62,745 entries is far too broad to use as a "hard whitelist" — it includes many resolvers that have no claim to critical status.
   - The hard-tier public DNS set should contain only well-known, widely-used, authoritative public resolvers: Cloudflare 1.1.1.1, Google 8.8.8.8, Quad9 9.9.9.9, Cisco OpenDNS 208.67.222.222, etc.
8. **Tier recommendation**: **reject** for critical-infrastructure hard-tier use. It is too broad to function as a "never block" reference.
9. **Source quality grade**: **D** — 62,745 undifferentiated open resolvers from an opaque source. No authoritative chain to verify individual resolver criticality.
10. **License / redistribution**: CC0 1.0 (MISP).
11. **`critical_role` recommendation**: none for critical use — reject.
12. **Notes / drift / problems**:
   - The SOW recommends building a **curated small set** of core public DNS resolver IPs from official provider sources (Cloudflare, Google, Quad9, Cisco) as the hard-tier reference.
   - The MISP public-dns-v4 list may still be useful as a **secondary reference** to detect that a feed hits known-resolver space broadly — but it should not be used as the primary hard-tier reference.
   - 10+ months stale.

---

### misp_smtp_receiving_ips.yaml

1. **Internal name**: `misp_smtp_receiving_ips`
2. **Current category**: `provider_infrastructure`
3. **Current `use:` roles**: none
4. **Current `dont_redistribute`**: not set
5. **Source URL**: `https://raw.githubusercontent.com/MISP/misp-warninglists/main/lists/smtp-receiving-ips/list.json`
6. **Upstream sanity check**:
   - URL returns HTTP 200. MISP version: **20260428** — current. Count: **270 individual IPs** (no CIDR).
   - No explicit source citation in the MISP list description. The MISP description simply says "List of IP addresses for known SMTP servers."
   - Data: individual IPv4 addresses (e.g., `103.129.252.43`, `103.168.172.216`). No documentation of which providers are included or the derivation methodology.
7. **Semantic check**:
   - "SMTP receiving IPs" means the destination MX IPs that major email providers use to receive inbound email. Blocking these IPs would prevent delivery of legitimate email to those providers' users.
   - The semantic is: if a blocklist contains these IPs, blocking the list would drop email destined for major mailboxes.
   - However, the data quality is uncertain — no documented methodology, no provider attribution, individual IPs (not CIDR), 270 entries.
8. **Tier recommendation**: **soft** — email receiving infrastructure, but quality concerns due to lack of source documentation.
9. **Source quality grade**: **B** — current update timestamp but no documented methodology or provider attribution. Individual IPs rather than provider-published CIDR ranges.
10. **License / redistribution**: CC0 1.0 (MISP).
11. **`critical_role` recommendation**: `email_delivery`
12. **Notes / drift / problems**:
   - No provider documentation — it is unclear which mail providers' MX IPs are included.
   - Better alternatives exist for specific providers: Google Workspace inbound MX IPs, Microsoft 365 inbound MX, etc. are documented by those providers.
   - Keep as secondary, labeled `source_type: secondary`, `source_quality: B`.

---

### misp_smtp_sending_ips.yaml

1. **Internal name**: `misp_smtp_sending_ips`
2. **Current category**: `provider_infrastructure`
3. **Current `use:` roles**: none
4. **Current `dont_redistribute`**: not set
5. **Source URL**: `https://raw.githubusercontent.com/MISP/misp-warninglists/main/lists/smtp-sending-ips/list.json`
6. **Upstream sanity check**:
   - URL returns HTTP 200. MISP version: **20260428** — current. Count: **974 entries** (mix of CIDR and individual IPs).
   - Sample entries include AWS/GCP/Azure-looking ranges (`100.24.127.128/25`, `100.25.99.0/25` look like AWS) — suggests bulk email providers hosted on cloud.
   - No documented methodology or provider breakdown in the MISP list.
7. **Semantic check**:
   - "SMTP sending IPs" means source IPs from which major mail providers send outbound email. Blocking these would cause false positives on email from legitimate bulk/transactional senders.
   - These are often used in SPF allowlisting. However, the undocumented source is a concern — bulk email sending IPs can include legitimate ESPs and spam operations.
   - Contrast with `misp_google_gmail_sending_ips` which has a verified, documented source (Google SPF).
8. **Tier recommendation**: **soft** — email sending infrastructure, but quality is lower than Gmail-specific list.
9. **Source quality grade**: **B** — current but undocumented methodology.
10. **License / redistribution**: CC0 1.0 (MISP).
11. **`critical_role` recommendation**: `email_delivery`
12. **Notes / drift / problems**:
   - The lack of provider documentation is a concern. Individual well-known providers' SPF records (Google, Microsoft, Amazon SES, SendGrid, Mailchimp) are better primary sources.
   - Keep as secondary. Note that the 974 entries likely include major cloud provider SMTP ranges without clear attribution.

---

### misp_stackpath.yaml

1. **Internal name**: `misp_stackpath`
2. **Current category**: `provider_infrastructure`
3. **Current `use:` roles**: none
4. **Current `dont_redistribute`**: not set
5. **Source URL**: `https://raw.githubusercontent.com/MISP/misp-warninglists/main/lists/stackpath/list.json`
6. **Upstream sanity check**:
   - URL returns HTTP 200.
   - MISP description: "List of known Stackpath (Highwinds) CDN IP ranges." Version: **20240422** — over 1 year stale.
   - Primary source URL cited: `https://support.stackpath.com/hc/en-us/articles/360001091666-Whitelist-CDN-WAF-IP-Blocks` — **connection refused / unreachable**.
   - Count: **173 entries** (IPv4 and IPv6 ranges in live upstream; 247 in local clone — discrepancy possibly due to local clone version vs live GitHub).
7. **Semantic check**:
   - **CRITICAL FINDING**: StackPath sold its CDN line of business to Akamai in **August 2023** (confirmed via Wikipedia). The transaction did not include StackPath personnel or technology infrastructure — Akamai acquired enterprise customer contracts and assets.
   - Post-acquisition, StackPath's CDN infrastructure was absorbed into Akamai. The specific StackPath IP ranges are either now operated by Akamai or have been decommissioned.
   - The StackPath support portal is unreachable, confirming the company's CDN operations have ceased.
   - These IP ranges are either now Akamai edge IPs (tracked in `misp_akamai.yaml`) or no longer active CDN ranges.
8. **Tier recommendation**: **reject** — the operating entity (StackPath CDN) ceased to exist in August 2023. This feed represents abandoned/transferred IP ranges.
9. **Source quality grade**: **D** — source URL is unreachable, data is 1+ year stale, operating entity dissolved.
10. **License / redistribution**: CC0 1.0 (MISP).
11. **`critical_role` recommendation**: none — reject and remove.
12. **Notes / drift / problems**:
   - **This feed should be removed**. The provider no longer exists as an independent entity.
   - Any ranges that transferred to Akamai are now covered by `misp_akamai.yaml`.
   - Any ranges that were decommissioned are no longer relevant.
   - Keeping this feed creates false critical-infrastructure classification for orphaned or reassigned IP space.

---

### misp_telegram.yaml

1. **Internal name**: `misp_telegram`
2. **Current category**: `provider_infrastructure`
3. **Current `use:` roles**: none
4. **Current `dont_redistribute`**: not set
5. **Source URL**: `https://raw.githubusercontent.com/MISP/misp-warninglists/main/lists/telegram-ips/list.json`
6. **Upstream sanity check**:
   - URL returns HTTP 200.
   - MISP description cites `https://core.telegram.org/resources/cidr.txt`. Version: **20260119** (January 2026). GitHub last commit: **2026-03-19T11:19:26Z** — recent.
   - MISP count: **14 entries** (IPv4 and IPv6, matching primary).
   - Live primary `core.telegram.org/resources/cidr.txt` returns exactly the same 14 entries as MISP.
   - **Perfect match** — MISP = primary.
7. **Semantic check**:
   - Telegram publishes `cidr.txt` as the official IP ranges for Telegram messenger servers. These are the server IPs of Telegram's backend infrastructure.
   - Blocking these IPs prevents Telegram clients from communicating with Telegram servers — it's an application-level block of a messaging service.
   - Telegram is used by hundreds of millions of users globally and is popular in specific regions. It is also known to host threat actor communications, criminal content, and bypass censorship.
   - **Telegram is not critical internet infrastructure** in the DNS/CDN/update sense. It is a messaging platform. Blocking Telegram has been done intentionally by several governments and enterprises.
8. **Tier recommendation**: **contextual** — Telegram is a major communication platform but not critical network infrastructure in the sense of DNS, CDN edge, or cloud control plane. Its presence in a blocklist may be intentional (enterprise policy) or a false positive (individual Telegram servers abused for C2).
9. **Source quality grade**: **A** — official Telegram source, current and exact match.
10. **License / redistribution**: CC0 1.0 (MISP). Telegram's CIDR list is public.
11. **`critical_role` recommendation**: None that fits the current schema cleanly. If categorized, it would be something like `messaging_platform` — but this is not a standard critical-infra role. Recommend keeping as provider reference without critical-infrastructure tagging.
12. **Notes / drift / problems**:
   - The SOW does not explicitly address Telegram. Telegram's CIDR list should stay as a provider infrastructure reference, but it should **not** receive `use: [critical_infrastructure]` tagging.
   - The distinction: blocking `1.1.1.1` breaks DNS for many users; blocking Telegram is a deliberate policy choice that affects a specific communication service.

---

### misp_tenable_cloud.yaml

1. **Internal name**: `misp_tenable_cloud`
2. **Current category**: `provider_infrastructure`
3. **Current `use:` roles**: none
4. **Current `dont_redistribute`**: not set
5. **Source URL**: `https://raw.githubusercontent.com/MISP/misp-warninglists/main/lists/tenable-cloud-ipv4/list.json`
6. **Upstream sanity check**:
   - URL returns HTTP 200. MISP version: **20260428** — current. Count: 39 entries.
   - MISP description: "Tenable IPv4 Cloud Sensor addresses used for scanning Internet-facing infrastructure."
   - Data: AWS-range-looking CIDR prefixes (e.g., `13.115.104.128/25`, `13.210.1.64/26`) — these are Tenable's cloud scanner instances running on AWS.
7. **Semantic check**:
   - These are IPs from which Tenable's cloud vulnerability scanner (Tenable.io) conducts scans of customers' internet-facing infrastructure.
   - A blocklist containing these IPs would cause Tenable's scanner to be blocked — which would prevent legitimate security scans.
   - From a threat-intel perspective, these IPs can appear in network logs as port scanners. The MISP list's purpose is to help distinguish Tenable scanner traffic from malicious scanning.
   - The SOW explicitly excludes this: "Tenable cloud and scanner feeds: scanner infrastructure, not critical service infrastructure."
8. **Tier recommendation**: **reject** from critical-infrastructure classification. Scanner infrastructure.
9. **Source quality grade**: **A** — current; Tenable publishes official scanner IP lists.
10. **License / redistribution**: CC0 1.0 (MISP).
11. **`critical_role` recommendation**: none — reject.
12. **Notes / drift / problems**:
   - The existing YAML `info` field correctly says "Commercial vulnerability scanning infrastructure — informational, not for blocking."
   - No change needed in the YAML itself beyond confirming this will not receive `use: [critical_infrastructure]` tagging.
   - Could be useful in an `organizations` category (known benign scanner) similar to Googlebot.

---

### misp_umbrella_blockpage.yaml

1. **Internal name**: `misp_umbrella_blockpage`
2. **Current category**: `provider_infrastructure`
3. **Current `use:` roles**: none
4. **Current `dont_redistribute`**: not set
5. **Source URL**: `https://raw.githubusercontent.com/MISP/misp-warninglists/main/lists/umbrella-blockpage-v4/list.json`
6. **Upstream sanity check**:
   - URL returns HTTP 200. MISP version: **20260428** — current. Count: **6 IPs** only.
   - Entries: `146.112.61.104`, `146.112.61.105`, `146.112.61.106`, `146.112.61.107`, `146.112.61.108`, `146.112.61.110`.
   - These are Cisco Umbrella's blockpage server IPs. When Umbrella DNS blocks a domain, clients are redirected to these IPs which serve the "this site is blocked" page.
7. **Semantic check**:
   - **Critical semantic distinction**: these IPs are the **blockpage delivery servers**, NOT the Cisco Umbrella **recursive DNS resolver** addresses (208.67.222.222, 208.67.220.220, etc.).
   - Blocking these 6 IPs would prevent users from seeing Cisco Umbrella's "blocked site" notification page. It would NOT break DNS resolution or the Umbrella service itself.
   - The blast radius of blocking these 6 IPs is: users behind Cisco Umbrella who hit a blocked site would get a connection error instead of a block page. DNS still resolves correctly.
   - This is very different from blocking the Umbrella **resolver** IPs (which would break DNS for Umbrella users).
   - The SOW explicitly excludes this: "Cisco Umbrella blockpage endpoints: blockpage only."
8. **Tier recommendation**: **reject** from critical-infrastructure classification. Blockpage delivery IPs, not resolver infrastructure.
9. **Source quality grade**: **A** — 6 IPs, current, likely directly from Cisco documentation.
10. **License / redistribution**: CC0 1.0 (MISP).
11. **`critical_role` recommendation**: none — reject.
12. **Notes / drift / problems**:
   - The Cisco Umbrella **resolver IPs** (208.67.222.222, 208.67.220.220) ARE hard-tier critical infrastructure (public DNS resolvers). The **blockpage IPs** (146.112.61.x) are not.
   - This is a good example of where semantic precision matters: same provider, two sets of IPs, completely different criticality.
   - The blockpage IPs have legitimate use as an allowlisting reference (to ensure blockpage delivery is not broken), but that is not the critical-infrastructure model.

---

### misp_zscaler.yaml

1. **Internal name**: `misp_zscaler`
2. **Current category**: `provider_infrastructure`
3. **Current `use:` roles**: none
4. **Current `dont_redistribute`**: not set
5. **Source URL**: `https://raw.githubusercontent.com/MISP/misp-warninglists/main/lists/zscaler/list.json`
6. **Upstream sanity check**:
   - URL returns HTTP 200.
   - MISP description cites `https://config.zscaler.com/api/zscaler.net/hubs/cidr/json/recommended`. Version: **20260428** — current.
   - MISP count: **78 entries**. Primary API returns **87 entries**.
   - **Drift**: 9 ranges in primary not present in MISP. The primary API (`cloudName: zscaler.net`, `type: recommended`) is authoritative.
   - Primary API is accessible (HTTP GET returns JSON; `method not allowed` is only on HEAD, not GET).
7. **Semantic check**:
   - Zscaler is a cloud security proxy (CASB/SWG/ZTNA). Corporate users route web traffic through Zscaler's proxy nodes. The IPs are Zscaler's hub/proxy infrastructure.
   - Blocking Zscaler hub IPs would break corporate internet access for enterprises using Zscaler as their secure web gateway.
   - This is a **soft** critical-infrastructure case: Zscaler-dependent enterprises would lose internet access if these IPs are blocked. However, Zscaler is not universal internet infrastructure like DNS or CDN.
8. **Tier recommendation**: **soft/contextual** — Zscaler proxy infrastructure matters for enterprises using it, but it is optional enterprise tooling (not public internet critical infrastructure like DNS). Suggest `contextual` with `identity_saas` or a new `cloud_proxy` role.
9. **Source quality grade**: **A** — official Zscaler API, machine-readable, current.
10. **License / redistribution**: CC0 1.0 (MISP). Zscaler's public IP list has no stated restriction.
11. **`critical_role` recommendation**: None that fits cleanly. Zscaler is a cloud security proxy, not a DNS/CDN/cloud-provider role. If a role must be assigned: `identity_saas` (closest in current schema, since Zscaler handles identity/access proxy). Or introduce a `cloud_proxy` role.
12. **Notes / drift / problems**:
   - **Drift confirmed**: MISP has 78, primary has 87. Recommend replacing MISP source with primary `config.zscaler.com/api/zscaler.net/hubs/cidr/json/recommended`.
   - Note the `zscaler.net` is Zscaler's commercial cloud; there are also `zscalerone.net`, `zscalertwo.net`, `zscalerthree.net`, `zscloud.net` for other customer tiers — this feed covers only `zscaler.net`.
   - The existing YAML `info` says "Cloud-proxy infrastructure, not for blocking" — semantically correct.

---

## MISP source replacement priority

For each MISP feed, whether a primary upstream exists and what action to take:

| Feed | Primary upstream available? | Primary URL | Recommendation |
|---|---|---|---|
| misp_amazon_aws | YES | `https://ip-ranges.amazonaws.com/ip-ranges.json` | Replace with primary |
| misp_cloudflare | YES | `https://www.cloudflare.com/ips-v4` + `ips-v6`, or `https://api.cloudflare.com/client/v4/ips` | Replace with primary |
| misp_fastly | YES | `https://api.fastly.com/public-ip-list` | Replace with primary (MISP is stale and has range drift) |
| misp_github | YES | `https://api.github.com/meta` | Replace with primary; scope to core keys, not `actions` |
| misp_googlebot | YES | `https://developers.google.com/search/apis/ipranges/googlebot.json` | Primary exists but reject for critical infra; recategorize |
| misp_google_gcp | YES | `https://www.gstatic.com/ipranges/cloud.json` | Replace with primary (MISP has 49% coverage) |
| misp_google_gmail_sending_ips | YES (DNS SPF) | `_spf.google.com` TXT record | Primary is DNS-derived; MISP matches; upgrade to DNS-derived primary |
| misp_akamai | NO | none (BGP-derived only) | Keep as MISP secondary; label `generated_bgp` |
| misp_apple | PARTIAL | RIR/ASN data, `https://support.apple.com/en-us/101555` | Keep MISP as best available; label `curated_static` |
| misp_microsoft_azure | PARTIAL | Azure service tags download (no stable API without auth) | Keep MISP as secondary; label `secondary` |
| misp_microsoft_azure_china | PARTIAL | Same | Keep MISP as secondary |
| misp_microsoft_azure_germany | NO (deprecated) | Azure Germany is closed | Flag obsolete; verify and remove |
| misp_microsoft_azure_us_gov | PARTIAL | Download portal only | Keep MISP as secondary |
| misp_microsoft_office365_ip | YES | `https://endpoints.office.com/endpoints/worldwide?clientrequestid=<GUID>` | Replace/supplement with primary |
| misp_microsoft_office365_cn | YES | `https://endpoints.office.com/endpoints/china?...` | Replace/supplement with primary |
| misp_openai_gptbot | YES | `https://openai.com/gptbot-ranges.txt` | Primary exists; reject for critical infra |
| misp_ovh_cluster | PARTIAL | OVH French docs (static HTML) | Reject for critical infra |
| misp_public_dns | PARTIAL | Per-provider docs | Reject for critical-infra hard use; build curated set instead |
| misp_smtp_receiving_ips | NO | No per-provider documented source | Keep as secondary; note undocumented |
| misp_smtp_sending_ips | PARTIAL | Per-provider SPF records | Keep as secondary; upgrade specific providers to SPF-derived |
| misp_stackpath | NO | Provider dissolved | Remove |
| misp_telegram | YES | `https://core.telegram.org/resources/cidr.txt` | Replace with primary; keep as reference but not critical infra |
| misp_tenable_cloud | YES | Tenable official source | Primary exists; reject for critical infra |
| misp_umbrella_blockpage | YES | Cisco documentation | Primary exists; reject for critical infra (wrong semantic) |
| misp_zscaler | YES | `https://config.zscaler.com/api/zscaler.net/hubs/cidr/json/recommended` | Replace with primary (MISP missing 9 ranges) |

---

## Drift / problems found

1. **AWS major drift** (`misp_amazon_aws`): MISP publishes 3,629 entries vs the primary's 10,161. For a contextual-tier reference, this gap is significant.

2. **GCP major drift** (`misp_google_gcp`): MISP publishes 424 entries vs the primary's 862 IPv4 prefixes (49% coverage). The primary is current (syncToken 2026-04-28).

3. **Fastly range drift** (`misp_fastly`): MISP has a supernet `146.75.0.0/16` where primary now uses `146.75.0.0/17` (split), and two new ranges (`140.248.64.0/18`, `140.248.128.0/17`) are in the primary but absent from MISP. MISP is over 1 year stale.

4. **Zscaler undercount** (`misp_zscaler`): MISP has 78 entries, primary has 87 — 9 missing.

5. **StackPath dissolved** (`misp_stackpath`): StackPath CDN was sold to Akamai in August 2023. The provider no longer independently operates CDN infrastructure. The MISP list is orphaned, its source URL is unreachable, and the data is 1+ year stale. **Remove this feed.**

6. **Azure Germany deprecated** (`misp_microsoft_azure_germany`): Azure Germany (T-Systems trustee model) was shut down by 2021. This feed may be tracking 36 defunct/reassigned ranges. **Flag for removal verification.**

7. **Akamai BGP-derived and stale** (`misp_akamai`): No official Akamai IP range publication exists. The MISP list is from BGP search and is 1 year old. For a major CDN, this is a significant coverage concern.

8. **Public DNS list too broad** (`misp_public_dns`): 62,745 individual resolver IPs is far too broad for hard-tier critical-infrastructure use. The list is also 10+ months stale. A curated set of 20-30 well-known resolver IPs is the correct hard-tier approach.

9. **GitHub actions-included** (`misp_github`): The 4,447-entry MISP list flattens GitHub Actions runner ranges (broad Azure space) together with GitHub's core service ranges (~100 entries). The `actions` category alone has 6,237 entries in the live API. Using this flat list for critical-infra reference overstates GitHub's "service" footprint.

10. **Microsoft Azure MISP undocumented source**: The `microsoft-azure/list.json` MISP file does not cite a source URL in its metadata. The MISP count (2,683) vs expected Azure IP count (4,000+ in recent downloads) suggests it is incomplete and 5 months stale.

11. **SMTP list methodology gap**: Both `misp_smtp_receiving_ips` and `misp_smtp_sending_ips` lack provider attribution and derivation documentation. It is unclear which providers' IPs are included.

12. **Umbrella blockpage vs resolver confusion** (`misp_umbrella_blockpage`): The 6 IPs are blockpage delivery servers, not the Umbrella DNS resolver IPs (208.67.222.222, 208.67.220.220). The latter are the genuinely critical infrastructure. The current YAML info text is unclear on this distinction.

13. **Crawler category drift** (`misp_googlebot`, `misp_openai_gptbot`): Both are correctly described as "crawler" in their `info` text, but they are categorized under `provider_infrastructure`. They belong in an `organizations` category (benign scanners).

14. **datacenters.yaml primary data 6+ years stale**: The `client9/ipcat` `datacenters.csv` has had no commits since 2019-02-20. The data is severely outdated and missing large swaths of modern cloud infrastructure.

---

## Open questions

1. **GitHub actions tier**: Should GitHub Actions runner IPs (`actions` key in `/meta`) be contextual or excluded from the soft `developer_platform` reference entirely? The current approach of including all 4,447 MISP entries without distinction conflates very different criticality levels.

2. **Zscaler multi-tier**: Zscaler has multiple cloud tiers (`zscaler.net`, `zscalerone.net`, `zscalertwo.net`, `zscalerthree.net`, `zscloud.net`). Should the critical-infra reference include all tiers or only `zscaler.net` (the main commercial cloud)?

3. **StackPath IP fate**: Before removing `misp_stackpath`, confirm whether the ranges that transferred to Akamai are now captured in `misp_akamai`. If yes, removal is clean. If not, removing StackPath creates a gap in Akamai coverage.

4. **Azure Germany removal**: Verify whether `misp_microsoft_azure_germany` tracks live or decommissioned ranges. If the IPs have been reassigned to standard Azure regions, they are already covered by `misp_microsoft_azure`.

5. **SMTP list governance**: Who maintains `misp_smtp_sending_ips` and `misp_smtp_receiving_ips`? The lack of source attribution is a quality concern. Should these be replaced with per-provider SPF-derived feeds (Gmail, Microsoft, Amazon SES)?

6. **Telegram tier**: Is Telegram `contextual` (policy-dependent, block if your threat model requires it) or should it remain as a provider reference without critical-infrastructure classification? Telegram's dual-use nature (legitimate messenger + C2 infrastructure) makes it semantically different from CDNs or cloud providers.

7. **OVH global cloud vs cluster**: The current `misp_ovh_cluster` tracks OVH shared hosting cluster IPs, not OVH Cloud (global cloud computing). Should a separate OVH Cloud IP feed be investigated as a contextual-tier cloud_customer_hosting reference?

---

## Sources consulted

- Local MISP warninglists clone at `/tmp/misp-warninglists-critical` (commit `9397afe`)
- GitHub API: last commit dates for each `lists/<name>/list.json` file in MISP/misp-warninglists
- Live primary sources:
  - `https://ip-ranges.amazonaws.com/ip-ranges.json` (AWS; verified 2026-04-29)
  - `https://www.cloudflare.com/ips-v4` (Cloudflare; verified 2026-04-29)
  - `https://api.fastly.com/public-ip-list` (Fastly; verified 2026-04-29)
  - `https://api.github.com/meta` (GitHub; verified 2026-04-29)
  - `https://www.gstatic.com/ipranges/cloud.json` (GCP; verified 2026-04-29, syncToken 2026-04-28)
  - `https://config.zscaler.com/api/zscaler.net/hubs/cidr/json/recommended` (Zscaler; verified 2026-04-29)
  - `https://core.telegram.org/resources/cidr.txt` (Telegram; verified 2026-04-29)
  - `https://endpoints.office.com/version?clientrequestid=...` (Office 365; verified 2026-04-29)
  - DNS SPF `_spf.google.com` (Gmail sending IPs; verified 2026-04-29)
  - `https://raw.githubusercontent.com/client9/ipcat/master/datacenters.csv` (datacenters; verified 2026-04-29)
- Wikipedia: StackPath acquisition by Akamai (August 2023)
- SOW-0017 analysis and decisions in `.agents/sow/current/SOW-0017-20260426-review-critical-asns.md`
- Knowledge document `.agents/knowledge/critical-infrastructure.md`
- Config spec `.agents/sow/specs/config.md` (critical: schema, valid roles and source types)
