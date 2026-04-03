# Critical Infrastructure Reference Feeds

This project models critical infrastructure as configurable reference feeds.

Operators can add, remove, or override these sources in config without patching
or rebuilding the Go binary.

Broad provider and customer-hosting ranges are modeled separately with
`use: [provider_context]`. They publish as normal context feeds but are not used
as critical-infrastructure warning truth.

## Configuration

Use a normal source or merge with `use: [critical_infrastructure]` and a
typed `critical:` block.

Do not combine `critical_infrastructure` with `provider_context`, `bogons`,
`asn`, or `geoip`. Critical infrastructure reference feeds must produce normal
IPv4 set artifacts. The provider names `providers` and `infrastructure` are
reserved, and public feed names must not collide with generated
`<feed>_critical_*` artifact names.

Static curated data belongs in YAML:

```yaml
sources:
  critical_public_dns_core:
    redistributable: false
    static:
      - 1.1.1.1
      - 8.8.8.8
    frequency: 0
    ipv: ipv4
    output: netset
    processor: [passthrough]
    use: [critical_infrastructure]
    critical:
      tier: hard
      role: public_dns_core
      source_type: curated_static
      source_quality: C
      rationale: Core public recursive DNS resolver addresses are operational dependencies.
```

Downloaded reference feeds use `url:` instead of `static:`.

Shipped critical reference feeds should be public as metadata and overlap
inputs, but not automatically public as raw downloadable feeds. Set
`redistributable: false` unless the source has been reviewed and there is an
explicit decision to publish or compose the raw reference body.

For `critical_infrastructure` sources, every `static:` line must be a valid IPv4
address or IPv4 CIDR. Put human comments in YAML comments, not inside the static
line value.

Do not use the legacy top-level `infrastructure_asns` list. Current configs
must model critical infrastructure as reference feeds so operators can review
and customize the exact IP space being checked.

Use top-level `critical_asn_context` only for the separate secondary ASN signal.
It is intentionally narrow and validation rejects known broad hyperscaler or
customer-hosting ASNs. This signal is context, not a replacement for reference
feed overlap.

The current overlap implementation is IPv4-only. IPv6 critical-infrastructure
sources must wait for the IPv6 overlap writer so the public artifacts do not
claim unsupported coverage.

## Generated Artifacts

For every comparable IPv4 public feed, processing writes:

- `<feed>_critical_infrastructure.json`
- `<feed>_critical_<provider>.json` for each loaded provider

The aggregate artifact includes a `provider_set_id`, `complete`, and
`missing_providers` fields. Each per-provider artifact also includes the same
`provider_set_id`.

The provider-set ID is a fingerprint of the configured reference providers,
their acquisition/processing-shape config, and their processed range content.
When `critical_asn_context` is configured, it also fingerprints configured ASN
provider source shape and processed content because aggregate payloads may
include ASN-context matches. It intentionally excludes local processing
timestamps and version counters so no-op refreshes do not create a scheduler
loop. If a provider is added, removed, retagged, changes URL/static/processor
configuration, or its processed ranges change, the next scheduler pass forces a
provider refresh and regenerates critical-overlap artifacts. Public API and
direct JSON routes reject files whose provider-set ID no longer matches the
current config.
Removed-provider JSON artifacts are deleted during the rebuild.

Reference feeds do not get self-overlap artifacts.

Provider-context feeds also do not get critical-overlap artifacts. They remain
ordinary public context feeds so operators can inspect broad provider exposure
without turning cloud/customer-hosting space into critical warning truth.

## Public API

- `/api/v1/sets/{name}/infrastructure`
- `/api/v1/sets/{name}/infrastructure/providers`
- `/api/v1/sets/{name}/infrastructure/{provider}`

These endpoints serve already-published files only.

When a feed also matches configured critical ASN context and an ASN attribution
artifact is available, the aggregate payload includes an `asn_context` object.
This object is a secondary hint and does not increase `critical_ips`.

Critical reference provider metadata is public through the provider-list
endpoint. Raw feed-body routes such as `/api/v1/sets/{provider}/data`,
`/files/{provider}.netset`, `/{provider}.netset`, and `/api/v1/compose` must
still enforce `redistributable`; the shipped catalog marks critical reference
providers non-redistributable by default.

## Operator Notes

- `hard` means blocking the overlap is likely to break core infrastructure.
- `soft` means the overlap deserves review before operational use.
- `contextual` means policy depends on the operator's environment.
- The site does not automatically label a feed good or bad.
