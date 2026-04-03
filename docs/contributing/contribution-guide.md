# Contribution Guide

You will learn how to contribute new feeds and improvements to the update-ipsets catalog.

## Process overview

1. Fork the repository
2. Add or modify YAML configuration files
3. Test locally with the daemon
4. Submit a pull request

## What reviewers check

When you submit a new feed, reviewers verify:

- The URL works and returns IP data
- The feed produces valid IPs after processing
- The license is compatible (see [License Requirements](license-requirements.md))
- Attribution text is correct and complete
- The YAML file is in the correct directory with correct fields
- The feed does not duplicate an existing entry
- Category and metadata are appropriate

## YAML file placement

```
configs/firehol/
  sources/           # Direct source feeds
    <category>/      # Category subdirectory (e.g., web_reputation, abuse)
      <feed>.yaml    # One file per feed
  merges/            # Merge feeds
    <name>.yaml      # One file per merge
  artifacts/         # Artifact parents
    <name>.yaml      # One file per artifact
  runtime.yaml       # Runtime settings
  categories.yaml    # Category definitions
```

## Source feed fields

A typical source feed YAML contains:

```yaml
name: my_new_feed
url: https://example.com/blocklist.txt
frequency: 3600
output: ipset
category: web_reputation
maintainer: [Example Corp]
license: CC-BY-SA-4.0
redistributable: true
attribution: |
  Data provided by Example Corp.
  https://example.com/terms
processors:
  - strip_comments
  - strip_blank_lines
  - cidr_expand
```

## Merge feed fields

```yaml
name: my_merge
sources:
  - feed_a
  - feed_b
exclude:
  - whitelist_feed
frequency: 3600
output: netset
category: combined
maintainer: [Curator Name]
license: multiple
redistributable: true
```

## Testing locally

1. Start the daemon with your config:
   ```bash
   update-ipsets daemon --config configs/firehol --enable-all --listen :18888 \
     --admin-auth-mode=disabled --allow-unauthenticated-admin
   ```

2. Check the admin UI at `http://localhost:18888/admin`
3. Verify the feed appears in the catalog
4. Wait for the first download cycle or trigger a recheck from the admin UI
5. Confirm the feed produces IPs and has a `healthy` status

## Submitting

Push your changes to your fork and open a pull request. Include:

- The feed URL and what it contains
- Why it is useful for the catalog
- Any license or attribution notes
- Confirmation that you tested it locally
