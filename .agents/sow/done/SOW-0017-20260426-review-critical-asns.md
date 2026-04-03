# SOW-0017 | 2026-04-26 | review-critical-asns

## Status

completed
reopened 2026-04-29 for regression: shipped critical-infrastructure reference
catalog appears imbalanced toward broad contextual cloud/provider ranges while
service-specific soft references from the approved research are absent
completed 2026-04-30 after implementation, local validation, install, and
running-service smoke checks

## Requirements

### Purpose

Protect users of public IP reputation feeds from high-blast-radius false
positives by making critical-infrastructure overlap explicit, evidence-based,
and cheap to consume from precomputed public artifacts. The purpose is not to
declare cloud/provider space "safe"; it is to show when a feed touches
infrastructure where blind blocking can create disproportionate operational
damage.

### User request quoted verbatim

> 17: I need your help with this. Now we have added some ASNs to critical infra,
> which affects warnings per feed on the UI, however I am not they are really
> critical. The key problem with critical infra, we that we need to protect key
> public DNS servers like 1.1.1.1 and 8.8.8.8, cloudlare IPs and generally infra
> that is highly unlikely to be "bad" and will cause significant operational
> issues if blacklisted. I need your experience/knowledge, thinking, research
> capabilities to come up with a list of ASNs that are really critical. The
> result of this analysis should also be documented in the methodology pages of
> the site (with reasoning "why this is critical?"), user documentation, etc.

Latest request 2026-04-28:

> I need you to come up with a detailed plan on what needs to be done, and I
> also want a detailed methodology page and documentation. This is critical work
> for many users out there. Also our feed page should surface all these in the
> right way. Please do this analysis.

Latest request 2026-04-29:

> I think you need to try harder to complete the list of critical hard, soft and
> contextual before we start this. Use subagents/tasks for searching

Given the current critical infrastructure ASN list may be too wide, when this SOW is complete, then every ASN in the list must have a justified criticality classification or be removed/reclassified.

Given ASN criticality affects public insights and interpretation, when changes are proposed, then evidence, category, false-positive risk, and user impact must be documented.

Given this is curated data, when the review is complete, then future maintenance rules must be documented.

Given critical infrastructure classification affects warnings per feed in the
UI, when this SOW is complete, then the classification must protect genuinely
critical public infrastructure from being treated as ordinary suspicious
targets while avoiding over-broad warnings.

Given examples include key public DNS and high-impact internet infrastructure,
when the reviewed ASN list is proposed, then each ASN must explain why it is
critical, why blacklist hits there are highly likely to create operational
problems, and what evidence supports that classification.

Given users need to understand the warning, when this SOW is complete, then the
methodology pages and user/operator documentation must explain the critical ASN
taxonomy and reasoning.

Given this can affect many users, when this SOW is implemented, then feed-page
warnings must distinguish hard, soft, and contextual infrastructure overlap and
must link to methodology explaining why each class matters.

Given public serving must stay cheap, when this SOW is implemented, then public
feed pages and API routes must read precomputed local artifacts only; they must
not compute critical-infrastructure overlaps at request time.

## Analysis

Initial sources to consult:

- `configs/firehol/infrastructure-asns.yaml`
- Public methodology pages for infrastructure ASNs.
- Insight rules that use infrastructure ASN classification.
- External authoritative sources for ASN ownership and service criticality.

Current known context:

- The old tracker says critical infrastructure ASNs affect analytical truth.
- 2026-04-28 user decision: the main purpose is to protect key public DNS
  servers such as `1.1.1.1` and `8.8.8.8`, Cloudflare IPs, and generally
  infrastructure that is highly unlikely to be "bad" and would cause significant
  operational issues if blacklisted.
- 2026-04-28 user request: use assistant research/experience to produce a list
  of ASNs that are really critical, not just broad or convenient.
- 2026-04-28 user requirement: document the result in methodology pages and
  user documentation, with reasoning for "why this is critical".
- Evidence must be verified during implementation. The examples above are
  the user's stated target cases; ASN ownership, routing, and service criticality
  must be checked against authoritative or primary sources before proposing the
  final list.

Local evidence gathered 2026-04-28:

- Current list is ASN-wide and lives at
  `configs/firehol/infrastructure-asns.yaml`.
- Current entries before the 2026-04-29 cleanup were broad:
  - CDN/shared edge: Cloudflare AS13335, Akamai AS16625/AS20940, Fastly AS54113.
  - Hyperscaler/customer cloud: Google AS15169, GCP AS396982, Microsoft
    AS8075/AS8068, AWS AS16509/AS14618, Meta AS32934.
  - Consumer/developer platforms: Apple AS714/AS6185, X/Twitter AS13414,
    GitHub AS36459, GitLab-labelled AS35995.
- The UI treats any configured ASN hit as a prominent "Critical
  infrastructure" warning. Evidence:
  `ui/src/components/feed-detail/section-asn.tsx:308`.
- The engine does a direct ASN lookup against the configured list and sums all
  matching IPs. Evidence: `pkg/engine/asn.go:309`.
- Current methodology says the list includes "large CDNs, hyperscale clouds,
  major developer platforms, and corporate networks". Evidence:
  `pkg/web/static/methodology/infrastructure-asns.md:7`.
- Live local API impact from the current list:
  - AWS AS16509: 208 feeds / 386,976,088 attributed IPs.
  - Microsoft AS8075: 195 feeds / 186,610,782 attributed IPs.
  - GCP AS396982: 205 feeds / 53,975,983 attributed IPs.
  - Akamai AS20940: 144 feeds / 31,959,935 attributed IPs.
  - Cloudflare AS13335: 139 feeds / 1,163,934 attributed IPs.
  These broad cloud ASNs dominate warnings and may be too noisy for the user's
  "really critical / highly unlikely bad" purpose.
- Team Cymru ASN whois returned AS35995 as `TWITTER - Twitter Inc.`, while the
  local config labelled AS35995 as GitLab before the 2026-04-29 cleanup. This
  was a concrete data-quality issue.
- Team Cymru origin-ASN lookup for the user's examples:
  - `1.1.1.1` and `1.0.0.1` -> AS13335 Cloudflare.
  - `8.8.8.8` and `8.8.4.4` -> AS15169 Google.
  - `9.9.9.9` and `149.112.112.112` -> AS19281 Quad9.
  - `208.67.222.222` and `208.67.220.220` -> AS36692 Cisco Umbrella/OpenDNS.

External evidence gathered 2026-04-28:

- Cloudflare documents `1.1.1.1` as its public DNS resolver and lists the
  resolver IPs at `https://developers.cloudflare.com/1.1.1.1/ip-addresses/`.
- Google documents Google Public DNS IPv4 addresses `8.8.8.8` and `8.8.4.4` at
  `https://developers.google.com/speed/public-dns/docs/using`.
- Quad9 documents public recursive DNS addresses `9.9.9.9` and
  `149.112.112.112`, and describes a worldwide anycast resolver network at
  `https://quad9.net/service/service-addresses-and-features/`.
- Cisco documents Umbrella anycast resolver addresses
  `208.67.222.222`/`208.67.220.220` and AS36692 at
  `https://umbrella.cisco.com/blog/why-the-cisco-umbrella-global-network-uses-anycast-routing`.
- AWS documents its IP ranges as ranges used by AWS services/customer networks,
  and explicitly notes EC2 address space can back non-EC2 services:
  `https://docs.aws.amazon.com/vpc/latest/userguide/aws-ip-ranges.html`.
  This supports treating AWS as provider infrastructure/collateral-risk data,
  not automatically as "highly unlikely bad" critical infrastructure.
- Microsoft publishes Azure IP/service tags weekly and describes them as Azure
  public cloud/service ranges:
  `https://www.microsoft.com/en-us/download/details.aspx?id=56519`. This
  supports service-tag/prefix handling for Azure better than broad ASN-wide
  criticality.
- GitHub documents service IP ranges via its Meta API and warns the list is not
  exhaustive: `https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/about-githubs-ip-addresses`.

Project knowledge read 2026-04-28:

- Read `.agents/knowledge/critical-infrastructure.md` in full, 934 lines.
- Core definition from the knowledge file: critical internet infrastructure is
  defined by blast radius and dependency count, not by brand size or ASN size.
- The document recommends a tiered exclusion model:
  - hard whitelist: should never appear in blocklists, such as public recursive
    DNS resolvers, root DNS, and core anycast infrastructure
  - soft whitelist: should almost never be blocked and needs documented
    justification, such as CDN edges, CA validation, OS update, and some shared
    service infrastructure
  - contextual whitelist: infrastructure whose criticality depends on the
    consumer's environment, such as broad cloud/customer hosting ranges
- The document is explicit that ASN-level rules are unsafe for multi-tenant
  cloud/hosting ASNs. ASN data is useful evidence and a source for generating
  reference sets, but not sufficient as the final warning unit for AWS/Azure/GCP
  customer-cloud space.
- The document says aggregator-grade protection should cross-check feeds against
  a maintained exclusion set and publish quality/overlap signals; this aligns
  with moving from `infrastructure_asns` as direct warning truth to generated or
  downloaded critical-infrastructure reference feeds plus overlap artifacts.
- Important categories to cover beyond the current list: public recursive DNS,
  DNS root servers, NTP/time, certificate revocation/CA validation, OS/software
  update infrastructure, CDN/shared edge, cloud control-plane/provider ranges,
  email delivery, identity, and selected development/commerce infrastructure.
- The document's diagnostics establish immediate red flags for any feed listing
  public DNS IPs (`8.8.8.8`, `1.1.1.1`, `9.9.9.9`,
  `208.67.222.222`, etc.) and large aggregate prefixes from major
  cloud/CDN ASNs.
- The document's validation trail already resolves several ASN mistakes that
  matter here: Quad9 should use AS19281, Akamai is AS16625/AS20940, AS16615 is
  wrong, Let's Encrypt should not use AS42408, Okta is AS19745, and Huawei Cloud
  is AS55990.

## Implications and decisions

- Over-broad critical ASN classification can mislead users.
- Under-broad classification can miss important infrastructure concentration.
- Evidence standards must be explicit.
- This list should be conservative. Critical ASN status should mean
  "blacklisting this is likely to break important public infrastructure", not
  "large, well-known, or cloud-hosting provider".
- Cloud and CDN providers may contain both critical infrastructure and ordinary
  customer-hosted workloads. The SOW must decide whether the unit of warning is
  ASN-wide, prefix/service-specific, or a smaller curated subset if the data
  model supports it.

### Decision 1: What should "critical infrastructure ASN" mean?

Evidence:

- The current implementation is ASN-wide. A single configured ASN hit triggers
  the feed-detail warning block and public ASN infrastructure role.
- Broad customer-cloud ASNs currently dominate warning volume.
- the user's stated goal is narrower: protect key public DNS servers, Cloudflare
  IPs, and infrastructure highly unlikely to be "bad" where blacklisting causes
  major operational harm.

Options:

A. Conservative ASN-only cleanup.
- Meaning: keep only ASNs whose whole ASN is defensible as high-collateral
  public infrastructure under the existing data model.
- Likely direction: keep/add public DNS and shared edge candidates; remove or
  reclassify broad customer-cloud and ordinary consumer-platform ASNs.
- Pros: small implementation, matches current engine/UI model, easy to explain.
- Cons: still coarse; CDN ASNs can carry customer abuse and are not as clean as
  public recursive DNS.
- Risks: may under-report provider-cloud collateral risk unless separate
  provider-infrastructure feeds are emphasized.

B. Split criticality into explicit roles/severities.
- Meaning: keep ASN-wide data, but distinguish `public_dns`, `shared_edge`,
  `developer_platform`, `major_consumer_service`, and `customer_cloud`; only
  some roles produce the strong critical warning.
- Pros: more truthful UI; preserves useful context without calling everything
  "critical".
- Cons: requires schema/API/UI/docs changes.
- Risks: larger scope and more design surface in this SOW.

C. Move critical warnings from ASN-wide to curated prefix/reference feeds where
possible.
- Meaning: use public-DNS/provider-infrastructure feeds or new curated feeds for
  exact prefixes; ASNs remain context, not the primary warning unit.
- Pros: most accurate for AWS/Azure/GCP and service-specific infra.
- Cons: larger engine/UI change; requires deciding feed semantics and artifact
  generation rules.
- Risks: highest implementation scope; may overlap with provider-infrastructure
  feed review work.

Recommendation for this SOW after reading project knowledge:

- Use a C+B hybrid as the target model:
  - critical infrastructure should be represented as generated/downloaded
    reference feeds with tiers/roles
  - all public feeds should get precomputed overlap artifacts against those
    reference feeds
  - UI wording should distinguish hard, soft, and contextual overlap instead of
    treating all matches as the same "critical infrastructure" warning
- Keep ASN evidence only as one way to generate or validate reference feeds,
  especially for providers without authoritative IP feeds.
- Do not proceed with a simple ASN-only cleanup as the final product model; it
  can be used only as an interim compatibility/migration step if needed.

Additional local analysis 2026-04-28:

- The project already validates `use: [critical_infrastructure]` as an
  ipset-compatible role. Evidence: `pkg/config/config.go:153-157`,
  `pkg/config/config.go:266-272`, `pkg/config/validate.go:73-88`, and
  `.agents/sow/specs/config.md:217-234`.
- No configured source currently declares `use: [critical_infrastructure]`.
  Evidence: `rg` over `configs/firehol` returned no matches.
- The config comment says `critical_infrastructure` drives per-feed comparison
  files, but no engine writer exists for that role. Current fan-out only knows
  ASN, GeoIP, and bogons. Evidence: `pkg/engine/helpers.go:607-655`.
- The bogon implementation is the closest existing pattern:
  - loads all `use: [bogons]` sources from committed local sets:
    `pkg/engine/bogons.go:52-83`
  - writes one precomputed `<feed>_bogons_<provider>.json` per feed/provider:
    `pkg/engine/bogons.go:141-230`
  - recomputes all target feeds when a provider changes via
    `targetFeedsForFanOut`.
- The current ASN implementation embeds "critical infrastructure" inside ASN
  artifacts by checking `infrastructure_asns`. Evidence:
  `pkg/engine/asn.go:231-294`, `pkg/engine/asn.go:309-327`.
- The current insight text is ASN-specific and overstates the model:
  `pkg/insights/rules_asn.go:48-80`.
- The feed page renders the critical warning inside the ASN section and tells
  users that matched IPs belong to ASNs hosting hyperscalers/CDNs/developer
  platforms. Evidence: `ui/src/components/feed-detail/section-asn.tsx:190-192`
  and `ui/src/components/feed-detail/section-asn.tsx:381-420`.
- Current public methodology is ASN-centric and says the list includes large
  clouds and corporate networks. Evidence:
  `pkg/web/static/methodology/infrastructure-asns.md:1-68` and
  `pkg/web/static/methodology/infrastructure-present.md:1-26`.
- Existing public route pattern already serves ASN and bogon provider artifacts
  from disk, with no live computation. Evidence: `pkg/web/server.go:526-560`.
- Existing specs require this exact model:
  - public browsing must use precomputed artifacts, not recomputation:
    `.agents/sow/specs/operating-principles.md:22-55`
  - new provider/comparison dimensions must define producer, refresh, repair,
    serving path, and WebDir behavior:
    `.agents/sow/specs/operating-principles.md:48-56`
  - feed-scoped artifacts must not be generated on first request:
    `.agents/sow/specs/feeds.md:473-478`
  - provider-triggered changes must regenerate dependent facts:
    `.agents/sow/specs/processing-engine.md:190-201`

