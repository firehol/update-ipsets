# update-ipsets Operator Manual

Welcome to the operator manual for **update-ipsets** — a tool that downloads, normalizes, compares, and publishes public IP-based threat and blocking feeds.

## What you'll find here

This manual covers everything you need to deploy, configure, monitor, and maintain update-ipsets:

- **Getting started** — install and run your first instance in minutes
- **Configuration** — the YAML catalog, runtime settings, feed families, and all options
- **Running** — daemon flags, systemd, TLS, authentication, listeners
- **Pipeline** — how feeds flow from download to published output
- **Admin UI** — runtime status, feed inventory, operator actions
- **API reference** — all public endpoints, rate limits, and response formats
- **Monitoring** — OpenTelemetry, Netdata integration, log structure
- **CLI tools** — iprange, query, and enable subcommands
- **Troubleshooting** — common issues and how to fix them
- **Catalog maintenance** — how operators add and validate local catalog feeds

## Reading order

New to update-ipsets? Start here:

1. [About update-ipsets](about-update-ipsets.md) — what it does and why
2. [Quick Start](quick-start.md) — get running in 5 minutes
3. [Installation](installation/installation.md) — production deployment
4. [Configuration Concepts](configuration/configuration-concepts.md) — how the catalog works
5. [Feed Families](feeds/feed-families.md) — the six feed families
6. [Pipeline Overview](pipeline/pipeline-overview.md) — how data flows

Then branch to the sections you need.

## For catalog operators

If you maintain a local feed catalog:

1. Read [Feed Families](feeds/feed-families.md) to pick the right type
2. Follow [Step by Step: Add a Feed](contributing/step-by-step-add-feed.md)
3. Check [License Requirements](contributing/license-requirements.md) before publishing redistributed data

## Need help?

- Check [Troubleshooting](troubleshooting/common-issues.md) for common problems
- Browse the [Glossary](glossary.md) for term definitions
- Read the [API Reference](api/api-overview.md) for endpoint details
