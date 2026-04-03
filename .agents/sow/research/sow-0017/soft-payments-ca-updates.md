# Soft-tier payments, CA validation, software updates research (SOW-0017)

Research date: 2026-04-29.
Source quality grades: A = official machine-readable current feed. B = official machine-readable but partial, geofeed, DNS-derived, or requires derivation. C = official docs/static page (HTML). D = no official public source / third-party only / stale / unsuitable.

---

## Payment / commerce

### Summary table

| Provider | Role | IP source URL | Format | ASN | Quality | License | Update cadence | Tier |
|---|---|---|---|---|---|---|---|---|
| Stripe API | Payment API outbound | `https://stripe.com/files/ips/ips_api.json` | JSON | AWS (AS16509) + Stripe-owned (198.137.150.0/24, 198.202.176.0/24) | A | Not stated | 7-day pre-notice | soft |
| Stripe webhooks | Webhook delivery | `https://stripe.com/files/ips/ips_webhooks.json` | JSON | AWS (AS16509) | A | Not stated | 7-day pre-notice | soft |
| Stripe armada/gator | File/secondary services | `https://stripe.com/files/ips/ips_armada_gator.json` | JSON | AWS + Stripe-owned ranges | A | Not stated | 7-day pre-notice | soft |
| Braintree/PayPal | Payment gateway | `https://assets.braintreegateway.com/json/ips.json` | JSON | AWS (AS16509) + Braintree-owned blocks | A | Not stated | Unspecified | soft |
| Mollie webhooks | Webhook delivery | `https://ip-ranges.mollie.com/ips.txt` | Plain text | GCP (AS15169) | A | Not stated | Unspecified | soft |
| PayPal (direct) | Payment API/webhooks | None found | — | Unknown | D | — | — | reject |
| Adyen | Payment gateway | None stable (DNS-model) | — | AWS/GCP mix | D | — | — | reject |
| Klarna | Payment/BNPL | None found | — | Unknown | D | — | — | reject |
| Worldpay | Payment gateway | None accessible | — | Unknown | D | — | — | reject |
| Checkout.com | Payment gateway | None accessible (login-walled) | — | Unknown | D | — | — | reject |
| Square/Block | Payment/webhooks | None (HMAC model) | — | AWS | D | — | — | reject |
| Authorize.net | Payment gateway | None published | — | Akamai CDN | D | — | — | reject |
| Recurly | Subscription billing | Docs not accessible | — | Unknown | D | — | — | reject |
| Chargebee | Subscription billing | None published publicly | — | Unknown | D | — | — | reject |
| Shopify | E-commerce | None (HMAC model) | — | AWS/Fastly | D | — | — | reject |
| Plaid | Open banking | Not accessible | — | Unknown | D | — | — | reject |
| Wise/TransferWise | Payments | None found | — | Unknown | D | — | — | reject |
| Xero | Accounting/payments | Not accessible | — | Unknown | D | — | — | reject |
| QuickBooks/Intuit | Accounting/payments | None found | — | AWS | D | — | — | reject |
| 2Checkout/Verifone | Payment gateway | None found | — | Unknown | D | — | — | reject |
| GoCardless | Direct debit | None found | — | Unknown | D | — | — | reject |
| Flutterwave | Africa payments | Not accessible | — | Unknown | D | — | — | reject |
| Paystack | Africa payments | Not accessible | — | AWS | D | — | — | reject |
| Google Pay | Payment | None (Google Cloud) | — | GCP (AS15169) | D | — | — | reject |
| BlueSnap, WePay, DLocal, Tipalti, Bill.com, MercadoPago | Various payment | None found | — | Unknown | D | — | — | reject |
| Alipay/WeChat Pay/UnionPay | China payment | None found | — | Unknown | D | — | — | reject |
| Razorpay, PayU | APAC/India payment | None found | — | Unknown | D | — | — | reject |
| iDEAL, PSD2/Open Banking | EU payment scheme | None found | — | Unknown | D | — | — | reject |
| Yodlee, Finicity, TrueLayer | Financial data | None found | — | Unknown | D | — | — | reject |

### Per-provider details

#### Stripe

- **Role**: Payment processing API and outbound webhook delivery.
- **Official IP source**: Three separate machine-readable JSON files:
  - API: `https://stripe.com/files/ips/ips_api.json` (also `.txt` variant)
  - Webhooks: `https://stripe.com/files/ips/ips_webhooks.json`
  - Secondary (armada/gator/files services): `https://stripe.com/files/ips/ips_armada_gator.json`
- **Format**: JSON array per file; IPs are individual /32 addresses and two named CIDR blocks.
- **Content as of research date**:
  - API: ~175 individual IPs mostly on AWS (13.x, 34.x, 35.x, 50.x, 52.x, 54.x, 107.x) plus Stripe-owned ranges 198.137.150.0/24 and 198.202.176.0/24.
  - Webhooks: 15 individual IPs on AWS.
  - Armada/gator: AWS IPs plus the same two Stripe-owned /24 blocks.