External/open-source analysis 2026-04-28:

- MISP warninglists are already used by this repo as secondary provider
  infrastructure feeds. Local clone `/tmp/misp-warninglists-critical` at
  commit `9397afe` shows:
  - `public-dns-v4` has 62,745 entries, so it is not a small "never block"
    canonical public-DNS set; it is a broad public resolver list.
  - `cloudflare` points to Cloudflare's official IP ranges and has 22 entries.
  - `fastly` points to Fastly's public IP list and has 19 entries.
  - `amazon-aws` points to AWS `ip-ranges.json` and has 3,629 entries.
  - `google-gcp` points to Google `cloud.json` and has 424 entries.
  - `microsoft-azure` has 2,683 entries but the cloned version is dated
    2024-12-03, so it should not be treated as the best authoritative Azure
    freshness source without validation.
- MISP is useful as a secondary/community source, especially because its license
  is compatible in the current configs, but criticality tiering should prefer
  primary provider sources where they exist.
- Official source examples checked:
  - Cloudflare documents `1.1.1.1`, `1.0.0.1`, and IPv6 equivalents as public
    DNS resolver addresses.
  - Google documents `8.8.8.8`, `8.8.4.4`, and IPv6 equivalents as Google
    Public DNS.
  - Quad9 documents `9.9.9.9`, `149.112.112.112`, and IPv6 equivalents.
  - Cisco documents Umbrella/OpenDNS anycast resolver addresses
    `208.67.222.222` and `208.67.220.220`.
  - InterNIC publishes current root hints at
    `https://www.internic.net/domain/named.root`.
  - AWS publishes current ranges in `ip-ranges.json`, but explicitly says the
    list is for AWS services/customers and some services use EC2 space; this is
    contextual, not "never bad".
  - Microsoft documents Azure service tags as service-prefix groups and warns
    that all Azure IPs include other Azure customers; this is contextual.
  - GitHub documents Meta API ranges and warns they are not exhaustive.
  - Apple documents enterprise firewall guidance and says IP-only firewalls can
    allow `17.0.0.0/8` plus IPv6 ranges, but many Apple hosts use changing CNAME
    chains and CDN dependencies.

Working conclusion:

- Current ASN-wide warning is not defensible as the final product. It conflates:
  - small dedicated anycast infrastructure where overlap should be treated as a
    feed-quality red flag
  - shared CDN/provider edges where overlap is high collateral-risk but may need
    review
  - broad cloud/customer ranges where overlap is expected and context-dependent
- The product should implement critical-infrastructure overlap as a first-class
  reference-feed comparison dimension, not as an ASN decoration.
- The old ASN list should be retained only as migration evidence/context until
  every current entry is either represented by a reference feed/tier or removed.

Expanded research 2026-04-29:

- user requested a harder pass before implementation. Six read-only research
  tasks were run across hard, soft, contextual, and local-catalog evidence.
- Source quality convention for this SOW:
  - A: official machine-readable current feed/API.
  - B: official machine-readable but partial, geofeed, DNS-derived, or requiring
    derivation.
  - C: official docs/static page, not a clean dynamic feed.
  - D: no official public source found, third-party only, stale, or unsuitable.
- Product rule confirmed by all research tasks: "critical" must be source-feed
  based, not ASN based. The existence of a large or important ASN is not enough.
- Hard overlap must be exact IP/prefix based and backed by official exact
  sources. Broad Cloudflare/AWS/Azure/GCP/CDN ranges are not hard just because
  the provider is important.
- Broad cloud/customer-hosting ranges are contextual. They can include important
  provider services and abusive customer workloads at the same time.
- Soft provider-operated service ranges must be scoped by role/source and must
  not be described as "never bad".
- The model must preserve metadata such as provider, service, tag, region,
  product, direction, source type, and source freshness. A single flattened
  "critical infrastructure" set would hide the context operators need.

Hard candidate list, researched 2026-04-29:

- `critical_dns_root`:
  - tier: hard
  - role: `dns_root`
  - source quality: A/C official IANA and InterNIC root hints
  - source: `https://www.iana.org/domains/root/servers`,
    `https://www.iana.org/domains/root/files`,
    `https://www.internic.net/domain/named.root`
  - include: yes
  - rationale: the DNS root server addresses are dedicated authoritative root
    anycast addresses; blacklisting them is a feed-quality emergency.
  - implementation note: generate from the root hints file; do not use root
    operator ASNs as the warning unit.
- `critical_dns_as112`:
  - tier: hard
  - role: `dns_sink_infrastructure`
  - source quality: C official RFC/project source
  - source: RFC 7534/RFC 7535 and `https://www.as112.net/`
  - include: yes
  - prefixes: `192.175.48.0/24`, `2620:4f:8000::/48`,
    `192.31.196.0/24`, `2001:4:112::/48`
  - rationale: AS112 sinks leaked special-use reverse-DNS traffic; use RFC
    service prefixes, not volunteer node inventories.
- Public recursive DNS, core/extended:
  - tier: hard
  - role: `public_dns_core` or `public_dns_extended`
  - include now when official exact addresses are published:
    - Cloudflare 1.1.1.1 and Families:
      `https://developers.cloudflare.com/1.1.1.1/ip-addresses/`
    - Google Public DNS:
      `https://developers.google.com/speed/public-dns/docs/using`
    - Google DNS64:
      `https://developers.google.com/speed/public-dns/docs/dns64`
    - Quad9 services: `https://docs.quad9.net/services/`
    - Cisco Umbrella/OpenDNS:
      `https://umbrella.cisco.com/products/recursive-dns-services`
    - Vercara UltraDNS Public:
      `https://vercara.digicert.com/ultra-dns-public`
    - AdGuard Public DNS: `https://adguard-dns.io/en/public-dns.html`
    - Control D IP ranges:
      `https://docs.controld.com/docs/control-d-ip-ranges`
    - Mullvad encrypted DNS:
      `https://mullvad.net/en/help/dns-over-https-and-dns-over-tls`
    - DNS.SB: `https://dns.sb/servers/`
    - DNS4EU: `https://joindns4.eu/for-public`
    - CIRA Canadian Shield:
      `https://www.cira.ca/en/canadian-shield/configure/summary-cira-canadian-shield-dns-resolver-addresses/`
    - AliDNS: `https://www.alidns.com/`
    - DNSPod public DNS:
      `https://docs.dnspod.com/public-dns/public-dns-introduction/`
  - include only after conflict/service-health checks:
    - CleanBrowsing, because official pages conflict on at least one Adult
      secondary address.
    - Comodo Secure DNS, because the official source looks legacy and service
      health/currentness needs verification.
  - reject for hard:
    - MISP `public-dns-v4`: too broad; local clone has 62,745 entries.
    - NextDNS: no clean official exact stable public IP/range source found.
    - DNS0.eu: official site says the service is discontinued.
    - OpenNIC: volunteer/dynamic service directory, unsuitable as stable hard
      reference feed.
    - DNS.WATCH and Yandex: no reliable current official exact source verified.
- Public NTP/time:
  - tier: hard where exact official IPs/prefixes are published
  - role: `public_time`
  - include now:
    - Cloudflare Time Services:
      `https://developers.cloudflare.com/time-services/ntp/usage/`
    - Google Public NTP: `https://developers.google.com/time`
    - NIST Internet Time Service:
      `https://tf.nist.gov/tf-cgi/servers.cgi/en-en/`
    - Netnod Swedish Distributed Time Service:
      `https://www.netnod.se/swedish-distributed-time-service`
  - include only with DNS-refresh policy or as soft/manual review:
    - TimeNL: official docs publish hostnames; exact static IP publication was
      not verified.
    - Meta public NTP: official blog publishes hostnames, not an exact stable IP
      feed.
  - reject for hard:
    - NTP Pool: official docs say the returned IP set changes every few minutes
      and cannot be allowlisted as a full IP list.
    - USNO: exact public source not verified in this pass.

Soft candidate list, researched 2026-04-29:

- CDN/shared edge:
  - include now:
    - `critical_soft_cloudflare_edge`; role `cdn_edge`; source quality A;
      source `https://www.cloudflare.com/ips-v4`,
      `https://www.cloudflare.com/ips-v6`,
      `https://api.cloudflare.com/client/v4/ips`
    - `critical_soft_fastly_edge`; role `cdn_edge`; source quality A; source
      `https://api.fastly.com/public-ip-list`
    - `critical_soft_aws_cloudfront`; role `cdn_edge`; source quality A; source
      AWS `ip-ranges.json` filtered to CloudFront service values
    - `critical_soft_azure_frontdoor`; role `cdn_edge`; source quality A; source
      Azure service tags, filtered to Front Door tags
    - `critical_soft_google_service_edge`; role `google_service_edge`; source
      quality B; derive `goog.json - cloud.json`
  - include later/secondary only:
    - Akamai edge: no unauthenticated official complete public bulk feed found;
      MISP/BGP-derived data can be marked `secondary` or `generated_bgp`.
    - OVHcloud CDN Infrastructure: official docs exist but are static/manual
      and state limitations.
  - reject for now:
    - Vercel and Netlify edge: no official stable public edge IP feed found.
- Developer platform and supply chain:
  - include now:
    - GitHub Meta API and GHCR/packages ranges:
      `https://api.github.com/meta`
    - GitLab.com limited published ranges from official GitLab docs; caveat:
      the former AS35995 GitLab label was wrong and must not be reused.
    - Atlassian Cloud / Bitbucket:
      `https://ip-ranges.atlassian.com/`
    - Azure DevOps from Azure service tags.
    - Terraform Cloud:
      `https://app.terraform.io/api/meta/ip-ranges`
  - include later:
    - Microsoft Container Registry, after verifying the exact Azure service tag
      and docs.
    - GitLab registry/CDN, because docs require GCS/GCDN context and are not a
      complete image-layer feed by themselves.
    - Maven Central/Sonatype, if an official range/feed is verified.
  - reject for now:
    - Docker Hub, Quay, npm, PyPI, RubyGems: no official stable public IP/range
      feed verified; most are hostname/CDN based.
- Payment/commerce:
  - include now:
    - Stripe API/webhooks: `https://docs.stripe.com/ips`,
      `https://stripe.com/files/ips/ips_api.json`,
      `https://stripe.com/files/ips/ips_webhooks.json`
    - Braintree/PayPal Braintree:
      `https://developer.paypal.com/braintree/docs/reference/general/braintree-ip-addresses`,
      `https://assets.braintreegateway.com/json/ips.json`
    - Mollie webhooks: `https://ip-ranges.mollie.com/ips.txt`
  - include with caution or later:
    - PayPal endpoint ranges: official static docs need exact source/freshness
      check before shipping.
    - Adyen: official model recommends DNS/domain allowlisting and dynamic
      resolution of `out.adyen.com`; unsuitable as a static feed unless the
      product supports DNS-refresh feeds.
    - Klarna and Worldpay: official static docs need exact current extraction
      and licensing/freshness checks.
  - reject:
    - Shopify webhooks: no fixed static webhook IP range; HMAC validation is the
      intended model.
- Certificate validation/revocation:
  - include now:
    - DigiCert Certificate Status IP Addresses:
      `https://knowledge.digicert.com/alerts/digicert-certificate-status-ip-address`
  - reject or keep as later:
    - GlobalSign: no stable official IP feed found.
    - Sectigo: official guidance says not to use destination IPs because they
      change.
    - Let's Encrypt validation: validation IPs are intentionally not published
      as stable ranges.
    - Google Trust Services and Cloudflare CA: no exact public CA status IP feed
      verified.
- Software update:
  - include only as contextual/soft with clear wording:
    - Apple enterprise network ranges from
      `https://support.apple.com/en-us/101555`; source is official but broad
      (`17.0.0.0/8` plus IPv6 ranges) and Apple documents CDN/CNAME dynamics.
  - reject as static IP feeds:
    - Windows Update, Ubuntu mirrors, Debian mirrors, Fedora mirrors, Docker
      package repositories: official models are URL/DNS/mirror based, not a
      stable public IP feed.
- Identity/SaaS/observability/control-plane:
  - include now where source quality is A or B:
    - Okta: `https://s3.amazonaws.com/okta-ip-ranges/ip_ranges.json`
    - Auth0: `https://cdn.auth0.com/ip-ranges.json`
    - Microsoft 365/Exchange/Teams/SharePoint endpoint API:
      `https://endpoints.office.com/endpoints/worldwide?clientrequestid=<GUID>`
    - Google Workspace/Gmail outbound, SPF-derived from `_spf.google.com`
    - Atlassian Cloud: `https://ip-ranges.atlassian.com/`
    - Salesforce Hyperforce:
      `https://ip-ranges.salesforce.com/ip-ranges.json`
    - Zoom TXT ranges:
      `https://assets.zoom.us/docs/ipranges/Zoom.txt`
    - Datadog: `https://ip-ranges.datadoghq.com/`
    - New Relic public Synthetics JSON from official docs
    - Grafana Cloud per-feature allowlist feeds from official docs/API
  - include with caution where source quality is C:
    - PagerDuty, Splunk On-Call, Sentry, Proofpoint Essentials, Mimecast.
  - reject for now:
    - Slack, ServiceNow, Splunk Cloud, Honeycomb, Elastic Cloud: sources found
      were customer allowlists, per-instance support data, or missing public
      provider-operated source ranges.

Contextual candidate list, researched 2026-04-29:

- Include now as contextual only:
  - AWS global/Gov/China ranges: `https://ip-ranges.amazonaws.com/ip-ranges.json`
  - Azure Public/Gov/China service tags:
    `https://www.microsoft.com/download/details.aspx?id=56519`,
    `https://www.microsoft.com/download/details.aspx?id=57063`,
    `https://www.microsoft.com/download/details.aspx?id=57062`
  - GCP cloud ranges: `https://www.gstatic.com/ipranges/cloud.json`
  - Oracle Cloud public ranges:
    `https://docs.oracle.com/en-us/iaas/tools/public_ip_ranges.json`
  - DigitalOcean geofeed: `https://www.digitalocean.com/geo/google.csv`
  - Linode/Akamai Cloud geofeed: `https://geoip.linode.com/`
  - Vultr/Constant geofeed: `https://geofeed.constant.com/`
  - Scaleway network information, official static docs
  - IBM Cloud Classic, official static docs/tooling, with caution because no
    stable public JSON URL was found.
- Reject for now as contextual source feeds:
  - Alibaba Cloud, Tencent Cloud, Huawei Cloud, broad OVHcloud hosting, Hetzner,
    Equinix Metal: no stable official global public IP feed verified in this
    pass. ASN-only fallback would recreate the false-positive problem this SOW
    is fixing.

Local-catalog fit, researched 2026-04-29:

- Existing `configs/firehol/sources/provider_infrastructure/` has useful MISP
  sources for Cloudflare, Fastly, Akamai, AWS, GCP, Azure, Microsoft 365, Gmail,
  GitHub, Apple, public DNS, SMTP, Zscaler, Telegram, and others.
- None currently declare `use: [critical_infrastructure]`.
- MISP sources can be reused only with correct `critical_source_type`:
  `secondary` unless they are merely republishing an official machine-readable
  source and freshness is validated.
