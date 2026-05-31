# Catalog Maintenance Guide

You will learn how to maintain a local update-ipsets catalog: where YAML files live, what to check before enabling a feed, and how to validate the result with the daemon.

## Maintenance workflow

1. Add or modify YAML configuration files under the catalog directory.
2. Validate the feed through the daemon and admin UI.
3. Confirm public API and raw-download behavior.
4. Record license and attribution details before publishing redistributed data.

## What to verify

Before relying on a new or changed feed, verify:

- The URL works and returns IP or CIDR data.
- The configured processor pipeline produces valid IP ranges.
- The category matches the feed's operational meaning.
- The license and attribution fields match the direct upstream's terms.
- The feed does not duplicate an existing catalog entry.
- Public raw downloads are allowed only when `redistributable` is true.

## YAML file placement

```text
configs/firehol/
  sources/
    <category>/
      <feed>.yaml
  merges/
    <name>.yaml
  artifacts/
    <name>.yaml
  runtime.yaml
  categories.yaml
```

## Source feed example

```yaml
sources:
  my_new_feed:
    url: https://example.com/blocklist.txt
    frequency: 60
    ipv: ipv4
    output: ipset
    category: malware_infrastructure
    maintainer: Example Corp
    maintainer_url: https://example.com/
    license: CC-BY-SA-4.0
    redistributable: true
    attribution: |
      Data provided by Example Corp.
      https://example.com/terms
    info: '[Example Corp](https://example.com/) example blocklist'
    processor:
      - remove_comments
      - extract_ipv4_cidr
```

## Merge feed example

```yaml
merges:
  my_merge:
    frequency: 60
    ipv: ipv4
    output: netset
    category: intrusion
    maintainer: Local Catalog Operator
    license: multiple
    redistributable: true
    sources:
      - feed_a
      - feed_b
    exclude:
      - whitelist_feed
```

## Local validation

Start the daemon with your catalog:

```bash
update-ipsets daemon --config configs/firehol --enable-all --listen :18888 \
  --admin-auth-mode=disabled --allow-unauthenticated-admin
```

Then check:

1. The feed appears in the admin UI at `http://localhost:18888/admin`.
2. Recheck completes without download or processing errors.
3. The feed appears in the public catalog: `curl http://localhost:18888/api/v1/sets/<name>`.
4. Raw data is available only when redistribution is allowed: `curl http://localhost:18888/api/v1/sets/<name>/data`.

## See also

- [Step by Step: Add a Feed](step-by-step-add-feed.md)
- [License Requirements](license-requirements.md)
- [YAML Field Reference](../feeds/yaml-field-reference.md)
