# Step by Step: Add a Feed

You will learn how to add a source feed to a catalog and validate it as an operator.

## Step 1: Choose the feed family

Most new feeds are **source feeds**: they download an IP or CIDR list from a URL.

If your input is different:

- A union or difference of existing feeds belongs in `merges/`.
- A time-windowed view of an existing feed uses the parent's `history` field.
- A multi-part upstream artifact belongs in `artifacts/` with child feeds using `artifact://`.
- A small local list can use `static:` in a normal source definition.

## Step 2: Choose a category

Use a category that matches the feed's operational meaning:

| Category | Use for |
|----------|---------|
| `intrusion` | Active hostile access attempts and exploitation traffic |
| `malware_infrastructure` | Malware command-and-control and distribution infrastructure |
| `messaging_abuse` | Spam and messaging abuse sources |
| `service_abuse` | Abusive service behavior and bot activity |
| `anonymizers` | Tor exits, VPN exits, relays, and open proxies |
| `scanners` | Internet scanners and reconnaissance sources |
| `policy_risk` | Policy-risk lists that may need local review before blocking |
| `provider_infrastructure` | Provider and critical-infrastructure reference data |
| `special_use` | Bogons, reserved ranges, and other special-use address space |

## Step 3: Create the YAML file

Place the file under the matching category directory:

```bash
configs/firehol/sources/<category>/<feedname>.yaml
```

Use lowercase names with underscores. Avoid path separators, commas, reserved filename characters (`: * ? " < > |`), control characters, and non-ASCII characters.

## Step 4: Write the source definition

```yaml
sources:
  example_blocklist:
    url: https://example.com/blocklist.txt
    frequency: 60
    ipv: ipv4
    output: ipset
    category: malware_infrastructure
    maintainer: Example Security Team
    maintainer_url: https://example.com/blocklist
    license: CC-BY-4.0
    redistributable: true
    attribution: |
      Data provided by Example Security Team under CC-BY-4.0.
      Source: https://example.com/blocklist
    info: '[Example Security Team](https://example.com/blocklist) malware infrastructure blocklist'
    processor:
      - remove_comments
      - extract_ipv4_cidr
```

Key fields:

| Field | Required | Meaning |
|-------|----------|---------|
| YAML key under `sources:` | Yes | Feed identity and URL slug |
| `url` | Yes for downloaded feeds | Download URL |
| `frequency` | Yes | Minutes between checks; `0` means no independent wall-clock cadence |
| `ipv` | Yes | Use `ipv4` for current feed processing. IPv6 feed processing is not complete in this release. |
| `output` | Yes | `ipset` for individual IPs, `netset` for CIDR ranges |
| `category` | Yes | Existing category key |
| `processor` | Usually | Normalization pipeline |
| `maintainer` | Recommended | Upstream maintainer name |
| `maintainer_url` | Recommended | Upstream maintainer or feed page |
| `license` | Recommended | Direct upstream license or terms summary |
| `redistributable` | Optional | Defaults to `true`; set `false` only when terms forbid republication |

## Step 5: Test locally

Start the daemon with the updated catalog:

```bash
update-ipsets daemon --config configs/firehol --enable-all \
  --listen :18888 \
  --admin-auth-mode=disabled --allow-unauthenticated-admin
```

In the admin UI:

1. Find the feed in the feed table.
2. Confirm it is enabled.
3. Trigger a recheck.
4. Wait for download and processing to complete.
5. Confirm the feed has entries, unless it is intentionally empty.

## Step 6: Validate public behavior

```bash
curl http://localhost:18888/api/v1/sets/example_blocklist
curl http://localhost:18888/api/v1/sets/example_blocklist/data
```

The raw data endpoint is expected to fail for hidden, archived, or non-redistributable feeds.

## Step 7: Check license and attribution

If the direct upstream requires attribution, include it in `attribution`. If redistribution is forbidden, set `redistributable: false`.

See [License Requirements](license-requirements.md).