- Do not classify these as critical-infrastructure references:
  - `datacenters`: too broad; provider context only.
  - OVH cluster: hosting cluster, not global critical infrastructure.
  - Tenable cloud and scanner feeds: scanner infrastructure, not critical
    service infrastructure.
  - crawlers such as Googlebot/OpenAI GPTBot: useful references, not critical
    infrastructure.
  - Cisco Umbrella blockpage endpoints: blockpage only.
  - VPN/anonymizer and sinkhole feeds: different semantics.
- IPv6 is a material gap: current local provider feeds are mostly `ipv: ipv4`,
  while several hard/soft sources publish IPv6 addresses.

Implementation implications from expanded research:

- The first shipped list should be a baseline, not a claim of global
  completeness. "Complete" for this SOW means the categories and evidence rules
  are complete enough to prevent accidental over-broad critical classification.
- Use strong source-quality labels in API/UI/docs so users can tell official
  machine feeds from static docs and secondary sets.
- Public DNS hard references should probably be split by provider internally,
  but may also have an aggregate `critical_public_dns_core` family for page
  summaries.
- DNS-root and AS112 are separate roles from public recursive DNS.
- Public NTP/time should include leap-smear semantics where known; Google time
  and non-smearing NTP should not be implied equivalent operationally.
- Do not add DNS-derived dynamic services until the downloader/processor has an
  explicit DNS-resolution feed type with TTL/freshness metadata. This affects
  Adyen, TimeNL, Meta NTP, and many package/update ecosystems.
- Keep broad cloud/customer hosting warnings blue/contextual. They are
  collateral-risk signals, not feed-quality emergencies.
- Existing ASN infrastructure warning should be demoted to ASN context once the
  reference-feed artifacts exist.

Research reconciliation 2026-04-29:

- Additional research corpus is present under
  `.agents/sow/research/sow-0017/` with eight files covering hard DNS/time,
  soft CDN/dev/payment/CA/update/SaaS sources, contextual clouds, and local
  catalog verification.
- Material local verification against live upstreams:
  - AWS `ip-ranges.json`: 10,161 IPv4 prefixes, 5,387 IPv6 prefixes, 15,548
    total current prefixes. Current service values include `CODEBUILD`,
    `GLOBALACCELERATOR`, and `CLOUDFRONT`; no `SES` service tag is present.
  - GCP `cloud.json`: 862 IPv4 prefixes, 48 IPv6 prefixes, 910 total current
    prefixes.
  - GitHub Meta API: `actions` alone has 6,237 ranges, while service buckets
    such as `hooks`, `web`, `api`, `git`, `packages`, `pages`, `codespaces`,
    and `copilot` are separately exposed. A flattened GitHub feed hides this
    distinction and overstates GitHub service infrastructure.
  - MISP warninglists currently expose 3,629 AWS entries, 424 GCP entries, and
    4,447 GitHub entries in the local catalog. Compared to the primary AWS/GCP
    upstreams above, MISP is useful secondary evidence, not the right source of
    truth for contextual cloud coverage.
  - Zscaler publishes a reachable provider API for hub CIDRs. Its product role
    is cloud proxy/security service, not CDN edge or cloud customer hosting.
- Existing catalog implications:
  - `misp_stackpath.yaml` is stale for critical-reference purposes. Akamai's
    August 24, 2023 announcement says StackPath decided to cease CDN operations
    and Akamai acquired select enterprise customer contracts. Do not promote
    StackPath as a live critical-infrastructure reference feed. user approved
    deleting it from the catalog because this MISP source had just been added
    and should not remain if stale.
  - `misp_github.yaml` must not be used as the final GitHub critical reference
    feed. Use GitHub Meta API categories directly so Actions runner ranges,
    service ranges, Pages, Packages, Codespaces, and Copilot stay distinct.
  - AWS/GCP critical-reference feeds must prefer primary upstream manifests
    over MISP. MISP drift is now quantified and must be visible in the decision
    record.
  - Zscaler requires a `cloud_proxy` critical role if included; otherwise it
    should be excluded from this SOW's first reference catalog.
- Schema implication accepted for v1:
  - Do not add multi-tier metadata to one source. When a provider legitimately
    has two semantics, split it into two reference feeds, e.g. Apple
    broad/network ranges as contextual and Apple update/service-specific ranges
    as soft when a narrower source is available. This keeps warning semantics
    inspectable and avoids one flattened feed that means two different things.
  - Add `authoritative_plain_text` as a source type for official plain text
    endpoints such as Cloudflare `ips-v4`/`ips-v6`, Zoom text ranges, or
    provider `ips.txt` files.
- Additional hard/soft/contextual candidates from the research corpus:
  - Hard: root DNS, AS112, exact documented recursive DNS service IPs, and
    exact official non-smearing time sources. Google Public NTP should be
    labelled separately because leap-smearing is operationally different from
    strict UTC sources.
  - Soft: Cloudflare, Fastly, AWS CloudFront, AWS Global Accelerator, Azure
    Front Door service tags, Imperva, GitHub Meta service categories,
    Atlassian, Terraform Cloud, Azure DevOps, Microsoft Container Registry,
    CircleCI, AWS CodeBuild, Okta, Auth0, Salesforce Hyperforce, Microsoft 365,
    Zoom, Datadog, New Relic, Grafana Cloud, Stripe, Braintree, and Mollie when
    source licensing/redistribution permits.
  - Contextual: AWS, GCP, OCI, Azure Public/Gov/China, DigitalOcean geofeed,
    Linode/Akamai geofeed, Vultr/Constant geofeed, Equinix Metal geofeed,
    Scaleway static docs, IBM Cloud Classic static docs/tooling, Yandex Cloud,
    and T-Systems OTC only when each source is verified and labelled by source
    quality.
- Rejections to preserve:
  - DNS0.eu, ScrubIT, Strongarm, and other discontinued services must not be
    shipped as current reference feeds.
  - NextDNS, OpenNIC, DNS.WATCH, 114DNS, IIJ IPv6, DNSPod IPv6, Buildkite,
    Snyk, SonarCloud, Codecov, and Bitrise require re-verification before
    inclusion where the research corpus marked source gaps.
  - NTP Pool, Meta NTP, TimeNL, Adyen, package registries, Windows Update, most
    CA OCSP/CRL endpoints, and DNS-derived SaaS/update ecosystems should not be
    shipped as static IP feeds until a DNS-resolution feed type with
    TTL/freshness semantics exists.

### Proposed tier model

Hard critical infrastructure:

- Meaning: should not appear in a general-purpose blocklist without emergency,
  highly specific justification.
- Initial roles:
  - `public_dns_core`
  - `dns_root`
  - selected well-known anycast service endpoints where IPs are dedicated to the
    service.
- Feed-page treatment: red/severe quality warning.
- Initial sources:
  - curated core public DNS set from official docs: Cloudflare, Google, Quad9,
    Cisco Umbrella/OpenDNS
  - root DNS addresses generated from InterNIC `named.root`

Soft critical infrastructure:

- Meaning: usually wrong to block broadly; legitimate malicious activity can
  still occur behind shared infrastructure, so the warning should demand
  specificity and review.
- Initial roles:
  - `cdn_edge`
  - `software_update`
  - `certificate_validation`
  - `developer_platform`
  - `payment_or_commerce`
  - selected shared service infrastructure.
- Feed-page treatment: amber high-collateral-risk warning with exact provider
  names and ranges.
- Initial sources:
  - Cloudflare published IP ranges
  - Fastly public IP list
  - Akamai BGP-derived ranges, explicitly marked non-primary if no official
    public manifest exists
  - GitHub ranges from Meta API/MISP, with GitHub's "not exhaustive" limitation
  - Apple ranges as soft/contextual service infrastructure, not as a hard
    "never bad" rule

Contextual infrastructure:

- Meaning: can contain both critical services and abusive tenants. Overlap is
  useful policy context, not evidence the feed is wrong.
- Initial roles:
  - `cloud_provider`
  - `cloud_service_tag`
  - `email_delivery`
  - `identity`
  - broad SaaS/platform ranges.
- Feed-page treatment: blue/neutral "policy-dependent collateral risk".
- Initial sources:
  - AWS `ip-ranges.json`
  - Google Cloud `cloud.json`
  - Azure service tags / Microsoft download or API data
  - Office 365 / SMTP provider infrastructure where already present

### Proposed public artifact model

- Critical-infrastructure reference providers are ordinary committed sets marked
  with `use: [critical_infrastructure]`.
- Each such provider has typed metadata:
  - `critical.tier`: `hard`, `soft`, or `contextual`
  - `critical.role`: e.g. `public_dns_core`, `dns_root`, `cdn_edge`,
    `cloud_provider`, `developer_platform`, `software_update`
  - `critical.source_type`: e.g. `authoritative_provider_json`,
    `authoritative_provider_api`, `authoritative_plain_text`,
    `authoritative_service_tag_json`, `authoritative_static_docs`,
    `authoritative_root_hints`, `authoritative_rfc`, `curated_static`,
    `secondary`, `generated_bgp`, `dns_derived`, or `analytical_only`
  - `critical.source_quality`: `A`, `B`, `C`, or `D`
  - `critical.rationale`: short public explanation
- Engine writes:
  - provider detail artifacts:
    `<feed>_critical_<provider>.json`
  - aggregate artifact:
    `<feed>_critical_infrastructure.json`
- Aggregate artifact must de-duplicate overlapping providers by IP count, so a
  Cloudflare IP present in both a core DNS set and the broader Cloudflare CDN set
  does not inflate the total. It should still show all matched providers/roles
  for interpretability.
- Public routes should mirror existing bogon routes:
  - `GET /api/v1/sets/{name}/infrastructure`
  - `GET /api/v1/sets/{name}/infrastructure/{provider}`
  - optional provider index at `GET /api/v1/sets/{name}/infrastructure/providers`
    only if it avoids ambiguity with aggregate vs index naming.
- Public handlers must return missing/not-ready when artifacts are absent; they
  must not compute overlap on request.

### Proposed feed-page surface

- Add a dedicated section near the top of the feed page, after insights and
  before ASN/geography:
  - title: `Critical infrastructure overlap`
  - three compact tier summaries: hard, soft, contextual
  - for each tier: matched IP count, percent of feed, number of reference
    providers hit, and strongest severity
  - table of matched reference providers with provider, role, tier, overlap
    count, percent, source type, and methodology link
  - hard hits should be visually prominent and explain that blind enforcement can
    break DNS/root/service dependencies
  - contextual cloud hits should not be red; copy must say "policy-dependent"
    and "requires specificity", not "feed is wrong"
- Keep ASN composition as ASN composition. The ASN table may still show a subtle
  "infrastructure operator" badge, but the decisive warning must come from the
  reference-feed overlap artifact.
- Insights should be updated:
  - replace ASN-specific `infrastructure_present` with tier-aware findings
  - hard overlap fires at any positive count for feeds with at least one IP
  - soft/contextual overlap should include thresholds and should never imply the
    feed is categorically bad without context

### Proposed methodology page

Primary page: `pkg/web/static/methodology/critical-infrastructure-overlap.md`.

Sections:

1. What this measures
   - Every public feed is compared against maintained critical-infrastructure
     reference feeds.
   - The result is an overlap fact, not a block/allow decision.
2. Why this matters
   - Blindly enforcing IP feeds can break shared infrastructure.
   - Public DNS, root DNS, CDN edges, cloud control planes, update servers,
     certificate validation, email, identity, and developer/payment
     infrastructure have high blast radius.
3. What "critical" means here
   - Defined by blast radius and dependency count.
   - Not equivalent to "big company", "well-managed", or "never abused".
4. Tiers
   - Hard: public recursive DNS, root DNS, dedicated anycast/core service
     addresses.
   - Soft: CDN edge, software update, certificate validation, developer/payment
     service infrastructure.
   - Contextual: cloud/customer hosting, SaaS, identity, email delivery.
5. Source hierarchy
   - Primary provider-published IP/range feeds first.
   - Official static docs for stable anycast addresses.
   - Generated BGP/RDAP/ASN-derived sets where no primary source exists.
   - Secondary/community sources such as MISP, marked as secondary.
6. How the comparison is computed
   - Load committed reference sets with `use: [critical_infrastructure]`.
   - For each public feed, compute set overlap during processing/reprocess.
   - Write provider and aggregate JSON artifacts.
   - Serve only those artifacts on public requests.
7. How to read a feed-page warning
   - Hard overlap: treat as a feed-quality emergency.
   - Soft overlap: review specificity and justification.
   - Contextual overlap: decide against local policy; cloud abuse can be real.
8. Known limitations
   - Source staleness.
   - Provider manifests not exhaustive.
   - IPv6 coverage may lag if initial implementation is IPv4-first.
   - BGP/RDAP proves routing ownership, not service role.
   - MISP/public-DNS broad lists are useful references but not all hard critical.
9. Maintenance rules
   - Every reference provider needs source, tier, role, freshness cadence, and
     rationale.
   - Provider-source changes trigger recomputation of all feed overlaps.
   - Any manual curated set change must include cited evidence.
10. Related pages
   - ASN attribution
   - Bogon classification
   - Feed insights

Existing methodology pages to update:

- `infrastructure-asns.md`: either replace with a short historical/ASN-context
  page or redirect conceptually to the new page.
- `infrastructure-present.md`: rewrite as tier-aware critical infrastructure
  overlap.
- `asn-attribution.md`: remove the implication that ASN infrastructure is the
  primary criticality signal.
- `bogon-classification.md`: update related links.

### Proposed user/operator documentation

Primary doc: `docs/critical-infrastructure.md`.

Sections:

1. Purpose and user promise.
2. How to interpret hard/soft/contextual overlap.
3. Recommended operator actions:
   - hard overlap: do not enforce blindly; investigate immediately
   - soft overlap: check exact IP/prefix and threat context
   - contextual cloud overlap: apply local policy, but require specificity
4. API examples:
   - list infrastructure providers
   - get aggregate overlap for a feed
   - get provider-specific overlap for a feed
   - compose a feed and check overlap if SOW-0013 later exposes composition via
     MCP/docs API
5. Adding a new critical-infrastructure reference provider:
   - prefer primary source
   - set tier/role/source type/rationale
   - document licensing/redistribution
   - test generation and serving
6. Maintaining source freshness.
7. Troubleshooting:
   - artifact missing
   - provider source stale
   - mismatch between ASN context and reference-feed overlap
8. Limitations and false expectations:
   - not a guarantee a feed is good/bad
   - not an enforcement whitelist by itself
   - not exhaustive for every organization's SaaS dependencies

### Implementation design decisions

Decision 1: product direction.

Selected by user: C+B hybrid. Move critical warnings to curated/generated
critical-infrastructure reference-feed overlaps, with tiered hard/soft/contextual
UI/API wording. ASN data may be used as evidence or migration input, but it is
not the final warning unit.

Decision 2: schema for tier metadata.

A. Reuse `Source.Attributes` with string keys.
- Pros: smallest config change; no new YAML schema.
- Cons: weak validation; typos silently become product bugs; public API shape is
  less self-documenting.
- Risk: high for this SOW, because tier names and roles drive public warnings.

B. Add typed `critical:` metadata to `Source` and copied merge expansion.
- Pros: validation can reject unknown tiers/roles/source types; docs/API can be
  generated from a stable struct; safer for future contributors.
- Cons: more code/tests.
- Recommendation: B.

Selected by user: B.

Decision 3: first implementation IP family.

A. IPv4-only first, with explicit methodology limitation and tests.
- Pros: aligns with current provider-infrastructure catalog and most current
  feed volume; smaller implementation.
- Cons: incomplete protection for IPv6 feeds and modern infrastructure.
- Risk: users may assume completeness unless docs and UI label it clearly.