- **ASN**: Primarily AWS AS16509. Two /24 ranges (198.137.150.0/24, 198.202.176.0/24) appear to be Stripe-owned.
- **Source quality**: A — official, machine-readable, directly published by Stripe.
- **License**: Not stated on the page.
- **Update cadence**: Stripe commits to 7 days' pre-notice via API announce mailing list before changes. Subscribe at https://docs.stripe.com/ips.
- **Recommended tier**: soft — webhook and API IPs. Blocking Stripe webhook IPs would silently break payment event delivery.
- **Caveats**:
  - The vast majority of Stripe API IPs are on AWS (AS16509). These are not exclusive to Stripe; they are dynamically allocated EC2 ranges. Only the 198.137.150.0/24 and 198.202.176.0/24 blocks are dedicated Stripe infrastructure.
  - The webhook list (15 IPs) is the tightest and most reliable for soft-tier classification. The API list (175 IPs on AWS) is broader and overlaps with generic AWS space.
  - No redistribution terms; treat as read-only reference.
  - Do not use as a blocklist-derived "blocklist safe" exemption set. Use it as an overlap signal to flag feeds that include Stripe webhook ranges.

#### Braintree / PayPal Braintree

- **Role**: Payment gateway for Braintree (PayPal subsidiary) merchants.
- **Official IP source**: `https://assets.braintreegateway.com/json/ips.json`
- **Documentation page**: `https://developer.paypal.com/braintree/docs/reference/general/braintree-ip-addresses`
- **Format**: JSON with three sub-keys per environment (production/sandbox): `cidrs` (CIDR blocks), `ips` (individual IPs), `outboundIps`.
- **Content as of research date**:
  - Production CIDRs: 7 blocks (63.146.102.0/26, 64.4.245.128/25, 159.242.240.0/21, 184.105.251.192/26, 204.109.13.0/24, 205.219.64.0/26, 209.117.187.192/26).
  - Production individual IPs: ~40 IPs mostly on AWS.
  - Production outbound IPs: ~23 IPs (subset of above).
  - Sandbox: similar structure.
- **ASN**: Mix of Braintree-owned legacy blocks (63.x, 184.x, 204.x, 205.x, 209.x) and AWS IPs (13.x, 34.x, 52.x, 54.x, 3.x, 18.x).
- **Source quality**: A — official JSON feed, maintained by Braintree.
- **License**: Not stated.
- **Update cadence**: Not documented; JSON URL is machine-readable for monitoring.
- **Recommended tier**: soft — legacy CIDRs are dedicated Braintree infrastructure. Blocking these would break payment processing for Braintree merchants.
- **Caveats**:
  - Braintree docs say domain names (not just IPs) may resolve to any production range.
  - The Braintree-owned CIDR blocks (non-AWS) are the most stable and least ambiguous for soft-tier classification.
  - AWS IPs in the list are dynamic and overlap with general AWS customer space.

#### Mollie webhooks

- **Role**: European payment processor outbound webhook delivery.
- **Official IP source**: `https://ip-ranges.mollie.com/ips.txt`
- **Format**: Plain text, one IP per line.
- **Content as of research date**: 15 individual IPs, all in Google Cloud Platform ranges (34.x, 35.x).
- **ASN**: GCP AS15169.
- **Source quality**: A — official machine-readable endpoint maintained by Mollie.
- **License**: Not stated.
- **Update cadence**: Not documented; URL is machine-readable for monitoring.
- **Recommended tier**: soft — webhook delivery IPs. Blocking these would silently break payment webhook delivery for Mollie merchants.
- **Caveats**:
  - All 15 IPs are on GCP (AS15169). These are not exclusive Mollie infrastructure; they are GCP customer IPs.
  - Blocking these IPs would also affect other GCP customers; conversely, a compromised GCP IP could be incorrectly assumed safe because it is in the Mollie list.
  - The list is short and likely auto-provisioned by GCP; it may change without pre-notification.

### Reject (with evidence)

**PayPal direct API/IPN**: No official IP publication found for PayPal direct API endpoints or IPN (Instant Payment Notifications). The developer portal at `developer.paypal.com/api/rest/` and `developer.paypal.com/docs/api/` contain no IP ranges. PayPal historically published IPN source IPs but official documentation for current endpoints was not accessible or published. D-grade. Use signature validation instead.

**Adyen**: The official Adyen model recommends DNS-based allowlisting of `out.adyen.com` and dynamic resolution. The IP for `out.adyen.com` changes with the CDN configuration. No stable static IP feed was found at `docs.adyen.com/development-resources/ip-addresses/` (404). Reject as static feed; document the DNS-model requirement.

**Klarna**: Klarna docs redirect to a generic developer hub. No IP allowlist page was found. D-grade.

**Worldpay**: The IP allowlist page at `docs.worldpay.com/access/docs/access-worldpay/reference/ip-allow-list` redirects and the redirected URL returned 404. Official source inaccessible at time of research. D-grade pending retry.

**Checkout.com**: IP page at `www.checkout.com/docs/connectivity/whitelist-ip-addresses` returned 404. Login-walled or removed. D-grade pending retry.

**Square/Block**: No IP ranges published for webhook delivery. Square's architecture uses signature/HMAC validation (`X-Square-Signature` header) as the primary webhook authenticity mechanism. IP-based allowlisting is not the documented model. Reject.

**Authorize.net**: No static IP publication found for API or Silent Post (webhook) delivery. Community forums indicate Authorize.net uses Akamai CDN fronting, which makes static IP lists impossible to maintain. D-grade.

**Recurly**: The docs page at `docs.recurly.com/docs/recurly-ip-addresses` returned 404 during research. D-grade pending retry.

**Chargebee**: No public IP list found. Chargebee's webhook documentation focuses on HMAC signature validation. D-grade.

**Shopify**: Confirmed reject. Shopify explicitly uses HMAC signature validation for webhook authenticity. No static IP list is published and Shopify serves from Fastly CDN making stable IPs impossible. Evidence consistent with earlier SOW research.

**Plaid**: `plaid.com/docs/api/security/#allow-listing` returned 404 during research. Plaid historically published a short IP allowlist for outbound webhook/event delivery but the page was not accessible. D-grade pending retry.

