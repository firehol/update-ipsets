# Step by Step: Add a New Feed

You will learn how to add a new source feed to the catalog, from choosing the family to submitting the contribution.

## Step 1: Choose the feed family

Most new feeds are **plain source feeds** — they download an IP list from a URL.

If your feed is instead:

- A union or difference of existing feeds → add a **merge** in `merges/`
- A time-windowed view of an existing feed → that's a **history derivative**, configured via the parent's `retention` field
- A multi-part artifact that produces child feeds → add an **artifact parent** in `artifacts/`

This guide covers the most common case: adding a plain source feed.

## Step 2: Create the YAML file

Create a file in the appropriate category directory:

```bash
configs/firehol/sources/<category>/<feedname>.yaml
```

Use lowercase, underscores for spaces, and no special characters. Choose a category that matches the feed's purpose:

| Category | Description |
|----------|-------------|
| `web_reputation` | Web security, phishing, malware distribution |
| `abuse` | Spam, botnet command-and-control |
| `attacks` | Active attack sources |
| `malware` | Malware-related IPs |
| `reputation` | General reputation scores |
| `geolocation` | Geographic IP lists |
| `bogons` | Bogon and reserved address space |

If no category fits, use the closest match or discuss with maintainers.

## Step 3: Write the configuration

Here is a complete example for a typical source feed:

```yaml
name: example_blocklist
url: https://example.com/blocklist.txt
frequency: 3600
output: ipset
category: web_reputation
maintainer: [Example Security Team]
homepage: https://example.com/blocklist
license: CC-BY-4.0
redistributable: true
attribution: |
  Data provided by Example Security Team under CC-BY-4.0.
  Source: https://example.com/blocklist
processors:
  - strip_comments
  - strip_blank_lines
```

### Key fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Feed identity — lowercase, no spaces or special characters |
| `url` | Yes | Download URL (`https://`, `http://`, or `file:///`) |
| `frequency` | Yes | Update interval in seconds (0 = manual only) |
| `output` | Yes | `ipset` (one IP per line) or `netset` (CIDR per line) |
| `category` | Yes | Must match a category defined in `categories.yaml` |
| `maintainer` | Yes | List of maintainer names |
| `license` | Yes | SPDX identifier or freeform text |
| `redistributable` | No | Defaults to `true`. Set to `false` only if terms explicitly forbid redistribution |
| `attribution` | No | Required text that accompanies redistribution |
| `processors` | No | List of processing steps to normalize the input |
| `homepage` | No | Upstream project URL |

## Step 4: Test locally

Start the daemon with the updated config:

```bash
update-ipsets daemon --config configs/firehol --enable-all \
  --listen :18888 \
  --admin-auth-mode=disabled --allow-unauthenticated-admin
```

Check the admin UI at `http://localhost:18888/admin`:

1. Find your feed in the feed table
2. Verify it shows as enabled
3. Trigger a recheck from the feed's action menu
4. Wait for download and processing to complete
5. Confirm the feed shows IPs and has a `healthy` status

## Step 5: Validate

Confirm these points before submitting:

- The feed downloads without errors
- The feed produces a non-zero number of IPs (unless it is genuinely empty)
- The feed appears in the public API: `curl http://localhost:18888/api/v1/sets/<name>`
- The raw data is accessible: `curl http://localhost:18888/api/v1/sets/<name>/data`

## Step 6: Add license and attribution

If the upstream requires attribution, include the full text in the `attribution` field. Use SPDX license identifiers when possible (`MIT`, `CC-BY-4.0`, `Apache-2.0`, etc.).

See [License Requirements](license-requirements.md) for the full policy.

## Step 7: Submit

Push your changes and open a pull request. Include:

- A short description of the feed
- Confirmation that you tested it locally
- Any notes about the upstream license or terms