B. IPv4 + IPv6 from the start.
- Pros: more complete and future-proof.
- Cons: larger implementation; ASN/geolocation paths are still IPv4-focused in
  places; needs careful mixed-family artifact design.
- Recommendation: A for first release, but keep schema family-aware and create a
  follow-up SOW for IPv6 parity.

Selected by user: A.

Decision 4: public visibility of reference feeds.

A. Show critical-infrastructure reference feeds as ordinary public feed pages in
the provider-infrastructure category.
- Pros: transparent; users can inspect/download where licensing permits.
- Cons: users may confuse reference feeds with threat feeds.
- Risk: needs strong copy and category labeling.

B. Hide them from the normal feed explorer, expose them only through methodology
and provider lists.
- Pros: avoids confusion.
- Cons: less transparent; harder for users to inspect what was compared.
- Recommendation: A for redistributable reference feeds, with clear `reference,
  not a threat feed` copy. Non-redistributable sources stay analytical only.

Selected by user: A.

Decision 5: treatment of the current `infrastructure_asns` list.

A. Remove it once reference-feed overlap exists.
- Pros: eliminates misleading ASN-wide warnings.
- Cons: loses simple ASN operator context.

B. Keep it as ASN-context metadata only; stop using it for critical warnings.
- Pros: preserves useful "this ASN is an infrastructure operator" context while
  avoiding false red warnings.
- Cons: still another curated list to maintain.
- Recommendation: B during migration, with a later cleanup decision after the
  reference-feed model has real data.

Selected by user: A. Remove `infrastructure_asns` once reference-feed overlap
exists. The list may be used during migration only; it must not survive as a
long-term public ASN-context feature.

Decision 6: Apple AS714/AS6185 tagging.

A. Keep current `corporate` category and add only annotations.
- Pros: smallest immediate change.
- Cons: keeps update-critical semantics mixed with general corporate services.

B. Add a temporary category such as `corporate_critical` or `os_update`.
- Pros: makes Apple update criticality explicit immediately.
- Cons: creates churn before typed `critical:` metadata lands.

C. Defer category/tag migration to the typed `critical:` schema.
- Pros: avoids double migration and keeps semantics in the new first-class
  critical metadata model.
- Cons: Apple remains `corporate` during the short migration window.

Selected by user: C.

User decision:

- 2026-04-29 user approved fixing the verified Opus/cross-check findings
  before implementation.
- Approved immediate correction scope:
  - correct knowledge-doc factual errors for AS40443/DigiCert,
    AS60924/Alibaba, AWS SES service tags, metadata-service IPv6, Apple tiering,
    and no-license community cloud sources
  - fix the local AS35995 GitLab/Twitter config bug conservatively by removing
    the incorrect GitLab ASN entry from the current ASN context list
  - do not expand the old ASN-wide warning model with extra Twitter ASNs during
    this patch, because SOW-0017 is moving critical warnings away from ASN-wide
    truth
  - keep GitLab as a future soft dev-platform reference-feed candidate using
    GitLab's official published IP ranges, not AS35995
  - keep cloud metadata service protection separate from public internet
    critical-infrastructure feeds unless/until a dedicated local-control-plane
    role is implemented
- At cleanup time, the larger implementation design decisions were still open;
  the locked choices below supersede that temporary state.
- 2026-04-29 user locked the implementation design choices:
  - Decision 1: C — move critical warnings to curated/generated
    critical-infrastructure reference-feed overlaps, with the B-style tiered
    hard/soft/contextual model for UI/API wording.
  - Decision 2: B — add typed `critical:` metadata to source/merge config
    instead of free-form `Source.Attributes` strings.
  - Decision 3: A — ship IPv4-first, with schema/artifacts family-aware and
    docs/UI explicitly saying IPv6 coverage is incomplete in v1.
  - Decision 4: A — show redistributable reference feeds publicly as
    inspectable reference feeds, with clear "reference, not threat feed"
    wording; non-redistributable/analytical-only sources stay hidden from
    download.
  - Decision 5: A — remove `infrastructure_asns` once reference-feed overlap
    exists, instead of preserving it as a long-term ASN context list.
  - Decision 6: C — defer Apple AS714/AS6185 category/tag migration to the
    typed `critical:` schema, avoiding temporary category churn.
  - Implication of Decision 5: the current ASN list may remain only as a
    temporary migration aid while building the reference-feed model. It must not
    remain as public warning truth after the new artifacts and UI are live.

## Plan

Chunked SOW - reasoning: the change touches product semantics, source catalog,
engine artifact generation, public APIs, UI warnings, methodology, operator
docs, tests, and specs. Implementation must be staged so each layer has a clear
contract and validation path.

1. `finalize-product-decisions` - high risk
   - Record user decisions for tier model, schema shape, first IP family,
     reference-feed visibility, and `infrastructure_asns` migration.
   - Output: completed on 2026-04-29; SOW decisions updated before code
     changes.

2. `schema-and-config-contract` - medium risk
   - Add typed critical metadata or approved attribute schema.
   - Validate tier, role, source type, and rationale.
   - Ensure merge-expanded sources preserve `use: [critical_infrastructure]`
     and critical metadata when approved.
   - Update `.agents/sow/specs/config.md`.

3. `reference-feed-catalog` - high risk
   - Mark approved provider-infrastructure feeds with
     `use: [critical_infrastructure]` and tier metadata.
   - Add curated/generated core public DNS and root DNS reference sources if
     approved.
   - Prefer primary provider sources for Cloudflare/Fastly/AWS/GCP/Azure where
     existing processors can safely extract them; otherwise keep MISP secondary
     with source-type labeling.
   - Corrected/deprecated misleading ASN entries such as AS35995 labelled
     GitLab after verification.
   - Treat `configs/firehol/infrastructure-asns.yaml` as migration input only;
     remove its public warning role and delete the list once reference-feed
     overlap artifacts replace it.

4. `engine-critical-artifacts` - high risk
   - Implement critical provider loader, provider structs, provider JSON schema,
     aggregate JSON schema, and de-duplicated union counts.
   - Add `UseCriticalInfrastructure` to provider fan-out logic so a reference
     provider update regenerates every feed's overlap artifacts.
   - Integrate writer into the heavy phase after feed sets exist and before
     insights run.
   - Add integrity/manifest expectations and repair/reprocess support.
   - Update `.agents/sow/specs/processing-engine.md`,
     `.agents/sow/specs/pipeline.md`, `.agents/sow/specs/feeds.md`, and
     `.agents/sow/specs/operating-principles.md`.

5. `public-api-and-metadata` - medium risk
   - Add critical-infrastructure provider list and feed/provider routes.
   - Ensure routes serve the configured published `WebDir` artifact tree and
     return missing/not-ready without live recomputation.
   - Add metadata links/summary fields only where they have direct user value.
   - Update `.agents/sow/specs/website.md`.

6. `insights-and-feed-page` - high risk
   - Replace ASN-only critical warning with tier-aware critical-overlap insight
     rules.
   - Add a feed-detail `Critical infrastructure overlap` section near the top of
     the page.
   - Keep ASN composition focused on ASN facts; downgrade ASN infra badges to
     contextual operator information.
   - Build visual hierarchy: hard = severe, soft = review, contextual = policy
     dependent.

7. `methodology-and-docs` - medium risk
   - Add `critical-infrastructure-overlap.md` methodology page.
   - Update `infrastructure-present.md`, `infrastructure-asns.md`,
     `asn-attribution.md`, and related links.
   - Add `docs/critical-infrastructure.md` operator/user documentation with API
     examples and maintenance instructions.

8. `tests-and-real-use-validation` - high risk
   - Config validation tests for tier metadata and critical role propagation.
   - Engine tests for provider loader, per-provider artifact, aggregate
     de-duplication, provider-update fan-out, stale artifact cleanup, and
     WebDir serving.
   - Public API tests proving artifact-only behavior.
   - Insight tests for hard/soft/contextual thresholds.
   - UI type/build/lint validation.
   - Real-use validation through local reprocess/install and curl checks against
     affected public routes.

9. `review-and-retrospection` - medium risk
   - Run required independent reviews for high-risk changes.
   - Fix findings and rerun until no material findings remain or gaps are
     explicitly accepted.
   - Update specs and project skills with lessons before closing.

## Execution log

2026-04-28:

- Moved SOW from `pending/` to `current/`.
- Started local inventory:
  - current curated list: `configs/firehol/infrastructure-asns.yaml`
  - methodology docs: `pkg/web/static/methodology/infrastructure-asns.md`,
    `pkg/web/static/methodology/infrastructure-present.md`,
    `pkg/web/static/methodology/asn-attribution.md`
  - engine path: `pkg/engine/asn.go` builds infrastructure summaries from the
    configured ASN list; `pkg/engine/ip_context.go`, entity sidecars, and home
    summaries reuse the same classification
  - UI path: `ui/src/components/feed-detail/section-asn.tsx` renders the
    critical-infrastructure warning block; ASN/country/IP detail pages expose
    `infrastructure_role`
- Queried local live API for current warning impact by ASN.
- Verified sample public DNS origin ASNs using Team Cymru DNS.
- Checked official/current documentation for Cloudflare, Google Public DNS,
  Quad9, Cisco Umbrella/OpenDNS, AWS IP ranges, Microsoft Azure service tags,
  and GitHub IP ranges.
- Read `.agents/knowledge/critical-infrastructure.md` in full and updated the
  recommendation from ASN-list cleanup toward a tiered reference-feed overlap
  model.
- No config/code changes made yet; design decision pending.

2026-04-29:

- Ran six read-only search/research tasks to complete the hard, soft, and
  contextual candidate list before implementation:
  - hard public DNS/DNS-root/AS112/NTP/time
  - soft CDN/developer/payment/CA/software-update candidates
  - contextual cloud/SaaS/email/identity/observability candidates
  - local catalog/MISP fit and exclusions
- Cross-checked representative official sources directly:
  - IANA root servers and InterNIC root hints
  - RFC 7534/7535 AS112 service prefixes
  - Cloudflare, Google, Quad9, Cisco, Vercara, AdGuard, Control D, Mullvad,
    DNS.SB public DNS pages
  - Cloudflare, Google, NIST public time/NTP pages
  - AWS IP ranges, Azure service tags, Google `goog.json`/`cloud.json`,
    Cloudflare IP ranges, Fastly public IP list
  - Auth0, Okta, Datadog, Atlassian, Stripe, Braintree, Mollie, Adyen official
    IP/allowlist pages
- Added an expanded candidate taxonomy and source-quality rules to the SOW.
- No config/code changes were made during the research phase; implementation
  design decisions were still pending at that point.
- Recorded user approval to fix verified factual issues before implementation.
- Corrected `.agents/knowledge/critical-infrastructure.md`:
  - AS40443 is CDK Global, not DigiCert.
  - AS60924 is ORIXCOM, not Alibaba.
  - AWS `ip-ranges.json` has no `SES` service tag.
  - GCP IPv6 metadata remains unverified; Google docs say to use the IPv4
    metadata address even with IPv6-only instances.
  - Apple `17.0.0.0/8` is official but broad, so it is soft/contextual, not
    hard.
  - Cloud metadata is local/operator enforcement context, not a normal public
    internet reference feed.
  - no-license/community cloud feeds are secondary only and need explicit
    source-quality/license caveats.
- Removed the incorrect AS35995 GitLab entry from
  `configs/firehol/infrastructure-asns.yaml`; GitLab will be represented later
  through official IP ranges in the reference-feed model, not this ASN list.
- Implemented the `schema-and-config-contract` chunk:
  - added typed `critical:` metadata to `pkg/config.Source` and
    `pkg/config.Merge`
  - added validation that `use: [critical_infrastructure]` requires
    `critical:` and that `critical:` is invalid without the role
  - validates tier, role, source type, source quality, and non-empty rationale
  - copies `critical:` metadata from merge declarations to expanded merge
    sources
  - clears inherited critical metadata from history derivatives, which are
    plain derived feed bodies rather than critical reference providers
  - added `cloud_proxy` as a validated role after the Zscaler catalog review
  - added `authoritative_plain_text` as a validated source type for official
    CIDR text endpoints
  - added config tests for validation and merge propagation
  - updated `.agents/sow/specs/config.md` with the typed metadata contract
  - updated the SOW and critical-infrastructure knowledge file with the
    eight-file research corpus reconciliation: StackPath stale source status,
    AWS/GCP MISP drift counts, GitHub MISP flattening, Zscaler role, Apple
    split-feed rule, new verified candidates, and rejects/open questions
  - removed `configs/firehol/sources/provider_infrastructure/misp_stackpath.yaml`
    after user approved deleting the stale MISP source
  - updated catalog count tests from 397 to 396 runtime sources

## Validation

- 2026-04-29 correction cleanup:
  - `rg` check found no remaining active bad claims for `DigiCert (AS40443)`,
    `AS60924` as Alibaba, Fastly `public-ip-ip-list`, `service: SES`,
    `GitLab Inc`, or `GitLab.com SaaS` in the patched knowledge/config files.
    Remaining AS35995/AS60924 mentions are historical correction notes.
  - `go test ./pkg/config` passed after removing the incorrect AS35995 config
    entry.
- 2026-04-29 schema/config contract:
  - `gofmt -w pkg/config/config.go pkg/config/expand.go pkg/config/validate.go
    pkg/config/config_test.go`
  - `go test ./pkg/config` passed.
- 2026-04-29 research reconciliation/schema extension:
  - Verified live upstream counts for AWS `ip-ranges.json`, GCP `cloud.json`,
    GitHub Meta API, local MISP AWS/GCP/GitHub warninglists, MISP StackPath,
    and Zscaler hub CIDR API.
  - `gofmt -w pkg/config/config.go pkg/config/expand.go pkg/config/validate.go
    pkg/config/config_test.go`
  - `go test ./pkg/config` passed.
- 2026-04-29 StackPath catalog removal:
  - Removed `misp_stackpath` after user approval because StackPath CDN ceased
    operations and the MISP source is stale for the current catalog.
  - Updated duplicated catalog count assertions to 396 runtime sources.
  - `go test ./pkg/config ./pkg/processor` passed.
  - `go test ./...` passed.
  - `git diff --check` passed.

2026-04-29 implementation:

- Implemented the reference-feed overlap model selected by user:
  - removed the maintained `configs/firehol/infrastructure-asns.yaml` list from
    the FireHOL catalog so ASN-wide hits no longer drive critical warnings
  - changed config validation to reject any non-empty top-level
    `infrastructure_asns` list; remaining structs/read paths are inert for
    supported configs and can be removed as cleanup once this SOW lands
  - added three initial IPv4 hard-tier reference feeds under
    `configs/firehol/sources/provider_infrastructure/critical_dns.yaml`:
    `critical_public_dns_core`, `critical_dns_root_servers`, and
    `critical_as112`
  - added eight initial official/provider-backed range references under
    `configs/firehol/sources/provider_infrastructure/critical_provider_ranges.yaml`:
    soft Cloudflare edge, soft Fastly edge, contextual AWS cloud, contextual GCP
    cloud, contextual Oracle Cloud, contextual DigitalOcean geofeed, contextual
    Linode geofeed, and contextual Vultr geofeed
  - reference feed bodies use YAML `static:` data, not Go code, after user
    rejected hardcoded IP/CIDR lists in the binary