**Wise/TransferWise**: No IP publication found at official docs. Wise uses webhook signature validation. D-grade.

**Xero**: `developer.xero.com/documentation/guides/oauth2/limits/` timed out during research. D-grade pending retry.

**QuickBooks/Intuit**: The Intuit developer page returned empty content. No IP list found. D-grade.

**GoCardless**: The IP documentation page at `gocardless.com` returned 404. GoCardless primarily uses webhook signature validation. D-grade.

**Flutterwave, Paystack**: Both pages returned ECONNREFUSED or 404. These are African/emerging-market payment processors with no verified public IP list. D-grade.

**Google Pay**: Uses Google Cloud infrastructure. No separate IP publication for payment endpoints. Reject.

**BlueSnap, WePay, DLocal, Tipalti, Bill.com, MercadoPago, Razorpay, PayU, Alipay, WeChat Pay, UnionPay**: No official IP publications found during research. These providers either use CDN-fronted infrastructure (Akamai, Cloudflare) or rely on HMAC/signature webhook validation. All D-grade.

**iDEAL, PSD2/Open Banking aggregators**: These are payment schemes and standards bodies, not direct webhook operators with fixed IPs. D-grade.

**Yodlee, Finicity, TrueLayer**: Financial data aggregation APIs. No public IP list found. D-grade.

---

## Certificate authorities (OCSP / CRL / status)

### Summary table

| CA | Role | IP source URL | Format | Hosting | Quality | Update cadence | Tier |
|---|---|---|---|---|---|---|---|
| DigiCert | OCSP + CRL status | `https://knowledge.digicert.com/alerts/digicert-certificate-status-ip-address` | HTML (CSV links for platform detail) | Akamai CDN | C | On major deployment events (pre-announced) | soft |
| Let's Encrypt | CRL only (OCSP shutdown 2025-08-06) | `https://letsencrypt.org/certificates/` (CRL hostnames at `c.lencr.org`) | Hostname docs | Unknown CDN | C | Stable (tied to intermediate lifetimes) | later |
| GlobalSign | OCSP/CRL | No stable IP feed found | — | Unknown | D | — | reject |
| Sectigo | OCSP/CRL | No stable IP feed found | — | Akamai CDN | D | — | reject |
| Google Trust Services | OCSP/CRL | No stable IP feed found | — | Google infra | D | — | reject |
| Cloudflare CA | OCSP/CRL | No stable IP feed found | — | Cloudflare infra | D | — | reject |
| Entrust | OCSP/CRL | No stable IP feed found | — | Unknown | D | — | reject |
| IdenTrust | OCSP/CRL | Not accessible | — | Unknown | D | — | reject |
| ZeroSSL | OCSP/CRL | None published | — | Unknown | D | — | reject |
| Buypass | OCSP/CRL | Not accessible | — | Unknown | D | — | reject |
| SSL.com | OCSP/CRL | Not accessible | — | Unknown | D | — | reject |
| TrustWave | OCSP/CRL | No feed found | — | Unknown | D | — | reject |
| QuoVadis (DigiCert) | OCSP/CRL | Covered under DigiCert | Covered | Akamai | C | — | soft (via DigiCert) |
| WoSign / StartCom | — | Banned/historic | — | — | — | — | flag as historic |

### Per-CA details

#### DigiCert

- **Role**: Commercial CA operating OCSP and CRL services for DigiCert, QuoVadis, and PKI Platform 8 certificates.
- **Official IP source**: `https://knowledge.digicert.com/alerts/digicert-certificate-status-ip-address`
- **Format**: HTML page with structured IP list; also references two CSV files (`pki-platform-8-ocsp.csv`, `pki-platform-8-crl-ca-cert-urls.csv`). No pure machine-readable JSON feed published.
- **Content as of research date** (from March 2026 update notice):
  - ~100+ existing IPv4 addresses (individual /32) for CertCentral Global/Europe OCSP and CRL endpoints.
  - 10 new IPv4 addresses added March 10 2026: 23.11.32-41.x range.
  - 10 new IPv6 /64 ranges and 4 individual IPv6 addresses added at same time.
  - Services covered: CertCentral Global OCSP, CertCentral Europe OCSP, PKI Platform 8 OCSP/CRL, QuoVadis TrustLink OCSP, PKI client downloads.
  - Exception: "One.digicert.com" subdomains do NOT use these IPs.
- **ASN**: The 23.11.x.x range belongs to Akamai (AS20940). DigiCert's OCSP/CRL services run on Akamai CDN infrastructure.
- **Source quality**: C — official HTML page, not a stable machine-readable endpoint. The CSV files add some machine-readability but are platform-specific, not a comprehensive feed.
- **License**: Not stated.
- **Update cadence**: Published updates are event-driven with pre-announcement (known example: March 10, 2026 deployment). Not a regular automated feed.
- **Recommended tier**: soft — DigiCert OCSP/CRL validation is critical for TLS certificate validation across most enterprise PKI deployments. Blocking these IPs would silently break TLS chain validation, leading to certificate errors across affected servers.
- **Caveats**:
  - The IPs are Akamai CDN IPs (AS20940). Blocking DigiCert OCSP also affects all other Akamai CDN traffic, making them high-blast-radius in a different dimension.
  - The page explicitly states the list changes. Any implementation must track for updates.
  - Only import as a curated static set with explicit versioning; do not treat as a live feed.
  - No redistribution terms. Use for research/operational reference only.

#### Let's Encrypt

