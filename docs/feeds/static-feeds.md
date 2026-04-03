# Static Feeds

You will learn how to define a curated IP/CIDR list directly in YAML, when static feeds are the right choice, and how they behave with `frequency: 0`.

## What a static feed is

A static feed provides its IP/CIDR data directly in the `static:` field of the source YAML. There is no upstream URL to download. The data lives in the configuration file itself.

Use static feeds for small curated reference lists that operators should be able to customize without rebuilding the binary.

## When to use a static feed

- Small curated lists (dozens or hundreds of entries, not millions)
- Reference data that operators may need to edit locally
- Critical infrastructure addresses that rarely change
- Baseline bogon or reserved ranges

Do not use static feeds for large dynamic threat intelligence — use a source feed with a URL instead.

## Example

```yaml
sources:
  critical_public_dns_core:
    license: Curated static reference from provider-published public DNS documentation
    redistributable: false
    label: Core public DNS resolvers
    static:
      - 1.1.1.1
      - 1.0.0.1
      - 8.8.8.8
      - 8.8.4.4
      - 9.9.9.9
      - 149.112.112.112
      - 208.67.222.222
      - 208.67.220.220
    frequency: 0
    ipv: ipv4
    output: netset
    processor:
      - passthrough
    processor_raw: passthrough
    category: provider_infrastructure
    provenance: primary
    info: Curated IPv4 service addresses for Cloudflare 1.1.1.1, Google Public DNS, Quad9, and OpenDNS public resolvers.
    maintainer: FireHOL
    maintainer_url: https://iplists.firehol.org/
    enabled_by_all: true
    use: [critical_infrastructure]
    critical:
      tier: hard
      role: public_dns_core
      source_type: curated_static
      source_quality: C
      rationale: Core public recursive DNS resolver anycast addresses; blocking them can immediately break name resolution.
```

## Frequency behavior

Set `frequency: 0` on a static feed. The feed is not auto-scheduled by wall-clock cadence.

However, the scheduler still detects configuration changes. When the `static:` body in the YAML changes (because an operator edited the file and reloaded), the scheduler compares the materialized source body with the current config and queues the source for reprocessing.

This means: editing a static feed's IP list and sending SIGHUP triggers reprocessing automatically.

## Static entries parse at validation time

For critical infrastructure static feeds, every entry in `static:` parses as an IPv4 address or IPv4 CIDR at config-validation time. Invalid entries cause the entire configuration to be rejected before the daemon starts.

## No compiled-in lists

Static feeds are YAML data, not Go code. Operators can edit the IP/CIDR list in the installed catalog without rebuilding. The `static:` field is the supported way to provide small operator-customizable reference data.