- Implemented generic config-backed static source support:
  - `pkg/config.Source.Static []string`
  - validation rejects `url:` + `static:` together, empty static lines, and
    multiline static entries
  - validation rejects critical-infrastructure IPv6 sources/merges in v1 so the
    IPv4-only limitation is enforced by config, not only by documentation
  - download staging materializes `static:` lines as the raw source body and
    then uses the normal processor/finalize path
  - scheduler snapshots compare the materialized `.source` body with the current
    YAML `static:` body so edits to `frequency: 0` static feeds trigger a due
    download/materialization
  - history derivatives clear inherited `static:` bodies so derived sources do
    not accidentally contain both `url:` and `static:`
- Implemented critical overlap artifacts:
  - loads committed `use: [critical_infrastructure]` reference sets
  - skips reference-feed self-overlap targets
  - skips non-IPv4 targets for v1
  - writes `<feed>_critical_infrastructure.json`
  - writes `<feed>_critical_<provider>.json`, including zero-overlap provider
    artifacts
  - computes each provider intersection in one pass and de-duplicates aggregate
    tier totals by IP, avoiding avoidable repeated intersection work
  - provider updates fan out to all public feeds only for the artifact family
    that depends on that provider role; a critical-reference update no longer
    forces unrelated geo, ASN, bogon, or entity sidecar rebuilds
- Implemented public API routes:
  - `GET /api/v1/sets/{name}/infrastructure`
  - `GET /api/v1/sets/{name}/infrastructure/providers`
  - `GET /api/v1/sets/{name}/infrastructure/{provider}`
  - routes serve published artifacts only; they do not compute overlap on
    request
  - provider routes verify the provider still exists in current config before
    serving the file, so stale removed-provider artifacts return 404
- Updated insights:
  - `infrastructure_present` now reads the critical aggregate artifact instead
    of ASN-wide `infrastructure.total_ips`
  - insight wording now says "critical infrastructure reference feeds"
- Updated public feed page:
  - added a dedicated Critical Infrastructure overlap section before ASN
    composition
  - removed the old ASN-section critical-infrastructure warning block
  - removed remaining ASN-table/chart critical-infrastructure markers so stale
    ASN artifacts cannot keep presenting the retired ASN-wide model as current
  - removed old ASN infrastructure badges from ASN detail, ASN index, country
    detail, and IP lookup UI surfaces; critical infrastructure is now surfaced
    through the dedicated reference-feed overlap section
- Updated methodology/docs/specs:
  - rewrote `infrastructure-asns.md` as the reference-feed methodology page
    while preserving the public slug
  - rewrote `infrastructure-present.md` for aggregate overlap artifacts
  - updated ASN and bogon methodology links
  - added `docs/critical-infrastructure-reference-feeds.md`
  - updated config/feed/website specs for `static:` sources, reference-feed
    critical metadata, overlap artifacts, and public API routes
  - updated project coding/testing skills so future helpers do not hardcode
    operator-policy IP/CIDR lists in Go/UI and must test config-backed static
    source behavior
- First external review pass findings addressed:
  - static config edits now trigger scheduler due state
  - static curated source quality downgraded to `C`
  - reference feed pages no longer show a misleading missing-overlap warning for
    themselves
  - FileSet errors in the critical writer are returned instead of swallowed
  - removed-provider artifacts are blocked by config-aware route validation
  - tab indentation and stale static inheritance in history derivatives were
    fixed
  - IPv4-only scope is enforced in validation and documented in UI/methodology
  - the initial catalog now includes hard, soft, and contextual references
    instead of hard DNS-only references
  - the methodology static example now marks curated static sources as
    `source_quality: C`
  - `.agents/sow/specs/files-layout.md` now lists the critical aggregate and
    provider artifact filenames
- Fixed a local CPU fan-out regression found during review:
  - the shared `targetFeedsForFanOut` helper now accepts caller-specific
    provider roles
  - geo writers fan out on geo provider updates only
  - bogon writers fan out on bogon provider updates only
  - ASN writers fan out on ASN and bogon provider updates, because ASN unknown
    bucket splitting depends on bogon data
  - critical writers fan out on critical-reference provider updates only
  - entity sidecars fan out on geo/ASN provider updates only

- [x] Acceptance criteria evidence
- [x] Local validation evidence recorded
- [x] Real-use validation evidence on a running service
- [x] Second-pass cross-model reviewer findings logged + addressed
- [x] Repeat cross-model reviewer pass after fixes reports no new blockers
- [x] Lessons extracted (or "none, reasoning: ...")
- [x] Same-failure-at-other-scales check

- 2026-04-29 reference-feed implementation:
  - `gofmt -w pkg/config/config.go pkg/config/validate.go
    pkg/config/config_test.go pkg/config/config_coverage_test.go
    pkg/config/catalog_verify_test.go pkg/processor/processor_test.go
    pkg/engine/download_stage.go pkg/engine/critical_test.go
    pkg/engine/insights.go pkg/insights/rules_asn.go`
  - `go test ./pkg/config ./pkg/processor ./pkg/engine ./pkg/web
    ./pkg/insights` passed after fixing the signal-snapshot test setup
  - `pnpm --dir ui build` passed; Vite reported the existing font-resolution
    and chunk-size warnings
  - `pnpm --dir ui lint` passed
- 2026-04-29 post-hardcoding-fix validation:
  - YAML parse check passed for `configs/firehol/**/*.yaml` and
    `configs/firehol/*.yaml`
  - `go test ./pkg/config ./pkg/processor ./pkg/scheduler ./pkg/engine
    ./pkg/web` passed
  - `go test ./...` passed
  - `pnpm --dir ui build` passed; Vite reported the existing font-resolution
    and chunk-size warnings
  - `pnpm --dir ui lint` passed
  - `make lint` passed
  - `make test` passed
  - `git diff --check` passed
- 2026-04-29 role-scoped fan-out fix:
  - `gofmt -w pkg/engine/helpers.go pkg/engine/geoloc.go
    pkg/engine/bogons.go pkg/engine/asn.go pkg/engine/critical.go
    pkg/engine/entity_feed_sidecar.go pkg/engine/entity_artifacts.go
    pkg/engine/critical_test.go`
  - `go test ./pkg/engine ./pkg/scheduler ./pkg/web` passed
  - `git diff --check` passed
- 2026-04-29 UI ASN-surface cleanup:
  - `pnpm --dir ui lint` passed
  - `pnpm --dir ui build` passed; Vite reported the existing font-resolution
    and chunk-size warnings
  - `git diff --check` passed
- 2026-04-29 second-pass external review fixes:
  - aligned integrity expectations with the critical writer's v1 target filter,
    so IPv6 feeds and critical reference feeds do not receive false missing
    critical-overlap artifact findings
  - added `TestExpectedSecondaryFilesSkipsCriticalArtifactsForIPv6Feeds`
  - removed stale ASN-section copy that implied ASN-wide criticality still lived
    in that section
  - normalized the catalog verification source-list order/indentation around
    `critical_*` feeds
  - documented the integrity rule in `.agents/sow/specs/integrity.md`
  - `go test ./pkg/engine ./pkg/web ./pkg/config ./pkg/scheduler` passed
  - `pnpm --dir ui lint` passed
  - `pnpm --dir ui build` passed; Vite reported the existing font-resolution
    and chunk-size warnings
  - `git diff --check` passed
- 2026-04-29 stale-provider-set and no-hardcoding hardening:
  - kept critical IP/CIDR data in config-backed YAML `static:` or upstream
    `url:` sources; no critical service-address list is compiled into Go/UI
    code
  - added a runtime critical provider-set identity that fingerprints configured
    provider metadata plus processed provider cache state
  - scheduler snapshots now mark critical reference sources due with forced
    refresh when the current provider-set identity differs from
    `lib/critical_infrastructure/provider_set_id`
  - the engine regenerates all critical-overlap artifacts and dependent
    insights on provider-set drift, but avoids unrelated geo/ASN/bogon/entity
    fan-out when that is the only reason for the run
  - aggregate and per-provider critical JSON artifacts now carry
    `provider_set_id`; public API routes, direct static JSON routes, and
    integrity checks reject stale provider-set payloads
  - removed stale removed-provider critical JSON artifacts during rebuild and
    returned those paths from the staged publisher so git-backed web sync can
    stage deletions
  - removed the old ASN `infrastructure` payload surface from newly generated
    ASN artifacts and TypeScript types, so the retired ASN-wide warning model
    cannot reappear through stale UI contracts
  - validation now rejects `use: [critical_infrastructure]` combined with
    provider-database roles such as `asn` or `geoip`
  - updated config, files-layout, pipeline, integrity, website specs and the
    public/operator documentation with provider-set drift and stale-artifact
    semantics
  - `go test ./pkg/engine ./pkg/web ./pkg/config ./pkg/scheduler
    ./pkg/insights` passed
- 2026-04-29 provider-set stability regression found during real-use
  validation:
  - installed service logs showed the daemon reprocessing the same 11 critical
    providers about every scheduler tick after each forced refresh
  - root cause: the provider-set fingerprint included volatile `Version` and
    `ProcessedDate`, so the act of reprocessing changed the fingerprint again
  - fixed by adding stable processed-set `content_hash` to cache entries and
    using provider metadata + `content_hash`/cardinality for the provider-set
    identity
  - added regression coverage proving `Version`/`ProcessedDate` changes do not
    mark the provider set changed, while `content_hash` changes do
  - installed/restarted the development service with `./install.sh`
  - real-use validation showed the daemon settled to `running=false`, critical
    providers no longer had `critical infrastructure provider set changed` due
    details, and `abuseipdb_1d` critical aggregate/API/direct routes returned
    `200`
  - `go test ./pkg/engine ./pkg/scheduler ./pkg/config ./pkg/web` passed
  - `make test` passed
  - `make lint` passed
  - `make build` passed
  - `pnpm --dir ui lint` passed
  - `pnpm --dir ui build` passed; Vite reported the existing font-resolution
    and chunk-size warnings
  - `git diff --check` passed
- 2026-04-29 repeat-review hardening before close:
  - changed `infrastructure_present` from aggregate-only to tier-aware facts:
    hard-tier overlap fires on any positive count for non-empty feeds, while
    soft/contextual overlap keeps sample/threshold guards and policy-context
    wording
  - feed detail now renders explicit hard, soft, and contextual summary tiles
    instead of only aggregate + hard-tier count
  - cached the current critical provider-set ID in the engine and refreshed it
    on startup/reload/provider processing, so public stale-artifact checks no
    longer rebuild the provider-set identity from cache entries per request
  - integrity now expects per-provider critical-overlap files only for
    configured critical providers with materialized latest sets; the aggregate
    file still records unavailable providers via `missing_providers`
  - moved the critical tier presentation order to `config.CriticalTiers()` so
    validation and the aggregate writer share one tier list
  - removed output-family/file-name cache fields from the provider-set runtime
    identity; stable semantic identity now depends on provider metadata,
    processing config, `content_hash`, and cardinality
  - public API/direct artifact routes and insight snapshot loading now reject
    stale critical artifacts for feeds that are no longer comparable targets
    such as IPv6 feeds or critical reference feeds
  - startup/reload cleanup now removes stale critical artifacts for removed
    providers and target-eligibility changes, not only when all providers are
    removed
  - updated `infrastructure-present` methodology with hard vs soft/contextual
    thresholds
  - updated specs and project skills with the no-hardcoding, cached-stale-ID,
    tier-aware insight, and loaded-provider integrity rules
  - `go test ./pkg/insights ./pkg/engine ./pkg/web ./pkg/scheduler
    ./pkg/config` passed
  - `pnpm --dir ui lint` passed
- 2026-04-29 final close-out:
  - Codex, Claude, and GLM repeat reviews reported no technical blockers; the
    only blocking item was completing this SOW close-out
  - fixed the AS112 catalog citation so the entry references both RFC 7534 and
    RFC 7535, matching the two configured IPv4 service prefixes
  - same-failure audit searched for other provider-set/fingerprint mechanisms
    using volatile cache fields; no other provider-set marker path exists, and
    `TestBuildSnapshotCriticalProviderSetChangesAreForcedDue` pins that
    `Version`, `ProcessedDate`, and `CheckedDate` changes do not retrigger the
    critical provider-set loop while `content_hash` and processing config do
  - hardcoded critical-IP scan found remaining literals only in tests, one UI
    placeholder, and a public API example, not production policy data
  - installed and restarted the development service with `./install.sh`
  - live service smoke:
    - `/healthz` returned `ok`
    - `/api/v1/sets` returned 387 public sets
    - `/api/v1/sets/abuseipdb_1d/infrastructure/providers` returned 11
      critical providers, all with `redistributable:false`
    - raw critical provider routes
      `/api/v1/sets/critical_public_dns_core/data`,
      `/files/critical_public_dns_core.netset`, and
      `/critical_public_dns_core.netset` returned `404`
    - `/api/v1/compose?include=critical_public_dns_core&format=single`
      returned `400`
    - `/api/v1/sets/abuseipdb_1d/infrastructure` returned `200` with
      `critical_ips:4441`, `complete:true`, and contextual-tier overlap
    - `/api/v1/sets/abuseipdb_1d/infrastructure/critical_public_dns_core`
      returned `200` with `critical_ips:0`
    - `/abuseipdb_1d_critical_infrastructure.json` returned `200`
    - admin snapshot had empty queues and zero
      `critical infrastructure provider set changed` due items
	  - final local validation passed:
	    - `go test ./pkg/config ./pkg/engine ./pkg/web ./pkg/scheduler
	      ./pkg/insights`
    - `(cd tools/dronebl2ipsets && go test ./...)`
    - `make test`
    - `make lint`
    - `make build`
    - `make race`
    - `pnpm --dir ui lint`
	    - `pnpm --dir ui build` (existing InterDisplay font-resolution and Vite
	      chunk-size warnings only)
	    - `git diff --check`
- 2026-04-29 regression cycle:
  - reopened SOW-0017 after user found two presentation/process regressions
  - added `.agents/skills/project-content-surfaces/SKILL.md` defining separate
    success criteria for SOWs, specs, public methodology, operator docs, public
    UI copy, admin UI copy, code comments, and project skills
  - updated `AGENTS.md`, project coding/reviewing/testing skills, and
    `.agents/sow/specs/website.md` so future changes must validate the target
    surface/audience instead of copying text mechanically across artifacts
  - rewrote `pkg/web/static/methodology/infrastructure-asns.md` as an end-user
    methodology page covering meaning, levels, current hard/soft/contextual
    coverage, strengths, weaknesses, missing/deferred coverage, and
    interpretation
  - rewrote `pkg/web/static/methodology/infrastructure-present.md` around the
    insight's risk meaning, thresholds, interpretation, and limitations
  - changed the feed-detail matched-reference table so its default order is
    `hard`, then `soft`, then `contextual`, with matched-IP count as the
    secondary sort inside each tier
  - added a compound comparator hook to the shared editorial `DataTable` so this
    ordering is explicit rather than relying on only one raw numeric column
  - validation passed:
    - `pnpm --dir ui lint`
    - `pnpm --dir ui build` (existing InterDisplay font-resolution and Vite
      chunk-size warnings only)
    - `go test ./pkg/web ./pkg/config ./pkg/engine ./pkg/insights`
    - `git diff --check`
    - methodology surface scan found no misplaced implementation markers in the
      two critical-infrastructure public methodology pages
    - live `/healthz` returned `ok`
    - live methodology API served the rewritten critical-infrastructure pages
    - Playwright loaded `/ipsets/bitwire_inbound` and confirmed the first matched
      rows are hard DNS root, then soft Cloudflare/Fastly, then contextual cloud
      providers despite the contextual providers having much larger matched-IP
      counts

