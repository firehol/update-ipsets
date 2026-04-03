# Critical infrastructure overlap present

This insight reports that a feed overlaps at least one critical-infrastructure reference range.

The insight is about operational risk, not feed guilt. It means a blocking policy based on this feed may affect shared services, so the overlap deserves review before enforcement.

## What counts as important

The insight treats levels differently:

- Hard overlap is always important when the feed is non-empty. A single public DNS or root-DNS service address can be operationally significant.
- Soft overlap needs enough volume to be visible as a meaningful pattern, because shared service ranges are larger and can include customer-facing traffic.
- Contextual overlap is shown separately for useful but coarse references, such as tenant-mixed CI runner ranges or broad provider guidance.
- Broad cloud and hosting provider ranges are provider context, not critical-infrastructure warning truth, so they should not drive this insight by themselves.

## Threshold

- Hard tier: any positive overlap on a non-empty feed.
- Soft or contextual tiers: at least 100 IPs in the feed, and either at least 10 overlapping IPs or at least 0.01% overlap.

The threshold prevents broad provider ranges from creating noise on tiny feeds while still surfacing material cloud/CDN overlap.

## How to interpret it

- Hard: investigate immediately, even for a very small count.
- Soft: review the provider role and decide whether the feed purpose justifies the overlap.
- Contextual: treat it as local-policy information. A cloud-hosting overlap may be expected for some abuse feeds and unacceptable for others.
- Provider context: use it for collateral-risk analysis outside this insight; it is deliberately separated from critical warning truth.

On feed pages, matched reference feeds should be read in criticality order first, then by matched IP count within the same level. Size alone is not the risk model.

## When this insight can be wrong or incomplete

- Missing reference coverage can produce false negatives.
- Contextual and ASN-context signals can produce warnings that require operator judgment rather than automatic rejection.
- A provider range can include both legitimate shared services and malicious customer workloads.
- Provider-context ranges are useful but intentionally excluded from this insight because broad cloud/customer-hosting ranges are too noisy as critical warning truth.
- IPv6 critical infrastructure is not covered in this release.

## Related

- [Critical infrastructure overlap](/methodology/infrastructure-asns)
