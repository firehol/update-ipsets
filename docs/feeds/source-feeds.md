# Source Feeds

You will learn how to configure a direct upstream feed that downloads from HTTP, HTTPS, or a local file.

## What a source feed is

A source feed fetches content from an external URL on a fixed cadence, processes the raw download through a pipeline of transformations, and produces a normalized IP set.

## Key fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | yes (YAML key) | Unique feed identifier — used as filename, URL slug, and reference key |
| `url` | yes | Download URL — `https://`, `http://`, or `file:///` |
| `frequency` | yes | Seconds between automatic checks. `0` means not auto-scheduled. |
| `output` | yes | `ipset` (one IP per line) or `netset` (one CIDR per line) |
| `category` | yes | Category key from `categories.yaml` |
| `processor` | yes | List of transformation steps applied to the download |
| `info` | recommended | Markdown description shown on the public feed-detail page |
| `license` | recommended | SPDX identifier or free-text license |
| `maintainer` | recommended | Feed maintainer name |

## URL types

| URL form | Behavior |
|----------|----------|
| `https://...` | Standard HTTPS download |
| `http://...` | HTTP download (use HTTPS when available) |
| `file:///absolute/path` | Read a local file |

Environment variable interpolation is supported in URLs:

```yaml
url: https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-ASN&license_key=${MAXMIND_LICENSE_KEY}&suffix=tar.gz
```

## Frequency

`frequency` sets the number of seconds between automatic checks. Common values:

| Value | Meaning |
|-------|---------|
| `5` | Check every 5 minutes (aggressive, for fast-changing feeds) |
| `30` | Every 30 minutes |
| `1440` | Daily |
| `10080` | Weekly |
| `0` | No auto-scheduling (static feeds, artifact children) |

A frequency of `0` does not mean the feed is dead. The scheduler still detects configuration changes and queues reprocessing when the source definition changes.

## Processors

Processors form a pipeline — each step transforms the data before passing it to the next. Common processors:

| Processor | Purpose |
|-----------|---------|
| `remove_comments` | Strip comment lines |
| `extract_ipv4_cidr` | Extract IPv4 CIDRs from structured text |
| `dshield_format` | Parse DShield block.txt format |
| `torproject_exits` | Parse Tor exit-addresses format |
| `passthrough` | No transformation |

The `processor` field sets the pipeline for the normalized output. The `processor_raw` field sets the pipeline for the raw download archive.

## Simple source example

```yaml
sources:
  feodo:
    license: CC0 1.0
    url: https://feodotracker.abuse.ch/downloads/ipblocklist_recommended.txt
    frequency: 30
    ipv: ipv4
    output: ipset
    processor:
      - remove_comments
    processor_raw: remove_comments
    category: malware_infrastructure
    info: '[Abuse.ch Feodo tracker](https://feodotracker.abuse.ch) trojan IP blocklist'
    maintainer: Abuse.ch
    maintainer_url: https://feodotracker.abuse.ch/
    enabled_by_all: true
```

## Complex source example

```yaml
sources:
  dshield:
    license: CC BY-NC-SA 4.0
    url: https://feeds.dshield.org/block.txt
    frequency: 10
    history:
      - 1440
      - 10080
      - 43200
    ipv: ipv4
    output: netset
    processor:
      - dshield_format
    processor_raw: dshield_parser
    category: intrusion
    info: '[DShield.org](https://dshield.org/) top 20 attacking class C subnets'
    maintainer: DShield.org
    maintainer_url: https://dshield.org/
    enabled_by_all: true
```

This source checks every 10 minutes, produces a netset, declares history windows (1 day, 7 days, 30 days), and uses a custom DShield parser.

## File location

Each source feed lives in `sources/<category>/<name>.yaml`. The category subdirectory must match the `category:` field in the source definition.
