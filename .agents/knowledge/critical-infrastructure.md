# Critical Internet Infrastructure Identification for IP Blocklist False-Positive Prevention

---

## §1 Scope & Applicability

This expertise covers the identification, classification, and protection of internet infrastructure whose inclusion in IP reputation feeds causes disproportionate operational damage. The domain boundary extends from publicly routable critical services (recursive DNS resolvers, CDN edges, cloud provider control planes, certificate validation infrastructure, time synchronization, email delivery, identity federation) through the BGP/ASN-level sourcing of their prefixes, to the operational processes for maintaining exclusion lists in threat intelligence aggregation pipelines.

**You need this knowledge when** you aggregate, curate, consume, or automate actions from IP blocklists or reputation feeds — specifically:

- You operate a blocklist aggregation service (like firehol's iplists) and want to proactively detect feeds that poison critical infrastructure IPs.
- You consume third-party IP feeds and want to build local whitelist/exemption policies before an outage teaches you why.
- You are building an automated firewall or SD-WAN policy driven by threat intelligence and need a "never block" safety layer.
- You run a security operations center and want to flag blocklist entries that, if enforced, would disable shared services your organization depends on.
- You are evaluating the quality of IP feeds and need "ground truth" infrastructure that a good feed should never list (or should list only with extreme specificity and documented justification).

**This is the WRONG knowledge when** you are doing targeted threat hunting against a specific actor who may be abusing cloud infrastructure (in that case, blocking a specific AWS IP for a specific abuse scenario may be correct); when you are building allowlists for outbound traffic (inverse problem — different scope); when you need application-layer allowlisting (SNI, URL, domain — this document deals with network-layer IP/ASN).

**Cynefin classification**: Complicated. The set of critical infrastructure actors is knowable and enumerable, but large. There are discoverable rules (e.g., "never block a recursive DNS resolver anycast IP without understanding blast radius"). Expertise is required because the boundaries are not obvious — a /24 that looks like a random hosting range might be Google Public DNS. The domain is not complex (relationships don't shift unpredictably) but it is complicated (many actors, many feeds, many ways to get it wrong).

**Reader prerequisites**: Basic understanding of IP networking (CIDR notation, anycast), BGP and ASN concepts, what an IP blocklist/reputation feed is and how it is consumed (firewall rules, DNS RPZ, BGP blackholes), and familiarity with at least one threat intelligence aggregation workflow.

**Scale dimensions that change applicability**:

- Single-homed small business with one upstream: a simple "don't block these 10 services" list suffices.
- Multi-site enterprise with diverse SaaS dependencies: needs full cloud provider ASN coverage and active monitoring.
- ISP/MSP enforcing blocklists on customer traffic: needs exhaustive coverage plus an escalation process for novel infrastructure, because blast radius is customer-facing.
- Blocklist aggregator (firehol's use case): needs programmatic access to authoritative prefix sources for ~50-100 critical ASNs, automated cross-checking, and proof-of-quality scoring.

---

## §2 Mental Model & Core Concepts

### Core Concepts

**Critical Internet Infrastructure (CII)**: IP prefixes and ASNs operated by organizations whose services are shared across large numbers of dependent networks, such that blocking these prefixes causes cascading, disproportionate damage. CII is not a formal IETF term; it is a practitioner category defined by blast radius and dependency count.

**Blast Radius**: The scope of services degraded or destroyed when a prefix is blocked. Blast radius is the primary ranking axis for criticality. Blocking 8.8.8.8 has blast radius proportional to "every internal resolver that forwards to it" — potentially all DNS for an organization. Blocking a random VPS IP has blast radius of one service or tenant.

**Anycast Infrastructure**: Many critical services (DNS resolvers, NTP, root DNS) use anycast — the same IP announced from dozens to hundreds of locations. Blocking one anycast IP degrades performance globally for all users, not just one region. Anycast IPs are disproportionately critical compared to unicast IPs.

**Shared Infrastructure / Co-tenancy Problem**: Cloud providers and CDNs host legitimate and abusive services on the same IP ranges. A blocklist that includes an entire /24 for one abusive tenant blocks thousands of legitimate tenants. This is the fundamental tension driving false positives.

**ASN-level vs Prefix-level vs IP-level Blocking**: ASN blocking (blocking all prefixes originated by an ASN) is extremely dangerous for cloud/CDN/hosting ASNs. Prefix-level blocking is more targeted but still dangerous for anycast and shared infrastructure. IP-level blocking is the minimum-safe granularity for shared infrastructure, and even then carries risk for services behind load balancers.

**Feed Quality Scoring**: Measuring IP feeds against a known "should never block" set to detect overblocking. A feed that blocks 8.8.8.8 has a quality problem. A feed that blocks 0.1% of AWS's space may have a specificity problem or may be correctly targeting specific abuse — context matters.

**Whitelist / Exclusion Set**: A curated set of IPs, prefixes, or ASNs that are excluded from blocklist enforcement. In this domain, the exclusion set is the primary artifact — it represents the practitioner's model of what is too critical to block without extreme justification.

**Tiered Whitelist Model**: Three tiers reflecting graduated criticality:

- **Hard whitelist** (Tier 1): Infrastructure that should never appear in any blocklist under any circumstances. Overlap = immediate feed quality flag.
- **Soft whitelist** (Tier 2): Infrastructure that should almost never be blocked and requires specific documented justification. Overlap = alert + human review.
- **Contextual whitelist** (Tier 3): Infrastructure whose criticality depends on the consumer's architecture. Overlap = flag for consumer-side evaluation.

### Forces

- **Specificity vs Coverage**: Blocklist maintainers want to catch all abuse (coverage); critical infrastructure operators want zero false positives (specificity). These directly conflict for shared infrastructure.
- **Freshness vs Stability**: Infrastructure prefixes change (cloud providers add ranges). Exclusion sets must be refreshed, but automated refresh without verification can import errors.
- **Automation vs Judgment**: Automated blocklist enforcement is fast; judgment about whether a specific listing is justified requires context. Blocking Google DNS automatically is almost always wrong; blocking a specific AWS IP that is actively attacking you may be justified despite it being "cloud infrastructure."
- **Feed Aggregation Poisoning**: Aggregators combine many feeds. A single low-quality feed can poison the aggregate. The aggregator must detect and mitigate this.

### Counterintuitive Truths

- **"Well-managed" does not mean "never abused."** Google, AWS, and Cloudflare are extremely well-managed, but attackers use their services too. A blocklist that lists an AWS IP is not necessarily wrong — it depends on whether the listing is for a specific abusive resource or a blanket inclusion.
- **The most dangerous false positives are the ones you don't notice immediately.** Blocking a CDN IP degrades performance subtly before it causes visible failures. Users experience slower page loads, not error pages.
- **ASN reputation is nearly useless for cloud/hosting ASNs.** The ASN tells you who operates the network, not what's running on it. AS16509 (AWS) hosts everything from Netflix's control plane to cryptocurrency scams.
- **Smaller services can have outsized blast radius.** Quad9 (AS19281) is a small ASN, but if your DNS resolvers forward to 9.9.9.9, blocking it is catastrophic for you specifically.
- **The IP range data itself has a half-life.** Cloud providers add and remove ranges regularly. A whitelist built from a static snapshot degrades within weeks.
- **"Source IPs in honeypot logs" is not "malicious IPs."** DNS resolution traffic will pollute any unfiltered feed. Open resolvers appearing in honeypot logs are being used as reflectors, not as attackers.

### Vocabulary Glossary

- **ASN (Autonomous System Number)**: A globally unique identifier for a network that operates its own routing policy. Used in BGP to identify who announces which IP prefixes.
- **BGP (Border Gateway Protocol)**: The routing protocol that glues the internet together. BGP announcements map ASNs to IP prefixes.
- **CIDR**: Classless Inter-Domain Routing notation (e.g., 8.8.8.0/24). The standard way to express IP prefix ranges.
- **Prefix**: A contiguous block of IP addresses expressed in CIDR notation. An IP's "home" in the routing system.
- **Anycast**: A routing technique where the same IP address is announced from multiple locations. Traffic is routed to the "nearest" instance. Common for DNS, NTP, and CDN edges.
- **RPZ (Response Policy Zone)**: A DNS-level mechanism for blocking domains/IPs at the resolver level. One of the consumption paths for IP feeds.
- **RDAP (Registration Data Access Protocol)**: The successor to WHOIS for querying IP/ASN registration data from RIRs.
- **RIR**: Regional Internet Registry (ARIN, RIPE NCC, APNIC, LACNIC, AFRINIC).
- **IRR (Internet Routing Registry)**: A database where networks can register their routing intentions. Used for prefix validation.
- **ROA (Route Origin Authorization)**: An RPKI cryptographically signed object that authorizes an ASN to announce a prefix. Can be used to validate prefix-to-ASN mappings.
- **OCSP (Online Certificate Status Protocol)**: Certificate revocation checking. Now deprecated by Let's Encrypt (shutdown August 2025); replaced by CRL at `lencr.org`.
- **CRL (Certificate Revocation List)**: Certificate revocation list distribution point. Currently used by Let's Encrypt.

### Mental Models / Analogies

- **Think of critical infrastructure like utility lines.** You don't cut the water main because one apartment has a leak. You fix the apartment. Blocklisting a /16 of AWS to stop one abusive VM is cutting the water main.
- **Think of the exclusion set as a "minimum viable internet."** If everything on the exclusion set stopped working simultaneously, how broken would the internet be for a typical organization? That's the test.
- **Think of feed quality scoring like academic peer review.** You evaluate a feed not by what it catches, but by what it gets wrong against known-good infrastructure.

### Invariants

- Any anycast IP used by public recursive DNS resolvers should be treated as critical infrastructure. Blocking these IPs breaks DNS for every downstream user.
- Cloud provider IP ranges expand over time and never contract significantly. Exclusion sets must be refreshed on a cadence.
- No ASN-based rule is safe for multi-tenant hosting/cloud ASNs. Specificity must be at prefix or IP level.
- The set of infrastructure categories that qualify as "critical" is relatively stable (DNS, CDN, cloud control plane, CA, NTP, update, email, identity). The specific IPs within each category change.
- Root DNS servers cannot be safely blocked without breaking global name resolution.
- Certificate revocation infrastructure must be reachable for TLS to function in most default configurations.
- Time synchronization is required for cryptographic validation (TLS certificates have validity windows).

---

## §3 Recognition Cues

**Cue 1: Blocklist entry matches a known public service IP**

- **Signature**: An IP or prefix in the blocklist exactly matches or is contained within a range associated with a public anycast service (e.g., 8.8.8.0/24, 1.1.1.0/24, 9.9.9.0/24, 208.67.222.0/24).
- **Implication**: Almost certainly a false positive. These ranges are dedicated to public DNS resolution and are not used to host arbitrary content.
- **Discriminator from "legitimate DNS abuse"**: The rare case where a DNS resolver IP appears in abuse reports is almost always due to the resolver being used as an open resolver in a DNS amplification DDoS. Even then, blocking the IP causes more harm than the amplification. Rate-limiting is the correct response, not blocklisting.

**Cue 2: Blocklist entry covers a large aggregate prefix belonging to a cloud/CDN ASN**

- **Signature**: A /16, /12, or /8 entry that matches a cloud provider's announced space (e.g., any prefix within 3.0.0.0/8 which is heavily used by Amazon).
- **Implication**: The feed is operating at insufficient specificity. Large aggregate blocks in hosting space indicate either laziness or a feed designed for a very different threat model.
- **Discriminator**: Some feeds intentionally block all hosting/cloud IP ranges as a policy ("no legitimate user traffic comes from AWS"). This is a legitimate policy choice for some use cases (e.g., blocking SSH brute force from all clouds) but is not appropriate for general-purpose security feeds.

**Cue 3: Feed contains entries for IPs that resolve to well-known domains**

- **Signature**: Reverse DNS or forward DNS for the blocked IP maps to infrastructure like `dns.google`, `cloudflare.com`, `*.azure.com`, `*.amazonaws.com`, `*.akamaiedge.net`.
- **Implication**: The infrastructure is verified shared/managed. Blocklisting requires specific justification.
- **Discriminator**: Not all `*.amazonaws.com` IPs are critical — S3 and CloudFront are critical; EC2 instances may be individually abusive. The domain pattern indicates shared infrastructure, which raises the bar for justification but doesn't automatically mean "do not block."

**Cue 4: Multiple feeds independently list the same "critical" IP**

- **Signature**: The IP appears in several independent feeds simultaneously.
- **Implication**: Either (a) the IP is genuinely involved in abuse despite being nominally "critical infrastructure," or (b) a correlated observation error (multiple feeds saw the same DNS amplification reflection and interpreted it as malicious originating traffic). This requires human investigation.
- **Discriminator**: If the feeds list the IP for different reasons or different timestamps, it's more likely genuine. If all feeds reference the same incident window and same abuse type, it's likely correlated observation.

**Cue 5: Feed lists an IP in a prefix covered by an RPKI ROA for a known infrastructure ASN**

- **Signature**: The IP falls within a prefix that has a valid RPKI ROA authorizing a known infrastructure ASN (AS13335, AS15169, AS8075, AS20940, etc.).
- **Implication**: The prefix is cryptographically validated as belonging to that operator. Treat with the same caution as a confirmed operator-announced range.
- **Discriminator**: RPKI does not tell you what service runs on the IP — only who routes it. This is infrastructure confirmation, not service confirmation.

**Cue 6: Feed is generated from honeypot/log analysis without ASN filtering**

- **Signature**: Feed description mentions "honeypot," "Cowrie," "Dionaea," or similar without mentioning ASN-based exclusion.
- **Implication**: The feed will contain CDN edge nodes that spider honeypots, cloud provider health checks, and DNS resolver lookups — all legitimate traffic to the honeypot that looks like "source IPs attacking."
- **Discriminator**: Check if the feed maintainer explicitly documents ASN-based filtering. Absence of such documentation is a warning sign.

**Cue 7: Feed's IPs appear in multiple CDNs simultaneously**

- **Signature**: BGP looking glass shows multiple major CDNs announcing the same prefix range.
- **Implication**: It's infrastructure IP space shared across providers. Blocking it has multi-CDN collateral damage.
- **Discriminator**: Use `bgpview.io` or `whatsmydns.net` to check which ASNs announce a prefix.

**Expert Expectations (normal conditions)**:

- A well-curated blocklist should have near-zero overlap with public DNS resolver IPs, root DNS server IPs, NTP pool IPs, and major CDN edge IPs. Zero overlap is achievable and should be the target.
- Some overlap with cloud provider ranges (AWS, Azure, GCP) is expected and often legitimate — but it should be at individual IP granularity, not prefix level.
- A feed that has never listed any cloud IP is probably not looking at cloud-hosted threats, which means it has a coverage gap in the opposite direction.
- The total "critical infrastructure exclusion set" for a general-purpose aggregator should cover approximately 50-100 ASNs and result in a few thousand prefixes. This is a manageable set.

**Early-Warning Cues**:

- A feed that suddenly adds large numbers of entries in a single update is likely doing a bulk inclusion of a hosting/cloud range. Check immediately.
- A feed that lists IPs in the 1.0.0.0/24, 8.8.4.0/24, 8.8.8.0/24, 9.9.9.0/24 ranges is almost certainly including DNS infrastructure. These should trigger immediate review.
- A feed whose description says it covers "data centers" or "hosting" or "proxy" without further specificity is high-risk for cloud/CDN false positives.
- A new feed that has not been validated against a critical infrastructure exclusion set should be assumed to contain false positives until proven otherwise.
- A feed that lists IPs with "abuse-high" thresholding (e.g., >1000 abuse reports) will block shared infrastructure that accumulates abuse reports from compromised end-users, even though the infrastructure itself is clean.

---

## §4 Signals, Metrics & Success Criteria

### Primary KPIs

**KPI 1: Critical Infrastructure Overlap Count**

- **What it measures**: Number of IPs/prefixes in the aggregated blocklist that overlap with the critical infrastructure exclusion set.
- **How to measure**: Intersect blocklist entries (IPs or prefixes) with exclusion set entries; count unique overlaps.
- **Unit**: Count of overlapping IPs or prefixes.
- **Healthy range**: 0 for dedicated infrastructure IPs (DNS resolvers, root servers, NTP stratum-1). Near-zero for CDN edge IPs. Small and justified for cloud provider IPs.
- **Unhealthy range**: Any overlap with DNS resolvers/root servers. More than a handful of overlaps with CDN/cloud ranges without specific abuse documentation.
- **Reaction speed**: Coincident — detectable as soon as feeds are aggregated.

**KPI 2: Per-Feed False Positive Rate Against Critical Infrastructure**

- **What it measures**: For each individual feed in the aggregator, what fraction of its entries hit the exclusion set.
- **How to measure**: Per feed, compute (exclusion set overlap count) / (total entries in feed).
- **Unit**: Ratio (0.0 to 1.0).
- **Healthy range**: 0.0 for feeds that claim to be curated. Near 0.0 for all feeds.
- **Unhealthy range**: > 0.001 for a "curated" feed suggests systematic overblocking. > 0.01 for any feed is alarming.
- **Reaction speed**: Coincident.

**KPI 3: Exclusion Set Freshness Age**

- **What it measures**: Time since the critical infrastructure exclusion set sources were last refreshed from authoritative feeds.
- **How to measure**: Timestamp of last successful fetch of each authoritative source compared to current time.
- **Unit**: Days.
- **Healthy range**: < 7 days for cloud providers (they add ranges frequently). < 30 days for stable infrastructure (DNS resolvers, root servers).
- **Unhealthy range**: > 14 days for cloud providers. > 90 days for stable infrastructure.
- **Reaction speed**: Lagging — staleness doesn't cause immediate failures but creates a growing window for false negatives in the exclusion set.

### Secondary Metrics

**Feed Quality Score (per feed)**: Composite score combining false positive rate, specificity (IP vs prefix vs ASN granularity), update frequency, and historical reliability. Used for ranking feeds in the aggregator.

**Mean Time to Exclusion (MTTE)**: Hours from critical infrastructure detection in a feed to exclusion set update. Target: < 4 hours.

**Dependency Coverage**: Percentage of the organization's actual AS dependencies with exclusion rules. Target: > 95%.

### Success Criteria

- **Zero unintended blocks** of public recursive DNS resolvers (8.8.8.0/24, 8.8.4.0/24, 1.1.1.0/24, 1.0.0.0/24, 9.9.9.0/24, 208.67.222.0/24, 208.67.220.0/24).
- **Zero unintended blocks** of DNS root server anycast prefixes.
- **Zero unintended blocks** of NTP services the operator actually depends on.
  Do not publish `pool.ntp.org` as a static global IP feed; the pool is
  DNS-distributed volunteer infrastructure and needs operator-local DNS/TTL
  handling if it is protected by IP policy.
- **Zero unintended blocks** of major CDN edge IPs (Cloudflare, Akamai, Fastly front-end ranges).
- **Documented justification** for any block of cloud provider IP (AWS, Azure, GCP, Oracle) — the block may be valid but must have supporting evidence.
- **Automated alerting** when any new feed entry matches the exclusion set.
- **Exclusion set refreshed** within 24 hours of any infrastructure provider publishing new ranges.

### Failure Criteria

- Any DNS resolver IP appears in the enforced blocklist without a documented exception process.
- A cloud provider's control-plane or metadata-service range is blocked (this can take down VMs and services within that cloud).
- A certificate authority OCSP/CRL IP is blocked (this breaks certificate validation chain).
- A Windows Update / Apple Update / Linux repository IP is blocked at scale (this breaks security patching).

### Correlation Warnings

- Feed size and false positive rate are NOT correlated. A large feed is not necessarily worse; it may be more comprehensive. A small "curated" feed can be just as destructive if it includes one critical IP.
- Feed update frequency and quality are NOT correlated. A feed that updates daily is not necessarily better than one that updates weekly. The content of updates matters.
- Multiple feeds listing the same cloud IP is NOT necessarily evidence of accuracy. It may be correlated observation of reflected/proxied traffic. Each listing needs independent verification.
- High volume AND low ASN diversity indicates the feed is blocking entire providers rather than specific malicious IPs.

### Sampling / Aggregation Caveats

- When counting overlaps, distinguish between "this specific IP is listed" and "this IP is within a listed aggregate prefix." The latter is far more common and more dangerous.
- When scoring feeds, weight by the criticality of the infrastructure hit. One DNS resolver IP hit should count more heavily than ten EC2 instance IPs.
- When measuring freshness, track each authoritative source independently. AWS may publish daily while Quad9's ranges may be stable for months.
- Percentiles are meaningless for critical infrastructure: it's binary — blocked or not. Use coverage metrics, not latency-style percentiles.

---

## §5 Actors, Roles & Incentives

### Role 1: Blocklist Aggregator Operator

- **Primary goal**: Provide accurate, useful aggregated IP feeds with quality indicators.
- **Success definition**: Feeds are consumed without causing operational damage; quality scores are trusted by the community.
- **Failure definition**: A consumer blocks critical infrastructure based on an aggregated feed, causing public-facing outage.
- **Distorting pressures**: Volume of feeds to process (hundreds of feeds, each with different formats and update cadences); desire to include many feeds for coverage; limited time for manual curation; community pressure to add feeds quickly vs pressure to ensure quality.
- **Common blind spots**: Assuming feed maintainers have already filtered critical infrastructure (many have not); underestimating the blast radius of niche services (NTP, OCSP).
- **How they think about the domain**: Primarily in terms of data engineering — ingestion, normalization, deduplication, aggregation. The "what should not be blocked" perspective requires active effort to maintain.
- **Most common failure mode**: Ingesting a new feed without validating it against the critical infrastructure exclusion set first, then a consumer auto-enforces the aggregate and breaks their own DNS.

### Role 2: Individual Feed Maintainer

- **Primary goal**: Produce a feed that identifies their specific threat category (spam, SSH brute force, botnet C2, etc.).
- **Success definition**: High true positive rate for their category. Community recognition. Feed is widely used.
- **Failure definition**: Feed is ignored due to poor quality or reputation.
- **Distorting pressures**: Speed (getting feeds published quickly); coverage pressure ("my feed should catch everything"); lack of resources for deduplication or whitelisting; may not be aware of downstream enforcement implications.
- **Common blind spots**: Many feed maintainers operate in a specific threat niche and do not think about infrastructure blast radius at all. They see an IP attacking their SSH server and list it, without checking that it's an AWS IP hosting 1000 other services.
- **How they think about the domain**: In terms of their threat category. A spam feed maintainer thinks about spam sources. An SSH brute-force feed maintainer thinks about attack sources. Neither thinks about "what if someone blocks all traffic to this IP."
- **Most common failure mode**: Listing shared infrastructure IPs without granularity — e.g., listing an entire /24 of a hosting provider because one IP in that range was seen scanning.

### Role 3: Feed Consumer / Security Operator

- **Primary goal**: Protect their network using threat intelligence feeds.
- **Success definition**: Blocked threats with minimal false positives affecting business operations.
- **Failure definition**: Operational outage caused by blocking legitimate critical infrastructure.
- **Distorting pressures**: Pressure to automate (can't manually review thousands of blocklist entries); pressure to block aggressively ("why didn't we block this known-bad IP?"); compliance requirements; on-call fatigue leading to overly permissive OR overly aggressive policies.
- **Common blind spots**: Assuming the feed aggregator has already filtered dangerous IPs; not understanding that their specific infrastructure dependencies may differ from the aggregator's assumptions.
- **How they think about the domain**: In terms of risk management — "what's the cost of missing a threat vs the cost of a false positive." This is a fundamentally different frame than the feed maintainer or aggregator.
- **Most common failure mode**: Blindly auto-enforcing aggregated feeds without a local exclusion set customized to their specific dependencies.

### Role 4: Infrastructure Provider (Google, Cloudflare, AWS, etc.)

- **Primary goal**: Operate reliable services. Minimize abuse originating from their infrastructure.
- **Success definition**: Services are reachable; abuse rates are manageable; their IP ranges are not widely blocklisted.
- **Failure definition**: Their IP ranges appear on major blocklists, causing their customers' services to be unreachable from portions of the internet.
- **Distorting pressures**: Scale (managing millions of IPs); abuse-report volume; desire to publish their ranges for transparency vs fear of exposing internal topology; responsibility to handle abuse vs resource constraints.
- **Common blind spots**: May not publish IP ranges in easily consumable formats; may not understand how blocklist aggregators work; may not realize that their abuse-handling latency creates a window where blocklists are the only "solution" consumers see.
- **How they think about the domain**: In terms of abuse handling and service reliability. They see blocklists as a somewhat hostile force — a sign that their abuse handling is insufficient.
- **Most common failure mode**: Not publishing authoritative IP range lists in machine-readable formats, forcing aggregators to use less reliable sources.

### Inter-Role Dynamics

**Aggregator ↔ Feed Maintainer**: The aggregator depends on feeds for content. The maintainer depends on the aggregator for distribution. The aggregator has power (can exclude low-quality feeds) but also liability. Tension: aggregator wants quality guarantees; maintainer wants broad distribution without extra work.

**Aggregator ↔ Consumer**: Consumer trusts the aggregator's quality filtering. When that trust is violated (false positive causes outage), the consumer may abandon the aggregator entirely. This is the highest-stakes relationship.

**Consumer ↔ Infrastructure Provider**: Consumer depends on the provider's services. Provider depends on the consumer not blocking their IPs. They rarely communicate directly, creating a coordination problem.

**Feed Maintainer ↔ Infrastructure Provider**: Feed maintainer lists IPs that may belong to the infrastructure provider. Provider may or may not be aware. Provider's abuse team may be slow to respond, motivating the feed maintainer to list first and ask questions later.

**Key friction point**: No party has complete information. The feed maintainer doesn't know the full blast radius of the IPs they list. The consumer doesn't know the full dependency chain of their infrastructure. The aggregator is in the middle, trying to add value through quality filtering.

---

## §6 Patterns & Anti-Patterns

### Positive Patterns

**Pattern: Whitelist-First Aggregation**

- **Context**: Any blocklist aggregation pipeline.
- **Forces**: Speed of adding new feeds vs safety of validating them; completeness vs accuracy.
- **Solution**: Before any feed enters the aggregated output, cross-reference every entry against a maintained critical infrastructure exclusion set. Entries that overlap are flagged, not silently included. Flagged entries go to a review queue, not the main feed.
- **Consequences**: Positive — prevents catastrophic false positives. Negative — adds latency to feed ingestion; requires exclusion set maintenance; may delay legitimate blocks on abused cloud IPs.
- **Related patterns**: Feed Quality Scoring, Layered Whitelist.

**Pattern: Layered Whitelist (Hard / Soft / Contextual)**

- **Context**: Building the exclusion set itself.
- **Forces**: Some infrastructure should NEVER be blocked (DNS resolvers); some should almost never be blocked but has edge cases (CDN edges); some depends entirely on the consumer's architecture (cloud provider IPs).
- **Solution**: Three tiers:
  - **Hard whitelist**: IPs that should never appear in any blocklist under any circumstances (public recursive DNS resolvers, root DNS servers, NTP stratum-1/2). Overlap = immediate feed quality flag.
  - **Soft whitelist**: IPs that should almost never be blocked and require specific documented justification (CDN edges, certificate validation, OS update servers). Overlap = alert + human review.
  - **Contextual whitelist**: IPs whose criticality depends on the consumer's architecture (cloud provider ranges, SaaS platforms). Overlap = flag for consumer-side evaluation.
- **Consequences**: Positive — appropriate response per risk level. Negative — more complex implementation; requires maintaining three lists with different policies.
- **Related patterns**: Whitelist-First Aggregation.

**Pattern: Authoritative Source Refresh Pipeline**

- **Context**: Maintaining the exclusion set over time.
- **Forces**: Infrastructure changes (cloud providers add ranges); staleness creates blind spots; manual maintenance doesn't scale.
- **Solution**: Automated pipeline that fetches authoritative prefix sources on a schedule (daily for cloud providers, weekly for stable infrastructure), parses the output, diffs against current exclusion set, and updates. Changes are logged and reviewable.
- **Consequences**: Positive — exclusion set stays current. Negative — requires engineering investment; must handle source format changes, outages, and parse errors gracefully.
- **Related patterns**: RIR Fallback.

**Pattern: RIR Fallback**

- **Context**: An infrastructure operator does not publish their own IP range list.
- **Forces**: Authoritative operator-published lists are preferred but not always available; RIR databases are always available but may include allocated-but-not-routed space.
- **Solution**: Use RIR WHOIS/RDAP to enumerate all prefixes allocated to the operator's ASN. Where the operator publishes their own list, prefer it. Where they don't, fall back to RIR data. Where RIR data is stale or imprecise, cross-reference with BGP route views.
- **Consequences**: Positive — complete coverage even for operators who don't self-publish. Negative — RIR data may include unadvertised prefixes or lag behind actual routing.
- **Related patterns**: Authoritative Source Refresh Pipeline.

**Pattern: Per-Feed Quality Scorecard**

- **Context**: Evaluating and ranking feeds.
- **Forces**: Many feeds to choose from; varying quality; need objective comparison.
- **Solution**: For each feed, compute: overlap count with hard whitelist (zero-tolerance metric), overlap count with soft whitelist, overlap count with contextual whitelist, update frequency, IP-level vs prefix-level specificity, and historical trend. Publish the scorecard alongside the feed.
- **Consequences**: Positive — drives feed quality through transparency; helps consumers choose feeds. Negative — may disincentivize feeds that legitimately need to list some cloud IPs; scoring methodology becomes a point of contention.
- **Related patterns**: Whitelist-First Aggregation.

**Pattern: Anycast IP Special Handling**

- **Context**: IPs that are anycast (same IP, many global locations).
- **Forces**: Blocking an anycast IP affects all locations, not just one; anycast IPs are disproportionately likely to be critical infrastructure.
- **Solution**: Maintain a specific list of known anycast service IPs. Treat them as hard-whitelist regardless of what other signals suggest. The blast radius is inherently global.
- **Consequences**: Positive — prevents the most catastrophic false positives. Negative — requires identifying which IPs are anycast (not always obvious from BGP alone).
- **Related patterns**: Layered Whitelist.

### Anti-Patterns

**Anti-Pattern: "Trust the Feed"**

- **How it arises**: The aggregator assumes each feed maintainer has already filtered critical infrastructure. "They're the expert in their threat category; they wouldn't list a DNS resolver."
- **Damage caused**: Feed maintainers often have no idea what IPs they're listing in terms of infrastructure role. They see an IP attacking their server and list it.
- **Recognition signs**: No exclusion set cross-checking in the aggregation pipeline. New feeds are included without validation.
- **Escape path**: Implement whitelist-first aggregation as a mandatory step.

**Anti-Pattern: Static Whitelist**

- **How it arises**: Someone builds an exclusion set once, manually, based on current knowledge, and never updates it.
- **Damage caused**: As cloud providers add ranges, new critical infrastructure comes online, and services change providers, the exclusion set becomes increasingly incomplete. False positives slip through.
- **Recognition signs**: Exclusion set has a "last updated" date more than 30 days in the past. No automated refresh pipeline.
- **Escape path**: Build an authoritative source refresh pipeline. Schedule regular updates.

**Anti-Pattern: Prefix-Blind Listing**

- **How it arises**: A feed lists a /24 or /16 that happens to contain the abusive IP, rather than listing just the individual IP.
- **Damage caused**: Every other IP in that prefix is also blocked. For shared infrastructure, this can affect thousands of services.
- **Recognition signs**: Feed entries are predominantly /24 or larger. Feed description does not mention IP-level specificity.
- **Escape path**: Validate feed granularity. Deaggregate or reject prefix-level entries for shared infrastructure ranges. Score feeds lower for prefix-level listing.

**Anti-Pattern: Whack-a-Mole Whitelisting**

- **How it arises**: Instead of proactively building a comprehensive exclusion set, the operator adds IPs to the exclusion set only after a false positive incident occurs.
- **Damage caused**: Reactive whitelisting means every critical IP is learned through an outage. Some outages may not be attributed to the blocklist (subtle performance degradation, intermittent failures).
- **Recognition signs**: Exclusion set contains only IPs that have already caused incidents. No proactive enumeration of infrastructure ASNs.
- **Escape path**: Proactively enumerate all critical infrastructure ASNs and their prefixes. Use the categories in this document as a starting checklist.

**Anti-Pattern: Honeypot-Derived Without Curation**

- **How it arises**: Feed generated from honeypots (Cowrie, Dionaea, Mwchids) without ASN filtering.
- **Damage caused**: The feed contains IPs of CDN crawlers, cloud provider health checks, and DNS resolver lookups — all legitimate traffic to the honeypot that looks like "source IPs attacking."
- **Recognition signs**: Feed description mentions "honeypot" without ASN filtering documentation. Contains IPs from AS13335, AS15169, AS16509 without justification.
- **Escape path**: Implement ASN-based filtering before IP-level analysis. Exclude known infrastructure ASNs regardless of observed behavior.

**Anti-Pattern: "Abuse-High IPs" Thresholding**

- **How it arises**: Setting a block threshold at "IPs with >1000 abuse reports" without distinguishing infrastructure from compromised hosts.
- **Damage caused**: Shared infrastructure that accumulates abuse reports from compromised end-users gets blocked, even though the infrastructure itself is clean.
- **Recognition signs**: Feed contains large blocks from major cloud/CDN providers with no per-IP justification.
- **Escape path**: Separate infrastructure reputation from end-user reputation. Use ASN-based attribution, not abuse-count thresholds.

---

## §7 Tools & Capabilities

### Capability 1: BGP/ASN-to-Prefix Enumeration

- **What it does**: Given an ASN, returns all IP prefixes currently announced by that ASN.
- **Why needed**: This is the foundational capability for building the exclusion set. You must be able to go from "Cloudflare is AS13335" to "here are all the prefixes Cloudflare announces."
- **Typical I/O**: Input: ASN (e.g., AS13335). Output: List of CIDR prefixes.
- **Example tools**:
  - **Hurricane Electric BGP Toolkit (bgp.he.net)**: Web-based. Free. Strengths: Comprehensive historical data, good UI for manual lookups. Limitations: Not easily scriptable; no official API. Currency: Widely used, actively maintained.
  - **RIPE RIS (ris.ripe.net)**: API-based. Free. Strengths: Authoritative data from a major RIR; excellent API; historical data; RISwhois programmatic interface. Limitations: Data reflects what RIPE's collectors see, may have gaps for some regions. Currency: Widely used by practitioners.
  - **RouteViews (routeviews.org)**: Raw MRT/RIB data. Free. Strengths: Independent data source, operated by University of Oregon, long historical record. Limitations: Raw data format requires parsing; not a friendly API. Currency: Long-standing, stable.
  - **bgp.tools**: Web + API. Freemium. Strengths: Clean interface, real-time data, good API. Limitations: Rate-limited free tier. Currency: Actively developed, growing in popularity.
  - **CYMRU Bogon/ASN lookup (team-cymru.com)**: DNS-based. Free. Strengths: Extremely lightweight, scriptable via DNS queries, fast. Limitations: Limited to prefix-to-ASN mapping. Currency: Long-standing, reliable.
  - **IPinfo.io**: API. Freemium/commercial. Strengths: Clean API, ASN-to-prefix mapping, good documentation. Limitations: Paid for high-volume queries. Currency: Widely used.

### Capability 2: Authoritative Provider IP Range Fetching

- **What it does**: Fetches machine-readable IP range lists published by the infrastructure operators themselves.
- **Why needed**: Operator-published lists are the most accurate and current source for their IP space.
- **Typical I/O**: Input: Provider name and/or published URL. Output: List of CIDR prefixes in structured format (JSON, plain text).
- **Example sources** (ordered by authoritativeness):

| Provider | URL | Format | Update Cadence |
|----------|-----|--------|---------------|
| AWS | `https://ip-ranges.amazonaws.com/ip-ranges.json` | JSON | ~weekly |
| Azure | `https://www.microsoft.com/en-us/download/details.aspx?id=56519` | JSON | Monthly |
| Cloudflare | `https://www.cloudflare.com/ips-v4` + `/ips-v6` | Plain text | Irregular |
| Fastly | `https://api.fastly.com/public-ip-list` | JSON | Event-driven (changes pre-announced) |
| GCP | `https://www.gstatic.com/ipranges/cloud.json` | JSON | ~Irregular |
| Oracle Cloud | `https://docs.oracle.com/iaas/tools/public_ip_ranges.json` | JSON | ~Monthly |
| Google Public DNS | `https://developers.google.com/speed/public-dns/docs/using` | Documentation | Stable |
| Quad9 | `https://quad9.net/service/service-addresses-and-features` | Documentation | Stable |
| OpenDNS | `https://support.opendns.com/hc/en-us/articles/227986727` | Documentation | Stable |
| Verisign DNS | `https://www.verisign.com/en_US/security-services/public-dns/` | Documentation | Stable |
| Salesforce | `https://help.salesforce.com/s/articleView?id=000384438` | Documentation (JS-rendered) | Subject to change |
| GitHub | `https://api.github.com/meta` | JSON | Dynamic |
| Apple | `https://support.apple.com/en-us/101555` | Documentation | Stable but broad; treat `17.0.0.0/8` and documented IPv6 blocks as soft/contextual, not hard |
| Akamai | Customer portal (Luna/Control Center) | GUI export | Dynamic |
| Alibaba Cloud | No official endpoint. Prefer project-owned BGP/RDAP enumeration if needed; community feeds such as `sourcecidr.com` or `disposable/cloud-ip-ranges` are secondary only and require explicit license/quality annotation | JSON/BGP | Community-maintained / not authoritative |
| Tencent Cloud | No official endpoint. Use `https://bgp.tools/as/45090` or `https://ipinfo.io/AS45090` | BGP data | Real-time |
| Huawei Cloud | No official endpoint. Use `https://ipinfo.io/AS55990` or `https://whois.ipip.net/AS55990` | BGP data | Real-time |
| OVH | `https://www.ovhcloud.com/en-gb/network/transparency/` | Documentation | Regular |
| DigitalOcean | `https://www.digitalocean.com/docs/platform/` | Documentation | Regular |

### Capability 3: RIR WHOIS/RDAP Querying

- **What it does**: Queries the Regional Internet Registries for IP allocation and ASN registration data.
- **Why needed**: Fallback when operators don't publish their own lists; validation of operator-published data; finding new allocations.
- **Typical I/O**: Input: ASN or IP or prefix. Output: Registration data including allocated prefixes, org name, contacts.
- **Example tools**:
  - **ARIN WHOIS/RDAP (rdap.arin.net)**: North America. REST API.
  - **RIPE NCC RDAP (rdap.db.ripe.net)**: Europe/Middle East/Central Asia. REST API.
  - **APNIC WHOIS/RDAP (rdap.apnic.net)**: Asia-Pacific. REST API.
  - **LACNIC RDAP (rdap.lacnic.net)**: Latin America/Caribbean. REST API.
  - **AFRINIC RDAP (rdap.afrinic.net)**: Africa. REST API.
  - **Ripestat (stat.ripe.net)**: RIPE's rich tool suite with APIs for prefix/ASN/routing data. Excellent for batch queries and visualization.

### Capability 4: RPKI/ROA Validation

- **What it does**: Validates that a prefix is legitimately announced by an ASN using cryptographic attestations.
- **Why needed**: Confirms that a prefix genuinely belongs to a critical infrastructure operator. Detects prefix hijacks.
- **Typical I/O**: Input: prefix + ASN. Output: Valid/Invalid/NotFound.
- **Example tools**:
  - **RPKI Validator (RIPE NCC)**: Open source. Runs locally. Strengths: Authoritative validation. Limitations: Requires maintenance of a local cache.
  - **Cloudflare RPKI looking glass (rpki.cloudflare.com)**: Web-based. Free. Strengths: Easy to use, visual. Limitations: Not scriptable at scale.
  - **rtrlib**: Open source C library for RPKI-RTR protocol. For integration into custom tools.

### Capability 5: Feed Cross-Checking / Comparison Engine

- **What it does**: Takes an aggregated blocklist and an exclusion set and identifies overlaps, with alerting and reporting.
- **Why needed**: This is the core operational capability for the firehol use case. Without it, false positives go undetected.
- **Typical I/O**: Input: blocklist feed (list of IPs/prefixes) + exclusion set (list of IPs/prefixes). Output: overlap report with metadata (which feed, which exclusion set entry, severity).
- **Example tools**:
  - **iprange**: Open source tool (part of firehol ecosystem). Efficient set operations on IP ranges (intersection, union, complement). Critical for exclusion set cross-checking at scale.
  - **Custom scripts**: Most practitioners build custom tooling here. Python with `netaddr` or `ipaddress` libraries is common.
  - **Category gap**: There is no mature, widely-adopted general-purpose tool specifically for "blocklist vs critical infrastructure exclusion set" cross-checking with alerting. This is typically custom-built.

### Capability 6: DNS/Service Verification

- **What it does**: Confirms what service runs on a given IP (via reverse DNS, active probing, or service fingerprinting).
- **Why needed**: When an overlap is detected, you need to confirm whether the IP is actually running the critical service or if it's been repurposed.
- **Typical I/O**: Input: IP address. Output: Service identification (DNS resolver, web server, etc.).
- **Example tools**:
  - **Reverse DNS lookup**: Standard DNS tool. PTR records can identify infrastructure (e.g., `dns.google`).
  - **Active DNS probing**: Send a DNS query to the IP and see if it responds as a resolver.
  - **Censys (censys.io)**: Commercial/scanned. Scans the entire internet and catalogs services per IP. Strengths: Rich service identification. Limitations: May not have real-time data; commercial for full access.
  - **Shodan (shodan.io)**: Commercial/scanned. Similar to Censys. Strengths: Good for service fingerprinting. Limitations: Commercial; data may be stale for some IPs.
  - **CDNPlanet (cdnplanet.com/cdndetect/)**: Identifies which CDN (Cloudflare, Akamai, Fastly, etc.) an IP belongs to.

### Cross-References to Actors

- **Aggregator Operator** uses: Capabilities 1-6 (all).
- **Feed Maintainer** should use: Capability 6 (to check what they're listing), Capability 2 (to avoid listing known infrastructure).
- **Consumer** uses: Capability 2 (to build local exclusion set for their specific dependencies).
- **Infrastructure Provider** provides: Capability 2 (they publish the data everyone else depends on).

---

## §8 Trade-offs & Constraints

### Fundamental Tensions

**Coverage vs Safety (the central tension)**: The more IPs a feed covers, the more threats it catches. The more it covers, the more likely it is to include critical infrastructure. There is no solution to this tension — only management through specificity (IP-level vs prefix-level) and exclusion sets. This is the CAP theorem of blocklists: you cannot maximize both coverage and safety simultaneously. You choose a point on the spectrum.

**Specificity vs Maintainability**: IP-level listings are the safest for shared infrastructure but produce enormous feeds (millions of entries). Prefix-level listings are more maintainable but more dangerous. ASN-level listings are easy to maintain but catastrophically dangerous for multi-tenant ASNs. Every practitioner must choose their granularity and accept the trade-off.

**Automation vs Human Judgment**: Fully automated feed ingestion + enforcement is fast and scales. It also has no judgment and will block 8.8.8.8 if a feed tells it to. Human review of every entry is safe but doesn't scale beyond a few thousand entries. The trade-off is managed by automating the "clearly safe" cases (hard exclusion set overlap = automatic rejection) and routing ambiguous cases to humans.

**Freshness vs Validation**: Pulling exclusion set sources frequently keeps the exclusion set current. It also introduces risk — if an authoritative source is compromised or misconfigured, the exclusion set could be corrupted. Validation (checking that new exclusion set entries make sense) adds latency. The trade-off is managed by having multiple sources and cross-validating.

### Alternatives Considered and Rejected

**Reject: "Block all hosting/cloud IPs"**: Some feeds and policies intentionally block all IP ranges belonging to cloud/hosting providers, on the theory that legitimate user traffic doesn't come from data centers. **Why rejected for general-purpose use**: Many critical services (SaaS, APIs, CDNs, webhooks) are hosted in clouds. Blocking all cloud IPs breaks the modern internet. **Failure mechanism**: Consumer's SaaS services, APIs, CDN-delivered content, and webhook callbacks all stop working.

**Reject: "Trust feed maintainers to self-police"**: Relying on each feed maintainer to avoid listing critical infrastructure. **Why rejected**: Feed maintainers often lack the expertise, incentive, or resources to identify critical infrastructure. They operate in narrow threat categories. **Failure mechanism**: Maintain, don't blame — the aggregator must add this value.

**Reject: "Only use feeds that guarantee no false positives"**: Limiting the aggregator to feeds that make zero-false-positive claims. **Why rejected**: No feed can guarantee this. Claims are not enforceable. And the best feeds for some threat categories may have occasional false positives that are manageable with exclusion sets. **Failure mechanism**: Aggregator loses coverage of important threat categories, or believes claims that aren't true.

**Reject: "Manual exclusion set maintenance only"**: Having a human curate the exclusion set without automated source fetching. **Why rejected**: Doesn't scale. Cloud providers add ranges frequently. The human will fall behind. **Failure mechanism**: New critical ranges are missed until a false positive incident occurs.

### Hard Constraints

- **BGP routing reality**: You cannot determine what service runs on an IP from BGP alone. BGP tells you who routes the prefix, not what's on it. This constrains exclusion set accuracy.
- **Rate limits on RIR/WHOIS queries**: RIRs enforce rate limits on their public WHOIS/RDAP services. Bulk queries require careful engineering or commercial arrangements.
- **IPv4 address scarcity driving fragmentation**: As IPv4 space is bought, sold, and transferred, prefix ownership changes. A prefix that belonged to a critical service last year may not this year. Exclusion set entries have a validity period.
- **Cloud provider IP ranges change weekly**: Any static reference becomes stale within days. Must use dynamic sources or accept inaccuracy.
- **Some providers don't publish IP ranges**: Alibaba Cloud, Tencent Cloud,
  Huawei Cloud, and Akamai do not publish downloadable IP range lists. Use
  provider-published sources where they exist; where they do not, BGP/RDAP
  enumeration is secondary evidence and must be labelled as such.

### Soft Constraints

- **Convention: Use provider-published lists first**: When an operator publishes their own IP range list, it's considered authoritative. Deviating (using RIR data instead) requires justification.
- **Convention: Document exclusion set inclusions**: Each entry in the exclusion set should have a source, a date, and a rationale. Undocumented entries are suspect.
- **Cost of violation**: Undocumented exclusion set entries lead to disputes and can hide errors.

### Deliberate Simplifications

- **Treating all IPs within a known critical prefix as equally critical**: In practice, not every IP in a /24 used by a DNS provider is equally critical (some may be unused). But the simplification of treating the entire prefix as critical is safer and more maintainable than trying to identify which individual IPs are active.
- **Ignoring IPv6 in the initial implementation**: Many blocklist aggregators focus on IPv4 first due to the volume of historical IPv4-only feeds. This is a pragmatic simplification, not a long-term strategy. IPv6 critical infrastructure exists and must be covered eventually.
- **Using ASNs as context instead of truth**: ASN ownership is useful evidence
  for who routes a prefix, but it does not prove the service role of every IP in
  that ASN. Large cloud, CDN, SaaS, and corporate ASNs must not be treated as
  hard critical reference feeds by themselves. Use ASN data to generate or
  validate more precise reference sets, and expose ASN-wide matches as
  contextual unless the ASN is small, dedicated, and independently verified for
  the critical role.

---

## §9 Maturity Levels

### Stage 1: Reactive (Post-Incident Whitelisting)

- **Characteristic behaviors**: No proactive exclusion set. Critical infrastructure IPs are added to the exclusion set only after a false positive causes a visible incident. Each incident is a surprise.
- **Measurable indicators**: Exclusion set exists but is small (< 20 entries) and grows only after incidents. No automated cross-checking. Feed quality is not systematically measured.
- **Blind spots**: Does not know what it doesn't know. Cannot detect near-misses. Subtle performance degradation from partial blocking goes unnoticed.
- **Transition enablers**: Experience: A few painful incidents that demonstrate the need for proactive protection. Learning: Reading this document or equivalent, discovering that the problem is enumerable.
- **Common stuck points**: "We haven't had an incident recently, so it's not a priority." Breaking through requires either another incident or leadership buy-in driven by risk assessment.

### Stage 2: Manual Curation

- **Characteristic behaviors**: A human maintains an exclusion set of known critical infrastructure. The exclusion set is cross-referenced against feeds, but manually or with simple scripts. New feeds are spot-checked before full inclusion.
- **Measurable indicators**: Exclusion set covers the obvious categories (DNS resolvers, root servers, maybe major CDNs). 30-100 entries. Cross-checking happens but is not fully automated. Exclusion set is updated irregularly.
- **Blind spots**: Niche infrastructure (NTP, OCSP, specific SaaS) may be missed. Cloud provider range additions go undetected between manual updates.
- **Transition enablers**: Tooling: Building or adopting scripts that automate the cross-check. Learning: Discovering the breadth of critical infrastructure categories.
- **Common stuck points**: "The exclusion set is good enough." Breaking through requires demonstrating gaps.

### Stage 3: Automated Cross-Checking

- **Characteristic behaviors**: Automated pipeline fetches feeds, cross-references against exclusion set, and flags/hard-blocks overlaps. Exclusion set is maintained from authoritative sources on a schedule. New feeds are automatically validated before inclusion.
- **Measurable indicators**: Exclusion set covers all major categories (DNS, CDN, cloud, CA, NTP, update, email, identity). 100+ ASNs, thousands of prefixes. Cross-checking is part of the CI/CD pipeline. Feed quality scores are computed and published.
- **Blind spots**: May still miss niche or regional infrastructure. May over-rely on automation without human review of flagged items. May not handle the contextual exclusion set tier well.
- **Transition enablers**: Engineering investment in the refresh pipeline and cross-checking automation. Learning from false positives that slip through.
- **Common stuck points**: "The automation catches everything." Breaking through requires discovering a category that was missed.

### Stage 4: Comprehensive Monitoring & Community Feedback

- **Characteristic behaviors**: Full coverage of critical infrastructure categories. Layered exclusion set (hard/soft/contextual). Active monitoring of feed quality over time. Community can report false positives and have them validated against the exclusion set. Exclusion set sources are refreshed daily for cloud providers.
- **Measurable indicators**: Near-zero undetected critical infrastructure overlaps. Feed quality scores are published and drive feed inclusion/exclusion decisions. Community reports are handled within 24 hours.
- **Blind spots**: Edge cases around new infrastructure categories. Regional differences in infrastructure criticality.
- **Transition enablers**: Community engagement: Feedback loops from consumers. Tooling maturity: Automated everything with human review for edge cases.

### Stage 5: Predictive & Adaptive

- **Characteristic behaviors**: System detects NEW critical infrastructure before it's explicitly listed (e.g., detecting that a new anycast IP is being provisioned for a DNS resolver by monitoring DNS client behavior). Exclusion set is informed by dependency analysis. Feed quality scoring incorporates trend analysis.
- **Measurable indicators**: New critical infrastructure is detected within days of deployment, before false positives occur. Dependency-based exclusion set adapts to changing internet topology.
- **Blind spots**: Novel infrastructure categories not yet envisioned. But the system is designed to learn.
- **Transition enablers**: Advanced data analysis capabilities (ML, anomaly detection on traffic patterns). Deep integration with internet measurement platforms.

### Progression Dynamics

- Progression is roughly linear: you must pass through each stage. You cannot have automated cross-checking without an exclusion set to check against.
- Regressions occur when: exclusion set maintenance is deprioritized and becomes stale; key personnel leave without knowledge transfer; the exclusion set infrastructure breaks and is not fixed.
- What causes regressions: organizational priorities shift; budget cuts; "it's been working fine, we don't need to maintain it" (complacency after a period of zero incidents).

---

## §10 Prerequisites & Minimum Viable Conditions

### Environmental Prerequisites

1. **Access to at least one BGP/routing data source** (RIPE RIS, RouteViews, Hurricane Electric, or equivalent). Without this, you cannot enumerate prefixes for ASNs that don't self-publish.
   - **Failure mode if absent**: You cannot build the exclusion set. You are limited to manually entered IPs, which is Stage 1 maturity at best.

2. **Network access to infrastructure providers' published IP range endpoints** (AWS ip-ranges.json, Microsoft download, Cloudflare IPs page, etc.). Many of these are public HTTPS endpoints.
   - **Failure mode if absent**: You cannot fetch authoritative prefix lists. You rely on stale data or less authoritative sources.

3. **RDAP/WHOIS access to at least the major RIRs** (RIPE, ARIN, APNIC). Needed for fallback enumeration.
   - **Failure mode if absent**: You cannot enumerate prefixes for operators that don't self-publish. Coverage gaps.

4. **A working IP set operations tool** (iprange, or Python with netaddr/ipaddress, or equivalent). Needed to efficiently compute intersections between feeds (potentially millions of IPs) and exclusion sets (thousands of prefixes).
   - **Failure mode if absent**: Cross-checking is too slow to be practical. You fall back to manual spot-checking.

### Skill Prerequisites

1. **Ability to read BGP data**: Understanding what an ASN is, what a prefix announcement means, and how to interpret routing table output. This is non-negotiable.
   - **Failure mode if absent**: You cannot validate exclusion set entries or debug cross-checking results.

2. **Scripting ability**: Python, bash, or equivalent. The refresh pipeline, cross-checking, and alerting require automation.
   - **Failure mode if absent**: You are stuck at Stage 2 permanently.

3. **Understanding of CIDR notation and IP set operations**: Intersection, containment, aggregation. You need to understand that "8.8.8.1 is contained in 8.8.8.0/24".
   - **Failure mode if absent**: You make errors in exclusion set cross-checking.

### Tooling Prerequisites

1. **An IP set operations library/tool** (iprange, netaddr, ipaddress). Minimum viable.
2. **An HTTP client** for fetching authoritative sources (curl, wget, Python requests).
3. **A DNS client** for RPKI/DNS-based lookups and reverse DNS verification.
4. **Scheduled task runner** (cron, systemd timers, CI/CD pipeline) for automated exclusion set refresh.

### Data / Telemetry Access

1. **Access to the aggregated blocklist data** (the feeds you are protecting). Obvious but necessary.
2. **Access to the exclusion set sources** (listed in §7, Capability 2). Must be reachable from the automation environment.

### Organizational Prerequisites

1. **Authority to exclude/flag feed entries**: The person maintaining the exclusion set must have the authority to flag or exclude entries that match the exclusion set. If they can only observe but not act, the exclusion set is theater.
   - **Failure mode if absent**: Exclusion set overlaps are detected but not prevented. Incidents continue.

2. **Mandate for quality over quantity**: The organization must value feed quality enough to accept that some feeds or entries will be excluded.

### Time / Attention Prerequisites

1. **Initial build**: 20-40 hours to enumerate critical ASNs, fetch and validate their prefixes, build the exclusion set, and implement cross-checking. This is a one-time investment.
2. **Ongoing maintenance**: 2-4 hours per month for reviewing exclusion set changes, investigating flagged items, and updating the ASN list. Plus automated refresh time.
3. **Incident response time**: When a critical infrastructure false positive is detected, someone must be available to respond within hours (for aggregator operators) or minutes (for real-time enforcement contexts).

---

## §11 Common Pitfalls & Failure Modes

### Pitfall 1: The "Silent Degradation" from CDN Blocking

- **Situation**: A feed includes IPs belonging to a CDN edge (Cloudflare, Akamai, Fastly). The consumer's firewall enforces the block.
- **What goes wrong**: The consumer doesn't lose all access to websites behind the CDN. Instead, some requests fail, some succeed (depending on which edge IP is resolved). Performance degrades inconsistently.
- **Observable symptom**: Intermittent website failures that correlate with DNS resolution returning blocked IPs. No single point of failure. Hard to diagnose.
- **Impact / blast radius**: All websites and services behind the affected CDN. For Cloudflare, this can be millions of domains.
- **Why easy to make**: The consumer doesn't realize they depend on CDN edge IPs. The failure is intermittent. It's not obviously a "blocklist issue."
- **Recovery path**: Identify the blocked CDN IPs from firewall logs. Remove from enforcement. Whitelist the CDN's published IP ranges.
- **What makes recovery harder**: The intermittent nature makes it hard to reproduce. The consumer may try other troubleshooting before identifying the blocklist as the cause.
- **Prevention**: Include major CDN edge IPs in the exclusion set. Cloudflare publishes their IPs; Akamai and others can be enumerated via ASN.
- **Silent failure**: YES. This is a silent failure — the system appears to work but is degraded.

### Pitfall 2: DNS Resolution Collapse from Resolver Blocking

- **Situation**: A feed includes 8.8.8.8 or 1.1.1.1. The consumer's firewall blocks outbound traffic to these IPs.
- **What goes wrong**: All DNS resolution using these resolvers fails. Every system that relies on them stops resolving. Email stops. Web browsing stops. Internal services that depend on external name resolution stop.
- **Observable symptom**: Complete DNS failure. Nothing can be looked up.
- **Impact / blast radius**: Total organizational internet outage if these are primary resolvers.
- **Why easy to make**: The feed lists the IP without context. The consumer auto-enforces. The IP looks like any other IP in the feed.
- **Recovery path**: Immediately remove the IP from enforcement. If the organization's DNS is completely down, this requires someone with direct access to the firewall who can bypass normal change processes.
- **What makes recovery harder**: If DNS is down, the operator may not be able to access documentation, tools, or communication channels. They're operating blind.
- **Prevention**: Hard-whitelist all public recursive DNS resolver IPs. These should be the first entries in any exclusion set.
- **Silent failure**: NO. This is spectacularly visible. But it's also catastrophic.

### Pitfall 3: Cloud Provider Metadata Service Blocking (Self-Inflicted)

- **Situation**: A cloud-hosted VM's firewall blocks traffic to the cloud provider's metadata service (e.g., 169.254.169.254 for AWS/Azure/GCP).
- **What goes wrong**: VMs cannot access their own metadata (instance ID, IAM roles, network configuration). Services that depend on metadata at startup fail.
- **Observable symptom**: VM startup failures. IAM role acquisition failures. Applications that dynamically configure based on metadata break.
- **Impact / blast radius**: All VMs in the affected cloud account.
- **Why easy to make**: The metadata IP is a link-local address (169.254.x.x), which some feeds include as part of bogon or suspicious ranges.
- **Recovery path**: Whitelist `169.254.169.254/32` in the enforcement layer.
  For AWS Nitro instances with IPv6, also whitelist `fd00:ec2::254` when IPv6
  IMDS is enabled.
- **Prevention**: Include cloud metadata service IPs in local/operator
  enforcement exclusions. These are link-local or unique-local control-plane
  endpoints, not public internet infrastructure feeds. Google Cloud's primary
  docs say to use the IPv4 metadata address even with IPv6-only instances;
  Azure's public IMDS docs verified here document `169.254.169.254`.
- **Silent failure**: Partially. Some VMs may already have cached metadata and continue working; new VMs fail. This creates an inconsistent state.

### Pitfall 4: Certificate Validation Breakdown from OCSP/CRL Blocking

- **Situation**: A feed includes IPs hosting OCSP responders or CRL distribution points for major certificate authorities.
- **What goes wrong**: Certificate validation fails for any application that checks OCSP/CRL. HTTPS connections are refused. Software updates that verify signatures fail.
- **Observable symptom**: Browser security warnings ("certificate cannot be verified"). Automated API calls fail with certificate errors. VPN connections that validate server certificates fail.
- **Impact / blast radius**: All TLS-secured communications that validate against the affected CA.
- **Why easy to make**: OCSP responder IPs are not widely known. They may be hosted on CDN infrastructure (making them look like CDN IPs) or on the CA's own infrastructure.
- **Recovery path**: Whitelist the CA's OCSP/CRL infrastructure. Extract OCSP/CRL URLs from certificates in your environment, resolve those domains, and whitelist the resulting IPs.
- **What makes recovery harder**: The error manifests as "certificate errors" which most troubleshooters attribute to expired or misconfigured certificates, not to network-level blocking.
- **Prevention**: Enumerate major CAs' certificate-status infrastructure where
  they publish dedicated IPs or stable status sources. DigiCert publishes
  certificate-status IP information; do not infer DigiCert from AS40443. Note:
  Let's Encrypt shut down OCSP in August 2025; CRL is now at `lencr.org`, and
  no stable Let's Encrypt IP reference feed has been verified.
- **Silent failure**: YES. Many applications silently fall back to less strict validation or cache previous OCSP responses. The failure is intermittent and environment-dependent.

### Pitfall 5: Time Synchronization Failure from NTP Blocking

- **Situation**: A feed includes IPs of NTP servers (NTP Pool, NIST, national time services). The consumer's firewall blocks outbound NTP traffic to these IPs.
- **What goes wrong**: System clocks drift. Kerberos authentication fails (time-sensitive). TLS certificate validation fails (clock skew). Log timestamps become unreliable. Distributed system coordination breaks.
- **Observable symptom**: Authentication failures that correlate with time drift. Certificate errors. "Clock skew detected" warnings in logs.
- **Impact / blast radius**: All systems using the affected NTP servers for time synchronization. If these are the only NTP sources, all systems gradually fail.
- **Why easy to make**: NTP pool IPs are hosted by volunteers on diverse networks. They look like random IPs. A feed that lists a VPS provider's IP range may catch NTP pool servers hosted on that provider.
- **Recovery path**: Remove the specific blocked NTP server IPs from
  enforcement. For environments that depend on NTP Pool, use DNS-based
  service discovery with freshness/TTL handling rather than a static public
  reference feed. Ensure redundant NTP sources are not all blocked.
- **What makes recovery harder**: Time drift is gradual. Systems may work for hours or days after NTP is blocked before the drift becomes critical.
- **Prevention**: Identify exact NTP/time infrastructure from official sources
  such as Cloudflare, Google, NIST, Netnod, and national time services that
  publish exact IPs/prefixes. Treat NTP Pool as operator-local DNS-discovery
  policy, not as a static hard public reference feed.
- **Silent failure**: YES. This is one of the most insidious silent failures. Everything degrades slowly and inconsistently.

### Pitfall 6: Software Update Blocking

- **Situation**: A feed includes IPs hosting OS/software update servers (Windows Update, Apple Software Update, Linux distribution mirrors).
- **What goes wrong**: Security patches are not downloaded. Systems fall behind on updates. Vulnerabilities are not patched.
- **Observable symptom**: No immediate symptom. Systems continue to run but become increasingly vulnerable. When the next major vulnerability is disclosed, systems don't get patched, and they're compromised.
- **Impact / blast radius**: All systems depending on the blocked update servers. Could be an entire fleet.
- **Why easy to make**: Update server IPs may be hosted on CDNs or cloud infrastructure, making them look like any other CDN/cloud IP.
- **Recovery path**: Identify and whitelist the update server IPs. Force an update cycle to catch up on missed patches.
- **What makes recovery harder**: The failure is completely silent until a vulnerability is exploited. There's no immediate symptom to investigate.
- **Prevention**: Whitelist update infrastructure for all OS vendors in use
  using vendor-supported URL/domain or scoped IP guidance. Do not assume all
  Microsoft AS8075/Azure ranges are Windows Update. Apple's published
  `17.0.0.0/8` and IPv6 blocks are authoritative but broad, so treat them as
  soft/contextual. Linux mirror networks are distributed and are usually not a
  stable public IP-feed target.
- **Silent failure**: YES. The most dangerous kind — the consequence is completely decoupled in time from the cause.

### Compounding Failure: DNS + CDN + OCSP

- **Scenario**: A feed includes IPs that happen to cover DNS resolvers, CDN edges, and OCSP responders simultaneously (e.g., a large aggregate prefix that covers multiple services). The consumer enforces the block.
- **Compounding effect**: DNS fails (can't resolve names). CDN is blocked (even if names resolve, content can't load). OCSP is blocked (even if content loads, certificate validation fails). The internet appears completely broken. Troubleshooting is nearly impossible because every tool and every service is affected.
- **Why worse than sum**: Each failure blocks the recovery path for the others. You can't reach documentation (DNS down). You can't verify certificates (OCSP down). You can't load internal dashboards if they're behind a CDN.
- **Prevention**: Comprehensive exclusion set covering all categories. This compounding scenario is why the exclusion set must cover ALL critical categories, not just the obvious ones.

---

## §12 Worked Examples & Case Studies

### Success Case: Proactive Exclusion Set Catches Feed Poisoning

- **Initial context**: An aggregator (similar to firehol) has a comprehensive exclusion set with automated cross-checking. They are evaluating a new feed that claims to list "known SSH brute force sources."
- **What was done**: The automated cross-check flagged that the new feed contained 8.8.8.8 and 1.1.1.1. Investigation revealed that the feed maintainer had observed DNS amplification attacks being reflected through these resolvers and listed them as "attack sources." The feed was included but with a quality warning, and the DNS resolver IPs were excluded from the aggregated output.
- **What happened**: Consumers never saw Google DNS or Cloudflare DNS in their blocklists. The feed maintainer was notified and educated about the difference between reflection sources and attack sources. Feed quality improved.
- **Lesson**: Automated cross-checking caught a false positive that would have been catastrophic if auto-enforced.
- **Patterns illustrated**: Whitelist-First Aggregation (§6), Per-Feed Quality Scorecard (§6).
- **Pitfalls avoided**: Pitfall 2 (DNS Resolution Collapse).

### Failure Case: Cloud Provider Range Blindly Included

- **Initial context**: A mid-size company uses a commercial firewall that auto-enforces IP reputation feeds. They do not maintain their own exclusion set.
- **What went wrong**: The firewall vendor's feed included a range of AWS IPs (a /24) that happened to include the IP of the company's CRM SaaS provider's API endpoint. The company's sales team suddenly couldn't access the CRM.
- **Symptoms**: CRM web interface loaded (served from a different CDN IP), but API calls (made directly to an AWS-hosted endpoint) timed out. The CRM provider's status page showed green. Internal monitoring showed API timeouts.
- **Root cause**: The IP reputation feed listed the /24 due to one abusive EC2 instance in that range. The firewall vendor didn't differentiate between the abusive instance and the legitimate SaaS API.
- **Recovery**: The company spent 6 hours troubleshooting before someone checked the firewall logs and found the blocked IP. They added a manual exclusion entry for the CRM API IP.
- **What would have prevented**: An exclusion set maintained by the company that included their critical SaaS dependencies. Or the firewall vendor using the aggregator's exclusion set.
- **Lesson**: You cannot delegate critical infrastructure protection entirely to your feed provider. You must know your own dependencies.
- **Patterns illustrated**: Anti-Pattern: "Trust the Feed" (§6).
- **Pitfalls illustrated**: Pitfall 1 (Silent Degradation — partial service failure that's hard to diagnose).

### Edge Case: Legitimate Blocking of Cloud Infrastructure

- **Initial context**: An aggregator's exclusion set includes all AWS IP ranges (contextual exclusion set tier). A feed lists a specific AWS IP (a single /32) that is hosting a known phishing site.
- **What was done**: The cross-check flagged the IP as an AWS IP (contextual exclusion set overlap). Human review confirmed the IP was hosting a phishing site actively targeting the feed maintainer's users. The listing was justified and allowed through with documentation.
- **What happened**: The phishing site was blocked by consumers. No legitimate AWS services were affected because the listing was at /32 granularity.
- **Lesson**: The exclusion set should not prevent legitimate blocking of cloud IPs — it should ensure such blocking is specific and justified. The contextual exclusion set tier handles this.
- **Patterns illustrated**: Layered Whitelist (§6) — contextual tier in action.
- **Pitfalls avoided**: Over-whitelisting that would protect malicious infrastructure.

### Reference Incident: DNS-based Blocklists and Amplification Attack IPs

- **Context**: Multiple IP feeds have historically listed open DNS resolvers (including 8.8.8.8 and other public resolvers) because these resolvers can be used in DNS amplification DDoS attacks. The resolvers appear in netflow data as "sources of large DNS responses."
- **Dynamics**: The resolvers are not malicious — they're being abused as reflectors. Listing them in a blocklist is counterproductive because blocking them breaks DNS for the blocker, not for the attacker.
- **Relevance**: This is the canonical example of a critical infrastructure false positive. It's so common that it should be one of the first test cases for any exclusion set.
- **Lesson**: Understand the difference between "source of abuse" and "infrastructure abused." Critical infrastructure is almost always in the latter category.
- **Pitfalls illustrated**: Pitfall 2 (DNS Resolution Collapse).

---

## §13 Diagnostic Quick Reference

### Red Flags — Abort Current Plan If:

- Your blocklist contains ANY of these IPs: 8.8.8.8, 8.8.4.4, 1.1.1.1, 1.0.0.1, 9.9.9.9, 208.67.222.222, 208.67.220.220 → **IMMEDIATELY remove and investigate the feed that contributed them.**
- Your blocklist contains ANY /16 or larger prefix from these ASNs: AS13335 (Cloudflare), AS15169 (Google), AS16509 (Amazon/AWS), AS8075 (Microsoft), AS20940 (Akamai), AS54113 (Fastly) → **You are blocking entire shared infrastructure ranges. Deaggregate or remove.**
- Your enforcement is causing intermittent website failures, DNS timeouts, or certificate validation errors → **Check for CDN/DNS/OCSP IP blocks immediately.**
- A newly ingested feed has > 10,000 new entries in its first update → **Bulk inclusion likely; validate against exclusion set before enforcement.**
- A feed's description contains "datacenter," "hosting," "proxy," or "VPN" without IP-level specificity → **High risk of cloud/CDN false positives.**
- A feed includes any transit provider ASN (AS3356 Level3, AS174 Cogent, AS6939 Hurricane Electric) → **Escalate immediately; blocking backbone providers fragments internet connectivity.**

### Escalation Triggers — Stop and Get More Expertise:

- A cloud provider IP (AWS, Azure, GCP) appears in the blocklist and you need to decide whether it's a legitimate target → **Requires human investigation of the specific IP/service.**
- You are considering adding a new infrastructure ASN to the exclusion set → **Validate that the ASN genuinely belongs to critical infrastructure (check RIR, BGP, reverse DNS).**
- A consumer reports a false positive that is NOT in your exclusion set → **Your exclusion set has a gap. Investigate the reported IP's role and add the category.**
- You are considering ASN-level blocking for any ASN with > 100 announced prefixes → **ASN-level blocking of large ASNs will cause catastrophic collateral damage. Do not proceed without expert review.**

### First Moves Under Common Situations:

**Situation: Building a critical infrastructure exclusion set from scratch**

1. Start with public DNS resolvers: add exact provider-documented resolver IPs
   first (`8.8.8.8`, `8.8.4.4`, `1.1.1.1`, `1.0.0.1`, `9.9.9.9`,
   `149.112.112.112`, `208.67.222.222`, `208.67.220.220`, and IPv6
   equivalents). Do not expand to whole `/24` ranges unless the provider
   publishes that range as service infrastructure.
2. Add Cloudflare published ranges: fetch from `https://www.cloudflare.com/ips-v4`.
3. Add root DNS server prefixes: fetch from `https://www.internic.net/domain/named.root` and derive the serving IPs.
4. Add major cloud providers: fetch AWS ranges from `https://ip-ranges.amazonaws.com/ip-ranges.json`, Azure from Microsoft's published JSON, Google Cloud from their documentation.
5. Add Fastly ranges: fetch from `https://api.fastly.com/public-ip-list`.
6. Add Akamai: enumerate AS20940 and AS16625 prefixes via BGP data.
7. Add NTP/time infrastructure from exact official sources: Cloudflare Time
   Services, Google Public NTP, NIST ITS, Netnod, and other national services
   only when exact IPs/prefixes are published. Do not enumerate
   `pool.ntp.org` into a static public reference feed.
8. Add CA infrastructure from dedicated certificate-status sources where they
   exist, such as DigiCert's published OCSP/CRL IP page. Do not use AS40443 for
   DigiCert; Team Cymru maps AS40443 to CDK Global, not DigiCert.
9. Add OS update infrastructure only where the source is scoped and defensible.
   Apple's `17.0.0.0/8` and documented IPv6 blocks are official but very broad,
   so they are soft/contextual, not hard. Microsoft Update is not equivalent to
   all AS8075/Azure ranges.
10. Implement automated refresh for authoritative machine feeds and explicit
    review/diff alerts for static-doc and secondary/BGP-derived sources.

**Situation: Investigating a reported false positive**

1. Look up the blocked IP in WHOIS/RDAP → who owns it?
2. Look up the IP's ASN → which network?
3. Reverse DNS lookup → does it identify the service?
4. Check against your exclusion set → was it supposed to be protected?
5. If not in exclusion set but is critical infrastructure → add the category to the exclusion set (not just this IP).
6. Identify which feed contributed the IP → flag the feed.

**Situation: Evaluating a new feed for inclusion**

1. Fetch the feed.
2. Cross-reference against entire exclusion set (all tiers).
3. Count hard-whitelist overlaps → if > 0, reject or require feed fix.
4. Count soft-whitelist overlaps → review each manually.
5. Check granularity → is it IP-level or prefix-level? Prefix-level for cloud ASNs is a red flag.
6. Check feed volume and update frequency → is it reasonable for the claimed threat category?
7. Assign quality score → publish alongside feed if aggregator.

### Critical Checks (Commonly Skipped):

- **Check for DNS resolver IPs**: Always. This is the #1 catastrophic false positive.
- **Check for 169.254.169.254**: Cloud metadata service. Blocking this kills
  VMs. Treat it as a local/operator enforcement exclusion, not a public
  internet critical-infrastructure feed.
- **Check for IPv6**: If you're doing IPv6 feeds, you need IPv6 exclusion sets too.
- **Check exclusion set freshness**: When was it last refreshed? If > 7 days for cloud providers, refresh now.
- **Check the feed's own policy**: Does the feed explicitly state it lists shared infrastructure? Some do. That's useful context for quality scoring.
- **Check for prefix aggregation effects**: An IP may not match an exclusion set entry directly, but the /24 containing it might be in the exclusion set. Use prefix containment checks, not just equality.

### Top Decision Points:

| Question | If Yes | If No |
|---|---|---|
| Does the IP match the hard whitelist? | **Remove from feed. Alert.** | Continue to next check. |
| Does the IP match the soft whitelist? | **Flag for human review.** | Continue to next check. |
| Is the IP in a cloud/hosting ASN? | **Flag as contextual. Require justification.** | Normal processing. |
| Is the feed listing at prefix level (> /32)? | **Deaggregate or reject for shared infrastructure ASNs.** | Normal processing. |
| Is this a newly added feed (< 7 days)? | **Extra scrutiny. Check all overlaps manually.** | Normal quality scoring applies. |
| Has the exclusion set been refreshed in the last 7 days? | Good. | **Refresh now. Prioritize cloud provider ranges.** |

---

## §14 Synthesis Notes & Validation Trail

### Per-Advisor Profile

**Advisor GLM (skill-distill-glm)**: Produced the most comprehensive and longest document. Strongest on DNS resolver enumeration, cloud provider categorization, and worked examples. Deep coverage of the co-tenancy problem and blast radius modeling. Appended a detailed ASN reference table that proved highly useful. Tended toward verbosity in §6 patterns and §11 pitfalls. Correctly identified the layered whitelist model (hard/soft/contextual).

**Advisor Kimi (skill-distill-kimi)**: Strong on BGP/ASN intelligence and dependency modeling. Best treatment of the "dependency-aware blocking" pattern and graduated response. Provided solid coverage of the "infrastructure stack" mental model. Shallow on email delivery infrastructure and payment infrastructure. Correctly noted that IPv6 critical infrastructure is often less protected.

**Advisor Qwen (skill-distill-qwen)**: Concise and practical. Best treatment of the "trust tiering" model (Tier-1/Tier-2/Tier-3). Strong on feed quality scoring and CIDR normalization. Lightest on specific infrastructure enumeration. Good coverage of maturity levels from reactive to dynamic trust scoring.

**Advisor MiniMax (skill-distill-minimax)**: Focused on pointer-level detail. Best treatment of Akamai (though with an ASN error), Fastly, and email delivery infrastructure. Provided the clearest "infrastructure stack" layer model (Layer 0 transit through Layer 5 specialized services). Lightest on anti-patterns and failure modes. Good coverage of the honeypot-derived feed problem.

### Discrepancies Identified and Resolved

**Discrepancy 1: Quad9 ASN assignment**
Three advisors cited AS54994 for Quad9. Online validation confirmed AS19281 (QUAD9-AS-1, CleanerDNS Inc.) as the correct primary ASN for 9.9.9.9. AS54994 belongs to Meteverse Limited, a Canadian hosting company with no relation to Quad9. Additionally, AS398891 is Quad9's internal/administrative ASN. Resolution: Use AS19281 as the Quad9 exclusion set anchor. AS54994 should NOT be excluded as "Quad9."

**Discrepancy 2: Akamai ASN**
Two advisors cited AS16615 as an Akamai ASN. Online validation found NO evidence of AS16615. The actual Akamai Technologies ASN is AS16625. Akamai International BV operates AS20940. Resolution: Use AS20940 and AS16625 for Akamai exclusion. AS16615 does not exist.

**Discrepancy 3: Let's Encrypt ASN**
One advisor cited AS42408 as "Let's Encrypt / ISRG." Online validation found AS42408 belongs to Transvyaz-N LLC, a Russian telecommunications company, with zero connection to Let's Encrypt. Let's Encrypt does not publish IP addresses for its infrastructure. OCSP was shut down August 6, 2025; CRL is now at `lencr.org`. Resolution: Do not use AS42408 for Let's Encrypt. Exclude via domain-level allowlisting (`/.well-known/acme-challenge/`) rather than IP-based blocking.

**Discrepancy 4: Okta ASN**
Advisors noted uncertainty about Okta's ASN (citing AS538427 and AS539671). Online validation found NEITHER ASN is associated with Okta. Both have zero announced IP prefixes. Okta's actual ASN is AS19745 (Okta, Inc.), owning 8.35.185.0/24 and several IPv6 /48s. Resolution: Use AS19745 for Okta exclusion.

**Discrepancy 5: Huawei Cloud ASN**
One advisor cited AS30637 as a Huawei Cloud ASN. Online validation found AS30637 belongs to Unitas Global, a US-based network operator. Huawei Cloud's actual ASN is AS55990 (Huawei Cloud Service Data Center). Resolution: Use AS55990 for Huawei Cloud exclusion.

**Discrepancy 6: Alibaba Cloud official IP feed**
No advisor could cite an official Alibaba Cloud JSON endpoint. Online
validation confirmed there is NONE — Alibaba Cloud does not publish a
centralized machine-readable IP range manifest. Two community-maintained
alternatives were found: `sourcecidr.com/feeds/alibaba-cloud.json` and the
GitHub repository `disposable/cloud-ip-ranges`, but
`disposable/cloud-ip-ranges` currently has no repository license. Resolution:
do not treat community-maintained sources as authoritative or primary. Prefer a
project-owned BGP/RDAP enumeration pipeline if Alibaba/Tencent/Huawei coverage
is required, or mark community sources as secondary with explicit
license/quality caveats after user approval.

**Discrepancy 7: DigiCert ASN**
The knowledge doc previously suggested DigiCert AS40443. Online validation
confirmed AS40443 belongs to CDK Global, LLC, not DigiCert. DigiCert publishes
dedicated OCSP/CRL and certificate-status IP information at its own knowledge
base pages. Resolution: do not use AS40443 for DigiCert. Use DigiCert's
published certificate-status IP source as a soft `certificate_validation`
reference, preserving the CA-validation role even when the addresses are served
through CDN or dedicated status infrastructure.

**Discrepancy 8: AWS SES service tag**
The knowledge doc implied AWS SES could be extracted from an AWS `SES` service
tag. Current AWS `ip-ranges.json` has no `SES` service tag. Resolution: do not
implement `service=SES` filtering. Treat Amazon SES delivery as SPF/DNS-derived
or broader AWS regional context until a primary SES-specific IP source is
verified.

**Discrepancy 9: GitLab/Twitter AS35995**
Project configuration previously labelled AS35995 as GitLab. Team Cymru, IPinfo,
and other ASN sources identify AS35995 as Twitter Inc. GitLab.com publishes
specific Web/API fleet IP ranges and otherwise runs behind Cloudflare/GCP rather
than a dedicated GitLab ASN in this catalog. Resolution: remove AS35995 from the
GitLab/dev-platform ASN context list and represent GitLab through official
published IP ranges in the reference-feed model.

**Discrepancy 10: StackPath critical-reference freshness**
The local catalog still has a MISP StackPath warninglist, but StackPath is not a
live CDN critical-reference provider for new warnings. Akamai announced on
August 24, 2023 that StackPath had decided to cease CDN operations and Akamai
acquired select enterprise customer contracts. Resolution: do not promote
StackPath as a current critical-infrastructure reference feed. Costa approved
removing `misp_stackpath.yaml` from this catalog because it was recently added
from MISP and stale sources should not remain.

**Discrepancy 11: AWS/GCP MISP drift**
Live source checks showed AWS publishes 15,548 total current prefixes in
`ip-ranges.json` (10,161 IPv4, 5,387 IPv6), while the local MISP AWS warninglist
has 3,629 entries. GCP publishes 910 total current prefixes in `cloud.json` (862
IPv4, 48 IPv6), while the local MISP GCP warninglist has 424 entries.
Resolution: for contextual cloud-provider critical references, use primary
provider manifests; keep MISP only as secondary/community evidence.

**Discrepancy 12: GitHub MISP flattening**
The local MISP GitHub warninglist flattens GitHub service ranges with GitHub
Actions runner ranges. GitHub's Meta API exposes separate buckets; current live
checks showed `actions` alone has 6,237 ranges while service buckets such as
`web`, `api`, `git`, `hooks`, `packages`, `pages`, `codespaces`, and `copilot`
are distinct. Resolution: use GitHub Meta API categories directly for critical
reference feeds; do not use the flattened MISP GitHub list as the warning unit.

**Discrepancy 13: Zscaler role**
Zscaler hub CIDRs are not CDN edge or generic cloud-hosting ranges. They are
cloud proxy/security-service infrastructure. Resolution: if included, classify
Zscaler with a `cloud_proxy` role and role-specific methodology copy; otherwise
exclude it from the first reference catalog.

### Unique-Advisor Claims Validated

**GLM: Layered whitelist (hard/soft/contextual) model**: Validated. This three-tier model is the most operationally useful framing discovered across all advisors. Incorporated into §2 core concepts and §6 patterns.

**Kimi: Dependency-Aware Blocking pattern**: Validated. The concept of mapping organizational dependency graphs before implementing blocks is sound and complements the exclusion set approach. Incorporated into §6 patterns.

**Qwen: Trust tiering (Tier-1/Tier-2/Tier-3)**: Validated. The tiering model provides a useful shorthand for prioritizing exclusion set entries. Aligned with the hard/soft/contextual model from GLM.

**MiniMax: Infrastructure stack layer model (Layer 0 transit through Layer 5 specialized services)**: Validated. This provides the most intuitive mental model for explaining blast radius to non-technical stakeholders. Incorporated into §2 mental model.

**GLM: Compound failure scenario (DNS + CDN + OCSP)**: Validated. This cascading failure mode is the most severe outcome of incomplete exclusion sets and worth explicit documentation. Incorporated into §11 pitfalls.

### Apparent Gaps Filled Online

**Gap 1: Email delivery infrastructure (SendGrid, Mailgun)**: Not covered by any advisor. Online validation identified SendGrid AS11377 and Mailgun AS396479 with their respective IP ranges. Incorporated into §7 tools and §2 vocabulary.

**Gap 2: Payment infrastructure (Stripe)**: Not covered by any advisor. Online validation identified Stripe AS5091, AS394562, and AS141743 with their official API IP list published at `docs.stripe.com/ips`. Incorporated into §7 tools.

**Gap 3: Chinese cloud providers (Alibaba Cloud, Tencent Cloud, Huawei Cloud)**:
Minimally covered. Online validation confirmed Alibaba Cloud AS37963/AS45102/AS45103,
Tencent Cloud AS45090, and Huawei Cloud AS55990. AS60924 was previously listed
here incorrectly; Team Cymru maps AS60924 to ORIXCOM, IE. None of these
providers have a verified official centralized downloadable IP range manifest.
BGP/RDAP enumeration is secondary evidence and should be labelled as such.

**Gap 4: NTP Pool dynamic nature**: Advisors treated NTP Pool IPs as static.
Online validation clarified that NTP Pool servers require static IPs, but the
DNS distribution mechanism is dynamic and volunteer membership changes. This is
not suitable for a static public reference feed. Operator-specific DNS-based
allowlisting may be useful locally, but baseline public critical-infrastructure
feeds should prefer exact official NTP/time IPs or prefixes.

**Gap 5: Apple Software Update infrastructure**: Minimally covered. Online
validation confirmed Apple operates AS714 (parent) and AS6185 (CDN), with
official guidance for enterprises that only support IP-based firewalls to allow
the entire `17.0.0.0/8` IPv4 block plus three IPv6 blocks. This is authoritative
but broad, so it belongs in soft/contextual critical infrastructure, not hard.
Software Update specifically uses hostnames such as `swcdn.apple.com`,
`swdist.apple.com`, and `updates.cdn-apple.com`.

**Gap 6: Fastly official API**: Mentioned by one advisor but not with the
official endpoint. Online validation confirmed
`https://api.fastly.com/public-ip-list` is the official
no-authentication-required endpoint. Incorporated into §7 tools.

### Searches Performed

| Query | Yield |
|-------|-------|
| Quad9 DNS service 9.9.9.9 ASN AS19281 AS54994 | Confirmed AS19281 = Quad9; AS54994 = unrelated Meteverse Limited |
| Alibaba Cloud AS37963 IP ranges JSON endpoint | Confirmed no official endpoint; community sources identified |
| Tencent Cloud AS45090 IP ranges official documentation | Confirmed no official endpoint; BGP sources identified |
| Let's Encrypt ISRG OCSP CRL infrastructure AS42408 | Confirmed AS42408 = unrelated Russian ISP; OCSP shut Aug 2025 |
| Akamai edge IP ranges AS20940 AS16615 official published list | Confirmed AS16615 does not exist; AS20940 + AS16625 are Akamai ASNs |
| NTP Pool Project pool.ntp.org dynamic IP addresses | Confirmed servers require static IPs; DNS distribution is dynamic |
| SendGrid Mailgun email delivery infrastructure ASNs | Confirmed SendGrid AS11377, Mailgun AS396479 with IP ranges |
| Huawei Cloud AS55990 AS30637 IP ranges | Confirmed AS55990 = Huawei Cloud; AS30637 = Unitas Global |
| Apple Software Update CDN AS6185 AS714 IP address ranges | Confirmed AS714/AS6185; official guidance allows 17.0.0.0/8, but this is broad and should be soft/contextual |
| Stripe payment infrastructure ASNs IP ranges | Confirmed Stripe AS5091/AS394562/AS141743; official docs at stripe.com/ips |
| GitHub AS36459 IP address ranges official documentation | Confirmed official Meta API at api.github.com/meta |
| Salesforce AS14340 IP address ranges official documentation | Confirmed official docs at help.salesforce.com/s/articleView?id=000384438 |
| Okta AS538427 AS539671 IP address ranges | Confirmed neither ASN is Okta; actual ASN is AS19745 |
| Fastly AS54113 IP address ranges official documentation | Confirmed official API at api.fastly.com/public-ip-list |

### Judgment Calls

**JC1: Inclusion of "minor" DNS resolvers**: Advisors varied in how many DNS
resolvers to enumerate. Decision: include publicly documented recursive DNS
resolvers when the operator publishes exact service IPs or prefixes. Core
global resolvers such as Google, Cloudflare, Quad9, and OpenDNS/Cisco are hard
critical. Other public, filtering, encrypted, or regional resolvers are soft
unless the source and service role justify hard treatment. Do not include a
resolver from memory or reputation alone; NextDNS remains pending until a clean
official stable IP/range source is verified.

**JC2: Treatment of Let's Encrypt**: Since Let's Encrypt shut down OCSP in August 2025 and now uses CRL at lencr.org (with no published IP list), the practical recommendation shifts from "exclude Let's Encrypt IPs" to "allow HTTP for ACME challenges." Incorporated as a soft constraint in §8.

**JC3: Alibaba Cloud exclusion set strategy**: Since Alibaba Cloud has no
official IP range endpoint, the exclusion set must not treat third-party JSON as
authoritative. Decision: prefer project-owned BGP/RDAP enumeration if this
coverage is required. Community-maintained sources such as sourcecidr.com and
the GitHub disposable/cloud-ip-ranges repository can be considered only as
secondary cross-checks, with explicit source-quality and license caveats. Flag
this as a tooling gap.

**JC4: Scope creep toward SaaS services**: Several advisors included SaaS services (Slack, Jira, Figma, etc.) in critical infrastructure. Decision: Limit the exclusion set to foundational infrastructure (DNS, CDN, cloud, CA, NTP, update, email, identity) that has broad blast radius. SaaS-specific services belong in the consumer's contextual exclusion set, not the aggregator's baseline. GitHub, Salesforce, and Stripe are included because they are broadly depended-upon development/commerce infrastructure.

**JC5: Dual-semantics providers**: Some providers have both broad contextual
network space and narrower soft service infrastructure. Apple is the concrete
example: `17.0.0.0/8` and Apple's documented IPv6 blocks are official but broad,
while update/service-specific endpoints may deserve soft classification when a
narrow source exists. Decision: do not make one source multi-tier in v1. Split
the provider into separate reference feeds with separate `critical:` metadata so
users can inspect exactly what each warning means.

**JC6: Plain-text authoritative feeds**: Several official sources publish plain
CIDR text rather than JSON/API, including Cloudflare `ips-v4`/`ips-v6`, Zoom
text ranges, and provider `ips.txt` endpoints. Decision: treat these as primary
when official and current, using `authoritative_plain_text` as the source type
instead of degrading them to `authoritative_static_docs` or `secondary`.

### Unresolved Uncertainties

**UNC1: Akamai complete IP range enumeration**: Akamai does not publish a public IP range list. The exclusion set can only be maintained via BGP enumeration of AS20940 and AS16625. The completeness of BGP-based enumeration depends on collector coverage. Worth flagging as a persistent gap.

**UNC2: NTP Pool server IP volatility**: While NTP Pool servers require static
IPs, the DNS-based distribution means the set of IPs serving pool.ntp.org
changes over time as servers join/leave the pool. A static public reference feed
will be incomplete quickly and should not be shipped as a hard critical feed.
DNS-based allowlisting can be useful for an operator's own enforcement policy,
but it needs explicit TTL/freshness semantics and is a different feature from
static public reference feeds.

**UNC3: Public DNS provider gaps**: DNSPod IPv6, 114DNS ASN/service ownership,
DNS.WATCH current official service addresses, and IIJ Public DNS IPv6 remain
unverified. Do not include these in the shipped hard catalog until the exact
official source and current service role are verified.

**UNC4: CI/developer SaaS provider gaps**: Buildkite, Snyk, SonarCloud,
Codecov, and Bitrise documentation was unreachable or inconclusive in the last
research pass. Treat them as pending, not rejected forever.

**UNC5: DNS-derived/static-IP mismatch**: Adyen, TimeNL, Meta NTP, package
registries, Windows Update, and several CA/OCSP/CRL paths are important
dependencies but do not fit the current static IP feed model. They need a
separate DNS-resolution feed type with TTL/freshness metadata or explicit
operator-local policy support.

**UNC6: CA infrastructure IP enumeration**: Many CAs host OCSP/CRL
infrastructure on CDN partners, but transitive CDN coverage alone hides the
reason an overlap matters. Preserve a distinct `certificate_validation` role
when a CA publishes dedicated status IPs or stable status sources, starting with
DigiCert. Extracting OCSP/CRL URLs from certificates in a specific environment
is still the most reliable local method, but it is environment-specific and not
always generalizable to a public aggregator.

**UNC7: IPv6 coverage completeness**: All advisors focused primarily on IPv4. IPv6 critical infrastructure exists and is growing. The exclusion set should include IPv6 equivalents for all listed infrastructure. This remains partially incomplete.

**UNC8: Regional infrastructure criticality**: The document treats critical infrastructure as globally uniform. Some infrastructure is more critical in specific regions (e.g., national time services, regional CDNs). The contextual exclusion set tier is designed to handle this, but the baseline exclusion set may over-protect or under-protect regionally dependent organizations.