## Outcome

Completed.

SOW-0017 replaces ASN-wide critical infrastructure warnings with typed,
config-backed critical-infrastructure reference feeds and precomputed per-feed
overlap artifacts. The public UI/API now surfaces hard, soft, and contextual
critical-infrastructure overlap from generated artifacts, while raw reference
feed bodies remain governed by `redistributable`.

The legacy `infrastructure_asns` catalog is removed and rejected during config
validation. New critical reference data is stored in
`configs/firehol/sources/provider_infrastructure/critical_dns.yaml` and
`critical_provider_ranges.yaml`, using `static:` for curated narrow service
prefixes and `url:` for provider-published ranges. Shipped critical reference
feeds are non-redistributable by default.

Public serving remains cache-first: critical-overlap routes read published JSON
artifacts only and reject stale `provider_set_id` payloads. Scheduler/runtime
logic detects provider-set drift, refreshes critical providers, regenerates
affected overlap artifacts, and avoids unrelated geo/ASN/bogon/entity fan-out
when critical-provider drift is the only reason for work.

Regression fixes added a durable content-surface guardrail and corrected the
feed-detail default ordering. Public methodology now explains the user-facing
meaning and limits of critical-infrastructure overlap. Feed pages now surface
hard-tier matches before larger soft/contextual matches, so small DNS/root
overlaps are not buried below broad cloud-provider coverage.

## Lessons extracted

- Critical infrastructure must be modeled as exact reference IP feeds, not
  broad ASNs. ASN-wide warnings produce too many false positives and are too
  easy to reintroduce accidentally.
- Public metadata visibility and raw redistribution are different policies.
  Critical reference providers may be visible as methodology/overlap inputs
  while their raw bodies remain unavailable for direct download or compose.
- Provider-set fingerprints must exclude volatile processing fields. The
  provider-set loop regression happened because `Version` and `ProcessedDate`
  were treated as semantic identity; `content_hash` and processing-shape config
  are the stable signals.
- Scheduler due logic must not repeatedly force-refresh critical providers for
  provider-set drift while an engine run is active. The active run owns marker
  publication; requeueing during that window can hammer upstream services before
  the marker has a chance to settle.
- Critical IP/CIDR policy data belongs in YAML `static:`, `url:`, or merge
  configuration, not Go or UI code. This lesson is recorded in project-coding,
  project-reviewing, project-testing, `AGENTS.md`, docs, and specs.
- Public route tests for generated suffix artifacts must include exact feed
  names that look like generated artifacts, stale provider IDs, removed
  providers, non-comparable targets, raw-route denial, and compose denial.
- Public methodology, operator docs, specs, SOWs, public UI, and admin UI are
  different surfaces with different success criteria. A SOW/spec implementation
  explanation must not be promoted into a public methodology page unless it is
  rewritten for user interpretation.
- Critical-infrastructure UI must follow the risk model before volume. Hard
  matches belong above larger soft/contextual matches because operational blast
  radius is not the same as matched-IP count.

## Regression

### Regression 1 — 2026-04-29

**Reported by:** user

**Symptom:** the public critical-infrastructure methodology page explained
configuration, artifact names, API routes, and code locations instead of
explaining what critical infrastructure is, what hard/soft/contextual levels
mean, why each level matters, strengths, weaknesses, and missing coverage.

**Detection:** user review of the public methodology page. Concrete evidence:
`pkg/web/static/methodology/infrastructure-asns.md` contained YAML/schema
details, artifact names, API route lists, and code-path references in a public
methodology page.

**Original Outcome (for reference):** SOW-0017 claimed methodology/docs/specs
were complete.

**Why the original Validation missed this:** validation and reviewers checked
technical consistency with the implementation, but did not validate the
surface/audience contract of each artifact. The missing review question was
"does this public methodology page answer the user's public interpretation
needs?"

**Fix scope:** add a project content-surface skill, strengthen `AGENTS.md` and
review/testing/coding skills with explicit surface contracts, rewrite the public
methodology pages for user interpretation, and keep implementation/config/API
details in operator docs and specs.

### Regression 2 — 2026-04-29

**Reported by:** user

**Symptom:** the feed-detail "Matched reference feeds" table sorted
critical-infrastructure overlaps by matched IP count, causing broad contextual
cloud/provider overlaps to bury hard-tier public DNS matches.

**Detection:** user review of the public feed page. Concrete evidence:
`ui/src/components/feed-detail/section-critical-infrastructure.tsx` used
`initialSortKey="critical_ips"` and `initialSortDir="desc"` for the critical
reference-feed table.

**Original Outcome (for reference):** SOW-0017 claimed the feed page surfaced
hard/soft/contextual critical-infrastructure overlap in the right way.

**Why the original Validation missed this:** validation checked that tier data
was present and visually labeled, but not that default ordering followed the
criticality/risk model. The table optimized for volume, not operator risk.

**Fix scope:** make the default table order criticality-first
(`hard`, `soft`, `contextual`), then matched IP count within the same tier, and
record this UI interpretation rule in the website spec and project review
skill.

### Regression 3 — 2026-04-29

**Reported by:** user.

**Symptom:** both admin integrity checks list many feeds after recent changes.
Pipeline integrity reports critical-overlap JSON as malformed, while entity
artifact integrity reports hundreds of feed entity sidecars as stale.

**Detection:**

- `GET /api/v1/admin/integrity` returned `status: issues`, `count: 4`.
  Three feeds with names containing `_bogons` report 11 malformed critical
  provider files each, for example
  `cidr_report_bogons_critical_critical_as112.json`.
- `GET /api/v1/admin/integrity/entities` returned `status: issues`,
  `count: 329`, all `feed_sidecar_stale`, all with repair action
  `refresh_feed`. Example: `abuseipdb_1d` sidecar mtime
  `2026-04-29T09:34:19Z` is compared to
  `abuseipdb_1d_asn_caida_prefix2as.json` mtime
  `2026-04-29T09:56:23Z`.
- The reported malformed critical provider files are valid critical-provider
  JSON on disk; the current finding is therefore a validator classification
  false positive, not corrupt JSON.

**Original Outcome (for reference):** SOW-0017 claimed critical-overlap routes
read published JSON artifacts, reject stale provider-set payloads, and that
integrity expectations were aligned with critical-overlap artifacts.

**Why the original Validation missed this:** validation covered generic sample
feeds and feed names containing `_critical`, but did not include a feed name that
looks like another generated artifact family (`_bogons`) combined with a
critical provider artifact. Entity validation accepted mtime-driven freshness as
the contract, so broad provider-derived artifact rewrites still explode into a
large operator-facing stale list.

**Fix scope:** identify and correct the structural integrity fragility: typed
artifact validation must not reverse-engineer artifact type from substring
matches, and entity freshness must distinguish semantic drift from harmless
derived-artifact mtime churn. Add tests/spec/skill guardrails before closing.

#### Regression 3 analysis

Facts:

- Pipeline integrity validates each expected secondary path in
  `pkg/engine/integrity.go` by iterating `expectedSecondaryFiles()` and passing
  only a relative filename string into `validateStructuredSecondary()`.
- `validateStructuredSecondary()` classifies artifact type with substring and
  suffix checks. `_asn_` and `_bogons_` are tested before critical-overlap
  artifacts, so a valid file like
  `cidr_report_bogons_critical_critical_as112.json` is parsed as a bogon
  overlap payload instead of a critical provider payload.
- The same file served through the public direct route and through
  `/api/v1/sets/cidr_report_bogons/infrastructure/critical_as112` is valid
  critical provider JSON with the current `provider_set_id`; the integrity
  finding is a false positive.
- `data_shield_critical` is a normal public feed whose name includes the word
  `critical`. The live web tree contains both old-style
  `data_shield_critical_<provider>.json` critical artifacts and current
  `data_shield_critical_critical_<provider>.json` artifacts, while its normal
  ASN/geo/bogon/comparison/insight/retention secondaries are missing. This is a
  real missing-output finding and a second example of suffix grammar ambiguity
  around feed names that look like generated artifact names.
- Entity integrity computes each feed sidecar reference as the newest of the
  feed's geo artifact, ASN artifact, latest set, geo provider data file, and ASN
  provider data file. It reports `feed_sidecar_stale` when the sidecar file mtime
  is older than that newest local input.
- The current pipeline does not consistently maintain those mtimes as logical
  source-change times. Public staged JSON files are written with current
  filesystem time and published by rename; the publish step does not apply the
  logical source timestamp carried in `output.GeneratedFile.Timestamp`.
- Live entity integrity reported hundreds of stale feed sidecars, with examples
  comparing sidecar mtimes around `2026-04-29T09:34Z` to ASN artifact mtimes
  around `2026-04-29T09:56Z`. All sampled references ended in
  `_asn_caida_prefix2as.json`.
- The project spec already says entity-sidecar freshness metadata may be used to
  avoid false-positive repair, and that freshness must not cascade into broad
  country/ASN repairs unless concrete semantic drift exists.

Root causes:

1. Pipeline integrity uses filename heuristics as the artifact type system.
   Expected artifacts are produced as strings; the validator later guesses the
   schema from substrings. This is fragile because feed names and provider names
   are not escaped from generated-family markers such as `_bogons_`,
   `_critical_`, and `_asn_`.
2. Entity integrity assumes mtimes are logical freshness timestamps, but the
   pipeline often leaves generated artifact mtimes as wall-clock write times.
   Broad rewrites of ASN/geo artifacts can therefore make hundreds of feed
   sidecars look stale even when the feed membership and entity contribution
   sidecar are unchanged. This is a pipeline timestamp-contract gap, not a
   fundamental problem with filesystem mtimes.
3. The pipeline has no single artifact manifest shared by writers, validators,
   cleanup, public routing, and repair. Every new artifact family currently
   requires separate edits to producer logic, integrity expectations, payload
   validation, stale cleanup, public route parsing, recovery planning, and tests.
   Missing one of those paths creates noisy integrity exceptions.

Working conclusion:

- The current noisy output is not evidence that hundreds of feeds are corrupted.
  The critical-provider malformed findings are false positives. The entity
  findings mostly indicate that the pipeline broke the logical-mtime contract
  after provider-derived artifact rewrites. `data_shield_critical` remains a
  real missing-secondary case that should be repaired, but it also exposes the
  same filename grammar weakness.

User design direction recorded 2026-04-29:

- The application has at least three different timestamp meanings:
  source timestamp, processing timestamp, and wall-clock timestamp.
- The product needs an explicit pipeline timestamp/integrity specification that
  defines which timestamp each file's mtime represents for each operation.
- Integrity checks may rely on mtime only after the pipeline consistently
  maintains that mtime contract.
- No semantic classification may be derived from feed-name substrings, prefixes,
  or suffixes. Config tags and typed fields are the authority for provider role,
  feed role, artifact family, and criticality. Exact configured-name lookup is
  allowed only as identity lookup, not as semantic inference.
- No file created by the application may accidentally inherit wall-clock mtime
  when that file participates in pipeline integrity. Writers and staged publish
  paths must deliberately set the file mtime to the timestamp defined by the
  pipeline timestamp contract, and integrity checks must validate that contract.

Required remediation order:

1. Write specs first:
   - semantic classification contract: config tags/typed fields are authoritative;
     feed-name substrings are never semantic classification
   - pipeline timestamp contract: source, processing, and wall-clock timestamps;
     per-file mtime ownership and integrity expectations
2. Create tests second:
   - tests must encode the new specs before code is changed
   - tests must fail against current fragile behavior where practical
3. Fix code third:
   - replace semantic filename matching with config-backed typed descriptors
   - make writers/publish paths deliberately set required mtimes
   - make integrity validate both contracts

Prevention rule added to `AGENTS.md`:

- agents must not derive semantic meaning from feed/provider/artifact name
  patterns; config fields, `use:` tags, and typed metadata are source of truth
- agents must treat generated file mtimes as pipeline integrity data and avoid
  accidental wall-clock mtimes for app-created files covered by integrity

#### Regression 3 remediation plan

Chunked remediation - reasoning: this is a structural integrity fix, not a
localized parser patch. The work touches specs, tests, engine artifact identity,
staged publish semantics, integrity, cleanup, public routes, and live repair.

1. `spec-semantic-classification` - high risk
   - Update specs before code to make this normative:
     config fields, `use:` tags, and typed metadata are source of truth for
     source/provider/artifact roles.
   - Define the allowed distinction between exact configured-name identity lookup
     and forbidden semantic inference from name substrings/prefixes/suffixes.
   - Expected spec targets: `.agents/sow/specs/config.md`,
     `.agents/sow/specs/integrity.md`, `.agents/sow/specs/files-layout.md`,
     `.agents/sow/specs/pipeline.md`, and `.agents/sow/specs/website.md` where
     public/direct artifact routing semantics are described.

2. `spec-pipeline-timestamps` - high risk
   - Add a timestamp contract before code:
     `source_timestamp`, `processing_timestamp`, and `wallclock_timestamp`.
   - Define, per durable file family, which timestamp owns mtime and why:
     raw source bodies, canonical `.ipset`/`.netset`, feed metadata JSON,
     geo/ASN/bogon/critical secondaries, comparison/retention/insight artifacts,
     entity feed sidecars, country/ASN sidecars, public country/ASN payloads,
     indexes, runtime markers, and scratch files.
   - Define where wall-clock mtime is allowed and where it is a bug.
   - Define integrity expectations that compare mtimes only after the pipeline
     has deliberately assigned the spec-owned timestamp.

3. `discrepancy-audit` - medium risk
   - Review the entire application for semantic name matching:
     `strings.Contains`, `HasPrefix`, `HasSuffix`, `TrimPrefix`, `TrimSuffix`,
     `Index`, path suffix helpers, direct route parsing, cleanup parsing, and
     tests that encode substring behavior.
   - Review all file creation/touch/publish paths for accidental mtimes:
     `writeFileAtomic`, `writeFileAtomicNoSync`, staged web/entity publish,
     public artifact writers, entity repair/touch helpers, copy-to-web helpers,
     and git/timestamp helper paths.
   - Record each finding as `allowed identity lookup`, `allowed path construction`,
     `must fix`, or `needs explicit spec exception`.

4. `tests-first` - high risk
   - Add or change tests before implementation so they encode the new specs:
     - critical/provider artifacts for feeds whose configured names contain
       `_bogons`, `_asn`, `_critical`, and other generated-family tokens
     - exact configured feed/provider identity lookup without substring
       classification
     - stale critical cleanup and direct routes using typed descriptors
     - staged publish applies deliberate mtimes to generated files
     - ASN/geo/bogon/critical secondaries carry the logical timestamp defined by
       the spec, not wall-clock write time
     - entity sidecar freshness uses the spec timestamp contract and does not
       produce broad false stale lists after no-op provider-derived rewrites
   - Add a codebase guard test where practical to catch new forbidden semantic
     name-pattern matching in engine/web integrity paths.

5. `typed-artifact-descriptors` - high risk
   - Introduce config-backed expected artifact descriptors:
     `kind`, `feed`, `provider`, `provider_use`, `critical_tier/role` where
     applicable, `rel_path`, `schema`, and validator.
   - Replace `expectedSecondaryFiles()` string-only output and
     `validateStructuredSecondary()` substring classification with descriptor
     validation.
   - Make cleanup and direct public artifact routing use exact descriptors or
     exact configured-name lookup, not substring parsing.
   - Keep filenames as storage addresses only; the descriptor carries semantics.