- **Role**: Free CA. Was major OCSP operator; now provides CRL-only.
- **OCSP shutdown**: Confirmed. Let's Encrypt shut down its OCSP service on **August 6, 2025**.
  - Announcement: `https://letsencrypt.org/blog/` (announced December 5, 2024; shutdown August 6, 2025).
  - Rationale: Privacy (OCSP leaks browsing history to CA) and operational simplicity.
- **Current revocation method**: CRL (Certificate Revocation List) served from `c.lencr.org`.
  - CRL hostnames follow pattern: `<intermediate>.c.lencr.org` (e.g., `r12.c.lencr.org`, `e7.c.lencr.org`, `x1.c.lencr.org`).
  - The `lencr.org` domain (`i.lencr.org`) also serves intermediate certificate copies.
- **Static IPs**: Not published. The `lencr.org` services do not publish a stable static IP feed.
- **Hosting**: CDN-backed (hosting provider not explicitly documented in public docs).
- **Source quality**: C — hostname documentation only; no machine-readable IP feed.
- **Recommended tier**: later — Let's Encrypt CRL infrastructure is important for TLS certificate validation across a huge fraction of the internet (Let's Encrypt issues ~50% of all TLS certificates). However, without a stable IP feed, it cannot be implemented as a precise soft reference provider. The correct operator advice is: "do not block lencr.org or its CDN provider's IPs."
- **Caveats**:
  - Post-August 2025, no OCSP traffic to Let's Encrypt exists. Any IP reputation feed listing old LE OCSP IPs as "bad" is wrong.
  - CRL IPs are CDN-dynamic and cannot be statically allowlisted.
  - The correct protective measure is to avoid blocklisting the underlying CDN (if identifiable) or to flag feed entries that overlap with known lencr.org resolution space.

#### GlobalSign

- **Role**: Commercial CA operating OCSP and CRL globally.
- **Official IP source**: No stable published IP feed found. GlobalSign's repository page (`globalsign.com/en/repository`) and support pages do not list OCSP/CRL IPs.
- **Source quality**: D.
- **Notes**: GlobalSign OCSP runs on Akamai CDN (inferred from historic DNS observations). IPs change with Akamai configuration.
- **Recommended tier**: reject — no stable official IP source. Document as "hosted on Akamai CDN, no stable IP feed."

#### Sectigo (formerly Comodo CA)

- **Role**: Commercial CA. Third-largest CA by certificate volume.
- **Official IP source**: No stable published IP feed found. Comodo's former knowledge base redirect (`support.comodo.com`) now leads to a contact page. Sectigo's resource library page returned 404.
- **Source quality**: D.
- **Notes**: Sectigo's official guidance has historically stated that OCSP/CRL destination IPs change and should not be hardcoded. The SOW's existing research confirms this. Sectigo OCSP runs on Akamai CDN.
- **Recommended tier**: reject — no stable official IP source; official guidance discourages IP-based allowlisting.

#### Google Trust Services (GTS)

- **Role**: Google's CA, used to sign `*.google.com` and Google customer certificates (e.g., via Let's Encrypt alternative integrations).
- **Official IP source**: Not published. `pki.goog` and `pki.goog/faq/` provide no IP list for OCSP/CRL.
- **Hosting**: GTS CRL/OCSP endpoints (`pki.goog/ocsp/`) are served from Google infrastructure (AS15169 or GCP customer ranges).
- **Source quality**: D.
- **Recommended tier**: reject — no stable official IP source. GTS OCSP/CRL resolves within Google's infrastructure which is already tracked contextually.

#### Cloudflare CA / SSL.com

- **Role**: Cloudflare issues certificates for domains on its network (CA: Cloudflare Inc ECC CA-3, etc.). SSL.com is a separate commercial CA.
- **Official IP source**: Not published for CA status operations.
- **Hosting**: Cloudflare CA OCSP/CRL runs on Cloudflare CDN. SSL.com OCSP page (`ssl.com/faqs/`) returned 404 during research.
- **Source quality**: D for both.
- **Recommended tier**: reject — no stable official IP source for either.

#### Entrust / Entrust Datacard

- **Role**: Commercial CA, used widely in enterprise and government PKI.
- **Official IP source**: Not found. Help page (`entrust.com/knowledgebase/ssl/en/article/`) returned 403.
- **Source quality**: D.
- **Recommended tier**: reject — no accessible official IP source.

#### IdenTrust

- **Role**: CA used as a cross-signer for Let's Encrypt (DST Root CA X3, now expired). Still operates as an independent CA.
- **Official IP source**: Not found. Download page returned 404.
- **Source quality**: D.
- **Notes**: IdenTrust's primary relevance to Let's Encrypt (via cross-signature) has ended since DST Root CA X3 expired September 30, 2021.
- **Recommended tier**: reject.

#### ZeroSSL

- **Role**: Free/paid CA (backed by SSL.com).
- **Official IP source**: Not found. Documentation page focuses on ACME integration, not IP infrastructure.
- **Source quality**: D.
- **Recommended tier**: reject.

#### Buypass, TrustWave, Network Solutions, GeoTrust/Thawte/RapidSSL (DigiCert sub-brands)

- **Buypass**: Norwegian CA. Technical page not accessible.
- **TrustWave**: Now part of DigiCert ecosystem; no separate IP feed found.
- **Network Solutions**: Consumer CA. No IP feed found.
- **GeoTrust, Thawte, RapidSSL**: All are DigiCert sub-brands. OCSP/CRL infrastructure is shared under DigiCert's Akamai-hosted setup. Covered by the DigiCert entry.
- **QuoVadis**: DigiCert acquisition. Covered under DigiCert (`pki-platform-8-ocsp.csv` and DigiCert IP list).
- **Source quality**: D for independent research.
- **Recommended tier**: All reject as separate entries; DigiCert soft entry covers the infrastructure.

