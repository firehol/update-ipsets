# Bogon classification

How we identify the IPs in each feed that fall in reserved or otherwise
non-routable IPv4 space, and how that feeds the ASN breakdown.

## What it shows

The public product presents bogon information in two ways:

- the feed-detail Bogons section
- the derived `bogon_present` insight rule

Together they answer:

- how many IPs in the feed overlap reserved/non-routable space
- which RFC-reserved ranges are involved
- whether third-party bogon lists agree with the authoritative baseline

A bogon is an IPv4 address whose presence on the public internet is, by definition, a misconfiguration. RFC 1918 private space (10/8, 172.16/12, 192.168/16), loopback (127/8), link-local (169.254/16), multicast (224/4), and the various test/documentation ranges (TEST-NET-1/2/3) are the most common cases. Their appearance in a public threat feed is a fact users may want to weigh: it usually points to an upstream collection bug, not a real attacker, and blocking them at the perimeter is a no-op for traffic that already cannot be routed.

## Data sources

Bogon classification draws from trusted bogon sources:

- the authoritative RFC-reserved baseline
- any trusted bogon reference feed included by the site

Only those marked sources participate in the bogon union used by ASN
attribution.

| Format | What it is | What it covers |
|---|---|---|
| RFC-reserved baseline | Always-present RFC-defined reserved IPv4 ranges. | The 15 ranges listed below. |
| Plain bogon reference | A trusted source such as `bogons`, `fullbogons`, or `iblocklist_bogons`. | Whatever the source currently contains. |
| Derived bogon reference | A trusted bogon-family source derived from other references. | The latest successfully published derived set. |

The UI renders:

- one authoritative RFC-reserved block
- one cross-reference table for every other available bogon source

### The authoritative RFC reserved baseline

The baseline is shipped with the product and never depends on a network
download. It contains exactly these 15 ranges:

| CIDR | Reserved for | RFC |
|---|---|---|
| `0.0.0.0/8` | Current network | RFC 1122 section 3.2.1.3 |
| `10.0.0.0/8` | RFC 1918 private (10/8) | RFC 1918 |
| `100.64.0.0/10` | Carrier-grade NAT | RFC 6598 |
| `127.0.0.0/8` | Loopback | RFC 1122 section 3.2.1.3 |
| `169.254.0.0/16` | Link-local | RFC 3927 |
| `172.16.0.0/12` | RFC 1918 private (172.16/12) | RFC 1918 |
| `192.0.0.0/24` | IETF protocol assignments | RFC 6890 |
| `192.0.2.0/24` | TEST-NET-1 | RFC 5737 |
| `192.88.99.0/24` | 6to4 relay anycast (deprecated) | RFC 7526 |
| `192.168.0.0/16` | RFC 1918 private (192.168/16) | RFC 1918 |
| `198.18.0.0/15` | Network benchmarking | RFC 2544 |
| `198.51.100.0/24` | TEST-NET-2 | RFC 5737 |
| `203.0.113.0/24` | TEST-NET-3 | RFC 5737 |
| `224.0.0.0/4` | IPv4 multicast | RFC 5771 |
| `240.0.0.0/4` | Reserved for future use | RFC 1112 |

The baseline is intentionally curated: every entry must cite the RFC that
defines the reservation.

## How counts are computed

For each available bogon provider, on every relevant processing run:

1. Compare the feed's IP ranges with the provider's bogon ranges.
2. Count the overlapping IPs.
3. For the authoritative RFC-reserved provider, also show the per-RFC-range
   breakdown.

| Field | Meaning |
|---|---|
| `feed_ips` | Total IPs in the feed |
| `bogon_ips` | IPs that fell in this provider's reference set |
| `percent` | `bogon_ips / feed_ips * 100` |
| `by_range` | Optional. Per-range breakdown, only populated for the rfc_reserved provider where each range has a known label and CIDR. |

## How this feeds the ASN breakdown

The ASN breakdown reports a three-bucket split per feed, per ASN provider:

```
feed_ips == attributed_ips + bogon_ips + unknown_ips
```

- **attributed_ips** — IPs that the ASN database has a real record for
- **bogon_ips** — IPs that fall in the union of every trusted bogon provider
  (including the authoritative RFC baseline)
- **unknown_ips** — IPs that are NOT bogon AND have no MMDB record

Computing the bogon union once and using it before ASN attribution keeps the
three ASN buckets stable across providers and across runs even when third-party
bogon feeds disagree at the edges.

## Current product usage

- The feed-detail Bogons section shows the authoritative RFC-reserved
  breakdown.
- The cross-reference table reuses ordinary pairwise comparison rows against the
  other available bogon feeds.
- The `bogon_present` insight rule takes the **maximum** bogon share across
  available bogon providers.
- If a trusted bogon provider is unavailable, that provider's comparison is
  missing rather than guessed on the fly.

## Related

- [ASN attribution](/methodology/asn-attribution) — how the bogon union changes the unknown bucket
- [Critical infrastructure reference feeds](/methodology/infrastructure-asns)

## Edge cases

- If an external bogon source has not produced a usable local file yet, it is
  skipped and the rest of the run continues.
- The authoritative RFC-reserved baseline is always available locally.
- Third-party bogon feeds can disagree about unallocated space or freshness at
  the edges. The product exposes that disagreement instead of hiding it.
