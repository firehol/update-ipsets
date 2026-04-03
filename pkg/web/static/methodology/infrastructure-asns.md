# Critical infrastructure overlap

Critical infrastructure overlap answers one practical question:

> If someone blocks this feed, could they accidentally block services that many
> legitimate users, networks, or businesses depend on?

It is not an allowlist. It is not proof that a feed is wrong. It is a blast
radius signal for operators who need to decide whether a feed is safe to use in
blocking policy.

## Why this is difficult

There is no complete public database of "critical infrastructure IPs".

Important services are often hidden behind CDNs, cloud front doors, DNS CNAMEs,
regional service tags, and third-party platforms. Some providers publish exact
machine-readable ranges. Some publish only broad network guidance. Some publish
nothing stable. Some ranges mix provider-operated services with
customer-controlled workloads, where abuse can be real.

This means the useful model is not "all critical infrastructure". That would be
false. The useful model is a conservative set of reference ranges with clear
levels, clear caveats, and visible omissions.

## Levels

| Level | Meaning | How to treat it |
|---|---|---|
| Hard | Small, explicit service addresses where even one match can break basic internet functions. | Investigate immediately. Count is less important than what matched. |
| Soft | Service-specific ranges for important providers, usually from official machine-readable sources. | Review carefully before broad blocking. Abuse can still occur, but the collateral risk is high. |
| Contextual | Useful but coarse signals, such as tenant-mixed CI runner ranges or very broad provider guidance. | Treat as local-policy context, not as proof that the feed is bad. |
| Provider context | Broad cloud or hosting provider ranges. | Useful for collateral-risk analysis, but not counted as critical-infrastructure warning truth. |
| ASN context | A separate secondary signal for a small number of service-owned ASNs. | Use only as fallback context when exact IP feeds are incomplete. |

Hard is intentionally narrow. A feed matching one public DNS resolver can be
more dangerous operationally than a feed matching thousands of generic hosting
addresses.

## What is covered

### Public DNS and DNS infrastructure

Hard references include core public recursive DNS service addresses, DNS root
server IPv4 service addresses, and AS112 DNS sink prefixes.

These are hard because they are small, explicit, and foundational. Blocking them
can break name resolution, resolver bootstrap paths, or DNS hygiene.

### CDN and edge infrastructure

Soft references include provider-published Cloudflare, Fastly, and AWS
CloudFront ranges. Akamai is included from a secondary source because an
equivalent official edge-range feed was not available.

These ranges front software distribution, package registries, SaaS, media,
commerce, APIs, and security services. They are not hard because shared edge
providers can also carry customer-controlled traffic.

### Developer and deployment platforms

Soft references include GitHub core service ranges and other developer-control
planes such as Atlassian Cloud and HCP Terraform.

GitHub Actions and Codespaces ranges are contextual, not soft, because they
include ephemeral customer-controlled compute workloads. That split is
deliberate: GitHub's official Meta API separates service categories, and
flattening them would exaggerate the "GitHub core service" surface.

### Identity, productivity, SaaS, and CRM

Soft references include Microsoft 365, Salesforce Hyperforce, Okta, Auth0, and
Zoom where provider-published IPv4 ranges exist.

These services are operationally important because blocking them can break
authentication, email, collaboration, CRM, support, meetings, or business
automation. They are soft because the ranges are service-specific but still not
evidence that every address is harmless.

### Payments and commerce

Soft references include Stripe API, Stripe webhook, Braintree, and Mollie
ranges where provider-published IP data exists.

Payment and webhook ranges are included because accidental blocking can disrupt
checkout, payment state changes, fraud workflows, and commerce automation.

### Software updates

Apple's documented IPv4 service network guidance is included as contextual
coverage. It is broad, so it is not hard or soft.

Windows Update, Ubuntu, Samsung, and many package registries are deferred. They
are important, but many of these systems are DNS/CDN-fronted and do not publish
complete stable IP lists that can be represented safely as static reference
feeds.

### Provider context

Broad AWS, Google Cloud, Oracle Cloud, DigitalOcean, Linode/Akamai Cloud, and
Vultr ranges are published as provider context, not as critical-infrastructure
warning truth.

This is a key balance decision. These ranges are useful to understand where a
feed touches cloud or hosting providers, but they also include large amounts of
customer-controlled space. Treating them as critical warnings would create too
many false positives.

### ASN context

ASN context is separate from reference-feed overlap. It is used only for a small
curated set of service-owned networks where ASN-wide attribution can help reveal
blast-radius risk and exact IP feeds may be incomplete.

Broad hyperscaler or customer-hosting ASNs are excluded from this signal. ASN
context is coarse by nature and should never replace exact reference ranges.

## Strengths

- The main signal uses explicit IP/CIDR reference feeds, not broad name matching.
- Hard, soft, contextual, provider-context, and ASN-context signals are kept
  separate so large cloud ranges do not bury small but serious DNS matches.
- Official provider sources are preferred when they exist.
- Secondary sources are used only where the absence of official data is itself
  part of the caveat.
- Each category has a reason, a level, and a known interpretation limit.

## Weaknesses

- Coverage is incomplete by design. A feed can miss this warning and still
  contain important infrastructure that is not represented yet.
- Provider data can lag reality, omit regions, or describe only part of a
  product.
- CDN and SaaS ranges can include both important legitimate traffic and abused
  customer-controlled traffic.
- Broad provider context and ASN context are coarse signals. They need local
  operator judgment.
- The current overlap comparison is IPv4-only.
- Critical infrastructure can still be compromised, proxied, misconfigured, or
  abused. A match is not a verdict of innocence.

## Missing coverage

Some important areas are missing because the available data is too weak for the
current model:

- IPv6 critical infrastructure coverage.
- Dynamic DNS-derived service endpoints that need TTL and freshness semantics.
- Windows Update and many OS/package update systems without exhaustive stable IP
  feeds.
- Ubuntu and other mirror ecosystems where operator-selected mirrors make one
  global static list misleading.
- Samsung and many mobile/IoT update paths without suitable public range feeds.
- CA validation endpoints that ride CDNs and are only covered transitively where
  the CDN itself is represented.
- NTP Pool, because its operators explicitly rely on dynamic pool membership.
- Cloud metadata service addresses, because they are local control-plane
  addresses, not public internet feed content.
- Alibaba Cloud, Tencent Cloud, Huawei Cloud, and similar providers where broad
  BGP/ASN enumeration would recreate the false-positive problem without a
  trustworthy official feed.
- Services such as HubSpot and many SaaS vendors where no sufficiently specific,
  stable, public IP range feed was selected.

## How to use it

- Hard match: inspect before blocking, even if the count is tiny.
- Soft match: review the service, the feed purpose, and the operational impact.
- Contextual match: use local policy; the overlap may be expected or may be too
  risky for your environment.
- Provider context: use it to understand hosting/cloud exposure, not as a
  critical-infrastructure failure.
- ASN context: treat it as a secondary hint that exact reference feeds may not
  tell the whole story.

The safest interpretation is conservative: critical infrastructure overlap is a
warning about operational blast radius, not a decision engine.

## Related

- [Critical infrastructure overlap present](/methodology/infrastructure-present)
- [ASN attribution](/methodology/asn-attribution)
- [Bogon classification](/methodology/bogon-classification)