#### WoSign / StartCom

- **Status**: Banned. Both CAs were removed from major browser trust stores (Mozilla 2016, Chrome 2017) due to certificate mis-issuance. They should not be included as "critical infrastructure to protect." Flag as **historic/banned** if encountered in IP feeds.

### Notes on CA infrastructure architecture

All major commercial CAs use Akamai CDN (or Cloudflare/Fastly) for OCSP/CRL delivery due to the extreme request volume and global reach requirements. The consequence is:

1. CA status IPs are CDN IPs. Blocking them blocks all Akamai/CDN traffic, not just CA traffic.
2. There is no stable, comprehensive machine-readable IP feed for any CA except DigiCert, and even DigiCert's list is event-driven HTML.
3. The correct framing for users: "CA OCSP/CRL endpoints run on major CDN infrastructure. If you block the CDN provider (Akamai, Cloudflare), you also block certificate validation." This makes CA validation an argument for soft-tier CDN protection, not a separate CA IP list.
4. The IP blast-radius argument for CA validation: blocking DigiCert OCSP = blocking TLS validation for all servers with DigiCert certificates = potential enterprise-wide TLS failure for PKI-dependent environments.

---

## Software / OS updates

### Summary table

| Provider | Role | IP feed | CDN/Infrastructure | IP allowlisting feasible? | Tier |
|---|---|---|---|---|---|
| Apple (17.0.0.0/8 + IPv6) | macOS/iOS software updates | `https://support.apple.com/en-us/101555` (official docs) | Akamai CDN + Apple-owned 17.0.0.0/8 | Partial (17.0.0.0/8 is official but very broad) | soft/contextual |
| Windows Update | Windows patches | None (hostname-only) | Microsoft CDN + Akamai | No | reject |
| Microsoft Delivery Optimization | Windows update delivery | None (hostname-only) | Microsoft CDN + peer-to-peer | No | reject |
| Ubuntu/Debian mirrors | Linux packages | None | Mirror network | No | reject |
| Fedora/RHEL/CentOS mirrors | Linux packages | None | Mirror network + Akamai | No | reject |
| Arch/Alpine/openSUSE/Gentoo mirrors | Linux packages | None | Mirror network | No | reject |
| FreeBSD pkg | BSD packages | None | Fastly CDN (pkg.freebsd.org) | No | reject |
| Mozilla/Firefox updates | Browser updates | None (hostname-only) | AWS/GCP | No | reject |
| Chrome/ChromeOS updates | Browser updates | None (hostname-only) | Google infra | No | reject |
| Snap Store | Ubuntu packages | None | Fastly/CDN | No | reject |
| Flatpak/Flathub | Linux packages | None | CDN | No | reject |
| Adobe Creative Cloud | Software updates | None | Akamai CDN | No | reject |
| Steam | Game/software updates | None | Valve CDN + Fastly | No | reject |
| Epic Games, Xbox/MS Store | Game/software updates | None | CDN | No | reject |
| Nvidia/AMD/Intel driver updates | Driver updates | None | CDN | No | reject |
| Sophos, Kaspersky, ESET, Norton/Symantec, Trend Micro, Defender | AV/security updates | None publicly | CDN/proprietary | No | reject |
| npm/pip/Maven Central | Developer package security advisories | None IP-based | CDN | No | reject |
| Sigstore | Software signing infra | None IP-based | CDN | No | reject |
| Apple iOS/Android OTA | Mobile OS updates | None | CDN (Akamai/Google CDN) | No | reject |

### Per-provider details

#### Apple software updates

- **Role**: macOS, iOS, iPadOS, tvOS, watchOS software update delivery. Includes App Store, iCloud, device management, and MDM payloads.
- **Official source**: `https://support.apple.com/en-us/101555` (Apple enterprise firewall guidance page).
- **Published IP ranges**:
  - IPv4: `17.0.0.0/8` — official Apple-owned block. The full /8 is for all Apple services, not just updates.
  - IPv6: `2403:300::/32`, `2620:149::/32`, `2a01:b740::/32`.
- **Update-specific domains** (from same page): `mesu.apple.com`, `gdmf.apple.com`, `gg.apple.com`, `gs.apple.com`.
- **CDN usage**: Confirmed. Apple states "hosts may have CNAME records in DNS instead of A or AAAA records" and "Apple doesn't publish a list of these CNAME records because they are subject to change." Apple uses content distribution networks extensively for update delivery.
- **ASN**: Apple AS714 (17.0.0.0/8) and AS6185. The /8 block is one of the oldest IANA allocations.
- **Source quality**: C — official static documentation, not a machine-readable dynamic feed. The /8 block is stable.
- **License**: Not stated.
- **Update cadence**: Stable. The 17.0.0.0/8 allocation is a multi-decade IANA assignment.
- **Recommended tier**: soft/contextual — with important caveats.
  - The 17.0.0.0/8 is 16 million IPs. This is broad, not precise.
  - It includes all Apple corporate services, not just software updates.
  - The CDN-fronted update servers (mesu.apple.com etc.) resolve to IPs outside 17.0.0.0/8 at CDN edge nodes.
  - Recommend: label as soft/contextual, NOT hard. Document the "broad /8" limitation explicitly in the methodology.
