# MCP Endpoint

You will learn how to connect AI agents and MCP-compatible tools to the update-ipsets service using the Model Context Protocol endpoint.

## Overview

The update-ipsets service exposes a public MCP endpoint at `/mcp` using the Streamable HTTP transport (MCP specification 2025-03-26). This allows AI agents, IDE integrations, and MCP-compatible tools to programmatically discover and analyze threat intelligence IP blocklists.

## Connecting

Point any MCP-compatible client at:

```
https://iplists.firehol.org/mcp
```

The endpoint supports:

- **POST** — send JSON-RPC messages (tool calls, initialization)
- **GET** — open SSE stream for server notifications
- **DELETE** — terminate an active session

The server manages sessions automatically via the `Mcp-Session-Id` header. Most MCP clients handle this transparently.

## Available tools

### `find_feeds` — discover feeds by metadata

Discover threat intelligence IP blocklists by searching and filtering public feed metadata. All filters are optional and combinable. Results are returned as markdown: a compact table for structured columns, followed by per-feed official name, maintainer, and description sections.

**Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `search` | string | Case-insensitive full-text search across feed names, official names, maintainers, short descriptions, and descriptions. Multiple terms must all match |
| `category` | string enum | Threat category values from the active public feed catalog |
| `maintainer` | string enum | Maintainer values from the active public feed catalog; matching is case-insensitive |
| `provenance` | string enum | Origin: `primary`, `secondary_upstream`, `secondary_merge`, `secondary_retention` |
| `health` | string enum | Feed health: `healthy`, `delayed`, `risky`, `archived`, `unmaintained`, `empty`, `unavailable` |
| `freshness` | string enum | Time since last update: `hour`, `day`, `week`, `month`, `older` |
| `cadence` | string enum | Update frequency: `hourly`, `daily`, `weekly`, `monthly`, `slower`, `unknown` |
| `uniqueness` | string enum | IP uniqueness vs other feeds: `very_high` (>=50%), `high` (20-50%), `medium` (5-20%), `low` (<5%), `unknown` |
| `license` | string enum | License values from the active public feed catalog |
| `redistributable` | string enum | `true`, `false` |
| `critical` | string enum | Critical infrastructure tier: `hard`, `soft`, `contextual` |
| `size_min` | number | Minimum unique IP count |
| `size_max` | number | Maximum unique IP count |

The table output includes a description column populated from the researched short description when available. Dates are formatted as RFC3339 UTC with relative age in parentheses, for example `2026-05-04T12:00:00Z (12d ago)`. `unique_share_pct` is formatted with one fractional digit and a percent sign.

**Example** — find all actively maintained anonymizer feeds:

```json
{
  "category": "anonymizers",
  "health": "healthy"
}
```

### `fetch_analysis` — get a full analysis page

Fetch a detailed markdown analysis page for a specific entity. Use `find_feeds` first to discover available feeds, then `fetch_analysis` to get the full analysis.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `type` | string | yes | Entity type: `feed`, `country`, `asn`, or `maintainer` |
| `name` | string | yes | Entity identifier (see below) |

**Name format by type:**

| Type | Format | Examples |
|------|--------|----------|
| `feed` | Feed name from `find_feeds` | `firehol_level1`, `tor_exits`, `spamhaus_drop` |
| `country` | ISO 3166-1 alpha-2 code | `US`, `CN`, `DE`, `BR` |
| `asn` | ASN number | `13335`, `14618`, `16509` |
| `maintainer` | Maintainer slug | `firehol`, `spamhaus`, `emergingthreats` |

**Example** — get analysis for the Tor exits feed:

```json
{
  "type": "feed",
  "name": "tor_exits"
}
```

The response is a pre-generated markdown document containing statistics, historical trends, geographic distribution, overlap with other feeds, and other analytical data. Feed markdown uses the configured default ASN and GeoIP providers, rolls retention tables into non-zero daily buckets with omitted days documented as zero, explains overlap percentages for both the current feed and the compared feed, and omits blank technical-spec rows.

Feed markdown is designed to be self-contained for AI clients while matching the public feed page semantics. The header exposes `Tracked since` from feed metadata; behavior history is a bounded retained artifact and must not be treated as proof of first publication.

The behavior section shows how the list moves over time using published history and changeset artifacts:

- additions, removals, and churn are shown for retained content changes; large activity windows are rolled up only to buckets that are not smaller than the feed's configured or observed cadence
- cadence reports gaps between published history timestamps/content-changing observations, not source checks or HTTP requests
- when no retained content changes are available, the markdown says so explicitly instead of emitting a zero-counter activity table

## Rate limits

The MCP endpoint shares the general API rate limit: **240 requests per minute per client IP**.

This is the same limit applied to `/api/v1/sets` and other read-only metadata endpoints. The stricter 10/min search rate limit does not apply to MCP tools because they read precomputed metadata, not scan live feed entries.

## CORS

The MCP endpoint sets CORS headers that allow cross-origin access from browsers:

```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, POST, DELETE, OPTIONS
Access-Control-Allow-Headers: Content-Type, Mcp-Session-Id, MCP-Protocol-Version, Last-Event-ID
Access-Control-Expose-Headers: Mcp-Session-Id
```

This enables browser-based MCP clients to connect directly.

## Security

- The MCP endpoint is **read-only**. No tool can modify server state or trigger computation.
- `fetch_analysis` serves pre-generated markdown artifacts — no on-demand generation.
- The endpoint does not expose admin operations, internal state, or configuration.
- Path traversal is guarded: entity names containing `..`, `/`, or `\` are rejected.

## Client examples

### Claude Desktop

Add to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "iplists": {
      "url": "https://iplists.firehol.org/mcp"
    }
  }
}
```

### opencode / other Streamable HTTP clients

Configure the MCP endpoint URL:

```
https://iplists.firehol.org/mcp
```

## Next steps

- [API Overview](api-overview.md) — REST API endpoints for direct HTTP access
- [Rate Limits and CORS](rate-limits-cors.md) — detailed rate limit and CORS policy
- [Feed Endpoints](feed-endpoints.md) — feed catalog and detail REST endpoints