6. `mtime-contract-implementation` - high risk
   - Extend staged publish/write helpers so generated files receive an explicit
     mtime before or after atomic publish according to the timestamp contract.
   - Replace wall-clock `touch` calls on integrity-participating artifacts with
     contract-aware timestamp assignment.
   - Ensure entity feed sidecars and public entity payloads use the logical input
     timestamp defined by the spec when content is unchanged.
   - Keep wall-clock timestamps only for runtime/scratch/observability files that
     the spec explicitly excludes from integrity truth.

7. `repair-live-state` - medium risk
   - Install/restart the development service after code passes local tests.
   - Reprocess or repair `data_shield_critical` and any affected feeds through
     admin/scheduler mechanisms, not manual file edits.
   - Let background work settle; do not mask integrity findings by deleting files
     directly.

8. `validation-and-review` - high risk
   - Required local validation:
     targeted Go tests for config/engine/web/integrity, then `make test`,
     `make lint`, `make build`, and nested module tests if touched.
   - Required live validation:
     `/healthz`, `/api/v1/status`, `/api/v1/admin/integrity`,
     `/api/v1/admin/integrity/entities`, representative critical direct/API
     routes, and `data_shield_critical` secondaries after repair.
   - Same-failure scan:
     prove no remaining semantic name-pattern matching exists in classification
     paths and no app-created integrity file can accidentally inherit wall-clock
     mtime.
   - High-risk review:
     run the project review checklist; use independent read-only reviewers before
     close if authorized/available, or record an explicit validation gap.

9. `close-sow` - medium risk
   - Update specs with final contracts and observed behavior.
   - Update project skills with the exact regression lessons.
   - Move SOW-0017 back to `done/` only after specs, tests, implementation, live
     integrity, review findings, same-failure scan, and lessons are complete.

#### Regression 3 execution log

Implemented 2026-04-29.

Specs updated first:

- `.agents/sow/specs/config.md` now states that semantic classification comes
  from config fields, `use:` roles, typed metadata, or exact configured-name
  identity lookup only. Feed/provider name substrings are not semantic truth.
- `.agents/sow/specs/integrity.md` now defines source, processing, and
  wall-clock timestamps and the mtime contract for integrity-participating
  artifacts.
- `.agents/sow/specs/files-layout.md`, `.agents/sow/specs/pipeline.md`, and
  `.agents/sow/specs/website.md` now document staged-publish mtime preservation
  and exact configured-identity route parsing.

Tests added before and during implementation:

- `pkg/engine/integrity_test.go` covers the exact regression case where
  `cidr_report_bogons_critical_critical_as112.json` must validate as a critical
  provider artifact, not as a bogon artifact.
- `pkg/engine/web_batch_test.go` covers staged publication applying explicit
  `output.GeneratedFile.Timestamp` mtimes before publish.
- `pkg/engine/file_contract_test.go` covers public metadata, history,
  changesets, and retention artifacts carrying mtimes at least as recent as the
  processing timestamp, not the source timestamp.
- `pkg/engine/critical_test.go` and `pkg/web/feature_test.go` cover the live
  collision where both `data_shield` and `data_shield_critical` are configured
  public feeds and the provider name itself starts with `critical_`.

Code changes:

- Replaced string-only expected secondary files with typed
  `secondaryArtifactDescriptor` validation in `pkg/engine/integrity.go` and
  `pkg/engine/integrity_payloads.go`.
- Critical direct public routes and stale cleanup now parse generated artifacts
  only against exact configured public feeds and configured critical providers.
  Exact current provider identity is preferred before stale-provider fallback,
  so `data_shield_critical_critical_as112.json` is treated as
  `data_shield` + `critical_as112`, not as `data_shield_critical` + `as112`.
- Added staged publish mtime application in `pkg/engine/web_batch.go` and wired
  it into normal run, entity rebuild, entity repair, and surgical entity
  refresh paths.
- Added `writeFileAtomicAt()` and processing-timestamp helpers; geo, ASN,
  bogon, critical, comparison, insights, public metadata/history/changesets,
  and public retention writers now assign contract-owned mtimes where integrity
  depends on them.
- Entity unchanged-file repair paths now use logical timestamps instead of
  wall-clock touches.
- Fixed the second root cause found during live validation: public
  history/changesets/retention artifacts were deliberately stamped with
  `SourceDate` while integrity compares them to `ProcessedDate`. They now use
  the feed processing timestamp.
- Fixed the third root cause found during delayed live validation: exact feed
  prefix matching alone was still ambiguous when one configured feed name was a
  prefix of another configured feed name and provider names began with
  `critical_`. The parser now uses exact configured feed and exact configured
  provider identity together.

Live repair:

- Installed the corrected binary with `./install.sh`, which restarted
  `update-ipsets.service`.
- Used `/api/v1/admin/integrity/entities/rebuild` for entity repair.
- Used `/api/v1/admin/integrity/reprocess` for stale/missing secondary repair.
- After the late `data_shield`/`data_shield_critical` ambiguity fix, installed
  again and reprocessed both feeds through
  `/api/v1/admin/feeds/data_shield/reprocess` and
  `/api/v1/admin/feeds/data_shield_critical/reprocess`.
- Did not manually delete or edit live artifacts.

Validation:

- `go test ./pkg/engine -run 'TestBashCompatibleHistoryChangesetAndRetentionFiles|TestValidateStructuredSecondaryUsesExactCriticalProviderDescriptor|TestStagedPublishBatchAppliesGeneratedFileTimestamps'` passed.
- `go test ./pkg/engine ./pkg/web ./pkg/config` passed.
- `make test` passed.
- `make lint` passed.
- `make build` passed.
- `./install.sh` passed and restarted the development service.
- `curl http://localhost:18888/healthz` returned `ok`.
- `/api/v1/admin/integrity` returned `clean`, count `0`.
- `/api/v1/admin/integrity/entities` returned `clean`, count `0`.
- `/api/v1/sets/cidr_report_bogons/infrastructure/critical_as112` returned the
  current hard-tier critical provider payload.
- `/cidr_report_bogons_critical_critical_as112.json` returned the same current
  critical provider payload.
- Post-repair spot checks confirmed stale public history/changesets/retention
  files were restamped to the new processing timestamps.
- Delayed settle-window validation after the final install returned clean for
  both `/api/v1/admin/integrity` and `/api/v1/admin/integrity/entities`.
- Direct artifact spot checks returned published bytes for
  `/data_shield_critical_critical_as112.json`,
  `/data_shield_critical_retention.json`, and
  `/cidr_report_bogons_critical_critical_as112.json`.

Same-failure scan:

- Reviewed remaining substring/suffix code in engine/web. Remaining generated
  artifact suffix parsing is storage-address decoding bounded by exact
  configured public feed/provider identities or by public-route path parsing;
  it is no longer used as semantic feed/provider classification.
- Reviewed file writers found by `writeFileAtomic`, `os.WriteFile`, `Chtimes`,
  `touchFileAt`, `GeneratedFile`, and `applyGeneratedFileTimestamps` searches.
  Integrity-participating public artifacts covered by this regression now have
  explicit producer-owned mtimes before publication. Scratch/test/runtime-only
  files remain outside integrity truth.

Skills/agent memory updated:

- `AGENTS.md` now carries the no-feed-name-matching and pipeline-mtime rules.
- `.agents/skills/project-coding/SKILL.md` now requires config/typed metadata
  for semantic classification and explicit mtimes for integrity artifacts.
- `.agents/skills/project-testing/SKILL.md` now requires adversarial configured
  names and end-to-end mtime/integrity publication assertions.
- `.agents/skills/project-reviewing/SKILL.md` now includes review gates for
  semantic name matching and accidental mtime drift.

Reviewer note:

- No external/subagent review was run for this final remediation pass because
  the active session rules only allow subagents or external assistants when the
  user explicitly requests them for the current task. The project review
  checklist, same-failure scan, local test gates, install, and live integrity
  validation were completed.

### Regression 4 — 2026-04-29

**Reported by:** user, after the previous regression fix and install.

**Symptom:**

- `/api/v1/admin/integrity` could be clean after repair, but
  `/api/v1/admin/integrity/entities` still reported settled entity findings.
- The live entity integrity result first showed 262 findings, mostly
  `detail_public_stale` for ASN pages whose public JSON mtime was older than
  the matching private entity sidecar.
- After background repair settled, the remaining live findings were two
  `feed_sidecar_stale` rows:
  - `cleanmx_phishing`: committed feed sidecar under
    `/opt/update-ipsets/lib/entities/feeds/cleanmx_phishing.json` was older
    than `/opt/update-ipsets/web/cleanmx_phishing_asn_caida_prefix2as.json`
  - `cleanmx_viruses`: committed feed sidecar under
    `/opt/update-ipsets/lib/entities/feeds/cleanmx_viruses.json` was older
    than `/opt/update-ipsets/web/cleanmx_viruses_asn_caida_prefix2as.json`
- The admin integrity findings tables could render hundreds of rows without an
  internal scrollbar, pushing the rest of the operator page out of reach.

**Detection:**

- Live admin API checks against the installed development service:
  - `/api/v1/admin/integrity` for feed-output integrity
  - `/api/v1/admin/integrity/entities` for entity-reference integrity
- UI review of the integrity/entity-integrity components.

**Original Outcome (for reference):**

- Regression 3 claimed both integrity surfaces were clean after install and
  repair, and claimed the mtime contract prevented accidental wall-clock drift.

**Why the original Validation missed this:**

- The previous validation checked settled cleanliness after one repair wave,
  but the tests did not cover the precise no-op entity-sidecar case where a
  provider-derived feed payload mtime moves forward while the feed entity
  sidecar JSON remains byte-identical.
- The mtime contract was explicit for public feed artifacts, but too vague for
  entity private/public detail symmetry. It did not state that country/ASN
  private sidecars and matching public JSON payloads must be assigned the same
  dependency-derived logical mtime when touched in one repair cycle.
- Admin UI validation focused on correctness of controls and results, not on
  large-result ergonomics for hundreds of findings.

**Root cause:**

- Facts:
  - Entity feed sidecar freshness is compared against provider-derived public
    geo/ASN artifacts.
  - When regenerated provider-derived inputs changed only mtimes, unchanged
    feed sidecar JSON was touched to the feed processing timestamp instead of
    the newer provider-reference timestamp it now covered.
  - Country/ASN private sidecars and public payloads could be published or
    touched through different paths, giving them different incidental mtimes
    even when they represented the same entity composition.
  - Feed-output integrity already returned `in_progress` while the engine run
    was active, but entity integrity only returned `in_progress` for visible
    entity background tasks. That left a real window after a feed input changed
    and before the coalesced entity refresh finished.
- Conclusion:
  - The pipeline still had an incomplete entity mtime owner model. The code was
    preserving some generated-file timestamps, but entity feed sidecars and
    country/ASN detail artifacts did not consistently derive mtimes from the
    newest logical input/reference across full rebuild, targeted repair,
    staged sidecar, and surgical refresh paths.
  - The admin entity-integrity endpoint also had an incomplete "settled state"
    guard. It was allowed to inspect entity artifacts while the main engine run
    was still mutating the feed inputs those entity artifacts depend on.

**Fix scope:**

- Backend:
  - Stamp unchanged feed entity sidecars to the max of their own logical source
    timestamp and the newest consumed geo/ASN/provider reference timestamp.
  - Write changed feed entity sidecars with the same logical timestamp instead
    of write-time mtime.
  - Derive country/ASN detail mtimes from the latest committed contributing
    feed sidecar timestamp plus row-level change timestamps.
  - Apply the same logical mtime to private country/ASN sidecars and generated
    public country/ASN payloads for full rebuild, targeted repair, staged
    sidecar, and surgical refresh paths.
  - Return `in_progress` from entity integrity while the main engine run is
    active, not only while entity-specific background tasks are active.
- UI:
  - Bound the feed-output integrity findings table with its own scrollbar.
  - Bound grouped entity-integrity findings with their own scrollbar and sticky
    group headers.
- Durable memory:
  - Update integrity/admin UI specs.
  - Update coding/testing/reviewing project skills with the entity-specific
    mtime and large-table lessons.

**Validation planned/completed:**

- Completed before install:
  - `go test ./pkg/engine -run 'TestRebuildEntityArtifactsForFeedsStampsUnchangedSidecarToProviderReference|TestCheckEntityArtifactsIntegrity|TestRefreshEntityArtifactsForFeedUpdatesSurgicallyPatchesAggregates'`
  - `go test ./pkg/engine ./pkg/web ./pkg/config`
  - `go test ./pkg/web -run 'TestEntityIntegrityBusyDuringEngineRunOrEntityBackgroundTask|TestHandleAdminEntityIntegrity'`
  - `make test`
  - `make lint`
  - `make build`
  - `pnpm --dir ui lint`
  - `pnpm --dir ui build`
- Completed live:
  - `./install.sh`
  - live `/healthz` returned `ok`
  - live `/api/v1/admin/integrity` returned `clean`, count `0`
  - live `/api/v1/admin/integrity/entities` returned `clean`, count `0`
  - entity repair/rebuild was triggered through
    `/api/v1/admin/integrity/entities/rebuild` for stale installed artifacts
  - a delayed settle-window check after the final install returned `clean`,
    count `0`, `running:false` for both integrity surfaces
  - live validation also confirmed entity integrity now reports `in_progress`
    instead of settled findings while the main engine run is active

**Same-failure scan:**

- Reviewed remaining entity detail `GeneratedFile` paths and fixed the
  health-transition public-only refresh path to stamp public payloads to the
  private sidecar mtime instead of relying on publish wall-clock timing.
- Reviewed admin entity-integrity busy-state gating and added a test for both
  engine-running and entity-background-task suppression.

**Skills/agent memory updated:**

- `.agents/skills/project-coding/SKILL.md` now includes the entity
  private/public logical-mtime symmetry rule.
- `.agents/skills/project-testing/SKILL.md` now includes entity no-op mtime
  and admin entity-integrity busy-state test requirements.
- `.agents/skills/project-reviewing/SKILL.md` now includes entity mtime
  symmetry, entity busy-state, and bounded admin findings table review gates.

### Regression 5 — 2026-04-29

**Reported by:** user, after regression 4 was installed and live entity
integrity still reported two settled feed-sidecar findings.

**Symptom:**

- Live `/api/v1/admin/integrity/entities` still reported:
  - `cleanmx_phishing`: `lib/entities/feeds/cleanmx_phishing.json` older than
    `web/cleanmx_phishing_asn_caida_prefix2as.json`
  - `cleanmx_viruses`: `lib/entities/feeds/cleanmx_viruses.json` older than
    `web/cleanmx_viruses_asn_caida_prefix2as.json`
- The admin UI formatted Go/RFC3339 entity-integrity times as milliseconds but
  passed them to `absoluteTime()`, which expects Unix seconds; this produced
  year `58295` dates in the findings table.

**Why regression 4 still missed this:**

- Regression 4 added direct entity mtime unit tests, but not a scenario that
  exercised the scheduler-style update path, provider fan-out, queued entity
  refresh settlement, and both integrity checks after each mocked update.
- The missing case is a carried stale feed sidecar plus a later bogon-provider
  update. ASN payloads are regenerated because the ASN writer depends on
  bogons for the `bogon_ips` versus `unknown_ips` split, but entity feed
  sidecars were scoped only to geo/ASN provider updates and ordinary feed
  updates.