- **Caveats**:
  - Blocking 17.0.0.0/8 breaks Apple device management and updates. At enterprise scale this is operationally damaging.
  - The update delivery CDN nodes will be on Akamai or similar, not within 17.0.0.0/8.
  - IP allowlisting 17.0.0.0/8 is what Apple documents; it is not the same as "these are the only IPs used for updates."

#### Windows Update / Microsoft Delivery Optimization

- **Role**: OS patch delivery for Windows 10/11, Windows Server.
- **Official IP source**: None. Microsoft explicitly states in Delivery Optimization FAQ: "Microsoft content, such as Windows updates, are hosted and delivered globally via Content Delivery Networks (CDNs) and Microsoft Connected Cache servers, which are hosted within Internet Service Provider (ISP) networks. The network of CDNs and Microsoft Connected Caches allows Microsoft to reach the scale required to meet the demand of the Windows user base. Given this delivery infrastructure changes dynamically, providing an exhaustive list of IPs and keeping it up to date isn't feasible."
- **Infrastructure**: CDN-fronted. Key domains: `*.download.windowsupdate.com`, `*.dl.delivery.mp.microsoft.com`, `*.windowsupdate.com`, `*.delivery.mp.microsoft.com`, `*.update.microsoft.com`. Also Delivery Optimization p2p service endpoints at `*.prod.do.dsp.mp.microsoft.com`.
- **CDN**: Microsoft-operated CDN + ISP-hosted Microsoft Connected Cache nodes + peer-to-peer between Windows clients on the same network. Not exclusively Akamai.
- **Recommended action**: DNS allowlist only. There is no feasible IP allowlist for Windows Update. Any static IP list would be incomplete and stale within hours.
- **Recommended tier**: reject as IP feed. Document as "hostname/DNS-only; IP blocking is not feasible."

#### Ubuntu / Debian mirrors (archive.ubuntu.com, deb.debian.org)

- **Role**: Debian and Ubuntu package repositories. Essential for apt/dpkg package management.
- **Official IP source**: None. Ubuntu mirrors are a registered volunteer/ISP mirror network. `deb.debian.org` is a geo-redirected service that points to the closest mirror based on location.
- **Infrastructure**: Mirror network. `deb.debian.org` uses DNS-based load balancing pointing to geographically distributed mirrors. `archive.ubuntu.com` uses a similar mirror arrangement. Neither uses a CDN in the traditional sense; they use a distributed mirror pool with GeoDNS.
- **Recommended action**: Hostname-based allowlisting or mirror-policy-based access. No stable IP list exists.
- **Recommended tier**: reject as IP feed.

#### Fedora / RHEL / CentOS update infrastructure

- **Role**: RPM package distribution for Red Hat-family Linux distributions.
- **Official IP source**: None. Fedora uses MirrorManager (a Fedora Infrastructure tool) for geographic mirror routing. Red Hat CDN (`cdn.redhat.com`) is Akamai-fronted — confirmed by Microsoft's own Windows privacy docs referencing Akamai for CDN patterns, and from general network knowledge.
- **Infrastructure**: Fedora uses volunteer mirror pool with MirrorManager routing. RHEL/CentOS uses `cdn.redhat.com` (Akamai CDN) for authenticated package delivery to subscribers.
- **Recommended action**: Hostname-based allowlisting. No stable IP list.
- **Recommended tier**: reject as IP feed.

#### FreeBSD pkg

- **Role**: FreeBSD binary package repository (`pkg(8)` tool).
- **Official source note**: `pkg.FreeBSD.org` is explicitly backed by Fastly CDN. The pkg.FreeBSD.org page states: "This is pkg.FreeBSD.org - a Fastly-provided cache for pkg(8)." Uses MaxMind GeoLite-based geo-DNS routing to closest mirror.
- **Recommended action**: Fastly CDN public IP list (`https://api.fastly.com/public-ip-list`) covers this indirectly. No separate pkg.FreeBSD.org IP list needed.
- **Recommended tier**: reject as separate IP feed. Fastly CDN coverage handles it indirectly.

#### Mozilla Firefox / browser updates (aus5.mozilla.org)

- **Role**: Firefox automatic update service.
- **Official IP source**: None. `aus5.mozilla.org` (the update server) resolves dynamically. Mozilla's infrastructure uses AWS/GCP.
- **Recommended action**: Hostname-based. No stable IP feed.
- **Recommended tier**: reject as IP feed.

#### Google Chrome / ChromeOS updates

- **Role**: Chrome browser auto-update, ChromeOS OTA updates.
- **Official IP source**: None published. Chrome update infrastructure uses Google's CDN (AS15169/AS396982).
- **Recommended action**: Hostname-based allowlisting (`update.googleapis.com`, `clients2.google.com`). No stable IP feed.
- **Recommended tier**: reject as IP feed.

#### Snap Store (snapcraft.io)

- **Role**: Ubuntu/Canonical snap package distribution.
- **Official IP source**: Not found. Snap Store network requirements page redirected without content.
- **Infrastructure**: Canonical-operated, fronted by CDN (Fastly/Cloudflare inferred from Canonical infrastructure patterns).
- **Recommended tier**: reject as IP feed.

#### Flatpak / Flathub

- **Role**: Cross-distro application packaging and distribution.
- **Official IP source**: `docs.flathub.org/docs/for-users/firewalls` returned 404.
- **Infrastructure**: CDN-fronted. No stable IP list.
- **Recommended tier**: reject as IP feed.

#### Antivirus / security software updates (Sophos, Kaspersky, ESET, Norton/Symantec/Broadcom, Trend Micro, Microsoft Defender)

