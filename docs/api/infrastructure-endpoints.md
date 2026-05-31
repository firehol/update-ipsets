# Infrastructure Endpoints

You will learn how to retrieve critical-infrastructure overlap data for feeds, list configured providers, and get per-provider detail.

## What critical-infrastructure overlap is

Critical-infrastructure overlap measures how much a threat feed intersects with known critical infrastructure reference sets like DNS root servers, root certificate authorities, and similar essential services.

Overlap is categorized into tiers:

- **Hard** — direct matches against critical reference feeds (e.g., DNS root servers)
- **Soft** — matches against important but not strictly critical infrastructure
- **Contextual** — matches against broad provider/customer hosting space (policy-dependent, not proof of a problem)

## Aggregate overlap summary

```
GET /api/v1/sets/{name}/infrastructure
```

Returns the aggregate critical-infrastructure overlap summary for one feed. Includes total matched IPs per tier, matched reference feeds, and completeness status.

Example:

```
GET /api/v1/sets/firehol_level1/infrastructure
```

Key response fields: `provider_set_id`, matched reference feeds per tier, `complete` flag, `missing_providers` list.

## Configured providers

```
GET /api/v1/sets/{name}/infrastructure/providers
```

Returns the list of configured critical-infrastructure reference providers. This endpoint exposes provider metadata even when per-provider overlap artifacts are not yet materialized.

Example:

```
GET /api/v1/sets/firehol_level1/infrastructure/providers
```

Key response fields: array of provider objects with name, description, and tier classification.

## Per-provider overlap detail

```
GET /api/v1/sets/{name}/infrastructure/{provider}
```

Returns the detailed overlap between one feed and one specific critical-infrastructure provider.

Example:

```
GET /api/v1/sets/firehol_level1/infrastructure/critical_dns_root_servers
```

Key response fields: matched IPs, matched ranges, overlap percentages, provider-set identity.

## Stale artifact handling

These endpoints are cache-first. They serve the published overlap artifacts that are present in the configured web artifact directory. Public requests do not regenerate missing or stale critical-infrastructure artifacts.

The daemon checks critical-infrastructure artifact consistency during pipeline and integrity work. If the configured provider set changes, the operator-facing integrity/admin surfaces are the place to confirm repair status.

## Feeds without comparison targets

Critical reference feeds and provider-context feeds do not get comparison targets. The infrastructure endpoints return `404` for these feeds because they are the reference set, not the feeds being compared against the reference.

IPv6 feeds also do not receive critical-infrastructure overlap artifacts in the current version.