**Root cause:**

- `pkg/engine/asn.go` correctly uses
  `targetFeedsForFanOut(..., config.UseASN, config.UseBogons)` because ASN
  comparison payloads consume the bogon union.
- `pkg/engine/entity_feed_sidecar.go` and `pkg/engine/entity_artifacts.go`
  used only `config.UseGeoIP, config.UseASN` for entity sidecar fan-out, even
  though entity sidecar freshness is compared against per-feed ASN payloads.
- Therefore a bogon-provider update could move the ASN reference payload ahead
  while leaving a byte-identical feed entity sidecar untouched.

**Fix scope:**

- Add `pkg/engine/pipeline_integrity_scenario_test.go`, a table-driven
  outside-in scenario harness:
  - each row advances a logical timestamp
  - rows mutate mocked feed/provider inputs with add/remove entries
  - each row runs `runSchedulerStyleOnce`
  - queued entity refresh work is settled synchronously
  - feed entries are verified when relevant
  - both feed-output and entity-artifact integrity must be clean after every row
- Add a regression scenario that backdates a feed entity sidecar to the
  production failure shape, then updates the bogon provider. The test fails
  without the entity bogon dependency edge and passes with it.
- Add `config.UseBogons` to entity feed-sidecar fan-out and targeted/full entity
  artifact repair fan-out.
- Fix admin entity-integrity time parsing to return Unix seconds.
- Update integrity spec and project skills so future integrity bugs become
  scenario rows/cases.

**Validation completed:**

- Completed:
  - temporarily removed the dependency fix and verified
    `TestPipelineIntegrityScenarioBogonUpdateRefreshesEntitySidecars` fails with
    `feed_sidecar_stale`
  - restored the fix and verified the focused test passes
  - `go test ./pkg/engine -count=1`
  - `go test ./pkg/engine ./pkg/web ./pkg/config -count=1`
  - `make test`
  - `make lint`
  - `make build`
  - `pnpm --dir ui lint`
  - `pnpm --dir ui build`
  - `./install.sh`
  - live `/healthz` returned `ok`
  - settled live `/api/v1/admin/integrity` returned `clean`, count `0`,
    `running:false`
  - settled live `/api/v1/admin/integrity/entities` returned `clean`, count
    `0`, `running:false`

## Regression

### Regression 2 — 2026-04-29

**Reported by:** user during post-implementation review.

**Symptom:** The active critical-infrastructure catalog appears imbalanced:
service-specific soft references researched in this SOW, such as GitHub,
Salesforce/Hyperforce, Microsoft 365, Akamai, Apple/software-update-related
coverage, identity/payment/developer platforms, and similar operationally
important services are absent from the `critical_infrastructure` role, while
broad cloud/customer-hosting ranges were shipped as contextual references.

**Detection:** User noticed that the live critical-infrastructure warnings do
not reflect the high-value companies/services discussed during research and
questioned whether broad cloud ranges create noisy false positives.

**Original Outcome (for reference):** SOW-0017 shipped the reference-feed
overlap model, hard DNS/root/AS112 references, Cloudflare/Fastly soft CDN
references, broad contextual cloud/hosting references, public API/UI surfacing,
methodology, and validation.

**Why the original Validation missed this:** Validation proved the mechanism,
artifact freshness, UI ordering, and first baseline references worked, but it
did not include an acceptance gate comparing the shipped reference catalog
against the researched candidate taxonomy and the user's fit-for-purpose goal.
The result allowed an "initial baseline" to close without proving that the
highest-value soft service references were represented before broad contextual
cloud ranges.

**Fix scope:** Re-audit the active critical-infrastructure catalog against the
SOW research and current upstream sources; separate broad cloud/customer-hosting
context from actual critical/service-specific soft references; decide whether
to demote, hide, or re-surface contextual cloud ranges; and implement the
approved catalog correction with methodology/spec/test updates.

**Initial investigation evidence:**

- Active `critical_infrastructure` config currently lives only in
  `configs/firehol/sources/provider_infrastructure/critical_dns.yaml` and
  `configs/firehol/sources/provider_infrastructure/critical_provider_ranges.yaml`.
- Active hard references are DNS-focused only:
  `critical_public_dns_core`, `critical_dns_root_servers`, and `critical_as112`.
- Active soft references are only Cloudflare edge and Fastly edge.
- Active contextual references are broad AWS, GCP, Oracle Cloud,
  DigitalOcean, Linode/Akamai Cloud, and Vultr/Constant ranges. These are
  tagged `cloud_customer_hosting`, which means they include provider services
  and customer workloads, including potentially abusive customer workloads.
- Existing MISP/provider-infrastructure catalog entries for GitHub, Apple,
  Akamai, Cloudflare, Microsoft 365, Gmail, SMTP, Zscaler, Telegram, AWS, GCP,
  Azure, and others do not currently declare `use: [critical_infrastructure]`.
- Live upstream checks on 2026-04-29 confirm service-specific machine sources:
  GitHub Meta API exposes distinct service buckets, Microsoft 365 exposes a
  supported endpoint web service, Salesforce Hyperforce publishes
  `ip-ranges.salesforce.com/ip-ranges.json`, Atlassian and Terraform Cloud
  expose JSON range feeds, Zoom exposes a plain-text range feed, and Okta
  exposes JSON IP ranges.
- The earlier MISP GitHub rejection was overbroad. MISP warninglists are
  explicitly designed to flag potential false positives. The GitHub MISP list
  is useful secondary data, but it flattens categories from `api.github.com/meta`
  into one unit. The correction should prefer GitHub Meta categories directly,
  not dismiss MISP as useless.
- Current code already supports config-driven JSON extraction through
  `json_path`, so many service-specific feeds can be added as YAML-only
  catalog entries rather than hardcoded code.

**User decisions recorded 2026-04-29:**

- Broad cloud/customer-hosting ranges: move to a separate provider-context
  surface, not default critical warnings.
- Missing service feeds: prefer primary official structured sources first; use
  MISP only where no primary exists or as secondary evidence.
- GitHub: split official Meta API semantics; core GitHub service buckets are
  soft, Actions/macOS Actions are contextual, and MISP is secondary.
- Software updates: add Apple broad ranges as contextual only; keep Windows
  Update, Ubuntu, Samsung, and similar DNS/CDN/update ecosystems deferred until
  URL/DNS-resolution feed support exists.

**Additional ASN-context consideration raised 2026-04-29:**

- The deleted `configs/firehol/infrastructure-asns.yaml` preserved a real design
  intuition: some operators have ASNs that are more service-owned than
  customer-hosting-owned, and ASN attribution can help find blast-radius risk
  when exact service feeds are incomplete.
- Verification does not support a general "services ASN vs customer ASN" rule.
  Example: live Google `cloud.json` sampled on 2026-04-29 had Google Cloud
  prefixes originated by multiple ASNs including AS396982, AS15169, AS19527,
  AS139190, and AS139070. AS15169 therefore cannot be treated as purely
  non-customer Google service space.
- ASN context may still be useful as a secondary signal for tightly curated
  operators that do not sell general public cloud/customer hosting, or where a
  provider explicitly says its published IP feed is incomplete. Candidate
  examples require explicit evidence and rationale before inclusion; likely
  starting candidates are Apple AS714/AS6185, Meta AS32934, X/Twitter AS13414,
  and possibly GitHub AS36459 as a fallback for non-exhaustive GitHub Meta
  coverage.
- Hyperscaler/customer-hosting ASNs such as AWS AS16509/AS14618, Microsoft
  AS8075/AS8068, Google AS15169/AS396982/related Cloud ASNs, and broad CDN
  customer-edge ASNs must not return as default critical warnings.

**User decision recorded 2026-04-29:** Add a small, explicitly separate
`critical_asn_context` signal for tightly curated service-owned ASNs only. This
must not resurrect the legacy `infrastructure_asns` warning model and must not
let broad hyperscaler/customer-hosting ASNs become default critical warnings.

**TODO — public methodology page correction:**

- Rewrite the relevant critical-infrastructure methodology page for end users
  and operators interpreting the signal, not for maintainers configuring it.
- The page must explicitly explain the stress in this domain: critical
  infrastructure does not have complete, standardized public IP data; many
  important services are CDN/DNS/front-door backed; many providers publish no
  exact ranges; and broad ASNs/cloud ranges can create false positives.
- The page must explain the balanced decisions taken to make detection useful:
  hard tier for narrow breakage-prone infrastructure, soft tier for
  service-specific authoritative ranges, contextual/provider context for broad
  or tenant-mixed surfaces, and ASN context only as a separate secondary signal.
- The page must discuss the categories of critical sources with concrete
  examples and caveats: public DNS/root/AS112, CDN edges, developer platforms,
  identity/SaaS, payments, software-update infrastructure, cloud/provider
  context, and ASN context.
- The page must include strengths, weaknesses, false-positive/false-negative
  risks, and what is missing/deferred because the available data is too weak or
  needs URL/DNS-resolution feed support.
- Forbidden in this page: config schemas, migration history, code paths,
  artifact filenames, and internal validation mechanics. Those belong in specs
  or operator documentation.

**Implementation completed 2026-04-29:**

- Added `provider_context` as a first-class config role for broad provider and
  customer-hosting ranges. AWS, Google Cloud, Oracle Cloud, DigitalOcean,
  Linode/Akamai Cloud, and Vultr were moved there so they remain visible as
  public context feeds but no longer act as critical-infrastructure warning
  truth.
- Added service-specific critical reference feeds from primary provider sources
  where available: AWS CloudFront, GitHub Meta API, Microsoft 365, Salesforce
  Hyperforce, Atlassian Cloud, HCP Terraform, Zoom, Okta, Auth0, Stripe API,
  Stripe webhooks, Braintree, Mollie, and Apple broad service-network context.
- Kept Akamai as a secondary soft CDN reference through the existing MISP
  warninglist merge, with source quality marked as secondary/low-confidence
  rather than pretending there is a primary Akamai edge feed.
- Split GitHub by official Meta API semantics. Core service buckets are soft;
  Actions, macOS Actions, and Codespaces are contextual because they represent
  tenant-controlled hosted compute and should not inflate the GitHub core
  service surface.
- Added top-level `critical_asn_context` for the separate secondary ASN signal:
  Apple AS714/AS6185, Meta AS32934, X/Twitter AS13414, and GitHub AS36459.
  Validation rejects known broad hyperscaler/customer-hosting ASNs so the old
  ASN-wide warning model cannot silently return.
- Extended the critical provider-set fingerprint to include ASN-provider source
  shape and stable processed content when ASN context is configured, so stale
  aggregate payloads are rejected if ASN attribution changes.
- Added `json_paths` processor support so one configured feed can extract
  several official JSON buckets without hardcoded code paths.
- Moved ASN comparison generation before critical-overlap generation in the
  heavy pipeline so critical aggregates can embed current ASN-context matches
  from staged ASN sidecars.
- Updated the feed page to keep critical reference-feed overlaps sorted by
  criticality before volume, put large tables inside their own scroll area, show
  provider-context feeds as context-only, and display ASN context separately
  from exact reference-feed overlap.
- Rewrote the public critical-infrastructure methodology page for users and
  operators: it now explains the levels, category rationale, stress/lack of
  data, strengths, weaknesses, false positives/negatives, and deferred areas.
- Updated operator docs and specs for `provider_context`, `critical_asn_context`,
  provider-set drift, API payload shape, and public website semantics.
- Install validation on 2026-04-30 exposed a scheduler-provider-set loop:
  while an engine run was already rebuilding critical artifacts, automatic due
  evaluation kept force-enqueueing critical providers because the provider-set
  marker is written only at the end of the run. This caused repeated GitHub and
  Stripe 403/429 fetches during startup. Added a scheduler guard so
  provider-set-drift downloads are not repeatedly enqueued while an engine run
  is active.

**Validation completed for Regression 2:**

- `go test ./pkg/processor -run 'TestCriticalServiceCatalogProcessors|TestJSON|TestFireholCatalogProcessors' -count=1`
- `go test ./pkg/config -run 'TestFireholCatalog|TestCatalog|TestValidateCritical|TestLoadDirectory' -count=1`
- `go test ./pkg/engine -run 'TestCritical|TestWriteCritical|TestExpectedSecondaryFilesSkipsCritical|TestValidateStructuredSecondary|TestCleanupStaleCritical|TestReloadCleansCritical' -count=1`
- `go test ./pkg/scheduler -run 'TestBuildSnapshotCriticalProviderSet|TestAutomaticDueSkipsCriticalProviderSetRefreshWhileEngineRuns|TestScheduled|TestManual' -count=1`
- `make test`
- `make lint`
- `make build`
- `pnpm --dir ui lint`
- `pnpm --dir ui build`
- `./install.sh`
- live `/healthz` returned `ok`
- live `/api/v1/status` reported `source_count: 423`, `merge_count: 16`
- live `/api/v1/sets` exposed new critical and provider-context feeds
- delayed live admin status after reinstall showed no download backlog
- delayed journal check after reinstall showed no repeated GitHub/Stripe
  provider-set download-loop failures

### Branch-Matrix Hardening — 2026-04-29

**Reported by:** user, immediately after the first outside-in scenario harness
was added.

**Request:**

- Add enough outside-in tests to exercise every important pipeline branch so
  integrity bugs are caught before runtime.

**Reality check:**

- Tests cannot prove "100% correct" integrity for all possible upstream,
  filesystem, timing, and operator-action combinations.
- What is feasible and useful is a branch-matrix scenario suite that enumerates
  known pipeline state-transition classes, mutates mocked inputs from the
  outside, runs the normal processing path, settles entity refreshes, and fails
  on any feed-output or entity-artifact integrity finding after every branch.

**Branches covered by the expanded scenario suite:**

- initial publish of ordinary feeds and providers
- initial merge composition after parents exist
- ordinary feed add/remove update
- same-body forced recheck
- same-body unforced check that skips processing
- geolocation provider update and fan-out
- ASN provider update and fan-out
- bogon provider update and ASN/entity-sidecar dependency
- merge `exclude` update and recomposition
- scoped reprocess
- global reprocess from committed local inputs
- critical-infrastructure provider-set marker repair

**Implementation notes:**

- `pkg/engine/pipeline_integrity_scenario_test.go` now has a reusable scenario
  harness with table rows containing logical timestamp deltas, mocked
  feed/provider add/remove mutations, direct or scheduler-style run mode,
  recheck/reprocess controls, and post-run payload assertions.
- Each row runs both integrity surfaces after entity refresh settlement:
  `CheckIntegrityWithOptions` and `CheckEntityArtifactsIntegrity`.
- The suite includes payload assertions for feed body membership, country
  mapping, ASN attribution, critical overlap, and the bogon/unknown split.

**Validation completed:**

- Completed:
  - `go test ./pkg/engine -run 'TestPipelineIntegrityScenario(CoreBranches|BogonUpdateRefreshesEntitySidecars)' -count=1`
  - `go test ./pkg/engine -count=1`
  - `go test ./pkg/engine ./pkg/web ./pkg/config -count=1`
  - `make test`
  - `make lint`
  - `make build`
  - `pnpm --dir ui lint`
  - `pnpm --dir ui build`
  - `./install.sh`
  - live `/healthz` returned `ok`
  - settled live `/api/v1/admin/integrity` returned `clean`, count `0`,
    `running:false`
  - settled live `/api/v1/admin/integrity/entities` returned `clean`, count
    `0`, `running:false`