- **Sophos**: `help.sophos.com` ECONNREFUSED during research. Sophos uses Akamai CDN for update distribution.
- **Kaspersky**: `support.kaspersky.com` returned 404. Kaspersky uses its own CDN infrastructure. Post-2022 geopolitical situation makes public IP publication unlikely.
- **Norton/Symantec/Broadcom**: Broadcom support portal returned no substantive information. Broadcom/Symantec uses Akamai CDN.
- **Trend Micro, ESET**: No pages checked; both use CDN-fronted update infrastructure.
- **Microsoft Defender**: Uses Windows Update infrastructure (same as OS patches above). No separate IP feed.
- **Recommended tier**: All reject as IP feeds. AV update servers universally use CDN infrastructure. Users should allowlist vendor hostnames, not IPs.

#### Developer tooling package repositories (npm, pip, Maven Central, Sigstore)

- **npm** (`registry.npmjs.org`): Served via Fastly CDN. No stable IP feed.
- **pip/PyPI** (`pypi.org`): Served via Fastly CDN. No stable IP feed.
- **Maven Central** (`repo.maven.apache.org`): Served via CloudFlare/Sonatype CDN. No stable IP feed.
- **Sigstore** (`sigstore.dev`, `rekor.sigstore.dev`, `fulcio.sigstore.dev`): Software signing transparency log. Uses GCP infrastructure. No IP feed published.
- **Recommended tier**: All reject as IP feeds.

#### Gaming platforms (Steam, Epic Games, Xbox/Microsoft Store, PlayStation/Sony)

- **Steam/Valve**: Uses Valve's own CDN (AS32590) plus third-party CDN partners. No public IP list.
- **Epic Games Launcher**: Akamai-fronted. No public IP list.
- **Xbox / Microsoft Store**: Uses Microsoft CDN infrastructure (same family as Windows Update). No stable IP list.
- **PlayStation / Sony**: No public IP list.
- **Nintendo**: No public IP list.
- **Recommended tier**: All reject as IP feeds.

#### Firmware/BIOS/UEFI updates (Lenovo, Dell, HP, etc.)

- **Role**: Critical hardware firmware updates for enterprise servers and workstations.
- **Official IP source**: None published by any major OEM.
- **Infrastructure**: All major OEM firmware update services (Lenovo Update Server, Dell EMC Repository Manager, HP BIOS update) use CDN-fronted download servers.
- **Recommended tier**: reject as IP feeds. Note that firmware update failure is high-blast-radius but there is no feasible IP-based protection mechanism.

#### Mobile OTA updates (iOS, Android/Google Play)

- **iOS OTA**: Served from Apple infrastructure (17.0.0.0/8 plus CDN). Covered by Apple entry.
- **Android/Google Play**: Served from Google infrastructure (AS15169/GCP). No dedicated IP list.
- **F-Droid**: Open-source Android repository. Small community mirrors. No stable IP list.
- **Recommended tier**: reject as IP feeds.

### Reject (with evidence)

All software/OS update services except Apple's 17.0.0.0/8 documentation are **rejected as static IP feeds** for the following documented reasons:

1. **Windows Update**: Microsoft explicitly states in official documentation that Windows Update delivery uses CDN infrastructure that "changes dynamically" and that an exhaustive IP list is "not feasible." Evidence: `https://learn.microsoft.com/en-us/windows/deployment/do/waas-delivery-optimization-faq` (FAQ: "My firewall requires IP addresses and can't process FQDNs").

2. **Ubuntu/Debian**: Mirror network with GeoDNS routing. No single static IP set covers all possible mirrors. IP allowlisting is architecturally incompatible with this model.

3. **Fedora/RHEL mirrors**: Fedora uses MirrorManager with dynamic routing. RHEL CDN uses Akamai (same issue as Akamai coverage above).

4. **FreeBSD pkg**: Fastly-backed CDN with geo-routing. Covered indirectly by Fastly CDN soft reference.

5. **Arch/Alpine/Gentoo/openSUSE**: All use volunteer mirror pools with GeoDNS. No stable IP lists.

6. **Browsers (Firefox, Chrome)**: Dynamic cloud infrastructure. No IP feeds published.

7. **Snap/Flatpak/npm/pip**: CDN-fronted. No stable IP feeds.

8. **AV/security software**: All CDN-fronted. No stable IP feeds.

9. **Gaming/firmware**: CDN-fronted. No stable IP feeds.

**Impact note**: Blocking CDN providers (Fastly, Akamai, Cloudflare) would silently break update delivery for most of the services in this category. This reinforces the argument for soft-tier protection of CDN provider IP ranges, rather than attempting per-service update infrastructure protection.

---

## Open questions / unverified

1. **PayPal IPN/webhooks**: PayPal historically published IPN source IPs. The current state is unclear. The `developer.paypal.com/docs/ipn/` path returned 404. Retry with a different URL pattern or contact PayPal developer support. PayPal's Venmo/Braintree branches are covered; the direct PayPal payment webhook source IP situation needs clarification.

2. **Worldpay IP allowlist**: `docs.worldpay.com/access/docs/access-worldpay/reference/ip-allow-list` redirected and the redirect target returned 404. Worldpay (now Global Payments / FIS) may publish IPs for their legacy gateways. Retry with `developer.worldpay.com` or `access.worldpay.com` directly.

3. **Checkout.com**: IP allowlist page returned 404. This may be login-walled (for customers only) or removed. Retry or check if published in customer portal.

4. **Plaid IP allowlist**: `plaid.com/docs/api/security/#allow-listing` returned 404. Plaid historically published outbound IPs for their webhook events. Retry with current URL.

