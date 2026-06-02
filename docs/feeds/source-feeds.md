# Source Feeds

You will learn how to configure a direct upstream feed that downloads from HTTP, HTTPS, or a local file.

## What a source feed is

A source feed fetches content from an external URL on a fixed cadence, processes the raw download through a pipeline of transformations, and produces a normalized IP set.

The current feed pipeline is IPv4-oriented. Use `ipv: ipv4` for source feeds in normal operation. The standalone `iprange` CLI has IPv6 mode, but public feed search, enrichment, and critical-infrastructure overlap are IPv4-only in this release.

## Key fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | yes (YAML key) | Unique feed identifier — used as filename, URL slug, and reference key |
| `url` | yes | Download URL — `https://`, `http://`, or `file:///` |
| `frequency` | no | Minutes between automatic checks. If omitted or `0`, the source is not auto-scheduled. |
| `output` | yes | `ipset` (one IP per line) or `netset` (one CIDR per line) |
| `category` | yes for normal public feeds | Category key from `categories.yaml`; required for normal public taxonomy participation. |
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

If the real URL contains a secret, set `attributes.public_url` to the sanitized URL you want users and APIs to see.

## Custom download options

Most feeds use the default HTTP downloader. For form-style exports or APIs that require headers, set curl-like options under `attributes.downloader_options`:

```yaml
attributes:
  downloader_options: >-
    --data 'export_type=text'
    --header 'Accept: text/plain'
```

`downloader_options` are parsed literally. URL fields support environment
variable templates, but downloader option values are not environment-expanded.
Do not put real secrets in catalog YAML.

Supported options are:

- `--data`, `--data-raw`, or `-d`
- `--request` or `-X`
- `--referer`
- `--user` or `-u`
- `--header` or `-H`

The `--data=...`, `--request=...`, `--referer=...`, and `--user=...` forms are
also accepted. Header values must use the separated form, for example
`--header 'Accept: text/plain'`.

## Frequency

`frequency` sets the number of minutes between automatic checks. Common values:

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

The `processor` field sets the normalized-output pipeline. When `processor` is
present, it is the pipeline the daemon runs. `processor_raw` is retained as a
legacy catalog field: if `processor` is omitted, the daemon treats
`processor_raw` as one legacy processor name; otherwise it is preserved as
metadata for compatibility and auditing. It is not a separate raw-archive
pipeline.

See [Processor Reference](processors.md) for the full supported processor list and arguments.

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
```

This source checks every 10 minutes, produces a netset, declares history windows (1 day, 7 days, 30 days), and uses a custom DShield parser.

## File location

Each source feed lives in `sources/<category>/<name>.yaml`. The category subdirectory must match the `category:` field in the source definition.