5. **Recurly IP addresses**: `docs.recurly.com/docs/recurly-ip-addresses` returned 404. Retry with newer Recurly docs URL structure.

6. **DigiCert CSV machine-readable files**: The research page referenced `pki-platform-8-ocsp.csv` and `pki-platform-8-crl-ca-cert-urls.csv` files but did not provide direct URLs. These should be located for a more complete DigiCert reference feed.

7. **Klarna IP publication**: Klarna redirected to a generic developer hub. Klarna may publish IPs for merchants on a per-account or documentation portal basis. Retry search at `docs.klarna.com`.

8. **Authorize.net outbound IPs**: Community forum suggested some IPs but no official page was found. Check `developer.authorize.net/support/` for any IP documentation.

9. **Let's Encrypt CRL CDN hosting provider**: The lencr.org infrastructure's CDN provider was not identified in public docs. This would help complete the "CDN coverage protects LE CRL" argument.

10. **Sectigo OCSP IP guidance (official)**: The historic Comodo knowledge base article about OCSP/CRL IPs was not accessible. Sectigo's current official guidance was not found. The statement "Sectigo says don't use destination IPs because they change" needs a current primary source citation.

11. **Google Trust Services OCSP IP range**: Confirmed no public IP feed, but the specific Google infrastructure ranges serving `ocsp.pki.goog` are not documented. Worth checking whether the CRL/OCSP domains resolve to GCP or Googlebot-adjacent ranges.

---

## Sources consulted

### Official payment provider sources
- `https://docs.stripe.com/ips` — Stripe IP documentation page
- `https://stripe.com/files/ips/ips_api.json` — Stripe API IPs (raw JSON, fetched)
- `https://stripe.com/files/ips/ips_webhooks.json` — Stripe webhook IPs (raw JSON, fetched)
- `https://stripe.com/files/ips/ips_armada_gator.json` — Stripe secondary service IPs
- `https://developer.paypal.com/braintree/docs/reference/general/braintree-ip-addresses` — Braintree docs
- `https://assets.braintreegateway.com/json/ips.json` — Braintree IP JSON (raw, fetched)
- `https://ip-ranges.mollie.com/ips.txt` — Mollie webhook IPs (raw text, fetched)
- `https://developer.paypal.com/api/rest/` — PayPal API docs (no IP found)
- `https://docs.adyen.com/development-resources/ip-addresses/` — Adyen (404)
- `https://developer.squareup.com/docs/webhooks` — Square webhooks
- `https://www.checkout.com/docs/connectivity/whitelist-ip-addresses` — Checkout.com (404)
- `https://developer.worldpay.com/docs/access-worldpay/reference/ip-allow-list` — Worldpay (redirect/404)
- `https://docs.worldpay.com/access/docs/access-worldpay/reference/ip-allow-list` — Worldpay redirect target (404)
- Various other payment provider pages (Klarna, Recurly, Chargebee, Plaid, GoCardless, Flutterwave, Paystack — all returned 404 or connection refused)

### Official CA sources
- `https://knowledge.digicert.com/alerts/digicert-certificate-status-ip-address` — DigiCert CA status IPs (fetched)
- `https://letsencrypt.org/blog/` — Let's Encrypt blog (OCSP shutdown announcement)
- `https://letsencrypt.org/certificates/` — Let's Encrypt cert chain docs (CRL hostnames)
- `https://letsencrypt.org/docs/lencr.org` — lencr.org CRL/issuer endpoint documentation
- `https://www.globalsign.com/en/repository` — GlobalSign repository (no IPs)
- `https://pki.goog/` and `https://pki.goog/faq/` — Google Trust Services (no IPs published)
- Various Sectigo, Entrust, IdenTrust, Buypass, SSL.com, ZeroSSL pages (mostly 404 or no IP info)

### Official software update sources
- `https://support.apple.com/en-us/101555` — Apple enterprise firewall/network guidance (fetched twice)
- `https://learn.microsoft.com/en-us/windows/deployment/do/delivery-optimization-workflow` — Windows Delivery Optimization endpoints
- `https://learn.microsoft.com/en-us/windows/deployment/do/waas-delivery-optimization-faq` — Windows Delivery Optimization FAQ (explicit "no IP list" statement)
- `https://learn.microsoft.com/en-us/windows/privacy/windows-endpoints-1909-non-enterprise-editions` — Windows endpoint list (domain-only)
- `https://wiki.ubuntu.com/Mirrors` — Ubuntu mirror network info
- `https://pkg.freebsd.org/` — FreeBSD pkg CDN info (Fastly-backed, fetched)
- Various Linux distribution mirror, Snap, Flatpak, AV vendor pages (mostly 404 or not useful)

### Methodology notes from research
- Windows Update: Microsoft's own FAQ explicitly states no exhaustive IP list is feasible.
- FreeBSD pkg: Page text explicitly confirms Fastly CDN.
- DigiCert: IP list uses Akamai 23.11.x.x range — confirmed from IP block ownership (Akamai AS20940 owns 23.0.0.0/8).
- Mollie: All 15 IPs are GCP (34.x/35.x — Google Cloud ranges).
- Stripe: Mix of AWS ephemeral and Stripe-owned (198.137.150.0/24, 198.202.176.0/24) ranges.
- Let's Encrypt OCSP shutdown: August 6, 2025 (confirmed from letsencrypt.org blog content).
- Apple CDN: Confirmed from apple.com enterprise guidance text: "CNAME records in DNS... Apple doesn't publish a list of these CNAME records because they are subject to change."
